package cobra

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"
	mesh2d "coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"

	"github.com/spf13/cobra"
)

const (
	defaultSeabedBuildMesh        = "output/mesh/msh/black-sea-edge-1000-detail-1000-frontal-quad.msh"
	defaultSeabedBuildBathymetry  = "output/export-01-verification/gebco2026-0.02deg.json"
	defaultSeabedBuildMetadata    = "output/export-01-verification/gebco2026-0.02deg.metadata.json"
	defaultSeabedBuildCoastline   = "data/cache/black-sea.geojson"
	defaultSeabedBuildOutput      = "output/seabed"
	defaultSeabedBuildMaxElements = 5_000_000
	defaultSeabedBuildMaxOutputMB = 2_048
)

var (
	seabedBuildMesh              string
	seabedBuildBathymetry        string
	seabedBuildBathymetryMeta    string
	seabedBuildCoastline         string
	seabedBuildOutput            string
	seabedBuildBoundaryDetail    float64
	seabedBuildMaxSourceDistance float64
	seabedBuildCoastTransition   float64
	seabedBuildRecoverWGS84      bool
	seabedBuildMaxNodes          int
	seabedBuildMaxCells          int
	seabedBuildMaxOutputMiB      int
	seabedBuildAllowLarge        bool
)

var seabedBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Назначить глубины плоской сетке и создать модель дна",
	Long: `Читает плоскую full-quad MSH Чёрного моря и проверенный производный
набор GEBCO с паспортом. Команда назначает отметки, согласует береговой ноль,
рассчитывает характеристики ячеек BATHY-03 и создаёт MSH/VTU/CSV/JSON.

Все артефакты сохраняются только внутри output/seabed/. До расчёта команда
проверяет число узлов, ячеек и ожидаемый размер экспортов. Старый плоский MSH
без блоков WGS 84 может быть явно восстановлен из исходного контура и той же
сферической LAEA-проекции; это фиксируется в build-report.json и build.log.

Команда не рисует SVG и не изменяет входной MSH: для визуализации используйте
«lito seabed render», а для независимой проверки рельефа — «lito seabed validate».`,
	RunE: runSeabedBuild,
}

func init() {
	seabedCmd.AddCommand(seabedBuildCmd)
	seabedBuildCmd.Flags().StringVar(&seabedBuildMesh, "mesh", defaultSeabedBuildMesh, "плоская full-quad MSH Чёрного моря")
	seabedBuildCmd.Flags().StringVar(&seabedBuildBathymetry, "bathymetry", defaultSeabedBuildBathymetry, "производный JSON батиметрии GEBCO")
	seabedBuildCmd.Flags().StringVar(&seabedBuildBathymetryMeta, "bathymetry-metadata", defaultSeabedBuildMetadata, "паспорт производного набора GEBCO")
	seabedBuildCmd.Flags().StringVar(&seabedBuildCoastline, "coastline", defaultSeabedBuildCoastline, "GeoJSON полного контура Чёрного моря для LAEA")
	seabedBuildCmd.Flags().StringVar(&seabedBuildOutput, "output", defaultSeabedBuildOutput, "каталог результата внутри output/")
	seabedBuildCmd.Flags().Float64Var(&seabedBuildBoundaryDetail, "boundary-detail", 1000, "детализация контура только для восстановления и проверки LAEA, м")
	seabedBuildCmd.Flags().Float64Var(&seabedBuildMaxSourceDistance, "max-source-distance", 50000, "явный предел ближайшей замены GEBCO, м")
	seabedBuildCmd.Flags().Float64Var(&seabedBuildCoastTransition, "coast-transition", 0, "ширина плавного перехода к береговому нулю, м; 0 означает два средних ребра")
	seabedBuildCmd.Flags().BoolVar(&seabedBuildRecoverWGS84, "recover-wgs84", true, "восстановить WGS 84 у старого плоского MSH из --coastline")
	seabedBuildCmd.Flags().IntVar(&seabedBuildMaxNodes, "max-nodes", defaultSeabedBuildMaxElements, "предельное число узлов до запуска сборки")
	seabedBuildCmd.Flags().IntVar(&seabedBuildMaxCells, "max-cells", defaultSeabedBuildMaxElements, "предельное число ячеек до запуска сборки")
	seabedBuildCmd.Flags().IntVar(&seabedBuildMaxOutputMiB, "max-output-mib", defaultSeabedBuildMaxOutputMB, "предельная оценка размера экспортов, МиБ")
	seabedBuildCmd.Flags().BoolVar(&seabedBuildAllowLarge, "allow-large", false, "разрешить сборку сверх защитных лимитов после проверки ресурсов")
}

// seabedBuildOptions объединяет параметры одного воспроизводимого запуска
// CLI-01. Его используют команда и интеграционные тесты без глобальных флагов.
type seabedBuildOptions struct {
	MeshPath           string
	BathymetryPath     string
	BathymetryMetadata string
	CoastlinePath      string
	OutputDirectory    string
	BoundaryDetailM    float64
	MaxSourceDistanceM float64
	CoastTransitionM   float64
	RecoverWGS84       bool
	MaxNodes           int
	MaxCells           int
	MaxOutputMiB       int
	AllowLarge         bool
}

// seabedBuildInput фиксирует вход сборки и его контрольную сумму.
type seabedBuildInput struct {
	Role      string `json:"role"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
}

// seabedBuildPreflight содержит проверяемую оценку ресурсов до сборки.
type seabedBuildPreflight struct {
	NodeCount            int   `json:"node_count"`
	CellCount            int   `json:"cell_count"`
	TriangleCount        int   `json:"triangle_count"`
	EstimatedOutputBytes int64 `json:"estimated_output_bytes"`
	MaxNodes             int   `json:"max_nodes"`
	MaxCells             int   `json:"max_cells"`
	MaxOutputMiB         int   `json:"max_output_mib"`
	LargeRunAllowed      bool  `json:"large_run_allowed"`
}

// seabedBuildArtifacts перечисляет все результаты сборки модели дна.
type seabedBuildArtifacts struct {
	Directory          string `json:"directory"`
	BathymetricMSH     string `json:"bathymetric_msh"`
	VTU                string `json:"vtu"`
	NodesCSV           string `json:"nodes_csv"`
	CellsCSV           string `json:"cells_csv"`
	ProfilesCSV        string `json:"profiles_csv"`
	ExportMetadataJSON string `json:"export_metadata_json"`
	ReconciliationJSON string `json:"reconciliation_json"`
	CorrectionsCSV     string `json:"reconciliation_corrections_csv"`
	CellDerivationJSON string `json:"cell_derivation_json"`
	BuildReportJSON    string `json:"build_report_json"`
	BuildLog           string `json:"build_log"`
}

// seabedBuildResources сохраняет наблюдаемую длительность и память Lito.
type seabedBuildResources struct {
	DurationSeconds        float64 `json:"duration_seconds"`
	HeapInUseAfterBuild    uint64  `json:"heap_in_use_after_build_bytes"`
	SystemMemoryAfterBuild uint64  `json:"system_memory_after_build_bytes"`
}

// seabedBuildReport — человекочитаемый и машинный журнал команды seabed build.
type seabedBuildReport struct {
	SchemaVersion        string                        `json:"schema_version"`
	GeneratedAt          string                        `json:"generated_at"`
	Inputs               []seabedBuildInput            `json:"inputs"`
	Projection           string                        `json:"projection"`
	CoordinatesRecovered bool                          `json:"coordinates_recovered_from_laea"`
	Preflight            seabedBuildPreflight          `json:"preflight"`
	Sampling             seabed.SamplingSummary        `json:"sampling"`
	Reconciliation       seabed.ReconciliationSummary  `json:"reconciliation"`
	CellDerivation       seabed.CellDerivationMetadata `json:"cell_derivation"`
	Resources            seabedBuildResources          `json:"resources"`
	Artifacts            seabedBuildArtifacts          `json:"artifacts"`
	Accepted             bool                          `json:"accepted"`
	Reasons              []string                      `json:"reasons"`
}

func runSeabedBuild(_ *cobra.Command, _ []string) error {
	options := seabedBuildOptions{
		MeshPath: seabedBuildMesh, BathymetryPath: seabedBuildBathymetry,
		BathymetryMetadata: seabedBuildBathymetryMeta, CoastlinePath: seabedBuildCoastline,
		OutputDirectory: seabedBuildOutput, BoundaryDetailM: seabedBuildBoundaryDetail,
		MaxSourceDistanceM: seabedBuildMaxSourceDistance, CoastTransitionM: seabedBuildCoastTransition,
		RecoverWGS84: seabedBuildRecoverWGS84, MaxNodes: seabedBuildMaxNodes,
		MaxCells: seabedBuildMaxCells, MaxOutputMiB: seabedBuildMaxOutputMiB,
		AllowLarge: seabedBuildAllowLarge,
	}
	if err := validateSeabedBuildOutput(options.OutputDirectory); err != nil {
		return err
	}
	report, err := executeSeabedBuild(options)
	if report.SchemaVersion != "" && !quiet {
		printSeabedBuildSummary(report)
	}
	return err
}

func executeSeabedBuild(options seabedBuildOptions) (seabedBuildReport, error) {
	if err := validateSeabedBuildOptions(options); err != nil {
		return seabedBuildReport{}, err
	}
	startedAt := time.Now()

	flatDocument, err := seabed.ReadMSH2(options.MeshPath)
	if err != nil {
		return seabedBuildReport{}, fmt.Errorf("чтение плоской MSH: %w", err)
	}
	if flatDocument.Metadata.ModelKind != seabed.MSHModelFlat {
		return seabedBuildReport{}, fmt.Errorf("файл %q уже является батиметрической моделью; для повторной визуализации используйте seabed render", options.MeshPath)
	}
	preflight := newSeabedBuildPreflight(flatDocument.Model.Mesh, options)
	if err := validateSeabedBuildPreflight(preflight); err != nil {
		return seabedBuildReport{}, err
	}

	polygon, err := coastline.LoadPolygon(coastline.LoadOptions{LocalPath: options.CoastlinePath})
	if err != nil {
		return seabedBuildReport{}, fmt.Errorf("загрузка полного контура Чёрного моря: %w", err)
	}
	domain, err := mesh2d.PrepareDomain(polygon.Outer, polygon.Holes, options.BoundaryDetailM)
	if err != nil {
		return seabedBuildReport{}, fmt.Errorf("подготовка LAEA для MSH: %w", err)
	}
	mesh, recovered, err := prepareSeabedBuildMesh(flatDocument.Model.Mesh, domain.Projection, options.RecoverWGS84)
	if err != nil {
		return seabedBuildReport{}, err
	}

	bathymetryData, err := os.ReadFile(options.BathymetryPath)
	if err != nil {
		return seabedBuildReport{}, fmt.Errorf("чтение батиметрии %q: %w", options.BathymetryPath, err)
	}
	passport, err := geometry.LoadBathymetryPassportFromFile(options.BathymetryMetadata)
	if err != nil {
		return seabedBuildReport{}, err
	}
	if err := passport.VerifyDataset(bathymetryData); err != nil {
		return seabedBuildReport{}, err
	}
	if warnings := passport.ReproducibilityWarnings(); len(warnings) > 0 {
		return seabedBuildReport{}, fmt.Errorf("паспорт батиметрии не воспроизводим: %s", strings.Join(warnings, "; "))
	}
	grid, err := geometry.LoadBathymetryFromJSON(bathymetryData, geometry.BathymetryLoadOptions{Resolution: passport.TargetResolutionDegrees})
	if err != nil {
		return seabedBuildReport{}, err
	}
	if len(grid.Points) != passport.PointCount {
		return seabedBuildReport{}, fmt.Errorf("паспорт батиметрии содержит %d точек, фактически загружено %d", passport.PointCount, len(grid.Points))
	}

	model, err := seabed.Build(mesh, seabed.RegularGridSampler{Grid: grid}, seabed.BuildConfig{
		MaxSourceDistanceM: options.MaxSourceDistanceM, CoastTransitionWidthM: options.CoastTransitionM,
		RegionThresholds: seabed.DefaultRegionThresholds(),
	})
	if err != nil {
		return seabedBuildReport{}, fmt.Errorf("построение модели дна: %w", err)
	}

	artifacts := seabedBuildArtifacts{
		Directory:          options.OutputDirectory,
		BathymetricMSH:     filepath.Join(options.OutputDirectory, "black-sea-depth.msh"),
		VTU:                filepath.Join(options.OutputDirectory, "black-sea-depth.vtu"),
		NodesCSV:           filepath.Join(options.OutputDirectory, "nodes.csv"),
		CellsCSV:           filepath.Join(options.OutputDirectory, "cells.csv"),
		ProfilesCSV:        filepath.Join(options.OutputDirectory, "profiles.csv"),
		ExportMetadataJSON: filepath.Join(options.OutputDirectory, "export-metadata.json"),
		ReconciliationJSON: filepath.Join(options.OutputDirectory, "reconciliation.json"),
		CorrectionsCSV:     filepath.Join(options.OutputDirectory, "reconciliation-corrections.csv"),
		CellDerivationJSON: filepath.Join(options.OutputDirectory, "cell-derivation.json"),
		BuildReportJSON:    filepath.Join(options.OutputDirectory, "build-report.json"),
		BuildLog:           filepath.Join(options.OutputDirectory, "build.log"),
	}
	inputs, err := describeSeabedBuildInputs(options)
	if err != nil {
		return seabedBuildReport{}, err
	}
	report := seabedBuildReport{
		SchemaVersion: "lito-seabed-build/v1", GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Inputs: inputs, Projection: domain.Projection.Description(), CoordinatesRecovered: recovered,
		Preflight: preflight, Sampling: model.Sampling, Reconciliation: model.Reconciliation,
		CellDerivation: model.CellDerivation, Artifacts: artifacts,
		Accepted: model.Accepted, Reasons: append([]string(nil), model.Reasons...),
	}

	if model.Accepted {
		profiles, _, err := seabed.SelectCoastToDeepProfiles(model)
		if err != nil {
			return report, err
		}
		metadata := seabed.NewExportMetadata(domain.Projection, passport.VerticalReference, passport.VerticalReferenceCaveat)
		if err := seabed.WriteMSH2(artifacts.BathymetricMSH, model); err != nil {
			return report, err
		}
		if err := seabed.WriteReconciliationJSON(artifacts.ReconciliationJSON, model.Reconciliation); err != nil {
			return report, err
		}
		if err := seabed.WriteCorrectionsCSV(artifacts.CorrectionsCSV, model.Reconciliation.Corrections); err != nil {
			return report, err
		}
		if err := seabed.WriteCellDerivationJSON(artifacts.CellDerivationJSON, model.CellDerivation); err != nil {
			return report, err
		}
		bundle, err := seabed.WriteExportBundle(model, seabed.ExportBundleConfig{Directory: options.OutputDirectory, Metadata: metadata, Profiles: profiles})
		if err != nil {
			return report, err
		}
		report.Artifacts.VTU = bundle.VTUPath
		report.Artifacts.NodesCSV = bundle.NodesCSVPath
		report.Artifacts.CellsCSV = bundle.CellsCSVPath
		report.Artifacts.ProfilesCSV = bundle.ProfilesCSVPath
		report.Artifacts.ExportMetadataJSON = bundle.MetadataJSONPath
	}
	report.Resources.DurationSeconds = time.Since(startedAt).Seconds()
	runtime.GC()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	report.Resources.HeapInUseAfterBuild = memory.HeapInuse
	report.Resources.SystemMemoryAfterBuild = memory.Sys
	if err := writeSeabedBuildReport(report.Artifacts.BuildReportJSON, report); err != nil {
		return report, err
	}
	if err := writeSeabedBuildLog(report.Artifacts.BuildLog, report); err != nil {
		return report, err
	}
	if !report.Accepted {
		return report, fmt.Errorf("модель дна не принята: %s; отчёт: %s", strings.Join(report.Reasons, "; "), report.Artifacts.BuildReportJSON)
	}
	return report, nil
}

func validateSeabedBuildOptions(options seabedBuildOptions) error {
	for flag, value := range map[string]string{
		"--mesh": options.MeshPath, "--bathymetry": options.BathymetryPath,
		"--bathymetry-metadata": options.BathymetryMetadata, "--coastline": options.CoastlinePath,
		"--output": options.OutputDirectory,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s не может быть пустым", flag)
		}
	}
	for flag, value := range map[string]float64{"--boundary-detail": options.BoundaryDetailM} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return fmt.Errorf("%s должно быть конечным положительным числом", flag)
		}
	}
	if math.IsNaN(options.MaxSourceDistanceM) || math.IsInf(options.MaxSourceDistanceM, 0) || options.MaxSourceDistanceM < 0 {
		return fmt.Errorf("--max-source-distance должно быть конечным неотрицательным числом")
	}
	if math.IsNaN(options.CoastTransitionM) || math.IsInf(options.CoastTransitionM, 0) || options.CoastTransitionM < 0 {
		return fmt.Errorf("--coast-transition должно быть конечным неотрицательным числом")
	}
	for flag, value := range map[string]int{
		"--max-nodes": options.MaxNodes, "--max-cells": options.MaxCells, "--max-output-mib": options.MaxOutputMiB,
	} {
		if value <= 0 {
			return fmt.Errorf("%s должно быть положительным целым числом", flag)
		}
	}
	return nil
}

func validateSeabedBuildOutput(directory string) error {
	root, err := filepath.Abs("output")
	if err != nil {
		return err
	}
	target, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("--output должен находиться внутри каталога output/, получен %q", directory)
	}
	return nil
}

func newSeabedBuildPreflight(mesh mesh2d.Mesh, options seabedBuildOptions) seabedBuildPreflight {
	return seabedBuildPreflight{
		NodeCount: len(mesh.Nodes) - 1, CellCount: len(mesh.Cells), TriangleCount: mesh.TriangleCount,
		EstimatedOutputBytes: estimateSeabedBuildOutputBytes(len(mesh.Nodes)-1, len(mesh.Cells)),
		MaxNodes:             options.MaxNodes, MaxCells: options.MaxCells, MaxOutputMiB: options.MaxOutputMiB,
		LargeRunAllowed: options.AllowLarge,
	}
}

func validateSeabedBuildPreflight(preflight seabedBuildPreflight) error {
	if preflight.NodeCount <= 0 || preflight.CellCount <= 0 {
		return fmt.Errorf("плоская MSH не содержит узлов или ячеек")
	}
	if preflight.LargeRunAllowed {
		return nil
	}
	if preflight.NodeCount > preflight.MaxNodes {
		return fmt.Errorf("MSH содержит %d узлов, что превышает --max-nodes=%d; проверьте ресурсы и повторите с --allow-large", preflight.NodeCount, preflight.MaxNodes)
	}
	if preflight.CellCount > preflight.MaxCells {
		return fmt.Errorf("MSH содержит %d ячеек, что превышает --max-cells=%d; проверьте ресурсы и повторите с --allow-large", preflight.CellCount, preflight.MaxCells)
	}
	limit := int64(preflight.MaxOutputMiB) * 1024 * 1024
	if preflight.EstimatedOutputBytes > limit {
		return fmt.Errorf("экспорт оценивается в %.1f МиБ, что превышает --max-output-mib=%d; проверьте место на диске и повторите с --allow-large",
			float64(preflight.EstimatedOutputBytes)/(1024*1024), preflight.MaxOutputMiB)
	}
	return nil
}

func estimateSeabedBuildOutputBytes(nodeCount, cellCount int) int64 {
	const baseBytes = 128 * 1024 * 1024
	return baseBytes + int64(nodeCount)*900 + int64(cellCount)*1050
}

func prepareSeabedBuildMesh(source mesh2d.Mesh, projection mesh2d.EqualAreaProjection, recoverWGS84 bool) (mesh2d.Mesh, bool, error) {
	if len(source.Nodes) <= 1 {
		return mesh2d.Mesh{}, false, fmt.Errorf("плоская MSH не содержит фактических узлов")
	}
	mesh := source
	if !mesh.Nodes[1].GeographicCoordinatesSet {
		if !recoverWGS84 {
			return mesh2d.Mesh{}, false, fmt.Errorf("плоская MSH не содержит WGS 84; повторите с --recover-wgs84 и исходным --coastline")
		}
		if err := projection.AssignGeographicCoordinates(mesh.Nodes); err != nil {
			return mesh2d.Mesh{}, false, fmt.Errorf("восстановление WGS 84 из LAEA: %w", err)
		}
		return mesh, true, nil
	}

	const maxProjectionDeviationM = 0.25
	for nodeID := 1; nodeID < len(mesh.Nodes); nodeID++ {
		node := mesh.Nodes[nodeID]
		if !node.GeographicCoordinatesSet {
			return mesh2d.Mesh{}, false, fmt.Errorf("координаты WGS 84 заданы не для всех узлов")
		}
		expected := projection.Project(geometry.LatLon{Lat: node.LatitudeDeg, Lon: node.LongitudeDeg})
		if distance := math.Hypot(expected.X-node.X, expected.Y-node.Y); distance > maxProjectionDeviationM {
			return mesh2d.Mesh{}, false, fmt.Errorf("узел %d не согласован с LAEA исходного контура: %.3f м > %.2f м", nodeID, distance, maxProjectionDeviationM)
		}
	}
	return mesh, false, nil
}

func describeSeabedBuildInputs(options seabedBuildOptions) ([]seabedBuildInput, error) {
	definitions := []struct{ role, path string }{
		{"плоская сетка", options.MeshPath}, {"батиметрия", options.BathymetryPath},
		{"паспорт батиметрии", options.BathymetryMetadata}, {"контур Чёрного моря", options.CoastlinePath},
	}
	inputs := make([]seabedBuildInput, 0, len(definitions))
	for _, definition := range definitions {
		checksum, err := fileSHA256(definition.path)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(definition.path)
		if err != nil {
			return nil, fmt.Errorf("чтение размера входа %q: %w", definition.path, err)
		}
		inputs = append(inputs, seabedBuildInput{Role: definition.role, Path: definition.path, SHA256: checksum, SizeBytes: info.Size()})
	}
	return inputs, nil
}

func writeSeabedBuildReport(path string, report seabedBuildReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта seabed build: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога отчёта seabed build: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта seabed build: %w", err)
	}
	return nil
}

func writeSeabedBuildLog(path string, report seabedBuildReport) error {
	lines := []string{
		"Команда: lito seabed build",
		"Схема: " + report.SchemaVersion,
		"Создано (UTC): " + report.GeneratedAt,
		fmt.Sprintf("Принято: %t", report.Accepted),
		fmt.Sprintf("WGS 84 восстановлен из LAEA: %t", report.CoordinatesRecovered),
		fmt.Sprintf("Узлов: %d; ячеек: %d; треугольников: %d", report.Preflight.NodeCount, report.Preflight.CellCount, report.Preflight.TriangleCount),
		fmt.Sprintf("Оценка экспортов: %.1f МиБ", float64(report.Preflight.EstimatedOutputBytes)/(1024*1024)),
		fmt.Sprintf("Длительность: %.3f с", report.Resources.DurationSeconds),
		fmt.Sprintf("HeapInuse после сборки: %d байт", report.Resources.HeapInUseAfterBuild),
		fmt.Sprintf("Sys после сборки: %d байт", report.Resources.SystemMemoryAfterBuild),
		"Отчёт: " + report.Artifacts.BuildReportJSON,
	}
	if len(report.Reasons) > 0 {
		lines = append(lines, "Причины отказа: "+strings.Join(report.Reasons, "; "))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога журнала seabed build: %w", err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("сохранение журнала seabed build: %w", err)
	}
	return nil
}

func printSeabedBuildSummary(report seabedBuildReport) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Показатель\tЗначение\tСтатус")
	fmt.Fprintln(writer, "----------\t--------\t------")
	fmt.Fprintf(writer, "Узлы\t%d\t%s\n", report.Preflight.NodeCount, russianPass(report.Preflight.NodeCount <= report.Preflight.MaxNodes || report.Preflight.LargeRunAllowed))
	fmt.Fprintf(writer, "Ячейки\t%d\t%s\n", report.Preflight.CellCount, russianPass(report.Preflight.CellCount <= report.Preflight.MaxCells || report.Preflight.LargeRunAllowed))
	fmt.Fprintf(writer, "Покрытие узлов\t%.2f %%\t%s\n", report.Sampling.CoveragePercent, russianPass(report.Sampling.NoDataNodeCount == 0))
	fmt.Fprintf(writer, "Покрытие ячеек\t%.2f %%\t%s\n", report.CellDerivation.Summary.CoveragePercent, russianPass(report.CellDerivation.Summary.NoDataCellCount == 0))
	fmt.Fprintf(writer, "WGS 84 из LAEA\t%t\t%s\n", report.CoordinatesRecovered, russianPass(true))
	fmt.Fprintf(writer, "MSH/VTU/CSV\t%s\t%s\n", report.Artifacts.Directory, russianPass(report.Accepted))
	fmt.Fprintf(writer, "Журнал\t%s\t%s\n", report.Artifacts.BuildLog, russianPass(report.Accepted))
	_ = writer.Flush()
}
