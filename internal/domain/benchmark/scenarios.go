package benchmark

import (
	"fmt"
	"sort"

	"coastal-geometry/internal/domain/geometry"
)

// ScenarioConfig defines parameters for one simulation scenario
type ScenarioConfig struct {
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	ErosionStrength  float64 `json:"erosion_strength_m"`
	WaveDirection    float64 `json:"wave_direction_deg"`
	WindSpeed        float64 `json:"wind_speed_m_per_sec"`
	SeaLevelRise     float64 `json:"sea_level_rise_m_per_year"`
	StormProbability float64 `json:"storm_probability_per_step"`
	StormIntensity   float64 `json:"storm_intensity_multiplier"`
	YearsPerStep     float64 `json:"years_per_step"`
	TotalYears       int     `json:"total_years"`
}

// ScenarioResult represents the outcome of running one scenario
type ScenarioResult struct {
	Config          ScenarioConfig `json:"config"`
	SegmentRetreats []float64      `json:"segment_retreats_m_per_year"`
	TotalErodedKm   float64        `json:"total_eroded_km"`
	MeanRetreatRate float64        `json:"mean_retreat_rate_m_per_year"`
	MaxRetreatRate  float64        `json:"max_retreat_rate_m_per_year"`
	ErodingFraction float64        `json:"eroding_fraction"` // 0-1
	HotspotCount    int            `json:"hotspot_count"`
	CoastLengthKm   float64        `json:"coast_length_km"`
	CoastChangeKm   float64        `json:"coast_change_km"` // negative = shortening
}

// ScenarioDiff compares two scenarios (typically baseline vs modified)
type ScenarioDiff struct {
	Baseline             ScenarioConfig `json:"baseline"`
	Modified             ScenarioConfig `json:"modified"`
	MeanRetreatDelta     float64        `json:"mean_retreat_delta_m_per_year"`
	MaxRetreatDelta      float64        `json:"max_retreat_delta_m_per_year"`
	ErodingFractionDelta float64        `json:"eroding_fraction_delta"`
	HotspotShiftKm       float64        `json:"hotspot_shift_km"` // average shift of top hotspot
	NewHotspotCount      int            `json:"new_hotspot_count"`
	LostHotspotCount     int            `json:"lost_hotspot_count"`
}

// DefaultScenarios returns predefined climate scenarios for Black Sea
// These represent typical conditions plus climate change projections
func DefaultScenarios(strength, waveDir float64) []ScenarioConfig {
	return []ScenarioConfig{
		{
			Name:             "baseline",
			Description:      "Current conditions (no climate change)",
			ErosionStrength:  strength,
			WaveDirection:    waveDir,
			WindSpeed:        12,
			SeaLevelRise:     0,
			StormProbability: 0,
			StormIntensity:   1.0,
			YearsPerStep:     1.0,
			TotalYears:       10,
		},
		{
			Name:             "rcp45_2050",
			Description:      "RCP4.5 scenario for 2050: moderate climate change",
			ErosionStrength:  strength,
			WaveDirection:    waveDir,
			WindSpeed:        13,    // +8% wind by 2050
			SeaLevelRise:     0.005, // 5mm/year
			StormProbability: 0.1,
			StormIntensity:   1.2,
			YearsPerStep:     1.0,
			TotalYears:       10,
		},
		{
			Name:             "rcp85_2050",
			Description:      "RCP8.5 scenario for 2050: high emissions pathway",
			ErosionStrength:  strength,
			WaveDirection:    waveDir,
			WindSpeed:        14,    // +17% wind
			SeaLevelRise:     0.008, // 8mm/year
			StormProbability: 0.15,
			StormIntensity:   1.5,
			YearsPerStep:     1.0,
			TotalYears:       10,
		},
		{
			Name:             "rcp85_2100",
			Description:      "RCP8.5 scenario for 2100: extreme warming",
			ErosionStrength:  strength,
			WaveDirection:    waveDir,
			WindSpeed:        16,    // +33% wind
			SeaLevelRise:     0.012, // 12mm/year
			StormProbability: 0.2,
			StormIntensity:   2.0,
			YearsPerStep:     1.0,
			TotalYears:       10,
		},
		{
			Name:             "storm_surge",
			Description:      "Major storm surge event (1-in-100 year)",
			ErosionStrength:  strength,
			WaveDirection:    waveDir,
			WindSpeed:        25, // extreme wind
			SeaLevelRise:     0.0,
			StormProbability: 0.5,
			StormIntensity:   3.0,
			YearsPerStep:     1.0,
			TotalYears:       10,
		},
	}
}

// RunScenario executes one scenario and returns its results
func RunScenario(site BenchmarkSite, scenario ScenarioConfig, bathymetry *geometry.BathymetryGrid) (ScenarioResult, error) {
	if len(site.Coastline) < 3 {
		return ScenarioResult{}, fmt.Errorf("site %q has too few coastline points", site.ID)
	}

	steps := int(float64(scenario.TotalYears) / scenario.YearsPerStep)
	if steps < 1 {
		steps = 1
	}

	options := geometry.WaveErosionOptions{
		StrengthMeters:           scenario.ErosionStrength,
		WindSourceDirectionDeg:   scenario.WaveDirection,
		WindSpeedMetersPerSecond: scenario.WindSpeed,
		FetchSpreadDeg:           55,
		FetchSamples:             9,
		MaxFetchMeters:           150_000,
		DepthScaleMeters:         4000,
		ExposurePower:            1.5,
		MaxRetreatMeters:         scenario.ErosionStrength * 3,
		BathymetryGrid:           bathymetry,
	}

	// Note: SLR and storm parameters affect baseline; the core model uses these as
	// modifiers on the retreat calculation. For now we encode them as wind speed
	// multipliers since the underlying model doesn't yet have direct SLR support.
	// TODO: When model has SLR/storm fields, pass them directly
	stormEffect := 1.0 + scenario.StormProbability*(scenario.StormIntensity-1.0)
	slrEffect := 1.0 + scenario.SeaLevelRise*float64(scenario.TotalYears)*0.5
	options.WindSpeedMetersPerSecond *= stormEffect * slrEffect
	if options.WindSpeedMetersPerSecond < 1 {
		options.WindSpeedMetersPerSecond = 1
	}

	snapshots := geometry.SimulateWaveErosionWithSeed(site.Coastline, steps, options, 42)
	initial := snapshots[0]
	final := snapshots[len(snapshots)-1]

	retreats := make([]float64, len(initial))
	for i := range initial {
		retreats[i] = computeSegmentRetreat(initial, final, i) / float64(scenario.TotalYears)
	}

	// Aggregate metrics
	var sumRetreat, maxRetreat float64
	var erodingCount int
	for _, r := range retreats {
		if r > 0 {
			sumRetreat += r
			erodingCount++
			if r > maxRetreat {
				maxRetreat = r
			}
		}
	}

	n := len(retreats)
	erodingFraction := 0.0
	meanRetreat := 0.0
	if n > 0 {
		erodingFraction = float64(erodingCount) / float64(n)
	}
	if erodingCount > 0 {
		meanRetreat = sumRetreat / float64(erodingCount)
	}

	// Coast length change
	var initialLen, finalLen float64
	for i := 0; i+1 < len(initial); i++ {
		initialLen += haversineKm(initial[i], initial[i+1])
	}
	for i := 0; i+1 < len(final); i++ {
		finalLen += haversineKm(final[i], final[i+1])
	}

	// Count hotspots using rates (simplified)
	rates := make([]SegmentRate, len(retreats))
	for i, r := range retreats {
		rates[i] = SegmentRate{Index: i, RetreatRate: r, Center: initial[i]}
	}
	hotspots := FindHotspots(rates, initial, 100, 0.75)

	return ScenarioResult{
		Config:          scenario,
		SegmentRetreats: retreats,
		TotalErodedKm:   initialLen - finalLen,
		MeanRetreatRate: meanRetreat,
		MaxRetreatRate:  maxRetreat,
		ErodingFraction: erodingFraction,
		HotspotCount:    len(hotspots),
		CoastLengthKm:   initialLen,
		CoastChangeKm:   finalLen - initialLen,
	}, nil
}

// RunScenarios runs all provided scenarios for a site and returns their results
func RunScenarios(site BenchmarkSite, scenarios []ScenarioConfig, bathymetry *geometry.BathymetryGrid) ([]ScenarioResult, error) {
	results := make([]ScenarioResult, 0, len(scenarios))
	for _, sc := range scenarios {
		res, err := RunScenario(site, sc, bathymetry)
		if err != nil {
			return nil, fmt.Errorf("run scenario %q: %w", sc.Name, err)
		}
		results = append(results, res)
	}
	return results, nil
}

// CompareScenarios produces diffs between baseline and all other scenarios
// coastline is needed to compute hotspot positions; if nil, hotspot shift is 0
func CompareScenarios(results []ScenarioResult, coastline []geometry.LatLon) []ScenarioDiff {
	if len(results) < 2 {
		return nil
	}

	baseline := results[0]
	var diffs []ScenarioDiff

	for _, r := range results[1:] {
		baselineRates := ratesToSegmentRates(baseline.SegmentRetreats, baseline.Config)
		modifiedRates := ratesToSegmentRates(r.SegmentRetreats, r.Config)

		var hotspotShiftKm float64
		if coastline != nil && len(coastline) > 0 {
			baselineHotspots := FindHotspots(baselineRates, coastline, 1, 0.75)
			modifiedHotspots := FindHotspots(modifiedRates, coastline, 1, 0.75)
			if len(baselineHotspots) > 0 && len(modifiedHotspots) > 0 {
				hotspotShiftKm = haversineKm(baselineHotspots[0].Center, modifiedHotspots[0].Center)
			}
		}

		diffs = append(diffs, ScenarioDiff{
			Baseline:             baseline.Config,
			Modified:             r.Config,
			MeanRetreatDelta:     r.MeanRetreatRate - baseline.MeanRetreatRate,
			MaxRetreatDelta:      r.MaxRetreatRate - baseline.MaxRetreatRate,
			ErodingFractionDelta: r.ErodingFraction - baseline.ErodingFraction,
			HotspotShiftKm:       hotspotShiftKm,
			NewHotspotCount:      max(0, r.HotspotCount-baseline.HotspotCount),
			LostHotspotCount:     max(0, baseline.HotspotCount-r.HotspotCount),
		})
	}

	return diffs
}

func ratesToSegmentRates(rates []float64, cfg ScenarioConfig) []SegmentRate {
	result := make([]SegmentRate, len(rates))
	for i, r := range rates {
		result[i] = SegmentRate{Index: i, RetreatRate: r}
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SortScenariosByImpact sorts scenarios by mean retreat rate (descending)
func SortScenariosByImpact(results []ScenarioResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].MeanRetreatRate > results[j].MeanRetreatRate
	})
}
