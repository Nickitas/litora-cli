package geometry

import (
	"testing"
)

func TestValidateTemporalParameters(t *testing.T) {
	tests := []struct {
		name     string
		params   TemporalParameters
		wantWarn bool // хотим увидеть предупреждения
	}{
		{
			name: "валидные параметры",
			params: TemporalParameters{
				YearsPerStep:       1.0,
				MinYearsPerStep:    0.1,
				MaxYearsPerStep:    10.0,
				StormProbability:   0.1,
				SeaLevelRise:       0.001,
			},
			wantWarn: false,
		},
		{
			name: "YearsPerStep меньше минимума",
			params: TemporalParameters{
				YearsPerStep:       0.5,
				MinYearsPerStep:    1.0,
				MaxYearsPerStep:    10.0,
			},
			wantWarn: true,
		},
		{
			name: "YearsPerStep больше максимума",
			params: TemporalParameters{
				YearsPerStep:       15.0,
				MinYearsPerStep:    1.0,
				MaxYearsPerStep:    10.0,
			},
			wantWarn: true,
		},
		{
			name: "высокая вероятность шторма",
			params: TemporalParameters{
				YearsPerStep:       1.0,
				MinYearsPerStep:    0.1,
				MaxYearsPerStep:    10.0,
				StormProbability:   0.8,
			},
			wantWarn: true,
		},
		{
			name: "высокий подъём уровня моря",
			params: TemporalParameters{
				YearsPerStep:       1.0,
				MinYearsPerStep:    0.1,
				MaxYearsPerStep:    10.0,
				SeaLevelRise:       0.02,
			},
			wantWarn: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateTemporalParameters(tt.params)
			hasWarnings := len(warnings) > 0
			
			if tt.wantWarn && !hasWarnings {
				t.Errorf("ValidateTemporalParameters() expected warnings, got none")
			}
			if !tt.wantWarn && hasWarnings {
				t.Errorf("ValidateTemporalParameters() expected no warnings, got %v", warnings)
			}
		})
	}
}

func TestGetTemporalSummary(t *testing.T) {
	// Создаем тестовый результат TemporalResult
	result := TemporalResult{
		TotalYears:         100,
		StormCount:         5,
		AccumulatedErosion: 150.5,
		FinalSeaLevelRise:  0.3,
		Snapshots: [][]LatLon{
			{{Lat: 45, Lon: 30}, {Lat: 45, Lon: 31}, {Lat: 46, Lon: 30}},
			{{Lat: 45, Lon: 30}, {Lat: 45, Lon: 31}, {Lat: 46, Lon: 30}},
			{{Lat: 45.1, Lon: 30}, {Lat: 45, Lon: 31}, {Lat: 46, Lon: 30}},
			{{Lat: 45.1, Lon: 30}, {Lat: 45, Lon: 31}, {Lat: 46, Lon: 30}},
		},
	}

	summary := GetTemporalSummary(result)

	// Проверяем основные поля
	if summary["total_years"] != float64(100) {
		t.Errorf("Expected total_years=100, got %v", summary["total_years"])
	}
	if summary["storm_count"] != 5 {
		t.Errorf("Expected storm_count=5, got %v", summary["storm_count"])
	}
	if summary["accumulated_erosion_m"] != 150.5 {
		t.Errorf("Expected accumulated_erosion_m=150.5, got %v", summary["accumulated_erosion_m"])
	}

	// Проверяем вычисляемые поля
	steps, ok := summary["total_steps"].(int)
	if !ok || steps != 3 {
		t.Errorf("Expected total_steps=3, got %v (type %T)", summary["total_steps"], summary["total_steps"])
	}

	stormFreq, ok := summary["storm_frequency"].(float64)
	if !ok {
		t.Errorf("Expected storm_frequency to be float64, got %T", summary["storm_frequency"])
	} else if stormFreq != float64(5)/float64(3) {
		t.Errorf("Expected storm_frequency=%v, got %v", float64(5)/float64(3), stormFreq)
	}
}

func TestCalculateErosionMetrics(t *testing.T) {
	points := []LatLon{
		{Lat: 45, Lon: 30},
		{Lat: 45, Lon: 31},
		{Lat: 46, Lon: 31},
		{Lat: 46, Lon: 30},
	}

	result := TemporalResult{
		Snapshots: [][]LatLon{
			points, // Step 0 - initial
			// Step 1 - slightly eroded
			{
				{Lat: 45, Lon: 30.01},
				{Lat: 45, Lon: 31.01},
				{Lat: 46, Lon: 31.01},
				{Lat: 46, Lon: 30.01},
			},
			// Step 2 - more eroded
			{
				{Lat: 45, Lon: 30.02},
				{Lat: 45, Lon: 31.02},
				{Lat: 46, Lon: 31.02},
				{Lat: 46, Lon: 30.02},
			},
		},
		TemporalStates: []TemporalState{
			{Step: 0, Year: 0},
			{Step: 1, Year: 10},
			{Step: 2, Year: 20},
		},
	}

	metrics := CalculateErosionMetrics(result)

	if len(metrics) != len(result.Snapshots) {
		t.Errorf("CalculateErosionMetrics() returned %d metrics, want %d", len(metrics), len(result.Snapshots))
	}

	// Проверяем начальный шаг
	if metrics[0].Step != 0 {
		t.Errorf("Initial metric has step %d, want 0", metrics[0].Step)
	}

	// Проверяем, что длина рассчитывается
	for i, m := range metrics {
		if m.Step != i {
			t.Errorf("Metric %d has step %d, want %d", i, m.Step, i)
		}
		if m.LengthKm <= 0 {
			t.Errorf("Metric %d has invalid length %f", i, m.LengthKm)
		}
	}
}
