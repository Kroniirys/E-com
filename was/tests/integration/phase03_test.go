//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/internal/database"
	"ecommerce.local/base/internal/httpapi"
	"ecommerce.local/base/internal/httpserver"
	"ecommerce.local/base/internal/modules/cart"
	"ecommerce.local/base/internal/modules/catalog"
	"ecommerce.local/base/internal/modules/inventory"
	"ecommerce.local/base/internal/modules/orders"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/problem"
	"ecommerce.local/base/migrations"

	"github.com/google/uuid"
)

func requireOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func buyer(t *testing.T, f *phaseFixture, n int) (users.User, users.Address) {
	t.Helper()
	u, e := users.Insert(f.DB, users.Input{Email: fmt.Sprintf("buyer%d@phase03.test", n), Name: "Buyer", Role: "Customer", Status: "Active"}, string(sharedHash))
	requireOK(t, e)
	a, e := f.Users.SaveAddress(context.Background(), u.ID, "", users.AddressInput{Recipient: "Buyer", Phone: "0901234567", Address: "Original address", IsDefault: true})
	requireOK(t, e)
	return u, a
}
func stockProduct(t *testing.T, f *phaseFixture, n int, quantity int64) string {
	t.Helper()
	p := f.product(t, fmt.Sprintf("Phase03 product %d", n), "Active")
	_, e := f.Inventory.Adjust(context.Background(), f.Admin.ID, uuid.NewString(), inventory.Adjustment{ProductID: p.ID, Delta: quantity, Reason: "Phase03 stock"})
	requireOK(t, e)
	return p.ID
}
func quote(t *testing.T, f *phaseFixture, u users.User, a users.Address, product, method string) orders.CheckoutInput {
	t.Helper()
	v, e := cart.New(f.DB, true).Mutate(context.Background(), u.ID, "add", "", product, 1)
	requireOK(t, e)
	return orders.CheckoutInput{AddressID: a.ID, CartVersion: v.Version, QuoteHash: v.QuoteHash, PaymentMethod: method}
}
func count(t *testing.T, f *phaseFixture, table, where string, args ...any) int64 {
	t.Helper()
	var n int64
	requireOK(t, f.DB.Table(table).Where(where, args...).Count(&n).Error)
	return n
}
func quantity(t *testing.T, f *phaseFixture, id string) int64 {
	t.Helper()
	var q int64
	requireOK(t, f.DB.Table("inventory").Where("product_id=?", id).Pluck("quantity", &q).Error)
	return q
}
func TestPhase03CheckoutSnapshotsReplayAndOwnership(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	s := orders.New(f.DB, true)
	u, a := buyer(t, f, 1)
	p := stockProduct(t, f, 1, 5)
	in := quote(t, f, u, a, p, "COD")
	key := uuid.NewString()
	o, e := s.Checkout(ctx, u.ID, key, in, "checkout-test")
	requireOK(t, e)
	if o.Status != "Pending" || o.Payment.Status != "Pending" || o.TotalMinor != 31000 || len(o.Items) != 1 || len(o.History) != 1 {
		t.Fatalf("invalid checkout: %+v", o)
	}
	if quantity(t, f, p) != 4 {
		t.Fatal("stock not deducted")
	}
	requireOK(t, f.DB.Exec("UPDATE products SET name='Changed',price_minor=9000 WHERE id=?", p).Error)
	_, e = f.Users.SaveAddress(ctx, u.ID, a.ID, users.AddressInput{Recipient: "Changed", Phone: a.Phone, Address: "Changed address", IsDefault: true})
	requireOK(t, e)
	requireOK(t, f.DB.Exec("UPDATE store_settings SET shipping_fee_minor=90000").Error)
	restarted := orders.New(f.DB, true)
	again, e := restarted.Checkout(ctx, u.ID, key, in, "after-response-loss")
	requireOK(t, e)
	if again.ID != o.ID || again.TotalMinor != 31000 || again.ShippingAddress.Address != "Original address" || again.Items[0].UnitPriceMinor != 1000 {
		t.Fatal("replay/snapshot changed")
	}
	fresh, e := restarted.Get(ctx, u.ID, o.ID, false)
	requireOK(t, e)
	if fresh.Items[0].Name != o.Items[0].Name {
		t.Fatal("product snapshot changed")
	}
	changed := in
	changed.PaymentMethod = "Mock"
	if _, e = s.Checkout(ctx, u.ID, key, changed, ""); problem.Normalize(e).Status != 409 {
		t.Fatal("key payload conflict missing")
	}
	if _, e = s.Checkout(ctx, u.ID, uuid.NewString(), in, ""); problem.Normalize(e).Status != 409 {
		t.Fatal("consumed cart accepted")
	}
	other, _ := buyer(t, f, 2)
	if _, e = s.Get(ctx, other.ID, o.ID, false); problem.Normalize(e).Status != 404 {
		t.Fatal("order IDOR")
	}
	token, e := f.Auth.Login(ctx, other.Email, testPassword)
	requireOK(t, e)
	code, data, _ := f.request(t, "GET", "/api/v1/orders/"+o.ID, token.AccessToken, nil, "", nil)
	assertStatus(t, code, 404, data)
	code, data, _ = f.request(t, "GET", "/api/v1/admin/orders/"+o.ID, token.AccessToken, nil, "", nil)
	assertStatus(t, code, 403, data)
	if count(t, f, "outbox_events", "type='order.created'") != 1 || count(t, f, "inventory_transactions", "reason='order.checkout'") != 1 {
		t.Fatal("replayed side effects")
	}
	t.Logf("SQL: orders=%d, checkout_history=%d, order.created=%d, stock=%d", count(t, f, "orders", "true"), count(t, f, "inventory_transactions", "reason='order.checkout'"), count(t, f, "outbox_events", "type='order.created'"), quantity(t, f, p))
}
func TestPhase03StockFiveTwentyBuyers(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	p := stockProduct(t, f, 1, 5)
	s := orders.New(f.DB, true)
	type intent struct {
		User  users.User
		Input orders.CheckoutInput
	}
	intents := []intent{}
	for i := 0; i < 20; i++ {
		u, a := buyer(t, f, i)
		intents = append(intents, intent{u, quote(t, f, u, a, p, "COD")})
	}
	var success atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	failures := make(chan error, 20)
	for _, in := range intents {
		wg.Add(1)
		go func(in intent) {
			defer wg.Done()
			<-start
			_, e := s.Checkout(ctx, in.User.ID, uuid.NewString(), in.Input, "concurrent-buyers")
			if e == nil {
				success.Add(1)
			} else if problem.Normalize(e).Status != 409 {
				failures <- e
			}
		}(in)
	}
	close(start)
	wg.Wait()
	close(failures)
	for e := range failures {
		t.Error(e)
	}
	if success.Load() != 5 || quantity(t, f, p) != 0 || count(t, f, "orders", "true") != 5 || count(t, f, "inventory_transactions", "reason='order.checkout'") != 5 || count(t, f, "outbox_events", "type='order.created'") != 5 || count(t, f, "outbox_events", "type='inventory.updated'") != 5 {
		t.Fatalf("oversell or orphan effects: success=%d", success.Load())
	}
	t.Log("SQL: 20 buyers, 5 orders, stock=0, checkout histories=5, order.created=5, inventory.updated=5")
}
func TestPhase03SameCartConcurrentKeysAndMutation(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	p := stockProduct(t, f, 1, 10)
	u, a := buyer(t, f, 1)
	in := quote(t, f, u, a, p, "COD")
	s := orders.New(f.DB, true)
	key := uuid.NewString()
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o, e := s.Checkout(ctx, u.ID, key, in, "same-key")
			if e != nil {
				t.Error(e)
			} else {
				ids <- o.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for next := range ids {
		if id != "" && id != next {
			t.Fatal("same key created multiple orders")
		}
		id = next
	}
	if quantity(t, f, p) != 9 || count(t, f, "orders", "true") != 1 {
		t.Fatal("duplicate checkout")
	}
	v, e := cart.New(f.DB, true).Mutate(ctx, u.ID, "add", "", p, 1)
	requireOK(t, e)
	in.CartVersion = v.Version
	in.QuoteHash = v.QuoteHash
	var successes atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.Checkout(ctx, u.ID, uuid.NewString(), in, ""); e == nil {
				successes.Add(1)
			} else if problem.Normalize(e).Status != 409 {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("different keys consumed same cart twice")
	}
	v, e = cart.New(f.DB, true).Mutate(ctx, u.ID, "add", "", p, 1)
	requireOK(t, e)
	in.CartVersion = v.Version
	in.QuoteHash = v.QuoteHash
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, e := s.Checkout(ctx, u.ID, uuid.NewString(), in, "")
		if e != nil && problem.Normalize(e).Status != 409 {
			t.Error(e)
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		_, e := cart.New(f.DB, true).Mutate(ctx, u.ID, "update", v.Items[0].ID, "", 2)
		if e != nil && problem.Normalize(e).Status != 404 {
			t.Error(e)
		}
	}()
	close(start)
	wg.Wait()
	if count(t, f, "orders", "true") > 3 || quantity(t, f, p) < 7 {
		t.Fatal("cart edit race produced extra order/stock mutation")
	}
}
func TestPhase03QuoteValidationRollbackAndCartBoundaries(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	s := orders.New(f.DB, true)
	c := cart.New(f.DB, true)
	u, a := buyer(t, f, 1)
	p := stockProduct(t, f, 1, 5)
	p2 := stockProduct(t, f, 2, 5)
	in := quote(t, f, u, a, p, "COD")
	requireOK(t, f.DB.Exec("UPDATE products SET price_minor=2000,version=version+1 WHERE id=?", p).Error)
	if _, e := s.Checkout(ctx, u.ID, uuid.NewString(), in, ""); problem.Normalize(e).Code != "QUOTE_CHANGED" {
		t.Fatal("stale quote accepted", e)
	}
	v, e := c.Mutate(ctx, u.ID, "add", "", p2, 1)
	requireOK(t, e)
	in.CartVersion = v.Version
	in.QuoteHash = v.QuoteHash
	_, e = f.Inventory.Adjust(ctx, f.Admin.ID, uuid.NewString(), inventory.Adjustment{ProductID: p2, Delta: -5, Reason: "Sell elsewhere"})
	requireOK(t, e)
	if _, e = s.Checkout(ctx, u.ID, uuid.NewString(), in, ""); problem.Normalize(e).Status != 409 {
		t.Fatal("multi-item shortage accepted")
	}
	if quantity(t, f, p) != 5 || count(t, f, "orders", "true") != 0 {
		t.Fatal("partial checkout")
	}
	_, e = f.Inventory.Adjust(ctx, f.Admin.ID, uuid.NewString(), inventory.Adjustment{ProductID: p2, Delta: 5, Reason: "Restock"})
	requireOK(t, e)
	requireOK(t, f.DB.Exec("ALTER TABLE outbox_events ADD CONSTRAINT reject_order_event CHECK(type<>'order.created')").Error)
	if _, e = s.Checkout(ctx, u.ID, uuid.NewString(), in, ""); e == nil {
		t.Fatal("injected failure did not fail")
	}
	if quantity(t, f, p) != 5 || quantity(t, f, p2) != 5 || count(t, f, "orders", "true") != 0 || count(t, f, "payments", "true") != 0 || count(t, f, "outbox_events", "true") != 0 || count(t, f, "idempotency_records", "operation='orders.checkout'") != 0 || count(t, f, "cart_items", "true") != 2 {
		t.Fatal("transaction rollback left partial effects")
	}
	requireOK(t, f.DB.Exec("ALTER TABLE outbox_events DROP CONSTRAINT reject_order_event").Error)
	other, b := buyer(t, f, 2)
	bad := in
	bad.AddressID = b.ID
	if _, e = s.Checkout(ctx, u.ID, uuid.NewString(), bad, ""); problem.Normalize(e).Status != 404 {
		t.Fatal("address ownership missing")
	}
	if _, e = c.Mutate(ctx, other.ID, "delete", v.Items[0].ID, "", 0); problem.Normalize(e).Status != 404 {
		t.Fatal("cart item IDOR")
	}
	if _, e = c.Mutate(ctx, u.ID, "update", v.Items[0].ID, "", 0); problem.Normalize(e).Status != 400 {
		t.Fatal("quantity zero accepted")
	}
	requireOK(t, f.DB.Exec("UPDATE products SET price_minor=9000000000000 WHERE id=?", p).Error)
	v, e = c.Get(ctx, u.ID)
	requireOK(t, e)
	if v.PricingValid {
		t.Fatal("overflow quote accepted")
	}
	_, e = c.Mutate(ctx, u.ID, "delete", v.Items[0].ID, "", 0)
	requireOK(t, e)
	_, e = c.Mutate(ctx, u.ID, "clear", "", "", 0)
	requireOK(t, e)
	if count(t, f, "cart_items", "true") != 0 {
		t.Fatal("clear did not persist")
	}
}
func TestPhase03CODLifecycleCancelAndFailed(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	s := orders.New(f.DB, true)
	u, a := buyer(t, f, 1)
	p := stockProduct(t, f, 1, 8)
	o, e := s.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "COD"), "")
	requireOK(t, e)
	if _, e = s.Transition(ctx, u.ID, o.ID, uuid.NewString(), "Shipped", "", "", false); problem.Normalize(e).Status != 403 {
		t.Fatal("customer changed lifecycle")
	}
	for _, status := range []string{"Confirmed", "Processing", "Shipped", "Delivered"} {
		key := uuid.NewString()
		o, e = s.Transition(ctx, f.Admin.ID, o.ID, key, status, "Admin action", "", true)
		requireOK(t, e)
		again, e := s.Transition(ctx, f.Admin.ID, o.ID, key, status, "Admin action", "", true)
		requireOK(t, e)
		if again.Version != o.Version {
			t.Fatal("transition replay duplicated")
		}
	}
	if o.Payment.Status != "Paid" || len(o.History) != 5 || count(t, f, "outbox_events", "type='payment.completed'") != 1 {
		t.Fatal("COD delivered payment/history invalid")
	}
	if _, e = s.Transition(ctx, u.ID, o.ID, uuid.NewString(), "Cancelled", "", "", false); problem.Normalize(e).Status != 409 {
		t.Fatal("delivered order cancelled")
	}
	o, e = s.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "COD"), "")
	requireOK(t, e)
	key := uuid.NewString()
	_, e = s.Transition(ctx, u.ID, o.ID, key, "Cancelled", "Customer cancellation", "", false)
	requireOK(t, e)
	_, e = s.Transition(ctx, u.ID, o.ID, key, "Cancelled", "Customer cancellation", "", false)
	requireOK(t, e)
	if quantity(t, f, p) != 7 || count(t, f, "inventory_transactions", "reason='order.release'") != 1 {
		t.Fatal("double stock restore")
	}
	o, e = s.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "COD"), "")
	requireOK(t, e)
	o, e = s.Transition(ctx, f.Admin.ID, o.ID, uuid.NewString(), "Failed", "Final validation failure", "", true)
	requireOK(t, e)
	if o.Payment.Status != "Cancelled" || quantity(t, f, p) != 7 {
		t.Fatal("failed order did not release stock")
	}
}
func TestPhase03MockRetryReplayAndCancelRace(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	s := orders.New(f.DB, true)
	u, a := buyer(t, f, 1)
	p := stockProduct(t, f, 1, 10)
	o, e := s.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "Mock"), "")
	requireOK(t, e)
	if _, e = s.Transition(ctx, f.Admin.ID, o.ID, uuid.NewString(), "Confirmed", "", "", true); problem.Normalize(e).Status != 409 {
		t.Fatal("admin confirmed unpaid Mock")
	}
	first, e := s.BeginAttempt(ctx, u.ID, o.ID, uuid.NewString())
	requireOK(t, e)
	if _, e = s.BeginAttempt(ctx, u.ID, o.ID, uuid.NewString()); problem.Normalize(e).Status != 409 {
		t.Fatal("two pending attempts")
	}
	o, e = s.Simulate(ctx, u.ID, first.ID, uuid.NewString(), "Failure", "DECLINED", "")
	requireOK(t, e)
	if o.Status != "Pending" || o.Payment.Status != "Pending" || o.Payment.Attempts[0].Status != "Failed" {
		t.Fatal("attempt failure finalized order")
	}
	second, e := s.BeginAttempt(ctx, u.ID, o.ID, uuid.NewString())
	requireOK(t, e)
	key := uuid.NewString()
	o, e = s.Simulate(ctx, u.ID, second.ID, key, "Success", "", "")
	requireOK(t, e)
	_, e = s.Simulate(ctx, u.ID, second.ID, key, "Success", "", "")
	requireOK(t, e)
	if _, e = s.Simulate(ctx, u.ID, first.ID, uuid.NewString(), "Success", "", ""); problem.Normalize(e).Status != 409 {
		t.Fatal("failed attempt resurrected")
	}
	if o.Status != "Confirmed" || o.Payment.Status != "Paid" || count(t, f, "payment_attempts", "status='Paid'") != 1 || count(t, f, "outbox_events", "type='payment.completed'") != 1 {
		t.Fatal("payment replay invalid")
	}
	if _, e = s.Transition(ctx, u.ID, o.ID, uuid.NewString(), "Cancelled", "", "", false); problem.Normalize(e).Status != 409 {
		t.Fatal("paid order cancelled")
	}
	for i := 0; i < 8; i++ {
		o, e = s.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "Mock"), "")
		requireOK(t, e)
		attempt, e := s.BeginAttempt(ctx, u.ID, o.ID, uuid.NewString())
		requireOK(t, e)
		var wg sync.WaitGroup
		wg.Add(2)
		start := make(chan struct{})
		go func(id string) {
			defer wg.Done()
			<-start
			_, e := s.Transition(ctx, u.ID, id, uuid.NewString(), "Cancelled", "", "", false)
			if e != nil && problem.Normalize(e).Status != 409 {
				t.Error(e)
			}
		}(o.ID)
		go func() {
			defer wg.Done()
			<-start
			_, e := s.Simulate(ctx, u.ID, attempt.ID, uuid.NewString(), "Success", "", "")
			if e != nil && problem.Normalize(e).Status != 409 {
				t.Error(e)
			}
		}()
		close(start)
		wg.Wait()
		fresh, e := s.Get(ctx, u.ID, o.ID, false)
		requireOK(t, e)
		if fresh.Status == "Cancelled" {
			if fresh.Payment.Status != "Cancelled" {
				t.Fatal("cancel/payment split")
			}
			if _, e = s.Simulate(ctx, u.ID, attempt.ID, uuid.NewString(), "Success", "", ""); problem.Normalize(e).Status != 409 {
				t.Fatal("late payment accepted")
			}
		} else if fresh.Status != "Confirmed" || fresh.Payment.Status != "Paid" {
			t.Fatal("race split state")
		}
	}
	if count(t, f, "inventory", "quantity<0") != 0 {
		t.Fatal("negative stock")
	}
}
func TestPhase03HTTPStrictContractAndMockDisabled(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	u, a := buyer(t, f, 1)
	p := stockProduct(t, f, 1, 5)
	in := quote(t, f, u, a, p, "COD")
	token, e := f.Auth.Login(ctx, u.Email, testPassword)
	requireOK(t, e)
	key := uuid.NewString()
	code, data, _ := f.request(t, "POST", "/api/v1/orders", token.AccessToken, in, "", map[string]string{"Idempotency-Key": key})
	assertStatus(t, code, 201, data)
	var payload struct{ Data orders.Order }
	requireOK(t, json.Unmarshal(data, &payload))
	if payload.Data.ID == "" || strings.Contains(string(data), "password_hash") {
		t.Fatal("invalid order response")
	}
	code, data, _ = f.request(t, "POST", "/api/v1/orders", token.AccessToken, in, "", nil)
	assertStatus(t, code, 400, data)
	code, data, _ = f.request(t, "POST", "/api/v1/orders", token.AccessToken, map[string]any{"address_id": a.ID, "cart_version": 1, "quote_hash": in.QuoteHash, "payment_method": "COD", "total_minor": 1}, "", map[string]string{"Idempotency-Key": uuid.NewString()})
	assertStatus(t, code, 400, data)
	disabled := orders.New(f.DB, false)
	endpoints := &httpapi.API{Auth: f.Auth, Users: f.Users, Catalog: f.Catalog, Inventory: f.Inventory, Cart: cart.New(f.DB, false), Orders: disabled, Origins: []string{testOrigin}}
	server := httptest.NewServer(httpserver.New(config.Config{Environment: "test", AllowedOrigins: []string{testOrigin}, RequestTimeout: 5 * time.Second}, f.DB, slog.New(slog.NewTextHandler(io.Discard, nil)), endpoints.Register).Handler)
	defer server.Close()
	original := f.Server
	f.Server = server
	defer func() { f.Server = original }()
	for _, path := range []string{"/api/v1/payment-attempts/" + uuid.NewString() + "/simulate", "/api/v1/orders/" + payload.Data.ID + "/payment-attempts"} {
		code, data, _ = f.request(t, "POST", path, token.AccessToken, map[string]string{"result": "Success"}, "", nil)
		assertStatus(t, code, 404, data)
	}
	in.PaymentMethod = "Mock"
	if _, e = disabled.Checkout(ctx, u.ID, uuid.NewString(), in, ""); problem.Normalize(e).Status != 400 {
		t.Fatal("disabled mock checkout accepted")
	}
}

func TestPhase03FirstCartAndAdminAdjustmentRace(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	u, a := buyer(t, f, 1)
	c := cart.New(f.DB, true)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := c.Get(ctx, u.ID)
			if e != nil {
				t.Error(e)
			} else if v.Version != 1 || len(v.Items) != 0 {
				t.Error("invalid initial cart")
			}
		}()
	}
	wg.Wait()
	if count(t, f, "carts", "user_id=?", u.ID) != 1 {
		t.Fatal("first cart race duplicated rows")
	}
	p := stockProduct(t, f, 1, 1)
	in := quote(t, f, u, a, p, "COD")
	s := orders.New(f.DB, true)
	start := make(chan struct{})
	var checkoutSuccess atomic.Bool
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, e := s.Checkout(ctx, u.ID, uuid.NewString(), in, "adjustment-race")
		if e == nil {
			checkoutSuccess.Store(true)
		} else if problem.Normalize(e).Status != 409 {
			t.Error(e)
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		_, e := f.Inventory.Adjust(ctx, f.Admin.ID, uuid.NewString(), inventory.Adjustment{ProductID: p, Delta: -1, Reason: "Concurrent adjustment"})
		if e != nil && problem.Normalize(e).Status != 409 {
			t.Error(e)
		}
	}()
	close(start)
	wg.Wait()
	if quantity(t, f, p) != 0 {
		t.Fatal("stock race lost an update")
	}
	want := int64(0)
	if checkoutSuccess.Load() {
		want = 1
	}
	if count(t, f, "orders", "true") != want {
		t.Fatal("stock race produced partial order")
	}
	var delta int64
	requireOK(t, f.DB.Raw("SELECT COALESCE(SUM(delta),0) FROM inventory_transactions WHERE product_id=?", p).Scan(&delta).Error)
	if delta != 0 {
		t.Fatal("stock/history mismatch")
	}
}
func TestPhase03IntentNamespacesShippingAndRetryLimit(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	s := orders.New(f.DB, true)
	u, a := buyer(t, f, 1)
	p := stockProduct(t, f, 1, 10)
	in := quote(t, f, u, a, p, "COD")
	requireOK(t, f.DB.Exec("UPDATE store_settings SET shipping_fee_minor=40000").Error)
	if _, e := s.Checkout(ctx, u.ID, uuid.NewString(), in, ""); problem.Normalize(e).Code != "QUOTE_CHANGED" {
		t.Fatal("shipping quote was not checked")
	}
	v, e := cart.New(f.DB, true).Get(ctx, u.ID)
	requireOK(t, e)
	in.CartVersion = v.Version
	in.QuoteHash = v.QuoteHash
	first, e := s.Checkout(ctx, u.ID, uuid.NewString(), in, "")
	requireOK(t, e)
	second, e := s.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "COD"), "")
	requireOK(t, e)
	key := uuid.NewString()
	_, e = s.Transition(ctx, u.ID, first.ID, key, "Cancelled", "", "", false)
	requireOK(t, e)
	if _, e = s.Transition(ctx, u.ID, second.ID, key, "Cancelled", "", "", false); problem.Normalize(e).Code != "IDEMPOTENCY_CONFLICT" {
		t.Fatal("key replay crossed order IDs")
	}
	o, e := s.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "Mock"), "")
	requireOK(t, e)
	for i := 0; i < 5; i++ {
		attempt, e := s.BeginAttempt(ctx, u.ID, o.ID, uuid.NewString())
		requireOK(t, e)
		_, e = s.Simulate(ctx, u.ID, attempt.ID, uuid.NewString(), "Failure", "DECLINED", "")
		requireOK(t, e)
	}
	if _, e = s.BeginAttempt(ctx, u.ID, o.ID, uuid.NewString()); problem.Normalize(e).Code != "PAYMENT_RETRY_LIMIT" {
		t.Fatal("mock attempt limit missing")
	}
	o, e = s.Transition(ctx, u.ID, o.ID, uuid.NewString(), "Cancelled", "", "", false)
	requireOK(t, e)
	if o.Payment.Status != "Cancelled" {
		t.Fatal("exhausted mock order cannot cancel")
	}
	requireOK(t, f.DB.Exec("UPDATE products SET status='Inactive',version=version+1 WHERE id=?", p).Error)
	v, e = cart.New(f.DB, true).Get(ctx, u.ID)
	requireOK(t, e)
	if len(v.Items) != 0 {
		t.Fatal("checkout did not clear cart")
	}
	if _, e = cart.New(f.DB, true).Mutate(ctx, u.ID, "add", "", p, 1); problem.Normalize(e).Code != "PRODUCT_UNAVAILABLE" {
		t.Fatal("inactive product accepted")
	}
}

func TestPhase03ReplayDoesNotSurvivePrivilegeScopeChange(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	s := orders.New(f.DB, true)
	owner, address := buyer(t, f, 1)
	p := stockProduct(t, f, 1, 3)
	o, e := s.Checkout(ctx, owner.ID, uuid.NewString(), quote(t, f, owner, address, p, "COD"), "")
	requireOK(t, e)
	delegated, e := f.Users.AdminCreate(ctx, f.Admin.ID, users.Input{Email: "delegated@phase03.test", Name: "Delegated Admin", Password: testPassword, Role: "Admin", Status: "Active"})
	requireOK(t, e)
	adminToken := f.login(t, delegated.Email)
	key := uuid.NewString()
	code, data, _ := f.request(t, "PATCH", "/api/v1/admin/orders/"+o.ID+"/status", adminToken.AccessToken, map[string]string{"status": "Cancelled", "reason": "Customer cancellation"}, "", map[string]string{"Idempotency-Key": key})
	assertStatus(t, code, 200, data)
	_, e = f.Users.AdminUpdate(ctx, f.Admin.ID, delegated.ID, users.Input{Email: delegated.Email, Name: delegated.Name, Role: "Customer", Status: "Active"}, false)
	requireOK(t, e)
	customerToken := f.login(t, delegated.Email)
	code, data, _ = f.request(t, "POST", "/api/v1/orders/"+o.ID+"/cancel", customerToken.AccessToken, nil, "", map[string]string{"Idempotency-Key": key})
	assertStatus(t, code, 404, data)
	if strings.Contains(string(data), "Original address") {
		t.Fatal("cached order snapshot leaked across privilege scope")
	}
	code, data, _ = f.request(t, "PATCH", "/api/v1/admin/orders/"+o.ID+"/status", customerToken.AccessToken, map[string]string{"status": "Cancelled", "reason": "Customer cancellation"}, "", map[string]string{"Idempotency-Key": key})
	assertStatus(t, code, 403, data)
}

func TestPhase03ConcurrentPaymentSuccessAndOutboxPrivacy(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	s := orders.New(f.DB, true)
	u, a := buyer(t, f, 1)
	p := stockProduct(t, f, 1, 4)
	o, e := s.Checkout(ctx, u.ID, uuid.NewString(), quote(t, f, u, a, p, "Mock"), "payment-concurrency")
	requireOK(t, e)
	attempt, e := s.BeginAttempt(ctx, u.ID, o.ID, uuid.NewString())
	requireOK(t, e)
	var wg sync.WaitGroup
	var success atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := s.Simulate(ctx, u.ID, attempt.ID, uuid.NewString(), "Success", "", "payment-concurrency")
			if e == nil {
				success.Add(1)
			} else if problem.Normalize(e).Status != 409 {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 || count(t, f, "payment_attempts", "status='Paid'") != 1 || count(t, f, "outbox_events", "type='payment.completed'") != 1 || quantity(t, f, p) != 3 {
		t.Fatal("concurrent success duplicated payment")
	}
	var payloads []string
	requireOK(t, f.DB.Table("outbox_events").Pluck("payload::text", &payloads).Error)
	for _, payload := range payloads {
		if strings.Contains(payload, a.Address) || strings.Contains(payload, "password") || strings.Contains(payload, "token") {
			t.Fatal("private data in outbox")
		}
	}
	t.Log("SQL: 12 concurrent success requests, Paid attempts=1, payment.completed=1, stock deductions=1")
}
func TestPhase03UpgradeFromTwoAndIsolatedRollback(t *testing.T) {
	db, dsn := isolatedDB(t)
	ctx := context.Background()
	m, e := migrations.New(dsn)
	requireOK(t, e)
	defer m.Close()
	requireOK(t, m.Steps(2))
	requireOK(t, database.Seed(ctx, db, "Upgrade Store", 30000))
	admin, e := users.Insert(db, users.Input{Email: "upgrade-admin@phase03.test", Name: "Upgrade Admin", Role: "Admin", Status: "Active"}, "unused-migration-test-hash")
	requireOK(t, e)
	product, e := catalog.New(db).SaveProduct(ctx, "", catalog.ProductInput{SKU: "UPGRADE-3", Name: "Existing product", PriceMinor: 1000, OriginalPriceMinor: 1000, Status: "Active", Images: []catalog.ImageInput{}, CategoryIDs: []string{}})
	requireOK(t, e)
	_, e = inventory.New(db).Adjust(ctx, admin.ID, uuid.NewString(), inventory.Adjustment{ProductID: product.ID, Delta: 4, Reason: "Existing stock"})
	requireOK(t, e)
	requireOK(t, migrations.Up(m))
	requireOK(t, database.CheckReady(ctx, db))
	var before int64
	requireOK(t, db.Table("inventory").Where("product_id=?", product.ID).Pluck("quantity", &before).Error)
	if before != 4 {
		t.Fatal("upgrade changed stock")
	}
	address, e := users.New(db).SaveAddress(ctx, admin.ID, "", users.AddressInput{Recipient: "Upgrade Buyer", Phone: "0901234567", Address: "Upgrade address", IsDefault: true})
	requireOK(t, e)
	v, e := cart.New(db, true).Mutate(ctx, admin.ID, "add", "", product.ID, 1)
	requireOK(t, e)
	_, e = orders.New(db, true).Checkout(ctx, admin.ID, uuid.NewString(), orders.CheckoutInput{AddressID: address.ID, CartVersion: v.Version, QuoteHash: v.QuoteHash, PaymentMethod: "COD"}, "")
	requireOK(t, e)
	requireOK(t, m.Steps(-1))
	if database.CheckReady(ctx, db) == nil {
		t.Fatal("schema2 reported ready for schema3 app")
	}
	var existing int64
	requireOK(t, db.Table("products").Where("id=?", product.ID).Count(&existing).Error)
	if existing != 1 {
		t.Fatal("test rollback removed old data")
	}
	requireOK(t, migrations.Up(m))
	requireOK(t, database.CheckReady(ctx, db))
}
