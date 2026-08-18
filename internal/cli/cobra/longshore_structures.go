package cobra

import (
	"fmt"

	"coastal-geometry/internal/domain/geometry"
)

// loadLongshoreStructures читает сооружения только по явному пути. Пустой
// флаг означает отсутствие данных о сооружениях, а не вымышленный барьер.
func loadLongshoreStructures(path string) ([]geometry.LongshoreStructure, error) {
	if path == "" {
		return nil, nil
	}
	structures, err := geometry.LoadLongshoreStructures(path)
	if err != nil {
		return nil, fmt.Errorf("загрузка сооружений: %w", err)
	}
	return structures, nil
}
