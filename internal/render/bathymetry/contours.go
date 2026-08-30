package bathymetry

import (
	"math"

	"coastal-geometry/internal/domain/seabed"
)

type mapPoint struct {
	x float64
	y float64
}

type contourSegment struct {
	start mapPoint
	end   mapPoint
}

func buildContourSegments(model seabed.Model, depthM float64) []contourSegment {
	segments := make([]contourSegment, 0, len(model.Cells)/3)
	for _, cell := range model.Cells {
		points := make([]mapPoint, 4)
		depths := make([]float64, 4)
		valid := true
		for index, nodeID := range cell.NodeIDs {
			if nodeID <= 0 || nodeID >= len(model.Nodes) || model.Nodes[nodeID].WaterDepthM == nil {
				valid = false
				break
			}
			node := model.Nodes[nodeID]
			points[index] = mapPoint{x: node.XM, y: node.YM}
			depths[index] = *node.WaterDepthM
		}
		if !valid {
			continue
		}

		intersections := make([]mapPoint, 0, 4)
		for edge := 0; edge < 4; edge++ {
			next := (edge + 1) % 4
			leftDepth, rightDepth := depths[edge], depths[next]
			if !crossesLevel(leftDepth, rightDepth, depthM) {
				continue
			}
			ratio := (depthM - leftDepth) / (rightDepth - leftDepth)
			intersections = append(intersections, mapPoint{
				x: points[edge].x + ratio*(points[next].x-points[edge].x),
				y: points[edge].y + ratio*(points[next].y-points[edge].y),
			})
		}
		if len(intersections) == 2 {
			segments = append(segments, contourSegment{start: intersections[0], end: intersections[1]})
		}
		if len(intersections) == 4 {
			centerDepth := (depths[0] + depths[1] + depths[2] + depths[3]) / 4
			if (centerDepth >= depthM) == (depths[0] >= depthM) {
				segments = append(segments,
					contourSegment{start: intersections[0], end: intersections[1]},
					contourSegment{start: intersections[2], end: intersections[3]},
				)
			} else {
				segments = append(segments,
					contourSegment{start: intersections[0], end: intersections[3]},
					contourSegment{start: intersections[1], end: intersections[2]},
				)
			}
		}
	}
	return segments
}

func crossesLevel(left, right, level float64) bool {
	if math.Abs(left-right) <= 1e-12 {
		return false
	}
	return (left < level && right >= level) || (right < level && left >= level)
}

func segmentLength(segment contourSegment) float64 {
	return math.Hypot(segment.end.x-segment.start.x, segment.end.y-segment.start.y)
}
