package main

import (
	"context"
	"ecommerce.local/base/internal/database"
	"ecommerce.local/base/migrations"
	"fmt"
	"github.com/google/uuid"
	"os"
	"regexp"
	"strings"
	"time"
)

func main() { os.Exit(run()) }
func run() int {
	raw := os.Getenv("TEST_DATABASE_URL")
	if !migrations.AllowTestReset("test", raw, "delete-test-schema") {
		fmt.Fprintln(os.Stderr, "testscope requires a safe *_test database")
		return 1
	}
	if len(os.Args) < 2 || (os.Args[1] != "create" && os.Args[1] != "drop") {
		fmt.Fprintln(os.Stderr, "usage: testscope create | drop ecom_e2e_<id>")
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := database.Open(ctx, raw)
	if err != nil {
		fmt.Fprintln(os.Stderr, "test database unavailable")
		return 1
	}
	pool, _ := db.DB()
	defer pool.Close()
	if os.Args[1] == "create" {
		name := "ecom_e2e_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if err := db.WithContext(ctx).Exec("CREATE SCHEMA " + name).Error; err != nil {
			fmt.Fprintln(os.Stderr, "test schema create failed")
			return 1
		}
		fmt.Println(name)
		return 0
	}
	if len(os.Args) != 3 || !regexp.MustCompile(`^ecom_e2e_[0-9a-f]{32}$`).MatchString(os.Args[2]) {
		fmt.Fprintln(os.Stderr, "refusing to drop a schema not owned by the E2E runner")
		return 1
	}
	if err := db.WithContext(ctx).Exec("DROP SCHEMA " + os.Args[2] + " CASCADE").Error; err != nil {
		fmt.Fprintln(os.Stderr, "test schema cleanup failed")
		return 1
	}
	return 0
}
