package bootstrap

import (
	"context"
	"ecommerce.local/base/internal/modules/cart"
	"ecommerce.local/base/internal/modules/catalog"
	"ecommerce.local/base/internal/modules/coupons"
	"ecommerce.local/base/internal/modules/inventory"
	"ecommerce.local/base/internal/modules/orders"
	"ecommerce.local/base/internal/modules/reviews"
	"ecommerce.local/base/internal/modules/users"
	"errors"
	"gorm.io/gorm"
	"time"
)

// Phase04Demo is opt-in and creates a delivered purchase through the real services.
func Phase04Demo(ctx context.Context, db *gorm.DB, opts DemoOptions) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var admin, customer users.User
		if e := users.Query(tx).Where("u.email=?", authEmail(opts.AdminEmail)).Take(&admin).Error; e != nil {
			return e
		}
		if e := users.AdminGuard(tx, admin.ID); e != nil {
			return e
		}
		if e := users.Query(tx).Where("u.email=?", authEmail(opts.CustomerEmail)).Take(&customer).Error; e != nil {
			return e
		}
		var completed int64
		if e := tx.Table("idempotency_records").Where("actor_id=? AND operation='orders.checkout' AND key='seed-phase04-checkout-v1'", customer.ID).Count(&completed).Error; e != nil {
			return e
		}
		if completed > 0 {
			return nil
		}
		locked, e := users.Locked(tx, customer.ID)
		if e != nil {
			return e
		}
		if locked.Status != "Active" {
			return errors.New("demo customer must be active")
		}
		row, e := cart.Locked(tx, customer.ID)
		if e != nil {
			return e
		}
		var count int64
		if e = tx.Table("cart_items").Where("cart_id=?", row.ID).Count(&count).Error; e != nil {
			return e
		}
		if count > 0 {
			return errors.New("refusing to modify a nonempty customer cart")
		}
		c, e := coupons.New(tx).Save(ctx, admin.ID, "", coupons.Input{Code: "BASE10", Kind: "Percentage", Value: 10, UsageLimit: 100, ExpiresAt: time.Now().UTC().AddDate(1, 0, 0), Enabled: true})
		if e != nil {
			return e
		}
		p, e := catalog.New(tx).SaveProduct(ctx, "", catalog.ProductInput{SKU: "BASE-REVIEW", Name: "Sản phẩm mẫu có đánh giá", Description: "Fixture Delivered cho Phase 04", PriceMinor: 100000, OriginalPriceMinor: 100000, Status: "Active", Images: []catalog.ImageInput{}, CategoryIDs: []string{}})
		if e != nil {
			return e
		}
		if _, e = inventory.New(tx).Adjust(ctx, admin.ID, "seed-phase04-stock-v1", inventory.Adjustment{ProductID: p.ID, Delta: 1, Reason: "Phase04 demo stock"}); e != nil {
			return e
		}
		address, e := users.New(tx).SaveAddress(ctx, customer.ID, "", users.AddressInput{Recipient: "Demo Customer", Phone: "0900000000", Address: "Địa chỉ mẫu cho fixture Phase 04", IsDefault: false})
		if e != nil {
			return e
		}
		view, e := cart.New(tx, true).Mutate(ctx, customer.ID, "add", "", p.ID, 1)
		if e != nil {
			return e
		}
		service := orders.New(tx, true)
		preview, e := service.PreviewCoupon(ctx, customer.ID, c.Code, view.Version)
		if e != nil {
			return e
		}
		o, e := service.Checkout(ctx, customer.ID, "seed-phase04-checkout-v1", orders.CheckoutInput{AddressID: address.ID, CartVersion: view.Version, QuoteHash: preview.QuoteHash, CouponCode: c.Code, PaymentMethod: "COD"}, "seed-phase04")
		if e != nil {
			return e
		}
		for _, status := range []string{"Confirmed", "Processing", "Shipped", "Delivered"} {
			o, e = service.Transition(ctx, admin.ID, o.ID, "seed-phase04-"+status, status, "Demo lifecycle", "seed-phase04", true)
			if e != nil {
				return e
			}
		}
		_, e = reviews.New(tx).Create(ctx, customer.ID, p.ID, reviews.Input{Rating: 5, Content: "Đánh giá mẫu từ đơn đã giao thành công."})
		return e
	})
}
