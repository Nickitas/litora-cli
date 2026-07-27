package cli

import (
	"fmt"
	"strings"
)

func runSourceCommand(app *App) error {
	if app == nil || app.SourceInspection == nil {
		return fmt.Errorf("source inspection is not available")
	}

	meta := app.SourceInspection.Metadata

	fmt.Println("")
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
	fmt.Println("        МЕТАДАННЫЕ ИСТОЧНИКА БЕРЕГОВОЙ ЛИНИИ")
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
	fmt.Println("")
	fmt.Printf("Набор данных:                           %s\n", valueOrDash(app.SourceInspection.DatasetName))
	fmt.Printf("Формат:                                %s\n", valueOrDash(meta.Format))
	fmt.Printf("Корневой тип:                          %s\n", valueOrDash(meta.RootType))
	fmt.Printf("Количество features:                   %d\n", meta.FeatureCount)
	fmt.Printf("Типы геометрии:                        %s\n", valueOrDash(strings.Join(meta.GeometryTypes, ", ")))
	fmt.Printf("Точек в извлечённой береговой линии:   %d\n", meta.CoastlinePointCount)
	fmt.Printf("Размер payload:                        %d байт\n", meta.PayloadBytes)
	fmt.Printf("Имя набора:                            %s\n", valueOrDash(meta.Name))
	fmt.Printf("Marine Regions ID:                     %s\n", valueOrDash(meta.RegionID))
	if !meta.Bounds.IsZero() {
		fmt.Printf("Bounds:                                lat %.4f..%.4f, lon %.4f..%.4f\n", meta.Bounds.MinLat, meta.Bounds.MaxLat, meta.Bounds.MinLon, meta.Bounds.MaxLon)
	}
	if app.SourceInspection.CachePath != "" {
		fmt.Printf("Кэш:                                   %s\n", app.SourceInspection.CachePath)
	}
	fmt.Printf("Snapshot:                              %s\n", app.SourceInspection.SnapshotPath)
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")

	return nil
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}
