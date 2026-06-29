package geometry

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// LithologyProfile represents the complete lithology profile for a region
type LithologyProfile struct {
	Metadata  LithologyMetadata        `json:"metadata"`
	Points    []LithologyPoint         `json:"points"`
	Classes   map[string]LithologyClass `json:"classes"`
	Baselines map[string]ErosionBaseline `json:"erosion_baselines"`
}

// LithologyMetadata contains profile metadata
type LithologyMetadata struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Created    string   `json:"created"`
	Sources    []string `json:"sources"`
	Resolution float64  `json:"resolution"`
	Bounds     Bounds   `json:"bounds"`
	Regions    []string `json:"regions"`
	Note       string   `json:"note,omitempty"`
}

// Bounds represents geographic bounds
type Bounds struct {
	MinLat float64 `json:"min_lat"`
	MaxLat float64 `json:"max_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLon float64 `json:"max_lon"`
}

// LithologyPoint represents a single lithology measurement point
type LithologyPoint struct {
	Lat         float64  `json:"lat"`
	Lon         float64  `json:"lon"`
	Region      string   `json:"region"`
	Lithology   string   `json:"lithology_class"`
	Resistance  float64  `json:"resistance"`
	Color       string   `json:"color"`
	Description string   `json:"description"`
	Confidence  string   `json:"confidence"`
	Source      string   `json:"source"`
	ErosionObserved *float64 `json:"erosion_observed,omitempty"`
	Note        string   `json:"note,omitempty"`
	Dynamic     bool     `json:"dynamic,omitempty"`
}

// LithologyClass defines a rock type class
type LithologyClass struct {
	Resistance  float64  `json:"resistance"`
	Color       string   `json:"color"`
	Description string   `json:"description"`
	ErosionRange []float64 `json:"erosion_range,omitempty"`
	Dynamic     bool    `json:"dynamic,omitempty"`
	Note        string  `json:"note,omitempty"`
}

// ErosionBaseline defines baseline erosion rates for resistance classes
type ErosionBaseline struct {
	ResistanceRange [2]float64             `json:"resistance_range"`
	ErosionMYear    map[string]float64    `json:"erosion_m_year"`
	Description    string                 `json:"description"`
	Note           string                 `json:"note,omitempty"`
}

// LoadLithologyProfile loads a lithology profile from JSON data
func LoadLithologyProfile(data []byte) (*LithologyProfile, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty lithology data")
	}

	var profile LithologyProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("unmarshal lithology profile: %w", err)
	}

	// Валидация
	if err := validateLithologyProfile(&profile); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &profile, nil
}

// LoadLithologyProfileFromFile loads a lithology profile from a file
func LoadLithologyProfileFromFile(path string) (*LithologyProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read lithology file %q: %w", path, err)
	}
	return LoadLithologyProfile(data)
}

// GetLithologyAt returns the lithology state at a given location using IDW interpolation
func (p *LithologyProfile) GetLithologyAt(lat, lon float64) LithologyState {
	// Проверка границ
	if lat < p.Metadata.Bounds.MinLat || lat > p.Metadata.Bounds.MaxLat ||
		lon < p.Metadata.Bounds.MinLon || lon > p.Metadata.Bounds.MaxLon {
		// Out of bounds → return default
		return p.getDefaultLithology()
	}

	// Если нет точек → default
	if len(p.Points) == 0 {
		return p.getDefaultLithology()
	}

	// Для 1 точки используем её напрямую
	if len(p.Points) == 1 {
		point := &p.Points[0]
		if _, ok := p.Classes[point.Lithology]; !ok {
			return p.getDefaultLithology()
		}
		return LithologyState{
			Class:       point.Lithology,
			Resistance:  point.Resistance,
			Color:       point.Color,
			Description: point.Description,
		}
	}

	// IDW интерполяция по N ближайшим точкам
	return p.interpolateLithologyIDW(lat, lon)
}

// interpolateLithologyIDW interpolates lithology using inverse distance weighting
func (p *LithologyProfile) interpolateLithologyIDW(lat, lon float64) LithologyState {
	const maxPoints = 6     // максимальное число точек для интерполяции
	const power = 2.0       // степень для IDW (стандартное значение)

	// Найти ближайшие точки
	nearby := p.findNearbyPoints(lat, lon, maxPoints)
	if len(nearby) == 0 {
		return p.getDefaultLithology()
	}

	// Если только одна точка в радиусе
	if len(nearby) == 1 {
		point := nearby[0]
		if _, ok := p.Classes[point.Lithology]; !ok {
			return p.getDefaultLithology()
		}
		return LithologyState{
			Class:       point.Lithology,
			Resistance:  point.Resistance,
			Color:       point.Color,
			Description: point.Description,
		}
	}

	// Рассчитать веса IDW
	weights := make([]float64, len(nearby))
	weightSum := 0.0

	for i, point := range nearby {
		// Евклидово расстояние (градусы)
		dist := math.Sqrt(math.Pow(point.Lat-lat, 2) + math.Pow(point.Lon-lon, 2))

		// Избегаем деления на ноль
		if dist < 1e-6 {
			// Точно совпадает — используем эту точку
			if _, ok := p.Classes[point.Lithology]; ok {
				return LithologyState{
					Class:       point.Lithology,
					Resistance:  point.Resistance,
					Color:       point.Color,
					Description: point.Description,
				}
			}
			return p.getDefaultLithology()
		}

		// IDW вес: 1 / distance^power
		weights[i] = 1.0 / math.Pow(dist, power)
		weightSum += weights[i]
	}

	// Нормализовать веса
	for i := range weights {
		weights[i] /= weightSum
	}

	// Интерполировать resistance (взвешенное среднее)
	interpolatedResistance := 0.0
	for i, point := range nearby {
		interpolatedResistance += weights[i] * point.Resistance
	}

	// Для класса и цвета — взвешенное голосование (выбираем с максимальным весом)
	maxWeightIdx := 0
	for i := range weights {
		if weights[i] > weights[maxWeightIdx] {
			maxWeightIdx = i
		}
	}

	dominantPoint := nearby[maxWeightIdx]

	// Валидация класса
	if _, ok := p.Classes[dominantPoint.Lithology]; !ok {
		return p.getDefaultLithology()
	}

	return LithologyState{
		Class:       dominantPoint.Lithology,
		Resistance:  interpolatedResistance,
		Color:       dominantPoint.Color,
		Description: dominantPoint.Description,
	}
}

// findNearbyPoints finds the N closest lithology points to the given coordinates
func (p *LithologyProfile) findNearbyPoints(lat, lon float64, n int) []*LithologyPoint {
	if len(p.Points) == 0 {
		return nil
	}

	// Ограничиваем n числом доступных точек
	if n > len(p.Points) {
		n = len(p.Points)
	}

	// Создаём список с расстояниями
	type pointDist struct {
		point *LithologyPoint
		dist  float64
	}

	pointDists := make([]pointDist, len(p.Points))
	for i := range p.Points {
		dist := math.Sqrt(
			math.Pow(p.Points[i].Lat-lat, 2) + math.Pow(p.Points[i].Lon-lon, 2),
		)
		pointDists[i] = pointDist{point: &p.Points[i], dist: dist}
	}

	// Полная сортировка по расстоянию (simple selection sort)
	for i := 0; i < len(pointDists)-1; i++ {
		minIdx := i
		for j := i + 1; j < len(pointDists); j++ {
			if pointDists[j].dist < pointDists[minIdx].dist {
				minIdx = j
			}
		}
		pointDists[i], pointDists[minIdx] = pointDists[minIdx], pointDists[i]
	}

	// Вернуть первые n точек
	result := make([]*LithologyPoint, n)
	for i := 0; i < n; i++ {
		result[i] = pointDists[i].point
	}

	return result
}

// findClosestPoint finds the closest lithology point to the given coordinates
func (p *LithologyProfile) findClosestPoint(lat, lon float64) *LithologyPoint {
	nearby := p.findNearbyPoints(lat, lon, 1)
	if len(nearby) == 0 {
		return nil
	}
	return nearby[0]
}

// getDefaultLithology returns default lithology state
func (p *LithologyProfile) getDefaultLithology() LithologyState {
	defaultClass := p.getDefaultClass()
	return LithologyState{
		Class:       "limestone",
		Resistance:  defaultClass.Resistance,
		Color:       defaultClass.Color,
		Description: "default lithology",
	}
}

// getDefaultClass returns default lithology class
func (p *LithologyProfile) getDefaultClass() LithologyClass {
	// Попробовать найти limestone, иначе fallback
	if class, ok := p.Classes["limestone"]; ok {
		return class
	}

	// Fallback к reasonable defaults
	return LithologyClass{
		Resistance:  2.5,
		Color:       "#8b8b8b",
		Description: "fallback lithology",
	}
}

// validateLithologyProfile validates the lithology profile
func validateLithologyProfile(profile *LithologyProfile) error {
	// Проверка метаданных
	if profile.Metadata.Name == "" {
		return fmt.Errorf("missing profile name")
	}

	// Проверка границ
	if profile.Metadata.Bounds.MinLat >= profile.Metadata.Bounds.MaxLat {
		return fmt.Errorf("invalid latitude bounds")
	}
	if profile.Metadata.Bounds.MinLon >= profile.Metadata.Bounds.MaxLon {
		return fmt.Errorf("invalid longitude bounds")
	}

	// Проверка точек
	for i, point := range profile.Points {
		if point.Lat < -90 || point.Lat > 90 {
			return fmt.Errorf("point %d: invalid latitude %.4f", i, point.Lat)
		}
		if point.Lon < -180 || point.Lon > 180 {
			return fmt.Errorf("point %d: invalid longitude %.4f", i, point.Lon)
		}
		if point.Resistance <= 0 || point.Resistance > 20 {
			return fmt.Errorf("point %d: invalid resistance %.2f (expected 0-20)", i, point.Resistance)
		}
		if point.Lithology == "" {
			return fmt.Errorf("point %d: missing lithology class", i)
		}
	}

	// Проверка классов
	for name, class := range profile.Classes {
		if class.Resistance <= 0 {
			return fmt.Errorf("class %s: invalid resistance", name)
		}
		if class.Color == "" {
			return fmt.Errorf("class %s: missing color", name)
		}
	}

	return nil
}

// GetStatistics returns statistics about the lithology profile
func (p *LithologyProfile) GetStatistics() map[string]interface{} {
	stats := map[string]interface{}{
		"name":         p.Metadata.Name,
		"version":      p.Metadata.Version,
		"num_points":   len(p.Points),
		"num_classes":  len(p.Classes),
		"num_baselines": len(p.Baselines),
		"resolution":   p.Metadata.Resolution,
		"bounds":       p.Metadata.Bounds,
		"regions":      p.Metadata.Regions,
	}

	// Resistance statistics
	if len(p.Points) > 0 {
		minR := p.Points[0].Resistance
		maxR := p.Points[0].Resistance
		sumR := 0.0

		for _, point := range p.Points {
			if point.Resistance < minR {
				minR = point.Resistance
			}
			if point.Resistance > maxR {
				maxR = point.Resistance
			}
			sumR += point.Resistance
		}

		stats["resistance_min"] = minR
		stats["resistance_max"] = maxR
		stats["resistance_mean"] = sumR / float64(len(p.Points))
	}

	// Confidence distribution
	confidenceDist := make(map[string]int)
	for _, point := range p.Points {
		confidenceDist[point.Confidence]++
	}
	stats["confidence_distribution"] = confidenceDist

	return stats
}

// CreateDefaultBlackSeaProfile creates a default Black Sea lithology profile
// Used when no profile is provided
func CreateDefaultBlackSeaProfile() *LithologyProfile {
	return &LithologyProfile{
		Metadata: LithologyMetadata{
			Name:       "Default Black Sea Lithology",
			Version:    "1.0-fallback",
			Created:    "auto-generated",
			Sources:    []string{"fallback"},
			Resolution: 1.0, // грубое разрешение
			Bounds: Bounds{
				MinLat: 40.0,
				MaxLat: 47.0,
				MinLon: 27.0,
				MaxLon: 42.0,
			},
			Regions: []string{"crimea", "turkey", "bulgaria", "romania", "georgia", "russia"},
			Note:    "Fallback profile when no lithology data is available",
		},
		Points: []LithologyPoint{
			// Crimea (average)
			{Lat: 45.0, Lon: 34.5, Region: "crimea", Lithology: "limestone", Resistance: 4.0, Color: "#6b6b6b", Description: "Crimean limestone", Confidence: "low"},
			// Turkey (average)
			{Lat: 41.5, Lon: 40.0, Region: "turkey", Lithology: "volcanic", Resistance: 6.5, Color: "#4a4a4a", Description: "Pontic volcanic", Confidence: "low"},
			// Bulgaria (average)
			{Lat: 42.5, Lon: 28.0, Region: "bulgaria", Lithology: "limestone", Resistance: 4.0, Color: "#6b6b6b", Description: "Bulgarian limestone", Confidence: "low"},
			// Romania (average - soft)
			{Lat: 44.5, Lon: 29.0, Region: "romania", Lithology: "clay", Resistance: 1.2, Color: "#c4a484", Description: "Romanian clay", Confidence: "low"},
			// Georgia (average)
			{Lat: 42.5, Lon: 41.5, Region: "georgia", Lithology: "sedimentary", Resistance: 2.5, Color: "#8b8b8b", Description: "Caucasus sedimentary", Confidence: "low"},
		},
		Classes: map[string]LithologyClass{
			"limestone": {
				Resistance:  4.0,
				Color:       "#6b6b6b",
				Description: "Sarmatian/Neogene limestone",
			},
			"volcanic": {
				Resistance:  6.5,
				Color:       "#4a4a4a",
				Description: "Volcanic rocks",
			},
			"clay": {
				Resistance:  1.2,
				Color:       "#c4a484",
				Description: "Clayey formations",
			},
			"sedimentary": {
				Resistance:  2.5,
				Color:       "#8b8b8b",
				Description: "Sedimentary rocks",
			},
		},
		Baselines: map[string]ErosionBaseline{
			"very_soft": {
				ResistanceRange: [2]float64{0.8, 1.4},
				ErosionMYear: map[string]float64{
					"min": 5.0, "max": 12.0, "mean": 7.5,
				},
				Description: "Soft sediments — very rapid erosion",
			},
			"soft": {
				ResistanceRange: [2]float64{1.5, 2.4},
				ErosionMYear: map[string]float64{
					"min": 2.0, "max": 5.0, "mean": 3.5,
				},
				Description: "Consolidated sediments — rapid erosion",
			},
			"medium": {
				ResistanceRange: [2]float64{2.5, 3.9},
				ErosionMYear: map[string]float64{
					"min": 1.0, "max": 3.0, "mean": 2.0,
				},
				Description: "Sandstone, conglomerate — significant erosion",
			},
			"medium_hard": {
				ResistanceRange: [2]float64{4.0, 5.9},
				ErosionMYear: map[string]float64{
					"min": 0.5, "max": 2.0, "mean": 1.2,
				},
				Description: "Limestone, metamorphic — noticeable erosion",
			},
			"hard": {
				ResistanceRange: [2]float64{6.0, 7.9},
				ErosionMYear: map[string]float64{
					"min": 0.3, "max": 1.0, "mean": 0.6,
				},
				Description: "Volcanic rocks — moderate erosion",
			},
			"very_hard": {
				ResistanceRange: [2]float64{8.0, 10.0},
				ErosionMYear: map[string]float64{
					"min": 0.1, "max": 0.5, "mean": 0.3,
				},
				Description: "Serpentinite, granite — very slow erosion",
			},
		},
	}
}