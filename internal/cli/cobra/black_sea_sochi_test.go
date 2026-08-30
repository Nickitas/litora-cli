package cobra

import (
	"testing"

	"coastal-geometry/internal/domain/geometry"
)

func TestMapOSMStructuresToSochiAppliesOnlyAttachedGroyne(t *testing.T) {
	coast := []geometry.LatLon{{Lat: 43.60, Lon: 39.70}, {Lat: 43.60, Lon: 39.71}, {Lat: 43.60, Lon: 39.72}}
	raw := []byte(`{
  "elements": [
    {"id": 10, "tags": {"man_made":"groyne"}, "geometry": [{"lat":43.60,"lon":39.71},{"lat":43.59,"lon":39.71}]},
    {"id": 11, "tags": {"man_made":"breakwater"}, "geometry": [{"lat":43.60,"lon":39.72},{"lat":43.59,"lon":39.72}]}
  ]
}`)
	structures, inventory, err := mapOSMStructuresToSochi(raw, coast, "2026-08-18T00:00:00Z")
	if err != nil {
		t.Fatalf("mapOSMStructuresToSochi вернула ошибку: %v", err)
	}
	if len(structures) != 1 {
		t.Fatalf("должна примениться только буна, получено %d: %s", len(structures), inventory)
	}
	if structures[0].LeftPointIndex != 1 || structures[0].TransmissionCoefficient != 0 {
		t.Fatalf("неверная привязка сооружения: %#v", structures[0])
	}
}
