package coastline

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestFetchCoastlineDataParsesGeoJSONPolygon(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/geo+json")
		fmt.Fprint(w, `{
			"type": "FeatureCollection",
			"features": [
				{
					"type": "Feature",
					"geometry": {
						"type": "Polygon",
						"coordinates": [[
							[5.0, 5.0],
							[6.0, 6.0],
							[7.0, 5.0],
							[5.0, 5.0]
						]]
					}
				},
				{
					"type": "Feature",
					"geometry": {
						"type": "Polygon",
						"coordinates": [[
							[30.73, 46.48],
							[32.49, 45.33],
							[34.10, 44.94],
							[39.75, 43.70],
							[41.63, 41.65],
							[30.73, 46.48]
						]]
					}
				}
			]
		}`)
	}))
	defer server.Close()

	points, err := fetchCoastlineData(server.Client(), server.URL, DefaultBlackSeaBounds)
	if err != nil {
		t.Fatalf("fetchCoastlineData returned error: %v", err)
	}

	if len(points) != 6 {
		t.Fatalf("expected 6 polygon points inside Black Sea bounds, got %d", len(points))
	}
	if points[0].Lat != 46.48 || points[0].Lon != 30.73 {
		t.Fatalf("unexpected first point: %+v", points[0])
	}
	if points[len(points)-1].Lat != 46.48 || points[len(points)-1].Lon != 30.73 {
		t.Fatalf("expected closed ring to remain intact, got %+v", points[len(points)-1])
	}
}

func TestLoadUsesRemoteGeoJSONWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	fallbackPath := filepath.Join(dir, "fallback.json")
	cachePath := filepath.Join(dir, "cache.geojson")
	if err := os.WriteFile(fallbackPath, []byte(`[
		{"lat": 10.0, "lon": 10.0},
		{"lat": 11.0, "lon": 11.0}
	]`), 0o644); err != nil {
		t.Fatalf("write fallback json: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/geo+json")
		fmt.Fprint(w, `{
			"type": "Feature",
			"geometry": {
				"type": "LineString",
				"coordinates": [
					[30.73, 46.48],
					[32.49, 45.33],
					[34.10, 44.94],
					[39.75, 43.70]
				]
			}
		}`)
	}))
	defer server.Close()

	result, err := Load(LoadOptions{
		LocalPath:    fallbackPath,
		RemoteURL:    server.URL,
		RemoteBounds: DefaultBlackSeaBounds,
		CachePath:    cachePath,
		HTTPClient:   server.Client(),
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if result.Source != server.URL {
		t.Fatalf("expected remote source %q, got %q", server.URL, result.Source)
	}
	if len(result.LoadWarnings) != 0 {
		t.Fatalf("expected no load warnings, got %+v", result.LoadWarnings)
	}
	if len(result.Points) != 4 {
		t.Fatalf("expected 4 remote points, got %d", len(result.Points))
	}
}

func TestLoadPreservesClosedPolygonRing(t *testing.T) {
	dir := t.TempDir()
	fallbackPath := filepath.Join(dir, "fallback.json")
	cachePath := filepath.Join(dir, "cache.geojson")
	if err := os.WriteFile(fallbackPath, []byte(`[
		{"lat": 10.0, "lon": 10.0},
		{"lat": 11.0, "lon": 11.0}
	]`), 0o644); err != nil {
		t.Fatalf("write fallback json: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/geo+json")
		fmt.Fprint(w, `{
			"type": "Feature",
			"geometry": {
				"type": "Polygon",
				"coordinates": [[
					[30.73, 46.48],
					[32.49, 45.33],
					[34.10, 44.94],
					[39.75, 43.70],
					[30.73, 46.48]
				]]
			}
		}`)
	}))
	defer server.Close()

	result, err := Load(LoadOptions{
		LocalPath:  fallbackPath,
		RemoteURL:  server.URL,
		CachePath:  cachePath,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(result.Points) != 5 {
		t.Fatalf("expected closed ring with 5 points, got %d", len(result.Points))
	}
	if result.Points[0] != result.Points[len(result.Points)-1] {
		t.Fatalf("expected normalized polygon ring to stay closed, got first=%+v last=%+v", result.Points[0], result.Points[len(result.Points)-1])
	}
}

func TestLoadFallsBackToLocalJSONWhenRemoteFails(t *testing.T) {
	dir := t.TempDir()
	fallbackPath := filepath.Join(dir, "fallback.json")
	cachePath := filepath.Join(dir, "cache.geojson")
	if err := os.WriteFile(fallbackPath, []byte(`[
		{"lat": 46.48, "lon": 30.73},
		{"lat": 41.65, "lon": 41.63}
	]`), 0o644); err != nil {
		t.Fatalf("write fallback json: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary failure", http.StatusBadGateway)
	}))
	defer server.Close()

	result, err := Load(LoadOptions{
		LocalPath:    fallbackPath,
		RemoteURL:    server.URL,
		RemoteBounds: DefaultBlackSeaBounds,
		CachePath:    cachePath,
		HTTPClient:   server.Client(),
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if result.Source != fallbackPath {
		t.Fatalf("expected fallback source %q, got %q", fallbackPath, result.Source)
	}
	if len(result.Points) != 2 {
		t.Fatalf("expected 2 fallback points, got %d", len(result.Points))
	}
	if len(result.LoadWarnings) != 1 {
		t.Fatalf("expected one load warning, got %+v", result.LoadWarnings)
	}
	if !strings.Contains(result.LoadWarnings[0], "using local fallback") {
		t.Fatalf("unexpected load warning: %+v", result.LoadWarnings)
	}
}

func TestLoadUsesCacheWithoutRemoteRequest(t *testing.T) {
	dir := t.TempDir()
	fallbackPath := filepath.Join(dir, "fallback.json")
	cachePath := filepath.Join(dir, "cache.geojson")
	if err := os.WriteFile(fallbackPath, []byte(`[
		{"lat": 10.0, "lon": 10.0},
		{"lat": 11.0, "lon": 11.0}
	]`), 0o644); err != nil {
		t.Fatalf("write fallback json: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte(`{
		"type": "Feature",
		"geometry": {
			"type": "LineString",
			"coordinates": [
				[30.73, 46.48],
				[32.49, 45.33],
				[34.10, 44.94],
				[39.75, 43.70]
			]
		}
	}`), 0o644); err != nil {
		t.Fatalf("write cache geojson: %v", err)
	}

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer server.Close()

	result, err := Load(LoadOptions{
		LocalPath:  fallbackPath,
		RemoteURL:  server.URL,
		CachePath:  cachePath,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if hits.Load() != 0 {
		t.Fatalf("expected no remote requests, got %d", hits.Load())
	}
	if len(result.Points) != 4 {
		t.Fatalf("expected 4 cached points, got %d", len(result.Points))
	}
	if !strings.Contains(result.Source, "cached copy") {
		t.Fatalf("expected cached source label, got %q", result.Source)
	}
}

func TestLoadRefreshesRemoteCache(t *testing.T) {
	dir := t.TempDir()
	fallbackPath := filepath.Join(dir, "fallback.json")
	cachePath := filepath.Join(dir, "cache.geojson")
	if err := os.WriteFile(fallbackPath, []byte(`[
		{"lat": 10.0, "lon": 10.0},
		{"lat": 11.0, "lon": 11.0}
	]`), 0o644); err != nil {
		t.Fatalf("write fallback json: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte(`{
		"type": "Feature",
		"geometry": {
			"type": "LineString",
			"coordinates": [
				[30.73, 46.48],
				[32.49, 45.33]
			]
		}
	}`), 0o644); err != nil {
		t.Fatalf("write stale cache geojson: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/geo+json")
		fmt.Fprint(w, `{
			"type": "Feature",
			"geometry": {
				"type": "LineString",
				"coordinates": [
					[30.73, 46.48],
					[32.49, 45.33],
					[34.10, 44.94],
					[39.75, 43.70]
				]
			}
		}`)
	}))
	defer server.Close()

	result, err := Load(LoadOptions{
		LocalPath:  fallbackPath,
		RemoteURL:  server.URL,
		CachePath:  cachePath,
		Refresh:    true,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if result.Source != server.URL {
		t.Fatalf("expected refreshed remote source %q, got %q", server.URL, result.Source)
	}
	if len(result.Points) != 4 {
		t.Fatalf("expected 4 refreshed points, got %d", len(result.Points))
	}

	cached, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read updated cache: %v", err)
	}
	if !strings.Contains(string(cached), "[39.75, 43.70]") {
		t.Fatalf("expected cache to be updated, got %s", string(cached))
	}
}

func TestLoadUsesStaleCacheWhenRefreshFails(t *testing.T) {
	dir := t.TempDir()
	fallbackPath := filepath.Join(dir, "fallback.json")
	cachePath := filepath.Join(dir, "cache.geojson")
	if err := os.WriteFile(fallbackPath, []byte(`[
		{"lat": 10.0, "lon": 10.0},
		{"lat": 11.0, "lon": 11.0}
	]`), 0o644); err != nil {
		t.Fatalf("write fallback json: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte(`{
		"type": "Feature",
		"geometry": {
			"type": "LineString",
			"coordinates": [
				[30.73, 46.48],
				[32.49, 45.33],
				[34.10, 44.94]
			]
		}
	}`), 0o644); err != nil {
		t.Fatalf("write cache geojson: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary failure", http.StatusBadGateway)
	}))
	defer server.Close()

	result, err := Load(LoadOptions{
		LocalPath:  fallbackPath,
		RemoteURL:  server.URL,
		CachePath:  cachePath,
		Refresh:    true,
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if !strings.Contains(result.Source, "cached copy") {
		t.Fatalf("expected stale cache source, got %q", result.Source)
	}
	if len(result.LoadWarnings) != 1 || !strings.Contains(result.LoadWarnings[0], "using cached GeoJSON") {
		t.Fatalf("expected cached warning, got %+v", result.LoadWarnings)
	}
	if len(result.Points) != 3 {
		t.Fatalf("expected 3 stale cached points, got %d", len(result.Points))
	}
}

func TestInspectSourceSavesSnapshotAndExtractsMetadata(t *testing.T) {
	dir := t.TempDir()
	fallbackPath := filepath.Join(dir, "fallback.json")
	cachePath := filepath.Join(dir, "cache.geojson")
	snapshotPath := filepath.Join(dir, "snapshot.geojson")
	if err := os.WriteFile(fallbackPath, []byte(`[
		{"lat": 10.0, "lon": 10.0},
		{"lat": 11.0, "lon": 11.0}
	]`), 0o644); err != nil {
		t.Fatalf("write fallback json: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/geo+json")
		fmt.Fprint(w, `{
			"type": "FeatureCollection",
			"features": [
				{
					"type": "Feature",
					"properties": {
						"name": "Black Sea",
						"mrgid": 3319
					},
					"geometry": {
						"type": "Polygon",
						"coordinates": [[
							[30.73, 46.48],
							[32.49, 45.33],
							[34.10, 44.94],
							[39.75, 43.70],
							[30.73, 46.48]
						]]
					}
				}
			]
		}`)
	}))
	defer server.Close()

	result, err := InspectSource(InspectOptions{
		LocalPath:    fallbackPath,
		RemoteURL:    server.URL,
		CachePath:    cachePath,
		SnapshotPath: snapshotPath,
		Refresh:      true,
		HTTPClient:   server.Client(),
	})
	if err != nil {
		t.Fatalf("InspectSource returned error: %v", err)
	}

	expectedSnapshot, err := filepath.Abs(snapshotPath)
	if err != nil {
		t.Fatalf("resolve expected snapshot path: %v", err)
	}

	if result.Source != server.URL {
		t.Fatalf("expected remote source %q, got %q", server.URL, result.Source)
	}
	if result.DatasetName != "Black Sea" {
		t.Fatalf("expected dataset name Black Sea, got %q", result.DatasetName)
	}
	if result.CachePath != cachePath {
		t.Fatalf("expected cache path %q, got %q", cachePath, result.CachePath)
	}
	if result.SnapshotPath != expectedSnapshot {
		t.Fatalf("expected snapshot path %q, got %q", expectedSnapshot, result.SnapshotPath)
	}
	if result.Metadata.Format != "GeoJSON" {
		t.Fatalf("expected GeoJSON format, got %q", result.Metadata.Format)
	}
	if result.Metadata.RootType != "FeatureCollection" {
		t.Fatalf("expected FeatureCollection root type, got %q", result.Metadata.RootType)
	}
	if result.Metadata.Name != "Black Sea" {
		t.Fatalf("expected metadata name Black Sea, got %q", result.Metadata.Name)
	}
	if result.Metadata.RegionID != "3319" {
		t.Fatalf("expected region id 3319, got %q", result.Metadata.RegionID)
	}
	if result.Metadata.FeatureCount != 1 {
		t.Fatalf("expected 1 feature, got %d", result.Metadata.FeatureCount)
	}
	if len(result.Metadata.GeometryTypes) != 1 || result.Metadata.GeometryTypes[0] != "Polygon" {
		t.Fatalf("unexpected geometry types: %+v", result.Metadata.GeometryTypes)
	}
	if result.Metadata.CoastlinePointCount != 5 {
		t.Fatalf("expected 5 coastline points, got %d", result.Metadata.CoastlinePointCount)
	}

	snapshot, err := os.ReadFile(result.SnapshotPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if !strings.Contains(string(snapshot), `"name": "Black Sea"`) {
		t.Fatalf("expected snapshot to contain feature properties, got %s", string(snapshot))
	}

	cached, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}
	if !strings.Contains(string(cached), `"mrgid": 3319`) {
		t.Fatalf("expected cache to be updated, got %s", string(cached))
	}
}

func TestInspectSourceFallsBackToLocalAndGeneratesSnapshot(t *testing.T) {
	dir := t.TempDir()
	fallbackPath := filepath.Join(dir, "fallback.json")
	cachePath := filepath.Join(dir, "cache.geojson")
	snapshotDir := filepath.Join(dir, "snapshots")
	if err := os.WriteFile(fallbackPath, []byte(`[
		{"lat": 46.48, "lon": 30.73},
		{"lat": 41.65, "lon": 41.63}
	]`), 0o644); err != nil {
		t.Fatalf("write fallback json: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary failure", http.StatusBadGateway)
	}))
	defer server.Close()

	result, err := InspectSource(InspectOptions{
		LocalPath:    fallbackPath,
		RemoteURL:    server.URL,
		CachePath:    cachePath,
		SnapshotPath: snapshotDir,
		HTTPClient:   server.Client(),
	})
	if err != nil {
		t.Fatalf("InspectSource returned error: %v", err)
	}

	if result.Source != fallbackPath {
		t.Fatalf("expected local fallback source %q, got %q", fallbackPath, result.Source)
	}
	if result.Metadata.Format != "point-array" {
		t.Fatalf("expected point-array format, got %q", result.Metadata.Format)
	}
	if len(result.LoadWarnings) != 1 || !strings.Contains(result.LoadWarnings[0], "using local fallback") {
		t.Fatalf("expected local fallback warning, got %+v", result.LoadWarnings)
	}
	if !strings.HasPrefix(result.SnapshotPath, snapshotDir) {
		t.Fatalf("expected snapshot in %q, got %q", snapshotDir, result.SnapshotPath)
	}
	if !strings.HasSuffix(result.SnapshotPath, ".json") {
		t.Fatalf("expected JSON snapshot, got %q", result.SnapshotPath)
	}
	if _, err := os.Stat(result.SnapshotPath); err != nil {
		t.Fatalf("expected snapshot to exist: %v", err)
	}
}
