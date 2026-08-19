package cli

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
)

const (
	// Максимальное количество точек для упрощения SVG береговой линии
	coastlineSVGMaxPoints = 5000
	// Максимальное количество точек для упрощения в серийных SVG
	seriesSVGMaxPoints = 5000
	// Ограничение на количество точек базовой модели
	modelBaseMaxPointsCap = 3072
	// Бudget точек для модельных кривых
	modelCurvePointBudget = 400000
)

// geometryViews хранит различные представления геометрии для обработки
type geometryViews struct {
	RenderBase  []geometry.LatLon // База для рендеринга (упрощённая)
	ModelBase   []geometry.LatLon // База для моделирования
	ProcessInfo []string          // Информация о выполненных обработках
}

var currentConfig config // текущая конфигурация, устанавливается через setCurrentConfig

// setCurrentConfig устанавливает текущую конфигурацию для использования в prepareGeometryViews
func setCurrentConfig(cfg config) {
	currentConfig = cfg
}

// prepareGeometryViews подготавливает представления геометрии для рендеринга и моделирования
func prepareGeometryViews(points []geometry.LatLon, command string, iterations int) geometryViews {
	cfg := currentConfig // устанавливается через setCurrentConfig перед вызовом prepareGeometryViews

	views := geometryViews{
		RenderBase: points,
		ModelBase:  points,
	}

	if commandUsesCoastlineSVG(command) {
		renderResult := geometry.SimplifyPolyline(points, geometry.SimplifyOptions{MaxPoints: coastlineSVGMaxPoints})
		views.RenderBase = renderResult.Points
		if renderResult.Applied {
			views.ProcessInfo = append(views.ProcessInfo, formatSimplificationNote(
				"упрощение SVG-изображения береговой линии",
				points,
				renderResult.Points,
				fmt.Sprintf("для рендеринга (max %d точек)", coastlineSVGMaxPoints),
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
					"упрощение синтетической основы",
					points,
					modelResult.Points,
					fmt.Sprintf("для этапов модели (цель %d точек при бюджете итераций %d)", target, iterations),
				))
			}
		}
	}

	return views
}

// simplifyForSeriesSVG упрощает геометрию для рендеринга в серийных SVG
func simplifyForSeriesSVG(points []geometry.LatLon) geometry.SimplifyResult {
	return geometry.SimplifyPolyline(points, geometry.SimplifyOptions{MaxPoints: seriesSVGMaxPoints})
}

// formatSimplificationNote форматирует заметку об упрощении геометрии
func formatSimplificationNote(label string, original, simplified []geometry.LatLon, suffix string) string {
	return fmt.Sprintf("%s: %d -> %d точек, %.0f -> %.0f км %s",
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
	case cmdAll, cmdKochDemo:
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

// powInt возводит целое число в степень (base^exponent)
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
