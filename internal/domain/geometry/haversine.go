package geometry

import (
	"github.com/paulmach/orb/geo"
)

// EarthRadiusKM — радиус Земли в километрах
const EarthRadiusKM = 6371.0

// Haversine возвращает расстояние по большому кругу в километрах между двумя точками.
// Использует расчёт расстояния из библиотеки orb на основе формулы гаверсинуса.
func Haversine(a, b LatLon) float64 {
	// geo.Distance возвращает метры, конвертируем в километры
	return geo.Distance(ToORB(a), ToORB(b)) / 1000
}
