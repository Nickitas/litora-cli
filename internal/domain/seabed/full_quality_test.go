package seabed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvaluateFullBlackSeaQualityAcceptsConnectedFullQuadModel(t *testing.T) {
	model := buildFullQualityModel(t)
	config := fullQualityTestConfig(model)

	report, err := EvaluateFullBlackSeaQuality(model, config)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Accepted {
		t.Fatalf("корректный полный контур должен пройти QA-03: %v", report.Reasons)
	}
	if report.PublicationReady {
		t.Fatal("системный прогон без независимого QA-02 не должен считаться публикационно готовым")
	}
	if report.Topology.CellComponentCount != 1 || report.Topology.UnexpectedBoundaryEdgeCount != 0 {
		t.Fatalf("неверная топология: %+v", report.Topology)
	}
	if report.Depth.PositiveElevationCount != 0 || report.Depth.NoDataCellCount != 0 {
		t.Fatalf("неверная проверка глубин: %+v", report.Depth)
	}
	if report.EdgeSize.MeanEdgeM != 100 {
		t.Fatalf("ожидалось среднее ребро 100 м, получено %.12f", report.EdgeSize.MeanEdgeM)
	}

	config.IndependentReliefValidationPassed = true
	publicationReport, err := EvaluateFullBlackSeaQuality(model, config)
	if err != nil {
		t.Fatal(err)
	}
	if !publicationReport.PublicationReady {
		t.Fatal("принятый прогон с независимым QA-02 должен быть публикационно готов")
	}
}

func TestEvaluateFullBlackSeaQualityFindsGapAndPositiveElevation(t *testing.T) {
	model := buildFullQualityModel(t)
	model.Mesh.BoundaryEdges = model.Mesh.BoundaryEdges[1:]
	positive := 5.0
	zero := 0.0
	model.Nodes[9].ElevationM = &positive
	model.Nodes[9].WaterDepthM = &zero

	report, err := EvaluateFullBlackSeaQuality(model, fullQualityTestConfig(model))
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted {
		t.Fatal("разрыв границы и положительная отметка должны отклонить QA-03")
	}
	if report.Topology.UnexpectedBoundaryEdgeCount != 1 {
		t.Fatalf("ожидался один необъяснённый разрыв, получено %+v", report.Topology)
	}
	if report.Depth.PositiveElevationCount != 1 {
		t.Fatalf("ожидалась одна положительная отметка, получено %+v", report.Depth)
	}
	if len(report.Reasons) == 0 {
		t.Fatal("отчёт обязан объяснить отклонение")
	}
}

func TestEvaluateFullBlackSeaQualityFindsSelfIntersectingQuad(t *testing.T) {
	model := buildFullQualityModel(t)
	model.Mesh.Cells[0].Nodes = [4]int{1, 9, 2, 8}

	report, err := EvaluateFullBlackSeaQuality(model, fullQualityTestConfig(model))
	if err != nil {
		t.Fatal(err)
	}
	if report.Topology.SelfIntersectingCellCount != 1 || report.Topology.Accepted {
		t.Fatalf("самопересекающаяся ячейка не обнаружена: %+v", report.Topology)
	}
}

func TestEvaluateFullBlackSeaQualityExplainsLocalDepthOutlierWithoutHidingIntegrals(t *testing.T) {
	model := buildFullQualityModel(t)
	config := fullQualityTestConfig(model)
	config.PublishedReferences[0].MaxDepthM = 100

	report, err := EvaluateFullBlackSeaQuality(model, config)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Accepted || report.PublishedComparisons[0].DepthAccepted {
		t.Fatalf("локальный экстремум должен остаться диагностикой при принятых площади и объёме: %+v", report.PublishedComparisons[0])
	}
	if len(report.Warnings) == 0 || !strings.Contains(report.Warnings[0], "максимальная глубина") {
		t.Fatalf("локальное отклонение обязано быть объяснено: %v", report.Warnings)
	}
}

func TestEvaluateFullBlackSeaQualityRejectsExcessiveNearestFallback(t *testing.T) {
	model, err := Build(threeByThreeMesh(), constantSampler(-50, SamplingNearest, 10_000), BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	config := fullQualityTestConfig(model)
	config.NearestFallbackMaxPercent = 5
	config.LongFallbackWarningM = 5_000

	report, err := EvaluateFullBlackSeaQuality(model, config)
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted || report.Depth.Accepted || report.Depth.NearestFallbackPercent <= 5 {
		t.Fatalf("избыточная ближайшая замена должна отклонить QA-03: %+v", report.Depth)
	}
	if report.Depth.LongFallbackNodeCount == 0 || len(report.Warnings) == 0 {
		t.Fatalf("дальняя замена должна быть явно объяснена: depth=%+v warnings=%v", report.Depth, report.Warnings)
	}
}

func TestFullBlackSeaQualityJSONAndTSV(t *testing.T) {
	model := buildFullQualityModel(t)
	report, err := EvaluateFullBlackSeaQuality(model, fullQualityTestConfig(model))
	if err != nil {
		t.Fatal(err)
	}
	report.Resources = FullBlackSeaResources{DurationSeconds: 1.25, PeakHeapInUseBytes: 1024}
	directory := t.TempDir()
	jsonPath := filepath.Join(directory, "full-quality.json")
	tsvPath := filepath.Join(directory, "full-quality.tsv")
	if err := WriteFullBlackSeaQualityJSON(jsonPath, report); err != nil {
		t.Fatal(err)
	}
	if err := WriteFullBlackSeaQualityTSV(tsvPath, report); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded FullBlackSeaQualityReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != FullBlackSeaQualitySchemaVersion || !decoded.Accepted {
		t.Fatalf("неверный JSON QA-03: %+v", decoded)
	}
	tsv, err := os.ReadFile(tsvPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"Раздел\tПоказатель", "Интегралы\tПлощадь", "Ресурсы\tОбщая длительность"} {
		if !strings.Contains(string(tsv), marker) {
			t.Fatalf("TSV QA-03 не содержит %q:\n%s", marker, tsv)
		}
	}
}

func buildFullQualityModel(t *testing.T) Model {
	t.Helper()
	model, err := Build(threeByThreeMesh(), constantSampler(-50, SamplingExact, 0), BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Accepted {
		t.Fatalf("синтетическая модель должна быть принята: %v", model.Reasons)
	}
	return model
}

func fullQualityTestConfig(model Model) FullBlackSeaQualityConfig {
	areaKM2 := 0.0
	volumeKM3 := 0.0
	maxDepthM := 0.0
	for _, cell := range model.Cells {
		areaKM2 += cell.AreaM2 / 1e6
		volumeKM3 += cell.AreaM2 * cell.WaterDepthMeanM / 1e9
	}
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		if model.Nodes[nodeID].WaterDepthM != nil && *model.Nodes[nodeID].WaterDepthM > maxDepthM {
			maxDepthM = *model.Nodes[nodeID].WaterDepthM
		}
	}
	return FullBlackSeaQualityConfig{
		TargetEdgeM: 100, MeanEdgeTolerancePercent: 1,
		ExpectedBounds: GeographicBounds{
			MinLatitudeDeg:  model.Mesh.Nodes[1].LatitudeDeg,
			MaxLatitudeDeg:  model.Mesh.Nodes[5].LatitudeDeg,
			MinLongitudeDeg: model.Mesh.Nodes[1].LongitudeDeg,
			MaxLongitudeDeg: model.Mesh.Nodes[3].LongitudeDeg,
		},
		ExtentToleranceDeg: 1e-9, CoastlineReferenceAreaKM2: areaKM2,
		CoastlineAreaTolerancePercent: 1, PublishedAreaTolerancePercent: 1,
		PublishedVolumeTolerancePercent: 1, PublishedDepthTolerancePercent: 1,
		PublishedReferences: []PublishedBasinReference{{
			ID: "synthetic", Title: "Синтетический ориентир", Citation: "аналитический тест",
			URL: "https://example.invalid/synthetic", AreaKM2: areaKM2, VolumeKM3: volumeKM3,
			MaxDepthM: maxDepthM, Primary: true,
		}},
	}
}
