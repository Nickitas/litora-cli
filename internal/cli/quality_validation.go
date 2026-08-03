package cli

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
	"math"
	"sort"
)

// printModelQualityMetrics рассчитывает и выводит метрики качества модели эрозии
func printModelQualityMetrics(snapshots [][]geometry.LatLon, sedimentResult *geometry.SedimentTransportResult, temporalResult *geometry.TemporalResult) error {
	if len(snapshots) < 2 {
		fmt.Println("\n  ⚠️  Недостаточно данных для расчёта метрик качества (нужно ≥ 2 snapshots)")
		return nil
	}

	fmt.Println("\n  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  МЕТРИКИ КАЧЕСТВА МОДЕЛИ")
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 1. Рассчитываем метрики эрозии для каждого шага
	var erosionMetrics []geometry.ErosionMetrics
	if temporalResult != nil && len(temporalResult.Snapshots) > 0 {
		// Используем данные из TemporalResult
		erosionMetrics = geometry.CalculateErosionMetrics(*temporalResult)
	} else {
		// Создаём простые метрики из снимков
		erosionMetrics = make([]geometry.ErosionMetrics, len(snapshots))
		for i, snapshot := range snapshots {
			length := geometry.PolylineLength(snapshot)
			area := geometry.Area(snapshot)

			// Рассчитываем отступание в метрах если есть предыдущий снимок
			var meanRetreat, maxRetreat float64
			if i > 0 && len(snapshots[i-1]) > 0 && len(snapshot) > 0 {
				retreats := calculateRetreatMeters(snapshots[i-1], snapshot)
				if len(retreats) > 0 {
					meanRetreat = mean(retreats)
					maxRetreat = maxFloat64(retreats)
				}
			}

			// Эродированный объём (упрощённо: изменение длины × глубина 1 м)
			var erodedM3 float64
			if i > 0 {
				prevLen := geometry.PolylineLength(snapshots[i-1])
				erodedM3 = (prevLen - length) * 1000 * 1.0 // км→м × глубина
			}

			erosionMetrics[i] = geometry.ErosionMetrics{
				Step:              i,
				LengthKm:          length,
				AreaKm2:           area,
				ErodedM3:          erodedM3,
				DepositedM3:       0, // Будет заполнено через sediment transport
				NetChangeM3:       erodedM3,
				MeanRetreatMeters: meanRetreat,
				MaxRetreatMeters:  maxRetreat,
				FractalDimension:  0, // Будет рассчитан если нужно
			}
		}
	}

	// 2. Рассчитываем фрактальные размерности для последних снимков
	for i := range erosionMetrics {
		if i == 0 || i == len(erosionMetrics)-1 || i%3 == 0 {
			if len(snapshots[i]) > 10 {
				erosionMetrics[i].FractalDimension = fractalDimensionBoxCounting(snapshots[i], 10)
			}
		}
	}

	// 3. Если sedimentResult не предоставлен, создаём упрощённый вариант
	var finalSedimentResult *geometry.SedimentTransportResult
	if sedimentResult != nil {
		finalSedimentResult = sedimentResult
	} else {
		// Создаём упрощённый результат транспорта наносов (без реального расчёта)
		// Примечание: для корректной валидации нужен CalculateSedimentTransport
		totalEroded := 0.0
		totalDeposited := 0.0
		for _, m := range erosionMetrics {
			if m.ErodedM3 > 0 {
				totalEroded += m.ErodedM3
			}
			if m.DepositedM3 > 0 {
				totalDeposited += m.DepositedM3
			}
		}

		// Рассчитываем MassBalance правильно (с учётом транспорта)
		// Формула: |eroded - (deposited + transport)| / eroded
		// В упрощённом случае transport = 0
		var massBalance float64
		if totalEroded > 0 {
			totalAccountedFor := totalDeposited // transport = 0 в упрощённом случае
			massBalance = math.Abs(totalEroded-totalAccountedFor) / totalEroded
		} else {
			massBalance = 0
		}

		// Валидность: допуск 15%
		isValid := massBalance < 0.15

		finalSedimentResult = &geometry.SedimentTransportResult{
			TotalBudget: geometry.SedimentBudget{
				ErodedVolume:    totalEroded,
				DepositedVolume: totalDeposited,
				TransportVolume: 0, // не считается в упрощённом случае
				NetChange:       totalEroded - totalDeposited,
			},
			MassBalance: massBalance,
			IsValid:     isValid,
		}
	}

	// 4. Рассчитываем метрики качества модели
	qualityMetrics := geometry.CalculateModelQualityMetrics(erosionMetrics, *finalSedimentResult)

	// 5. Выводим все метрики качества
	printQualityMetric("Стабильность размерности", qualityMetrics.DimensionStability, 0.7, true,
		map[bool]string{true: "стабильная геометрия", false: "нестабильная геометрия"})
	printQualityMetric("Баланс массы", qualityMetrics.MassBalance, 0.15, false,
		map[bool]string{true: "сохранение массы", false: "нарушение баланса массы"})
	printQualityMetric("Пространственная автокорреляция", qualityMetrics.SpatialAutocorr, 0.0, true,
		map[bool]string{true: "нормальный паттерн", false: "аномальный паттерн"})
	printQualityMetric("Скорость сходимости", qualityMetrics.ConvergenceRate, 0.5, true,
		map[bool]string{true: "модель сходится", false: "модель не сходится"})
	printQualityMetric("Скорость транспорта наносов", qualityMetrics.SedimentTransportRate, 0.0, true,
		map[bool]string{true: "активный транспорт", false: "низкий транспорт"})
	printQualityMetric("Индекс аккумуляции", qualityMetrics.AccumulationIndex*100, 50.0, false,
		map[bool]string{true: "нормальная аккумуляция", false: "чрезмерная аккумуляция"})

	// Дополнительные метрики без пороговых значений
	fmt.Printf("  • Очаги эрозии: %d\n", qualityMetrics.ErosionHotspots)
	fmt.Printf("  • Скорость изменения береговой линии: %.2f м/шаг\n", qualityMetrics.ShorelineChangeRate)

	fmt.Println()

	// 6. Выводим предупреждения если есть
	if len(qualityMetrics.Warnings) > 0 {
		fmt.Println("  ПРЕДУПРЕЖДЕНИЯ:")
		sort.Strings(qualityMetrics.Warnings)
		for _, warning := range qualityMetrics.Warnings {
			fmt.Printf("  • %s\n", warning)
		}
		fmt.Println()
	}

	// 7. Итоговая оценка
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if qualityMetrics.IsValidModel {
		fmt.Println("  ИТОГОВАЯ ОЦЕНКА: МОДЕЛЬ НАУЧНО ВАЛИДНА ✓")
	} else {
		fmt.Println("  ИТОГОВАЯ ОЦЕНКА: МОДЕЛЬ ТРЕБУЕТ ДОРАБОТКИ ✗")
	}
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return nil
}

// printQualityMetric выводит одну метрику качества с визуальным индикатором и статусом
func printQualityMetric(name string, value, threshold float64, higherIsBetter bool, statusMap map[bool]string) {
	// Определяем валидность
	valid := false
	if higherIsBetter {
		valid = value >= threshold
	} else {
		valid = value <= threshold
	}

	// Визуальный индикатор
	indicator := "✓"
	if !valid {
		indicator = "✗"
	} else if value < threshold && higherIsBetter || value > threshold && !higherIsBetter {
		indicator = "⚠️"
	}

	// Статус
	status := statusMap[valid]
	if status == "" {
		status = statusMap[true]
	}

	fmt.Printf("  %s %s: %.2f (%s)\n", indicator, name, value, status)
}

// calculateRetreatMeters рассчитывает отступание береговой линии в метрах для каждой точки
func calculateRetreatMeters(prev, current []geometry.LatLon) []float64 {
	if len(prev) != len(current) || len(prev) == 0 {
		return nil
	}

	retreats := make([]float64, len(prev))

	for i := range prev {
		// Расстояние между соответствующими точками
		dist := geometry.Haversine(prev[i], current[i])
		retreats[i] = dist
	}

	return retreats
}

// mean вычисляет среднее значение массива чисел
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// maxFloat64 вычисляет максимальное значение в массиве чисел
func maxFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maximum := values[0]
	for _, v := range values {
		if v > maximum {
			maximum = v
		}
	}
	return maximum
}

// fractalDimensionBoxCounting вычисляет фрактальную размерность методом box-counting
// Используем упрощённую версию для производительности
func fractalDimensionBoxCounting(points []geometry.LatLon, maxScales int) float64 {
	if len(points) < 4 {
		return 1.0 // минимальная размерность для линии
	}

	// Простая реализация: вычисляем размерность используя вариацию масштаба
	scales := []int{2, 4, 8, 16, 32}
	if len(scales) > maxScales {
		scales = scales[:maxScales]
	}

	logScales := make([]float64, 0)
	logCounts := make([]float64, 0)

	for _, scale := range scales {
		count := countBoxes(points, scale)
		if count > 0 {
			logScales = append(logScales, math.Log(float64(scale)))
			logCounts = append(logCounts, math.Log(float64(count)))
		}
	}

	if len(logScales) < 2 {
		return 1.0
	}

	// Линейная регрессия для оценки размерности
	n := float64(len(logScales))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i := range logScales {
		sumX += logScales[i]
		sumY += logCounts[i]
		sumXY += logScales[i] * logCounts[i]
		sumX2 += logScales[i] * logScales[i]
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	dimension := -slope // D = -slope

	// Ограничиваем диапазон [1, 2]
	if dimension < 1.0 {
		return 1.0
	}
	if dimension > 2.0 {
		return 2.0
	}
	return dimension
}

// countBoxes подсчитывает количество занятых ячеек для метода box-counting
func countBoxes(points []geometry.LatLon, scale int) int {
	if len(points) < 2 {
		return 0
	}

	// Находим границы
	minLat, maxLat := points[0].Lat, points[0].Lat
	minLon, maxLon := points[0].Lon, points[0].Lon

	for _, p := range points {
		if p.Lat < minLat {
			minLat = p.Lat
		}
		if p.Lat > maxLat {
			maxLat = p.Lat
		}
		if p.Lon < minLon {
			minLon = p.Lon
		}
		if p.Lon > maxLon {
			maxLon = p.Lon
		}
	}

	// Простое подсчитывание ячеек
	latRange := maxLat - minLat
	lonRange := maxLon - minLon

	if latRange <= 0 || lonRange <= 0 {
		return 0
	}

	// Вычисляем размер ячейки
	boxSizeLat := latRange / float64(scale)
	boxSizeLon := lonRange / float64(scale)

	// Подсчитываем занятые ячейки
	occupied := make(map[string]bool)
	for _, p := range points {
		boxLat := int((p.Lat - minLat) / boxSizeLat)
		boxLon := int((p.Lon - minLon) / boxSizeLon)
		key := fmt.Sprintf("%d_%d", boxLat, boxLon)
		occupied[key] = true
	}

	return len(occupied)
}

// Вспомогательные математические функции
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
