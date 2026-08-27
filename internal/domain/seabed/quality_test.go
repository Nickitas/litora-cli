package seabed

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coastal-geometry/internal/domain/mesh"
)

func TestEvaluateReliefQualityAcceptsControlledShiftWithinSourceResolution(t *testing.T) {
	reference := buildReliefQualityModel(t, 0)
	evaluated := buildReliefQualityModel(t, 5)
	report, err := EvaluateReliefQuality(
		reference, testExportMetadata(), evaluated, testExportMetadata(), testReliefReferencePassport(),
		testReliefQualityConfig(evaluated),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !report.MetricsAccepted || !report.PublicationReady || len(report.Reasons) != 0 {
		t.Fatalf("контролируемое смещение должно укладываться в разрешение источника: %+v", report.Reasons)
	}
	if math.Abs(report.Depth.BiasM-5) > 1e-9 || math.Abs(report.Depth.MAEM-5) > 1e-9 || math.Abs(report.Depth.RMSEM-5) > 1e-9 {
		t.Fatalf("ошибка постоянного смещения рассчитана неверно: %+v", report.Depth)
	}
	if len(report.Isobaths) != 1 || !report.Isobaths[0].Comparable || math.Abs(report.Isobaths[0].P95DistanceM-25) > 1e-7 {
		t.Fatalf("расстояние между изобатами должно быть 25 м: %+v", report.Isobaths)
	}
	if report.Slope.RMSEDeg > 1e-9 || report.Mesh.P05QuadQuality != 1 || report.Mesh.TargetSizeCompliancePercent != 100 {
		t.Fatalf("уклон и квадратная сетка должны сохраняться точно: slope=%+v mesh=%+v", report.Slope, report.Mesh)
	}
	if len(report.DepthBands) != 2 || !report.DepthBands[0].Accepted || !report.DepthBands[1].Accepted {
		t.Fatalf("площади глубинных зон должны укладываться в пространственный допуск: %+v", report.DepthBands)
	}
	if len(report.WorstCells) != 4 || report.WorstCells[0].AbsoluteErrorM != 5 {
		t.Fatalf("локальные худшие ячейки не сохранены: %+v", report.WorstCells)
	}
}

func TestEvaluateReliefQualityRejectsLargeBiasAndMissingIsobath(t *testing.T) {
	reference := buildReliefQualityModel(t, 0)
	evaluated := buildReliefQualityModel(t, 60)
	report, err := EvaluateReliefQuality(
		reference, testExportMetadata(), evaluated, testExportMetadata(), testReliefReferencePassport(),
		testReliefQualityConfig(evaluated),
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.MetricsAccepted || report.PublicationReady || len(report.Reasons) == 0 {
		t.Fatalf("большое смещение не должно проходить QA-02: %+v", report)
	}
	joined := strings.Join(report.Reasons, "\n")
	for _, marker := range []string{"RMSE глубины", "P95 ошибки глубины", "изобата 80 м несопоставима"} {
		if !strings.Contains(joined, marker) {
			t.Fatalf("причины не содержат %q: %s", marker, joined)
		}
	}
}

func TestReliefQualityKeepsInterproductControlOutOfPublicationStatus(t *testing.T) {
	reference := buildReliefQualityModel(t, 0)
	evaluated := buildReliefQualityModel(t, 0)
	passport := testReliefReferencePassport()
	passport.ValidationClass = ReliefValidationInterproduct
	report, err := EvaluateReliefQuality(
		reference, testExportMetadata(), evaluated, testExportMetadata(), passport,
		testReliefQualityConfig(evaluated),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !report.MetricsAccepted || report.PublicationReady {
		t.Fatalf("межпродуктовый контроль должен пройти метрики, но не называться независимой публикационной проверкой: %+v", report)
	}
}

func TestModelDepthSamplerMakesFallbackDistanceExplicit(t *testing.T) {
	model := buildReliefQualityModel(t, 0)
	sampler, err := NewModelDepthSampler(model)
	if err != nil {
		t.Fatal(err)
	}
	inside, err := sampler.Sample(125, 175, 0)
	if err != nil || inside.NearestFallback || !inside.GradientAvailable {
		t.Fatalf("внутренняя точка должна интерполироваться: sample=%+v err=%v", inside, err)
	}
	if _, err := sampler.Sample(350, 150, 0); err == nil {
		t.Fatal("точка вне покрытия не должна скрыто получать ближайшее значение")
	}
	fallback, err := sampler.Sample(350, 150, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !fallback.NearestFallback || math.Abs(fallback.SourceDistanceM-math.Sqrt(5_000)) > 1e-9 {
		t.Fatalf("ближайшая замена должна сообщать фактическое расстояние: %+v", fallback)
	}
}

func TestIsobathExtractionKeepsContourAlignedWithGridEdge(t *testing.T) {
	model := buildReliefQualityModel(t, 0)
	metrics := evaluateIsobathPreservation(model, model, []float64{70}, 100)
	if len(metrics) != 1 || !metrics[0].Comparable || metrics[0].P95DistanceM != 0 {
		t.Fatalf("изобата на узловой линии должна сравниваться без смещения: %+v", metrics)
	}
	if math.Abs(metrics[0].ReferenceLengthKM-0.3) > 1e-9 || math.Abs(metrics[0].EvaluatedLengthKM-0.3) > 1e-9 {
		t.Fatalf("общая грань изобаты не должна дублироваться: %+v", metrics[0])
	}
}

func TestReliefSegmentIndexMatchesBruteForceDistance(t *testing.T) {
	segments := make([]reliefContourSegment, 0, 64)
	for index := 0; index < 64; index++ {
		x := float64((index*37)%211) - 40
		y := float64((index*83)%197) - 30
		segments = append(segments, reliefContourSegment{
			start: mesh.Point{X: x, Y: y},
			end:   mesh.Point{X: x + float64(10+index%9), Y: y + float64(index%13-6)},
		})
	}
	index := newReliefSegmentIndex(segments)
	for pointIndex := 0; pointIndex < 100; pointIndex++ {
		point := mesh.Point{
			X: float64((pointIndex*47)%251) - 60,
			Y: float64((pointIndex*61)%229) - 50,
		}
		expected := math.Inf(1)
		for _, segment := range segments {
			expected = math.Min(expected, pointSegmentDistanceM(point, segment))
		}
		actual := index.nearestDistance(point)
		if math.Abs(actual-expected) > 1e-9 {
			t.Fatalf("пространственный индекс изменил точное расстояние для точки %+v: %.12f != %.12f", point, actual, expected)
		}
	}
}

func TestReliefQualityReportAndPassportRoundTrip(t *testing.T) {
	directory := t.TempDir()
	passportPath := filepath.Join(directory, "reference.json")
	passportData, err := json.Marshal(testReliefReferencePassport())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(passportPath, passportData, 0o644); err != nil {
		t.Fatal(err)
	}
	passport, err := ReadReliefReferencePassport(passportPath)
	if err != nil {
		t.Fatal(err)
	}
	if passport.ExcludedSources == nil || passport.Limitations == nil {
		t.Fatal("пустые массивы паспорта должны нормализоваться")
	}
	reference := buildReliefQualityModel(t, 0)
	evaluated := buildReliefQualityModel(t, 5)
	report, err := EvaluateReliefQuality(
		reference, testExportMetadata(), evaluated, testExportMetadata(), passport,
		testReliefQualityConfig(evaluated),
	)
	if err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(directory, "relief-quality.json")
	tsvPath := filepath.Join(directory, "relief-quality.tsv")
	if err := WriteReliefQualityJSON(jsonPath, report); err != nil {
		t.Fatal(err)
	}
	if err := WriteReliefQualityTSV(tsvPath, report); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ReliefQualityReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != ReliefQualitySchemaVersion || !decoded.MetricsAccepted || len(decoded.Isobaths) != 1 {
		t.Fatalf("JSON QA-02 потерял поля: %+v", decoded)
	}
	tsv, err := os.ReadFile(tsvPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"Раздел\tПоказатель", "Глубина\tRMSE", "Изобаты\tP95 расстояния 80 м"} {
		if !strings.Contains(string(tsv), marker) {
			t.Fatalf("TSV QA-02 не содержит %q:\n%s", marker, tsv)
		}
	}
}

func buildReliefQualityModel(t *testing.T, depthShiftM float64) Model {
	t.Helper()
	model, err := Build(reliefQualityMesh(), samplerFunc(func(_, longitudeDeg, _ float64) (Sample, error) {
		xM := (longitudeDeg - 34) * 80_000
		waterDepthM := 50 + 0.2*xM + depthShiftM
		return Sample{ElevationM: -waterDepthM, Method: SamplingExact, SourceDistanceM: 0, SourceDistanceSet: true}, nil
	}), BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Accepted {
		t.Fatalf("синтетическая модель должна быть принята: %v", model.Reasons)
	}
	return model
}

func reliefQualityMesh() mesh.Mesh {
	nodes := []mesh.Point{{}}
	for row := 0; row < 4; row++ {
		for column := 0; column < 4; column++ {
			nodes = append(nodes, georeferencedPoint(float64(column*100), float64(row*100)))
		}
	}
	cells := make([]mesh.Cell, 0, 9)
	for row := 0; row < 3; row++ {
		for column := 0; column < 3; column++ {
			bottomLeft := 1 + row*4 + column
			cells = append(cells, mesh.Cell{Nodes: [4]int{bottomLeft, bottomLeft + 1, bottomLeft + 5, bottomLeft + 4}, NodeCount: 4})
		}
	}
	return mesh.Mesh{Nodes: nodes, Cells: cells, QuadCount: len(cells), SurfacePhysicalTag: mesh.PhysicalWaterSurface}
}

func testReliefReferencePassport() ReliefReferencePassport {
	return ReliefReferencePassport{
		SchemaVersion:         ReliefReferencePassportSchemaVersion,
		Title:                 "Независимая синтетическая контрольная сетка",
		SourceProduct:         "аналитическая плоскость",
		SourceVersion:         "1",
		DatasetSHA256:         strings.Repeat("a", 64),
		HorizontalResolutionM: 100,
		VerticalUncertaintyM:  4,
		VerticalReference:     testExportMetadata().VerticalReference,
		ValidationClass:       ReliefValidationIndependent,
		SamplingDesign:        "центры независимой регулярной сетки",
	}
}

func testReliefQualityConfig(model Model) ReliefQualityConfig {
	targetSizeM := make([]float64, len(model.Nodes))
	zones := make([]string, len(model.Nodes))
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		targetSizeM[nodeID] = 100
		zones[nodeID] = "control"
	}
	return ReliefQualityConfig{
		IsobathsM: []float64{80}, WorstCellCount: 4,
		TargetSizeM: targetSizeM, TargetZones: zones,
		TargetZoneNames: map[string]string{"control": "Контрольная зона"},
	}
}
