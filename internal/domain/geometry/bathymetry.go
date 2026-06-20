package geometry

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// BathymetryPoint represents a single depth measurement at a location.
// Depth is negative for underwater (e.g., -100 = 100 meters below sea level).
type BathymetryPoint struct {
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Depth float64 `json:"depth"`
}

// BathymetryGrid stores depth data in a regular latitude-longitude grid.
type BathymetryGrid struct {
	Points     map[string]BathymetryPoint
	Resolution float64
	bounds     struct {
		MinLat, MaxLat float64
		MinLon, MaxLon float64
	}
}

// BathymetryLoadOptions controls how bathymetry data is loaded.
type BathymetryLoadOptions struct {
	LocalPath  string
	RemoteURL  string
	CachePath  string
	Refresh    bool
	Resolution float64
}

// BathymetryLoadResult contains metadata from loading bathymetry.
type BathymetryLoadResult struct {
	Grid         *BathymetryGrid
	PointCount   int
	Resolution   float64
	Source       string
	LoadWarnings []string
}

// LoadBathymetryFromJSON loads bathymetry data from a JSON byte slice.
// The JSON should be an array of objects with lat, lon, and depth fields.
func LoadBathymetryFromJSON(data []byte, options BathymetryLoadOptions) (*BathymetryGrid, error) {
	if options.Resolution <= 0 {
		options.Resolution = 0.01
	}

	var rawPoints []BathymetryPoint
	if err := json.Unmarshal(data, &rawPoints); err != nil {
		return nil, fmt.Errorf("unmarshal bathymetry JSON: %w", err)
	}

	if len(rawPoints) == 0 {
		return nil, fmt.Errorf("bathymetry data is empty")
	}

	// Валидация точек
	if err := validateBathymetryPoints(rawPoints); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	grid, err := BuildGrid(rawPoints, options.Resolution)
	if err != nil {
		return nil, fmt.Errorf("build bathymetry grid: %w", err)
	}

	// Валидация построенной сетки
	if err := validateBathymetryGrid(grid); err != nil {
		return nil, fmt.Errorf("grid validation failed: %w", err)
	}

	return grid, nil
}

// BuildGrid creates a BathymetryGrid from a slice of points.
func BuildGrid(points []BathymetryPoint, resolution float64) (*BathymetryGrid, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("cannot build grid from empty points")
	}
	if resolution <= 0 {
		return nil, fmt.Errorf("resolution must be positive, got %f", resolution)
	}

	grid := &BathymetryGrid{
		Points:     make(map[string]BathymetryPoint),
		Resolution: resolution,
	}

	grid.bounds.MinLat = points[0].Lat
	grid.bounds.MaxLat = points[0].Lat
	grid.bounds.MinLon = points[0].Lon
	grid.bounds.MaxLon = points[0].Lon

	for _, p := range points {
		if p.Lat < grid.bounds.MinLat {
			grid.bounds.MinLat = p.Lat
		}
		if p.Lat > grid.bounds.MaxLat {
			grid.bounds.MaxLat = p.Lat
		}
		if p.Lon < grid.bounds.MinLon {
			grid.bounds.MinLon = p.Lon
		}
		if p.Lon > grid.bounds.MaxLon {
			grid.bounds.MaxLon = p.Lon
		}

		key := gridKey(p.Lat, p.Lon, resolution)
		grid.Points[key] = p
	}

	return grid, nil
}

// InterpolateDepth returns the depth at a given location using bilinear interpolation.
// Returns an error if the location is outside the grid bounds.
func (g *BathymetryGrid) InterpolateDepth(lat, lon float64) (float64, error) {
	if lat < g.bounds.MinLat || lat > g.bounds.MaxLat ||
		lon < g.bounds.MinLon || lon > g.bounds.MaxLon {
		return 0, fmt.Errorf("coordinates (%f, %f) outside grid bounds [%f, %f] x [%f, %f]",
			lat, lon, g.bounds.MinLat, g.bounds.MaxLat, g.bounds.MinLon, g.bounds.MaxLon)
	}

	lat0 := math.Floor(lat/g.Resolution) * g.Resolution
	lon0 := math.Floor(lon/g.Resolution) * g.Resolution
	lat1 := lat0 + g.Resolution
	lon1 := lon0 + g.Resolution

	key00 := gridKey(lat0, lon0, g.Resolution)
	key01 := gridKey(lat0, lon1, g.Resolution)
	key10 := gridKey(lat1, lon0, g.Resolution)
	key11 := gridKey(lat1, lon1, g.Resolution)

	p00, ok00 := g.Points[key00]
	p01, ok01 := g.Points[key01]
	p10, ok10 := g.Points[key10]
	p11, ok11 := g.Points[key11]

	if !ok00 || !ok01 || !ok10 || !ok11 {
		return 0, fmt.Errorf("missing neighbor points for interpolation at (%f, %f)", lat, lon)
	}

	fx := (lon - lon0) / g.Resolution
	fy := (lat - lat0) / g.Resolution

	i0 := bilinearInterpolate1D(p00.Depth, p01.Depth, fx)
	i1 := bilinearInterpolate1D(p10.Depth, p11.Depth, fx)
	depth := bilinearInterpolate1D(i0, i1, fy)

	return depth, nil
}

func gridKey(lat, lon, resolution float64) string {
	latIdx := math.Floor(lat / resolution)
	lonIdx := math.Floor(lon / resolution)
	return fmt.Sprintf("%s,%s", strconv.FormatFloat(latIdx, 'f', -1, 64), strconv.FormatFloat(lonIdx, 'f', -1, 64))
}

func bilinearInterpolate1D(v0, v1, t float64) float64 {
	return v0 + t*(v1-v0)
}

func validateBathymetryPoints(points []BathymetryPoint) error {
	// Константы для Чёрного моря с tolerant margin для учёта погрешности на границах
	const (
		minLat = 40.0
		maxLat = 47.0
		minLon = 27.0
		maxLon = 42.5 // Расширено для GEBCO данных (формально 42.0)
		margin = 0.1 // Tolerant margin для boundary issues (градусы)
		maxDepth = -3000.0 // Максимальная глубина с запасом
	)

	for i, p := range points {
		// Проверка координат с tolerant margin
		if p.Lat < minLat-margin || p.Lat > maxLat+margin {
			return fmt.Errorf("point %d: latitude %.4f outside Black Sea bounds [%.1f, %.1f]", i, p.Lat, minLat, maxLat)
		}
		if p.Lon < minLon-margin || p.Lon > maxLon+margin {
			return fmt.Errorf("point %d: longitude %.4f outside Black Sea bounds [%.1f, %.1f]", i, p.Lon, minLon, maxLon)
		}

		// Проверка глубины
		if p.Depth > 0 {
			return fmt.Errorf("point %d: positive depth %.2f (should be underwater, negative)", i, p.Depth)
		}
		if p.Depth < maxDepth {
			return fmt.Errorf("point %d: depth %.2f exceeds realistic Black Sea depth (max ~-2212m)", i, p.Depth)
		}

		// Проверка на NaN/Inf
		if math.IsNaN(p.Lat) || math.IsInf(p.Lat, 0) {
			return fmt.Errorf("point %d: latitude is NaN/Inf", i)
		}
		if math.IsNaN(p.Lon) || math.IsInf(p.Lon, 0) {
			return fmt.Errorf("point %d: longitude is NaN/Inf", i)
		}
		if math.IsNaN(p.Depth) || math.IsInf(p.Depth, 0) {
			return fmt.Errorf("point %d: depth is NaN/Inf", i)
		}
	}

	return nil
}

func validateBathymetryGrid(grid *BathymetryGrid) error {
	if len(grid.Points) == 0 {
		return fmt.Errorf("grid has no points")
	}

	// Проверка разрешения
	if grid.Resolution <= 0 {
		return fmt.Errorf("invalid resolution: %f", grid.Resolution)
	}
	if grid.Resolution > 0.1 {
		return fmt.Errorf("resolution too coarse: %f (max 0.1°)", grid.Resolution)
	}

	return nil
}

// physicalDepthFactor calculates erosion factor based on water depth.
// Deeper water allows more wave energy, resulting in higher erosion.
// depthMeters: negative for underwater (e.g., -100 = 100m below sea level)
func physicalDepthFactor(depthMeters, fetchMeters, depthScale float64) float64 {
	effectiveDepth := math.Max(0, -depthMeters)
	return 1 - math.Exp(-effectiveDepth/depthScale)
}
