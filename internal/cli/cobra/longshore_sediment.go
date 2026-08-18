package cobra

import (
	"fmt"

	"coastal-geometry/internal/domain/geometry"
)

// loadLongshoreSedimentSources читает профиль только когда пользователь явно
// передал его: отсутствие файла означает нулевые внешние источники, а не
// вымышленное питание берега.
func loadLongshoreSedimentSources(path string) ([]geometry.LongshoreSedimentSource, error) {
	if path == "" {
		return nil, nil
	}
	sources, err := geometry.LoadLongshoreSedimentSources(path)
	if err != nil {
		return nil, fmt.Errorf("загрузка источников наносов: %w", err)
	}
	return sources, nil
}
