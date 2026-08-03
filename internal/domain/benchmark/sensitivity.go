package benchmark

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"coastal-geometry/internal/domain/geometry"
)

// SensitivityResult описывает, как параметр влияет на выход модели
type SensitivityResult struct {
	Parameter        string    `json:"parameter"`  // например "erosion_strength"
	Values           []float64 `json:"values"`     // протестированные значения параметра
	RMSE             []float64 `json:"rmse"`       // RMSE для каждого значения
	MAE              []float64 `json:"mae"`        // MAE для каждого значения
	RSquared         []float64 `json:"r_squared"`  // R² для каждого значения
	BestValue        float64   `json:"best_value"` // значение с наименьшим RMSE
	BestRMSE         float64   `json:"best_rmse"`
	WorstValue       float64   `json:"worst_value"` // значение с наибольшим RMSE
	WorstRMSE        float64   `json:"worst_rmse"`
	SensitivityScore float64   `json:"sensitivity_score"` // (max-min)/max нормализовано
	LocalSensitivity float64   `json:"local_sensitivity"` // производная в лучшей точке
}

// ConfidenceInterval представляет доверительный интервал параметра из бутстрепа
type ConfidenceInterval struct {
	Parameter string  `json:"parameter"`
	Mean      float64 `json:"mean"`
	StdDev    float64 `json:"std_dev"`
	Median    float64 `json:"median"`
	Lower95   float64 `json:"lower_95"` // 2.5 процентиль
	Upper95   float64 `json:"upper_95"` // 97.5 процентиль
	Lower68   float64 `json:"lower_68"` // 16 процентиль (~1 сигма)
	Upper68   float64 `json:"upper_68"` // 84 процентиль
	BestFit   float64 `json:"best_fit"` // исходная лучшая подгонка (без ресемплинга)
}

// NullModelComparison сравнивает модель с простыми базовыми линиями
type NullModelComparison struct {
	ModelRMSE       float64 `json:"model_rmse"`
	MeanModelRMSE   float64 `json:"mean_model_rmse"`   // предсказывать среднее наблюдаемое
	LinearTrendRMSE float64 `json:"linear_trend_rmse"` // предсказывать линейный тренд (одно значение)
	SkillScore      float64 `json:"skill_score"`       // (null - model) / null, 1=идеально, 0=как null
	Improvement     float64 `json:"improvement_pct"`   // % улучшения по сравнению с null моделью
}

// FullAnalysis объединяет калибровку, чувствительность, CI и сравнение с null моделью
type FullAnalysis struct {
	BestFit         CalibrationResultItem `json:"best_fit"`
	Sensitivities   []SensitivityResult   `json:"sensitivities"`
	StrengthCI      ConfidenceInterval    `json:"strength_ci"`
	WaveDirectionCI ConfidenceInterval    `json:"wave_direction_ci"`
	NullModel       NullModelComparison   `json:"null_model"`
}

// sensitivityIteration represents a single parameter test iteration
type sensitivityIteration struct {
	index   int
	value   float64
	lat     float64 // For wave direction sensitivity
	lon     float64 // For wave direction sensitivity
	steps   int
	config  CalibrationConfig
	site    BenchmarkSite
	latGrid *geometry.BathymetryGrid
}

// sensitivityResultData holds iteration results
type sensitivityResultData struct {
	index   int
	rmse    float64
	mae     float64
	rSq     float64
	epsilon error // For error propagation
}

// AnalyzeSensitivity выполняет однопараметрический анализ чувствительности
// Для каждого параметра удерживает остальные на лучшей подгонке и варьирует этот
func AnalyzeSensitivity(site BenchmarkSite, config CalibrationConfig, bestFit CalibrationResultItem) []SensitivityResult {
	var results []SensitivityResult

	// 1. Sensitivity to erosion_strength (hold wave_dir at best)
	strengthSens := sensitivityToStrength(site, config, bestFit)
	results = append(results, strengthSens)

	// 2. Sensitivity to wave_direction (hold strength at best)
	dirSens := sensitivityToWaveDirection(site, config, bestFit)
	results = append(results, dirSens)

	return results
}

func sensitivityToStrength(site BenchmarkSite, config CalibrationConfig, bestFit CalibrationResultItem) SensitivityResult {
	// Тестируем диапазон вокруг лучшей подгонки
	values := generateTestRange(bestFit.ErosionStrength, 0.1, 3.0, 12)

	rmse := make([]float64, len(values))
	mae := make([]float64, len(values))
	rSq := make([]float64, len(values))

	steps := int(float64(config.TotalYears) / config.YearsPerStep)
	if steps < 1 {
		steps = 1
	}

	for i, v := range values {
		item := runCalibrationIteration(site, config, v, bestFit.WaveDirection, steps)
		rmse[i] = item.ValidationMetrics.RMSE
		mae[i] = item.ValidationMetrics.MAE
		rSq[i] = item.ValidationMetrics.RSquared
	}

	return summarizeSensitivity("erosion_strength_m", values, rmse, mae, rSq)
}

func sensitivityToWaveDirection(site BenchmarkSite, config CalibrationConfig, bestFit CalibrationResultItem) SensitivityResult {
	values := generateTestRange(bestFit.WaveDirection, 0.1, 3.0, 12)
	// Wrap to [0, 360)
	for i, v := range values {
		values[i] = math.Mod(math.Mod(v, 360)+360, 360)
	}

	rmse := make([]float64, len(values))
	mae := make([]float64, len(values))
	rSq := make([]float64, len(values))

	steps := int(float64(config.TotalYears) / config.YearsPerStep)
	if steps < 1 {
		steps = 1
	}

	for i, v := range values {
		item := runCalibrationIteration(site, config, bestFit.ErosionStrength, v, steps)
		rmse[i] = item.ValidationMetrics.RMSE
		mae[i] = item.ValidationMetrics.MAE
		rSq[i] = item.ValidationMetrics.RSquared
	}

	return summarizeSensitivity("wave_direction_deg", values, rmse, mae, rSq)
}

// generateTestRange генерирует значения вокруг center от (center/factor) до (center*factor)
func generateTestRange(center, minFactor, maxFactor float64, n int) []float64 {
	if center <= 0 || n < 2 {
		return []float64{center}
	}
	values := make([]float64, n)
	step := (maxFactor - minFactor) / float64(n-1)
	for i := 0; i < n; i++ {
		factor := minFactor + step*float64(i)
		values[i] = center * factor
	}
	return values
}

func summarizeSensitivity(name string, values, rmse, mae, rSq []float64) SensitivityResult {
	if len(values) == 0 {
		return SensitivityResult{Parameter: name}
	}

	bestIdx := 0
	worstIdx := 0
	for i := range values {
		if rmse[i] < rmse[bestIdx] {
			bestIdx = i
		}
		if rmse[i] > rmse[worstIdx] {
			worstIdx = i
		}
	}

	minRMSE := rmse[bestIdx]
	maxRMSE := rmse[worstIdx]
	sensScore := 0.0
	if maxRMSE > 0 {
		sensScore = (maxRMSE - minRMSE) / maxRMSE
	}

	// Локальная чувствительность: производная в лучшей точке
	// Используем прямую/обратную/центральную разность
	localSens := 0.0
	if bestIdx > 0 && bestIdx < len(values)-1 {
		dVal := values[bestIdx+1] - values[bestIdx-1]
		if dVal != 0 {
			localSens = (rmse[bestIdx+1] - rmse[bestIdx-1]) / dVal
		}
	}

	return SensitivityResult{
		Parameter:        name,
		Values:           values,
		RMSE:             rmse,
		MAE:              mae,
		RSquared:         rSq,
		BestValue:        values[bestIdx],
		BestRMSE:         rmse[bestIdx],
		WorstValue:       values[worstIdx],
		WorstRMSE:        rmse[worstIdx],
		SensitivityScore: sensScore,
		LocalSensitivity: localSens,
	}
}

// BootstrapConfidenceIntervals оценивает доверительные интервалы для параметров лучшей подгонки
// путём запуска калибровки на ресемплированных наборах наблюдений
func BootstrapConfidenceIntervals(
	site BenchmarkSite,
	config CalibrationConfig,
	bootstrapIterations int,
	rng func() float64,
) (ConfidenceInterval, ConfidenceInterval) {
	if bootstrapIterations < 1 {
		bootstrapIterations = 200
	}
	if rng == nil {
		rng = mathRandFloat
	}

	// Получаем исходную лучшую подгонку
	origResults, _ := Calibrate(site, config)
	origBest := origResults[0]

	// Хранилище для параметров лучшей подгонки бутстрепа
	strengths := make([]float64, 0, bootstrapIterations)
	directions := make([]float64, 0, bootstrapIterations)

	nObs := len(site.ObservedErosion)

	for iter := 0; iter < bootstrapIterations; iter++ {
		// Ресемплируем наблюдения с возвращением
		resampled := resampleObservations(site.ObservedErosion, nObs, rng)
		if len(resampled) == 0 {
			continue
		}

		// Создаём временный участок с ресемплированными наблюдениями
		tempSite := site
		tempSite.ObservedErosion = resampled

		// Запускаем калибровку
		results, err := Calibrate(tempSite, config)
		if err != nil || len(results) == 0 {
			continue
		}

		strengths = append(strengths, results[0].ErosionStrength)
		directions = append(directions, results[0].WaveDirection)
	}

	strengthCI := computeCI("erosion_strength_m", strengths, origBest.ErosionStrength)
	dirCI := computeCIDirectional("wave_direction_deg", directions, origBest.WaveDirection)

	return strengthCI, dirCI
}

// computeCI вычисляет доверительный интервал для скалярных значений
func computeCI(name string, values []float64, bestFit float64) ConfidenceInterval {
	if len(values) == 0 {
		return ConfidenceInterval{Parameter: name, BestFit: bestFit}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	mean := meanOf(values)
	stdDev := stdDevOf(values, mean)
	median := percentile(sorted, 0.5)
	lower95 := percentile(sorted, 0.025)
	upper95 := percentile(sorted, 0.975)
	lower68 := percentile(sorted, 0.16)
	upper68 := percentile(sorted, 0.84)

	return ConfidenceInterval{
		Parameter: name,
		Mean:      mean,
		StdDev:    stdDev,
		Median:    median,
		Lower95:   lower95,
		Upper95:   upper95,
		Lower68:   lower68,
		Upper68:   upper68,
		BestFit:   bestFit,
	}
}

// computeCIDirectional обрабатывает угловые данные (направление волны)
// путём вычисления доверительного интервала на исходных значениях, когда распределение унимодально
func computeCIDirectional(name string, values []float64, bestFit float64) ConfidenceInterval {
	ci := computeCI(name, values, bestFit)
	// Заворачиваем значения в [0, 360)
	ci.Median = math.Mod(math.Mod(ci.Median, 360)+360, 360)
	ci.Lower95 = math.Mod(math.Mod(ci.Lower95, 360)+360, 360)
	ci.Upper95 = math.Mod(math.Mod(ci.Upper95, 360)+360, 360)
	ci.Lower68 = math.Mod(math.Mod(ci.Lower68, 360)+360, 360)
	ci.Upper68 = math.Mod(math.Mod(ci.Upper68, 360)+360, 360)
	ci.BestFit = math.Mod(math.Mod(ci.BestFit, 360)+360, 360)
	return ci
}

// resampleObservations выполняет бутстреп-ресемплинг
func resampleObservations(obs []ErosionObservation, n int, rng func() float64) []ErosionObservation {
	if len(obs) == 0 {
		return nil
	}
	resampled := make([]ErosionObservation, n)
	for i := 0; i < n; i++ {
		idx := int(rng() * float64(len(obs)))
		if idx >= len(obs) {
			idx = len(obs) - 1
		}
		resampled[i] = obs[idx]
	}
	return resampled
}

// CompareWithNullModel сравнивает производительность модели со средней базовой линией
func CompareWithNullModel(observed, modeled []float64) NullModelComparison {
	if len(observed) == 0 || len(modeled) == 0 {
		return NullModelComparison{}
	}

	meanObs := meanOf(observed)

	// RMSE модели
	modelRMSE := rmseBetween(observed, modeled)

	// RMSE средней модели (предсказываем среднее для всех)
	meanModelRMSE := rmseConstant(observed, meanObs)

	// RMSE линейного тренда (одна лучшая подгонка постоянной для простоты)
	// Здесь нет независимой переменной, поэтому используем среднее как простейший тренд
	linearTrendRMSE := meanModelRMSE

	// Оценка навыка: (null - model) / null
	// 1 = идеально, 0 = как null, отрицательное = хуже null
	skillScore := 0.0
	if meanModelRMSE > 0 {
		skillScore = (meanModelRMSE - modelRMSE) / meanModelRMSE
	}

	improvement := 0.0
	if meanModelRMSE > 0 {
		improvement = (meanModelRMSE - modelRMSE) / meanModelRMSE * 100
	}

	return NullModelComparison{
		ModelRMSE:       modelRMSE,
		MeanModelRMSE:   meanModelRMSE,
		LinearTrendRMSE: linearTrendRMSE,
		SkillScore:      skillScore,
		Improvement:     improvement,
	}
}

// RunFullAnalysis выполняет калибровку, анализ чувствительности, CI, сравнение с null моделью
func RunFullAnalysis(site BenchmarkSite, config CalibrationConfig, bootstrapIter int) (FullAnalysis, error) {
	// 1. Получаем лучшую подгонку
	results, err := Calibrate(site, config)
	if err != nil {
		return FullAnalysis{}, err
	}
	if len(results) == 0 {
		return FullAnalysis{}, fmt.Errorf("no calibration results")
	}
	bestFit := results[0]

	// 2. Анализ чувствительности
	sensitivities := AnalyzeSensitivity(site, config, bestFit)

	// 3. Доверительные интервалы бутстрепа
	strengthCI, dirCI := BootstrapConfidenceIntervals(site, config, bootstrapIter, nil)

	// 4. Сравнение с null моделью
	var observed, modeled []float64
	for _, c := range bestFit.ComparisonPoints {
		observed = append(observed, c.Observed)
		modeled = append(modeled, c.Modeled)
	}
	nullModel := CompareWithNullModel(observed, modeled)

	return FullAnalysis{
		BestFit:         bestFit,
		Sensitivities:   sensitivities,
		StrengthCI:      strengthCI,
		WaveDirectionCI: dirCI,
		NullModel:       nullModel,
	}, nil
}

// RunFullAnalysisWithBathymetry - то же, что RunFullAnalysis, но с батиметрией
func RunFullAnalysisWithBathymetry(site BenchmarkSite, config CalibrationConfig, bathymetry *geometry.BathymetryGrid, bootstrapIter int) (FullAnalysis, error) {
	// 1. Получаем лучшую подгонку
	results, err := CalibrateWithBathymetry(site, config, bathymetry)
	if err != nil {
		return FullAnalysis{}, err
	}
	if len(results) == 0 {
		return FullAnalysis{}, fmt.Errorf("no calibration results")
	}
	bestFit := results[0]

	// 2. Анализ чувствительности (с батиметрией)
	sensitivities := analyzeSensitivityWithBathymetry(site, config, bestFit, bathymetry)

	// 3. Доверительные интервалы бутстрепа
	strengthCI, dirCI := bootstrapWithBathymetry(site, config, bathymetry, bootstrapIter)

	// 4. Сравнение с null моделью
	var observed, modeled []float64
	for _, c := range bestFit.ComparisonPoints {
		observed = append(observed, c.Observed)
		modeled = append(modeled, c.Modeled)
	}
	nullModel := CompareWithNullModel(observed, modeled)

	return FullAnalysis{
		BestFit:         bestFit,
		Sensitivities:   sensitivities,
		StrengthCI:      strengthCI,
		WaveDirectionCI: dirCI,
		NullModel:       nullModel,
	}, nil
}

func analyzeSensitivityWithBathymetry(site BenchmarkSite, config CalibrationConfig, bestFit CalibrationResultItem, bathymetry *geometry.BathymetryGrid) []SensitivityResult {
	config.BathymetryGrid = bathymetry
	return AnalyzeSensitivity(site, config, bestFit)
}

func bootstrapWithBathymetry(site BenchmarkSite, config CalibrationConfig, bathymetry *geometry.BathymetryGrid, iter int) (ConfidenceInterval, ConfidenceInterval) {
	config.BathymetryGrid = bathymetry

	// Get original best fit
	origResults, _ := CalibrateWithBathymetry(site, config, bathymetry)
	if len(origResults) == 0 {
		return ConfidenceInterval{Parameter: "erosion_strength_m"}, ConfidenceInterval{Parameter: "wave_direction_deg"}
	}
	origBest := origResults[0]

	strengths := make([]float64, 0, iter)
	directions := make([]float64, 0, iter)

	nObs := len(site.ObservedErosion)

	for i := 0; i < iter; i++ {
		resampled := resampleObservations(site.ObservedErosion, nObs, mathRandFloat)
		if len(resampled) == 0 {
			continue
		}
		tempSite := site
		tempSite.ObservedErosion = resampled

		results, err := CalibrateWithBathymetry(tempSite, config, bathymetry)
		if err != nil || len(results) == 0 {
			continue
		}
		strengths = append(strengths, results[0].ErosionStrength)
		directions = append(directions, results[0].WaveDirection)
	}

	return computeCI("erosion_strength_m", strengths, origBest.ErosionStrength),
		computeCIDirectional("wave_direction_deg", directions, origBest.WaveDirection)
}

// Math helpers
func meanOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stdDevOf(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var sumSq float64
	for _, v := range values {
		sumSq += (v - mean) * (v - mean)
	}
	return math.Sqrt(sumSq / float64(len(values)-1))
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

func rmseBetween(observed, modeled []float64) float64 {
	if len(observed) == 0 {
		return 0
	}
	var sumSq float64
	for i := range observed {
		diff := modeled[i] - observed[i]
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(observed)))
}

func rmseConstant(observed []float64, constant float64) float64 {
	if len(observed) == 0 {
		return 0
	}
	var sumSq float64
	for _, o := range observed {
		diff := constant - o
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(observed)))
}

// mathRandFloat returns a uniform random in [0, 1) using math/rand
func mathRandFloat() float64 {
	// Use a deterministic seedless source; production should pass a real rng
	return pseudoRandom()
}

// pseudoRandom provides a simple LCG for bootstrap when no RNG supplied
var prngState uint64 = 42

func pseudoRandom() float64 {
	prngState = prngState*6364136223846793005 + 1442695040888963407
	return float64(prngState>>11) / float64(1<<53)
}

// ========== Параллельные оптимизированные версии ==========

// sensitivityToStrengthParallel выполняет анализ чувствительности силы с параллельными итерациями
func sensitivityToStrengthParallel(ctx context.Context, site BenchmarkSite, config CalibrationConfig, bestFit CalibrationResultItem, bathymetry *geometry.BathymetryGrid) SensitivityResult {
	values := generateTestRange(bestFit.ErosionStrength, 0.1, 3.0, 12)

	rmse := make([]float64, len(values))
	mae := make([]float64, len(values))
	rSq := make([]float64, len(values))

	steps := int(float64(config.TotalYears) / config.YearsPerStep)
	if steps < 1 {
		steps = 1
	}

	results := make(chan sensitivityResultData, len(values))
	numWorkers := sensitivityMin(runtime.NumCPU(), 4)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		startIdx := w
		wg.Add(1)
		go func(workerID, start int) {
			defer wg.Done()

			for i := start; i < len(values); i += numWorkers {
				select {
				case <-ctx.Done():
					results <- sensitivityResultData{
						index:   i,
						epsilon: ctx.Err(),
					}
					return
				default:
				}

				var item CalibrationResultItem

				if bathymetry != nil {
					item = runCalibrationIterationWithBathymetry(site, config, values[i], bestFit.WaveDirection, steps, bathymetry)
				} else {
					item = runCalibrationIteration(site, config, values[i], bestFit.WaveDirection, steps)
				}

				results <- sensitivityResultData{
					index: i,
					rmse:  item.ValidationMetrics.RMSE,
					mae:   item.ValidationMetrics.MAE,
					rSq:   item.ValidationMetrics.RSquared,
				}
			}
		}(w, startIdx)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	successCount := 0
	for res := range results {
		if res.epsilon != nil {
			continue
		}
		rmse[res.index] = res.rmse
		mae[res.index] = res.mae
		rSq[res.index] = res.rSq
		successCount++
	}

	if successCount < len(values)/2 {
		return SensitivityResult{Parameter: "erosion_strength_m"}
	}

	return summarizeSensitivity("erosion_strength_m", values, rmse, mae, rSq)
}

// sensitivityToWaveDirectionParallel выполняет анализ чувствительности направления волн с параллельными итерациями
func sensitivityToWaveDirectionParallel(ctx context.Context, site BenchmarkSite, config CalibrationConfig, bestFit CalibrationResultItem, bathymetry *geometry.BathymetryGrid) SensitivityResult {
	values := generateTestRange(bestFit.WaveDirection, 0.1, 3.0, 12)

	// Wrap to [0, 360)
	for i := range values {
		values[i] = math.Mod(math.Mod(values[i], 360)+360, 360)
	}

	rmse := make([]float64, len(values))
	mae := make([]float64, len(values))
	rSq := make([]float64, len(values))

	steps := int(float64(config.TotalYears) / config.YearsPerStep)
	if steps < 1 {
		steps = 1
	}

	results := make(chan sensitivityResultData, len(values))
	numWorkers := sensitivityMin(runtime.NumCPU(), 4)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		startIdx := w
		wg.Add(1)
		go func(workerID, start int) {
			defer wg.Done()

			for i := start; i < len(values); i += numWorkers {
				select {
				case <-ctx.Done():
					results <- sensitivityResultData{
						index:   i,
						epsilon: ctx.Err(),
					}
					return
				default:
				}

				var item CalibrationResultItem

				if bathymetry != nil {
					item = runCalibrationIterationWithBathymetry(site, config, bestFit.ErosionStrength, values[i], steps, bathymetry)
				} else {
					item = runCalibrationIteration(site, config, bestFit.ErosionStrength, values[i], steps)
				}

				results <- sensitivityResultData{
					index: i,
					rmse:  item.ValidationMetrics.RMSE,
					mae:   item.ValidationMetrics.MAE,
					rSq:   item.ValidationMetrics.RSquared,
				}
			}
		}(w, startIdx)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	successCount := 0
	for res := range results {
		if res.epsilon != nil {
			continue
		}
		rmse[res.index] = res.rmse
		mae[res.index] = res.mae
		rSq[res.index] = res.rSq
		successCount++
	}

	if successCount < len(values)/2 {
		return SensitivityResult{Parameter: "wave_direction_deg"}
	}

	return summarizeSensitivity("wave_direction_deg", values, rmse, mae, rSq)
}

// AnalyzeSensitivityParallel выполняет параллельный анализ чувствительности для обоих параметров
func AnalyzeSensitivityParallel(ctx context.Context, site BenchmarkSite, config CalibrationConfig, bestFit CalibrationResultItem, bathymetry *geometry.BathymetryGrid) []SensitivityResult {
	var results []SensitivityResult

	// Используем каналы для конкурентного анализа параметров
	type paramResult struct {
		sensitivity SensitivityResult
		err         error
		parameter   string
	}

	resultCh := make(chan paramResult, 2)

	// Запускаем оба анализа параллельно
	go func() {
		configWithBath := config
		if bathymetry != nil {
			configWithBath.BathymetryGrid = bathymetry
		}
		strengthSens := sensitivityToStrengthParallel(ctx, site, configWithBath, bestFit, bathymetry)
		resultCh <- paramResult{sensitivity: strengthSens, parameter: "erosion_strength_m"}
	}()

	go func() {
		configWithBath := config
		if bathymetry != nil {
			configWithBath.BathymetryGrid = bathymetry
		}
		dirSens := sensitivityToWaveDirectionParallel(ctx, site, configWithBath, bestFit, bathymetry)
		resultCh <- paramResult{sensitivity: dirSens, parameter: "wave_direction_deg"}
	}()

	// Собираем результаты
	for i := 0; i < 2; i++ {
		select {
		case <-ctx.Done():
			return results
		case res := <-resultCh:
			results = append(results, res.sensitivity)
		}
	}

	return results
}

// BootstrapConfidenceIntervalsParallel выполняет расчёт доверительных интервалов бутстрепа с параллельными итерациями
func BootstrapConfidenceIntervalsParallel(
	ctx context.Context,
	site BenchmarkSite,
	config CalibrationConfig,
	bootstrapIterations int,
	bathymetry *geometry.BathymetryGrid,
) (ConfidenceInterval, ConfidenceInterval) {
	if bootstrapIterations < 1 {
		bootstrapIterations = 200
	}

	// Получаем исходную лучшую подгонку
	var origResults []CalibrationResultItem
	var err error

	if bathymetry != nil {
		origResults, err = CalibrateWithBathymetry(site, config, bathymetry)
	} else {
		origResults, err = Calibrate(site, config)
	}

	if err != nil || len(origResults) == 0 {
		return ConfidenceInterval{Parameter: "erosion_strength_m"}, ConfidenceInterval{Parameter: "wave_direction_deg"}
	}
	origBest := origResults[0]

	// Хранилище с конкурентным доступом
	type accumulator struct {
		mu         sync.Mutex
		strengths  []float64
		directions []float64
	}

	acc := &accumulator{
		strengths:  make([]float64, 0, bootstrapIterations),
		directions: make([]float64, 0, bootstrapIterations),
	}

	nObs := len(site.ObservedErosion)

	var wg sync.WaitGroup
	for iter := 0; iter < bootstrapIterations; iter++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			resampled := resampleObservations(site.ObservedErosion, nObs, pseudoRandom)
			if len(resampled) == 0 {
				return
			}

			tempSite := site
			tempSite.ObservedErosion = resampled

			var results []CalibrationResultItem
			var localErr error

			if bathymetry != nil {
				results, localErr = CalibrateWithBathymetry(tempSite, config, bathymetry)
			} else {
				results, localErr = Calibrate(tempSite, config)
			}

			if localErr != nil || len(results) == 0 {
				return
			}

			acc.mu.Lock()
			acc.strengths = append(acc.strengths, results[0].ErosionStrength)
			acc.directions = append(acc.directions, results[0].WaveDirection)
			acc.mu.Unlock()
		}(iter)
	}

	wg.Wait()

	if ctx.Err() != nil {
		return ConfidenceInterval{Parameter: "erosion_strength_m"}, ConfidenceInterval{Parameter: "wave_direction_deg"}
	}

	strengthCI := computeCI("erosion_strength_m", acc.strengths, origBest.ErosionStrength)
	dirCI := computeCIDirectional("wave_direction_deg", acc.directions, origBest.WaveDirection)

	return strengthCI, dirCI
}

// RunFullAnalysisParallel выполняет полный анализ с параллельными компонентами
func RunFullAnalysisParallel(ctx context.Context, site BenchmarkSite, config CalibrationConfig, bathymetry *geometry.BathymetryGrid, bootstrapIter int) (FullAnalysis, error) {
	// Фаза 1: Получаем лучшую подгонку
	results, err := func() ([]CalibrationResultItem, error) {
		if bathymetry != nil {
			return CalibrateWithBathymetry(site, config, bathymetry)
		}
		return Calibrate(site, config)
	}()

	if err != nil {
		return FullAnalysis{}, err
	}
	if len(results) == 0 {
		return FullAnalysis{}, fmt.Errorf("no calibration results")
	}
	bestFit := results[0]

	// Фаза 2: Запускаем анализ чувствительности и CI параллельно
	type analysisPhase struct {
		sensitivities []SensitivityResult
		strengthCI    ConfidenceInterval
		dirCI         ConfidenceInterval
	}

	phaseCh := make(chan analysisPhase, 1)
	errCh := make(chan error, 1)

	go func() {
		// Анализ чувствительности
		sensitivities := AnalyzeSensitivityParallel(ctx, site, config, bestFit, bathymetry)

		// Доверительные интервалы бутстрепа
		strengthCI, dirCI := BootstrapConfidenceIntervalsParallel(ctx, site, config, bootstrapIter, bathymetry)

		select {
		case phaseCh <- analysisPhase{
			sensitivities: sensitivities,
			strengthCI:    strengthCI,
			dirCI:         dirCI,
		}:
		case <-ctx.Done():
		}
	}()

	// Сравнение с null моделью (может выполняться параллельно с вышесказанным)
	var observed, modeled []float64
	for _, c := range bestFit.ComparisonPoints {
		observed = append(observed, c.Observed)
		modeled = append(modeled, c.Modeled)
	}
	nullModel := CompareWithNullModel(observed, modeled)

	select {
	case <-ctx.Done():
		return FullAnalysis{}, ctx.Err()
	case phase := <-phaseCh:
		return FullAnalysis{
			BestFit:         bestFit,
			Sensitivities:   phase.sensitivities,
			StrengthCI:      phase.strengthCI,
			WaveDirectionCI: phase.dirCI,
			NullModel:       nullModel,
		}, nil
	case err := <-errCh:
		return FullAnalysis{}, err
	}
}

// runCalibrationIterationWithBathymetry - вспомогательная функция, выполняющая калибровку с батиметрией
func runCalibrationIterationWithBathymetry(site BenchmarkSite, config CalibrationConfig, strength, waveDir float64, steps int, bathymetry *geometry.BathymetryGrid) CalibrationResultItem {
	config.BathymetryGrid = bathymetry
	return runCalibrationIteration(site, config, strength, waveDir, steps)
}

// Вспомогательная функция для min (избежание конфликтов имён)
func sensitivityMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
