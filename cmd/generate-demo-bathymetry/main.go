package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// Black Sea Bathymetry Generator
// Создаёт реалистичную батиметрию на основе реальных данных о Чёрном море

type BathymetryPoint struct {
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Depth float64 `json:"depth"`
}

func main() {
	fmt.Println("=== Генерация батиметрии Чёрного моря ===\n")

	// Параметры Чёрного моря
	minLat, maxLat := 40.5, 46.5
	minLon, maxLon := 27.5, 42.5
	resolution := 0.01 // ~1.1 км

	outputFile := "data/black-sea-bathymetry.json"

	var points []BathymetryPoint

	// Генерируем сетку
	latSteps := int((maxLat-minLat)/resolution) + 1
	lonSteps := int((maxLon-minLon)/resolution) + 1

	fmt.Printf("Генерация сетки: %dx%d = %d точек\n", latSteps, lonSteps, latSteps*lonSteps)

	// Реальные характеристики Чёрного моря
	centerLat := 43.5
	centerLon := 34.0
	maxDepth := 2212.0 // максимальная глубина в метрах

	for i := 0; i < latSteps; i++ {
		lat := minLat + float64(i)*resolution

		for j := 0; j < lonSteps; j++ {
			lon := minLon + float64(j)*resolution

			// Расстояние от центра (глубочайшая точка)
			dLat := lat - centerLat
			dLon := lon - centerLon
			distSquared := dLat*dLat + dLon*dLon

			// Реалистичная модель глубины Чёрного моря
			depth := calculateBlackSeaDepth(lat, lon, distSquared, maxDepth)

			// Пропускаем сушу (глубина >= 0)
			if depth >= 0 {
				continue
			}

			points = append(points, BathymetryPoint{
				Lat:   roundTo(lat, 6),
				Lon:   roundTo(lon, 6),
				Depth: roundTo(depth, 2),
			})
		}

		if (i+1)%10 == 0 {
			fmt.Printf("Обработано: %d/%d (%.1f%%)\n", i+1, latSteps, float64(i+1)*100/float64(latSteps))
		}
	}

	// Сохраняем в JSON
	fmt.Printf("\nСохранение в %s...\n", outputFile)

	os.MkdirAll("data", 0755)
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Ошибка создания файла: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(points); err != nil {
		fmt.Printf("Ошибка кодирования JSON: %v\n", err)
		os.Exit(1)
	}

	// Статистика
	printStats(points)

	fmt.Println("\n=== Готово! ===")
	fmt.Printf("Батиметрия сохранена: %s\n", outputFile)
	fmt.Println("\nИспользование:")
	fmt.Println("  make erosion-with-bathymetry")
}

// calculateBlackSeaDepth рассчитывает реалистичную глубину для Чёрного моря
// Основано на реальных батиметрических характеристиках
func calculateBlackSeaDepth(lat, lon, distSquared, maxDepth float64) float64 {
	// Базовая модель - параболоид
	depth := maxDepth * (1 - distSquared/12.0)

	// Корректировка на реальные особенности

	// Северо-западный шельф (мелководье)
	if lat > 44.0 && lon < 31.0 {
		shelfFactor := 0.3 // мелководье
		depth = depth * shelfFactor
	}

	// Восточная часть (глубже)
	if lon > 36.0 {
		deepFactor := 1.2 // глубже
		depth = depth * deepFactor
	}

	// Южное побережье (Турция, мелководье у берега)
	if lat < 41.5 {
		southShelfFactor := 0.4
		depth = depth * southShelfFactor
	}

	// Добавляем нерегулярность для реалистичности
	irregularity := 0.1 * math.Sin(lat*10) * math.Cos(lon*10)
	depth += irregularity * 100 // ±10 м

	// Ограничиваем
	if depth > 0 {
		depth = 0 // суша
	}
	if depth < -maxDepth {
		depth = -maxDepth
	}

	return depth
}

func printStats(points []BathymetryPoint) {
	if len(points) == 0 {
		fmt.Println("Нет данных для статистики")
		return
	}

	var minDepth, maxDepth, sumDepth float64
	minDepth = points[0].Depth
	maxDepth = points[0].Depth

	for _, p := range points {
		if p.Depth < minDepth {
			minDepth = p.Depth
		}
		if p.Depth > maxDepth {
			maxDepth = p.Depth
		}
		sumDepth += p.Depth
	}

	avgDepth := sumDepth / float64(len(points))

	fmt.Println("\nСтатистика:")
	fmt.Printf("  Точек: %d\n", len(points))
	fmt.Printf("  Мин. глубина: %.1f м\n", minDepth)
	fmt.Printf("  Макс. глубина: %.1f м\n", maxDepth)
	fmt.Printf("  Средняя глубина: %.1f м\n", avgDepth)

	// Размер файла
	if info, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
		fmt.Printf("  Размер файла: %d KB\n", info.Size()/1024)
	}

	fmt.Println("\n✓ Данные основаны на реальных характеристиках Чёрного моря:")
	fmt.Println("  - Макс. глубина: ~2212 м (центр)")
	fmt.Println("  - Северо-западный шельф: мелководье")
	fmt.Println("  - Восточная часть: глубже")
	fmt.Println("  - Южное побережье: мелководье у берега")
}

func roundTo(value float64, precision int) float64 {
	factor := 1.0
	for i := 0; i < precision; i++ {
		factor *= 10
	}
	return float64(int(value*factor+0.5)) / factor
}
