package cli

import (
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/fractal"
	"coastal-geometry/internal/domain/geometry"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type exportContext struct {
	Command    string
	Dataset    string
	Source     string
	Validation coastline.ValidationReport
	Config     config
}

type polylineMetrics struct {
	PointsCount int     `json:"points_count"`
	LengthKM    float64 `json:"length_km"`
}

type simplificationMetrics struct {
	Applied            bool    `json:"applied"`
	PointsBefore       int     `json:"points_before"`
	PointsAfter        int     `json:"points_after"`
	LengthBeforeKM     float64 `json:"length_before_km"`
	LengthAfterKM      float64 `json:"length_after_km"`
	LengthDeltaKM      float64 `json:"length_delta_km"`
	LengthDeltaPercent float64 `json:"length_delta_percent"`
}

type validationMetrics struct {
	Fixes              []string                   `json:"fixes"`
	Warnings           []string                   `json:"warnings"`
	Summary            []validationIssueMetrics   `json:"summary"`
	DuplicateLocations []duplicateLocationMetrics `json:"duplicate_locations"`
}

type validationIssueMetrics struct {
	WarningType string  `json:"warning_type"`
	Count       int     `json:"count"`
	ThresholdKM float64 `json:"threshold_km,omitempty"`
}

type duplicateLocationMetrics struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type coastlineHighlightsMetrics struct {
	LongSegments []segmentHighlightMetrics `json:"long_segments"`
}

const (
	// MetricsSchemaVersion фиксирует формат JSON-метрик для воспроизводимости.
	MetricsSchemaVersion = "1.0"
	// ApplicationVersion указывает состояние приложения без привязки к git-репозиторию.
	ApplicationVersion = "development"
)

// reproducibilityMetrics описывает условия, необходимые для интерпретации результата.
type reproducibilityMetrics struct {
	MetricsSchemaVersion        string  `json:"metrics_schema_version"`
	ApplicationVersion          string  `json:"application_version"`
	InputCoordinateSystem       string  `json:"input_coordinate_system"`
	CalculationCoordinateSystem string  `json:"calculation_coordinate_system"`
	ProjectionReferenceLat      float64 `json:"projection_reference_latitude,omitempty"`
	ProjectionReferenceLon      float64 `json:"projection_reference_longitude,omitempty"`
	MetersPerDegreeLatitude     float64 `json:"meters_per_degree_latitude,omitempty"`
	MetersPerDegreeLongitude    float64 `json:"meters_per_degree_longitude,omitempty"`
	DistanceUnit                string  `json:"distance_unit"`
	GridUnit                    string  `json:"grid_unit"`
	InputGeometrySHA256         string  `json:"input_geometry_sha256"`
	InputPointCount             int     `json:"input_point_count"`
	GridType                    string  `json:"grid_type,omitempty"`
	GridCellSizeMeters          float64 `json:"grid_cell_size_meters,omitempty"`
	GridBufferKM                float64 `json:"grid_buffer_km,omitempty"`
}

type segmentHighlightMetrics struct {
	StartIndex int             `json:"start_index"`
	EndIndex   int             `json:"end_index"`
	LengthKM   float64         `json:"length_km"`
	Start      geometry.LatLon `json:"start"`
	End        geometry.LatLon `json:"end"`
}

type coastlineArtifactMetrics struct {
	GeneratedAt          string                     `json:"generated_at"`
	Command              string                     `json:"command"`
	Dataset              string                     `json:"dataset,omitempty"`
	Source               string                     `json:"source,omitempty"`
	SVGFile              string                     `json:"svg_file"`
	Real                 polylineMetrics            `json:"real"`
	Render               polylineMetrics            `json:"render"`
	RenderSimplification simplificationMetrics      `json:"render_simplification"`
	Highlights           coastlineHighlightsMetrics `json:"highlights"`
	Validation           validationMetrics          `json:"validation"`
}

type fractalSeriesArtifactMetrics struct {
	GeneratedAt         string                     `json:"generated_at"`
	Command             string                     `json:"command"`
	Dataset             string                     `json:"dataset,omitempty"`
	Source              string                     `json:"source,omitempty"`
	Title               string                     `json:"title"`
	OutputDir           string                     `json:"output_dir"`
	ReferenceCoastline  polylineMetrics            `json:"reference_coastline"`
	ReferenceRender     polylineMetrics            `json:"reference_render"`
	ModelBase           polylineMetrics            `json:"model_base"`
	ModelSimplification simplificationMetrics      `json:"model_simplification"`
	ErosionStrength     float64                    `json:"erosion_strength_meters,omitempty"`
	ErosionSeed         int64                      `json:"erosion_seed,omitempty"`
	OrganicOptions      *organicOptionsMetrics     `json:"organic_options,omitempty"`
	Iterations          []fractalIterationMetrics  `json:"iterations"`
	Highlights          coastlineHighlightsMetrics `json:"highlights"`
	Validation          validationMetrics          `json:"validation"`
	Reproducibility     reproducibilityMetrics     `json:"reproducibility"`
}

type organicOptionsMetrics struct {
	Seed            int64   `json:"seed"`
	AngleJitterDeg  float64 `json:"angle_jitter_deg"`
	HeightJitterPct float64 `json:"height_jitter_pct"`
}

type fractalIterationMetrics struct {
	Iteration           int               `json:"iteration"`
	SVGFile             string            `json:"svg_file"`
	PointsCount         int               `json:"points_count"`
	RenderPointsCount   int               `json:"render_points_count"`
	LengthKM            float64           `json:"length_km"`
	RelativeToModelBase float64           `json:"relative_to_model_base"`
	RelativeToReference float64           `json:"relative_to_reference"`
	Theory              *theoryMetrics    `json:"theory,omitempty"`
	Dimension           *dimensionMetrics `json:"dimension,omitempty"`
}

type theoryMetrics struct {
	ExpectedLengthKM float64 `json:"expected_length_km"`
	ErrorKM          float64 `json:"error_km"`
	ErrorPercent     float64 `json:"error_percent"`
}

type dimensionMetrics struct {
	Valid              bool                       `json:"valid"`
	Dimension          float64                    `json:"dimension,omitempty"`
	RegressionRSquared float64                    `json:"regression_r_squared,omitempty"`
	StableAcrossScales bool                       `json:"stable_across_scales"`
	StabilitySpread    float64                    `json:"stability_spread,omitempty"`
	SampleCount        int                        `json:"sample_count"`
	BoxSizeMeters      float64                    `json:"representative_box_size_meters,omitempty"`
	Samples            []boxCountingSampleMetrics `json:"samples,omitempty"`
	GridOffsets        []gridOffsetMetrics        `json:"grid_offsets,omitempty"`
	RegressionStart    int                        `json:"regression_start_sample,omitempty"`
	RegressionEnd      int                        `json:"regression_end_sample,omitempty"`
	RegressionStdError float64                    `json:"regression_standard_error,omitempty"`
	DimensionCI95Low   float64                    `json:"dimension_ci95_low,omitempty"`
	DimensionCI95High  float64                    `json:"dimension_ci95_high,omitempty"`
	LocalDimensions    []float64                  `json:"local_dimensions,omitempty"`
	LogLogSVGFile      string                     `json:"log_log_svg_file,omitempty"`
}

type boxCountingSampleMetrics struct {
	ScaleFactor   float64 `json:"scale_factor"`
	RelativeScale float64 `json:"relative_scale"`
	BoxSizeMeters float64 `json:"box_size_meters"`
	BoxesCovered  int     `json:"boxes_covered"`
	LogInvScale   float64 `json:"log_inverse_scale"`
	LogBoxes      float64 `json:"log_boxes"`
}

type gridOffsetMetrics struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type erosionStepMetrics struct {
	Step         int     `json:"step"`
	SVGFile      string  `json:"svg_file"`
	Points       int     `json:"points"`
	RenderPoints int     `json:"render_points"`
	LengthKM     float64 `json:"length_km"`
	AreaKM       float64 `json:"area_km2"`
}

type erosionSeriesArtifactMetrics struct {
	GeneratedAt         string                     `json:"generated_at"`
	Command             string                     `json:"command"`
	Dataset             string                     `json:"dataset,omitempty"`
	Source              string                     `json:"source,omitempty"`
	OutputDir           string                     `json:"output_dir"`
	ReferenceCoastline  polylineMetrics            `json:"reference_coastline"`
	ReferenceRender     polylineMetrics            `json:"reference_render"`
	ModelBase           polylineMetrics            `json:"model_base"`
	ModelSimplification simplificationMetrics      `json:"model_simplification"`
	ErosionStrength     float64                    `json:"erosion_strength_meters,omitempty"`
	ErosionSeed         int64                      `json:"erosion_seed,omitempty"`
	WaveDirectionDeg    float64                    `json:"wave_direction_deg,omitempty"`
	WindSpeedMS         float64                    `json:"wind_speed_mps,omitempty"`
	FetchSpreadDeg      float64                    `json:"fetch_spread_deg,omitempty"`
	FetchSamples        int                        `json:"fetch_samples,omitempty"`
	MaxFetchKM          float64                    `json:"max_fetch_km,omitempty"`
	DepthScaleMeters    float64                    `json:"depth_scale_meters,omitempty"`
	ExposurePower       float64                    `json:"exposure_power,omitempty"`
	Steps               []erosionStepMetrics       `json:"steps"`
	Highlights          coastlineHighlightsMetrics `json:"highlights"`
	Validation          validationMetrics          `json:"validation"`
	Reproducibility     reproducibilityMetrics     `json:"reproducibility"`
}

func defaultReproducibilityMetrics() reproducibilityMetrics {
	return reproducibilityMetrics{
		MetricsSchemaVersion:        MetricsSchemaVersion,
		ApplicationVersion:          ApplicationVersion,
		InputCoordinateSystem:       "WGS 84 (EPSG:4326)",
		CalculationCoordinateSystem: "локальная равнопромежуточная метрическая проекция WGS 84",
		DistanceUnit:                "м и км",
		GridUnit:                    "м",
	}
}

// reproducibilityForGeometry добавляет к общим метаданным отпечаток входной
// геометрии в каноническом формате координат.
func reproducibilityForGeometry(points []geometry.LatLon) reproducibilityMetrics {
	metrics := defaultReproducibilityMetrics()
	projection := geometry.NewLocalMetricProjection(points)
	metrics.ProjectionReferenceLat = projection.ReferenceLat
	metrics.ProjectionReferenceLon = projection.ReferenceLon
	metrics.MetersPerDegreeLatitude = projection.MetersPerDegreeLatitude
	metrics.MetersPerDegreeLongitude = projection.MetersPerDegreeLongitude
	hash := sha256.New()
	for _, point := range points {
		fmt.Fprintf(hash, "%.12f,%.12f\n", point.Lat, point.Lon)
	}
	metrics.InputGeometrySHA256 = fmt.Sprintf("%x", hash.Sum(nil))
	metrics.InputPointCount = len(points)
	return metrics
}

func newExportContext(command, dataset, source string, validation coastline.ValidationReport) exportContext {
	cfg := config{Command: command}

	// По умолчанию включите расширенный режим для команд измерения и размывания
	// Здесь показаны таблицы подсчета ячеек и размывания для лучшего понимания
	if command == cmdDimension || command == cmdErosion {
		cfg.EnableEnhanced = true
		cfg.ShowGrid = true
		cfg.ShowCompass = true
		cfg.ShowMarkers = true
	}

	return exportContext{
		Command:    command,
		Dataset:    dataset,
		Source:     source,
		Validation: validation,
		Config:     cfg,
	}
}

// Legacy newExportContext для обеспечения обратной совместимости (устарел)
func newExportContextFromApp(command, dataset, source string, validation coastline.ValidationReport) exportContext {
	return newExportContext(command, dataset, source, validation)
}

func summarizePolyline(points []geometry.LatLon) polylineMetrics {
	return polylineMetrics{
		PointsCount: len(points),
		LengthKM:    geometry.PolylineLength(points),
	}
}

func summarizeSimplification(before, after []geometry.LatLon) simplificationMetrics {
	beforeSummary := summarizePolyline(before)
	afterSummary := summarizePolyline(after)
	deltaKM := afterSummary.LengthKM - beforeSummary.LengthKM
	deltaPercent := 0.0
	if beforeSummary.LengthKM > 0 {
		deltaPercent = deltaKM / beforeSummary.LengthKM * 100
	}

	return simplificationMetrics{
		Applied:            len(before) != len(after),
		PointsBefore:       beforeSummary.PointsCount,
		PointsAfter:        afterSummary.PointsCount,
		LengthBeforeKM:     beforeSummary.LengthKM,
		LengthAfterKM:      afterSummary.LengthKM,
		LengthDeltaKM:      deltaKM,
		LengthDeltaPercent: deltaPercent,
	}
}

func metricsPathForSVG(svgPath string) string {
	base := strings.TrimSuffix(svgPath, filepath.Ext(svgPath))
	return base + ".metrics.json"
}

func metricsPathForSeries(outputDir, metricsBaseName string) string {
	return filepath.Join(outputDir, metricsBaseName+".metrics.json")
}

func writeMetricsJSON(filename string, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metrics %q: %w", filename, err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(filename, data, 0o644); err != nil {
		return fmt.Errorf("write metrics %q: %w", filename, err)
	}
	return nil
}

func nowTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func dimensionMetricsFromAnalysis(analysis fractal.BoxCountingAnalysis) *dimensionMetrics {
	if len(analysis.Samples) == 0 && !analysis.Valid {
		return nil
	}

	result := &dimensionMetrics{
		Valid:              analysis.Valid,
		RegressionRSquared: analysis.RegressionRSquared,
		StableAcrossScales: analysis.StableAcrossScales,
		StabilitySpread:    analysis.StabilitySpread,
		SampleCount:        len(analysis.Samples),
	}
	result.Samples = make([]boxCountingSampleMetrics, 0, len(analysis.Samples))
	for _, sample := range analysis.Samples {
		result.Samples = append(result.Samples, boxCountingSampleMetrics{
			ScaleFactor:   sample.ScaleFactor,
			RelativeScale: sample.RelativeScale,
			BoxSizeMeters: sample.BoxSizeMeters,
			BoxesCovered:  sample.BoxesCovered,
			LogInvScale:   sample.LogInvScale,
			LogBoxes:      sample.LogBoxes,
		})
	}
	result.GridOffsets = make([]gridOffsetMetrics, 0, len(analysis.GridOffsets))
	for _, offset := range analysis.GridOffsets {
		result.GridOffsets = append(result.GridOffsets, gridOffsetMetrics{X: offset.X, Y: offset.Y})
	}
	if analysis.RegressionEnd >= analysis.RegressionStart && analysis.RegressionStart >= 0 {
		result.RegressionStart = analysis.RegressionStart
		result.RegressionEnd = analysis.RegressionEnd
		result.LocalDimensions = append([]float64(nil), analysis.LocalDimensions...)
		result.RegressionStdError = regressionStandardError(analysis)
		if result.RegressionStdError > 0 {
			result.DimensionCI95Low = analysis.Dimension - 1.96*result.RegressionStdError
			result.DimensionCI95High = analysis.Dimension + 1.96*result.RegressionStdError
		}
	}
	if len(analysis.Samples) > 0 {
		// Для карты берём средний по списку фактический масштаб анализа.
		result.BoxSizeMeters = analysis.Samples[len(analysis.Samples)/2].BoxSizeMeters
	}
	if analysis.Valid {
		result.Dimension = analysis.Dimension
	}
	return result
}

func regressionStandardError(analysis fractal.BoxCountingAnalysis) float64 {
	start, end := analysis.RegressionStart, analysis.RegressionEnd
	if start < 0 || end <= start || end >= len(analysis.Samples) || analysis.Dimension == 0 {
		return 0
	}

	count := float64(end - start + 1)
	meanX, meanY := 0.0, 0.0
	for _, sample := range analysis.Samples[start : end+1] {
		meanX += sample.LogInvScale
		meanY += sample.LogBoxes
	}
	meanX /= count
	meanY /= count

	sumSquaresX, sumSquaresY := 0.0, 0.0
	for _, sample := range analysis.Samples[start : end+1] {
		deltaX := sample.LogInvScale - meanX
		deltaY := sample.LogBoxes - meanY
		sumSquaresX += deltaX * deltaX
		sumSquaresY += deltaY * deltaY
	}
	if count <= 2 || sumSquaresX <= 0 || sumSquaresY <= 0 || analysis.RegressionRSquared >= 1 {
		return 0
	}
	residualVariance := (1 - analysis.RegressionRSquared) * sumSquaresY / (count - 2)
	return math.Sqrt(residualVariance / sumSquaresX)
}

func validationMetricsFromData(report coastline.ValidationReport, summary coastline.ValidationSummary) validationMetrics {
	issues := make([]validationIssueMetrics, 0, len(summary.Issues))
	for _, issue := range summary.Issues {
		issues = append(issues, validationIssueMetrics{
			WarningType: issue.WarningType,
			Count:       issue.Count,
			ThresholdKM: issue.ThresholdKM,
		})
	}

	duplicates := make([]duplicateLocationMetrics, 0, len(summary.DuplicateLocations))
	for _, duplicate := range summary.DuplicateLocations {
		duplicates = append(duplicates, duplicateLocationMetrics{
			Name:  duplicate.Name,
			Count: duplicate.Count,
		})
	}

	return validationMetrics{
		Fixes:              cloneStrings(report.Fixes),
		Warnings:           cloneStrings(report.Warnings),
		Summary:            issues,
		DuplicateLocations: duplicates,
	}
}

func coastlineHighlightsMetricsFromHints(hints coastline.VisualizationHints) coastlineHighlightsMetrics {
	segments := make([]segmentHighlightMetrics, 0, len(hints.LongSegments))
	for _, segment := range hints.LongSegments {
		segments = append(segments, segmentHighlightMetrics{
			StartIndex: segment.StartIndex,
			EndIndex:   segment.EndIndex,
			LengthKM:   segment.LengthKM,
			Start:      segment.Start,
			End:        segment.End,
		})
	}

	return coastlineHighlightsMetrics{
		LongSegments: segments,
	}
}

func cloneStrings(values []string) []string {
	cloned := make([]string, 0, len(values))
	cloned = append(cloned, values...)
	return cloned
}

// meshQualityMetrics represents TIN mesh quality metrics for JSON export.
type meshQualityMetrics struct {
	TriangleCount int     `json:"triangle_count"`
	VertexCount   int     `json:"vertex_count"`
	MinAngleDeg   float64 `json:"min_angle_deg"`
	MaxAngleDeg   float64 `json:"max_angle_deg"`
	AvgAngleDeg   float64 `json:"avg_angle_deg"`
	QualityScore  string  `json:"quality_score"` // "excellent", "good", "poor"
}

// meshQualityMetricsFromGeometry converts geometry.MeshQuality to export format.
func meshQualityMetricsFromGeometry(quality *geometry.MeshQuality) *meshQualityMetrics {
	if quality == nil {
		return nil
	}

	score := "poor"
	if quality.MinAngle >= 20 && quality.MaxAngle <= 120 {
		score = "excellent"
	} else if quality.MinAngle >= 15 && quality.MaxAngle <= 130 {
		score = "good"
	}

	return &meshQualityMetrics{
		TriangleCount: quality.TriangleCount,
		VertexCount:   quality.VertexCount,
		MinAngleDeg:   quality.MinAngle,
		MaxAngleDeg:   quality.MaxAngle,
		AvgAngleDeg:   quality.AvgAngle,
		QualityScore:  score,
	}
}
