package geometry

import (
	"math"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geo"
)

// EarthRadiusKM — радиус Земли в километрах
const EarthRadiusKM = 6371.0

// Haversine возвращает расстояние по большому кругу в километрах между двумя точками.
// Использует расчёт расстояния из библиотеки orb на основе формулы гаверсинуса.
func Haversine(a, b LatLon) float64 {
	// geo.Distance возвращает метры, конвертируем в километры
	return geo.Distance(ToORB(a), ToORB(b)) / 1000
}

// PolylineLength возвращает общую длину полилинии в километрах с использованием orb/geo.
func PolylineLength(points []LatLon) float64 {
	if len(points) < 2 {
		return 0
	}

	lineString := ToORBLineString(points)
	// geo.Length возвращает метры, конвертируем в километры
	return geo.Length(orb.LineString(lineString)) / 1000
}

// Area возвращает площадь полигона в квадратных километрах с использованием orb/geo.
// Если полилиния не замкнута, она замыкается соединением последней точки с первой.
func Area(points []LatLon) float64 {
	if len(points) < 3 {
		return 0
	}

	// Обеспечиваем замыкание
	ring := ToORBLineString(points)
	if len(ring) == 0 {
		return 0
	}

	// Проверяем, замкнут ли контур, если нет — замыкаем
	if ring[0] != ring[len(ring)-1] {
		ring = append(ring, ring[0])
	}

	// geo.Area возвращает квадратные метры
	areaMeters2 := geo.Area(orb.Polygon{orb.Ring(ring)})
	return areaMeters2 / 1_000_000 // м² -> км²
}

// SignedArea возвращает знаковую площадь полигона в квадратных километрах.
// Положительная для полигонов против часовой стрелки, отрицательная для по часовой.
func SignedArea(points []LatLon) float64 {
	if len(points) < 3 {
		return 0
	}

	ring := ToORBLineString(points)
	if len(ring) == 0 {
		return 0
	}

	if ring[0] != ring[len(ring)-1] {
		ring = append(ring, ring[0])
	}

	// geo.SignedArea возвращает квадратные метры
	areaMeters2 := geo.SignedArea(orb.Ring(ring))
	return areaMeters2 / 1_000_000 // м² -> км²
}

// projectToMetersLocal проецирует точки на локальную плоскость в метрах
func projectToMetersLocal(points []LatLon) []pointXY {
	if len(points) == 0 {
		return nil
	}

	refLat := 0.0
	refLon := 0.0
	for _, p := range points {
		refLat += p.Lat
		refLon += p.Lon
	}
	refLat /= float64(len(points))
	refLon /= float64(len(points))

	metersPerDegLat := 111194.9
	metersPerDegLon := metersPerDegLat * math.Cos(refLat*math.Pi/180.0)
	if math.Abs(metersPerDegLon) < 1e-9 {
		metersPerDegLon = metersPerDegLat
	}

	projected := make([]pointXY, len(points))
	for i, p := range points {
		projected[i] = pointXY{
			X: (p.Lon - refLon) * metersPerDegLon,
			Y: (p.Lat - refLat) * metersPerDegLat,
		}
	}

	// Обеспечиваем замыкание для расчёта площади
	if points[0] != points[len(points)-1] {
		projected = append(projected, projected[0])
	}

	return projected
}
