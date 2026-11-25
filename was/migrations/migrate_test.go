package migrations

import "testing"

func TestResetGuardRejectsDatabaseOverrides(t *testing.T) {
	valid := "postgres://u:p@localhost/ecom_test?sslmode=disable"
	if !AllowTestReset("test", valid, "delete-test-schema") {
		t.Fatal("isolated test reset rejected")
	}
	cases := []struct{ environment, url, confirmation string }{
		{"development", valid, "delete-test-schema"},
		{"test", "postgres://u:p@localhost/ecom", "delete-test-schema"},
		{"test", valid, ""},
		{"test", valid + "&dbname=ecom", "delete-test-schema"},
		{"test", valid + "&database=ecom", "delete-test-schema"},
		{"test", "postgres://u:p@localhost/ecom_test%2Fother_test", "delete-test-schema"},
	}
	for _, tc := range cases {
		if AllowTestReset(tc.environment, tc.url, tc.confirmation) {
			t.Fatalf("unsafe reset accepted: %s", tc.url)
		}
	}
}
