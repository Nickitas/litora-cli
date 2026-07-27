package cli

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
)

const (
	coastlineSVGMaxPoints = 3200
	seriesSVGMaxPoints    = 1800
	modelBaseMaxPointsCap = 3072
	modelCurvePointBudget = 400000
)

type geometryViews struct {
	RenderBase  []geometry.LatLon
	ModelBase   []geometry.LatLon
	ProcessInfo []string
}

var currentConfig config

func setCurrentConfig(cfg config) {
	currentConfig = cfg
}

func prepareGeometryViews(points []geometry.LatLon, command string, iterations int) geometryViews {
	cfg := currentConfig // set via setter before prepareGeometryViews is called

	views := geometryViews{
		RenderBase: points,
		ModelBase:  points,
	}

	if commandUsesCoastlineSVG(command) {
		renderResult := geometry.SimplifyPolyline(points, geometry.SimplifyOptions{MaxPoints: coastlineSVGMaxPoints})
		views.RenderBase = renderResult.Points
		if renderResult.Applied {
			views.ProcessInfo = append(views.ProcessInfo, formatSimplificationNote(
				"coastline SVG simplification",
				points,
				renderResult.Points,
				fmt.Sprintf("for rendering (max %d points)", coastlineSVGMaxPoints),
			))
		}
	}

	if commandUsesModelBase(command) {
		if cfg.DisableSimplify {
			views.ModelBase = points
		} else {
			target := modelBaseTargetPoints(iterations)
			if cfg.ModelMaxPoints > 0 && cfg.ModelMaxPoints < target {
				target = cfg.ModelMaxPoints
			}
			modelResult := geometry.SimplifyPolyline(points, geometry.SimplifyOptions{MaxPoints: target})
			views.ModelBase = modelResult.Points
			if modelResult.Applied {
				views.ProcessInfo = append(views.ProcessInfo, formatSimplificationNote(
					"synthetic base simplification",
					points,
					modelResult.Points,
					fmt.Sprintf("for model stages (target %d points at iteration budget %d)", target, iterations),
				))
			}
		}
	}

	return views
}

func simplifyForSeriesSVG(points []geometry.LatLon) geometry.SimplifyResult {
	return geometry.SimplifyPolyline(points, geometry.SimplifyOptions{MaxPoints: seriesSVGMaxPoints})
}

func formatSimplificationNote(label string, original, simplified []geometry.LatLon, suffix string) string {
	return fmt.Sprintf("%s: %d -> %d points, %.0f -> %.0f km %s",
		label,
		len(original),
		len(simplified),
		geometry.PolylineLength(original),
		geometry.PolylineLength(simplified),
		suffix,
	)
}

func commandUsesCoastlineSVG(command string) bool {
	switch command {
	case cmdCoastline, cmdAll:
		return true
	default:
		return false
	}
}

func commandUsesModelBase(command string) bool {
	switch command {
	case cmdAll, cmdDimension:
		return true
	default:
		return false
	}
}

func modelBaseTargetPoints(iterations int) int {
	growthFactor := powInt(4, iterations)
	if growthFactor < 1 {
		growthFactor = 1
	}

	target := modelCurvePointBudget/growthFactor + 1
	if target > modelBaseMaxPointsCap {
		target = modelBaseMaxPointsCap
	}
	if target < 4 {
		target = 4
	}
	return target
}

func powInt(base, exponent int) int {
	if exponent <= 0 {
		return 1
	}

	result := 1
	for i := 0; i < exponent; i++ {
		result *= base
	}
	return result
}
