package fractal

// Константы для анализа фрактальной размерности

const (
	// minScaleSamples - минимальное количество масштабных уровней для анализа
	minScaleSamples = 4

	// minStableLocalSlopes - минимальное количество локальных наклонов для стабильности
	minStableLocalSlopes = 3

	// minRegressionRSquared - минимальное значение R² для линейной регрессии
	minRegressionRSquared = 0.98

	// maxLocalSlopeSpread - максимальный разброс локальных наклонов для стабильности
	maxLocalSlopeSpread = 0.18

	// minValidDimension - минимальное допустимое значение фрактальной размерности
	minValidDimension = 0.5

	// maxValidDimension - максимальное допустимое значение фрактальной размерности
	maxValidDimension = 3.0

	// minBboxSize - минимальный размер ограничивающей рамки в метрах
	minBboxSize = 1.0
)

// Константы для параллельной обработки

const (
	// maxConvWorkers - максимальное количество воркеров для конвертации координат
	maxConvWorkers = 4

	// maxScaleWorkers - максимальное количество воркеров для обработки масштабов
	maxScaleWorkers = 8

	// maxBboxWorkers - максимальное количество воркеров для вычисления границ
	maxBboxWorkers = 4

	// maxSpreadWorkers - максимальное количество воркеров для вычисления разброса
	maxSpreadWorkers = 4
)

// defaultScaleFactors - коэффициенты масштабирования для box-counting (более плотная сетка для уменьшения чувствительности к выбору масштаба)
var defaultScaleFactors = []float64{4, 6, 8, 12, 16, 24, 32, 48, 64, 96, 128, 192, 256}

// gridOffsets - смещения сетки для усреднения результатов box-counting
var gridOffsets = [][2]float64{
	{0, 0},     // Без смещения
	{0.5, 0},   // Смещение по X
	{0, 0.5},   // Смещение по Y
	{0.5, 0.5}, // Смещение по X и Y
}
