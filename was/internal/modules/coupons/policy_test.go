package coupons

import (
	"testing"
	"time"
)

func TestDiscountPolicy(t *testing.T) {
	now := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	base := Coupon{Enabled: true, Kind: "Percentage", Value: 15, UsageLimit: 2, ExpiresAt: now.Add(time.Hour)}
	if v, e := Discount(base, 1001, now); e != nil || v != 150 {
		t.Fatal(v, e)
	}
	c := base
	c.Value = 100
	if v, e := Discount(c, 1001, now); e != nil || v != 1001 {
		t.Fatal(v, e)
	}
	c = base
	c.Kind = "Fixed"
	c.Value = 2000
	if v, e := Discount(c, 1001, now); e != nil || v != 1001 {
		t.Fatal(v, e)
	}
	for _, mutate := range []func(*Coupon){func(c *Coupon) { c.Enabled = false }, func(c *Coupon) { c.ExpiresAt = now }, func(c *Coupon) { c.UsedCount = 2 }, func(c *Coupon) { c.MinimumMinor = 1002 }} {
		c = base
		mutate(&c)
		if _, e := Discount(c, 1001, now); e == nil {
			t.Fatal("ineligible coupon accepted", c)
		}
	}
	if _, e := Discount(base, 9000000000001, now); e == nil {
		t.Fatal("unbounded subtotal accepted")
	}
	c = base
	c.Value = 99
	if v, e := Discount(c, 9000000000000, now); e != nil || v != 8910000000000 {
		t.Fatal("large discount", v, e)
	}
}
