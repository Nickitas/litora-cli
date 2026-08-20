package mesh

import "math"

// EvaluateQuality вычисляет метрики без обращения к внутренним оценкам Gmsh,
// чтобы все алгоритмы сравнивались одной и той же процедурой.
func EvaluateQuality(domain PreparedDomain, generated Mesh, targetEdgeMeters float64) QualityMetrics {
	metrics := QualityMetrics{
		ReferenceAreaKM2:                  domain.ReferenceAreaM2 / 1_000_000,
		SimplifiedBoundaryAreaKM2:         domain.SimplifiedAreaM2 / 1_000_000,
		CumulativeFeatureAreaDeviationKM2: domain.CumulativeFeatureDeviationM2 / 1_000_000,
		CellCount:                         len(generated.Cells),
		QuadCount:                         generated.QuadCount,
		TriangleCount:                     generated.TriangleCount,
		MinEdgeMeters:                     math.Inf(1),
	}

	qualityHistogram := [101]int64{}
	var meshAreaM2, edgeSum, qualitySum float64
	var edgeCount, qualityCount int
	for _, cell := range generated.Cells {
		points := cellPoints(generated.Nodes, cell)
		if len(points) < 3 {
			continue
		}
		meshAreaM2 += math.Abs(signedRingArea(closeMetricRing(points)))
		for index := range points {
			length := pointDistance(points[index], points[(index+1)%len(points)])
			edgeSum += length
			edgeCount++
			if length < metrics.MinEdgeMeters {
				metrics.MinEdgeMeters = length
			}
			if length > metrics.MaxEdgeMeters {
				metrics.MaxEdgeMeters = length
			}
		}
		quality := cellQuality(points)
		qualitySum += quality
		qualityCount++
		bucket := int(math.Round(quality * 100))
		if bucket < 0 {
			bucket = 0
		}
		if bucket > 100 {
			bucket = 100
		}
		qualityHistogram[bucket]++
	}
	if math.IsInf(metrics.MinEdgeMeters, 1) {
		metrics.MinEdgeMeters = 0
	}
	if edgeCount > 0 {
		metrics.MeanEdgeMeters = edgeSum / float64(edgeCount)
	}
	if qualityCount > 0 {
		metrics.MeanCellQuality = qualitySum / float64(qualityCount)
		metrics.P05CellQuality = histogramPercentile(qualityHistogram, int64(qualityCount), 0.05)
	}
	if len(generated.Cells) > 0 {
		metrics.QuadSharePercent = 100 * float64(generated.QuadCount) / float64(len(generated.Cells))
	}

	metrics.MeshAreaKM2 = meshAreaM2 / 1_000_000
	metrics.MeshAreaDeviationKM2 = math.Abs(meshAreaM2-domain.ReferenceAreaM2) / 1_000_000
	metrics.BoundaryRMSMeters, metrics.BoundaryHausdorffMeters = boundaryDistances(domain.SimplifiedRings, generated)

	meshClosureError := math.Abs(meshAreaM2 - domain.SimplifiedAreaM2)
	coastalReferenceArea := domain.BoundaryLengthM * targetEdgeMeters
	if coastalReferenceArea > 0 {
		metrics.CoastalAreaDeviationPercent = 100 * (domain.CumulativeFeatureDeviationM2 + meshClosureError) / coastalReferenceArea
	}
	areaFit := 1 - clamp01(metrics.CoastalAreaDeviationPercent/100)
	boundaryFit := 1 - clamp01(metrics.BoundaryRMSMeters/targetEdgeMeters)
	quadFit := clamp01(metrics.QuadSharePercent / 100)
	metrics.CompositeScore = 100 * (0.55*areaFit + 0.20*boundaryFit + 0.20*clamp01(metrics.MeanCellQuality) + 0.05*quadFit)
	return metrics
}

func cellPoints(nodes []Point, cell Cell) []Point {
	result := make([]Point, 0, cell.NodeCount)
	for index := 0; index < cell.NodeCount; index++ {
		node := cell.Nodes[index]
		if node <= 0 || node >= len(nodes) {
			return nil
		}
		result = append(result, nodes[node])
	}
	return result
}

func cellQuality(points []Point) float64 {
	if len(points) != 4 {
		return 0
	}
	minEdge, maxEdge := math.Inf(1), 0.0
	maxAngleDeviation := 0.0
	for index := range points {
		previous := points[(index+len(points)-1)%len(points)]
		current := points[index]
		next := points[(index+1)%len(points)]
		edge := pointDistance(current, next)
		if edge < minEdge {
			minEdge = edge
		}
		if edge > maxEdge {
			maxEdge = edge
		}
		angle := cornerAngleDegrees(previous, current, next)
		deviation := math.Abs(angle - 90)
		if deviation > maxAngleDeviation {
			maxAngleDeviation = deviation
		}
	}
	if maxEdge <= 0 || math.IsInf(minEdge, 1) {
		return 0
	}
	aspectQuality := minEdge / maxEdge
	angleQuality := clamp01(1 - maxAngleDeviation/90)
	return math.Sqrt(aspectQuality * angleQuality)
}

func cornerAngleDegrees(previous, current, next Point) float64 {
	ax, ay := previous.X-current.X, previous.Y-current.Y
	bx, by := next.X-current.X, next.Y-current.Y
	denominator := math.Hypot(ax, ay) * math.Hypot(bx, by)
	if denominator <= 0 {
		return 0
	}
	cosine := (ax*bx + ay*by) / denominator
	cosine = math.Max(-1, math.Min(1, cosine))
	return math.Acos(cosine) * 180 / math.Pi
}

func histogramPercentile(histogram [101]int64, count int64, percentile float64) float64 {
	threshold := int64(math.Ceil(float64(count) * percentile))
	if threshold < 1 {
		threshold = 1
	}
	var cumulative int64
	for index, value := range histogram {
		cumulative += value
		if cumulative >= threshold {
			return float64(index) / 100
		}
	}
	return 1
}

func boundaryDistances(originalRings [][]Point, generated Mesh) (rms, hausdorff float64) {
	originalSegments := ringSegments(originalRings)
	meshSegments := make([][2]Point, 0, len(generated.BoundaryEdges))
	for _, edge := range generated.BoundaryEdges {
		if edge[0] <= 0 || edge[0] >= len(generated.Nodes) || edge[1] <= 0 || edge[1] >= len(generated.Nodes) {
			continue
		}
		meshSegments = append(meshSegments, [2]Point{generated.Nodes[edge[0]], generated.Nodes[edge[1]]})
	}
	if len(originalSegments) == 0 || len(meshSegments) == 0 {
		return 0, 0
	}

	var sumSquares float64
	count := 0
	for _, point := range sampledSegmentPoints(originalSegments, 2000) {
		distance := distanceToSegments(point, meshSegments)
		sumSquares += distance * distance
		count++
		if distance > hausdorff {
			hausdorff = distance
		}
	}
	for _, point := range sampledSegmentPoints(meshSegments, 2000) {
		distance := distanceToSegments(point, originalSegments)
		sumSquares += distance * distance
		count++
		if distance > hausdorff {
			hausdorff = distance
		}
	}
	if count > 0 {
		rms = math.Sqrt(sumSquares / float64(count))
	}
	return rms, hausdorff
}

func ringSegments(rings [][]Point) [][2]Point {
	var result [][2]Point
	for _, ring := range rings {
		for index := 1; index < len(ring); index++ {
			result = append(result, [2]Point{ring[index-1], ring[index]})
		}
		if len(ring) > 2 && ring[0] != ring[len(ring)-1] {
			result = append(result, [2]Point{ring[len(ring)-1], ring[0]})
		}
	}
	return result
}

func sampledSegmentPoints(segments [][2]Point, limit int) []Point {
	stride := 1
	if len(segments) > limit {
		stride = int(math.Ceil(float64(len(segments)) / float64(limit)))
	}
	result := make([]Point, 0, min(len(segments), limit))
	for index := 0; index < len(segments); index += stride {
		segment := segments[index]
		result = append(result, Point{X: (segment[0].X + segment[1].X) / 2, Y: (segment[0].Y + segment[1].Y) / 2})
	}
	return result
}

func distanceToSegments(point Point, segments [][2]Point) float64 {
	minimum := math.Inf(1)
	for _, segment := range segments {
		if distance := pointSegmentDistance(point, segment[0], segment[1]); distance < minimum {
			minimum = distance
		}
	}
	return minimum
}

func pointSegmentDistance(point, a, b Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	denominator := dx*dx + dy*dy
	if denominator <= 0 {
		return pointDistance(point, a)
	}
	t := ((point.X-a.X)*dx + (point.Y-a.Y)*dy) / denominator
	t = math.Max(0, math.Min(1, t))
	return pointDistance(point, Point{X: a.X + t*dx, Y: a.Y + t*dy})
}

func clamp01(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}
