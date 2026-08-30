package cobra

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"
)

const (
	blackSeaSochiCoastlinePath = "data/examples/sochi-local-segment.geojson"
	blackSeaSochiDataDir       = "data/black-sea/sochi"
)

type blackSeaSochiDataPaths struct {
	Coastline        string
	Bathymetry       string
	Waves            string
	Structures       string
	WaveSource       string
	StructureWarning string
}

// prepareBlackSeaSochiData подготавливает открытые данные для стартового
// сценария Сочи. Сначала используются проверенные локальные файлы, чтобы
// обычный запуск не зависел от сети. Параметр refresh запрашивает свежие
// волны, глубины и инвентарь сооружений и заменяет кэш только после проверки.
func prepareBlackSeaSochiData(refresh bool) (blackSeaSochiDataPaths, error) {
	if _, err := os.Stat(blackSeaSochiCoastlinePath); err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("демонстрационный сегмент Сочи отсутствует: %w", err)
	}
	if err := os.MkdirAll(blackSeaSochiDataDir, 0o755); err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("создание каталога данных Сочи: %w", err)
	}
	if !refresh {
		if cached, ok := loadCachedBlackSeaSochiData(); ok {
			return cached, nil
		}
	}

	client := &http.Client{Timeout: 45 * time.Second}
	waveURL := "https://marine-api.open-meteo.com/v1/marine?latitude=43.62&longitude=39.68&hourly=wave_height,wave_period,wave_direction&forecast_days=7&timezone=UTC"
	waveData, err := downloadBlackSeaData(client, waveURL)
	if err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("загрузка прогноза Open-Meteo Marine: %w", err)
	}

	generatedAt := time.Now().UTC().Format(time.RFC3339)
	waveSource := "Open-Meteo Marine, точка 43.625N 39.70834E, выгружено " + generatedAt
	wavePath := filepath.Join(blackSeaSochiDataDir, "waves-open-meteo.json")
	if err := os.WriteFile(wavePath, waveData, 0o644); err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("сохранение волн: %w", err)
	}
	if _, err := geometry.LoadWaveClimate(wavePath, waveSource); err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("проверка загруженного волнового ряда: %w", err)
	}

	bathymetry, err := downloadSochiBathymetry(client)
	if err != nil {
		return blackSeaSochiDataPaths{}, err
	}
	bathymetryPath := filepath.Join(blackSeaSochiDataDir, "bathymetry-emodnet.json")
	if err := writeJSONFile(bathymetryPath, bathymetry); err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("сохранение батиметрии: %w", err)
	}
	if _, err := geometry.LoadBathymetryFromJSON(mustMarshalJSON(bathymetry), geometry.BathymetryLoadOptions{Resolution: 0.005}); err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("проверка загруженной батиметрии: %w", err)
	}

	coast, err := coastline.Load(coastline.LoadOptions{LocalPath: blackSeaSochiCoastlinePath})
	if err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("загрузка сегмента Сочи для привязки сооружений: %w", err)
	}
	structureClient := &http.Client{Timeout: 12 * time.Second}
	structures, rawStructures, inventory, structureWarning := downloadSochiStructures(structureClient, coast.Points, generatedAt)
	structuresPath := filepath.Join(blackSeaSochiDataDir, "structures-osm.json")
	if structureWarning != "" {
		// Кэш сохраняет ранее полученные открытые данные при временных лимитах
		// Overpass; пустой ответ не подменяет подтверждённый инвентарь.
		if cached, err := geometry.LoadLongshoreStructures(structuresPath); err == nil && len(cached) > 0 {
			structures = cached
			inventory = fmt.Sprintf("кэш OSM: применено %d сооружений", len(cached))
			structureWarning += "; использован ранее загруженный кэш сооружений"
		}
	}
	if structureWarning == "" || len(structures) == 0 {
		if err := writeJSONFile(structuresPath, structures); err != nil {
			return blackSeaSochiDataPaths{}, fmt.Errorf("сохранение сооружений: %w", err)
		}
	}
	if len(rawStructures) > 0 {
		if err := os.WriteFile(filepath.Join(blackSeaSochiDataDir, "structures-osm-raw.json"), rawStructures, 0o644); err != nil {
			return blackSeaSochiDataPaths{}, fmt.Errorf("сохранение исходного ответа OSM: %w", err)
		}
	}

	manifest := map[string]string{
		"generated_at":         generatedAt,
		"coastline":            blackSeaSochiCoastlinePath,
		"coastline_source":     "OpenStreetMap, way 59530506, выгрузка 2026-08-17, ODbL",
		"wave_source":          waveSource,
		"wave_endpoint":        waveURL,
		"bathymetry_source":    "EMODnet Bathymetry REST /depth_sample; reference GEBCO",
		"bathymetry_endpoint":  "https://rest.emodnet-bathymetry.eu/depth_sample",
		"structures_source":    "OpenStreetMap / Overpass: только берегопримыкающие man_made=groyne получают полный перехват",
		"structures_inventory": inventory,
		"structures_warning":   structureWarning,
	}
	if err := writeJSONFile(filepath.Join(blackSeaSochiDataDir, "manifest.json"), manifest); err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("сохранение манифеста: %w", err)
	}
	return blackSeaSochiDataPaths{Coastline: blackSeaSochiCoastlinePath, Bathymetry: bathymetryPath, Waves: wavePath, Structures: structuresPath, WaveSource: waveSource, StructureWarning: structureWarning}, nil
}

// loadCachedBlackSeaSochiData возвращает только полный и валидный локальный
// набор. Благодаря проверке файлов ошибочный или неполный кэш не может
// незаметно попасть в инженерный расчёт.
func loadCachedBlackSeaSochiData() (blackSeaSochiDataPaths, bool) {
	wavesPath := filepath.Join(blackSeaSochiDataDir, "waves-open-meteo.json")
	bathymetryPath := filepath.Join(blackSeaSochiDataDir, "bathymetry-emodnet.json")
	structuresPath := filepath.Join(blackSeaSochiDataDir, "structures-osm.json")
	manifestPath := filepath.Join(blackSeaSochiDataDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return blackSeaSochiDataPaths{}, false
	}
	manifest := make(map[string]string)
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return blackSeaSochiDataPaths{}, false
	}
	waveSource := strings.TrimSpace(manifest["wave_source"])
	if waveSource == "" {
		return blackSeaSochiDataPaths{}, false
	}
	if _, err := geometry.LoadWaveClimate(wavesPath, waveSource); err != nil {
		return blackSeaSochiDataPaths{}, false
	}
	bathymetryData, err := os.ReadFile(bathymetryPath)
	if err != nil {
		return blackSeaSochiDataPaths{}, false
	}
	if _, err := geometry.LoadBathymetryFromJSON(bathymetryData, geometry.BathymetryLoadOptions{Resolution: 0.005}); err != nil {
		return blackSeaSochiDataPaths{}, false
	}
	structures, err := geometry.LoadLongshoreStructures(structuresPath)
	structureWarning := ""
	if err != nil {
		structuresPath = ""
		structureWarning = "локальный инвентарь сооружений отсутствует; расчёт выполнен без сооружений"
	} else if len(structures) == 0 {
		// Историческая ошибка обновления не означает ошибку текущего запуска:
		// сохранённый пустой инвентарь является валидным консервативным входом.
		structureWarning = "локальный инвентарь не содержит применимых сооружений; для новой выгрузки используйте --refresh"
	}
	return blackSeaSochiDataPaths{
		Coastline:        blackSeaSochiCoastlinePath,
		Bathymetry:       bathymetryPath,
		Waves:            wavesPath,
		Structures:       structuresPath,
		WaveSource:       waveSource,
		StructureWarning: structureWarning,
	}, true
}

// downloadBlackSeaData получает данные открытого сервиса и проверяет код ответа.
func downloadBlackSeaData(client *http.Client, endpoint string) ([]byte, error) {
	response, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер вернул %s", response.Status)
	}
	return io.ReadAll(response.Body)
}

// downloadSochiBathymetry получает реальные морские глубины, расположенные
// с морской стороны готового участка. Грубое разрежение явно отмечается в
// результате расчёта расстоянием до исходной точки батиметрии.
func downloadSochiBathymetry(client *http.Client) ([]geometry.BathymetryPoint, error) {
	coordinates := []struct{ lat, lon float64 }{
		{43.635, 39.660}, {43.630, 39.670}, {43.625, 39.680},
		{43.620, 39.690}, {43.610, 39.700}, {43.600, 39.705},
		{43.590, 39.710},
	}
	points := make([]geometry.BathymetryPoint, 0, len(coordinates))
	for _, coordinate := range coordinates {
		query := url.Values{}
		query.Set("geom", fmt.Sprintf("POINT(%.3f %.3f)", coordinate.lon, coordinate.lat))
		data, err := downloadBlackSeaData(client, "https://rest.emodnet-bathymetry.eu/depth_sample?"+query.Encode())
		if err != nil {
			return nil, fmt.Errorf("загрузка глубины %.3f, %.3f: %w", coordinate.lat, coordinate.lon, err)
		}
		var sample struct {
			Smoothed float64 `json:"smoothed"`
		}
		if err := json.Unmarshal(data, &sample); err != nil {
			return nil, fmt.Errorf("разбор глубины %.3f, %.3f: %w", coordinate.lat, coordinate.lon, err)
		}
		if sample.Smoothed >= 0 {
			return nil, fmt.Errorf("точка %.3f, %.3f не является морской", coordinate.lat, coordinate.lon)
		}
		points = append(points, geometry.BathymetryPoint{Lat: coordinate.lat, Lon: coordinate.lon, Depth: sample.Smoothed})
	}
	return points, nil
}

// downloadSochiStructures получает геометрию бун и волноломов из OSM. Только
// буны, конец которых находится не далее 200 м от входной линии берега,
// превращаются в непроницаемую грань CERC. Для остальных объектов открытая
// геометрия не даёт достоверного коэффициента пропуска, поэтому они не влияют
// на расчёт и учитываются только в инвентаре.
func downloadSochiStructures(client *http.Client, coast []geometry.LatLon, generatedAt string) ([]geometry.LongshoreStructure, []byte, string, string) {
	query := `[out:json][timeout:60];(way["man_made"="groyne"](43.54,39.64,43.67,39.85);way["man_made"="breakwater"](43.54,39.64,43.67,39.85););out tags geom;`
	endpoints := []string{
		"https://overpass.kumi.systems/api/interpreter",
		"https://overpass.private.coffee/api/interpreter",
	}
	var lastError error
	for _, endpoint := range endpoints {
		values := url.Values{}
		values.Set("data", query)
		raw, err := downloadBlackSeaData(client, endpoint+"?"+values.Encode())
		if err != nil {
			lastError = err
			continue
		}
		structures, inventory, err := mapOSMStructuresToSochi(raw, coast, generatedAt)
		if err != nil {
			lastError = err
			continue
		}
		return structures, raw, inventory, ""
	}
	if lastError == nil {
		lastError = fmt.Errorf("нет ответа от серверов Overpass")
	}
	return []geometry.LongshoreStructure{}, nil, "данные OSM недоступны", "не удалось обновить инвентарь сооружений: " + lastError.Error() + "; расчёт выполнен без сооружений"
}

func mapOSMStructuresToSochi(raw []byte, coast []geometry.LatLon, generatedAt string) ([]geometry.LongshoreStructure, string, error) {
	var response struct {
		Elements []struct {
			ID       int64             `json:"id"`
			Tags     map[string]string `json:"tags"`
			Geometry []struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			} `json:"geometry"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, "", err
	}
	structures := make([]geometry.LongshoreStructure, 0)
	usedFaces := make(map[int]struct{})
	groyneCount, breakwaterCount, skipped := 0, 0, 0
	for _, element := range response.Elements {
		kind := element.Tags["man_made"]
		if kind == "groyne" {
			groyneCount++
		} else if kind == "breakwater" {
			breakwaterCount++
		}
		if kind != "groyne" || len(element.Geometry) < 2 {
			skipped++
			continue
		}
		first := element.Geometry[0]
		last := element.Geometry[len(element.Geometry)-1]
		index, distance := nearestSochiCoastPoint(coast, []geometry.LatLon{{Lat: first.Lat, Lon: first.Lon}, {Lat: last.Lat, Lon: last.Lon}})
		if distance > 200 || len(coast) < 2 {
			skipped++
			continue
		}
		if index == len(coast)-1 {
			index--
		}
		if _, exists := usedFaces[index]; exists {
			skipped++
			continue
		}
		usedFaces[index] = struct{}{}
		description := strings.TrimSpace(element.Tags["name"])
		if description == "" {
			description = "берегопримыкающая буна OpenStreetMap"
		}
		structures = append(structures, geometry.LongshoreStructure{
			LeftPointIndex:          index,
			TransmissionCoefficient: 0,
			Kind:                    "буна",
			Description:             description,
			DataSource:              fmt.Sprintf("OpenStreetMap way %d, выгружено %s", element.ID, generatedAt),
		})
	}
	inventory := fmt.Sprintf("OSM: бун %d, волноломов %d, применено %d, пропущено %d", groyneCount, breakwaterCount, len(structures), skipped)
	return structures, inventory, nil
}

func nearestSochiCoastPoint(coast []geometry.LatLon, candidates []geometry.LatLon) (int, float64) {
	index, distance := 0, math.Inf(1)
	for coastIndex, coastPoint := range coast {
		for _, candidate := range candidates {
			current := geometry.Haversine(coastPoint, candidate) * 1000
			if current < distance {
				index, distance = coastIndex, current
			}
		}
	}
	return index, distance
}

// writeJSONFile сохраняет набор входных данных в читаемом JSON-формате.
func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// mustMarshalJSON используется только для проверки только что собранного
// массива числовых точек, который не содержит значений, не кодируемых JSON.
func mustMarshalJSON(value any) []byte {
	data, _ := json.Marshal(value)
	return data
}
