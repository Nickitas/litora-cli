package cobra

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"
)

func TestWriteBlackSeaMapGeoJSONWritesBothLayers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "black-sea.geojson")
	contour := coastline.LoadResult{
		Source: "открытый источник",
		Points: []geometry.LatLon{{Lat: 42, Lon: 30}, {Lat: 43, Lon: 31}},
	}
	sochi := coastline.LoadResult{
		Source: "локальный участок",
		Points: []geometry.LatLon{{Lat: 43.6, Lon: 39.7}, {Lat: 43.61, Lon: 39.71}},
	}
	if err := writeBlackSeaMapGeoJSON(path, contour, sochi); err != nil {
		t.Fatalf("writeBlackSeaMapGeoJSON: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение результата: %v", err)
	}
	var result blackSeaMapGeoJSON
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("разбор GeoJSON: %v", err)
	}
	if result.Type != "FeatureCollection" || len(result.Features) != 2 {
		t.Fatalf("неверная структура GeoJSON: %#v", result)
	}
	if got := result.Features[0].Geometry.Coordinates[0]; got[0] != 30 || got[1] != 42 {
		t.Fatalf("ожидался порядок GeoJSON [долгота, широта], получено %#v", got)
	}
}
