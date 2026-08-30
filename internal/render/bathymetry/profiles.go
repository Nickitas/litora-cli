package bathymetry

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"coastal-geometry/internal/domain/seabed"
)

const (
	profileMapLeft     = 52.0
	profileMapTop      = 142.0
	profileMapWidth    = 500.0
	profileMapHeight   = 360.0
	profileChartLeft   = 620.0
	profileChartWidth  = 900.0
	profileChartHeight = 205.0
)

// ProfilesConfig задаёт происхождение и уже воспроизводимо выбранные трассы
// для карты расположения и метрических разрезов VIEW-03.
type ProfilesConfig struct {
	Title            string
	Source           string
	SourceChecksum   string
	Metadata         seabed.ExportMetadata
	Profiles         []seabed.Profile
	SelectionReports []seabed.ProfileSelectionReport
}

// ProfilesReport описывает профили без вертикального преувеличения и способ
// построения линии между фактическими узловыми выборками.
type ProfilesReport struct {
	ProfileCount         int                             `json:"profile_count"`
	VerticalExaggeration float64                         `json:"vertical_exaggeration"`
	CommonDepthScale     bool                            `json:"common_depth_scale"`
	MaxDepthM            float64                         `json:"max_depth_m"`
	ControlPointsShown   bool                            `json:"control_points_shown"`
	InterpolationMethod  string                          `json:"interpolation_method"`
	Profiles             []seabed.ProfileSelectionReport `json:"profiles"`
}

type profileMapTransform struct {
	minX   float64
	minY   float64
	left   float64
	top    float64
	scale  float64
	height float64
}

func (transform profileMapTransform) project(point mapPoint) mapPoint {
	return mapPoint{
		x: transform.left + (point.x-transform.minX)*transform.scale,
		y: transform.top + transform.height - (point.y-transform.minY)*transform.scale,
	}
}

type profileSample struct {
	distanceM float64
	depthM    float64
	node      seabed.Node
}

// WriteProfilesSVG создаёт карту трасс и три разреза с общей вертикальной
// шкалой. Профили используют исходные метры без вертикального преувеличения;
// между узлами поверхности проводится линейная интерполяция по ребру сетки.
func WriteProfilesSVG(path string, model seabed.Model, config ProfilesConfig) (ProfilesReport, error) {
	if !model.Accepted {
		return ProfilesReport{}, fmt.Errorf("профили строятся только для принятой модели lito-seabed/v1")
	}
	if len(config.Profiles) != 3 || len(config.SelectionReports) != len(config.Profiles) {
		return ProfilesReport{}, fmt.Errorf("для VIEW-03 требуются три профиля и три согласованных отчёта выбора")
	}
	if strings.TrimSpace(config.Source) == "" {
		return ProfilesReport{}, fmt.Errorf("для профилей обязателен источник батиметрии")
	}
	if config.Metadata.HorizontalLinearUnit != "m" || config.Metadata.VerticalUnit != "m" || strings.TrimSpace(config.Metadata.VerticalReference) == "" {
		return ProfilesReport{}, fmt.Errorf("профили требуют метрические единицы и вертикальную систему")
	}
	minX, maxX, minY, maxY, maxDepthM, err := modelBounds(model)
	if err != nil {
		return ProfilesReport{}, err
	}
	mapTransform := newProfileMapTransform(minX, maxX, minY, maxY)
	report := ProfilesReport{
		ProfileCount: len(config.Profiles), VerticalExaggeration: 1,
		CommonDepthScale: true, MaxDepthM: maxDepthM, ControlPointsShown: true,
		InterpolationMethod: "piecewise_linear_along_mesh_edges",
		Profiles:            append([]seabed.ProfileSelectionReport(nil), config.SelectionReports...),
	}

	mapSurface := renderProfileMapSurface(model, mapTransform, maxDepthM)
	mapContours := renderProfileMapContours(model, mapTransform, maxDepthM)
	mapBoundaries := renderProfileMapBoundaries(model, mapTransform)
	mapPaths := renderProfileMapPaths(model, config.Profiles, mapTransform)

	chartTops := []float64{145, 454, 763}
	var charts strings.Builder
	for index, profile := range config.Profiles {
		samples, err := profileSamples(model, profile)
		if err != nil {
			return ProfilesReport{}, err
		}
		charts.WriteString(renderProfileChart(profile, config.SelectionReports[index], samples, maxDepthM, chartTops[index], profileColours[index%len(profileColours)]))
	}

	title := strings.TrimSpace(config.Title)
	if title == "" {
		title = "Профили рельефа дна Чёрного моря"
	}
	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="1100" viewBox="0 0 1600 1100" role="img" aria-labelledby="profiles-title profiles-description" data-vertical-exaggeration="1" data-common-depth-scale="true">
  <title id="profiles-title">%s</title>
  <desc id="profiles-description">Карта трёх трасс от внешнего берега к глубоководному ядру и метрические разрезы с узловыми выборками и линейно интерполированной поверхностью.</desc>
  <defs>
    <clipPath id="profile-map-clip"><rect x="52" y="142" width="500" height="360" rx="10"/></clipPath>
  </defs>
  <rect width="1600" height="1100" fill="#f3f0e8"/>
  <rect x="20" y="20" width="1560" height="1060" rx="24" fill="#fbfaf6" stroke="#cfc9bd"/>
  <text x="52" y="60" font-family="Helvetica, Arial, sans-serif" font-size="28" font-weight="700" fill="#172b3a">%s</text>
  <text x="52" y="89" font-family="Helvetica, Arial, sans-serif" font-size="13" fill="#52636f">Три воспроизводимые трассы по рёбрам сетки · вертикаль ×1 · единая шкала глубины 0–%.0f м</text>
  <text x="52" y="124" font-family="Helvetica, Arial, sans-serif" font-size="15" font-weight="700" fill="#172b3a">Расположение трасс</text>
  <rect x="52" y="142" width="500" height="360" rx="10" fill="#dce8eb" stroke="#9daeb4"/>
  <g clip-path="url(#profile-map-clip)" shape-rendering="geometricPrecision">
%s%s%s%s  </g>
%s%s
  <text x="52" y="1052" font-family="Helvetica, Arial, sans-serif" font-size="10.5" fill="#52636f">Точки — узлы выборки MSH; линия между ними — линейная интерполяция по фактическим рёбрам. Полный исходный растр GEBCO здесь не дублируется.</text>
  <text x="1518" y="1052" text-anchor="end" font-family="Helvetica, Arial, sans-serif" font-size="10.5" fill="#52636f">Не использовать для навигации.</text>
</svg>
`, escape(title), escape(title), maxDepthM, mapSurface, mapContours, mapBoundaries, mapPaths, renderProfileSidebar(config, report), charts.String())

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ProfilesReport{}, fmt.Errorf("создание каталога профилей: %w", err)
	}
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return ProfilesReport{}, fmt.Errorf("сохранение профилей %q: %w", path, err)
	}
	return report, nil
}

func newProfileMapTransform(minX, maxX, minY, maxY float64) profileMapTransform {
	padding := 15.0
	spanX, spanY := maxX-minX, maxY-minY
	scale := math.Min((profileMapWidth-2*padding)/spanX, (profileMapHeight-2*padding)/spanY)
	contentWidth, contentHeight := spanX*scale, spanY*scale
	return profileMapTransform{
		minX: minX, minY: minY, scale: scale, height: contentHeight,
		left: profileMapLeft + (profileMapWidth-contentWidth)/2,
		top:  profileMapTop + (profileMapHeight-contentHeight)/2,
	}
}

func renderProfileMapSurface(model seabed.Model, transform profileMapTransform, maxDepthM float64) string {
	var result strings.Builder
	for _, cell := range model.Cells {
		colour := depthColour(cell.WaterDepthMeanM, maxDepthM)
		fmt.Fprintf(&result, "    <polygon class=\"profile-map-cell\" fill=\"%s\" stroke=\"%s\" stroke-width=\"0.55\" points=\"", colour, colour)
		for index, nodeID := range cell.NodeIDs {
			node := model.Nodes[nodeID]
			point := transform.project(mapPoint{x: node.XM, y: node.YM})
			if index > 0 {
				result.WriteByte(' ')
			}
			fmt.Fprintf(&result, "%.2f,%.2f", point.x, point.y)
		}
		result.WriteString("\"/>\n")
	}
	return result.String()
}

func renderProfileMapContours(model seabed.Model, transform profileMapTransform, maxDepthM float64) string {
	levels := normalizeIsobaths([]float64{200, 1000, 2000}, maxDepthM)
	var result strings.Builder
	for _, depthM := range levels {
		for _, segment := range buildContourSegments(model, depthM) {
			start, end := transform.project(segment.start), transform.project(segment.end)
			fmt.Fprintf(&result, "    <line class=\"profile-map-isobath\" data-depth-m=\"%.0f\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#f8fbff\" stroke-opacity=\"0.7\" stroke-width=\"0.8\"/>\n", depthM, start.x, start.y, end.x, end.y)
		}
	}
	return result.String()
}

func renderProfileMapBoundaries(model seabed.Model, transform profileMapTransform) string {
	var result strings.Builder
	for _, edge := range model.BoundaryEdges {
		if edge.NodeIDs[0] <= 0 || edge.NodeIDs[1] <= 0 || edge.NodeIDs[0] >= len(model.Nodes) || edge.NodeIDs[1] >= len(model.Nodes) {
			continue
		}
		leftNode, rightNode := model.Nodes[edge.NodeIDs[0]], model.Nodes[edge.NodeIDs[1]]
		left := transform.project(mapPoint{x: leftNode.XM, y: leftNode.YM})
		right := transform.project(mapPoint{x: rightNode.XM, y: rightNode.YM})
		fmt.Fprintf(&result, "    <line class=\"profile-map-boundary\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#172b3a\" stroke-width=\"1.2\"/>\n", left.x, left.y, right.x, right.y)
	}
	return result.String()
}

func renderProfileMapPaths(model seabed.Model, profiles []seabed.Profile, transform profileMapTransform) string {
	var result strings.Builder
	for index, profile := range profiles {
		colour := profileColours[index%len(profileColours)]
		var points strings.Builder
		for pointIndex, nodeID := range profile.NodeIDs {
			node := model.Nodes[nodeID]
			point := transform.project(mapPoint{x: node.XM, y: node.YM})
			if pointIndex > 0 {
				points.WriteByte(' ')
			}
			fmt.Fprintf(&points, "%.2f,%.2f", point.x, point.y)
		}
		fmt.Fprintf(&result, "    <polyline class=\"profile-map-path-halo\" points=\"%s\" fill=\"none\" stroke=\"#fffdf8\" stroke-width=\"5\" stroke-linejoin=\"round\"/>\n", points.String())
		fmt.Fprintf(&result, "    <polyline class=\"profile-map-path\" data-profile-id=\"%s\" points=\"%s\" fill=\"none\" stroke=\"%s\" stroke-width=\"2.7\" stroke-linejoin=\"round\"/>\n", escape(profile.ID), points.String(), colour)
		startNode, endNode := model.Nodes[profile.NodeIDs[0]], model.Nodes[profile.NodeIDs[len(profile.NodeIDs)-1]]
		start := transform.project(mapPoint{x: startNode.XM, y: startNode.YM})
		end := transform.project(mapPoint{x: endNode.XM, y: endNode.YM})
		fmt.Fprintf(&result, "    <circle class=\"profile-map-start\" cx=\"%.2f\" cy=\"%.2f\" r=\"4\" fill=\"#fffdf8\" stroke=\"%s\" stroke-width=\"2\"/><circle class=\"profile-map-end\" cx=\"%.2f\" cy=\"%.2f\" r=\"4\" fill=\"%s\" stroke=\"#fffdf8\" stroke-width=\"1.5\"/>\n", start.x, start.y, colour, end.x, end.y, colour)
	}
	return result.String()
}

func profileSamples(model seabed.Model, profile seabed.Profile) ([]profileSample, error) {
	result := make([]profileSample, 0, len(profile.NodeIDs))
	distanceM := 0.0
	for index, nodeID := range profile.NodeIDs {
		if nodeID <= 0 || nodeID >= len(model.Nodes) || model.Nodes[nodeID].WaterDepthM == nil {
			return nil, fmt.Errorf("профиль %q содержит узел %d без глубины", profile.ID, nodeID)
		}
		node := model.Nodes[nodeID]
		if index > 0 {
			previous := model.Nodes[profile.NodeIDs[index-1]]
			distanceM += math.Hypot(node.XM-previous.XM, node.YM-previous.YM)
		}
		result = append(result, profileSample{distanceM: distanceM, depthM: *node.WaterDepthM, node: node})
	}
	return result, nil
}

func renderProfileChart(profile seabed.Profile, selection seabed.ProfileSelectionReport, samples []profileSample, maxDepthM, top float64, colour string) string {
	plotLeft, plotTop := profileChartLeft, top+54
	plotWidth, plotHeight := profileChartWidth, profileChartHeight
	maxDistanceM := samples[len(samples)-1].distanceM
	if maxDistanceM <= 0 {
		maxDistanceM = 1
	}
	project := func(sample profileSample) mapPoint {
		return mapPoint{
			x: plotLeft + sample.distanceM/maxDistanceM*plotWidth,
			y: plotTop + sample.depthM/maxDepthM*plotHeight,
		}
	}
	var linePoints, areaPoints, controls strings.Builder
	fmt.Fprintf(&areaPoints, "%.2f,%.2f ", plotLeft, plotTop)
	for index, sample := range samples {
		point := project(sample)
		if index > 0 {
			linePoints.WriteByte(' ')
		}
		fmt.Fprintf(&linePoints, "%.2f,%.2f", point.x, point.y)
		fmt.Fprintf(&areaPoints, "%.2f,%.2f ", point.x, point.y)
		fill, radius := "#f9fcfd", 3.1
		switch sample.node.SamplingMethod {
		case seabed.SamplingNearest:
			fill, radius = "#c34b35", 3.7
		case seabed.SamplingCoastlineConstraint:
			fill, radius = "#172b3a", 3.5
		case seabed.SamplingExact:
			fill, radius = "#36a6b5", 3.5
		}
		fmt.Fprintf(&controls, "      <circle class=\"profile-control-point\" data-node-id=\"%d\" data-sampling-method=\"%s\" cx=\"%.2f\" cy=\"%.2f\" r=\"%.1f\" fill=\"%s\" stroke=\"%s\" stroke-width=\"1.2\"/>\n", sample.node.ID, escape(string(sample.node.SamplingMethod)), point.x, point.y, radius, fill, colour)
	}
	fmt.Fprintf(&areaPoints, "%.2f,%.2f", plotLeft+plotWidth, plotTop)

	var axes strings.Builder
	for _, depthM := range []float64{0, 500, 1000, 1500, 2000} {
		if depthM > maxDepthM {
			continue
		}
		y := plotTop + depthM/maxDepthM*plotHeight
		fmt.Fprintf(&axes, "      <line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#ccd6d9\" stroke-width=\"0.8\"/><text x=\"%.2f\" y=\"%.2f\" text-anchor=\"end\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"9.5\" fill=\"#52636f\">%.0f</text>\n", plotLeft, y, plotLeft+plotWidth, y, plotLeft-8, y+3, -depthM)
	}
	for tick := 0; tick <= 4; tick++ {
		x := plotLeft + float64(tick)*plotWidth/4
		distanceKM := float64(tick) * maxDistanceM / 4000
		fmt.Fprintf(&axes, "      <line x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#84949c\"/><text x=\"%.2f\" y=\"%.2f\" text-anchor=\"middle\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"9.5\" fill=\"#52636f\">%.0f</text>\n", x, plotTop+plotHeight, x, plotTop+plotHeight+5, x, plotTop+plotHeight+18, distanceKM)
	}

	return fmt.Sprintf(`  <g class="profile-chart" id="%s">
    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="17" font-weight="700" fill="#172b3a">%s</text>
    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="10.5" fill="#52636f">%.1f км · %d узлов · берег 0 м → %.1f м · максимум %.1f м</text>
    <rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="#f7fafb" stroke="#a9b5ba"/>
%s    <polygon class="profile-water-column" points="%s" fill="#d7e8ed" fill-opacity="0.72"/>
    <polyline class="profile-surface-halo" points="%s" fill="none" stroke="#fffdf8" stroke-width="5" stroke-linejoin="round"/>
    <polyline class="profile-surface" data-interpolation="piecewise-linear" points="%s" fill="none" stroke="%s" stroke-width="2.5" stroke-linejoin="round"/>
%s    <text x="%.2f" y="%.2f" transform="rotate(-90 %.2f %.2f)" text-anchor="middle" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="#52636f">Отметка, м</text>
    <text x="%.2f" y="%.2f" text-anchor="middle" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="#52636f">Расстояние от берега, км</text>
  </g>
`, escape(profile.ID), profileChartLeft, top+21, escape(profile.Name), profileChartLeft, top+42, selection.LengthM/1000, selection.PointCount, selection.EndDepthM, selection.MaxDepthM, plotLeft, plotTop, plotWidth, plotHeight, axes.String(), areaPoints.String(), linePoints.String(), linePoints.String(), colour, controls.String(), plotLeft-52, plotTop+plotHeight/2, plotLeft-52, plotTop+plotHeight/2, plotLeft+plotWidth/2, plotTop+plotHeight+37)
}

func renderProfileSidebar(config ProfilesConfig, report ProfilesReport) string {
	var result strings.Builder
	result.WriteString("  <g class=\"profile-sidebar\" font-family=\"Helvetica, Arial, sans-serif\">\n")
	result.WriteString("    <text x=\"52\" y=\"546\" font-size=\"15\" font-weight=\"700\" fill=\"#172b3a\">Как выбраны трассы</text>\n")
	for index, selection := range config.SelectionReports {
		y := 579 + index*86
		colour := profileColours[index%len(profileColours)]
		fmt.Fprintf(&result, "    <line x1=\"52\" y1=\"%d\" x2=\"88\" y2=\"%d\" stroke=\"%s\" stroke-width=\"3\"/><text x=\"98\" y=\"%d\" font-size=\"11.5\" font-weight=\"700\" fill=\"#334852\">%s</text>\n", y, y, colour, y+4, escape(selection.Name))
		for lineIndex, line := range wrapText(selection.SelectionBasis, 54) {
			if lineIndex > 1 {
				break
			}
			fmt.Fprintf(&result, "    <text x=\"98\" y=\"%d\" font-size=\"10\" fill=\"#52636f\">%s</text>\n", y+23+lineIndex*13, escape(line))
		}
		fmt.Fprintf(&result, "    <text x=\"98\" y=\"%d\" font-size=\"9.5\" fill=\"#52636f\">узлы %d → %d · %.1f км</text>\n", y+53, selection.StartNodeID, selection.EndNodeID, selection.LengthM/1000)
	}
	result.WriteString("    <text x=\"52\" y=\"838\" font-size=\"15\" font-weight=\"700\" fill=\"#172b3a\">Контроль построения</text>\n")
	rows := []string{
		"Вертикальное преувеличение: ×1",
		"Горизонталь и вертикаль: метры",
		"Общая шкала глубины для трёх профилей",
		"Линия: линейно между соседними узлами",
		fmt.Sprintf("Вертикальная система: %s", config.Metadata.VerticalReference),
	}
	for index, row := range rows {
		for lineIndex, line := range wrapText(row, 58) {
			fmt.Fprintf(&result, "    <text x=\"52\" y=\"%d\" font-size=\"10.2\" fill=\"#52636f\">%s</text>\n", 865+index*24+lineIndex*12, escape(line))
		}
	}
	for index, line := range wrapText("Источник: "+config.Source, 72) {
		if index > 1 {
			break
		}
		fmt.Fprintf(&result, "    <text x=\"52\" y=\"%d\" font-size=\"9.2\" fill=\"#52636f\">%s</text>\n", 1006+index*13, escape(line))
	}
	if checksum := strings.TrimSpace(config.SourceChecksum); checksum != "" {
		if len(checksum) > 16 {
			checksum = checksum[:16] + "…"
		}
		fmt.Fprintf(&result, "    <text x=\"52\" y=\"1042\" font-size=\"9.2\" fill=\"#52636f\">SHA-256: %s</text>\n", escape(checksum))
	}
	result.WriteString("  </g>\n")
	return result.String()
}
