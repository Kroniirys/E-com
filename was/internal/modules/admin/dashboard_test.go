package admin

import (
	"testing"
	"time"
)

func TestDateRangeTimezoneAndDST(t *testing.T) {
	r, e := ParseRange("2026-03-08", "2026-03-09", "America/New_York")
	if e != nil || r.To.Sub(*r.From) != 23*time.Hour {
		t.Fatal("DST day range", r, e)
	}
	r, e = ParseRange("2026-11-26", "2026-11-27", "Asia/Ho_Chi_Minh")
	if e != nil || r.From.Hour() != 17 || r.From.Day() != 25 {
		t.Fatal("local date boundary", r, e)
	}
	for _, v := range [][3]string{{"2026-11-26", "2026-11-26", "UTC"}, {"wrong", "", "UTC"}, {"", "", "Invalid/Zone"}, {"", "", "Local"}} {
		if _, e := ParseRange(v[0], v[1], v[2]); e == nil {
			t.Fatal("invalid range accepted", v)
		}
	}
}
