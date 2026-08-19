package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"
)

func TestBuildDimensionChartSeparatesScientificAndKochTheory(t *testing.T) {
	dimensions := []*dimensionMetrics{{Valid: true, Dimension: 1.17}}

	scientific := buildDimensionChart(dimensions, false)
	if len(scientific.Series) != 1 {
		t.Fatalf("научный график должен содержать только оценку наблюдений, получено серий: %d", len(scientific.Series))
	}
	if scientific.Series[0].Label != "Оценка" {
		t.Fatalf("неожиданная серия научного графика: %q", scientific.Series[0].Label)
	}

	demo := buildDimensionChart(dimensions, true)
	if len(demo.Series) != 2 || demo.Series[1].Label != "Теория" {
		t.Fatalf("теоретическая линия Коха должна присутствовать только в демонстрационном графике: %#v", demo.Series)
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
	if strings.Contains(string(data), "Koch") || strings.Contains(string(data), "Кох") {
		t.Fatal("научные метрики dimension не должны содержать сравнение с Кохом")
	}

	var metrics fractalSeriesArtifactMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		t.Fatalf("разбор метрик dimension: %v", err)
	}
	if metrics.GeometryKind != "наблюдаемая" {
		t.Fatalf("ожидалась наблюдаемая геометрия, получено %q", metrics.GeometryKind)
	}
	if metrics.OrganicOptions != nil {
		t.Fatal("в научных метриках dimension не должно быть параметров органического генератора")
	}
	if len(metrics.Iterations) != 1 || metrics.Iterations[0].PointsCount != len(points) {
		t.Fatalf("расчёт должен использовать все исходные точки: %#v", metrics.Iterations)
	}
}
