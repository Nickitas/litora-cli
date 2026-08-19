package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"
)

func TestWriteErosionSVGSeriesMarksDemoArtifacts(t *testing.T) {
	points := []geometry.LatLon{{Lat: 43.60, Lon: 39.70}, {Lat: 43.61, Lon: 39.71}, {Lat: 43.62, Lon: 39.72}}
	snapshots := [][]geometry.LatLon{points, append([]geometry.LatLon(nil), points...)}
	output := NewOutputPathManager(t.TempDir())
	classification := geometry.ScenarioClassification{
		ScenarioStatus:   geometry.ScenarioStatusDemo,
		UsageLimitations: []string{"не для публикации"},
	}

	err := WriteErosionSVGSeries(points, snapshots, 1, 0, 0, geometry.WaveErosionOptions{
		WindSourceDirectionDeg: 180,
	}, output, "Сочи", "Open-Meteo Marine", coastline.ValidationReport{}, classification)
	if err != nil {
		t.Fatalf("создание демонстрационных артефактов: %v", err)
	}

	metricsData, err := os.ReadFile(output.MetricsPath("erosion.metrics.json"))
	if err != nil {
		t.Fatalf("чтение метрик: %v", err)
	}
	var metrics erosionSeriesArtifactMetrics
	if err := json.Unmarshal(metricsData, &metrics); err != nil {
		t.Fatalf("разбор метрик: %v", err)
	}
	if metrics.Report.ScenarioStatus != geometry.ScenarioStatusDemo {
		t.Fatalf("статус метрик = %q, требуется demo", metrics.Report.ScenarioStatus)
	}
	if len(metrics.Report.UsageLimitations) != 1 {
		t.Fatalf("ограничения не перенесены в метрики: %+v", metrics.Report)
	}

	svgData, err := os.ReadFile(output.SVGPath("erosion_step_0.svg"))
	if err != nil {
		t.Fatalf("чтение SVG: %v", err)
	}
	hasStatus := strings.Contains(string(svgData), "Статус сценария: demo")
	hasWarning := strings.Contains(string(svgData), "DEMO:")
	if !hasStatus || !hasWarning {
		t.Fatalf("SVG не маркирован как demo: status=%t, warning=%t", hasStatus, hasWarning)
	}
}
