package geometry

import (
	"encoding/json"
	"math"
	"strings"
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

func TestRunLongshoreCERCAccountsForBoundaryFlux(t *testing.T) {
	points := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.005}, {Lat: 0.002, Lon: 0.01}, {Lat: 0, Lon: 0.015}}
	config := testLongshoreConfig(t)
	config.LeftBoundaryTransportM3S = 0.01
	config.RightBoundaryTransportM3S = 0.002
	climate := WaveClimate{Source: "проверочный ряд", Conditions: []WaveCondition{{DurationHours: 1, SignificantWaveHeightM: 1.5, PeakPeriodSeconds: 6, DirectionFromDeg: 0}}}
	result, err := RunLongshoreCERC(points, climate, config)
	if err != nil {
		t.Fatalf("RunLongshoreCERC вернула ошибку: %v", err)
	}
	step := result.Steps[0]
	expected := (config.LeftBoundaryTransportM3S - config.RightBoundaryTransportM3S) * secondsPerHour
	if math.Abs(step.BoundaryNetVolumeM3-expected) > 1e-9 {
		t.Fatalf("граничный объём %.9f, ожидалось %.9f", step.BoundaryNetVolumeM3, expected)
	}
	if math.Abs(step.BalanceClosureResidualM3) > 1e-6 {
		t.Fatalf("невязка после учёта границ: %.12f м³", step.BalanceClosureResidualM3)
	}
}

func TestRunLongshoreCERCReportsInputQuality(t *testing.T) {
	points := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.005}, {Lat: 0.002, Lon: 0.01}, {Lat: 0, Lon: 0.015}}
	config := testLongshoreConfig(t)
	config.BathymetrySHA256 = "abc123"
	config.BathymetryPassport = "bathymetry.metadata.json"
	config.BathymetryStatus = BathymetryStatusVerifiedDerived
	result, err := RunLongshoreCERC(points, WaveClimate{Source: "проверочный ряд", Conditions: []WaveCondition{{DurationHours: 3, SignificantWaveHeightM: 1.5, PeakPeriodSeconds: 6, DirectionFromDeg: 0}}}, config)
	if err != nil {
		t.Fatalf("RunLongshoreCERC вернула ошибку: %v", err)
	}
	if result.InputQuality.WaveClimate.TotalDurationHours != 3 {
		t.Fatalf("неверная длительность ряда: %.2f", result.InputQuality.WaveClimate.TotalDurationHours)
	}
	if result.InputQuality.Bathymetry.SampleCount == 0 {
		t.Fatal("должны быть учтены использованные батиметрические выборки")
	}
	if result.BathymetrySHA256 != config.BathymetrySHA256 || result.BathymetryPassport != config.BathymetryPassport || result.BathymetryStatus != config.BathymetryStatus {
		t.Fatalf("происхождение батиметрии не перенесено в результат: %+v", result)
	}
}

// TestRunLongshoreCERCCarriesScenarioClassification проверяет, что статус
// интерпретации не теряется при успешном численном расчёте и сериализации.
func TestRunLongshoreCERCCarriesScenarioClassification(t *testing.T) {
	points := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.005}, {Lat: 0.002, Lon: 0.01}, {Lat: 0, Lon: 0.015}}
	config := testLongshoreConfig(t)
	config.Scenario = ScenarioClassification{
		ScenarioStatus:   ScenarioStatusDemo,
		UsageLimitations: []string{"не для публикации"},
	}
	climate := WaveClimate{Source: "проверочный ряд", Conditions: []WaveCondition{{DurationHours: 1, SignificantWaveHeightM: 1.5, PeakPeriodSeconds: 6, DirectionFromDeg: 0}}}

	result, err := RunLongshoreCERC(points, climate, config)
	if err != nil {
		t.Fatalf("RunLongshoreCERC вернула ошибку: %v", err)
	}
	if result.ScenarioStatus != ScenarioStatusDemo || len(result.UsageLimitations) != 1 {
		t.Fatalf("классификация сценария не перенесена: %+v", result.ScenarioClassification)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("сериализация результата: %v", err)
	}
	if !strings.Contains(string(data), `"scenario_status":"demo"`) {
		t.Fatalf("JSON не содержит статус demo: %s", data)
	}

	config.Scenario = ScenarioClassification{}
	result, err = RunLongshoreCERC(points, climate, config)
	if err != nil {
		t.Fatalf("RunLongshoreCERC без классификации вернула ошибку: %v", err)
	}
	if result.ScenarioStatus != ScenarioStatusUnclassified {
		t.Fatalf("статус по умолчанию = %q, требуется %q", result.ScenarioStatus, ScenarioStatusUnclassified)
	}
}

func TestRunLongshoreCERCAccountsForSedimentSource(t *testing.T) {
	points := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.005}, {Lat: 0.002, Lon: 0.01}, {Lat: 0, Lon: 0.015}}
	config := testLongshoreConfig(t)
	config.SedimentSources = []LongshoreSedimentSource{{PointIndex: 1, SourceRateM3S: 0.01, Description: "питание", DataSource: "проверочное измерение"}}
	climate := WaveClimate{Source: "проверочный ряд", Conditions: []WaveCondition{{DurationHours: 1, SignificantWaveHeightM: 1.5, PeakPeriodSeconds: 6, DirectionFromDeg: 0}}}
	result, err := RunLongshoreCERC(points, climate, config)
	if err != nil {
		t.Fatalf("RunLongshoreCERC вернула ошибку: %v", err)
	}
	step := result.Steps[0]
	if math.Abs(step.ExternalSedimentVolumeM3-36) > 1e-9 {
		t.Fatalf("внешний объём %.9f, ожидалось 36", step.ExternalSedimentVolumeM3)
	}
	if math.Abs(step.BalanceClosureResidualM3) > 1e-6 {
		t.Fatalf("невязка с источником: %.12f м³", step.BalanceClosureResidualM3)
	}
	if step.Cells[1].ExternalSedimentVolumeM3 != 36 {
		t.Fatalf("ячейка источника получила %.9f м³", step.Cells[1].ExternalSedimentVolumeM3)
	}
}

func TestRunLongshoreCERCReducesFluxAtStructure(t *testing.T) {
	points := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.005}, {Lat: 0.002, Lon: 0.01}, {Lat: 0, Lon: 0.015}}
	climate := WaveClimate{Source: "проверочный ряд", Conditions: []WaveCondition{{DurationHours: 1, SignificantWaveHeightM: 1.5, PeakPeriodSeconds: 6, DirectionFromDeg: 0}}}
	without, err := RunLongshoreCERC(points, climate, testLongshoreConfig(t))
	if err != nil {
		t.Fatalf("расчёт без сооружения: %v", err)
	}
	config := testLongshoreConfig(t)
	config.Structures = []LongshoreStructure{{LeftPointIndex: 1, TransmissionCoefficient: 0, Kind: "буна", Description: "проверочная буна", DataSource: "проверочное обследование"}}
	with, err := RunLongshoreCERC(points, climate, config)
	if err != nil {
		t.Fatalf("расчёт с сооружением: %v", err)
	}
	if math.Abs(with.Steps[0].Cells[1].RightFaceTransportM3S) > 1e-12 {
		t.Fatalf("поток через непроницаемую буна равен %.12f", with.Steps[0].Cells[1].RightFaceTransportM3S)
	}
	if math.Abs(without.Steps[0].Cells[1].RightFaceTransportM3S) < 1e-12 {
		t.Fatal("проверочный поток без сооружения должен быть ненулевым")
	}
	if math.Abs(with.Steps[0].BalanceClosureResidualM3) > 1e-6 {
		t.Fatalf("сооружение не должно нарушать баланс: %.12f", with.Steps[0].BalanceClosureResidualM3)
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
