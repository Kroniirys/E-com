package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/internal/database"
)

func TestLivenessSurvivesUnavailableDatabase(t *testing.T) {
	cfg := config.Config{RequestTimeout: time.Second, AllowedOrigins: []string{"http://localhost:5173"}}
	deps := dependencies{ready: func(context.Context) error { return errors.New("secret database failure") }, store: func(context.Context) (database.StoreSettings, error) { panic("must not be called") }}
	r := router(cfg, deps, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	for _, path := range []string{"/health/live", "/health/ready", "/api/v1/foundation", "/api/v1/products"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		expected := 503
		if path == "/health/live" {
			expected = 200
		}
		if path == "/api/v1/products" {
			expected = 404
		}
		if w.Code != expected {
			t.Fatalf("%s: %d", path, w.Code)
		}
		if strings.Contains(w.Body.String(), "secret") {
			t.Fatal("dependency error leaked")
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Fatal("missing request ID")
		}
	}
}

func TestMiddlewareRejectsOriginAndRecoversPanic(t *testing.T) {
	cfg := config.Config{RequestTimeout: time.Second, AllowedOrigins: []string{"http://localhost:5173"}}
	deps := dependencies{ready: func(context.Context) error { panic("sensitive") }}
	r := router(cfg, deps, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health/live", nil)
	req.Header.Set("Origin", "http://evil.test")
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/health/ready", nil))
	if w.Code != 500 || strings.Contains(w.Body.String(), "sensitive") {
		t.Fatal("unsafe panic recovery")
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest("OPTIONS", "/health/live", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	r.ServeHTTP(w, req)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatal("invalid CORS preflight")
	}
}

func TestOpenAPIIsValidJSONAndMatchesPhaseOneRoutes(t *testing.T) {
	r := router(config.Config{RequestTimeout: time.Second}, dependencies{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/openapi.json", nil))
	var spec struct {
		Paths map[string]map[string]struct {
			Implemented bool `json:"x-implemented"`
		}
	}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	implemented := 0
	for _, methods := range spec.Paths {
		for _, op := range methods {
			if op.Implemented {
				implemented++
			}
		}
	}
	if w.Code != 200 || implemented != 4 {
		t.Fatalf("phase-one contract mismatch: %d", implemented)
	}
}
