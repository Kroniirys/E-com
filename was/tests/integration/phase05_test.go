//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"ecommerce.local/base/internal/modules/catalog"
	"ecommerce.local/base/internal/modules/notifications"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/cache"
	"ecommerce.local/base/internal/platform/limit"
	"ecommerce.local/base/internal/worker"

	"gorm.io/gorm"
)

func TestPhase05CacheGenerationLateFillVisibilityFallback(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	p := f.product(t, "Cache old", "Active")
	c := cache.New(f.Limiter.Client, f.DSN)
	s := catalog.New(f.DB)
	s.Cache = c
	loaded := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, e := cache.Read(ctx, f.DB, c, "late-fill", time.Minute, func(tx *gorm.DB) (catalog.Product, error) {
			v, e := catalog.New(tx).Product(ctx, p.ID, false)
			close(loaded)
			<-release
			return v, e
		})
		done <- e
	}()
	<-loaded
	input := catalog.ProductInput{SKU: p.SKU, Name: "Cache new", PriceMinor: 2000, OriginalPriceMinor: 2000, Status: "Active", Images: []catalog.ImageInput{}, CategoryIDs: []string{}}
	_, e := f.Catalog.SaveProduct(ctx, p.ID, input)
	requireOK(t, e)
	close(release)
	requireOK(t, <-done)
	v, e := cache.Read(ctx, f.DB, c, "late-fill", time.Minute, func(tx *gorm.DB) (catalog.Product, error) { return catalog.New(tx).Product(ctx, p.ID, false) })
	requireOK(t, e)
	hitLoaderCalled := false
	_, e = cache.Read(ctx, f.DB, c, "late-fill", time.Minute, func(tx *gorm.DB) (catalog.Product, error) {
		hitLoaderCalled = true
		return catalog.New(tx).Product(ctx, p.ID, false)
	})
	requireOK(t, e)
	if hitLoaderCalled {
		t.Fatal("cache did not serve a same-generation hit")
	}
	if v.Name != "Cache new" {
		t.Fatal("late fill poisoned new generation")
	}
	_, e = s.Products(ctx, users.Page{Number: 1, Size: 20}, false)
	requireOK(t, e)
	requireOK(t, f.Catalog.DeleteProduct(ctx, p.ID))
	vlist, e := s.Products(ctx, users.Page{Number: 1, Size: 20}, false)
	requireOK(t, e)
	for _, v := range vlist.Items {
		if v.ID == p.ID {
			t.Fatal("cached inactive product leaked")
		}
	}
	bad, e := limit.New("redis://127.0.0.1:1/0", "test:")
	requireOK(t, e)
	defer bad.Client.Close()
	s.Cache = cache.New(bad.Client, f.DSN)
	_, e = s.Categories(ctx, users.Page{Number: 1, Size: 20})
	requireOK(t, e)
}
func TestPhase05NotificationDedupeOwnershipAndOrdering(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	p := stockProduct(t, f, 1, 5)
	u, o := deliveredFixture(t, f, 1, p)
	var rows []worker.Record
	requireOK(t, f.DB.Table("outbox_events").Where("aggregate_id=? AND aggregate_type='order'", o.ID).Order("aggregate_version DESC").Find(&rows).Error)
	processor := worker.Processor{DB: f.DB, Cache: cache.New(f.Limiter.Client, f.DSN)}
	var wg sync.WaitGroup
	for _, r := range rows {
		event := worker.Event{ID: r.ID, Type: r.Type, Version: r.Version, AggregateID: r.AggregateID, AggregateType: r.AggregateType, AggregateVersion: r.AggregateVersion, OccurredAt: r.OccurredAt, Payload: r.Payload}
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func(e worker.Event) {
				defer wg.Done()
				if err := processor.Process(ctx, "notifications", e); err != nil {
					t.Error(err)
				}
			}(event)
		}
	}
	wg.Wait()
	if count(t, f, "notifications", "user_id=?", u.ID) != int64(len(rows)) {
		t.Fatal("duplicate notification")
	}
	service := notifications.New(f.DB)
	list, e := service.List(ctx, u.ID, users.Page{Number: 1, Size: 20})
	requireOK(t, e)
	if list.UnreadCount != int64(len(rows)) || list.Items[0].AggregateVersion != 5 {
		t.Fatal("notification order/count incorrect")
	}
	if service.Read(ctx, f.Customer.ID, list.Items[0].ID) == nil {
		t.Fatal("notification IDOR")
	}
	requireOK(t, service.Read(ctx, u.ID, list.Items[0].ID))
	requireOK(t, service.Read(ctx, u.ID, list.Items[0].ID))
	row := rows[0]
	var payload map[string]any
	requireOK(t, json.Unmarshal(row.Payload, &payload))
	payload["user_id"] = f.Customer.ID
	raw, _ := json.Marshal(payload)
	event := worker.Event{ID: "00000000-0000-0000-0000-000000000005", Type: row.Type, Version: 1, AggregateID: row.AggregateID, AggregateType: row.AggregateType, AggregateVersion: row.AggregateVersion, OccurredAt: row.OccurredAt, Payload: raw}
	if e = processor.Process(ctx, "notifications", event); !worker.IsPermanent(e) {
		t.Fatal("forged owner accepted", e)
	}
}
func TestPhase05ClaimOwnershipAndLeaseRecovery(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	f.product(t, "Claim", "Active")
	first, ok, e := worker.Claim(ctx, f.DB)
	requireOK(t, e)
	if !ok {
		t.Fatal("no producer event")
	}
	requireOK(t, f.DB.Exec("UPDATE outbox_events SET lease_until=now()-interval '1 second' WHERE id=?", first.ID).Error)
	second, ok, e := worker.Claim(ctx, f.DB)
	requireOK(t, e)
	if !ok || second.ID != first.ID || second.ClaimToken == first.ClaimToken {
		t.Fatal("expired claim not reclaimed")
	}
	if worker.Mark(ctx, f.DB, first) == nil {
		t.Fatal("stale worker marked new claim")
	}
	requireOK(t, worker.Mark(ctx, f.DB, second))
}
