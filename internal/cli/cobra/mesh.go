package cobra

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/coastline"
	mesh2d "coastal-geometry/internal/domain/mesh"
	svgrender "coastal-geometry/internal/render/svg"

	"github.com/spf13/cobra"
)

var (
	meshInput            string
	meshSourceURL        string
	meshRefresh          bool
	meshOutput           string
	meshCellSizes        string
	meshBoundaryDetails  string
	meshGenerators       string
	meshGmshPath         string
	meshMaxCells         int64
	meshAllowLarge       bool
	meshGeneratorTimeout time.Duration
)

var meshCmd = &cobra.Command{
	Use:   "mesh",
	Short: "Построить и сравнить расчётные 2D-сетки Чёрного моря",
	Long: `Строит четырёхугольные сетки по всей поверхности Чёрного моря
открытым генератором Gmsh. Для каждой пары «детализация береговой линии —
целевая длина ребра» сравниваются независимые алгоритмы Delaunay,
Frontal-Delaunay for Quads и упаковка параллелограммов.

Критерий сохранения берега суммирует абсолютные локальные изменения площади,
поэтому уменьшение залива не компенсируется увеличением соседней косы. В
ранжирование также входят RMS-отклонение границы, доля четырёхугольников и
геометрическое качество ячеек. Полные сетки сохраняются в MSH, обзор — в SVG.`,
	RunE: runMesh,
}

func init() {
	rootCmd.AddCommand(meshCmd)
	meshCmd.Flags().StringVar(&meshInput, "input", "", "локальный JSON/GeoJSON полигона; по умолчанию полный контур Чёрного моря")
	meshCmd.Flags().StringVar(&meshSourceURL, "source-url", "", "явный удалённый GeoJSON-источник полигона")
	meshCmd.Flags().BoolVar(&meshRefresh, "refresh", false, "обновить открытый контур Чёрного моря MarineRegions")
	meshCmd.Flags().StringVar(&meshOutput, "output", "", "каталог вывода (по умолчанию: ./output)")
	meshCmd.Flags().StringVar(&meshCellSizes, "cell-sizes", "1000,500,300,200", "целевые длины рёбер ячеек в метрах через запятую")
	meshCmd.Flags().StringVar(&meshBoundaryDetails, "boundary-details", "1000,500,300,200", "метрические допуски детализации берега через запятую")
	meshCmd.Flags().StringVar(&meshGenerators, "generators", "delaunay,frontal-quad,parallelograms", "алгоритмы Gmsh через запятую")
	meshCmd.Flags().StringVar(&meshGmshPath, "gmsh", "", "путь к бинарному файлу Gmsh")
	meshCmd.Flags().Int64Var(&meshMaxCells, "max-cells", 15_000_000, "предельная оценка числа ячеек для одного запуска")
	meshCmd.Flags().BoolVar(&meshAllowLarge, "allow-large", false, "разрешить запуск сверх --max-cells после проверки свободных ресурсов")
	meshCmd.Flags().DurationVar(&meshGeneratorTimeout, "generator-timeout", 5*time.Minute, "лимит времени одного запуска Gmsh")
}

type meshComparisonReport struct {
	SchemaVersion                     string            `json:"schema_version"`
	GeneratedAt                       string            `json:"generated_at"`
	DatasetName                       string            `json:"dataset_name"`
	Source                            string            `json:"source"`
	GmshPath                          string            `json:"gmsh_path"`
	GmshVersion                       string            `json:"gmsh_version"`
	Projection                        string            `json:"projection"`
	ProjectionRoundTripMaxErrorMeters float64           `json:"projection_round_trip_max_error_meters"`
	Criterion                         meshCriterion     `json:"criterion"`
	Levels                            []meshLevelReport `json:"levels"`
}

type meshCriterion struct {
	Method  string             `json:"method"`
	Weights map[string]float64 `json:"weights"`
	Note    string             `json:"note"`
}

type meshLevelReport struct {
	TargetEdgeMeters              float64          `json:"target_edge_meters"`
	BoundaryDetailMeters          float64          `json:"boundary_detail_meters"`
	EffectiveBoundaryDetailMeters float64          `json:"effective_boundary_detail_meters"`
	EstimatedCellCount            int64            `json:"estimated_cell_count"`
	OriginalPointCount            int              `json:"original_point_count"`
	SimplifiedPointCount          int              `json:"simplified_point_count"`
	BestGenerator                 mesh2d.Algorithm `json:"best_generator"`
	Results                       []meshRunReport  `json:"results"`
}

type meshRunReport struct {
	Rank            int                   `json:"rank,omitempty"`
	Algorithm       mesh2d.Algorithm      `json:"algorithm"`
	DurationSeconds float64               `json:"duration_seconds"`
	Error           string                `json:"error,omitempty"`
	Metrics         mesh2d.QualityMetrics `json:"metrics"`
	GeoFile         string                `json:"geo_file"`
	MeshFile        string                `json:"mesh_file,omitempty"`
	SVGFile         string                `json:"svg_file,omitempty"`
	LogFile         string                `json:"log_file"`
}

func runMesh(_ *cobra.Command, _ []string) error {
	cellSizes, err := parsePositiveFloatList(meshCellSizes, "длины рёбер")
	if err != nil {
		return err
	}
	details, err := parsePositiveFloatList(meshBoundaryDetails, "детализации берега")
	if err != nil {
		return err
	}
	details, err = broadcastDetails(details, len(cellSizes))
	if err != nil {
		return err
	}
	algorithms, err := parseMeshAlgorithms(meshGenerators)
	if err != nil {
		return err
	}

	loadOptions := coastline.LoadOptions{LocalPath: meshInput, RemoteURL: meshSourceURL, Refresh: meshRefresh}
	if meshInput == "" && meshSourceURL == "" {
		if !meshRefresh {
			if _, statErr := os.Stat(filepath.Join(coastline.DefaultCoastlineCacheDir, "black-sea.geojson")); statErr == nil {
				loadOptions.LocalPath = filepath.Join(coastline.DefaultCoastlineCacheDir, "black-sea.geojson")
			} else {
				loadOptions.RemoteURL = coastline.DefaultCoastlineGeoJSONURL
			}
		} else {
			loadOptions.RemoteURL = coastline.DefaultCoastlineGeoJSONURL
		}
	}
	polygon, err := coastline.LoadPolygon(loadOptions)
	if err != nil {
		return fmt.Errorf("загрузка поверхности Чёрного моря: %w", err)
	}
	for _, warning := range append(polygon.LoadWarnings, polygon.Validation.Warnings...) {
		fmt.Printf("Предупреждение: %s\n", warning)
	}

	outputMgr := cli.NewOutputPathManager(meshOutput)
	if err := outputMgr.EnsureDirectories(); err != nil {
		return fmt.Errorf("подготовка каталогов вывода: %w", err)
	}
	gmshPath, err := mesh2d.ResolveGmshPath(meshGmshPath, outputMgr.BaseDir())
	if err != nil {
		return err
	}
	gmshVersion, err := mesh2d.GmshVersion(gmshPath)
	if err != nil {
		return err
	}
	fmt.Printf("Загружено Чёрное море: %s; внешних точек %d, островов %d\n", polygon.Source, len(polygon.Outer), len(polygon.Holes))
	fmt.Printf("Генератор: Gmsh %s (%s)\n", gmshVersion, gmshPath)

	report := meshComparisonReport{
		SchemaVersion: "lito-mesh-comparison/v1",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		DatasetName:   polygon.DatasetName,
		Source:        polygon.Source,
		GmshPath:      gmshPath,
		GmshVersion:   gmshVersion,
		Criterion: meshCriterion{
			Method:  "детерминированная геометрическая оценка",
			Weights: map[string]float64{"сохранение_локальной_площади": 0.55, "отклонение_границы": 0.20, "качество_ячеек": 0.20, "доля_четырёхугольников": 0.05},
			Note:    "ИИ не применяется: без размеченного эталона детерминированная метрика воспроизводимее нейросетевой оценки.",
		},
	}

	for index, edgeMeters := range cellSizes {
		detailMeters := details[index]
		prepared, prepareErr := mesh2d.PrepareDomain(polygon.Outer, polygon.Holes, detailMeters)
		if prepareErr != nil {
			return fmt.Errorf("подготовка границы %.0f м: %w", detailMeters, prepareErr)
		}
		if report.Projection == "" {
			report.Projection = fmt.Sprintf("%s; центр %.6f°, %.6f°", prepared.Projection.Description(), prepared.Projection.ReferenceLat, prepared.Projection.ReferenceLon)
			report.ProjectionRoundTripMaxErrorMeters = prepared.ProjectionRoundTripMaxErrorMeters
			fmt.Printf("Обратная проекция WGS 84: максимальная ошибка цикла %.9f м\n", prepared.ProjectionRoundTripMaxErrorMeters)
		}
		estimated := prepared.EstimatedCellCount(edgeMeters)
		if meshMaxCells > 0 && estimated > meshMaxCells && !meshAllowLarge {
			return fmt.Errorf("сетка %.0f м оценивается в %d ячеек, что превышает --max-cells=%d; проверьте место на диске и повторите с --allow-large", edgeMeters, estimated, meshMaxCells)
		}

		level := meshLevelReport{
			TargetEdgeMeters: edgeMeters, BoundaryDetailMeters: detailMeters, EffectiveBoundaryDetailMeters: prepared.EffectiveBoundaryToleranceMeters, EstimatedCellCount: estimated,
			OriginalPointCount: prepared.OriginalPointCount, SimplifiedPointCount: prepared.SimplifiedPointCount,
		}
		fmt.Printf("\nСетка: ребро %.0f м, детализация берега %.0f м, оценка %d ячеек\n", edgeMeters, detailMeters, estimated)
		if prepared.EffectiveBoundaryToleranceMeters < detailMeters {
			fmt.Printf("  Топологически безопасный фактический допуск: %.2f м\n", prepared.EffectiveBoundaryToleranceMeters)
		}
		for _, algorithm := range algorithms {
			baseName := fmt.Sprintf("black-sea-edge-%.0f-detail-%.0f-%s", edgeMeters, detailMeters, algorithm)
			geoPath := outputMgr.MeshPath(filepath.Join("geo", baseName+".geo"))
			mshPath := outputMgr.MeshPath(filepath.Join("msh", baseName+".msh"))
			logPath := outputMgr.MeshPath(filepath.Join("logs", baseName+".log"))
			svgPath := outputMgr.MeshPath(filepath.Join("svg", baseName+".svg"))
			fmt.Printf("  • %s...\n", algorithm.RussianName())
			startedAt := time.Now()
			generated, generateErr := mesh2d.GenerateGmsh(prepared, mesh2d.GenerationConfig{
				Algorithm: algorithm, TargetEdgeMeters: edgeMeters, GeoPath: geoPath, MeshPath: mshPath, LogPath: logPath, GmshPath: gmshPath, Timeout: meshGeneratorTimeout,
			})
			durationSeconds := time.Since(startedAt).Seconds()
			if generateErr != nil {
				message := fmt.Sprintf("генератор %s, ребро %.0f м: %v", algorithm, edgeMeters, generateErr)
				level.Results = append(level.Results, meshRunReport{Algorithm: algorithm, DurationSeconds: durationSeconds, Error: message, GeoFile: geoPath, LogFile: logPath})
				fmt.Printf("    Пропущен: %v\n", generateErr)
				continue
			}
			metrics := mesh2d.EvaluateQuality(prepared, generated, edgeMeters)
			if err := svgrender.DrawMeshReportSVG(prepared, generated, metrics, svgrender.MeshReportOptions{
				DatasetName: polygon.DatasetName, Source: polygon.Source, Algorithm: algorithm,
				TargetEdgeMeters: edgeMeters, BoundaryDetailMeters: detailMeters, EffectiveBoundaryDetailMeters: prepared.EffectiveBoundaryToleranceMeters, FullMeshPath: mshPath,
				OriginalPointCount: prepared.OriginalPointCount, SimplifiedPointCount: prepared.SimplifiedPointCount,
			}, svgPath); err != nil {
				return fmt.Errorf("SVG сетки %s: %w", algorithm, err)
			}
			level.Results = append(level.Results, meshRunReport{Algorithm: algorithm, DurationSeconds: durationSeconds, Metrics: metrics, GeoFile: geoPath, MeshFile: mshPath, SVGFile: svgPath, LogFile: logPath})
			fmt.Printf("    %d ячеек; четырёхугольники %.2f%%; Σ|ΔS| %.3f км²; оценка %.2f/100\n", metrics.CellCount, metrics.QuadSharePercent, metrics.CumulativeFeatureAreaDeviationKM2, metrics.CompositeScore)
		}
		sort.SliceStable(level.Results, func(left, right int) bool {
			if level.Results[left].Error != "" || level.Results[right].Error != "" {
				return level.Results[left].Error == "" && level.Results[right].Error != ""
			}
			if level.Results[left].Metrics.CompositeScore == level.Results[right].Metrics.CompositeScore {
				return level.Results[left].Algorithm < level.Results[right].Algorithm
			}
			return level.Results[left].Metrics.CompositeScore > level.Results[right].Metrics.CompositeScore
		})
		successCount := 0
		for resultIndex := range level.Results {
			if level.Results[resultIndex].Error == "" {
				successCount++
				level.Results[resultIndex].Rank = successCount
			}
		}
		if successCount == 0 {
			return fmt.Errorf("ни один генератор не построил сетку с ребром %.0f м", edgeMeters)
		}
		level.BestGenerator = level.Results[0].Algorithm
		fmt.Printf("  Лучший генератор: %s\n", level.BestGenerator.RussianName())
		report.Levels = append(report.Levels, level)
	}

	jsonPath := outputMgr.MeshPath("mesh-comparison.json")
	tsvPath := outputMgr.MeshPath("mesh-comparison.tsv")
	if err := writeMeshComparisonJSON(jsonPath, report); err != nil {
		return err
	}
	if err := writeMeshComparisonTSV(tsvPath, report); err != nil {
		return err
	}
	printMeshSummary(report)
	fmt.Printf("\nОтчёты сохранены: %s, %s\n", jsonPath, tsvPath)
	return nil
}

func parsePositiveFloatList(value, label string) ([]float64, error) {
	parts := strings.Split(value, ",")
	result := make([]float64, 0, len(parts))
	for _, part := range parts {
		number, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || number <= 0 {
			return nil, fmt.Errorf("некорректное значение %s %q", label, part)
		}
		result = append(result, number)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("список %s пуст", label)
	}
	return result, nil
}

func broadcastDetails(details []float64, count int) ([]float64, error) {
	if len(details) == count {
		return details, nil
	}
	if len(details) == 1 {
		result := make([]float64, count)
		for index := range result {
			result[index] = details[0]
		}
		return result, nil
	}
	return nil, fmt.Errorf("число --boundary-details должно равняться числу --cell-sizes либо быть равно одному")
}

func parseMeshAlgorithms(value string) ([]mesh2d.Algorithm, error) {
	parts := strings.Split(value, ",")
	result := make([]mesh2d.Algorithm, 0, len(parts))
	seen := make(map[mesh2d.Algorithm]bool)
	for _, part := range parts {
		algorithm, err := mesh2d.ParseAlgorithm(part)
		if err != nil {
			return nil, err
		}
		if !seen[algorithm] {
			seen[algorithm] = true
			result = append(result, algorithm)
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("не указан ни один генератор")
	}
	return result, nil
}

func writeMeshComparisonJSON(path string, report meshComparisonReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта сеток: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта сеток %q: %w", path, err)
	}
	return nil
}

func writeMeshComparisonTSV(path string, report meshComparisonReport) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("создание таблицы сеток %q: %w", path, err)
	}
	defer file.Close()
	writer := tabwriter.NewWriter(file, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Ребро, м\tЗаданный допуск, м\tФактический допуск, м\tМесто\tГенератор\tВремя, с\tЯчейки\tЧетырёхугольники, %\tΣ|ΔS|, км²\tRMS, м\tКачество\tИтог\tОшибка")
	fmt.Fprintln(writer, "---------\t-------------------\t---------------------\t-----\t---------\t--------\t------\t-------------------\t----------\t------\t--------\t----\t------")
	for _, level := range report.Levels {
		for _, result := range level.Results {
			rank := "—"
			if result.Rank > 0 {
				rank = strconv.Itoa(result.Rank)
			}
			fmt.Fprintf(writer, "%.0f\t%.0f\t%.2f\t%s\t%s\t%.2f\t%d\t%.2f\t%.6f\t%.2f\t%.4f\t%.2f\t%s\n",
				level.TargetEdgeMeters, level.BoundaryDetailMeters, level.EffectiveBoundaryDetailMeters, rank, result.Algorithm.RussianName(), result.DurationSeconds, result.Metrics.CellCount,
				result.Metrics.QuadSharePercent, result.Metrics.CumulativeFeatureAreaDeviationKM2, result.Metrics.BoundaryRMSMeters,
				result.Metrics.MeanCellQuality, result.Metrics.CompositeScore, result.Error)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("запись таблицы сеток: %w", err)
	}
	return nil
}

func printMeshSummary(report meshComparisonReport) {
	fmt.Println("\nСРАВНЕНИЕ ГЕНЕРАТОРОВ 2D-СЕТОК")
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Ребро, м\tДетализация, м\tЛучший генератор\tИтог")
	fmt.Fprintln(writer, "---------\t---------------\t-----------------\t----")
	for _, level := range report.Levels {
		fmt.Fprintf(writer, "%.0f\t%.0f\t%s\t%.2f\n", level.TargetEdgeMeters, level.BoundaryDetailMeters, level.BestGenerator.RussianName(), level.Results[0].Metrics.CompositeScore)
	}
	writer.Flush()
}
