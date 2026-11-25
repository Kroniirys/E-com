package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/internal/database"
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
	if err != nil {
		cancel()
		log.Error("database initialization failed")
		return 1
	}
	pool, _ := db.DB()
	defer pool.Close()
	err = database.CheckReady(dbCtx, db)
	cancel()
	if err != nil {
		log.Error("worker schema or seed is not ready")
		return 1
	}
	log.Info("worker bootstrap ready", "mode", "foundation-only", "consumers_registered", 0)
	<-ctx.Done()
	log.Info("worker shutdown complete")
	return 0
}
