package adaptive

import (
	"encoding/csv"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

func TestBuildSizeFieldExplainsFeaturesAndLimitsNeighbourGrowth(t *testing.T) {
	model := syntheticFieldModel()
	config := DefaultConfig()
	field, err := BuildSizeField(model, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(field.Nodes) != 9 || field.Report.Summary.NodeCount != 9 || field.Report.Summary.CellCount != 4 {
		t.Fatalf("неверная полнота поля: %+v", field.Report.Summary)
	}
	if field.Report.AdaptiveMeshGenerated {
		t.Fatal("ADAPT-01 не должен утверждать, что адаптивная сетка уже построена")
	}

	straightCoast := field.Nodes[1]
	if math.Abs(straightCoast.CoastCurvatureDeg) > 1e-9 || straightCoast.RawTargetSizeM != config.StraightCoastSizeM {
		t.Fatalf("прямой берег должен получить исходные 300 м: %+v", straightCoast)
	}
	corner := field.Nodes[0]
	if corner.CoastCurvatureDeg < 89.9 || corner.RawTargetSizeM != config.MinSizeM {
		t.Fatalf("угол берега должен получить исходные 200 м: %+v", corner)
	}
	deep := field.Nodes[4]
	if deep.BaseSizeM != config.DeepSizeM || deep.RawTargetSizeM < 0.99*config.DeepSizeM {
		t.Fatalf("далёкое ровное глубоководье должно стремиться к 1000 м: %+v", deep)
	}
	if deep.TargetSizeM >= deep.RawTargetSizeM || !deep.GrowthLimited {
		t.Fatalf("на коротком синтетическом каркасе глубокий узел должен быть плавно согласован с берегом: %+v", deep)
	}
	summary := field.Report.Summary
	if summary.RawMaxAdjacentRatio <= config.MaxNeighbourRatio {
		t.Fatalf("синтетическое исходное поле должно содержать устраняемый скачок: %+v", summary)
	}
	if summary.FinalMaxAdjacentRatio > config.MaxNeighbourRatio+1e-9 {
		t.Fatalf("ограничение соседнего отношения нарушено: %+v", summary)
	}
	if summary.FinalMaxSizeGradientPerM > config.MaxSizeGradientPerM+1e-9 {
		t.Fatalf("ограничение пространственного роста нарушено: %+v", summary)
	}
	if summary.GrowthLimitedNodeCount == 0 {
		t.Fatal("ожидалась явная коррекция роста")
	}

	repeated, err := BuildSizeField(model, config)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(field, repeated) {
		t.Fatal("поле должно воспроизводиться без случайности генератора")
	}
}

func TestBuildSizeFieldUsesDepthGradientRefinement(t *testing.T) {
	model := syntheticFieldModel()
	model.Cells[0].SlopeDeg = 10
	field, err := BuildSizeField(model, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	value := field.Nodes[4]
	if value.DepthGradientDeg != 10 || value.GradientRefinementM <= 0 {
		t.Fatalf("градиент BATHY-03 должен входить в объяснение размера: %+v", value)
	}
}

func TestBuildSizeFieldRejectsInvalidGradation(t *testing.T) {
	config := DefaultConfig()
	config.MaxNeighbourRatio = 1
	_, err := BuildSizeField(syntheticFieldModel(), config)
	if err == nil {
		t.Fatal("отношение соседних размеров не больше единицы должно быть отклонено")
	}
}

func TestBuildSizeFieldKeepsNoDataGradientFallbackExplicit(t *testing.T) {
	model := syntheticFieldModel()
	model.Nodes[5].WaterDepthM = nil
	model.Nodes[5].ElevationM = nil
	model.Cells = nil
	field, err := BuildSizeField(model, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	value := field.Nodes[4]
	if value.DepthAvailable || value.GradientAvailable {
		t.Fatalf("NoData не должен выглядеть измеренным значением: %+v", value)
	}
	if value.DepthGradientDeg != 0 || value.EffectiveGradientDeg != DefaultConfig().SlopeReferenceDeg {
		t.Fatalf("измеренный и эффективный градиенты должны быть разделены: %+v", value)
	}
	if value.GradientRefinementM <= 0 || field.Report.Summary.NoDataNodeCount != 1 {
		t.Fatalf("консервативное уточнение и NoData должны быть отражены в отчёте: %+v", value)
	}
}

func TestWriteFieldCSVPreservesEveryFormulaComponent(t *testing.T) {
	field, err := BuildSizeField(syntheticFieldModel(), DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "size-field.csv")
	if err := WriteFieldCSV(path, field); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 10 {
		t.Fatalf("ожидались заголовок и 9 узлов, получено строк: %d", len(rows))
	}
	wanted := map[string]bool{
		"distance_to_coast_m": false, "coast_curvature_deg": false,
		"depth_gradient_deg": false, "effective_gradient_deg": false, "raw_target_size_m": false,
		"target_size_m": false, "growth_limited": false,
	}
	for _, header := range rows[0] {
		if _, ok := wanted[header]; ok {
			wanted[header] = true
		}
	}
	for header, found := range wanted {
		if !found {
			t.Fatalf("CSV не содержит объясняющее поле %q", header)
		}
	}
}

func syntheticFieldModel() seabed.Model {
	const spacing = 150_000.0
	depths := []float64{0, 0, 0, 0, 2200, 0, 0, 0, 0}
	nodes := make([]seabed.Node, 10)
	meshNodes := make([]mesh.Point, 10)
	for index := 0; index < 9; index++ {
		id := index + 1
		x := float64(index%3) * spacing
		y := float64(index/3) * spacing
		depth := depths[index]
		elevation := -depth
		boundary := index != 4
		kind := seabed.BoundaryNone
		if boundary {
			kind = seabed.BoundaryCoastline
		}
		nodes[id] = seabed.Node{
			ID: id, XM: x, YM: y, LongitudeDeg: 34 + x/100_000, LatitudeDeg: 44 + y/100_000,
			ElevationM: &elevation, WaterDepthM: &depth, IsBoundary: boundary, BoundaryKind: kind,
		}
		meshNodes[id] = mesh.Point{X: x, Y: y}
	}
	cellNodeIDs := [][4]int{{1, 2, 5, 4}, {2, 3, 6, 5}, {4, 5, 8, 7}, {5, 6, 9, 8}}
	meshCells := make([]mesh.Cell, 0, len(cellNodeIDs))
	cells := make([]seabed.Cell, 0, len(cellNodeIDs))
	for index, ids := range cellNodeIDs {
		meshCells = append(meshCells, mesh.Cell{Nodes: ids, NodeCount: 4})
		cells = append(cells, seabed.Cell{ID: index + 1, NodeIDs: ids, SlopeDeg: 0, Region: seabed.RegionShelf})
	}
	boundaryPairs := [][2]int{{1, 2}, {2, 3}, {3, 6}, {6, 9}, {9, 8}, {8, 7}, {7, 4}, {4, 1}}
	boundaryEdges := make([]seabed.BoundaryEdge, 0, len(boundaryPairs))
	for _, pair := range boundaryPairs {
		boundaryEdges = append(boundaryEdges, seabed.BoundaryEdge{NodeIDs: pair, Kind: seabed.BoundaryCoastline})
	}
	return seabed.Model{
		Mesh:  mesh.Mesh{Nodes: meshNodes, Cells: meshCells, QuadCount: len(meshCells)},
		Nodes: nodes, Cells: cells, BoundaryEdges: boundaryEdges, Accepted: true,
		CellDerivation: seabed.CellDerivationMetadata{RegionThresholds: seabed.DefaultRegionThresholds()},
	}
}
