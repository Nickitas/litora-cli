package cli

import (
	"encoding/json"
	"os"
	"testing"

	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"
)

func TestBuildDimensionChartShowsObservedEstimate(t *testing.T) {
	dimensions := []*dimensionMetrics{{Valid: true, Dimension: 1.17}}

	scientific := buildDimensionChart(dimensions)
	if len(scientific.Series) != 1 {
		t.Fatalf("научный график должен содержать только оценку наблюдений, получено серий: %d", len(scientific.Series))
	}
	if scientific.Series[0].Label != "Оценка" {
		t.Fatalf("неожиданная серия научного графика: %q", scientific.Series[0].Label)
	}

}

func TestWriteDimensionSVGMarksObservedGeometry(t *testing.T) {
	points := make([]geometry.LatLon, 0, 80)
	for index := 0; index < 80; index++ {
		points = append(points, geometry.LatLon{
			Lat: 43 + float64(index)*0.002,
			Lon: 39 + float64(index%7)*0.003 + float64(index)*0.001,
		})
	}
	output := NewOutputPathManager(t.TempDir())
	if err := WriteDimensionSVG(points, output, "тест", "наблюдения", coastline.ValidationReport{}); err != nil {
		t.Fatalf("создание научного отчёта dimension: %v", err)
	}

	data, err := os.ReadFile(output.MetricsPath("dimension.metrics.json"))
	if err != nil {
		t.Fatalf("чтение метрик dimension: %v", err)
	}
	var metrics fractalSeriesArtifactMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		t.Fatalf("разбор метрик dimension: %v", err)
	}
	if metrics.GeometryKind != "наблюдаемая" {
		t.Fatalf("ожидалась наблюдаемая геометрия, получено %q", metrics.GeometryKind)
	}
	if len(metrics.Iterations) != 1 || metrics.Iterations[0].PointsCount != len(points) {
		t.Fatalf("расчёт должен использовать все исходные точки: %#v", metrics.Iterations)
	}
}
