package geometry

import (
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geo"
)

// PolylineLength возвращает общую длину полилинии в километрах с использованием orb/geo.
func PolylineLength(points []LatLon) float64 {
	if len(points) < 2 {
		return 0
	}

	lineString := ToORBLineString(points)
	// geo.Length возвращает метры, конвертируем в километры
	return geo.Length(orb.LineString(lineString)) / 1000
}
