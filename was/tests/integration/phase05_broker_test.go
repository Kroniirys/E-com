//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"ecommerce.local/base/internal/infrastructure/rabbitmq"
	"ecommerce.local/base/internal/modules/orders"
	"ecommerce.local/base/internal/platform/cache"
	"ecommerce.local/base/internal/worker"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

func testBroker(t *testing.T) (*rabbitmq.Broker, string) {
	t.Helper()
	raw := os.Getenv("TEST_RABBITMQ_URL")
	if raw == "" {
		t.Fatal("TEST_RABBITMQ_URL is required for real AMQP integration")
	}
	prefix := "ecom_it_" + uuid.NewString()
	b, e := rabbitmq.Open(raw, prefix)
	requireOK(t, e)
	t.Cleanup(func() {
		b.Close()
		cleanup, e := rabbitmq.Open(raw, prefix)
		if e == nil {
			ch, e := cleanup.Conn.Channel()
			if e == nil {
				for _, purpose := range []string{"notifications", "catalog"} {
					for _, suffix := range []string{"", ".retry", ".dlq"} {
						_, _ = ch.QueueDelete(cleanup.Queue(purpose)+suffix, false, false, false)
					}
				}
				_ = ch.ExchangeDelete(prefix+".events", false, false)
				ch.Close()
			}
			cleanup.Close()
		}
	})
	return b, raw
}
func await(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("bounded wait timed out")
}
func TestPhase05RabbitMandatoryConfirmCrashReplay(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	b, _ := testBroker(t)
	if b.Publish(ctx, b.Prefix+".events", "unroutable.test", amqp.Publishing{ContentType: "application/json", MessageId: uuid.NewString(), Body: []byte(`{}`)}) == nil {
		t.Fatal("mandatory return was treated as success")
	}
	f.product(t, "AMQP replay", "Active")
	r, ok, e := worker.Claim(ctx, f.DB)
	requireOK(t, e)
	if !ok {
		t.Fatal("no outbox record")
	}
	event := worker.Event{ID: r.ID, Type: r.Type, Version: r.Version, AggregateID: r.AggregateID, AggregateType: r.AggregateType, AggregateVersion: r.AggregateVersion, OccurredAt: r.OccurredAt, Payload: r.Payload}
	raw, e := json.Marshal(event)
	requireOK(t, e)
	requireOK(t, b.Event(ctx, r.Type, r.ID, raw))
	if count(t, f, "outbox_events", "published_at IS NOT NULL") != 0 {
		t.Fatal("published without mark")
	}
	requireOK(t, f.DB.Exec("UPDATE outbox_events SET lease_until=now()-interval '1 second' WHERE id=?", r.ID).Error)
	next, ok, e := worker.Claim(ctx, f.DB)
	requireOK(t, e)
	if !ok || next.ID != r.ID {
		t.Fatal("claim not recovered")
	}
	requireOK(t, b.Event(ctx, r.Type, r.ID, raw))
	requireOK(t, worker.Mark(ctx, f.DB, next))
	ch, e := b.Conn.Channel()
	requireOK(t, e)
	defer ch.Close()
	processor := worker.Processor{DB: f.DB, Cache: cache.New(f.Limiter.Client, f.DSN)}
	for i := 0; i < 2; i++ {
		d, ok, e := ch.Get(b.Queue("catalog"), false)
		requireOK(t, e)
		if !ok || d.MessageId != r.ID {
			t.Fatal("lost duplicate identity")
		}
		requireOK(t, processor.Process(ctx, "catalog", event))
		requireOK(t, d.Ack(false))
	}
	if count(t, f, "processed_events", "consumer='catalog'") != 1 {
		t.Fatal("duplicate inbox processing")
	}
}
func TestPhase05RabbitRetryDLQAndRecovery(t *testing.T) {
	f := fixture(t)
	b, raw := testBroker(t)
	ctx, cancel := context.WithCancel(context.Background())
	w := worker.Worker{DB: f.DB, URL: raw, Prefix: b.Prefix, Processor: &worker.Processor{DB: f.DB, Cache: cache.New(f.Limiter.Client, f.DSN)}}
	done := make(chan error, 1)
	requireOK(t, f.DB.Exec("ALTER TABLE processed_events RENAME TO paused_inbox").Error)
	go func() { done <- w.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("worker did not shut down")
		}
	})
	p := stockProduct(t, f, 1, 3)
	u, o := deliveredFixture(t, f, 1, p)
	ch, e := b.Conn.Channel()
	requireOK(t, e)
	defer ch.Close()
	await(t, func() bool {
		q, e := ch.QueueInspect(b.Queue("notifications") + ".retry")
		return e == nil && q.Messages > 0
	})
	requireOK(t, f.DB.Exec("ALTER TABLE paused_inbox RENAME TO processed_events").Error)
	await(t, func() bool { return count(t, f, "notifications", "user_id=?", u.ID) == 6 })
	var paymentBody string
	requireOK(t, f.DB.Table("notifications").Where("order_id=? AND kind='payment.completed'", o.ID).Pluck("body", &paymentBody).Error)
	if !strings.Contains(paymentBody, "Đã thanh toán") {
		t.Fatal("payment notification state missing")
	}
	if count(t, f, "outbox_events", "aggregate_id=? AND published_at IS NULL", o.ID) != 0 {
		t.Fatal("relay backlog not marked")
	}
	poison := worker.Event{ID: uuid.NewString(), Version: 99, Type: "order.created", AggregateType: "order", AggregateID: o.ID, AggregateVersion: 1, OccurredAt: time.Now(), Payload: json.RawMessage(`{}`)}
	body, _ := json.Marshal(poison)
	requireOK(t, b.Publish(ctx, "", b.Queue("notifications"), amqp.Publishing{ContentType: "application/json", MessageId: poison.ID, Body: body}))
	await(t, func() bool {
		q, e := ch.QueueInspect(b.Queue("notifications") + ".dlq")
		return e == nil && q.Messages == 1
	})
	d, ok, e := ch.Get(b.Queue("notifications")+".dlq", false)
	requireOK(t, e)
	if !ok || d.MessageId != poison.ID {
		t.Fatal("DLQ changed identity")
	}
	requireOK(t, b.Publish(ctx, "", b.Queue("notifications"), amqp.Publishing{ContentType: d.ContentType, MessageId: d.MessageId, Body: d.Body, Headers: amqp.Table{"attempt": int32(0)}}))
	requireOK(t, d.Ack(false))
	await(t, func() bool {
		q, e := ch.QueueInspect(b.Queue("notifications") + ".dlq")
		return e == nil && q.Messages == 1
	})
	if count(t, f, "notifications", "user_id=?", u.ID) != 6 {
		t.Fatal("poison replay changed side effects")
	}
}

func TestPhase05CommitBeforeAckRedelivery(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	b, _ := testBroker(t)
	p := stockProduct(t, f, 1, 2)
	u, a := buyer(t, f, 1)
	svc := orders.New(f.DB, true)
	o, e := svc.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "COD"), "")
	requireOK(t, e)
	var r worker.Record
	requireOK(t, f.DB.Table("outbox_events").Where("aggregate_id=? AND type='order.created'", o.ID).Take(&r).Error)
	event := worker.Event{ID: r.ID, Type: r.Type, Version: r.Version, AggregateID: r.AggregateID, AggregateType: r.AggregateType, AggregateVersion: r.AggregateVersion, OccurredAt: r.OccurredAt, Payload: r.Payload}
	body, _ := json.Marshal(event)
	requireOK(t, b.Event(ctx, r.Type, r.ID, body))
	ch, e := b.Conn.Channel()
	requireOK(t, e)
	d, ok, e := ch.Get(b.Queue("notifications"), false)
	requireOK(t, e)
	if !ok {
		t.Fatal("missing delivery")
	}
	processor := worker.Processor{DB: f.DB}
	requireOK(t, processor.Process(ctx, "notifications", event))
	requireOK(t, ch.Close())
	next, e := b.Conn.Channel()
	requireOK(t, e)
	defer next.Close()
	var redelivery amqp.Delivery
	await(t, func() bool {
		d, ok, e := next.Get(b.Queue("notifications"), false)
		if e != nil {
			t.Error(e)
			return false
		}
		if ok {
			redelivery = d
		}
		return ok
	})
	if redelivery.MessageId != d.MessageId || !redelivery.Redelivered {
		t.Fatal("unacked committed event was not redelivered")
	}
	requireOK(t, processor.Process(ctx, "notifications", event))
	requireOK(t, redelivery.Ack(false))
	if count(t, f, "notifications", "event_id=?", event.ID) != 1 || count(t, f, "processed_events", "consumer='notifications' AND event_id=?", event.ID) != 1 {
		t.Fatal("commit-before-ack duplicated side effect")
	}
}
func TestPhase05TransientRetryExhaustionAndDLQRecovery(t *testing.T) {
	f := fixture(t)
	b, raw := testBroker(t)
	ctx, cancel := context.WithCancel(context.Background())
	p := stockProduct(t, f, 1, 2)
	u, a := buyer(t, f, 1)
	o, e := orders.New(f.DB, true).Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "COD"), "")
	requireOK(t, e)
	requireOK(t, f.DB.Exec("ALTER TABLE processed_events RENAME TO paused_inbox").Error)
	service := worker.Worker{DB: f.DB, URL: raw, Prefix: b.Prefix, Processor: &worker.Processor{DB: f.DB, Cache: cache.New(f.Limiter.Client, f.DSN)}}
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("worker shutdown timeout")
		}
	})
	ch, e := b.Conn.Channel()
	requireOK(t, e)
	defer ch.Close()
	await(t, func() bool {
		q, e := ch.QueueInspect(b.Queue("notifications") + ".dlq")
		return e == nil && q.Messages == 1
	})
	d, ok, e := ch.Get(b.Queue("notifications")+".dlq", false)
	requireOK(t, e)
	if !ok || d.Headers["attempt"] != int32(4) {
		t.Fatal("transient delivery did not exhaust three retries", d.Headers)
	}
	if count(t, f, "notifications", "order_id=?", o.ID) != 0 {
		t.Fatal("failed transaction made notification")
	}
	requireOK(t, f.DB.Exec("ALTER TABLE paused_inbox RENAME TO processed_events").Error)
	requireOK(t, b.Publish(ctx, "", b.Queue("notifications"), amqp.Publishing{ContentType: d.ContentType, MessageId: d.MessageId, Body: d.Body, Headers: amqp.Table{"attempt": int32(0)}}))
	requireOK(t, d.Ack(false))
	await(t, func() bool { return count(t, f, "notifications", "order_id=?", o.ID) == 1 })
}

func TestPhase05TwoRelaysClaimDistinctEvents(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	first, raw := testBroker(t)
	second, e := rabbitmq.Open(raw, first.Prefix)
	requireOK(t, e)
	defer second.Close()
	for i := 0; i < 8; i++ {
		f.product(t, fmt.Sprintf("Relay product %d", i), "Active")
	}
	var wg sync.WaitGroup
	for _, broker := range []*rabbitmq.Broker{first, second} {
		wg.Add(1)
		go func(b *rabbitmq.Broker) {
			defer wg.Done()
			for {
				worked, e := worker.RelayOnce(ctx, f.DB, b)
				if e != nil {
					t.Error(e)
					return
				}
				if !worked {
					return
				}
			}
		}(broker)
	}
	wg.Wait()
	if count(t, f, "outbox_events", "published_at IS NOT NULL") != 8 {
		t.Fatal("relay missed events")
	}
	ch, e := first.Conn.Channel()
	requireOK(t, e)
	defer ch.Close()
	q, e := ch.QueueInspect(first.Queue("catalog"))
	requireOK(t, e)
	if q.Messages != 8 {
		t.Fatal("concurrent relays duplicated or lost publication", q.Messages)
	}
}
