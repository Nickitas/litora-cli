package cobra

import (
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	adaptivemodel "coastal-geometry/internal/domain/adaptive"
	"coastal-geometry/internal/domain/coastline"
	mesh2d "coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"

	"github.com/spf13/cobra"
)

var (
	seabedAdaptiveGmshInput          string
	seabedAdaptiveGmshMetadata       string
	seabedAdaptiveGmshField          string
	seabedAdaptiveGmshFieldReport    string
	seabedAdaptiveGmshCoastline      string
	seabedAdaptiveGmshOutput         string
	seabedAdaptiveGmshBoundaryDetail float64
	seabedAdaptiveGmshGenerator      string
	seabedAdaptiveGmshPath           string
	seabedAdaptiveGmshMaxCells       int64
	seabedAdaptiveGmshAllowLarge     bool
	seabedAdaptiveGmshTimeout        time.Duration
)

var seabedAdaptiveGmshCmd = &cobra.Command{
	Use:   "generate-adaptive",
	Short: "Передать поле размера в Gmsh и построить full-quad сетку",
	Long: `Читает неизменённое поле ADAPT-01, создаёт скалярный Gmsh PostView и
использует его как единственный Background Field. До запуска команда оценивает
число ячеек, память и размер MSH. После Gmsh все исходные элементы согласованно
делятся на четырёхугольники, физические метки берега сохраняются, а полный MSH
проверяется на треугольники, вырожденность и несогласованные внутренние рёбра.`,
	RunE: runSeabedAdaptiveGmsh,
}

func init() {
	seabedCmd.AddCommand(seabedAdaptiveGmshCmd)
	seabedAdaptiveGmshCmd.Flags().StringVar(&seabedAdaptiveGmshInput, "input", "", "батиметрический MSH; по умолчанию output/seabed/black-sea-depth.msh")
	seabedAdaptiveGmshCmd.Flags().StringVar(&seabedAdaptiveGmshMetadata, "metadata", "", "паспорт EXPORT-02; по умолчанию рядом с входным MSH")
	seabedAdaptiveGmshCmd.Flags().StringVar(&seabedAdaptiveGmshField, "field", "", "CSV поля ADAPT-01; по умолчанию output/seabed/adaptive/size-field.csv")
	seabedAdaptiveGmshCmd.Flags().StringVar(&seabedAdaptiveGmshFieldReport, "field-report", "", "JSON поля ADAPT-01; по умолчанию output/seabed/adaptive/size-field.json")
	seabedAdaptiveGmshCmd.Flags().StringVar(&seabedAdaptiveGmshCoastline, "coastline", "data/cache/black-sea.geojson", "GeoJSON полного контура Чёрного моря")
	seabedAdaptiveGmshCmd.Flags().StringVar(&seabedAdaptiveGmshOutput, "output", "output", "корневой каталог результатов")
	seabedAdaptiveGmshCmd.Flags().Float64Var(&seabedAdaptiveGmshBoundaryDetail, "boundary-detail", 200, "детализация береговой линии, м")
	seabedAdaptiveGmshCmd.Flags().StringVar(&seabedAdaptiveGmshGenerator, "generator", "delaunay", "алгоритм Gmsh: delaunay, frontal-quad или parallelograms")
	seabedAdaptiveGmshCmd.Flags().StringVar(&seabedAdaptiveGmshPath, "gmsh", "", "путь к бинарному файлу Gmsh")
	seabedAdaptiveGmshCmd.Flags().Int64Var(&seabedAdaptiveGmshMaxCells, "max-cells", 5_000_000, "предельная оценка числа итоговых ячеек")
	seabedAdaptiveGmshCmd.Flags().BoolVar(&seabedAdaptiveGmshAllowLarge, "allow-large", false, "разрешить запуск сверх --max-cells после проверки ресурсов")
	seabedAdaptiveGmshCmd.Flags().DurationVar(&seabedAdaptiveGmshTimeout, "generator-timeout", 10*time.Minute, "лимит времени Gmsh")
}

func runSeabedAdaptiveGmsh(_ *cobra.Command, _ []string) error {
	paths := resolveAdaptiveGmshInputPaths()
	document, err := seabed.ReadMSH2(paths.inputMSH)
	if err != nil {
		return fmt.Errorf("чтение модели дна для ADAPT-02: %w", err)
	}
	if document.Metadata.ModelKind != seabed.MSHModelSeabed || document.Metadata.SchemaVersion != seabed.SeabedMSHSchemaVersion || !document.Model.Accepted {
		return fmt.Errorf("файл %q не является принятой батиметрической моделью %s", paths.inputMSH, seabed.SeabedMSHSchemaVersion)
	}
	metadata, err := seabed.ReadExportMetadataJSON(paths.metadata)
	if err != nil {
		return err
	}
	fieldReport, err := adaptivemodel.ReadReportJSON(paths.fieldReport)
	if err != nil {
		return err
	}
	field, err := adaptivemodel.ReadTargetSizeFieldCSV(paths.fieldCSV, document.Model)
	if err != nil {
		return err
	}
	if err := field.ValidateAgainstReport(fieldReport); err != nil {
		return fmt.Errorf("CSV и JSON ADAPT-01 не согласованы: %w", err)
	}

	polygon, err := coastline.LoadPolygon(coastline.LoadOptions{LocalPath: paths.coastline})
	if err != nil {
		return fmt.Errorf("загрузка полного контура Чёрного моря: %w", err)
	}
	domain, err := mesh2d.PrepareDomain(polygon.Outer, polygon.Holes, seabedAdaptiveGmshBoundaryDetail)
	if err != nil {
		return fmt.Errorf("подготовка берега для ADAPT-02: %w", err)
	}
	if math.Abs(domain.Projection.ReferenceLat-metadata.ProjectionReferenceLatitudeDeg) > 1e-6 ||
		math.Abs(domain.Projection.ReferenceLon-metadata.ProjectionReferenceLongitudeDeg) > 1e-6 {
		return fmt.Errorf("центр LAEA берега %.9f°, %.9f° не совпадает с моделью дна %.9f°, %.9f°",
			domain.Projection.ReferenceLat, domain.Projection.ReferenceLon,
			metadata.ProjectionReferenceLatitudeDeg, metadata.ProjectionReferenceLongitudeDeg)
	}
	algorithm, err := mesh2d.ParseAlgorithm(seabedAdaptiveGmshGenerator)
	if err != nil {
		return err
	}
	preflight, err := mesh2d.EstimateAdaptiveSize(document.Model.Mesh, field.TargetSizeM)
	if err != nil {
		return err
	}
	if seabedAdaptiveGmshMaxCells > 0 && preflight.EstimatedCellCount > seabedAdaptiveGmshMaxCells && !seabedAdaptiveGmshAllowLarge {
		return fmt.Errorf("адаптивная сетка оценивается в %d ячеек, %.1f МиБ памяти и %.1f МиБ MSH; это превышает --max-cells=%d; проверьте ресурсы и повторите с --allow-large",
			preflight.EstimatedCellCount, mebibytes(preflight.EstimatedMemoryBytes), mebibytes(preflight.EstimatedDiskBytes), seabedAdaptiveGmshMaxCells)
	}
	gmshPath, err := mesh2d.ResolveGmshPath(seabedAdaptiveGmshPath, seabedAdaptiveGmshOutput)
	if err != nil {
		return err
	}
	gmshVersion, err := mesh2d.GmshVersion(gmshPath)
	if err != nil {
		return err
	}

	gmshDir := filepath.Join(seabedAdaptiveGmshOutput, "seabed", "adaptive", "gmsh")
	backgroundPath := filepath.Join(gmshDir, "background-field.pos")
	geoPath := filepath.Join(gmshDir, "black-sea-adaptive.geo")
	meshPath := filepath.Join(gmshDir, "black-sea-adaptive.msh")
	logPath := filepath.Join(gmshDir, "gmsh.log")
	edgeTSVPath := filepath.Join(gmshDir, "edge-statistics.tsv")
	reportPath := filepath.Join(gmshDir, "generation-report.json")
	if !quiet {
		fmt.Printf("Предварительная оценка: %d ячеек; память %.1f МиБ; MSH %.1f МиБ\n",
			preflight.EstimatedCellCount, mebibytes(preflight.EstimatedMemoryBytes), mebibytes(preflight.EstimatedDiskBytes))
		fmt.Printf("Gmsh %s: %s\n", gmshVersion, gmshPath)
	}
	generated, resources, err := mesh2d.GenerateAdaptiveGmsh(domain, document.Model.Mesh, field.TargetSizeM, mesh2d.AdaptiveGenerationConfig{
		Algorithm: algorithm, BackgroundFieldPath: backgroundPath, GeoPath: geoPath,
		MeshPath: meshPath, LogPath: logPath, GmshPath: gmshPath, Timeout: seabedAdaptiveGmshTimeout,
		MinimumTargetSizeM: field.MinSizeM, MaximumTargetSizeM: field.MaxSizeM,
	})
	if err != nil {
		return err
	}
	topology := mesh2d.ValidateFullQuadMesh(generated)
	zoneNames := make(map[string]string, len(fieldReport.Summary.Zones))
	for _, zone := range fieldReport.Summary.Zones {
		zoneNames[zone.ID] = zone.Name
	}
	zones, err := mesh2d.EvaluateAdaptiveEdges(generated, document.Model.Mesh, field.TargetSizeM, field.Zones, zoneNames)
	if err != nil {
		return err
	}
	if err := adaptivemodel.WriteGmshEdgeTSV(edgeTSVPath, zones); err != nil {
		return err
	}
	inputChecksum, err := adaptiveFileSHA256(paths.inputMSH)
	if err != nil {
		return err
	}
	fieldChecksum, err := adaptiveFileSHA256(paths.fieldCSV)
	if err != nil {
		return err
	}
	metadataChecksum, err := adaptiveFileSHA256(paths.metadata)
	if err != nil {
		return err
	}
	fieldReportChecksum, err := adaptiveFileSHA256(paths.fieldReport)
	if err != nil {
		return err
	}
	coastlineChecksum, err := adaptiveFileSHA256(paths.coastline)
	if err != nil {
		return err
	}
	report := adaptivemodel.GmshReport{
		SchemaVersion: adaptivemodel.GmshReportSchemaVersion, GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		InputMSH: paths.inputMSH, InputMSHSHA256: inputChecksum,
		ExportMetadata: paths.metadata, ExportMetadataSHA256: metadataChecksum,
		SizeFieldCSV: paths.fieldCSV, SizeFieldCSVSHA256: fieldChecksum,
		SizeFieldReport: paths.fieldReport, SizeFieldReportSHA256: fieldReportChecksum,
		Coastline: paths.coastline, CoastlineSHA256: coastlineChecksum,
		BoundaryDetailMeters: seabedAdaptiveGmshBoundaryDetail, EffectiveBoundaryDetailMeters: domain.EffectiveBoundaryToleranceMeters,
		Projection:                        fmt.Sprintf("%s; центр %.9f°, %.9f°", domain.Projection.Description(), domain.Projection.ReferenceLat, domain.Projection.ReferenceLon),
		ProjectionRoundTripMaxErrorMeters: domain.ProjectionRoundTripMaxErrorMeters,
		Algorithm:                         algorithm, GmshPath: gmshPath, GmshVersion: gmshVersion,
		BackgroundField: adaptivemodel.BackgroundFieldReport{
			Format: "Gmsh parsed POS: scalar quadrangles SQ", FieldType: "PostView",
			Interpolation: "линейная интерполяция PostView; ближайшее значение вне опорной области",
			UseClosest:    true, SupportNodeCount: field.NodeCount, SupportCellCount: len(document.Model.Mesh.Cells),
			MinimumTargetSizeM: field.MinSizeM, MaximumTargetSizeM: field.MaxSizeM, PreSubdivisionScale: 2.5,
		},
		Preflight: preflight, MaximumCellCount: seabedAdaptiveGmshMaxCells, LargeRunAllowed: seabedAdaptiveGmshAllowLarge,
		Resources: resources, Topology: topology,
		EdgeSamplingMethod: "каждая сторона full-quad ячейки; цель и зона ближайшего узла поля ADAPT-01; квантили по гистограммам 5 м и 0,005",
		Zones:              zones,
		Artifacts:          adaptivemodel.GmshArtifacts{BackgroundPOS: backgroundPath, Geo: geoPath, MSH: meshPath, Log: logPath, EdgeTSV: edgeTSVPath},
		Accepted:           topology.Accepted && len(zones) > 0,
	}
	if !topology.Accepted {
		report.Reasons = append(report.Reasons, topology.Reasons...)
	}
	if len(zones) == 0 {
		report.Reasons = append(report.Reasons, "не рассчитана зональная статистика рёбер")
	}
	if err := adaptivemodel.WriteGmshReportJSON(reportPath, report); err != nil {
		return err
	}
	if !report.Accepted {
		return fmt.Errorf("адаптивная сетка не прошла проверку; отчёт: %s", reportPath)
	}
	if quiet {
		return nil
	}
	fmt.Printf("Адаптивная full-quad сетка: %s\n", meshPath)
	fmt.Printf("Background Field: %s\n", backgroundPath)
	fmt.Printf("Отчёт ADAPT-02: %s\n", reportPath)
	fmt.Printf("Ресурсы: Gmsh %.2f с; postprocess %.2f с; пик Gmsh %.1f МиБ; heap Lito %.1f МиБ\n",
		resources.GmshDurationSeconds, resources.PostprocessDurationSeconds,
		mebibytes(resources.PeakRSSBytes), float64(resources.LitoHeapInUseBytes)/(1024*1024))
	fmt.Printf("Топология: %d ячеек; треугольников %d; несогласованных внутренних рёбер %d\n",
		topology.CellCount, topology.TriangleCount, topology.UnmatchedInteriorEdgeCount)
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Зона\tРёбер\tЦель, м\tФакт, м\tФакт/цель\tВ допуске, %")
	fmt.Fprintln(writer, "----\t-----\t-------\t-------\t----------\t------------")
	for _, zone := range zones {
		fmt.Fprintf(writer, "%s\t%d\t%.0f–%.0f\t%.0f\t%.3f\t%.1f\n", zone.Name, zone.EdgeObservationCount,
			zone.TargetMinM, zone.TargetMaxM, zone.ActualMeanM, zone.RatioMean, zone.WithinTolerancePct)
	}
	return writer.Flush()
}

type adaptiveGmshPaths struct {
	inputMSH, metadata, fieldCSV, fieldReport, coastline string
}

func resolveAdaptiveGmshInputPaths() adaptiveGmshPaths {
	input := strings.TrimSpace(seabedAdaptiveGmshInput)
	if input == "" {
		input = filepath.Join(seabedAdaptiveGmshOutput, "seabed", "black-sea-depth.msh")
	}
	metadata := strings.TrimSpace(seabedAdaptiveGmshMetadata)
	if metadata == "" {
		metadata = filepath.Join(filepath.Dir(input), "export-metadata.json")
	}
	field := strings.TrimSpace(seabedAdaptiveGmshField)
	if field == "" {
		field = filepath.Join(seabedAdaptiveGmshOutput, "seabed", "adaptive", "size-field.csv")
	}
	fieldReport := strings.TrimSpace(seabedAdaptiveGmshFieldReport)
	if fieldReport == "" {
		fieldReport = filepath.Join(seabedAdaptiveGmshOutput, "seabed", "adaptive", "size-field.json")
	}
	return adaptiveGmshPaths{inputMSH: input, metadata: metadata, fieldCSV: field, fieldReport: fieldReport, coastline: strings.TrimSpace(seabedAdaptiveGmshCoastline)}
}

func adaptiveFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("открытие %q для SHA-256: %w", path, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("вычисление SHA-256 %q: %w", path, err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func mebibytes(bytes int64) float64 {
	return float64(bytes) / (1024 * 1024)
}
