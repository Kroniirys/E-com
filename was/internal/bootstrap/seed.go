package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ecommerce.local/base/internal/modules/catalog"
	"ecommerce.local/base/internal/modules/inventory"
	"ecommerce.local/base/internal/modules/users"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type DemoOptions struct{ AdminEmail, AdminPassword, CustomerEmail, CustomerPassword string }

func (o *DemoOptions) Validate() error {
	for _, input := range []users.Input{{Email: o.AdminEmail, Name: "Demo Admin", Role: "Admin", Status: "Active", Password: o.AdminPassword}, {Email: o.CustomerEmail, Name: "Demo Customer", Role: "Customer", Status: "Active", Password: o.CustomerPassword}} {
		candidate := input
		if err := users.Validate(&candidate); err != nil {
			return err
		}
		if !users.ValidPassword(input.Password) {
			return fmt.Errorf("demo passwords must contain at least 12 characters and at most 72 UTF-8 bytes")
		}
	}
	if authEmail(o.AdminEmail) == authEmail(o.CustomerEmail) {
		return fmt.Errorf("demo admin and customer emails must differ")
	}
	return nil
}
func authEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
func Demo(ctx context.Context, db *gorm.DB, opts DemoOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	adminHash, err := bcrypt.GenerateFromPassword([]byte(opts.AdminPassword), 12)
	if err != nil {
		return err
	}
	customerHash, err := bcrypt.GenerateFromPassword([]byte(opts.CustomerPassword), 12)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ensureUser := func(email, name, role, hash string) (users.User, error) {
			var found users.User
			err := users.Query(tx).Where("u.email=?", authEmail(email)).Take(&found).Error
			if err == nil {
				if found.Role != role {
					return found, fmt.Errorf("configured demo email belongs to a different role")
				}
				return found, nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return found, err
			}
			return users.Insert(tx, users.Input{Email: authEmail(email), Name: name, Role: role, Status: "Active"}, hash)
		}
		admin, err := ensureUser(opts.AdminEmail, "Demo Admin", "Admin", string(adminHash))
		if err != nil {
			return err
		}
		if _, err := ensureUser(opts.CustomerEmail, "Demo Customer", "Customer", string(customerHash)); err != nil {
			return err
		}
		catService := catalog.New(tx)
		invService := inventory.New(tx)
		categoryIDs := map[string]string{}
		for _, input := range []catalog.CategoryInput{{Name: "Đèn & ánh sáng", Slug: "lighting"}, {Name: "Ghế & nội thất", Slug: "furniture"}} {
			var found catalog.Category
			err := tx.Where("slug=?", input.Slug).Take(&found).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				found, err = catService.SaveCategory(ctx, "", input)
			}
			if err != nil {
				return err
			}
			if found.DeletedAt == nil {
				categoryIDs[input.Slug] = found.ID
			}
		}
		fixtures := []struct {
			SKU, Name, Status, Category string
			Price, Stock                int64
		}{{"BASE-LAMP", "Đèn bàn Halo", "Active", "lighting", 890000, 12}, {"BASE-CHAIR", "Ghế gỗ Nordic", "Active", "furniture", 1490000, 8}, {"BASE-EMPTY", "Đèn treo Luna", "Active", "lighting", 1290000, 0}, {"BASE-DRAFT", "Ghế mẫu thử", "Draft", "furniture", 990000, 2}, {"BASE-INACTIVE", "Đèn ngừng bán", "Inactive", "lighting", 490000, 3}}
		for _, fixture := range fixtures {
			var count int64
			if err := tx.Table("products").Where("sku=?", fixture.SKU).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			ids := []string{}
			if id := categoryIDs[fixture.Category]; id != "" {
				ids = append(ids, id)
			}
			product, err := catService.SaveProduct(ctx, "", catalog.ProductInput{SKU: fixture.SKU, Name: fixture.Name, Description: "Sản phẩm mẫu phục vụ kiểm chứng và demo hệ thống.", Status: fixture.Status, PriceMinor: fixture.Price, OriginalPriceMinor: fixture.Price, Images: []catalog.ImageInput{}, CategoryIDs: ids})
			if err != nil {
				return err
			}
			if fixture.Stock > 0 {
				if _, err := invService.Adjust(ctx, admin.ID, "seed-stock-"+product.ID, inventory.Adjustment{ProductID: product.ID, Delta: fixture.Stock, Reason: "Initial demo stock"}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
