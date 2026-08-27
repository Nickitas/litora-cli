package cobra

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"coastal-geometry/internal/domain/geometry"
	mesh2d "coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

func TestSeabedFullCommandExposesReproducibleInputs(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"seabed", "check-full"})
	if err != nil {
		t.Fatal(err)
	}
	if command != seabedFullCmd {
		t.Fatalf("найдена неверная команда: %s", command.CommandPath())
	}
	for _, flag := range []string{
		"mesh", "mesh-report", "coastline", "bathymetry", "bathymetry-metadata", "output",
		"generator", "target-edge", "boundary-detail", "max-source-distance", "coast-transition",
		"isobaths", "vertical-exaggeration", "control-points",
	} {
		if command.Flags().Lookup(flag) == nil {
			t.Fatalf("команда seabed check-full не содержит флаг --%s", flag)
		}
	}
	for _, marker := range []string{"Одной воспроизводимой командой", "необъяснённых дыр", "опубликованными ориентирами", "publication_ready"} {
		if !strings.Contains(command.Long, marker) {
			t.Fatalf("описание seabed check-full не объясняет %q", marker)
		}
	}
}

func TestHelpForSeabedFullIsRussian(t *testing.T) {
	prepareRussianHelp()
	usage := seabedFullCmd.UsageString()
	for _, marker := range []string{"Использование:", "Флаги:", "полного контура", "показать справку"} {
		if !strings.Contains(usage, marker) {
			t.Fatalf("русская справка QA-03 не содержит %q:\n%s", marker, usage)
		}
	}
	for _, marker := range []string{"Usage:", "Flags:", "help for check-full", "[flags]", "(default "} {
		if strings.Contains(usage, marker) {
			t.Fatalf("в справке QA-03 остался англоязычный legacy-текст %q:\n%s", marker, usage)
		}
	}
}

func TestExecuteFullBlackSeaRunCreatesAuditableBundle(t *testing.T) {
	directory := t.TempDir()
	meshPath := filepath.Join(directory, "flat.msh")
	meshReportPath := filepath.Join(directory, "mesh-comparison.json")
	coastlinePath := filepath.Join(directory, "black-sea.geojson")
	bathymetryPath := filepath.Join(directory, "bathymetry.json")
	bathymetryMetadataPath := filepath.Join(directory, "bathymetry.metadata.json")
	outputDirectory := filepath.Join(directory, "output", "seabed", "full")

	flatMesh := syntheticFullRunMesh()
	if err := mesh2d.WriteMSH2(meshPath, flatMesh); err != nil {
		t.Fatal(err)
	}
	meshReport := meshComparisonReport{
		SchemaVersion: "lito-mesh-comparison/v1", GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		DatasetName: "Black Sea", Source: coastlinePath, GmshVersion: "test-4.15.2",
		Levels: []meshLevelReport{{
			TargetEdgeMeters: 100, BoundaryDetailMeters: 100, BestGenerator: mesh2d.AlgorithmFrontalQuad,
			Results: []meshRunReport{{
				Algorithm: mesh2d.AlgorithmFrontalQuad, MeshFile: meshPath,
				Metrics: mesh2d.QualityMetrics{CellCount: 4, QuadCount: 4, QuadSharePercent: 100},
			}},
		}},
	}
	writeTestJSON(t, meshReportPath, meshReport)
	coastlineJSON := `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"MRGID":3319,"name":"Black Sea"},"geometry":{"type":"Polygon","coordinates":[[[34,43],[34.002,43],[34.002,43.002],[34,43.002],[34,43]]]}}]}`
	if err := os.WriteFile(coastlinePath, []byte(coastlineJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	points := make([]geometry.BathymetryPoint, 0, 9)
	for row := 0; row < 3; row++ {
		for column := 0; column < 3; column++ {
			points = append(points, geometry.BathymetryPoint{Lat: 43 + float64(row)*0.001, Lon: 34 + float64(column)*0.001, Depth: -50})
		}
	}
	bathymetryData, err := json.Marshal(points)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bathymetryPath, bathymetryData, 0o644); err != nil {
		t.Fatal(err)
	}
	downloadedAt := "2026-08-27T00:00:00Z"
	sourceNetCDF := "synthetic.nc"
	sourceChecksum := strings.Repeat("b", 64)
	sourceStep := 15.0
	passport := geometry.BathymetryPassport{
		SchemaVersion: "1.0", Title: "Синтетическая батиметрия Чёрного моря", Status: geometry.BathymetryStatusVerifiedDerived,
		DatasetFile: filepath.Base(bathymetryPath), DatasetSHA256: fmt.Sprintf("%x", sha256.Sum256(bathymetryData)),
		CreatedAt: downloadedAt, PointCount: len(points),
		Bounds:                  geometry.Bounds{MinLat: 43, MaxLat: 43.002, MinLon: 34, MaxLon: 34.002},
		TargetResolutionDegrees: 0.001, TargetResolutionArcSeconds: 3.6,
		SourceProduct: "Синтетический GEBCO для интеграционного теста", SourceDownloadedAt: &downloadedAt,
		SourceNetCDF: &sourceNetCDF, SourceNetCDFSHA256: &sourceChecksum, SourceGridIntervalArcSeconds: &sourceStep,
		HorizontalReference: "WGS 84", VerticalReference: "Средний уровень моря",
		VerticalReferenceCaveat: "Синтетическая вертикальная система используется только в интеграционном тесте.",
		ResamplingMethod:        "аналитическая регулярная сетка", ProcessingScript: "интеграционный тест",
		License: "тестовые данные", Attribution: "Синтетический тест QA-03",
	}
	writeTestJSON(t, bathymetryMetadataPath, passport)

	qualityConfig := seabed.FullBlackSeaQualityConfig{
		TargetEdgeM: 100, MeanEdgeTolerancePercent: 1,
		ExpectedBounds:     seabed.GeographicBounds{MinLatitudeDeg: 43, MaxLatitudeDeg: 43.002, MinLongitudeDeg: 34, MaxLongitudeDeg: 34.002},
		ExtentToleranceDeg: 1e-9, CoastlineReferenceAreaKM2: 0.04, CoastlineAreaTolerancePercent: 1,
		PublishedAreaTolerancePercent: 1, PublishedVolumeTolerancePercent: 1, PublishedDepthTolerancePercent: 1,
		PublishedReferences: []seabed.PublishedBasinReference{{
			ID: "synthetic", Title: "Синтетический ориентир", Citation: "интеграционный тест",
			URL: "https://example.invalid/synthetic", AreaKM2: 0.04, VolumeKM3: 0.00025, MaxDepthM: 25, Primary: true,
		}},
	}
	report, err := executeFullBlackSeaRun(fullRunOptions{
		MeshPath: meshPath, MeshReportPath: meshReportPath, CoastlinePath: coastlinePath,
		BathymetryPath: bathymetryPath, BathymetryMetadata: bathymetryMetadataPath,
		OutputDirectory: outputDirectory, Generator: mesh2d.AlgorithmFrontalQuad,
		TargetEdgeM: 100, BoundaryDetailM: 100, MaxSourceDistanceM: 500,
		IsobathsM: []float64{5, 10, 20}, VerticalScale: 10, ControlPoints: true,
		QualityConfig: &qualityConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Accepted || report.PublicationReady {
		t.Fatalf("неверный итог интеграционного QA-03: accepted=%v publication=%v reasons=%v", report.Accepted, report.PublicationReady, report.Reasons)
	}
	for _, path := range []string{
		report.Artifacts.BathymetricMSH, report.Artifacts.VTU, report.Artifacts.NodesCSV,
		report.Artifacts.CellsCSV, report.Artifacts.OverviewSVG, report.Artifacts.Relief3DSVG,
		report.Artifacts.QualityJSON, report.Artifacts.QualityTSV,
	} {
		if info, statErr := os.Stat(path); statErr != nil || info.Size() == 0 {
			t.Fatalf("артефакт полного прогона не создан: %s (%v)", path, statErr)
		}
	}
	if len(report.Resources.Stages) < 6 || report.Resources.DurationSeconds <= 0 || len(report.Inputs) != 5 {
		t.Fatalf("ресурсный журнал или входы неполны: resources=%+v inputs=%+v", report.Resources, report.Inputs)
	}
}

func syntheticFullRunMesh() mesh2d.Mesh {
	nodes := []mesh2d.Point{{}}
	for row := 0; row < 3; row++ {
		for column := 0; column < 3; column++ {
			nodes = append(nodes, mesh2d.Point{
				X: float64(column * 100), Y: float64(row * 100),
				LongitudeDeg: 34 + float64(column)*0.001, LatitudeDeg: 43 + float64(row)*0.001,
				GeographicCoordinatesSet: true,
			})
		}
	}
	return mesh2d.Mesh{
		Nodes: nodes,
		Cells: []mesh2d.Cell{
			{Nodes: [4]int{1, 2, 5, 4}, NodeCount: 4}, {Nodes: [4]int{2, 3, 6, 5}, NodeCount: 4},
			{Nodes: [4]int{4, 5, 8, 7}, NodeCount: 4}, {Nodes: [4]int{5, 6, 9, 8}, NodeCount: 4},
		},
		BoundaryEdges: [][2]int{{1, 2}, {2, 3}, {3, 6}, {6, 9}, {9, 8}, {8, 7}, {7, 4}, {4, 1}},
		QuadCount:     4, SurfacePhysicalTag: mesh2d.PhysicalWaterSurface,
	}
}

func writeTestJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
