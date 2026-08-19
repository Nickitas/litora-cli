package benchmark

import (
	"fmt"
	"sort"
)

// CrossValidationFold содержит результат одного leave-one-site-out прогона.
// Параметры выбраны только по всем сайтам, кроме HeldOutSiteID.
type CrossValidationFold struct {
	HeldOutSiteID   string            `json:"held_out_site_id"`
	TrainingSiteIDs []string          `json:"training_site_ids"`
	ErosionStrength float64           `json:"erosion_strength"`
	WaveDirection   float64           `json:"wave_direction"`
	TrainingMetrics ValidationMetrics `json:"training_metrics"`
	ExternalMetrics ValidationMetrics `json:"external_metrics"`
}

// SkippedCrossValidationSite объясняет, почему эталон не вошёл в проверку.
type SkippedCrossValidationSite struct {
	SiteID string `json:"site_id"`
	Reason string `json:"reason"`
}

// CrossSiteValidationResult представляет leave-one-site-out проверку. Каждый
// сайт ровно один раз служит независимой внешней выборкой.
type CrossSiteValidationResult struct {
	Folds                 []CrossValidationFold        `json:"folds"`
	PooledExternalMetrics ValidationMetrics            `json:"pooled_external_metrics"`
	SkippedSites          []SkippedCrossValidationSite `json:"skipped_sites,omitempty"`
	Warnings              []string                     `json:"warnings,omitempty"`
}

type calibrationParameterPair struct {
	strength float64
	waveDir  float64
}

type calibratedCrossValidationSite struct {
	site    BenchmarkSite
	results map[calibrationParameterPair]CalibrationResultItem
}

// CrossValidateSites подбирает общую пару параметров на N-1 эталонах и
// проверяет её на исключённом сайте. В отличие от локальной калибровки,
// наблюдения проверяемого сайта не участвуют ни в выборе силы, ни в выборе
// направления волны.
func CrossValidateSites(sites []BenchmarkSite, config CalibrationConfig) (CrossSiteValidationResult, error) {
	if len(sites) < 2 {
		return CrossSiteValidationResult{}, fmt.Errorf("для межсайтовой проверки нужны как минимум два эталона")
	}

	usable := make([]calibratedCrossValidationSite, 0, len(sites))
	result := CrossSiteValidationResult{}
	for _, site := range sites {
		if len(site.ObservedErosion) == 0 {
			result.SkippedSites = append(result.SkippedSites, SkippedCrossValidationSite{SiteID: site.ID, Reason: "нет наблюдений эрозии"})
			continue
		}
		items, err := Calibrate(site, config)
		if err != nil {
			result.SkippedSites = append(result.SkippedSites, SkippedCrossValidationSite{SiteID: site.ID, Reason: err.Error()})
			continue
		}
		byParameter := make(map[calibrationParameterPair]CalibrationResultItem, len(items))
		for _, item := range items {
			byParameter[calibrationParameterPair{strength: item.ErosionStrength, waveDir: item.WaveDirection}] = item
		}
		usable = append(usable, calibratedCrossValidationSite{site: site, results: byParameter})
	}
	if len(usable) < 2 {
		return result, fmt.Errorf("после контроля качества осталось меньше двух пригодных эталонов")
	}

	pairs := parameterPairs(config)
	var pooledExternal []ComparisonPoint
	for heldOutIndex, heldOut := range usable {
		bestPair, trainingMetrics, err := selectCrossValidationParameters(usable, heldOutIndex, pairs)
		if err != nil {
			return result, err
		}
		externalItem := heldOut.results[bestPair]
		externalMetrics := computeValidationMetricsWithTests(externalItem.ComparisonPoints, len(pairs))
		trainingIDs := make([]string, 0, len(usable)-1)
		for index, candidate := range usable {
			if index != heldOutIndex {
				trainingIDs = append(trainingIDs, candidate.site.ID)
			}
		}
		result.Folds = append(result.Folds, CrossValidationFold{
			HeldOutSiteID: heldOut.site.ID, TrainingSiteIDs: trainingIDs,
			ErosionStrength: bestPair.strength, WaveDirection: bestPair.waveDir,
			TrainingMetrics: trainingMetrics, ExternalMetrics: externalMetrics,
		})
		pooledExternal = append(pooledExternal, externalItem.ComparisonPoints...)
	}
	sort.Slice(result.Folds, func(i, j int) bool { return result.Folds[i].HeldOutSiteID < result.Folds[j].HeldOutSiteID })
	result.PooledExternalMetrics = computeValidationMetricsWithTests(pooledExternal, len(pairs))
	result.Warnings = append(result.Warnings,
		"межсайтовая проверка оценивает переносимость исторической эвристики, а не заменяет физическую валидацию CERC-модели")
	if result.PooledExternalMetrics.N < 10 {
		result.Warnings = append(result.Warnings, "во внешней проверке меньше десяти наблюдений: метрики следует трактовать описательно")
	}
	return result, nil
}

func parameterPairs(config CalibrationConfig) []calibrationParameterPair {
	pairs := make([]calibrationParameterPair, 0, len(config.ErosionStrengths)*len(config.WaveDirections))
	for _, strength := range config.ErosionStrengths {
		for _, waveDir := range config.WaveDirections {
			pairs = append(pairs, calibrationParameterPair{strength: strength, waveDir: waveDir})
		}
	}
	return pairs
}

func selectCrossValidationParameters(sites []calibratedCrossValidationSite, heldOutIndex int, pairs []calibrationParameterPair) (calibrationParameterPair, ValidationMetrics, error) {
	if heldOutIndex < 0 || heldOutIndex >= len(sites) {
		return calibrationParameterPair{}, ValidationMetrics{}, fmt.Errorf("некорректный индекс исключённого эталона")
	}
	var bestPair calibrationParameterPair
	var bestMetrics ValidationMetrics
	found := false
	for _, pair := range pairs {
		var comparisons []ComparisonPoint
		for index, site := range sites {
			if index == heldOutIndex {
				continue
			}
			item, ok := site.results[pair]
			if !ok {
				return calibrationParameterPair{}, ValidationMetrics{}, fmt.Errorf("на эталоне %q нет результата для комбинации параметров", site.site.ID)
			}
			// В межсайтовой проверке разрешено использовать все пригодные точки
			// обучающих сайтов: внешний сайт остаётся полностью независимым.
			comparisons = append(comparisons, item.ComparisonPoints...)
		}
		metrics := computeValidationMetricsWithTests(comparisons, len(pairs))
		if !found || metrics.WeightedRMSE < bestMetrics.WeightedRMSE {
			bestPair, bestMetrics, found = pair, metrics, true
		}
	}
	if !found {
		return calibrationParameterPair{}, ValidationMetrics{}, fmt.Errorf("нет комбинаций параметров для межсайтового выбора")
	}
	return bestPair, bestMetrics, nil
}
