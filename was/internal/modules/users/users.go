package users

import (
	"context"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"ecommerce.local/base/internal/platform/problem"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const CustomerRoleID = "00000000-0000-0000-0000-000000000001"
const AdminRoleID = "00000000-0000-0000-0000-000000000002"

type User struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	Name           string     `json:"name"`
	PasswordHash   string     `json:"-"`
	RoleID         string     `json:"-"`
	Role           string     `json:"role"`
	Status         string     `json:"status"`
	SessionVersion int64      `json:"-"`
	DeletedAt      *time.Time `json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
type Input struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	Password string `json:"password,omitempty"`
}
type Address struct {
	ID        string    `json:"id"`
	UserID    string    `json:"-"`
	Recipient string    `json:"recipient"`
	Phone     string    `json:"phone"`
	Address   string    `json:"address"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (Address) TableName() string { return "user_addresses" }

type AddressInput struct {
	Recipient string `json:"recipient"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
	IsDefault bool   `json:"is_default"`
}
type Page struct {
	Number     int
	Size       int
	Query      string
	Sort       string
	CategoryID string
}
type List[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}
type Service struct{ DB *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db} }
func ValidID(id string) bool   { _, err := uuid.Parse(id); return err == nil && len(id) == 36 }
func ValidPassword(password string) bool {
	return utf8.RuneCountInString(password) >= 12 && len(password) <= 72
}
func Like(q string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
}
func RoleID(role string) string {
	if role == "Admin" {
		return AdminRoleID
	}
	return CustomerRoleID
}
func Validate(input *Input) error {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)
	addr, err := mail.ParseAddress(input.Email)
	if err != nil || addr.Address != input.Email || len(input.Email) > 254 {
		return problem.Invalid("A valid email address is required")
	}
	if n := utf8.RuneCountInString(input.Name); n < 1 || n > 120 {
		return problem.Invalid("Name must contain 1 to 120 characters")
	}
	if input.Role != "Customer" && input.Role != "Admin" {
		return problem.Invalid("Role must be Customer or Admin")
	}
	if input.Status != "Active" && input.Status != "Disabled" {
		return problem.Invalid("Status must be Active or Disabled")
	}
	return nil
}
func Query(db *gorm.DB) *gorm.DB {
	return db.Table("users AS u").Select("u.*, roles.code AS role").Joins("JOIN roles ON roles.id=u.role_id").Where("u.deleted_at IS NULL")
}
func Get(db *gorm.DB, id string) (User, error) {
	var user User
	if !ValidID(id) {
		return user, problem.Missing()
	}
	err := Query(db).Where("u.id=?", id).Take(&user).Error
	return user, err
}
func Locked(db *gorm.DB, id string) (User, error) {
	var user User
	err := db.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).Take(&user).Error
	return user, err
}
func Insert(db *gorm.DB, input Input, passwordHash string) (User, error) {
	user := User{ID: uuid.NewString(), Email: input.Email, Name: input.Name, RoleID: RoleID(input.Role), Role: input.Role, Status: input.Status, SessionVersion: 1}
	err := db.Table("users").Create(map[string]any{"id": user.ID, "email": user.Email, "name": user.Name, "password_hash": passwordHash, "role_id": user.RoleID, "status": user.Status}).Error
	if err != nil {
		return User{}, err
	}
	return Get(db, user.ID)
}
func (s *Service) Me(ctx context.Context, id string) (User, error) {
	return Get(s.DB.WithContext(ctx), id)
}
func (s *Service) Profile(ctx context.Context, id, name string) (User, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 {
		return User{}, problem.Invalid("Name must contain 1 to 120 characters")
	}
	if err := s.DB.WithContext(ctx).Table("users").Where("id=? AND deleted_at IS NULL", id).Updates(map[string]any{"name": name, "updated_at": time.Now().UTC()}).Error; err != nil {
		return User{}, err
	}
	return Get(s.DB.WithContext(ctx), id)
}
func (s *Service) List(ctx context.Context, page Page) (List[User], error) {
	result := List[User]{Items: []User{}, Page: page.Number, PageSize: page.Size}
	q := Query(s.DB.WithContext(ctx))
	if page.Query != "" {
		like := Like(page.Query)
		q = q.Where("u.name ILIKE ? OR u.email ILIKE ?", like, like)
	}
	if err := q.Count(&result.Total).Error; err != nil {
		return result, err
	}
	order := map[string]string{"": "u.created_at DESC, u.id DESC", "created_at": "u.created_at DESC, u.id DESC", "name": "u.name ASC, u.id ASC", "email": "u.email ASC, u.id ASC"}[page.Sort]
	if order == "" {
		return result, problem.Invalid("Unsupported user sort")
	}
	err := q.Order(order).Limit(page.Size).Offset((page.Number - 1) * page.Size).Find(&result.Items).Error
	return result, err
}
func (s *Service) AdminGet(ctx context.Context, id string) (User, error) {
	return Get(s.DB.WithContext(ctx), id)
}
func AdminGuard(tx *gorm.DB, actorID string) error {
	var role struct{ ID string }
	if err := tx.Table("roles").Clauses(clause.Locking{Strength: "UPDATE"}).Where("code='Admin'").Take(&role).Error; err != nil {
		return err
	}
	actor, err := Get(tx, actorID)
	if err != nil {
		return err
	}
	if actor.Role != "Admin" || actor.Status != "Active" {
		return problem.Forbidden()
	}
	return nil
}
func (s *Service) AdminCreate(ctx context.Context, actorID string, input Input) (User, error) {
	if err := Validate(&input); err != nil {
		return User{}, err
	}
	if !ValidPassword(input.Password) {
		return User{}, problem.Invalid("Password must contain at least 12 characters and at most 72 UTF-8 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return User{}, err
	}
	var result User
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := AdminGuard(tx, actorID); err != nil {
			return err
		}
		var e error
		result, e = Insert(tx, input, string(hash))
		return e
	})
	return result, err
}
func (s *Service) AdminUpdate(ctx context.Context, actorID, id string, input Input, remove bool) (User, error) {
	if !ValidID(id) {
		return User{}, problem.Missing()
	}
	if !remove {
		if err := Validate(&input); err != nil {
			return User{}, err
		}
	}
	var result User
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := AdminGuard(tx, actorID); err != nil {
			return err
		}
		old, err := Locked(tx, id)
		if err != nil {
			return err
		}
		nextRole, nextStatus := input.Role, input.Status
		if remove {
			nextRole = "Customer"
			nextStatus = "Disabled"
		}
		if old.RoleID == AdminRoleID && old.Status == "Active" && (nextRole != "Admin" || nextStatus != "Active" || remove) {
			var count int64
			if err := tx.Table("users").Where("role_id=? AND status='Active' AND deleted_at IS NULL", AdminRoleID).Count(&count).Error; err != nil {
				return err
			}
			if count <= 1 {
				return problem.New(409, "LAST_ADMIN", "The last active admin cannot be disabled or demoted")
			}
		}
		changes := map[string]any{"updated_at": time.Now().UTC(), "session_version": gorm.Expr("session_version+1")}
		if remove {
			changes["deleted_at"] = time.Now().UTC()
			changes["status"] = "Disabled"
		} else {
			changes["email"] = input.Email
			changes["name"] = input.Name
			changes["role_id"] = RoleID(input.Role)
			changes["status"] = input.Status
		}
		if err := tx.Table("users").Where("id=?", id).Updates(changes).Error; err != nil {
			return err
		}
		if remove {
			return nil
		}
		result, err = Get(tx, id)
		return err
	})
	return result, err
}
func validateAddress(in *AddressInput) error {
	in.Recipient = strings.TrimSpace(in.Recipient)
	in.Phone = strings.TrimSpace(in.Phone)
	in.Address = strings.TrimSpace(in.Address)
	if n := utf8.RuneCountInString(in.Recipient); n < 1 || n > 120 {
		return problem.Invalid("Recipient must contain 1 to 120 characters")
	}
	if n := len(in.Phone); n < 8 || n > 20 {
		return problem.Invalid("Phone must contain 8 to 20 characters")
	}
	for _, c := range in.Phone {
		if !strings.ContainsRune("+0123456789 ()-", c) {
			return problem.Invalid("Phone contains unsupported characters")
		}
	}
	if n := utf8.RuneCountInString(in.Address); n < 1 || n > 500 {
		return problem.Invalid("Address must contain 1 to 500 characters")
	}
	return nil
}
func (s *Service) Addresses(ctx context.Context, userID string, page Page) (List[Address], error) {
	result := List[Address]{Items: []Address{}, Page: page.Number, PageSize: page.Size}
	q := s.DB.WithContext(ctx).Model(&Address{}).Where("user_id=?", userID)
	if err := q.Count(&result.Total).Error; err != nil {
		return result, err
	}
	err := q.Order("is_default DESC, created_at DESC, id DESC").Limit(page.Size).Offset((page.Number - 1) * page.Size).Find(&result.Items).Error
	return result, err
}
func (s *Service) SaveAddress(ctx context.Context, userID, id string, input AddressInput) (Address, error) {
	if err := validateAddress(&input); err != nil {
		return Address{}, err
	}
	if id != "" && !ValidID(id) {
		return Address{}, problem.Missing()
	}
	var result Address
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := Locked(tx, userID); err != nil {
			return err
		}
		if id != "" {
			if err := tx.Where("id=? AND user_id=?", id, userID).Take(&result).Error; err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Model(&Address{}).Where("user_id=?", userID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			input.IsDefault = true
		}
		if input.IsDefault {
			if err := tx.Model(&Address{}).Where("user_id=?", userID).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		if id == "" {
			id = uuid.NewString()
			result = Address{ID: id, UserID: userID, Recipient: input.Recipient, Phone: input.Phone, Address: input.Address, IsDefault: input.IsDefault}
			return tx.Create(&result).Error
		}
		changes := map[string]any{"recipient": input.Recipient, "phone": input.Phone, "address": input.Address, "is_default": input.IsDefault, "updated_at": time.Now().UTC()}
		if err := tx.Model(&Address{}).Where("id=? AND user_id=?", id, userID).Updates(changes).Error; err != nil {
			return err
		}
		return tx.Where("id=?", id).Take(&result).Error
	})
	return result, err
}
func (s *Service) DeleteAddress(ctx context.Context, userID, id string) error {
	if !ValidID(id) {
		return problem.Missing()
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := Locked(tx, userID); err != nil {
			return err
		}
		result := tx.Where("id=? AND user_id=?", id, userID).Delete(&Address{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return problem.Missing()
		}
		return nil
	})
}
