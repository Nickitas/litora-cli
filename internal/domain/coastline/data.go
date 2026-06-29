package coastline

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"coastal-geometry/internal/domain/geometry"
)

const (
	DefaultCoastlineJSONPath = "data/black-sea.json"
	DefaultCoastlineCacheDir = "data/cache"
	defaultHTTPTimeout       = 12 * time.Second
	marineRegionsWFSURL      = "https://geo.vliz.be/geoserver/MarineRegions/wfs"
	blackSeaMarineRegionID   = 3319
)

var DefaultBlackSeaBounds = GeoBounds{
	MinLat: 40.5,
	MaxLat: 46.8,
	MinLon: 27.0,
	MaxLon: 42.2,
}

var DefaultCoastlineGeoJSONURL = buildDefaultCoastlineGeoJSONURL()

type ValidationReport struct {
	Fixes    []string
	Warnings []string
}

type GeoBounds struct {
	MinLat float64
	MaxLat float64
	MinLon float64
	MaxLon float64
}

func (b GeoBounds) IsZero() bool {
	return b.MinLat == 0 && b.MaxLat == 0 && b.MinLon == 0 && b.MaxLon == 0
}

func (b GeoBounds) Contains(point geometry.LatLon) bool {
	return point.Lat >= b.MinLat && point.Lat <= b.MaxLat &&
		point.Lon >= b.MinLon && point.Lon <= b.MaxLon
}

type LoadOptions struct {
	LocalPath    string
	RemoteURL    string
	RemoteBounds GeoBounds
	CachePath    string
	Refresh      bool
	HTTPClient   *http.Client
}

type LoadResult struct {
	Points       []geometry.LatLon
	Validation   ValidationReport
	Source       string
	DatasetName  string
	LoadWarnings []string
}

func LoadFromJSON(filename string) ([]geometry.LatLon, ValidationReport, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, ValidationReport{}, fmt.Errorf("read coastline json %q: %w", filename, err)
	}

	normalized, report, err := loadCoastlineData(data, filename, GeoBounds{})
	if err != nil {
		return nil, ValidationReport{}, err
	}

	return normalized, report, nil
}

func Load(options LoadOptions) (LoadResult, error) {
	localPath := options.LocalPath
	if strings.TrimSpace(localPath) == "" {
		localPath = DefaultCoastlineJSONPath
	}

	remoteURL := strings.TrimSpace(options.RemoteURL)
	cachePath := strings.TrimSpace(options.CachePath)
	payload, err := resolveSourcePayload(localPath, remoteURL, cachePath, options.Refresh, options.HTTPClient)
	if err != nil {
		return LoadResult{}, err
	}

	points, report, err := loadCoastlineData(payload.Payload, payload.Source, options.RemoteBounds)
	if err != nil {
		return LoadResult{}, err
	}

	datasetName := filepath.Base(localPath)
	if metadata, metaErr := inspectSourceMetadata(payload.Payload); metaErr == nil {
		datasetName = datasetNameFromMetadata(metadata, localPath, remoteURL)
	}

	return LoadResult{
		Points:       points,
		Validation:   report,
		Source:       payload.Source,
		DatasetName:  datasetName,
		LoadWarnings: payload.LoadWarnings,
	}, nil
}

func FetchCoastlineData(url string) ([]geometry.LatLon, error) {
	return fetchCoastlineData(nil, url, GeoBounds{})
}

func fetchCoastlineData(client *http.Client, url string, bounds GeoBounds) ([]geometry.LatLon, error) {
	payload, err := fetchCoastlinePayload(client, url)
	if err != nil {
		return nil, err
	}

	points, _, err := loadCoastlineData(payload, url, bounds)
	if err != nil {
		return nil, err
	}

	return points, nil
}

func fetchCoastlinePayload(client *http.Client, url string) ([]byte, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("remote url is empty")
	}

	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build GET request for %q: %w", url, err)
	}
	req.Header.Set("Accept", "application/geo+json, application/json;q=0.9, */*;q=0.1")
	req.Header.Set("User-Agent", "lito/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request coastline url %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request coastline url %q: unexpected status %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read coastline response %q: %w", url, err)
	}

	return body, nil
}

func loadCachedCoastline(cachePath string, bounds GeoBounds) ([]geometry.LatLon, ValidationReport, error) {
	if strings.TrimSpace(cachePath) == "" {
		return nil, ValidationReport{}, fmt.Errorf("cache path is empty")
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, ValidationReport{}, fmt.Errorf("read coastline cache %q: %w", cachePath, err)
	}

	return loadCoastlineData(data, cachePath, bounds)
}

func writeCoastlineCache(cachePath string, data []byte) error {
	if strings.TrimSpace(cachePath) == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return fmt.Errorf("create cache directory for %q: %w", cachePath, err)
	}

	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
		return fmt.Errorf("write cache file %q: %w", cachePath, err)
	}

	return nil
}

func loadCoastlineData(data []byte, source string, bounds GeoBounds) ([]geometry.LatLon, ValidationReport, error) {
	points, err := parseCoastlineData(data, bounds)
	if err != nil {
		return nil, ValidationReport{}, fmt.Errorf("parse coastline data %q: %w", source, err)
	}

	normalized, report, err := normalizeLoadedPoints(points)
	if err != nil {
		return nil, ValidationReport{}, fmt.Errorf("validate coastline data %q: %w", source, err)
	}

	return normalized, report, nil
}

func normalizeLoadedPoints(points []geometry.LatLon) ([]geometry.LatLon, ValidationReport, error) {
	closed := isClosedPolyline(points)
	if closed {
		points = points[:len(points)-1]
	}

	if len(points) < 2 {
		return nil, ValidationReport{}, fmt.Errorf("coastline data must contain at least 2 points")
	}

	for i, point := range points {
		if point.Lat < -90 || point.Lat > 90 {
			return nil, ValidationReport{}, fmt.Errorf("coastline data has invalid latitude at index %d: %f", i, point.Lat)
		}
		if point.Lon < -180 || point.Lon > 180 {
			return nil, ValidationReport{}, fmt.Errorf("coastline data has invalid longitude at index %d: %f", i, point.Lon)
		}
	}

	normalized, report, err := validateAndNormalizePoints(points)
	if err != nil {
		return nil, ValidationReport{}, err
	}

	if closed && len(normalized) > 0 {
		normalized = append(normalized, normalized[0])
	}

	return normalized, report, nil
}

func isClosedPolyline(points []geometry.LatLon) bool {
	if len(points) < 2 {
		return false
	}
	return pointKey(points[0]) == pointKey(points[len(points)-1])
}

func parseCoastlineData(data []byte, bounds GeoBounds) ([]geometry.LatLon, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty coastline payload")
	}

	switch trimmed[0] {
	case '[':
		var points []geometry.LatLon
		if err := json.Unmarshal(trimmed, &points); err != nil {
			return nil, fmt.Errorf("parse point array: %w", err)
		}
		return points, nil
	case '{':
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(trimmed, &envelope); err != nil {
			return nil, fmt.Errorf("parse json envelope: %w", err)
		}
		switch strings.ToLower(envelope.Type) {
		case "featurecollection", "feature", "polygon", "multipolygon", "linestring", "multilinestring", "geometrycollection":
			return parseGeoJSONPoints(trimmed, bounds)
		default:
			return nil, fmt.Errorf("unsupported json object type %q", envelope.Type)
		}
	default:
		return nil, fmt.Errorf("unsupported coastline payload")
	}
}

type geoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []geoJSONFeature `json:"features"`
}

type geoJSONFeature struct {
	Type     string           `json:"type"`
	Geometry *geoJSONGeometry `json:"geometry"`
}

type geoJSONGeometry struct {
	Type        string            `json:"type"`
	Coordinates json.RawMessage   `json:"coordinates"`
	Geometries  []geoJSONGeometry `json:"geometries"`
}

func parseGeoJSONPoints(data []byte, bounds GeoBounds) ([]geometry.LatLon, error) {
	var collection geoJSONFeatureCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("parse geojson root: %w", err)
	}

	var sequences [][]geometry.LatLon
	switch strings.ToLower(collection.Type) {
	case "featurecollection":
		for _, feature := range collection.Features {
			if feature.Geometry == nil {
				continue
			}
			paths, err := geometrySequencesFromGeoJSON(*feature.Geometry)
			if err != nil {
				return nil, err
			}
			sequences = append(sequences, paths...)
		}
	case "feature":
		var feature geoJSONFeature
		if err := json.Unmarshal(data, &feature); err != nil {
			return nil, fmt.Errorf("parse geojson feature: %w", err)
		}
		if feature.Geometry == nil {
			return nil, fmt.Errorf("geojson feature has no geometry")
		}

		paths, err := geometrySequencesFromGeoJSON(*feature.Geometry)
		if err != nil {
			return nil, err
		}
		sequences = append(sequences, paths...)
	default:
		var geometry geoJSONGeometry
		if err := json.Unmarshal(data, &geometry); err != nil {
			return nil, fmt.Errorf("parse geojson geometry: %w", err)
		}

		paths, err := geometrySequencesFromGeoJSON(geometry)
		if err != nil {
			return nil, err
		}
		sequences = append(sequences, paths...)
	}

	if len(sequences) == 0 {
		return nil, fmt.Errorf("geojson does not contain coastline geometry")
	}

	filtered := filterGeoJSONSequences(sequences, bounds)
	if len(filtered) == 0 {
		if !bounds.IsZero() {
			return nil, fmt.Errorf("geojson does not contain coordinates inside target bounds")
		}
		return nil, fmt.Errorf("geojson does not contain enough coordinates")
	}

	best := bestSequence(filtered)
	if len(best) < 2 {
		return nil, fmt.Errorf("geojson sequence does not contain enough coordinates")
	}

	return best, nil
}

func geometrySequencesFromGeoJSON(geom geoJSONGeometry) ([][]geometry.LatLon, error) {
	switch strings.ToLower(geom.Type) {
	case "linestring":
		points, err := decodeCoordinateSequence(geom.Coordinates)
		if err != nil {
			return nil, fmt.Errorf("parse linestring coordinates: %w", err)
		}
		return [][]geometry.LatLon{points}, nil
	case "multilinestring":
		var raw []json.RawMessage
		if err := json.Unmarshal(geom.Coordinates, &raw); err != nil {
			return nil, fmt.Errorf("parse multilinestring coordinates: %w", err)
		}

		sequences := make([][]geometry.LatLon, 0, len(raw))
		for _, item := range raw {
			points, err := decodeCoordinateSequence(item)
			if err != nil {
				return nil, fmt.Errorf("parse multilinestring path: %w", err)
			}
			sequences = append(sequences, points)
		}
		return sequences, nil
	case "polygon":
		var raw []json.RawMessage
		if err := json.Unmarshal(geom.Coordinates, &raw); err != nil {
			return nil, fmt.Errorf("parse polygon coordinates: %w", err)
		}

		sequences := make([][]geometry.LatLon, 0, len(raw))
		for _, ring := range raw {
			points, err := decodeCoordinateSequence(ring)
			if err != nil {
				return nil, fmt.Errorf("parse polygon ring: %w", err)
			}
			sequences = append(sequences, points)
		}
		return sequences, nil
	case "multipolygon":
		var polygons []json.RawMessage
		if err := json.Unmarshal(geom.Coordinates, &polygons); err != nil {
			return nil, fmt.Errorf("parse multipolygon coordinates: %w", err)
		}

		sequences := make([][]geometry.LatLon, 0, len(polygons))
		for _, polygon := range polygons {
			var rings []json.RawMessage
			if err := json.Unmarshal(polygon, &rings); err != nil {
				return nil, fmt.Errorf("parse multipolygon rings: %w", err)
			}
			for _, ring := range rings {
				points, err := decodeCoordinateSequence(ring)
				if err != nil {
					return nil, fmt.Errorf("parse multipolygon ring: %w", err)
				}
				sequences = append(sequences, points)
			}
		}
		return sequences, nil
	case "geometrycollection":
		sequences := make([][]geometry.LatLon, 0, len(geom.Geometries))
		for _, item := range geom.Geometries {
			paths, err := geometrySequencesFromGeoJSON(item)
			if err != nil {
				return nil, err
			}
			sequences = append(sequences, paths...)
		}
		return sequences, nil
	default:
		return nil, fmt.Errorf("unsupported geojson geometry type %q", geom.Type)
	}
}

func decodeCoordinateSequence(data json.RawMessage) ([]geometry.LatLon, error) {
	var raw [][]float64
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	points := make([]geometry.LatLon, 0, len(raw))
	for idx, coordinate := range raw {
		if len(coordinate) < 2 {
			return nil, fmt.Errorf("coordinate at index %d must contain lon/lat", idx)
		}
		points = append(points, geometry.LatLon{
			Lat: coordinate[1],
			Lon: coordinate[0],
		})
	}

	return points, nil
}

func filterGeoJSONSequences(sequences [][]geometry.LatLon, bounds GeoBounds) [][]geometry.LatLon {
	if bounds.IsZero() {
		return sequences
	}

	filtered := make([][]geometry.LatLon, 0, len(sequences))
	for _, sequence := range sequences {
		var current []geometry.LatLon
		for _, point := range sequence {
			if bounds.Contains(point) {
				current = append(current, point)
				continue
			}
			if len(current) >= 2 {
				filtered = append(filtered, current)
			}
			current = nil
		}
		if len(current) >= 2 {
			filtered = append(filtered, current)
		}
	}
	return filtered
}

func bestSequence(sequences [][]geometry.LatLon) []geometry.LatLon {
	best := sequences[0]
	bestLength := geometry.PolylineLength(best)
	bestPoints := len(best)

	for _, sequence := range sequences[1:] {
		length := geometry.PolylineLength(sequence)
		if length > bestLength || (length == bestLength && len(sequence) > bestPoints) {
			best = sequence
			bestLength = length
			bestPoints = len(sequence)
		}
	}

	return best
}

func buildDefaultCoastlineGeoJSONURL() string {
	query := url.Values{
		"service":      {"WFS"},
		"version":      {"1.0.0"},
		"request":      {"GetFeature"},
		"typeName":     {"iho"},
		"cql_filter":   {fmt.Sprintf("mrgid=%d", blackSeaMarineRegionID)},
		"outputFormat": {"application/json"},
	}

	return marineRegionsWFSURL + "?" + query.Encode()
}

func defaultCoastlineCachePath(remoteURL string) string {
	if remoteURL == DefaultCoastlineGeoJSONURL {
		return filepath.Join(DefaultCoastlineCacheDir, "black-sea.geojson")
	}

	sum := sha1.Sum([]byte(remoteURL))
	return filepath.Join(DefaultCoastlineCacheDir, fmt.Sprintf("coastline-%x.geojson", sum[:6]))
}

func cachedSourceLabel(cachePath, remoteURL string) string {
	return fmt.Sprintf("%s (cached copy of %s)", cachePath, remoteURL)
}
