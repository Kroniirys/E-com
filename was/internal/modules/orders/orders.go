package orders

import (
	"context"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"ecommerce.local/base/internal/modules/cart"
	"ecommerce.local/base/internal/modules/inventory"
	"ecommerce.local/base/internal/modules/payments"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/idempotency"
	"ecommerce.local/base/internal/platform/outbox"
	"ecommerce.local/base/internal/platform/problem"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Address struct {
	Recipient string `json:"recipient"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
}
type Order struct {
	ID                  string           `json:"id"`
	UserID              string           `json:"user_id"`
	Status              string           `json:"status"`
	Currency            string           `json:"currency"`
	SubtotalMinor       int64            `json:"subtotal_minor"`
	DiscountMinor       int64            `json:"discount_minor"`
	ShippingMinor       int64            `json:"shipping_minor"`
	TotalMinor          int64            `json:"total_minor"`
	ShippingAddress     Address          `json:"shipping_address" gorm:"serializer:json;type:jsonb"`
	Version             int64            `json:"version"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"-"`
	Items               []Item           `json:"items" gorm:"-"`
	Payment             payments.Payment `json:"payment" gorm:"-"`
	History             []History        `json:"history" gorm:"-"`
	MockPaymentsEnabled bool             `json:"mock_payments_enabled" gorm:"-"`
}
type Item struct {
	ID             string `json:"-"`
	OrderID        string `json:"-"`
	ProductID      string `json:"product_id"`
	SKU            string `json:"sku"`
	Name           string `json:"name"`
	ImageURL       string `json:"image_url"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	LineTotalMinor int64  `json:"line_total_minor"`
}

func (Item) TableName() string { return "order_items" }

type History struct {
	ID         string    `json:"id"`
	OrderID    string    `json:"-"`
	ActorID    string    `json:"actor_id"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

func (History) TableName() string { return "order_status_history" }

type CheckoutInput struct {
	AddressID     string `json:"address_id"`
	CartVersion   int64  `json:"cart_version"`
	QuoteHash     string `json:"quote_hash"`
	PaymentMethod string `json:"payment_method"`
}
type Service struct {
	DB          *gorm.DB
	MockEnabled bool
}

func New(db *gorm.DB, mock bool) *Service { return &Service{db, mock} }
func conflict(code, message string) error { return problem.New(409, code, message) }
func guard(tx *gorm.DB, actorID string, admin bool) error {
	u, e := users.Get(tx, actorID)
	if e != nil {
		return e
	}
	if u.Status != "Active" || (admin && u.Role != "Admin") {
		return problem.Forbidden()
	}
	return nil
}
func (s *Service) load(tx *gorm.DB, id string) (Order, error) {
	var o Order
	if err := tx.Where("id=?", id).Take(&o).Error; err != nil {
		return o, err
	}
	o.Items = []Item{}
	o.History = []History{}
	o.MockPaymentsEnabled = s.MockEnabled
	if err := tx.Where("order_id=?", id).Order("product_id").Find(&o.Items).Error; err != nil {
		return o, err
	}
	if err := tx.Where("order_id=?", id).Take(&o.Payment).Error; err != nil {
		return o, err
	}
	o.Payment.Attempts = []payments.Attempt{}
	if err := tx.Where("payment_id=?", o.Payment.ID).Order("created_at,id").Find(&o.Payment.Attempts).Error; err != nil {
		return o, err
	}
	err := tx.Where("order_id=?", id).Order("created_at,id").Find(&o.History).Error
	return o, err
}
func locked(tx *gorm.DB, id, actorID string, admin bool) (Order, error) {
	var o Order
	if !users.ValidID(id) {
		return o, problem.Missing()
	}
	q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", id)
	if !admin {
		q = q.Where("user_id=?", actorID)
	}
	err := q.Take(&o).Error
	return o, err
}
func (s *Service) Get(ctx context.Context, actorID, id string, admin bool) (Order, error) {
	var o Order
	if !users.ValidID(id) {
		return o, problem.Missing()
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := guard(tx, actorID, admin); e != nil {
			return e
		}
		var owner struct{ UserID string }
		q := tx.Table("orders").Where("id=?", id)
		if !admin {
			q = q.Where("user_id=?", actorID)
		}
		if e := q.Take(&owner).Error; e != nil {
			return e
		}
		var e error
		o, e = s.load(tx, id)
		return e
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return o, err
}
func (s *Service) List(ctx context.Context, actorID string, admin bool, page users.Page) (users.List[Order], error) {
	list := users.List[Order]{Items: []Order{}, Page: page.Number, PageSize: page.Size}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := guard(tx, actorID, admin); e != nil {
			return e
		}
		q := tx.Model(&Order{})
		if !admin {
			q = q.Where("user_id=?", actorID)
		}
		if page.Query != "" {
			q = q.Where("id::text ILIKE ?", users.Like(page.Query))
		}
		if page.Sort != "" && page.Sort != "created_at" {
			return problem.Invalid("Unsupported order sort")
		}
		if e := q.Count(&list.Total).Error; e != nil {
			return e
		}
		var rows []Order
		if e := q.Order("created_at DESC,id DESC").Limit(page.Size).Offset((page.Number - 1) * page.Size).Find(&rows).Error; e != nil {
			return e
		}
		for _, row := range rows {
			o, e := s.load(tx, row.ID)
			if e != nil {
				return e
			}
			list.Items = append(list.Items, o)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return list, err
}
func history(tx *gorm.DB, o Order, actor, from, reason string) error {
	return tx.Create(&History{ID: uuid.NewString(), OrderID: o.ID, ActorID: actor, FromStatus: from, ToStatus: o.Status, Reason: reason}).Error
}
func event(tx *gorm.DB, o Order, kind, correlation, from string) error {
	payload := map[string]any{"order_id": o.ID, "user_id": o.UserID, "status": o.Status}
	if kind == "order.created" {
		payload["currency"] = o.Currency
		payload["total_minor"] = o.TotalMinor
	}
	if kind == "order.status_changed" {
		payload["from_status"] = from
		payload["to_status"] = o.Status
	}
	if kind == "order.cancelled" {
		payload["reason_code"] = "cancelled"
	}
	return outbox.Write(tx, "order", o.ID, o.Version, kind, correlation, payload)
}
func paidEvent(tx *gorm.DB, o Order, p payments.Payment, correlation string) error {
	return outbox.Write(tx, "payment", p.ID, 2, "payment.completed", correlation, map[string]any{"payment_id": p.ID, "order_id": o.ID, "user_id": o.UserID, "method": p.Method, "amount_minor": p.AmountMinor})
}
func (s *Service) Checkout(ctx context.Context, actor, key string, in CheckoutInput, correlation string) (Order, error) {
	_, quoteError := hex.DecodeString(in.QuoteHash)
	if !users.ValidID(in.AddressID) || in.CartVersion < 1 || len(in.QuoteHash) != 64 || quoteError != nil || in.QuoteHash != strings.ToLower(in.QuoteHash) || (in.PaymentMethod != "COD" && in.PaymentMethod != "Mock") {
		return Order{}, problem.Invalid("A valid address, cart version, quote and payment method are required")
	}
	if in.PaymentMethod == "Mock" && !s.MockEnabled {
		return Order{}, problem.Invalid("Mock payments are disabled")
	}
	return idempotency.Run(ctx, s.DB, actor, "orders.checkout", key, in, false, func(tx *gorm.DB) (Order, error) {
		var empty Order
		var user users.User
		if e := tx.Table("users").Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Where("id=? AND status='Active' AND deleted_at IS NULL", actor).Take(&user).Error; e != nil {
			return empty, e
		}
		row, e := cart.Locked(tx, actor)
		if e != nil {
			return empty, e
		}
		if row.Version != in.CartVersion {
			return empty, conflict("CART_CHANGED", "Cart changed; review it before checkout")
		}
		var ids []string
		if e = tx.Table("cart_items").Where("cart_id=?", row.ID).Order("product_id").Pluck("product_id", &ids).Error; e != nil {
			return empty, e
		}
		if len(ids) == 0 {
			return empty, conflict("CART_EMPTY", "Cart is empty")
		}
		var products []struct{ ID string }
		if e = tx.Table("products").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id").Find(&products).Error; e != nil {
			return empty, e
		}
		view, e := cart.Load(tx, row, s.MockEnabled)
		if e != nil {
			return empty, e
		}
		if view.QuoteHash != in.QuoteHash {
			return empty, conflict("QUOTE_CHANGED", "Prices changed; review and confirm the current quote")
		}
		if !view.PricingValid {
			return empty, problem.Invalid("Cart exceeds the money range")
		}
		for _, item := range view.Items {
			if !item.Available {
				return empty, conflict("PRODUCT_UNAVAILABLE", "An item is unavailable or does not have enough stock")
			}
		}
		var address users.Address
		if e = tx.Where("id=? AND user_id=?", in.AddressID, actor).Take(&address).Error; e != nil {
			return empty, e
		}
		o := Order{ID: uuid.NewString(), UserID: actor, Status: "Pending", Currency: "VND", SubtotalMinor: view.SubtotalMinor, ShippingMinor: view.ShippingMinor, TotalMinor: view.TotalMinor, Version: 1, ShippingAddress: Address{address.Recipient, address.Phone, address.Address}}
		if e = tx.Create(&o).Error; e != nil {
			return empty, e
		}
		for _, item := range view.Items {
			if e = inventory.Change(tx, item.ProductID, actor, -item.Quantity, "order.checkout", o.ID, correlation); e != nil {
				return empty, e
			}
			snapshot := Item{ID: uuid.NewString(), OrderID: o.ID, ProductID: item.ProductID, SKU: item.SKU, Name: item.Name, ImageURL: item.ImageURL, Quantity: item.Quantity, UnitPriceMinor: item.UnitPriceMinor, LineTotalMinor: item.LineTotalMinor}
			if e = tx.Create(&snapshot).Error; e != nil {
				return empty, e
			}
		}
		if _, e = payments.Create(tx, o.ID, in.PaymentMethod, o.TotalMinor); e != nil {
			return empty, e
		}
		if e = history(tx, o, actor, "", "Checkout"); e != nil {
			return empty, e
		}
		if e = event(tx, o, "order.created", correlation, ""); e != nil {
			return empty, e
		}
		if e = tx.Exec("DELETE FROM cart_items WHERE cart_id=?", row.ID).Error; e != nil {
			return empty, e
		}
		if e = tx.Model(&cart.Row{}).Where("id=?", row.ID).Update("version", gorm.Expr("version+1")).Error; e != nil {
			return empty, e
		}
		return s.load(tx, o.ID)
	})
}
func (s *Service) Transition(ctx context.Context, actor, id, key, to, reason, correlation string, admin bool) (Order, error) {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > 500 {
		return Order{}, problem.Invalid("Reason exceeds 500 characters")
	}
	if !admin && to != "Cancelled" {
		return Order{}, problem.Forbidden()
	}
	input := struct{ ID, To, Reason string }{id, to, reason}
	operation := "orders.transition"
	if admin {
		operation = "orders.admin-transition"
	}
	return idempotency.Run(ctx, s.DB, actor, operation, key, input, admin, func(tx *gorm.DB) (Order, error) {
		o, e := locked(tx, id, actor, admin)
		if e != nil {
			return o, e
		}
		p, e := payments.Locked(tx, id)
		if e != nil {
			return o, e
		}
		if !CanTransition(o.Status, to, p.Method, p.Status) {
			return o, conflict("INVALID_TRANSITION", "Order cannot move to the requested state")
		}
		from := o.Status
		o.Status = to
		o.Version++
		if to == "Cancelled" || to == "Failed" {
			var items []Item
			if e = tx.Where("order_id=?", id).Order("product_id").Find(&items).Error; e != nil {
				return o, e
			}
			for _, item := range items {
				if e = inventory.Change(tx, item.ProductID, actor, item.Quantity, "order.release", id, correlation); e != nil {
					return o, e
				}
			}
			if e = payments.Cancel(tx, p); e != nil {
				return o, e
			}
		}
		if to == "Delivered" && p.Method == "COD" {
			if e = payments.MarkPaid(tx, p.ID); e != nil {
				return o, e
			}
			if e = paidEvent(tx, o, p, correlation); e != nil {
				return o, e
			}
		}
		if e = tx.Model(&Order{}).Where("id=?", id).Updates(map[string]any{"status": to, "version": o.Version, "updated_at": time.Now().UTC()}).Error; e != nil {
			return o, e
		}
		if e = history(tx, o, actor, from, reason); e != nil {
			return o, e
		}
		kind := "order.status_changed"
		if to == "Confirmed" {
			kind = "order.confirmed"
		}
		if to == "Cancelled" {
			kind = "order.cancelled"
		}
		if e = event(tx, o, kind, correlation, from); e != nil {
			return o, e
		}
		return s.load(tx, id)
	})
}
func (s *Service) BeginAttempt(ctx context.Context, actor, id, key string) (payments.Attempt, error) {
	if !s.MockEnabled {
		return payments.Attempt{}, problem.Missing()
	}
	return idempotency.Run(ctx, s.DB, actor, "payments.begin", key, struct{ OrderID string }{id}, false, func(tx *gorm.DB) (payments.Attempt, error) {
		o, e := locked(tx, id, actor, false)
		if e != nil {
			return payments.Attempt{}, e
		}
		if o.Status != "Pending" {
			return payments.Attempt{}, conflict("INVALID_TRANSITION", "Only pending orders accept attempts")
		}
		p, e := payments.Locked(tx, id)
		if e != nil {
			return payments.Attempt{}, e
		}
		var count int64
		if e = tx.Model(&payments.Attempt{}).Where("payment_id=?", p.ID).Count(&count).Error; e != nil {
			return payments.Attempt{}, e
		}
		if count >= 5 {
			return payments.Attempt{}, conflict("PAYMENT_RETRY_LIMIT", "At most five mock attempts are allowed")
		}
		return payments.Begin(tx, p)
	})
}
func (s *Service) Simulate(ctx context.Context, actor, attemptID, key, result, code, correlation string) (Order, error) {
	if !s.MockEnabled {
		return Order{}, problem.Missing()
	}
	if !users.ValidID(attemptID) || (result != "Success" && result != "Failure") || utf8.RuneCountInString(code) > 100 {
		return Order{}, problem.Invalid("A valid attempt and simulation result are required")
	}
	if result == "Success" {
		code = ""
	} else if code == "" {
		code = "MOCK_DECLINED"
	}
	input := struct{ AttemptID, Result, Code string }{attemptID, result, code}
	return idempotency.Run(ctx, s.DB, actor, "payments.simulate", key, input, false, func(tx *gorm.DB) (Order, error) {
		var target struct{ OrderID string }
		if e := tx.Table("payment_attempts a").Select("p.order_id").Joins("JOIN payments p ON p.id=a.payment_id").Where("a.id=?", attemptID).Take(&target).Error; e != nil {
			return Order{}, e
		}
		o, e := locked(tx, target.OrderID, actor, false)
		if e != nil {
			return o, e
		}
		p, e := payments.Locked(tx, o.ID)
		if e != nil {
			return o, e
		}
		if o.Status != "Pending" || p.Method != "Mock" {
			return o, conflict("INVALID_TRANSITION", "Order does not accept payment results")
		}
		if _, e = payments.Resolve(tx, p, attemptID, result, code); e != nil {
			return o, e
		}
		if result == "Success" {
			o.Status = "Confirmed"
			o.Version++
			if e = tx.Model(&Order{}).Where("id=?", o.ID).Updates(map[string]any{"status": o.Status, "version": o.Version, "updated_at": time.Now().UTC()}).Error; e != nil {
				return o, e
			}
			if e = history(tx, o, actor, "Pending", "Mock payment completed"); e != nil {
				return o, e
			}
			if e = event(tx, o, "order.confirmed", correlation, "Pending"); e != nil {
				return o, e
			}
			if e = paidEvent(tx, o, p, correlation); e != nil {
				return o, e
			}
		}
		return s.load(tx, o.ID)
	})
}
