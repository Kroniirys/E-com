package orders

import "testing"

func TestTransitions(t *testing.T) {
	for _, v := range [][4]string{{"Pending", "Confirmed", "COD", "Pending"}, {"Confirmed", "Processing", "Mock", "Paid"}, {"Processing", "Shipped", "COD", "Pending"}, {"Shipped", "Delivered", "COD", "Pending"}, {"Pending", "Failed", "Mock", "Pending"}, {"Confirmed", "Cancelled", "COD", "Pending"}} {
		if !CanTransition(v[0], v[1], v[2], v[3]) {
			t.Fatal("rejected", v)
		}
	}
	for _, v := range [][4]string{{"Pending", "Confirmed", "Mock", "Pending"}, {"Confirmed", "Processing", "Mock", "Pending"}, {"Confirmed", "Cancelled", "Mock", "Paid"}, {"Shipped", "Cancelled", "COD", "Pending"}, {"Delivered", "Delivered", "COD", "Paid"}, {"Pending", "Shipped", "COD", "Pending"}} {
		if CanTransition(v[0], v[1], v[2], v[3]) {
			t.Fatal("accepted", v)
		}
	}
}
