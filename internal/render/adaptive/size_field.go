package adaptive

import (
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strings"

	fieldmodel "coastal-geometry/internal/domain/adaptive"
	"coastal-geometry/internal/domain/seabed"
)

const (
	mapLeft   = 54.0
	mapTop    = 128.0
	mapWidth  = 1110.0
	mapHeight = 750.0
)

// SizeFieldMapConfig задаёт заголовок и проверяемую атрибуцию карты.
type SizeFieldMapConfig struct {
	Title  string
	Source string
}

// SizeFieldMapReport описывает только свойства отображения, не повторяя
// числовой отчёт доменного расчёта.
type SizeFieldMapReport struct {
	NodeCount             int     `json:"node_count"`
	CellCount             int     `json:"cell_count"`
	RenderedMinSizeM      float64 `json:"rendered_min_size_m"`
	RenderedMaxSizeM      float64 `json:"rendered_max_size_m"`
	MeshEdgesDrawn        bool    `json:"mesh_edges_drawn"`
	AdaptiveMeshGenerated bool    `json:"adaptive_mesh_generated"`
}

type point struct {
	x float64
	y float64
}

type transform struct {
	minX    float64
	minY    float64
	originX float64
	originY float64
	scale   float64
	height  float64
}

func (value transform) project(source point) point {
	return point{
		x: value.originX + (source.x-value.minX)*value.scale,
		y: value.originY + value.height - (source.y-value.minY)*value.scale,
	}
}

// WriteSizeFieldSVG создаёт обзорную карту h(x,y). Цвет ячейки является
// средним четырёх узловых значений; рёбра исходного каркаса не рисуются, чтобы
// их нельзя было принять за результат будущего адаптивного генератора.
func WriteSizeFieldSVG(path string, model seabed.Model, field fieldmodel.Field, config SizeFieldMapConfig) (SizeFieldMapReport, error) {
	if !model.Accepted {
		return SizeFieldMapReport{}, fmt.Errorf("карта поля размера строится только для принятой модели lito-seabed/v1")
	}
	if field.Report.SchemaVersion != fieldmodel.SchemaVersion || len(field.Nodes) != len(model.Nodes)-1 {
		return SizeFieldMapReport{}, fmt.Errorf("поле размера не согласовано с узлами модели")
	}
	if strings.TrimSpace(config.Source) == "" {
		return SizeFieldMapReport{}, fmt.Errorf("для карты поля размера обязателен источник батиметрии")
	}
	minimumX, maximumX, minimumY, maximumY, err := bounds(model)
	if err != nil {
		return SizeFieldMapReport{}, err
	}
	projection := newTransform(minimumX, maximumX, minimumY, maximumY)
	values := make(map[int]fieldmodel.NodeValue, len(field.Nodes))
	for _, value := range field.Nodes {
		if value.NodeID <= 0 || value.NodeID >= len(model.Nodes) {
			return SizeFieldMapReport{}, fmt.Errorf("поле содержит отсутствующий узел %d", value.NodeID)
		}
		values[value.NodeID] = value
	}

	var cells strings.Builder
	for _, cell := range model.Mesh.Cells {
		if cell.NodeCount != 4 {
			return SizeFieldMapReport{}, fmt.Errorf("карта поля принимает только четырёхугольные ячейки")
		}
		target := 0.0
		for _, nodeID := range cell.Nodes {
			value, ok := values[nodeID]
			if !ok {
				return SizeFieldMapReport{}, fmt.Errorf("для узла %d отсутствует значение поля", nodeID)
			}
			target += value.TargetSizeM / 4
		}
		fmt.Fprintf(&cells, "    <polygon class=\"size-field-cell\" data-target-size-m=\"%.3f\" fill=\"%s\" stroke=\"%s\" stroke-width=\"0.3\" points=\"", target, sizeColour(target, field.Report.Config), sizeColour(target, field.Report.Config))
		for index, nodeID := range cell.Nodes {
			node := model.Nodes[nodeID]
			position := projection.project(point{x: node.XM, y: node.YM})
			if index > 0 {
				cells.WriteByte(' ')
			}
			fmt.Fprintf(&cells, "%.2f,%.2f", position.x, position.y)
		}
		cells.WriteString("\"/>\n")
	}

	var boundaries strings.Builder
	for _, edge := range model.BoundaryEdges {
		if edge.Kind != seabed.BoundaryCoastline && edge.Kind != seabed.BoundaryIsland {
			continue
		}
		startNode, endNode := model.Nodes[edge.NodeIDs[0]], model.Nodes[edge.NodeIDs[1]]
		start := projection.project(point{x: startNode.XM, y: startNode.YM})
		end := projection.project(point{x: endNode.XM, y: endNode.YM})
		fmt.Fprintf(&boundaries, "    <line class=\"size-field-coast\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"#153948\" stroke-width=\"1.25\" stroke-linecap=\"round\"/>\n", start.x, start.y, end.x, end.y)
	}

	title := strings.TrimSpace(config.Title)
	if title == "" {
		title = "Поле требуемого размера ячейки Чёрного моря"
	}
	summary := field.Report.Summary
	legend := renderLegend(field.Report.Config)
	scaleBar := renderScaleBar(maximumX-minimumX, projection.scale)
	zones := renderZoneSummary(summary.Zones)
	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="1000" viewBox="0 0 1600 1000" role="img" aria-labelledby="size-field-title size-field-description" data-schema="%s" data-adaptive-mesh-generated="false" data-mesh-edges-drawn="false">
  <title id="size-field-title">%s</title>
  <desc id="size-field-description">Целевой размер будущей четырёхугольной ячейки по расстоянию до берега, кривизне берега и градиенту глубины.</desc>
  <defs>
    <linearGradient id="size-scale" x1="0%%" y1="0%%" x2="100%%" y2="0%%">
      <stop offset="0%%" stop-color="#124f5c"/><stop offset="12.5%%" stop-color="#2a8c96"/>
      <stop offset="37.5%%" stop-color="#d7bd70"/><stop offset="100%%" stop-color="#c76549"/>
    </linearGradient>
    <clipPath id="field-map"><rect x="54" y="128" width="1110" height="750"/></clipPath>
  </defs>
  <rect width="1600" height="1000" fill="#f3f0e8"/>
  <rect x="20" y="20" width="1560" height="960" rx="24" fill="#fbfaf6" stroke="#cfc9bd"/>
  <text x="54" y="62" font-family="Helvetica, Arial, sans-serif" font-size="28" font-weight="700" fill="#172b3a">%s</text>
  <text x="54" y="90" font-family="Helvetica, Arial, sans-serif" font-size="13" fill="#52636f">h(x,y), м · меньшее значение означает более подробную будущую сетку</text>
  <rect x="54" y="128" width="1110" height="750" fill="#e7e1d6" stroke="#b9b2a5"/>
  <g clip-path="url(#field-map)" shape-rendering="geometricPrecision">
%s%s  </g>
  <rect x="54" y="128" width="1110" height="750" fill="none" stroke="#a9a195"/>
%s
  <rect x="1194" y="128" width="352" height="750" rx="16" fill="#f0ece2" stroke="#d1cabc"/>
  <text x="1220" y="166" font-family="Helvetica, Arial, sans-serif" font-size="18" font-weight="700" fill="#172b3a">Как читать поле</text>
  <text x="1220" y="197" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#52636f">Цвет — целевая длина ребра, не глубина.</text>
  <text x="1220" y="217" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#52636f">Берег, резкие формы и уклоны уточняют поле.</text>
%s
  <line x1="1220" y1="330" x2="1520" y2="330" stroke="#d1cabc"/>
  <text x="1220" y="362" font-family="Helvetica, Arial, sans-serif" font-size="15" font-weight="700" fill="#172b3a">Контроль плавности</text>
  <text x="1220" y="390" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#52636f">Скорректировано узлов: %d из %d</text>
  <text x="1220" y="412" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#52636f">Макс. соседнее отношение: %.3f ≤ %.3f</text>
  <text x="1220" y="434" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#52636f">Макс. градиент h: %.4f ≤ %.4f м/м</text>
  <text x="1220" y="456" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#52636f">Медиана: %.0f м · P05–P95: %.0f–%.0f м</text>
  <line x1="1220" y1="478" x2="1520" y2="478" stroke="#d1cabc"/>
  <text x="1220" y="510" font-family="Helvetica, Arial, sans-serif" font-size="15" font-weight="700" fill="#172b3a">Зоны результата</text>
%s
  <line x1="1220" y1="714" x2="1520" y2="714" stroke="#d1cabc"/>
  <text x="1220" y="746" font-family="Helvetica, Arial, sans-serif" font-size="15" font-weight="700" fill="#172b3a">Статус этапа</text>
  <text x="1220" y="774" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#52636f">Поле рассчитано на исходном каркасе.</text>
  <text x="1220" y="796" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#52636f">Адаптивная сетка ещё не построена.</text>
  <text x="1220" y="818" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#52636f">Передача h(x,y) в Gmsh — ADAPT-02.</text>
  <text x="54" y="918" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#5d6b73">Источник батиметрии: %s</text>
  <text x="54" y="940" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#5d6b73">Карта задания размера: рёбра исходного каркаса скрыты; формулы и признаки сохранены в size-field.csv и size-field.json.</text>
  <text x="54" y="962" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#5d6b73">Не использовать для навигации или задач безопасности на море.</text>
</svg>
`, fieldmodel.SchemaVersion, escape(title), escape(title), cells.String(), boundaries.String(), scaleBar, legend,
		summary.GrowthLimitedNodeCount, summary.NodeCount, summary.FinalMaxAdjacentRatio, field.Report.Config.MaxNeighbourRatio,
		summary.FinalMaxSizeGradientPerM, field.Report.Config.MaxSizeGradientPerM, summary.Target.MedianM, summary.Target.P05M, summary.Target.P95M,
		zones, escape(config.Source))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return SizeFieldMapReport{}, fmt.Errorf("создание каталога карты поля размера: %w", err)
	}
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return SizeFieldMapReport{}, fmt.Errorf("сохранение карты поля размера %q: %w", path, err)
	}
	return SizeFieldMapReport{
		NodeCount: len(field.Nodes), CellCount: len(model.Mesh.Cells),
		RenderedMinSizeM: summary.Target.MinM, RenderedMaxSizeM: summary.Target.MaxM,
		MeshEdgesDrawn: false, AdaptiveMeshGenerated: false,
	}, nil
}

func bounds(model seabed.Model) (float64, float64, float64, float64, error) {
	minimumX, minimumY := math.Inf(1), math.Inf(1)
	maximumX, maximumY := math.Inf(-1), math.Inf(-1)
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		if !finite(node.XM) || !finite(node.YM) {
			return 0, 0, 0, 0, fmt.Errorf("узел %d содержит некорректные координаты", nodeID)
		}
		minimumX, maximumX = math.Min(minimumX, node.XM), math.Max(maximumX, node.XM)
		minimumY, maximumY = math.Min(minimumY, node.YM), math.Max(maximumY, node.YM)
	}
	if maximumX <= minimumX || maximumY <= minimumY {
		return 0, 0, 0, 0, fmt.Errorf("модель имеет нулевую область карты")
	}
	return minimumX, maximumX, minimumY, maximumY, nil
}

func newTransform(minimumX, maximumX, minimumY, maximumY float64) transform {
	spanX, spanY := maximumX-minimumX, maximumY-minimumY
	padding := 10.0
	scale := math.Min((mapWidth-2*padding)/spanX, (mapHeight-2*padding)/spanY)
	contentWidth, contentHeight := spanX*scale, spanY*scale
	return transform{
		minX: minimumX, minY: minimumY, scale: scale, height: contentHeight,
		originX: mapLeft + (mapWidth-contentWidth)/2,
		originY: mapTop + (mapHeight-contentHeight)/2,
	}
}

func sizeColour(sizeM float64, config fieldmodel.Config) string {
	stops := []struct {
		value            float64
		red, green, blue int
	}{
		{config.MinSizeM, 18, 79, 92},
		{config.StraightCoastSizeM, 42, 140, 150},
		{config.ShelfSizeM, 215, 189, 112},
		{config.DeepSizeM, 199, 101, 73},
	}
	sizeM = math.Max(config.MinSizeM, math.Min(config.DeepSizeM, sizeM))
	for index := 1; index < len(stops); index++ {
		left, right := stops[index-1], stops[index]
		if sizeM > right.value {
			continue
		}
		ratio := 0.0
		if right.value > left.value {
			ratio = (sizeM - left.value) / (right.value - left.value)
		}
		return fmt.Sprintf("#%02x%02x%02x",
			int(math.Round(float64(left.red)+float64(right.red-left.red)*ratio)),
			int(math.Round(float64(left.green)+float64(right.green-left.green)*ratio)),
			int(math.Round(float64(left.blue)+float64(right.blue-left.blue)*ratio)),
		)
	}
	last := stops[len(stops)-1]
	return fmt.Sprintf("#%02x%02x%02x", last.red, last.green, last.blue)
}

func renderLegend(config fieldmodel.Config) string {
	return fmt.Sprintf(`  <rect x="1220" y="246" width="300" height="18" rx="4" fill="url(#size-scale)"/>
  <text x="1220" y="285" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#52636f" text-anchor="start">%.0f</text>
  <text x="1258" y="285" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#52636f" text-anchor="middle">%.0f</text>
  <text x="1333" y="285" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#52636f" text-anchor="middle">%.0f</text>
  <text x="1520" y="285" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#52636f" text-anchor="end">%.0f</text>
  <text x="1220" y="310" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#5d6b73">подробно</text>
  <text x="1520" y="310" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#5d6b73" text-anchor="end">крупно</text>`,
		config.MinSizeM, config.StraightCoastSizeM, config.ShelfSizeM, config.DeepSizeM)
}

func renderZoneSummary(zones []fieldmodel.ZoneSummary) string {
	var result strings.Builder
	y := 538.0
	for index, zone := range zones {
		if index >= 7 {
			break
		}
		fmt.Fprintf(&result, "  <text x=\"1220\" y=\"%.0f\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11\" fill=\"#52636f\">%s · n=%d</text>\n", y, escape(zone.Name), zone.NodeCount)
		fmt.Fprintf(&result, "  <text x=\"1520\" y=\"%.0f\" font-family=\"Helvetica, Arial, sans-serif\" font-size=\"11\" fill=\"#172b3a\" text-anchor=\"end\">%.0f–%.0f м</text>\n", y, zone.Target.MinM, zone.Target.MaxM)
		y += 24
	}
	return result.String()
}

func renderScaleBar(spanX, scale float64) string {
	target := spanX / 5
	power := math.Pow(10, math.Floor(math.Log10(target)))
	lengthM := math.Floor(target/power) * power
	if lengthM <= 0 {
		lengthM = power
	}
	lengthPixels := lengthM * scale
	label := fmt.Sprintf("%.0f км", lengthM/1000)
	return fmt.Sprintf(`  <g aria-label="масштаб">
    <line x1="78" y1="842" x2="%.2f" y2="842" stroke="#172b3a" stroke-width="3"/>
    <line x1="78" y1="835" x2="78" y2="849" stroke="#172b3a"/>
    <line x1="%.2f" y1="835" x2="%.2f" y2="849" stroke="#172b3a"/>
    <text x="%.2f" y="830" text-anchor="middle" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#172b3a">%s</text>
  </g>`, 78+lengthPixels, 78+lengthPixels, 78+lengthPixels, 78+lengthPixels/2, label)
}

func escape(value string) string {
	return html.EscapeString(value)
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
