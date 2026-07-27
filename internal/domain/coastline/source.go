package coastline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"coastal-geometry/internal/domain/geometry"
)

const DefaultCoastlineSnapshotDir = "data/snapshots"

type InspectOptions struct {
	LocalPath    string
	RemoteURL    string
	CachePath    string
	SnapshotPath string
	Refresh      bool
	HTTPClient   *http.Client
}

type SourceMetadata struct {
	Name                string
	RegionID            string
	Format              string
	RootType            string
	FeatureCount        int
	GeometryTypes       []string
	CoastlinePointCount int
	PayloadBytes        int
	Bounds              GeoBounds
}

type SourceInspection struct {
	Source       string
	DatasetName  string
	CachePath    string
	SnapshotPath string
	Metadata     SourceMetadata
	LoadWarnings []string
}

type resolvedSourcePayload struct {
	Payload      []byte
	Source       string
	CachePath    string
	LoadWarnings []string
}

type sourceFeatureCollection struct {
	Type       string                 `json:"type"`
	Properties map[string]any         `json:"properties"`
	Features   []sourceGeoJSONFeature `json:"features"`
	Geometry   *geoJSONGeometry       `json:"geometry"`
}

type sourceGeoJSONFeature struct {
	Type       string           `json:"type"`
	Properties map[string]any   `json:"properties"`
	Geometry   *geoJSONGeometry `json:"geometry"`
}

func InspectSource(options InspectOptions) (SourceInspection, error) {
	localPath := options.LocalPath
	if strings.TrimSpace(localPath) == "" {
		localPath = DefaultCoastlineJSONPath
	}

	remoteURL := strings.TrimSpace(options.RemoteURL)
	cachePath := strings.TrimSpace(options.CachePath)
	if remoteURL != "" && cachePath == "" {
		cachePath = defaultCoastlineCachePath(remoteURL)
	}

	payload, err := resolveSourcePayload(localPath, remoteURL, cachePath, options.Refresh, options.HTTPClient)
	if err != nil {
		return SourceInspection{}, err
	}

	metadata, err := inspectSourceMetadata(payload.Payload)
	if err != nil {
		return SourceInspection{}, fmt.Errorf("inspect coastline source %q: %w", payload.Source, err)
	}

	datasetName := datasetNameFromMetadata(metadata, localPath, remoteURL)
	snapshotPath, err := resolveSnapshotPath(options.SnapshotPath, metadata, datasetName)
	if err != nil {
		return SourceInspection{}, err
	}

	if err := writeSnapshot(snapshotPath, payload.Payload); err != nil {
		return SourceInspection{}, err
	}

	return SourceInspection{
		Source:       payload.Source,
		DatasetName:  datasetName,
		CachePath:    cachePath,
		SnapshotPath: snapshotPath,
		Metadata:     metadata,
		LoadWarnings: payload.LoadWarnings,
	}, nil
}

func resolveSourcePayload(localPath, remoteURL, cachePath string, refresh bool, client *http.Client) (resolvedSourcePayload, error) {
	if strings.TrimSpace(localPath) == "" {
		localPath = DefaultCoastlineJSONPath
	}

	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		payload, err := os.ReadFile(localPath)
		if err != nil {
			return resolvedSourcePayload{}, fmt.Errorf("read coastline json %q: %w", localPath, err)
		}
		return resolvedSourcePayload{
			Payload: payload,
			Source:  localPath,
		}, nil
	}

	if strings.TrimSpace(cachePath) == "" {
		cachePath = defaultCoastlineCachePath(remoteURL)
	}

	if !refresh {
		cached, err := os.ReadFile(cachePath)
		if err == nil {
			return resolvedSourcePayload{
				Payload:   cached,
				Source:    cachedSourceLabel(cachePath, remoteURL),
				CachePath: cachePath,
			}, nil
		}
	}

	remotePayload, remoteErr := fetchCoastlinePayload(client, remoteURL)
	if remoteErr == nil {
		result := resolvedSourcePayload{
			Payload:   remotePayload,
			Source:    remoteURL,
			CachePath: cachePath,
		}
		if cacheErr := writeCoastlineCache(cachePath, remotePayload); cacheErr != nil {
			result.LoadWarnings = append(result.LoadWarnings, fmt.Sprintf("unable to update coastline cache %q: %v", cachePath, cacheErr))
		}
		return result, nil
	}

	cached, cacheErr := os.ReadFile(cachePath)
	if cacheErr == nil {
		return resolvedSourcePayload{
			Payload:   cached,
			Source:    cachedSourceLabel(cachePath, remoteURL),
			CachePath: cachePath,
			LoadWarnings: []string{
				fmt.Sprintf("remote source %q unavailable, using cached GeoJSON %q: %v", remoteURL, cachePath, remoteErr),
			},
		}, nil
	}

	localPayload, localErr := os.ReadFile(localPath)
	if localErr != nil {
		return resolvedSourcePayload{}, fmt.Errorf("load coastline from remote %q: %v; load cache %q: %v; load fallback %q: %w", remoteURL, remoteErr, cachePath, cacheErr, localPath, localErr)
	}

	return resolvedSourcePayload{
		Payload: localPayload,
		Source:  localPath,
		LoadWarnings: []string{
			fmt.Sprintf("remote source %q unavailable, using local fallback %q: %v", remoteURL, localPath, remoteErr),
		},
	}, nil
}

func inspectSourceMetadata(data []byte) (SourceMetadata, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return SourceMetadata{}, fmt.Errorf("empty coastline payload")
	}

	points, err := parseCoastlineData(trimmed, GeoBounds{})
	if err != nil {
		return SourceMetadata{}, err
	}

	meta := SourceMetadata{
		PayloadBytes:        len(trimmed),
		CoastlinePointCount: len(points),
		Bounds:              boundsFromPoints(points),
	}

	switch trimmed[0] {
	case '[':
		meta.Format = "point-array"
		meta.RootType = "array"
		meta.FeatureCount = 1
		meta.GeometryTypes = []string{"PointArray"}
		return meta, nil
	case '{':
		meta.Format = "GeoJSON"
	default:
		return SourceMetadata{}, fmt.Errorf("unsupported coastline payload")
	}

	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return SourceMetadata{}, fmt.Errorf("parse json envelope: %w", err)
	}
	meta.RootType = envelope.Type

	var root sourceFeatureCollection
	if err := json.Unmarshal(trimmed, &root); err != nil {
		return SourceMetadata{}, fmt.Errorf("parse geojson envelope: %w", err)
	}

	geometryTypes := map[string]struct{}{}
	switch strings.ToLower(root.Type) {
	case "featurecollection":
		meta.FeatureCount = len(root.Features)
		meta.Name = propertyString(root.Properties, "name")
		meta.RegionID = propertyString(root.Properties, "mrgid")
		for _, feature := range root.Features {
			if meta.Name == "" {
				meta.Name = propertyString(feature.Properties, "name")
			}
			if meta.RegionID == "" {
				meta.RegionID = propertyString(feature.Properties, "mrgid")
			}
			if feature.Geometry != nil {
				geometryTypes[feature.Geometry.Type] = struct{}{}
			}
		}
	case "feature":
		meta.FeatureCount = 1
		if meta.Name == "" {
			meta.Name = propertyString(root.Properties, "name")
		}
		if meta.RegionID == "" {
			meta.RegionID = propertyString(root.Properties, "mrgid")
		}
		if root.Geometry != nil {
			geometryTypes[root.Geometry.Type] = struct{}{}
		}
	default:
		meta.FeatureCount = 1
		if root.Type != "" {
			geometryTypes[root.Type] = struct{}{}
		}
	}

	meta.GeometryTypes = geometryTypesList(geometryTypes)
	return meta, nil
}

func boundsFromPoints(points []geometry.LatLon) GeoBounds {
	if len(points) == 0 {
		return GeoBounds{}
	}

	bounds := GeoBounds{
		MinLat: points[0].Lat,
		MaxLat: points[0].Lat,
		MinLon: points[0].Lon,
		MaxLon: points[0].Lon,
	}
	for _, point := range points[1:] {
		if point.Lat < bounds.MinLat {
			bounds.MinLat = point.Lat
		}
		if point.Lat > bounds.MaxLat {
			bounds.MaxLat = point.Lat
		}
		if point.Lon < bounds.MinLon {
			bounds.MinLon = point.Lon
		}
		if point.Lon > bounds.MaxLon {
			bounds.MaxLon = point.Lon
		}
	}

	return bounds
}

func geometryTypesList(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}

	list := make([]string, 0, len(set))
	for name := range set {
		list = append(list, name)
	}
	sort.Strings(list)
	return list
}

func propertyString(properties map[string]any, key string) string {
	if len(properties) == 0 {
		return ""
	}

	for propKey, value := range properties {
		if !strings.EqualFold(propKey, key) {
			continue
		}
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(typed)
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		case int:
			return strconv.Itoa(typed)
		case int64:
			return strconv.FormatInt(typed, 10)
		case json.Number:
			return typed.String()
		default:
			return strings.TrimSpace(fmt.Sprint(typed))
		}
	}

	return ""
}

func datasetNameFromMetadata(meta SourceMetadata, localPath, remoteURL string) string {
	if strings.TrimSpace(meta.Name) != "" {
		return strings.TrimSpace(meta.Name)
	}
	if strings.TrimSpace(remoteURL) != "" {
		if parsed, err := url.Parse(remoteURL); err == nil {
			if base := filepath.Base(parsed.Path); base != "." && base != "/" && base != "" {
				return base
			}
			if parsed.Host != "" {
				return parsed.Host
			}
		}
		return remoteURL
	}
	if strings.TrimSpace(localPath) != "" {
		return filepath.Base(localPath)
	}
	return "coastline"
}

func resolveSnapshotPath(output string, meta SourceMetadata, datasetName string) (string, error) {
	filename := snapshotFilename(meta, datasetName, time.Now().UTC())

	if strings.TrimSpace(output) == "" {
		if err := os.MkdirAll(DefaultCoastlineSnapshotDir, 0o755); err != nil {
			return "", fmt.Errorf("create snapshot directory %q: %w", DefaultCoastlineSnapshotDir, err)
		}
		return filepath.Abs(filepath.Join(DefaultCoastlineSnapshotDir, filename))
	}

	lower := strings.ToLower(output)
	if strings.HasSuffix(lower, ".geojson") || strings.HasSuffix(lower, ".json") {
		dir := filepath.Dir(output)
		if dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", fmt.Errorf("create snapshot directory %q: %w", dir, err)
			}
		}
		return filepath.Abs(output)
	}

	if err := os.MkdirAll(output, 0o755); err != nil {
		return "", fmt.Errorf("create snapshot directory %q: %w", output, err)
	}

	return filepath.Abs(filepath.Join(output, filename))
}

func snapshotFilename(meta SourceMetadata, datasetName string, now time.Time) string {
	slug := slugify(datasetName)
	if slug == "" {
		slug = "coastline"
	}

	ext := ".geojson"
	if meta.Format == "point-array" {
		ext = ".json"
	}

	return fmt.Sprintf("%s-%s%s", slug, now.Format("20060102-150405"), ext)
}

func slugify(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if r > unicode.MaxASCII {
				continue
			}
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}

	result := strings.Trim(b.String(), "-")
	result = strings.ReplaceAll(result, "--", "-")
	return result
}

func writeSnapshot(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create snapshot directory for %q: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot %q: %w", path, err)
	}
	return nil
}
