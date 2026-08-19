package benchmark

import "testing"

func TestSelectCrossValidationParametersExcludesHeldOutSite(t *testing.T) {
	pairA := calibrationParameterPair{strength: 1, waveDir: 0}
	pairB := calibrationParameterPair{strength: 2, waveDir: 90}
	sites := []calibratedCrossValidationSite{
		{
			site: BenchmarkSite{ID: "исключённый"},
			results: map[calibrationParameterPair]CalibrationResultItem{
				pairA: {ComparisonPoints: []ComparisonPoint{{Observed: 0, Modeled: 0, Uncertainty: 1}}},
				pairB: {ComparisonPoints: []ComparisonPoint{{Observed: 0, Modeled: 100, Uncertainty: 1}}},
			},
		},
		{
			site: BenchmarkSite{ID: "обучающий"},
			results: map[calibrationParameterPair]CalibrationResultItem{
				pairA: {ComparisonPoints: []ComparisonPoint{{Observed: 0, Modeled: 3, Uncertainty: 1}}},
				pairB: {ComparisonPoints: []ComparisonPoint{{Observed: 0, Modeled: 1, Uncertainty: 1}}},
			},
		},
	}

	got, metrics, err := selectCrossValidationParameters(sites, 0, []calibrationParameterPair{pairA, pairB})
	if err != nil {
		t.Fatalf("выбор параметров: %v", err)
	}
	if got != pairB {
		t.Errorf("выбрана комбинация %+v; требуется %+v, так как исключённый сайт не должен влиять на выбор", got, pairB)
	}
	if metrics.WeightedRMSE != 1 {
		t.Errorf("взвешенный RMSE обучения = %.1f; требуется 1", metrics.WeightedRMSE)
	}
}
