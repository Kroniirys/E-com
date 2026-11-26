package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"ecommerce.local/base/internal/bootstrap"
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
	demo := os.Getenv("SEED_DEMO_DATA") == "true"
	opts := bootstrap.DemoOptions{AdminEmail: os.Getenv("SEED_ADMIN_EMAIL"), AdminPassword: os.Getenv("SEED_ADMIN_PASSWORD"), CustomerEmail: os.Getenv("SEED_CUSTOMER_EMAIL"), CustomerPassword: os.Getenv("SEED_CUSTOMER_PASSWORD")}
	if demo {
		if err := opts.Validate(); err != nil {
			fmt.Fprintln(os.Stderr, "demo seed requires distinct valid emails and passwords with at least 12 characters and at most 72 bytes")
			return 1
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if demo {
		if err := bootstrap.Demo(ctx, db, opts); err != nil {
			fmt.Fprintln(os.Stderr, "demo seed failed; existing data was not overwritten")
			return 1
		}
	}
	if os.Getenv("SEED_PHASE04_DATA") == "true" {
		if !demo {
			fmt.Fprintln(os.Stderr, "SEED_PHASE04_DATA requires SEED_DEMO_DATA=true")
			return 1
		}
		if err := bootstrap.Phase04Demo(ctx, db, opts); err != nil {
			fmt.Fprintln(os.Stderr, "Phase04 seed failed; ensure the demo cart is empty and fixture identifiers are unused")
			return 1
		}
	}
	fmt.Println("store settings seeded without overwriting existing data")
	return 0
}
