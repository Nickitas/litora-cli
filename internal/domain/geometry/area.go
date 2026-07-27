package geometry

import "math"

// Area returns polygon area in square kilometers using the shoelace formula.
// If the polyline is not closed, it is closed by connecting the last point to the first.
// Coordinates are projected to a local meters grid using mean latitude.
func Area(points []LatLon) float64 {
	if len(points) < 3 {
		return 0
	}

	projected := projectToMetersLocal(points)
	if len(projected) < 3 {
		return 0
	}

	areaMeters2 := 0.0
	last := projected[len(projected)-1]
	for _, p := range projected {
		areaMeters2 += (last.X*p.Y - p.X*last.Y)
		last = p
	}

	return math.Abs(areaMeters2) / 2 / 1_000_000 // m² -> km²
}

// projectToMetersLocal mirrors projectToMeters but is self-contained for area calc.
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

	metersPerDegLon := metersPerDegLat * math.Cos(refLat*math.Pi/180)
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

	// Ensure closure for area calculation
	if points[0] != points[len(points)-1] {
		projected = append(projected, projected[0])
	}

	return projected
}
