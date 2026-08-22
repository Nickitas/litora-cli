package cli

import (
	"coastal-geometry/internal/domain/geometry"
	svgrender "coastal-geometry/internal/render/svg"
	"fmt"
	"math"
)

// adaptiveGridParameters возвращает размер ячейки и буфер вокруг береговой
// линии для научной визуализации. Параметры зависят от протяжённости области.
func adaptiveGridParameters(points []geometry.LatLon) (cellSizeMeters, bufferKM, maxSpanMeters float64) {
	if len(points) == 0 {
		return 0, 0, 0
	}

	minLat, maxLat := points[0].Lat, points[0].Lat
	minLon, maxLon := points[0].Lon, points[0].Lon
	for _, point := range points[1:] {
		minLat = math.Min(minLat, point.Lat)
		maxLat = math.Max(maxLat, point.Lat)
		minLon = math.Min(minLon, point.Lon)
		maxLon = math.Max(maxLon, point.Lon)
	}

	projection := geometry.NewLocalMetricProjection(points)
	latSpan := (maxLat - minLat) * projection.MetersPerDegreeLatitude
	lonSpan := (maxLon - minLon) * projection.MetersPerDegreeLongitude
	maxSpanMeters = math.Max(latSpan, lonSpan)

	switch {
	case maxSpanMeters > 1000000:
		cellSizeMeters = 5000
	case maxSpanMeters > 500000:
		cellSizeMeters = 3000
	case maxSpanMeters > 200000:
		cellSizeMeters = 2000
	case maxSpanMeters > 100000:
		cellSizeMeters = 1000
	default:
		cellSizeMeters = 500
	}

	switch {
	case maxSpanMeters > 1000000:
		bufferKM = math.Min(maxSpanMeters*0.05, 100000) / 1000
	case maxSpanMeters > 500000:
		bufferKM = math.Min(maxSpanMeters*0.08, 80000) / 1000
	case maxSpanMeters > 100000:
		bufferKM = math.Min(maxSpanMeters*0.10, 50000) / 1000
	default:
		bufferKM = 10
	}

	return cellSizeMeters, bufferKM, maxSpanMeters
}

// buildEnhancedOptions создаёт расширенные настройки SVG из конфигурации
// Функция формирует опции для отображения сетки, компаса и маркеров.
func buildEnhancedOptions(cfg config, points []geometry.LatLon, waveDir float64) *svgrender.EnhancedDocument {
	if !cfg.EnableEnhanced {
		return nil
	}

	var gridOpts *svgrender.GridOptions
	if cfg.ShowGrid {
		gridOpts = &svgrender.GridOptions{
			Show:          true,
			ShowLatLabels: true,
			ShowLonLabels: true,
			LatStep:       cfg.GridStep,
			LonStep:       cfg.GridStep,
			LineColor:     "#d6d0c4",
			LabelColor:    "#8a9aa6",
			FontSize:      9,
			Opacity:       0.5,
			DashArray:     "3 3",
		}
	}

	var compassOpts *svgrender.CompassOptions
	if cfg.ShowCompass {
		compassOpts = &svgrender.CompassOptions{
			Show:          true,
			Size:          float64(cfg.CompassSize),
			WindDirection: waveDir,
			ShowWindArrow: cfg.Command != cmdDimension && cfg.Command != cmdKochDemo,
			Label:         "",
			Style:         cfg.CompassStyle,
		}
		if cfg.Command == cmdDimension || cfg.Command == cmdKochDemo {
			compassOpts.WindDirection = -1
		}
	}

	var markerOpts *svgrender.MarkerOptions
	if cfg.ShowMarkers && len(points) > 1 && cfg.Command != cmdDimension && cfg.Command != cmdKochDemo && cfg.Command != cmdErosion {
		markers := []svgrender.Marker{
			{
				Lat:     points[0].Lat,
				Lon:     points[0].Lon,
				Label:   "Начало",
				Color:   "#2d6a4f",
				Size:    10,
				Shape:   "circle",
				Tooltip: "Начальная точка",
			},
		}

		// Добавляем маркер конца если отличается от начала
		lastIdx := len(points) - 1
		if lastIdx > 0 {
			lastPoint := points[lastIdx]
			if lastPoint.Lat != points[0].Lat || lastPoint.Lon != points[0].Lon {
				markers = append(markers, svgrender.Marker{
					Lat:     lastPoint.Lat,
					Lon:     lastPoint.Lon,
					Label:   "Конец",
					Color:   "#c2410c",
					Size:    10,
					Shape:   "diamond",
					Tooltip: "Конечная точка",
				})
			}
		}

		markerOpts = &svgrender.MarkerOptions{
			Show:         true,
			Markers:      markers,
			DefaultSize:  8,
			DefaultColor: "#c2410c",
			ShowLabels:   true,
		}
	}

	return &svgrender.EnhancedDocument{
		GridOptions:    gridOpts,
		CompassOptions: compassOpts,
		MarkerOptions:  markerOpts,
	}
}

// wrapDocumentForEnhanced преобразует Document в EnhancedDocument с настройками конфигурации
// Добавляет визуализацию транспорта наносов, если доступны данные
func wrapDocumentForEnhanced(doc svgrender.Document, cfg config, points []geometry.LatLon, waveDir float64, sedimentResult *geometry.SedimentTransportResult, renderPoints []geometry.LatLon) svgrender.EnhancedDocument {
	modeName := "Научный SVG-режим"
	if cfg.ScenarioStatus == geometry.ScenarioStatusDemo {
		modeName = "Демонстрационный SVG-режим"
	} else if cfg.ScenarioStatus == geometry.ScenarioStatusUnclassified {
		modeName = "SVG-режим без подтверждённого исследовательского статуса"
	} else if cfg.Command == cmdKochDemo {
		modeName = "Учебный SVG-режим"
	} else if cfg.Command == cmdErosion {
		modeName = "Расчётный SVG-режим"
	}
	fmt.Printf("🔧 %s: включён=%v, точек=%d\n", modeName, cfg.EnableEnhanced, len(points))

	var enhanced *svgrender.EnhancedDocument

	if cfg.EnableEnhanced {
		enhanced = buildEnhancedOptions(cfg, points, waveDir)
		fmt.Printf("   Расширенные элементы карты подготовлены: %v\n", enhanced != nil)
	}

	// Create base enhanced document
	enhancedDoc := svgrender.EnhancedDocument{Document: doc}
	if enhanced != nil {
		enhancedDoc.GridOptions = enhanced.GridOptions
		enhancedDoc.CompassOptions = enhanced.CompassOptions
		enhancedDoc.MarkerOptions = enhanced.MarkerOptions
	}
	enhancedDoc.MinimalMap = cfg.Command == cmdDimension || cfg.Command == cmdKochDemo || cfg.Command == cmdErosion
	if enhancedDoc.MinimalMap {
		// На box-counting-карте аналитическая сетка заменяет координатную.
		enhancedDoc.GridOptions = nil
	}

	// Добавляем визуализацию box-counting сетки для фрактального анализа
	if cfg.EnableEnhanced && len(points) > 2 {
		fmt.Printf("   🔧 Подготовка сетки box-counting для %d точек\n", len(points))

		// Calculate bounding box
		minLat, maxLat := points[0].Lat, points[0].Lat
		minLon, maxLon := points[0].Lon, points[0].Lon
		for _, p := range points[1:] {
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

		fmt.Printf("   📍 Исходная область: широта [%.3f, %.3f], долгота [%.3f, %.3f]\n", minLat, maxLat, minLon, maxLon)

		// Размер сетки зависит от протяжённости области, но при наличии
		// расчётного размера используется именно он.
		optimalCellSize, bufferKM, maxSpan := adaptiveGridParameters(points)
		if cfg.BoxCountingBoxSize > 0 {
			optimalCellSize = cfg.BoxCountingBoxSize
			fmt.Printf("   📏 Размер сетки взят из расчёта box-counting: %.0f м\n", optimalCellSize)
		}

		if cfg.BoxCountingBoxSize <= 0 {
			fmt.Printf("   📐 Размер сетки для визуализации: область %.0f км -> размер %.0f м\n", maxSpan/1000, optimalCellSize)
		}

		fmt.Printf("   🎯 Буферная зона вокруг берега: %.0f км\n", bufferKM)

		// Add box-counting grid for visualization (using adaptive cell size and buffer zone)
		enhancedDoc.BoxCountingGridOptions = &svgrender.BoxCountingGridOptions{
			Show:               true,
			Points:             points,
			BoxSize:            optimalCellSize,
			MinLat:             minLat,
			MaxLat:             maxLat,
			MinLon:             minLon,
			MaxLon:             maxLon,
			ShowCoveredBoxes:   true,
			ShowAllBoxes:       true,
			ShowCoverageDegree: true, // enable color coding by coverage degree
			CoveredColor:       "rgba(193, 65, 12, 0.3)",
			UncoveredColor:     "none",
			LineColor:          "#8794a0",
			LineWidth:          0.6,
			Opacity:            0.32,
			LabelScaleFactors:  []float64{64, 128, 256}, // дополнительные подписи масштабов
			BufferZoneKM:       bufferKM,                // use buffer zone instead of full sea
			ContextGrid:        false,                   // can be enabled for context grid
			RegressionWindow:   cfg.BoxCountingRegressionMin > 0 && cfg.BoxCountingRegressionMax > 0 && optimalCellSize >= cfg.BoxCountingRegressionMin && optimalCellSize <= cfg.BoxCountingRegressionMax,
			RegressionMinBox:   cfg.BoxCountingRegressionMin,
			RegressionMaxBox:   cfg.BoxCountingRegressionMax,
			LogLogSVGFile:      cfg.BoxCountingLogLogSVGFile,
		}

		fmt.Printf("   ✅ Сетка box-counting настроена: включена=%v, размер ячейки=%.0f м\n",
			enhancedDoc.BoxCountingGridOptions.Show, enhancedDoc.BoxCountingGridOptions.BoxSize)
	} else {
		fmt.Printf("   ⚠️  Сетка box-counting пропущена: включено=%v, точек=%d\n", cfg.EnableEnhanced, len(points))
	}

	// Добавляем визуализацию транспорта наносов если доступны данные
	if sedimentResult != nil && len(sedimentResult.States) > 0 && cfg.EnableEnhanced {
		// Используем точки рендера если доступны, иначе исходные точки
		displayPoints := renderPoints
		if len(displayPoints) == 0 {
			displayPoints = points
		}

		// Debug output
		fmt.Printf("🔧 Добавление визуализации транспорта наносов: %d состояний, %d точек\n",
			len(sedimentResult.States), len(displayPoints))

		// Подсчёт точек аккумуляции и эрозии
		accumCount := 0
		erosionCount := 0
		for _, state := range sedimentResult.States {
			if state.IsAccumulating {
				accumCount++
			}
			if state.IsEroding {
				erosionCount++
			}
		}
		fmt.Printf("   📊 Статистика наносов: %d аккумуляция, %d эрозия точек\n",
			accumCount, erosionCount)

		enhancedDoc.SedimentTransportOptions = &svgrender.SedimentTransportOptions{
			Show:                 true,
			Points:               displayPoints,
			SedimentStates:       sedimentResult.States,
			ShowAccumulation:     true,
			ShowErosion:          true,
			ShowTransportVectors: true,
			AccumulationColor:    "#2d6a4f", // green
			ErosionColor:         "#c2410c", // red
			VectorColor:          "#1f6f8b", // blue
			VectorScale:          1000,
			MarkerSize:           8,
		}
	} else if cfg.Command == cmdErosion {
		fmt.Printf("⚠️  Визуализация транспорта наносов пропущена: результат=%v, состояний=%d, улучшено=%v\n",
			sedimentResult != nil,
			func() int {
				if sedimentResult != nil {
					return len(sedimentResult.States)
				}
				return 0
			}(),
			cfg.EnableEnhanced)
	}

	return enhancedDoc
}

// wrapErosionDocumentForEnhanced преобразует Document в EnhancedDocument с erosion grid
func wrapErosionDocumentForEnhanced(doc svgrender.Document, cfg config, points []geometry.LatLon, waveOptions geometry.WaveErosionOptions) svgrender.EnhancedDocument {
	fmt.Printf("🔧 wrapErosionDocumentForEnhanced: EnableEnhanced=%v, points=%d, waveDir=%.0f\n",
		cfg.EnableEnhanced, len(points), waveOptions.WindSourceDirectionDeg)

	enhanced := buildEnhancedOptions(cfg, points, waveOptions.WindSourceDirectionDeg)
	if enhanced == nil {
		fmt.Printf("   ⚠️  buildEnhancedOptions returned nil\n")
		return svgrender.EnhancedDocument{Document: doc}
	}

	enhanced.Document = doc
	enhanced.MinimalMap = cfg.Command == cmdDimension || cfg.Command == cmdKochDemo || cfg.Command == cmdErosion
	if enhanced.MinimalMap {
		// Аналитическая сетка заменяет координатную на научной карте.
		enhanced.GridOptions = nil
	}

	// Add erosion grid for wave visualization
	if cfg.EnableEnhanced && len(points) > 2 && waveOptions.WindSourceDirectionDeg >= 0 {
		// Calculate bounding box
		minLat, maxLat := points[0].Lat, points[0].Lat
		minLon, maxLon := points[0].Lon, points[0].Lon
		for _, p := range points[1:] {
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

		optimalCellSize, bufferKM, _ := adaptiveGridParameters(points)

		fmt.Printf("   📐 Размер ячейки эрозионной сетки: %.0f м\n", optimalCellSize)
		fmt.Printf("   🎯 Буферная зона для эрозии: %.0f км\n", bufferKM)

		// Add erosion cell grid for wave modeling visualization
		enhanced.ErosionGridOptions = &svgrender.ErosionGridOptions{
			Show:            true,
			Points:          points,
			CellSize:        optimalCellSize,
			MinLat:          minLat,
			MaxLat:          maxLat,
			MinLon:          minLon,
			MaxLon:          maxLon,
			ShowCells:       true,
			ShowWaveVectors: true,
			WaveDirection:   waveOptions.WindSourceDirectionDeg,
			LineColor:       "#8794a0",
			LineWidth:       0.6,
			Opacity:         0.32,
			VectorColor:     "#c2410c",
			VectorLength:    14.0,
			BufferZoneKM:    bufferKM, // use buffer zone
		}

		fmt.Printf("   ✅ ErosionGridOptions set: Show=%v, CellSize=%.0f\n",
			enhanced.ErosionGridOptions.Show, enhanced.ErosionGridOptions.CellSize)
	} else {
		fmt.Printf("   ⚠️  Skipping erosion grid: EnableEnhanced=%v, points=%d, waveDir=%.0f\n",
			cfg.EnableEnhanced, len(points), waveOptions.WindSourceDirectionDeg)
	}

	return *enhanced
}
