package problem

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type Error struct {
	Status     int
	Code       string
	Message    string
	RetryAfter int
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}
func Invalid(message string) *Error { return New(400, "INVALID_INPUT", message) }
func Unauthorized() *Error {
	return New(401, "UNAUTHORIZED", "Authentication is required or the session is no longer valid")
}
func Forbidden() *Error {
	return New(403, "FORBIDDEN", "You do not have permission for this operation")
}
func Missing() *Error { return New(404, "NOT_FOUND", "The requested resource is not available") }
func Normalize(err error) *Error {
	var p *Error
	if errors.As(err, &p) {
		return p
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Missing()
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return New(409, "CONFLICT", "A unique value is already in use")
		case "23503", "23514", "22P02":
			return Invalid("The request violates a data constraint")
		case "40001", "40P01":
			return New(409, "RETRY_REQUIRED", "Concurrent data changed; retry the operation")
		}
	}
	return New(503, "DEPENDENCY_UNAVAILABLE", "A required service is temporarily unavailable")
}
