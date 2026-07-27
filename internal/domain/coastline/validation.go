package coastline

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"coastal-geometry/internal/domain/geometry"
)

const longSegmentWarningKM = 450.0

type segmentIntersection struct {
	First  int
	Second int
}

func validateAndNormalizePoints(points []geometry.LatLon) ([]geometry.LatLon, ValidationReport, error) {
	report := ValidationReport{}

	deduped, removed := removeDuplicateCoordinates(points)
	if removed > 0 {
		report.Fixes = append(report.Fixes, fmt.Sprintf("удалены повторяющиеся координаты: %d", removed))
	}
	if len(deduped) < 2 {
		return nil, report, fmt.Errorf("после удаления дубликатов осталось меньше 2 точек")
	}

	best := chooseBestOrder(deduped)
	if !samePointOrder(deduped, best) {
		report.Fixes = append(report.Fixes, "точки автоматически переупорядочены по обходу контура")
	}

	intersections := findSelfIntersections(best)
	if len(intersections) > 0 {
		return nil, report, fmt.Errorf("полилиния имеет self-intersection: пересекаются сегменты %s", formatIntersections(intersections))
	}

	report.Warnings = append(report.Warnings, duplicateLocationWarnings(best)...)
	report.Warnings = append(report.Warnings, longSegmentWarnings(best, longSegmentWarningKM)...)

	return best, report, nil
}

func removeDuplicateCoordinates(points []geometry.LatLon) ([]geometry.LatLon, int) {
	seen := make(map[string]struct{}, len(points))
	result := make([]geometry.LatLon, 0, len(points))
	removed := 0

	for _, point := range points {
		key := pointKey(point)
		if _, ok := seen[key]; ok {
			removed++
			continue
		}
		seen[key] = struct{}{}
		result = append(result, point)
	}

	return result, removed
}

func chooseBestOrder(points []geometry.LatLon) []geometry.LatLon {
	candidates := [][]geometry.LatLon{
		slices.Clone(points),
		reversePoints(points),
	}

	for _, start := range candidateStartIndices(points) {
		candidate := greedyTraversal(points, start)
		candidates = append(candidates, candidate, reversePoints(candidate))
	}

	best := candidates[0]
	bestScore := scoreOrder(best)
	for _, candidate := range candidates[1:] {
		score := scoreOrder(candidate)
		if score.less(bestScore) {
			best = candidate
			bestScore = score
		}
	}

	return best
}

type orderScore struct {
	intersections int
	longSegments  int
	maxSegmentKM  float64
	totalLengthKM float64
}

func (s orderScore) less(other orderScore) bool {
	if s.intersections != other.intersections {
		return s.intersections < other.intersections
	}
	if s.longSegments != other.longSegments {
		return s.longSegments < other.longSegments
	}
	if math.Abs(s.maxSegmentKM-other.maxSegmentKM) > 1e-9 {
		return s.maxSegmentKM < other.maxSegmentKM
	}
	return s.totalLengthKM < other.totalLengthKM
}

func scoreOrder(points []geometry.LatLon) orderScore {
	var maxSegment float64
	var longSegments int
	var total float64

	for i := 1; i < len(points); i++ {
		length := geometry.Haversine(points[i-1], points[i])
		total += length
		if length > maxSegment {
			maxSegment = length
		}
		if length > longSegmentWarningKM {
			longSegments++
		}
	}

	return orderScore{
		intersections: len(findSelfIntersections(points)),
		longSegments:  longSegments,
		maxSegmentKM:  maxSegment,
		totalLengthKM: total,
	}
}

func candidateStartIndices(points []geometry.LatLon) []int {
	if len(points) == 0 {
		return nil
	}

	indices := []int{0}
	minLat, maxLat, minLon, maxLon := 0, 0, 0, 0
	for i := 1; i < len(points); i++ {
		if points[i].Lat < points[minLat].Lat {
			minLat = i
		}
		if points[i].Lat > points[maxLat].Lat {
			maxLat = i
		}
		if points[i].Lon < points[minLon].Lon {
			minLon = i
		}
		if points[i].Lon > points[maxLon].Lon {
			maxLon = i
		}
	}

	for _, idx := range []int{minLat, maxLat, minLon, maxLon} {
		seen := false
		for _, current := range indices {
			if current == idx {
				seen = true
				break
			}
		}
		if !seen {
			indices = append(indices, idx)
		}
	}

	return indices
}

func greedyTraversal(points []geometry.LatLon, start int) []geometry.LatLon {
	used := make([]bool, len(points))
	result := make([]geometry.LatLon, 0, len(points))
	current := start

	for len(result) < len(points) {
		result = append(result, points[current])
		used[current] = true

		next := -1
		bestDistance := math.MaxFloat64
		for i := range points {
			if used[i] {
				continue
			}
			distance := geometry.Haversine(points[current], points[i])
			if distance < bestDistance {
				bestDistance = distance
				next = i
			}
		}
		if next == -1 {
			break
		}
		current = next
	}

	return result
}

func reversePoints(points []geometry.LatLon) []geometry.LatLon {
	reversed := slices.Clone(points)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return reversed
}

func samePointOrder(a, b []geometry.LatLon) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if pointKey(a[i]) != pointKey(b[i]) {
			return false
		}
	}
	return true
}

func pointKey(point geometry.LatLon) string {
	return strconv.FormatFloat(point.Lat, 'f', 6, 64) + "|" + strconv.FormatFloat(point.Lon, 'f', 6, 64)
}

func duplicateLocationWarnings(points []geometry.LatLon) []string {
	if len(points) > 200 {
		return nil
	}

	counts := map[string]int{}
	for _, point := range points {
		name := getLocationName(point)
		if name != "—" {
			counts[name]++
		}
	}

	var warnings []string
	for name, count := range counts {
		if count > 1 {
			warnings = append(warnings, fmt.Sprintf("обнаружен повторяющийся ориентир %q: %d точек", name, count))
		}
	}
	slices.Sort(warnings)
	return warnings
}

func longSegmentWarnings(points []geometry.LatLon, thresholdKM float64) []string {
	var warnings []string
	for i := 1; i < len(points); i++ {
		length := geometry.Haversine(points[i-1], points[i])
		if length > thresholdKM {
			warnings = append(warnings, fmt.Sprintf("сегмент %d-%d имеет длину %.0f км, это больше порога %.0f км", i, i+1, length, thresholdKM))
		}
	}
	return warnings
}

func findSelfIntersections(points []geometry.LatLon) []segmentIntersection {
	var intersections []segmentIntersection
	for i := 0; i < len(points)-1; i++ {
		for j := i + 2; j < len(points)-1; j++ {
			if segmentsIntersect(points[i], points[i+1], points[j], points[j+1]) {
				intersections = append(intersections, segmentIntersection{First: i + 1, Second: j + 1})
			}
		}
	}
	return intersections
}

func segmentsIntersect(a, b, c, d geometry.LatLon) bool {
	const eps = 1e-9

	if samePoint(a, c) || samePoint(a, d) || samePoint(b, c) || samePoint(b, d) {
		return false
	}

	o1 := orientation(a, b, c)
	o2 := orientation(a, b, d)
	o3 := orientation(c, d, a)
	o4 := orientation(c, d, b)

	if o1*o2 < -eps && o3*o4 < -eps {
		return true
	}

	if math.Abs(o1) <= eps && onSegment(a, c, b) {
		return true
	}
	if math.Abs(o2) <= eps && onSegment(a, d, b) {
		return true
	}
	if math.Abs(o3) <= eps && onSegment(c, a, d) {
		return true
	}
	if math.Abs(o4) <= eps && onSegment(c, b, d) {
		return true
	}

	return false
}

func orientation(a, b, c geometry.LatLon) float64 {
	return (b.Lon-a.Lon)*(c.Lat-a.Lat) - (b.Lat-a.Lat)*(c.Lon-a.Lon)
}

func onSegment(a, b, c geometry.LatLon) bool {
	const eps = 1e-9
	return b.Lon <= math.Max(a.Lon, c.Lon)+eps &&
		b.Lon >= math.Min(a.Lon, c.Lon)-eps &&
		b.Lat <= math.Max(a.Lat, c.Lat)+eps &&
		b.Lat >= math.Min(a.Lat, c.Lat)-eps
}

func samePoint(a, b geometry.LatLon) bool {
	return math.Abs(a.Lat-b.Lat) <= 1e-9 && math.Abs(a.Lon-b.Lon) <= 1e-9
}

func formatIntersections(intersections []segmentIntersection) string {
	parts := make([]string, 0, len(intersections))
	for _, intersection := range intersections {
		parts = append(parts, fmt.Sprintf("%d и %d", intersection.First, intersection.Second))
	}
	return strings.Join(parts, ", ")
}
