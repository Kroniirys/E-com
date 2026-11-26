package notifications

import (
	"context"
	"database/sql"
	"time"

	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/problem"

	"gorm.io/gorm"
)

type Notification struct {
	ID               string     `json:"id"`
	UserID           string     `json:"-"`
	EventID          string     `json:"event_id"`
	Kind             string     `json:"kind"`
	Title            string     `json:"title"`
	Body             string     `json:"body"`
	OrderID          string     `json:"order_id"`
	OccurredAt       time.Time  `json:"occurred_at"`
	AggregateVersion int64      `json:"aggregate_version"`
	ReadAt           *time.Time `json:"read_at"`
	CreatedAt        time.Time  `json:"created_at"`
}
type List struct {
	users.List[Notification]
	UnreadCount int64 `json:"unread_count"`
}
type Service struct{ DB *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db} }
func (s *Service) List(ctx context.Context, user string, p users.Page) (List, error) {
	v := List{List: users.List[Notification]{Items: []Notification{}, Page: p.Number, PageSize: p.Size}}
	if p.Sort != "" && p.Sort != "created_at" {
		return v, problem.Invalid("Unsupported notification sort")
	}
	e := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&Notification{}).Where("user_id=?", user)
		if e := q.Count(&v.Total).Error; e != nil {
			return e
		}
		if e := tx.Model(&Notification{}).Where("user_id=? AND read_at IS NULL", user).Count(&v.UnreadCount).Error; e != nil {
			return e
		}
		return tx.Where("user_id=?", user).Order("occurred_at DESC,aggregate_version DESC,id DESC").Limit(p.Size).Offset((p.Number - 1) * p.Size).Find(&v.Items).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return v, e
}
func (s *Service) Read(ctx context.Context, user, id string) error {
	if !users.ValidID(id) {
		return problem.Missing()
	}
	r := s.DB.WithContext(ctx).Model(&Notification{}).Where("id=? AND user_id=?", id, user).Update("read_at", gorm.Expr("COALESCE(read_at,now())"))
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return problem.Missing()
	}
	return nil
}
