package config

import "testing"

func TestSecurityConfiguration(t *testing.T) {
	env := map[string]string{"AUTH_JWT_SECRET": "test-only-secret-with-32-characters-minimum", "REDIS_URL": "redis://localhost:6379/0"}
	get := func(k string) string { return env[k] }
	if _, err := LoadSecurity(get, "development"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSecurity(get, "production"); err == nil {
		t.Fatal("production accepted non-TLS Redis")
	}
	env["REDIS_URL"] = "rediss://localhost:6379/0"
	if _, err := LoadSecurity(get, "production"); err != nil {
		t.Fatal(err)
	}
	env["AUTH_JWT_SECRET"] = "short"
	if _, err := LoadSecurity(get, "development"); err == nil {
		t.Fatal("short secret accepted")
	}
}

func TestMockPaymentPolicy(t *testing.T) {
	env := map[string]string{"AUTH_JWT_SECRET": "test-only-secret-with-32-characters-minimum", "REDIS_URL": "rediss://localhost:6379/0"}
	get := func(k string) string { return env[k] }
	c, e := LoadSecurity(get, "test")
	if e != nil || !c.MockPayments {
		t.Fatal("local mock default", e)
	}
	c, e = LoadSecurity(get, "production")
	if e != nil || c.MockPayments {
		t.Fatal("production mock default", e)
	}
	env["MOCK_PAYMENTS_ENABLED"] = "true"
	if _, e = LoadSecurity(get, "production"); e == nil {
		t.Fatal("production enabled simulation")
	}
	env["MOCK_PAYMENTS_ENABLED"] = "false"
	c, e = LoadSecurity(get, "test")
	if e != nil || c.MockPayments {
		t.Fatal("explicit disable", e)
	}
	env["MOCK_PAYMENTS_ENABLED"] = "invalid"
	if _, e = LoadSecurity(get, "test"); e == nil {
		t.Fatal("invalid flag accepted")
	}
}
