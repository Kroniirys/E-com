package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/internal/database"
	"ecommerce.local/base/internal/httpapi"
	"ecommerce.local/base/internal/httpserver"
	"ecommerce.local/base/internal/modules/admin"
	"ecommerce.local/base/internal/modules/auth"
	"ecommerce.local/base/internal/modules/cart"
	"ecommerce.local/base/internal/modules/catalog"
	"ecommerce.local/base/internal/modules/coupons"
	"ecommerce.local/base/internal/modules/inventory"
	"ecommerce.local/base/internal/modules/notifications"
	"ecommerce.local/base/internal/modules/orders"
	"ecommerce.local/base/internal/modules/reviews"
	"ecommerce.local/base/internal/modules/users"
	"ecommerce.local/base/internal/platform/cache"
	"ecommerce.local/base/internal/platform/limit"
	appRuntime "ecommerce.local/base/internal/runtime"
)

func main() { os.Exit(run()) }

func run() int {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("configuration rejected", "error", err.Error())
		return 1
	}
	security, err := config.LoadSecurity(os.Getenv, cfg.Environment)
	if err != nil {
		log.Error("security configuration rejected", "error", err.Error())
		return 1
	}
	redisLimiter, err := limit.New(security.RedisURL, security.RatePrefix)
	if err != nil {
		log.Error("Redis configuration rejected")
		return 1
	}
	defer redisLimiter.Client.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dbCtx, cancel := context.WithTimeout(ctx, cfg.DBTimeout)
	db, err := database.Open(dbCtx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		log.Error("database initialization failed")
		return 1
	}
	pool, _ := db.DB()
	defer pool.Close()
	authService, err := auth.New(db, security.Secret, redisLimiter)
	if err != nil {
		log.Error("authentication initialization failed")
		return 1
	}
	catalogService := catalog.New(db)
	if os.Getenv("CACHE_ENABLED") != "false" {
		catalogService.Cache = cache.New(redisLimiter.Client, cfg.DatabaseURL)
	}
	endpoints := &httpapi.API{Notifications: notifications.New(db), Coupons: coupons.New(db), Reviews: reviews.New(db), Dashboard: admin.New(db), Cart: cart.New(db, security.MockPayments), Orders: orders.New(db, security.MockPayments), Auth: authService, Users: users.New(db), Catalog: catalogService, Inventory: inventory.New(db), Origins: cfg.AllowedOrigins, SecureCookie: security.SecureCookie}
	server := httpserver.New(cfg, db, log, endpoints.Register)
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		log.Error("HTTP listener unavailable")
		return 1
	}
	log.Info("API started", "address", cfg.HTTPAddr, "version", cfg.Version, "environment", cfg.Environment)
	if err := appRuntime.Serve(ctx, server, listener, cfg.ShutdownTimeout); err != nil {
		log.Error("HTTP server stopped with error")
		return 1
	}
	log.Info("API shutdown complete")
	return 0
}
