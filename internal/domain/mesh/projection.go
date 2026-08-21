package mesh

import (
	"fmt"
	"math"

	"coastal-geometry/internal/domain/geometry"
)

const equalAreaEarthRadiusMeters = 6_371_008.8

// EqualAreaProjection реализует сферическую азимутальную равноплощадную
// проекцию Ламберта с центром в среднем положении точек Чёрного моря.
type EqualAreaProjection struct {
	ReferenceLat float64 `json:"reference_lat"`
	ReferenceLon float64 `json:"reference_lon"`
}

// NewEqualAreaProjection выбирает центр равноплощадной проекции.
func NewEqualAreaProjection(rings [][]geometry.LatLon) EqualAreaProjection {
	var lat, lon float64
	count := 0
	if len(rings) > 0 {
		for _, point := range openRing(rings[0]) {
			lat += point.Lat
			lon += point.Lon
			count++
		}
	}
	if count == 0 {
		return EqualAreaProjection{}
	}
	return EqualAreaProjection{ReferenceLat: lat / float64(count), ReferenceLon: lon / float64(count)}
}

// Project переводит WGS 84 координату в локальную равноплощадную плоскость.
func (projection EqualAreaProjection) Project(point geometry.LatLon) Point {
	phi0 := projection.ReferenceLat * math.Pi / 180
	lambda0 := projection.ReferenceLon * math.Pi / 180
	phi := point.Lat * math.Pi / 180
	lambda := point.Lon * math.Pi / 180
	delta := lambda - lambda0
	denominator := 1 + math.Sin(phi0)*math.Sin(phi) + math.Cos(phi0)*math.Cos(phi)*math.Cos(delta)
	if denominator <= 1e-12 {
		return Point{}
	}
	k := math.Sqrt(2 / denominator)
	return Point{
		X:                        equalAreaEarthRadiusMeters * k * math.Cos(phi) * math.Sin(delta),
		Y:                        equalAreaEarthRadiusMeters * k * (math.Cos(phi0)*math.Sin(phi) - math.Sin(phi0)*math.Cos(phi)*math.Cos(delta)),
		LongitudeDeg:             point.Lon,
		LatitudeDeg:              point.Lat,
		GeographicCoordinatesSet: true,
	}
}

// Unproject переводит координату локальной плоскости LAEA обратно в WGS 84.
// Вычисление использует ту же сферическую модель и радиус, что и Project.
func (projection EqualAreaProjection) Unproject(point Point) (geometry.LatLon, error) {
	if !validProjectionReference(projection) {
		return geometry.LatLon{}, fmt.Errorf("некорректный центр проекции LAEA: %.6f°, %.6f°", projection.ReferenceLat, projection.ReferenceLon)
	}
	if math.IsNaN(point.X) || math.IsInf(point.X, 0) || math.IsNaN(point.Y) || math.IsInf(point.Y, 0) {
		return geometry.LatLon{}, fmt.Errorf("координаты LAEA должны быть конечными")
	}

	rho := math.Hypot(point.X, point.Y)
	if rho > 2*equalAreaEarthRadiusMeters+1e-6 {
		return geometry.LatLon{}, fmt.Errorf("координата LAEA находится вне допустимого круга проекции")
	}
	if rho <= 1e-12 {
		return geometry.LatLon{Lat: projection.ReferenceLat, Lon: normalizeLongitude(projection.ReferenceLon)}, nil
	}

	ratio := math.Min(1, rho/(2*equalAreaEarthRadiusMeters))
	c := 2 * math.Asin(ratio)
	sinC, cosC := math.Sin(c), math.Cos(c)
	phi0 := projection.ReferenceLat * math.Pi / 180
	lambda0 := projection.ReferenceLon * math.Pi / 180
	sinPhi0, cosPhi0 := math.Sin(phi0), math.Cos(phi0)

	sinPhi := cosC*sinPhi0 + point.Y*sinC*cosPhi0/rho
	sinPhi = math.Max(-1, math.Min(1, sinPhi))
	phi := math.Asin(sinPhi)
	lambda := lambda0 + math.Atan2(
		point.X*sinC,
		rho*cosPhi0*cosC-point.Y*sinPhi0*sinC,
	)

	result := geometry.LatLon{
		Lat: phi * 180 / math.Pi,
		Lon: normalizeLongitude(lambda * 180 / math.Pi),
	}
	if !result.IsValid() || math.IsNaN(result.Lat) || math.IsNaN(result.Lon) {
		return geometry.LatLon{}, fmt.Errorf("обратная проекция LAEA дала некорректную координату")
	}
	return result, nil
}

// AssignGeographicCoordinates вычисляет WGS 84 для каждого фактического узла
// сетки. Нулевой элемент сохраняется как служебный, поскольку идентификаторы
// узлов MSH начинаются с единицы.
func (projection EqualAreaProjection) AssignGeographicCoordinates(nodes []Point) error {
	if len(nodes) <= 1 {
		return fmt.Errorf("сетка не содержит фактических узлов")
	}
	for nodeID := 1; nodeID < len(nodes); nodeID++ {
		coordinate, err := projection.Unproject(nodes[nodeID])
		if err != nil {
			return fmt.Errorf("обратная проекция узла %d: %w", nodeID, err)
		}
		nodes[nodeID].LongitudeDeg = coordinate.Lon
		nodes[nodeID].LatitudeDeg = coordinate.Lat
		nodes[nodeID].GeographicCoordinatesSet = true
	}
	return nil
}

// RoundTripErrorMeters возвращает геодезическое отклонение после цикла
// WGS 84 → LAEA → WGS 84 в метрах.
func (projection EqualAreaProjection) RoundTripErrorMeters(point geometry.LatLon) (float64, error) {
	recovered, err := projection.Unproject(projection.Project(point))
	if err != nil {
		return 0, err
	}
	return geometry.Haversine(point, recovered) * 1000, nil
}

// Description возвращает русскоязычное описание системы координат отчёта.
func (projection EqualAreaProjection) Description() string {
	return "сферическая азимутальная равноплощадная проекция Ламберта (LAEA), метры"
}

func validProjectionReference(projection EqualAreaProjection) bool {
	return !math.IsNaN(projection.ReferenceLat) && !math.IsInf(projection.ReferenceLat, 0) &&
		!math.IsNaN(projection.ReferenceLon) && !math.IsInf(projection.ReferenceLon, 0) &&
		projection.ReferenceLat >= -90 && projection.ReferenceLat <= 90 &&
		projection.ReferenceLon >= -180 && projection.ReferenceLon <= 180
}

func normalizeLongitude(longitude float64) float64 {
	longitude = math.Mod(longitude+180, 360)
	if longitude < 0 {
		longitude += 360
	}
	return longitude - 180
}

func openRing(points []geometry.LatLon) []geometry.LatLon {
	if len(points) > 1 && points[0] == points[len(points)-1] {
		return points[:len(points)-1]
	}
	return points
}
