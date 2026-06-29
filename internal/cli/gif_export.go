package cli

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"math"
	"os"
)

// GIFConfig содержит настройки для генерации GIF анимации
type GIFConfig struct {
	OutputPath    string   // путь для сохранения GIF файла
	FPS           int      // кадры в секунду
	SkipEvery     int      // пропускать каждый N-ный кадр
	Width         int      // ширина изображения
	Height        int      // высота изображения
	ColorByChange bool     // цветовое кодирование изменений
	ShowInitial   bool     // показывать начальное состояние
	ShowMetrics   bool     // показывать метрики на кадрах
	ShowScaleBar  bool     // показывать масштабную линейку
	ScaleBarKM    float64  // длина scale bar в км (0 = auto)
	ShowColorLegend bool     // показывать цветовую легенду
	ColorLegendPos string   // позиция легенды (bottom|right|none)
	GeoLabels       string   // geographic labels (none|major|all)
	TemporalStates  []geometry.TemporalState // временные состояния для меток
		Colors         int      // количество цветов в палитре
		Compression    string   // уровень сжатия (low|medium|high)
	ShowTimeStamp   bool     // показывать временные метки
}

// DefaultGIFConfig возвращает настройки по умолчанию
func DefaultGIFConfig() GIFConfig {
	return GIFConfig{
		OutputPath:    "erosion_animation.gif",
		FPS:           10,
		SkipEvery:     1,
		Width:         1200,  // увеличенное разрешение
		Height:        800,
		ColorByChange: true,  // включено по умолчанию
		ShowInitial:   true,   // показываем начальное состояние
		ShowMetrics:   true,
		ShowScaleBar:  true,
		ScaleBarKM:    0,
		ColorLegendPos:   "right",
		Colors:         16,
		Compression:    "medium",
		GeoLabels:        "major",
		ShowTimeStamp:   true,
	}
}

// GenerateErosionGIF создает GIF анимацию эрозии
func GenerateErosionGIF(snapshots [][]geometry.LatLon, outputPath string, fps int, skipEvery int) error {
	config := DefaultGIFConfig()
	config.OutputPath = outputPath
	config.FPS = fps
	config.SkipEvery = skipEvery

	return GenerateErosionGIFWithConfig(snapshots, config)
}

// GenerateErosionGIFWithConfig создает GIF с настройками
func GenerateErosionGIFWithConfig(snapshots [][]geometry.LatLon, config GIFConfig) error {
	if len(snapshots) == 0 {
		return nil
	}

	// Фильтруем кадры
	filteredSnapshots := filterSnapshots(snapshots, config.SkipEvery)
	if len(filteredSnapshots) < 2 {
		filteredSnapshots = snapshots
		if len(filteredSnapshots) > 2 {
			filteredSnapshots = filteredSnapshots[:2]
		}
	}

	// Анализируем изменения для цветового кодирования
	segmentChanges := analyzeChanges(filteredSnapshots)

	// Создаем высококачественную палитру
	palette := createOptimizedPalette(config)

	// Создаем кадры
	var frames []*image.Paletted

	for i, snapshot := range filteredSnapshots {
		img := image.NewPaletted(image.Rect(0, 0, config.Width, config.Height), palette)

		// Фон - вода
		draw.Draw(img, img.Bounds(), &image.Uniform{palette[0]}, image.Point{}, draw.Src)

		// Рисуем береговую линию с цветовыми кодами
			minLat, maxLat, minLon, maxLon := computeBounds(snapshot)

			// Рисуем береговую линию с цветовыми кодами и scale bar
		maxErosion, maxDeposition := computeMaxErosion(segmentChanges)

		// Рисуем цветовую легенду если включена
		if config.ShowColorLegend {
			drawColorLegend(img, maxErosion, maxDeposition, config)
		}
			drawGeoLabels(img, minLat, maxLat, minLon, maxLon, config)
			drawEnhancedColoredCoastline(img, snapshot, filteredSnapshots[0], segmentChanges, minLat, maxLat, minLon, maxLon, config)
		drawColoredCoastline(img, snapshot, filteredSnapshots[0], segmentChanges, config)

		// Добавляем метрики
		if config.ShowMetrics {
			drawEnhancedMetrics(img, i, filteredSnapshots, config)
		}
		drawTimeStamp(img, i, config)

		frames = append(frames, img)
	}

	// Создаем GIF
	delay := computeDelay(config.FPS)
	gifFile := &gif.GIF{
		Image: frames,
		Delay: make([]int, len(frames)),
	}

	for i := range gifFile.Delay {
		gifFile.Delay[i] = delay
	}

	// Сохраняем
	file, err := os.Create(config.OutputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return gif.EncodeAll(file, gifFile)
}

// SegmentChangeInfo хранит информацию об изменении сегмента
type SegmentChangeInfo struct {
	ErosionPerStep float64 // средняя эрозия за шаг
	TotalErosion   float64 // общая эрозия
	Variance       float64 // вариативность
}

// analyzeChanges анализирует изменения сегментов
func analyzeChanges(snapshots [][]geometry.LatLon) []SegmentChangeInfo {
	if len(snapshots) < 2 {
		return []SegmentChangeInfo{}
	}

	initial := snapshots[0]
	final := snapshots[len(snapshots)-1]
	numSteps := len(snapshots) - 1

	changes := make([]SegmentChangeInfo, 0)
	minPoints := min(len(initial), len(final))

	for i := 0; i < minPoints-1; i++ {
		// Длина сегмента в начале
		initialLength := geometry.Haversine(initial[i], initial[i+1])

		// Длина сегмента в конце
		finalLength := geometry.Haversine(final[i], final[i+1])

		// Общая эрозия (в метрах)
		totalErosion := (initialLength - finalLength) * 1000 // км в метры

		// Эрозия за шаг
		erosionPerStep := totalErosion / float64(numSteps)

		changes = append(changes, SegmentChangeInfo{
			ErosionPerStep: erosionPerStep,
			TotalErosion:   totalErosion,
			Variance:       math.Abs(totalErosion), // упрощенная вариативность
		})
	}

	return changes
}

// createHighQualityPalette создает высококачественную палитру для научной визуализации

// drawColoredCoastline рисует береговую линию с цветовыми кодами
func drawColoredCoastline(img *image.Paletted, current, initial []geometry.LatLon, changes []SegmentChangeInfo, config GIFConfig) {
	if len(current) < 2 {
		return
	}

	// Вычисляем границы и масштаб
	minLat, maxLat, minLon, maxLon := computeBounds(current)
	padding := 50
	drawWidth := config.Width - 2*padding
	drawHeight := config.Height - 2*padding

	lonSpan := maxLon - minLon
	if lonSpan == 0 {
		lonSpan = 1.0
	}
	latSpan := maxLat - minLat
	if latSpan == 0 {
		latSpan = 1.0
	}

	scale := math.Min(float64(drawWidth)/lonSpan, float64(drawHeight)/latSpan)
	contentWidth := lonSpan * scale
	contentHeight := latSpan * scale

	originX := float64(padding) + (float64(drawWidth)-contentWidth)/2
	originY := float64(padding) + (float64(drawHeight)-contentHeight)/2

	// Находим максимальную эрозию для нормализации
	maxErosion := 0.0
	maxDeposition := 0.0

	for _, change := range changes {
		if change.TotalErosion > maxErosion {
			maxErosion = change.TotalErosion
		}
		if -change.TotalErosion > maxDeposition {
			maxDeposition = -change.TotalErosion
		}
	}

	// Используем максимум для нормализации
	maxChange := math.Max(maxErosion, maxDeposition)
	if maxChange == 0 {
		maxChange = 1.0 // избегаем деления на ноль
	}

	// Рисуем сегменты с цветовыми кодами
	for i := 0; i < len(current)-1 && i < len(changes); i++ {
		x1 := int(originX + (current[i].Lon-minLon)*scale)
		y1 := int(originY + contentHeight - (current[i].Lat-minLat)*scale)
		x2 := int(originX + (current[i+1].Lon-minLon)*scale)
		y2 := int(originY + contentHeight - (current[i+1].Lat-minLat)*scale)

		// Проверяем границы
		if x1 < 0 || x1 >= config.Width || y1 < 0 || y1 >= config.Height ||
			x2 < 0 || x2 >= config.Width || y2 < 0 || y2 >= config.Height {
			continue
		}

		// Определяем цвет на основе изменения
		colorIndex := 2 // дефолтный (стабильный)

		if config.ColorByChange && len(changes) > i {
			change := changes[i]
			normalizedChange := math.Abs(change.TotalErosion) / maxChange

			if change.TotalErosion > 0 {
				// Эрозия - градиент зеленый -> красный
				if normalizedChange > 0.8 {
					colorIndex = 8 // экстремальная эрозия
				} else if normalizedChange > 0.6 {
					colorIndex = 7 // очень сильная
				} else if normalizedChange > 0.4 {
					colorIndex = 6 // сильная
				} else if normalizedChange > 0.2 {
					colorIndex = 5 // средняя
				} else if normalizedChange > 0.05 {
					colorIndex = 4 // умеренная
				} else {
					colorIndex = 3 // слабая
				}
			} else if change.TotalErosion < 0 {
				// Аккумуляция - градиент голубой
				if normalizedChange > 0.6 {
					colorIndex = 11 // сильная аккумуляция
				} else if normalizedChange > 0.3 {
					colorIndex = 10 // умеренная
				} else {
					colorIndex = 9 // слабая
				}
			}
		}

		// Рисуем линию с выбранным цветом
		drawThickLine(img, x1, y1, x2, y2, uint8(colorIndex), 2)
	}

	// Рисуем начальное состояние если включено
	if config.ShowInitial && len(initial) > 1 {
		for i := 0; i < len(initial)-1; i++ {
			x1 := int(originX + (initial[i].Lon-minLon)*scale)
			y1 := int(originY + contentHeight - (initial[i].Lat-minLat)*scale)
			x2 := int(originX + (initial[i+1].Lon-minLon)*scale)
			y2 := int(originY + contentHeight - (initial[i+1].Lat-minLat)*scale)

			if x1 >= 0 && x1 < config.Width && y1 >= 0 && y1 < config.Height &&
				x2 >= 0 && x2 < config.Width && y2 >= 0 && y2 < config.Height {
				// Рисуем тонкую серую линию (начальное состояние)
				drawThickLine(img, x1, y1, x2, y2, 12, 1)
			}
		}
	}
}

// drawEnhancedMetrics рисует улучшенные метрики
func drawEnhancedMetrics(img *image.Paletted, frameIndex int, snapshots [][]geometry.LatLon, config GIFConfig) {
	if len(snapshots) == 0 {
		return
	}

	current := snapshots[frameIndex]
	lengthKm := geometry.PolylineLength(current)

	// Вычисляем accumulated erosion
	accumulatedErosion := 0.0
	if frameIndex > 0 && len(snapshots[0]) > 0 {
		initialLength := geometry.PolylineLength(snapshots[0])
		accumulatedErosion = (initialLength - lengthKm) * 1000 // км в метры
	}

	// Формируем компактные метрики
	metricsY := config.Height - 40

	line1 := fmt.Sprintf("Frame: %d/%d | Length: %.1f km", frameIndex+1, len(snapshots), lengthKm)
	line2 := fmt.Sprintf("Erosion: %.1f m | Step: %.1f m", accumulatedErosion, accumulatedErosion/float64(frameIndex))

	// Рисуем метрики (упрощенно)
	drawSimpleText(img, line1, 20, metricsY, 13)
	drawSimpleText(img, line2, 20, metricsY+15, 13)
}

// drawThickLine рисует линию заданной толщины
func drawThickLine(img *image.Paletted, x0, y0, x1, y1 int, colorIndex uint8, thickness int) {
	for t := 0; t < thickness; t++ {
		offset := t - thickness/2
		dx := abs(x1 - x0)
		dy := abs(y1 - y0)
		sx, sy := 1, 1
		if x0 > x1 {
			sx = -1
		}
		if y0 > y1 {
			sy = -1
		}
		err := dx - dy

		for {
			setPixel(img, x0+offset, y0, colorIndex)
			if x0 == x1 && y0 == y1 {
				break
			}
			e2 := 2 * err
			if e2 > -dy {
				err -= dy
				x0 += sx
			}
			if e2 < dx {
				err += dx
				y0 += sy
			}
		}
	}
}

// Вспомогательные функции
func filterSnapshots(snapshots [][]geometry.LatLon, skipEvery int) [][]geometry.LatLon {
	if skipEvery <= 1 {
		return snapshots
	}

	var filtered [][]geometry.LatLon
	for i := 0; i < len(snapshots); i += skipEvery {
		filtered = append(filtered, snapshots[i])
	}

	// Всегда включаем последний кадр
	if len(filtered) > 0 && len(snapshots) > 0 &&
		&filtered[len(filtered)-1] != &snapshots[len(snapshots)-1] {
		filtered = append(filtered, snapshots[len(snapshots)-1])
	}

	return filtered
}

func computeBounds(snapshots []geometry.LatLon) (minLat, maxLat, minLon, maxLon float64) {
	if len(snapshots) == 0 {
		return 0, 0, 0, 0
	}

	minLat = snapshots[0].Lat
	maxLat = snapshots[0].Lat
	minLon = snapshots[0].Lon
	maxLon = snapshots[0].Lon

	for _, point := range snapshots {
		if point.Lat < minLat {
			minLat = point.Lat
		}
		if point.Lat > maxLat {
			maxLat = point.Lat
		}
		if point.Lon < minLon {
			minLon = point.Lon
		}
		if point.Lon > maxLon {
			maxLon = point.Lon
		}
	}

	return
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func setPixel(img *image.Paletted, x, y int, colorIndex uint8) {
	if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
		img.SetColorIndex(x, y, colorIndex)
	}
}

func drawSimpleText(img *image.Paletted, text string, x, y int, colorIndex uint8) {
	// Упрощенная реализация - пропускаем отрисовку текста
	// В реальном проекте здесь лучше использовать библиотеку для рендеринга текста
	_ = text
	_ = x
	_ = y
	_ = colorIndex
}

func computeDelay(fps int) int {
	if fps <= 0 {
		fps = 10
	}
	delay := 100 / fps
	if delay < 1 {
		delay = 1
	}
	return delay
}
// drawScaleBar рисует масштабную линейку на GIF
func drawScaleBar(img *image.Paletted, minLat, maxLat, minLon, maxLon float64, config GIFConfig) {
	if !config.ShowScaleBar {
		return
	}

	// Вычисляем подходящий масштаб
	latSpan := maxLat - minLat
	if latSpan == 0 {
		latSpan = 1.0
	}

	// Приблизительное расстояние на средних широтах (Черное море ~43°N)
	// 1 градус широты ≈ 111 км
	kmPerDegree := 111.0
	spanKm := latSpan * kmPerDegree

	// Определяем длину scale bar
	targetKm := config.ScaleBarKM
	if targetKm <= 0 {
		// Автоматический выбор красивого числа
		targetKm = calculateNiceScaleBarLength(spanKm)
	}

	// Конвертируем км в пиксели
	// Используем текущую проекцию
	scaleKmPerPx := spanKm / float64(config.Height)
	barPx := targetKm / scaleKmPerPx

	// Ограничиваем размер scale bar (10-20% от ширины)
	maxBarPx := float64(config.Width) * 0.2
	minBarPx := 50.0

	if barPx > maxBarPx {
		barPx = maxBarPx
		// Пересчитываем км
		targetKm = barPx * scaleKmPerPx
	} else if barPx < minBarPx {
		barPx = minBarPx
		targetKm = barPx * scaleKmPerPx
	}

	// Позиция scale bar (левый нижний угол)
	padding := 30
	barX := float64(padding)
	barY := float64(config.Height - padding - 20) // 20px для текста
	barHeight := 6.0 // толщина линии в пикселях

	// Рисуем горизонтальную линию
	for x := int(barX); x < int(barX+barPx); x++ {
		for y := int(barY); y < int(barY+barHeight); y++ {
			if x >= 0 && x < config.Width && y >= 0 && y < config.Height {
				img.SetColorIndex(x, y, 13) // белый цвет
			}
		}
	}

	// Рисуем вертикальные засечки на концах
	tickHeight := barHeight + 8
	for y := int(barY - 2); y < int(barY + tickHeight); y++ {
		if y >= 0 && y < config.Height {
			img.SetColorIndex(int(barX), y, 13)              // левая засечка
			img.SetColorIndex(int(barX+barPx), y, 13)       // правая засечка
		}
	}

	// Формируем текст масштаба
	var scaleText string
	if targetKm >= 1.0 {
		scaleText = fmt.Sprintf("%.0f km", targetKm)
	} else {
		scaleText = fmt.Sprintf("%.1f km", targetKm)
	}

	// Рисуем текст (упрощенно)
	textY := int(barY + tickHeight + 12)
	drawSimpleText(img, scaleText, int(barX), textY, 13)
}

// calculateNiceScaleBarLength вычисляет "красивое" число для scale bar
func calculateNiceScaleBarLength(spanKm float64) float64 {
	// Идеальные числа: 10, 25, 50, 100, 250, 500 км
	niceNumbers := []float64{10, 25, 50, 100, 250, 500}

	// Выбираем число, которое составляет 10-20% от span
	targetPercent := 0.15 // 15% от общего span
	targetKm := spanKm * targetPercent

	// Находим ближайшее "красивое" число
	closestIdx := 0
	minDiff := math.Abs(niceNumbers[0] - targetKm)

	for i, num := range niceNumbers {
		diff := math.Abs(num - targetKm)
		if diff < minDiff {
			minDiff = diff
			closestIdx = i
		}
	}

	return niceNumbers[closestIdx]
}

// drawEnhancedColoredCoastline улучшенная версия отрисовки с scale bar
func drawEnhancedColoredCoastline(img *image.Paletted, current, initial []geometry.LatLon, changes []SegmentChangeInfo, minLat, maxLat, minLon, maxLon float64, config GIFConfig) {
	// Сначала рисуем coastline (используем существующую логику)
	drawColoredCoastlineInternal(img, current, initial, changes, minLat, maxLat, minLon, maxLon, config)

	// Затем рисуем scale bar
	drawScaleBar(img, minLat, maxLat, minLon, maxLon, config)
}

// drawColoredCoastline внутренняя версия для совместимости
func drawColoredCoastlineInternal(img *image.Paletted, current, initial []geometry.LatLon, changes []SegmentChangeInfo, minLat, maxLat, minLon, maxLon float64, config GIFConfig) {
	if len(current) < 2 {
		return
	}

	padding := 50
	drawWidth := config.Width - 2*padding
	drawHeight := config.Height - 2*padding

	lonSpan := maxLon - minLon
	if lonSpan == 0 {
		lonSpan = 1.0
	}
	latSpan := maxLat - minLat
	if latSpan == 0 {
		latSpan = 1.0
	}

	scale := math.Min(float64(drawWidth)/lonSpan, float64(drawHeight)/latSpan)
	contentWidth := lonSpan * scale
	contentHeight := latSpan * scale

	originX := float64(padding) + (float64(drawWidth)-contentWidth)/2
	originY := float64(padding) + (float64(drawHeight)-contentHeight)/2

	// Находим максимальную эрозию для нормализации
	maxErosion := 0.0
	maxDeposition := 0.0

	for _, change := range changes {
		if change.TotalErosion > maxErosion {
			maxErosion = change.TotalErosion
		}
		if -change.TotalErosion > maxDeposition {
			maxDeposition = -change.TotalErosion
		}
	}

	maxChange := math.Max(maxErosion, maxDeposition)
	if maxChange == 0 {
		maxChange = 1.0
	}

	// Рисуем сегменты с цветовыми кодами
	for i := 0; i < len(current)-1 && i < len(changes); i++ {
		x1 := int(originX + (current[i].Lon-minLon)*scale)
		y1 := int(originY + contentHeight - (current[i].Lat-minLat)*scale)
		x2 := int(originX + (current[i+1].Lon-minLon)*scale)
		y2 := int(originY + contentHeight - (current[i+1].Lat-minLat)*scale)

		if x1 < 0 || x1 >= config.Width || y1 < 0 || y1 >= config.Height ||
			x2 < 0 || x2 >= config.Width || y2 < 0 || y2 >= config.Height {
			continue
		}

		colorIndex := uint8(2) // дефолтный

		if config.ColorByChange && len(changes) > i {
			change := changes[i]
			normalizedChange := math.Abs(change.TotalErosion) / maxChange

			if change.TotalErosion > 0 {
				if normalizedChange > 0.8 {
					colorIndex = 8
				} else if normalizedChange > 0.6 {
					colorIndex = 7
				} else if normalizedChange > 0.4 {
					colorIndex = 6
				} else if normalizedChange > 0.2 {
					colorIndex = 5
				} else if normalizedChange > 0.05 {
					colorIndex = 4
				} else {
					colorIndex = 3
				}
			} else if change.TotalErosion < 0 {
				if normalizedChange > 0.6 {
					colorIndex = 11
				} else if normalizedChange > 0.3 {
					colorIndex = 10
				} else {
					colorIndex = 9
				}
			}
		}

		drawThickLine(img, x1, y1, x2, y2, colorIndex, 2)
	}

	// Рисуем начальное состояние
	if config.ShowInitial && len(initial) > 1 {
		for i := 0; i < len(initial)-1; i++ {
			x1 := int(originX + (initial[i].Lon-minLon)*scale)
			y1 := int(originY + contentHeight - (initial[i].Lat-minLat)*scale)
			x2 := int(originX + (initial[i+1].Lon-minLon)*scale)
			y2 := int(originY + contentHeight - (initial[i+1].Lat-minLat)*scale)

			if x1 >= 0 && x1 < config.Width && y1 >= 0 && y1 < config.Height &&
				x2 >= 0 && x2 < config.Width && y2 >= 0 && y2 < config.Height {
				drawThickLine(img, x1, y1, x2, y2, 12, 1)
			}
		}
	}
}

// drawColorLegend рисует цветовую легенду для интерпретации цветов
func drawColorLegend(img *image.Paletted, maxErosion, maxDeposition float64, config GIFConfig) {
	if !config.ShowColorLegend || config.ColorLegendPos == "none" {
		return
	}

	// Размеры легенды
	var legendWidth, legendHeight float64
	var legendX, legendY float64

	switch config.ColorLegendPos {
	case "right":
		// Вертикальная легенда справа
		legendWidth = 60
		legendHeight = float64(config.Height - 100)
		legendX = float64(config.Width - int(legendWidth) - 20)
		legendY = 50

	case "bottom":
		// Горизонтальная легенда внизу
		legendWidth = float64(config.Width - 100)
		legendHeight = 50
		legendX = 50
		legendY = float64(config.Height - int(legendHeight) - 20)

	default:
		return
	}

	// Полупрозрачный фон для легенды
	bgColorIndex := uint8(1) // темно-синий фон
	for x := int(legendX); x < int(legendX+legendWidth); x++ {
		for y := int(legendY); y < int(legendY+legendHeight); y++ {
			if x >= 0 && x < config.Width && y >= 0 && y < config.Height {
				img.SetColorIndex(x, y, bgColorIndex)
			}
		}
	}

	// Рамка легенды
	borderColor := uint8(13) // белый
	drawRect(img, int(legendX), int(legendY), int(legendWidth), int(legendHeight), borderColor)

	if config.ColorLegendPos == "right" {
		drawVerticalColorLegend(img, legendX, legendY, legendWidth, legendHeight, maxErosion, maxDeposition)
	} else {
		drawHorizontalColorLegend(img, legendX, legendY, legendWidth, legendHeight, maxErosion, maxDeposition)
	}
}

// drawVerticalColorLegend рисует вертикальную цветовую легенду справа
func drawVerticalColorLegend(img *image.Paletted, legendX, legendY, legendWidth, legendHeight float64, maxErosion, maxDeposition float64) {
	// Параметры градиента
	gradientStart := legendY + 20
	gradientEnd := legendY + float64(legendHeight) - 40
	gradientWidth := 20.0
	gradientX := legendX + float64(legendWidth-20)/2 - gradientWidth/2

	// Рисуем градиент эрозии (сверху вниз: сильная -> слабая)
	erosionColors := []uint8{8, 7, 6, 5, 4, 3} // от темно-красного к желто-зеленому
	stepHeight := (gradientEnd - gradientStart) / float64(len(erosionColors)-1)

	for i, colorIdx := range erosionColors {
		y := gradientStart + float64(i)*stepHeight
		drawRect(img, int(gradientX), int(y), int(gradientWidth), int(stepHeight)+2, colorIdx)
	}

	// Рисуем градиент аккумуляции (ниже эрозии)
	if maxDeposition > 0 {
		depositionStart := gradientEnd + 10
		depositionEnd := legendY + float64(legendHeight) - 20
		depositionHeight := depositionEnd - depositionStart
		if depositionHeight > 20 {
			accColors := []uint8{11, 10, 9} // от темно-синего к светло-голубому
			accStep := depositionHeight / float64(len(accColors)-1)

			for i, colorIdx := range accColors {
				y := depositionStart + float64(i)*accStep
				drawRect(img, int(gradientX), int(y), int(gradientWidth), int(accStep)+2, colorIdx)
			}
		}
	}

	// Заголовки
	legendTitle := "Erosion"
	if maxDeposition > maxErosion {
		legendTitle = "Changes"
	}

	drawSimpleText(img, legendTitle, int(legendX+5), int(legendY+5), 13)

	// Подписи значений
	axisY := int(gradientStart - 5)
	axisY2 := int(gradientEnd + 5)

	drawSimpleText(img, "Max", int(gradientX-25), axisY, 13)   // сильная эрозия
	drawSimpleText(img, "Min", int(gradientX-25), axisY2, 13) // слабая эрозия

	// Численные значения (в метрах)
	drawSimpleText(img, formatMeters(maxErosion), int(legendX+25), axisY, 13)
	drawSimpleText(img, "0m", int(gradientX+25), axisY2, 13)

	// Если есть аккумуляция, добавляем подписи
	if maxDeposition > 0 {
		accY := int(gradientEnd + 10)
		drawSimpleText(img, "Dep.", int(legendX-25), accY, 13)
		drawSimpleText(img, formatMeters(maxDeposition), int(legendX+25), int(legendY+float64(legendHeight)-25), 13)
	}
}

// drawHorizontalColorLegend рисует горизонтальную цветовую легенду снизу
func drawHorizontalColorLegend(img *image.Paletted, legendX, legendY, legendWidth, legendHeight float64, maxErosion, maxDeposition float64) {
	// Параметры градиента
	gradientStart := legendX + 80
	gradientEnd := legendX + float64(legendWidth) - 80
	gradientTop := legendY + 15
	gradientHeight := 12.0

	// Рисуем горизонтальный градиент эрозии
	erosionColors := []uint8{3, 4, 5, 6, 7, 8} // от слабой к сильной
	stepWidth := (gradientEnd - gradientStart) / float64(len(erosionColors)-1)

	for i, colorIdx := range erosionColors {
		x := gradientStart + float64(i)*stepWidth
		drawRect(img, int(x), int(gradientTop), int(stepWidth)+2, int(gradientHeight), colorIdx)
	}

	// Заголовок
	drawSimpleText(img, "Erosion intensity:", int(legendX), int(legendY), 13)

	// Подписи под градиентом
	labelY := int(gradientTop + gradientHeight + 5)
	drawSimpleText(img, "Weak", int(gradientStart), labelY, 13)
	drawSimpleText(img, "Strong", int(gradientEnd-30), labelY, 13)

	// Численные значения
	drawSimpleText(img, formatMeters(maxErosion), int(gradientStart), labelY+15, 13)
	drawSimpleText(img, formatMeters(0), int(gradientEnd-30), labelY+15, 13)

	// Если есть аккумуляция, добавляем справа
	if maxDeposition > 0 {
		accStart := gradientEnd + 20
		drawSimpleText(img, "Deposition:", int(accStart), int(gradientTop), 13)
		drawRect(img, int(accStart+70), int(gradientTop), int(30), int(gradientHeight), 9) // голубой
		drawSimpleText(img, formatMeters(maxDeposition), int(accStart+70), int(gradientTop)+int(gradientHeight)+5, 13)
	}
}

// drawRect рисует прямоугольник
func drawRect(img *image.Paletted, x, y, width, height int, colorIndex uint8) {
	for i := x; i < x+width; i++ {
		for j := y; j < y+height; j++ {
			if i >= 0 && i < img.Bounds().Dx() && j >= 0 && j < img.Bounds().Dy() {
				img.SetColorIndex(i, j, colorIndex)
		}
	}
}
}

// formatMeters форматирует метры в понятный формат
func formatMeters(meters float64) string {
	if math.Abs(meters) < 1.0 {
		return fmt.Sprintf("%.1fm", meters)
	}
	return fmt.Sprintf("%.0fm", meters)
}

// computeMaxErosion вычисляет максимальную эрозию и аккумуляцию
func computeMaxErosion(changes []SegmentChangeInfo) (maxErosion, maxDeposition float64) {
	maxErosion = 0.0
	maxDeposition = 0.0

	for _, change := range changes {
		if change.TotalErosion > maxErosion {
			maxErosion = change.TotalErosion
		}
		if -change.TotalErosion > maxDeposition {
			maxDeposition = -change.TotalErosion
		}
	}

	return maxErosion, maxDeposition
}

// GeoPoint представляет географическую точку с меткой
type GeoPoint struct {
	Name     string
	Lat      float64
	Lon      float64
	Category string // "city", "cape", "bay"
	Priority int    // 1 = major, 2 = minor
}

// blackSeaPoints содержит базу данных географических точек Черного моря
var blackSeaPoints = []GeoPoint{
	// Основные города (major)
	{Name: "Odessa", Lat: 46.4825, Lon: 30.7233, Category: "city", Priority: 1},
	{Name: "Sevastopol", Lat: 44.6167, Lon: 33.5250, Category: "city", Priority: 1},
	{Name: "Batumi", Lat: 41.6423, Lon: 41.6339, Category: "city", Priority: 1},
	{Name: "Varna", Lat: 43.2050, Lon: 27.9100, Category: "city", Priority: 1},
	{Name: "Constanta", Lat: 44.1800, Lon: 28.6300, Category: "city", Priority: 1},
	
	// Дополнительные города (minor)
	{Name: "Yalta", Lat: 44.4930, Lon: 34.1650, Category: "city", Priority: 2},
	{Name: "Sochi", Lat: 43.6028, Lon: 39.7342, Category: "city", Priority: 2},
	{Name: "Trabzon", Lat: 41.0027, Lon: 39.7168, Category: "city", Priority: 2},
	{Name: "Samsun", Lat: 41.2867, Lon: 36.3300, Category: "city", Priority: 2},
	
	// Мысы (major)
	{Name: "Kerch Cape", Lat: 45.3500, Lon: 36.4500, Category: "cape", Priority: 1},
	{Name: "Taman Cape", Lat: 45.3330, Lon: 36.6700, Category: "cape", Priority: 1},
	{Name: "Crimean Cape", Lat: 45.1500, Lon: 33.4500, Category: "cape", Priority: 1},
	
	// Заливы (major)
	{Name: "Karkinit Bay", Lat: 45.7000, Lon: 33.0000, Category: "bay", Priority: 1},
	{Name: "Kalamit Bay", Lat: 45.4000, Lon: 32.8000, Category: "bay", Priority: 1},
}

// drawGeoLabels рисует географические метки на изображении
func drawGeoLabels(img *image.Paletted, minLat, maxLat, minLon, maxLon float64, config GIFConfig) {
	if config.GeoLabels == "none" {
		return
	}

	// Фильтруем точки по выбранному уровню детализации
	pointsToShow := filterGeoPoints(config.GeoLabels)

	for _, point := range pointsToShow {
		// Проверяем, попадает ли точка в видимую область
		if point.Lat < minLat || point.Lat > maxLat || point.Lon < minLon || point.Lon > maxLon {
			continue
		}

		// Преобразуем географические координаты в пиксельные
		x := int(float64(config.Width) * (point.Lon - minLon) / (maxLon - minLon))
		y := int(float64(config.Height) * (1 - (point.Lat - minLat) / (maxLat - minLat)))

		// Проверяем, не выходит ли точка за границы изображения
		if x < 0 || x >= config.Width || y < 0 || y >= config.Height {
			continue
		}

			// Определяем цвет маркера по категории
			markerColor := uint8(8) // оранжевый маркер по умолчанию
			switch point.Category {
			case "city":
				markerColor = 8 // оранжевый
			case "cape":
				markerColor = 7 // желто-оранжевый
			case "bay":
				markerColor = 9 // голубой
			}

		// Рисуем маркер (кружок)
		drawMarker(img, x, y, markerColor)

		// Рисуем текст метки
		drawSimpleText(img, point.Name, x+8, y-4, 11)
	}
}

// filterGeoPoints фильтрует географические точки по уровню детализации
func filterGeoPoints(level string) []GeoPoint {
	var filtered []GeoPoint

	for _, point := range blackSeaPoints {
		switch level {
		case "major":
			if point.Priority == 1 {
				filtered = append(filtered, point)
			}
		case "all":
			filtered = append(filtered, point)
		case "none":
			return nil
		}
	}

	return filtered
}

// drawMarker рисует маркер (кружок) в указанной позиции
func drawMarker(img *image.Paletted, x, y int, colorIndex uint8) {
	radius := 3
	for i := x - radius; i <= x+radius; i++ {
		for j := y - radius; j <= y+radius; j++ {
			if i >= 0 && i < img.Bounds().Dx() && j >= 0 && j < img.Bounds().Dy() {
				// Рисуем круг
				dx := i - x
				dy := j - y
				if dx*dx+dy*dy <= radius*radius {
					img.SetColorIndex(i, j, colorIndex)
				}
			}
		}
	}
}

// drawTimeStamp рисует временные метки на изображении
func drawTimeStamp(img *image.Paletted, frameIndex int, config GIFConfig) {
	if !config.ShowTimeStamp || len(config.TemporalStates) == 0 {
		return
	}

	// Проверяем, есть ли данные для этого кадра
	if frameIndex >= len(config.TemporalStates) {
		return
	}

	state := config.TemporalStates[frameIndex]
	
	// Позиция для временной метки (левый верхний угол)
	marginX := 10
	marginY := 10
	lineHeight := 14

	// Показываем год
	yearText := fmt.Sprintf("Year: %.1f", state.Year)
	drawSimpleText(img, yearText, marginX, marginY, 13)

	// Показываем индикатор шторма
	if state.IsStorm {
		stormText := "⛈️ Storm"
		drawSimpleText(img, stormText, marginX, marginY+lineHeight, 10) // красный цвет для шторма
	}

	// Показываем的其他 временные параметры
	if state.SeaLevelOffset > 0 {
		slrText := fmt.Sprintf("SLR: %.3f m/yr", state.SeaLevelOffset)
		drawSimpleText(img, slrText, marginX, marginY+lineHeight*2, 13)
	}
}

// createOptimizedPalette создает оптимизированную палитру на основе настроек
func createOptimizedPalette(config GIFConfig) color.Palette {
	// Определяем количество цветов
	colors := 16 // default
	if config.Colors > 0 {
		colors = config.Colors
	}
	
	// Ограничиваем диапазон
	if colors < 4 {
		colors = 4
	}
	if colors > 256 {
		colors = 256
	}
	
	// Базовая палитра
	basePalette := getBasePaletteForSize(colors)
	
	// Оптимизация в зависимости от уровня сжатия
	switch config.Compression {
	case "low":
		// Низкое сжатие - максимальное качество
		return basePalette
	case "high":
		// Высокое сжатие - уменьшаем палитру
		return reducePalette(basePalette, colors/2)
	default: // "medium"
		return basePalette
	}
}

// getBasePaletteForSize возвращает базовую палитру для указанного размера
func getBasePaletteForSize(size int) color.Palette {
	// Научная палитра для эрозии
	if size <= 8 {
		// Минимальная палитра для очень маленького размера
		return color.Palette{
			color.RGBA{0, 30, 60, 255},        // 0: темно-синяя вода
			color.RGBA{240, 240, 240, 255},     // 1: белый берег
			color.RGBA{34, 139, 34, 255},       // 2: зеленая эрозия
			color.RGBA{255, 140, 0, 255},       // 3: оранжевая эрозия
			color.RGBA{178, 34, 34, 255},        // 4: красная эрозия
			color.RGBA{135, 206, 235, 255},      // 5: голубая аккумуляция
			color.RGBA{255, 255, 255, 255},      // 6: белый текст
			color.RGBA{169, 169, 169, 255},      // 7: серый текст
		}
	}
	
	// Стандартная научная палитра (16 цветов)
	return createHighQualityPalette()
}

// reducePalette уменьшает палитру до указанного размера
func reducePalette(palette color.Palette, targetSize int) color.Palette {
	if len(palette) <= targetSize {
		return palette
	}
	
	// Сохраняем самые важные цвета
	important := []int{0, 1, len(palette)-1} // первый, второй, последний
	result := make(color.Palette, 0, targetSize)
	
	// Добавляем важные цвета
	for _, idx := range important {
		if idx < len(palette) {
			result = append(result, palette[idx])
		}
	}
	
	// Добавляем промежуточные цвета равномерно
	step := (len(palette) - len(important)) / (targetSize - len(important))
	added := make(map[int]bool)
	for _, idx := range important {
		added[idx] = true
	}
	
	for i := 0; i < len(palette) && len(result) < targetSize; i += step {
		if !added[i] {
			result = append(result, palette[i])
			added[i] = true
		}
	}
	
	// Если еще не достигли целевого размера, добавляем оставшиеся
	for i := 0; i < len(palette) && len(result) < targetSize; i++ {
		if !added[i] {
			result = append(result, palette[i])
		}
	}
	
	return result
}

// createHighQualityPalette создает научную палитру высокого качества
func createHighQualityPalette() color.Palette {
	return color.Palette{
		// Фон и вода
		color.RGBA{15, 30, 50, 255},     // 0: глубокая вода
		color.RGBA{25, 50, 80, 255},     // 1: вода

		// Цвета эрозии (градиент от слабой к сильной)
		color.RGBA{100, 200, 100, 255},   // 2: стабильный/слабая эрозия (светло-зеленый)
		color.RGBA{150, 180, 80, 255},    // 3: слабая эрозия (желто-зеленый)
		color.RGBA{200, 160, 60, 255},    // 4: умеренная эрозия (желто-оранжевый)
		color.RGBA{230, 120, 40, 255},    // 5: средняя эрозия (оранжевый)
		color.RGBA{250, 80, 30, 255},     // 6: сильная эрозия (красно-оранжевый)
		color.RGBA{255, 40, 20, 255},     // 7: очень сильная эрозия (ярко-красный)
		color.RGBA{200, 30, 10, 255},     // 8: экстремальная эрозия (темно-красный)

		// Цвета аккумуляции
		color.RGBA{50, 150, 200, 255},    // 9: слабая аккумуляция (светло-голубой)
		color.RGBA{30, 100, 180, 255},    // 10: умеренная аккумуляция (голубой)
		color.RGBA{20, 80, 160, 255},     // 11: сильная аккумуляция (синий)

		// Начальное состояние (серое)
		color.RGBA{120, 120, 120, 255},   // 12: начальная линия

		// Текст
		color.RGBA{255, 255, 240, 255},   // 13: текст (теплый белый)
		color.RGBA{40, 40, 40, 255},       // 14: темный текст
	}
}
