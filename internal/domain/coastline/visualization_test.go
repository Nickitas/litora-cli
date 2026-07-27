package coastline

import (
	"testing"

	"coastal-geometry/internal/domain/geometry"
)

func TestBuildVisualizationHintsDetectsLongSegments(t *testing.T) {
	points := []geometry.LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 0, Lon: 5},
		{Lat: 0, Lon: 5.1},
	}

	hints := BuildVisualizationHints(points)
	if len(hints.LongSegments) != 1 {
		t.Fatalf("expected 1 long segment highlight, got %d", len(hints.LongSegments))
	}

	segment := hints.LongSegments[0]
	if segment.StartIndex != 1 || segment.EndIndex != 2 {
		t.Fatalf("unexpected segment indices: %+v", segment)
	}
	if segment.LengthKM <= longSegmentWarningKM {
		t.Fatalf("expected highlighted segment to exceed %.0f km, got %.2f", longSegmentWarningKM, segment.LengthKM)
	}
}
