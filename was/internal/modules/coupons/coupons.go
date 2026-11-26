package coupons

import (
	"context"
	"crypto/sha256"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/money"
	"ecommerce.local/base/internal/platform/problem"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"regexp"
	"strings"
	"time"
)

type Coupon struct {
	ID           string     `json:"id"`
	Code         string     `json:"code"`
	Kind         string     `json:"kind"`
	Value        int64      `json:"value"`
	MinimumMinor int64      `json:"minimum_minor"`
	ExpiresAt    time.Time  `json:"expires_at"`
	UsageLimit   int64      `json:"usage_limit"`
	UsedCount    int64      `json:"used_count"`
	Enabled      bool       `json:"enabled"`
	Version      int64      `json:"version"`
	DeletedAt    *time.Time `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"-"`
}
type Input struct {
	Code         string    `json:"code"`
	Kind         string    `json:"kind"`
	Value        int64     `json:"value"`
	MinimumMinor int64     `json:"minimum_minor"`
	ExpiresAt    time.Time `json:"expires_at"`
	UsageLimit   int64     `json:"usage_limit"`
	Enabled      bool      `json:"enabled"`
}
type Snapshot struct {
	ID            string    `json:"id"`
	Code          string    `json:"code"`
	Kind          string    `json:"kind"`
	Value         int64     `json:"value"`
	MinimumMinor  int64     `json:"minimum_minor"`
	ExpiresAt     time.Time `json:"expires_at"`
	DiscountMinor int64     `json:"discount_minor"`
	Version       int64     `json:"version"`
}
type Preview struct {
	CartQuoteHash string `json:"cart_quote_hash"`
	Code          string `json:"code"`
	CartVersion   int64  `json:"cart_version"`
	QuoteHash     string `json:"quote_hash"`
	SubtotalMinor int64  `json:"subtotal_minor"`
	ShippingMinor int64  `json:"shipping_minor"`
	DiscountMinor int64  `json:"discount_minor"`
	TotalMinor    int64  `json:"total_minor"`
}
type Service struct{ DB *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db} }

var validCode = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)

func Normalize(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }
func unavailable() error {
	return problem.New(409, "COUPON_UNAVAILABLE", "Coupon is disabled, expired, exhausted or below its minimum")
}
func Validate(in *Input) error {
	in.Code = Normalize(in.Code)
	in.ExpiresAt = in.ExpiresAt.UTC()
	if !validCode.MatchString(in.Code) || (in.Kind != "Fixed" && in.Kind != "Percentage") || in.Value < 1 || in.Value > money.Max || (in.Kind == "Percentage" && in.Value > 100) || in.MinimumMinor < 0 || in.MinimumMinor > money.Max || in.UsageLimit < 1 || in.UsageLimit > money.Max || in.ExpiresAt.IsZero() {
		return problem.Invalid("Invalid coupon code, value, minimum, expiration or limit")
	}
	return nil
}
func Discount(c Coupon, subtotal int64, now time.Time) (int64, error) {
	if subtotal < 0 || subtotal > money.Max {
		return 0, problem.Invalid("Subtotal exceeds the money range")
	}
	if !c.Enabled || c.DeletedAt != nil || !c.ExpiresAt.After(now) || c.UsedCount >= c.UsageLimit || subtotal < c.MinimumMinor {
		return 0, unavailable()
	}
	if c.Kind == "Percentage" {
		if c.Value < 1 || c.Value > 100 {
			return 0, problem.Invalid("Percentage must be between 1 and 100")
		}
		return subtotal/100*c.Value + (subtotal%100*c.Value)/100, nil
	}
	if c.Kind != "Fixed" || c.Value < 1 || c.Value > money.Max {
		return 0, problem.Invalid("Invalid fixed discount")
	}
	if c.Value > subtotal {
		return subtotal, nil
	}
	return c.Value, nil
}
func Find(tx *gorm.DB, code string, lock bool) (Coupon, error) {
	var c Coupon
	code = Normalize(code)
	if !validCode.MatchString(code) {
		return c, problem.Invalid("Invalid coupon code")
	}
	q := tx.Where("code=? AND deleted_at IS NULL", code)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.Take(&c).Error
	if err == gorm.ErrRecordNotFound {
		return c, unavailable()
	}
	return c, err
}
func Apply(c Coupon, baseHash string, version, subtotal, shipping int64, now time.Time) (Preview, Snapshot, error) {
	discount, err := Discount(c, subtotal, now)
	if err != nil {
		return Preview{}, Snapshot{}, err
	}
	total, err := money.Add(subtotal-discount, shipping)
	if err != nil {
		return Preview{}, Snapshot{}, err
	}
	snapshot := Snapshot{c.ID, c.Code, c.Kind, c.Value, c.MinimumMinor, c.ExpiresAt, discount, c.Version}
	raw, _ := json.Marshal(struct {
		Base   string
		Coupon Snapshot
		Total  int64
	}{baseHash, snapshot, total})
	sum := sha256.Sum256(raw)
	return Preview{baseHash, c.Code, version, hex.EncodeToString(sum[:]), subtotal, shipping, discount, total}, snapshot, nil
}
func Consume(tx *gorm.DB, c Coupon, userID, orderID string, discount int64) error {
	r := tx.Exec("UPDATE coupons SET used_count=used_count+1 WHERE id=? AND enabled AND deleted_at IS NULL AND used_count<usage_limit AND expires_at>clock_timestamp()", c.ID)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return unavailable()
	}
	return tx.Exec("INSERT INTO coupon_usages(id,coupon_id,user_id,order_id,discount_minor) VALUES (?,?,?,?,?)", uuid.NewString(), c.ID, userID, orderID, discount).Error
}
func (s *Service) Save(ctx context.Context, actor, id string, in Input) (Coupon, error) {
	if err := Validate(&in); err != nil {
		return Coupon{}, err
	}
	if id != "" && !users.ValidID(id) {
		return Coupon{}, problem.Missing()
	}
	var c Coupon
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := users.AdminGuard(tx, actor); e != nil {
			return e
		}
		if id == "" {
			c = Coupon{ID: uuid.NewString(), Code: in.Code, Kind: in.Kind, Value: in.Value, MinimumMinor: in.MinimumMinor, ExpiresAt: in.ExpiresAt, UsageLimit: in.UsageLimit, Enabled: in.Enabled, Version: 1}
			return tx.Create(&c).Error
		}
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).Take(&c).Error; e != nil {
			return e
		}
		if in.UsageLimit < c.UsedCount {
			return problem.New(409, "COUPON_LIMIT_CONFLICT", "Limit cannot be smaller than the consumed usage count")
		}
		if e := tx.Model(&Coupon{}).Where("id=?", id).Updates(map[string]any{"code": in.Code, "kind": in.Kind, "value": in.Value, "minimum_minor": in.MinimumMinor, "expires_at": in.ExpiresAt, "usage_limit": in.UsageLimit, "enabled": in.Enabled, "version": c.Version + 1, "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		c = Coupon{}
		return tx.Where("id=?", id).Take(&c).Error
	})
	return c, err
}
func (s *Service) Get(ctx context.Context, id string) (Coupon, error) {
	var c Coupon
	if !users.ValidID(id) {
		return c, problem.Missing()
	}
	err := s.DB.WithContext(ctx).Where("id=? AND deleted_at IS NULL", id).Take(&c).Error
	return c, err
}
func (s *Service) List(ctx context.Context, p users.Page) (users.List[Coupon], error) {
	v := users.List[Coupon]{Items: []Coupon{}, Page: p.Number, PageSize: p.Size}
	q := s.DB.WithContext(ctx).Model(&Coupon{}).Where("deleted_at IS NULL")
	if p.Query != "" {
		q = q.Where("code ILIKE ?", users.Like(p.Query))
	}
	sort := map[string]string{"": "created_at DESC,id DESC", "created_at": "created_at DESC,id DESC", "code": "code ASC,id ASC"}[p.Sort]
	if sort == "" {
		return v, problem.Invalid("Unsupported coupon sort")
	}
	if e := q.Count(&v.Total).Error; e != nil {
		return v, e
	}
	e := q.Order(sort).Limit(p.Size).Offset((p.Number - 1) * p.Size).Find(&v.Items).Error
	return v, e
}
func (s *Service) Archive(ctx context.Context, actor, id string) error {
	if !users.ValidID(id) {
		return problem.Missing()
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := users.AdminGuard(tx, actor); e != nil {
			return e
		}
		r := tx.Model(&Coupon{}).Where("id=? AND deleted_at IS NULL", id).Updates(map[string]any{"deleted_at": time.Now().UTC(), "enabled": false, "version": gorm.Expr("version+1"), "updated_at": time.Now().UTC()})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return problem.Missing()
		}
		return nil
	})
}
