package bathymetry

import (
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"coastal-geometry/internal/domain/seabed"
)

const (
	mapLeft   = 54.0
	mapTop    = 128.0
	mapWidth  = 1170.0
	mapHeight = 760.0
)

// OverviewConfig задаёт проверяемое происхождение и уровни обзорной карты.
// Metadata переносит проекцию и вертикальную систему из экспорта EXPORT-02.
type OverviewConfig struct {
	Title          string
	Source         string
	SourceChecksum string
	Metadata       seabed.ExportMetadata
	IsobathsM      []float64
}

// OverviewReport содержит числовую сводку фактически построенной карты.
type OverviewReport struct {
	NodeCount             int       `json:"node_count"`
	CellCount             int       `json:"cell_count"`
	MaxDepthM             float64   `json:"max_depth_m"`
	NoDataNodePercent     float64   `json:"nodata_node_percent"`
	NoDataCellPercent     float64   `json:"nodata_cell_percent"`
	RenderedIsobathsM     []float64 `json:"rendered_isobaths_m"`
	AverageCellEdgePixels float64   `json:"average_cell_edge_pixels"`
	MeshEdgesDrawn        bool      `json:"mesh_edges_drawn"`
}

type mapTransform struct {
	minX    float64
	minY    float64
	originX float64
	originY float64
	scale   float64
	height  float64
}

func (transform mapTransform) project(point mapPoint) mapPoint {
	return mapPoint{
		x: transform.originX + (point.x-transform.minX)*transform.scale,
		y: transform.originY + transform.height - (point.y-transform.minY)*transform.scale,
	}
}

// WriteOverviewSVG создаёт обзорную карту принятой батиметрической модели.
// Внутренние рёбра намеренно не рисуются независимо от их экранного размера:
// обзор показывает рельеф, а полная геометрия ячеек относится к VIEW-02.
func WriteOverviewSVG(path string, model seabed.Model, config OverviewConfig) (OverviewReport, error) {
	if !model.Accepted {
		return OverviewReport{}, fmt.Errorf("обзорная карта строится только для принятой модели lito-seabed/v1")
	}
	if len(model.Nodes) <= 1 || len(model.Cells) == 0 {
		return OverviewReport{}, fmt.Errorf("модель не содержит узлов и ячеек для обзорной карты")
	}
	if strings.TrimSpace(config.Source) == "" {
		return OverviewReport{}, fmt.Errorf("для карты обязателен источник батиметрии")
	}
	if strings.TrimSpace(config.Metadata.VerticalReference) == "" || config.Metadata.VerticalUnit != "m" {
		return OverviewReport{}, fmt.Errorf("для карты обязательны вертикальная система и единица «м»")
	}

	minX, maxX, minY, maxY, maxDepthM, err := modelBounds(model)
	if err != nil {
		return OverviewReport{}, err
	}
	transform := newMapTransform(minX, maxX, minY, maxY)
	isobaths := normalizeIsobaths(config.IsobathsM, maxDepthM)
	report := buildReport(model, transform.scale, maxDepthM, isobaths)

	var cells strings.Builder
	for _, cell := range model.Cells {
		colour := depthColour(cell.WaterDepthMeanM, maxDepthM)
		fmt.Fprintf(&cells, "    <polygon class=\"bathymetry-cell\" data-depth-m=\"%.3f\" fill=\"%s\" stroke=\"%s\" stroke-width=\"0.65\" stroke-linejoin=\"round\" points=\"", cell.WaterDepthMeanM, colour, colour)
		for index, nodeID := range cell.NodeIDs {
			node := model.Nodes[nodeID]
			point := transform.project(mapPoint{x: node.XM, y: node.YM})
			if index > 0 {
				cells.WriteByte(' ')
			}
			fmt.Fprintf(&cells, "%.2f,%.2f", point.x, point.y)
		}
		cells.WriteString("\"/>\n")
	}

	contours, labels := renderContours(model, transform, isobaths)
	boundaries := renderBoundaries(model, transform)
	legend := renderLegend(maxDepthM, model.CellDerivation.RegionThresholds)
	scaleBar := renderScaleBar(maxX-minX, transform.scale)
	metadata := renderMetadata(config, report)

	title := strings.TrimSpace(config.Title)
	if title == "" {
		title = "Батиметрия Чёрного моря"
	}
	subtitle := fmt.Sprintf("Глубина относительно %s · ячейки без рёбер · изобаты по узловым значениям", config.Metadata.VerticalReference)
	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="1000" viewBox="0 0 1600 1000" role="img" aria-labelledby="map-title map-description">
  <title id="map-title">%s</title>
  <desc id="map-description">Обзорная карта глубин Чёрного моря с числовой легендой, подписанными изобатами и береговой линией.</desc>
  <defs>
    <clipPath id="sea-map"><rect x="54" y="128" width="1170" height="760"/></clipPath>
%s  </defs>
  <rect width="1600" height="1000" fill="#f3f0e8"/>
  <rect x="20" y="20" width="1560" height="960" rx="24" fill="#fbfaf6" stroke="#cfc9bd"/>
  <text x="54" y="62" font-family="Helvetica, Arial, sans-serif" font-size="28" font-weight="700" fill="#172b3a">%s</text>
  <text x="54" y="90" font-family="Helvetica, Arial, sans-serif" font-size="13" fill="#52636f">%s</text>
  <rect x="54" y="128" width="1170" height="760" fill="#e7e1d6" stroke="#b9b2a5"/>
  <g clip-path="url(#sea-map)" shape-rendering="geometricPrecision">
%s%s%s%s  </g>
  <rect x="54" y="128" width="1170" height="760" fill="none" stroke="#a9a195"/>
%s%s%s
  <text x="54" y="930" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#5d6b73">Обзорный масштаб: внутренняя сетка скрыта; фактические четырёхугольники показаны в mesh-details.svg.</text>
  <text x="54" y="952" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#5d6b73">Не использовать для навигации или задач безопасности на море.</text>
</svg>
`, escape(title), renderGradient(maxDepthM), escape(title), escape(subtitle), cells.String(), contours, labels, boundaries, legend, scaleBar, metadata)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return OverviewReport{}, fmt.Errorf("создание каталога обзорной карты: %w", err)
	}
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return OverviewReport{}, fmt.Errorf("сохранение обзорной карты %q: %w", path, err)
	}
	return report, nil
}

func modelBounds(model seabed.Model) (minX, maxX, minY, maxY, maxDepthM float64, err error) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		if !finite(node.XM) || !finite(node.YM) {
			return 0, 0, 0, 0, 0, fmt.Errorf("узел %d содержит некорректные координаты", nodeID)
		}
		if node.WaterDepthM == nil || !finite(*node.WaterDepthM) || *node.WaterDepthM < 0 {
			continue
		}
		minX, maxX = math.Min(minX, node.XM), math.Max(maxX, node.XM)
		minY, maxY = math.Min(minY, node.YM), math.Max(maxY, node.YM)
		maxDepthM = math.Max(maxDepthM, *node.WaterDepthM)
	}
	if !finite(minX) || maxX <= minX || maxY <= minY {
		return 0, 0, 0, 0, 0, fmt.Errorf("модель имеет нулевую или неопределённую область")
	}
	if maxDepthM <= 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("модель не содержит положительных глубин")
	}
	return minX, maxX, minY, maxY, maxDepthM, nil
}

func newMapTransform(minX, maxX, minY, maxY float64) mapTransform {
	spanX, spanY := maxX-minX, maxY-minY
	padding := 10.0
	scale := math.Min((mapWidth-2*padding)/spanX, (mapHeight-2*padding)/spanY)
	contentWidth, contentHeight := spanX*scale, spanY*scale
	return mapTransform{
		minX: minX, minY: minY, scale: scale, height: contentHeight,
		originX: mapLeft + (mapWidth-contentWidth)/2,
		originY: mapTop + (mapHeight-contentHeight)/2,
	}
}

func normalizeIsobaths(values []float64, maxDepthM float64) []float64 {
	if len(values) == 0 {
		values = []float64{20, 50, 100, 200, 500, 1000, 1500, 2000}
	}
	unique := make(map[float64]bool, len(values))
	result := make([]float64, 0, len(values))
	for _, value := range values {
		if !finite(value) || value <= 0 || value >= maxDepthM || unique[value] {
			continue
		}
		unique[value] = true
		result = append(result, value)
	}
	sort.Float64s(result)
	return result
}

func buildReport(model seabed.Model, scale, maxDepthM float64, isobaths []float64) OverviewReport {
	nodeNoData := model.Sampling.NoDataNodeCount
	if model.Sampling.TotalNodeCount == 0 {
		for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
			if model.Nodes[nodeID].WaterDepthM == nil {
				nodeNoData++
			}
		}
	}
	cellNoData := model.CellDerivation.Summary.NoDataCellCount
	totalCells := model.CellDerivation.Summary.TotalCellCount
	if totalCells == 0 {
		totalCells = len(model.Mesh.Cells)
	}
	if totalCells == 0 {
		totalCells = len(model.Cells)
	}

	edgeSum := 0.0
	edgeCount := 0
	for _, cell := range model.Cells {
		for edge := 0; edge < 4; edge++ {
			left := model.Nodes[cell.NodeIDs[edge]]
			right := model.Nodes[cell.NodeIDs[(edge+1)%4]]
			edgeSum += math.Hypot(right.XM-left.XM, right.YM-left.YM) * scale
			edgeCount++
		}
	}
	report := OverviewReport{
		NodeCount: len(model.Nodes) - 1, CellCount: len(model.Cells), MaxDepthM: maxDepthM,
		RenderedIsobathsM: append([]float64(nil), isobaths...), MeshEdgesDrawn: false,
	}
	if len(model.Nodes) > 1 {
		report.NoDataNodePercent = 100 * float64(nodeNoData) / float64(len(model.Nodes)-1)
	}
	if totalCells > 0 {
		report.NoDataCellPercent = 100 * float64(cellNoData) / float64(totalCells)
	}
	if edgeCount > 0 {
		report.AverageCellEdgePixels = edgeSum / float64(edgeCount)
	}
	return report
}

func renderContours(model seabed.Model, transform mapTransform, isobaths []float64) (string, string) {
	var lines, labels strings.Builder
	for _, depthM := range isobaths {
		segments := buildContourSegments(model, depthM)
		var longest contourSegment
		longestLength := 0.0
		for _, segment := range segments {
			start, end := transform.project(segment.start), transform.project(segment.end)
			fmt.Fprintf(&lines, "    <line class=\"isobath\" data-depth-m=\"%.0f\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#f8fbff\" stroke-width=\"0.9\" stroke-opacity=\"0.82\"/>\n", depthM, start.x, start.y, end.x, end.y)
			if length := segmentLength(segment); length > longestLength {
				longest, longestLength = segment, length
			}
		}
		if longestLength <= 0 {
			continue
		}
		start, end := transform.project(longest.start), transform.project(longest.end)
		x, y := (start.x+end.x)/2, (start.y+end.y)/2
		angle := math.Atan2(end.y-start.y, end.x-start.x) * 180 / math.Pi
		if angle > 90 {
			angle -= 180
		}
		if angle < -90 {
			angle += 180
		}
		label := fmt.Sprintf("%.0f м", depthM)
		labelWidth := 7.0*float64(utf8.RuneCountInString(label)) + 8
		fmt.Fprintf(&labels, "    <g class=\"isobath-label\" transform=\"translate(%.2f %.2f) rotate(%.2f)\"><rect x=\"%.2f\" y=\"-9\" width=\"%.2f\" height=\"14\" rx=\"2\" fill=\"#f8fbff\" fill-opacity=\"0.86\"/><text y=\"2\" text-anchor=\"middle\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"10\" font-weight=\"700\" fill=\"#18334b\">%s</text></g>\n", x, y, angle, -labelWidth/2, labelWidth, label)
	}
	return lines.String(), labels.String()
}

func renderBoundaries(model seabed.Model, transform mapTransform) string {
	var result strings.Builder
	for _, edge := range model.BoundaryEdges {
		if edge.NodeIDs[0] <= 0 || edge.NodeIDs[0] >= len(model.Nodes) || edge.NodeIDs[1] <= 0 || edge.NodeIDs[1] >= len(model.Nodes) {
			continue
		}
		leftNode, rightNode := model.Nodes[edge.NodeIDs[0]], model.Nodes[edge.NodeIDs[1]]
		left := transform.project(mapPoint{x: leftNode.XM, y: leftNode.YM})
		right := transform.project(mapPoint{x: rightNode.XM, y: rightNode.YM})
		class, colour, width, dash := "coastline", "#242a2c", 1.7, ""
		switch edge.Kind {
		case seabed.BoundaryIsland:
			class, colour, width = "island-boundary", "#4a4036", 1.5
		case seabed.BoundaryOpen:
			class, colour, width, dash = "open-boundary", "#d95f3d", 1.4, ` stroke-dasharray="7 5"`
		}
		fmt.Fprintf(&result, "    <line class=\"%s\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\" stroke-width=\"%.1f\"%s/>\n", class, left.x, left.y, right.x, right.y, colour, width, dash)
	}
	return result.String()
}

func renderGradient(maxDepthM float64) string {
	var result strings.Builder
	result.WriteString("    <linearGradient id=\"depth-scale\" x1=\"0\" y1=\"0\" x2=\"0\" y2=\"1\">\n")
	for _, stop := range bathymetryStops(maxDepthM) {
		offset := 100 * stop.depthM / maxDepthM
		fmt.Fprintf(&result, "      <stop offset=\"%.3f%%\" stop-color=\"%s\"/>\n", offset, rgbHex(stop.red, stop.green, stop.blue))
	}
	result.WriteString("    </linearGradient>\n")
	return result.String()
}

func renderLegend(maxDepthM float64, thresholds seabed.RegionThresholds) string {
	legendX, legendY, legendHeight := 1292.0, 174.0, 390.0
	ticks := normalizeIsobaths([]float64{0, 20, 200, 500, 1000, 1500, 2000, maxDepthM}, maxDepthM+1)
	if len(ticks) == 0 || ticks[0] != 0 {
		ticks = append([]float64{0}, ticks...)
	}
	if ticks[len(ticks)-1] != maxDepthM {
		ticks = append(ticks, maxDepthM)
	}
	var result strings.Builder
	result.WriteString("  <g class=\"depth-legend\">\n")
	fmt.Fprintf(&result, "    <text x=\"%.0f\" y=\"142\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"15\" font-weight=\"700\" fill=\"#172b3a\">Глубина, м</text>\n", legendX)
	fmt.Fprintf(&result, "    <rect x=\"%.0f\" y=\"%.0f\" width=\"32\" height=\"%.0f\" fill=\"url(#depth-scale)\" stroke=\"#82909a\"/>\n", legendX, legendY, legendHeight)
	for _, tick := range ticks {
		y := legendY + legendHeight*tick/maxDepthM
		fmt.Fprintf(&result, "    <line x1=\"%.0f\" y1=\"%.2f\" x2=\"%.0f\" y2=\"%.2f\" stroke=\"#42535f\"/><text x=\"%.0f\" y=\"%.2f\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11\" fill=\"#334852\">%.0f</text>\n", legendX+32, y, legendX+39, y, legendX+45, y+4, tick)
	}
	result.WriteString("    <text x=\"1292\" y=\"602\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"13\" font-weight=\"700\" fill=\"#172b3a\">Зоны рельефа</text>\n")
	fmt.Fprintf(&result, "    <text x=\"1292\" y=\"625\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11\" fill=\"#52636f\">0–%.0f м · прибрежье</text>\n", thresholds.CoastMaxDepthM)
	fmt.Fprintf(&result, "    <text x=\"1292\" y=\"645\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11\" fill=\"#52636f\">%.0f–%.0f м · шельф</text>\n", thresholds.CoastMaxDepthM, thresholds.ShelfMaxDepthM)
	fmt.Fprintf(&result, "    <text x=\"1292\" y=\"665\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11\" fill=\"#52636f\">%.0f–%.0f м · склон</text>\n", thresholds.ShelfMaxDepthM, thresholds.SlopeMaxDepthM)
	fmt.Fprintf(&result, "    <text x=\"1292\" y=\"685\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11\" fill=\"#52636f\">&gt; %.0f м · котловина</text>\n", thresholds.SlopeMaxDepthM)
	result.WriteString("  </g>\n")
	return result.String()
}

func renderScaleBar(spanXM, scale float64) string {
	distanceM := niceScaleDistance(spanXM / 5)
	barWidth := distanceM * scale
	x, y := mapLeft+28, mapTop+mapHeight-32
	return fmt.Sprintf(`  <g class="scale-bar">
    <rect x="%.2f" y="%.2f" width="%.2f" height="25" rx="3" fill="#fbfaf6" fill-opacity="0.86"/>
    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#172b3a" stroke-width="3"/>
    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#172b3a"/>
    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#172b3a"/>
    <text x="%.2f" y="%.2f" text-anchor="middle" font-family="Helvetica, Arial, sans-serif" font-size="11" font-weight="700" fill="#172b3a">%.0f км</text>
  </g>
`, x-8, y-19, barWidth+16, x, y, x+barWidth, y, x, y-5, x, y+5, x+barWidth, y-5, x+barWidth, y+5, x+barWidth/2, y-7, distanceM/1000)
}

func niceScaleDistance(targetM float64) float64 {
	if targetM <= 0 {
		return 1
	}
	power := math.Pow(10, math.Floor(math.Log10(targetM)))
	for _, multiplier := range []float64{5, 2, 1} {
		candidate := multiplier * power
		if candidate <= targetM {
			return candidate
		}
	}
	return power
}

func renderMetadata(config OverviewConfig, report OverviewReport) string {
	var result strings.Builder
	result.WriteString("  <g class=\"map-metadata\">\n")
	result.WriteString("    <text x=\"1292\" y=\"730\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"13\" font-weight=\"700\" fill=\"#172b3a\">Контроль карты</text>\n")
	rows := []string{
		fmt.Sprintf("Узлы: %d", report.NodeCount),
		fmt.Sprintf("Ячейки: %d", report.CellCount),
		fmt.Sprintf("Макс. глубина: %.1f м", report.MaxDepthM),
		fmt.Sprintf("NoData узлов: %.2f%%", report.NoDataNodePercent),
		fmt.Sprintf("NoData ячеек: %.2f%%", report.NoDataCellPercent),
		"Проекция: LAEA, X/Y в метрах",
	}
	for index, row := range rows {
		fmt.Fprintf(&result, "    <text x=\"1292\" y=\"%d\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11\" fill=\"#52636f\">%s</text>\n", 753+index*19, escape(row))
	}
	result.WriteString("    <text x=\"54\" y=\"908\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11\" font-weight=\"700\" fill=\"#334852\">Источник:</text>\n")
	for index, line := range wrapText(config.Source, 150) {
		fmt.Fprintf(&result, "    <text x=\"112\" y=\"%d\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"10\" fill=\"#52636f\">%s</text>\n", 908+index*13, escape(line))
		if index == 1 {
			break
		}
	}
	if checksum := strings.TrimSpace(config.SourceChecksum); checksum != "" {
		if len(checksum) > 16 {
			checksum = checksum[:16] + "…"
		}
		fmt.Fprintf(&result, "    <text x=\"1040\" y=\"908\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"10\" fill=\"#52636f\">SHA-256: %s</text>\n", escape(checksum))
	}
	result.WriteString("  </g>\n")
	return result.String()
}

func wrapText(value string, limit int) []string {
	words := strings.Fields(value)
	if len(words) == 0 {
		return nil
	}
	lines := []string{words[0]}
	for _, word := range words[1:] {
		last := len(lines) - 1
		if utf8.RuneCountInString(lines[last])+1+utf8.RuneCountInString(word) <= limit {
			lines[last] += " " + word
		} else {
			lines = append(lines, word)
		}
	}
	return lines
}

func escape(value string) string {
	return html.EscapeString(value)
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
