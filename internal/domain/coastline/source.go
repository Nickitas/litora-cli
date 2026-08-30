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

	"coastal-geometry/internal/domain/blacksea"
	"coastal-geometry/internal/domain/geometry"
)

// ValidationReport содержит результаты проверки и исправления данных
type ValidationReport struct {
	Fixes    []string // Выполненные исправления
	Warnings []string // Предупреждения
}

// GeoBounds описывает прямоугольную область на карте
type GeoBounds struct {
	MinLat float64
	MaxLat float64
	MinLon float64
	MaxLon float64
}

// IsZero проверяет, что границы не установлены
func (b GeoBounds) IsZero() bool {
	return b.MinLat == 0 && b.MaxLat == 0 && b.MinLon == 0 && b.MaxLon == 0
}

// Contains проверяет, что точка находится внутри границ
func (b GeoBounds) Contains(point geometry.LatLon) bool {
	return point.Lat >= b.MinLat && point.Lat <= b.MaxLat &&
		point.Lon >= b.MinLon && point.Lon <= b.MaxLon
}

// LoadOptions параметры загрузки береговой линии
type LoadOptions struct {
	LocalPath    string       // Локальный путь к файлу; если задан, имеет приоритет над удалённым источником
	RemoteURL    string       // URL удалённого источника; используется только при пустом LocalPath
	RemoteBounds GeoBounds    // Границы для фильтрации
	CachePath    string       // Путь к кэшу
	Refresh      bool         // Принудительное обновление кэша
	HTTPClient   *http.Client // HTTP-клиент для запросов
}

// LoadResult результат загрузки береговой линии
type LoadResult struct {
	Points       []geometry.LatLon // Нормализованные точки
	Validation   ValidationReport  // Отчёт о валидации
	Source       string            // Источник данных
	DatasetName  string            // Имя набора данных
	LoadWarnings []string          // Предупреждения при загрузке
}

// PolygonLoadResult содержит замкнутую внешнюю границу Чёрного моря и внутренние
// кольца островов. В отличие от Load результат не отбрасывает малые кольца
// GeoJSON, поэтому пригоден для построения сетки по всей поверхности моря.
type PolygonLoadResult struct {
	Outer        []geometry.LatLon
	Holes        [][]geometry.LatLon
	Validation   ValidationReport
	Source       string
	DatasetName  string
	LoadWarnings []string
}

// LoadFromJSON загружает береговую линию из локального JSON-файла
func LoadFromJSON(filename string) ([]geometry.LatLon, ValidationReport, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, ValidationReport{}, fmt.Errorf("ошибка чтения JSON береговой линии %q: %w", filename, err)
	}

	normalized, report, err := loadCoastlineData(data, filename, GeoBounds{})
	if err != nil {
		return nil, ValidationReport{}, err
	}

	return normalized, report, nil
}

// Load загружает береговую линию с различными источниками данных
func Load(options LoadOptions) (LoadResult, error) {
	localPath := strings.TrimSpace(options.LocalPath)
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

// LoadPolygon загружает полигон Чёрного моря, сохраняя острова как внутренние
// кольца. Для простого массива координат создаётся один внешний контур.
func LoadPolygon(options LoadOptions) (PolygonLoadResult, error) {
	localPath := strings.TrimSpace(options.LocalPath)
	remoteURL := strings.TrimSpace(options.RemoteURL)
	cachePath := strings.TrimSpace(options.CachePath)
	payload, err := resolveSourcePayload(localPath, remoteURL, cachePath, options.Refresh, options.HTTPClient)
	if err != nil {
		return PolygonLoadResult{}, err
	}

	sequences, err := parsePolygonSequences(payload.Payload, options.RemoteBounds)
	if err != nil {
		return PolygonLoadResult{}, fmt.Errorf("ошибка парсинга полигона Чёрного моря %q: %w", payload.Source, err)
	}
	if len(sequences) == 0 {
		return PolygonLoadResult{}, fmt.Errorf("источник %q не содержит колец Чёрного моря", payload.Source)
	}

	outerIndex := largestRingIndex(sequences)
	outer, report, err := normalizeMeshRing(sequences[outerIndex])
	if err != nil {
		return PolygonLoadResult{}, fmt.Errorf("ошибка внешнего кольца Чёрного моря %q: %w", payload.Source, err)
	}

	holes := make([][]geometry.LatLon, 0, len(sequences)-1)
	for index, sequence := range sequences {
		if index == outerIndex || len(sequence) < 3 {
			continue
		}
		ring, ringReport, ringErr := normalizeMeshRing(sequence)
		if ringErr != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("внутреннее кольцо %d пропущено: %v", index, ringErr))
			continue
		}
		if !pointInRing(ring[0], outer) {
			report.Warnings = append(report.Warnings, fmt.Sprintf("отдельное кольцо %d вне основного полигона Чёрного моря пропущено", index))
			continue
		}
		report.Fixes = append(report.Fixes, ringReport.Fixes...)
		report.Warnings = append(report.Warnings, ringReport.Warnings...)
		holes = append(holes, ring)
	}

	datasetName := filepath.Base(localPath)
	if metadata, metaErr := inspectSourceMetadata(payload.Payload); metaErr == nil {
		datasetName = datasetNameFromMetadata(metadata, localPath, remoteURL)
	}

	return PolygonLoadResult{
		Outer:        outer,
		Holes:        holes,
		Validation:   report,
		Source:       payload.Source,
		DatasetName:  datasetName,
		LoadWarnings: payload.LoadWarnings,
	}, nil
}

// FetchCoastlineData загружает данные береговой линии по URL
func FetchCoastlineData(url string) ([]geometry.LatLon, error) {
	return fetchCoastlineData(nil, url, GeoBounds{})
}

// fetchCoastlineData загружает данные береговой линии по URL с опциональной фильтрацией по границам
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

// fetchCoastlinePayload выполняет HTTP-запрос для получения GeoJSON
func fetchCoastlinePayload(client *http.Client, url string) ([]byte, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("URL удалённого источника пуст")
	}

	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания GET-запроса для %q: %w", url, err)
	}
	req.Header.Set("Accept", "application/geo+json, application/json;q=0.9, */*;q=0.1")
	req.Header.Set("User-Agent", "lito/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса URL береговой линии %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка запроса URL береговой линии %q: неожиданный статус %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа от %q: %w", url, err)
	}

	return body, nil
}

// loadCachedCoastline загружает береговую линию из кэша
func loadCachedCoastline(cachePath string, bounds GeoBounds) ([]geometry.LatLon, ValidationReport, error) {
	if strings.TrimSpace(cachePath) == "" {
		return nil, ValidationReport{}, fmt.Errorf("путь к кэшу пуст")
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, ValidationReport{}, fmt.Errorf("ошибка чтения кэша береговой линии %q: %w", cachePath, err)
	}

	return loadCoastlineData(data, cachePath, bounds)
}

// writeCoastlineCache записывает данные береговой линии в кэш
func writeCoastlineCache(cachePath string, data []byte) error {
	if strings.TrimSpace(cachePath) == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), dirPermissions); err != nil {
		return fmt.Errorf("ошибка создания директории кэша для %q: %w", cachePath, err)
	}

	if err := os.WriteFile(cachePath, data, filePermissions); err != nil {
		return fmt.Errorf("ошибка записи файла кэша %q: %w", cachePath, err)
	}

	return nil
}

// loadCoastlineData загружает и нормализует данные береговой линии
func loadCoastlineData(data []byte, source string, bounds GeoBounds) ([]geometry.LatLon, ValidationReport, error) {
	points, err := parseCoastlineData(data, bounds)
	if err != nil {
		return nil, ValidationReport{}, fmt.Errorf("ошибка парсинга данных береговой линии %q: %w", source, err)
	}

	normalized, report, err := normalizeLoadedPoints(points)
	if err != nil {
		return nil, ValidationReport{}, fmt.Errorf("ошибка валидации данных береговой линии %q: %w", source, err)
	}

	return normalized, report, nil
}

// normalizeLoadedPoints нормализует загруженные точки
func normalizeLoadedPoints(points []geometry.LatLon) ([]geometry.LatLon, ValidationReport, error) {
	closed := isClosedPolyline(points)
	if closed {
		points = points[:len(points)-1]
	}

	if len(points) < 2 {
		return nil, ValidationReport{}, fmt.Errorf("данные береговой линии должны содержать минимум 2 точки")
	}

	// Валидация координат
	for i, point := range points {
		if point.Lat < minValidLatitude || point.Lat > maxValidLatitude {
			return nil, ValidationReport{}, fmt.Errorf("данные береговой линии содержат недопустимую широту в индексе %d: %f", i, point.Lat)
		}
		if point.Lon < minValidLongitude || point.Lon > maxValidLongitude {
			return nil, ValidationReport{}, fmt.Errorf("данные береговой линии содержат недопустимую долготу в индексе %d: %f", i, point.Lon)
		}
		if !blacksea.Contains(point.Lat, point.Lon) {
			return nil, ValidationReport{}, fmt.Errorf(
				"точка береговой линии в индексе %d (%.6f, %.6f) находится вне области Чёрного моря [%.1f, %.1f] × [%.1f, %.1f]",
				i, point.Lat, point.Lon,
				blacksea.MinLatitude, blacksea.MaxLatitude,
				blacksea.MinLongitude, blacksea.MaxLongitude,
			)
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

// isClosedPolyline проверяет, является ли полилиния замкнутой
func isClosedPolyline(points []geometry.LatLon) bool {
	if len(points) < 2 {
		return false
	}
	return pointKey(points[0]) == pointKey(points[len(points)-1])
}

// parseCoastlineData парсит данные береговой линии из различных форматов
func parseCoastlineData(data []byte, bounds GeoBounds) ([]geometry.LatLon, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("пустые данные береговой линии")
	}

	switch trimmed[0] {
	case '[':
		var points []geometry.LatLon
		if err := json.Unmarshal(trimmed, &points); err != nil {
			return nil, fmt.Errorf("ошибка парсинга массива точек: %w", err)
		}
		return points, nil
	case '{':
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(trimmed, &envelope); err != nil {
			return nil, fmt.Errorf("ошибка парсинга JSON-оболочки: %w", err)
		}
		switch strings.ToLower(envelope.Type) {
		case "featurecollection", "feature", "polygon", "multipolygon", "linestring", "multilinestring", "geometrycollection":
			return parseGeoJSONPoints(trimmed, bounds)
		default:
			return parseCoastlineEnvelope(trimmed, envelope.Type)
		}
	default:
		return nil, fmt.Errorf("неподдерживаемый формат данных береговой линии")
	}
}

// parseCoastlineEnvelope извлекает последовательность координат из JSON-объекта
// с полем coastline. Такой компактный формат удобен для локальных наблюдений,
// не представленных в виде GeoJSON.
func parseCoastlineEnvelope(data []byte, objectType string) ([]geometry.LatLon, error) {
	var envelope struct {
		Coastline []geometry.LatLon `json:"coastline"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON-объекта: %w", err)
	}
	if len(envelope.Coastline) >= 2 {
		return envelope.Coastline, nil
	}
	return nil, fmt.Errorf("неподдерживаемый тип JSON-объекта: %q", objectType)
}

// geoJSONFeatureCollection представляет коллекцию GeoJSON-объектов
type geoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []geoJSONFeature `json:"features"`
}

// geoJSONFeature представляет GeoJSON-объект
type geoJSONFeature struct {
	Type     string           `json:"type"`
	Geometry *geoJSONGeometry `json:"geometry"`
}

// geoJSONGeometry представляет геометрию GeoJSON
type geoJSONGeometry struct {
	Type        string            `json:"type"`
	Coordinates json.RawMessage   `json:"coordinates"`
	Geometries  []geoJSONGeometry `json:"geometries"`
}

// parseGeoJSONPoints парсит точки из GeoJSON
func parseGeoJSONPoints(data []byte, bounds GeoBounds) ([]geometry.LatLon, error) {
	sequences, err := parseGeoJSONSequences(data)
	if err != nil {
		return nil, err
	}

	filtered := filterGeoJSONSequences(sequences, bounds)
	if len(filtered) == 0 {
		if !bounds.IsZero() {
			return nil, fmt.Errorf("GeoJSON не содержит координат внутри указанных границ")
		}
		return nil, fmt.Errorf("GeoJSON не содержит достаточно координат")
	}

	best := bestSequence(filtered)
	if len(best) < 2 {
		return nil, fmt.Errorf("последовательность GeoJSON не содержит достаточно координат")
	}

	return best, nil
}

// parseGeoJSONSequences извлекает все линейные последовательности GeoJSON,
// не выбирая только самое длинное кольцо.
func parseGeoJSONSequences(data []byte) ([][]geometry.LatLon, error) {
	var collection geoJSONFeatureCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("ошибка парсинга корня GeoJSON: %w", err)
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
			return nil, fmt.Errorf("ошибка парсинга GeoJSON-объекта: %w", err)
		}
		if feature.Geometry == nil {
			return nil, fmt.Errorf("GeoJSON-объект не содержит геометрии")
		}

		paths, err := geometrySequencesFromGeoJSON(*feature.Geometry)
		if err != nil {
			return nil, err
		}
		sequences = append(sequences, paths...)
	default:
		var geometry geoJSONGeometry
		if err := json.Unmarshal(data, &geometry); err != nil {
			return nil, fmt.Errorf("ошибка парсинга геометрии GeoJSON: %w", err)
		}

		paths, err := geometrySequencesFromGeoJSON(geometry)
		if err != nil {
			return nil, err
		}
		sequences = append(sequences, paths...)
	}

	if len(sequences) == 0 {
		return nil, fmt.Errorf("GeoJSON не содержит геометрии береговой линии")
	}

	return sequences, nil
}

func parsePolygonSequences(data []byte, bounds GeoBounds) ([][]geometry.LatLon, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("пустые данные Чёрного моря")
	}
	if trimmed[0] != '{' {
		points, err := parseCoastlineData(trimmed, bounds)
		if err != nil {
			return nil, err
		}
		return [][]geometry.LatLon{points}, nil
	}

	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, err
	}
	switch strings.ToLower(envelope.Type) {
	case "featurecollection", "feature", "polygon", "multipolygon", "linestring", "multilinestring", "geometrycollection":
		sequences, err := parseGeoJSONSequences(trimmed)
		if err != nil {
			return nil, err
		}
		return filterGeoJSONSequences(sequences, bounds), nil
	default:
		points, err := parseCoastlineEnvelope(trimmed, envelope.Type)
		if err != nil {
			return nil, err
		}
		return [][]geometry.LatLon{points}, nil
	}
}

func largestRingIndex(sequences [][]geometry.LatLon) int {
	bestIndex := 0
	bestArea := geometry.Area(sequences[0])
	for index := 1; index < len(sequences); index++ {
		if area := geometry.Area(sequences[index]); area > bestArea {
			bestArea = area
			bestIndex = index
		}
	}
	return bestIndex
}

func normalizeMeshRing(points []geometry.LatLon) ([]geometry.LatLon, ValidationReport, error) {
	if len(points) < 3 {
		return nil, ValidationReport{}, fmt.Errorf("кольцо должно содержать минимум три точки")
	}
	working := append([]geometry.LatLon(nil), points...)
	if isClosedPolyline(working) {
		working = working[:len(working)-1]
	}
	report := ValidationReport{}
	deduplicated := make([]geometry.LatLon, 0, len(working))
	for index, point := range working {
		if point.Lat < minValidLatitude || point.Lat > maxValidLatitude || point.Lon < minValidLongitude || point.Lon > maxValidLongitude {
			return nil, report, fmt.Errorf("недопустимая координата кольца в индексе %d", index)
		}
		if !blacksea.Contains(point.Lat, point.Lon) {
			return nil, report, fmt.Errorf("координата кольца в индексе %d находится вне области Чёрного моря", index)
		}
		if len(deduplicated) > 0 && pointKey(deduplicated[len(deduplicated)-1]) == pointKey(point) {
			continue
		}
		deduplicated = append(deduplicated, point)
	}
	if len(deduplicated) < 3 {
		return nil, report, fmt.Errorf("после удаления соседних дубликатов осталось меньше трёх точек")
	}
	if removed := len(working) - len(deduplicated); removed > 0 {
		report.Fixes = append(report.Fixes, fmt.Sprintf("удалены соседние повторяющиеся координаты кольца: %d", removed))
	}
	if intersections := findSelfIntersections(deduplicated); len(intersections) > 0 {
		return nil, report, fmt.Errorf("кольцо имеет самопересечения: сегменты %s", formatIntersections(intersections))
	}
	deduplicated = append(deduplicated, deduplicated[0])
	return deduplicated, report, nil
}

func pointInRing(point geometry.LatLon, ring []geometry.LatLon) bool {
	inside := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		a, b := ring[i], ring[j]
		crosses := (a.Lat > point.Lat) != (b.Lat > point.Lat)
		if crosses && point.Lon < (b.Lon-a.Lon)*(point.Lat-a.Lat)/(b.Lat-a.Lat)+a.Lon {
			inside = !inside
		}
	}
	return inside
}

// geometrySequencesFromGeoJSON извлекает последовательности точек из геометрии GeoJSON
func geometrySequencesFromGeoJSON(geom geoJSONGeometry) ([][]geometry.LatLon, error) {
	switch strings.ToLower(geom.Type) {
	case "linestring":
		points, err := decodeCoordinateSequence(geom.Coordinates)
		if err != nil {
			return nil, fmt.Errorf("ошибка парсинга координат LineString: %w", err)
		}
		return [][]geometry.LatLon{points}, nil
	case "multilinestring":
		var raw []json.RawMessage
		if err := json.Unmarshal(geom.Coordinates, &raw); err != nil {
			return nil, fmt.Errorf("ошибка парсинга координат MultiLineString: %w", err)
		}

		sequences := make([][]geometry.LatLon, 0, len(raw))
		for _, item := range raw {
			points, err := decodeCoordinateSequence(item)
			if err != nil {
				return nil, fmt.Errorf("ошибка парсинга пути MultiLineString: %w", err)
			}
			sequences = append(sequences, points)
		}
		return sequences, nil
	case "polygon":
		var raw []json.RawMessage
		if err := json.Unmarshal(geom.Coordinates, &raw); err != nil {
			return nil, fmt.Errorf("ошибка парсинга координат Polygon: %w", err)
		}

		sequences := make([][]geometry.LatLon, 0, len(raw))
		for _, ring := range raw {
			points, err := decodeCoordinateSequence(ring)
			if err != nil {
				return nil, fmt.Errorf("ошибка парсинга кольца Polygon: %w", err)
			}
			sequences = append(sequences, points)
		}
		return sequences, nil
	case "multipolygon":
		var polygons []json.RawMessage
		if err := json.Unmarshal(geom.Coordinates, &polygons); err != nil {
			return nil, fmt.Errorf("ошибка парсинга координат MultiPolygon: %w", err)
		}

		sequences := make([][]geometry.LatLon, 0, len(polygons))
		for _, polygon := range polygons {
			var rings []json.RawMessage
			if err := json.Unmarshal(polygon, &rings); err != nil {
				return nil, fmt.Errorf("ошибка парсинга колец MultiPolygon: %w", err)
			}
			for _, ring := range rings {
				points, err := decodeCoordinateSequence(ring)
				if err != nil {
					return nil, fmt.Errorf("ошибка парсинга кольца MultiPolygon: %w", err)
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
		return nil, fmt.Errorf("неподдерживаемый тип геометрии GeoJSON: %q", geom.Type)
	}
}

// decodeCoordinateSequence декодирует последовательность координат
func decodeCoordinateSequence(data json.RawMessage) ([]geometry.LatLon, error) {
	var raw [][]float64
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	points := make([]geometry.LatLon, 0, len(raw))
	for idx, coordinate := range raw {
		if len(coordinate) < 2 {
			return nil, fmt.Errorf("координата в индексе %d должна содержать долготу/широту", idx)
		}
		points = append(points, geometry.LatLon{
			Lat: coordinate[1],
			Lon: coordinate[0],
		})
	}

	return points, nil
}

// filterGeoJSONSequences фильтрует последовательности по границам
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

// bestSequence выбирает лучшую последовательность (самую длинную)
func bestSequence(sequences [][]geometry.LatLon) []geometry.LatLon {
	if len(sequences) == 0 {
		return nil
	}

	// Находим самую длинную отдельную линию для моделирования
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

// AllSequences возвращает все последовательности, отсортированные по длине (по убыванию)
func AllSequences(sequences [][]geometry.LatLon) [][]geometry.LatLon {
	if len(sequences) == 0 {
		return nil
	}

	// Сортировка по длине (по убыванию)
	sorted := make([][]geometry.LatLon, len(sequences))
	copy(sorted, sequences)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if geometry.PolylineLength(sorted[j]) > geometry.PolylineLength(sorted[i]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// buildDefaultCoastlineGeoJSONURL строит URL GeoJSON береговой линии по умолчанию
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

// defaultCoastlineCachePath возвращает путь к кэшу по умолчанию для указанного URL
func defaultCoastlineCachePath(remoteURL string) string {
	if remoteURL == DefaultCoastlineGeoJSONURL {
		return filepath.Join(DefaultCoastlineCacheDir, "black-sea.geojson")
	}

	sum := sha1.Sum([]byte(remoteURL))
	return filepath.Join(DefaultCoastlineCacheDir, fmt.Sprintf("coastline-%x.geojson", sum[:6]))
}

// cachedSourceLabel возвращает метку для кэшированного источника
func cachedSourceLabel(cachePath, remoteURL string) string {
	return fmt.Sprintf("%s (кешированная копия %s)", cachePath, remoteURL)
}

// Типы для инспекции источника данных

// InspectOptions параметры инспекции источника
type InspectOptions struct {
	LocalPath    string
	RemoteURL    string
	CachePath    string
	SnapshotPath string
	Refresh      bool
	HTTPClient   *http.Client
}

// SourceMetadata метаданные источника данных
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

// SourceInspection результат инспекции источника
type SourceInspection struct {
	Source       string
	DatasetName  string
	CachePath    string
	SnapshotPath string
	Metadata     SourceMetadata
	LoadWarnings []string
}

// resolvedSourcePayload представляет разрешённый источник данных
type resolvedSourcePayload struct {
	Payload      []byte
	Source       string
	CachePath    string
	LoadWarnings []string
}

// sourceFeatureCollection представляет коллекцию объектов источника
type sourceFeatureCollection struct {
	Type       string                 `json:"type"`
	Properties map[string]any         `json:"properties"`
	Features   []sourceGeoJSONFeature `json:"features"`
	Geometry   *geoJSONGeometry       `json:"geometry"`
}

// sourceGeoJSONFeature представляет объект источника
type sourceGeoJSONFeature struct {
	Type       string           `json:"type"`
	Properties map[string]any   `json:"properties"`
	Geometry   *geoJSONGeometry `json:"geometry"`
}

// InspectSource инспектирует источник данных береговой линии
func InspectSource(options InspectOptions) (SourceInspection, error) {
	localPath := strings.TrimSpace(options.LocalPath)
	remoteURL := strings.TrimSpace(options.RemoteURL)
	cachePath := strings.TrimSpace(options.CachePath)
	if localPath == "" && remoteURL != "" && cachePath == "" {
		cachePath = defaultCoastlineCachePath(remoteURL)
	}

	payload, err := resolveSourcePayload(localPath, remoteURL, cachePath, options.Refresh, options.HTTPClient)
	if err != nil {
		return SourceInspection{}, err
	}

	metadata, err := inspectSourceMetadata(payload.Payload)
	if err != nil {
		return SourceInspection{}, fmt.Errorf("ошибка инспекции источника береговой линии %q: %w", payload.Source, err)
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

// resolveSourcePayload разрешает источник данных (локальный, удалённый или кэш)
func resolveSourcePayload(localPath, remoteURL, cachePath string, refresh bool, client *http.Client) (resolvedSourcePayload, error) {
	localPath = strings.TrimSpace(localPath)
	remoteURL = strings.TrimSpace(remoteURL)
	if localPath != "" {
		payload, err := os.ReadFile(localPath)
		if err != nil {
			return resolvedSourcePayload{}, fmt.Errorf("ошибка чтения JSON береговой линии %q: %w", localPath, err)
		}
		return resolvedSourcePayload{
			Payload: payload,
			Source:  localPath,
		}, nil
	}

	if remoteURL == "" {
		localPath = DefaultCoastlineJSONPath
		payload, err := os.ReadFile(localPath)
		if err != nil {
			return resolvedSourcePayload{}, fmt.Errorf("ошибка чтения JSON береговой линии %q: %w", localPath, err)
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
			result.LoadWarnings = append(result.LoadWarnings, fmt.Sprintf("невозможно обновить кэш береговой линии %q: %v", cachePath, cacheErr))
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
				fmt.Sprintf("удалённый источник %q недоступен, используется кэшированный GeoJSON %q: %v", remoteURL, cachePath, remoteErr),
			},
		}, nil
	}

	return resolvedSourcePayload{}, fmt.Errorf("ошибка загрузки из удалённого источника %q: %v; ошибка загрузки из кэша %q: %w", remoteURL, remoteErr, cachePath, cacheErr)
}

// inspectSourceMetadata инспектирует метаданные источника
func inspectSourceMetadata(data []byte) (SourceMetadata, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return SourceMetadata{}, fmt.Errorf("пустые данные береговой линии")
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
		return SourceMetadata{}, fmt.Errorf("неподдерживаемый формат данных береговой линии")
	}

	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return SourceMetadata{}, fmt.Errorf("ошибка парсинга JSON-оболочки: %w", err)
	}
	meta.RootType = envelope.Type

	var root sourceFeatureCollection
	if err := json.Unmarshal(trimmed, &root); err != nil {
		return SourceMetadata{}, fmt.Errorf("ошибка парсинга GeoJSON-оболочки: %w", err)
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

// boundsFromPoints вычисляет границы по множеству точек
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

// geometryTypesList преобразует множество типов геометрии в отсортированный список
func geometryTypesList(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}

	list := make([]string, 0, len(set))
	for name := range set {
		list = append(list, name)
	}
	// Используем встроенную сортировку вместо "sort"
	for i := 0; i < len(list)-1; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j] < list[i] {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list
}

// propertyString извлекает строковое значение свойства
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
			return fmt.Sprintf("%.0f", typed)
		case int:
			return fmt.Sprintf("%d", typed)
		default:
			return strings.TrimSpace(fmt.Sprint(typed))
		}
	}

	return ""
}

// datasetNameFromMetadata получает имя набора данных из метаданных
func datasetNameFromMetadata(meta SourceMetadata, localPath, remoteURL string) string {
	if strings.TrimSpace(meta.Name) != "" {
		return strings.TrimSpace(meta.Name)
	}
	if strings.TrimSpace(localPath) != "" {
		return filepath.Base(localPath)
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
	return "coastline"
}

// resolveSnapshotPath разрешает путь к снимку
func resolveSnapshotPath(output string, meta SourceMetadata, datasetName string) (string, error) {
	filename := snapshotFilename(meta, datasetName, time.Now().UTC())

	if strings.TrimSpace(output) == "" {
		if err := os.MkdirAll(DefaultCoastlineSnapshotDir, dirPermissions); err != nil {
			return "", fmt.Errorf("ошибка создания директории снимков %q: %w", DefaultCoastlineSnapshotDir, err)
		}
		return filepath.Abs(filepath.Join(DefaultCoastlineSnapshotDir, filename))
	}

	lower := strings.ToLower(output)
	if strings.HasSuffix(lower, ".geojson") || strings.HasSuffix(lower, ".json") {
		dir := filepath.Dir(output)
		if dir != "." {
			if err := os.MkdirAll(dir, dirPermissions); err != nil {
				return "", fmt.Errorf("ошибка создания директории %q: %w", dir, err)
			}
		}
		return filepath.Abs(output)
	}

	if err := os.MkdirAll(output, dirPermissions); err != nil {
		return "", fmt.Errorf("ошибка создания директории %q: %w", output, err)
	}

	return filepath.Abs(filepath.Join(output, filename))
}

// snapshotFilename генерирует имя файла снимка
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

// slugify преобразует строку в slug
func slugify(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}

	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
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

// writeSnapshot записывает снимок данных
func writeSnapshot(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPermissions); err != nil {
		return fmt.Errorf("ошибка создания директории для снимка %q: %w", path, err)
	}
	if err := os.WriteFile(path, data, filePermissions); err != nil {
		return fmt.Errorf("ошибка записи снимка %q: %w", path, err)
	}
	return nil
}
