package geometry

import (
	"math"
	"testing"
)

// TestRunLongshoreCERCConservesSediment проверяет баланс ячеек при нулевом
// граничном потоке: размыв должен равняться аккумуляции.
func TestRunLongshoreCERCConservesSediment(t *testing.T) {
	points := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.005}, {Lat: 0.002, Lon: 0.01}, {Lat: 0, Lon: 0.015}}
	config := testLongshoreConfig(t)
	climate := WaveClimate{Source: "проверочный ряд", Conditions: []WaveCondition{{DurationHours: 3, SignificantWaveHeightM: 1.5, PeakPeriodSeconds: 6, DirectionFromDeg: 0}}}
	result, err := RunLongshoreCERC(points, climate, config)
	if err != nil {
		t.Fatalf("RunLongshoreCERC вернула ошибку: %v", err)
	}
	if len(result.Snapshots) != 2 || len(result.Steps) != 1 {
		t.Fatalf("неверное число состояний: снимков=%d, шагов=%d", len(result.Snapshots), len(result.Steps))
	}
	step := result.Steps[0]
	if math.Abs(step.MassBalanceM3) > 1e-6 {
		t.Fatalf("нарушен баланс наносов: %.12f м³", step.MassBalanceM3)
	}
	if math.Abs(step.ErodedVolumeM3-step.DepositedVolumeM3) > 1e-6 {
		t.Fatalf("размыв %.9f м³ не равен аккумуляции %.9f м³", step.ErodedVolumeM3, step.DepositedVolumeM3)
	}
}

// TestRunLongshoreCERCHigherWavesIncreaseTransport проверяет влияние Hs на
// поток CERC, которое отсутствовало в прежней эвристической модели.
func TestRunLongshoreCERCHigherWavesIncreaseTransport(t *testing.T) {
	points := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.005}, {Lat: 0.002, Lon: 0.01}, {Lat: 0, Lon: 0.015}}
	config := testLongshoreConfig(t)
	run := func(height float64) LongshoreModelResult {
		result, err := RunLongshoreCERC(points, WaveClimate{Source: "проверочный ряд", Conditions: []WaveCondition{{DurationHours: 3, SignificantWaveHeightM: height, PeakPeriodSeconds: 6, DirectionFromDeg: 0}}}, config)
		if err != nil {
			t.Fatalf("RunLongshoreCERC вернула ошибку: %v", err)
		}
		return result
	}
	low, high := run(0.7), run(1.4)
	if totalTransportMagnitude(high.Steps[0]) <= totalTransportMagnitude(low.Steps[0]) {
		t.Fatalf("более высокая волна должна усилить транспорт: %.9f <= %.9f", totalTransportMagnitude(high.Steps[0]), totalTransportMagnitude(low.Steps[0]))
	}
}

func testLongshoreConfig(t *testing.T) LongshoreModelConfig {
	t.Helper()
	points := make([]BathymetryPoint, 0, 25)
	for lat := -0.02; lat <= 0.02001; lat += 0.01 {
		for lon := -0.02; lon <= 0.02001; lon += 0.01 {
			points = append(points, BathymetryPoint{Lat: lat, Lon: lon, Depth: -12})
		}
	}
	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("BuildGrid вернула ошибку: %v", err)
	}
	return LongshoreModelConfig{Bathymetry: grid, BathymetrySource: "проверочная сетка", OffshoreSampleDistanceM: 10}
}

func totalTransportMagnitude(step LongshoreStepResult) float64 {
	total := 0.0
	for _, cell := range step.Cells {
		total += math.Abs(cell.LongshoreTransportM3S)
	}
	return total
}
