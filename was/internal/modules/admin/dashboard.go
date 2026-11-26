package admin

import (
	"context"
	"database/sql"
	"ecommerce.local/base/internal/platform/problem"
	"gorm.io/gorm"
	"time"
	_ "time/tzdata"
)

type Range struct {
	From     *time.Time `json:"from"`
	To       *time.Time `json:"to"`
	Timezone string     `json:"timezone"`
}
type Dashboard struct {
	UserCount               int64  `json:"user_count"`
	ProductCount            int64  `json:"product_count"`
	OrderCount              int64  `json:"order_count"`
	DeliveredPaidTotalMinor string `json:"delivered_paid_total_minor"`
	Range                   Range  `json:"range"`
}
type Service struct{ DB *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db} }
func ParseRange(from, to, zone string) (Range, error) {
	r := Range{Timezone: zone}
	if zone == "" {
		r.Timezone = "UTC"
	}
	if r.Timezone == "Local" {
		return r, problem.Invalid("An explicit IANA timezone is required")
	}
	location, err := time.LoadLocation(r.Timezone)
	if err != nil {
		return r, problem.Invalid("Invalid IANA timezone")
	}
	for _, entry := range []struct {
		Raw    string
		Target **time.Time
	}{{from, &r.From}, {to, &r.To}} {
		if entry.Raw != "" {
			value, e := time.ParseInLocation("2006-01-02", entry.Raw, location)
			if e != nil {
				return r, problem.Invalid("Dates must use YYYY-MM-DD")
			}
			value = value.UTC()
			*entry.Target = &value
		}
	}
	if r.From != nil && r.To != nil && !r.From.Before(*r.To) {
		return r, problem.Invalid("to must be later than from; to is exclusive")
	}
	return r, nil
}
func filtered(q *gorm.DB, prefix string, r Range) *gorm.DB {
	if r.From != nil {
		q = q.Where(prefix+"created_at >= ?", *r.From)
	}
	if r.To != nil {
		q = q.Where(prefix+"created_at < ?", *r.To)
	}
	return q
}
func (s *Service) Get(ctx context.Context, r Range) (Dashboard, error) {
	d := Dashboard{Range: r, DeliveredPaidTotalMinor: "0"}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := filtered(tx.Table("users").Where("deleted_at IS NULL"), "", r).Count(&d.UserCount).Error; e != nil {
			return e
		}
		if e := filtered(tx.Table("products").Where("deleted_at IS NULL"), "", r).Count(&d.ProductCount).Error; e != nil {
			return e
		}
		if e := filtered(tx.Table("orders"), "", r).Count(&d.OrderCount).Error; e != nil {
			return e
		}
		return filtered(tx.Table("orders o").Joins("JOIN payments p ON p.order_id=o.id").Where("o.status='Delivered' AND p.status='Paid'"), "o.", r).Select("COALESCE(SUM(o.total_minor),0)::text").Scan(&d.DeliveredPaidTotalMinor).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return d, err
}
