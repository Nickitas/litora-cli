package geometry

import (
	"fmt"
	"math"
)

// ModelQualityMetrics метрики качества модели для научной валидации
type ModelQualityMetrics struct {
	DimensionStability float64 // изменение фрактальной размерности D во времени
	MassBalance        float64 // баланс массы (eroded - deposited)
	SpatialAutocorr    float64 // пространственная автокорреляция соседних участков
	ConvergenceRate    float64 // скорость изменения метрик (сходимость)

	// Дополнительные статистики
	DimensionVariance  float64  // вариация размерности
	MassBalanceTrend   float64  // тренд баланса массы
	SpatialCorrelation float64  // корреляция Moran's I
	IsValidModel       bool     // флаг валидности модели
	Warnings           []string // предупреждения

	// Расширенные метрики (Extended Metrics v2.0)
	SedimentTransportRate float64 // объем наносов в транспорте (m³/шаг)
	AccumulationIndex     float64 // процент точек с аккумуляцией (0-1)
	ErosionHotspots       int     // число кластеров высокой эрозии
	ShorelineChangeRate   float64 // скорость изменения береговой линии (м/шаг)
}

// ValidationMetricsTimeSeries временной ряд метрик для анализа сходимости
type ValidationMetricsTimeSeries struct {
	Steps               []int     // номера шагов
	Dimensions          []float64 // фрактальные размерности по шагам
	MassBalances        []float64 // балансы массы по шагам
	SpatialCorrelations []float64 // пространственные корреляции по шагам
	ConvergenceRates    []float64 // скорости сходимости по шагам
}

// CalculateModelQualityMetrics рассчитывает метрики качества модели
func CalculateModelQualityMetrics(
	erosionMetrics []ErosionMetrics,
	sedimentResult SedimentTransportResult,
) ModelQualityMetrics {

	if len(erosionMetrics) < 2 {
		return ModelQualityMetrics{
			IsValidModel: false,
			Warnings:     []string{"недостаточно данных для проверки"},
		}
	}

	metrics := ModelQualityMetrics{}

	// 1. DimensionStability: изменение фрактальной размерности во времени
	metrics.DimensionStability = calculateDimensionStability(erosionMetrics)

	// 2. MassBalance: баланс массы (нормализованное отношение)
	metrics.MassBalance = sedimentResult.MassBalance

	// 3. SpatialAutocorr: пространственная автокорреляция
	metrics.SpatialAutocorr = calculateSpatialAutocorrelation(erosionMetrics)

	// 4. ConvergenceRate: скорость изменения метрик
	metrics.ConvergenceRate = calculateConvergenceRate(erosionMetrics)

	// Дополнительные метрики
	metrics.DimensionVariance = calculateDimensionVariance(erosionMetrics)
	metrics.MassBalanceTrend = calculateMassBalanceTrend(erosionMetrics)
	metrics.SpatialCorrelation = calculateMoransI(erosionMetrics)

	// Расширенные метрики (Extended Metrics v2.0)
	metrics.SedimentTransportRate = calculateSedimentTransportRate(sedimentResult)
	metrics.AccumulationIndex = calculateAccumulationIndex(sedimentResult)
	metrics.ErosionHotspots = calculateErosionHotspots(erosionMetrics, sedimentResult)
	metrics.ShorelineChangeRate = calculateShorelineChangeRate(erosionMetrics)

	// Валидация модели
	metrics.IsValidModel = validateModelQuality(metrics)
	metrics.Warnings = generateValidationWarnings(metrics)

	return metrics
}

// calculateDimensionStability рассчитывает стабильность фрактальной размерности
func calculateDimensionStability(metrics []ErosionMetrics) float64 {
	if len(metrics) < 2 {
		return 0
	}

	// Собираем фрактальные размерности
	dimensions := make([]float64, 0, len(metrics))
	for _, m := range metrics {
		if m.FractalDimension > 0 {
			dimensions = append(dimensions, m.FractalDimension)
		}
	}

	if len(dimensions) < 2 {
		return 0
	}

	// Проверяем, что все значения одинаковы
	meanDim := mean(dimensions)
	allSame := true
	for _, d := range dimensions {
		if math.Abs(d-meanDim) > 1e-9 {
			allSame = false
			break
		}
	}

	// Если все значения одинаковы - идеальная стабильность
	if allSame {
		return 1.0
	}

	// Коэффициент вариации (CV = std/mean)
	variance := 0.0
	for _, d := range dimensions {
		diff := d - meanDim
		variance += diff * diff
	}
	variance /= float64(len(dimensions))
	cv := math.Sqrt(variance) / meanDim

	// Стабильность = 1 - CV (экспоненциальное затухание для больших CV)
	stability := math.Exp(-cv * 2.0)
	return math.Max(0, math.Min(1, stability))
}

// calculateSpatialAutocorrelation рассчитывает пространственную автокорреляцию
func calculateSpatialAutocorrelation(metrics []ErosionMetrics) float64 {
	if len(metrics) < 2 {
		return 0
	}

	// Используем изменение длины береговой линии как proxy
	// для пространственной корреляции
	lengths := make([]float64, len(metrics))
	for i, m := range metrics {
		lengths[i] = m.LengthKm
	}

	// Рассчитываем автокорреляцию с лагом 1
	return calculateLag1Autocorrelation(lengths)
}

// calculateConvergenceRate рассчитывает скорость изменения метрик
func calculateConvergenceRate(metrics []ErosionMetrics) float64 {
	if len(metrics) < 3 {
		return 0
	}

	// Рассчитываем скорость изменения длины береговой линии
	// Конвергенция = замедление изменений со временем
	rates := make([]float64, len(metrics)-1)
	for i := 0; i < len(metrics)-1; i++ {
		lengthChange := math.Abs(metrics[i+1].LengthKm - metrics[i].LengthKm)
		rates[i] = lengthChange
	}

	// Проверяем, что есть вариация в rates
	totalRate := 0.0
	for _, r := range rates {
		totalRate += r
	}
	meanRate := totalRate / float64(len(rates))

	if meanRate < 1e-6 {
		// Если изменений нет - полная конвергенция
		return 1.0
	}

	// Если rate уменьшается → модель сходится
	if len(rates) < 2 {
		return 0
	}

	// Тренд rate: положительный = расхождение, отрицательный = сходимость
	trend := calculateLinearTrend(rates)

	// Нормируем тренд относительно среднего rate
	normalizedTrend := trend / meanRate

	// Конвергенция: отрицательный тренд → высокая конвергенция
	// Конвертиуем из [-∞, +∞] в [0, 1]
	convergence := 1.0 / (1.0 + math.Abs(normalizedTrend))
	return math.Max(0, math.Min(1, convergence))
}

// calculateDimensionVariance рассчитывает вариацию размерности
func calculateDimensionVariance(metrics []ErosionMetrics) float64 {
	dimensions := make([]float64, 0, len(metrics))
	for _, m := range metrics {
		if m.FractalDimension > 0 {
			dimensions = append(dimensions, m.FractalDimension)
		}
	}

	if len(dimensions) < 2 {
		return 0
	}

	meanDim := mean(dimensions)
	variance := 0.0
	for _, d := range dimensions {
		diff := d - meanDim
		variance += diff * diff
	}
	variance /= float64(len(dimensions))

	return variance
}

// calculateMassBalanceTrend рассчитывает тренд баланса массы
func calculateMassBalanceTrend(metrics []ErosionMetrics) float64 {
	if len(metrics) < 3 {
		return 0
	}

	// Используем нормализованные значения баланса массы
	// Чтобы избежать больших абсолютных значений (m³)
	balances := make([]float64, len(metrics))
	for i, m := range metrics {
		// Нормализуем NetChange относительно ErodedM3
		if m.ErodedM3 != 0 {
			balances[i] = m.NetChangeM3 / m.ErodedM3
		} else {
			balances[i] = 0
		}
	}

	return calculateLinearTrend(balances)
}

// calculateMoransI рассчитывает Moran's I (пространственная корреляция)
func calculateMoransI(metrics []ErosionMetrics) float64 {
	if len(metrics) < 3 {
		return 0
	}

	// Упрощённая версия Moran's I для временного ряда
	// I = (N/W) * ΣΣwij(xi-x̄)(xj-x̄) / Σ(xi-x̄)²
	// Для временного ряда wij = 1 для соседних шагов

	// Используем длину береговой линии как переменную
	values := make([]float64, len(metrics))
	for i, m := range metrics {
		values[i] = m.LengthKm
	}

	n := float64(len(values))
	if n < 3 {
		return 0
	}

	meanVal := mean(values)

	// Сумма квадратов отклонений
	sumSquares := 0.0
	for _, v := range values {
		diff := v - meanVal
		sumSquares += diff * diff
	}

	if sumSquares < 1e-9 {
		return 0
	}

	// Пространственная ковариация (лаг 1)
	numerator := 0.0
	weightSum := 0.0

	for i := 0; i < len(values)-1; i++ {
		xi := values[i]
		xj := values[i+1]
		wij := 1.0 // вес для соседних шагов

		numerator += wij * (xi - meanVal) * (xj - meanVal)
		weightSum += wij
	}

	moransI := (n / weightSum) * (numerator / sumSquares)

	return math.Max(-1, math.Min(1, moransI))
}

// calculateLag1Autocorrelation рассчитывает автокорреляцию с лагом 1
func calculateLag1Autocorrelation(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	n := len(values)
	meanVal := mean(values)

	// Проверяем, что все значения одинаковы
	allSame := true
	for _, v := range values {
		if math.Abs(v-meanVal) > 1e-9 {
			allSame = false
			break
		}
	}

	// Если все значения одинаковы - идеальная положительная корреляция
	if allSame {
		return 1.0
	}

	// Ковариация с лагом 1
	covLag1 := 0.0
	for i := 0; i < n-1; i++ {
		covLag1 += (values[i] - meanVal) * (values[i+1] - meanVal)
	}
	covLag1 /= float64(n - 1)

	// Дисперсия
	variance := 0.0
	for _, v := range values {
		diff := v - meanVal
		variance += diff * diff
	}
	variance /= float64(n)

	if variance < 1e-9 {
		return 0
	}

	// Автокорреляция
	autocorr := covLag1 / variance
	return math.Max(-1, math.Min(1, autocorr))
}

// calculateLinearTrend рассчитывает линейный тренд (наклон линии регрессии)
func calculateLinearTrend(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	n := float64(len(values))
	x := make([]float64, len(values))
	for i := range x {
		x[i] = float64(i)
	}

	// Линейная регрессия: y = a + bx
	// b = (NΣxy - ΣxΣy) / (NΣx² - (Σx)²)

	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i := range values {
		sumX += x[i]
		sumY += values[i]
		sumXY += x[i] * values[i]
		sumX2 += x[i] * x[i]
	}

	denominator := n*sumX2 - sumX*sumX
	if math.Abs(denominator) < 1e-9 {
		return 0
	}

	slope := (n*sumXY - sumX*sumY) / denominator

	return slope
}

// calculateSedimentTransportRate рассчитывает объем наносов в транспорте
func calculateSedimentTransportRate(sedimentResult SedimentTransportResult) float64 {
	// Объем наносов в транспорте (m³/шаг)
	return sedimentResult.TotalBudget.TransportVolume
}

// calculateAccumulationIndex рассчитывает процент точек с аккумуляцией
func calculateAccumulationIndex(sedimentResult SedimentTransportResult) float64 {
	if len(sedimentResult.States) == 0 {
		return 0
	}

	// Подсчитываем точки с аккумуляцией
	accumulatingPoints := 0
	for _, state := range sedimentResult.States {
		if state.IsAccumulating {
			accumulatingPoints++
		}
	}

	// Индекс аккумуляции (0-1)
	accumulationIndex := float64(accumulatingPoints) / float64(len(sedimentResult.States))
	return accumulationIndex
}

// calculateErosionHotspots выявляет кластеры высокой эрозии
func calculateErosionHotspots(erosionMetrics []ErosionMetrics, sedimentResult SedimentTransportResult) int {
	if len(sedimentResult.States) == 0 {
		return 0
	}

	// Порог высокой эрозии: 95-й перцентиль (строгий критерий)
	retreats := make([]float64, 0)
	for _, state := range sedimentResult.States {
		if state.IsEroding && state.LocalBudget.ErodedVolume > 0 {
			retreats = append(retreats, state.LocalBudget.ErodedVolume)
		}
	}

	if len(retreats) == 0 {
		return 0
	}

	// Сортируем для нахождения перцентиля
	for i := 0; i < len(retreats); i++ {
		for j := i + 1; j < len(retreats); j++ {
			if retreats[i] < retreats[j] {
				retreats[i], retreats[j] = retreats[j], retreats[i]
			}
		}
	}

	// 95-й перцентиль (только 5% наиболее эродирующих точек)
	percentile95 := retreats[len(retreats)*19/20]
	if percentile95 <= 0 {
		return 0
	}

	// Подсчитываем hotspots (точки выше 95-го перцентиля)
	hotspots := 0
	for _, state := range sedimentResult.States {
		if state.IsEroding && state.LocalBudget.ErodedVolume > percentile95 {
			hotspots++
		}
	}

	// Дополнительно: ограничиваем число hotspots до 5% от общего числа точек
	maxHotspots := len(sedimentResult.States) * 5 / 100
	if hotspots > maxHotspots {
		hotspots = maxHotspots
	}

	return hotspots
}

// calculateShorelineChangeRate рассчитывает скорость изменения береговой линии
func calculateShorelineChangeRate(erosionMetrics []ErosionMetrics) float64 {
	if len(erosionMetrics) < 2 {
		return 0
	}

	// Используем средний retreat вместо изменения длины
	// Это более реалистично для береговой линии
	totalRetreat := 0.0
	count := 0

	for _, m := range erosionMetrics {
		if m.MeanRetreatMeters > 0 {
			totalRetreat += m.MeanRetreatMeters
			count++
		}
	}

	if count == 0 {
		return 0
	}

	// Средний retreat (м/шаг)
	meanRetreatRate := totalRetreat / float64(count)

	return meanRetreatRate
}

// validateModelQuality валидирует качество модели
func validateModelQuality(metrics ModelQualityMetrics) bool {
	// Критерии валидности:
	// 1. Стабильность размерности > 0.7 (размерность не должна сильно меняться)
	// 2. Баланс массы ≈ 0 (допуск ±15% от эродированного объёма)
	// 3. Пространственная автокорреляция не экстремальная ([-0.3, 0.8])
	// 4. Сходимость > 0.5 (модель должна сходиться)

	if metrics.DimensionStability < 0.7 {
		return false
	}

	// MassBalance в абсолютном значении
	absMassBalance := math.Abs(metrics.MassBalance)
	if absMassBalance > 0.15 { // допуск 15%
		return false
	}

	if metrics.SpatialAutocorr < -0.3 || metrics.SpatialAutocorr > 0.8 {
		return false
	}

	if metrics.ConvergenceRate < 0.5 {
		return false
	}

	return true
}

// generateValidationWarnings генерирует предупреждения на основе метрик
func generateValidationWarnings(metrics ModelQualityMetrics) []string {
	warnings := []string{}

	if metrics.DimensionStability < 0.7 {
		warnings = append(warnings,
			fmt.Sprintf("Низкая стабильность размеров: %.2f (ожидается > 0,7)",
				metrics.DimensionStability))
	}

	if math.Abs(metrics.MassBalance) > 0.15 {
		warnings = append(warnings,
			fmt.Sprintf("Плохой баланс массы: %.4f (ожидаемый |баланс| < 0,15)",
				metrics.MassBalance))
	}

	if metrics.SpatialAutocorr < -0.3 {
		warnings = append(warnings,
			fmt.Sprintf("Отрицательная пространственная автокорреляция: %.2f (необычная картина)",
				metrics.SpatialAutocorr))
	}

	if metrics.SpatialAutocorr > 0.8 {
		warnings = append(warnings,
			fmt.Sprintf("Высокая пространственная автокорреляция: %.2f (чрезмерное сглаживание)",
				metrics.SpatialAutocorr))
	}

	if metrics.ConvergenceRate < 0.5 {
		warnings = append(warnings,
			fmt.Sprintf("Низкая скорость сходимости: %.2f (модель может не сходиться)",
				metrics.ConvergenceRate))
	}

	if metrics.DimensionVariance > 0.1 {
		warnings = append(warnings,
			fmt.Sprintf("Высокая вариация размерности: %.4f (нестабильная геометрия)",
				metrics.DimensionVariance))
	}

	if math.Abs(metrics.MassBalanceTrend) > 1.0 {
		warnings = append(warnings,
			fmt.Sprintf("Высокий тренд баланса массы: %.4f (перемещение)",
				metrics.MassBalanceTrend))
	}

	// Предупреждения для расширенных метрик
	if metrics.SedimentTransportRate < 10.0 {
		warnings = append(warnings,
			fmt.Sprintf("Низкий объем наносов в транспорте: %.2f м³/шаг (недостаточно транспорта)",
				metrics.SedimentTransportRate))
	}

	if metrics.AccumulationIndex < 0.1 {
		warnings = append(warnings,
			fmt.Sprintf("Низкий индекс аккумуляции: %.2f (высокая эрозия)",
				metrics.AccumulationIndex))
	}

	if metrics.AccumulationIndex > 0.8 {
		warnings = append(warnings,
			fmt.Sprintf("Высокий индекс аккумуляции: %.2f (высокая депозиция)",
				metrics.AccumulationIndex))
	}

	if metrics.ErosionHotspots > 100 {
		warnings = append(warnings,
			fmt.Sprintf("Высокое количество кластеров высокой эрозии: %d (возможная нестабильность)",
				metrics.ErosionHotspots))
	}

	if math.Abs(metrics.ShorelineChangeRate) > 100.0 {
		warnings = append(warnings,
			fmt.Sprintf("Высокая скорость изменения береговой линии: %.2f м/шаг (быстрый изменение)",
				metrics.ShorelineChangeRate))
	}

	return warnings
}

// GetQualityMetricsSummary возвращает сводку метрик качества
func GetQualityMetricsSummary(metrics ModelQualityMetrics) map[string]interface{} {
	summary := make(map[string]interface{})

	summary["dimension_stability"] = metrics.DimensionStability
	summary["mass_balance"] = metrics.MassBalance
	summary["spatial_autocorr"] = metrics.SpatialAutocorr
	summary["convergence_rate"] = metrics.ConvergenceRate
	summary["dimension_variance"] = metrics.DimensionVariance
	summary["mass_balance_trend"] = metrics.MassBalanceTrend
	summary["spatial_correlation_morans_i"] = metrics.SpatialCorrelation
	summary["is_valid_model"] = metrics.IsValidModel
	summary["warnings"] = metrics.Warnings

	// Расширенные метрики (Extended Metrics v2.0)
	summary["sediment_transport_rate"] = metrics.SedimentTransportRate
	summary["accumulation_index"] = metrics.AccumulationIndex
	summary["erosion_hotspots"] = metrics.ErosionHotspots
	summary["shoreline_change_rate"] = metrics.ShorelineChangeRate

	return summary
}

// CalculateTimeSeriesMetrics рассчитывает временные ряды метрик для анализа
func CalculateTimeSeriesMetrics(erosionMetrics []ErosionMetrics) ValidationMetricsTimeSeries {
	if len(erosionMetrics) == 0 {
		return ValidationMetricsTimeSeries{}
	}

	ts := ValidationMetricsTimeSeries{
		Steps:               make([]int, len(erosionMetrics)),
		Dimensions:          make([]float64, len(erosionMetrics)),
		MassBalances:        make([]float64, len(erosionMetrics)),
		SpatialCorrelations: make([]float64, len(erosionMetrics)),
		ConvergenceRates:    make([]float64, len(erosionMetrics)),
	}

	for i, m := range erosionMetrics {
		ts.Steps[i] = m.Step
		ts.Dimensions[i] = m.FractalDimension
		ts.MassBalances[i] = m.NetChangeM3
	}

	// Рассчитываем пространственные корреляции для каждого шага
	for i := 1; i < len(erosionMetrics); i++ {
		startIdx := i - 2
		if startIdx < 0 {
			startIdx = 0
		}
		window := erosionMetrics[startIdx : i+1]
		if len(window) >= 2 {
			lengths := make([]float64, len(window))
			for j, w := range window {
				lengths[j] = w.LengthKm
			}
			ts.SpatialCorrelations[i] = calculateLag1Autocorrelation(lengths)
		}
	}

	// Рассчитываем скорости сходимости
	for i := 2; i < len(erosionMetrics); i++ {
		startIdx := i - 3
		if startIdx < 0 {
			startIdx = 0
		}
		window := erosionMetrics[startIdx : i+1]
		ts.ConvergenceRates[i] = calculateConvergenceRate(window)
	}

	return ts
}

// ValidateModelConvergence проверяет сходимость модели
func ValidateModelConvergence(ts ValidationMetricsTimeSeries) (bool, []string) {
	if len(ts.ConvergenceRates) < 3 {
		return false, []string{"Недостаточно данных для анализа сходимости"}
	}

	warnings := []string{}
	isConvergent := true

	// Проверяем, что convergence rate растёт или стабилен
	lastRates := ts.ConvergenceRates[len(ts.ConvergenceRates)-3:]
	trend := calculateLinearTrend(lastRates)

	if trend < -0.1 {
		isConvergent = false
		warnings = append(warnings,
			fmt.Sprintf("Скорость сходимости снижается: тренд=%.3f (разрыв)", trend))
	}

	// Проверяем стабильность размерности в последних шагах
	if len(ts.Dimensions) >= 3 {
		lastDims := ts.Dimensions[len(ts.Dimensions)-3:]
		dimVariance := 0.0
		meanDim := mean(lastDims)
		for _, d := range lastDims {
			diff := d - meanDim
			dimVariance += diff * diff
		}
		dimVariance /= float64(len(lastDims))

		if dimVariance > 0.05 {
			isConvergent = false
			warnings = append(warnings,
				fmt.Sprintf("Разброс размерности в последних шагах: %.4f (нестабильно)", dimVariance))
		}
	}

	return isConvergent, warnings
}
