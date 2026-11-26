package orders

func CanTransition(from, to, method, payment string) bool {
	if to == "Cancelled" {
		return (from == "Pending" || from == "Confirmed") && payment == "Pending"
	}
	if to == "Failed" {
		return from == "Pending" && payment == "Pending"
	}
	switch from {
	case "Pending":
		return to == "Confirmed" && method == "COD" && payment == "Pending"
	case "Confirmed":
		return to == "Processing" && (method == "COD" || payment == "Paid")
	case "Processing":
		return to == "Shipped"
	case "Shipped":
		return to == "Delivered"
	}
	return false
}
