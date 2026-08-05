package geometry

import "testing"

func TestLocalMetricProjectionAccountsForLatitude(t *testing.T) {
	lowLatitude := NewLocalMetricProjection([]LatLon{{Lat: 30, Lon: 35}})
	highLatitude := NewLocalMetricProjection([]LatLon{{Lat: 45, Lon: 35}})

	if lowLatitude.MetersPerDegreeLongitude <= highLatitude.MetersPerDegreeLongitude {
		t.Fatalf("длина градуса долготы не уменьшается к высокой широте: %.3f <= %.3f",
			lowLatitude.MetersPerDegreeLongitude, highLatitude.MetersPerDegreeLongitude)
	}
	if lowLatitude.MetersPerDegreeLatitude <= 0 || highLatitude.MetersPerDegreeLatitude <= 0 {
		t.Fatal("длина градуса широты должна быть положительной")
	}
}

func TestLocalMetricProjectionRoundTrip(t *testing.T) {
	projection := NewLocalMetricProjection([]LatLon{
		{Lat: 42, Lon: 30},
		{Lat: 45, Lon: 40},
	})
	original := LatLon{Lat: 43.25, Lon: 36.75}
	restored := projection.Unproject(projection.Project(original))

	if difference := restored.Lat - original.Lat; difference > 1e-12 || difference < -1e-12 {
		t.Fatalf("широта не восстановлена: исходная %.12f, полученная %.12f", original.Lat, restored.Lat)
	}
	if difference := restored.Lon - original.Lon; difference > 1e-12 || difference < -1e-12 {
		t.Fatalf("долгота не восстановлена: исходная %.12f, полученная %.12f", original.Lon, restored.Lon)
	}
}
