package svg

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
)

// ExampleEnhancedVisualization demonstrates how to use enhanced SVG features
func ExampleEnhancedVisualization() {
	// Sample coastline points (simplified Black Sea region)
	points := []geometry.LatLon{
		{Lat: 43.5, Lon: 28.5}, {Lat: 43.6, Lon: 28.7},
		{Lat: 43.7, Lon: 29.0}, {Lat: 43.5, Lon: 29.3},
		{Lat: 43.3, Lon: 29.5}, {Lat: 43.2, Lon: 29.2},
		{Lat: 43.1, Lon: 28.8}, {Lat: 43.3, Lon: 28.5},
	}

	// Create enhanced document with additional map elements
	_ = EnhancedDocument{
		Document: Document{
			Title:    "Береговая линия Чёрного моря",
			Subtitle: "Эрозия с дополнительными элементами карты",
			Layers: []Layer{
				{
					Label:       "Исходная линия",
					Points:      points,
					LengthKM:    geometry.PolylineLength(points),
					Stroke:      "#1f6f8b",
					StrokeWidth: 2.5,
					Opacity:     1.0,
				},
			},
		},
		GridOptions: &GridOptions{
			Show:          true,
			ShowLatLabels: true,
			ShowLonLabels: true,
			LatStep:       0.2, // Every 0.2 degrees
			LonStep:       0.2,
			LineColor:     "#d6d0c4",
			LabelColor:    "#8a9aa6",
			FontSize:      9,
			Opacity:       0.5,
			DashArray:     "3 3",
		},
		CompassOptions: &CompassOptions{
			Show:          true,
			Size:          70,
			WindDirection: 315, // NW wind direction
			ShowWindArrow: true,
			Label:         "Направление ветра",
			Style:         "modern", // or "classic", "minimal"
		},
		MarkerOptions: &MarkerOptions{
			Show: true,
			Markers: []Marker{
				{
					Lat:     43.5,
					Lon:     28.5,
					Label:   "Начало",
					Color:   "#2d6a4f",
					Size:    10,
					Shape:   "circle",
					Tooltip: "Начальная точка измерений",
				},
				{
					Lat:     43.3,
					Lon:     29.5,
					Label:   "Конец",
					Color:   "#c2410c",
					Size:    10,
					Shape:   "diamond",
					Tooltip: "Конечная точка измерений",
				},
			},
			DefaultSize:  8,
			DefaultColor: "#c2410c",
			ShowLabels:   true,
		},
		IsolineOptions: &IsolineOptions{
			Show:      false, // Set to true if bathymetry data is available
			DepthStep: 50,    // 50 meters between contours
			MinDepth:  -200,
			MaxDepth:  0,
			LineColor: "#4a90b8",
			LabelColor: "#2c5f7a",
			LineWidth: 1.0,
			Opacity:   0.4,
		},
	}

	fmt.Println("Enhanced document created with:")
	fmt.Println("- Coordinate grid with labels")
	fmt.Println("- Compass/wind rose showing wind direction")
	fmt.Println("- Markers for key points (start/end)")
	fmt.Println("- Isoline options for depth contours")

	// To actually generate the SVG:
	// err := DrawEnhancedSVG(doc, "output_enhanced.svg")
	// if err != nil {
	//     log.Fatal(err)
	// }
}

// ExampleCompassStyles demonstrates different compass styles
func ExampleCompassStyles() {
	styles := []string{"modern", "classic", "minimal"}

	for _, style := range styles {
		opts := CompassOptions{
			Show:          true,
			Size:          60,
			WindDirection: 45, // NE wind
			ShowWindArrow: true,
			Label:         "Ветер " + style,
			Style:         style,
		}

		fmt.Printf("Compass style: %s\n", style)
		fmt.Printf("  Size: %.0fpx\n", opts.Size)
		fmt.Printf("  Wind direction: %.0f°\n", opts.WindDirection)
	}
}

// ExampleGridConfiguration demonstrates different grid configurations
func ExampleGridConfiguration() {
	// Fine grid for detailed maps
	fineGrid := GridOptions{
		Show:          true,
		ShowLatLabels: true,
		ShowLonLabels: true,
		LatStep:       0.1, // Every 0.1 degrees (~11km)
		LonStep:       0.1,
		LineColor:     "#d6d0c4",
		LabelColor:    "#8a9aa6",
		FontSize:      8,
		Opacity:       0.4,
		DashArray:     "2 2",
	}

	// Coarse grid for overview maps
	coarseGrid := GridOptions{
		Show:          true,
		ShowLatLabels: true,
		ShowLonLabels: true,
		LatStep:       1.0, // Every 1 degree (~111km)
		LonStep:       1.0,
		LineColor:     "#8a9aa6",
		LabelColor:    "#6b7a87",
		FontSize:      11,
		Opacity:       0.7,
		DashArray:     "5 5",
	}

	fmt.Printf("Fine grid: %.2f° step\n", fineGrid.LatStep)
	fmt.Printf("Coarse grid: %.1f° step\n", coarseGrid.LatStep)
}

// ExampleMarkerShapes demonstrates different marker shapes
func ExampleMarkerShapes() {
	shapes := []string{"circle", "square", "diamond", "triangle"}

	for _, shape := range shapes {
		marker := Marker{
			Lat:     44.0,
			Lon:     38.0,
			Label:   shape,
			Color:   "#1f6f8b",
			Size:    12,
			Shape:   shape,
			Tooltip: "Example " + shape,
		}

		fmt.Printf("Marker: %s, color: %s, size: %.0f\n",
			marker.Shape, marker.Color, marker.Size)
	}
}
