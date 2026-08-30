package cobra

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"
	mesh2d "coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"

	"github.com/spf13/cobra"
)

var (
	seabedFullMesh               string
	seabedFullMeshReport         string
	seabedFullCoastline          string
	seabedFullBathymetry         string
	seabedFullBathymetryMetadata string
	seabedFullOutput             string
	seabedFullGenerator          string
	seabedFullTargetEdge         float64
	seabedFullBoundaryDetail     float64
	seabedFullMaxSourceDistance  float64
	seabedFullCoastTransition    float64
	seabedFullIsobaths           string
	seabedFullVerticalScale      float64
	seabedFullControlPoints      bool
)

var seabedFullCmd = &cobra.Command{
	Use:   "check-full",
	Short: "Проверить полный контур Чёрного моря на сетке 1000 м",
	Long: `Одной воспроизводимой командой читает зафиксированную full-quad сетку
1000 м всего Чёрного моря, назначает глубины из проверенного производного
набора GEBCO, экспортирует MSH/VTU/CSV и строит карты VIEW-01–VIEW-03.

QA-03 проверяет географический охват, связность, отсутствие необъяснённых дыр,
самопересечений, нечетырёхугольных ячеек, NoData и положительных отметок.
Площадь и объём сравниваются с исходным контуром MarineRegions и отдельно с
опубликованными ориентирами. JSON сохраняет контрольные суммы входов, версии
GEBCO/Gmsh/Go, длительность каждого этапа и наблюдавшийся пик памяти.

Системное принятие полного контура не выдаётся за публикационную готовность:
для publication_ready дополнительно нужен независимый отчёт QA-02 класса
independent_measurements.`,
	RunE: runSeabedFull,
}

func init() {
	seabedCmd.AddCommand(seabedFullCmd)
	seabedFullCmd.Flags().StringVar(&seabedFullMesh, "mesh", "output/mesh/msh/black-sea-edge-1000-detail-1000-frontal-quad.msh", "плоская full-quad MSH всего Чёрного моря")
	seabedFullCmd.Flags().StringVar(&seabedFullMeshReport, "mesh-report", "output/mesh/mesh-comparison.json", "паспорт запуска генераторов 2D-сетки")
	seabedFullCmd.Flags().StringVar(&seabedFullCoastline, "coastline", "data/cache/black-sea.geojson", "GeoJSON полного контура Чёрного моря")
	seabedFullCmd.Flags().StringVar(&seabedFullBathymetry, "bathymetry", "output/export-01-verification/gebco2026-0.02deg.json", "производный JSON батиметрии GEBCO")
	seabedFullCmd.Flags().StringVar(&seabedFullBathymetryMetadata, "bathymetry-metadata", "output/export-01-verification/gebco2026-0.02deg.metadata.json", "паспорт производного набора GEBCO")
	seabedFullCmd.Flags().StringVar(&seabedFullOutput, "output", "output/seabed/full-black-sea-1000m", "каталог полного результата QA-03")
	seabedFullCmd.Flags().StringVar(&seabedFullGenerator, "generator", "frontal-quad", "генератор контрольной сетки из mesh-report")
	seabedFullCmd.Flags().Float64Var(&seabedFullTargetEdge, "target-edge", 1000, "контрольная средняя длина ребра, м")
	seabedFullCmd.Flags().Float64Var(&seabedFullBoundaryDetail, "boundary-detail", 1000, "допуск детализации исходного берега, м")
	seabedFullCmd.Flags().Float64Var(&seabedFullMaxSourceDistance, "max-source-distance", 50000, "явный предел ближайшей замены GEBCO у несовпадающих береговых масок, м")
	seabedFullCmd.Flags().Float64Var(&seabedFullCoastTransition, "coast-transition", 0, "ширина перехода к Z=0, м; 0 означает два средних ребра")
	seabedFullCmd.Flags().StringVar(&seabedFullIsobaths, "isobaths", "20,50,100,200,500,1000,1500,2000", "глубины изобат в метрах через запятую")
	seabedFullCmd.Flags().Float64Var(&seabedFullVerticalScale, "vertical-exaggeration", 40, "вертикальное преувеличение 3D-вида")
	seabedFullCmd.Flags().BoolVar(&seabedFullControlPoints, "control-points", true, "показывать узловые выборки на 3D-виде")
}

type fullRunOptions struct {
	MeshPath           string
	MeshReportPath     string
	CoastlinePath      string
	BathymetryPath     string
	BathymetryMetadata string
	OutputDirectory    string
	Generator          mesh2d.Algorithm
	TargetEdgeM        float64
	BoundaryDetailM    float64
	MaxSourceDistanceM float64
	CoastTransitionM   float64
	IsobathsM          []float64
	VerticalScale      float64
	ControlPoints      bool
	QualityConfig      *seabed.FullBlackSeaQualityConfig
	SkipVisualizations bool
}

func runSeabedFull(_ *cobra.Command, _ []string) error {
	generator, err := mesh2d.ParseAlgorithm(seabedFullGenerator)
	if err != nil {
		return err
	}
	isobaths, err := parsePositiveFloatList(seabedFullIsobaths, "глубины изобат")
	if err != nil {
		return err
	}
	options := fullRunOptions{
		MeshPath: seabedFullMesh, MeshReportPath: seabedFullMeshReport,
		CoastlinePath: seabedFullCoastline, BathymetryPath: seabedFullBathymetry,
		BathymetryMetadata: seabedFullBathymetryMetadata, OutputDirectory: seabedFullOutput,
		Generator: generator, TargetEdgeM: seabedFullTargetEdge, BoundaryDetailM: seabedFullBoundaryDetail,
		MaxSourceDistanceM: seabedFullMaxSourceDistance, CoastTransitionM: seabedFullCoastTransition,
		IsobathsM: isobaths, VerticalScale: seabedFullVerticalScale, ControlPoints: seabedFullControlPoints,
	}
	report, err := executeFullBlackSeaRun(options)
	if report.SchemaVersion != "" && !quiet {
		printFullBlackSeaSummary(report)
	}
	return err
}

func executeFullBlackSeaRun(options fullRunOptions) (seabed.FullBlackSeaQualityReport, error) {
	if err := validateFullRunOptions(options); err != nil {
		return seabed.FullBlackSeaQualityReport{}, err
	}
	tracker := newFullRunResourceTracker()
	defer tracker.closeIfRunning()

	stageStarted := time.Now()
	meshPassport, selectedRun, err := readFullMeshPassport(options)
	if err != nil {
		return seabed.FullBlackSeaQualityReport{}, err
	}
	polygon, err := coastline.LoadPolygon(coastline.LoadOptions{LocalPath: options.CoastlinePath})
	if err != nil {
		return seabed.FullBlackSeaQualityReport{}, fmt.Errorf("чтение полного контура Чёрного моря: %w", err)
	}
	domain, err := mesh2d.PrepareDomain(polygon.Outer, polygon.Holes, options.BoundaryDetailM)
	if err != nil {
		return seabed.FullBlackSeaQualityReport{}, fmt.Errorf("подготовка полного контура Чёрного моря: %w", err)
	}
	flatMesh, err := mesh2d.ReadMSH2(options.MeshPath)
	if err != nil {
		return seabed.FullBlackSeaQualityReport{}, fmt.Errorf("чтение контрольной MSH: %w", err)
	}
	if len(flatMesh.Nodes) > 1 && !flatMesh.Nodes[1].GeographicCoordinatesSet {
		if err := domain.Projection.AssignGeographicCoordinates(flatMesh.Nodes); err != nil {
			return seabed.FullBlackSeaQualityReport{}, fmt.Errorf("GEO-01 для контрольной MSH: %w", err)
		}
	}
	tracker.finishStage("geometry", "Контур и плоская сетка", stageStarted)

	stageStarted = time.Now()
	bathymetryData, err := os.ReadFile(options.BathymetryPath)
	if err != nil {
		return seabed.FullBlackSeaQualityReport{}, fmt.Errorf("чтение батиметрии %q: %w", options.BathymetryPath, err)
	}
	bathymetryPassport, err := geometry.LoadBathymetryPassportFromFile(options.BathymetryMetadata)
	if err != nil {
		return seabed.FullBlackSeaQualityReport{}, err
	}
	if err := bathymetryPassport.VerifyDataset(bathymetryData); err != nil {
		return seabed.FullBlackSeaQualityReport{}, err
	}
	if warnings := bathymetryPassport.ReproducibilityWarnings(); len(warnings) > 0 {
		return seabed.FullBlackSeaQualityReport{}, fmt.Errorf("паспорт батиметрии не проходит QA-03: %s", strings.Join(warnings, "; "))
	}
	bathymetryGrid, err := geometry.LoadBathymetryFromJSON(bathymetryData, geometry.BathymetryLoadOptions{Resolution: bathymetryPassport.TargetResolutionDegrees})
	if err != nil {
		return seabed.FullBlackSeaQualityReport{}, err
	}
	if len(bathymetryGrid.Points) != bathymetryPassport.PointCount {
		return seabed.FullBlackSeaQualityReport{}, fmt.Errorf("паспорт GEBCO содержит %d точек, фактически загружено %d", bathymetryPassport.PointCount, len(bathymetryGrid.Points))
	}
	tracker.finishStage("bathymetry", "Паспорт и сетка GEBCO", stageStarted)

	stageStarted = time.Now()
	model, err := seabed.Build(flatMesh, seabed.RegularGridSampler{Grid: bathymetryGrid}, seabed.BuildConfig{
		MaxSourceDistanceM: options.MaxSourceDistanceM, CoastTransitionWidthM: options.CoastTransitionM,
		RegionThresholds: seabed.DefaultRegionThresholds(),
	})
	if err != nil {
		return seabed.FullBlackSeaQualityReport{}, fmt.Errorf("построение полной модели дна: %w", err)
	}
	tracker.finishStage("build", "Привязка глубины и BATHY-03", stageStarted)

	stageStarted = time.Now()
	qualityConfig := seabed.DefaultFullBlackSeaQualityConfig()
	qualityConfig.TargetEdgeM = options.TargetEdgeM
	qualityConfig.ExpectedBounds = geographicBoundsOf(polygon.Outer)
	qualityConfig.CoastlineReferenceAreaKM2 = domain.ReferenceAreaM2 / 1e6
	qualityConfig.LongFallbackWarningM = 2 * math.Sqrt2 * bathymetryPassport.TargetResolutionDegrees * 111_195
	if options.QualityConfig != nil {
		qualityConfig = *options.QualityConfig
	}
	report, err := seabed.EvaluateFullBlackSeaQuality(model, qualityConfig)
	if err != nil {
		return seabed.FullBlackSeaQualityReport{}, err
	}
	tracker.finishStage("quality", "Научные и топологические проверки", stageStarted)

	inputs, err := describeFullRunInputs(options, bathymetryPassport.SourceProduct, meshPassport.GmshVersion)
	if err != nil {
		return report, err
	}
	report.Inputs = inputs

	qualityJSONPath := filepath.Join(options.OutputDirectory, "full-quality.json")
	qualityTSVPath := filepath.Join(options.OutputDirectory, "full-quality.tsv")
	artifacts := seabed.FullBlackSeaArtifacts{
		Directory:          options.OutputDirectory,
		BathymetricMSH:     filepath.Join(options.OutputDirectory, "black-sea-depth.msh"),
		VTU:                filepath.Join(options.OutputDirectory, "black-sea-depth.vtu"),
		NodesCSV:           filepath.Join(options.OutputDirectory, "nodes.csv"),
		CellsCSV:           filepath.Join(options.OutputDirectory, "cells.csv"),
		ProfilesCSV:        filepath.Join(options.OutputDirectory, "profiles.csv"),
		ExportMetadataJSON: filepath.Join(options.OutputDirectory, "export-metadata.json"),
		ReconciliationJSON: filepath.Join(options.OutputDirectory, "reconciliation.json"),
		CorrectionsCSV:     filepath.Join(options.OutputDirectory, "reconciliation-corrections.csv"),
		OverviewSVG:        filepath.Join(options.OutputDirectory, "svg", "bathymetry-overview.svg"),
		MeshDetailsSVG:     filepath.Join(options.OutputDirectory, "svg", "mesh-details.svg"),
		Relief3DSVG:        filepath.Join(options.OutputDirectory, "svg", "seabed-3d.svg"),
		ProfilesSVG:        filepath.Join(options.OutputDirectory, "svg", "profiles.svg"),
		VisualizationJSON:  filepath.Join(options.OutputDirectory, "bathymetry-overview.json"),
		QualityJSON:        qualityJSONPath, QualityTSV: qualityTSVPath,
	}
	report.Artifacts = artifacts

	if model.Accepted {
		stageStarted = time.Now()
		profiles, profileSelections, profileErr := seabed.SelectCoastToDeepProfiles(model)
		if profileErr != nil {
			return report, profileErr
		}
		metadata := seabed.NewExportMetadata(domain.Projection, bathymetryPassport.VerticalReference, bathymetryPassport.VerticalReferenceCaveat)
		if err := seabed.WriteMSH2(artifacts.BathymetricMSH, model); err != nil {
			return report, err
		}
		if err := seabed.WriteCorrectionsCSV(artifacts.CorrectionsCSV, model.Reconciliation.Corrections); err != nil {
			return report, err
		}
		if err := seabed.WriteReconciliationJSON(artifacts.ReconciliationJSON, model.Reconciliation); err != nil {
			return report, err
		}
		bundle, err := seabed.WriteExportBundle(model, seabed.ExportBundleConfig{Directory: options.OutputDirectory, Metadata: metadata, Profiles: profiles})
		if err != nil {
			return report, err
		}
		artifacts.VTU, artifacts.NodesCSV, artifacts.CellsCSV = bundle.VTUPath, bundle.NodesCSVPath, bundle.CellsCSVPath
		artifacts.ProfilesCSV, artifacts.ExportMetadataJSON = bundle.ProfilesCSVPath, bundle.MetadataJSONPath
		report.Artifacts = artifacts
		tracker.finishStage("export", "MSH, VTU и таблицы", stageStarted)

		if !options.SkipVisualizations {
			stageStarted = time.Now()
			_, err = writeSeabedVisualizations(model, metadata, seabedVisualizationOptions{
				InputPath: artifacts.BathymetricMSH, MetadataPath: artifacts.ExportMetadataJSON,
				SourceMetadataPath: options.BathymetryMetadata, OutputDirectory: options.OutputDirectory,
				Source: bathymetryPassport.Attribution, SourceChecksum: bathymetryPassport.DatasetSHA256,
				IsobathsM: options.IsobathsM, VerticalScale: options.VerticalScale, ControlPoints: options.ControlPoints,
				Profiles: profiles, ProfileSelections: profileSelections,
			})
			if err != nil {
				return report, err
			}
			tracker.finishStage("render", "Карты, 3D и профили", stageStarted)
		}
	}

	report.Resources = tracker.stop(meshPassport.GmshVersion)
	if selectedRun.Metrics.CellCount != report.Topology.CellCount {
		report.Accepted = false
		report.Reasons = append(report.Reasons, fmt.Sprintf("mesh-report содержит %d ячеек, модель — %d", selectedRun.Metrics.CellCount, report.Topology.CellCount))
	}
	if err := seabed.WriteFullBlackSeaQualityJSON(qualityJSONPath, report); err != nil {
		return report, err
	}
	if err := seabed.WriteFullBlackSeaQualityTSV(qualityTSVPath, report); err != nil {
		return report, err
	}
	if !report.Accepted {
		return report, fmt.Errorf("QA-03 не пройдена: %s; отчёт: %s", strings.Join(report.Reasons, "; "), qualityJSONPath)
	}
	return report, nil
}

func validateFullRunOptions(options fullRunOptions) error {
	for flag, value := range map[string]string{
		"--mesh": options.MeshPath, "--mesh-report": options.MeshReportPath,
		"--coastline": options.CoastlinePath, "--bathymetry": options.BathymetryPath,
		"--bathymetry-metadata": options.BathymetryMetadata, "--output": options.OutputDirectory,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s не может быть пустым", flag)
		}
	}
	for flag, value := range map[string]float64{
		"--target-edge": options.TargetEdgeM, "--boundary-detail": options.BoundaryDetailM,
		"--max-source-distance": options.MaxSourceDistanceM, "--vertical-exaggeration": options.VerticalScale,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return fmt.Errorf("%s должно быть конечным положительным числом", flag)
		}
	}
	if math.IsNaN(options.CoastTransitionM) || math.IsInf(options.CoastTransitionM, 0) || options.CoastTransitionM < 0 {
		return fmt.Errorf("--coast-transition должно быть конечным неотрицательным числом")
	}
	if options.VerticalScale < 1 {
		return fmt.Errorf("--vertical-exaggeration должно быть не меньше 1")
	}
	if len(options.IsobathsM) == 0 && !options.SkipVisualizations {
		return fmt.Errorf("нужно задать хотя бы одну изобату")
	}
	return nil
}

func readFullMeshPassport(options fullRunOptions) (meshComparisonReport, meshRunReport, error) {
	data, err := os.ReadFile(options.MeshReportPath)
	if err != nil {
		return meshComparisonReport{}, meshRunReport{}, fmt.Errorf("чтение mesh-report %q: %w", options.MeshReportPath, err)
	}
	var report meshComparisonReport
	if err := json.Unmarshal(data, &report); err != nil {
		return meshComparisonReport{}, meshRunReport{}, fmt.Errorf("разбор mesh-report %q: %w", options.MeshReportPath, err)
	}
	if report.SchemaVersion != "lito-mesh-comparison/v1" || !strings.EqualFold(report.DatasetName, "Black Sea") {
		return meshComparisonReport{}, meshRunReport{}, fmt.Errorf("mesh-report не описывает полный контур Чёрного моря")
	}
	for _, level := range report.Levels {
		if math.Abs(level.TargetEdgeMeters-options.TargetEdgeM) > 1e-9 || math.Abs(level.BoundaryDetailMeters-options.BoundaryDetailM) > 1e-9 {
			continue
		}
		for _, run := range level.Results {
			if run.Algorithm != options.Generator {
				continue
			}
			if run.Error != "" || run.MeshFile == "" {
				return meshComparisonReport{}, meshRunReport{}, fmt.Errorf("контрольный запуск генератора %s завершился ошибкой: %s", options.Generator, run.Error)
			}
			same, err := sameFilesystemPath(run.MeshFile, options.MeshPath)
			if err != nil {
				return meshComparisonReport{}, meshRunReport{}, err
			}
			if !same {
				return meshComparisonReport{}, meshRunReport{}, fmt.Errorf("--mesh не совпадает с MSH выбранного запуска в mesh-report")
			}
			return report, run, nil
		}
	}
	return meshComparisonReport{}, meshRunReport{}, fmt.Errorf("mesh-report не содержит успешный запуск %s для %.0f/%.0f м", options.Generator, options.TargetEdgeM, options.BoundaryDetailM)
}

func sameFilesystemPath(left, right string) (bool, error) {
	leftAbs, err := filepath.Abs(filepath.Clean(left))
	if err != nil {
		return false, err
	}
	rightAbs, err := filepath.Abs(filepath.Clean(right))
	if err != nil {
		return false, err
	}
	return leftAbs == rightAbs, nil
}

func geographicBoundsOf(points []geometry.LatLon) seabed.GeographicBounds {
	bounds := seabed.GeographicBounds{
		MinLatitudeDeg: math.Inf(1), MaxLatitudeDeg: math.Inf(-1),
		MinLongitudeDeg: math.Inf(1), MaxLongitudeDeg: math.Inf(-1),
	}
	for _, point := range points {
		bounds.MinLatitudeDeg = math.Min(bounds.MinLatitudeDeg, point.Lat)
		bounds.MaxLatitudeDeg = math.Max(bounds.MaxLatitudeDeg, point.Lat)
		bounds.MinLongitudeDeg = math.Min(bounds.MinLongitudeDeg, point.Lon)
		bounds.MaxLongitudeDeg = math.Max(bounds.MaxLongitudeDeg, point.Lon)
	}
	return bounds
}

func describeFullRunInputs(options fullRunOptions, bathymetryVersion, gmshVersion string) ([]seabed.FullBlackSeaInput, error) {
	definitions := []struct {
		role, path, version string
	}{
		{"плоская сетка 1000 м", options.MeshPath, "Gmsh " + gmshVersion},
		{"паспорт генерации сетки", options.MeshReportPath, "lito-mesh-comparison/v1"},
		{"полный контур Чёрного моря", options.CoastlinePath, "MarineRegions MRGID 3319"},
		{"батиметрия", options.BathymetryPath, bathymetryVersion},
		{"паспорт батиметрии", options.BathymetryMetadata, bathymetryVersion},
	}
	inputs := make([]seabed.FullBlackSeaInput, 0, len(definitions))
	for _, definition := range definitions {
		checksum, err := fileSHA256(definition.path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(definition.path)
		if err != nil {
			return nil, fmt.Errorf("чтение размера входа %q: %w", definition.path, err)
		}
		inputs = append(inputs, seabed.FullBlackSeaInput{
			Role: definition.role, Path: definition.path, SHA256: checksum,
			SizeBytes: info.Size(), DataVersion: definition.version,
		})
	}
	return inputs, nil
}

func printFullBlackSeaSummary(report seabed.FullBlackSeaQualityReport) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Проверка\tЗначение\tСтатус")
	fmt.Fprintln(writer, "--------\t--------\t------")
	fmt.Fprintf(writer, "Полный охват\tΔ %.4f°\t%s\n", report.Extent.MaxDeviationDeg, russianPass(report.Extent.Accepted))
	fmt.Fprintf(writer, "Ячейки\t%d full-quad\t%s\n", report.Topology.CellCount, russianPass(report.Topology.Accepted))
	fmt.Fprintf(writer, "Дыры/разрывы\t%d\t%s\n", report.Topology.UnexpectedBoundaryEdgeCount+report.Depth.NoDataCellCount, russianPass(report.Topology.Accepted && report.Depth.NoDataCellCount == 0))
	fmt.Fprintf(writer, "Самопересечения\t%d\t%s\n", report.Topology.SelfIntersectingCellCount+report.Topology.BoundaryIntersectionCount, russianPass(report.Topology.SelfIntersectingCellCount+report.Topology.BoundaryIntersectionCount == 0))
	fmt.Fprintf(writer, "Положительные отметки\t%d\t%s\n", report.Depth.PositiveElevationCount, russianPass(report.Depth.PositiveElevationCount == 0))
	fmt.Fprintf(writer, "Среднее ребро\t%.1f м\t%s\n", report.EdgeSize.MeanEdgeM, russianPass(report.EdgeSize.Accepted))
	fmt.Fprintf(writer, "Площадь\t%.1f км²\t%s\n", report.Integrals.AreaKM2, russianPass(report.Integrals.CoastlineAreaAccepted))
	fmt.Fprintf(writer, "Объём\t%.1f км³\t%s\n", report.Integrals.VolumeKM3, russianPass(publishedComparisonsAcceptedCLI(report.PublishedComparisons)))
	fmt.Fprintf(writer, "QA-03\t%s\t%s\n", report.Artifacts.QualityJSON, russianPass(report.Accepted))
	fmt.Fprintf(writer, "Публикационная готовность\tнужен независимый QA-02\t%s\n", russianPass(report.PublicationReady))
	_ = writer.Flush()
	for _, warning := range report.Warnings {
		fmt.Printf("Предупреждение QA-03: %s\n", warning)
	}
}

func publishedComparisonsAcceptedCLI(comparisons []seabed.PublishedBasinComparison) bool {
	for _, comparison := range comparisons {
		if comparison.Reference.Primary {
			return comparison.Accepted
		}
	}
	return false
}

func russianPass(value bool) string {
	if value {
		return "пройдено"
	}
	return "не пройдено"
}

type fullRunResourceTracker struct {
	mu       sync.Mutex
	started  time.Time
	peakHeap uint64
	peakSys  uint64
	stages   []seabed.FullBlackSeaRunStage
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopped  bool
}

func newFullRunResourceTracker() *fullRunResourceTracker {
	tracker := &fullRunResourceTracker{started: time.Now(), stopCh: make(chan struct{}), doneCh: make(chan struct{})}
	tracker.sample()
	go func() {
		defer close(tracker.doneCh)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				tracker.sample()
			case <-tracker.stopCh:
				tracker.sample()
				return
			}
		}
	}()
	return tracker
}

func (tracker *fullRunResourceTracker) sample() (uint64, uint64) {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if memory.HeapInuse > tracker.peakHeap {
		tracker.peakHeap = memory.HeapInuse
	}
	if memory.Sys > tracker.peakSys {
		tracker.peakSys = memory.Sys
	}
	return memory.HeapInuse, memory.Sys
}

func (tracker *fullRunResourceTracker) finishStage(id, title string, started time.Time) {
	heap, system := tracker.sample()
	tracker.mu.Lock()
	tracker.stages = append(tracker.stages, seabed.FullBlackSeaRunStage{
		ID: id, Title: title, DurationSeconds: time.Since(started).Seconds(),
		HeapInUseBytes: heap, SystemBytes: system,
	})
	tracker.mu.Unlock()
}

func (tracker *fullRunResourceTracker) stop(gmshVersion string) seabed.FullBlackSeaResources {
	tracker.mu.Lock()
	if !tracker.stopped {
		tracker.stopped = true
		close(tracker.stopCh)
	}
	tracker.mu.Unlock()
	<-tracker.doneCh
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	finished := time.Now()
	return seabed.FullBlackSeaResources{
		StartedAt: tracker.started.UTC().Format(time.RFC3339), FinishedAt: finished.UTC().Format(time.RFC3339),
		DurationSeconds: finished.Sub(tracker.started).Seconds(), PeakHeapInUseBytes: tracker.peakHeap,
		PeakSystemBytes: tracker.peakSys, GoVersion: runtime.Version(), LitoRevision: currentLitoRevision(),
		GmshVersion: gmshVersion, Stages: append([]seabed.FullBlackSeaRunStage(nil), tracker.stages...),
	}
}

func (tracker *fullRunResourceTracker) closeIfRunning() {
	tracker.mu.Lock()
	if tracker.stopped {
		tracker.mu.Unlock()
		return
	}
	tracker.stopped = true
	close(tracker.stopCh)
	tracker.mu.Unlock()
	<-tracker.doneCh
}

func currentLitoRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	revision := ""
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if modified && revision != "" {
		return revision + "+modified"
	}
	return revision
}
