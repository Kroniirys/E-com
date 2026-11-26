//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"ecommerce.local/base/api"
	"ecommerce.local/base/internal/modules/catalog"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/problem"

	"github.com/gin-gonic/gin"
)

func TestImplementedContractMatchesRouter(t *testing.T) {
	f := fixture(t)
	var spec struct {
		Paths map[string]map[string]struct {
			Implemented bool `json:"x-implemented"`
		}
	}
	if err := json.Unmarshal(api.OpenAPI, &spec); err != nil {
		t.Fatal(err)
	}
	engine, ok := f.Server.Config.Handler.(*gin.Engine)
	if !ok {
		t.Fatal("unexpected router type")
	}
	routeSet := map[string]bool{}
	parameter := regexp.MustCompile(`:([a-zA-Z_]+)`)
	for _, route := range engine.Routes() {
		path := parameter.ReplaceAllString(route.Path, `{$1}`)
		key := strings.ToLower(route.Method) + " " + path
		routeSet[key] = true
		if !spec.Paths[path][strings.ToLower(route.Method)].Implemented {
			t.Fatalf("route has no implemented contract: %s", key)
		}
	}
	count := 0
	for path, methods := range spec.Paths {
		for method, operation := range methods {
			if operation.Implemented {
				count++
				if !routeSet[method+" "+path] {
					t.Fatalf("implemented contract lacks route: %s %s", method, path)
				}
			}
		}
	}
	if count != 64 {
		t.Fatalf("expected64 implemented routes, got%d", count)
	}
}

func TestPhase02AdminCRUDValidationAndNormalization(t *testing.T) {
	f := fixture(t)
	ctx := context.Background()
	admin := f.login(t, f.Admin.Email)
	body := map[string]any{"email": " Managed@Phase02.Test ", "name": "Managed User", "role": "Customer", "status": "Disabled", "password": testPassword}
	code, data, _ := f.request(t, http.MethodPost, "/api/v1/admin/users", admin.AccessToken, body, "", nil)
	assertStatus(t, code, 201, data)
	var created struct{ Data users.User }
	if err := json.Unmarshal(data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.Email != "managed@phase02.test" || created.Data.Status != "Disabled" {
		t.Fatal("user normalization/state mismatch")
	}
	if strings.Contains(string(data), "password_hash") || strings.Contains(string(data), "session_version") || strings.Contains(string(data), testPassword) {
		t.Fatal("private user data leaked")
	}
	code, data, _ = f.request(t, http.MethodPost, "/api/v1/admin/users", admin.AccessToken, body, "", nil)
	assertStatus(t, code, 409, data)
	delete(body, "password")
	body["status"] = "Active"
	code, data, _ = f.request(t, http.MethodPut, "/api/v1/admin/users/"+created.Data.ID, admin.AccessToken, body, "", nil)
	assertStatus(t, code, 200, data)
	if _, err := f.Auth.Login(ctx, "managed@phase02.test", testPassword); err != nil {
		t.Fatal(err)
	}
	code, data, _ = f.request(t, http.MethodDelete, "/api/v1/admin/users/"+created.Data.ID, admin.AccessToken, nil, "", nil)
	assertStatus(t, code, 204, data)
	if _, err := f.Auth.Login(ctx, "managed@phase02.test", testPassword); problem.Normalize(err).Status != 401 {
		t.Fatal("deleted user can log in")
	}
	for _, password := range []string{strings.Repeat("á", 6), strings.Repeat("á", 37)} {
		code, data, _ = f.request(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{"email": "unicode@phase02.test", "name": "Unicode", "password": password}, "", nil)
		assertStatus(t, code, 400, data)
	}
	code, data, _ = f.request(t, http.MethodPost, "/api/v1/auth/register", "", map[string]any{"email": "Unicode@Phase02.Test", "name": "Unicode", "password": strings.Repeat("á", 12)}, "", nil)
	assertStatus(t, code, 201, data)
	product := f.product(t, "Unique product", "Active")
	input := catalog.ProductInput{SKU: strings.ToLower(product.SKU), Name: "Duplicate SKU", Status: "Active", PriceMinor: 1000, OriginalPriceMinor: 1000, Images: []catalog.ImageInput{}, CategoryIDs: []string{}}
	if _, err := f.Catalog.SaveProduct(ctx, "", input); problem.Normalize(err).Status != 409 {
		t.Fatal("SKU uniqueness is case-sensitive")
	}
	code, data, _ = f.request(t, http.MethodPost, "/api/v1/admin/products", admin.AccessToken, map[string]any{"sku": "forbidden-stock", "name": "Forbidden", "description": "", "price_minor": 0, "original_price_minor": 0, "status": "Active", "images": []any{}, "category_ids": []string{}, "stock": 20}, "", nil)
	assertStatus(t, code, 400, data)
	key := "family-retry-after"
	for i := 0; i < 5; i++ {
		_, err := f.Auth.Rate(ctx, key, "same-family", 5)
		if err != nil {
			t.Fatal(err)
		}
	}
	seconds, err := f.Auth.Rate(ctx, key, "same-family", 5)
	if err == nil || seconds < 1 || problem.Normalize(err).RetryAfter != seconds {
		t.Fatal("family rate limit lost Retry-After metadata")
	}
	for i := 0; i < 6; i++ {
		code, data, _ = f.request(t, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"email": "unknown@phase02.test", "password": testPassword}, "", map[string]string{"X-Forwarded-For": fmt.Sprintf("192.0.2.%d", i+1)})
		want := 401
		if i == 5 {
			want = 429
		}
		assertStatus(t, code, want, data)
	}
}
