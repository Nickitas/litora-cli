package geometry

func PolylineLength(points []LatLon) float64 {
	if len(points) < 2 {
		return 0
	}
	var total float64
	for i := 1; i < len(points); i++ {
		total += Haversine(points[i-1], points[i])
	}
	return total
}
