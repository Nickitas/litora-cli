# Enhanced SVG Visualization Features

## Overview

The enhanced SVG visualization module adds professional cartographic elements to coastline maps, including coordinate grids, compass/wind roses, key point markers, and depth contour lines (isolines).

## New Components

### 1. Coordinate Grid

Displays latitude and longitude grid lines with optional labels.

```go
GridOptions{
    Show:          true,     // Enable grid
    ShowLatLabels: true,     // Show latitude labels
    ShowLonLabels: true,     // Show longitude labels
    LatStep:       0.2,     // Degrees between latitude lines
    LonStep:       0.2,     // Degrees between longitude lines
    LineColor:     "#d6d0c4", // Grid line color
    LabelColor:    "#8a9aa6", // Label text color
    FontSize:      9,       // Label font size
    Opacity:       0.5,     // Line opacity (0-1)
    DashArray:     "3 3",   // Dash pattern (CSS style)
}
```

**Usage Example:**
```go
doc := EnhancedDocument{
    // ... other fields ...
    GridOptions: &GridOptions{
        Show:          true,
        ShowLatLabels: true,
        ShowLonLabels: true,
        LatStep:       0.2,
        LonStep:       0.2,
    },
}
```

### 2. Compass / Wind Rose

Displays a compass rose with optional wind direction arrow (useful for wave erosion visualization).

```go
CompassOptions{
    Show:          true,    // Enable compass
    X:             0,       // X position (0 = auto, top-left)
    Y:             0,       // Y position (0 = auto, top-left)
    Size:          70,      // Compass size in pixels
    WindDirection: 315,     // Wind direction (degrees from north)
    ShowWindArrow: true,    // Show wind direction arrow
    Label:         "Wind",  // Optional label text
    Style:         "modern", // "modern", "classic", or "minimal"
}
```

**Styles:**
- **modern**: Clean, minimalist design with cardinal directions
- **classic**: Traditional compass star with decorative elements
- **minimal**: Simple circle with N indicator and tick marks

### 3. Point Markers

Marks key locations on the map (start/end points, measurement stations, etc.)

```go
MarkerOptions{
    Show: true,
    Markers: []Marker{
        {
            Lat:     43.5,
            Lon:     28.5,
            Label:   "Start",
            Color:   "#2d6a4f",
            Size:    10,
            Shape:   "circle",   // "circle", "square", "diamond", "triangle"
            Tooltip: "Starting point",
        },
        {
            Lat:     43.3,
            Lon:     29.5,
            Label:   "End",
            Color:   "#c2410c",
            Size:    10,
            Shape:   "diamond",
            Tooltip: "Ending point",
        },
    },
    DefaultSize:  8,
    DefaultColor: "#c2410c",
    ShowLabels:   true,
}
```

**Marker Shapes:**
- `circle`: Circular marker
- `square`: Square marker
- `diamond`: Diamond-shaped marker
- `triangle`: Triangle marker

### 4. Depth Contour Lines (Isolines)

Displays bathymetry contour lines showing water depth.

```go
IsolineOptions{
    Show:      true,    // Enable isolines
    DepthStep: 50,      // Meters between contour lines
    MinDepth:  -200,    // Minimum depth to display
    MaxDepth:  0,       // Maximum depth (sea level)
    LineColor: "#4a90b8", // Contour line color
    LabelColor: "#2c5f7a", // Depth label color
    LineWidth: 1.0,     // Line width
    Opacity:   0.4,     // Line opacity
    LabelInterval: 2,   // Label every Nth line
}
```

**Note:** Requires bathymetry data. The `BathymetryGrid` interface must implement `GetIsolinePoints(depthStep float64) []Isoline`.

## Complete Example

```go
package main

import (
    "coastal-geometry/internal/domain/geometry"
    "coastal-geometry/internal/render/svg"
    "log"
)

func main() {
    points := []geometry.LatLon{
        {Lat: 43.5, Lon: 28.5}, {Lat: 43.6, Lon: 28.7},
        {Lat: 43.7, Lon: 29.0}, {Lat: 43.5, Lon: 29.3},
        {Lat: 43.3, Lon: 29.5}, {Lat: 43.2, Lon: 29.2},
        {Lat: 43.1, Lon: 28.8}, {Lat: 43.3, Lon: 28.5},
    }

    doc := svg.EnhancedDocument{
        Document: svg.Document{
            Title:    "Black Sea Coastline",
            Subtitle: "Enhanced visualization with cartographic elements",
            Layers: []svg.Layer{
                {
                    Label:       "Original coastline",
                    Points:      points,
                    LengthKM:    geometry.PolylineLength(points),
                    Stroke:      "#1f6f8b",
                    StrokeWidth: 2.5,
                    Opacity:     1.0,
                },
            },
        },
        GridOptions: &svg.GridOptions{
            Show:          true,
            ShowLatLabels: true,
            ShowLonLabels: true,
            LatStep:       0.2,
            LonStep:       0.2,
        },
        CompassOptions: &svg.CompassOptions{
            Show:          true,
            Size:          70,
            WindDirection: 315,
            ShowWindArrow: true,
            Style:         "modern",
        },
        MarkerOptions: &svg.MarkerOptions{
            Show: true,
            Markers: []svg.Marker{
                {
                    Lat:   43.5, Lon: 28.5,
                    Label: "Start", Color: "#2d6a4f",
                    Size:  10, Shape: "circle",
                },
                {
                    Lat:   43.3, Lon: 29.5,
                    Label: "End", Color: "#c2410c",
                    Size:  10, Shape: "diamond",
                },
            },
            ShowLabels: true,
        },
    }

    err := svg.DrawEnhancedSVG(doc, "output_enhanced.svg")
    if err != nil {
        log.Fatal(err)
    }
}
```

## Integration with Existing Code

The enhanced visualization is fully compatible with existing `Document` structures. Simply wrap your existing document in an `EnhancedDocument` and add the desired options:

```go
// Existing code
doc := svg.Document{
    Title:  "My Map",
    Layers: []svg.Layer{...},
}

// Enhanced version
enhancedDoc := svg.EnhancedDocument{
    Document:      doc,
    GridOptions:   &svg.GridOptions{Show: true},
    CompassOptions: &svg.CompassOptions{Show: true},
}
```

## Performance Considerations

- **Grid rendering**: O(n) where n is the number of grid lines (typically < 100)
- **Compass rendering**: O(1) - fixed complexity regardless of map size
- **Markers**: O(m) where m is the number of markers
- **Isolines**: O(k) where k is the number of contour points (can be large for detailed bathymetry)

For best performance with isolines, consider:
- Using larger `DepthStep` values (e.g., 100m instead of 50m)
- Limiting the depth range with `MinDepth`/`MaxDepth`
- Simplifying bathymetry data for overview maps

## Future Enhancements

Potential additions to consider:
- Animated SVG elements for erosion steps
- Interactive tooltips (using SVG `<title>` elements)
- Gradient fills for depth visualization
- Time-series animation support
- Export to additional formats (PNG, PDF)
