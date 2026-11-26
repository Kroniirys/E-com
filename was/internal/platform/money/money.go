package money

import "ecommerce.local/base/internal/platform/problem"

const Max int64 = 9000000000000

func Line(price, quantity int64) (int64, error) {
	if price < 0 || price > Max || quantity < 1 || quantity > 1000 || (price > 0 && quantity > Max/price) {
		return 0, problem.Invalid("Line amount exceeds the supported money range")
	}
	return price * quantity, nil
}
func Add(a, b int64) (int64, error) {
	if a < 0 || b < 0 || a > Max || b > Max-a {
		return 0, problem.Invalid("Total exceeds the supported money range")
	}
	return a + b, nil
}
