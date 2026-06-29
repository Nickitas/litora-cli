package cli

import (
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/fractal"
	"coastal-geometry/internal/domain/geometry"
	"encoding/json"
	"fmt"
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
	Valid              bool    `json:"valid"`
	Dimension          float64 `json:"dimension,omitempty"`
	RegressionRSquared float64 `json:"regression_r_squared,omitempty"`
	StableAcrossScales bool    `json:"stable_across_scales"`
	StabilitySpread    float64 `json:"stability_spread,omitempty"`
	SampleCount        int     `json:"sample_count"`
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
}

func newExportContext(app *App) exportContext {
	if app == nil {
		return exportContext{}
	}

	return exportContext{
		Command:    app.Config.Command,
		Dataset:    app.Dataset,
		Source:     app.DataSource,
		Validation: app.Validation,
	}
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
	if analysis.Valid {
		result.Dimension = analysis.Dimension
	}
	return result
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
