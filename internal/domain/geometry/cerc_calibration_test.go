package geometry

import "testing"

func TestCalibrateCERCCoefficientFindsSyntheticCoefficient(t *testing.T) {
	points := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.005}, {Lat: 0.002, Lon: 0.01}, {Lat: 0, Lon: 0.015}}
	climate := annualTestClimate()
	config := testLongshoreConfig(t)
	config.CERCCoefficient = 0.39
	model, err := RunLongshoreCERC(points, climate, config)
	if err != nil {
		t.Fatalf("подготовка синтетического наблюдения: %v", err)
	}
	duration := totalWaveDurationHours(climate)
	rate := 0.0
	for _, step := range model.Steps {
		rate += step.Cells[1].ShorelineChangeM
	}
	rate = -rate / duration * hoursPerJulianYear
	results, err := CalibrateCERCCoefficient(points, climate, []ShorelineRateObservation{{Lat: points[1].Lat, Lon: points[1].Lon, ShorelineChangeRateMPerYear: rate}}, CERCCalibrationConfig{
		Model:                testLongshoreConfig(t),
		CERCCoefficients:     []float64{0.2, 0.39},
		MaxDistanceMeters:    100,
		MinWaveDurationHours: 8760,
	})
	if err != nil {
		t.Fatalf("CalibrateCERCCoefficient вернула ошибку: %v", err)
	}
	if len(results) != 2 || results[0].CERCCoefficient != 0.39 {
		t.Fatalf("ожидался лучший коэффициент 0.39, получено %#v", results)
	}
}

func TestCalibrateCERCCoefficientRejectsShortWaveClimate(t *testing.T) {
	_, err := CalibrateCERCCoefficient([]LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.005}, {Lat: 0, Lon: 0.01}}, WaveClimate{Source: "короткий", Conditions: []WaveCondition{{DurationHours: 1, SignificantWaveHeightM: 1, PeakPeriodSeconds: 5, DirectionFromDeg: 0}}}, []ShorelineRateObservation{{Lat: 0, Lon: 0}}, CERCCalibrationConfig{CERCCoefficients: []float64{0.39}})
	if err == nil {
		t.Fatal("краткий ряд не должен допускаться к годовой калибровке")
	}
}

func annualTestClimate() WaveClimate {
	conditions := make([]WaveCondition, 12)
	for i := range conditions {
		conditions[i] = WaveCondition{DurationHours: 730, SignificantWaveHeightM: 1.2, PeakPeriodSeconds: 6, DirectionFromDeg: 0}
	}
	return WaveClimate{Source: "синтетический годовой ряд", Conditions: conditions}
}
