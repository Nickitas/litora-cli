package benchmark

import (
	"testing"

	"coastal-geometry/internal/domain/geometry"
)

func TestDefaultScenarios(t *testing.T) {
	scenarios := DefaultScenarios(10.0, 90.0)
	if len(scenarios) < 4 {
		t.Errorf("ожидаются как минимум 4 сценария, получено %d", len(scenarios))
	}

	// Проверяем, что базовый сценарий существует
	foundBaseline := false
	for _, s := range scenarios {
		if s.Name == "baseline" {
			foundBaseline = true
			if s.ErosionStrength != 10.0 {
				t.Errorf("силa baseline = %v, требуется 10.0", s.ErosionStrength)
			}
			if s.WaveDirection != 90.0 {
				t.Errorf("направление baseline = %v, требуется 90.0", s.WaveDirection)
			}
		}
	}
	if !foundBaseline {
		t.Error("baseline сценарий отсутствует")
	}

	// Проверяем, что существуют сценарии RCP
	rcpCount := 0
	for _, s := range scenarios {
		if len(s.Name) >= 3 && s.Name[:3] == "rcp" {
			rcpCount++
		}
	}
	if rcpCount < 2 {
		t.Errorf("ожидаются как минимум 2 сценария RCP, получено %d", rcpCount)
	}
}

func TestRunScenario(t *testing.T) {
	site := BenchmarkSite{
		ID: "test",
		Coastline: []geometry.LatLon{
			{Lat: 41.8, Lon: 41.8},
			{Lat: 41.81, Lon: 41.81},
			{Lat: 41.82, Lon: 41.82},
			{Lat: 41.83, Lon: 41.83},
			{Lat: 41.84, Lon: 41.84},
		},
	}
	scenario := ScenarioConfig{
		Name:            "test",
		ErosionStrength: 5.0,
		WaveDirection:   90.0,
		WindSpeed:       12.0,
		YearsPerStep:    1.0,
		TotalYears:      2,
	}

	result, err := RunScenario(site, scenario, nil)
	if err != nil {
		t.Fatalf("RunScenario не удалось: %v", err)
	}

	if len(result.SegmentRetreats) != 5 {
		t.Errorf("длина отступлений сегментов = %d, требуется 5", len(result.SegmentRetreats))
	}
	if result.CoastLengthKm <= 0 {
		t.Errorf("длина берега = %v, требуется > 0", result.CoastLengthKm)
	}
}

func TestCompareScenarios(t *testing.T) {
	coastline := []geometry.LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 0.001, Lon: 0.001},
		{Lat: 0.002, Lon: 0.002},
	}
	results := []ScenarioResult{
		{
			Config:          ScenarioConfig{Name: "baseline"},
			MeanRetreatRate: 1.0,
			MaxRetreatRate:  2.0,
			ErodingFraction: 0.3,
			HotspotCount:    2,
			SegmentRetreats: []float64{0.5, 2.0, 0.5},
		},
		{
			Config:          ScenarioConfig{Name: "high"},
			MeanRetreatRate: 2.0,
			MaxRetreatRate:  3.5,
			ErodingFraction: 0.5,
			HotspotCount:    3,
			SegmentRetreats: []float64{1.0, 3.5, 1.5},
		},
	}

	diffs := CompareScenarios(results, coastline)
	if len(diffs) != 1 {
		t.Fatalf("ожидаются 1 разница, получено %d", len(diffs))
	}

	if diffs[0].MeanRetreatDelta <= 0 {
		t.Errorf("дельта = %v, требуется > 0", diffs[0].MeanRetreatDelta)
	}
	if diffs[0].Modified.Name != "high" {
		t.Errorf("изменённое имя = %q, требуется 'high'", diffs[0].Modified.Name)
	}
}
