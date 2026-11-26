package inventory

import (
	"context"
	"crypto/sha256"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/problem"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxQuantity int64 = 9000000000000

var validKey = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

type Stock struct {
	ProductName string    `json:"product_name,omitempty" gorm:"->"`
	SKU         string    `json:"sku,omitempty" gorm:"->"`
	ProductID   string    `json:"product_id"`
	Quantity    int64     `json:"quantity"`
	Version     int64     `json:"version"`
	UpdatedAt   time.Time `json:"-"`
}

func (Stock) TableName() string { return "inventory" }

type Transaction struct {
	ID             string    `json:"id"`
	ProductID      string    `json:"product_id"`
	ActorID        string    `json:"actor_id"`
	Delta          int64     `json:"delta"`
	BeforeQuantity int64     `json:"before"`
	AfterQuantity  int64     `json:"after"`
	Reason         string    `json:"reason"`
	Reference      string    `json:"reference"`
	CreatedAt      time.Time `json:"created_at"`
}

func (Transaction) TableName() string { return "inventory_transactions" }

type Adjustment struct {
	ProductID string `json:"product_id"`
	Delta     int64  `json:"delta"`
	Reason    string `json:"reason"`
}
type record struct {
	ID           string
	PayloadHash  string
	ResponseBody json.RawMessage
}
type Service struct{ DB *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db} }
func EnsureZero(tx *gorm.DB, productID string) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Stock{ProductID: productID, Version: 1}).Error
}
func (s *Service) List(ctx context.Context, page users.Page) (users.List[Stock], error) {
	result := users.List[Stock]{Items: []Stock{}, Page: page.Number, PageSize: page.Size}
	q := s.DB.WithContext(ctx).Table("inventory AS i").Select("i.*, p.name AS product_name, p.sku AS sku").Joins("JOIN products p ON p.id=i.product_id").Where("p.deleted_at IS NULL")
	if err := q.Count(&result.Total).Error; err != nil {
		return result, err
	}
	err := q.Order("i.product_id ASC").Limit(page.Size).Offset((page.Number - 1) * page.Size).Find(&result.Items).Error
	return result, err
}
func (s *Service) History(ctx context.Context, id string, page users.Page) (users.List[Transaction], error) {
	result := users.List[Transaction]{Items: []Transaction{}, Page: page.Number, PageSize: page.Size}
	if !users.ValidID(id) {
		return result, problem.Missing()
	}
	var count int64
	if err := s.DB.WithContext(ctx).Table("products").Where("id=? AND deleted_at IS NULL", id).Count(&count).Error; err != nil {
		return result, err
	}
	if count == 0 {
		return result, problem.Missing()
	}
	q := s.DB.WithContext(ctx).Model(&Transaction{}).Where("product_id=?", id)
	if err := q.Count(&result.Total).Error; err != nil {
		return result, err
	}
	err := q.Order("created_at DESC, id DESC").Limit(page.Size).Offset((page.Number - 1) * page.Size).Find(&result.Items).Error
	return result, err
}
func (s *Service) Adjust(ctx context.Context, actorID, key string, input Adjustment) (Stock, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if !users.ValidID(input.ProductID) || !validKey.MatchString(key) || input.Delta == 0 || input.Delta > MaxQuantity || input.Delta < -MaxQuantity || utf8.RuneCountInString(input.Reason) < 1 || utf8.RuneCountInString(input.Reason) > 500 {
		return Stock{}, problem.Invalid("A valid product, nonzero bounded delta, reason and Idempotency-Key are required")
	}
	data, _ := json.Marshal(input)
	hash := sha256.Sum256(data)
	fingerprint := hex.EncodeToString(hash[:])
	var result Stock
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var actor users.User
		actor, err := users.Get(tx, actorID)
		if err != nil {
			return err
		}
		if actor.Role != "Admin" || actor.Status != "Active" {
			return problem.Forbidden()
		}
		id := uuid.NewString()
		if err := tx.Exec("INSERT INTO idempotency_records (id,actor_id,operation,key,payload_hash) VALUES (?,?,'inventory.adjust',?,?) ON CONFLICT (actor_id,operation,key) DO NOTHING", id, actorID, key, fingerprint).Error; err != nil {
			return err
		}
		var entry record
		if err := tx.Table("idempotency_records").Clauses(clause.Locking{Strength: "UPDATE"}).Where("actor_id=? AND operation='inventory.adjust' AND key=?", actorID, key).Take(&entry).Error; err != nil {
			return err
		}
		if entry.PayloadHash != fingerprint {
			return problem.New(409, "IDEMPOTENCY_CONFLICT", "The key was already used with a different payload")
		}
		if len(entry.ResponseBody) > 0 {
			return json.Unmarshal(entry.ResponseBody, &result)
		}
		var count int64
		if err := tx.Table("products").Where("id=? AND deleted_at IS NULL", input.ProductID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return problem.Missing()
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id=?", input.ProductID).Take(&result).Error; err != nil {
			return err
		}
		before := result.Quantity
		if input.Delta > 0 && before > MaxQuantity-input.Delta {
			return problem.New(409, "STOCK_LIMIT", "Stock would exceed its limit")
		}
		if input.Delta < 0 && before < -input.Delta {
			return problem.New(409, "INSUFFICIENT_STOCK", "Stock cannot become negative")
		}
		result.Quantity += input.Delta
		result.Version++
		if err := tx.Model(&Stock{}).Where("product_id=?", input.ProductID).Updates(map[string]any{"quantity": result.Quantity, "version": result.Version, "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		movement := Transaction{ID: uuid.NewString(), ProductID: input.ProductID, ActorID: actorID, Delta: input.Delta, BeforeQuantity: before, AfterQuantity: result.Quantity, Reason: input.Reason, Reference: entry.ID}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}
		response, _ := json.Marshal(result)
		return tx.Exec("UPDATE idempotency_records SET response_status=200,response_body=?::jsonb WHERE id=?", string(response), entry.ID).Error
	})
	return result, err
}
