package benchmark

import (
	"testing"

	"coastal-geometry/internal/domain/geometry"
)

func TestFindHotspots(t *testing.T) {
	// Побережье с одним чётким участком активной эрозии в середине
	coastline := []geometry.LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 0.001, Lon: 0.001},
		{Lat: 0.002, Lon: 0.002},
		{Lat: 0.003, Lon: 0.003},
		{Lat: 0.004, Lon: 0.004},
		{Lat: 0.005, Lon: 0.005},
	}

	// Скорости отступления: низкие, высокие в середине, низкие
	rates := []SegmentRate{
		{Index: 0, RetreatRate: 0.1, Center: coastline[0]},
		{Index: 1, RetreatRate: 0.2, Center: coastline[1]},
		{Index: 2, RetreatRate: 5.0, Center: coastline[2]},
		{Index: 3, RetreatRate: 5.5, Center: coastline[3]},
		{Index: 4, RetreatRate: 0.3, Center: coastline[4]},
		{Index: 5, RetreatRate: 0.1, Center: coastline[5]},
	}

	hotspots := FindHotspots(rates, coastline, 3, 0.5)
	if len(hotspots) == 0 {
		t.Fatal("ожидается, что будет как минимум 1 точка доступа")
	}

	if hotspots[0].MeanRetreatRate < 3.0 {
		t.Errorf("средняя скорость в верхней точке доступа = %v, потребность > 3,0", hotspots[0].MeanRetreatRate)
	}
	if hotspots[0].Rank != 1 {
		t.Errorf("рейтинг лучших точек доступа = %d, хочу 1", hotspots[0].Rank)
	}
}

func TestFindHotspotsNoErosion(t *testing.T) {
	coastline := []geometry.LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 0.001, Lon: 0.001},
		{Lat: 0.002, Lon: 0.002},
	}
	rates := []SegmentRate{
		{Index: 0, RetreatRate: 0.0, Center: coastline[0]},
		{Index: 1, RetreatRate: 0.0, Center: coastline[1]},
		{Index: 2, RetreatRate: 0.0, Center: coastline[2]},
	}

	hotspots := FindHotspots(rates, coastline, 3, 0.5)
	if len(hotspots) != 0 {
		t.Errorf("ожидаемые 0 горячих точек для нулевой эрозии, получено %d", len(hotspots))
	}
}

func TestSegmentRates(t *testing.T) {
	site := BenchmarkSite{
		ID: "test",
		Coastline: []geometry.LatLon{
			{Lat: 0, Lon: 0},
			{Lat: 0.001, Lon: 0.001},
			{Lat: 0.002, Lon: 0.002},
			{Lat: 0.003, Lon: 0.003},
		},
	}
	config := DefaultCalibrationConfig()
	config.TotalYears = 1
	config.YearsPerStep = 1

	rates := SegmentRates(site, config, 10.0, 90.0)
	if len(rates) != 4 {
		t.Errorf("rates len = %d, want 4", len(rates))
	}
	for i, r := range rates {
		if r.Index != i {
			t.Errorf("rate[%d].Index = %d, want %d", i, r.Index, i)
		}
	}
}
