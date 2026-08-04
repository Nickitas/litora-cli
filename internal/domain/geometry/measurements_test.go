package geometry

import (
	"testing"
)

func TestHaversine(t *testing.T) {
	tests := []struct {
		name    string
		p1      LatLon
		p2      LatLon
		minDist float64
		maxDist float64
	}{
		{
			name:    "одна и та же точка",
			p1:      LatLon{Lat: 55.75, Lon: 37.61},
			p2:      LatLon{Lat: 55.75, Lon: 37.61},
			minDist: 0,
			maxDist: 0.1, // допускаем небольшую погрешность
		},
		{
			name:    "Москва - Санкт-Петербург (примерно 630 км)",
			p1:      LatLon{Lat: 55.75, Lon: 37.61}, // Москва
			p2:      LatLon{Lat: 59.93, Lon: 30.33}, // Санкт-Петербург
			minDist: 630,
			maxDist: 640,
		},
		{
			name:    "короткое расстояние",
			p1:      LatLon{Lat: 55.75, Lon: 37.61},
			p2:      LatLon{Lat: 55.76, Lon: 37.62},
			minDist: 1,
			maxDist: 2,
		},
		{
			name:    "экватор - большой путь",
			p1:      LatLon{Lat: 0, Lon: 0},
			p2:      LatLon{Lat: 0, Lon: 90},
			minDist: 10015, // примерно 1/4 окружности Земли
			maxDist: 10020,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist := Haversine(tt.p1, tt.p2)
			if dist < tt.minDist || dist > tt.maxDist {
				t.Errorf("Haversine() = %v, want between %v and %v", dist, tt.minDist, tt.maxDist)
			}
		})
	}
}

func TestPolylineLength(t *testing.T) {
	tests := []struct {
		name      string
		points    []LatLon
		minLength float64
		maxLength float64
	}{
		{
			name:      "пустая полилиния",
			points:    []LatLon{},
			minLength: 0,
			maxLength: 0,
		},
		{
			name:      "одна точка",
			points:    []LatLon{{Lat: 55.75, Lon: 37.61}},
			minLength: 0,
			maxLength: 0,
		},
		{
			name: "две точки - короткое расстояние",
			points: []LatLon{
				{Lat: 55.75, Lon: 37.61},
				{Lat: 55.76, Lon: 37.62},
			},
			minLength: 1,
			maxLength: 2,
		},
		{
			name: "три точки - треугольник",
			points: []LatLon{
				{Lat: 55.75, Lon: 37.61},
				{Lat: 55.76, Lon: 37.62},
				{Lat: 55.75, Lon: 37.62},
			},
			minLength: 2,
			maxLength: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			length := PolylineLength(tt.points)
			if length < tt.minLength || length > tt.maxLength {
				t.Errorf("PolylineLength() = %v, want between %v and %v", length, tt.minLength, tt.maxLength)
			}
		})
	}
}

func TestArea(t *testing.T) {
	tests := []struct {
		name    string
		points  []LatLon
		minArea float64
		maxArea float64
	}{
		{
			name:    "менее 3 точек - нулевая площадь",
			points:  []LatLon{{Lat: 55, Lon: 37}, {Lat: 56, Lon: 38}},
			minArea: 0,
			maxArea: 0,
		},
		{
			name: "треугольник в Крыму",
			points: []LatLon{
				{Lat: 45.0, Lon: 34.0},
				{Lat: 45.0, Lon: 35.0},
				{Lat: 46.0, Lon: 34.5},
			},
			minArea: 4000, // примерно 4000+ км²
			maxArea: 5000,
		},
		{
			name: "замкнутый полигон (квадрат)",
			points: []LatLon{
				{Lat: 45.0, Lon: 34.0},
				{Lat: 45.0, Lon: 35.0},
				{Lat: 46.0, Lon: 35.0},
				{Lat: 46.0, Lon: 34.0},
				{Lat: 45.0, Lon: 34.0}, // замкнутый
			},
			minArea: 6000,
			maxArea: 12000,
		},
		{
			name: "незамкнутый полигон (должен замкнуться автоматически)",
			points: []LatLon{
				{Lat: 45.0, Lon: 34.0},
				{Lat: 45.0, Lon: 35.0},
				{Lat: 46.0, Lon: 35.0},
				{Lat: 46.0, Lon: 34.0},
				// не замкнутый - но функция должна замкнуть автоматически
			},
			minArea: 6000,
			maxArea: 12000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			area := Area(tt.points)
			if area < tt.minArea || area > tt.maxArea {
				t.Errorf("Area() = %v, want between %v and %v", area, tt.minArea, tt.maxArea)
			}
		})
	}
}
