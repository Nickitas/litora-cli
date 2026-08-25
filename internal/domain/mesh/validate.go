package mesh

import (
	"fmt"
	"math"
)

// TopologyValidation описывает строгую проверку полной четырёхугольной сетки.
type TopologyValidation struct {
	NodeCount                  int         `json:"node_count"`
	CellCount                  int         `json:"cell_count"`
	TriangleCount              int         `json:"triangle_count"`
	NonQuadCellCount           int         `json:"non_quad_cell_count"`
	DegenerateCellCount        int         `json:"degenerate_cell_count"`
	NonManifoldEdgeCount       int         `json:"non_manifold_edge_count"`
	UnmatchedInteriorEdgeCount int         `json:"unmatched_interior_edge_count"`
	BoundaryEdgeCount          int         `json:"boundary_edge_count"`
	UntaggedBoundaryEdgeCount  int         `json:"untagged_boundary_edge_count"`
	InvalidBoundaryTagCount    int         `json:"invalid_boundary_tag_count"`
	BoundaryTagCounts          map[int]int `json:"boundary_tag_counts"`
	Accepted                   bool        `json:"accepted"`
	Reasons                    []string    `json:"reasons"`
}

// ValidateFullQuadMesh проверяет индексы, площади, кратность общих рёбер и
// совпадение топологической границы с физическими линейными элементами MSH.
// Висячий узел проявляется как несогласованное внутреннее ребро кратности 1.
func ValidateFullQuadMesh(generated Mesh) TopologyValidation {
	report := TopologyValidation{
		NodeCount: len(generated.Nodes) - 1, CellCount: len(generated.Cells),
		TriangleCount: generated.TriangleCount, BoundaryEdgeCount: len(generated.BoundaryEdges),
		BoundaryTagCounts: make(map[int]int),
	}
	boundary := make(map[[2]int]bool, len(generated.BoundaryEdges))
	for index, edge := range generated.BoundaryEdges {
		key := normalizedEdge(edge[0], edge[1])
		if boundary[key] {
			report.NonManifoldEdgeCount++
		}
		boundary[key] = true
		tag := 0
		if index < len(generated.BoundaryPhysicalTags) {
			tag = generated.BoundaryPhysicalTags[index]
		}
		report.BoundaryTagCounts[tag]++
		if tag == 0 {
			report.UntaggedBoundaryEdgeCount++
		} else if tag != PhysicalCoastline && tag != PhysicalIsland && tag != PhysicalOpenBoundary {
			report.InvalidBoundaryTagCount++
		}
	}
	incidence := make(map[[2]int]uint8, 2*len(generated.Cells))
	for _, cell := range generated.Cells {
		if cell.NodeCount != 4 {
			report.NonQuadCellCount++
			continue
		}
		area2 := 0.0
		seen := make(map[int]bool, 4)
		valid := true
		for side := 0; side < 4; side++ {
			a, b := cell.Nodes[side], cell.Nodes[(side+1)%4]
			if a <= 0 || a >= len(generated.Nodes) || b <= 0 || b >= len(generated.Nodes) || a == b || seen[a] {
				valid = false
				continue
			}
			seen[a] = true
			left, right := generated.Nodes[a], generated.Nodes[b]
			area2 += left.X*right.Y - right.X*left.Y
			key := normalizedEdge(a, b)
			if incidence[key] < math.MaxUint8 {
				incidence[key]++
			}
		}
		if !valid || math.Abs(area2) <= 1e-9 {
			report.DegenerateCellCount++
		}
	}
	for edge, count := range incidence {
		if count > 2 {
			report.NonManifoldEdgeCount++
		}
		if count == 1 && !boundary[edge] {
			report.UnmatchedInteriorEdgeCount++
		}
	}
	for edge := range boundary {
		if incidence[edge] != 1 {
			report.NonManifoldEdgeCount++
		}
	}
	if report.TriangleCount != 0 || report.NonQuadCellCount != 0 {
		report.Reasons = append(report.Reasons, "в сетке присутствуют нечетырёхугольные ячейки")
	}
	if report.DegenerateCellCount != 0 {
		report.Reasons = append(report.Reasons, fmt.Sprintf("вырожденных ячеек: %d", report.DegenerateCellCount))
	}
	if report.NonManifoldEdgeCount != 0 || report.UnmatchedInteriorEdgeCount != 0 {
		report.Reasons = append(report.Reasons, fmt.Sprintf("несогласованных рёбер: %d", report.NonManifoldEdgeCount+report.UnmatchedInteriorEdgeCount))
	}
	if report.BoundaryEdgeCount == 0 || report.UntaggedBoundaryEdgeCount != 0 || report.InvalidBoundaryTagCount != 0 || len(generated.BoundaryPhysicalTags) != len(generated.BoundaryEdges) || report.BoundaryTagCounts[PhysicalCoastline] == 0 {
		report.Reasons = append(report.Reasons, "не все граничные рёбра имеют физические метки")
	}
	if generated.SurfacePhysicalTag != PhysicalWaterSurface {
		report.Reasons = append(report.Reasons, "поверхность не имеет физическую метку водоёма")
	}
	report.Accepted = len(report.Reasons) == 0
	return report
}
