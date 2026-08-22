package bathymetry

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

func TestWriteOverviewSVGShowsDepthContoursLegendAndProvenance(t *testing.T) {
	model := syntheticOverviewModel()
	path := filepath.Join(t.TempDir(), "bathymetry-overview.svg")
	metadata := seabed.NewExportMetadata(
		mesh.EqualAreaProjection{ReferenceLat: 44, ReferenceLon: 34},
		"средний уровень моря по допущению GEBCO",
		"В мелководье исходные вертикальные системы могут различаться.",
	)
	report, err := WriteOverviewSVG(path, model, OverviewConfig{
		Source:   "GEBCO Bathymetric Compilation Group 2026, DOI 10.5285/example",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.NodeCount != 9 || report.CellCount != 4 || report.MaxDepthM != 120 || report.MeshEdgesDrawn {
		t.Fatalf("неверная сводка обзорной карты: %+v", report)
	}
	if len(report.RenderedIsobathsM) != 3 || report.RenderedIsobathsM[0] != 20 || report.RenderedIsobathsM[2] != 100 {
		t.Fatalf("неверно выбраны изобаты: %v", report.RenderedIsobathsM)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, marker := range []string{
		`class="bathymetry-cell"`, `class="isobath"`, `class="isobath-label"`,
		`id="depth-scale"`, "Глубина, м", "NoData узлов: 0.00%", "Источник:",
		"GEBCO Bathymetric Compilation Group 2026", "средний уровень моря",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("обзорная карта не содержит обязательный элемент %q", marker)
		}
	}
	if strings.Contains(text, `class="mesh-edge"`) {
		t.Fatal("обзорная карта не должна содержать внутренние рёбра сетки")
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

func TestWriteOverviewSVGRejectsUnacceptedModel(t *testing.T) {
	model := syntheticOverviewModel()
	model.Accepted = false
	_, err := WriteOverviewSVG(filepath.Join(t.TempDir(), "map.svg"), model, OverviewConfig{
		Source: "GEBCO",
		Metadata: seabed.NewExportMetadata(
			mesh.EqualAreaProjection{ReferenceLat: 44, ReferenceLon: 34},
			"средний уровень моря",
			"Оговорка вертикальной системы.",
		),
	})
	if err == nil || !strings.Contains(err.Error(), "только для принятой модели") {
		t.Fatalf("непринятая модель должна быть отклонена, получено: %v", err)
	}
}

func TestWriteMeshDetailsSVGShowsUnthinnedCellsQualityAndLocalContours(t *testing.T) {
	model := syntheticOverviewModel()
	model.Cells[0].Region = seabed.RegionCoast
	model.Cells[0].SlopeDeg = 0.5
	model.Cells[1].Region = seabed.RegionShelf
	model.Cells[1].WaterDepthMeanM = 90
	model.Cells[2].Region = seabed.RegionSlope
	model.Cells[2].SlopeDeg = 4
	model.Cells[3].Region = seabed.RegionSlope
	model.Cells[3].SlopeDeg = 8
	path := filepath.Join(t.TempDir(), "mesh-details.svg")
	metadata := seabed.NewExportMetadata(
		mesh.EqualAreaProjection{ReferenceLat: 44, ReferenceLon: 34},
		"средний уровень моря по допущению GEBCO",
		"В мелководье исходные вертикальные системы могут различаться.",
	)
	report, err := WriteMeshDetailsSVG(path, model, MeshDetailsConfig{
		Source:   "GEBCO Bathymetric Compilation Group 2026, DOI 10.5285/example",
		Metadata: metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Unthinned || report.FullMeshCellCount != 4 || len(report.Fragments) != 3 {
		t.Fatalf("неверная сводка детальных фрагментов: %+v", report)
	}
	for _, fragment := range report.Fragments {
		if fragment.CellCount == 0 || fragment.EdgeMinM <= 0 || fragment.EdgeMinM > fragment.EdgeMeanM || fragment.EdgeMeanM > fragment.EdgeMaxM {
			t.Fatalf("некорректные метрики фрагмента %+v", fragment)
		}
		if fragment.QualityMin <= 0 || fragment.QualityMin > fragment.QualityMean || fragment.QualityMean > fragment.QualityMax {
			t.Fatalf("некорректное качество фрагмента %+v", fragment)
		}
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, marker := range []string{
		`data-unthinned="true"`, `class="mesh-detail-panel"`, `class="detail-cell"`,
		`class="detail-isobath`, "1. Сложный берег", "2. Шельф", "3. Крутой склон",
		"Рёбра: мин", "без прореживания", "Геометрическое качество Q", "Источник:",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("детальная карта не содержит обязательный элемент %q", marker)
		}
	}
	if strings.Count(text, `class="mesh-detail-panel"`) != 3 {
		t.Fatalf("ожидалось три фрагмента, получено %d", strings.Count(text, `class="mesh-detail-panel"`))
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

func syntheticOverviewModel() seabed.Model {
	depths := []float64{0, 0, 0, 0, 120, 0, 0, 0, 0}
	nodes := make([]seabed.Node, 10)
	meshNodes := make([]mesh.Point, 10)
	for index := 0; index < 9; index++ {
		id := index + 1
		x := float64(index%3) * 100
		y := float64(index/3) * 100
		depth := depths[index]
		elevation := -depth
		boundary := index != 4
		kind := seabed.BoundaryNone
		method := seabed.SamplingBilinear
		quality := seabed.QualityVerified
		if boundary {
			kind = seabed.BoundaryCoastline
			method = seabed.SamplingCoastlineConstraint
			quality = seabed.QualityConstrained
		}
		nodes[id] = seabed.Node{
			ID: id, XM: x, YM: y, LongitudeDeg: 34 + x/100000, LatitudeDeg: 44 + y/100000,
			ElevationM: &elevation, WaterDepthM: &depth, SamplingMethod: method,
			QualityFlag: quality, IsBoundary: boundary, BoundaryKind: kind,
		}
		meshNodes[id] = mesh.Point{X: x, Y: y}
	}
	cells := []seabed.Cell{
		{ID: 1, NodeIDs: [4]int{1, 2, 5, 4}, WaterDepthMeanM: 30, Region: seabed.RegionShelf},
		{ID: 2, NodeIDs: [4]int{2, 3, 6, 5}, WaterDepthMeanM: 30, Region: seabed.RegionShelf},
		{ID: 3, NodeIDs: [4]int{4, 5, 8, 7}, WaterDepthMeanM: 30, Region: seabed.RegionShelf},
		{ID: 4, NodeIDs: [4]int{5, 6, 9, 8}, WaterDepthMeanM: 30, Region: seabed.RegionShelf},
	}
	meshCells := []mesh.Cell{
		{Nodes: cells[0].NodeIDs, NodeCount: 4}, {Nodes: cells[1].NodeIDs, NodeCount: 4},
		{Nodes: cells[2].NodeIDs, NodeCount: 4}, {Nodes: cells[3].NodeIDs, NodeCount: 4},
	}
	edges := []seabed.BoundaryEdge{
		{NodeIDs: [2]int{1, 2}, Kind: seabed.BoundaryCoastline}, {NodeIDs: [2]int{2, 3}, Kind: seabed.BoundaryCoastline},
		{NodeIDs: [2]int{3, 6}, Kind: seabed.BoundaryCoastline}, {NodeIDs: [2]int{6, 9}, Kind: seabed.BoundaryCoastline},
		{NodeIDs: [2]int{9, 8}, Kind: seabed.BoundaryCoastline}, {NodeIDs: [2]int{8, 7}, Kind: seabed.BoundaryCoastline},
		{NodeIDs: [2]int{7, 4}, Kind: seabed.BoundaryCoastline}, {NodeIDs: [2]int{4, 1}, Kind: seabed.BoundaryCoastline},
	}
	return seabed.Model{
		Mesh:  mesh.Mesh{Nodes: meshNodes, Cells: meshCells, QuadCount: 4},
		Nodes: nodes, Cells: cells, BoundaryEdges: edges, Accepted: true,
		Sampling: seabed.SamplingSummary{TotalNodeCount: 9, AssignedNodeCount: 9, CoveragePercent: 100},
		CellDerivation: seabed.CellDerivationMetadata{
			RegionThresholds: seabed.DefaultRegionThresholds(),
			Summary:          seabed.CellSummary{TotalCellCount: 4, AssignedCellCount: 4, CoveragePercent: 100},
		},
	}
}
