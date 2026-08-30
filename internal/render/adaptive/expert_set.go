package adaptive

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

// ExpertFragmentSVGConfig задаёт одинаковое окно, источник берега и изобаты
// для одной анонимной карточки AI-01. Название генератора в SVG не передаётся.
type ExpertFragmentSVGConfig struct {
	PresentationID string
	FragmentID     string
	Feature        string
	CenterX        float64
	CenterY        float64
	WidthM         float64
	HeightM        float64
	IsobathsM      []float64
	SourceRings    [][]mesh.Point
}

// ExpertFragmentSVGReport фиксирует фактическое содержимое карточки, чтобы
// пустой фрагмент нельзя было выдать за материал для экспертной оценки.
type ExpertFragmentSVGReport struct {
	VisibleCellCount         int
	VisibleContourCount      int
	VisibleCoastSegmentCount int
}

type expertBounds struct {
	minX, minY float64
	maxX, maxY float64
}

type expertPoint struct {
	x, y float64
}

type expertSegment struct {
	start, end expertPoint
}

// WriteExpertFragmentSVG создаёт анонимную карточку фактической full-quad
// сетки. В каждом варианте одной особенности используются одинаковые окно,
// исходный берег и изобаты фиксированной модели дна BATHY-03.
func WriteExpertFragmentSVG(path string, reference seabed.Model, candidate mesh.Mesh, config ExpertFragmentSVGConfig) (ExpertFragmentSVGReport, error) {
	if !reference.Accepted {
		return ExpertFragmentSVGReport{}, fmt.Errorf("для карточки AI-01 нужна принятая модель дна")
	}
	if len(candidate.Nodes) <= 1 || len(candidate.Cells) == 0 {
		return ExpertFragmentSVGReport{}, fmt.Errorf("кандидатная сетка AI-01 не содержит узлов и ячеек")
	}
	if strings.TrimSpace(config.PresentationID) == "" || strings.TrimSpace(config.FragmentID) == "" || strings.TrimSpace(config.Feature) == "" {
		return ExpertFragmentSVGReport{}, fmt.Errorf("карточка AI-01 должна иметь анонимный идентификатор, фрагмент и тип особенности")
	}
	if !finiteExpertRender(config.CenterX) || !finiteExpertRender(config.CenterY) || !finiteExpertRender(config.WidthM) || !finiteExpertRender(config.HeightM) || config.WidthM <= 0 || config.HeightM <= 0 {
		return ExpertFragmentSVGReport{}, fmt.Errorf("карточка AI-01 имеет некорректное окно LAEA")
	}
	if len(config.IsobathsM) == 0 {
		return ExpertFragmentSVGReport{}, fmt.Errorf("карточка AI-01 должна содержать опорные изобаты")
	}
	bounds := expertBounds{minX: config.CenterX - config.WidthM/2, maxX: config.CenterX + config.WidthM/2, minY: config.CenterY - config.HeightM/2, maxY: config.CenterY + config.HeightM/2}
	transform := newExpertTransform(bounds)

	var candidateCells, contours, coast strings.Builder
	visibleCells := 0
	for _, cell := range candidate.Cells {
		if cell.NodeCount != 4 || !expertMeshCellIntersects(candidate, cell, bounds) {
			continue
		}
		visibleCells++
		candidateCells.WriteString("      <polygon class=\"candidate-cell\" points=\"")
		for index, nodeID := range cell.Nodes {
			if nodeID <= 0 || nodeID >= len(candidate.Nodes) {
				continue
			}
			point := transform.project(expertPoint{x: candidate.Nodes[nodeID].X, y: candidate.Nodes[nodeID].Y})
			if index > 0 {
				candidateCells.WriteByte(' ')
			}
			fmt.Fprintf(&candidateCells, "%.2f,%.2f", point.x, point.y)
		}
		candidateCells.WriteString("\"/>\n")
	}
	if visibleCells == 0 {
		return ExpertFragmentSVGReport{}, fmt.Errorf("фрагмент %q не пересекает ни одной ячейки кандидатной сетки", config.FragmentID)
	}

	visibleContours := 0
	for index, depthM := range config.IsobathsM {
		if !finiteExpertRender(depthM) || depthM <= 0 {
			return ExpertFragmentSVGReport{}, fmt.Errorf("фрагмент %q содержит некорректную изобату", config.FragmentID)
		}
		colour := expertContourColour(index)
		for _, segment := range expertContourSegments(reference, depthM, bounds) {
			start, end := transform.project(segment.start), transform.project(segment.end)
			fmt.Fprintf(&contours, "      <line class=\"reference-contour\" data-depth-m=\"%.0f\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\" stroke=\"%s\"/>\n", depthM, start.x, start.y, end.x, end.y, colour)
			visibleContours++
		}
	}

	visibleCoast := 0
	for _, ring := range config.SourceRings {
		if len(ring) < 2 {
			continue
		}
		for index := 0; index < len(ring)-1; index++ {
			segment := expertSegment{start: expertPoint{x: ring[index].X, y: ring[index].Y}, end: expertPoint{x: ring[index+1].X, y: ring[index+1].Y}}
			if !expertSegmentIntersects(segment, bounds) {
				continue
			}
			start, end := transform.project(segment.start), transform.project(segment.end)
			fmt.Fprintf(&coast, "      <line class=\"source-coast\" x1=\"%.2f\" y1=\"%.2f\" x2=\"%.2f\" y2=\"%.2f\"/>\n", start.x, start.y, end.x, end.y)
			visibleCoast++
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ExpertFragmentSVGReport{}, fmt.Errorf("создание каталога карточки AI-01: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return ExpertFragmentSVGReport{}, fmt.Errorf("создание SVG-карточки AI-01 %q: %w", path, err)
	}
	defer file.Close()

	widthKM, heightKM := config.WidthM/1000, config.HeightM/1000
	_, writeErr := fmt.Fprintf(file, `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="960" height="720" viewBox="0 0 960 720" role="img" aria-labelledby="title description">
  <title id="title">Анонимная карточка %s</title>
  <desc id="description">Фрагмент %s. Генератор и уровень намеренно не указаны.</desc>
  <defs>
    <clipPath id="plot"><rect x="68" y="108" width="824" height="496" rx="4"/></clipPath>
  </defs>
  <rect width="960" height="720" fill="#f7fafb"/>
  <rect x="34" y="30" width="892" height="650" rx="14" fill="#ffffff" stroke="#d9e3e8"/>
  <text x="68" y="69" font-family="Helvetica, Arial, sans-serif" font-size="22" font-weight="700" fill="#193743">Карточка %s</text>
  <text x="68" y="92" font-family="Helvetica, Arial, sans-serif" font-size="13" fill="#52636f">Особенность: %s · одинаковое окно %.0f × %.0f км · LAEA, м</text>
  <rect x="68" y="108" width="824" height="496" rx="4" fill="#eef7fa" stroke="#cbdde5"/>
  <g clip-path="url(#plot)">
    <g class="reference-contours" fill="none" stroke-width="1.5" stroke-linecap="round">
%s    </g>
    <g class="candidate-mesh" fill="#ffffff" fill-opacity="0.44" stroke="#2d5c72" stroke-width="0.72" stroke-linejoin="round">
%s    </g>
    <g class="source-coastline" fill="none" stroke="#1e2930" stroke-width="2.1" stroke-linecap="round">
%s    </g>
  </g>
  <g font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#405361">
    <line x1="68" y1="634" x2="92" y2="634" stroke="#1e2930" stroke-width="2.1"/><text x="99" y="638">исходный берег</text>
    <line x1="238" y1="634" x2="262" y2="634" stroke="#2d5c72" stroke-width="1.1"/><text x="269" y="638">кандидатная сетка</text>
    <line x1="442" y1="634" x2="466" y2="634" stroke="#1582a5" stroke-width="1.5"/><text x="473" y="638">изобаты BATHY-03</text>
  </g>
  <text x="68" y="663" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#667985">Сравнивайте форму границы, читаемость изобат, достаточность размера ячеек и визуальные артефакты. Не пытайтесь определить генератор.</text>
</svg>
`, expertEscape(config.PresentationID), expertEscape(config.FragmentID), expertEscape(config.PresentationID), expertEscape(config.Feature), widthKM, heightKM, contours.String(), candidateCells.String(), coast.String())
	if writeErr != nil {
		return ExpertFragmentSVGReport{}, fmt.Errorf("запись SVG-карточки AI-01: %w", writeErr)
	}
	return ExpertFragmentSVGReport{VisibleCellCount: visibleCells, VisibleContourCount: visibleContours, VisibleCoastSegmentCount: visibleCoast}, nil
}

type expertTransform struct {
	bounds        expertBounds
	left, top     float64
	width, height float64
}

func newExpertTransform(bounds expertBounds) expertTransform {
	return expertTransform{bounds: bounds, left: 68, top: 108, width: 824, height: 496}
}

func (transform expertTransform) project(point expertPoint) expertPoint {
	return expertPoint{
		x: transform.left + (point.x-transform.bounds.minX)/(transform.bounds.maxX-transform.bounds.minX)*transform.width,
		y: transform.top + (transform.bounds.maxY-point.y)/(transform.bounds.maxY-transform.bounds.minY)*transform.height,
	}
}

func expertMeshCellIntersects(candidate mesh.Mesh, cell mesh.Cell, bounds expertBounds) bool {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, nodeID := range cell.Nodes {
		if nodeID <= 0 || nodeID >= len(candidate.Nodes) {
			return false
		}
		point := candidate.Nodes[nodeID]
		minX, maxX = math.Min(minX, point.X), math.Max(maxX, point.X)
		minY, maxY = math.Min(minY, point.Y), math.Max(maxY, point.Y)
	}
	return maxX >= bounds.minX && minX <= bounds.maxX && maxY >= bounds.minY && minY <= bounds.maxY
}

func expertContourSegments(reference seabed.Model, depthM float64, bounds expertBounds) []expertSegment {
	segments := make([]expertSegment, 0)
	for _, cell := range reference.Cells {
		points := [4]expertPoint{}
		depths := [4]float64{}
		valid := true
		for index, nodeID := range cell.NodeIDs {
			if nodeID <= 0 || nodeID >= len(reference.Nodes) || reference.Nodes[nodeID].WaterDepthM == nil {
				valid = false
				break
			}
			node := reference.Nodes[nodeID]
			points[index], depths[index] = expertPoint{x: node.XM, y: node.YM}, *node.WaterDepthM
		}
		if !valid || !expertPointsIntersectBounds(points, bounds) {
			continue
		}
		intersections := make([]expertPoint, 0, 4)
		for edge := 0; edge < 4; edge++ {
			next := (edge + 1) % 4
			left, right := depths[edge], depths[next]
			if (left < depthM && right < depthM) || (left > depthM && right > depthM) || math.Abs(left-right) <= 1e-12 {
				continue
			}
			ratio := (depthM - left) / (right - left)
			if ratio < -1e-9 || ratio > 1+1e-9 {
				continue
			}
			intersections = append(intersections, expertPoint{x: points[edge].x + ratio*(points[next].x-points[edge].x), y: points[edge].y + ratio*(points[next].y-points[edge].y)})
		}
		switch len(intersections) {
		case 2:
			segments = append(segments, expertSegment{start: intersections[0], end: intersections[1]})
		case 4:
			center := (depths[0] + depths[1] + depths[2] + depths[3]) / 4
			if (center >= depthM) == (depths[0] >= depthM) {
				segments = append(segments, expertSegment{start: intersections[0], end: intersections[1]}, expertSegment{start: intersections[2], end: intersections[3]})
			} else {
				segments = append(segments, expertSegment{start: intersections[0], end: intersections[3]}, expertSegment{start: intersections[1], end: intersections[2]})
			}
		}
	}
	return segments
}

func expertPointsIntersectBounds(points [4]expertPoint, bounds expertBounds) bool {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, point := range points {
		minX, maxX = math.Min(minX, point.x), math.Max(maxX, point.x)
		minY, maxY = math.Min(minY, point.y), math.Max(maxY, point.y)
	}
	return maxX >= bounds.minX && minX <= bounds.maxX && maxY >= bounds.minY && minY <= bounds.maxY
}

func expertSegmentIntersects(segment expertSegment, bounds expertBounds) bool {
	minX, maxX := math.Min(segment.start.x, segment.end.x), math.Max(segment.start.x, segment.end.x)
	minY, maxY := math.Min(segment.start.y, segment.end.y), math.Max(segment.start.y, segment.end.y)
	return maxX >= bounds.minX && minX <= bounds.maxX && maxY >= bounds.minY && minY <= bounds.maxY
}

func expertContourColour(index int) string {
	colours := []string{"#1582a5", "#246f9e", "#405f96", "#664f89", "#8a4b7c"}
	return colours[index%len(colours)]
}

func finiteExpertRender(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func expertEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;")
	return replacer.Replace(value)
}
