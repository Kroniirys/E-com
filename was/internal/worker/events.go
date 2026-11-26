package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"ecommerce.local/base/internal/platform/cache"
	"ecommerce.local/base/internal/platform/metrics"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Event struct {
	ID               string          `json:"event_id"`
	Type             string          `json:"type"`
	Version          int             `json:"version"`
	AggregateID      string          `json:"aggregate_id"`
	AggregateType    string          `json:"aggregate_type"`
	AggregateVersion int64           `json:"aggregate_version"`
	OccurredAt       time.Time       `json:"occurred_at"`
	CorrelationID    string          `json:"correlation_id"`
	Payload          json.RawMessage `json:"payload"`
}

var kinds = map[string]string{"order.created": "order", "order.confirmed": "order", "order.cancelled": "order", "order.status_changed": "order", "payment.completed": "payment", "inventory.updated": "inventory", "product.changed": "product", "category.changed": "category"}

type Permanent struct{ Reason string }

func (e *Permanent) Error() string { return e.Reason }
func IsPermanent(e error) bool     { var p *Permanent; return errors.As(e, &p) }
func Validate(e Event) error {
	_, a := uuid.Parse(e.ID)
	_, b := uuid.Parse(e.AggregateID)
	if a != nil || b != nil || len(e.ID) != 36 || len(e.AggregateID) != 36 || e.Version != 1 || e.AggregateVersion < 1 || e.OccurredAt.IsZero() || kinds[e.Type] == "" || kinds[e.Type] != e.AggregateType {
		return &Permanent{"invalid event envelope"}
	}
	return nil
}

type Processor struct {
	DB    *gorm.DB
	Cache *cache.Cache
}

func (p *Processor) Process(ctx context.Context, purpose string, event Event) error {
	if e := Validate(event); e != nil {
		return e
	}
	if purpose == "catalog" {
		if event.Type != "product.changed" && event.Type != "category.changed" && event.Type != "inventory.updated" {
			return &Permanent{"invalid catalog event"}
		}
		var payload map[string]json.RawMessage
		if json.Unmarshal(event.Payload, &payload) != nil {
			return &Permanent{"invalid catalog payload"}
		}
		idKey, versionKey, table := "product_id", "product_version", "products"
		if event.Type == "category.changed" {
			idKey, versionKey, table = "category_id", "category_version", "categories"
		}
		if event.Type == "inventory.updated" {
			versionKey = "inventory_version"
		}
		var id string
		var eventVersion int64
		if json.Unmarshal(payload[idKey], &id) != nil || json.Unmarshal(payload[versionKey], &eventVersion) != nil || id != event.AggregateID || eventVersion != event.AggregateVersion {
			return &Permanent{"catalog payload mismatch"}
		}
		var existing int64
		if e := p.DB.WithContext(ctx).Table(table).Where("id=?", id).Count(&existing).Error; e != nil {
			return e
		}
		if existing != 1 {
			return &Permanent{"unknown catalog aggregate"}
		}
		var version int64
		if e := p.DB.WithContext(ctx).Table("catalog_generation").Where("id=true").Pluck("version", &version).Error; e != nil {
			return e
		}
		if p.Cache == nil {
			return errors.New("cache unavailable")
		}
		if e := p.Cache.Observe(ctx, version); e != nil {
			return e
		}
	}
	return p.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if purpose != "catalog" && purpose != "notifications" {
			return &Permanent{"unknown consumer"}
		}
		r := tx.Exec("INSERT INTO processed_events(consumer,event_id) VALUES (?,?) ON CONFLICT DO NOTHING", purpose, event.ID)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			metrics.Add("ecom_consumer_duplicate_total", 1)
			return nil
		}
		if purpose == "catalog" {
			metrics.Add("ecom_cache_invalidation_total", 1)
			return nil
		}
		var payload struct {
			OrderID   string `json:"order_id"`
			UserID    string `json:"user_id"`
			Status    string `json:"status"`
			From      string `json:"from_status"`
			To        string `json:"to_status"`
			PaymentID string `json:"payment_id"`
			Method    string `json:"method"`
			Amount    int64  `json:"amount_minor"`
		}
		if e := json.Unmarshal(event.Payload, &payload); e != nil {
			return &Permanent{"invalid notification payload"}
		}
		var order struct {
			ID, UserID string
			Version    int64
		}
		orderID := event.AggregateID
		if event.Type == "payment.completed" {
			var payment struct {
				OrderID, Method, Status string
				AmountMinor             int64
			}
			r := tx.Table("payments").Where("id=?", event.AggregateID).Take(&payment)
			if r.Error != nil {
				if errors.Is(r.Error, gorm.ErrRecordNotFound) {
					return &Permanent{"unknown payment"}
				}
				return r.Error
			}
			if payload.PaymentID != event.AggregateID || payload.OrderID != payment.OrderID || payload.Method != payment.Method || payload.Amount != payment.AmountMinor || payment.Status != "Paid" {
				return &Permanent{"payment payload mismatch"}
			}
			orderID = payment.OrderID
		} else if kinds[event.Type] != "order" {
			return &Permanent{"invalid notification type"}
		}
		if e := tx.Table("orders").Where("id=?", orderID).Take(&order).Error; e != nil {
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return &Permanent{"unknown order"}
			}
			return e
		}
		if event.AggregateType == "order" && event.AggregateVersion > order.Version {
			return &Permanent{"future order version"}
		}
		if payload.OrderID != order.ID || payload.UserID != order.UserID {
			return &Permanent{"notification ownership mismatch"}
		}
		title := map[string]string{"order.created": "Đặt hàng thành công", "order.confirmed": "Đơn hàng đã xác nhận", "order.cancelled": "Đơn hàng đã hủy", "payment.completed": "Thanh toán thành công", "order.status_changed": "Trạng thái đơn hàng thay đổi"}[event.Type]
		state := payload.Status
		if event.Type == "payment.completed" {
			state = "Paid"
		}
		if event.Type == "order.status_changed" {
			state = payload.To
			if state != "Processing" && state != "Shipped" && state != "Delivered" && state != "Failed" {
				return &Permanent{"invalid order status event"}
			}
		}
		if event.Type == "order.created" && state != "Pending" || event.Type == "order.confirmed" && state != "Confirmed" || event.Type == "order.cancelled" && state != "Cancelled" {
			return &Permanent{"invalid lifecycle payload"}
		}
		if e := tx.Exec("INSERT INTO notifications(id,user_id,event_id,kind,title,body,order_id,occurred_at,aggregate_version) VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT DO NOTHING", uuid.NewString(), order.UserID, event.ID, event.Type, title, fmt.Sprintf("Đơn %s · %s", order.ID, map[string]string{"Pending": "Chờ xác nhận", "Confirmed": "Đã xác nhận", "Cancelled": "Đã hủy", "Processing": "Đang xử lý", "Shipped": "Đang giao", "Delivered": "Đã giao", "Failed": "Không thể hoàn tất", "Paid": "Đã thanh toán"}[state]), order.ID, event.OccurredAt, event.AggregateVersion).Error; e != nil {
			return e
		}
		return nil
	})
}
func MockEmail(event Event) {
	slog.Info("mock email only", "event_id", event.ID, "event_type", event.Type, "aggregate_id", event.AggregateID)
}
