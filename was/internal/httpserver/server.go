package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"ecommerce.local/base/api"
	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/internal/database"
	"ecommerce.local/base/internal/platform/metrics"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type dependencies struct {
	ready func(context.Context) error
	store func(context.Context) (database.StoreSettings, error)
}

func New(c config.Config, db *gorm.DB, log *slog.Logger, registrations ...func(*gin.Engine)) *http.Server {
	deps := dependencies{ready: func(ctx context.Context) error { return database.CheckReady(ctx, db) }, store: func(ctx context.Context) (database.StoreSettings, error) { return database.Store(ctx, db) }}
	return &http.Server{Addr: c.HTTPAddr, Handler: router(c, deps, log, registrations...), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: c.RequestTimeout + 5*time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
}

func failure(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": message, "request_id": c.GetString("request_id")}})
}

func router(cfg config.Config, deps dependencies, log *slog.Logger, registrations ...func(*gin.Engine)) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	_ = r.SetTrustedProxies(nil)
	r.Use(func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if !validRequestID.MatchString(id) {
			var b [16]byte
			if _, err := rand.Read(b[:]); err != nil {
				failure(c, 500, "INTERNAL_ERROR", "Request could not be initialized")
				return
			}
			id = hex.EncodeToString(b[:])
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Cache-Control", "no-store")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.RequestTimeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		start := time.Now()
		defer func() {
			if recover() != nil {
				log.Error("request panic recovered", "request_id", id)
				failure(c, 500, "INTERNAL_ERROR", "An internal error occurred")
			}
			path := c.FullPath()
			if path == "" {
				path = "unmatched"
			}
			metrics.HTTP(path, c.Writer.Status(), time.Since(start))
			log.Info("http request", "request_id", id, "method", c.Request.Method, "route", path, "status", c.Writer.Status(), "order_id", c.GetString("order_id"), "attempt_id", c.GetString("attempt_id"), "error_code", c.GetString("error_code"), "duration_ms", time.Since(start).Milliseconds())
		}()
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Vary", "Origin")
			allowed := false
			for _, v := range cfg.AllowedOrigins {
				if origin == v {
					allowed = true
					break
				}
			}
			if !allowed {
				failure(c, 403, "ORIGIN_FORBIDDEN", "Origin is not allowed")
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		}
		if c.Request.Method == http.MethodOptions {
			if origin == "" {
				failure(c, 400, "ORIGIN_REQUIRED", "Preflight requires Origin")
				return
			}
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, Idempotency-Key")
			c.Status(204)
			c.Abort()
			return
		}
		c.Next()
	})
	r.GET("/health/live", func(c *gin.Context) { c.JSON(200, gin.H{"data": gin.H{"status": "alive"}}) })
	r.GET("/health/ready", func(c *gin.Context) {
		if err := deps.ready(c.Request.Context()); err != nil {
			failure(c, 503, "NOT_READY", "Database, migration or seed is not ready")
			return
		}
		c.JSON(200, gin.H{"data": gin.H{"status": "ready", "database": "up", "schema_version": database.SchemaVersion}})
	})
	r.GET("/api/v1/foundation", func(c *gin.Context) {
		if err := deps.ready(c.Request.Context()); err != nil {
			failure(c, 503, "NOT_READY", "Database, migration or seed is not ready")
			return
		}
		store, err := deps.store(c.Request.Context())
		if err != nil {
			failure(c, 503, "NOT_READY", "Store settings are not ready")
			return
		}
		c.JSON(200, gin.H{"data": gin.H{"service": "ecommerce-api", "version": cfg.Version, "environment": cfg.Environment, "database": "up", "schema_version": database.SchemaVersion, "store": store}})
	})
	r.GET("/openapi.json", func(c *gin.Context) { c.Data(200, "application/json; charset=utf-8", api.OpenAPI) })
	for _, register := range registrations {
		register(r)
	}
	r.NoRoute(func(c *gin.Context) { failure(c, 404, "NOT_FOUND", "Endpoint is not available") })
	r.NoMethod(func(c *gin.Context) { failure(c, 405, "METHOD_NOT_ALLOWED", "Method is not allowed") })
	return r
}
