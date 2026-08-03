package benchmark

import (
	"testing"

	"coastal-geometry/internal/domain/geometry"
)

func TestComputeValidationMetrics(t *testing.T) {
	comparisons := []ComparisonPoint{
		{Observed: 1.0, Modeled: 1.1},
		{Observed: 2.0, Modeled: 1.9},
		{Observed: 0.5, Modeled: 0.6},
		{Observed: 1.5, Modeled: 1.4},
	}
	m := computeValidationMetrics(comparisons)

	if m.N != 4 {
		t.Errorf("N = %d, требуется 4", m.N)
	}
	if m.RMSE <= 0 {
		t.Errorf("RMSE = %v, требуется > 0", m.RMSE)
	}
	if m.MAE <= 0 {
		t.Errorf("MAE = %v, требуется > 0", m.MAE)
	}
	// Идеальная корреляция должна давать R² близкий к 1
	if m.RSquared < 0.95 {
		t.Errorf("R² = %v, требуется > 0.95 для почти идеальной корреляции", m.RSquared)
	}
}

func TestComputeValidationMetricsPerfect(t *testing.T) {
	comparisons := []ComparisonPoint{
		{Observed: 1.0, Modeled: 1.0},
		{Observed: 2.0, Modeled: 2.0},
		{Observed: 3.0, Modeled: 3.0},
	}
	m := computeValidationMetrics(comparisons)
	if m.RMSE > 1e-9 {
		t.Errorf("Идеальная подгонка RMSE = %v, требуется 0", m.RMSE)
	}
	if m.RSquared < 0.999 {
		t.Errorf("Идеальная подгонка R² = %v, требуется ~1", m.RSquared)
	}
}

func TestNearestSegmentIndex(t *testing.T) {
	coastline := []geometry.LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 1, Lon: 1},
		{Lat: 2, Lon: 2},
		{Lat: 3, Lon: 3},
	}
	tests := []struct {
		target  geometry.LatLon
		wantIdx int
	}{
		{geometry.LatLon{Lat: 0.1, Lon: 0.1}, 0},
		{geometry.LatLon{Lat: 1.1, Lon: 1.1}, 1},
		{geometry.LatLon{Lat: 2.9, Lon: 2.9}, 3},
	}
	for _, tt := range tests {
		got := nearestSegmentIndex(coastline, tt.target)
		if got != tt.wantIdx {
			t.Errorf("nearestSegmentIndex(%v) = %d, требуется %d", tt.target, got, tt.wantIdx)
		}
	}
}

func TestHaversineKm(t *testing.T) {
	// Та же точка
	if d := haversineKm(geometry.LatLon{Lat: 45, Lon: 30}, geometry.LatLon{Lat: 45, Lon: 30}); d > 1e-6 {
		t.Errorf("Расстояние одной и той же точки = %v, требуется 0", d)
	}
	// Известно: Одесса - Кобулети ~ 1020 км
	odessa := geometry.LatLon{Lat: 46.46, Lon: 30.73}
	kobuleti := geometry.LatLon{Lat: 41.82, Lon: 41.78}
	d := haversineKm(odessa, kobuleti)
	if d < 950 || d > 1100 {
		t.Errorf("Одесса-Кобулети = %v км, требуется 950-1100", d)
	}
}

func TestPearsonCorrelation(t *testing.T) {
	// Идеальная положительная корреляция
	perfect := []ComparisonPoint{
		{Observed: 1, Modeled: 2},
		{Observed: 2, Modeled: 4},
		{Observed: 3, Modeled: 6},
	}
	if r := pearsonCorrelation(perfect); r < 0.999 {
		t.Errorf("Идеальная корреляция r = %v, требуется ~1", r)
	}

	// Идеальная отрицательная корреляция
	negative := []ComparisonPoint{
		{Observed: 1, Modeled: 10},
		{Observed: 5, Modeled: 5},
		{Observed: 10, Modeled: 1},
	}
	if r := pearsonCorrelation(negative); r > -0.99 {
		t.Errorf("Отрицательная корреляция r = %v, требуется ~-1", r)
	}
}
