package users

import (
	"strings"
	"testing"
)

func TestValidationAndLiteralSearch(t *testing.T) {
	in := Input{Email: "  Customer@Example.test ", Name: "  Test  ", Role: "Customer", Status: "Active"}
	if err := Validate(&in); err != nil {
		t.Fatal(err)
	}
	if in.Email != "customer@example.test" || in.Name != "Test" {
		t.Fatal("normalization failed")
	}
	for _, email := range []string{"", "invalid", "Name <user@example.test>"} {
		candidate := in
		candidate.Email = email
		if Validate(&candidate) == nil {
			t.Fatal("invalid email accepted")
		}
	}
	if ValidPassword(strings.Repeat("á", 6)) || ValidPassword(strings.Repeat("á", 37)) || !ValidPassword(strings.Repeat("á", 12)) || !ValidPassword(strings.Repeat("a", 12)) {
		t.Fatal("password byte limit is incorrect")
	}
	cases := map[string]string{`a%b`: `%a\%b%`, `a_b`: `%a\_b%`, `a\b`: `%a\\b%`}
	for input, want := range cases {
		if got := Like(input); got != want {
			t.Fatalf("literal %q: got %q want %q", input, got, want)
		}
	}
}
