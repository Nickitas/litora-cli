package geometry

import "testing"

// TestSimplifyPolylineKeepsEndpointsAndRespectsBudget проверяет сохранение концевых точек и соблюдение бюджета
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
		t.Fatal("Ожидалось применение упрощения")
	}
	if len(result.Points) > 4 {
		t.Fatalf("Ожидалось не более 4 точек, получено %d", len(result.Points))
	}
	if result.Points[0] != points[0] {
		t.Fatalf("Ожидалось сохранение первой точки, получено %+v", result.Points[0])
	}
	if result.Points[len(result.Points)-1] != points[len(points)-1] {
		t.Fatalf("Ожидалось сохранение последней точки, получено %+v", result.Points[len(result.Points)-1])
	}
}

// TestSimplifyPolylinePreservesClosedRing проверяет сохранение замкнутого контура
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
		t.Fatal("Ожидалось, что упрощённая полилиния останется замкнутой")
	}
	if result.Points[0] != result.Points[len(result.Points)-1] {
		t.Fatalf("Ожидалось сохранение замыкания кольца, получено: первая=%+v последняя=%+v", result.Points[0], result.Points[len(result.Points)-1])
	}
	if len(result.Points) > 5 {
		t.Fatalf("Ожидалось не более 5 точек, получено %d", len(result.Points))
	}
}

// TestSimplifyPolylineLeavesShortPolylineUntouched проверяет сохранение короткой полилинии
func TestSimplifyPolylineLeavesShortPolylineUntouched(t *testing.T) {
	points := []LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 1, Lon: 1},
	}

	result := SimplifyPolyline(points, SimplifyOptions{MaxPoints: 8})
	if result.Applied {
		t.Fatal("Ожидалось отсутствие упрощения для двухточечной полилинии")
	}
	if len(result.Points) != len(points) {
		t.Fatalf("Ожидалось сохранение исходных точек, получено %d", len(result.Points))
	}
}

func TestSimplifyPolylineEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		points         []LatLon
		maxPoints      int
		wantApplied    bool
		wantSimplified bool
	}{
		{
			name:           "менее 3 точек - не упрощается",
			points:         []LatLon{{Lat: 45, Lon: 34}, {Lat: 46, Lon: 35}},
			maxPoints:      10,
			wantApplied:    false,
			wantSimplified: false,
		},
		{
			name: "maxPoints >= len(points) - не упрощается",
			points: []LatLon{
				{Lat: 45, Lon: 34},
				{Lat: 45, Lon: 35},
				{Lat: 46, Lon: 35},
			},
			maxPoints:      5,
			wantApplied:    false,
			wantSimplified: false,
		},
		{
			name: "замкнутая полилиния с 4 точками и maxPoints=4 - не упрощается",
			points: []LatLon{
				{Lat: 45, Lon: 34},
				{Lat: 45, Lon: 35},
				{Lat: 46, Lon: 35},
				{Lat: 45, Lon: 34}, // замкнутая
			},
			maxPoints:      4,
			wantApplied:    false,
			wantSimplified: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SimplifyPolyline(tt.points, SimplifyOptions{MaxPoints: tt.maxPoints})

			if result.Applied != tt.wantApplied {
				t.Errorf("SimplifyPolyline().Applied = %v, want %v", result.Applied, tt.wantApplied)
			}

			simplified := len(result.Points) < len(tt.points)
			if simplified != tt.wantSimplified {
				t.Errorf("SimplifyPolyline() simplified = %v, want %v", simplified, tt.wantSimplified)
			}
		})
	}
}

// TestSimplifyPolylineWithTolerancePreservesClosedRing проверяет метрический
// режим упрощения, используемый генераторами 2D-сеток.
func TestSimplifyPolylineWithTolerancePreservesClosedRing(t *testing.T) {
	points := []LatLon{
		{Lat: 43.0, Lon: 34.0},
		{Lat: 43.00001, Lon: 34.005},
		{Lat: 43.0, Lon: 34.01},
		{Lat: 43.01, Lon: 34.01},
		{Lat: 43.01, Lon: 34.0},
		{Lat: 43.0, Lon: 34.0},
	}

	result := SimplifyPolylineWithTolerance(points, 10)
	if !result.OriginalClosed || !result.SimplifiedClosed {
		t.Fatalf("замкнутость должна сохраняться: %+v", result)
	}
	if result.Points[0] != result.Points[len(result.Points)-1] {
		t.Fatal("результат должен завершаться исходной первой точкой")
	}
	if result.ToleranceMeters != 10 {
		t.Fatalf("неверный допуск: %.1f", result.ToleranceMeters)
	}
}

func TestSimplifyClosedPolylineWithToleranceDoesNotDegenerate(t *testing.T) {
	points := []LatLon{
		{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.01}, {Lat: 0, Lon: 0.02},
		{Lat: 0.01, Lon: 0.02}, {Lat: 0.02, Lon: 0.02}, {Lat: 0.02, Lon: 0.01},
		{Lat: 0.02, Lon: 0}, {Lat: 0.01, Lon: 0}, {Lat: 0, Lon: 0},
	}
	result := SimplifyPolylineWithTolerance(points, 100)
	if !result.SimplifiedClosed || result.Points[0] != result.Points[len(result.Points)-1] {
		t.Fatal("упрощённый замкнутый контур должен оставаться замкнутым")
	}
	if len(result.Points) < 5 {
		t.Fatalf("прямоугольное кольцо не должно вырождаться, точек: %d", len(result.Points))
	}
}
