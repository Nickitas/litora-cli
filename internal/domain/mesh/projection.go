package mesh

import (
	"math"

	"coastal-geometry/internal/domain/geometry"
)

const equalAreaEarthRadiusMeters = 6_371_008.8

// EqualAreaProjection реализует сферическую азимутальную равноплощадную
// проекцию Ламберта с центром в среднем положении точек водоёма.
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
		X: equalAreaEarthRadiusMeters * k * math.Cos(phi) * math.Sin(delta),
		Y: equalAreaEarthRadiusMeters * k * (math.Cos(phi0)*math.Sin(phi) - math.Sin(phi0)*math.Cos(phi)*math.Cos(delta)),
	}
}

// Description возвращает русскоязычное описание системы координат отчёта.
func (projection EqualAreaProjection) Description() string {
	return "сферическая азимутальная равноплощадная проекция Ламберта (LAEA), метры"
}

func openRing(points []geometry.LatLon) []geometry.LatLon {
	if len(points) > 1 && points[0] == points[len(points)-1] {
		return points[:len(points)-1]
	}
	return points
}
