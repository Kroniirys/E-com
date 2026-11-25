package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/internal/database"
)

func main() { os.Exit(run()) }
func run() int {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if cfg.Environment != "development" && cfg.Environment != "test" {
		fmt.Fprintln(os.Stderr, "seed is allowed only in development or test")
		return 1
	}
	name := os.Getenv("SEED_STORE_NAME")
	fee, err := strconv.ParseInt(os.Getenv("SEED_SHIPPING_FEE_VND"), 10, 64)
	if err != nil || name == "" {
		fmt.Fprintln(os.Stderr, "SEED_STORE_NAME and integer SEED_SHIPPING_FEE_VND are required")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBTimeout)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed database connection failed")
		return 1
	}
	pool, _ := db.DB()
	defer pool.Close()
	if err := database.Seed(ctx, db, name, fee); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("store settings seeded without overwriting existing data")
	return 0
}
