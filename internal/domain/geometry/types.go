package geometry

import "math"

// LatLon представляет точку с географическими координатами
type LatLon struct {
	Lat float64 `json:"lat"` // Широта в градусах [-90, 90]
	Lon float64 `json:"lon"` // Долгота в градусах [-180, 180]
}

// IsLatValid проверяет, что широта в допустимом диапазоне
func (p LatLon) IsLatValid() bool {
	return p.Lat >= -90 && p.Lat <= 90
}

// IsLonValid проверяет, что долгота в допустимом диапазоне
func (p LatLon) IsLonValid() bool {
	return p.Lon >= -180 && p.Lon <= 180
}

// IsValid проверяет, что обе координаты в допустимых диапазонах
func (p LatLon) IsValid() bool {
	return p.IsLatValid() && p.IsLonValid()
}

// Vector2D представляет двумерный вектор для локальных вычислений
type Vector2D struct {
	X float64 // Компонента X
	Y float64 // Компонента Y
}

// Magnitude вычисляет длину вектора
func (v Vector2D) Magnitude() float64 {
	return sqrt(v.X*v.X + v.Y*v.Y)
}

// Normalize нормализует вектор (делает единичной длины)
func (v Vector2D) Normalize() Vector2D {
	mag := v.Magnitude()
	if mag == 0 {
		return Vector2D{X: 0, Y: 0}
	}
	return Vector2D{X: v.X / mag, Y: v.Y / mag}
}

// Dot вычисляет скалярное произведение с другим вектором
func (v Vector2D) Dot(other Vector2D) float64 {
	return v.X*other.X + v.Y*other.Y
}

// Add складывает два вектора
func (v Vector2D) Add(other Vector2D) Vector2D {
	return Vector2D{X: v.X + other.X, Y: v.Y + other.Y}
}

// Sub вычитает другой вектор из текущего
func (v Vector2D) Sub(other Vector2D) Vector2D {
	return Vector2D{X: v.X - other.X, Y: v.Y - other.Y}
}

// Scale умножает вектор на скаляр
func (v Vector2D) Scale(scalar float64) Vector2D {
	return Vector2D{X: v.X * scalar, Y: v.Y * scalar}
}

// sqrt — вспомогательная функция для вычисления квадратного корня
func sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	return math.Sqrt(x)
}
