package geometry

import "testing"

func TestSimplifyPolylineKeepsEndpointsAndRespectsBudget(t *testing.T) {
	points := []LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 0.2, Lon: 0.1},
		{Lat: 0.4, Lon: 0.0},
		{Lat: 0.6, Lon: 0.1},
		{Lat: 0.8, Lon: 0.0},
		{Lat: 1.0, Lon: 0.0},
	}

	result := SimplifyPolyline(points, SimplifyOptions{MaxPoints: 4})
	if !result.Applied {
		t.Fatal("expected simplification to be applied")
	}
	if len(result.Points) > 4 {
		t.Fatalf("expected at most 4 points, got %d", len(result.Points))
	}
	if result.Points[0] != points[0] {
		t.Fatalf("expected first point to be preserved, got %+v", result.Points[0])
	}
	if result.Points[len(result.Points)-1] != points[len(points)-1] {
		t.Fatalf("expected last point to be preserved, got %+v", result.Points[len(result.Points)-1])
	}
}

func TestSimplifyPolylinePreservesClosedRing(t *testing.T) {
	points := []LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 0, Lon: 1},
		{Lat: 1, Lon: 1},
		{Lat: 1, Lon: 0},
		{Lat: 0.4, Lon: 0.2},
		{Lat: 0, Lon: 0},
	}

	result := SimplifyPolyline(points, SimplifyOptions{MaxPoints: 5})
	if !result.SimplifiedClosed {
		t.Fatal("expected simplified polyline to remain closed")
	}
	if result.Points[0] != result.Points[len(result.Points)-1] {
		t.Fatalf("expected ring closure to be preserved, got first=%+v last=%+v", result.Points[0], result.Points[len(result.Points)-1])
	}
	if len(result.Points) > 5 {
		t.Fatalf("expected at most 5 points, got %d", len(result.Points))
	}
}

func TestSimplifyPolylineLeavesShortPolylineUntouched(t *testing.T) {
	points := []LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 1, Lon: 1},
	}

	result := SimplifyPolyline(points, SimplifyOptions{MaxPoints: 8})
	if result.Applied {
		t.Fatal("expected no simplification for a two-point polyline")
	}
	if len(result.Points) != len(points) {
		t.Fatalf("expected original points to be preserved, got %d", len(result.Points))
	}
}
