package catalog

import "testing"

func TestProductValidation(t *testing.T) {
	base := ProductInput{SKU: " sku-1 ", Name: " Lamp ", Status: "Active", PriceMinor: 100, OriginalPriceMinor: 120, Images: []ImageInput{}, CategoryIDs: []string{}}
	if err := validateProduct(&base); err != nil {
		t.Fatal(err)
	}
	if base.SKU != "SKU-1" {
		t.Fatal("SKU not normalized")
	}
	for _, url := range []string{"javascript:alert(1)", "file:///etc/passwd", "http://user:pass@example.test/image"} {
		in := base
		in.Images = []ImageInput{{URL: url}}
		if validateProduct(&in) == nil {
			t.Fatal("unsafe image accepted")
		}
	}
	for _, price := range []int64{-1, MaxMoney + 1} {
		in := base
		in.PriceMinor = price
		if validateProduct(&in) == nil {
			t.Fatal("invalid price accepted")
		}
	}
}
