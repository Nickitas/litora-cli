package geometry

import (
	"math"
	"testing"
)

func TestLatLonIsLatValid(t *testing.T) {
	tests := []struct {
		name     string
		lat      float64
		expected bool
	}{
		{name: "валидная широта 0", lat: 0, expected: true},
		{name: "валидная широта 45", lat: 45, expected: true},
		{name: "валидная широта -45", lat: -45, expected: true},
		{name: "граница северной широты", lat: 90, expected: true},
		{name: "граница южной широты", lat: -90, expected: true},
		{name: "превышение северной границы", lat: 91, expected: false},
		{name: "превышение южной границы", lat: -91, expected: false},
		{name: "неверная широта", lat: 100, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := LatLon{Lat: tt.lat}
			got := p.IsLatValid()
			if got != tt.expected {
				t.Errorf("IsLatValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLatLonIsLonValid(t *testing.T) {
	tests := []struct {
		name     string
		lon      float64
		expected bool
	}{
		{name: "валидная долгота 0", lon: 0, expected: true},
		{name: "валидная долгота 90", lon: 90, expected: true},
		{name: "валидная долгота -90", lon: -90, expected: true},
		{name: "граница восточной долготы", lon: 180, expected: true},
		{name: "граница западной долготы", lon: -180, expected: true},
		{name: "превышение восточной границы", lon: 181, expected: false},
		{name: "превышение западной границы", lon: -181, expected: false},
		{name: "неверная долгота", lon: 200, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := LatLon{Lon: tt.lon}
			got := p.IsLonValid()
			if got != tt.expected {
				t.Errorf("IsLonValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLatLonIsValid(t *testing.T) {
	tests := []struct {
		name     string
		lat      float64
		lon      float64
		expected bool
	}{
		{name: "валидные координаты Москвы", lat: 55.75, lon: 37.61, expected: true},
		{name: "валидные координаты Сиднея", lat: -33.86, lon: 151.20, expected: true},
		{name: "неверная широта", lat: 95, lon: 0, expected: false},
		{name: "неверная долгота", lat: 0, lon: 185, expected: false},
		{name: "неверные обе координаты", lat: 95, lon: 185, expected: false},
		{name: "граничные значения", lat: 90, lon: 180, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := LatLon{Lat: tt.lat, Lon: tt.lon}
			got := p.IsValid()
			if got != tt.expected {
				t.Errorf("IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVector2DMagnitude(t *testing.T) {
	tests := []struct {
		name     string
		v        Vector2D
		expected float64
	}{
		{
			name:     "стандартный вектор 3-4-5",
			v:        Vector2D{X: 3, Y: 4},
			expected: 5.0,
		},
		{
			name:     "нулевой вектор",
			v:        Vector2D{X: 0, Y: 0},
			expected: 0.0,
		},
		{
			name:     "единичный вектор по X",
			v:        Vector2D{X: 1, Y: 0},
			expected: 1.0,
		},
		{
			name:     "единичный вектор по Y",
			v:        Vector2D{X: 0, Y: 1},
			expected: 1.0,
		},
		{
			name:     "вектор с отрицательными компонентами",
			v:        Vector2D{X: -3, Y: -4},
			expected: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Magnitude()
			if math.Abs(got-tt.expected) > 1e-9 {
				t.Errorf("Magnitude() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestVector2DNormalize(t *testing.T) {
	tests := []struct {
		name     string
		v        Vector2D
		expected Vector2D
	}{
		{
			name:     "стандартный вектор 3-4",
			v:        Vector2D{X: 3, Y: 4},
			expected: Vector2D{X: 0.6, Y: 0.8},
		},
		{
			name:     "нулевой вектор",
			v:        Vector2D{X: 0, Y: 0},
			expected: Vector2D{X: 0, Y: 0},
		},
		{
			name:     "уже нормализованный вектор",
			v:        Vector2D{X: 1, Y: 0},
			expected: Vector2D{X: 1, Y: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.v.Normalize()
			// Проверяем с точностью до 1e-9
			if math.Abs(got.X-tt.expected.X) > 1e-9 || math.Abs(got.Y-tt.expected.Y) > 1e-9 {
				t.Errorf("Normalize() = %v, want %v", got, tt.expected)
			}
			// Проверяем, что результат имеет единичную длину (если не нулевой)
			if tt.v.Magnitude() > 0 {
				mag := got.Magnitude()
				if math.Abs(mag-1.0) > 1e-9 {
					t.Errorf("Normalize() result has magnitude %v, want 1.0", mag)
				}
			}
		})
	}
}

func TestVector2DDot(t *testing.T) {
	v1 := Vector2D{X: 1, Y: 2}
	v2 := Vector2D{X: 3, Y: 4}
	expected := float64(1*3 + 2*4) // 11
	got := v1.Dot(v2)
	if got != expected {
		t.Errorf("Dot() = %v, want %v", got, expected)
	}
}

func TestVector2DAdd(t *testing.T) {
	v1 := Vector2D{X: 1, Y: 2}
	v2 := Vector2D{X: 3, Y: 4}
	expected := Vector2D{X: 4, Y: 6}
	got := v1.Add(v2)
	if got != expected {
		t.Errorf("Add() = %v, want %v", got, expected)
	}
}

func TestVector2DSub(t *testing.T) {
	v1 := Vector2D{X: 4, Y: 6}
	v2 := Vector2D{X: 1, Y: 2}
	expected := Vector2D{X: 3, Y: 4}
	got := v1.Sub(v2)
	if got != expected {
		t.Errorf("Sub() = %v, want %v", got, expected)
	}
}

func TestVector2DScale(t *testing.T) {
	v := Vector2D{X: 2, Y: 3}
	expected := Vector2D{X: 6, Y: 9}
	got := v.Scale(3.0)
	if got != expected {
		t.Errorf("Scale() = %v, want %v", got, expected)
	}
}
