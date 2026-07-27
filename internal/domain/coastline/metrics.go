package coastline

import (
	"fmt"
	"strings"

	"coastal-geometry/internal/domain/geometry"
)

const maxConsolePoints = 30

func MainCalculation(coast []geometry.LatLon, datasetName, source string) SanityCheckResult {
	segmentCount := 0
	if len(coast) > 1 {
		segmentCount = len(coast) - 1
	}

	fmt.Println(strings.Repeat("═", 80))
	fmt.Println("\tБЕРЕГОВАЯ ЛИНИЯ ЧЁРНОГО МОРЯ")
	fmt.Println(strings.Repeat("═", 80))

	fmt.Printf("\nКоличество точек:                        %d\n", len(coast))
	fmt.Printf("Количество сегментов:                    %d\n", segmentCount)
	if source != "" {
		fmt.Printf("Источник данных:                         %s\n", source)
	}

	totalLength := geometry.PolylineLength(coast)
	sanity := SanityCheck(datasetName, totalLength)
	fmt.Printf("Общая длина береговой линии:              %.0f км\n", totalLength)
	if segmentCount > 0 {
		fmt.Printf("Средняя длина сегмента:                   %.1f км\n\n", totalLength/float64(segmentCount))
	} else {
		fmt.Printf("Средняя длина сегмента:                   0.0 км\n\n")
	}

	if sanity.Warning != "" {
		fmt.Println(sanity.Warning)
		fmt.Println()
	}

	fmt.Println("Ключевые точки береговой линии:")
	if len(coast) > maxConsolePoints {
		fmt.Printf("Показаны %d равномерно распределённых точек из %d.\n", maxConsolePoints, len(coast))
	}
	fmt.Println(strings.Repeat("─", 80))
	fmt.Printf("%-4s %-11s %-11s %-25s\n", "№", "Широта", "Долгота", "Город / ориентир")
	fmt.Println(strings.Repeat("─", 80))

	for _, entry := range consolePointSelection(coast) {
		if entry.placeholder != "" {
			fmt.Println(entry.placeholder)
			continue
		}
		name := getLocationName(entry.point)
		fmt.Printf("%-4d %-11.4f %-11.4f %-25s\n", entry.index+1, entry.point.Lat, entry.point.Lon, name)
	}

	fmt.Println(strings.Repeat("═", 80))
	fmt.Printf("Итого: %.0f км\n", totalLength)
	return sanity
}

type consolePointEntry struct {
	index       int
	point       geometry.LatLon
	placeholder string
}

func consolePointSelection(points []geometry.LatLon) []consolePointEntry {
	if len(points) <= maxConsolePoints {
		result := make([]consolePointEntry, 0, len(points))
		for i, point := range points {
			result = append(result, consolePointEntry{index: i, point: point})
		}
		return result
	}

	result := make([]consolePointEntry, 0, maxConsolePoints)
	seen := make(map[int]struct{}, maxConsolePoints)
	lastIndex := len(points) - 1

	for i := 0; i < maxConsolePoints; i++ {
		index := i * lastIndex / (maxConsolePoints - 1)
		if _, ok := seen[index]; ok {
			continue
		}
		seen[index] = struct{}{}
		result = append(result, consolePointEntry{index: index, point: points[index]})
	}
	return result
}
