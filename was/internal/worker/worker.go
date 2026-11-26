package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"ecommerce.local/base/internal/infrastructure/rabbitmq"
	"ecommerce.local/base/internal/platform/metrics"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm"
)

type Record struct {
	ID, Type, AggregateID, AggregateType, CorrelationID string
	Version                                             int
	AggregateVersion                                    int64
	Payload                                             json.RawMessage
	OccurredAt                                          time.Time
	Attempts                                            int
	ClaimToken                                          string
}
type Worker struct {
	DB          *gorm.DB
	Processor   *Processor
	URL, Prefix string
	Ready       atomic.Bool
}

func Claim(ctx context.Context, db *gorm.DB) (Record, bool, error) {
	var row Record
	token := uuid.NewString()
	r := db.WithContext(ctx).Raw(`WITH candidate AS (SELECT id FROM outbox_events WHERE published_at IS NULL AND next_attempt_at<=now() AND (lease_until IS NULL OR lease_until<now()) ORDER BY occurred_at,id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE outbox_events o SET claim_token=?,lease_until=now()+interval '10 seconds',attempts=o.attempts+1 FROM candidate c WHERE o.id=c.id RETURNING o.*`, token).Scan(&row)
	return row, r.RowsAffected == 1, r.Error
}
func Mark(ctx context.Context, db *gorm.DB, r Record) error {
	result := db.WithContext(ctx).Exec("UPDATE outbox_events SET published_at=now(),claim_token=NULL,lease_until=NULL,last_error='' WHERE id=? AND claim_token=? AND lease_until>now() AND published_at IS NULL", r.ID, r.ClaimToken)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("outbox claim expired")
	}
	return nil
}
func Release(ctx context.Context, db *gorm.DB, r Record) error {
	return db.WithContext(ctx).Exec("UPDATE outbox_events SET claim_token=NULL,lease_until=NULL,last_error='PUBLISH_FAILED',next_attempt_at=now()+least(attempts,30)*interval '1 second' WHERE id=? AND claim_token=? AND published_at IS NULL", r.ID, r.ClaimToken).Error
}
func RelayOnce(ctx context.Context, db *gorm.DB, b *rabbitmq.Broker) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	row, ok, e := Claim(ctx, db)
	if e != nil || !ok {
		if e != nil {
			metrics.Add("ecom_relay_failures_total", 1)
		}
		return false, e
	}
	event := Event{row.ID, row.Type, row.Version, row.AggregateID, row.AggregateType, row.AggregateVersion, row.OccurredAt, row.CorrelationID, row.Payload}
	raw, e := json.Marshal(event)
	if e == nil {
		e = b.Event(ctx, event.Type, event.ID, raw)
	}
	if e != nil {
		_ = Release(ctx, db, row)
		metrics.Add("ecom_relay_failures_total", 1)
		return true, e
	}
	e = Mark(ctx, db, row)
	if e == nil {
		metrics.Add("ecom_relay_published_total", 1)
		slog.Info("outbox published", "event_id", row.ID, "event_type", row.Type, "aggregate_id", row.AggregateID, "request_id", row.CorrelationID)
	}
	return true, e
}
func (w *Worker) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		b, e := rabbitmq.Open(w.URL, w.Prefix)
		if e != nil {
			w.Ready.Store(false)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
				continue
			}
		}
		w.Ready.Store(true)
		session, cancel := context.WithCancel(ctx)
		errs := make(chan error, 5)
		var wg sync.WaitGroup
		for _, purpose := range []string{"notifications", "catalog"} {
			for _, retry := range []bool{false, true} {
				wg.Add(1)
				go func(p string, retry bool) { defer wg.Done(); errs <- w.consume(session, b, p, retry) }(purpose, retry)
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for session.Err() == nil {
				worked, e := RelayOnce(session, w.DB, b)
				if e != nil {
					errs <- e
					return
				}
				if !worked {
					select {
					case <-session.Done():
						return
					case <-time.After(100 * time.Millisecond):
					}
				}
			}
		}()
		select {
		case <-ctx.Done():
		case <-errs:
		}
		w.Ready.Store(false)
		cancel()
		b.Close()
		wg.Wait()
		if ctx.Err() != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
		}
	}
	return nil
}
func (w *Worker) consume(ctx context.Context, b *rabbitmq.Broker, purpose string, retry bool) error {
	ch, e := b.Conn.Channel()
	if e != nil {
		return e
	}
	defer ch.Close()
	if e = ch.Qos(1, 0, false); e != nil {
		return e
	}
	q := b.Queue(purpose)
	if retry {
		q += ".retry"
	}
	deliveries, e := ch.ConsumeWithContext(ctx, q, "", false, false, false, false, nil)
	if e != nil {
		return e
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("consumer closed")
			}
			attempt := 0
			validHeader := true
			if raw, ok := d.Headers["attempt"]; ok {
				switch value := raw.(type) {
				case int32:
					attempt = int(value)
				case int64:
					attempt = int(value)
				case string:
					n, e := strconv.Atoi(value)
					attempt = n
					validHeader = e == nil
				default:
					validHeader = false
				}
			}
			if attempt < 0 || attempt > 3 {
				validHeader = false
			}
			if retry {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(time.Second):
				}
			}
			var event Event
			err := json.Unmarshal(d.Body, &event)
			if err != nil || !validHeader || d.ContentType != "application/json" || len(d.Body) > 65536 || d.MessageId != event.ID {
				err = &Permanent{"invalid delivery"}
			} else {
				messageCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				err = w.Processor.Process(messageCtx, purpose, event)
				cancel()
			}
			if ctx.Err() != nil {
				return nil
			}
			if err == nil {
				if purpose == "notifications" {
					MockEmail(event)
				}
				metrics.Add("ecom_consumer_success_total", 1)
				if e = d.Ack(false); e != nil {
					return e
				}
				continue
			}
			target := b.Queue(purpose) + ".retry"
			metric := "ecom_consumer_retry_total"
			if IsPermanent(err) || attempt >= 3 {
				target = b.Queue(purpose) + ".dlq"
				metric = "ecom_consumer_dlq_total"
			}
			headers := amqp.Table{"attempt": int32(attempt + 1), "source_consumer": purpose}
			if e = b.Publish(ctx, "", target, amqp.Publishing{ContentType: "application/json", MessageId: d.MessageId, Body: d.Body, Headers: headers}); e != nil {
				return fmt.Errorf("retry transfer failed")
			}
			metrics.Add(metric, 1)
			if e = d.Ack(false); e != nil {
				return e
			}
		}
	}
}
