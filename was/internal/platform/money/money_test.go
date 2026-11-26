package money

import "testing"

func TestBoundedMoney(t *testing.T) {
	if got, err := Line(150000, 2); err != nil || got != 300000 {
		t.Fatal(got, err)
	}
	for _, v := range [][2]int64{{-1, 1}, {1, 0}, {Max, 2}, {Max + 1, 1}, {1, 1001}} {
		if _, err := Line(v[0], v[1]); err == nil {
			t.Fatal("accepted invalid line", v)
		}
	}
	if _, err := Add(Max, 1); err == nil {
		t.Fatal("accepted overflowing total")
	}
	if got, err := Add(Max-1, 1); err != nil || got != Max {
		t.Fatal(got, err)
	}
}
