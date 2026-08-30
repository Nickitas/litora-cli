package cli

import (
	"coastal-geometry/internal/domain/geometry"
	"encoding/json"
	"fmt"
	"os"
)

// WriteLongshoreModelJSON сохраняет полный пространственный баланс CERC в
// output/erosion. Файл содержит исходный волновой ряд, снимки и объёмы по
// ячейкам, поэтому пригоден для независимой проверки расчёта.
func WriteLongshoreModelJSON(model geometry.LongshoreModelResult, output *OutputPathManager) error {
	if output == nil {
		return fmt.Errorf("не задан менеджер каталога вывода")
	}
	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return fmt.Errorf("сериализация баланса CERC: %w", err)
	}
	path := output.ErosionPath("longshore-cerc.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("запись %q: %w", path, err)
	}
	return nil
}
