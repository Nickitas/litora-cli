package benchmark

import (
	"math"
	"strings"
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

	// Проверяем, что наборы названы параметрическими и не приписаны RCP.
	parametricCount := 0
	for _, s := range scenarios {
		if strings.HasPrefix(s.Name, "parametric_") {
			parametricCount++
		}
		if s.ParameterBasis != ScenarioParameterBasisManual {
			t.Errorf("parameter_basis сценария %q = %q, требуется manual", s.Name, s.ParameterBasis)
		}
		if s.Source != nil {
			t.Errorf("source сценария %q должен быть nil без документированного источника", s.Name)
		}
		if strings.Contains(strings.ToLower(s.Name+" "+s.Description), "rcp") {
			t.Errorf("сценарий %q ошибочно обозначен как климатическая траектория RCP", s.Name)
		}
	}
	if parametricCount != 4 {
		t.Errorf("ожидаются 4 параметрических сценария, получено %d", parametricCount)
	}
}

func TestScenarioConfigValidate(t *testing.T) {
	valid := ScenarioConfig{
		Name:              "valid",
		Description:       "Проверочный сценарий",
		ErosionStrength:   10,
		WaveDirection:     90,
		WindSpeed:         12,
		SeaLevelRiseProxy: 0.005,
		StormWeight:       0.1,
		StormIntensity:    1.2,
		ParameterBasis:    ScenarioParameterBasisManual,
		YearsPerStep:      1,
		TotalYears:        10,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("корректный сценарий отклонён: %v", err)
	}

	emptySource := " "
	tests := []struct {
		name   string
		mutate func(*ScenarioConfig)
	}{
		{name: "пустое имя", mutate: func(s *ScenarioConfig) { s.Name = " " }},
		{name: "пустое описание", mutate: func(s *ScenarioConfig) { s.Description = " " }},
		{name: "NaN", mutate: func(s *ScenarioConfig) { s.WindSpeed = math.NaN() }},
		{name: "бесконечность", mutate: func(s *ScenarioConfig) { s.SeaLevelRiseProxy = math.Inf(1) }},
		{name: "нулевая эрозия", mutate: func(s *ScenarioConfig) { s.ErosionStrength = 0 }},
		{name: "направление вне диапазона", mutate: func(s *ScenarioConfig) { s.WaveDirection = 360 }},
		{name: "нулевой ветер", mutate: func(s *ScenarioConfig) { s.WindSpeed = 0 }},
		{name: "отрицательный прокси", mutate: func(s *ScenarioConfig) { s.SeaLevelRiseProxy = -0.001 }},
		{name: "отрицательный вес шторма", mutate: func(s *ScenarioConfig) { s.StormWeight = -0.1 }},
		{name: "слишком большой вес шторма", mutate: func(s *ScenarioConfig) { s.StormWeight = 1.1 }},
		{name: "малая интенсивность", mutate: func(s *ScenarioConfig) { s.StormIntensity = 0.9 }},
		{name: "неизвестная основа", mutate: func(s *ScenarioConfig) { s.ParameterBasis = "external" }},
		{name: "пустой источник", mutate: func(s *ScenarioConfig) { s.Source = &emptySource }},
		{name: "нулевой шаг", mutate: func(s *ScenarioConfig) { s.YearsPerStep = 0 }},
		{name: "нулевой период", mutate: func(s *ScenarioConfig) { s.TotalYears = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := valid
			tt.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("ожидалась ошибка валидации")
			}
		})
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
		Description:     "Проверочный сценарий",
		ErosionStrength: 5.0,
		WaveDirection:   90.0,
		WindSpeed:       12.0,
		StormIntensity:  1.0,
		ParameterBasis:  ScenarioParameterBasisManual,
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
