package adaptive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

func TestWriteExpertFragmentSVGUsesMeshCoastAndContours(t *testing.T) {
	zero, hundred := 0.0, 100.0
	reference := seabed.Model{
		Accepted: true,
		Nodes: []seabed.Node{{},
			{ID: 1, XM: 0, YM: 0, WaterDepthM: &zero}, {ID: 2, XM: 2, YM: 0, WaterDepthM: &zero},
			{ID: 3, XM: 2, YM: 2, WaterDepthM: &hundred}, {ID: 4, XM: 0, YM: 2, WaterDepthM: &hundred},
		},
		Cells: []seabed.Cell{{ID: 1, NodeIDs: [4]int{1, 2, 3, 4}}},
	}
	candidate := mesh.Mesh{
		Nodes: []mesh.Point{{}, {X: 0, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 2}, {X: 0, Y: 2}},
		Cells: []mesh.Cell{{Nodes: [4]int{1, 2, 3, 4}, NodeCount: 4}},
	}
	path := filepath.Join(t.TempDir(), "card.svg")
	report, err := WriteExpertFragmentSVG(path, reference, candidate, ExpertFragmentSVGConfig{
		PresentationID: "E-001", FragmentID: "synthetic-spit", Feature: "коса", CenterX: 1, CenterY: 1,
		WidthM: 2, HeightM: 2, IsobathsM: []float64{50}, SourceRings: [][]mesh.Point{{{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 2}, {X: 0, Y: 2}, {X: 0, Y: 0}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.VisibleCellCount != 1 || report.VisibleContourCount == 0 || report.VisibleCoastSegmentCount == 0 {
		t.Fatalf("карточка не содержит обязательных слоёв: %+v", report)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "candidate-cell") || !strings.Contains(string(data), "reference-contour") || strings.Contains(string(data), "delaunay") {
		t.Fatalf("SVG-карточка не является анонимной или неполна: %s", data)
	}
}
