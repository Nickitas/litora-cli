package cli

import (
	"coastal-geometry/internal/domain/geometry"
	svgrender "coastal-geometry/internal/render/svg"
	"fmt"
)

// buildEnhancedOptions создаёт расширенные настройки SVG из конфигурации
// Функция формирует опции для отображения сетки, компаса, маркеров и изолиний
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
			ShowWindArrow: true,
			Label:         "Ветер",
			Style:         cfg.CompassStyle,
		}
	}

	var markerOpts *svgrender.MarkerOptions
	if cfg.ShowMarkers && len(points) > 1 {
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

	var isolineOpts *svgrender.IsolineOptions
	if cfg.ShowIsolines {
		isolineOpts = &svgrender.IsolineOptions{
			Show:          true,
			DepthStep:     50,
			MinDepth:      -200,
			MaxDepth:      0,
			LineColor:     "#4a90b8",
			LabelColor:    "#2c5f7a",
			LineWidth:     1.0,
			Opacity:       0.4,
			LabelInterval: 2,
		}
	}

	return &svgrender.EnhancedDocument{
		GridOptions:    gridOpts,
		CompassOptions: compassOpts,
		MarkerOptions:  markerOpts,
		IsolineOptions: isolineOpts,
	}
}

// wrapDocumentForEnhanced преобразует Document в EnhancedDocument с настройками конфигурации
// Добавляет визуализацию транспорта наносов, если доступны данные
func wrapDocumentForEnhanced(doc svgrender.Document, cfg config, points []geometry.LatLon, waveDir float64, sedimentResult *geometry.SedimentTransportResult, renderPoints []geometry.LatLon) svgrender.EnhancedDocument {
	enhanced := buildEnhancedOptions(cfg, points, waveDir)
	if enhanced == nil {
		return svgrender.EnhancedDocument{Document: doc}
	}

	enhanced.Document = doc

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

		enhanced.SedimentTransportOptions = &svgrender.SedimentTransportOptions{
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
	} else {
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

	return *enhanced
}
