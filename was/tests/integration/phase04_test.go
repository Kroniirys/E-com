//go:build integration

package integration

import (
	"context"
	"crypto/sha256"
	"ecommerce.local/base/internal/bootstrap"
	"ecommerce.local/base/internal/database"
	"ecommerce.local/base/internal/modules/admin"
	"ecommerce.local/base/internal/modules/catalog"
	"ecommerce.local/base/internal/modules/coupons"
	"ecommerce.local/base/internal/modules/orders"
	"ecommerce.local/base/internal/modules/reviews"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/problem"
	"ecommerce.local/base/migrations"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func couponFixture(t *testing.T, f *phaseFixture, limit int64) coupons.Coupon {
	t.Helper()
	c, e := coupons.New(f.DB).Save(context.Background(), f.Admin.ID, "", coupons.Input{Code: " test10 ", Kind: "Percentage", Value: 10, ExpiresAt: time.Now().Add(time.Hour), UsageLimit: limit, Enabled: true})
	requireOK(t, e)
	return c
}
func TestPhase04CouponConcurrentLimitSnapshotAndReplay(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	s := orders.New(f.DB, true)
	c := couponFixture(t, f, 3)
	p := stockProduct(t, f, 1, 5)
	type intent struct {
		user string
		in   orders.CheckoutInput
		key  string
	}
	var intents []intent
	for i := 0; i < 20; i++ {
		u, a := buyer(t, f, i)
		in := quote(t, f, u, a, p, "COD")
		preview, e := s.PreviewCoupon(ctx, u.ID, c.Code, in.CartVersion)
		requireOK(t, e)
		in.CouponCode = c.Code
		in.QuoteHash = preview.QuoteHash
		intents = append(intents, intent{u.ID, in, uuid.NewString()})
	}
	var wg sync.WaitGroup
	var successes atomic.Int32
	results := make(chan intent, 20)
	for _, v := range intents {
		wg.Add(1)
		go func(v intent) {
			defer wg.Done()
			o, e := s.Checkout(ctx, v.user, v.key, v.in, "")
			if e == nil {
				if o.DiscountMinor != 100 || o.TotalMinor != 30900 {
					t.Error("wrong discount")
				}
				successes.Add(1)
				results <- v
			} else if problem.Normalize(e).Status != 409 {
				t.Error(e)
			}
		}(v)
	}
	wg.Wait()
	close(results)
	if successes.Load() != 3 || quantity(t, f, p) != 2 || count(t, f, "coupon_usages", "true") != 3 {
		t.Fatal("usage/stock invariant failed", successes.Load())
	}
	current, e := coupons.New(f.DB).Get(ctx, c.ID)
	requireOK(t, e)
	if current.UsedCount != 3 || current.Version != 1 {
		t.Fatal("consumption changed terms")
	}
	for v := range results {
		o, e := s.Checkout(ctx, v.user, v.key, v.in, "")
		requireOK(t, e)
		_, e = s.Transition(ctx, v.user, o.ID, uuid.NewString(), "Cancelled", "", "", false)
		requireOK(t, e)
	}
	current, e = coupons.New(f.DB).Get(ctx, c.ID)
	requireOK(t, e)
	if current.UsedCount != 3 {
		t.Fatal("cancel refunded coupon usage")
	}
	t.Log("20 buyers: 3 coupon orders, used_count=3, no duplicate replay or coupon refund")
}
func TestPhase04CouponTermsAndRollback(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	s := orders.New(f.DB, true)
	c := couponFixture(t, f, 5)
	p := stockProduct(t, f, 1, 4)
	u, a := buyer(t, f, 1)
	in := quote(t, f, u, a, p, "COD")
	preview, e := s.PreviewCoupon(ctx, u.ID, c.Code, in.CartVersion)
	requireOK(t, e)
	in.CouponCode = c.Code
	in.QuoteHash = preview.QuoteHash
	input := coupons.Input{Code: c.Code, Kind: c.Kind, Value: 20, ExpiresAt: c.ExpiresAt, UsageLimit: 5, Enabled: true}
	_, e = coupons.New(f.DB).Save(ctx, f.Admin.ID, c.ID, input)
	requireOK(t, e)
	if _, e = s.Checkout(ctx, u.ID, uuid.NewString(), in, ""); problem.Normalize(e).Code != "QUOTE_CHANGED" {
		t.Fatal("stale coupon terms accepted", e)
	}
	preview, e = s.PreviewCoupon(ctx, u.ID, c.Code, in.CartVersion)
	requireOK(t, e)
	in.QuoteHash = preview.QuoteHash
	requireOK(t, f.DB.Exec("ALTER TABLE outbox_events ADD CONSTRAINT reject_coupon_event CHECK(type<>'order.created')").Error)
	if _, e = s.Checkout(ctx, u.ID, uuid.NewString(), in, ""); e == nil {
		t.Fatal("fault injection failed")
	}
	current, e := coupons.New(f.DB).Get(ctx, c.ID)
	requireOK(t, e)
	if current.UsedCount != 0 || quantity(t, f, p) != 4 || count(t, f, "coupon_usages", "true") != 0 {
		t.Fatal("rollback consumed coupon/stock")
	}
	requireOK(t, f.DB.Exec("ALTER TABLE outbox_events DROP CONSTRAINT reject_coupon_event").Error)
	key := uuid.NewString()
	o, e := s.Checkout(ctx, u.ID, key, in, "")
	requireOK(t, e)
	requireOK(t, coupons.New(f.DB).Archive(ctx, f.Admin.ID, c.ID))
	again, e := s.Checkout(ctx, u.ID, key, in, "")
	requireOK(t, e)
	if again.ID != o.ID || again.CouponSnapshot.DiscountMinor != 200 {
		t.Fatal("archived coupon changed replay snapshot")
	}
}
func deliveredFixture(t *testing.T, f *phaseFixture, n int, p string) (users.User, orders.Order) {
	t.Helper()
	ctx := context.Background()
	u, a := buyer(t, f, n)
	s := orders.New(f.DB, true)
	o, e := s.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "COD"), "")
	requireOK(t, e)
	for _, status := range []string{"Confirmed", "Processing", "Shipped", "Delivered"} {
		o, e = s.Transition(ctx, f.Admin.ID, o.ID, uuid.NewString(), status, "Fixture delivery", "", true)
		requireOK(t, e)
	}
	return u, o
}
func TestPhase04ReviewEligibilityVisibilityAndUnique(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	p := stockProduct(t, f, 1, 8)
	s := reviews.New(f.DB)
	u, _ := buyer(t, f, 1)
	if _, e := s.Create(ctx, u.ID, p, reviews.Input{Rating: 5, Content: "Not purchased"}); problem.Normalize(e).Status != 403 {
		t.Fatal("unearned review accepted")
	}
	buyer1, _ := deliveredFixture(t, f, 2, p)
	buyer2, _ := deliveredFixture(t, f, 3, p)
	var wg sync.WaitGroup
	var success atomic.Int32
	var reviewID string
	var mu sync.Mutex
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.Create(ctx, buyer1.ID, p, reviews.Input{Rating: 5, Content: "<script>unsafe()</script>"})
			if e == nil {
				success.Add(1)
				mu.Lock()
				reviewID = r.ID
				mu.Unlock()
			} else if problem.Normalize(e).Status != 409 {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatal("duplicate reviews", success.Load())
	}
	_, e := s.Create(ctx, buyer2.ID, p, reviews.Input{Rating: 3, Content: "Useful"})
	requireOK(t, e)
	page := users.Page{Number: 1, Size: 20}
	v, e := s.Public(ctx, p, page)
	requireOK(t, e)
	if v.Total != 2 || v.Summary.AverageRating != 4 {
		t.Fatal("wrong aggregate", v.Summary)
	}
	_, e = s.Moderate(ctx, f.Admin.ID, reviewID, "Hidden", "Moderation reason")
	requireOK(t, e)
	v, e = s.Public(ctx, p, page)
	requireOK(t, e)
	if v.Total != 1 || v.Summary.AverageRating != 3 {
		t.Fatal("hidden review included")
	}
	eligible, e := s.Eligibility(ctx, buyer1.ID, p)
	requireOK(t, e)
	if eligible.CanReview || eligible.Review == nil || eligible.Review.Visibility != "Hidden" {
		t.Fatal("owner lost hidden review")
	}
	if e = s.Delete(ctx, u.ID, reviewID); problem.Normalize(e).Status != 404 {
		t.Fatal("review ownership missing")
	}
	requireOK(t, s.Delete(ctx, buyer1.ID, reviewID))
	if _, e = s.Create(ctx, buyer1.ID, p, reviews.Input{Rating: 4, Content: "Recreate"}); problem.Normalize(e).Status != 409 {
		t.Fatal("soft delete bypassed uniqueness")
	}
	requireOK(t, f.DB.Exec("UPDATE products SET status='Inactive' WHERE id=?", p).Error)
	if _, e = s.Public(ctx, p, page); problem.Normalize(e).Status != 404 {
		t.Fatal("inactive product review leaked")
	}
}
func TestPhase04DashboardDateAndExactSum(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	p := stockProduct(t, f, 1, 6)
	for i := 0; i < 2; i++ {
		_, o := deliveredFixture(t, f, i, p)
		requireOK(t, f.DB.Exec("UPDATE orders SET created_at=? WHERE id=?", time.Date(2026, 11, 25, 17+i, 0, 0, 0, time.UTC), o.ID).Error)
	}
	var first struct{ ID string }
	requireOK(t, f.DB.Table("orders").Order("created_at,id").Take(&first).Error)
	requireOK(t, f.DB.Exec("UPDATE order_items SET unit_price_minor=8999999970000,line_total_minor=8999999970000 WHERE order_id=?", first.ID).Error)
	requireOK(t, f.DB.Exec("UPDATE orders SET subtotal_minor=8999999970000,total_minor=9000000000000 WHERE id=?", first.ID).Error)
	requireOK(t, f.DB.Exec("UPDATE payments SET amount_minor=9000000000000 WHERE order_id=?", first.ID).Error)
	for i, status := range []string{"Pending", "Cancelled", "Delivered"} {
		u, a := buyer(t, f, 10+i)
		svc := orders.New(f.DB, true)
		o, e := svc.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "COD"), "")
		requireOK(t, e)
		if status == "Cancelled" {
			_, e = svc.Transition(ctx, u.ID, o.ID, uuid.NewString(), "Cancelled", "", "", false)
			requireOK(t, e)
		}
		if status == "Delivered" {
			requireOK(t, f.DB.Exec("UPDATE orders SET status='Delivered' WHERE id=?", o.ID).Error)
		}
		requireOK(t, f.DB.Exec("UPDATE orders SET created_at='2026-11-25T20:00:00Z' WHERE id=?", o.ID).Error)
	}
	span, e := admin.ParseRange("2026-11-26", "2026-11-27", "Asia/Ho_Chi_Minh")
	requireOK(t, e)
	d, e := admin.New(f.DB).Get(ctx, span)
	requireOK(t, e)
	if d.OrderCount != 5 || d.DeliveredPaidTotalMinor != "9000000031000" {
		t.Fatal(fmt.Sprintf("unexpected dashboard %+v", d))
	}
	span, e = admin.ParseRange("2026-11-27", "2026-11-28", "Asia/Ho_Chi_Minh")
	requireOK(t, e)
	d, e = admin.New(f.DB).Get(ctx, span)
	requireOK(t, e)
	if d.OrderCount != 0 || d.DeliveredPaidTotalMinor != "0" {
		t.Fatal("exclusive date boundary failed")
	}
}

func TestPhase04OptInSeedKeepsBusinessData(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	opts := bootstrap.DemoOptions{AdminEmail: f.Admin.Email, AdminPassword: testPassword, CustomerEmail: f.Customer.Email, CustomerPassword: testPassword}
	requireOK(t, bootstrap.Phase04Demo(ctx, f.DB, opts))
	requireOK(t, bootstrap.Phase04Demo(ctx, f.DB, opts))
	if count(t, f, "orders", "status='Delivered'") != 1 || count(t, f, "reviews", "true") != 1 || count(t, f, "coupon_usages", "true") != 1 {
		t.Fatal("seed did not preserve delivered relation or duplicated fixtures")
	}
}

func TestPhase04LegacyCheckoutReplayAfterUpgrade(t *testing.T) {
	db, dsn := isolatedDB(t)
	ctx := context.Background()
	m, e := migrations.New(dsn)
	requireOK(t, e)
	defer m.Close()
	requireOK(t, m.Steps(3))
	requireOK(t, database.Seed(ctx, db, "Legacy", 30000))
	u, e := users.Insert(db, users.Input{Email: "legacy@phase04.test", Name: "Legacy", Role: "Customer", Status: "Active"}, "unused-legacy-fixture-hash")
	requireOK(t, e)
	p, e := catalog.New(db).SaveProduct(ctx, "", catalog.ProductInput{SKU: "LEGACY", Name: "Legacy product", PriceMinor: 1000, OriginalPriceMinor: 1000, Status: "Active", Images: []catalog.ImageInput{}, CategoryIDs: []string{}})
	requireOK(t, e)
	id := uuid.NewString()
	payment := uuid.NewString()
	addressID := uuid.NewString()
	requireOK(t, db.Exec("INSERT INTO orders(id,user_id,status,currency,subtotal_minor,discount_minor,shipping_minor,total_minor,shipping_address) VALUES (?,?,'Pending','VND',1000,0,30000,31000,'{\"recipient\":\"Legacy\",\"phone\":\"0901234567\",\"address\":\"Legacy address\"}'::jsonb)", id, u.ID).Error)
	requireOK(t, db.Exec("INSERT INTO payments(id,order_id,method,status,amount_minor) VALUES (?,?,'COD','Pending',31000)", payment, id).Error)
	requireOK(t, db.Exec("INSERT INTO order_items(id,order_id,product_id,sku,name,image_url,quantity,unit_price_minor,line_total_minor) VALUES (?,?,?,'LEGACY','Legacy product','',1,1000,1000)", uuid.NewString(), id, p.ID).Error)
	oldRequest := struct {
		AddressID     string `json:"address_id"`
		CartVersion   int64  `json:"cart_version"`
		QuoteHash     string `json:"quote_hash"`
		PaymentMethod string `json:"payment_method"`
	}{addressID, 2, strings.Repeat("a", 64), "COD"}
	raw, e := json.Marshal(oldRequest)
	requireOK(t, e)
	sum := sha256.Sum256(raw)
	o, e := orders.New(db, true).Get(ctx, u.ID, id, false)
	requireOK(t, e)
	body, e := json.Marshal(o)
	requireOK(t, e)
	var legacy map[string]json.RawMessage
	requireOK(t, json.Unmarshal(body, &legacy))
	delete(legacy, "coupon_snapshot")
	body, e = json.Marshal(legacy)
	requireOK(t, e)
	key := uuid.NewString()
	requireOK(t, db.Exec("INSERT INTO idempotency_records(id,actor_id,operation,key,payload_hash,response_status,response_body) VALUES (?,?,'orders.checkout',?,?,201,?::jsonb)", uuid.NewString(), u.ID, key, hex.EncodeToString(sum[:]), string(body)).Error)
	requireOK(t, migrations.Up(m))
	var input orders.CheckoutInput
	requireOK(t, json.Unmarshal(raw, &input))
	updated, e := json.Marshal(input)
	requireOK(t, e)
	if string(updated) != string(raw) {
		t.Fatal("legacy request fingerprint changed")
	}
	replay, e := orders.New(db, true).Checkout(ctx, u.ID, key, input, "")
	requireOK(t, e)
	if replay.ID != id || replay.TotalMinor != 31000 || replay.CouponSnapshot != nil {
		t.Fatal("legacy replay changed")
	}
}

func TestPhase04RollbackGuardAndModerationHTTP(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	p := stockProduct(t, f, 1, 3)
	u, _ := deliveredFixture(t, f, 1, p)
	r, e := reviews.New(f.DB).Create(ctx, u.ID, p, reviews.Input{Rating: 5, Content: "HTTP moderation"})
	requireOK(t, e)
	token := f.login(t, f.Admin.Email)
	code, body, _ := f.request(t, "PATCH", "/api/v1/admin/reviews/"+r.ID+"/moderation", token.AccessToken, map[string]string{"visibility": "Hidden", "reason": "Audit reason"}, "", nil)
	assertStatus(t, code, 200, body)
	var response struct{ Data map[string]json.RawMessage }
	requireOK(t, json.Unmarshal(body, &response))
	for _, key := range []string{"moderation_reason", "moderated_by", "moderated_at"} {
		if _, ok := response.Data[key]; !ok {
			t.Fatal("missing moderation DTO field", key)
		}
	}
	m, e := migrations.New(f.DSN)
	requireOK(t, e)
	defer m.Close()
	if e = m.Steps(-1); e == nil {
		t.Fatal("rollback discarded review data")
	}
	if count(t, f, "reviews", "id=?", r.ID) != 1 {
		t.Fatal("rollback guard modified data")
	}
}
