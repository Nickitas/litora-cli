package cli

import (
	"coastal-geometry/internal/domain/geometry"
	svgrender "coastal-geometry/internal/render/svg"
)

// buildEnhancedOptions creates enhanced SVG options from config
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

		// Add end marker if different from start
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
			Show:         true,
			DepthStep:    50,
			MinDepth:     -200,
			MaxDepth:     0,
			LineColor:    "#4a90b8",
			LabelColor:   "#2c5f7a",
			LineWidth:    1.0,
			Opacity:      0.4,
			LabelInterval: 2,
		}
	}

	return &svgrender.EnhancedDocument{
		GridOptions:    gridOpts,
		CompassOptions: compassOpts,
		MarkerOptions:  markerOpts,
		IsolineOptions:  isolineOpts,
	}
}

// wrapDocumentForEnhanced converts Document to EnhancedDocument with config options
func wrapDocumentForEnhanced(doc svgrender.Document, cfg config, points []geometry.LatLon, waveDir float64) svgrender.EnhancedDocument {
	enhanced := buildEnhancedOptions(cfg, points, waveDir)
	if enhanced == nil {
		return svgrender.EnhancedDocument{Document: doc}
	}

	enhanced.Document = doc
	return *enhanced
}
