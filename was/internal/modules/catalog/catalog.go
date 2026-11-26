package catalog

import (
	"context"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"ecommerce.local/base/internal/modules/inventory"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/cache"
	"ecommerce.local/base/internal/platform/outbox"
	"ecommerce.local/base/internal/platform/problem"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const MaxMoney int64 = 9000000000000

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Category struct {
	Version   int64      `json:"-"`
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	DeletedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"-"`
	UpdatedAt time.Time  `json:"-"`
}

func (Category) TableName() string { return "categories" }

type CategoryInput struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
type Image struct {
	ID        string `json:"-"`
	ProductID string `json:"-"`
	URL       string `json:"url"`
	Alt       string `json:"alt"`
	SortOrder int    `json:"sort_order"`
}

func (Image) TableName() string { return "product_images" }

type Product struct {
	ID                 string     `json:"id"`
	SKU                string     `json:"sku"`
	Name               string     `json:"name"`
	Description        string     `json:"description"`
	PriceMinor         int64      `json:"price_minor"`
	OriginalPriceMinor int64      `json:"original_price_minor"`
	Status             string     `json:"status"`
	Version            int64      `json:"version"`
	Stock              int64      `json:"stock" gorm:"->"`
	OutOfStock         bool       `json:"out_of_stock" gorm:"-"`
	Images             []Image    `json:"images" gorm:"-"`
	CategoryIDs        []string   `json:"category_ids" gorm:"-"`
	DeletedAt          *time.Time `json:"-"`
	CreatedAt          time.Time  `json:"-"`
	UpdatedAt          time.Time  `json:"-"`
}

func (Product) TableName() string { return "products" }

type ProductInput struct {
	SKU                string       `json:"sku"`
	Name               string       `json:"name"`
	Description        string       `json:"description"`
	PriceMinor         int64        `json:"price_minor"`
	OriginalPriceMinor int64        `json:"original_price_minor"`
	Status             string       `json:"status"`
	Images             []ImageInput `json:"images"`
	CategoryIDs        []string     `json:"category_ids"`
}
type ImageInput struct {
	URL       string `json:"url"`
	Alt       string `json:"alt"`
	SortOrder int    `json:"sort_order"`
}
type Service struct {
	DB    *gorm.DB
	Cache *cache.Cache
}

func New(db *gorm.DB) *Service { return &Service{DB: db} }
func validateCategory(in *CategoryInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 120 {
		return problem.Invalid("Category name must contain 1 to 120 characters")
	}
	if len(in.Slug) > 120 || !slugPattern.MatchString(in.Slug) {
		return problem.Invalid("Category slug is invalid")
	}
	return nil
}
func (s *Service) categories(ctx context.Context, page users.Page) (users.List[Category], error) {
	result := users.List[Category]{Items: []Category{}, Page: page.Number, PageSize: page.Size}
	q := s.DB.WithContext(ctx).Model(&Category{}).Where("deleted_at IS NULL")
	if page.Query != "" {
		q = q.Where("name ILIKE ?", users.Like(page.Query))
	}
	if err := q.Count(&result.Total).Error; err != nil {
		return result, err
	}
	err := q.Order("name ASC, id ASC").Limit(page.Size).Offset((page.Number - 1) * page.Size).Find(&result.Items).Error
	return result, err
}
func (s *Service) category(ctx context.Context, id string) (Category, error) {
	var result Category
	if !users.ValidID(id) {
		return result, problem.Missing()
	}
	err := s.DB.WithContext(ctx).Where("id=? AND deleted_at IS NULL", id).Take(&result).Error
	return result, err
}
func (s *Service) SaveCategory(ctx context.Context, id string, in CategoryInput) (Category, error) {
	if err := validateCategory(&in); err != nil {
		return Category{}, err
	}
	if id != "" && !users.ValidID(id) {
		return Category{}, problem.Missing()
	}
	var result Category
	if id == "" {
		result = Category{ID: uuid.NewString(), Name: in.Name, Slug: in.Slug, Version: 1}
		err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if e := tx.Create(&result).Error; e != nil {
				return e
			}
			return outbox.Write(tx, "category", result.ID, 1, "category.changed", "", map[string]any{"category_id": result.ID, "category_version": 1, "change_kind": "created"})
		})
		return result, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).Take(&result).Error; err != nil {
			return err
		}
		if e := tx.Model(&Category{}).Where("id=?", id).Updates(map[string]any{"name": in.Name, "slug": in.Slug, "version": gorm.Expr("version+1"), "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		return outbox.Write(tx, "category", id, result.Version+1, "category.changed", "", map[string]any{"category_id": id, "category_version": result.Version + 1, "change_kind": "updated"})
	})
	if err != nil {
		return result, err
	}
	return s.Category(ctx, id)
}
func (s *Service) DeleteCategory(ctx context.Context, id string) error {
	if !users.ValidID(id) {
		return problem.Missing()
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var result Category
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).Take(&result).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM product_categories WHERE category_id=?", id).Error; err != nil {
			return err
		}
		if e := tx.Model(&Category{}).Where("id=?", id).Updates(map[string]any{"deleted_at": time.Now().UTC(), "version": gorm.Expr("version+1"), "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		return outbox.Write(tx, "category", id, result.Version+1, "category.changed", "", map[string]any{"category_id": id, "category_version": result.Version + 1, "change_kind": "deleted"})
	})
}
func validateProduct(in *ProductInput) error {
	in.SKU = strings.ToUpper(strings.TrimSpace(in.SKU))
	in.Name = strings.TrimSpace(in.Name)
	if n := len(in.SKU); n < 1 || n > 64 {
		return problem.Invalid("SKU must contain 1 to 64 bytes")
	}
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 200 {
		return problem.Invalid("Product name must contain 1 to 200 characters")
	}
	if utf8.RuneCountInString(in.Description) > 10000 || in.PriceMinor < 0 || in.PriceMinor > MaxMoney || in.OriginalPriceMinor < 0 || in.OriginalPriceMinor > MaxMoney {
		return problem.Invalid("Product description or price is invalid")
	}
	if in.Status != "Draft" && in.Status != "Active" && in.Status != "Inactive" {
		return problem.Invalid("Product status is invalid")
	}
	if len(in.Images) > 10 || len(in.CategoryIDs) > 100 {
		return problem.Invalid("Too many product images or categories")
	}
	for _, image := range in.Images {
		u, err := url.Parse(image.URL)
		if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || len(image.URL) > 2048 || utf8.RuneCountInString(image.Alt) > 200 || image.SortOrder < 0 {
			return problem.Invalid("Product image must be an HTTP(S) URL with valid metadata")
		}
	}
	seen := map[string]bool{}
	for _, id := range in.CategoryIDs {
		if !users.ValidID(id) || seen[id] {
			return problem.Invalid("Category IDs must be unique UUIDs")
		}
		seen[id] = true
	}
	sort.Strings(in.CategoryIDs)
	return nil
}
func productQuery(db *gorm.DB, admin bool) *gorm.DB {
	q := db.Table("products AS p").Select("p.*, inventory.quantity AS stock").Joins("JOIN inventory ON inventory.product_id=p.id").Where("p.deleted_at IS NULL")
	if !admin {
		q = q.Where("p.status='Active'")
	}
	return q
}
func hydrate(db *gorm.DB, p *Product) error {
	p.Images = []Image{}
	p.CategoryIDs = []string{}
	p.OutOfStock = p.Stock == 0
	if err := db.Where("product_id=?", p.ID).Order("sort_order ASC, id ASC").Find(&p.Images).Error; err != nil {
		return err
	}
	return db.Table("product_categories AS pc").Joins("JOIN categories c ON c.id=pc.category_id AND c.deleted_at IS NULL").Where("pc.product_id=?", p.ID).Order("pc.category_id ASC").Pluck("pc.category_id", &p.CategoryIDs).Error
}
func (s *Service) product(ctx context.Context, id string, admin bool) (Product, error) {
	var result Product
	if !users.ValidID(id) {
		return result, problem.Missing()
	}
	db := s.DB.WithContext(ctx)
	if err := productQuery(db, admin).Where("p.id=?", id).Take(&result).Error; err != nil {
		return result, err
	}
	err := hydrate(db, &result)
	return result, err
}
func (s *Service) products(ctx context.Context, page users.Page, admin bool) (users.List[Product], error) {
	result := users.List[Product]{Items: []Product{}, Page: page.Number, PageSize: page.Size}
	db := s.DB.WithContext(ctx)
	q := productQuery(db, admin)
	if page.Query != "" {
		like := users.Like(page.Query)
		q = q.Where("p.name ILIKE ? OR p.sku ILIKE ?", like, like)
	}
	if page.CategoryID != "" {
		if !users.ValidID(page.CategoryID) {
			return result, problem.Invalid("category_id must be UUID")
		}
		q = q.Where("EXISTS (SELECT 1 FROM product_categories pc JOIN categories c ON c.id=pc.category_id AND c.deleted_at IS NULL WHERE pc.product_id=p.id AND pc.category_id=?)", page.CategoryID)
	}
	order := map[string]string{"": "p.created_at DESC, p.id DESC", "created_at": "p.created_at DESC, p.id DESC", "name": "p.name ASC, p.id ASC", "price_asc": "p.price_minor ASC, p.id ASC", "price_desc": "p.price_minor DESC, p.id DESC"}[page.Sort]
	if order == "" {
		return result, problem.Invalid("Unsupported product sort")
	}
	if err := q.Count(&result.Total).Error; err != nil {
		return result, err
	}
	if err := q.Order(order).Limit(page.Size).Offset((page.Number - 1) * page.Size).Find(&result.Items).Error; err != nil {
		return result, err
	}
	for i := range result.Items {
		if err := hydrate(db, &result.Items[i]); err != nil {
			return result, err
		}
	}
	return result, nil
}
func (s *Service) SaveProduct(ctx context.Context, id string, in ProductInput) (Product, error) {
	if err := validateProduct(&in); err != nil {
		return Product{}, err
	}
	if id != "" && !users.ValidID(id) {
		return Product{}, problem.Missing()
	}
	newProduct := id == ""
	if newProduct {
		id = uuid.NewString()
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if !newProduct {
			var old Product
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).Take(&old).Error; err != nil {
				return err
			}
		}
		if len(in.CategoryIDs) > 0 {
			var categories []Category
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id IN ? AND deleted_at IS NULL", in.CategoryIDs).Order("id ASC").Find(&categories).Error; err != nil {
				return err
			}
			if len(categories) != len(in.CategoryIDs) {
				return problem.Invalid("A selected category is not available")
			}
		}
		if newProduct {
			p := Product{ID: id, SKU: in.SKU, Name: in.Name, Description: in.Description, PriceMinor: in.PriceMinor, OriginalPriceMinor: in.OriginalPriceMinor, Status: in.Status, Version: 1}
			if err := tx.Create(&p).Error; err != nil {
				return err
			}
			if err := inventory.EnsureZero(tx, id); err != nil {
				return err
			}
		} else {
			if err := tx.Model(&Product{}).Where("id=?", id).Updates(map[string]any{"sku": in.SKU, "name": in.Name, "description": in.Description, "price_minor": in.PriceMinor, "original_price_minor": in.OriginalPriceMinor, "status": in.Status, "version": gorm.Expr("version+1"), "updated_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("product_id=?", id).Delete(&Image{}).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM product_categories WHERE product_id=?", id).Error; err != nil {
			return err
		}
		for _, image := range in.Images {
			if err := tx.Create(&Image{ID: uuid.NewString(), ProductID: id, URL: image.URL, Alt: image.Alt, SortOrder: image.SortOrder}).Error; err != nil {
				return err
			}
		}
		for _, categoryID := range in.CategoryIDs {
			if err := tx.Exec("INSERT INTO product_categories (product_id,category_id) VALUES (?,?)", id, categoryID).Error; err != nil {
				return err
			}
		}
		var version int64
		if e := tx.Table("products").Where("id=?", id).Pluck("version", &version).Error; e != nil {
			return e
		}
		return outbox.Write(tx, "product", id, version, "product.changed", "", map[string]any{"product_id": id, "product_version": version, "change_kind": "saved"})
	})
	if err != nil {
		return Product{}, err
	}
	return s.Product(ctx, id, true)
}
func (s *Service) DeleteProduct(ctx context.Context, id string) error {
	if !users.ValidID(id) {
		return problem.Missing()
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p Product
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND deleted_at IS NULL", id).Take(&p).Error; e != nil {
			return e
		}
		if e := tx.Model(&Product{}).Where("id=?", id).Updates(map[string]any{"deleted_at": time.Now().UTC(), "status": "Inactive", "version": p.Version + 1, "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		return outbox.Write(tx, "product", id, p.Version+1, "product.changed", "", map[string]any{"product_id": id, "product_version": p.Version + 1, "change_kind": "deleted"})
	})
}

func (s *Service) Categories(ctx context.Context, p users.Page) (users.List[Category], error) {
	raw, _ := json.Marshal(p)
	return cache.Read(ctx, s.DB, s.Cache, "categories:"+string(raw), 30*time.Second, func(tx *gorm.DB) (users.List[Category], error) { return New(tx).categories(ctx, p) })
}
func (s *Service) Category(ctx context.Context, id string) (Category, error) {
	return cache.Read(ctx, s.DB, s.Cache, "category:"+id, 60*time.Second, func(tx *gorm.DB) (Category, error) { return New(tx).category(ctx, id) })
}
func (s *Service) Product(ctx context.Context, id string, admin bool) (Product, error) {
	if admin {
		return s.product(ctx, id, true)
	}
	return cache.Read(ctx, s.DB, s.Cache, "product:"+id, 60*time.Second, func(tx *gorm.DB) (Product, error) { return New(tx).product(ctx, id, false) })
}
func (s *Service) Products(ctx context.Context, p users.Page, admin bool) (users.List[Product], error) {
	if admin {
		return s.products(ctx, p, true)
	}
	raw, _ := json.Marshal(p)
	return cache.Read(ctx, s.DB, s.Cache, "products:"+string(raw), 30*time.Second, func(tx *gorm.DB) (users.List[Product], error) { return New(tx).products(ctx, p, false) })
}
