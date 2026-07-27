package svg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coastal-geometry/internal/domain/geometry"
)

func TestDrawDocumentIncludesLayersScaleAndLengths(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "fractal.svg")

	err := DrawDocument(Document{
		Title:    "Test Fractal",
		Subtitle: "Очень длинный подзаголовок для проверки аккуратного переноса текста внутри SVG-отчёта",
		Layers: []Layer{
			{
				Label:       "Исходная полилиния с длинной подписью",
				Points:      []geometry.LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 1}},
				LengthKM:    100,
				Stroke:      "#000000",
				StrokeWidth: 2,
				Opacity:     1,
				DashArray:   "6 4",
			},
			{
				Label:       "Итерация 1",
				Points:      []geometry.LatLon{{Lat: 0, Lon: 0}, {Lat: 0.2, Lon: 0.5}, {Lat: 0, Lon: 1}},
				LengthKM:    140,
				Stroke:      "#ff0000",
				StrokeWidth: 3,
				Opacity:     1,
			},
		},
		Charts: []Chart{
			{
				Title: "Длина по итерациям",
				Series: []ChartSeries{
					{Label: "Измерено", Values: []float64{100, 140}, Stroke: "#000000"},
					{Label: "Теория", Values: []float64{100, 133.33}, Stroke: "#ff0000", DashArray: "5 4"},
				},
			},
		},
		Highlights: []HighlightSegment{
			{
				Start:       geometry.LatLon{Lat: 0, Lon: 0.2},
				End:         geometry.LatLon{Lat: 0.2, Lon: 0.5},
				Stroke:      "#c2410c",
				StrokeWidth: 4,
				Opacity:     1,
			},
		},
		StatCards: []StatCard{
			{
				Title: "Контроль геометрии",
				Items: []StatItem{
					{Label: "Сегменты > 450 км", Value: "1", Tone: "#c2410c"},
					{Label: "Автоисправления", Value: "0", Tone: "#3f6b4b"},
				},
			},
		},
		Alerts: []string{"сегмент 1-2: 500 км"},
		Meta:   []string{"Текущая длина: 140 км и дополнительная строка для проверки переноса внутри карточки сводки"},
	}, filename)
	if err != nil {
		t.Fatalf("DrawDocument returned error: %v", err)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read svg: %v", err)
	}

	svg := string(content)
	for _, expected := range []string{
		"Исходная полилиния",
		"Итерация 1",
		"Масштаб",
		"100 км",
		"140 км",
		"stroke-dasharray",
		"Длина по итерациям",
		"Измерено",
		"Теория",
		"Сводка",
		"переноса внутри карточки",
		"Контроль геометрии",
		"Сегменты &gt; 450 км",
		"Автоисправления",
		"Предупреждения",
		"сегмент 1-2: 500 км",
	} {
		if !strings.Contains(svg, expected) {
			t.Fatalf("expected SVG to contain %q", expected)
		}
	}
}

func TestWrapTextRespectsLimit(t *testing.T) {
	lines := wrapText("очень длинная строка для проверки переноса текста", 12)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped output, got %+v", lines)
	}

	for _, line := range lines {
		if runeCount(line) > 12 {
			t.Fatalf("expected each wrapped line to fit limit, got %q", line)
		}
	}
}
