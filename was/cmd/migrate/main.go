package main

import (
	"fmt"
	"os"

	"ecommerce.local/base/internal/config"
	"ecommerce.local/base/migrations"
)

func main() { os.Exit(run()) }
func run() int {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	action := "up"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}
	if action != "up" && action != "version" && action != "down-one" {
		fmt.Fprintln(os.Stderr, "usage: migrate [up|version|down-one]")
		return 1
	}
	if action == "down-one" && !migrations.AllowTestReset(cfg.Environment, cfg.DatabaseURL, os.Getenv("ALLOW_TEST_RESET")) {
		fmt.Fprintln(os.Stderr, "down-one requires APP_ENV=test, *_test database and ALLOW_TEST_RESET=delete-test-schema")
		return 1
	}
	m, err := migrations.New(cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migration initialization failed")
		return 1
	}
	defer m.Close()
	switch action {
	case "up":
		err = migrations.Up(m)
	case "down-one":
		err = m.Steps(-1)
	case "version":
		v, dirty, e := m.Version()
		if e != nil {
			err = e
		} else {
			fmt.Printf("version=%d dirty=%t\n", v, dirty)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "migration failed; inspect schema version and migration files")
		return 1
	}
	return 0
}
