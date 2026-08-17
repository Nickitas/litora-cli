package cobra

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"coastal-geometry/internal/domain/geometry"
)

const (
	blackSeaSochiCoastlinePath = "data/examples/sochi-local-segment.geojson"
	blackSeaSochiDataDir       = "data/black-sea/sochi"
)

type blackSeaSochiDataPaths struct {
	Coastline  string
	Bathymetry string
	Waves      string
	WaveSource string
}

// prepareBlackSeaSochiData загружает открытые данные для стартового сценария
// Сочи и сохраняет их вместе с метаданными происхождения. Береговая линия
// поставляется с программой, а прогноз волн и глубины обновляются при запуске.
func prepareBlackSeaSochiData() (blackSeaSochiDataPaths, error) {
	if _, err := os.Stat(blackSeaSochiCoastlinePath); err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("демонстрационный сегмент Сочи отсутствует: %w", err)
	}
	if err := os.MkdirAll(blackSeaSochiDataDir, 0o755); err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("создание каталога данных Сочи: %w", err)
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

	manifest := map[string]string{
		"generated_at":        generatedAt,
		"coastline":           blackSeaSochiCoastlinePath,
		"coastline_source":    "OpenStreetMap, way 59530506, выгрузка 2026-08-17, ODbL",
		"wave_source":         waveSource,
		"wave_endpoint":       waveURL,
		"bathymetry_source":   "EMODnet Bathymetry REST /depth_sample; reference GEBCO",
		"bathymetry_endpoint": "https://rest.emodnet-bathymetry.eu/depth_sample",
	}
	if err := writeJSONFile(filepath.Join(blackSeaSochiDataDir, "manifest.json"), manifest); err != nil {
		return blackSeaSochiDataPaths{}, fmt.Errorf("сохранение манифеста: %w", err)
	}
	return blackSeaSochiDataPaths{Coastline: blackSeaSochiCoastlinePath, Bathymetry: bathymetryPath, Waves: wavePath, WaveSource: waveSource}, nil
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
