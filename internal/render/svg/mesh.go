package svg

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	mesh2d "coastal-geometry/internal/domain/mesh"
)

const meshPreviewCellLimit = 120000

// MeshReportOptions задаёт подписи научной карты расчётной 2D-сетки.
type MeshReportOptions struct {
	DatasetName                   string
	Source                        string
	Algorithm                     mesh2d.Algorithm
	TargetEdgeMeters              float64
	BoundaryDetailMeters          float64
	EffectiveBoundaryDetailMeters float64
	FullMeshPath                  string
	OriginalPointCount            int
	SimplifiedPointCount          int
}

// DrawMeshReportSVG создаёт обзор всей поверхности водоёма по фактическим
// ячейкам Gmsh. Для очень больших сеток SVG равномерно прореживается, а полный
// набор без потерь остаётся в MSH-файле.
func DrawMeshReportSVG(domain mesh2d.PreparedDomain, generated mesh2d.Mesh, metrics mesh2d.QualityMetrics, options MeshReportOptions, filename string) error {
	if len(domain.SimplifiedRings) == 0 || len(generated.Cells) == 0 {
		return fmt.Errorf("для SVG необходимы граница и ячейки 2D-сетки")
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return fmt.Errorf("создать каталог SVG-сетки: %w", err)
	}

	minX, maxX, minY, maxY := meshBounds(domain.SimplifiedRings)
	spanX, spanY := maxX-minX, maxY-minY
	if spanX <= 0 || spanY <= 0 {
		return fmt.Errorf("нулевой размер области 2D-сетки")
	}
	const (
		width  = 1440.0
		height = 940.0
		left   = 56.0
		top    = 190.0
		mapW   = 1328.0
		mapH   = 650.0
	)
	scale := math.Min(mapW/spanX, mapH/spanY)
	contentW, contentH := spanX*scale, spanY*scale
	originX := left + (mapW-contentW)/2
	originY := top + (mapH-contentH)/2
	project := func(point mesh2d.Point) (float64, float64) {
		return originX + (point.X-minX)*scale, originY + contentH - (point.Y-minY)*scale
	}

	stride := 1
	if len(generated.Cells) > meshPreviewCellLimit {
		stride = int(math.Ceil(float64(len(generated.Cells)) / meshPreviewCellLimit))
	}
	var meshPath strings.Builder
	for index := 0; index < len(generated.Cells); index += stride {
		cell := generated.Cells[index]
		if cell.NodeCount < 3 {
			continue
		}
		for nodeIndex := 0; nodeIndex < cell.NodeCount; nodeIndex++ {
			nodeID := cell.Nodes[nodeIndex]
			if nodeID <= 0 || nodeID >= len(generated.Nodes) {
				continue
			}
			x, y := project(generated.Nodes[nodeID])
			if nodeIndex == 0 {
				fmt.Fprintf(&meshPath, "M%.2f %.2f", x, y)
			} else {
				fmt.Fprintf(&meshPath, "L%.2f %.2f", x, y)
			}
		}
		meshPath.WriteByte('Z')
	}

	var boundaries strings.Builder
	for ringIndex, ring := range domain.SimplifiedRings {
		color := "#16324f"
		if ringIndex > 0 {
			color = "#4f6d7a"
		}
		boundaries.WriteString(fmt.Sprintf(`<polyline fill="none" stroke="%s" stroke-width="1.8" points="`, color))
		for _, point := range ring {
			x, y := project(point)
			fmt.Fprintf(&boundaries, "%.2f,%.2f ", x, y)
		}
		boundaries.WriteString(`"/>` + "\n")
	}

	previewNote := "полная сетка показана без прореживания"
	if stride > 1 {
		previewNote = fmt.Sprintf("SVG-превью прорежено 1:%d; полный набор ячеек сохранён в MSH", stride)
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="1440" height="940" viewBox="0 0 1440 940">
  <rect width="100%%" height="100%%" fill="#f7f4ea"/>
  <rect x="20" y="20" width="1400" height="900" rx="28" fill="#fcfbf7" stroke="#d6d0c4"/>
  <text x="56" y="58" font-family="Helvetica, Arial, sans-serif" font-size="24" font-weight="700" fill="#16324f">Расчётная 2D-сетка водоёма</text>
  <text x="56" y="84" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#4f6d7a">Набор: %s · генератор: %s</text>
  <text x="56" y="104" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#4f6d7a">Ребро: %.0f м · допуск берега: %.0f м (фактически %.2f м) · точки: %d → %d</text>
  <text x="56" y="124" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#4f6d7a">Ячейки: %d · четырёхугольники: %.2f%% · оценка качества: %.2f/100</text>
  <text x="56" y="144" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#4f6d7a">Σ|ΔS| особенностей: %.3f км² · RMS границы: %.1f м · %s</text>
  <defs><clipPath id="mesh-map"><rect x="56" y="190" width="1328" height="650"/></clipPath></defs>
  <g clip-path="url(#mesh-map)">
    <path d="%s" fill="none" stroke="#8794a0" stroke-width="0.45" stroke-opacity="0.55"/>
%s  </g>
  <rect x="56" y="190" width="1328" height="650" fill="none" stroke="#d6d0c4"/>
  <text x="56" y="870" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#6b7a87">Проекция: %s</text>
  <text x="56" y="890" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#6b7a87">Источник: %s · полный файл: %s</text>
</svg>
`, escapeText(options.DatasetName), escapeText(options.Algorithm.RussianName()),
		options.TargetEdgeMeters, options.BoundaryDetailMeters, options.EffectiveBoundaryDetailMeters, options.OriginalPointCount, options.SimplifiedPointCount,
		metrics.CellCount, metrics.QuadSharePercent, metrics.CompositeScore,
		metrics.CumulativeFeatureAreaDeviationKM2, metrics.BoundaryRMSMeters, escapeText(previewNote),
		meshPath.String(), boundaries.String(), escapeText(domain.Projection.Description()), escapeText(options.Source), escapeText(options.FullMeshPath))
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		return fmt.Errorf("сохранение SVG-сетки %q: %w", filename, err)
	}
	return nil
}

func meshBounds(rings [][]mesh2d.Point) (minX, maxX, minY, maxY float64) {
	first := rings[0][0]
	minX, maxX, minY, maxY = first.X, first.X, first.Y, first.Y
	for _, ring := range rings {
		for _, point := range ring {
			minX = math.Min(minX, point.X)
			maxX = math.Max(maxX, point.X)
			minY = math.Min(minY, point.Y)
			maxY = math.Max(maxY, point.Y)
		}
	}
	return minX, maxX, minY, maxY
}
