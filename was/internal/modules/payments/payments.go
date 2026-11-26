package payments

import (
	"time"

	"ecommerce.local/base/internal/platform/problem"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Payment struct {
	ID          string     `json:"id"`
	OrderID     string     `json:"order_id"`
	Method      string     `json:"method"`
	Status      string     `json:"status"`
	AmountMinor int64      `json:"amount_minor"`
	PaidAt      *time.Time `json:"-"`
	Attempts    []Attempt  `json:"attempts" gorm:"-"`
}
type Attempt struct {
	ID          string    `json:"id"`
	PaymentID   string    `json:"payment_id"`
	Status      string    `json:"status"`
	FailureCode string    `json:"failure_code"`
	CreatedAt   time.Time `json:"created_at"`
}

func (Attempt) TableName() string { return "payment_attempts" }
func Create(tx *gorm.DB, orderID, method string, total int64) (Payment, error) {
	p := Payment{ID: uuid.NewString(), OrderID: orderID, Method: method, Status: "Pending", AmountMinor: total, Attempts: []Attempt{}}
	return p, tx.Create(&p).Error
}
func Locked(tx *gorm.DB, orderID string) (Payment, error) {
	var p Payment
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_id=?", orderID).Take(&p).Error
	return p, err
}
func Begin(tx *gorm.DB, p Payment) (Attempt, error) {
	var a Attempt
	if p.Method != "Mock" || p.Status != "Pending" {
		return a, problem.New(409, "INVALID_TRANSITION", "Payment cannot start a new attempt")
	}
	var count int64
	if err := tx.Model(&Attempt{}).Where("payment_id=? AND status IN ('Pending','Paid')", p.ID).Count(&count).Error; err != nil {
		return a, err
	}
	if count != 0 {
		return a, problem.New(409, "ATTEMPT_PENDING", "Finish the existing pending attempt first")
	}
	a = Attempt{ID: uuid.NewString(), PaymentID: p.ID, Status: "Pending"}
	return a, tx.Create(&a).Error
}
func Resolve(tx *gorm.DB, p Payment, id, result, code string) (Attempt, error) {
	var a Attempt
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND payment_id=?", id, p.ID).Take(&a).Error; err != nil {
		return a, err
	}
	if a.Status != "Pending" || p.Status != "Pending" {
		return a, problem.New(409, "INVALID_TRANSITION", "The payment attempt is already final")
	}
	a.Status = "Failed"
	a.FailureCode = code
	if result == "Success" {
		a.Status = "Paid"
		a.FailureCode = ""
	}
	if err := tx.Model(&Attempt{}).Where("id=?", a.ID).Updates(map[string]any{"status": a.Status, "failure_code": a.FailureCode}).Error; err != nil {
		return a, err
	}
	if a.Status == "Paid" {
		return a, MarkPaid(tx, p.ID)
	}
	return a, nil
}
func MarkPaid(tx *gorm.DB, id string) error {
	return tx.Model(&Payment{}).Where("id=? AND status='Pending'", id).Updates(map[string]any{"status": "Paid", "paid_at": time.Now().UTC()}).Error
}
func Cancel(tx *gorm.DB, p Payment) error {
	if err := tx.Model(&Attempt{}).Where("payment_id=? AND status='Pending'", p.ID).Update("status", "Cancelled").Error; err != nil {
		return err
	}
	return tx.Model(&Payment{}).Where("id=? AND status='Pending'", p.ID).Update("status", "Cancelled").Error
}
