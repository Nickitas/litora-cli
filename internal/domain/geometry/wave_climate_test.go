package geometry

import "testing"

// TestWaveClimateValidateRequiresProvenance проверяет запрет расчёта без
// происхождения волновых данных.
func TestWaveClimateValidateRequiresProvenance(t *testing.T) {
	climate := WaveClimate{Conditions: []WaveCondition{{DurationHours: 1, SignificantWaveHeightM: 1, PeakPeriodSeconds: 5, DirectionFromDeg: 20}}}
	if err := climate.Validate(); err == nil {
		t.Fatal("ожидалась ошибка для ряда без источника")
	}
}

// TestLoadOpenMeteoMarineClimate проверяет преобразование открытого почасового
// ответа к воспроизводимому внутреннему формату.
func TestLoadOpenMeteoMarineClimate(t *testing.T) {
	data := []byte(`{"hourly":{"time":["2026-08-17T00:00"],"wave_height":[1.2],"wave_period":[5.5],"wave_direction":[90]}}`)
	climate, err := loadOpenMeteoMarineClimate(data)
	if err != nil {
		t.Fatalf("loadOpenMeteoMarineClimate вернула ошибку: %v", err)
	}
	if len(climate.Conditions) != 1 || climate.Conditions[0].SignificantWaveHeightM != 1.2 || climate.Conditions[0].PeakPeriodSeconds != 5.5 {
		t.Fatalf("неверное преобразование Open-Meteo: %+v", climate.Conditions)
	}
}
