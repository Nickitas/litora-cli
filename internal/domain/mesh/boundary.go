package mesh

import (
	"fmt"
	"math"

	"coastal-geometry/internal/domain/geometry"
)

// PrepareDomain упрощает все кольца с единым метрическим допуском и переводит
// их в равноплощадную систему координат. Первое кольцо считается внешним,
// последующие — островами, исключаемыми из поверхности воды.
func PrepareDomain(outer []geometry.LatLon, holes [][]geometry.LatLon, toleranceMeters float64) (PreparedDomain, error) {
	if toleranceMeters <= 0 {
		return PreparedDomain{}, fmt.Errorf("детализация береговой линии должна быть положительной")
	}
	outer = closeGeoRing(outer)
	if len(outer) < 4 {
		return PreparedDomain{}, fmt.Errorf("внешнее кольцо должно содержать минимум три вершины")
	}

	geoRings := make([][]geometry.LatLon, 0, len(holes)+1)
	geoRings = append(geoRings, outer)
	for _, hole := range holes {
		hole = closeGeoRing(hole)
		if len(hole) >= 4 {
			geoRings = append(geoRings, hole)
		}
	}
	projection := NewEqualAreaProjection(geoRings)

	result := PreparedDomain{Projection: projection}
	retainedGeoRings := make([][]geometry.LatLon, 0, len(geoRings))
	retainedOriginalMetric := make([][]Point, 0, len(geoRings))
	for ringIndex, ring := range geoRings {
		originalMetric := projectRing(projection, ring)
		result.OriginalRings = append(result.OriginalRings, originalMetric)
		result.OriginalPointCount += len(ring)
		result.BoundaryLengthM += ringLength(originalMetric)

		// Остров, площадь которого меньше квадрата заданной детализации,
		// не может быть устойчиво представлен ячейками этого масштаба. Его
		// площадь учитывается как явная потеря береговой особенности.
		if ringIndex > 0 && math.Abs(signedRingArea(originalMetric)) < toleranceMeters*toleranceMeters {
			result.CumulativeFeatureDeviationM2 += math.Abs(signedRingArea(originalMetric))
			continue
		}
		retainedGeoRings = append(retainedGeoRings, ring)
		retainedOriginalMetric = append(retainedOriginalMetric, originalMetric)
	}

	// Внешнее кольцо нельзя исключить, поэтому при самопересечении его допуск
	// постепенно уменьшается. Остров, который при заданном масштабе пересекает
	// упрощённый берег или сам себя, считается неразрешённой особенностью: он
	// исключается, а вся его площадь добавляется к отклонению. Так узкий пролив
	// не заставляет незаметно повышать детализацию всей акватории.
	outerGeo, outerMetric, effectiveTolerance := simplifySimpleRing(retainedGeoRings[0], toleranceMeters, projection, true)
	if len(outerMetric) == 0 {
		return PreparedDomain{}, fmt.Errorf("внешнее кольцо не удалось упростить без самопересечений")
	}
	result.EffectiveBoundaryToleranceMeters = effectiveTolerance
	result.SimplifiedRings = append(result.SimplifiedRings, outerMetric)
	result.SimplifiedPointCount += len(outerGeo)
	result.CumulativeFeatureDeviationM2 += cumulativeChordAreaDeviation(retainedOriginalMetric[0], outerMetric)

	for index := 1; index < len(retainedGeoRings); index++ {
		holeGeo, holeMetric, _ := simplifySimpleRing(retainedGeoRings[index], toleranceMeters, projection, false)
		if len(holeMetric) == 0 || ringsIntersect(outerMetric, holeMetric) || !pointInMetricRing(openMetricRing(holeMetric)[0], outerMetric) || intersectsAcceptedHole(result.SimplifiedRings[1:], holeMetric) {
			result.CumulativeFeatureDeviationM2 += math.Abs(signedRingArea(retainedOriginalMetric[index]))
			continue
		}
		result.SimplifiedRings = append(result.SimplifiedRings, holeMetric)
		result.SimplifiedPointCount += len(holeGeo)
		result.CumulativeFeatureDeviationM2 += cumulativeChordAreaDeviation(retainedOriginalMetric[index], holeMetric)
	}

	result.ReferenceAreaM2 = domainArea(result.OriginalRings)
	result.SimplifiedAreaM2 = domainArea(result.SimplifiedRings)
	if result.ReferenceAreaM2 <= 0 || result.SimplifiedAreaM2 <= 0 {
		return PreparedDomain{}, fmt.Errorf("площадь подготовленного водоёма должна быть положительной")
	}
	return result, nil
}

func simplifySimpleRing(ring []geometry.LatLon, toleranceMeters float64, projection EqualAreaProjection, allowRefinement bool) ([]geometry.LatLon, []Point, float64) {
	effectiveTolerance := toleranceMeters
	attempts := 1
	if allowRefinement {
		attempts = 13
	}
	for attempt := 0; attempt < attempts; attempt++ {
		simplified := geometry.SimplifyPolylineWithTolerance(ring, toleranceMeters).Points
		simplified = closeGeoRing(simplified)
		simplified = removeShortGeoEdges(simplified, effectiveTolerance/2, projection)
		simplified = repairRingTopology(ring, simplified, projection)
		metric := projectRing(projection, simplified)
		if len(simplified) >= 4 && !ringSelfIntersects(metric) {
			return simplified, metric, effectiveTolerance
		}
		effectiveTolerance /= 2
		toleranceMeters = effectiveTolerance
	}
	return nil, nil, 0
}

// repairRingTopology возвращает часть исходных вершин в пересекающиеся хорды.
// Добавление вершин не увеличивает погрешность упрощения, зато сохраняет
// заданный метрический допуск и корректную последовательность границы.
func repairRingTopology(original, simplified []geometry.LatLon, projection EqualAreaProjection) []geometry.LatLon {
	originalOpen := openRing(original)
	simplifiedOpen := openRing(simplified)
	if len(originalOpen) < 3 || len(simplifiedOpen) < 3 {
		return nil
	}
	indices := originalIndices(originalOpen, simplifiedOpen)
	if len(indices) != len(simplifiedOpen) {
		return nil
	}
	metric := projectRing(projection, simplifiedOpen)
	for additions := 0; additions < len(originalOpen); additions++ {
		firstEdge, secondEdge, crossed := firstRingIntersection(metric)
		if !crossed {
			return closeGeoRing(simplifiedOpen)
		}
		firstCandidate, firstSpan := splitCandidate(originalOpen, indices, firstEdge, projection)
		secondCandidate, secondSpan := splitCandidate(originalOpen, indices, secondEdge, projection)
		insertEdge, candidate := firstEdge, firstCandidate
		if secondSpan > firstSpan {
			insertEdge, candidate = secondEdge, secondCandidate
		}
		if candidate < 0 {
			return nil
		}
		insertAt := insertEdge + 1
		simplifiedOpen = append(simplifiedOpen, geometry.LatLon{})
		copy(simplifiedOpen[insertAt+1:], simplifiedOpen[insertAt:])
		simplifiedOpen[insertAt] = originalOpen[candidate]
		indices = append(indices, 0)
		copy(indices[insertAt+1:], indices[insertAt:])
		indices[insertAt] = candidate
		metric = projectRing(projection, simplifiedOpen)
	}
	return nil
}

func originalIndices(original, simplified []geometry.LatLon) []int {
	indices := make([]int, 0, len(simplified))
	searchFrom := 0
	for _, point := range simplified {
		found := -1
		for index := searchFrom; index < len(original); index++ {
			if original[index] == point {
				found = index
				break
			}
		}
		if found < 0 {
			return nil
		}
		indices = append(indices, found)
		searchFrom = found + 1
	}
	return indices
}

func firstRingIntersection(open []Point) (int, int, bool) {
	for first := 0; first < len(open); first++ {
		firstNext := (first + 1) % len(open)
		for second := first + 1; second < len(open); second++ {
			secondNext := (second + 1) % len(open)
			if firstNext == second || secondNext == first {
				continue
			}
			if metricSegmentsIntersect(open[first], open[firstNext], open[second], open[secondNext]) {
				return first, second, true
			}
		}
	}
	return 0, 0, false
}

func splitCandidate(original []geometry.LatLon, indices []int, edge int, projection EqualAreaProjection) (int, int) {
	start := indices[edge]
	end := indices[(edge+1)%len(indices)]
	if end <= start {
		end += len(original)
	}
	span := end - start - 1
	if span <= 0 {
		return -1, span
	}
	a := projection.Project(original[start])
	b := projection.Project(original[end%len(original)])
	bestIndex := -1
	bestDistance := -1.0
	for cyclic := start + 1; cyclic < end; cyclic++ {
		index := cyclic % len(original)
		if distance := squaredPointSegmentDistance(projection.Project(original[index]), a, b); distance > bestDistance {
			bestDistance = distance
			bestIndex = index
		}
	}
	return bestIndex, span
}

func squaredPointSegmentDistance(point, a, b Point) float64 {
	dx, dy := b.X-a.X, b.Y-a.Y
	if dx == 0 && dy == 0 {
		distanceX, distanceY := point.X-a.X, point.Y-a.Y
		return distanceX*distanceX + distanceY*distanceY
	}
	position := ((point.X-a.X)*dx + (point.Y-a.Y)*dy) / (dx*dx + dy*dy)
	position = math.Max(0, math.Min(1, position))
	projection := Point{X: a.X + position*dx, Y: a.Y + position*dy}
	distanceX, distanceY := point.X-projection.X, point.Y-projection.Y
	return distanceX*distanceX + distanceY*distanceY
}

func intersectsAcceptedHole(accepted [][]Point, candidate []Point) bool {
	for _, hole := range accepted {
		if ringsIntersect(hole, candidate) || pointInMetricRing(openMetricRing(candidate)[0], hole) || pointInMetricRing(openMetricRing(hole)[0], candidate) {
			return true
		}
	}
	return false
}

func validDomainTopology(rings [][]Point) bool {
	if len(rings) == 0 || ringSelfIntersects(rings[0]) {
		return false
	}
	outer := rings[0]
	for index := 1; index < len(rings); index++ {
		hole := rings[index]
		if ringSelfIntersects(hole) || ringsIntersect(outer, hole) || !pointInMetricRing(openMetricRing(hole)[0], outer) {
			return false
		}
		for previous := 1; previous < index; previous++ {
			if ringsIntersect(rings[previous], hole) || pointInMetricRing(openMetricRing(hole)[0], rings[previous]) || pointInMetricRing(openMetricRing(rings[previous])[0], hole) {
				return false
			}
		}
	}
	return true
}

func ringSelfIntersects(ring []Point) bool {
	open := openMetricRing(ring)
	if len(open) < 3 {
		return true
	}
	for first := 0; first < len(open); first++ {
		firstNext := (first + 1) % len(open)
		for second := first + 1; second < len(open); second++ {
			secondNext := (second + 1) % len(open)
			if first == second || firstNext == second || secondNext == first {
				continue
			}
			if metricSegmentsIntersect(open[first], open[firstNext], open[second], open[secondNext]) {
				return true
			}
		}
	}
	return false
}

func ringsIntersect(firstRing, secondRing []Point) bool {
	first := openMetricRing(firstRing)
	second := openMetricRing(secondRing)
	for firstIndex := range first {
		firstNext := (firstIndex + 1) % len(first)
		for secondIndex := range second {
			secondNext := (secondIndex + 1) % len(second)
			if metricSegmentsIntersect(first[firstIndex], first[firstNext], second[secondIndex], second[secondNext]) {
				return true
			}
		}
	}
	return false
}

func metricSegmentsIntersect(a, b, c, d Point) bool {
	const epsilon = 1e-7
	orientation := func(first, second, third Point) float64 {
		return (second.X-first.X)*(third.Y-first.Y) - (second.Y-first.Y)*(third.X-first.X)
	}
	onSegment := func(first, point, second Point) bool {
		return point.X <= math.Max(first.X, second.X)+epsilon && point.X >= math.Min(first.X, second.X)-epsilon &&
			point.Y <= math.Max(first.Y, second.Y)+epsilon && point.Y >= math.Min(first.Y, second.Y)-epsilon
	}
	o1, o2 := orientation(a, b, c), orientation(a, b, d)
	o3, o4 := orientation(c, d, a), orientation(c, d, b)
	if ((o1 > epsilon && o2 < -epsilon) || (o1 < -epsilon && o2 > epsilon)) &&
		((o3 > epsilon && o4 < -epsilon) || (o3 < -epsilon && o4 > epsilon)) {
		return true
	}
	return math.Abs(o1) <= epsilon && onSegment(a, c, b) ||
		math.Abs(o2) <= epsilon && onSegment(a, d, b) ||
		math.Abs(o3) <= epsilon && onSegment(c, a, d) ||
		math.Abs(o4) <= epsilon && onSegment(c, b, d)
}

func pointInMetricRing(point Point, ring []Point) bool {
	open := openMetricRing(ring)
	inside := false
	for index, previous := 0, len(open)-1; index < len(open); previous, index = index, index+1 {
		a, b := open[index], open[previous]
		if (a.Y > point.Y) != (b.Y > point.Y) && point.X < (b.X-a.X)*(point.Y-a.Y)/(b.Y-a.Y)+a.X {
			inside = !inside
		}
	}
	return inside
}

func removeShortGeoEdges(ring []geometry.LatLon, minimumMeters float64, projection EqualAreaProjection) []geometry.LatLon {
	open := openRing(ring)
	if len(open) <= 3 || minimumMeters <= 0 {
		return closeGeoRing(open)
	}
	kept := make([]geometry.LatLon, 0, len(open))
	kept = append(kept, open[0])
	for _, point := range open[1:] {
		if pointDistance(projection.Project(kept[len(kept)-1]), projection.Project(point)) >= minimumMeters {
			kept = append(kept, point)
		}
	}
	if len(kept) > 3 && pointDistance(projection.Project(kept[len(kept)-1]), projection.Project(kept[0])) < minimumMeters {
		kept = kept[:len(kept)-1]
	}
	return closeGeoRing(kept)
}

// EstimatedCellCount оценивает порядок числа четырёхугольников до запуска
// внешнего генератора и позволяет заранее ограничить слишком большой расчёт.
func (domain PreparedDomain) EstimatedCellCount(targetEdgeMeters float64) int64 {
	if targetEdgeMeters <= 0 {
		return 0
	}
	// Исходная триангуляция строится с увеличенным шагом, после чего каждый
	// треугольник превращается в три четырёхугольника. Для планирования памяти
	// используется консервативный коэффициент 1,5 ячейки на квадрат шага.
	return int64(math.Ceil(1.5 * domain.SimplifiedAreaM2 / (targetEdgeMeters * targetEdgeMeters)))
}

func closeGeoRing(points []geometry.LatLon) []geometry.LatLon {
	result := append([]geometry.LatLon(nil), points...)
	if len(result) > 0 && result[0] != result[len(result)-1] {
		result = append(result, result[0])
	}
	return result
}

func projectRing(projection EqualAreaProjection, ring []geometry.LatLon) []Point {
	result := make([]Point, len(ring))
	for index, point := range ring {
		result[index] = projection.Project(point)
	}
	return result
}

func signedRingArea(ring []Point) float64 {
	if len(ring) < 3 {
		return 0
	}
	var sum float64
	for index := 0; index < len(ring)-1; index++ {
		sum += ring[index].X*ring[index+1].Y - ring[index+1].X*ring[index].Y
	}
	if ring[0] != ring[len(ring)-1] {
		sum += ring[len(ring)-1].X*ring[0].Y - ring[0].X*ring[len(ring)-1].Y
	}
	return sum / 2
}

func domainArea(rings [][]Point) float64 {
	if len(rings) == 0 {
		return 0
	}
	area := math.Abs(signedRingArea(rings[0]))
	for _, ring := range rings[1:] {
		area -= math.Abs(signedRingArea(ring))
	}
	return area
}

func ringLength(ring []Point) float64 {
	var result float64
	for index := 1; index < len(ring); index++ {
		result += pointDistance(ring[index-1], ring[index])
	}
	return result
}

func pointDistance(a, b Point) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

// cumulativeChordAreaDeviation суммирует абсолютные площади между каждым
// упрощённым ребром и соответствующим исходным фрагментом берега. Благодаря
// модулю локальные потери заливов не компенсируются приростом площади кос.
func cumulativeChordAreaDeviation(original, simplified []Point) float64 {
	original = openMetricRing(original)
	simplified = openMetricRing(simplified)
	if len(original) < 3 || len(simplified) < 2 {
		return 0
	}

	indices := make([]int, 0, len(simplified))
	searchFrom := 0
	for _, point := range simplified {
		found := -1
		for index := searchFrom; index < len(original); index++ {
			if original[index] == point {
				found = index
				break
			}
		}
		if found < 0 {
			return math.Abs(math.Abs(signedRingArea(closeMetricRing(original))) - math.Abs(signedRingArea(closeMetricRing(simplified))))
		}
		indices = append(indices, found)
		searchFrom = found + 1
	}

	var deviation float64
	for index := 1; index < len(indices); index++ {
		arc := append([]Point(nil), original[indices[index-1]:indices[index]+1]...)
		deviation += math.Abs(signedRingArea(closeMetricRing(arc)))
	}
	if last := indices[len(indices)-1]; last < len(original)-1 {
		arc := append([]Point(nil), original[last:]...)
		arc = append(arc, original[0])
		deviation += math.Abs(signedRingArea(closeMetricRing(arc)))
	}
	return deviation
}

func openMetricRing(points []Point) []Point {
	if len(points) > 1 && points[0] == points[len(points)-1] {
		return points[:len(points)-1]
	}
	return points
}

func closeMetricRing(points []Point) []Point {
	result := append([]Point(nil), points...)
	if len(result) > 0 && result[0] != result[len(result)-1] {
		result = append(result, result[0])
	}
	return result
}
