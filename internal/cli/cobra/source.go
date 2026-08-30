package cobra

import (
	"fmt"

	"coastal-geometry/internal/domain/coastline"

	"github.com/spf13/cobra"
)

var (
	sourceInput   string
	sourceURL     string
	sourceRefresh bool
	sourceOutput  string
)

var sourceCmd = &cobra.Command{
	Use:   "source",
	Short: "Проверка и создание моментального снимка источника данных о береговой линии",
	Long: `Проверьте источник данных о береговой линии, покажите метаданные и, при необходимости,
сохраните снимок для воспроизводимых прогонов.`,
	RunE: runSource,
}

func init() {
	rootCmd.AddCommand(sourceCmd)

	sourceCmd.Flags().StringVar(&sourceInput, "input", "", "путь к локальному JSON/GeoJSON береговой линии")
	sourceCmd.Flags().StringVar(&sourceURL, "source-url", "", "явно включить удалённый GeoJSON-источник")
	sourceCmd.Flags().BoolVar(&sourceRefresh, "refresh", false, "force refresh of remote cache")
	sourceCmd.Flags().StringVar(&sourceOutput, "output", "", "snapshot file or directory (default: ./data/snapshots)")
}

func runSource(cmd *cobra.Command, args []string) error {
	inspection, err := coastline.InspectSource(coastline.InspectOptions{
		LocalPath:    sourceInput,
		RemoteURL:    sourceURL,
		SnapshotPath: sourceOutput,
		Refresh:      sourceRefresh,
	})
	if err != nil {
		return err
	}

	printSourceInspection(inspection)
	return nil
}

func printSourceInspection(inspection coastline.SourceInspection) {
	meta := inspection.Metadata

	fmt.Println("")
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
	fmt.Println("        МЕТАДАННЫЕ ИСТОЧНИКА БЕРЕГОВОЙ ЛИНИИ")
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
	fmt.Println("")
	fmt.Printf("Набор данных:                           %s\n", valueOrDash(inspection.DatasetName))
	fmt.Printf("Формат:                                %s\n", valueOrDash(meta.Format))
	fmt.Printf("Корневой тип:                          %s\n", valueOrDash(meta.RootType))
	fmt.Printf("Количество features:                   %d\n", meta.FeatureCount)
	fmt.Printf("Типы геометрии:                        %s\n", valueOrDash(fmt.Sprint(meta.GeometryTypes)))
	fmt.Printf("Точек в извлечённой береговой линии:   %d\n", meta.CoastlinePointCount)
	fmt.Printf("Размер payload:                        %d байт\n", meta.PayloadBytes)
	fmt.Printf("Имя набора:                            %s\n", valueOrDash(meta.Name))
	fmt.Printf("Marine Regions ID:                     %s\n", valueOrDash(meta.RegionID))
	if !meta.Bounds.IsZero() {
		fmt.Printf("Границы:                                lat %.4f..%.4f, lon %.4f..%.4f\n", meta.Bounds.MinLat, meta.Bounds.MaxLat, meta.Bounds.MinLon, meta.Bounds.MaxLon)
	}
	if inspection.CachePath != "" {
		fmt.Printf("Кэш:                                   %s\n", inspection.CachePath)
	}
	fmt.Printf("Snapshot:                              %s\n", inspection.SnapshotPath)
	fmt.Println("════════════════════════════════════════════════════════════════════════════════")
}

func valueOrDash(v interface{}) string {
	if s, ok := v.(string); ok {
		if s == "" {
			return "—"
		}
		return s
	}
	return fmt.Sprintf("%v", v)
}
