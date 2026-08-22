package bathymetry

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

const (
	detailPlotLeft   = 390.0
	detailPlotWidth  = 1150.0
	detailPlotHeight = 390.0
)

// MeshDetailsConfig задаёт происхождение и уровни локальных изобат для
// увеличенных фрагментов фактической сетки.
type MeshDetailsConfig struct {
	Title          string
	Source         string
	SourceChecksum string
	Metadata       seabed.ExportMetadata
	IsobathsM      []float64
}

// MeshDetailsReport содержит проверяемую сводку трёх непрореженных фрагментов.
type MeshDetailsReport struct {
	FullMeshCellCount int                  `json:"full_mesh_cell_count"`
	RenderedCellCount int                  `json:"rendered_cell_count"`
	Unthinned         bool                 `json:"unthinned"`
	QualityMetric     string               `json:"quality_metric"`
	Fragments         []MeshFragmentReport `json:"fragments"`
}

// MeshFragmentReport описывает выбор участка, диапазон фактических рёбер,
// глубин и геометрического качества показанных четырёхугольников.
type MeshFragmentReport struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	SelectionMethod    string    `json:"selection_method"`
	CenterLongitudeDeg float64   `json:"center_longitude_deg"`
	CenterLatitudeDeg  float64   `json:"center_latitude_deg"`
	WindowWidthM       float64   `json:"window_width_m"`
	WindowHeightM      float64   `json:"window_height_m"`
	CellCount          int       `json:"cell_count"`
	BoundaryEdgeCount  int       `json:"boundary_edge_count"`
	EdgeMinM           float64   `json:"edge_min_m"`
	EdgeMeanM          float64   `json:"edge_mean_m"`
	EdgeMaxM           float64   `json:"edge_max_m"`
	QualityMin         float64   `json:"quality_min"`
	QualityMean        float64   `json:"quality_mean"`
	QualityMax         float64   `json:"quality_max"`
	DepthMinM          float64   `json:"depth_min_m"`
	DepthMeanM         float64   `json:"depth_mean_m"`
	DepthMaxM          float64   `json:"depth_max_m"`
	RenderedIsobathsM  []float64 `json:"rendered_isobaths_m"`
}

type detailCell struct {
	cell           seabed.Cell
	center         mapPoint
	longitudeDeg   float64
	latitudeDeg    float64
	meanEdgeM      float64
	quality        float64
	coastCurvature float64
}

type detailSelection struct {
	id     string
	title  string
	method string
	seed   detailCell
}

type detailBounds struct {
	minX float64
	maxX float64
	minY float64
	maxY float64
}

func (bounds detailBounds) intersects(minX, maxX, minY, maxY float64) bool {
	return maxX >= bounds.minX && minX <= bounds.maxX && maxY >= bounds.minY && minY <= bounds.maxY
}

type detailTransform struct {
	bounds detailBounds
	left   float64
	top    float64
}

func (transform detailTransform) project(point mapPoint) mapPoint {
	return mapPoint{
		x: transform.left + (point.x-transform.bounds.minX)*detailPlotWidth/(transform.bounds.maxX-transform.bounds.minX),
		y: transform.top + detailPlotHeight - (point.y-transform.bounds.minY)*detailPlotHeight/(transform.bounds.maxY-transform.bounds.minY),
	}
}

// WriteMeshDetailsSVG строит три воспроизводимо выбранных увеличенных окна.
// Внутри каждого окна выводятся все пересекающие его ячейки без выборки или
// прореживания, поэтому форма и размер рёбер соответствуют входному MSH.
func WriteMeshDetailsSVG(path string, model seabed.Model, config MeshDetailsConfig) (MeshDetailsReport, error) {
	if !model.Accepted {
		return MeshDetailsReport{}, fmt.Errorf("фрагменты сетки строятся только для принятой модели lito-seabed/v1")
	}
	if len(model.Nodes) <= 1 || len(model.Cells) == 0 {
		return MeshDetailsReport{}, fmt.Errorf("модель не содержит узлов и ячеек для фрагментов сетки")
	}
	if strings.TrimSpace(config.Source) == "" {
		return MeshDetailsReport{}, fmt.Errorf("для фрагментов сетки обязателен источник батиметрии")
	}
	if strings.TrimSpace(config.Metadata.VerticalReference) == "" || config.Metadata.VerticalUnit != "m" {
		return MeshDetailsReport{}, fmt.Errorf("для фрагментов сетки обязательны вертикальная система и единица «м»")
	}

	_, _, _, _, maxDepthM, err := modelBounds(model)
	if err != nil {
		return MeshDetailsReport{}, err
	}
	cells := buildDetailCells(model)
	if len(cells) < 3 {
		return MeshDetailsReport{}, fmt.Errorf("для трёх фрагментов нужны как минимум три корректные четырёхугольные ячейки")
	}
	selections := selectDetailFragments(model, cells)
	isobaths := normalizeIsobaths(config.IsobathsM, maxDepthM)

	var definitions, panels strings.Builder
	report := MeshDetailsReport{
		FullMeshCellCount: len(model.Cells),
		Unthinned:         true,
		QualityMetric:     "sqrt((min_edge/max_edge)*(1-max_angle_deviation_deg/90))",
		Fragments:         make([]MeshFragmentReport, 0, len(selections)),
	}
	rowTops := []float64{126, 596, 1066}
	for index, selection := range selections {
		bounds, visible := chooseDetailWindow(model, cells, selection.seed)
		transform := detailTransform{bounds: bounds, left: detailPlotLeft, top: rowTops[index] + 28}
		fragment, content := renderDetailFragment(model, visible, selection, bounds, transform, isobaths, index)
		report.Fragments = append(report.Fragments, fragment)
		report.RenderedCellCount += fragment.CellCount
		fmt.Fprintf(&definitions, "    <clipPath id=\"detail-clip-%d\"><rect x=\"%.0f\" y=\"%.0f\" width=\"%.0f\" height=\"%.0f\" rx=\"8\"/></clipPath>\n", index+1, detailPlotLeft, rowTops[index]+28, detailPlotWidth, detailPlotHeight)
		panels.WriteString(content)
	}

	title := strings.TrimSpace(config.Title)
	if title == "" {
		title = "Фактическая сетка Чёрного моря: контрольные фрагменты"
	}
	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="1600" viewBox="0 0 1600 1600" role="img" aria-labelledby="details-title details-description" data-unthinned="true">
  <title id="details-title">%s</title>
  <desc id="details-description">Три увеличенных непрореженных фрагмента фактической четырёхугольной сетки: сложный берег, шельф и крутой склон.</desc>
  <defs>
%s    <linearGradient id="quality-scale" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0%%" stop-color="#c33d3d"/>
      <stop offset="50%%" stop-color="#f0bf4c"/>
      <stop offset="100%%" stop-color="#2f8f68"/>
    </linearGradient>
  </defs>
  <rect width="1600" height="1600" fill="#f3f0e8"/>
  <rect x="20" y="20" width="1560" height="1560" rx="24" fill="#fbfaf6" stroke="#cfc9bd"/>
  <text x="54" y="58" font-family="Helvetica, Arial, sans-serif" font-size="27" font-weight="700" fill="#172b3a">%s</text>
  <text x="54" y="85" font-family="Helvetica, Arial, sans-serif" font-size="13" fill="#52636f">Все пересекающие окно ячейки показаны без прореживания · изобаты рассчитаны по узловым глубинам</text>
  <g class="quality-legend">
    <text x="1070" y="48" font-family="Helvetica, Arial, sans-serif" font-size="12" font-weight="700" fill="#172b3a">Геометрическое качество Q</text>
    <rect x="1070" y="58" width="360" height="14" rx="7" fill="url(#quality-scale)" stroke="#8e8a82"/>
    <text x="1070" y="88" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="#52636f">0 · искажённая</text>
    <text x="1430" y="88" text-anchor="end" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="#52636f">1 · квадрат</text>
  </g>
%s  <text x="54" y="1544" font-family="Helvetica, Arial, sans-serif" font-size="10.5" fill="#52636f">Q = √[(lмин/lмакс) · (1 − max|α−90°|/90°)] · цвет показывает геометрию, а не достоверность батиметрии.</text>
%s</svg>
`, escape(title), definitions.String(), escape(title), panels.String(), renderDetailProvenance(config))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return MeshDetailsReport{}, fmt.Errorf("создание каталога фрагментов сетки: %w", err)
	}
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return MeshDetailsReport{}, fmt.Errorf("сохранение фрагментов сетки %q: %w", path, err)
	}
	return report, nil
}

func buildDetailCells(model seabed.Model) []detailCell {
	curvatures := coastlineNodeCurvatures(model)
	meshNodes := make([]mesh.Point, len(model.Nodes))
	for id := 1; id < len(model.Nodes); id++ {
		meshNodes[id] = mesh.Point{X: model.Nodes[id].XM, Y: model.Nodes[id].YM}
	}
	result := make([]detailCell, 0, len(model.Cells))
	for _, cell := range model.Cells {
		var centerX, centerY, longitude, latitude, edgeSum, curvature float64
		valid := true
		for index, nodeID := range cell.NodeIDs {
			if nodeID <= 0 || nodeID >= len(model.Nodes) {
				valid = false
				break
			}
			node := model.Nodes[nodeID]
			nextID := cell.NodeIDs[(index+1)%4]
			if nextID <= 0 || nextID >= len(model.Nodes) {
				valid = false
				break
			}
			next := model.Nodes[nextID]
			centerX += node.XM
			centerY += node.YM
			longitude += node.LongitudeDeg
			latitude += node.LatitudeDeg
			edgeSum += math.Hypot(next.XM-node.XM, next.YM-node.YM)
			curvature = math.Max(curvature, curvatures[nodeID])
		}
		if !valid {
			continue
		}
		meshCell := mesh.Cell{Nodes: cell.NodeIDs, NodeCount: 4}
		result = append(result, detailCell{
			cell: cell, center: mapPoint{x: centerX / 4, y: centerY / 4},
			longitudeDeg: longitude / 4, latitudeDeg: latitude / 4,
			meanEdgeM: edgeSum / 4, quality: mesh.QuadrilateralQuality(meshNodes, meshCell),
			coastCurvature: curvature,
		})
	}
	return result
}

func coastlineNodeCurvatures(model seabed.Model) map[int]float64 {
	neighbours := make(map[int][]int)
	for _, edge := range model.BoundaryEdges {
		if edge.Kind != seabed.BoundaryCoastline && edge.Kind != seabed.BoundaryIsland {
			continue
		}
		a, b := edge.NodeIDs[0], edge.NodeIDs[1]
		if a <= 0 || b <= 0 || a >= len(model.Nodes) || b >= len(model.Nodes) {
			continue
		}
		neighbours[a] = append(neighbours[a], b)
		neighbours[b] = append(neighbours[b], a)
	}
	result := make(map[int]float64, len(neighbours))
	for nodeID, adjacent := range neighbours {
		origin := model.Nodes[nodeID]
		for left := 0; left < len(adjacent); left++ {
			for right := left + 1; right < len(adjacent); right++ {
				a, b := model.Nodes[adjacent[left]], model.Nodes[adjacent[right]]
				ax, ay := a.XM-origin.XM, a.YM-origin.YM
				bx, by := b.XM-origin.XM, b.YM-origin.YM
				denominator := math.Hypot(ax, ay) * math.Hypot(bx, by)
				if denominator <= 0 {
					continue
				}
				angle := math.Acos(math.Max(-1, math.Min(1, (ax*bx+ay*by)/denominator)))
				result[nodeID] = math.Max(result[nodeID], math.Pi-angle)
			}
		}
	}
	return result
}

func selectDetailFragments(model seabed.Model, cells []detailCell) []detailSelection {
	used := make(map[int]bool)
	coast := bestDetailCell(cells, used, func(cell detailCell) (float64, bool) {
		return cell.coastCurvature, cell.coastCurvature > 0
	})
	coastMethod := "максимальная локальная кривизна береговой границы"
	if coast.cell.ID == 0 {
		coast, coastMethod = fallbackDetailCell(cells, used), "запасной выбор: доступная ячейка с максимальным уклоном"
	}
	used[coast.cell.ID] = true

	thresholds := model.CellDerivation.RegionThresholds
	if thresholds.ShelfMaxDepthM <= thresholds.CoastMaxDepthM {
		thresholds = seabed.DefaultRegionThresholds()
	}
	shelfTarget := (thresholds.CoastMaxDepthM + thresholds.ShelfMaxDepthM) / 2
	shelf := bestShelfDetailCell(cells, used, shelfTarget)
	shelfMethod := fmt.Sprintf("шельфовая глубина, ближайшая к %.0f м", shelfTarget)
	if shelf.cell.ID == 0 {
		shelf, shelfMethod = fallbackDetailCell(cells, used), "запасной выбор: доступная ячейка с максимальным уклоном"
	}
	used[shelf.cell.ID] = true

	slope := bestDetailCell(cells, used, func(cell detailCell) (float64, bool) {
		return cell.cell.SlopeDeg, cell.cell.Region == seabed.RegionSlope
	})
	slopeMethod := "максимальный уклон среди ячеек материкового склона"
	if slope.cell.ID == 0 {
		slope, slopeMethod = fallbackDetailCell(cells, used), "запасной выбор: максимальный уклон доступной ячейки"
	}

	return []detailSelection{
		{id: "complex-coast", title: "1. Сложный берег", method: coastMethod, seed: coast},
		{id: "shelf", title: "2. Шельф", method: shelfMethod, seed: shelf},
		{id: "steep-slope", title: "3. Крутой склон", method: slopeMethod, seed: slope},
	}
}

func bestDetailCell(cells []detailCell, used map[int]bool, score func(detailCell) (float64, bool)) detailCell {
	bestScore := math.Inf(-1)
	var best detailCell
	for _, cell := range cells {
		value, eligible := score(cell)
		if !eligible || used[cell.cell.ID] {
			continue
		}
		if value > bestScore || (value == bestScore && (best.cell.ID == 0 || cell.cell.ID < best.cell.ID)) {
			best, bestScore = cell, value
		}
	}
	return best
}

func bestShelfDetailCell(cells []detailCell, used map[int]bool, targetDepthM float64) detailCell {
	bestDistance, bestSlope := math.Inf(1), math.Inf(1)
	var best detailCell
	for _, cell := range cells {
		if used[cell.cell.ID] || cell.cell.Region != seabed.RegionShelf {
			continue
		}
		distance := math.Abs(cell.cell.WaterDepthMeanM - targetDepthM)
		if distance < bestDistance || (distance == bestDistance && (cell.cell.SlopeDeg < bestSlope || (cell.cell.SlopeDeg == bestSlope && (best.cell.ID == 0 || cell.cell.ID < best.cell.ID)))) {
			best, bestDistance, bestSlope = cell, distance, cell.cell.SlopeDeg
		}
	}
	return best
}

func fallbackDetailCell(cells []detailCell, used map[int]bool) detailCell {
	var best detailCell
	for _, cell := range cells {
		if used[cell.cell.ID] {
			continue
		}
		if best.cell.ID == 0 || cell.cell.SlopeDeg > best.cell.SlopeDeg || (cell.cell.SlopeDeg == best.cell.SlopeDeg && cell.cell.ID < best.cell.ID) {
			best = cell
		}
	}
	return best
}

func chooseDetailWindow(model seabed.Model, cells []detailCell, seed detailCell) (detailBounds, []detailCell) {
	edge := seed.meanEdgeM
	if edge <= 0 {
		edge = 1000
	}
	halfWidth, halfHeight := 9*edge, 3.05*edge
	var bounds detailBounds
	var visible []detailCell
	for attempt := 0; attempt < 8; attempt++ {
		bounds = detailBounds{minX: seed.center.x - halfWidth, maxX: seed.center.x + halfWidth, minY: seed.center.y - halfHeight, maxY: seed.center.y + halfHeight}
		visible = visibleDetailCells(model, cells, bounds)
		if len(visible) < 18 {
			halfWidth *= 1.35
			halfHeight *= 1.35
			continue
		}
		if len(visible) > 360 {
			halfWidth *= 0.82
			halfHeight *= 0.82
			continue
		}
		break
	}
	return bounds, visible
}

func visibleDetailCells(model seabed.Model, cells []detailCell, bounds detailBounds) []detailCell {
	result := make([]detailCell, 0, 160)
	for _, candidate := range cells {
		minX, minY := math.Inf(1), math.Inf(1)
		maxX, maxY := math.Inf(-1), math.Inf(-1)
		valid := true
		for _, nodeID := range candidate.cell.NodeIDs {
			if nodeID <= 0 || nodeID >= len(model.Nodes) {
				valid = false
				break
			}
			node := model.Nodes[nodeID]
			minX, maxX = math.Min(minX, node.XM), math.Max(maxX, node.XM)
			minY, maxY = math.Min(minY, node.YM), math.Max(maxY, node.YM)
		}
		if valid && bounds.intersects(minX, maxX, minY, maxY) {
			result = append(result, candidate)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].cell.ID < result[right].cell.ID })
	return result
}

func renderDetailFragment(model seabed.Model, visible []detailCell, selection detailSelection, bounds detailBounds, transform detailTransform, isobaths []float64, index int) (MeshFragmentReport, string) {
	rowTop := transform.top - 28
	fragment := summarizeDetailFragment(model, visible, selection, bounds, isobaths)
	var cells, contours, contourLabels, boundaries strings.Builder
	for _, item := range visible {
		fmt.Fprintf(&cells, "      <polygon class=\"detail-cell\" data-cell-id=\"%d\" data-quality=\"%.4f\" fill=\"%s\" stroke=\"#263840\" stroke-width=\"0.72\" stroke-linejoin=\"round\" points=\"", item.cell.ID, item.quality, qualityColour(item.quality))
		for nodeIndex, nodeID := range item.cell.NodeIDs {
			node := model.Nodes[nodeID]
			point := transform.project(mapPoint{x: node.XM, y: node.YM})
			if nodeIndex > 0 {
				cells.WriteByte(' ')
			}
			fmt.Fprintf(&cells, "%.2f,%.2f", point.x, point.y)
		}
		cells.WriteString("\"/>\n")
	}

	for _, depthM := range isobaths {
		segments := visibleContourSegments(model, depthM, bounds)
		var longest contourSegment
		longestLength := 0.0
		for _, segment := range segments {
			start, end := transform.project(segment.start), transform.project(segment.end)
			fmt.Fprintf(&contours, "      <line class=\"detail-isobath detail-isobath-halo\" data-depth-m=\"%.0f\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#f8fbff\" stroke-width=\"3.2\" stroke-opacity=\"0.9\"/>\n", depthM, start.x, start.y, end.x, end.y)
			fmt.Fprintf(&contours, "      <line class=\"detail-isobath\" data-depth-m=\"%.0f\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#145f89\" stroke-width=\"1.15\"/>\n", depthM, start.x, start.y, end.x, end.y)
			if length := segmentLength(segment); length > longestLength {
				longest, longestLength = segment, length
			}
		}
		if longestLength > 0 {
			start, end := transform.project(longest.start), transform.project(longest.end)
			x, y := (start.x+end.x)/2, (start.y+end.y)/2
			label := fmt.Sprintf("%.0f м", depthM)
			labelWidth := 6.6*float64(utf8.RuneCountInString(label)) + 8
			fmt.Fprintf(&contourLabels, "      <g class=\"detail-isobath-label\"><rect x=\"%.2f\" y=\"%.2f\" width=\"%.2f\" height=\"15\" rx=\"3\" fill=\"#f8fbff\" fill-opacity=\"0.9\"/><text x=\"%.2f\" y=\"%.2f\" text-anchor=\"middle\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"9.5\" font-weight=\"700\" fill=\"#145f89\">%s</text></g>\n", x-labelWidth/2, y-10, labelWidth, x, y+1, label)
		}
	}

	for _, edge := range model.BoundaryEdges {
		if edge.NodeIDs[0] <= 0 || edge.NodeIDs[1] <= 0 || edge.NodeIDs[0] >= len(model.Nodes) || edge.NodeIDs[1] >= len(model.Nodes) {
			continue
		}
		a, b := model.Nodes[edge.NodeIDs[0]], model.Nodes[edge.NodeIDs[1]]
		if !bounds.intersects(math.Min(a.XM, b.XM), math.Max(a.XM, b.XM), math.Min(a.YM, b.YM), math.Max(a.YM, b.YM)) {
			continue
		}
		left, right := transform.project(mapPoint{x: a.XM, y: a.YM}), transform.project(mapPoint{x: b.XM, y: b.YM})
		class, colour, dash := "detail-coastline", "#10191d", ""
		if edge.Kind == seabed.BoundaryOpen {
			class, colour, dash = "detail-open-boundary", "#9c3d2c", ` stroke-dasharray="7 5"`
		}
		fmt.Fprintf(&boundaries, "      <line class=\"%s\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\" stroke-width=\"2.5\"%s/>\n", class, left.x, left.y, right.x, right.y, colour, dash)
	}

	var panel strings.Builder
	fmt.Fprintf(&panel, "  <g class=\"mesh-detail-panel\" id=\"%s\" data-selection=\"%s\">\n", selection.id, escape(selection.method))
	fmt.Fprintf(&panel, "    <rect x=\"40\" y=\"%.0f\" width=\"1520\" height=\"444\" rx=\"15\" fill=\"#f7f5ef\" stroke=\"#cbc6bb\"/>\n", rowTop)
	fmt.Fprintf(&panel, "    <text x=\"58\" y=\"%.0f\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"21\" font-weight=\"700\" fill=\"#172b3a\">%s</text>\n", rowTop+38, escape(selection.title))
	methodLines := wrapText(selection.method, 40)
	for lineIndex, line := range methodLines {
		if lineIndex > 1 {
			break
		}
		fmt.Fprintf(&panel, "    <text x=\"58\" y=\"%.0f\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11\" fill=\"#52636f\">%s</text>\n", rowTop+61+float64(lineIndex*15), escape(line))
	}
	stats := []string{
		fmt.Sprintf("Центр: %.4f° в.д. · %.4f° с.ш.", fragment.CenterLongitudeDeg, fragment.CenterLatitudeDeg),
		fmt.Sprintf("Ячейки: %d из %d · без прореживания", fragment.CellCount, len(model.Cells)),
		fmt.Sprintf("Рёбра: мин %.0f · сред %.0f · макс %.0f м", fragment.EdgeMinM, fragment.EdgeMeanM, fragment.EdgeMaxM),
		fmt.Sprintf("Глубина ячеек: %.1f–%.1f м · сред %.1f м", fragment.DepthMinM, fragment.DepthMaxM, fragment.DepthMeanM),
		fmt.Sprintf("Качество Q: %.3f–%.3f · сред %.3f", fragment.QualityMin, fragment.QualityMax, fragment.QualityMean),
		fmt.Sprintf("Окно LAEA: %.1f × %.1f км", fragment.WindowWidthM/1000, fragment.WindowHeightM/1000),
	}
	for statIndex, stat := range stats {
		fmt.Fprintf(&panel, "    <text x=\"58\" y=\"%.0f\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11.5\"%s fill=\"#334852\">%s</text>\n", rowTop+116+float64(statIndex*25), boldAttribute(statIndex == 2), escape(stat))
	}
	panel.WriteString(renderDetailKey(rowTop + 292))
	fmt.Fprintf(&panel, "    <rect x=\"%.0f\" y=\"%.0f\" width=\"%.0f\" height=\"%.0f\" rx=\"8\" fill=\"#dbe4e8\" stroke=\"#9caab0\"/>\n", detailPlotLeft, transform.top, detailPlotWidth, detailPlotHeight)
	fmt.Fprintf(&panel, "    <g clip-path=\"url(#detail-clip-%d)\" shape-rendering=\"geometricPrecision\">\n%s%s%s%s    </g>\n", index+1, cells.String(), contours.String(), contourLabels.String(), boundaries.String())
	panel.WriteString(renderLocalScaleBar(bounds, transform))
	panel.WriteString(renderNorthArrow(transform.top))
	panel.WriteString("  </g>\n")
	return fragment, panel.String()
}

func summarizeDetailFragment(model seabed.Model, visible []detailCell, selection detailSelection, bounds detailBounds, isobaths []float64) MeshFragmentReport {
	report := MeshFragmentReport{
		ID: selection.id, Title: selection.title, SelectionMethod: selection.method,
		CenterLongitudeDeg: selection.seed.longitudeDeg, CenterLatitudeDeg: selection.seed.latitudeDeg,
		WindowWidthM: bounds.maxX - bounds.minX, WindowHeightM: bounds.maxY - bounds.minY,
		CellCount: len(visible), EdgeMinM: math.Inf(1), QualityMin: math.Inf(1), DepthMinM: math.Inf(1),
	}
	type edgeKey struct{ a, b int }
	edges := make(map[edgeKey]float64)
	var qualitySum, depthSum float64
	for _, item := range visible {
		report.QualityMin = math.Min(report.QualityMin, item.quality)
		report.QualityMax = math.Max(report.QualityMax, item.quality)
		report.DepthMinM = math.Min(report.DepthMinM, item.cell.WaterDepthMeanM)
		report.DepthMaxM = math.Max(report.DepthMaxM, item.cell.WaterDepthMeanM)
		qualitySum += item.quality
		depthSum += item.cell.WaterDepthMeanM
		for index, nodeID := range item.cell.NodeIDs {
			nextID := item.cell.NodeIDs[(index+1)%4]
			a, b := nodeID, nextID
			if a > b {
				a, b = b, a
			}
			left, right := model.Nodes[a], model.Nodes[b]
			edges[edgeKey{a: a, b: b}] = math.Hypot(right.XM-left.XM, right.YM-left.YM)
		}
	}
	var edgeSum float64
	for _, length := range edges {
		report.EdgeMinM = math.Min(report.EdgeMinM, length)
		report.EdgeMaxM = math.Max(report.EdgeMaxM, length)
		edgeSum += length
	}
	if len(edges) > 0 {
		report.EdgeMeanM = edgeSum / float64(len(edges))
	}
	if len(visible) > 0 {
		report.QualityMean = qualitySum / float64(len(visible))
		report.DepthMeanM = depthSum / float64(len(visible))
	}
	if math.IsInf(report.EdgeMinM, 1) {
		report.EdgeMinM = 0
	}
	if math.IsInf(report.QualityMin, 1) {
		report.QualityMin = 0
	}
	if math.IsInf(report.DepthMinM, 1) {
		report.DepthMinM = 0
	}
	for _, edge := range model.BoundaryEdges {
		if edge.NodeIDs[0] <= 0 || edge.NodeIDs[1] <= 0 || edge.NodeIDs[0] >= len(model.Nodes) || edge.NodeIDs[1] >= len(model.Nodes) {
			continue
		}
		a, b := model.Nodes[edge.NodeIDs[0]], model.Nodes[edge.NodeIDs[1]]
		if bounds.intersects(math.Min(a.XM, b.XM), math.Max(a.XM, b.XM), math.Min(a.YM, b.YM), math.Max(a.YM, b.YM)) {
			report.BoundaryEdgeCount++
		}
	}
	for _, depthM := range isobaths {
		if len(visibleContourSegments(model, depthM, bounds)) > 0 {
			report.RenderedIsobathsM = append(report.RenderedIsobathsM, depthM)
		}
	}
	return report
}

func visibleContourSegments(model seabed.Model, depthM float64, bounds detailBounds) []contourSegment {
	all := buildContourSegments(model, depthM)
	result := make([]contourSegment, 0, len(all)/3)
	for _, segment := range all {
		if bounds.intersects(math.Min(segment.start.x, segment.end.x), math.Max(segment.start.x, segment.end.x), math.Min(segment.start.y, segment.end.y), math.Max(segment.start.y, segment.end.y)) {
			result = append(result, segment)
		}
	}
	return result
}

func qualityColour(quality float64) string {
	quality = math.Max(0, math.Min(1, quality))
	if quality <= 0.5 {
		t := quality / 0.5
		return rgbHex(int(math.Round(195+(240-195)*t)), int(math.Round(61+(191-61)*t)), int(math.Round(61+(76-61)*t)))
	}
	t := (quality - 0.5) / 0.5
	return rgbHex(int(math.Round(240+(47-240)*t)), int(math.Round(191+(143-191)*t)), int(math.Round(76+(104-76)*t)))
}

func renderDetailKey(y float64) string {
	return fmt.Sprintf(`    <g class="detail-key">
      <line x1="58" y1="%.0f" x2="98" y2="%.0f" stroke="#263840" stroke-width="1.2"/><text x="108" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="10.5" fill="#52636f">фактическое ребро</text>
      <line x1="58" y1="%.0f" x2="98" y2="%.0f" stroke="#145f89" stroke-width="2"/><text x="108" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="10.5" fill="#52636f">локальная изобата</text>
      <line x1="58" y1="%.0f" x2="98" y2="%.0f" stroke="#10191d" stroke-width="3"/><text x="108" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="10.5" fill="#52636f">береговая граница</text>
    </g>
`, y, y, y+4, y+25, y+25, y+29, y+50, y+50, y+54)
}

func renderLocalScaleBar(bounds detailBounds, transform detailTransform) string {
	distanceM := niceScaleDistance((bounds.maxX - bounds.minX) / 5)
	barWidth := distanceM * detailPlotWidth / (bounds.maxX - bounds.minX)
	x, y := detailPlotLeft+24, transform.top+detailPlotHeight-24
	label := fmt.Sprintf("%.0f км", distanceM/1000)
	if distanceM < 1000 {
		label = fmt.Sprintf("%.0f м", distanceM)
	}
	return fmt.Sprintf(`    <g class="detail-scale-bar">
      <rect x="%.2f" y="%.2f" width="%.2f" height="25" rx="3" fill="#fbfaf6" fill-opacity="0.88"/>
      <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#172b3a" stroke-width="3"/>
      <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#172b3a"/>
      <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#172b3a"/>
      <text x="%.2f" y="%.2f" text-anchor="middle" font-family="Helvetica, Arial, sans-serif" font-size="10" font-weight="700" fill="#172b3a">%s</text>
    </g>
`, x-8, y-19, barWidth+16, x, y, x+barWidth, y, x, y-5, x, y+5, x+barWidth, y-5, x+barWidth, y+5, x+barWidth/2, y-7, label)
}

func renderNorthArrow(top float64) string {
	x, y := detailPlotLeft+detailPlotWidth-28, top+31
	return fmt.Sprintf(`    <g class="north-arrow">
      <path d="M %.1f %.1f L %.1f %.1f L %.1f %.1f Z" fill="#172b3a"/>
      <text x="%.1f" y="%.1f" text-anchor="middle" font-family="Helvetica, Arial, sans-serif" font-size="10" font-weight="700" fill="#172b3a">С</text>
    </g>
`, x, y-14, x-6, y+2, x+6, y+2, x, y+14)
}

func renderDetailProvenance(config MeshDetailsConfig) string {
	var result strings.Builder
	result.WriteString("  <g class=\"detail-provenance\">\n")
	for index, line := range wrapText("Источник: "+config.Source, 145) {
		if index > 1 {
			break
		}
		fmt.Fprintf(&result, "    <text x=\"650\" y=\"%d\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"9.5\" fill=\"#52636f\">%s</text>\n", 1538+index*13, escape(line))
	}
	if checksum := strings.TrimSpace(config.SourceChecksum); checksum != "" {
		if len(checksum) > 16 {
			checksum = checksum[:16] + "…"
		}
		fmt.Fprintf(&result, "    <text x=\"650\" y=\"1568\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"9.5\" fill=\"#52636f\">SHA-256: %s · вертикальная система: %s</text>\n", escape(checksum), escape(config.Metadata.VerticalReference))
	}
	result.WriteString("  </g>\n")
	return result.String()
}

func boldAttribute(value bool) string {
	if value {
		return ` font-weight="700"`
	}
	return ""
}
