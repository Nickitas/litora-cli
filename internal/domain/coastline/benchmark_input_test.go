package coastline

import "testing"

// TestParseBenchmarkCoastline проверяет извлечение линии из файла эталонного
// участка для непосредственного использования в CERC-команде.
func TestParseBenchmarkCoastline(t *testing.T) {
	points, err := parseCoastlineData([]byte(`{"id":"example","coastline":[{"lat":45,"lon":36},{"lat":45.01,"lon":36.01}]}`), GeoBounds{})
	if err != nil {
		t.Fatalf("parseCoastlineData вернула ошибку: %v", err)
	}
	if len(points) != 2 || points[0].Lat != 45 || points[1].Lon != 36.01 {
		t.Fatalf("неверная береговая линия: %+v", points)
	}
}
