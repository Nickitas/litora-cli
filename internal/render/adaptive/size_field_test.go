package adaptive

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fieldmodel "coastal-geometry/internal/domain/adaptive"
	"coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

func TestWriteSizeFieldSVGExplainsTargetAndDoesNotClaimGeneratedMesh(t *testing.T) {
	model := renderFieldModel()
	field, err := fieldmodel.BuildSizeField(model, fieldmodel.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "size-field.svg")
	report, err := WriteSizeFieldSVG(path, model, field, SizeFieldMapConfig{Source: "GEBCO 2026"})
	if err != nil {
		t.Fatal(err)
	}
	if report.NodeCount != 9 || report.CellCount != 4 || report.MeshEdgesDrawn || report.AdaptiveMeshGenerated {
		t.Fatalf("неверный отчёт карты: %+v", report)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, marker := range []string{
		`data-schema="lito-adaptive-size-field/v1"`, `data-adaptive-mesh-generated="false"`,
		`data-mesh-edges-drawn="false"`, `class="size-field-cell"`, `class="size-field-coast"`,
		"Цвет — целевая длина ребра, не глубина", "Адаптивная сетка ещё не построена", "Источник батиметрии: GEBCO 2026",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("карта не содержит обязательный элемент %q", marker)
		}
	}
	decoder := xml.NewDecoder(strings.NewReader(text))
	for {
		if _, err := decoder.Token(); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("создан некорректный XML: %v", err)
		}
	}
}

func renderFieldModel() seabed.Model {
	depths := []float64{0, 0, 0, 0, 2200, 0, 0, 0, 0}
	nodes := make([]seabed.Node, 10)
	meshNodes := make([]mesh.Point, 10)
	for index := 0; index < 9; index++ {
		id := index + 1
		x := float64(index%3) * 10_000
		y := float64(index/3) * 10_000
		depth, elevation := depths[index], -depths[index]
		boundary := index != 4
		kind := seabed.BoundaryNone
		if boundary {
			kind = seabed.BoundaryCoastline
		}
		nodes[id] = seabed.Node{ID: id, XM: x, YM: y, LongitudeDeg: 34 + x/100_000, LatitudeDeg: 44 + y/100_000, ElevationM: &elevation, WaterDepthM: &depth, IsBoundary: boundary, BoundaryKind: kind}
		meshNodes[id] = mesh.Point{X: x, Y: y}
	}
	ids := [][4]int{{1, 2, 5, 4}, {2, 3, 6, 5}, {4, 5, 8, 7}, {5, 6, 9, 8}}
	meshCells := make([]mesh.Cell, 0, 4)
	cells := make([]seabed.Cell, 0, 4)
	for index, nodeIDs := range ids {
		meshCells = append(meshCells, mesh.Cell{Nodes: nodeIDs, NodeCount: 4})
		cells = append(cells, seabed.Cell{ID: index + 1, NodeIDs: nodeIDs, SlopeDeg: float64(index), Region: seabed.RegionSlope})
	}
	pairs := [][2]int{{1, 2}, {2, 3}, {3, 6}, {6, 9}, {9, 8}, {8, 7}, {7, 4}, {4, 1}}
	boundary := make([]seabed.BoundaryEdge, 0, len(pairs))
	for _, pair := range pairs {
		boundary = append(boundary, seabed.BoundaryEdge{NodeIDs: pair, Kind: seabed.BoundaryCoastline})
	}
	return seabed.Model{
		Mesh: mesh.Mesh{Nodes: meshNodes, Cells: meshCells, QuadCount: 4}, Nodes: nodes,
		Cells: cells, BoundaryEdges: boundary, Accepted: true,
		CellDerivation: seabed.CellDerivationMetadata{RegionThresholds: seabed.DefaultRegionThresholds()},
	}
}
