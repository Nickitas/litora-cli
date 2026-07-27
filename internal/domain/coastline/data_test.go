package coastline

import (
	"coastal-geometry/internal/domain/geometry"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFromJSON(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "coast.json")
	content := `[
		{"lat": 46.48, "lon": 30.73},
		{"lat": 41.65, "lon": 41.63}
	]`

	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp json: %v", err)
	}

	points, report, err := LoadFromJSON(filename)
	if err != nil {
		t.Fatalf("LoadFromJSON returned error: %v", err)
	}

	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(points))
	}
	if len(report.Fixes) != 0 {
		t.Fatalf("expected no fixes, got %+v", report)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "больше порога") {
		t.Fatalf("expected long-segment warning, got %+v", report)
	}

	if points[0].Lat != 46.48 || points[0].Lon != 30.73 {
		t.Fatalf("unexpected first point: %+v", points[0])
	}
}

func TestLoadFromJSONRejectsInvalidLatitude(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "invalid-coast.json")
	content := `[
		{"lat": 146.48, "lon": 30.73},
		{"lat": 41.65, "lon": 41.63}
	]`

	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp json: %v", err)
	}

	_, _, err := LoadFromJSON(filename)
	if err == nil {
		t.Fatal("expected error for invalid latitude, got nil")
	}

	if !strings.Contains(err.Error(), "invalid latitude") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFromJSONRemovesDuplicateCoordinates(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "duplicates.json")
	content := `[
		{"lat": 46.48, "lon": 30.73},
		{"lat": 46.48, "lon": 30.73},
		{"lat": 45.33, "lon": 32.49}
	]`

	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp json: %v", err)
	}

	points, report, err := LoadFromJSON(filename)
	if err != nil {
		t.Fatalf("LoadFromJSON returned error: %v", err)
	}

	if len(points) != 2 {
		t.Fatalf("expected duplicates to be removed, got %d points", len(points))
	}
	if len(report.Fixes) == 0 || !strings.Contains(report.Fixes[0], "удалены повторяющиеся координаты") {
		t.Fatalf("expected duplicate removal fix, got %+v", report.Fixes)
	}
}

func TestValidateAndNormalizePointsReordersContour(t *testing.T) {
	points := []geometry.LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 1, Lon: 1},
		{Lat: 1, Lon: 0},
		{Lat: 0, Lon: 1},
	}

	normalized, report, err := validateAndNormalizePoints(points)
	if err != nil {
		t.Fatalf("validateAndNormalizePoints returned error: %v", err)
	}

	if samePointOrder(points, normalized) {
		t.Fatalf("expected point order to change, got %+v", normalized)
	}
	if len(findSelfIntersections(normalized)) != 0 {
		t.Fatalf("expected normalized contour without intersections, got %+v", normalized)
	}
	if len(report.Fixes) == 0 || !strings.Contains(strings.Join(report.Fixes, " "), "переупорядочены") {
		t.Fatalf("expected reorder fix, got %+v", report.Fixes)
	}
}

func TestFindSelfIntersectionsDetectsCrossingSegments(t *testing.T) {
	points := []geometry.LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 1, Lon: 1},
		{Lat: 0, Lon: 1},
		{Lat: 1, Lon: 0},
	}

	intersections := findSelfIntersections(points)
	if len(intersections) != 1 {
		t.Fatalf("expected one intersection, got %+v", intersections)
	}
}

func TestValidateAndNormalizePointsWarnsOnLongSegmentsAndDuplicateLocations(t *testing.T) {
	points := []geometry.LatLon{
		{Lat: 46.48, Lon: 30.73},
		{Lat: 46.49, Lon: 30.74},
		{Lat: 41.28, Lon: 31.42},
	}

	_, report, err := validateAndNormalizePoints(points)
	if err != nil {
		t.Fatalf("validateAndNormalizePoints returned error: %v", err)
	}

	warnings := strings.Join(report.Warnings, " | ")
	if !strings.Contains(warnings, "повторяющийся ориентир") {
		t.Fatalf("expected duplicate location warning, got %+v", report.Warnings)
	}
	if !strings.Contains(warnings, "больше порога") {
		t.Fatalf("expected long segment warning, got %+v", report.Warnings)
	}
}

func TestSanityCheckWarningForBlackSea(t *testing.T) {
	result := SanityCheck("black-sea.json", 2104)
	if result.Valid {
		t.Fatalf("expected invalid sanity result, got %+v", result)
	}
	if !strings.Contains(result.Warning, "WARNING: coastline length likely incorrect") {
		t.Fatalf("expected sanity warning, got %q", result.Warning)
	}
}

func TestSanityCheckWarningSkippedForUnknownDataset(t *testing.T) {
	result := SanityCheck("custom.json", 2104)
	if result.Checked {
		t.Fatalf("expected unchecked sanity result for unknown dataset, got %+v", result)
	}
}
