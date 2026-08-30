package seabed

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"coastal-geometry/internal/domain/geometry"
	"coastal-geometry/internal/domain/mesh"
)

type samplerFunc func(latitudeDeg, longitudeDeg, maxSourceDistanceM float64) (Sample, error)

func (function samplerFunc) SampleElevation(latitudeDeg, longitudeDeg, maxSourceDistanceM float64) (Sample, error) {
	return function(latitudeDeg, longitudeDeg, maxSourceDistanceM)
}

func TestBuildAssignsContinuousCoastlineZeroAndSmoothTransition(t *testing.T) {
	source := threeByThreeMesh()
	sampler := constantSampler(-100, SamplingBilinear, 50)
	model, err := Build(source, sampler, BuildConfig{MaxSourceDistanceM: 2_000, CoastTransitionWidthM: 200})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Accepted {
		t.Fatalf("полная синтетическая модель должна быть принята: %v", model.Reasons)
	}
	for _, edge := range source.BoundaryEdges {
		for _, nodeID := range edge {
			node := model.Nodes[nodeID]
			if node.BoundaryKind != BoundaryCoastline || node.ElevationM == nil || *node.ElevationM != 0 || node.WaterDepthM == nil || *node.WaterDepthM != 0 {
				t.Fatalf("береговой узел %d не согласован с нулём: %+v", nodeID, node)
			}
		}
	}

	center := model.Nodes[9]
	if center.ElevationM == nil || math.Abs(*center.ElevationM+50) > 1e-9 {
		t.Fatalf("ожидался smoothstep-переход -100 → -50 в центре первой полосы, получено %+v", center.ElevationM)
	}
	if center.QualityFlag != QualityConstrained || center.SamplingMethod != SamplingBilinear {
		t.Fatalf("коррекция должна сохранять способ исходной выборки и менять флаг качества: %+v", center)
	}
	if model.Reconciliation.CorrectionCounts.CoastlineZero != 8 || model.Reconciliation.CorrectionCounts.CoastTransition != 1 {
		t.Fatalf("неверно подсчитаны коррекции: %+v", model.Reconciliation.CorrectionCounts)
	}
	if model.Reconciliation.DeepenedNodeCount != 0 {
		t.Fatal("плавная береговая коррекция не должна создавать новые провалы")
	}
	for _, correction := range model.Reconciliation.Corrections {
		if correction.Kind == CorrectionCoastTransition && *correction.CorrectedElevationM < *correction.OriginalElevationM {
			t.Fatalf("коррекция углубила узел: %+v", correction)
		}
	}
	if model.Sampling.CoveragePercent != 100 || model.Sampling.MethodCounts.CoastlineConstraint != 8 || model.Sampling.MethodCounts.Bilinear != 1 {
		t.Fatalf("неверная статистика покрытия: %+v", model.Sampling)
	}
}

func TestBuildRejectsPositiveElevationInsideAquatoryWithoutZeroFill(t *testing.T) {
	source := threeByThreeMesh()
	center := source.Nodes[9]
	sampler := samplerFunc(func(latitudeDeg, longitudeDeg, _ float64) (Sample, error) {
		elevation := -20.0
		if math.Abs(latitudeDeg-center.LatitudeDeg) < 1e-12 && math.Abs(longitudeDeg-center.LongitudeDeg) < 1e-12 {
			elevation = 4
		}
		return Sample{ElevationM: elevation, Method: SamplingBilinear, SourceDistanceM: 30, SourceDistanceSet: true}, nil
	})
	model, err := Build(source, sampler, BuildConfig{CoastTransitionWidthM: 200})
	if err != nil {
		t.Fatal(err)
	}
	node := model.Nodes[9]
	if node.ElevationM != nil || node.WaterDepthM != nil || node.QualityFlag != QualityRejected || node.SamplingMethod != SamplingNotSampled {
		t.Fatalf("положительная отметка не должна превращаться в нулевую глубину: %+v", node)
	}
	if model.Accepted || model.Reconciliation.CorrectionCounts.PositiveInsideRejected != 1 || model.Sampling.NoDataNodeCount != 1 {
		t.Fatalf("отклонение не отражено в отчёте: %+v", model)
	}
}

func TestBuildKeepsNoDataNull(t *testing.T) {
	source := threeByThreeMesh()
	center := source.Nodes[9]
	sampler := samplerFunc(func(latitudeDeg, longitudeDeg, _ float64) (Sample, error) {
		if math.Abs(latitudeDeg-center.LatitudeDeg) < 1e-12 && math.Abs(longitudeDeg-center.LongitudeDeg) < 1e-12 {
			return Sample{}, errors.New("нет покрытия")
		}
		return Sample{ElevationM: -10, Method: SamplingNearest, SourceDistanceM: 100, SourceDistanceSet: true}, nil
	})
	model, err := Build(source, sampler, BuildConfig{CoastTransitionWidthM: 200})
	if err != nil {
		t.Fatal(err)
	}
	node := model.Nodes[9]
	if node.ElevationM != nil || node.WaterDepthM != nil || node.QualityFlag != QualityNoData || node.SamplingMethod != SamplingNotSampled {
		t.Fatalf("NoData должен оставаться пустым: %+v", node)
	}
	if model.Accepted || model.Sampling.CoveragePercent >= 100 {
		t.Fatal("неполная модель не должна приниматься")
	}
	if model.CellDerivation.Summary.NoDataCellCount != 4 || model.CellDerivation.Summary.AssignedCellCount != 0 {
		t.Fatalf("все четыре ячейки с центральным NoData должны быть исключены: %+v", model.CellDerivation.Summary)
	}
}

func TestBoundaryClassificationSeparatesCoastIslandAndOpenBoundary(t *testing.T) {
	source := mesh.Mesh{Nodes: []mesh.Point{
		{},
		{X: 0, Y: 0}, {X: 200, Y: 0}, {X: 200, Y: 200}, {X: 0, Y: 200},
		{X: 80, Y: 80}, {X: 80, Y: 120}, {X: 120, Y: 120}, {X: 120, Y: 80},
		{X: 300, Y: 0}, {X: 350, Y: 0}, {X: 400, Y: 0},
	}, BoundaryEdges: [][2]int{
		{1, 2}, {2, 3}, {3, 4}, {4, 1},
		{5, 6}, {6, 7}, {7, 8}, {8, 5},
		{9, 10}, {10, 11},
	}}
	classification, err := classifyBoundaries(source, []BoundaryOverride{{NodeA: 1, NodeB: 2, Kind: BoundaryOpen}})
	if err != nil {
		t.Fatal(err)
	}
	for _, nodeID := range []int{3, 4} {
		if classification.nodeKinds[nodeID] != BoundaryCoastline {
			t.Fatalf("узел %d должен принадлежать внешнему берегу", nodeID)
		}
	}
	for _, nodeID := range []int{5, 6, 7, 8} {
		if classification.nodeKinds[nodeID] != BoundaryIsland {
			t.Fatalf("узел %d должен принадлежать острову", nodeID)
		}
	}
	for _, nodeID := range []int{1, 2, 9, 10, 11} {
		if classification.nodeKinds[nodeID] != BoundaryOpen {
			t.Fatalf("узел %d должен принадлежать открытой границе, получено %s", nodeID, classification.nodeKinds[nodeID])
		}
	}
}

func TestBuildAssignsZeroToIslandBoundary(t *testing.T) {
	source := mesh.Mesh{
		Nodes: []mesh.Point{
			{},
			georeferencedPoint(0, 0), georeferencedPoint(200, 0), georeferencedPoint(200, 200), georeferencedPoint(0, 200),
			georeferencedPoint(80, 80), georeferencedPoint(80, 120), georeferencedPoint(120, 120), georeferencedPoint(120, 80),
		},
		Cells: []mesh.Cell{
			{Nodes: [4]int{1, 2, 8, 5}, NodeCount: 4},
			{Nodes: [4]int{2, 3, 7, 8}, NodeCount: 4},
			{Nodes: [4]int{3, 4, 6, 7}, NodeCount: 4},
			{Nodes: [4]int{4, 1, 5, 6}, NodeCount: 4},
		},
		BoundaryEdges: [][2]int{
			{1, 2}, {2, 3}, {3, 4}, {4, 1},
			{5, 6}, {6, 7}, {7, 8}, {8, 5},
		},
		QuadCount: 4,
	}
	model, err := Build(source, constantSampler(-30, SamplingBilinear, 25), BuildConfig{CoastTransitionWidthM: 200})
	if err != nil {
		t.Fatal(err)
	}
	for _, nodeID := range []int{5, 6, 7, 8} {
		node := model.Nodes[nodeID]
		if node.BoundaryKind != BoundaryIsland || node.ElevationM == nil || *node.ElevationM != 0 || node.SamplingMethod != SamplingCoastlineConstraint {
			t.Fatalf("островной берег должен иметь отдельную метку и нулевой уровень: %+v", node)
		}
	}
	if model.Reconciliation.BoundaryCounts.Island != 4 {
		t.Fatalf("неверно подсчитаны островные узлы: %+v", model.Reconciliation.BoundaryCounts)
	}
}

func TestBuildDoesNotApplyCoastlineZeroToOpenBoundary(t *testing.T) {
	source := threeByThreeMesh()
	model, err := Build(source, constantSampler(-40, SamplingExact, 0), BuildConfig{
		CoastTransitionWidthM: 200,
		BoundaryOverrides:     []BoundaryOverride{{NodeA: 1, NodeB: 2, Kind: BoundaryOpen}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, nodeID := range []int{1, 2} {
		node := model.Nodes[nodeID]
		if node.BoundaryKind != BoundaryOpen || node.ElevationM == nil || *node.ElevationM != -40 || node.SamplingMethod != SamplingExact {
			t.Fatalf("открытая граница не должна получать береговой ноль: %+v", node)
		}
	}
}

func TestBuildRejectsNodeOutsideAquatoryCells(t *testing.T) {
	source := threeByThreeMesh()
	outside := georeferencedPoint(300, 300)
	source.Nodes = append(source.Nodes, outside)
	model, err := Build(source, constantSampler(-25, SamplingExact, 0), BuildConfig{CoastTransitionWidthM: 200})
	if err != nil {
		t.Fatal(err)
	}
	node := model.Nodes[10]
	if node.ElevationM != nil || node.QualityFlag != QualityRejected || model.Reconciliation.CorrectionCounts.OutsideAquatoryRejected != 1 {
		t.Fatalf("узел вне водных ячеек должен быть исключён из модели: %+v", node)
	}
}

func TestRegularGridSamplerRestoresPlaneAndReportsBowlError(t *testing.T) {
	planePoints := []geometry.BathymetryPoint{
		{Lat: 45, Lon: 30, Depth: -100},
		{Lat: 45, Lon: 30.01, Depth: -150},
		{Lat: 45.01, Lon: 30, Depth: -200},
		{Lat: 45.01, Lon: 30.01, Depth: -250},
	}
	grid, err := geometry.BuildGrid(planePoints, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	sampler := RegularGridSampler{Grid: grid}
	sample, err := sampler.SampleElevation(45.005, 30.005, 2_000)
	if err != nil {
		t.Fatal(err)
	}
	if sample.Method != SamplingBilinear || math.Abs(sample.ElevationM+175) > 1e-9 || !sample.SourceDistanceSet || sample.SourceDistanceM <= 0 {
		t.Fatalf("наклонная плоскость восстановлена неверно: %+v", sample)
	}
	exact, err := sampler.SampleElevation(45, 30, 2_000)
	if err != nil || exact.Method != SamplingExact || exact.SourceDistanceM != 0 {
		t.Fatalf("исходная точка должна распознаваться как точная: %+v, %v", exact, err)
	}

	bowlPoints := []geometry.BathymetryPoint{
		{Lat: 45, Lon: 30, Depth: 0},
		{Lat: 45, Lon: 30.01, Depth: -100},
		{Lat: 45.01, Lon: 30, Depth: -100},
		{Lat: 45.01, Lon: 30.01, Depth: -200},
	}
	bowlGrid, err := geometry.BuildGrid(bowlPoints, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	bowl, err := (RegularGridSampler{Grid: bowlGrid}).SampleElevation(45.005, 30.005, 2_000)
	if err != nil {
		t.Fatal(err)
	}
	analytical := -50.0
	if math.Abs(math.Abs(bowl.ElevationM-analytical)-50) > 1e-9 {
		t.Fatalf("для квадратичной чаши ожидалась известная погрешность билинейной аппроксимации 50 м, получено %.6f", bowl.ElevationM-analytical)
	}
}

func TestRegularGridSamplerMarksNearestFallback(t *testing.T) {
	grid, err := geometry.BuildGrid([]geometry.BathymetryPoint{
		{Lat: 45, Lon: 30, Depth: -10},
		{Lat: 45, Lon: 30.01, Depth: -20},
		{Lat: 45.01, Lon: 30, Depth: -30},
	}, 0.01)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := (RegularGridSampler{Grid: grid}).SampleElevation(45.005, 30.005, 2_000)
	if err != nil {
		t.Fatal(err)
	}
	if sample.Method != SamplingNearest || sample.SourceDistanceM <= 0 {
		t.Fatalf("пропуск регулярной ячейки должен дать ближайшую замену: %+v", sample)
	}
}

func TestCellDerivationRestoresFlatShelf(t *testing.T) {
	source := oneCellMesh()
	model, err := Build(source, constantSampler(-100, SamplingBilinear, 50), BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Accepted || len(model.Cells) != 1 {
		t.Fatalf("плоская ячейка должна быть принята: %+v", model.Reasons)
	}
	cell := model.Cells[0]
	if math.Abs(cell.AreaM2-10_000) > 1e-9 || cell.ElevationMinM != -100 || cell.ElevationMaxM != -100 || cell.ElevationMeanM != -100 || cell.WaterDepthMeanM != 100 {
		t.Fatalf("неверные площадь или глубины плоской ячейки: %+v", cell)
	}
	if cell.SlopeDeg > 1e-12 || cell.AspectDeg != nil || cell.RoughnessM > 1e-12 {
		t.Fatalf("плоская ячейка должна иметь нулевые уклон и шероховатость без экспозиции: %+v", cell)
	}
	if cell.Region != RegionShelf || cell.QualityFlag != CellQualityVerified || math.Abs(cell.QualityScore-0.95) > 1e-12 {
		t.Fatalf("неверная классификация плоской ячейки: %+v", cell)
	}
	metadata := model.CellDerivation
	if metadata.RegionThresholds != DefaultRegionThresholds() || metadata.HorizontalUnit != "m" || metadata.ElevationUnit != "m" || metadata.SlopeUnit != "degree" {
		t.Fatalf("метаданные BATHY-03 неполны: %+v", metadata)
	}
	if metadata.Summary.CoveragePercent != 100 || metadata.Summary.RegionCounts.Shelf != 1 || metadata.Summary.QualityCounts.Verified != 1 {
		t.Fatalf("неверная сводка ячеек: %+v", metadata.Summary)
	}
	repeated, err := Build(source, constantSampler(-100, SamplingBilinear, 50), BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(model.Cells, repeated.Cells) || !reflect.DeepEqual(model.CellDerivation, repeated.CellDerivation) {
		t.Fatal("производные характеристики должны воспроизводиться из тех же узлов")
	}
}

func TestCellDerivationCalculatesSlopeAndEasternAspect(t *testing.T) {
	sampler := samplerFunc(func(latitudeDeg, longitudeDeg, _ float64) (Sample, error) {
		x := (longitudeDeg - 34) * 80_000
		elevation := -100 - 0.1*x
		return Sample{ElevationM: elevation, Method: SamplingExact, SourceDistanceM: 0, SourceDistanceSet: true}, nil
	})
	model, err := Build(oneCellMesh(), sampler, BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cell := model.Cells[0]
	expectedSlope := math.Atan(0.1) * 180 / math.Pi
	if math.Abs(cell.SlopeDeg-expectedSlope) > 1e-8 {
		t.Fatalf("неверный уклон: %.12f°, ожидалось %.12f°", cell.SlopeDeg, expectedSlope)
	}
	if cell.AspectDeg == nil || math.Abs(*cell.AspectDeg-90) > 1e-8 {
		t.Fatalf("понижение на восток должно иметь экспозицию 90°, получено %+v", cell.AspectDeg)
	}
	if cell.RoughnessM > 1e-9 {
		t.Fatalf("наклонная плоскость не должна иметь шероховатость: %.12f м", cell.RoughnessM)
	}
}

func TestCellDerivationCalculatesRoughnessFromPlaneResiduals(t *testing.T) {
	sampler := samplerFunc(func(latitudeDeg, longitudeDeg, _ float64) (Sample, error) {
		x := (longitudeDeg - 34) * 80_000
		y := (latitudeDeg - 43) * 111_000
		elevation := -100.0
		if x > 50 && y > 50 {
			elevation = -120
		}
		return Sample{ElevationM: elevation, Method: SamplingExact, SourceDistanceM: 0, SourceDistanceSet: true}, nil
	})
	model, err := Build(oneCellMesh(), sampler, BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cell := model.Cells[0]
	if math.Abs(cell.RoughnessM-5) > 1e-8 {
		t.Fatalf("ожидался RMS остатка 5 м, получено %.12f м", cell.RoughnessM)
	}
	if cell.AspectDeg == nil || math.Abs(*cell.AspectDeg-45) > 1e-8 {
		t.Fatalf("локальное понижение северо-восточного узла должно дать 45°, получено %+v", cell.AspectDeg)
	}
}

func TestCellRegionsUseConfigurableThresholdsAndCoastalTopology(t *testing.T) {
	thresholds := RegionThresholds{CoastMaxDepthM: 10, ShelfMaxDepthM: 100, SlopeMaxDepthM: 1000}
	cases := []struct {
		depth    float64
		coastal  bool
		expected CellRegion
	}{
		{depth: 5, expected: RegionCoast},
		{depth: 50, expected: RegionShelf},
		{depth: 500, expected: RegionSlope},
		{depth: 1500, expected: RegionBasin},
		{depth: 1500, coastal: true, expected: RegionCoast},
	}
	for _, testCase := range cases {
		if actual := classifyCellRegion(testCase.depth, testCase.coastal, thresholds); actual != testCase.expected {
			t.Fatalf("глубина %.0f м, берег=%t: получено %s, ожидалось %s", testCase.depth, testCase.coastal, actual, testCase.expected)
		}
	}
	model, err := Build(oneCellMesh(), constantSampler(-50, SamplingNearest, 200), BuildConfig{RegionThresholds: thresholds})
	if err != nil {
		t.Fatal(err)
	}
	if model.Cells[0].Region != RegionShelf || model.Cells[0].QualityFlag != CellQualityFallback || math.Abs(model.Cells[0].QualityScore-0.6) > 1e-12 {
		t.Fatalf("настраиваемые пороги или качество ближайших замен не применены: %+v", model.Cells[0])
	}
	if _, err := Build(oneCellMesh(), constantSampler(-50, SamplingExact, 0), BuildConfig{
		RegionThresholds: RegionThresholds{CoastMaxDepthM: 100, ShelfMaxDepthM: 50, SlopeMaxDepthM: 1000},
	}); err == nil {
		t.Fatal("неупорядоченные пороги должны отклоняться")
	}
}

func TestCellExportsContainValuesAndDerivationMetadata(t *testing.T) {
	model, err := Build(oneCellMesh(), constantSampler(-100, SamplingBilinear, 50), BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	cellsPath := filepath.Join(directory, "cells.csv")
	metadataPath := filepath.Join(directory, "cell-derivation.json")
	if err := WriteCellsCSV(cellsPath, model); err != nil {
		t.Fatal(err)
	}
	if err := WriteCellDerivationJSON(metadataPath, model.CellDerivation); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(cellsPath)
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(file).ReadAll()
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1][1] != "[1,2,3,4]" || records[1][8] != "" || records[1][10] != string(RegionShelf) {
		t.Fatalf("cells.csv содержит неверные поля: %v", records)
	}
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded CellDerivationMetadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Summary.AssignedCellCount != 1 || decoded.SlopeAspectMethod == "" || decoded.RoughnessMethod == "" || decoded.RegionThresholds != DefaultRegionThresholds() {
		t.Fatalf("JSON-метаданные BATHY-03 неполны: %+v", decoded)
	}
}

func TestExportsKeepNullsAndCompleteCorrectionLedger(t *testing.T) {
	source := threeByThreeMesh()
	center := source.Nodes[9]
	sampler := samplerFunc(func(latitudeDeg, longitudeDeg, _ float64) (Sample, error) {
		if math.Abs(latitudeDeg-center.LatitudeDeg) < 1e-12 && math.Abs(longitudeDeg-center.LongitudeDeg) < 1e-12 {
			return Sample{}, errors.New("нет покрытия")
		}
		return Sample{ElevationM: -10, Method: SamplingBilinear, SourceDistanceM: 20, SourceDistanceSet: true}, nil
	})
	model, err := Build(source, sampler, BuildConfig{CoastTransitionWidthM: 200})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	nodesPath := filepath.Join(directory, "node-depth.csv")
	correctionsPath := filepath.Join(directory, "reconciliation-corrections.csv")
	reportPath := filepath.Join(directory, "reconciliation-report.json")
	if err := WriteNodeDepthCSV(nodesPath, model); err != nil {
		t.Fatal(err)
	}
	if err := WriteCorrectionsCSV(correctionsPath, model.Reconciliation.Corrections); err != nil {
		t.Fatal(err)
	}
	if err := WriteReconciliationJSON(reportPath, model.Reconciliation); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(nodesPath)
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(file).ReadAll()
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if records[9][5] != "" || records[9][6] != "" || records[9][9] != string(QualityNoData) {
		t.Fatalf("NoData в CSV должен оставаться пустым: %v", records[9])
	}

	correctionFile, err := os.Open(correctionsPath)
	if err != nil {
		t.Fatal(err)
	}
	correctionRows, err := csv.NewReader(correctionFile).ReadAll()
	_ = correctionFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(correctionRows)-1 != model.Reconciliation.TotalCorrections {
		t.Fatalf("CSV содержит %d исправлений вместо %d", len(correctionRows)-1, model.Reconciliation.TotalCorrections)
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ReconciliationSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Corrections) != decoded.TotalCorrections {
		t.Fatalf("JSON-отчёт потерял журнал исправлений: %+v", decoded)
	}
}

func constantSampler(elevation float64, method SamplingMethod, distance float64) ElevationSampler {
	return samplerFunc(func(_, _ float64, _ float64) (Sample, error) {
		return Sample{ElevationM: elevation, Method: method, SourceDistanceM: distance, SourceDistanceSet: true}, nil
	})
}

func threeByThreeMesh() mesh.Mesh {
	nodes := []mesh.Point{{}}
	for _, coordinate := range [][2]float64{
		{0, 0}, {100, 0}, {200, 0}, {200, 100}, {200, 200}, {100, 200}, {0, 200}, {0, 100}, {100, 100},
	} {
		nodes = append(nodes, georeferencedPoint(coordinate[0], coordinate[1]))
	}
	return mesh.Mesh{
		Nodes: nodes,
		Cells: []mesh.Cell{
			{Nodes: [4]int{1, 2, 9, 8}, NodeCount: 4},
			{Nodes: [4]int{2, 3, 4, 9}, NodeCount: 4},
			{Nodes: [4]int{9, 4, 5, 6}, NodeCount: 4},
			{Nodes: [4]int{8, 9, 6, 7}, NodeCount: 4},
		},
		BoundaryEdges: [][2]int{{1, 2}, {2, 3}, {3, 4}, {4, 5}, {5, 6}, {6, 7}, {7, 8}, {8, 1}},
		QuadCount:     4,
	}
}

func oneCellMesh() mesh.Mesh {
	return mesh.Mesh{
		Nodes: []mesh.Point{
			{},
			georeferencedPoint(0, 0),
			georeferencedPoint(100, 0),
			georeferencedPoint(100, 100),
			georeferencedPoint(0, 100),
		},
		Cells:     []mesh.Cell{{Nodes: [4]int{1, 2, 3, 4}, NodeCount: 4}},
		QuadCount: 1,
	}
}

func georeferencedPoint(x, y float64) mesh.Point {
	return mesh.Point{
		X:                        x,
		Y:                        y,
		LongitudeDeg:             34 + x/80_000,
		LatitudeDeg:              43 + y/111_000,
		GeographicCoordinatesSet: true,
	}
}
