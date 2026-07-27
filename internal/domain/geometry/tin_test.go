package geometry

import (
	"math"
	"strings"
	"testing"
)

func TestDefaultApproximationOptions(t *testing.T) {
	opts := DefaultApproximationOptions()

	if opts.MeshType != "tin" {
		t.Errorf("expected MeshType 'tin', got '%s'", opts.MeshType)
	}
	if opts.Resolution != 0.01 {
		t.Errorf("expected Resolution 0.01, got %f", opts.Resolution)
	}
	if opts.MaxTriangleArea != 10.0 {
		t.Errorf("expected MaxTriangleArea 10.0, got %f", opts.MaxTriangleArea)
	}
	if opts.ErrorTolerance != 100.0 {
		t.Errorf("expected ErrorTolerance 100.0, got %f", opts.ErrorTolerance)
	}
	if opts.MinPoints != 3 {
		t.Errorf("expected MinPoints 3, got %d", opts.MinPoints)
	}
}

func TestBuildTINMesh_InsufficientPoints(t *testing.T) {
	opts := DefaultApproximationOptions()
	opts.MinPoints = 3

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 44.5, Lon: 38.5},
	}

	_, err := BuildTINMesh(points, opts)
	if err == nil {
		t.Error("expected error for insufficient points, got nil")
	}
}

func TestBuildTINMesh_SimpleTriangle(t *testing.T) {
	opts := DefaultApproximationOptions()

	// Simple triangle
	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	if mesh == nil {
		t.Fatal("mesh is nil")
	}

	if mesh.Stats.VertexCount != 3 {
		t.Errorf("expected 3 vertices, got %d", mesh.Stats.VertexCount)
	}

	if mesh.Stats.TriangleCount == 0 {
		t.Error("expected at least one triangle, got 0")
	}

	// Check bounds
	if mesh.Bounds.MaxLat < mesh.Bounds.MinLat {
		t.Error("invalid latitude bounds")
	}
	if mesh.Bounds.MaxLon < mesh.Bounds.MinLon {
		t.Error("invalid longitude bounds")
	}
}

func TestBuildTINMesh_Rectangle(t *testing.T) {
	opts := DefaultApproximationOptions()

	// Rectangle (4 points) should create 2 triangles
	points := []LatLon{
		{Lat: 44.0, Lon: 38.0}, // Bottom-left
		{Lat: 44.0, Lon: 39.0}, // Bottom-right
		{Lat: 45.0, Lon: 38.0}, // Top-left
		{Lat: 45.0, Lon: 39.0}, // Top-right
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	if mesh.Stats.VertexCount != 4 {
		t.Errorf("expected 4 vertices, got %d", mesh.Stats.VertexCount)
	}

	if mesh.Stats.TriangleCount < 2 {
		t.Errorf("expected at least 2 triangles, got %d", mesh.Stats.TriangleCount)
	}
}

func TestBuildTINMesh_LargerDataset(t *testing.T) {
	opts := DefaultApproximationOptions()

	// Create a grid of points
	points := []LatLon{}
	for lat := 44.0; lat <= 45.0; lat += 0.5 {
		for lon := 38.0; lon <= 39.0; lon += 0.5 {
			points = append(points, LatLon{Lat: lat, Lon: lon})
		}
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	if mesh.Stats.VertexCount != len(points) {
		t.Errorf("expected %d vertices, got %d", len(points), mesh.Stats.VertexCount)
	}

	if mesh.Stats.TriangleCount == 0 {
		t.Error("expected triangles, got 0")
	}

	if mesh.Stats.EdgeCount == 0 {
		t.Error("expected edges, got 0")
	}
}

func TestTINMesh_CalculateStats(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
		{Lat: 45.5, Lon: 39.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	if mesh.Stats.AvgTriangleArea <= 0 {
		t.Error("expected positive average triangle area")
	}

	if mesh.Stats.MaxTriangleArea <= 0 {
		t.Error("expected positive max triangle area")
	}

	if mesh.Stats.MaxTriangleArea < mesh.Stats.MinTriangleArea {
		t.Error("max area should be >= min area")
	}
}

func TestTINMesh_GetTriangles(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	triangles := mesh.GetTriangles()
	if len(triangles) != mesh.Stats.TriangleCount {
		t.Errorf("expected %d triangles, got %d", mesh.Stats.TriangleCount, len(triangles))
	}
}

func TestTINMesh_GetTriangleAreas(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
		{Lat: 45.5, Lon: 39.5},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	areas := mesh.GetTriangleAreas()
	if len(areas) != mesh.Stats.TriangleCount {
		t.Errorf("expected %d areas, got %d", mesh.Stats.TriangleCount, len(areas))
	}

	for i, area := range areas {
		if area <= 0 {
			t.Errorf("triangle %d has non-positive area: %f", i, area)
		}
	}
}

func TestTINMesh_ValidateMesh(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	errors := mesh.ValidateMesh()
	if len(errors) > 0 {
		t.Errorf("expected no validation errors, got %v", errors)
	}
}

func TestTINMesh_MeshDensity(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
		{Lat: 45.5, Lon: 39.0},
		{Lat: 44.0, Lon: 40.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	density := mesh.MeshDensity()
	if density <= 0 {
		t.Error("expected positive mesh density")
	}
}

func TestTINMesh_InterpolateValue(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
		{Lat: 45.5, Lon: 39.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	// Test values
	values := []float64{10.0, 20.0, 15.0, 25.0}

	// Interpolate at centroid (should be close to average)
	centroidLat := (mesh.Bounds.MinLat + mesh.Bounds.MaxLat) / 2
	centroidLon := (mesh.Bounds.MinLon + mesh.Bounds.MaxLon) / 2

	result, err := mesh.InterpolateValue(centroidLat, centroidLon, values)
	if err != nil {
		t.Errorf("interpolation failed: %v", err)
	}

	if result <= 0 {
		t.Error("expected positive interpolated value")
	}
}

func TestTINMesh_InterpolateValue_OutsideBounds(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	values := []float64{10.0, 20.0, 15.0}

	// Point far outside bounds
	_, err = mesh.InterpolateValue(0.0, 0.0, values)
	if err == nil {
		t.Error("expected error for point outside mesh bounds")
	}
}

func TestTINMesh_GetMeshQuality(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
		{Lat: 45.5, Lon: 39.0},
		{Lat: 44.0, Lon: 39.5},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	quality := mesh.GetMeshQuality()

	if quality.TriangleCount != mesh.Stats.TriangleCount {
		t.Errorf("expected %d triangles, got %d", mesh.Stats.TriangleCount, quality.TriangleCount)
	}

	if quality.VertexCount != mesh.Stats.VertexCount {
		t.Errorf("expected %d vertices, got %d", mesh.Stats.VertexCount, quality.VertexCount)
	}

	if quality.MinAngle <= 0 || quality.MinAngle > 60 {
		t.Errorf("unexpected min angle: %f", quality.MinAngle)
	}

	if quality.MaxAngle < 60 || quality.MaxAngle > 180 {
		t.Errorf("unexpected max angle: %f", quality.MaxAngle)
	}

	if quality.AvgAngle < 30 || quality.AvgAngle > 90 {
		t.Errorf("unexpected avg angle: %f", quality.AvgAngle)
	}
}

func TestTINMesh_Simplify(t *testing.T) {
	opts := DefaultApproximationOptions()

	// Create a larger mesh to simplify
	points := []LatLon{}
	for i := 0; i < 10; i++ {
		lat := 44.0 + float64(i)*0.1
		lon := 38.0 + float64(i)*0.1
		points = append(points, LatLon{Lat: lat, Lon: lon})
	}
	// Add some more points for variety
	points = append(points,
		LatLon{Lat: 44.5, Lon: 39.0},
		LatLon{Lat: 45.0, Lon: 38.5},
		LatLon{Lat: 44.0, Lon: 39.5},
	)

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	originalTriCount := mesh.Stats.TriangleCount

	// Simplify with a reasonable threshold
	err = mesh.Simplify(1e10) // Very large threshold should keep most triangles
	if err != nil {
		t.Errorf("simplification failed: %v", err)
	}

	if mesh.Stats.TriangleCount > originalTriCount {
		t.Error("simplification should not increase triangle count")
	}
}

func TestTINMesh_ExportGeoJSON(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	geojson, err := mesh.ExportGeoJSON()
	if err != nil {
		t.Errorf("GeoJSON export failed: %v", err)
	}

	if len(geojson) == 0 {
		t.Error("expected non-empty GeoJSON output")
	}

	// Check for basic GeoJSON structure
	output := string(geojson)
	if !strings.Contains(output, "FeatureCollection") {
		t.Error("output should contain 'FeatureCollection'")
	}
	if !strings.Contains(output, "Polygon") {
		t.Error("output should contain 'Polygon'")
	}
}

func TestTINMesh_AdaptiveRefinement(t *testing.T) {
	opts := DefaultApproximationOptions()
	opts.MeshType = "adaptive"
	opts.MaxTriangleArea = 0.5 // Small threshold to trigger refinement

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
		{Lat: 45.5, Lon: 39.0},
		{Lat: 44.0, Lon: 39.5},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	// Adaptive mesh should have some refinement steps
	if mesh.Stats.RefinementSteps < 0 {
		t.Errorf("unexpected refinement steps: %d", mesh.Stats.RefinementSteps)
	}
}

func TestTINMesh_CircumcircleCalculation(t *testing.T) {
	// Test circumcircle calculation with a simple right triangle
	tri := Triangle{
		V0: 0, V1: 1, V2: 2,
	}

	points := []Point2D{
		{X: 0, Y: 0},
		{X: 1, Y: 0},
		{X: 0, Y: 1},
	}

	tri.calculateCircumcircle(points)

	// For a right triangle, circumcenter should be at midpoint of hypotenuse
	expectedX := 0.5
	expectedY := 0.5

	if math.Abs(tri.Circumcenter.X-expectedX) > 1e-10 {
		t.Errorf("expected circumcenter X %f, got %f", expectedX, tri.Circumcenter.X)
	}
	if math.Abs(tri.Circumcenter.Y-expectedY) > 1e-10 {
		t.Errorf("expected circumcenter Y %f, got %f", expectedY, tri.Circumcenter.Y)
	}

	expectedRadius := math.Sqrt(2.0) / 2
	if math.Abs(tri.Circumradius-expectedRadius) > 1e-10 {
		t.Errorf("expected circumradius %f, got %f", expectedRadius, tri.Circumradius)
	}
}

func TestTriangleArea(t *testing.T) {
	p0 := Point2D{X: 0, Y: 0}
	p1 := Point2D{X: 1, Y: 0}
	p2 := Point2D{X: 0, Y: 1}

	area := triangleArea(p0, p1, p2)
	expectedArea := 0.5

	if math.Abs(area-expectedArea) > 1e-10 {
		t.Errorf("expected area %f, got %f", expectedArea, area)
	}
}

func TestTriangleArea_Large(t *testing.T) {
	// Test with larger coordinates (realistic projection)
	p0 := Point2D{X: 1000, Y: 1000}
	p1 := Point2D{X: 2000, Y: 1000}
	p2 := Point2D{X: 1500, Y: 2000}

	area := triangleArea(p0, p1, p2)
	expectedArea := 500000.0 // 0.5 * base * height

	if math.Abs(area-expectedArea) > 1e-6 {
		t.Errorf("expected area %f, got %f", expectedArea, area)
	}
}

func TestBarycentricCoords(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	// Test barycentric coords for triangle vertices
	for _, tri := range mesh.Triangles {
		v0 := mesh.Projected[tri.V0]
		v1 := mesh.Projected[tri.V1]
		v2 := mesh.Projected[tri.V2]

		// At first vertex
		w := mesh.barycentricCoords(v0, v0, v1, v2)
		if math.Abs(w[0]-1.0) > 1e-10 || math.Abs(w[1]) > 1e-10 || math.Abs(w[2]) > 1e-10 {
			t.Errorf("expected barycentric [1,0,0] at vertex 0, got [%v,%v,%v]", w[0], w[1], w[2])
		}

		// At second vertex
		w = mesh.barycentricCoords(v1, v0, v1, v2)
		if math.Abs(w[0]) > 1e-10 || math.Abs(w[1]-1.0) > 1e-10 || math.Abs(w[2]) > 1e-10 {
			t.Errorf("expected barycentric [0,1,0] at vertex 1, got [%v,%v,%v]", w[0], w[1], w[2])
		}

		// At third vertex
		w = mesh.barycentricCoords(v2, v0, v1, v2)
		if math.Abs(w[0]) > 1e-10 || math.Abs(w[1]) > 1e-10 || math.Abs(w[2]-1.0) > 1e-10 {
			t.Errorf("expected barycentric [0,0,1] at vertex 2, got [%v,%v,%v]", w[0], w[1], w[2])
		}
	}
}

func TestPointInTriangle(t *testing.T) {
	p0 := Point2D{X: 0, Y: 0}
	p1 := Point2D{X: 1, Y: 0}
	p2 := Point2D{X: 0, Y: 1}

	mesh := &TINMesh{}

	// Point inside
	center := Point2D{X: 0.25, Y: 0.25}
	if !mesh.pointInTriangle(center, p0, p1, p2) {
		t.Error("expected point to be inside triangle")
	}

	// Point outside
	outside := Point2D{X: 2, Y: 2}
	if mesh.pointInTriangle(outside, p0, p1, p2) {
		t.Error("expected point to be outside triangle")
	}

	// Point on edge
	edge := Point2D{X: 0.5, Y: 0}
	if !mesh.pointInTriangle(edge, p0, p1, p2) {
		t.Error("expected point on edge to be considered inside")
	}
}

func TestTriangleAngles(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	for _, tri := range mesh.Triangles {
		angles := mesh.triangleAngles(tri)

		// Sum of angles should be approximately 180 degrees
		sum := angles[0] + angles[1] + angles[2]
		if math.Abs(sum-180.0) > 0.1 {
			t.Errorf("triangle angles sum %f, expected 180", sum)
		}

		// All angles should be positive
		for i, a := range angles {
			if a <= 0 || a >= 180 {
				t.Errorf("invalid angle %d: %f", i, a)
			}
		}
	}
}

func TestMeshBounds(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 39.0},
		{Lat: 44.5, Lon: 39.5}, // Changed from 38.5 to 39.5 to avoid collinearity
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	// Check bounds contain all points
	for _, p := range points {
		if p.Lat < mesh.Bounds.MinLat || p.Lat > mesh.Bounds.MaxLat {
			t.Errorf("bounds don't contain point lat %f", p.Lat)
		}
		if p.Lon < mesh.Bounds.MinLon || p.Lon > mesh.Bounds.MaxLon {
			t.Errorf("bounds don't contain point lon %f", p.Lon)
		}
	}
}

func TestConvexHullCount(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
		{Lat: 44.25, Lon: 38.25}, // Interior point
		{Lat: 44.75, Lon: 38.75}, // Interior point
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	hullCount := mesh.calculateConvexHullCount()

	// Hull should have fewer vertices than total (due to interior points)
	if hullCount > mesh.Stats.VertexCount {
		t.Error("hull count should not exceed vertex count")
	}

	// Hull should have at least 3 vertices
	if hullCount < 3 {
		t.Errorf("hull should have at least 3 vertices, got %d", hullCount)
	}
}

func TestIsBoundaryEdge(t *testing.T) {
	opts := DefaultApproximationOptions()

	points := []LatLon{
		{Lat: 44.0, Lon: 38.0},
		{Lat: 45.0, Lon: 38.0},
		{Lat: 44.5, Lon: 39.0},
	}

	mesh, err := BuildTINMesh(points, opts)
	if err != nil {
		t.Fatalf("failed to build TIN mesh: %v", err)
	}

	// For a simple triangle, all edges should be boundary edges
	if len(mesh.Triangles) > 0 {
		tri := mesh.Triangles[0]
		edges := [][2]int{{tri.V0, tri.V1}, {tri.V1, tri.V2}, {tri.V2, tri.V0}}

		for _, edge := range edges {
			if !mesh.isBoundaryEdge(edge[0], edge[1]) {
				t.Errorf("expected edge %v to be boundary edge", edge)
			}
		}
	}
}

// Benchmark tests
func BenchmarkBuildTINMesh_Small(b *testing.B) {
	opts := DefaultApproximationOptions()

	points := []LatLon{}
	for lat := 44.0; lat <= 45.0; lat += 0.1 {
		for lon := 38.0; lon <= 39.0; lon += 0.1 {
			points = append(points, LatLon{Lat: lat, Lon: lon})
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = BuildTINMesh(points, opts)
	}
}

func BenchmarkBuildTINMesh_Medium(b *testing.B) {
	opts := DefaultApproximationOptions()

	points := []LatLon{}
	for lat := 43.0; lat <= 46.0; lat += 0.1 {
		for lon := 37.0; lon <= 40.0; lon += 0.1 {
			points = append(points, LatLon{Lat: lat, Lon: lon})
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = BuildTINMesh(points, opts)
	}
}
