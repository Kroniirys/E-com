package inventory

import (
	"time"

	"ecommerce.local/base/internal/platform/outbox"
	"ecommerce.local/base/internal/platform/problem"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Change must be called inside the caller's transaction with products ordered by ID.
func Change(tx *gorm.DB, productID, actorID string, delta int64, reason, reference, correlation string) error {
	var stock Stock
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("product_id=?", productID).Take(&stock).Error; err != nil {
		return err
	}
	if delta == 0 || delta > MaxQuantity || delta < -MaxQuantity {
		return problem.Invalid("Invalid stock delta")
	}
	if delta < 0 && stock.Quantity < -delta {
		return problem.New(409, "INSUFFICIENT_STOCK", "Not enough stock")
	}
	if delta > 0 && stock.Quantity > MaxQuantity-delta {
		return problem.New(409, "STOCK_LIMIT", "Stock would exceed its limit")
	}
	before := stock.Quantity
	stock.Quantity += delta
	stock.Version++
	if err := tx.Model(&Stock{}).Where("product_id=?", productID).Updates(map[string]any{"quantity": stock.Quantity, "version": stock.Version, "updated_at": time.Now().UTC()}).Error; err != nil {
		return err
	}
	if err := tx.Create(&Transaction{ID: uuid.NewString(), ProductID: productID, ActorID: actorID, Delta: delta, BeforeQuantity: before, AfterQuantity: stock.Quantity, Reason: reason, Reference: reference}).Error; err != nil {
		return err
	}
	return outbox.Write(tx, "inventory", productID, stock.Version, "inventory.updated", correlation, map[string]any{"product_id": productID, "quantity": stock.Quantity, "inventory_version": stock.Version})
}
