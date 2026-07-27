package coastline

import (
	"testing"

	"coastal-geometry/internal/domain/geometry"
)

func TestBuildValidationSummaryIncludesLongSegmentsAndDuplicateLocations(t *testing.T) {
	points := []geometry.LatLon{
		{Lat: 46.48, Lon: 30.73},
		{Lat: 46.49, Lon: 30.74},
		{Lat: 0, Lon: 5},
	}

	summary := BuildValidationSummary(points)
	if len(summary.Issues) != 2 {
		t.Fatalf("expected 2 issue summaries, got %+v", summary)
	}
	if len(summary.DuplicateLocations) != 1 {
		t.Fatalf("expected 1 duplicate location summary, got %+v", summary.DuplicateLocations)
	}

	duplicate := summary.DuplicateLocations[0]
	if duplicate.Name != "Одесса, Украина" || duplicate.Count != 2 {
		t.Fatalf("unexpected duplicate location summary: %+v", duplicate)
	}

	var foundLongSegments bool
	var foundDuplicateLocations bool
	for _, issue := range summary.Issues {
		switch issue.WarningType {
		case WarningTypeLongSegment:
			foundLongSegments = true
			if issue.Count != 1 || issue.ThresholdKM != longSegmentWarningKM {
				t.Fatalf("unexpected long-segment issue summary: %+v", issue)
			}
		case WarningTypeDuplicateLocation:
			foundDuplicateLocations = true
			if issue.Count != 1 {
				t.Fatalf("unexpected duplicate-location issue summary: %+v", issue)
			}
		}
	}

	if !foundLongSegments || !foundDuplicateLocations {
		t.Fatalf("expected both issue types, got %+v", summary.Issues)
	}
}

func TestBuildValidationSummaryKeepsStableIssueRowsForCleanGeometry(t *testing.T) {
	points := []geometry.LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 0, Lon: 0.1},
		{Lat: 0.1, Lon: 0.2},
	}

	summary := BuildValidationSummary(points)
	if len(summary.Issues) != 2 {
		t.Fatalf("expected 2 stable issue rows, got %+v", summary.Issues)
	}
	if len(summary.DuplicateLocations) != 0 {
		t.Fatalf("expected no duplicate locations, got %+v", summary.DuplicateLocations)
	}

	for _, issue := range summary.Issues {
		switch issue.WarningType {
		case WarningTypeLongSegment:
			if issue.Count != 0 || issue.ThresholdKM != longSegmentWarningKM {
				t.Fatalf("unexpected long-segment issue summary: %+v", issue)
			}
		case WarningTypeDuplicateLocation:
			if issue.Count != 0 {
				t.Fatalf("unexpected duplicate-location issue summary: %+v", issue)
			}
		default:
			t.Fatalf("unexpected issue type: %+v", issue)
		}
	}
}
