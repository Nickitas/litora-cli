package bathymetry

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"coastal-geometry/internal/domain/seabed"
)

const (
	reliefPlotLeft   = 48.0
	reliefPlotTop    = 126.0
	reliefPlotWidth  = 1135.0
	reliefPlotHeight = 770.0
)

var profileColours = []string{"#d85f4a", "#236a91", "#66833d"}

// ReliefConfig задаёт происхождение, профильные трассы и исключительно
// визуальный коэффициент вертикального преувеличения 3D-представления.
type ReliefConfig struct {
	Title                string
	Source               string
	SourceChecksum       string
	Metadata             seabed.ExportMetadata
	VerticalExaggeration float64
	ControlPoints        bool
	Profiles             []seabed.Profile
}

// ReliefReport фиксирует масштаб и состав фактически построенного 3D-вида.
type ReliefReport struct {
	NodeCount                    int                 `json:"node_count"`
	CellCount                    int                 `json:"cell_count"`
	ProfileCount                 int                 `json:"profile_count"`
	MaxDepthM                    float64             `json:"max_depth_m"`
	VerticalExaggeration         float64             `json:"vertical_exaggeration"`
	DataCoordinatesUnchanged     bool                `json:"data_coordinates_unchanged"`
	HorizontalAndVerticalUnit    string              `json:"horizontal_and_vertical_unit"`
	ControlPointsShown           bool                `json:"control_points_shown"`
	ControlPointCount            int                 `json:"control_point_count"`
	SamplingMethodCounts         seabed.MethodCounts `json:"sampling_method_counts"`
	InterpolatedSurfaceCellCount int                 `json:"interpolated_surface_cell_count"`
}

type reliefRawPoint struct {
	x float64
	y float64
}

type reliefTransform struct {
	centerX              float64
	centerY              float64
	rotationCos          float64
	rotationSin          float64
	tiltSin              float64
	tiltCos              float64
	verticalExaggeration float64
	minRawX              float64
	minRawY              float64
	scale                float64
	offsetX              float64
	offsetY              float64
}

func (transform reliefTransform) raw(xM, yM, depthM float64) reliefRawPoint {
	dx, dy := xM-transform.centerX, yM-transform.centerY
	u := transform.rotationCos*dx - transform.rotationSin*dy
	v := transform.rotationSin*dx + transform.rotationCos*dy
	return reliefRawPoint{
		x: u,
		y: -v*transform.tiltSin + depthM*transform.verticalExaggeration*transform.tiltCos,
	}
}

func (transform reliefTransform) project(xM, yM, depthM float64) mapPoint {
	raw := transform.raw(xM, yM, depthM)
	return mapPoint{
		x: transform.offsetX + (raw.x-transform.minRawX)*transform.scale,
		y: transform.offsetY + (raw.y-transform.minRawY)*transform.scale,
	}
}

// WriteReliefSVG создаёт псевдотрёхмерную ортографическую проекцию принятой
// поверхности дна. При коэффициенте 1 вертикаль и горизонталь используют один
// метрический масштаб до наклона камеры; иной коэффициент меняет только SVG.
func WriteReliefSVG(path string, model seabed.Model, config ReliefConfig) (ReliefReport, error) {
	if !model.Accepted {
		return ReliefReport{}, fmt.Errorf("3D-рельеф строится только для принятой модели lito-seabed/v1")
	}
	if len(model.Nodes) <= 1 || len(model.Cells) == 0 {
		return ReliefReport{}, fmt.Errorf("модель не содержит узлов и ячеек для 3D-рельефа")
	}
	if strings.TrimSpace(config.Source) == "" {
		return ReliefReport{}, fmt.Errorf("для 3D-рельефа обязателен источник батиметрии")
	}
	if config.Metadata.HorizontalLinearUnit != "m" || config.Metadata.VerticalUnit != "m" || strings.TrimSpace(config.Metadata.VerticalReference) == "" {
		return ReliefReport{}, fmt.Errorf("3D-рельеф требует метрические горизонтальные и вертикальные единицы и вертикальную систему")
	}
	if !finite(config.VerticalExaggeration) || config.VerticalExaggeration < 1 {
		return ReliefReport{}, fmt.Errorf("вертикальное преувеличение должно быть конечным числом не меньше 1")
	}
	if len(config.Profiles) == 0 {
		return ReliefReport{}, fmt.Errorf("для 3D-рельефа не заданы контрольные профили")
	}

	minX, maxX, minY, maxY, maxDepthM, err := modelBounds(model)
	if err != nil {
		return ReliefReport{}, err
	}
	transform := newReliefTransform(model, minX, maxX, minY, maxY, config.VerticalExaggeration)
	report := ReliefReport{
		NodeCount: len(model.Nodes) - 1, CellCount: len(model.Cells), ProfileCount: len(config.Profiles),
		MaxDepthM: maxDepthM, VerticalExaggeration: config.VerticalExaggeration,
		DataCoordinatesUnchanged: true, HorizontalAndVerticalUnit: "m",
		ControlPointsShown: config.ControlPoints, SamplingMethodCounts: model.Sampling.MethodCounts,
		InterpolatedSurfaceCellCount: len(model.Cells),
	}
	if config.ControlPoints {
		report.ControlPointCount = len(model.Nodes) - 1
	}

	type orderedCell struct {
		cell  seabed.Cell
		order float64
	}
	ordered := make([]orderedCell, 0, len(model.Cells))
	for _, cell := range model.Cells {
		order := 0.0
		for _, nodeID := range cell.NodeIDs {
			node := model.Nodes[nodeID]
			depth := 0.0
			if node.WaterDepthM != nil {
				depth = *node.WaterDepthM
			}
			order += transform.raw(node.XM, node.YM, depth).y
		}
		ordered = append(ordered, orderedCell{cell: cell, order: order / 4})
	}
	sort.SliceStable(ordered, func(left, right int) bool {
		if ordered[left].order == ordered[right].order {
			return ordered[left].cell.ID < ordered[right].cell.ID
		}
		return ordered[left].order < ordered[right].order
	})

	var surface strings.Builder
	for _, item := range ordered {
		colour := depthColour(item.cell.WaterDepthMeanM, maxDepthM)
		fmt.Fprintf(&surface, "    <polygon class=\"relief-cell\" data-cell-id=\"%d\" data-depth-m=\"%.3f\" fill=\"%s\" stroke=\"#173c50\" stroke-opacity=\"0.18\" stroke-width=\"0.55\" points=\"", item.cell.ID, item.cell.WaterDepthMeanM, colour)
		for index, nodeID := range item.cell.NodeIDs {
			node := model.Nodes[nodeID]
			depth := 0.0
			if node.WaterDepthM != nil {
				depth = *node.WaterDepthM
			}
			point := transform.project(node.XM, node.YM, depth)
			if index > 0 {
				surface.WriteByte(' ')
			}
			fmt.Fprintf(&surface, "%.2f,%.2f", point.x, point.y)
		}
		surface.WriteString("\"/>\n")
	}

	boundaries := renderReliefBoundaries(model, transform)
	profilePaths := renderReliefProfiles(model, config.Profiles, transform)
	controlPoints := ""
	if config.ControlPoints {
		controlPoints = renderReliefControlPoints(model, transform)
	}
	legend := renderReliefLegend(config, report, maxDepthM)
	axes := renderReliefAxes(minX, maxX, minY, maxY, maxDepthM, transform)

	title := strings.TrimSpace(config.Title)
	if title == "" {
		title = "3D-рельеф дна Чёрного моря"
	}
	exaggerationLabel := fmt.Sprintf("Вертикальное преувеличение ×%.0f", config.VerticalExaggeration)
	if config.VerticalExaggeration == 1 {
		exaggerationLabel = "Вертикальное преувеличение ×1 · метрический режим"
	}
	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="1000" viewBox="0 0 1600 1000" role="img" aria-labelledby="relief-title relief-description" data-vertical-exaggeration="%.6g" data-coordinates-unchanged="true">
  <title id="relief-title">%s</title>
  <desc id="relief-description">Ортографическая 3D-проекция фактической батиметрической поверхности Чёрного моря с контрольными профилями и узловыми выборками глубины.</desc>
  <defs>
    <linearGradient id="relief-depth-scale" x1="0" y1="0" x2="1" y2="0">
%s    </linearGradient>
    <clipPath id="relief-plot"><rect x="48" y="126" width="1135" height="770" rx="12"/></clipPath>
  </defs>
  <rect width="1600" height="1000" fill="#f3f0e8"/>
  <rect x="20" y="20" width="1560" height="960" rx="24" fill="#fbfaf6" stroke="#cfc9bd"/>
  <text x="52" y="60" font-family="Helvetica, Arial, sans-serif" font-size="28" font-weight="700" fill="#172b3a">%s</text>
  <text x="52" y="89" font-family="Helvetica, Arial, sans-serif" font-size="13" fill="#52636f">%s · исходные X/Y/Z не изменены · ортографическая проекция</text>
  <rect x="48" y="126" width="1135" height="770" rx="12" fill="#dce8eb" stroke="#9daeb4"/>
  <g clip-path="url(#relief-plot)" shape-rendering="geometricPrecision">
%s%s%s%s  </g>
%s%s
  <text x="52" y="934" font-family="Helvetica, Arial, sans-serif" font-size="10.5" fill="#52636f">Поверхность построена по узловым глубинам MSH; точки контроля — узлы выборки, а не полный исходный растр GEBCO.</text>
  <text x="52" y="955" font-family="Helvetica, Arial, sans-serif" font-size="10.5" fill="#52636f">Не использовать для навигации или задач безопасности на море.</text>
</svg>
`, config.VerticalExaggeration, escape(title), renderReliefGradient(maxDepthM), escape(title), escape(exaggerationLabel), surface.String(), profilePaths, boundaries, controlPoints, axes, legend)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ReliefReport{}, fmt.Errorf("создание каталога 3D-рельефа: %w", err)
	}
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return ReliefReport{}, fmt.Errorf("сохранение 3D-рельефа %q: %w", path, err)
	}
	return report, nil
}

func newReliefTransform(model seabed.Model, minX, maxX, minY, maxY, exaggeration float64) reliefTransform {
	rotation := 24 * math.Pi / 180
	tilt := 31 * math.Pi / 180
	transform := reliefTransform{
		centerX: (minX + maxX) / 2, centerY: (minY + maxY) / 2,
		rotationCos: math.Cos(rotation), rotationSin: math.Sin(rotation),
		tiltSin: math.Sin(tilt), tiltCos: math.Cos(tilt), verticalExaggeration: exaggeration,
	}
	minRawX, minRawY := math.Inf(1), math.Inf(1)
	maxRawX, maxRawY := math.Inf(-1), math.Inf(-1)
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		depth := 0.0
		if node.WaterDepthM != nil {
			depth = *node.WaterDepthM
		}
		point := transform.raw(node.XM, node.YM, depth)
		minRawX, maxRawX = math.Min(minRawX, point.x), math.Max(maxRawX, point.x)
		minRawY, maxRawY = math.Min(minRawY, point.y), math.Max(maxRawY, point.y)
	}
	padding := 34.0
	transform.minRawX, transform.minRawY = minRawX, minRawY
	transform.scale = math.Min((reliefPlotWidth-2*padding)/(maxRawX-minRawX), (reliefPlotHeight-2*padding)/(maxRawY-minRawY))
	contentWidth := (maxRawX - minRawX) * transform.scale
	contentHeight := (maxRawY - minRawY) * transform.scale
	transform.offsetX = reliefPlotLeft + (reliefPlotWidth-contentWidth)/2
	transform.offsetY = reliefPlotTop + (reliefPlotHeight-contentHeight)/2
	return transform
}

func renderReliefBoundaries(model seabed.Model, transform reliefTransform) string {
	var result strings.Builder
	for _, edge := range model.BoundaryEdges {
		if edge.NodeIDs[0] <= 0 || edge.NodeIDs[1] <= 0 || edge.NodeIDs[0] >= len(model.Nodes) || edge.NodeIDs[1] >= len(model.Nodes) {
			continue
		}
		leftNode, rightNode := model.Nodes[edge.NodeIDs[0]], model.Nodes[edge.NodeIDs[1]]
		left := transform.project(leftNode.XM, leftNode.YM, 0)
		right := transform.project(rightNode.XM, rightNode.YM, 0)
		colour, dash := "#172b3a", ""
		if edge.Kind == seabed.BoundaryOpen {
			colour, dash = "#a64634", ` stroke-dasharray="7 5"`
		}
		fmt.Fprintf(&result, "    <line class=\"relief-boundary\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\" stroke-width=\"1.5\"%s/>\n", left.x, left.y, right.x, right.y, colour, dash)
	}
	return result.String()
}

func renderReliefProfiles(model seabed.Model, profiles []seabed.Profile, transform reliefTransform) string {
	var result strings.Builder
	for index, profile := range profiles {
		colour := profileColours[index%len(profileColours)]
		var points strings.Builder
		for pointIndex, nodeID := range profile.NodeIDs {
			if nodeID <= 0 || nodeID >= len(model.Nodes) || model.Nodes[nodeID].WaterDepthM == nil {
				continue
			}
			node := model.Nodes[nodeID]
			point := transform.project(node.XM, node.YM, *node.WaterDepthM)
			if pointIndex > 0 {
				points.WriteByte(' ')
			}
			fmt.Fprintf(&points, "%.2f,%.2f", point.x, point.y)
		}
		fmt.Fprintf(&result, "    <polyline class=\"relief-profile-halo\" data-profile-id=\"%s\" points=\"%s\" fill=\"none\" stroke=\"#fffdf8\" stroke-width=\"5.2\" stroke-linejoin=\"round\" stroke-linecap=\"round\"/>\n", escape(profile.ID), points.String())
		fmt.Fprintf(&result, "    <polyline class=\"relief-profile\" data-profile-id=\"%s\" points=\"%s\" fill=\"none\" stroke=\"%s\" stroke-width=\"2.7\" stroke-linejoin=\"round\" stroke-linecap=\"round\"/>\n", escape(profile.ID), points.String(), colour)
	}
	return result.String()
}

func renderReliefControlPoints(model seabed.Model, transform reliefTransform) string {
	var result strings.Builder
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		if node.WaterDepthM == nil {
			continue
		}
		point := transform.project(node.XM, node.YM, *node.WaterDepthM)
		colour, radius, opacity := "#f9fcfd", 1.15, 0.62
		switch node.SamplingMethod {
		case seabed.SamplingExact:
			colour, radius, opacity = "#36a6b5", 1.8, 0.9
		case seabed.SamplingNearest:
			colour, radius, opacity = "#c34b35", 2.0, 0.92
		case seabed.SamplingCoastlineConstraint:
			colour, radius, opacity = "#172b3a", 1.45, 0.82
		}
		fmt.Fprintf(&result, "    <circle class=\"relief-control-point\" data-node-id=\"%d\" data-sampling-method=\"%s\" cx=\"%.2f\" cy=\"%.2f\" r=\"%.2f\" fill=\"%s\" fill-opacity=\"%.2f\" stroke=\"#ffffff\" stroke-opacity=\"0.35\" stroke-width=\"0.35\"/>\n", node.ID, escape(string(node.SamplingMethod)), point.x, point.y, radius, colour, opacity)
	}
	return result.String()
}

func renderReliefGradient(maxDepthM float64) string {
	var result strings.Builder
	for _, stop := range bathymetryStops(maxDepthM) {
		fmt.Fprintf(&result, "      <stop offset=\"%.3f%%\" stop-color=\"%s\"/>\n", 100*stop.depthM/maxDepthM, rgbHex(stop.red, stop.green, stop.blue))
	}
	return result.String()
}

func renderReliefAxes(minX, maxX, minY, maxY, maxDepthM float64, transform reliefTransform) string {
	origin := transform.project(minX, minY, 0)
	xEnd := transform.project(maxX, minY, 0)
	yEnd := transform.project(minX, maxY, 0)
	zEnd := transform.project(minX, minY, maxDepthM)
	var result strings.Builder
	result.WriteString("  <g class=\"relief-axes\" font-family=\"Helvetica, Arial, sans-serif\">\n")
	fmt.Fprintf(&result, "    <line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#334852\" stroke-width=\"1.2\"/><text x=\"%.2f\" y=\"%.2f\" font-size=\"10\" font-weight=\"700\" fill=\"#334852\">X · LAEA</text>\n", origin.x, origin.y, xEnd.x, xEnd.y, xEnd.x+5, xEnd.y+3)
	fmt.Fprintf(&result, "    <line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#334852\" stroke-width=\"1.2\"/><text x=\"%.2f\" y=\"%.2f\" font-size=\"10\" font-weight=\"700\" fill=\"#334852\">Y · LAEA</text>\n", origin.x, origin.y, yEnd.x, yEnd.y, yEnd.x-7, yEnd.y-5)
	fmt.Fprintf(&result, "    <line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#334852\" stroke-width=\"1.2\"/>\n", origin.x, origin.y, zEnd.x, zEnd.y)
	depthTicks := []float64{0}
	depthTicks = append(depthTicks, normalizeIsobaths([]float64{500, 1000, 1500, 2000, maxDepthM}, maxDepthM+1)...)
	for _, depthM := range depthTicks {
		point := transform.project(minX, minY, depthM)
		label := fmt.Sprintf("−%.0f м", depthM)
		if depthM == 0 {
			label = "0 м"
		}
		fmt.Fprintf(&result, "    <line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#334852\"/><text x=\"%.2f\" y=\"%.2f\" text-anchor=\"end\" font-size=\"9\" fill=\"#52636f\">%s</text>\n", point.x-4, point.y, point.x+4, point.y, point.x-7, point.y+3, label)
	}
	result.WriteString("  </g>\n")
	return result.String()
}

func renderReliefLegend(config ReliefConfig, report ReliefReport, maxDepthM float64) string {
	const x = 1230.0
	var result strings.Builder
	result.WriteString("  <g class=\"relief-legend\" font-family=\"Helvetica, Arial, sans-serif\">\n")
	result.WriteString("    <text x=\"1230\" y=\"142\" font-size=\"15\" font-weight=\"700\" fill=\"#172b3a\">Глубина поверхности</text>\n")
	result.WriteString("    <rect x=\"1230\" y=\"158\" width=\"300\" height=\"18\" rx=\"9\" fill=\"url(#relief-depth-scale)\" stroke=\"#84949c\"/>\n")
	fmt.Fprintf(&result, "    <text x=\"1230\" y=\"194\" font-size=\"10\" fill=\"#52636f\">0 м</text><text x=\"1530\" y=\"194\" text-anchor=\"end\" font-size=\"10\" fill=\"#52636f\">%.0f м</text>\n", maxDepthM)
	result.WriteString("    <text x=\"1230\" y=\"235\" font-size=\"15\" font-weight=\"700\" fill=\"#172b3a\">Вертикаль</text>\n")
	fmt.Fprintf(&result, "    <text x=\"1230\" y=\"264\" font-size=\"22\" font-weight=\"700\" fill=\"#d05b43\">×%.0f</text>\n", report.VerticalExaggeration)
	result.WriteString("    <text x=\"1288\" y=\"262\" font-size=\"10.5\" fill=\"#52636f\">только преобразование SVG</text>\n")
	result.WriteString("    <text x=\"1230\" y=\"285\" font-size=\"10.5\" fill=\"#52636f\">X/Y и исходный Z остаются в метрах</text>\n")
	result.WriteString("    <text x=\"1230\" y=\"326\" font-size=\"15\" font-weight=\"700\" fill=\"#172b3a\">Профильные трассы</text>\n")
	for index, profile := range config.Profiles {
		y := 354 + index*31
		colour := profileColours[index%len(profileColours)]
		fmt.Fprintf(&result, "    <line x1=\"1230\" y1=\"%d\" x2=\"1265\" y2=\"%d\" stroke=\"%s\" stroke-width=\"3\"/><text x=\"1275\" y=\"%d\" font-size=\"10.5\" fill=\"#334852\">%s</text>\n", y, y, colour, y+4, escape(profile.Name))
	}
	if config.ControlPoints {
		result.WriteString("    <text x=\"1230\" y=\"475\" font-size=\"15\" font-weight=\"700\" fill=\"#172b3a\">Контроль узлов</text>\n")
		rows := []struct{ colour, label string }{{"#f9fcfd", "билинейная выборка"}, {"#c34b35", "ближайшая точка"}, {"#172b3a", "береговое условие Z = 0"}}
		for index, row := range rows {
			y := 501 + index*25
			fmt.Fprintf(&result, "    <circle cx=\"1238\" cy=\"%d\" r=\"4\" fill=\"%s\" stroke=\"#52636f\"/><text x=\"1252\" y=\"%d\" font-size=\"10.5\" fill=\"#52636f\">%s</text>\n", y, row.colour, y+4, row.label)
		}
		fmt.Fprintf(&result, "    <text x=\"1230\" y=\"584\" font-size=\"10\" fill=\"#52636f\">Показано узлов: %d</text>\n", report.ControlPointCount)
	}
	result.WriteString("    <text x=\"1230\" y=\"635\" font-size=\"15\" font-weight=\"700\" fill=\"#172b3a\">Контроль модели</text>\n")
	rows := []string{
		fmt.Sprintf("Ячейки поверхности: %d", report.CellCount),
		fmt.Sprintf("Узлы: %d", report.NodeCount),
		fmt.Sprintf("Макс. глубина: %.1f м", report.MaxDepthM),
		"Проекция: LAEA, X/Y в метрах",
		"Отметка: elevation_m, вверх +",
		fmt.Sprintf("Вертикальная система: %s", config.Metadata.VerticalReference),
	}
	for index, row := range rows {
		for lineIndex, line := range wrapText(row, 43) {
			fmt.Fprintf(&result, "    <text x=\"%.0f\" y=\"%d\" font-size=\"10.2\" fill=\"#52636f\">%s</text>\n", x, 660+index*29+lineIndex*13, escape(line))
		}
	}
	for index, line := range wrapText("Источник: "+config.Source, 48) {
		if index > 2 {
			break
		}
		fmt.Fprintf(&result, "    <text x=\"1230\" y=\"%d\" font-size=\"9.3\" fill=\"#52636f\">%s</text>\n", 855+index*13, escape(line))
	}
	if checksum := strings.TrimSpace(config.SourceChecksum); checksum != "" {
		if len(checksum) > 16 {
			checksum = checksum[:16] + "…"
		}
		fmt.Fprintf(&result, "    <text x=\"1230\" y=\"905\" font-size=\"9.3\" fill=\"#52636f\">SHA-256: %s</text>\n", escape(checksum))
	}
	result.WriteString("  </g>\n")
	return result.String()
}
