package geometry

import "math"

const EarthRadiusKM = 6371.0

func Haversine(a, b LatLon) float64 {
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1)*math.Cos(lat2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return EarthRadiusKM * c
}
