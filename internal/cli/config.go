package cli

// Командные константы для функций экспорта
const (
	cmdSource    = "source"
	cmdAll       = "all"
	cmdCoastline = "coastline"
	cmdDimension = "dimension"
	cmdErosion   = "erosion"
	cmdBenchmark = "benchmark"
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
	ShowIsolines   bool
	GridStep       float64
	CompassSize    int
	CompassStyle   string

	// Варианты упрощения (simplification)
	DisableSimplify bool
	ModelMaxPoints  int

	// Общие поля
	Command string
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
	case cmdBenchmark:
		return "lito benchmark"
	default:
		return "lito " + command
	}
}
