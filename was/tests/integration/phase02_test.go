//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce.local/base/internal/bootstrap"
	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/internal/database"
	"ecommerce.local/base/internal/httpapi"
	"ecommerce.local/base/internal/httpserver"
	"ecommerce.local/base/internal/modules/auth"
	"ecommerce.local/base/internal/modules/cart"
	"ecommerce.local/base/internal/modules/catalog"
	"ecommerce.local/base/internal/modules/inventory"
	"ecommerce.local/base/internal/modules/orders"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/limit"
	"ecommerce.local/base/internal/platform/problem"
	"ecommerce.local/base/migrations"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const testPassword = "Phase02-test-password-123"
const testSecret = "phase02-test-only-secret-not-for-deployment"
const testOrigin = "http://127.0.0.1:5173"

var hashOnce sync.Once
var sharedHash []byte

type phaseFixture struct {
	DB              *gorm.DB
	DSN             string
	Auth            *auth.Service
	Users           *users.Service
	Catalog         *catalog.Service
	Inventory       *inventory.Service
	Limiter         *limit.Redis
	Server          *httptest.Server
	Admin, Customer users.User
}

func fixture(t *testing.T) *phaseFixture {
	t.Helper()
	db, dsn := isolatedDB(t)
	m, err := migrations.New(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = m.Close() })
	if err := migrations.Up(m); err != nil {
		t.Fatal(err)
	}
	if err := database.Seed(context.Background(), db, "Integration Store", 30000); err != nil {
		t.Fatal(err)
	}
	raw := os.Getenv("TEST_REDIS_URL")
	if raw == "" {
		t.Fatal("TEST_REDIS_URL is required for real Redis integration")
	}
	limiter, err := limit.New(raw, "ecom:it:"+uuid.NewString()+":")
	if err != nil {
		t.Fatal(err)
	}
	if err := limiter.Client.Ping(context.Background()).Err(); err != nil {
		t.Fatal("Redis test service unavailable")
	}
	t.Cleanup(func() {
		var cursor uint64
		for {
			keys, next, e := limiter.Client.Scan(context.Background(), cursor, limiter.Prefix+"*", 100).Result()
			if e != nil {
				break
			}
			if len(keys) > 0 {
				_ = limiter.Client.Del(context.Background(), keys...).Err()
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
		_ = limiter.Client.Close()
	})
	hashOnce.Do(func() { sharedHash, _ = bcrypt.GenerateFromPassword([]byte(testPassword), 12) })
	admin, err := users.Insert(db, users.Input{Email: "admin@phase02.test", Name: "Admin", Role: "Admin", Status: "Active"}, string(sharedHash))
	if err != nil {
		t.Fatal(err)
	}
	customer, err := users.Insert(db, users.Input{Email: "customer@phase02.test", Name: "Customer", Role: "Customer", Status: "Active"}, string(sharedHash))
	if err != nil {
		t.Fatal(err)
	}
	authService, err := auth.New(db, testSecret, limiter)
	if err != nil {
		t.Fatal(err)
	}
	f := &phaseFixture{DB: db, DSN: dsn, Auth: authService, Users: users.New(db), Catalog: catalog.New(db), Inventory: inventory.New(db), Limiter: limiter, Admin: admin, Customer: customer}
	endpoints := &httpapi.API{Cart: cart.New(db, true), Orders: orders.New(db, true), Auth: f.Auth, Users: f.Users, Catalog: f.Catalog, Inventory: f.Inventory, Origins: []string{testOrigin}}
	cfg := config.Config{Environment: "test", Version: "phase02-integration", RequestTimeout: 5 * time.Second, AllowedOrigins: []string{testOrigin}}
	f.Server = httptest.NewServer(httpserver.New(cfg, db, slog.New(slog.NewJSONHandler(io.Discard, nil)), endpoints.Register).Handler)
	t.Cleanup(f.Server.Close)
	return f
}
func (f *phaseFixture) request(t *testing.T, method, path, token string, body any, cookie string, extra map[string]string) (int, []byte, http.Header) {
	t.Helper()
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		input = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, f.Server.URL+path, input)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", testOrigin)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: cookie})
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	response, err := f.Server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	return response.StatusCode, data, response.Header
}
func (f *phaseFixture) login(t *testing.T, email string) auth.Tokens {
	t.Helper()
	tokens, err := f.Auth.Login(context.Background(), email, testPassword)
	if err != nil {
		t.Fatal(err)
	}
	return tokens
}
func (f *phaseFixture) product(t *testing.T, name, status string) catalog.Product {
	t.Helper()
	p, err := f.Catalog.SaveProduct(context.Background(), "", catalog.ProductInput{SKU: uuid.NewString(), Name: name, Status: status, PriceMinor: 1000, OriginalPriceMinor: 1200, Images: []catalog.ImageInput{}, CategoryIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func assertStatus(t *testing.T, got, want int, data []byte) {
	t.Helper()
	if got != want {
		t.Fatalf("status %d want %d: %s", got, want, data)
	}
}

func TestPhase02AuthRotationReuseAndRevocation(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	login := f.login(t, f.Customer.Email)
	if _, err := f.Auth.Authenticate(ctx, login.AccessToken); err != nil {
		t.Fatal(err)
	}
	next, err := f.Auth.Refresh(ctx, login.Refresh)
	if err != nil {
		t.Fatal(err)
	}
	if next.Refresh == login.Refresh {
		t.Fatal("refresh did not rotate")
	}
	var plaintext int64
	f.DB.Table("refresh_tokens").Where("token_hash IN ?", []string{login.Refresh, next.Refresh}).Count(&plaintext)
	if plaintext != 0 {
		t.Fatal("refresh plaintext persisted")
	}
	if _, err := f.Auth.Refresh(ctx, login.Refresh); problem.Normalize(err).Status != 401 {
		t.Fatal("refresh replay was accepted")
	}
	if _, err := f.Auth.Authenticate(ctx, next.AccessToken); err == nil {
		t.Fatal("reuse did not revoke family access")
	}
	if _, err := f.Auth.Refresh(ctx, next.Refresh); err == nil {
		t.Fatal("revoked family refreshed")
	}
	login = f.login(t, f.Customer.Email)
	p, err := f.Auth.Authenticate(ctx, login.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Auth.ChangePassword(ctx, p, testPassword, "Changed-password-12345"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Auth.Authenticate(ctx, login.AccessToken); err == nil {
		t.Fatal("password change did not revoke access")
	}
	if _, err := f.Auth.Login(ctx, f.Customer.Email, testPassword); err == nil {
		t.Fatal("old password accepted")
	}
	if _, err := f.Auth.Login(ctx, f.Customer.Email, "Changed-password-12345"); err != nil {
		t.Fatal(err)
	}
}

func TestPhase02JWTValidationAndSessionState(t *testing.T) {
	f := fixture(t)
	login := f.login(t, f.Customer.Email)
	ctx := context.Background()
	var claims auth.Claims
	if _, _, err := jwt.NewParser().ParseUnverified(login.AccessToken, &claims); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		edit   func(*auth.Claims)
		method jwt.SigningMethod
		key    any
	}{
		{"expired", func(c *auth.Claims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour)) }, jwt.SigningMethodHS256, []byte(testSecret)},
		{"missing expiry", func(c *auth.Claims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256, []byte(testSecret)},
		{"wrong issuer", func(c *auth.Claims) { c.Issuer = "other" }, jwt.SigningMethodHS256, []byte(testSecret)},
		{"wrong audience", func(c *auth.Claims) { c.Audience = jwt.ClaimStrings{"other"} }, jwt.SigningMethodHS256, []byte(testSecret)},
		{"wrong algorithm", func(*auth.Claims) {}, jwt.SigningMethodHS384, []byte(testSecret)},
		{"none", func(*auth.Claims) {}, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType},
	}
	for _, tc := range cases {
		c := claims
		tc.edit(&c)
		raw, err := jwt.NewWithClaims(tc.method, c).SignedString(tc.key)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Auth.Authenticate(ctx, raw); err == nil {
			t.Fatalf("accepted %s token", tc.name)
		}
	}
	if err := f.Auth.Logout(ctx, login.Refresh); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Auth.Authenticate(ctx, login.AccessToken); err == nil {
		t.Fatal("logout access remains valid")
	}
	login = f.login(t, f.Customer.Email)
	_, err := f.Users.AdminUpdate(ctx, f.Admin.ID, f.Customer.ID, users.Input{Email: f.Customer.Email, Name: "Disabled", Role: "Customer", Status: "Disabled"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Auth.Authenticate(ctx, login.AccessToken); err == nil {
		t.Fatal("disabled user access remains valid")
	}
	if _, err := f.Auth.Refresh(ctx, login.Refresh); err == nil {
		t.Fatal("disabled user refreshed")
	}
}

func TestPhase02ConcurrentRefreshAndLastAdmin(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	login := f.login(t, f.Customer.Email)
	var results [2]auth.Tokens
	var errs [2]error
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = f.Auth.Refresh(ctx, login.Refresh) }(i)
	}
	wg.Wait()
	success := 0
	for i, err := range errs {
		if err == nil {
			success++
			if _, err := f.Auth.Authenticate(ctx, results[i].AccessToken); err == nil {
				t.Fatal("concurrent reuse left family active")
			}
		}
	}
	if success != 1 {
		t.Fatalf("refresh successes: %d", success)
	}
	other, err := f.Users.AdminCreate(ctx, f.Admin.ID, users.Input{Email: "other-admin@phase02.test", Name: "Admin 2", Password: testPassword, Role: "Admin", Status: "Active"})
	if err != nil {
		t.Fatal(err)
	}
	admins := []users.User{f.Admin, other}
	success = 0
	var count atomic.Int32
	for _, admin := range admins {
		wg.Add(1)
		go func(admin users.User) {
			defer wg.Done()
			_, err := f.Users.AdminUpdate(ctx, admin.ID, admin.ID, users.Input{Email: admin.Email, Name: admin.Name, Role: "Customer", Status: "Active"}, false)
			if err == nil {
				count.Add(1)
			}
		}(admin)
	}
	wg.Wait()
	var remaining int64
	f.DB.Table("users").Where("role_id=? AND status='Active' AND deleted_at IS NULL", users.AdminRoleID).Count(&remaining)
	if remaining != 1 || count.Load() != 1 {
		t.Fatalf("last-admin invariant: %d active, %d changes", remaining, count.Load())
	}
}

func TestPhase02HTTPPermissionsCookiesAndOwnership(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	customer := f.login(t, f.Customer.Email)
	admin := f.login(t, f.Admin.Email)
	code, data, _ := f.request(t, "GET", "/api/v1/admin/users", customer.AccessToken, nil, "", nil)
	assertStatus(t, code, 403, data)
	code, data, _ = f.request(t, "POST", "/api/v1/auth/register", "", map[string]any{"email": "new@phase02.test", "name": "New", "password": testPassword, "role": "Admin"}, "", nil)
	assertStatus(t, code, 400, data)
	code, data, headers := f.request(t, "POST", "/api/v1/auth/login", "", map[string]any{"email": f.Customer.Email, "password": testPassword}, "", nil)
	assertStatus(t, code, 200, data)
	cookie := headers.Get("Set-Cookie")
	if !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "SameSite=Lax") || !strings.Contains(cookie, "Path=/api/v1/auth") {
		t.Fatal("missing refresh cookie protection")
	}
	if strings.Contains(string(data), "refresh_token") {
		t.Fatal("refresh cookie leaked in JSON")
	}
	code, data, _ = f.request(t, "POST", "/api/v1/auth/refresh", "", nil, customer.Refresh, map[string]string{"Origin": "http://evil.test"})
	assertStatus(t, code, 403, data)
	code, data, _ = f.request(t, "PATCH", "/api/v1/me", customer.AccessToken, map[string]any{"name": "New", "role": "Admin"}, "", nil)
	assertStatus(t, code, 400, data)
	address, err := f.Users.SaveAddress(ctx, f.Admin.ID, "", users.AddressInput{Recipient: "Admin", Phone: "0901234567", Address: "Private address", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"recipient": "Other", "phone": "0901234567", "address": "Wrong owner", "is_default": true}
	code, data, _ = f.request(t, "PUT", "/api/v1/me/addresses/"+address.ID, customer.AccessToken, input, "", nil)
	assertStatus(t, code, 404, data)
	code, data, _ = f.request(t, "DELETE", "/api/v1/me/addresses/"+address.ID, customer.AccessToken, nil, "", nil)
	assertStatus(t, code, 404, data)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.Users.SaveAddress(ctx, f.Customer.ID, "", users.AddressInput{Recipient: "Customer", Phone: "0901234567", Address: "Home", IsDefault: true})
		}()
	}
	wg.Wait()
	var defaults int64
	f.DB.Table("user_addresses").Where("user_id=? AND is_default", f.Customer.ID).Count(&defaults)
	if defaults != 1 {
		t.Fatal("concurrent default addresses")
	}
	code, data, _ = f.request(t, "DELETE", "/api/v1/admin/users/"+f.Customer.ID, admin.AccessToken, nil, "", nil)
	assertStatus(t, code, 204, data)
	code, data, headers = f.request(t, "POST", "/api/v1/auth/logout", "", nil, customer.Refresh, nil)
	assertStatus(t, code, 204, data)
	if !strings.Contains(headers.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatal("deleted-user logout did not clear cookie")
	}
	code, data, _ = f.request(t, "GET", "/api/v1/auth/me", customer.AccessToken, nil, "", nil)
	assertStatus(t, code, 401, data)
}

func TestPhase02CatalogAtomicityVisibilityAndSearch(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	public := f.product(t, `Literal %_\ item`, "Active")
	_ = f.product(t, "Hidden", "Draft")
	_ = f.product(t, "Inactive", "Inactive")
	for _, query := range []string{"%", "_", `\`} {
		list, err := f.Catalog.Products(ctx, users.Page{Number: 1, Size: 20, Query: query}, false)
		if err != nil {
			t.Fatal(err)
		}
		if list.Total != 1 || len(list.Items) != 1 || list.Items[0].ID != public.ID {
			t.Fatalf("literal search %q wrong: %+v", query, list)
		}
	}
	list, err := f.Catalog.Products(ctx, users.Page{Number: 1, Size: 20}, false)
	if err != nil || list.Total != 1 {
		t.Fatal("hidden catalog leaked", err)
	}
	if !list.Items[0].OutOfStock {
		t.Fatal("zero stock not derived")
	}
	if _, err := f.Catalog.Products(ctx, users.Page{Number: 1, Size: 20, Sort: "price;DROP TABLE users"}, false); err == nil {
		t.Fatal("raw sort accepted")
	}
	category, err := f.Catalog.SaveCategory(ctx, "", catalog.CategoryInput{Name: "Lighting", Slug: "lighting"})
	if err != nil {
		t.Fatal(err)
	}
	in := catalog.ProductInput{SKU: public.SKU, Name: public.Name, Status: "Active", PriceMinor: 1000, OriginalPriceMinor: 1200, Images: []catalog.ImageInput{{URL: "https://example.test/valid.png", Alt: "original"}}, CategoryIDs: []string{category.ID}}
	if _, err := f.Catalog.SaveProduct(ctx, public.ID, in); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.Exec("ALTER TABLE product_images ADD CONSTRAINT phase02_reject_alt CHECK (alt <> 'reject-me')").Error; err != nil {
		t.Fatal(err)
	}
	in.Name = "Must rollback"
	in.Images[0].Alt = "reject-me"
	if _, err := f.Catalog.SaveProduct(ctx, public.ID, in); err == nil {
		t.Fatal("forced transaction failure was accepted")
	}
	restored, err := f.Catalog.Product(ctx, public.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Name != public.Name || len(restored.Images) != 1 || restored.Images[0].Alt != "original" {
		t.Fatal("product/image rollback not atomic")
	}
	if err := f.Catalog.DeleteCategory(ctx, category.ID); err != nil {
		t.Fatal(err)
	}
	restored, err = f.Catalog.Product(ctx, public.ID, true)
	if err != nil || len(restored.CategoryIDs) != 0 {
		t.Fatal("category left orphan assignment")
	}
	if err := f.Catalog.DeleteProduct(ctx, public.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Catalog.Product(ctx, public.ID, false); err == nil {
		t.Fatal("archived product public")
	}
	admin := f.login(t, f.Admin.Email)
	code, data, _ := f.request(t, "GET", "/api/v1/admin/products?page=0", admin.AccessToken, nil, "", nil)
	assertStatus(t, code, 400, data)
}

func TestPhase02InventoryConcurrencyIdempotencyAndRollback(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	product := f.product(t, "Stock", "Active")
	stock, err := f.Inventory.Adjust(ctx, f.Admin.ID, "initial-stock-key-123", inventory.Adjustment{ProductID: product.ID, Delta: 10, Reason: "Initial"})
	if err != nil || stock.Quantity != 10 {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var successes atomic.Int32
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := f.Inventory.Adjust(ctx, f.Admin.ID, fmt.Sprintf("decrement-key-%03d", i), inventory.Adjustment{ProductID: product.ID, Delta: -1, Reason: "Sale test"})
			if err == nil {
				successes.Add(1)
			} else if problem.Normalize(err).Status != 409 {
				t.Errorf("unexpected adjustment error: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if successes.Load() != 10 {
		t.Fatalf("expected 10 successes got %d", successes.Load())
	}
	f.DB.Where("product_id=?", product.ID).Take(&stock)
	if stock.Quantity != 0 {
		t.Fatal("stock invariant failed")
	}
	var count int64
	f.DB.Model(&inventory.Transaction{}).Where("product_id=?", product.ID).Count(&count)
	if count != 11 {
		t.Fatalf("history count %d", count)
	}
	key := "duplicate-adjust-key-123"
	input := inventory.Adjustment{ProductID: product.ID, Delta: 1, Reason: "Restock"}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := f.Inventory.Adjust(ctx, f.Admin.ID, key, input)
			if err != nil || result.Quantity != 1 {
				t.Errorf("idempotent result: %+v %v", result, err)
			}
		}()
	}
	wg.Wait()
	if _, err := f.Inventory.Adjust(ctx, f.Admin.ID, key, inventory.Adjustment{ProductID: product.ID, Delta: 2, Reason: "Restock"}); problem.Normalize(err).Status != 409 {
		t.Fatal("different payload reuse accepted")
	}
	f.DB.Where("product_id=?", product.ID).Take(&stock)
	if stock.Quantity != 1 {
		t.Fatal("duplicate request changed stock more than once")
	}
	before := stock.Quantity
	if _, err := f.Inventory.Adjust(ctx, f.Admin.ID, "negative-stock-key-123", inventory.Adjustment{ProductID: product.ID, Delta: -2, Reason: "Invalid"}); err == nil {
		t.Fatal("negative stock accepted")
	}
	f.DB.Where("product_id=?", product.ID).Take(&stock)
	if stock.Quantity != before {
		t.Fatal("failed adjustment changed stock")
	}
	other, err := f.Users.AdminCreate(ctx, f.Admin.ID, users.Input{Email: "stock-admin@phase02.test", Name: "Stock Admin", Role: "Admin", Status: "Active", Password: testPassword})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Inventory.Adjust(ctx, other.ID, key, input); err != nil {
		t.Fatal("different actor same key collided", err)
	}
}

func TestPhase02RedisLimitAndUnavailableAuth(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	var allowed atomic.Int32
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _, err := f.Limiter.Allow(ctx, "atomic-test", "same-client", 5)
			if err != nil {
				t.Error(err)
			}
			if ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if allowed.Load() != 5 {
		t.Fatalf("non-atomic limit: %d", allowed.Load())
	}
	for i := 0; i < 6; i++ {
		code, data, headers := f.request(t, "POST", "/api/v1/auth/login", "", map[string]any{"email": "unknown@phase02.test", "password": testPassword}, "", nil)
		want := 401
		if i == 5 {
			want = 429
			if headers.Get("Retry-After") == "" {
				t.Fatal("missing Retry-After")
			}
		}
		assertStatus(t, code, want, data)
	}
	down, err := limit.New("redis://127.0.0.1:1/0", "ecom:down:")
	if err != nil {
		t.Fatal(err)
	}
	defer down.Client.Close()
	original := f.Auth.Limiter
	f.Auth.Limiter = down
	defer func() { f.Auth.Limiter = original }()
	code, data, _ := f.request(t, "POST", "/api/v1/auth/login", "", map[string]any{"email": f.Admin.Email, "password": testPassword}, "", nil)
	assertStatus(t, code, 503, data)
	code, data, _ = f.request(t, "GET", "/api/v1/products", "", nil, "", nil)
	assertStatus(t, code, 200, data)
}

func TestPhase02UpgradeAndRepeatSeed(t *testing.T) {
	db, dsn := isolatedDB(t)
	m, err := migrations.New(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Steps(1); err != nil {
		t.Fatal(err)
	}
	if err := database.Seed(context.Background(), db, "Existing shop", 20000); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(m); err != nil {
		t.Fatal(err)
	}
	if err := database.CheckReady(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	options := bootstrap.DemoOptions{AdminEmail: "demo-admin@phase02.test", AdminPassword: testPassword, CustomerEmail: "demo-customer@phase02.test", CustomerPassword: testPassword}
	for i := 0; i < 2; i++ {
		if err := bootstrap.Demo(context.Background(), db, options); err != nil {
			t.Fatal(err)
		}
	}
	var userCount, productCount int64
	db.Table("users").Count(&userCount)
	db.Table("products").Count(&productCount)
	if userCount != 2 || productCount != 5 {
		t.Fatalf("seed duplicates users=%d products=%d", userCount, productCount)
	}
	store, err := database.Store(context.Background(), db)
	if err != nil || store.Name != "Existing shop" || store.ShippingFeeMinor != 20000 {
		t.Fatal("upgrade/seed overwrote store")
	}
	var user users.User
	if err := users.Query(db).Where("u.email=?", options.AdminEmail).Take(&user).Error; err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(testPassword)) != nil {
		t.Fatal("seed password incorrect")
	}
	cost, _ := bcrypt.Cost([]byte(user.PasswordHash))
	if cost != 12 {
		t.Fatal("wrong bcrypt cost")
	}
	var product catalog.Product
	if err := db.Table("products").Where("sku='BASE-LAMP'").Take(&product).Error; err != nil {
		t.Fatal(err)
	}
	inv := inventory.New(db)
	if _, err := inv.Adjust(context.Background(), user.ID, "seed-preserve-stock-123", inventory.Adjustment{ProductID: product.ID, Delta: 3, Reason: "Existing stock"}); err != nil {
		t.Fatal(err)
	}
	options.AdminPassword = "Must-not-overwrite-123"
	if err := bootstrap.Demo(context.Background(), db, options); err != nil {
		t.Fatal(err)
	}
	var stock inventory.Stock
	db.Where("product_id=?", product.ID).Take(&stock)
	if stock.Quantity != 15 {
		t.Fatal("repeat seed overwrote inventory")
	}
	var fresh users.User
	if err := users.Query(db).Where("u.id=?", user.ID).Take(&fresh).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.PasswordHash != user.PasswordHash {
		t.Fatal("repeat seed overwrote password")
	}
}
