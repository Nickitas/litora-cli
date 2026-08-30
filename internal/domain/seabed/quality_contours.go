package seabed

import (
	"math"
	"sort"

	"coastal-geometry/internal/domain/mesh"
)

type reliefContourSegment struct {
	start mesh.Point
	end   mesh.Point
}

func evaluateIsobathPreservation(reference, evaluated Model, isobaths []float64, resolutionM float64) []IsobathPreservationMetrics {
	result := make([]IsobathPreservationMetrics, 0, len(isobaths))
	for _, depthM := range isobaths {
		referenceSegments := modelContourSegments(reference, depthM)
		evaluatedSegments := modelContourSegments(evaluated, depthM)
		metric := IsobathPreservationMetrics{
			DepthM:            depthM,
			ReferenceLengthKM: contourLengthM(referenceSegments) / 1000,
			EvaluatedLengthKM: contourLengthM(evaluatedSegments) / 1000,
		}
		if len(referenceSegments) == 0 {
			metric.Reason = "опорная модель не содержит эту изобату"
			result = append(result, metric)
			continue
		}
		if len(evaluatedSegments) == 0 {
			metric.Reason = "проверяемая модель не содержит эту изобату"
			result = append(result, metric)
			continue
		}
		referenceIndex := newReliefSegmentIndex(referenceSegments)
		evaluatedIndex := newReliefSegmentIndex(evaluatedSegments)
		sampleStepM := math.Max(1, resolutionM/2)
		distances := directedContourDistances(referenceSegments, evaluatedIndex, sampleStepM)
		distances = append(distances, directedContourDistances(evaluatedSegments, referenceIndex, sampleStepM)...)
		if len(distances) == 0 {
			metric.Reason = "не сформированы контрольные точки изобаты"
			result = append(result, metric)
			continue
		}
		sort.Float64s(distances)
		metric.Comparable = true
		metric.SampleCount = len(distances)
		metric.MaxDistanceM = distances[len(distances)-1]
		metric.P95DistanceM = exactQualityQuantile(distances, 0.95)
		for _, distanceM := range distances {
			metric.MeanDistanceM += distanceM
		}
		metric.MeanDistanceM /= float64(len(distances))
		metric.Accepted = metric.P95DistanceM <= 2*resolutionM
		result = append(result, metric)
	}
	return result
}

func modelContourSegments(model Model, depthM float64) []reliefContourSegment {
	segments := make([]reliefContourSegment, 0)
	seen := make(map[reliefContourSegmentKey]bool)
	for _, cell := range model.Mesh.Cells {
		if cell.NodeCount != 4 {
			continue
		}
		for _, triangle := range [][3]int{{cell.Nodes[0], cell.Nodes[1], cell.Nodes[2]}, {cell.Nodes[0], cell.Nodes[2], cell.Nodes[3]}} {
			if segment, ok := triangleContourSegment(model.Nodes, triangle, depthM); ok {
				key := newReliefContourSegmentKey(segment)
				if !seen[key] {
					seen[key] = true
					segments = append(segments, segment)
				}
			}
		}
	}
	return segments
}

func triangleContourSegment(nodes []Node, ids [3]int, depthM float64) (reliefContourSegment, bool) {
	for edge := 0; edge < 3; edge++ {
		leftID, rightID := ids[edge], ids[(edge+1)%3]
		if leftID <= 0 || rightID <= 0 || leftID >= len(nodes) || rightID >= len(nodes) ||
			nodes[leftID].WaterDepthM == nil || nodes[rightID].WaterDepthM == nil {
			return reliefContourSegment{}, false
		}
		if math.Abs(*nodes[leftID].WaterDepthM-depthM) <= 1e-12 && math.Abs(*nodes[rightID].WaterDepthM-depthM) <= 1e-12 {
			return reliefContourSegment{
				start: mesh.Point{X: nodes[leftID].XM, Y: nodes[leftID].YM},
				end:   mesh.Point{X: nodes[rightID].XM, Y: nodes[rightID].YM},
			}, true
		}
	}
	points := make([]mesh.Point, 0, 3)
	for edge := 0; edge < 3; edge++ {
		leftID, rightID := ids[edge], ids[(edge+1)%3]
		if leftID <= 0 || rightID <= 0 || leftID >= len(nodes) || rightID >= len(nodes) ||
			nodes[leftID].WaterDepthM == nil || nodes[rightID].WaterDepthM == nil {
			return reliefContourSegment{}, false
		}
		leftDepth, rightDepth := *nodes[leftID].WaterDepthM, *nodes[rightID].WaterDepthM
		leftDelta, rightDelta := leftDepth-depthM, rightDepth-depthM
		if math.Abs(leftDelta) <= 1e-12 && math.Abs(rightDelta) <= 1e-12 {
			continue
		}
		if leftDelta*rightDelta > 0 {
			continue
		}
		fraction := 0.0
		if math.Abs(leftDelta) <= 1e-12 {
			fraction = 0
		} else if math.Abs(rightDelta) <= 1e-12 {
			fraction = 1
		} else {
			fraction = (depthM - leftDepth) / (rightDepth - leftDepth)
		}
		left, right := nodes[leftID], nodes[rightID]
		candidate := mesh.Point{
			X: left.XM + fraction*(right.XM-left.XM),
			Y: left.YM + fraction*(right.YM-left.YM),
		}
		duplicate := false
		for _, existing := range points {
			if math.Hypot(existing.X-candidate.X, existing.Y-candidate.Y) <= 1e-8 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			points = append(points, candidate)
		}
	}
	if len(points) != 2 || math.Hypot(points[1].X-points[0].X, points[1].Y-points[0].Y) <= 1e-8 {
		return reliefContourSegment{}, false
	}
	return reliefContourSegment{start: points[0], end: points[1]}, true
}

type reliefContourSegmentKey struct {
	startX, startY int64
	endX, endY     int64
}

func newReliefContourSegmentKey(segment reliefContourSegment) reliefContourSegmentKey {
	startX, startY := int64(math.Round(segment.start.X*1e6)), int64(math.Round(segment.start.Y*1e6))
	endX, endY := int64(math.Round(segment.end.X*1e6)), int64(math.Round(segment.end.Y*1e6))
	if startX > endX || (startX == endX && startY > endY) {
		startX, endX = endX, startX
		startY, endY = endY, startY
	}
	return reliefContourSegmentKey{startX: startX, startY: startY, endX: endX, endY: endY}
}

func contourLengthM(segments []reliefContourSegment) float64 {
	total := 0.0
	for _, segment := range segments {
		total += math.Hypot(segment.end.X-segment.start.X, segment.end.Y-segment.start.Y)
	}
	return total
}

type reliefSegmentIndex struct {
	segments      []reliefContourSegment
	bins          [][]int
	marks         []uint32
	generation    uint32
	columns, rows int
	minX, minY    float64
	maxX, maxY    float64
	stepX, stepY  float64
}

func newReliefSegmentIndex(segments []reliefContourSegment) *reliefSegmentIndex {
	index := &reliefSegmentIndex{
		segments: segments, marks: make([]uint32, len(segments)),
		minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1),
	}
	for _, segment := range segments {
		index.minX = math.Min(index.minX, math.Min(segment.start.X, segment.end.X))
		index.maxX = math.Max(index.maxX, math.Max(segment.start.X, segment.end.X))
		index.minY = math.Min(index.minY, math.Min(segment.start.Y, segment.end.Y))
		index.maxY = math.Max(index.maxY, math.Max(segment.start.Y, segment.end.Y))
	}
	resolution := int(math.Ceil(math.Sqrt(float64(len(segments)))))
	resolution = max(8, min(1024, resolution))
	index.columns, index.rows = resolution, resolution
	width, height := math.Max(1, index.maxX-index.minX), math.Max(1, index.maxY-index.minY)
	index.stepX, index.stepY = width/float64(index.columns), height/float64(index.rows)
	index.bins = make([][]int, index.columns*index.rows)
	for segmentID, segment := range segments {
		minColumn, minRow := index.binCoordinates(math.Min(segment.start.X, segment.end.X), math.Min(segment.start.Y, segment.end.Y))
		maxColumn, maxRow := index.binCoordinates(math.Max(segment.start.X, segment.end.X), math.Max(segment.start.Y, segment.end.Y))
		for row := minRow; row <= maxRow; row++ {
			for column := minColumn; column <= maxColumn; column++ {
				index.bins[row*index.columns+column] = append(index.bins[row*index.columns+column], segmentID)
			}
		}
	}
	return index
}

func (index *reliefSegmentIndex) nearestDistance(point mesh.Point) float64 {
	index.generation++
	if index.generation == 0 {
		clear(index.marks)
		index.generation = 1
	}
	centerColumn, centerRow := index.binCoordinates(point.X, point.Y)
	best := math.Inf(1)
	maximumRing := max(index.columns, index.rows)
	minimumStep := math.Min(index.stepX, index.stepY)
	for ring := 0; ring <= maximumRing; ring++ {
		minColumn, maxColumn := max(0, centerColumn-ring), min(index.columns-1, centerColumn+ring)
		minRow, maxRow := max(0, centerRow-ring), min(index.rows-1, centerRow+ring)
		for row := minRow; row <= maxRow; row++ {
			for column := minColumn; column <= maxColumn; column++ {
				if ring > 0 && row > minRow && row < maxRow && column > minColumn && column < maxColumn {
					continue
				}
				for _, segmentID := range index.bins[row*index.columns+column] {
					if index.marks[segmentID] == index.generation {
						continue
					}
					index.marks[segmentID] = index.generation
					best = math.Min(best, pointSegmentDistanceM(point, index.segments[segmentID]))
				}
			}
		}
		if ring > 1 && best <= float64(ring-1)*minimumStep {
			break
		}
	}
	return best
}

func (index *reliefSegmentIndex) binCoordinates(x, y float64) (int, int) {
	column := int(math.Floor((x - index.minX) / index.stepX))
	row := int(math.Floor((y - index.minY) / index.stepY))
	return max(0, min(index.columns-1, column)), max(0, min(index.rows-1, row))
}

func directedContourDistances(segments []reliefContourSegment, target *reliefSegmentIndex, sampleStepM float64) []float64 {
	result := make([]float64, 0, len(segments))
	for _, segment := range segments {
		lengthM := math.Hypot(segment.end.X-segment.start.X, segment.end.Y-segment.start.Y)
		sampleCount := max(1, int(math.Ceil(lengthM/sampleStepM)))
		for sampleIndex := 0; sampleIndex < sampleCount; sampleIndex++ {
			fraction := (float64(sampleIndex) + 0.5) / float64(sampleCount)
			point := mesh.Point{
				X: segment.start.X + fraction*(segment.end.X-segment.start.X),
				Y: segment.start.Y + fraction*(segment.end.Y-segment.start.Y),
			}
			if distanceM := target.nearestDistance(point); finite(distanceM) {
				result = append(result, distanceM)
			}
		}
	}
	return result
}

func pointSegmentDistanceM(point mesh.Point, segment reliefContourSegment) float64 {
	dx, dy := segment.end.X-segment.start.X, segment.end.Y-segment.start.Y
	denominator := dx*dx + dy*dy
	if denominator <= 1e-18 {
		return math.Hypot(point.X-segment.start.X, point.Y-segment.start.Y)
	}
	fraction := ((point.X-segment.start.X)*dx + (point.Y-segment.start.Y)*dy) / denominator
	fraction = math.Max(0, math.Min(1, fraction))
	nearestX := segment.start.X + fraction*dx
	nearestY := segment.start.Y + fraction*dy
	return math.Hypot(point.X-nearestX, point.Y-nearestY)
}

func exactQualityQuantile(sortedValues []float64, probability float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	index := int(math.Ceil(probability*float64(len(sortedValues)))) - 1
	index = max(0, min(len(sortedValues)-1, index))
	return sortedValues[index]
}
