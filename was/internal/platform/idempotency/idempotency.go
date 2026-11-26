package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"

	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/problem"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

func Retry(ctx context.Context, db *gorm.DB, work func(*gorm.DB) error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if e := ctx.Err(); e != nil {
			return e
		}
		err = db.WithContext(ctx).Transaction(work)
		var pg *pgconn.PgError
		if !errors.As(err, &pg) || (pg.Code != "40001" && pg.Code != "40P01") {
			return err
		}
	}
	return err
}
func Run[T any](ctx context.Context, db *gorm.DB, actorID, operation, key string, input any, admin bool, work func(*gorm.DB) (T, error)) (T, error) {
	var result T
	if !keyPattern.MatchString(key) {
		return result, problem.Invalid("A valid Idempotency-Key is required")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return result, err
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	err = Retry(ctx, db, func(tx *gorm.DB) error {
		var zero T
		result = zero
		user, e := users.Get(tx, actorID)
		if e != nil {
			return e
		}
		if user.Status != "Active" || (admin && user.Role != "Admin") {
			return problem.Forbidden()
		}
		if e = tx.Exec("INSERT INTO idempotency_records(id,actor_id,operation,key,payload_hash) VALUES (?,?,?,?,?) ON CONFLICT(actor_id,operation,key) DO NOTHING", uuid.NewString(), actorID, operation, key, hash).Error; e != nil {
			return e
		}
		var record struct {
			ID           string
			PayloadHash  string
			ResponseBody json.RawMessage
		}
		if e = tx.Table("idempotency_records").Clauses(clause.Locking{Strength: "UPDATE"}).Where("actor_id=? AND operation=? AND key=?", actorID, operation, key).Take(&record).Error; e != nil {
			return e
		}
		if record.PayloadHash != hash {
			return problem.New(409, "IDEMPOTENCY_CONFLICT", "The key belongs to a different intent")
		}
		if len(record.ResponseBody) > 0 {
			return json.Unmarshal(record.ResponseBody, &result)
		}
		result, e = work(tx)
		if e != nil {
			return e
		}
		response, e := json.Marshal(result)
		if e != nil {
			return e
		}
		status := 200
		if operation == "orders.checkout" || operation == "payments.begin" {
			status = 201
		}
		return tx.Exec("UPDATE idempotency_records SET response_status=?,response_body=?::jsonb WHERE id=?", status, string(response), record.ID).Error
	})
	return result, err
}
