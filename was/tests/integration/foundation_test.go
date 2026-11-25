//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/internal/database"
	"ecommerce.local/base/internal/httpserver"
	"ecommerce.local/base/migrations"
)

func isolatedDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("TEST_DATABASE_URL is required: use make test-integration or an isolated *_test database")
	}
	if !migrations.AllowTestReset("test", raw, "delete-test-schema") {
		t.Fatal("TEST_DATABASE_URL must point to a safe *_test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	base, err := database.Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := base.DB()
	t.Cleanup(func() { _ = pool.Close() })
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	schema := "ecom_it_" + hex.EncodeToString(b[:])
	if err := base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal("could not create isolated schema")
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "ecom_it_") {
			if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
				t.Error("test schema cleanup failed")
			}
		}
	})
	u, _ := url.Parse(raw)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := database.Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	dbPool, _ := db.DB()
	t.Cleanup(func() { _ = dbPool.Close() })
	return db, u.String()
}

func TestMigrationSeedAndRealHTTP(t *testing.T) {
	db, dsn := isolatedDB(t)
	ctx := context.Background()
	if database.CheckReady(ctx, db) == nil {
		t.Fatal("empty schema ready")
	}
	m, err := migrations.New(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = m.Close() })
	if err := migrations.Up(m); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(m); err != nil {
		t.Fatal("repeated migration failed")
	}
	if database.CheckReady(ctx, db) == nil {
		t.Fatal("unseeded schema ready")
	}
	for i := 0; i < 2; i++ {
		if err := database.Seed(ctx, db, "Cửa hàng tích hợp", 30000); err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Seed(ctx, db, "Không ghi đè", 90000); err != nil {
		t.Fatal(err)
	}
	store, err := database.Store(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if store.Name != "Cửa hàng tích hợp" || store.ShippingFeeMinor != 30000 {
		t.Fatal("seed overwrote store")
	}
	var count int64
	db.Model(&database.StoreSettings{}).Count(&count)
	if count != 1 {
		t.Fatal("duplicate settings")
	}
	cfg := config.Config{Environment: "test", Version: "integration", RequestTimeout: 3 * time.Second}
	server := httptest.NewServer(httpserver.New(cfg, db, slog.New(slog.NewJSONHandler(io.Discard, nil))).Handler)
	defer server.Close()
	for _, path := range []string{"/health/live", "/health/ready", "/api/v1/foundation"} {
		res, err := server.Client().Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("%s: %d %s", path, res.StatusCode, data)
		}
		if path == "/api/v1/foundation" {
			var payload struct {
				Data struct{ Store database.StoreSettings }
			}
			if err := json.Unmarshal(data, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Data.Store.Name != store.Name {
				t.Fatal("HTTP did not read database data")
			}
		}
	}
	if err := db.Exec("UPDATE schema_migrations SET dirty = true").Error; err != nil {
		t.Fatal(err)
	}
	if database.CheckReady(ctx, db) == nil {
		t.Fatal("dirty migration reported ready")
	}
	if err := db.Exec("UPDATE schema_migrations SET dirty = false").Error; err != nil {
		t.Fatal(err)
	}
	if err := m.Steps(-1); err != nil {
		t.Fatal("test rollback failed", err)
	}
	if database.CheckReady(ctx, db) == nil {
		t.Fatal("rolled back schema ready")
	}
	if err := migrations.Up(m); err != nil {
		t.Fatal("migration after rollback failed", err)
	}
}

func TestConstraintsAndSeedValidation(t *testing.T) {
	db, dsn := isolatedDB(t)
	m, err := migrations.New(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := migrations.Up(m); err != nil {
		t.Fatal(err)
	}
	if database.Seed(context.Background(), db, " ", 0) == nil {
		t.Fatal("blank store accepted")
	}
	if database.Seed(context.Background(), db, "Store", -1) == nil {
		t.Fatal("negative shipping accepted")
	}
	if err := database.Seed(context.Background(), db, "Store", 0); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE store_settings SET shipping_fee_minor = -1").Error; err == nil {
		t.Fatal("DB accepted negative shipping")
	}
	if err := db.Exec("UPDATE store_settings SET currency = 'USD'").Error; err == nil {
		t.Fatal("DB accepted unsupported currency")
	}
}
