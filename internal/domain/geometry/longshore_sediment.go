package geometry

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// LongshoreSedimentSource задаёт измеренный или проектный внешний приток
// наносов в ячейку one-line модели. Положительная скорость добавляет материал
// (питание пляжа, поступление из устья), отрицательная удаляет его (дноуглубление).
type LongshoreSedimentSource struct {
	PointIndex    int     `json:"point_index"`
	SourceRateM3S float64 `json:"source_rate_m3_s"`
	Description   string  `json:"description"`
	DataSource    string  `json:"data_source"`
}

// LoadLongshoreSedimentSources загружает пространственный профиль внешних
// источников и стоков наносов из JSON-массива. Происхождение обязательно,
// чтобы проектное питание не смешивалось с измеренным естественным стоком.
func LoadLongshoreSedimentSources(path string) ([]LongshoreSedimentSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение источников наносов %q: %w", path, err)
	}
	var sources []LongshoreSedimentSource
	if err := json.Unmarshal(data, &sources); err != nil {
		return nil, fmt.Errorf("разбор источников наносов %q: %w", path, err)
	}
	for i, source := range sources {
		if source.PointIndex < 0 {
			return nil, fmt.Errorf("источник наносов %d: point_index должен быть неотрицательным", i)
		}
		if math.IsNaN(source.SourceRateM3S) || math.IsInf(source.SourceRateM3S, 0) {
			return nil, fmt.Errorf("источник наносов %d: source_rate_m3_s равен NaN/Inf", i)
		}
		if strings.TrimSpace(source.DataSource) == "" {
			return nil, fmt.Errorf("источник наносов %d: укажите data_source", i)
		}
	}
	return sources, nil
}

func validateLongshoreSedimentSources(sources []LongshoreSedimentSource, pointCount int) error {
	used := make(map[int]struct{}, len(sources))
	for _, source := range sources {
		if source.PointIndex >= pointCount {
			return fmt.Errorf("источник наносов в ячейке %d вне сегмента из %d точек", source.PointIndex, pointCount)
		}
		if _, exists := used[source.PointIndex]; exists {
			return fmt.Errorf("для ячейки %d задано более одного источника наносов", source.PointIndex)
		}
		used[source.PointIndex] = struct{}{}
	}
	return nil
}

func longshoreSedimentSourceRates(sources []LongshoreSedimentSource, pointCount int) []float64 {
	rates := make([]float64, pointCount)
	for _, source := range sources {
		rates[source.PointIndex] = source.SourceRateM3S
	}
	return rates
}
