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

func TestNearestSegmentProjectionProjectsToLineNotVertex(t *testing.T) {
	coastline := []geometry.LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 1}, {Lat: 0, Lon: 2}}
	segment, position, distanceKm, ok := nearestSegmentProjection(coastline, geometry.LatLon{Lat: 0.01, Lon: 0.5})
	if !ok || segment != 0 {
		t.Fatalf("проекция: ok=%t, сегмент=%d; требуется сегмент 0", ok, segment)
	}
	if position < 0.49 || position > 0.51 {
		t.Errorf("положение проекции = %.3f, требуется около 0.5", position)
	}
	if distanceKm < 1 || distanceKm > 1.2 {
		t.Errorf("расстояние до линии = %.3f км, требуется около 1.11 км", distanceKm)
	}
}

func TestPrepareComparisonLocationsHonorsMaximumDistance(t *testing.T) {
	coastline := []geometry.LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.1}, {Lat: 0, Lon: 0.2}}
	observations := []ErosionObservation{
		{LatLon: geometry.LatLon{Lat: 0.001, Lon: 0.05}, ShorelineChangeRate: 1, Uncertainty: 0.2, StartDate: "2000-01-01", EndDate: "2001-01-01"},
		{LatLon: geometry.LatLon{Lat: 0.1, Lon: 0.05}, ShorelineChangeRate: 1, Uncertainty: 0.2, StartDate: "2000-01-01", EndDate: "2001-01-01"},
	}
	comparisons, summary := prepareComparisonLocations(coastline, observations, CalibrationConfig{MaxDistanceKm: 1, ValidationFraction: 0.25})
	if len(comparisons) != 1 || summary.ExcludedByDistance != 1 {
		t.Fatalf("принято %d, исключено %d; требуется 1 и 1", len(comparisons), summary.ExcludedByDistance)
	}
	if comparisons[0].DistanceToCoastKm > 1 {
		t.Errorf("допущено наблюдение в %.3f км при пределе 1 км", comparisons[0].DistanceToCoastKm)
	}
}

func TestPrepareComparisonLocationsRejectsInvalidObservationPeriod(t *testing.T) {
	coastline := []geometry.LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.1}, {Lat: 0, Lon: 0.2}}
	_, summary := prepareComparisonLocations(coastline, []ErosionObservation{{
		LatLon: geometry.LatLon{Lat: 0, Lon: 0.05}, StartDate: "2022-01-01", EndDate: "2021-01-01", Uncertainty: 0.2,
	}}, CalibrationConfig{MaxDistanceKm: 1})
	if summary.ExcludedInvalidPeriod != 1 || len(summary.Diagnostics) != 1 || summary.Diagnostics[0].Reason == "" {
		t.Errorf("неверная диагностика периода: %+v", summary)
	}
}

func TestPrepareComparisonLocationsRejectsMissingUncertainty(t *testing.T) {
	coastline := []geometry.LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.1}, {Lat: 0, Lon: 0.2}}
	_, summary := prepareComparisonLocations(coastline, []ErosionObservation{{
		LatLon: geometry.LatLon{Lat: 0, Lon: 0.05}, StartDate: "2020-01-01", EndDate: "2021-01-01", Uncertainty: 0,
	}}, CalibrationConfig{MaxDistanceKm: 1})
	if summary.ExcludedInvalidUncertainty != 1 || len(summary.Diagnostics) != 1 {
		t.Errorf("неверная диагностика неопределённости: %+v", summary)
	}
}

func TestObservationPeriodControlsSnapshotPosition(t *testing.T) {
	years, err := observationPeriodYears(ErosionObservation{StartDate: "2020-01-01", EndDate: "2021-07-02"})
	if err != nil || years < 1.49 || years > 1.51 {
		t.Fatalf("длительность = %.3f, ошибка = %v; требуется около 1.5 лет", years, err)
	}
	lower, upper, fraction := observationSnapshotPosition(1.5, 1, 3)
	if lower != 1 || upper != 2 || fraction != 0.5 {
		t.Errorf("положение снимка = %d, %d, %.3f; требуется 1, 2, 0.5", lower, upper, fraction)
	}
	steps := calibrationSteps([]MatchingDiagnostic{{Status: "принято", ObservationYears: 3.1}}, CalibrationConfig{YearsPerStep: 1})
	if steps != 4 {
		t.Errorf("шагов = %d, требуется 4", steps)
	}
}

func TestWeightedRMSEUsesObservationUncertainty(t *testing.T) {
	comparisons := []ComparisonPoint{
		{Observed: 0, Modeled: 1, Uncertainty: 0.1},
		{Observed: 0, Modeled: 5, Uncertainty: 10},
		{Observed: 0, Modeled: 5, Uncertainty: 10},
	}
	metrics := computeValidationMetrics(comparisons)
	if metrics.WeightedRMSE < 0.99 || metrics.WeightedRMSE > 1.01 {
		t.Errorf("взвешенный RMSE = %.3f, требуется около 1", metrics.WeightedRMSE)
	}
	if metrics.InferenceAllowed {
		t.Error("для трёх точек статистический вывод не должен быть доступен")
	}
}

func TestSmallValidationSampleDoesNotClaimSignificance(t *testing.T) {
	metrics := computeValidationMetricsWithTests([]ComparisonPoint{{Observed: 1, Modeled: 1}}, 128)
	if metrics.InferenceAllowed || metrics.Significant || metrics.PValue != 1 || metrics.AdjustedPValue != 1 {
		t.Errorf("малая выборка не должна иметь статистического вывода: %+v", metrics)
	}
}

func TestSpatialSplitKeepsTrainingAndValidationSeparate(t *testing.T) {
	if count := validationCount(4, 0.25); count != 1 {
		t.Fatalf("число проверочных точек = %d, требуется 1", count)
	}
	points := []ComparisonPoint{
		{CoastSegment: 0, SegmentPosition: 0.1},
		{CoastSegment: 1, SegmentPosition: 0.1},
		{CoastSegment: 2, SegmentPosition: 0.1},
		{CoastSegment: 3, SegmentPosition: 0.1},
	}
	for i := range points {
		points[i].Split = "training"
	}
	points[len(points)-1].Split = "validation"
	training, validation := splitComparisonPoints(points)
	if len(training) != 3 || len(validation) != 1 {
		t.Fatalf("разбиение: обучение=%d, проверка=%d; требуется 3/1", len(training), len(validation))
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
