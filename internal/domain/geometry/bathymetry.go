package geometry

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"coastal-geometry/internal/domain/blacksea"
)

// BathymetryPoint представляет одно измерение глубины в точке.
// Глубина отрицательна для подводных участков (напр., -100 = 100 метров под уровнем моря).
type BathymetryPoint struct {
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Depth float64 `json:"depth"`
}

// BathymetryGrid хранит данные глубин в регулярной сетке широта-долгота.
type BathymetryGrid struct {
	Points     map[string]BathymetryPoint
	Resolution float64
	bounds     struct {
		MinLat, MaxLat float64
		MinLon, MaxLon float64
	}
}

// BathymetryLoadOptions управляет загрузкой данных батиметрии.
type BathymetryLoadOptions struct {
	LocalPath  string
	RemoteURL  string
	CachePath  string
	Refresh    bool
	Resolution float64
}

// BathymetryLoadResult содержит метаданные из загрузки батиметрии.
type BathymetryLoadResult struct {
	Grid         *BathymetryGrid
	PointCount   int
	Resolution   float64
	Source       string
	LoadWarnings []string
}

// LoadBathymetryFromJSON загружает данные батиметрии из JSON-массива байтов.
// JSON должен быть массивом объектов с полями lat, lon и depth, а паспорт
// происхождения должен храниться рядом в отдельном файле .metadata.json.
func LoadBathymetryFromJSON(data []byte, options BathymetryLoadOptions) (*BathymetryGrid, error) {
	if options.Resolution <= 0 {
		options.Resolution = 0.01
	}

	var rawPoints []BathymetryPoint
	if err := json.Unmarshal(data, &rawPoints); err != nil {
		return nil, fmt.Errorf("батиметрия без отмели JSON: %w", err)
	}

	if len(rawPoints) == 0 {
		return nil, fmt.Errorf("данные батиметрии пусты")
	}

	// Валидация точек
	if err := validateBathymetryPoints(rawPoints); err != nil {
		return nil, fmt.Errorf("не удалось выполнить проверку: %w", err)
	}

	grid, err := BuildGrid(rawPoints, options.Resolution)
	if err != nil {
		return nil, fmt.Errorf("построение батиметрической сетки: %w", err)
	}

	// Валидация построенной сетки
	if err := validateBathymetryGrid(grid); err != nil {
		return nil, fmt.Errorf("не удалось выполнить проверку сетки: %w", err)
	}

	return grid, nil
}

// BuildGrid создаёт BathymetryGrid из набора точек.
func BuildGrid(points []BathymetryPoint, resolution float64) (*BathymetryGrid, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("невозможно построить сетку из пустых точек")
	}
	if resolution <= 0 {
		return nil, fmt.Errorf("решение должно быть положительным, получено %f", resolution)
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

// InterpolateDepth возвращает глубину в заданной точке с использованием билинейной интерполяции.
// Возвращает ошибку, если точка находится за пределами сетки.
func (g *BathymetryGrid) InterpolateDepth(lat, lon float64) (float64, error) {
	if lat < g.bounds.MinLat || lat > g.bounds.MaxLat ||
		lon < g.bounds.MinLon || lon > g.bounds.MaxLon {
		return 0, fmt.Errorf("координаты (%f, %ff) вне границ сетки [%f, %ff] x [%f, %ff]",
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
		return 0, fmt.Errorf("недостающие соседние точки для интерполяции в (%f, %ff)", lat, lon)
	}

	fx := (lon - lon0) / g.Resolution
	fy := (lat - lat0) / g.Resolution

	i0 := bilinearInterpolate1D(p00.Depth, p01.Depth, fx)
	i1 := bilinearInterpolate1D(p10.Depth, p11.Depth, fx)
	depth := bilinearInterpolate1D(i0, i1, fy)

	return depth, nil
}

// SampleDepth возвращает билинейно интерполированную глубину. Если около
// береговой линии не хватает соседних ячеек, функция ищет ближайшее реальное
// измерение, но только в пределах maxDistanceMeters. Расстояние 0 означает
// интерполяцию; положительное значение должно быть учтено при анализе точности.
func (g *BathymetryGrid) SampleDepth(lat, lon, maxDistanceMeters float64) (depth, distanceMeters float64, err error) {
	if g == nil {
		return 0, 0, fmt.Errorf("батиметрическая сетка не задана")
	}
	depth, err = g.InterpolateDepth(lat, lon)
	if err == nil {
		return depth, 0, nil
	}
	if maxDistanceMeters <= 0 || g.Resolution <= 0 {
		return 0, 0, err
	}

	latRadius := int(math.Ceil(maxDistanceMeters / metersPerDegLat / g.Resolution))
	metersPerDegLon := metersPerDegLat * math.Cos(lat*math.Pi/180)
	if math.Abs(metersPerDegLon) < 1e-9 {
		metersPerDegLon = metersPerDegLat
	}
	lonRadius := int(math.Ceil(maxDistanceMeters / metersPerDegLon / g.Resolution))
	latIndex := int(math.Floor(lat / g.Resolution))
	lonIndex := int(math.Floor(lon / g.Resolution))
	bestDistance := math.Inf(1)
	bestDepth := 0.0
	for latitude := latIndex - latRadius; latitude <= latIndex+latRadius; latitude++ {
		for longitude := lonIndex - lonRadius; longitude <= lonIndex+lonRadius; longitude++ {
			point, ok := g.Points[fmt.Sprintf("%d,%d", latitude, longitude)]
			if !ok {
				continue
			}
			distance := Haversine(LatLon{Lat: lat, Lon: lon}, LatLon{Lat: point.Lat, Lon: point.Lon}) * 1000
			if distance < bestDistance {
				bestDistance = distance
				bestDepth = point.Depth
			}
		}
	}
	if bestDistance <= maxDistanceMeters {
		return bestDepth, bestDistance, nil
	}
	return 0, 0, fmt.Errorf("%w; ближайшая точка не найдена в радиусе %.0f м", err, maxDistanceMeters)
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
	// Предел выбран с запасом относительно максимальной глубины Чёрного моря.
	// Значения хранятся как отрицательные отметки elevation_m.
	const maxBlackSeaDepth = -3000.0

	for i, p := range points {
		if math.IsNaN(p.Lat) || math.IsInf(p.Lat, 0) {
			return fmt.Errorf("точка %d: широта равна NaN/Inf", i)
		}
		if math.IsNaN(p.Lon) || math.IsInf(p.Lon, 0) {
			return fmt.Errorf("точка %d: долгота равна NaN/Inf", i)
		}
		if math.IsNaN(p.Depth) || math.IsInf(p.Depth, 0) {
			return fmt.Errorf("точка %d: глубина равна NaN/Inf", i)
		}
		if p.Lat < -90 || p.Lat > 90 {
			return fmt.Errorf("точка %d: недопустимая широта %.4f", i, p.Lat)
		}
		if p.Lon < -180 || p.Lon > 180 {
			return fmt.Errorf("точка %d: недопустимая долгота %.4f", i, p.Lon)
		}
		if !blacksea.Contains(p.Lat, p.Lon) {
			return fmt.Errorf(
				"точка %d: координаты (%.4f, %.4f) вне области Чёрного моря [%.1f, %.1f] × [%.1f, %.1f]",
				i, p.Lat, p.Lon,
				blacksea.MinLatitude, blacksea.MaxLatitude,
				blacksea.MinLongitude, blacksea.MaxLongitude,
			)
		}

		// Проверка глубины
		if p.Depth > 0 {
			return fmt.Errorf("точка %d: положительная глубина %.2f (должна быть под водой, отрицательная)", i, p.Depth)
		}
		if p.Depth < maxBlackSeaDepth {
			return fmt.Errorf("точка %d: отметка %.2f м ниже допустимого предела Чёрного моря %.0f м", i, p.Depth, maxBlackSeaDepth)
		}
	}

	return nil
}

func validateBathymetryGrid(grid *BathymetryGrid) error {
	if len(grid.Points) == 0 {
		return fmt.Errorf("сетка не имеет точек")
	}

	// Проверка разрешения
	if grid.Resolution <= 0 {
		return fmt.Errorf("недопустимое разрешение: %f", grid.Resolution)
	}
	if grid.Resolution > 0.1 {
		return fmt.Errorf("слишком грубое разрешение: %f (не более 0,1°)", grid.Resolution)
	}

	return nil
}

// physicalDepthFactor рассчитывает коэффициент эрозии на основе глубины воды.
// Более глубокие воды пропускают больше энергии волн, что приводит к более интенсивной эрозии.
// Показания глубиномеров: отрицательные для подводных (например, -100 = 100 м ниже уровня моря).
func physicalDepthFactor(depthMeters, fetchMeters, depthScale float64) float64 {
	effectiveDepth := math.Max(0, -depthMeters)
	return 1 - math.Exp(-effectiveDepth/depthScale)
}
