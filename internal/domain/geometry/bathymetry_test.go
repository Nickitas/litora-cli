package geometry

import (
	"math"
	"testing"
)

func TestLoadBathymetryFromJSON_ValidInput_Success(t *testing.T) {
	data := []byte(`[
		{"lat": 45.0, "lon": 30.0, "depth": -100},
		{"lat": 45.0, "lon": 30.01, "depth": -150},
		{"lat": 45.01, "lon": 30.0, "depth": -120},
		{"lat": 45.01, "lon": 30.01, "depth": -180}
	]`)

	grid, err := LoadBathymetryFromJSON(data, BathymetryLoadOptions{Resolution: 0.01})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if grid == nil {
		t.Fatal("expected grid, got nil")
	}

	if grid.bounds.MinLat != 45.0 || grid.bounds.MaxLat != 45.01 {
		t.Fatalf("expected lat bounds [45.0, 45.01], got [%f, %f]", grid.bounds.MinLat, grid.bounds.MaxLat)
	}

	if grid.bounds.MinLon != 30.0 || grid.bounds.MaxLon != 30.01 {
		t.Fatalf("expected lon bounds [30.0, 30.01], got [%f, %f]", grid.bounds.MinLon, grid.bounds.MaxLon)
	}
}

func TestLoadBathymetryFromJSON_EmptyArray_Error(t *testing.T) {
	data := []byte(`[]`)

	_, err := LoadBathymetryFromJSON(data, BathymetryLoadOptions{})
	if err == nil {
		t.Fatal("expected error for empty array, got nil")
	}

	if !containsString(err.Error(), "empty") {
		t.Fatalf("expected 'empty' in error message, got: %v", err)
	}
}

func TestLoadBathymetryFromJSON_InvalidCoordinates_Error(t *testing.T) {
	data := []byte(`[
		{"lat": 45.0, "lon": 30.0, "depth": -100},
		{"lat": 95.0, "lon": 30.0, "depth": -50}
	]`)

	_, err := LoadBathymetryFromJSON(data, BathymetryLoadOptions{})
	if err == nil {
		t.Fatal("expected error for invalid latitude, got nil")
	}

	if !containsString(err.Error(), "latitude") {
		t.Fatalf("expected 'latitude' in error message, got: %v", err)
	}
}

func TestBuildGrid_ValidPoints_Success(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
		{Lat: 45.0, Lon: 30.01, Depth: -150},
		{Lat: 45.01, Lon: 30.0, Depth: -120},
		{Lat: 45.01, Lon: 30.01, Depth: -180},
	}

	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if len(grid.Points) != 4 {
		t.Fatalf("expected 4 points in grid, got %d", len(grid.Points))
	}

	if grid.Resolution != 0.01 {
		t.Fatalf("expected resolution 0.01, got %f", grid.Resolution)
	}
}

func TestBuildGrid_EmptyPoints_Error(t *testing.T) {
	points := []BathymetryPoint{}

	_, err := BuildGrid(points, 0.01)
	if err == nil {
		t.Fatal("expected error for empty points, got nil")
	}
}

func TestBuildGrid_ZeroResolution_Error(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
	}

	_, err := BuildGrid(points, 0)
	if err == nil {
		t.Fatal("expected error for zero resolution, got nil")
	}
}

func TestInterpolateDepth_ExactMatch_ReturnsDepth(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
		{Lat: 45.0, Lon: 30.01, Depth: -150},
		{Lat: 45.01, Lon: 30.0, Depth: -120},
		{Lat: 45.01, Lon: 30.01, Depth: -180},
	}

	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("build grid: %v", err)
	}

	depth, err := grid.InterpolateDepth(45.0, 30.0)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if math.Abs(depth+100) > 0.01 {
		t.Fatalf("expected depth -100, got %f", depth)
	}
}

func TestInterpolateDepth_Bilinear_Interpolates(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
		{Lat: 45.0, Lon: 30.01, Depth: -150},
		{Lat: 45.01, Lon: 30.0, Depth: -120},
		{Lat: 45.01, Lon: 30.01, Depth: -180},
	}

	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("build grid: %v", err)
	}

	depth, err := grid.InterpolateDepth(45.005, 30.005)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	expected := -137.5
	if math.Abs(depth-expected) > 0.1 {
		t.Fatalf("expected depth ~%f, got %f", expected, depth)
	}
}

func TestInterpolateDepth_OutsideBounds_Error(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
	}

	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("build grid: %v", err)
	}

	_, err = grid.InterpolateDepth(50.0, 30.0)
	if err == nil {
		t.Fatal("expected error for outside bounds, got nil")
	}

	if !containsString(err.Error(), "outside grid bounds") {
		t.Fatalf("expected 'outside grid bounds' in error, got: %v", err)
	}
}

func TestInterpolateDepth_MissingNeighbors_Error(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
		{Lat: 45.0, Lon: 30.01, Depth: -150},
		{Lat: 45.0, Lon: 30.02, Depth: -200},
		{Lat: 45.01, Lon: 30.0, Depth: -120},
		{Lat: 45.01, Lon: 30.01, Depth: -180},
	}

	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("build grid: %v", err)
	}

	_, err = grid.InterpolateDepth(45.005, 30.015)
	if err == nil {
		t.Fatal("expected error for missing neighbors, got nil")
	}

	if !containsString(err.Error(), "missing neighbor") {
		t.Fatalf("expected 'missing neighbor' in error, got: %v", err)
	}
}

func TestPhysicalDepthFactor_DeepWater_HighFactor(t *testing.T) {
	depth := -100.0
	fetch := 500.0
	depthScale := 50.0

	factor := physicalDepthFactor(depth, fetch, depthScale)

	if factor <= 0.8 {
		t.Fatalf("expected high factor (>0.8) for deep water, got %f", factor)
	}
}

func TestPhysicalDepthFactor_ShallowWater_LowFactor(t *testing.T) {
	depth := -5.0
	fetch := 500.0
	depthScale := 50.0

	factor := physicalDepthFactor(depth, fetch, depthScale)

	if factor >= 0.2 {
		t.Fatalf("expected low factor (<0.2) for shallow water, got %f", factor)
	}
}

func TestPhysicalDepthFactor_ZeroDepth_ZeroFactor(t *testing.T) {
	depth := 0.0
	fetch := 500.0
	depthScale := 50.0

	factor := physicalDepthFactor(depth, fetch, depthScale)

	if factor != 0 {
		t.Fatalf("expected zero factor for zero depth, got %f", factor)
	}
}

func TestWaveErosionWithBathymetry_Integration(t *testing.T) {
	points := []LatLon{
		{Lat: 45.0, Lon: 30.0},
		{Lat: 45.01, Lon: 30.01},
		{Lat: 45.0, Lon: 30.02},
		{Lat: 45.0, Lon: 30.0},
	}

	bathyData := []byte(`[
		{"lat": 44.99, "lon": 29.99, "depth": -200},
		{"lat": 44.99, "lon": 30.0, "depth": -150},
		{"lat": 44.99, "lon": 30.01, "depth": -100},
		{"lat": 44.99, "lon": 30.02, "depth": -50},
		{"lat": 45.0, "lon": 29.99, "depth": -180},
		{"lat": 45.0, "lon": 30.0, "depth": -130},
		{"lat": 45.0, "lon": 30.01, "depth": -80},
		{"lat": 45.0, "lon": 30.02, "depth": -30},
		{"lat": 45.01, "lon": 29.99, "depth": -160},
		{"lat": 45.01, "lon": 30.0, "depth": -110},
		{"lat": 45.01, "lon": 30.01, "depth": -60},
		{"lat": 45.01, "lon": 30.02, "depth": -10}
	]`)

	grid, err := LoadBathymetryFromJSON(bathyData, BathymetryLoadOptions{Resolution: 0.01})
	if err != nil {
		t.Fatalf("load bathymetry: %v", err)
	}

	options := WaveErosionOptions{
		StrengthMeters:           20,
		WindSourceDirectionDeg:   90,
		WindSpeedMetersPerSecond: 10,
		FetchSpreadDeg:           45,
		FetchSamples:             7,
		MaxFetchMeters:           5000,
		DepthScaleMeters:         1000,
		ExposurePower:            1.2,
		BathymetryGrid:           grid,
	}

	snapshots := SimulateWaveErosionWithSeed(points, 1, options, 42)
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}

	eroded := snapshots[1]
	if len(eroded) != len(points) {
		t.Fatalf("expected %d points, got %d", len(points), len(eroded))
	}

	if eroded[0] != eroded[len(eroded)-1] {
		t.Fatal("expected closed ring to remain closed")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
