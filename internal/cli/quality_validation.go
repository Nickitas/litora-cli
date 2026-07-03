package cli

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
	"math"
	"sort"
)

// printModelQualityMetrics рассчитывает и выводит метрики качества модели
func printModelQualityMetrics(snapshots [][]geometry.LatLon, sedimentResult *geometry.SedimentTransportResult, temporalResult *geometry.TemporalResult) error {
	if len(snapshots) < 2 {
		fmt.Println("\n  ⚠️  Недостаточно данных для расчёта метрик качества (нужно ≥ 2 snapshots)")
		return nil
	}

	fmt.Println("\n  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  МЕТРИКИ КАЧЕСТВА МОДЕЛИ")
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 1. Рассчитываем метрики эрозии для каждого шага
	var erosionMetrics []geometry.ErosionMetrics
	if temporalResult != nil && len(temporalResult.Snapshots) > 0 {
		// Используем данные из TemporalResult
		erosionMetrics = geometry.CalculateErosionMetrics(*temporalResult)
	} else {
		// Создаём простые метрики из snapshots
		erosionMetrics = make([]geometry.ErosionMetrics, len(snapshots))
		for i, snapshot := range snapshots {
			length := geometry.PolylineLength(snapshot)
			area := geometry.Area(snapshot)

			// Рассчитываем retreat meters если есть предыдущий snapshot
			var meanRetreat, maxRetreat float64
			if i > 0 && len(snapshots[i-1]) > 0 && len(snapshot) > 0 {
				retreats := calculateRetreatMeters(snapshots[i-1], snapshot)
				if len(retreats) > 0 {
					meanRetreat = mean(retreats)
					maxRetreat = maxFloat64(retreats)
				}
			}

			// Эродированный объём (упрощённо: изменение длины × глубина 1м)
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

	// 2. Рассчитываем фрактальные размерности для последних snapshots
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
		// Создаём упрощённый sediment result (без реального транспорта наносов)
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

		// Рассчитываем MassBalance правильно (с учетом transport)
		// Формула: |eroded - (deposited + transport)| / eroded
		// В упрощенном случае transport = 0
		var massBalance float64
		if totalEroded > 0 {
			totalAccountedFor := totalDeposited // transport = 0 в упрощенном случае
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
				TransportVolume: 0, // не считается в упрощенном случае
				NetChange:       totalEroded - totalDeposited,
			},
			MassBalance: massBalance,
			IsValid:     isValid,
		}
	}

	// 4. Рассчитываем метрики качества модели
	qualityMetrics := geometry.CalculateModelQualityMetrics(erosionMetrics, *finalSedimentResult)

	// 5. Выводим базовые метрики
	printQualityMetric("Dimension Stability", qualityMetrics.DimensionStability, 0.7, true,
		map[bool]string{true: "стабильная геометрия", false: "нестабильная геометрия"})
	printQualityMetric("Mass Balance", qualityMetrics.MassBalance, 0.15, false,
		map[bool]string{true: "сохранение массы", false: "нарушение баланса массы"})
	printQualityMetric("Spatial Autocorr", qualityMetrics.SpatialAutocorr, 0.0, true,
		map[bool]string{true: "нормальный паттерн", false: "аномальный паттерн"})
	printQualityMetric("Convergence Rate", qualityMetrics.ConvergenceRate, 0.5, true,
		map[bool]string{true: "модель сходится", false: "модель не сходится"})

	fmt.Println()

	// 5.1. Выводим расширенные метрики (Extended Metrics v2.0)
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  РАСШИРЕННЫЕ МЕТРИКИ (v2.0)")
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	fmt.Printf("  Sediment Transport Rate: %.2f m³/step\n", qualityMetrics.SedimentTransportRate)
	fmt.Printf("  Accumulation Index: %.2f%%\n", qualityMetrics.AccumulationIndex*100)
	fmt.Printf("  Erosion Hotspots: %d\n", qualityMetrics.ErosionHotspots)
	fmt.Printf("  Shoreline Change Rate: %.2f m/step\n", qualityMetrics.ShorelineChangeRate)

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
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	return nil
}

// printQualityMetric выводит одну метрику качества с визуальным индикатором
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

// calculateRetreatMeters рассчитывает отступание для каждой точки
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

// mean и max - вспомогательные функции
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

// fractalDimensionBoxCounting - proxy для расчёта фрактальной размерности
// Используем упрощённую версию для performance
func fractalDimensionBoxCounting(points []geometry.LatLon, maxScales int) float64 {
	if len(points) < 4 {
		return 1.0 // minimum dimension for line
	}

	// Simple implementation: calculate dimension using scale variation
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

	// Linear regression to estimate dimension
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

	// constrain to [1, 2]
	if dimension < 1.0 {
		return 1.0
	}
	if dimension > 2.0 {
		return 2.0
	}
	return dimension
}

// countBoxes подсчитывает число занятых box'ов для box-counting
func countBoxes(points []geometry.LatLon, scale int) int {
	if len(points) < 2 {
		return 0
	}

	// Find bounds
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

	// Simple box counting
	latRange := maxLat - minLat
	lonRange := maxLon - minLon

	if latRange <= 0 || lonRange <= 0 {
		return 0
	}

	// Calculate box size
	boxSizeLat := latRange / float64(scale)
	boxSizeLon := lonRange / float64(scale)

	// Count occupied boxes
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
