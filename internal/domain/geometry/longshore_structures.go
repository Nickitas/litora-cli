package geometry

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// LongshoreStructure описывает сооружение на грани между двумя соседними
// ячейками. Коэффициент пропуска равен доле вдольберегового потока, проходящей
// через сооружение: 0 — полное перекрытие, 1 — сооружение не влияет на поток.
// Модель не заменяет этим полем дифракцию, отражение или 2D-обтекание.
type LongshoreStructure struct {
	LeftPointIndex          int     `json:"left_point_index"`
	TransmissionCoefficient float64 `json:"transmission_coefficient"`
	Kind                    string  `json:"kind"`
	Description             string  `json:"description"`
	DataSource              string  `json:"data_source"`
}

// LoadLongshoreStructures загружает профиль сооружений из JSON-массива.
// Происхождение обязательно: значение пропуска должно быть связано с
// обследованием, проектом или отдельным обоснованием.
func LoadLongshoreStructures(path string) ([]LongshoreStructure, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение сооружений %q: %w", path, err)
	}
	var structures []LongshoreStructure
	if err := json.Unmarshal(data, &structures); err != nil {
		return nil, fmt.Errorf("разбор сооружений %q: %w", path, err)
	}
	for i, structure := range structures {
		if structure.LeftPointIndex < 0 {
			return nil, fmt.Errorf("сооружение %d: left_point_index должен быть неотрицательным", i)
		}
		if math.IsNaN(structure.TransmissionCoefficient) || math.IsInf(structure.TransmissionCoefficient, 0) || structure.TransmissionCoefficient < 0 || structure.TransmissionCoefficient > 1 {
			return nil, fmt.Errorf("сооружение %d: transmission_coefficient должен быть в диапазоне [0; 1]", i)
		}
		if strings.TrimSpace(structure.Kind) == "" || strings.TrimSpace(structure.DataSource) == "" {
			return nil, fmt.Errorf("сооружение %d: укажите kind и data_source", i)
		}
	}
	return structures, nil
}

func validateLongshoreStructures(structures []LongshoreStructure, pointCount int) error {
	used := make(map[int]struct{}, len(structures))
	for _, structure := range structures {
		if structure.LeftPointIndex >= pointCount-1 {
			return fmt.Errorf("сооружение на грани %d вне сегмента из %d точек", structure.LeftPointIndex, pointCount)
		}
		if _, exists := used[structure.LeftPointIndex]; exists {
			return fmt.Errorf("для грани %d задано более одного сооружения", structure.LeftPointIndex)
		}
		used[structure.LeftPointIndex] = struct{}{}
	}
	return nil
}

func longshoreFaceTransmission(structures []LongshoreStructure, pointCount int) []float64 {
	transmission := make([]float64, pointCount-1)
	for i := range transmission {
		transmission[i] = 1
	}
	for _, structure := range structures {
		transmission[structure.LeftPointIndex] = structure.TransmissionCoefficient
	}
	return transmission
}
