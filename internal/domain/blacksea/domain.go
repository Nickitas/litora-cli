package blacksea

import "math"

const (
	// ID — устойчивый идентификатор единственной акватории Lito.
	ID = "black-sea"
	// Name — русское название единственной акватории Lito.
	Name = "Чёрное море"

	// MinLatitude — южная граница области загрузки с буфером.
	MinLatitude = 40.5
	// MaxLatitude — северная граница, включающая Днепровско-Бугский лиман.
	MaxLatitude = 47.5
	// MinLongitude — западная граница области загрузки с буфером.
	MinLongitude = 27.0
	// MaxLongitude — восточная граница области загрузки с буфером.
	MaxLongitude = 42.5
)

// Bounds описывает прямоугольную область загрузки Чёрного моря в WGS 84.
// Она не заменяет полигон акватории и используется только для раннего
// отклонения данных, заведомо относящихся к другим регионам.
type Bounds struct {
	MinLat float64 `json:"min_lat"`
	MaxLat float64 `json:"max_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLon float64 `json:"max_lon"`
}

// DefaultBounds возвращает зафиксированную область загрузки Чёрного моря.
func DefaultBounds() Bounds {
	return Bounds{
		MinLat: MinLatitude,
		MaxLat: MaxLatitude,
		MinLon: MinLongitude,
		MaxLon: MaxLongitude,
	}
}

// Contains сообщает, находится ли конечная координата внутри области загрузки
// Чёрного моря, включая её границы.
func Contains(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsInf(lat, 0) &&
		!math.IsNaN(lon) && !math.IsInf(lon, 0) &&
		lat >= MinLatitude && lat <= MaxLatitude &&
		lon >= MinLongitude && lon <= MaxLongitude
}
