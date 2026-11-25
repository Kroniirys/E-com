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
	"ecommerce.local/base/internal/httpserver"
	appRuntime "ecommerce.local/base/internal/runtime"
)

func main() { os.Exit(run()) }

func run() int {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("configuration rejected", "error", err.Error())
		return 1
	}
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
	server := httpserver.New(cfg, db, log)
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
