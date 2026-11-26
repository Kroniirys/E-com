package reviews

import (
	"context"
	"database/sql"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/problem"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
	"unicode/utf8"
)

type Review struct {
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	ProductID        string     `json:"product_id"`
	OrderID          string     `json:"-"`
	Rating           int        `json:"rating"`
	Content          string     `json:"content"`
	Visibility       string     `json:"visibility"`
	ModerationReason string     `json:"-"`
	ModeratedBy      *string    `json:"-"`
	ModeratedAt      *time.Time `json:"-"`
	DeletedAt        *time.Time `json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"-"`
	Deleted          bool       `json:"deleted" gorm:"-"`
}
type Input struct {
	Rating  int    `json:"rating"`
	Content string `json:"content"`
}
type Summary struct {
	Count         int64   `json:"count"`
	AverageRating float64 `json:"average_rating"`
}
type PublicList struct {
	users.List[Review]
	Summary Summary `json:"summary"`
}
type Eligibility struct {
	CanReview  bool    `json:"can_review"`
	ReasonCode string  `json:"reason_code"`
	Review     *Review `json:"review"`
}
type AdminReview struct {
	Review
	ModerationReason string     `json:"moderation_reason"`
	ModeratedBy      *string    `json:"moderated_by"`
	ModeratedAt      *time.Time `json:"moderated_at"`
}
type Service struct{ DB *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{db} }
func validate(in *Input) error {
	in.Content = strings.TrimSpace(in.Content)
	n := utf8.RuneCountInString(in.Content)
	if in.Rating < 1 || in.Rating > 5 || n < 1 || n > 2000 {
		return problem.Invalid("Rating must be 1 to 5 and content 1 to 2000 characters")
	}
	return nil
}
func activeProduct(tx *gorm.DB, id string, lock bool) error {
	if !users.ValidID(id) {
		return problem.Missing()
	}
	q := tx.Table("products").Where("id=? AND status='Active' AND deleted_at IS NULL", id)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "SHARE"})
	}
	var product struct{ ID string }
	return q.Take(&product).Error
}
func delivered(tx *gorm.DB, user, product string) (string, error) {
	var row struct{ ID string }
	err := tx.Table("orders o").Select("o.id").Joins("JOIN order_items i ON i.order_id=o.id").Where("o.user_id=? AND o.status='Delivered' AND i.product_id=?", user, product).Order("o.created_at,o.id").Take(&row).Error
	return row.ID, err
}
func (s *Service) Create(ctx context.Context, user, product string, in Input) (Review, error) {
	var r Review
	if e := validate(&in); e != nil {
		return r, e
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		u, e := users.Locked(tx, user)
		if e != nil {
			return e
		}
		if u.Status != "Active" {
			return problem.Forbidden()
		}
		if e = activeProduct(tx, product, true); e != nil {
			return e
		}
		order, e := delivered(tx, user, product)
		if e == gorm.ErrRecordNotFound {
			return problem.New(403, "REVIEW_INELIGIBLE", "A delivered purchase of this product is required")
		}
		if e != nil {
			return e
		}
		r = Review{ID: uuid.NewString(), UserID: user, ProductID: product, OrderID: order, Rating: in.Rating, Content: in.Content, Visibility: "Visible"}
		return tx.Create(&r).Error
	})
	return r, err
}
func (s *Service) Eligibility(ctx context.Context, user, product string) (Eligibility, error) {
	result := Eligibility{ReasonCode: "PURCHASE_REQUIRED"}
	if !users.ValidID(product) {
		return result, problem.Missing()
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var r Review
		e := tx.Where("user_id=? AND product_id=?", user, product).Take(&r).Error
		if e == nil {
			r.Deleted = r.DeletedAt != nil
			result.Review = &r
			result.ReasonCode = "ALREADY_REVIEWED"
			return nil
		}
		if e != gorm.ErrRecordNotFound {
			return e
		}
		if e = activeProduct(tx, product, false); e != nil {
			if e == gorm.ErrRecordNotFound {
				result.ReasonCode = "PRODUCT_UNAVAILABLE"
				return nil
			}
			return e
		}
		_, e = delivered(tx, user, product)
		if e == nil {
			result.CanReview = true
			result.ReasonCode = "ELIGIBLE"
			return nil
		}
		if e == gorm.ErrRecordNotFound {
			return nil
		}
		return e
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}
func (s *Service) Public(ctx context.Context, product string, p users.Page) (PublicList, error) {
	v := PublicList{List: users.List[Review]{Items: []Review{}, Page: p.Number, PageSize: p.Size}}
	if p.Sort != "" && p.Sort != "created_at" {
		return v, problem.Invalid("Unsupported review sort")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := activeProduct(tx, product, false); e != nil {
			return e
		}
		q := tx.Model(&Review{}).Where("product_id=? AND visibility='Visible' AND deleted_at IS NULL", product)
		if e := q.Select("COUNT(*) AS count, COALESCE(ROUND(AVG(rating),2),0) AS average_rating").Scan(&v.Summary).Error; e != nil {
			return e
		}
		v.Total = v.Summary.Count
		return tx.Where("product_id=? AND visibility='Visible' AND deleted_at IS NULL", product).Order("created_at DESC,id DESC").Limit(p.Size).Offset((p.Number - 1) * p.Size).Find(&v.Items).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return v, err
}
func (s *Service) Mine(ctx context.Context, user string, p users.Page) (users.List[Review], error) {
	v := users.List[Review]{Items: []Review{}, Page: p.Number, PageSize: p.Size}
	q := s.DB.WithContext(ctx).Model(&Review{}).Where("user_id=? AND deleted_at IS NULL", user)
	if e := q.Count(&v.Total).Error; e != nil {
		return v, e
	}
	e := q.Order("created_at DESC,id DESC").Limit(p.Size).Offset((p.Number - 1) * p.Size).Find(&v.Items).Error
	return v, e
}
func (s *Service) Delete(ctx context.Context, user, id string) error {
	if !users.ValidID(id) {
		return problem.Missing()
	}
	r := s.DB.WithContext(ctx).Model(&Review{}).Where("id=? AND user_id=? AND deleted_at IS NULL", id, user).Updates(map[string]any{"deleted_at": time.Now().UTC(), "updated_at": time.Now().UTC()})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return problem.Missing()
	}
	return nil
}
func (s *Service) AdminList(ctx context.Context, p users.Page, visibility string) (users.List[AdminReview], error) {
	v := users.List[AdminReview]{Items: []AdminReview{}, Page: p.Number, PageSize: p.Size}
	if visibility != "" && visibility != "Visible" && visibility != "Hidden" {
		return v, problem.Invalid("Unsupported review visibility")
	}
	if p.Sort != "" && p.Sort != "created_at" {
		return v, problem.Invalid("Unsupported review sort")
	}
	q := s.DB.WithContext(ctx).Model(&Review{}).Where("deleted_at IS NULL")
	if visibility != "" {
		q = q.Where("visibility=?", visibility)
	}
	if p.Query != "" {
		q = q.Where("content ILIKE ? OR product_id::text ILIKE ?", users.Like(p.Query), users.Like(p.Query))
	}
	if e := q.Count(&v.Total).Error; e != nil {
		return v, e
	}
	var rows []Review
	if e := q.Order("created_at DESC,id DESC").Limit(p.Size).Offset((p.Number - 1) * p.Size).Find(&rows).Error; e != nil {
		return v, e
	}
	for _, r := range rows {
		v.Items = append(v.Items, AdminReview{r, r.ModerationReason, r.ModeratedBy, r.ModeratedAt})
	}
	return v, nil
}
func (s *Service) Moderate(ctx context.Context, actor, id, visibility, reason string) (AdminReview, error) {
	reason = strings.TrimSpace(reason)
	if !users.ValidID(id) {
		return AdminReview{}, problem.Missing()
	}
	if (visibility != "Visible" && visibility != "Hidden") || utf8.RuneCountInString(reason) < 1 || utf8.RuneCountInString(reason) > 500 {
		return AdminReview{}, problem.Invalid("Visibility and a reason of 1 to 500 characters are required")
	}
	var r Review
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := users.AdminGuard(tx, actor); e != nil {
			return e
		}
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).Take(&r).Error; e != nil {
			return e
		}
		now := time.Now().UTC()
		if e := tx.Model(&Review{}).Where("id=?", id).Updates(map[string]any{"visibility": visibility, "moderation_reason": reason, "moderated_by": actor, "moderated_at": now, "updated_at": now}).Error; e != nil {
			return e
		}
		r.Visibility = visibility
		r.ModerationReason = reason
		r.ModeratedBy = &actor
		r.ModeratedAt = &now
		return nil
	})
	return AdminReview{r, r.ModerationReason, r.ModeratedBy, r.ModeratedAt}, err
}
