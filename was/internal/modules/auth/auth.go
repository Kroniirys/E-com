package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/limit"
	"ecommerce.local/base/internal/platform/problem"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const AccessTTL = 15 * time.Minute
const RefreshTTL = 7 * 24 * time.Hour
const issuer = "ecommerce-was"
const audience = "ecommerce-viewer"

type Service struct {
	DB        *gorm.DB
	Secret    []byte
	Limiter   limit.Limiter
	dummyHash []byte
}
type Claims struct {
	SessionID string `json:"sid"`
	Version   int64  `json:"ver"`
	jwt.RegisteredClaims
}
type Session struct {
	ID          string
	UserID      string
	UserVersion int64
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}

func (Session) TableName() string { return "auth_sessions" }

type RefreshToken struct {
	ID         string
	FamilyID   string
	TokenHash  string
	ExpiresAt  time.Time
	UsedAt     *time.Time
	ReplacedBy *string
	CreatedAt  time.Time
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

type Principal struct {
	User      users.User
	SessionID string
	Version   int64
}
type Tokens struct {
	AccessToken    string    `json:"access_token"`
	TokenType      string    `json:"token_type"`
	ExpiresIn      int       `json:"expires_in"`
	Refresh        string    `json:"-"`
	RefreshExpires time.Time `json:"-"`
}

func New(db *gorm.DB, secret string, limiter limit.Limiter) (*Service, error) {
	if len(secret) < 32 || limiter == nil {
		return nil, errors.New("auth requires a secret and rate limiter")
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte(uuid.NewString()), 12)
	if err != nil {
		return nil, err
	}
	return &Service{db, []byte(secret), limiter, dummy}, nil
}
func HashRefresh(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func opaque() (string, error) {
	var bytes [32]byte
	_, err := rand.Read(bytes[:])
	return base64.RawURLEncoding.EncodeToString(bytes[:]), err
}
func (s *Service) Rate(ctx context.Context, scope, key string, max int) (int, error) {
	ok, retry, err := s.Limiter.Allow(ctx, scope, key, max)
	if err != nil {
		return 0, problem.New(503, "AUTH_UNAVAILABLE", "Authentication is temporarily unavailable")
	}
	if !ok {
		p := problem.New(429, "RATE_LIMITED", "Too many attempts; try again later")
		p.RetryAfter = retry
		return retry, p
	}
	return 0, nil
}
func (s *Service) Register(ctx context.Context, input users.Input) (users.User, error) {
	input.Role = "Customer"
	input.Status = "Active"
	if err := users.Validate(&input); err != nil {
		return users.User{}, err
	}
	if !users.ValidPassword(input.Password) {
		return users.User{}, problem.Invalid("Password must contain at least 12 characters and at most 72 UTF-8 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return users.User{}, err
	}
	return users.Insert(s.DB.WithContext(ctx), input, string(hash))
}
func (s *Service) issue(user users.User, session Session, refresh string) (Tokens, error) {
	now := time.Now().UTC()
	claims := Claims{SessionID: session.ID, Version: user.SessionVersion, RegisteredClaims: jwt.RegisteredClaims{Issuer: issuer, Subject: user.ID, Audience: jwt.ClaimStrings{audience}, ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)), IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ID: uuid.NewString()}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.Secret)
	return Tokens{AccessToken: token, TokenType: "Bearer", ExpiresIn: int(AccessTTL.Seconds()), Refresh: refresh, RefreshExpires: session.ExpiresAt}, err
}
func (s *Service) Login(ctx context.Context, email, password string) (Tokens, error) {
	var candidate users.User
	err := users.Query(s.DB.WithContext(ctx)).Where("u.email=?", NormalizeEmail(email)).Take(&candidate).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return Tokens{}, err
	}
	hash := s.dummyHash
	if err == nil {
		hash = []byte(candidate.PasswordHash)
	}
	valid := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	if !valid || err != nil || candidate.Status != "Active" {
		return Tokens{}, problem.Unauthorized()
	}
	refresh, err := opaque()
	if err != nil {
		return Tokens{}, err
	}
	var result Tokens
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, e := users.Locked(tx, candidate.ID)
		if e != nil {
			return e
		}
		if user.Status != "Active" || user.PasswordHash != candidate.PasswordHash {
			return problem.Unauthorized()
		}
		session := Session{ID: uuid.NewString(), UserID: user.ID, UserVersion: user.SessionVersion, ExpiresAt: time.Now().UTC().Add(RefreshTTL)}
		if e := tx.Create(&session).Error; e != nil {
			return e
		}
		record := RefreshToken{ID: uuid.NewString(), FamilyID: session.ID, TokenHash: HashRefresh(refresh), ExpiresAt: session.ExpiresAt}
		if e := tx.Create(&record).Error; e != nil {
			return e
		}
		result, e = s.issue(user, session, refresh)
		return e
	})
	return result, err
}
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
func (s *Service) Refresh(ctx context.Context, raw string) (Tokens, error) {
	if len(raw) != 43 {
		return Tokens{}, problem.Unauthorized()
	}
	var initial RefreshToken
	if err := s.DB.WithContext(ctx).Where("token_hash=?", HashRefresh(raw)).Take(&initial).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Tokens{}, problem.Unauthorized()
		}
		return Tokens{}, err
	}
	var initialSession Session
	if err := s.DB.WithContext(ctx).Where("id=?", initial.FamilyID).Take(&initialSession).Error; err != nil {
		return Tokens{}, err
	}
	if _, err := s.Rate(ctx, "refresh-family", initial.FamilyID, 20); err != nil {
		return Tokens{}, err
	}
	var result Tokens
	var authError error
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, e := users.Locked(tx, initialSession.UserID)
		if e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				authError = problem.Unauthorized()
				return nil
			}
			return e
		}
		var session Session
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", initial.FamilyID).Take(&session).Error; e != nil {
			return e
		}
		var token RefreshToken
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", initial.ID).Take(&token).Error; e != nil {
			return e
		}
		now := time.Now().UTC()
		if token.UsedAt != nil || session.RevokedAt != nil || !token.ExpiresAt.After(now) || !session.ExpiresAt.After(now) || user.Status != "Active" || user.SessionVersion != session.UserVersion {
			if e := tx.Model(&Session{}).Where("id=? AND revoked_at IS NULL", session.ID).Update("revoked_at", now).Error; e != nil {
				return e
			}
			authError = problem.Unauthorized()
			return nil
		}
		nextRaw, e := opaque()
		if e != nil {
			return e
		}
		next := RefreshToken{ID: uuid.NewString(), FamilyID: session.ID, TokenHash: HashRefresh(nextRaw), ExpiresAt: session.ExpiresAt}
		if e := tx.Create(&next).Error; e != nil {
			return e
		}
		if e := tx.Model(&RefreshToken{}).Where("id=?", token.ID).Updates(map[string]any{"used_at": now, "replaced_by": next.ID}).Error; e != nil {
			return e
		}
		result, e = s.issue(user, session, nextRaw)
		return e
	})
	if err != nil {
		return Tokens{}, err
	}
	if authError != nil {
		return Tokens{}, authError
	}
	return result, nil
}
func (s *Service) Authenticate(ctx context.Context, raw string) (Principal, error) {
	claims := Claims{}
	parsed, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) { return s.Secret, nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(issuer), jwt.WithAudience(audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !parsed.Valid || !users.ValidID(claims.Subject) || !users.ValidID(claims.SessionID) {
		return Principal{}, problem.Unauthorized()
	}
	user, err := users.Get(s.DB.WithContext(ctx), claims.Subject)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Principal{}, problem.Unauthorized()
		}
		return Principal{}, err
	}
	if user.Status != "Active" || user.SessionVersion != claims.Version {
		return Principal{}, problem.Unauthorized()
	}
	var session Session
	err = s.DB.WithContext(ctx).Where("id=? AND user_id=? AND user_version=? AND revoked_at IS NULL AND expires_at > ?", claims.SessionID, user.ID, user.SessionVersion, time.Now().UTC()).Take(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Principal{}, problem.Unauthorized()
		}
		return Principal{}, err
	}
	return Principal{user, session.ID, claims.Version}, nil
}
func (s *Service) Logout(ctx context.Context, raw string) error {
	if len(raw) != 43 {
		return nil
	}
	var token RefreshToken
	if err := s.DB.WithContext(ctx).Where("token_hash=?", HashRefresh(raw)).Take(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	var session Session
	if err := s.DB.WithContext(ctx).Where("id=?", token.FamilyID).Take(&session).Error; err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner users.User
		if err := tx.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", session.UserID).Take(&owner).Error; err != nil {
			return err
		}
		return tx.Model(&Session{}).Where("id=? AND revoked_at IS NULL", session.ID).Update("revoked_at", time.Now().UTC()).Error
	})
}
func (s *Service) ChangePassword(ctx context.Context, principal Principal, current, next string) error {
	if !users.ValidPassword(next) {
		return problem.Invalid("Password must contain at least 12 characters and at most 72 UTF-8 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), 12)
	if err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := users.Locked(tx, principal.User.ID)
		if err != nil {
			return err
		}
		if user.SessionVersion != principal.Version || user.Status != "Active" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
			return problem.Unauthorized()
		}
		var session Session
		if err := tx.Where("id=? AND revoked_at IS NULL AND expires_at > ?", principal.SessionID, time.Now().UTC()).Take(&session).Error; err != nil {
			return problem.Unauthorized()
		}
		if err := tx.Table("users").Where("id=?", user.ID).Updates(map[string]any{"password_hash": string(hash), "session_version": gorm.Expr("session_version+1"), "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		return tx.Model(&Session{}).Where("user_id=? AND revoked_at IS NULL", user.ID).Update("revoked_at", time.Now().UTC()).Error
	})
}
