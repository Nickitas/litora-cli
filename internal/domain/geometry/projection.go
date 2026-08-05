package geometry

import "math"

// LocalMetricProjection — локальная равнопромежуточная метрическая проекция.
// Референсная точка задаётся центроидом входной геометрии, а масштабы по
// широте и долготе вычисляются по эллипсоиду WGS 84.
type LocalMetricProjection struct {
	ReferenceLat             float64
	ReferenceLon             float64
	MetersPerDegreeLatitude  float64
	MetersPerDegreeLongitude float64
}

// NewLocalMetricProjection создаёт единую метрическую систему координат для
// набора географических точек.
func NewLocalMetricProjection(points []LatLon) LocalMetricProjection {
	if len(points) == 0 {
		return LocalMetricProjection{
			MetersPerDegreeLatitude:  111132.92,
			MetersPerDegreeLongitude: 111412.84,
		}
	}

	var referenceLat, referenceLon float64
	for _, point := range points {
		referenceLat += point.Lat
		referenceLon += point.Lon
	}
	referenceLat /= float64(len(points))
	referenceLon /= float64(len(points))

	return LocalMetricProjection{
		ReferenceLat:             referenceLat,
		ReferenceLon:             referenceLon,
		MetersPerDegreeLatitude:  metersPerDegreeLatitude(referenceLat),
		MetersPerDegreeLongitude: metersPerDegreeLongitude(referenceLat),
	}
}

// Project переводит географическую точку в локальные координаты в метрах.
func (projection LocalMetricProjection) Project(point LatLon) Vector2D {
	return Vector2D{
		X: (point.Lon - projection.ReferenceLon) * projection.MetersPerDegreeLongitude,
		Y: (point.Lat - projection.ReferenceLat) * projection.MetersPerDegreeLatitude,
	}
}

// Unproject возвращает локальную метрическую точку в географические координаты.
func (projection LocalMetricProjection) Unproject(point Vector2D) LatLon {
	return LatLon{
		Lat: projection.ReferenceLat + point.Y/projection.MetersPerDegreeLatitude,
		Lon: projection.ReferenceLon + point.X/projection.MetersPerDegreeLongitude,
	}
}

// metersPerDegreeLatitude возвращает длину градуса широты на эллипсоиде WGS 84.
func metersPerDegreeLatitude(latitude float64) float64 {
	phi := latitude * math.Pi / 180
	return 111132.92 - 559.82*math.Cos(2*phi) + 1.175*math.Cos(4*phi) - 0.0023*math.Cos(6*phi)
}

// metersPerDegreeLongitude возвращает длину градуса долготы на эллипсоиде WGS 84.
func metersPerDegreeLongitude(latitude float64) float64 {
	phi := latitude * math.Pi / 180
	return 111412.84*math.Cos(phi) - 93.5*math.Cos(3*phi) + 0.118*math.Cos(5*phi)
}
