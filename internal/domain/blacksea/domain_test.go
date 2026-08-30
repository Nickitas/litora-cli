package blacksea

import "testing"

func TestContainsBlackSeaCoordinates(t *testing.T) {
	t.Parallel()

	if !Contains(43.0, 34.0) {
		t.Fatal("координата в Чёрном море должна приниматься")
	}
	if Contains(53.2, 107.4) {
		t.Fatal("координата другой акватории не должна приниматься")
	}
	if bounds := DefaultBounds(); bounds.MaxLat != 47.5 || bounds.MaxLon != 42.5 {
		t.Fatalf("получены неожиданные границы: %#v", bounds)
	}
}
