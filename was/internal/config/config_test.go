package config

import (
	"strings"
	"testing"
)

func TestConfigurationRejectsInvalidInputsWithoutLeakingSecrets(t *testing.T) {
	cases := []map[string]string{
		{},
		{"DATABASE_URL": "postgres://u:supersecret@host/base?sslmode=disable", "APP_ENV": "invalid"},
		{"DATABASE_URL": "postgres://u:supersecret@host/base?dbname=real"},
		{"DATABASE_URL": "postgres://u:supersecret@host/base?host=other"},
		{"DATABASE_URL": "postgres://u:supersecret@host/base%2Fother"},
		{"DATABASE_URL": "postgres://u:supersecret@host/base?sslmode=disable&sslmode=require"},
		{"DATABASE_URL": "postgres://u:supersecret@host/base", "REQUEST_TIMEOUT": "0s"},
		{"DATABASE_URL": "postgres://u:supersecret@host/base", "ALLOWED_ORIGINS": "*"},
		{"DATABASE_URL": "postgres://u:supersecret@host/base", "ALLOWED_ORIGINS": "https://site.test/path"},
		{"DATABASE_URL": "postgres://u:supersecret@host/base", "APP_ENV": "production"},
	}
	for i, env := range cases {
		_, err := Load(func(key string) string { return env[key] })
		if err == nil {
			t.Fatalf("case %d unexpectedly accepted", i)
		}
		if strings.Contains(err.Error(), "supersecret") {
			t.Fatal("secret leaked")
		}
	}
}

func TestValidConfiguration(t *testing.T) {
	env := map[string]string{"DATABASE_URL": "postgres://u:p@localhost/ecom_test?sslmode=disable&search_path=ecom_it_123", "ALLOWED_ORIGINS": "http://localhost:5173, http://localhost:8081"}
	c, err := Load(func(key string) string { return env[key] })
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || len(c.AllowedOrigins) != 2 || c.DBTimeout <= 0 {
		t.Fatalf("bad defaults: %+v", c)
	}
}
