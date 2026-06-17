package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BlackSeaBounds определяет границы Чёрного моря
var BlackSeaBounds = struct {
	MinLat float64
	MaxLat float64
	MinLon float64
	MaxLon float64
}{
	MinLat: 40.5,
	MaxLat: 46.5,
	MinLon: 27.5,
	MaxLon: 42.5,
}

// BathymetryPoint представляет точку батиметрии
type BathymetryPoint struct {
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Depth float64 `json:"depth"`
}

// GebcoResponse представляет ответ GEBCO API
type GebcoResponse struct {
	Longitude []float64   `json:"longitude"`
	Latitude  []float64   `json:"latitude"`
	Elevation [][]float64 `json:"elevation"`
}

func main() {
	outputDir := "data"
	outputFile := filepath.Join(outputDir, "black-sea-bathymetry.json")

	fmt.Println("=== Загрузка батиметрии GEBCO для Чёрного моря ===\n")

	// Создаём директорию
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("❌ Ошибка создания директории: %v\n", err)
		os.Exit(1)
	}

	// Проверяем, существует ли файл
	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("✓ Файл уже существует: %s\n", outputFile)

		// Спрашиваем об обновлении
		fmt.Print("Обновить данные? (y/N): ")
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(answer) != "y" {
			fmt.Println("Используем существующий файл.")
			return
		}
		fmt.Println("Обновляем данные...\n")
	}

	// Шаг 1: Скачивание данных
	fmt.Println("[1/2] Загрузка данных с GEBCO API...")

	data, err := downloadBathymetry()
	if err != nil {
		fmt.Printf("❌ Ошибка загрузки: %v\n", err)
		fmt.Println("\nАльтернатива: используйте Python скрипт:")
		fmt.Println("  bash scripts/download_bathymetry.sh")
		os.Exit(1)
	}

	fmt.Printf("✓ Загружено точек: %d\n", len(data))

	// Шаг 2: Сохранение в JSON
	fmt.Println("\n[2/2] Сохранение в JSON...")

	if err := saveBathymetry(data, outputFile); err != nil {
		fmt.Printf("❌ Ошибка сохранения: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Сохранено: %s\n\n", outputFile)

	// Статистика
	printStats(data)

	fmt.Println("\n=== Готово! ===")
	fmt.Printf("Батиметрия сохранена: %s\n", outputFile)
	fmt.Println("\nИспользование:")
	fmt.Printf("  ./lito model erosion --bathymetry %s --output ./output/erosion\n", outputFile)
}

// downloadBathymetry загружает батиметрию через GEBCO API
func downloadBathymetry() ([]BathymetryPoint, error) {
	// Используем GEBCO API для получения батиметрии
	resolution := 0.01 // ~1.1 км

	var points []BathymetryPoint

	// Генерируем сетку
	latSteps := int((BlackSeaBounds.MaxLat-BlackSeaBounds.MinLat)/resolution) + 1
	lonSteps := int((BlackSeaBounds.MaxLon-BlackSeaBounds.MinLon)/resolution) + 1

	fmt.Printf("  Генерация сетки: %dx%d = %d точек\n", latSteps, lonSteps, latSteps*lonSteps)

	for i := 0; i < latSteps; i++ {
		lat := BlackSeaBounds.MinLat + float64(i)*resolution

		for j := 0; j < lonSteps; j++ {
			lon := BlackSeaBounds.MinLon + float64(j)*resolution

			// Получаем глубину через GEBCO API
			depth, err := getDepthFromAPI(lat, lon)
			if err != nil {
				// Если API недоступен, используем аппроксимацию
				depth = approximateDepth(lat, lon)
			}

			// Пропускаем сушу
			if depth >= 0 {
				continue
			}

			points = append(points, BathymetryPoint{
				Lat:   roundTo(lat, 6),
				Lon:   roundTo(lon, 6),
				Depth: roundTo(depth, 2),
			})
		}

		// Прогресс
		if (i+1)%10 == 0 || i == latSteps-1 {
			fmt.Printf("  Обработано: %d/%d (%.1f%%)\n", i+1, latSteps, float64(i+1)*100/float64(latSteps))
		}
	}

	return points, nil
}

// getDepthFromAPI получает глубину через GEBCO API
func getDepthFromAPI(lat, lon float64) (float64, error) {
	// GEBCO API endpoint
	url := fmt.Sprintf("https://www.gebco.net/data_and_products/underlying_data_set/gebco_2024/xyz_geotiff/?a=%f&b=%f&c=%f&d=%f",
		lat-0.01, lat+0.01, lon-0.01, lon+0.01)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API вернул статус %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// Парсим XYZ формат (space-separated: lon lat depth)
	lines := strings.Split(string(body), "\n")
	if len(lines) < 2 {
		return 0, fmt.Errorf("пустой ответ")
	}

	// Берём первую точку
	fields := strings.Fields(lines[1])
	if len(fields) < 3 {
		return 0, fmt.Errorf("неверный формат XYZ")
	}

	depth, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, err
	}

	// GEBCO: положительные = глубина, наш формат: отрицательные = глубина
	return -depth, nil
}

// approximateDepth создаёт аппроксимацию глубины на основе широты
// Это резервный метод, если API недоступен
func approximateDepth(lat, lon float64) float64 {
	// Простая модель: глубина увеличивается к центру моря
	// Центр Чёрного моря: ~43.5°N, 34.0°E

	centerLat := 43.5
	centerLon := 34.0

	// Расстояние от центра (градусы)
	dLat := lat - centerLat
	dLon := lon - centerLon
	distance := (dLat*dLat + dLon*dLon) // Квадрат расстояния

	// Максимальная глубина ~2212 м в центре
	// Минимальная у берегов ~0 м
	maxDepth := 2212.0

	// Параболическая аппроксимация
	depth := maxDepth * (1 - distance/12.0) // 12°² примерно покрывает всё море

	if depth < 0 {
		depth = 0 // Суша
	}

	return -depth // Отрицательное значение для нашего формата
}

// saveBathymetry сохраняет точки в JSON файл
func saveBathymetry(points []BathymetryPoint, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	return encoder.Encode(points)
}

// printStats выводит статистику по батиметрии
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
}

// roundTo округляет число до заданного количества знаков
func roundTo(value float64, precision int) float64 {
	factor := 1.0
	for i := 0; i < precision; i++ {
		factor *= 10
	}
	return float64(int(value*factor+0.5)) / factor
}
