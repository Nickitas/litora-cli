package benchmark

import (
	"fmt"
	"sort"

	"coastal-geometry/internal/domain/geometry"
)

// ScenarioConfig определяет параметры одного сценария симуляции
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

// ScenarioResult представляет результат выполнения одного сценария
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

// ScenarioDiff сравнивает два сценария (обычно базовый и изменённый)
type ScenarioDiff struct {
	Baseline             ScenarioConfig `json:"baseline"`
	Modified             ScenarioConfig `json:"modified"`
	MeanRetreatDelta     float64        `json:"mean_retreat_delta_m_per_year"`
	MaxRetreatDelta      float64        `json:"max_retreat_delta_m_per_year"`
	ErodingFractionDelta float64        `json:"eroding_fraction_delta"`
	HotspotShiftKm       float64        `json:"hotspot_shift_km"` // среднее смещение топового участка
	NewHotspotCount      int            `json:"new_hotspot_count"`
	LostHotspotCount     int            `json:"lost_hotspot_count"`
}

// DefaultScenarios возвращает предопределённые климатические сценарии для Чёрного моря
// Это типичные условия плюс прогнозы климатических изменений
func DefaultScenarios(strength, waveDir float64) []ScenarioConfig {
	return []ScenarioConfig{
		{
			Name:             "baseline",
			Description:      "Текущие условия (без изменения климата)",
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
			Description:      "Сценарий RCP4.5 на 2050 год: умеренное изменение климата",
			ErosionStrength:  strength,
			WaveDirection:    waveDir,
			WindSpeed:        13,    // +8% ветра к 2050 году
			SeaLevelRise:     0.005, // 5мм/год
			StormProbability: 0.1,
			StormIntensity:   1.2,
			YearsPerStep:     1.0,
			TotalYears:       10,
		},
		{
			Name:             "rcp85_2050",
			Description:      "Сценарий RCP8.5 на 2050 год: высокий уровень выбросов",
			ErosionStrength:  strength,
			WaveDirection:    waveDir,
			WindSpeed:        14,    // +17% ветра
			SeaLevelRise:     0.008, // 8мм/год
			StormProbability: 0.15,
			StormIntensity:   1.5,
			YearsPerStep:     1.0,
			TotalYears:       10,
		},
		{
			Name:             "rcp85_2100",
			Description:      "Сценарий RCP8.5 на 2100 год: экстремальное потепление",
			ErosionStrength:  strength,
			WaveDirection:    waveDir,
			WindSpeed:        16,    // +33% ветра
			SeaLevelRise:     0.012, // 12мм/год
			StormProbability: 0.2,
			StormIntensity:   2.0,
			YearsPerStep:     1.0,
			TotalYears:       10,
		},
		{
			Name:             "storm_surge",
			Description:      "Крупное событие штормового нагона (1 раз в 100 лет)",
			ErosionStrength:  strength,
			WaveDirection:    waveDir,
			WindSpeed:        25, // экстремальный ветер
			SeaLevelRise:     0.0,
			StormProbability: 0.5,
			StormIntensity:   3.0,
			YearsPerStep:     1.0,
			TotalYears:       10,
		},
	}
}

// RunScenario выполняет один сценарий и возвращает его результаты
func RunScenario(site BenchmarkSite, scenario ScenarioConfig, bathymetry *geometry.BathymetryGrid) (ScenarioResult, error) {
	if len(site.Coastline) < 3 {
		return ScenarioResult{}, fmt.Errorf("на участке %q слишком мало точек береговой линии", site.ID)
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

	// Примечание: параметры SLR и штормов влияют на базовую линию; основная модель использует их как
	// модификаторы при расчёте отступления. В настоящее время мы кодируем их как множители скорости ветра,
	// т.к. базовая модель ещё не имеет прямой поддержки SLR.
	// TODO: Когда модель будет иметь поля SLR/штормов, передавать их напрямую
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

	// Агрегированные метрики
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

	// Изменение длины побережья
	var initialLen, finalLen float64
	for i := 0; i+1 < len(initial); i++ {
		initialLen += haversineKm(initial[i], initial[i+1])
	}
	for i := 0; i+1 < len(final); i++ {
		finalLen += haversineKm(final[i], final[i+1])
	}

	// Подсчитываем участки активной эрозии по скоростям (упрощённо)
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

// RunScenarios выполняет все предоставленные сценарии для участка и возвращает их результаты
func RunScenarios(site BenchmarkSite, scenarios []ScenarioConfig, bathymetry *geometry.BathymetryGrid) ([]ScenarioResult, error) {
	results := make([]ScenarioResult, 0, len(scenarios))
	for _, sc := range scenarios {
		res, err := RunScenario(site, sc, bathymetry)
		if err != nil {
			return nil, fmt.Errorf("выполнить сценарий %q: %w", sc.Name, err)
		}
		results = append(results, res)
	}
	return results, nil
}

// CompareScenarios создаёт сравнения между базовым и всеми остальными сценариями
// Побережье нужно для вычисления позиций участков; если nil, смещение = 0
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

// SortScenariosByImpact сортирует сценарии по средней скорости отступления (по убыванию)
func SortScenariosByImpact(results []ScenarioResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].MeanRetreatRate > results[j].MeanRetreatRate
	})
}
