package cobra

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
	seabedAdaptiveCompareInput          string
	seabedAdaptiveCompareMetadata       string
	seabedAdaptiveCompareField          string
	seabedAdaptiveCompareFieldReport    string
	seabedAdaptiveCompareCoastline      string
	seabedAdaptiveCompareOutput         string
	seabedAdaptiveCompareBoundaryDetail float64
	seabedAdaptiveCompareGenerators     string
	seabedAdaptiveCompareLevels         string
	seabedAdaptiveCompareGmshPath       string
	seabedAdaptiveCompareMaxCells       int64
	seabedAdaptiveCompareAllowLarge     bool
	seabedAdaptiveCompareTimeout        time.Duration
)

var seabedAdaptiveCompareCmd = &cobra.Command{
	Use:   "compare-adaptive",
	Short: "Сравнить генераторы адаптивной full-quad сетки",
	Long: `Запускает Delaunay, Frontal-Delaunay for Quads и упаковку
параллелограммов на одинаковых полях размера и одной границе Чёрного моря.
Для каждого контрольного диапазона оцениваются сохранение берега и батиметрии,
геометрия ячеек, соответствие целевому размеру, время и память. Тайм-аут или
ошибка одного генератора сохраняются в отчёте и не останавливают остальные.`,
	RunE: runSeabedAdaptiveCompare,
}

func init() {
	seabedCmd.AddCommand(seabedAdaptiveCompareCmd)
	seabedAdaptiveCompareCmd.Flags().StringVar(&seabedAdaptiveCompareInput, "input", "", "батиметрический MSH; по умолчанию output/seabed/black-sea-depth.msh")
	seabedAdaptiveCompareCmd.Flags().StringVar(&seabedAdaptiveCompareMetadata, "metadata", "", "паспорт EXPORT-02; по умолчанию рядом с входным MSH")
	seabedAdaptiveCompareCmd.Flags().StringVar(&seabedAdaptiveCompareField, "field", "", "CSV поля ADAPT-01; по умолчанию output/seabed/adaptive/size-field.csv")
	seabedAdaptiveCompareCmd.Flags().StringVar(&seabedAdaptiveCompareFieldReport, "field-report", "", "JSON поля ADAPT-01; по умолчанию output/seabed/adaptive/size-field.json")
	seabedAdaptiveCompareCmd.Flags().StringVar(&seabedAdaptiveCompareCoastline, "coastline", "data/cache/black-sea.geojson", "GeoJSON полного контура Чёрного моря")
	seabedAdaptiveCompareCmd.Flags().StringVar(&seabedAdaptiveCompareOutput, "output", "output", "корневой каталог результатов")
	seabedAdaptiveCompareCmd.Flags().Float64Var(&seabedAdaptiveCompareBoundaryDetail, "boundary-detail", 200, "детализация береговой линии, м")
	seabedAdaptiveCompareCmd.Flags().StringVar(&seabedAdaptiveCompareGenerators, "generators", "delaunay,frontal-quad,parallelograms", "алгоритмы Gmsh через запятую")
	seabedAdaptiveCompareCmd.Flags().StringVar(&seabedAdaptiveCompareLevels, "levels", "detailed:200:1000,coarse:500:1000", "уровни id:min:max через запятую")
	seabedAdaptiveCompareCmd.Flags().StringVar(&seabedAdaptiveCompareGmshPath, "gmsh", "", "путь к бинарному файлу Gmsh")
	seabedAdaptiveCompareCmd.Flags().Int64Var(&seabedAdaptiveCompareMaxCells, "max-cells", 5_000_000, "предельная оценка числа итоговых ячеек одного запуска")
	seabedAdaptiveCompareCmd.Flags().BoolVar(&seabedAdaptiveCompareAllowLarge, "allow-large", false, "разрешить уровень сверх --max-cells после проверки ресурсов")
	seabedAdaptiveCompareCmd.Flags().DurationVar(&seabedAdaptiveCompareTimeout, "generator-timeout", 10*time.Minute, "лимит времени одного запуска Gmsh")
}

func runSeabedAdaptiveCompare(_ *cobra.Command, _ []string) error {
	levels, err := parseAdaptiveComparisonLevels(seabedAdaptiveCompareLevels)
	if err != nil {
		return err
	}
	algorithms, err := parseMeshAlgorithms(seabedAdaptiveCompareGenerators)
	if err != nil {
		return err
	}
	paths := resolveAdaptiveCompareInputPaths()
	document, err := seabed.ReadMSH2(paths.inputMSH)
	if err != nil {
		return fmt.Errorf("чтение модели дна для ADAPT-03: %w", err)
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
	domain, err := mesh2d.PrepareDomain(polygon.Outer, polygon.Holes, seabedAdaptiveCompareBoundaryDetail)
	if err != nil {
		return fmt.Errorf("подготовка берега для ADAPT-03: %w", err)
	}
	if math.Abs(domain.Projection.ReferenceLat-metadata.ProjectionReferenceLatitudeDeg) > 1e-6 ||
		math.Abs(domain.Projection.ReferenceLon-metadata.ProjectionReferenceLongitudeDeg) > 1e-6 {
		return fmt.Errorf("центр LAEA берега не совпадает с моделью дна")
	}
	gmshPath, err := mesh2d.ResolveGmshPath(seabedAdaptiveCompareGmshPath, seabedAdaptiveCompareOutput)
	if err != nil {
		return err
	}
	gmshVersion, err := mesh2d.GmshVersion(gmshPath)
	if err != nil {
		return err
	}

	transformedFields := make(map[string][]float64, len(levels))
	preflights := make(map[string]mesh2d.AdaptivePreflight, len(levels))
	for _, level := range levels {
		values, transformErr := adaptivemodel.TransformTargetSizeField(field, level)
		if transformErr != nil {
			return transformErr
		}
		preflight, estimateErr := mesh2d.EstimateAdaptiveSize(document.Model.Mesh, values)
		if estimateErr != nil {
			return estimateErr
		}
		if seabedAdaptiveCompareMaxCells > 0 && preflight.EstimatedCellCount > seabedAdaptiveCompareMaxCells && !seabedAdaptiveCompareAllowLarge {
			return fmt.Errorf("уровень %q оценивается в %d ячеек и превышает --max-cells=%d; проверьте ресурсы и повторите с --allow-large",
				level.ID, preflight.EstimatedCellCount, seabedAdaptiveCompareMaxCells)
		}
		transformedFields[level.ID] = values
		preflights[level.ID] = preflight
	}

	checksums, err := adaptiveComparisonChecksums(paths)
	if err != nil {
		return err
	}
	report := adaptivemodel.NewComparisonReport()
	report.Inputs = adaptivemodel.AdaptiveComparisonInputs{
		InputMSH: paths.inputMSH, InputMSHSHA256: checksums.inputMSH,
		ExportMetadata: paths.metadata, ExportMetadataSHA256: checksums.metadata,
		SizeFieldCSV: paths.fieldCSV, SizeFieldCSVSHA256: checksums.fieldCSV,
		SizeFieldReport: paths.fieldReport, SizeFieldReportSHA256: checksums.fieldReport,
		Coastline: paths.coastline, CoastlineSHA256: checksums.coastline,
	}
	report.Projection = fmt.Sprintf("%s; центр %.9f°, %.9f°", domain.Projection.Description(), domain.Projection.ReferenceLat, domain.Projection.ReferenceLon)
	report.ProjectionRoundTripMaxErrorMeters = domain.ProjectionRoundTripMaxErrorMeters
	report.BoundaryDetailMeters = seabedAdaptiveCompareBoundaryDetail
	report.EffectiveBoundaryDetailMeters = domain.EffectiveBoundaryToleranceMeters
	report.GmshPath, report.GmshVersion = gmshPath, gmshVersion
	report.GeneratorTimeout = seabedAdaptiveCompareTimeout.String()
	report.MaximumCellCount, report.LargeRunAllowed = seabedAdaptiveCompareMaxCells, seabedAdaptiveCompareAllowLarge

	comparisonDir := filepath.Join(seabedAdaptiveCompareOutput, "seabed", "adaptive", "comparison")
	jsonPath := filepath.Join(comparisonDir, "adaptive-generator-comparison.json")
	tsvPath := filepath.Join(comparisonDir, "adaptive-generator-comparison.tsv")
	zoneNames := make(map[string]string, len(fieldReport.Summary.Zones))
	for _, zone := range fieldReport.Summary.Zones {
		zoneNames[zone.ID] = zone.Name
	}
	if !quiet {
		fmt.Printf("Gmsh %s: %s\n", gmshVersion, gmshPath)
		fmt.Printf("Контрольных уровней: %d; генераторов: %d; последовательных запусков: %d\n", len(levels), len(algorithms), len(levels)*len(algorithms))
	}
	for _, level := range levels {
		values := transformedFields[level.ID]
		levelReport := adaptivemodel.AdaptiveComparisonLevelReport{
			Level: level,
			Transformation: fmt.Sprintf("h_level = %.6g + clamp((h−%.6g)/(%.6g−%.6g),0,1)·(%.6g−%.6g)",
				level.MinimumSizeM, field.MinSizeM, field.MaxSizeM, field.MinSizeM, level.MaximumSizeM, level.MinimumSizeM),
			Preflight: preflights[level.ID],
		}
		if !quiet {
			fmt.Printf("\n%s: %.0f–%.0f м; оценка %d ячеек на запуск\n", level.Name, level.MinimumSizeM, level.MaximumSizeM, levelReport.Preflight.EstimatedCellCount)
		}
		for _, algorithm := range algorithms {
			runDir := filepath.Join(comparisonDir, level.ID, string(algorithm))
			artifacts := adaptivemodel.ComparisonArtifacts{
				BackgroundPOS: filepath.Join(runDir, "background-field.pos"),
				Geo:           filepath.Join(runDir, "black-sea-adaptive.geo"),
				MSH:           filepath.Join(runDir, "black-sea-adaptive.msh"),
				Log:           filepath.Join(runDir, "gmsh.log"),
				RunReportJSON: filepath.Join(runDir, "run-report.json"),
			}
			if !quiet {
				fmt.Printf("  • %s...\n", algorithm.RussianName())
			}
			generated, resources, generateErr := mesh2d.GenerateAdaptiveGmsh(domain, document.Model.Mesh, values, mesh2d.AdaptiveGenerationConfig{
				Algorithm: algorithm, BackgroundFieldPath: artifacts.BackgroundPOS, GeoPath: artifacts.Geo,
				MeshPath: artifacts.MSH, LogPath: artifacts.Log, GmshPath: gmshPath, Timeout: seabedAdaptiveCompareTimeout,
				MinimumTargetSizeM: level.MinimumSizeM, MaximumTargetSizeM: level.MaximumSizeM,
			})
			run := adaptivemodel.GeneratorComparisonRun{Algorithm: algorithm, Resources: resources, Artifacts: artifacts}
			if generateErr != nil {
				run.Error = generateErr.Error()
				run.Artifacts.MSH = ""
				levelReport.Runs = append(levelReport.Runs, run)
				if writeErr := adaptivemodel.WriteComparisonRunJSON(artifacts.RunReportJSON, run); writeErr != nil {
					return writeErr
				}
				if !quiet {
					fmt.Printf("    Ошибка: %v\n", generateErr)
				}
				continue
			}
			run.Topology = mesh2d.ValidateFullQuadMesh(generated)
			if !run.Topology.Accepted {
				run.Error = strings.Join(run.Topology.Reasons, "; ")
				levelReport.Runs = append(levelReport.Runs, run)
				if writeErr := adaptivemodel.WriteComparisonRunJSON(artifacts.RunReportJSON, run); writeErr != nil {
					return writeErr
				}
				generated = mesh2d.Mesh{}
				runtime.GC()
				continue
			}
			run.Geometry = mesh2d.EvaluateQuality(domain, generated, level.MinimumSizeM)
			run.EdgeZones, err = mesh2d.EvaluateAdaptiveEdges(generated, document.Model.Mesh, values, field.Zones, zoneNames)
			if err == nil {
				run.Bathymetry, err = adaptivemodel.EvaluateBathymetryPreservation(document.Model, generated, adaptivemodel.DefaultBathymetryComparisonConfig())
			}
			if err != nil {
				run.Error = err.Error()
			} else {
				run.Success = true
			}
			levelReport.Runs = append(levelReport.Runs, run)
			if writeErr := adaptivemodel.WriteComparisonRunJSON(artifacts.RunReportJSON, run); writeErr != nil {
				return writeErr
			}
			if !quiet {
				fmt.Printf("    %d ячеек; RMSE глубины %.4f м; Qср %.3f; размер в допуске %.1f%%\n",
					run.Topology.CellCount, run.Bathymetry.DepthRMSEM, run.Geometry.MeanCellQuality, adaptiveRunTargetCompliance(run.EdgeZones))
			}
			generated = mesh2d.Mesh{}
			runtime.GC()
		}
		if rankErr := adaptivemodel.RankComparisonLevel(&levelReport); rankErr != nil {
			report.Reasons = append(report.Reasons, rankErr.Error())
		} else if !levelReport.CommonBoundaryConfirmed {
			report.Reasons = append(report.Reasons, fmt.Sprintf("уровень %q не подтвердил одинаковую береговую ошибку", level.ID))
		}
		for _, run := range levelReport.Runs {
			if run.Artifacts.RunReportJSON != "" {
				if writeErr := adaptivemodel.WriteComparisonRunJSON(run.Artifacts.RunReportJSON, run); writeErr != nil {
					return writeErr
				}
			}
		}
		report.Levels = append(report.Levels, levelReport)
	}
	report.Accepted = len(report.Reasons) == 0 && len(report.Levels) == len(levels)
	if err := adaptivemodel.WriteComparisonJSON(jsonPath, report); err != nil {
		return err
	}
	if err := adaptivemodel.WriteComparisonTSV(tsvPath, report); err != nil {
		return err
	}
	if !quiet {
		printAdaptiveComparisonSummary(report)
		fmt.Printf("\nОтчёты ADAPT-03: %s, %s\n", jsonPath, tsvPath)
	}
	if !report.Accepted {
		return fmt.Errorf("ADAPT-03 завершён с непринятыми уровнями; причины сохранены в %s", jsonPath)
	}
	return nil
}

func parseAdaptiveComparisonLevels(value string) ([]adaptivemodel.TargetLevel, error) {
	parts := strings.Split(value, ",")
	levels := make([]adaptivemodel.TargetLevel, 0, len(parts))
	seen := make(map[string]bool)
	for _, part := range parts {
		fields := strings.Split(strings.TrimSpace(part), ":")
		if len(fields) != 3 {
			return nil, fmt.Errorf("уровень %q должен иметь формат id:min:max", part)
		}
		minimum, minErr := strconv.ParseFloat(fields[1], 64)
		maximum, maxErr := strconv.ParseFloat(fields[2], 64)
		id := strings.TrimSpace(fields[0])
		if minErr != nil || maxErr != nil || minimum <= 0 || maximum <= minimum || seen[id] {
			return nil, fmt.Errorf("уровень %q содержит некорректный или повторный диапазон", part)
		}
		seen[id] = true
		name := id
		switch id {
		case "detailed":
			name = "Подробная сетка"
		case "coarse":
			name = "Укрупнённая сетка"
		}
		levels = append(levels, adaptivemodel.TargetLevel{ID: id, Name: name, MinimumSizeM: minimum, MaximumSizeM: maximum})
	}
	if len(levels) == 0 {
		return nil, fmt.Errorf("список контрольных уровней пуст")
	}
	return levels, nil
}

type adaptiveComparePaths struct {
	inputMSH, metadata, fieldCSV, fieldReport, coastline string
}

func resolveAdaptiveCompareInputPaths() adaptiveComparePaths {
	input := strings.TrimSpace(seabedAdaptiveCompareInput)
	if input == "" {
		input = filepath.Join(seabedAdaptiveCompareOutput, "seabed", "black-sea-depth.msh")
	}
	metadata := strings.TrimSpace(seabedAdaptiveCompareMetadata)
	if metadata == "" {
		metadata = filepath.Join(filepath.Dir(input), "export-metadata.json")
	}
	field := strings.TrimSpace(seabedAdaptiveCompareField)
	if field == "" {
		field = filepath.Join(seabedAdaptiveCompareOutput, "seabed", "adaptive", "size-field.csv")
	}
	fieldReport := strings.TrimSpace(seabedAdaptiveCompareFieldReport)
	if fieldReport == "" {
		fieldReport = filepath.Join(seabedAdaptiveCompareOutput, "seabed", "adaptive", "size-field.json")
	}
	return adaptiveComparePaths{inputMSH: input, metadata: metadata, fieldCSV: field, fieldReport: fieldReport, coastline: strings.TrimSpace(seabedAdaptiveCompareCoastline)}
}

type adaptiveCompareChecksums struct {
	inputMSH, metadata, fieldCSV, fieldReport, coastline string
}

func adaptiveComparisonChecksums(paths adaptiveComparePaths) (adaptiveCompareChecksums, error) {
	values := []*string{&paths.inputMSH, &paths.metadata, &paths.fieldCSV, &paths.fieldReport, &paths.coastline}
	resultValues := make([]string, len(values))
	for index, path := range values {
		checksum, err := adaptiveFileSHA256(*path)
		if err != nil {
			return adaptiveCompareChecksums{}, err
		}
		resultValues[index] = checksum
	}
	return adaptiveCompareChecksums{inputMSH: resultValues[0], metadata: resultValues[1], fieldCSV: resultValues[2], fieldReport: resultValues[3], coastline: resultValues[4]}, nil
}

func adaptiveRunTargetCompliance(zones []mesh2d.ZoneEdgeStatistics) float64 {
	var sum float64
	var count int64
	for _, zone := range zones {
		sum += zone.WithinTolerancePct * float64(zone.EdgeObservationCount)
		count += zone.EdgeObservationCount
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func printAdaptiveComparisonSummary(report adaptivemodel.AdaptiveComparisonReport) {
	fmt.Println("\nРейтинг адаптивных генераторов:")
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Уровень\tМесто\tГенератор\tИтог\tБатиметрия\tГеометрия\tРазмер\tВремя, с")
	fmt.Fprintln(writer, "-------\t-----\t---------\t-----\t-----------\t----------\t------\t--------")
	for _, level := range report.Levels {
		for _, run := range level.Runs {
			if !run.Success {
				continue
			}
			duration := run.Resources.GmshDurationSeconds + run.Resources.PostprocessDurationSeconds + run.Bathymetry.DurationSeconds
			fmt.Fprintf(writer, "%s\t%d\t%s\t%.2f\t%.2f\t%.2f\t%.2f\t%.1f\n",
				level.Level.Name, run.Rank, run.Algorithm.RussianName(), run.Score.Overall,
				run.Score.Bathymetry, run.Score.CellGeometry, run.Score.TargetSizeCompliance, duration)
		}
	}
	_ = writer.Flush()
}
