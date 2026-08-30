package cobra

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coastal-geometry/internal/domain/geometry"
	mesh2d "coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

func TestSeabedBuildCommandExposesReproducibleInputsAndLimits(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"seabed", "build"})
	if err != nil {
		t.Fatal(err)
	}
	if command != seabedBuildCmd {
		t.Fatalf("найдена неверная команда: %s", command.CommandPath())
	}
	for _, flag := range []string{
		"mesh", "bathymetry", "bathymetry-metadata", "coastline", "output",
		"boundary-detail", "max-source-distance", "coast-transition", "recover-wgs84",
		"max-nodes", "max-cells", "max-output-mib", "allow-large",
	} {
		if command.Flags().Lookup(flag) == nil {
			t.Fatalf("команда seabed build не содержит флаг --%s", flag)
		}
	}
	for _, marker := range []string{"плоскую full-quad MSH", "output/seabed", "WGS 84", "seabed render"} {
		if !strings.Contains(command.Long, marker) {
			t.Fatalf("описание seabed build не объясняет %q", marker)
		}
	}
}

func TestHelpForSeabedBuildIsRussian(t *testing.T) {
	prepareRussianHelp()
	usage := seabedBuildCmd.UsageString()
	for _, marker := range []string{"Использование:", "Флаги:", "показать справку", "плоская full-quad"} {
		if !strings.Contains(usage, marker) {
			t.Fatalf("русская справка seabed build не содержит %q:\n%s", marker, usage)
		}
	}
	for _, marker := range []string{"Usage:", "Flags:", "help for build", "[flags]", "(default "} {
		if strings.Contains(usage, marker) {
			t.Fatalf("в справке seabed build остался англоязычный текст %q:\n%s", marker, usage)
		}
	}
}

func TestExecuteSeabedBuildCreatesExportBundleAndJournal(t *testing.T) {
	fixture := newSeabedBuildFixture(t, false)
	report, err := executeSeabedBuild(seabedBuildOptions{
		MeshPath: fixture.meshPath, BathymetryPath: fixture.bathymetryPath,
		BathymetryMetadata: fixture.bathymetryMetadataPath, CoastlinePath: fixture.coastlinePath,
		OutputDirectory: filepath.Join(fixture.directory, "result"), BoundaryDetailM: 100,
		MaxSourceDistanceM: 500, RecoverWGS84: true, MaxNodes: 100, MaxCells: 100,
		MaxOutputMiB: 256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Accepted || report.CoordinatesRecovered || len(report.Inputs) != 4 {
		t.Fatalf("неверный отчёт seabed build: %+v", report)
	}
	if report.Sampling.NoDataNodeCount != 0 || report.CellDerivation.Summary.NoDataCellCount != 0 {
		t.Fatalf("сборка не должна оставлять NoData: sampling=%+v cells=%+v", report.Sampling, report.CellDerivation.Summary)
	}
	if report.Resources.DurationSeconds <= 0 || report.Resources.HeapInUseAfterBuild == 0 {
		t.Fatalf("не сохранены ресурсы: %+v", report.Resources)
	}
	for _, path := range []string{
		report.Artifacts.BathymetricMSH, report.Artifacts.VTU, report.Artifacts.NodesCSV,
		report.Artifacts.CellsCSV, report.Artifacts.ProfilesCSV, report.Artifacts.ExportMetadataJSON,
		report.Artifacts.ReconciliationJSON, report.Artifacts.CorrectionsCSV,
		report.Artifacts.CellDerivationJSON, report.Artifacts.BuildReportJSON, report.Artifacts.BuildLog,
	} {
		info, statErr := os.Stat(path)
		if statErr != nil || info.Size() == 0 {
			t.Fatalf("артефакт seabed build не создан: %s (%v)", path, statErr)
		}
	}
	document, err := seabed.ReadMSH2(report.Artifacts.BathymetricMSH)
	if err != nil {
		t.Fatal(err)
	}
	if document.Metadata.ModelKind != seabed.MSHModelSeabed || !document.Model.Accepted {
		t.Fatalf("создан неверный MSH: metadata=%+v accepted=%v", document.Metadata, document.Model.Accepted)
	}
}

func TestPrepareSeabedBuildMeshRecoversLegacyWGS84OnlyWhenAllowed(t *testing.T) {
	fixture := newSeabedBuildFixture(t, true)
	flat, err := mesh2d.ReadMSH2(fixture.meshPath)
	if err != nil {
		t.Fatal(err)
	}
	projection := mesh2d.NewEqualAreaProjection([][]geometry.LatLon{fixture.outer})
	if _, _, err := prepareSeabedBuildMesh(flat, projection, false); err == nil || !strings.Contains(err.Error(), "--recover-wgs84") {
		t.Fatalf("ожидался явный отказ без восстановления WGS 84, получено: %v", err)
	}
	recovered, usedRecovery, err := prepareSeabedBuildMesh(flat, projection, true)
	if err != nil {
		t.Fatal(err)
	}
	if !usedRecovery || !recovered.Nodes[1].GeographicCoordinatesSet {
		t.Fatalf("WGS 84 не восстановлен: recovered=%v node=%+v", usedRecovery, recovered.Nodes[1])
	}
}

func TestSeabedBuildRejectsUnsafeOutputAndPreflight(t *testing.T) {
	if err := validateSeabedBuildOutput(filepath.Join(t.TempDir(), "outside-output")); err == nil {
		t.Fatal("ожидался отказ для каталога вне output/")
	}
	if err := validateSeabedBuildOutput(filepath.Join("output", "seabed", "experiment")); err != nil {
		t.Fatalf("каталог внутри output/ должен быть разрешён: %v", err)
	}
	preflight := seabedBuildPreflight{NodeCount: 101, CellCount: 4, EstimatedOutputBytes: 1, MaxNodes: 100, MaxCells: 100, MaxOutputMiB: 1}
	if err := validateSeabedBuildPreflight(preflight); err == nil || !strings.Contains(err.Error(), "--max-nodes") {
		t.Fatalf("ожидался защитный лимит узлов, получено: %v", err)
	}
	options := seabedBuildOptions{
		MeshPath: "mesh.msh", BathymetryPath: "bathymetry.json", BathymetryMetadata: "bathymetry.metadata.json",
		CoastlinePath: "black-sea.geojson", OutputDirectory: "output/seabed", BoundaryDetailM: 100,
		MaxSourceDistanceM: 0, MaxNodes: 1, MaxCells: 1, MaxOutputMiB: 1,
	}
	if err := validateSeabedBuildOptions(options); err != nil {
		t.Fatalf("нулевое расстояние должно запрещать fallback, а не отклонять запуск: %v", err)
	}
}

type seabedBuildFixture struct {
	directory              string
	meshPath               string
	coastlinePath          string
	bathymetryPath         string
	bathymetryMetadataPath string
	outer                  []geometry.LatLon
}

func newSeabedBuildFixture(t *testing.T, withoutWGS84 bool) seabedBuildFixture {
	t.Helper()
	directory := t.TempDir()
	outer := []geometry.LatLon{
		{Lat: 43, Lon: 34}, {Lat: 43, Lon: 34.002}, {Lat: 43.002, Lon: 34.002}, {Lat: 43.002, Lon: 34}, {Lat: 43, Lon: 34},
	}
	coastlinePath := filepath.Join(directory, "black-sea.geojson")
	coastlineJSON := `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"MRGID":3319,"name":"Black Sea"},"geometry":{"type":"Polygon","coordinates":[[[34,43],[34.002,43],[34.002,43.002],[34,43.002],[34,43]]]}}]}`
	if err := os.WriteFile(coastlinePath, []byte(coastlineJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	projection := mesh2d.NewEqualAreaProjection([][]geometry.LatLon{outer})
	meshPath := filepath.Join(directory, "flat.msh")
	if err := mesh2d.WriteMSH2(meshPath, newSeabedBuildMesh(projection, withoutWGS84)); err != nil {
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
	bathymetryPath := filepath.Join(directory, "bathymetry.json")
	if err := os.WriteFile(bathymetryPath, bathymetryData, 0o644); err != nil {
		t.Fatal(err)
	}
	downloadedAt := "2026-08-27T00:00:00Z"
	netcdf := "synthetic.nc"
	netcdfChecksum := strings.Repeat("a", 64)
	step := 15.0
	passport := geometry.BathymetryPassport{
		SchemaVersion: "1.0", Title: "Синтетическая батиметрия Чёрного моря", Status: geometry.BathymetryStatusVerifiedDerived,
		DatasetFile: filepath.Base(bathymetryPath), DatasetSHA256: fmt.Sprintf("%x", sha256.Sum256(bathymetryData)),
		CreatedAt: downloadedAt, PointCount: len(points),
		Bounds:                  geometry.Bounds{MinLat: 43, MaxLat: 43.002, MinLon: 34, MaxLon: 34.002},
		TargetResolutionDegrees: 0.001, TargetResolutionArcSeconds: 3.6,
		SourceProduct: "Синтетический GEBCO для CLI-01", SourceDownloadedAt: &downloadedAt,
		SourceNetCDF: &netcdf, SourceNetCDFSHA256: &netcdfChecksum, SourceGridIntervalArcSeconds: &step,
		HorizontalReference: "WGS 84", VerticalReference: "Средний уровень моря",
		VerticalReferenceCaveat: "Синтетическая вертикальная система используется только в интеграционном тесте.",
		ResamplingMethod:        "аналитическая регулярная сетка", ProcessingScript: "интеграционный тест",
		License: "тестовые данные", Attribution: "Синтетический тест CLI-01",
	}
	bathymetryMetadataPath := filepath.Join(directory, "bathymetry.metadata.json")
	writeTestJSON(t, bathymetryMetadataPath, passport)
	return seabedBuildFixture{
		directory: directory, meshPath: meshPath, coastlinePath: coastlinePath,
		bathymetryPath: bathymetryPath, bathymetryMetadataPath: bathymetryMetadataPath, outer: outer,
	}
}

func newSeabedBuildMesh(projection mesh2d.EqualAreaProjection, withoutWGS84 bool) mesh2d.Mesh {
	nodes := []mesh2d.Point{{}}
	for row := 0; row < 3; row++ {
		for column := 0; column < 3; column++ {
			point := projection.Project(geometry.LatLon{Lat: 43 + float64(row)*0.001, Lon: 34 + float64(column)*0.001})
			if withoutWGS84 {
				point.LongitudeDeg = 0
				point.LatitudeDeg = 0
				point.GeographicCoordinatesSet = false
			}
			nodes = append(nodes, point)
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
