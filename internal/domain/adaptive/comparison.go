package adaptive

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"coastal-geometry/internal/domain/mesh"
)

const (
	// ComparisonSchemaVersion задаёт машинный контракт рейтинга ADAPT-03.
	ComparisonSchemaVersion = "lito-adaptive-generator-comparison/v1"
)

var comparisonLevelIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// TargetLevel задаёт один контрольный диапазон поля размера. Исходные
// относительные пространственные различия сохраняются линейным преобразованием.
type TargetLevel struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	MinimumSizeM float64 `json:"minimum_size_m"`
	MaximumSizeM float64 `json:"maximum_size_m"`
}

// DefaultComparisonLevels возвращает подробный и укрупнённый масштабы,
// покрывающие исходное требование 200–1000 и 500–1000 м.
func DefaultComparisonLevels() []TargetLevel {
	return []TargetLevel{
		{ID: "detailed", Name: "Подробная сетка", MinimumSizeM: 200, MaximumSizeM: 1000},
		{ID: "coarse", Name: "Укрупнённая сетка", MinimumSizeM: 500, MaximumSizeM: 1000},
	}
}

// TransformTargetSizeField переносит исходный диапазон ADAPT-01 в контрольный
// диапазон, не меняя порядок и пространственный рисунок значений.
func TransformTargetSizeField(field TargetSizeField, level TargetLevel) ([]float64, error) {
	if !comparisonLevelIDPattern.MatchString(level.ID) || strings.TrimSpace(level.Name) == "" {
		return nil, fmt.Errorf("уровень сравнения имеет некорректный идентификатор или имя")
	}
	if !finite(level.MinimumSizeM) || !finite(level.MaximumSizeM) || level.MinimumSizeM <= 0 || level.MaximumSizeM <= level.MinimumSizeM {
		return nil, fmt.Errorf("уровень %q должен иметь положительный возрастающий диапазон", level.ID)
	}
	if len(field.TargetSizeM) <= 1 || field.MaxSizeM <= field.MinSizeM {
		return nil, fmt.Errorf("исходное поле размера не содержит изменяемого диапазона")
	}
	result := make([]float64, len(field.TargetSizeM))
	for nodeID := 1; nodeID < len(field.TargetSizeM); nodeID++ {
		normalized := (field.TargetSizeM[nodeID] - field.MinSizeM) / (field.MaxSizeM - field.MinSizeM)
		normalized = math.Max(0, math.Min(1, normalized))
		result[nodeID] = level.MinimumSizeM + normalized*(level.MaximumSizeM-level.MinimumSizeM)
	}
	return result, nil
}

// ComparisonCriterion фиксирует формулы и веса воспроизводимого рейтинга.
type ComparisonCriterion struct {
	Method          string             `json:"method"`
	Weights         map[string]float64 `json:"weights"`
	CoastlineScore  string             `json:"coastline_score"`
	BathymetryScore string             `json:"bathymetry_score"`
	GeometryScore   string             `json:"geometry_score"`
	TargetSizeScore string             `json:"target_size_score"`
	EfficiencyScore string             `json:"efficiency_score"`
	Limitation      string             `json:"limitation"`
}

// ComparisonScore хранит компоненты общей оценки 0–100.
type ComparisonScore struct {
	Coastline            float64 `json:"coastline"`
	Bathymetry           float64 `json:"bathymetry"`
	CellGeometry         float64 `json:"cell_geometry"`
	TargetSizeCompliance float64 `json:"target_size_compliance"`
	Efficiency           float64 `json:"efficiency"`
	Overall              float64 `json:"overall"`
}

// ComparisonArtifacts перечисляет файлы одного запуска генератора.
type ComparisonArtifacts struct {
	BackgroundPOS string `json:"background_pos"`
	Geo           string `json:"geo"`
	MSH           string `json:"msh,omitempty"`
	Log           string `json:"log"`
	RunReportJSON string `json:"run_report_json,omitempty"`
}

// GeneratorComparisonRun описывает успешный или неуспешный запуск одного
// генератора на одном контрольном диапазоне.
type GeneratorComparisonRun struct {
	Rank       int                           `json:"rank,omitempty"`
	Algorithm  mesh.Algorithm                `json:"algorithm"`
	Success    bool                          `json:"success"`
	Error      string                        `json:"error,omitempty"`
	Resources  mesh.GenerationResourceStats  `json:"resources"`
	Topology   mesh.TopologyValidation       `json:"topology"`
	Geometry   mesh.QualityMetrics           `json:"geometry"`
	EdgeZones  []mesh.ZoneEdgeStatistics     `json:"edge_zones"`
	Bathymetry BathymetryPreservationMetrics `json:"bathymetry"`
	Score      ComparisonScore               `json:"score"`
	Artifacts  ComparisonArtifacts           `json:"artifacts"`
}

// AdaptiveComparisonLevelReport объединяет одинаковое поле и все генераторы.
type AdaptiveComparisonLevelReport struct {
	Level                       TargetLevel              `json:"level"`
	Transformation              string                   `json:"transformation"`
	Preflight                   mesh.AdaptivePreflight   `json:"preflight"`
	BestGenerator               mesh.Algorithm           `json:"best_generator,omitempty"`
	SuccessfulGeneratorCount    int                      `json:"successful_generator_count"`
	FailedGeneratorCount        int                      `json:"failed_generator_count"`
	CoastlineDeviationSpreadPct float64                  `json:"coastline_deviation_spread_percent"`
	BoundaryRMSSpreadM          float64                  `json:"boundary_rms_spread_m"`
	CommonBoundaryConfirmed     bool                     `json:"common_boundary_confirmed"`
	Runs                        []GeneratorComparisonRun `json:"runs"`
}

// AdaptiveComparisonInputs фиксирует пути и контрольные суммы общих входов.
type AdaptiveComparisonInputs struct {
	InputMSH              string `json:"input_msh"`
	InputMSHSHA256        string `json:"input_msh_sha256"`
	ExportMetadata        string `json:"export_metadata"`
	ExportMetadataSHA256  string `json:"export_metadata_sha256"`
	SizeFieldCSV          string `json:"size_field_csv"`
	SizeFieldCSVSHA256    string `json:"size_field_csv_sha256"`
	SizeFieldReport       string `json:"size_field_report"`
	SizeFieldReportSHA256 string `json:"size_field_report_sha256"`
	Coastline             string `json:"coastline"`
	CoastlineSHA256       string `json:"coastline_sha256"`
}

// AdaptiveComparisonReport является полностью воспроизводимым итогом ADAPT-03.
type AdaptiveComparisonReport struct {
	SchemaVersion                     string                          `json:"schema_version"`
	GeneratedAt                       string                          `json:"generated_at"`
	Inputs                            AdaptiveComparisonInputs        `json:"inputs"`
	Projection                        string                          `json:"projection"`
	ProjectionRoundTripMaxErrorMeters float64                         `json:"projection_round_trip_max_error_meters"`
	BoundaryDetailMeters              float64                         `json:"boundary_detail_meters"`
	EffectiveBoundaryDetailMeters     float64                         `json:"effective_boundary_detail_meters"`
	GmshPath                          string                          `json:"gmsh_path"`
	GmshVersion                       string                          `json:"gmsh_version"`
	GeneratorTimeout                  string                          `json:"generator_timeout"`
	MaximumCellCount                  int64                           `json:"maximum_cell_count"`
	LargeRunAllowed                   bool                            `json:"large_run_allowed"`
	Criterion                         ComparisonCriterion             `json:"criterion"`
	Levels                            []AdaptiveComparisonLevelReport `json:"levels"`
	Accepted                          bool                            `json:"accepted"`
	Reasons                           []string                        `json:"reasons"`
}

// DefaultComparisonCriterion возвращает фиксированные веса ADAPT-03.
func DefaultComparisonCriterion() ComparisonCriterion {
	return ComparisonCriterion{
		Method: "детерминированная взвешенная оценка 0–100; сортировка по overall, затем по идентификатору алгоритма",
		Weights: map[string]float64{
			"сохранение_берега": 0.15, "сохранение_батиметрии": 0.35,
			"геометрия_ячеек": 0.20, "соответствие_размеру": 0.20, "эффективность": 0.10,
		},
		CoastlineScore:  "100 / (1 + coastal_area_deviation_percent + boundary_rms_m / minimum_size_m)",
		BathymetryScore: "100 / (1 + 10·L), L=0.40·RMSE/P95_depth + 0.20·|ΔV|/100 + 0.20·mean_isobath_area_error/100 + 0.15·slope_RMSE/90 + 0.05·nearest_fallback/100",
		GeometryScore:   "100·(0.60·mean_cell_quality + 0.40·P05_cell_quality)",
		TargetSizeScore: "взвешенная по числу наблюдений доля рёбер с 0.5 ≤ actual/target ≤ 1.5",
		EfficiencyScore: "50·min_duration/duration + 50·min_peak_RSS/peak_RSS внутри уровня",
		Limitation:      "батиметрия проверяется внутренним восстановлением поля BATHY-03 в центрах ячеек; внешняя отложенная проверка и научные пороги относятся к QA-02",
	}
}

// RankComparisonLevel рассчитывает компоненты, устойчиво сортирует успешные
// запуски и подтверждает одинаковость береговой ошибки.
func RankComparisonLevel(level *AdaptiveComparisonLevelReport) error {
	if level == nil || len(level.Runs) == 0 {
		return fmt.Errorf("уровень сравнения не содержит запусков")
	}
	minimumDuration, minimumRSS := math.Inf(1), math.Inf(1)
	for index := range level.Runs {
		run := &level.Runs[index]
		if !run.Success || !run.Topology.Accepted {
			level.FailedGeneratorCount++
			continue
		}
		level.SuccessfulGeneratorCount++
		duration := run.Resources.GmshDurationSeconds + run.Resources.PostprocessDurationSeconds + run.Bathymetry.DurationSeconds
		if duration > 0 {
			minimumDuration = math.Min(minimumDuration, duration)
		}
		if run.Resources.PeakRSSBytes > 0 {
			minimumRSS = math.Min(minimumRSS, float64(run.Resources.PeakRSSBytes))
		}
	}
	if level.SuccessfulGeneratorCount == 0 {
		return fmt.Errorf("ни один генератор не создал принятую сетку для уровня %q", level.Level.ID)
	}
	criterion := DefaultComparisonCriterion()
	for index := range level.Runs {
		run := &level.Runs[index]
		if !run.Success || !run.Topology.Accepted {
			continue
		}
		run.Score.Coastline = coastlineComparisonScore(run.Geometry, level.Level.MinimumSizeM)
		run.Score.Bathymetry = bathymetryComparisonScore(run.Bathymetry)
		run.Score.CellGeometry = 100 * (0.60*run.Geometry.MeanCellQuality + 0.40*run.Geometry.P05CellQuality)
		run.Score.TargetSizeCompliance = targetSizeCompliance(run.EdgeZones)
		duration := run.Resources.GmshDurationSeconds + run.Resources.PostprocessDurationSeconds + run.Bathymetry.DurationSeconds
		timeScore := 100.0
		if !math.IsInf(minimumDuration, 1) && duration > 0 {
			timeScore = 100 * minimumDuration / duration
		}
		memoryScore := 100.0
		if !math.IsInf(minimumRSS, 1) && run.Resources.PeakRSSBytes > 0 {
			memoryScore = 100 * minimumRSS / float64(run.Resources.PeakRSSBytes)
		}
		run.Score.Efficiency = 0.5*timeScore + 0.5*memoryScore
		run.Score.Overall =
			criterion.Weights["сохранение_берега"]*run.Score.Coastline +
				criterion.Weights["сохранение_батиметрии"]*run.Score.Bathymetry +
				criterion.Weights["геометрия_ячеек"]*run.Score.CellGeometry +
				criterion.Weights["соответствие_размеру"]*run.Score.TargetSizeCompliance +
				criterion.Weights["эффективность"]*run.Score.Efficiency
	}
	sort.SliceStable(level.Runs, func(left, right int) bool {
		leftRun, rightRun := level.Runs[left], level.Runs[right]
		if leftRun.Success != rightRun.Success {
			return leftRun.Success
		}
		if leftRun.Score.Overall == rightRun.Score.Overall {
			return leftRun.Algorithm < rightRun.Algorithm
		}
		return leftRun.Score.Overall > rightRun.Score.Overall
	})
	rank := 0
	minCoast, maxCoast := math.Inf(1), math.Inf(-1)
	minBoundary, maxBoundary := math.Inf(1), math.Inf(-1)
	for index := range level.Runs {
		run := &level.Runs[index]
		if !run.Success || !run.Topology.Accepted {
			continue
		}
		rank++
		run.Rank = rank
		if rank == 1 {
			level.BestGenerator = run.Algorithm
		}
		minCoast = math.Min(minCoast, run.Geometry.CoastalAreaDeviationPercent)
		maxCoast = math.Max(maxCoast, run.Geometry.CoastalAreaDeviationPercent)
		minBoundary = math.Min(minBoundary, run.Geometry.BoundaryRMSMeters)
		maxBoundary = math.Max(maxBoundary, run.Geometry.BoundaryRMSMeters)
	}
	level.CoastlineDeviationSpreadPct = maxCoast - minCoast
	level.BoundaryRMSSpreadM = maxBoundary - minBoundary
	level.CommonBoundaryConfirmed = level.CoastlineDeviationSpreadPct <= 1e-6 && level.BoundaryRMSSpreadM <= 1e-6
	return nil
}

func coastlineComparisonScore(metrics mesh.QualityMetrics, minimumSizeM float64) float64 {
	loss := metrics.CoastalAreaDeviationPercent
	if minimumSizeM > 0 {
		loss += metrics.BoundaryRMSMeters / minimumSizeM
	}
	return 100 / (1 + math.Max(0, loss))
}

func bathymetryComparisonScore(metrics BathymetryPreservationMetrics) float64 {
	depthScale := math.Max(1, metrics.ReferenceP95DepthM)
	loss := 0.40*metrics.DepthRMSEM/depthScale +
		0.20*math.Abs(metrics.WaterVolumeDeviationPercent)/100 +
		0.20*metrics.MeanIsobathAreaDeviationPct/100 +
		0.15*metrics.SlopeRMSEDeg/90 +
		0.05*metrics.NearestFallbackNodePercent/100
	return 100 / (1 + 10*math.Max(0, loss))
}

func targetSizeCompliance(zones []mesh.ZoneEdgeStatistics) float64 {
	var weighted float64
	var count int64
	for _, zone := range zones {
		weighted += zone.WithinTolerancePct * float64(zone.EdgeObservationCount)
		count += zone.EdgeObservationCount
	}
	if count == 0 {
		return 0
	}
	return weighted / float64(count)
}

// WriteComparisonRunJSON сохраняет автономный результат одного запуска до
// формирования общего рейтинга.
func WriteComparisonRunJSON(path string, run GeneratorComparisonRun) error {
	normalizeComparisonRun(&run)
	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта запуска ADAPT-03: %w", err)
	}
	return writeComparisonFile(path, append(data, '\n'))
}

// WriteComparisonJSON сохраняет общий рейтинг ADAPT-03.
func WriteComparisonJSON(path string, report AdaptiveComparisonReport) error {
	if report.SchemaVersion != ComparisonSchemaVersion {
		return fmt.Errorf("отчёт ADAPT-03 имеет неподдерживаемую схему %q", report.SchemaVersion)
	}
	normalizeComparisonReport(&report)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта ADAPT-03: %w", err)
	}
	return writeComparisonFile(path, append(data, '\n'))
}

// WriteComparisonTSV сохраняет компактный рейтинг всех уровней.
func WriteComparisonTSV(path string, report AdaptiveComparisonReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога TSV ADAPT-03: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("создание TSV ADAPT-03 %q: %w", path, err)
	}
	writer := tabwriter.NewWriter(file, 0, 0, 2, ' ', 0)
	writeErr := func() error {
		if _, err := fmt.Fprintln(writer, "Уровень\tДиапазон, м\tМесто\tГенератор\tСтатус\tИтог\tБерег\tБатиметрия\tГеометрия\tРазмер\tЭффективность\tЯчеек\tRMSE глубины, м\tΔ объёма, %\tВремя Gmsh, с\tПик RSS, МиБ\tОшибка"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(writer, "-------\t-----------\t-----\t---------\t------\t-----\t-----\t-----------\t----------\t------\t-------------\t------\t----------------\t-----------\t-------------\t-------------\t------"); err != nil {
			return err
		}
		for _, level := range report.Levels {
			for _, run := range level.Runs {
				status := "успех"
				if !run.Success {
					status = "ошибка"
				}
				if _, err := fmt.Fprintf(writer, "%s\t%.0f–%.0f\t%d\t%s\t%s\t%.3f\t%.3f\t%.3f\t%.3f\t%.3f\t%.3f\t%d\t%.4f\t%.5f\t%.2f\t%.1f\t%s\n",
					level.Level.Name, level.Level.MinimumSizeM, level.Level.MaximumSizeM, run.Rank,
					run.Algorithm.RussianName(), status, run.Score.Overall, run.Score.Coastline,
					run.Score.Bathymetry, run.Score.CellGeometry, run.Score.TargetSizeCompliance,
					run.Score.Efficiency, run.Topology.CellCount, run.Bathymetry.DepthRMSEM,
					run.Bathymetry.WaterVolumeDeviationPercent, run.Resources.GmshDurationSeconds,
					float64(run.Resources.PeakRSSBytes)/(1024*1024), run.Error); err != nil {
					return err
				}
			}
		}
		return writer.Flush()
	}()
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись TSV ADAPT-03 %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие TSV ADAPT-03 %q: %w", path, closeErr)
	}
	return nil
}

func writeComparisonFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога отчёта ADAPT-03: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта ADAPT-03 %q: %w", path, err)
	}
	return nil
}

// NewComparisonReport создаёт заголовок отчёта с фиксированной методикой.
func NewComparisonReport() AdaptiveComparisonReport {
	return AdaptiveComparisonReport{
		SchemaVersion: ComparisonSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Criterion:     DefaultComparisonCriterion(),
		Levels:        []AdaptiveComparisonLevelReport{},
		Reasons:       []string{},
	}
}

func normalizeComparisonReport(report *AdaptiveComparisonReport) {
	if report.Levels == nil {
		report.Levels = []AdaptiveComparisonLevelReport{}
	}
	if report.Reasons == nil {
		report.Reasons = []string{}
	}
	for levelIndex := range report.Levels {
		level := &report.Levels[levelIndex]
		if level.Runs == nil {
			level.Runs = []GeneratorComparisonRun{}
		}
		for runIndex := range level.Runs {
			normalizeComparisonRun(&level.Runs[runIndex])
		}
	}
}

func normalizeComparisonRun(run *GeneratorComparisonRun) {
	if run.Topology.BoundaryTagCounts == nil {
		run.Topology.BoundaryTagCounts = map[int]int{}
	}
	if run.Topology.Reasons == nil {
		run.Topology.Reasons = []string{}
	}
	if run.EdgeZones == nil {
		run.EdgeZones = []mesh.ZoneEdgeStatistics{}
	}
	if run.Bathymetry.Isobaths == nil {
		run.Bathymetry.Isobaths = []IsobathAreaError{}
	}
	if run.Bathymetry.WorstCells == nil {
		run.Bathymetry.WorstCells = []WorstBathymetryCell{}
	}
}
