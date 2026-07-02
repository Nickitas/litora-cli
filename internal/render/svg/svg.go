package svg

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
	"math"
	"os"
	"strings"
)

const (
	canvasWidth       = 1440
	canvasHeight      = 900
	padding           = 56.0
	sidebarWidth      = 320.0
	scaleBarYGap      = 42.0
	defaultStroke     = "#1f6f8b"
	defaultHeaderNote = "SVG содержит исходную полилинию, фрактальные итерации, масштаб и длину по слоям."
)

type Layer struct {
	Label       string
	Points      []geometry.LatLon
	LengthKM    float64
	Stroke      string
	StrokeWidth float64
	Opacity     float64
	DashArray   string
}

type HighlightSegment struct {
	Start       geometry.LatLon
	End         geometry.LatLon
	Stroke      string
	StrokeWidth float64
	Opacity     float64
}

type ChartSeries struct {
	Label     string
	Values    []float64
	Stroke    string
	DashArray string
}

type Chart struct {
	Title  string
	Series []ChartSeries
}

type StatItem struct {
	Label string
	Value string
	Tone  string
}

type StatCard struct {
	Title string
	Items []StatItem
}

type Document struct {
	Title      string
	Subtitle   string
	Layers     []Layer
	Highlights []HighlightSegment
	StatCards  []StatCard
	Charts     []Chart
	Alerts     []string
	Meta       []string
}

func DrawSVG(points []geometry.LatLon, filename, title string) error {
	return DrawDocument(Document{
		Title: title,
		Layers: []Layer{
			{
				Label:       "Исходная полилиния",
				Points:      points,
				LengthKM:    geometry.PolylineLength(points),
				Stroke:      defaultStroke,
				StrokeWidth: 3,
				Opacity:     1,
			},
		},
	}, filename)
}

func DrawDocument(doc Document, filename string) error {
	if len(doc.Layers) == 0 {
		return fmt.Errorf("need at least 1 layer to draw svg")
	}

	allPoints := flattenLayers(doc.Layers)
	if len(allPoints) < 2 {
		return fmt.Errorf("need at least 2 points to draw svg")
	}

	minLat, maxLat, minLon, maxLon := bounds(allPoints)
	lonSpan := maxLon - minLon
	latSpan := maxLat - minLat
	if lonSpan == 0 {
		lonSpan = 1
	}
	if latSpan == 0 {
		latSpan = 1
	}

	plotWidth := float64(canvasWidth) - sidebarWidth - 2*padding
	header, headerBottom := buildHeader(doc.Title, doc.Subtitle, padding, plotWidth)
	plotTopY := headerBottom + 24
	plotHeight := float64(canvasHeight) - plotTopY - padding
	scale := math.Min(plotWidth/lonSpan, plotHeight/latSpan)
	contentWidth := lonSpan * scale
	contentHeight := latSpan * scale
	originX := padding + (plotWidth-contentWidth)/2
	originY := plotTopY + (plotHeight-contentHeight)/2

	var layers strings.Builder
	for _, layer := range doc.Layers {
		polyline := projectPolyline(layer.Points, minLat, minLon, originX, originY, contentHeight, scale)
		layers.WriteString(fmt.Sprintf(
			`    <polyline fill="none" stroke="%s" stroke-width="%.2f" stroke-opacity="%.2f" stroke-linejoin="round" stroke-linecap="round"%s points="%s"/>`+"\n",
			escapeText(layerStroke(layer)),
			layerWidth(layer),
			layerOpacity(layer),
			layerDashAttribute(layer),
			polyline,
		))
	}

	var highlights strings.Builder
	for _, highlight := range doc.Highlights {
		x1 := originX + (highlight.Start.Lon-minLon)*scale
		y1 := originY + contentHeight - (highlight.Start.Lat-minLat)*scale
		x2 := originX + (highlight.End.Lon-minLon)*scale
		y2 := originY + contentHeight - (highlight.End.Lat-minLat)*scale

		highlights.WriteString(fmt.Sprintf(
			`    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="%.2f" stroke-opacity="%.2f" stroke-linecap="round"/>`+"\n",
			x1, y1, x2, y2,
			escapeText(highlightStroke(highlight.Stroke)),
			highlightWidth(highlight.StrokeWidth),
			highlightOpacity(highlight.Opacity),
		))
		highlights.WriteString(fmt.Sprintf(
			`    <circle cx="%.2f" cy="%.2f" r="3.2" fill="%s" fill-opacity="%.2f"/>`+"\n",
			x1, y1,
			escapeText(highlightStroke(highlight.Stroke)),
			highlightOpacity(highlight.Opacity),
		))
		highlights.WriteString(fmt.Sprintf(
			`    <circle cx="%.2f" cy="%.2f" r="3.2" fill="%s" fill-opacity="%.2f"/>`+"\n",
			x2, y2,
			escapeText(highlightStroke(highlight.Stroke)),
			highlightOpacity(highlight.Opacity),
		))
	}

	sidebarX := padding + plotWidth + 28
	legend, legendBottom := buildLegend(doc.Layers, sidebarX, plotTopY+10, sidebarWidth-56)

	statCards, statCardsBottom := buildStatCards(doc.StatCards, sidebarX, legendBottom+20, sidebarWidth-56)
	charts, chartsBottom := buildCharts(doc.Charts, sidebarX, statCardsBottom+18, sidebarWidth-56)
	alerts, alertsBottom := buildAlerts(doc.Alerts, sidebarX, chartsBottom+18, sidebarWidth-56)
	metaStartY := math.Max(608.0, alertsBottom+26)
	meta, metaBottom := buildMetaCard(doc.Meta, sidebarX, metaStartY, sidebarWidth-56)

	documentHeight := canvasHeight
	sidebarBottom := max(max(legendBottom, statCardsBottom), max(chartsBottom, alertsBottom))
	sidebarBottom = max(sidebarBottom, metaBottom)
	requiredHeight := int(math.Ceil(sidebarBottom + padding))
	if requiredHeight > documentHeight {
		documentHeight = requiredHeight
	}

	scaleBar := buildScaleBar(minLat, maxLat, minLon, maxLon, plotWidth, scale, padding, float64(documentHeight)-padding-scaleBarYGap)

	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
  <rect width="100%%" height="100%%" fill="#f7f4ea"/>
  <rect x="20" y="20" width="%d" height="%d" rx="28" fill="#fcfbf7" stroke="#d6d0c4"/>
  <rect x="%.0f" y="20" width="%.0f" height="%d" rx="24" fill="#f0ece2" stroke="#d6d0c4"/>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
</svg>
`, canvasWidth, documentHeight, canvasWidth, documentHeight,
		canvasWidth-40, documentHeight-40,
		padding+plotWidth+8, sidebarWidth-16, documentHeight-40,
		header,
		layers.String(),
		highlights.String(),
		legend,
		statCards,
		charts,
		alerts,
		meta,
		scaleBar,
	)

	if err := os.WriteFile(filename, []byte(svg), 0o644); err != nil {
		return fmt.Errorf("write svg %q: %w", filename, err)
	}

	return nil
}

func flattenLayers(layers []Layer) []geometry.LatLon {
	total := 0
	for _, layer := range layers {
		total += len(layer.Points)
	}

	points := make([]geometry.LatLon, 0, total)
	for _, layer := range layers {
		points = append(points, layer.Points...)
	}
	return points
}

func projectPolyline(points []geometry.LatLon, minLat, minLon, originX, originY, contentHeight, scale float64) string {
	var polyline strings.Builder
	for i, point := range points {
		x := originX + (point.Lon-minLon)*scale
		y := originY + contentHeight - (point.Lat-minLat)*scale
		if i > 0 {
			polyline.WriteByte(' ')
		}
		polyline.WriteString(fmt.Sprintf("%.2f,%.2f", x, y))
	}
	return polyline.String()
}

func buildHeader(title, subtitle string, x, width float64) (string, float64) {
	titleLines := wrapText(title, estimateCharLimit(width, 30))
	if len(titleLines) == 0 {
		titleLines = []string{title}
	}

	subtitleLines := wrapText(subtitle, estimateCharLimit(width, 14))
	noteLines := wrapText(defaultHeaderNote, estimateCharLimit(width, 13))

	var out strings.Builder
	titleY := 58.0
	for i, line := range titleLines {
		y := titleY + float64(i)*34
		out.WriteString(fmt.Sprintf(
			`  <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="30" font-weight="700" fill="#16324f">%s</text>`+"\n",
			x, y, escapeText(line),
		))
	}

	currentY := titleY + float64(len(titleLines))*34 - 6
	for i, line := range subtitleLines {
		y := currentY + 22 + float64(i)*18
		out.WriteString(fmt.Sprintf(
			`  <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="14" fill="#4f6d7a">%s</text>`+"\n",
			x, y, escapeText(line),
		))
	}

	currentY += 22 + float64(maxInt(len(subtitleLines)-1, 0))*18
	for i, line := range noteLines {
		y := currentY + 26 + float64(i)*16
		out.WriteString(fmt.Sprintf(
			`  <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="13" fill="#6b7a87">%s</text>`+"\n",
			x, y, escapeText(line),
		))
	}

	if len(noteLines) == 0 {
		return out.String(), currentY + 10
	}

	return out.String(), currentY + 26 + float64(len(noteLines)-1)*16
}

func buildLegend(layers []Layer, x, titleY, width float64) (string, float64) {
	if len(layers) == 0 {
		return "", titleY
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf(
		`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="18" font-weight="700" fill="#16324f">Слои и длины</text>`+"\n",
		x, titleY,
	))

	currentY := titleY + 26
	labelLimit := estimateCharLimit(width-58, 13)
	for idx, layer := range layers {
		labelLines := wrapText(layer.Label, labelLimit)
		if len(labelLines) == 0 {
			labelLines = []string{layer.Label}
		}

		rowTop := currentY
		swatchY := rowTop + 12
		out.WriteString(fmt.Sprintf(
			`    <line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="%s" stroke-width="%.2f" stroke-opacity="%.2f"%s/>`+"\n",
			x,
			swatchY,
			x+34,
			swatchY,
			escapeText(layerStroke(layer)),
			layerWidth(layer),
			layerOpacity(layer),
			layerDashAttribute(layer),
		))

		labelX := x + 46
		for i, line := range labelLines {
			lineY := rowTop + 10 + float64(i)*15
			out.WriteString(fmt.Sprintf(
				`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="13" fill="#16324f">%s</text>`+"\n",
				labelX, lineY, escapeText(line),
			))
		}

		lengthY := rowTop + 12 + float64(len(labelLines))*15
		out.WriteString(fmt.Sprintf(
			`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#6b7a87">%.0f км</text>`+"\n",
			labelX, lengthY, layer.LengthKM,
		))

		rowBottom := lengthY + 10
		if idx < len(layers)-1 {
			out.WriteString(fmt.Sprintf(
				`    <line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#ddd6c8" stroke-width="1"/>`+"\n",
				x, rowBottom+4, x+width, rowBottom+4,
			))
		}
		currentY = rowBottom + 16
	}

	return out.String(), currentY - 4
}

func buildMetaCard(lines []string, x, y, width float64) (string, float64) {
	if len(lines) == 0 {
		return "", y
	}

	wrappedEntries := make([][]string, 0, len(lines))
	totalLineCount := 0
	lineLimit := estimateCharLimit(width-28, 12)
	for _, line := range lines {
		wrapped := wrapText(line, lineLimit)
		if len(wrapped) == 0 {
			continue
		}
		wrappedEntries = append(wrappedEntries, wrapped)
		totalLineCount += len(wrapped)
	}
	if len(wrappedEntries) == 0 {
		return "", y
	}

	height := 42.0 + float64(totalLineCount)*16 + float64(len(wrappedEntries)-1)*12 + 12
	var out strings.Builder
	out.WriteString(fmt.Sprintf(
		`    <rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="18" fill="#f8f6ef" stroke="#d6d0c4"/>`+"\n",
		x, y, width, height,
	))
	out.WriteString(fmt.Sprintf(
		`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="14" font-weight="700" fill="#16324f">Сводка</text>`+"\n",
		x+14, y+20,
	))

	currentY := y + 42
	for entryIndex, entry := range wrappedEntries {
		for lineIndex, line := range entry {
			lineY := currentY + float64(lineIndex)*16
			out.WriteString(fmt.Sprintf(
				`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#4f6d7a">%s</text>`+"\n",
				x+14, lineY, escapeText(line),
			))
		}
		currentY += float64(len(entry)) * 16
		if entryIndex < len(wrappedEntries)-1 {
			out.WriteString(fmt.Sprintf(
				`    <line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#e7e1d5" stroke-width="1"/>`+"\n",
				x+14, currentY+4, x+width-14, currentY+4,
			))
			currentY += 12
		}
	}

	return out.String(), y + height
}

func buildCharts(charts []Chart, x, y, width float64) (string, float64) {
	if len(charts) == 0 {
		return "", y
	}

	height := 126.0
	gap := 18.0
	if len(charts) >= 2 {
		height = 118.0
	}

	var out strings.Builder
	currentY := y
	for _, chart := range charts {
		out.WriteString(buildChart(chart, x, currentY, width, height))
		currentY += height + gap
	}
	return out.String(), currentY - gap
}

func buildStatCards(cards []StatCard, x, y, width float64) (string, float64) {
	if len(cards) == 0 {
		return "", y
	}

	var out strings.Builder
	currentY := y
	gap := 18.0
	rendered := 0

	for _, card := range cards {
		if len(card.Items) == 0 {
			continue
		}

		labelLimit := estimateCharLimit(width-84, 12)
		wrappedLabels := make([][]string, 0, len(card.Items))
		height := 42.0
		for _, item := range card.Items {
			lines := wrapText(item.Label, labelLimit)
			if len(lines) == 0 {
				lines = []string{item.Label}
			}
			wrappedLabels = append(wrappedLabels, lines)
			height += math.Max(float64(len(lines))*14+8, 20)
		}

		out.WriteString(fmt.Sprintf(
			`    <rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="18" fill="#f8f6ef" stroke="#d6d0c4"/>`+"\n",
			x, currentY, width, height,
		))
		out.WriteString(fmt.Sprintf(
			`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="14" font-weight="700" fill="#16324f">%s</text>`+"\n",
			x+14, currentY+20, escapeText(card.Title),
		))

		rowY := currentY + 42
		for i, item := range card.Items {
			labelLines := wrappedLabels[i]
			rowHeight := math.Max(float64(len(labelLines))*14+8, 20)
			if i > 0 {
				out.WriteString(fmt.Sprintf(
					`    <line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#e7e1d5" stroke-width="1"/>`+"\n",
					x+14, rowY-8, x+width-14, rowY-8,
				))
			}
			for lineIndex, line := range labelLines {
				lineY := rowY + 10 + float64(lineIndex)*14
				out.WriteString(fmt.Sprintf(
					`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#4f6d7a">%s</text>`+"\n",
					x+14, lineY, escapeText(line),
				))
			}
			out.WriteString(fmt.Sprintf(
				`    <text x="%.0f" y="%.0f" text-anchor="end" font-family="Helvetica, Arial, sans-serif" font-size="13" font-weight="700" fill="%s">%s</text>`+"\n",
				x+width-14, rowY+10, escapeText(statTone(item.Tone)), escapeText(item.Value),
			))
			rowY += rowHeight
		}

		currentY += height + gap
		rendered++
	}

	if rendered == 0 {
		return "", y
	}

	return out.String(), currentY - gap
}

func buildAlerts(alerts []string, x, y, width float64) (string, float64) {
	if len(alerts) == 0 {
		return "", y
	}

	displayCount := min(len(alerts), 4)
	lineLimit := estimateCharLimit(width-28, 12)
	wrappedAlerts := make([][]string, 0, displayCount)
	lineCount := 0
	for i := 0; i < displayCount; i++ {
		lines := wrapText(alerts[i], lineLimit)
		if len(lines) == 0 {
			continue
		}
		if len(lines) > 2 {
			lines = lines[:2]
		}
		wrappedAlerts = append(wrappedAlerts, lines)
		lineCount += len(lines)
	}
	height := 44.0 + float64(lineCount)*16
	if len(alerts) > displayCount {
		height += 18
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf(
		`    <rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="18" fill="#fff7ed" stroke="#fdba74"/>`+"\n",
		x, y, width, height,
	))
	out.WriteString(fmt.Sprintf(
		`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="14" font-weight="700" fill="#9a3412">Предупреждения</text>`+"\n",
		x+14, y+20,
	))
	currentY := y + 40
	for _, lines := range wrappedAlerts {
		for _, line := range lines {
			out.WriteString(fmt.Sprintf(
				`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#9a3412">%s</text>`+"\n",
				x+14, currentY, escapeText(line),
			))
			currentY += 16
		}
	}
	if len(alerts) > displayCount {
		out.WriteString(fmt.Sprintf(
			`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#c2410c">... ещё %d</text>`+"\n",
			x+14, currentY, len(alerts)-displayCount,
		))
	}
	return out.String(), y + height
}

func buildChart(chart Chart, x, y, width, height float64) string {
	if len(chart.Series) == 0 {
		return ""
	}

	plotX := x + 14
	plotY := y + 44
	plotWidth := width - 28
	plotHeight := height - 66
	seriesLegendY := y + 31

	minValue, maxValue, maxLen, ok := chartBounds(chart.Series)
	if !ok {
		return ""
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf(
		`    <rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="18" fill="#fcfbf7" stroke="#d6d0c4"/>`+"\n",
		x, y, width, height,
	))
	out.WriteString(fmt.Sprintf(
		`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="14" font-weight="700" fill="#16324f">%s</text>`+"\n",
		x+14, y+20, escapeText(chart.Title),
	))

	legendX := x + 14
	for _, series := range chart.Series {
		if !seriesHasValues(series.Values) {
			continue
		}
		out.WriteString(fmt.Sprintf(
			`    <line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="%s" stroke-width="2.2"%s/>`+"\n",
			legendX, seriesLegendY, legendX+16, seriesLegendY,
			escapeText(chartStroke(series.Stroke)),
			chartDashAttribute(series.DashArray),
		))
		out.WriteString(fmt.Sprintf(
			`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#4f6d7a">%s</text>`+"\n",
			legendX+22, seriesLegendY+4, escapeText(series.Label),
		))
		legendX += 98
	}

	for _, gridY := range []float64{plotY, plotY + plotHeight/2, plotY + plotHeight} {
		out.WriteString(fmt.Sprintf(
			`    <line x1="%.0f" y1="%.2f" x2="%.0f" y2="%.2f" stroke="#ddd6c8" stroke-width="1"/>`+"\n",
			plotX, gridY, plotX+plotWidth, gridY,
		))
	}
	out.WriteString(fmt.Sprintf(
		`    <line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#8a9aa6" stroke-width="1.4"/>`+"\n",
		plotX, plotY+plotHeight, plotX+plotWidth, plotY+plotHeight,
	))

	topLabel := formatChartValue(maxValue)
	bottomLabel := formatChartValue(minValue)
	out.WriteString(fmt.Sprintf(
		`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#6b7a87">%s</text>`+"\n",
		plotX, plotY-6, escapeText(topLabel),
	))
	out.WriteString(fmt.Sprintf(
		`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#6b7a87">%s</text>`+"\n",
		plotX, plotY+plotHeight+14, escapeText(bottomLabel),
	))
	out.WriteString(fmt.Sprintf(
		`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#6b7a87">0</text>`+"\n",
		plotX, y+height-10,
	))
	out.WriteString(fmt.Sprintf(
		`    <text x="%.0f" y="%.0f" text-anchor="end" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#6b7a87">%d</text>`+"\n",
		plotX+plotWidth, y+height-10, maxInt(maxLen-1, 0),
	))

	for _, series := range chart.Series {
		polyline, points := chartPolyline(series.Values, minValue, maxValue, plotX, plotY, plotWidth, plotHeight)
		if len(points) == 0 {
			continue
		}
		out.WriteString(fmt.Sprintf(
			`    <polyline fill="none" stroke="%s" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"%s points="%s"/>`+"\n",
			escapeText(chartStroke(series.Stroke)),
			chartDashAttribute(series.DashArray),
			polyline,
		))
		for _, point := range points {
			out.WriteString(fmt.Sprintf(
				`    <circle cx="%.2f" cy="%.2f" r="2.8" fill="%s"/>`+"\n",
				point.X, point.Y, escapeText(chartStroke(series.Stroke)),
			))
		}
	}

	return out.String()
}

type chartPoint struct {
	X float64
	Y float64
}

func chartPolyline(values []float64, minValue, maxValue, plotX, plotY, plotWidth, plotHeight float64) (string, []chartPoint) {
	validCount := 0
	for _, value := range values {
		if isFinite(value) {
			validCount++
		}
	}
	if validCount == 0 {
		return "", nil
	}

	var polyline strings.Builder
	points := make([]chartPoint, 0, validCount)
	denominator := maxInt(len(values)-1, 1)
	valueSpan := maxValue - minValue
	if valueSpan <= 0 {
		valueSpan = 1
	}

	for i, value := range values {
		if !isFinite(value) {
			continue
		}
		x := plotX + plotWidth*float64(i)/float64(denominator)
		normalized := (value - minValue) / valueSpan
		y := plotY + plotHeight - normalized*plotHeight
		if polyline.Len() > 0 {
			polyline.WriteByte(' ')
		}
		polyline.WriteString(fmt.Sprintf("%.2f,%.2f", x, y))
		points = append(points, chartPoint{X: x, Y: y})
	}

	return polyline.String(), points
}

func chartBounds(series []ChartSeries) (minValue, maxValue float64, maxLen int, ok bool) {
	for _, line := range series {
		if len(line.Values) > maxLen {
			maxLen = len(line.Values)
		}
		for _, value := range line.Values {
			if !isFinite(value) {
				continue
			}
			if !ok {
				minValue = value
				maxValue = value
				ok = true
				continue
			}
			if value < minValue {
				minValue = value
			}
			if value > maxValue {
				maxValue = value
			}
		}
	}
	if !ok {
		return 0, 0, maxLen, false
	}
	if math.Abs(maxValue-minValue) < 1e-9 {
		padding := 1.0
		if math.Abs(maxValue) > 1 {
			padding = math.Abs(maxValue) * 0.05
		}
		minValue -= padding
		maxValue += padding
		return minValue, maxValue, maxLen, true
	}
	padding := (maxValue - minValue) * 0.08
	return minValue - padding, maxValue + padding, maxLen, true
}

func seriesHasValues(values []float64) bool {
	for _, value := range values {
		if isFinite(value) {
			return true
		}
	}
	return false
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func chartStroke(stroke string) string {
	if stroke == "" {
		return defaultStroke
	}
	return stroke
}

func statTone(tone string) string {
	if tone == "" {
		return "#16324f"
	}
	return tone
}

func chartDashAttribute(dash string) string {
	if dash == "" {
		return ""
	}
	return fmt.Sprintf(` stroke-dasharray="%s"`, escapeText(dash))
}

func formatChartValue(value float64) string {
	abs := math.Abs(value)
	switch {
	case abs >= 1000:
		return fmt.Sprintf("%.0f", value)
	case abs >= 100:
		return fmt.Sprintf("%.1f", value)
	case abs >= 10:
		return fmt.Sprintf("%.2f", value)
	default:
		return fmt.Sprintf("%.3f", value)
	}
}

func estimateCharLimit(width, fontSize float64) int {
	if width <= 0 || fontSize <= 0 {
		return 8
	}

	averageCharWidth := fontSize * 0.58
	limit := int(math.Floor(width / averageCharWidth))
	if limit < 8 {
		return 8
	}
	return limit
}

func wrapText(value string, limit int) []string {
	if limit < 2 {
		return []string{strings.TrimSpace(value)}
	}

	paragraphs := strings.Split(strings.TrimSpace(value), "\n")
	lines := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			continue
		}

		current := ""
		for _, word := range words {
			if current == "" {
				if runeCount(word) <= limit {
					current = word
					continue
				}
				chunks := splitLongWord(word, limit)
				lines = append(lines, chunks[:len(chunks)-1]...)
				current = chunks[len(chunks)-1]
				continue
			}

			if runeCount(current)+1+runeCount(word) <= limit {
				current += " " + word
				continue
			}

			lines = append(lines, current)
			if runeCount(word) <= limit {
				current = word
				continue
			}

			chunks := splitLongWord(word, limit)
			lines = append(lines, chunks[:len(chunks)-1]...)
			current = chunks[len(chunks)-1]
		}

		if current != "" {
			lines = append(lines, current)
		}
	}

	return lines
}

func splitLongWord(value string, limit int) []string {
	runes := []rune(value)
	if len(runes) <= limit {
		return []string{value}
	}

	chunks := make([]string, 0, (len(runes)+limit-1)/limit)
	for start := 0; start < len(runes); start += limit {
		end := min(start+limit, len(runes))
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func runeCount(value string) int {
	return len([]rune(value))
}

func buildScaleBar(minLat, maxLat, minLon, maxLon, plotWidth, scale, x, y float64) string {
	centerLat := (minLat + maxLat) / 2
	centerLon := (minLon + maxLon) / 2
	kmPerLonDegree := geometry.Haversine(
		geometry.LatLon{Lat: centerLat, Lon: centerLon},
		geometry.LatLon{Lat: centerLat, Lon: centerLon + 1},
	)
	if kmPerLonDegree <= 0 || scale <= 0 {
		return ""
	}

	kmPerPixel := kmPerLonDegree / scale
	targetKM := kmPerPixel * plotWidth * 0.22
	scaleKM := niceScaleLength(targetKM)
	barPixels := scaleKM / kmPerPixel

	return fmt.Sprintf(
		`    <line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#16324f" stroke-width="3"/>
    <line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#16324f" stroke-width="6"/>
    <line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#16324f" stroke-width="6"/>
    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="13" fill="#16324f">Масштаб ≈ %.0f км</text>`,
		x, y, x+barPixels, y,
		x, y-7, x, y+7,
		x+barPixels, y-7, x+barPixels, y+7,
		x, y-14, scaleKM,
	)
}

func niceScaleLength(value float64) float64 {
	if value <= 0 {
		return 1
	}

	power := math.Pow(10, math.Floor(math.Log10(value)))
	normalized := value / power

	switch {
	case normalized <= 1:
		return 1 * power
	case normalized <= 2:
		return 2 * power
	case normalized <= 5:
		return 5 * power
	default:
		return 10 * power
	}
}

func layerStroke(layer Layer) string {
	if layer.Stroke == "" {
		return defaultStroke
	}
	return layer.Stroke
}

func layerOpacity(layer Layer) float64 {
	if layer.Opacity <= 0 {
		return 1
	}
	return layer.Opacity
}

func layerWidth(layer Layer) float64 {
	if layer.StrokeWidth <= 0 {
		return 2
	}
	return layer.StrokeWidth
}

func layerDashAttribute(layer Layer) string {
	if layer.DashArray == "" {
		return ""
	}
	return fmt.Sprintf(` stroke-dasharray="%s"`, escapeText(layer.DashArray))
}

func highlightStroke(stroke string) string {
	if stroke == "" {
		return "#c2410c"
	}
	return stroke
}

func highlightWidth(width float64) float64 {
	if width <= 0 {
		return 4.5
	}
	return width
}

func highlightOpacity(opacity float64) float64 {
	if opacity <= 0 {
		return 0.95
	}
	return opacity
}

func bounds(points []geometry.LatLon) (minLat, maxLat, minLon, maxLon float64) {
	minLat, maxLat = points[0].Lat, points[0].Lat
	minLon, maxLon = points[0].Lon, points[0].Lon

	for _, point := range points[1:] {
		if point.Lat < minLat {
			minLat = point.Lat
		}
		if point.Lat > maxLat {
			maxLat = point.Lat
		}
		if point.Lon < minLon {
			minLon = point.Lon
		}
		if point.Lon > maxLon {
			maxLon = point.Lon
		}
	}

	return minLat, maxLat, minLon, maxLon
}

func escapeText(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		`'`, "&apos;",
	)
	return replacer.Replace(value)
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
