package cart

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"ecommerce.local/base/internal/database"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/idempotency"
	"ecommerce.local/base/internal/platform/money"
	"ecommerce.local/base/internal/platform/problem"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Row struct {
	ID      string
	UserID  string
	Version int64
}

func (Row) TableName() string { return "carts" }

type Item struct {
	ID             string `json:"id"`
	ProductID      string `json:"product_id"`
	Quantity       int64  `json:"quantity"`
	Name           string `json:"name"`
	SKU            string `json:"sku"`
	ImageURL       string `json:"image_url"`
	ProductVersion int64  `json:"product_version"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	LineTotalMinor int64  `json:"line_total_minor"`
	Stock          int64  `json:"stock"`
	Available      bool   `json:"available"`
}
type View struct {
	Version             int64  `json:"version"`
	QuoteHash           string `json:"quote_hash"`
	Items               []Item `json:"items"`
	SubtotalMinor       int64  `json:"subtotal_minor"`
	DiscountMinor       int64  `json:"discount_minor"`
	ShippingMinor       int64  `json:"shipping_minor"`
	TotalMinor          int64  `json:"total_minor"`
	PricingValid        bool   `json:"pricing_valid"`
	MockPaymentsEnabled bool   `json:"mock_payments_enabled"`
}
type Service struct {
	DB          *gorm.DB
	MockEnabled bool
}

func New(db *gorm.DB, mock bool) *Service { return &Service{db, mock} }
func Locked(tx *gorm.DB, userID string) (Row, error) {
	row := Row{ID: uuid.NewString(), UserID: userID, Version: 1}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoNothing: true}).Create(&row).Error; err != nil {
		return row, err
	}
	row = Row{}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id=?", userID).Take(&row).Error
	return row, err
}
func Load(tx *gorm.DB, row Row, mock bool) (View, error) {
	v := View{Version: row.Version, Items: []Item{}, PricingValid: true, MockPaymentsEnabled: mock}
	err := tx.Raw(`SELECT ci.id,ci.product_id,ci.quantity,p.name,p.sku,p.version AS product_version,p.price_minor AS unit_price_minor,i.quantity AS stock,
 (p.status='Active' AND p.deleted_at IS NULL AND i.quantity>=ci.quantity) AS available,
 COALESCE((SELECT url FROM product_images WHERE product_id=p.id ORDER BY sort_order,id LIMIT 1),'') AS image_url
 FROM cart_items ci JOIN products p ON p.id=ci.product_id JOIN inventory i ON i.product_id=p.id WHERE ci.cart_id=? ORDER BY ci.product_id`, row.ID).Scan(&v.Items).Error
	if err != nil {
		return v, err
	}
	for n := range v.Items {
		item := &v.Items[n]
		line, e := money.Line(item.UnitPriceMinor, item.Quantity)
		if e != nil {
			v.PricingValid = false
			continue
		}
		item.LineTotalMinor = line
		sum, e := money.Add(v.SubtotalMinor, line)
		if e != nil {
			v.PricingValid = false
		} else {
			v.SubtotalMinor = sum
		}
	}
	store, err := database.Store(tx.Statement.Context, tx)
	if err != nil {
		return v, err
	}
	if len(v.Items) > 0 {
		v.ShippingMinor = store.ShippingFeeMinor
	}
	v.TotalMinor, err = money.Add(v.SubtotalMinor, v.ShippingMinor)
	if err != nil {
		v.PricingValid = false
		v.TotalMinor = 0
	}
	type quoted struct {
		ProductID                string
		Quantity, Price, Version int64
	}
	items := []quoted{}
	for _, i := range v.Items {
		items = append(items, quoted{i.ProductID, i.Quantity, i.UnitPriceMinor, i.ProductVersion})
	}
	raw, _ := json.Marshal(struct {
		Cart              string
		Version, Shipping int64
		Currency          string
		Items             []quoted
	}{row.ID, row.Version, v.ShippingMinor, store.Currency, items})
	sum := sha256.Sum256(raw)
	v.QuoteHash = hex.EncodeToString(sum[:])
	return v, nil
}
func (s *Service) Get(ctx context.Context, userID string) (View, error) {
	var v View
	err := idempotency.Retry(ctx, s.DB, func(tx *gorm.DB) error {
		row, e := Locked(tx, userID)
		if e != nil {
			return e
		}
		v, e = Load(tx, row, s.MockEnabled)
		return e
	})
	return v, err
}
func (s *Service) Mutate(ctx context.Context, userID, action, id, productID string, quantity int64) (View, error) {
	var v View
	if action == "add" || action == "update" {
		if quantity < 1 || quantity > 1000 {
			return v, problem.Invalid("Quantity must be between 1 and 1000")
		}
	}
	if (action == "add" && !users.ValidID(productID)) || ((action == "update" || action == "delete") && !users.ValidID(id)) {
		return v, problem.Missing()
	}
	err := idempotency.Retry(ctx, s.DB, func(tx *gorm.DB) error {
		row, e := Locked(tx, userID)
		if e != nil {
			return e
		}
		if action == "clear" {
			if e = tx.Exec("DELETE FROM cart_items WHERE cart_id=?", row.ID).Error; e != nil {
				return e
			}
		} else if action == "delete" {
			r := tx.Exec("DELETE FROM cart_items WHERE id=? AND cart_id=?", id, row.ID)
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected != 1 {
				return problem.Missing()
			}
		} else {
			var item struct {
				ID, ProductID string
				Quantity      int64
			}
			if action == "update" {
				if e = tx.Table("cart_items").Where("id=? AND cart_id=?", id, row.ID).Take(&item).Error; e != nil {
					return e
				}
				productID = item.ProductID
			} else {
				if e = tx.Table("cart_items").Where("cart_id=? AND product_id=?", row.ID, productID).Find(&item).Error; e != nil {
					return e
				}
			}
			next := quantity
			if action == "add" {
				next += item.Quantity
			}
			if next > 1000 {
				return problem.Invalid("Quantity cannot exceed 1000")
			}
			var p struct {
				ID        string
				Status    string
				DeletedAt *string
			}
			if e = tx.Table("products").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", productID).Take(&p).Error; e != nil {
				return e
			}
			var stock struct{ Quantity int64 }
			if e = tx.Table("inventory").Where("product_id=?", productID).Take(&stock).Error; e != nil {
				return e
			}
			if p.Status != "Active" || p.DeletedAt != nil {
				return problem.New(409, "PRODUCT_UNAVAILABLE", "Product is no longer available")
			}
			if stock.Quantity < next {
				return problem.New(409, "INSUFFICIENT_STOCK", "Not enough stock")
			}
			if item.ID == "" {
				var count int64
				if e = tx.Table("cart_items").Where("cart_id=?", row.ID).Count(&count).Error; e != nil {
					return e
				}
				if count >= 100 {
					return problem.Invalid("Cart cannot contain more than 100 products")
				}
				e = tx.Exec("INSERT INTO cart_items(id,cart_id,product_id,quantity) VALUES (?,?,?,?)", uuid.NewString(), row.ID, productID, next).Error
			} else {
				e = tx.Exec("UPDATE cart_items SET quantity=? WHERE id=?", next, item.ID).Error
			}
			if e != nil {
				return e
			}
		}
		if e = tx.Model(&Row{}).Where("id=?", row.ID).Update("version", gorm.Expr("version+1")).Error; e != nil {
			return e
		}
		row.Version++
		v, e = Load(tx, row, s.MockEnabled)
		if e != nil {
			return e
		}
		if !v.PricingValid && action != "delete" && action != "clear" {
			return problem.Invalid("Cart exceeds the supported money range")
		}
		return nil
	})
	return v, err
}
