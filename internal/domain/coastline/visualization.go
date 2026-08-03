package coastline

import "coastal-geometry/internal/domain/geometry"

// SegmentHighlight представляет подсвеченный сегмент для визуализации
type SegmentHighlight struct {
	StartIndex int
	EndIndex   int
	Start      geometry.LatLon
	End        geometry.LatLon
	LengthKM   float64
}

// VisualizationHints содержит подсказки для визуализации
type VisualizationHints struct {
	LongSegments []SegmentHighlight
}

// BuildVisualizationHints строит подсказки для визуализации
func BuildVisualizationHints(points []geometry.LatLon) VisualizationHints {
	return VisualizationHints{
		LongSegments: collectLongSegmentHighlights(points, longSegmentWarningKM),
	}
}

// collectLongSegmentHighlights собирает подсветку длинных сегментов
func collectLongSegmentHighlights(points []geometry.LatLon, thresholdKM float64) []SegmentHighlight {
	highlights := make([]SegmentHighlight, 0)
	for i := 1; i < len(points); i++ {
		length := geometry.Haversine(points[i-1], points[i])
		if length <= thresholdKM {
			continue
		}
		highlights = append(highlights, SegmentHighlight{
			StartIndex: i,
			EndIndex:   i + 1,
			Start:      points[i-1],
			End:        points[i],
			LengthKM:   length,
		})
	}
	return highlights
}
