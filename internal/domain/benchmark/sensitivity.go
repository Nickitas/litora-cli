package benchmark

import (
	"fmt"
	"math"
	"sort"

	"coastal-geometry/internal/domain/geometry"
)

// SensitivityResult describes how a parameter affects model output
type SensitivityResult struct {
	Parameter        string    `json:"parameter"`  // e.g. "erosion_strength"
	Values           []float64 `json:"values"`     // parameter values tested
	RMSE             []float64 `json:"rmse"`       // RMSE at each value
	MAE              []float64 `json:"mae"`        // MAE at each value
	RSquared         []float64 `json:"r_squared"`  // R² at each value
	BestValue        float64   `json:"best_value"` // value with lowest RMSE
	BestRMSE         float64   `json:"best_rmse"`
	WorstValue       float64   `json:"worst_value"` // value with highest RMSE
	WorstRMSE        float64   `json:"worst_rmse"`
	SensitivityScore float64   `json:"sensitivity_score"` // (max-min)/max normalized
	LocalSensitivity float64   `json:"local_sensitivity"` // derivative at best point
}

// ConfidenceInterval represents a parameter confidence interval from bootstrap
type ConfidenceInterval struct {
	Parameter string  `json:"parameter"`
	Mean      float64 `json:"mean"`
	StdDev    float64 `json:"std_dev"`
	Median    float64 `json:"median"`
	Lower95   float64 `json:"lower_95"` // 2.5 percentile
	Upper95   float64 `json:"upper_95"` // 97.5 percentile
	Lower68   float64 `json:"lower_68"` // 16 percentile (~1 sigma)
	Upper68   float64 `json:"upper_68"` // 84 percentile
	BestFit   float64 `json:"best_fit"` // original best fit (no resampling)
}

// NullModelComparison compares model against simple baselines
type NullModelComparison struct {
	ModelRMSE       float64 `json:"model_rmse"`
	MeanModelRMSE   float64 `json:"mean_model_rmse"`   // predict mean observed
	LinearTrendRMSE float64 `json:"linear_trend_rmse"` // predict linear trend (single value)
	SkillScore      float64 `json:"skill_score"`       // (null - model) / null, 1=perfect, 0=as good as null
	Improvement     float64 `json:"improvement_pct"`   // % improvement over null model
}

// FullAnalysis combines calibration, sensitivity, CI, and null model comparison
type FullAnalysis struct {
	BestFit         CalibrationResultItem `json:"best_fit"`
	Sensitivities   []SensitivityResult   `json:"sensitivities"`
	StrengthCI      ConfidenceInterval    `json:"strength_ci"`
	WaveDirectionCI ConfidenceInterval    `json:"wave_direction_ci"`
	NullModel       NullModelComparison   `json:"null_model"`
}

// AnalyzeSensitivity performs one-at-a-time sensitivity analysis
// For each parameter, holds others at best-fit and varies this one
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
	// Test range around best fit
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

// generateTestRange generates values around center from (center/factor) to (center*factor)
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

	// Local sensitivity: derivative at best point
	// Use forward/backward/central difference
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

// BootstrapConfidenceIntervals estimates CI for best-fit parameters
// by running calibration on resampled observation sets
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

	// Get original best fit
	origResults, _ := Calibrate(site, config)
	origBest := origResults[0]

	// Storage for bootstrap best-fit parameters
	strengths := make([]float64, 0, bootstrapIterations)
	directions := make([]float64, 0, bootstrapIterations)

	nObs := len(site.ObservedErosion)

	for iter := 0; iter < bootstrapIterations; iter++ {
		// Resample observations with replacement
		resampled := resampleObservations(site.ObservedErosion, nObs, rng)
		if len(resampled) == 0 {
			continue
		}

		// Create temp site with resampled observations
		tempSite := site
		tempSite.ObservedErosion = resampled

		// Run calibration
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

// computeCI computes CI for scalar values
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

// computeCIDirectional handles angular data (wave direction)
// by computing CI on raw values when distribution is unimodal
func computeCIDirectional(name string, values []float64, bestFit float64) ConfidenceInterval {
	ci := computeCI(name, values, bestFit)
	// Wrap values to [0, 360)
	ci.Median = math.Mod(math.Mod(ci.Median, 360)+360, 360)
	ci.Lower95 = math.Mod(math.Mod(ci.Lower95, 360)+360, 360)
	ci.Upper95 = math.Mod(math.Mod(ci.Upper95, 360)+360, 360)
	ci.Lower68 = math.Mod(math.Mod(ci.Lower68, 360)+360, 360)
	ci.Upper68 = math.Mod(math.Mod(ci.Upper68, 360)+360, 360)
	ci.BestFit = math.Mod(math.Mod(ci.BestFit, 360)+360, 360)
	return ci
}

// resampleObservations performs bootstrap resampling
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

// CompareWithNullModel compares model performance to mean baseline
func CompareWithNullModel(observed, modeled []float64) NullModelComparison {
	if len(observed) == 0 || len(modeled) == 0 {
		return NullModelComparison{}
	}

	meanObs := meanOf(observed)

	// Model RMSE
	modelRMSE := rmseBetween(observed, modeled)

	// Mean model RMSE (predict the mean for all)
	meanModelRMSE := rmseConstant(observed, meanObs)

	// Linear trend RMSE (single best-fit constant for simplicity)
	// We don't have independent variable here, so use mean as simplest trend
	linearTrendRMSE := meanModelRMSE

	// Skill score: (null - model) / null
	// 1 = perfect, 0 = as good as null, negative = worse than null
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

// RunFullAnalysis runs calibration, sensitivity, CI, null model comparison
func RunFullAnalysis(site BenchmarkSite, config CalibrationConfig, bootstrapIter int) (FullAnalysis, error) {
	// 1. Get best fit
	results, err := Calibrate(site, config)
	if err != nil {
		return FullAnalysis{}, err
	}
	if len(results) == 0 {
		return FullAnalysis{}, fmt.Errorf("no calibration results")
	}
	bestFit := results[0]

	// 2. Sensitivity analysis
	sensitivities := AnalyzeSensitivity(site, config, bestFit)

	// 3. Bootstrap confidence intervals
	strengthCI, dirCI := BootstrapConfidenceIntervals(site, config, bootstrapIter, nil)

	// 4. Null model comparison
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

// RunFullAnalysisWithBathymetry is same as RunFullAnalysis but with bathymetry
func RunFullAnalysisWithBathymetry(site BenchmarkSite, config CalibrationConfig, bathymetry *geometry.BathymetryGrid, bootstrapIter int) (FullAnalysis, error) {
	// 1. Get best fit
	results, err := CalibrateWithBathymetry(site, config, bathymetry)
	if err != nil {
		return FullAnalysis{}, err
	}
	if len(results) == 0 {
		return FullAnalysis{}, fmt.Errorf("no calibration results")
	}
	bestFit := results[0]

	// 2. Sensitivity analysis (with bathymetry)
	sensitivities := analyzeSensitivityWithBathymetry(site, config, bestFit, bathymetry)

	// 3. Bootstrap confidence intervals
	strengthCI, dirCI := bootstrapWithBathymetry(site, config, bathymetry, bootstrapIter)

	// 4. Null model comparison
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
