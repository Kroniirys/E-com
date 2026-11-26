package reviews

import (
	"strings"
	"testing"
)

func TestReviewValidation(t *testing.T) {
	for _, v := range []Input{{0, "text"}, {6, "text"}, {5, " "}, {5, strings.Repeat("á", 2001)}} {
		if validate(&v) == nil {
			t.Fatal("invalid review accepted")
		}
	}
	v := Input{5, "  Nội dung tốt  "}
	if validate(&v) != nil || v.Content != "Nội dung tốt" {
		t.Fatal("valid unicode content rejected")
	}
}
