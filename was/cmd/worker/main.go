package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/internal/database"
	"ecommerce.local/base/internal/platform/cache"
	"ecommerce.local/base/internal/platform/limit"
	"ecommerce.local/base/internal/platform/metrics"
	"ecommerce.local/base/internal/worker"
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
	infrastructure, err := config.LoadWorker(os.Getenv, cfg.Environment)
	if err != nil {
		log.Error("worker configuration rejected")
		return 1
	}
	redisClient, err := limit.New(os.Getenv("REDIS_URL"), "worker:")
	if err != nil {
		log.Error("Redis configuration rejected")
		return 1
	}
	defer redisClient.Client.Close()
	slog.SetDefault(log)
	service := &worker.Worker{DB: db, URL: infrastructure.URL, Prefix: infrastructure.Prefix, Processor: &worker.Processor{DB: db, Cache: cache.New(redisClient.Client, cfg.DatabaseURL)}}
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/metrics", metrics.Handler(db))
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		check, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if !service.Ready.Load() || database.CheckReady(check, db) != nil || redisClient.Client.Ping(check).Err() != nil {
			w.WriteHeader(503)
			return
		}
		w.Header().Set("X-App-Version", cfg.Version)
		w.WriteHeader(200)
	})
	server := &http.Server{Addr: infrastructure.Address, Handler: mux, ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 5 * time.Second}
	go func() {
		if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Error("worker HTTP listener failed")
			stop()
		}
	}()
	log.Info("worker started", "mode", "relay-and-consumers")
	err = service.Run(ctx)
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
	if err != nil {
		return 1
	}
	log.Info("worker shutdown complete")
	return 0
}
