# Sediment Transport Visualization

## Overview

The sediment transport visualization feature provides comprehensive visual analysis of sediment dynamics along coastlines, including:
- **Accumulation zones** - areas where sediment deposits
- **Erosion points** - areas where sediment is being removed  
- **Longshore drift vectors** - directional arrows showing sediment transport paths

## Features

### 1. Accumulation/Erosion Mapping
- Green circles mark accumulation zones where sediment builds up
- Red circles mark erosion points where sediment is lost
- Size and opacity are configurable for different visual emphasis

### 2. Transport Vectors
- Blue arrows show the direction and magnitude of longshore drift
- Arrow length is proportional to sediment volume being transported
- Vectors automatically connect neighboring points based on sediment flow

### 3. Statistical Integration
- Automatically integrates with sediment transport calculations
- Displays accumulation/erosion point counts
- Shows total transport volumes and mass balance

## Usage

### Basic Example

```go
import (
    "coastal-geometry/internal/domain/geometry"
    svgrender "coastal-geometry/internal/render/svg"
)

// Your coastline points
points := []geometry.LatLon{
    {Lat: 43.5, Lon: 28.5}, 
    {Lat: 43.6, Lon: 28.7},
    // ... more points
}

// Your sediment transport calculation results
sedimentResult := geometry.CalculateSedimentTransport(
    points, erosionRates, waveData, lithology, params,
)

// Create enhanced document with sediment visualization
doc := svgrender.EnhancedDocument{
    Document: svgrender.Document{
        Title:  "Sediment Transport Analysis",
        Layers: []svgrender.Layer{
            {
                Label:  "Coastline",
                Points: points,
                Stroke: "#1f6f8b",
            },
        },
    },
    SedimentTransportOptions: &svgrender.SedimentTransportOptions{
        Show:                true,
        Points:              points,
        SedimentStates:      sedimentResult.States,
        ShowAccumulation:    true,
        ShowErosion:         true,
        ShowTransportVectors: true,
        AccumulationColor:   "#2d6a4f", // green
        ErosionColor:        "#c2410c", // red
        VectorColor:         "#1f6f8b", // blue
        VectorScale:         1000,
        MarkerSize:          8,
    },
}

// Generate the SVG
err := svgrender.DrawEnhancedSVG(doc, "sediment_analysis.svg")
```

## Configuration Options

### SedimentTransportOptions

| Option | Type | Description | Default |
|--------|------|-------------|---------|
| `Show` | bool | Enable sediment transport visualization | - |
| `Points` | []LatLon | Coastline points | - |
| `SedimentStates` | []SedimentState | Sediment transport calculation results | - |
| `ShowAccumulation` | bool | Show accumulation zones | true |
| `ShowErosion` | bool | Show erosion points | true |
| `ShowTransportVectors` | bool | Show transport vectors | true |
| `AccumulationColor` | string | Color for accumulation markers (hex) | "#2d6a4f" |
| `ErosionColor` | string | Color for erosion markers (hex) | "#c2410c" |
| `VectorColor` | string | Color for transport vectors (hex) | "#1f6f8b" |
| `VectorScale` | float64 | Scale factor for vector arrow length | 1000 |
| `MarkerSize` | float64 | Size of accumulation/erosion markers | 6 |

## Visual Elements

### Color Scheme
- **Green (#2d6a4f)**: Accumulation zones - sediment deposition
- **Red (#c2410c)**: Erosion points - sediment loss  
- **Blue (#1f6f8b)**: Transport vectors - sediment movement direction

### Vector Interpretation
- Arrow length indicates relative sediment volume
- Arrow direction shows predominant transport path
- Multiple arrows per point indicate bidirectional flow

## Integration with Existing Workflow

### With Erosion Analysis

The sediment transport visualization integrates seamlessly with existing erosion analysis:

```go
// After calculating erosion
snapshots := geometry.SimulateWaveErosion(points, steps, options)

// Calculate sediment transport
sedimentResult := calculateSedimentTransportForValidation(
    snapshots, bathymetryGrid, lithologyProfile, waveOptions,
)

// Create visualization with sediment transport
doc := svgrender.EnhancedDocument{
    Document: buildErosionDocument(snapshots),
    SedimentTransportOptions: &svgrender.SedimentTransportOptions{
        Show:           true,
        Points:         snapshots[len(snapshots)-1],
        SedimentStates:  sedimentResult.States,
        ShowTransportVectors: true,
    },
}
```

## Technical Details

### Data Requirements
- Points must be in `[]geometry.LatLon` format
- SedimentStates must come from `geometry.CalculateSedimentTransport()`
- Arrays must have matching lengths

### Rendering Process
1. Project geographic coordinates to SVG coordinate space
2. Draw accumulation markers as filled circles
3. Draw erosion markers as filled circles  
4. Calculate and draw transport vectors with arrowheads
5. Apply transparency and styling options

### Performance
- Vector rendering: O(n) where n = number of points
- Memory usage: Linear with point count
- SVG size: ~10-50KB depending on complexity

## Example Output

The generated SVG will show:
- Base coastline layer
- Green circles at accumulation zones
- Red circles at erosion points
- Blue arrows indicating sediment transport direction and magnitude
- Legend and statistical summary in sidebar

## Use Cases

1. **Coastal Management**: Identify problem areas needing intervention
2. **Research**: Analyze sediment transport patterns and dynamics
3. **Education**: Visual representation of coastal processes
4. **Planning**: Assess impacts of coastal engineering projects

## Future Enhancements

Potential improvements for future versions:
- Animated transport vectors showing temporal changes
- Heat maps for sediment accumulation rates
- Interactive tooltips with detailed sediment budget information
- Multi-year comparison visualizations