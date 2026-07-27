package geometry

import "math"

type SimplifyOptions struct {
	MaxPoints int
}

type SimplifyResult struct {
	Points           []LatLon
	OriginalCount    int
	SimplifiedCount  int
	ToleranceMeters  float64
	Applied          bool
	OriginalClosed   bool
	SimplifiedClosed bool
}

type pointXY struct {
	X float64
	Y float64
}

func SimplifyPolyline(points []LatLon, options SimplifyOptions) SimplifyResult {
	cloned := clonePoints(points)
	result := SimplifyResult{
		Points:          cloned,
		OriginalCount:   len(points),
		SimplifiedCount: len(points),
	}

	if len(points) < 3 || options.MaxPoints <= 0 || len(points) <= options.MaxPoints {
		result.OriginalClosed = isClosedPolyline(points)
		result.SimplifiedClosed = result.OriginalClosed
		return result
	}

	closed := isClosedPolyline(points)
	result.OriginalClosed = closed

	working := cloned
	target := options.MaxPoints
	minPoints := 2
	if closed {
		if len(working) <= 4 || target <= 4 {
			result.SimplifiedClosed = true
			return result
		}
		working = clonePoints(working[:len(working)-1])
		target--
		minPoints = 3
	}

	if target < minPoints {
		target = minPoints
	}
	if len(working) <= target {
		result.SimplifiedClosed = closed
		return result
	}

	projected := projectToMeters(working)
	diagonal := projectedDiagonal(projected)
	if diagonal <= 0 {
		result.SimplifiedClosed = closed
		return result
	}

	low := 0.0
	high := diagonal
	best := clonePoints(working)
	bestTolerance := 0.0

	for i := 0; i < 24; i++ {
		mid := (low + high) / 2
		simplified := simplifyWithTolerance(working, projected, mid)
		if len(simplified) > target {
			low = mid
			continue
		}
		if len(simplified) < minPoints {
			high = mid
			continue
		}

		best = simplified
		bestTolerance = mid
		high = mid
	}

	if len(best) == len(working) {
		result.SimplifiedClosed = closed
		return result
	}

	if closed {
		best = append(best, best[0])
	}

	result.Points = best
	result.SimplifiedCount = len(best)
	result.ToleranceMeters = bestTolerance
	result.Applied = true
	result.SimplifiedClosed = closed
	return result
}

func simplifyWithTolerance(points []LatLon, projected []pointXY, toleranceMeters float64) []LatLon {
	if len(points) < 3 || toleranceMeters <= 0 {
		return clonePoints(points)
	}

	keep := make([]bool, len(points))
	keep[0] = true
	keep[len(points)-1] = true
	markSimplifiedPoints(projected, keep, 0, len(points)-1, toleranceMeters*toleranceMeters)

	simplified := make([]LatLon, 0, len(points))
	for i, point := range points {
		if keep[i] {
			simplified = append(simplified, point)
		}
	}
	return simplified
}

func markSimplifiedPoints(projected []pointXY, keep []bool, start, end int, toleranceSquared float64) {
	if end-start < 2 {
		return
	}

	maxDistance := -1.0
	index := -1
	for i := start + 1; i < end; i++ {
		distance := squaredSegmentDistance(projected[i], projected[start], projected[end])
		if distance > maxDistance {
			maxDistance = distance
			index = i
		}
	}

	if index == -1 || maxDistance <= toleranceSquared {
		return
	}

	keep[index] = true
	markSimplifiedPoints(projected, keep, start, index, toleranceSquared)
	markSimplifiedPoints(projected, keep, index, end, toleranceSquared)
}

func squaredSegmentDistance(point, a, b pointXY) float64 {
	dx := b.X - a.X
	dy := b.Y - a.Y
	if dx == 0 && dy == 0 {
		return squaredDistance(point, a)
	}

	t := ((point.X-a.X)*dx + (point.Y-a.Y)*dy) / (dx*dx + dy*dy)
	switch {
	case t <= 0:
		return squaredDistance(point, a)
	case t >= 1:
		return squaredDistance(point, b)
	default:
		projection := pointXY{
			X: a.X + t*dx,
			Y: a.Y + t*dy,
		}
		return squaredDistance(point, projection)
	}
}

func squaredDistance(a, b pointXY) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}

func projectToMeters(points []LatLon) []pointXY {
	if len(points) == 0 {
		return nil
	}

	refLat := 0.0
	refLon := 0.0
	for _, point := range points {
		refLat += point.Lat
		refLon += point.Lon
	}
	refLat /= float64(len(points))
	refLon /= float64(len(points))

	metersPerDegLat := 111194.9
	metersPerDegLon := metersPerDegLat * math.Cos(refLat*math.Pi/180)
	if math.Abs(metersPerDegLon) < 1e-9 {
		metersPerDegLon = metersPerDegLat
	}

	projected := make([]pointXY, len(points))
	for i, point := range points {
		projected[i] = pointXY{
			X: (point.Lon - refLon) * metersPerDegLon,
			Y: (point.Lat - refLat) * metersPerDegLat,
		}
	}
	return projected
}

func projectedDiagonal(points []pointXY) float64 {
	if len(points) == 0 {
		return 0
	}

	minX, maxX := points[0].X, points[0].X
	minY, maxY := points[0].Y, points[0].Y
	for _, point := range points[1:] {
		if point.X < minX {
			minX = point.X
		}
		if point.X > maxX {
			maxX = point.X
		}
		if point.Y < minY {
			minY = point.Y
		}
		if point.Y > maxY {
			maxY = point.Y
		}
	}

	return math.Hypot(maxX-minX, maxY-minY)
}

func isClosedPolyline(points []LatLon) bool {
	if len(points) < 2 {
		return false
	}
	return points[0] == points[len(points)-1]
}

func clonePoints(points []LatLon) []LatLon {
	if len(points) == 0 {
		return nil
	}
	cloned := make([]LatLon, len(points))
	copy(cloned, points)
	return cloned
}
