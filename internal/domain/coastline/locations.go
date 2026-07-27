package coastline

import (
	"math"

	"coastal-geometry/internal/domain/geometry"
)

func getLocationName(p geometry.LatLon) string {
	const threshold = 0.15

	type location struct {
		lat  float64
		lon  float64
		name string
	}

	locations := []location{
		{lat: 46.48, lon: 30.73, name: "Одесса, Украина"},
		{lat: 45.33, lon: 32.49, name: "Евпатория, Крым"},
		{lat: 44.94, lon: 34.10, name: "Алушта, Крым"},
		{lat: 44.62, lon: 33.53, name: "Севастополь, Крым"},
		{lat: 44.55, lon: 38.10, name: "Геленджик, Россия"},
		{lat: 43.70, lon: 39.75, name: "Сочи, Россия"},
		{lat: 43.58, lon: 39.72, name: "Адлер, Россия"},
		{lat: 42.00, lon: 41.58, name: "Сухум, Абхазия"},
		{lat: 42.15, lon: 41.65, name: "Поти, Грузия"},
		{lat: 41.65, lon: 41.63, name: "Батуми, Грузия"},
		{lat: 41.55, lon: 41.57, name: "Чорох (граница)"},
		{lat: 41.02, lon: 40.27, name: "Трабзон, Турция"},
		{lat: 41.00, lon: 39.65, name: "Орду, Турция"},
		{lat: 41.28, lon: 31.42, name: "Синоп, Турция"},
		{lat: 43.00, lon: 28.00, name: "Варна, Болгария"},
	}

	bestName := "—"
	bestDistance := math.MaxFloat64
	for _, location := range locations {
		if math.Abs(p.Lat-location.lat) >= threshold || math.Abs(p.Lon-location.lon) >= threshold {
			continue
		}
		distance := math.Hypot(p.Lat-location.lat, p.Lon-location.lon)
		if distance < bestDistance {
			bestDistance = distance
			bestName = location.name
		}
	}
	return bestName
}
