package coastline

import (
	"slices"

	"coastal-geometry/internal/domain/geometry"
)

const (
	WarningTypeLongSegment       = "long_segment"
	WarningTypeDuplicateLocation = "duplicate_location"
)

type ValidationIssueSummary struct {
	WarningType string
	Count       int
	ThresholdKM float64
}

type DuplicateLocationSummary struct {
	Name  string
	Count int
}

type ValidationSummary struct {
	Issues             []ValidationIssueSummary
	DuplicateLocations []DuplicateLocationSummary
}

func BuildValidationSummary(points []geometry.LatLon) ValidationSummary {
	longSegments := collectLongSegmentHighlights(points, longSegmentWarningKM)
	duplicates := collectDuplicateLocations(points)

	issues := []ValidationIssueSummary{
		{
			WarningType: WarningTypeLongSegment,
			Count:       len(longSegments),
			ThresholdKM: longSegmentWarningKM,
		},
		{
			WarningType: WarningTypeDuplicateLocation,
			Count:       len(duplicates),
		},
	}

	return ValidationSummary{
		Issues:             issues,
		DuplicateLocations: duplicates,
	}
}

func collectDuplicateLocations(points []geometry.LatLon) []DuplicateLocationSummary {
	if len(points) > 200 {
		return nil
	}

	counts := map[string]int{}
	for _, point := range points {
		name := getLocationName(point)
		if name == "—" {
			continue
		}
		counts[name]++
	}

	duplicates := make([]DuplicateLocationSummary, 0, len(counts))
	for name, count := range counts {
		if count <= 1 {
			continue
		}
		duplicates = append(duplicates, DuplicateLocationSummary{
			Name:  name,
			Count: count,
		})
	}

	slices.SortFunc(duplicates, func(a, b DuplicateLocationSummary) int {
		if a.Name == b.Name {
			if a.Count < b.Count {
				return -1
			}
			if a.Count > b.Count {
				return 1
			}
			return 0
		}
		if a.Name < b.Name {
			return -1
		}
		return 1
	})

	return duplicates
}
