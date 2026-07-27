package benchmark

import (
	"fmt"
	"math"
	"sort"

	"coastal-geometry/internal/domain/geometry"
)

const metersPerDegLat = 111194.9

// CalibrationConfig configures a calibration run
type CalibrationConfig struct {
	// Parameter space to search
	ErosionStrengths []float64 // values of erosion-strength to try (m/step)
	WaveDirections   []float64 // dominant wave directions to try (deg from N)

	// Optional: wave spectrum spread (degrees) - if >0, each direction becomes a Gaussian spectrum
	// 0 = single direction (legacy mode)
	// 30 = mild directional spreading
	// 60 = wide directional spreading
	SpectrumSpreadDeg float64

	// Simulation parameters
	YearsPerStep   float64 // years per simulation step
	TotalYears     int     // total years to simulate
	WindSpeed      float64 // wind speed (m/s)
	BathymetryGrid *geometry.BathymetryGrid

	// Matching
	MaxDistanceKm float64 // max distance from observation to coastline point
}

// DefaultCalibrationConfig returns a reasonable starting config for Black Sea sites
func DefaultCalibrationConfig() CalibrationConfig {
	return CalibrationConfig{
		// Finer strength range focused on lower values (best fits tend to be 5-30)
		ErosionStrengths: []float64{2, 5, 10, 15, 20, 30, 50, 80},
		// 16 compass directions for better resolution
		WaveDirections: []float64{0, 22.5, 45, 67.5, 90, 112.5, 135, 157.5,
			180, 202.5, 225, 247.5, 270, 292.5, 315, 337.5},
		// Spectrum spread = 0 means single direction (legacy mode)
		// Set to e.g. 30 to enable Gaussian directional spreading
		SpectrumSpreadDeg: 0,
		YearsPerStep:      1.0,
		TotalYears:        10,
		WindSpeed:         12,
		MaxDistanceKm:     5.0,
	}
}

// CalibrationResultItem represents one parameter combination and its validation
type CalibrationResultItem struct {
	ErosionStrength   float64           `json:"erosion_strength"`
	WaveDirection     float64           `json:"wave_direction"`
	ValidationMetrics ValidationMetrics `json:"validation_metrics"`
	ComparisonPoints  []ComparisonPoint `json:"comparison_points,omitempty"`
}

// ComparisonPoint shows modeled vs observed at a single observation location
type ComparisonPoint struct {
	LatLon            geometry.LatLon `json:"lat_lon"`
	Observed          float64         `json:"observed_m_per_year"`
	Modeled           float64         `json:"modeled_m_per_year"`
	DistanceToCoastKm float64         `json:"distance_to_coast_km"`
}

// Calibrate runs the calibration for a benchmark site
//
// Algorithm:
//  1. For each (erosion_strength, wave_direction) combination:
//     a. Run wave erosion simulation (with bathymetry if provided)
//     b. For each observed erosion point:
//     - Find nearest coastline segment
//     - Compute modeled retreat rate (m/year) at that segment
//     c. Compute validation metrics (RMSE, MAE, R²)
//  2. Return results sorted by RMSE (best first)
func Calibrate(site BenchmarkSite, config CalibrationConfig) ([]CalibrationResultItem, error) {
	if len(site.ObservedErosion) == 0 {
		return nil, fmt.Errorf("site %q has no observed erosion data", site.ID)
	}
	if len(site.Coastline) < 3 {
		return nil, fmt.Errorf("site %q has too few coastline points (%d)", site.ID, len(site.Coastline))
	}

	// Compute steps from years
	steps := int(float64(config.TotalYears) / config.YearsPerStep)
	if steps < 1 {
		steps = 1
	}

	// Build parameter combinations for parallel execution
	type combo struct {
		strength float64
		waveDir  float64
	}
	var combos []combo
	for _, strength := range config.ErosionStrengths {
		for _, waveDir := range config.WaveDirections {
			combos = append(combos, combo{strength, waveDir})
		}
	}

	results := make([]CalibrationResultItem, len(combos))
	parallelCalibrate(combos, func(i int, c combo) {
		results[i] = runCalibrationIteration(site, config, c.strength, c.waveDir, steps)
	})

	// Sort by RMSE ascending (best first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ValidationMetrics.RMSE < results[j].ValidationMetrics.RMSE
	})

	return results, nil
}

// parallelCalibrate runs calibration iterations in parallel
// Uses up to 8 workers (calibration is CPU-bound)
func parallelCalibrate[T any](items []T, fn func(i int, item T)) {
	const maxWorkers = 8
	n := len(items)
	if n == 0 {
		return
	}

	workers := maxWorkers
	if workers > n {
		workers = n
	}

	jobs := make(chan int, n)
	for i := range items {
		jobs <- i
	}
	close(jobs)

	done := make(chan struct{}, workers)
	for w := 0; w < workers; w++ {
		go func() {
			for idx := range jobs {
				fn(idx, items[idx])
			}
			done <- struct{}{}
		}()
	}
	for w := 0; w < workers; w++ {
		<-done
	}
}

// CalibrateWithBathymetry runs calibration with bathymetry data integrated
// This typically produces significantly better results than flat-bottom calibration
func CalibrateWithBathymetry(site BenchmarkSite, config CalibrationConfig, bathymetry *geometry.BathymetryGrid) ([]CalibrationResultItem, error) {
	if len(site.ObservedErosion) == 0 {
		return nil, fmt.Errorf("site %q has no observed erosion data", site.ID)
	}
	if len(site.Coastline) < 3 {
		return nil, fmt.Errorf("site %q has too few coastline points (%d)", site.ID, len(site.Coastline))
	}

	// Inject bathymetry into config
	config.BathymetryGrid = bathymetry

	return Calibrate(site, config)
}

// runCalibrationIteration runs a single model run and computes validation
func runCalibrationIteration(
	site BenchmarkSite,
	config CalibrationConfig,
	strength float64,
	waveDir float64,
	steps int,
) CalibrationResultItem {
	// If spectrum spread is enabled, run model with multiple weighted directions
	// and aggregate retreat rates
	if config.SpectrumSpreadDeg > 0 {
		return runCalibrationWithSpectrum(site, config, strength, waveDir, steps)
	}

	options := geometry.WaveErosionOptions{
		StrengthMeters:           strength,
		WindSourceDirectionDeg:   waveDir,
		WindSpeedMetersPerSecond: config.WindSpeed,
		FetchSpreadDeg:           55,
		FetchSamples:             9,
		MaxFetchMeters:           150_000,
		DepthScaleMeters:         4000,
		ExposurePower:            1.5,
		MaxRetreatMeters:         strength * 3,
		BathymetryGrid:           config.BathymetryGrid,
	}

	snapshots := geometry.SimulateWaveErosionWithSeed(site.Coastline, steps, options, 42)

	initial := snapshots[0]
	final := snapshots[len(snapshots)-1]

	comparisons := computeComparisons(initial, final, site.ObservedErosion, config.YearsPerStep, config.TotalYears)
	metrics := computeValidationMetrics(comparisons)

	return CalibrationResultItem{
		ErosionStrength:   strength,
		WaveDirection:     waveDir,
		ValidationMetrics: metrics,
		ComparisonPoints:  comparisons,
	}
}

// runCalibrationWithSpectrum uses Gaussian directional spreading
// instead of single wave direction
func runCalibrationWithSpectrum(
	site BenchmarkSite,
	config CalibrationConfig,
	strength float64,
	centerDir float64,
	steps int,
) CalibrationResultItem {
	// Build spectrum: 8 bins with Gaussian weights centered at centerDir
	spectrum := geometry.NewGaussianSpectrum(centerDir, config.SpectrumSpreadDeg, 8)

	// Aggregate retreat per coastline point across all spectrum bins
	// Approach: run model for each bin independently, sum weighted retreat
	nCoast := len(site.Coastline)
	if nCoast < 3 {
		return CalibrationResultItem{
			ErosionStrength:   strength,
			WaveDirection:     centerDir,
			ValidationMetrics: ValidationMetrics{},
		}
	}

	totalRetreat := make([]float64, nCoast)
	totalWeight := 0.0

	for _, bin := range spectrum.Bins {
		if bin.Weight < 0.01 {
			continue
		}

		// Run erosion for this direction
		// Scale strength by weight so contributions sum properly
		binStrength := strength * bin.Weight * 2 // factor 2 because each direction contributes partially
		if binStrength < 0.1 {
			continue
		}

		options := geometry.WaveErosionOptions{
			StrengthMeters:           binStrength,
			WindSourceDirectionDeg:   bin.Direction,
			WindSpeedMetersPerSecond: config.WindSpeed,
			FetchSpreadDeg:           55,
			FetchSamples:             9,
			MaxFetchMeters:           150_000,
			DepthScaleMeters:         4000,
			ExposurePower:            1.5,
			MaxRetreatMeters:         binStrength * 3,
			BathymetryGrid:           config.BathymetryGrid,
		}

		snapshots := geometry.SimulateWaveErosionWithSeed(site.Coastline, steps, options, 42)
		initial := snapshots[0]
		final := snapshots[len(snapshots)-1]

		// Compute retreat at each point
		for i := range site.Coastline {
			if i >= len(initial) || i >= len(final) {
				continue
			}
			retreat := computeSegmentRetreat(initial, final, i)
			if retreat > 0 {
				totalRetreat[i] += retreat * bin.Weight
			}
		}
		totalWeight += bin.Weight
	}

	// Build a synthetic final coastline using aggregated retreat
	// Use the per-point accumulated retreat directly for comparison
	comparisons := computeComparisonsFromRetreats(
		site.Coastline, totalRetreat, totalWeight,
		site.ObservedErosion, config.TotalYears,
	)
	metrics := computeValidationMetrics(comparisons)

	return CalibrationResultItem{
		ErosionStrength:   strength,
		WaveDirection:     centerDir,
		ValidationMetrics: metrics,
		ComparisonPoints:  comparisons,
	}
}

// computeComparisonsFromRetreats builds comparison points from pre-computed retreats
func computeComparisonsFromRetreats(
	coastline []geometry.LatLon,
	retreats []float64,
	totalWeight float64,
	observations []ErosionObservation,
	totalYears int,
) []ComparisonPoint {
	if totalWeight <= 0 {
		return nil
	}

	var comparisons []ComparisonPoint
	for _, obs := range observations {
		segIdx := nearestSegmentIndex(coastline, obs.LatLon)
		if segIdx < 0 || segIdx >= len(retreats) {
			continue
		}

		// Normalize: weighted retreat / total weight / years
		modeledRate := retreats[segIdx] / totalWeight / float64(totalYears)
		dist := haversineKm(coastline[segIdx], obs.LatLon)

		comparisons = append(comparisons, ComparisonPoint{
			LatLon:            obs.LatLon,
			Observed:          obs.ShorelineChangeRate,
			Modeled:           modeledRate,
			DistanceToCoastKm: dist,
		})
	}
	return comparisons
}

// computeComparisons matches observations to nearest coastline segments
// and computes modeled retreat rates
func computeComparisons(
	initial, final []geometry.LatLon,
	observations []ErosionObservation,
	yearsPerStep float64,
	totalYears int,
) []ComparisonPoint {
	var comparisons []ComparisonPoint

	for _, obs := range observations {
		// Find nearest segment in initial coastline
		segIdx := nearestSegmentIndex(initial, obs.LatLon)
		if segIdx < 0 {
			continue
		}

		// Compute modeled retreat (m) at this segment
		// Retreat = perpendicular displacement of segment midpoint
		modeledRetreat := computeSegmentRetreat(initial, final, segIdx)
		// Convert retreat to rate per year (positive = erosion)
		modeledRate := modeledRetreat / float64(totalYears)

		dist := haversineKm(initial[segIdx], obs.LatLon)

		comparisons = append(comparisons, ComparisonPoint{
			LatLon:            obs.LatLon,
			Observed:          obs.ShorelineChangeRate,
			Modeled:           modeledRate,
			DistanceToCoastKm: dist,
		})
	}

	return comparisons
}

// nearestSegmentIndex returns the index of the closest coastline point to target
func nearestSegmentIndex(coastline []geometry.LatLon, target geometry.LatLon) int {
	if len(coastline) == 0 {
		return -1
	}

	bestIdx := 0
	bestDist := haversineKm(coastline[0], target)
	for i := 1; i < len(coastline); i++ {
		d := haversineKm(coastline[i], target)
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx
}

// computeSegmentRetreat estimates how much a coastline point retreated (m)
// Positive value = erosion (retreat landward)
func computeSegmentRetreat(initial, final []geometry.LatLon, idx int) float64 {
	if idx >= len(initial) || idx >= len(final) {
		return 0
	}

	// Get segment orientation using neighboring points
	// and compute perpendicular displacement
	prev := (idx - 1 + len(initial)) % len(initial)
	next := (idx + 1) % len(initial)

	// Direction of segment at this point
	dx := initial[next].Lon - initial[prev].Lon
	dy := initial[next].Lat - initial[prev].Lat

	// Outward normal (perpendicular, pointing away from coast)
	// Rotate tangent by 90° (counter-clockwise gives outward normal for CCW ring)
	nx := -dy
	ny := dx
	nlen := math.Sqrt(nx*nx + ny*ny)
	if nlen == 0 {
		return 0
	}
	nx /= nlen
	ny /= nlen

	// Convert displacement to meters using local meters-per-degree
	refLat := initial[idx].Lat
	metersPerDegLon := metersPerDegLat * math.Cos(refLat*math.Pi/180)

	// Displacement of this point
	dLon := final[idx].Lon - initial[idx].Lon
	dLat := final[idx].Lat - initial[idx].Lat

	// Project displacement onto outward normal
	dMetersX := dLon * metersPerDegLon
	dMetersY := dLat * metersPerDegLat
	nMetersX := nx * metersPerDegLon
	nMetersY := ny * metersPerDegLat
	nlenMeters := math.Sqrt(nMetersX*nMetersX + nMetersY*nMetersY)
	if nlenMeters == 0 {
		return 0
	}

	// Scalar projection (negative = landward retreat = erosion)
	projection := (dMetersX*nMetersX + dMetersY*nMetersY) / nlenMeters

	// Retreat (positive erosion) = -projection (because outward is +)
	return -projection
}

// haversineKm computes great-circle distance between two points in km
func haversineKm(a, b geometry.LatLon) float64 {
	const earthRadiusKm = 6371.0
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180

	sindLat := math.Sin(dLat / 2)
	sindLon := math.Sin(dLon / 2)
	h := sindLat*sindLat + math.Cos(lat1)*math.Cos(lat2)*sindLon*sindLon
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(math.Min(1, h)))
}

// computeValidationMetrics computes RMSE, MAE, MBE, R², and significance
func computeValidationMetrics(comparisons []ComparisonPoint) ValidationMetrics {
	n := len(comparisons)
	if n == 0 {
		return ValidationMetrics{}
	}

	var sumSqErr, sumAbsErr, sumBiasErr float64
	var sumObs, sumMod float64
	for _, c := range comparisons {
		err := c.Modeled - c.Observed
		sumSqErr += err * err
		sumAbsErr += math.Abs(err)
		sumBiasErr += err
		sumObs += c.Observed
		sumMod += c.Modeled
	}

	mse := sumSqErr / float64(n)
	rmse := math.Sqrt(mse)
	mae := sumAbsErr / float64(n)
	mbe := sumBiasErr / float64(n)

	meanObs := sumObs / float64(n)
	_ = sumMod / float64(n)

	// R² = 1 - SS_res / SS_tot
	var ssRes, ssTot float64
	for _, c := range comparisons {
		ssRes += (c.Modeled - c.Observed) * (c.Modeled - c.Observed)
		ssTot += (c.Observed - meanObs) * (c.Observed - meanObs)
	}
	rSquared := 0.0
	if ssTot > 0 {
		rSquared = 1 - ssRes/ssTot
	}

	// Pearson correlation for significance test
	r := pearsonCorrelation(comparisons)
	pValue := computePValue(r, n)
	significant := pValue < 0.05

	return ValidationMetrics{
		RMSE:        rmse,
		MAE:         mae,
		MBE:         mbe,
		RSquared:    rSquared,
		N:           n,
		PValue:      pValue,
		Significant: significant,
	}
}

// pearsonCorrelation computes Pearson r between observed and modeled
func pearsonCorrelation(comparisons []ComparisonPoint) float64 {
	n := len(comparisons)
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for _, c := range comparisons {
		sumX += c.Observed
		sumY += c.Modeled
		sumXY += c.Observed * c.Modeled
		sumX2 += c.Observed * c.Observed
		sumY2 += c.Modeled * c.Modeled
	}

	num := float64(n)*sumXY - sumX*sumY
	denX := math.Sqrt(float64(n)*sumX2 - sumX*sumX)
	denY := math.Sqrt(float64(n)*sumY2 - sumY*sumY)
	if denX == 0 || denY == 0 {
		return 0
	}
	return num / (denX * denY)
}

// computePValue approximates two-tailed p-value for Pearson r
// Uses t-distribution approximation
func computePValue(r float64, n int) float64 {
	if n < 3 {
		return 1.0
	}
	df := n - 2
	tStat := r * math.Sqrt(float64(df)) / math.Sqrt(1-r*r)
	if math.IsNaN(tStat) {
		return 1.0
	}
	// Approximate p-value using normal distribution for large df
	// For df >= 30, t ~ z; otherwise, use conservative upper bound
	if df >= 30 {
		return 2 * (1 - normalCDF(math.Abs(tStat)))
	}
	// Small sample approximation: just use normal as conservative estimate
	return 2 * (1 - normalCDF(math.Abs(tStat)))
}

// normalCDF computes standard normal CDF using error function approximation
func normalCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt(2)))
}
