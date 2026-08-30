package cli

// Командные константы для функций экспорта
const (
	cmdSource    = "source"
	cmdAll       = "all"
	cmdCoastline = "coastline"
	cmdDimension = "dimension"
	cmdErosion   = "erosion"
)

/* Упрощенная конфигурация для функций экспорта
 * Это минимальная конфигурационная структура, используемая функциями экспорта/вывода
 */
type config struct {
	// Расширенные возможности SVG
	EnableEnhanced bool
	ShowGrid       bool
	ShowCompass    bool
	ShowMarkers    bool
	GridStep       float64
	CompassSize    int
	CompassStyle   string

	// Варианты упрощения (simplification)
	DisableSimplify bool
	ModelMaxPoints  int

	// Общие поля
	Command        string
	ScenarioStatus string
	// BoxCountingBoxSize — фактический размер представительной ячейки анализа.
	BoxCountingBoxSize float64
	// Параметры регрессионного окна box-counting для научной SVG-легенды.
	BoxCountingRegressionMin float64
	BoxCountingRegressionMax float64
	BoxCountingLogLogSVGFile string
}

/*
 * Определение канонического пути к команде для отображения
 * returns: канонический путь к команде для отображения
 */
func canonicalCommandPath(command string) string {
	switch command {
	case cmdSource:
		return "lito source"
	case cmdAll:
		return "lito all"
	case cmdCoastline:
		return "lito coastline"
	case cmdDimension:
		return "lito dimension"
	case cmdErosion:
		return "lito erosion"
	default:
		return "lito " + command
	}
}
