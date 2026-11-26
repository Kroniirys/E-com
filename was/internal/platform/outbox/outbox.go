package outbox

import (
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func Write(tx *gorm.DB, kind, id string, version int64, event, correlation string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	eventID := uuid.NewString()
	err = tx.Exec("INSERT INTO outbox_events(id,aggregate_id,aggregate_type,type,version,aggregate_version,correlation_id,payload) VALUES (?,?,?,?,1,?,?,?::jsonb)", eventID, id, kind, event, version, correlation, string(raw)).Error
	if err == nil {
		slog.InfoContext(tx.Statement.Context, "outbox event staged", "event_id", eventID, "event_type", event, "aggregate_id", id, "request_id", correlation)
	}
	return err
}
