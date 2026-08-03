package benchmark

import (
	"math"
	"sort"

	"coastal-geometry/internal/domain/geometry"
)

// Hotspot представляет сегмент побережья со значительной эрозией
type Hotspot struct {
	Center          geometry.LatLon `json:"center"`
	StartIdx        int             `json:"start_idx"`
	EndIdx          int             `json:"end_idx"`
	MeanRetreatRate float64         `json:"mean_retreat_rate_m_per_year"`
	MaxRetreatRate  float64         `json:"max_retreat_rate_m_per_year"`
	LengthKm        float64         `json:"length_km"`
	Rank            int             `json:"rank"` // 1 = самый активный
}

// SegmentRate представляет модельную скорость отступления для каждого сегмента побережья
type SegmentRate struct {
	Index          int             `json:"index"`
	Center         geometry.LatLon `json:"center"`
	RetreatRate    float64         `json:"retreat_rate_m_per_year"`
	ObservedRate   float64         `json:"observed_rate_m_per_year,omitempty"`
	HasObservation bool            `json:"has_observation"`
	DistanceKm     float64         `json:"distance_km_to_nearest_observation,omitempty"`
}

// SegmentRates возвращает модельные скорости отступления для всех сегментов побережья
// используя одиночный прогон модели с заданными параметрами
func SegmentRates(site BenchmarkSite, config CalibrationConfig, strength, waveDir float64) []SegmentRate {
	steps := int(float64(config.TotalYears) / config.YearsPerStep)
	if steps < 1 {
		steps = 1
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

	// Вычисляем отступление для каждой точки
	retreats := make([]float64, len(initial))
	for i := range initial {
		retreats[i] = computeSegmentRetreat(initial, final, i) / float64(config.TotalYears)
	}

	// Формируем скорости сегментов
	rates := make([]SegmentRate, len(initial))
	for i := range initial {
		// Проверяем, есть ли рядом наблюдения
		var obsRate float64
		hasObs := false
		var minDist float64 = math.Inf(1)
		for _, obs := range site.ObservedErosion {
			d := haversineKm(initial[i], obs.LatLon)
			if d < minDist {
				minDist = d
				if d < 2.0 { // в пределах 2 км
					obsRate = obs.ShorelineChangeRate
					hasObs = true
				}
			}
		}

		rates[i] = SegmentRate{
			Index:          i,
			Center:         initial[i],
			RetreatRate:    retreats[i],
			ObservedRate:   obsRate,
			HasObservation: hasObs,
		}
		if hasObs {
			rates[i].DistanceKm = minDist
		}
	}

	return rates
}

// FindHotspots выделяет топ-N участков активной эрозии вдоль побережья
// Участки - это непрерывные сегменты с высокой скоростью отступления (выше порога)
func FindHotspots(rates []SegmentRate, coastline []geometry.LatLon, topN int, thresholdPercentile float64) []Hotspot {
	if len(rates) < 2 || topN < 1 {
		return nil
	}
	if thresholdPercentile <= 0 || thresholdPercentile > 1 {
		thresholdPercentile = 0.75
	}

	// Сортируем скорости отступления для нахождения порога
	sortedRates := make([]float64, len(rates))
	for i, r := range rates {
		sortedRates[i] = r.RetreatRate
	}
	sort.Float64s(sortedRates)
	thresholdIdx := int(float64(len(sortedRates)-1) * thresholdPercentile)
	threshold := sortedRates[thresholdIdx]

	// Находим непрерывные сегменты выше порога
	var hotspots []Hotspot
	i := 0
	for i < len(rates) {
		if rates[i].RetreatRate <= threshold {
			i++
			continue
		}

		// Начало участка активной эрозии
		start := i
		maxRate := rates[i].RetreatRate
		var sumRate float64
		for i < len(rates) && rates[i].RetreatRate > threshold {
			sumRate += rates[i].RetreatRate
			if rates[i].RetreatRate > maxRate {
				maxRate = rates[i].RetreatRate
			}
			i++
		}
		end := i - 1

		// Вычисляем свойства участка
		hotspots = append(hotspots, buildHotspot(coastline, rates, start, end, sumRate, maxRate))
	}

	// Сортируем по средней скорости отступления по убыванию
	sort.Slice(hotspots, func(a, b int) bool {
		return hotspots[a].MeanRetreatRate > hotspots[b].MeanRetreatRate
	})

	// Присваиваем рейтинги и ограничиваем топ-N
	for i := range hotspots {
		hotspots[i].Rank = i + 1
	}
	if len(hotspots) > topN {
		hotspots = hotspots[:topN]
	}
	return hotspots
}

func buildHotspot(coastline []geometry.LatLon, rates []SegmentRate, start, end int, sumRate, maxRate float64) Hotspot {
	n := end - start + 1
	meanRate := sumRate / float64(n)

	// Центр - это средняя точка
	centerIdx := (start + end) / 2
	center := coastline[centerIdx]

	// Длина вдоль побережья
	var lengthM float64
	for i := start; i < end && i+1 < len(coastline); i++ {
		lengthM += haversineKm(coastline[i], coastline[i+1]) * 1000
	}

	return Hotspot{
		Center:          center,
		StartIdx:        start,
		EndIdx:          end,
		MeanRetreatRate: meanRate,
		MaxRetreatRate:  maxRate,
		LengthKm:        lengthM / 1000,
	}
}
