package cobra

import (
	"fmt"
	"os"
	"text/tabwriter"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/fractal"
	"coastal-geometry/internal/domain/geometry"

	"github.com/spf13/cobra"
)

var (
	dimInput     string
	dimSourceURL string
	dimRefresh   bool
	dimOutput    string
)

var dimensionCmd = &cobra.Command{
	Use:   "dimension",
	Short: "Оценить фрактальную размерность наблюдаемой береговой линии",
	Long: `Оценить фрактальную размерность загруженной наблюдаемой береговой линии
методом box-counting. Команда не изменяет геометрию генератором Коха и не
сравнивает результат с теоретической размерностью кривой Коха.

Оценка характеризует масштабное поведение конкретного набора данных и зависит
от его разрешения и выбранного регрессионного окна. Синтетический учебный
сценарий доступен отдельно: lito koch-demo.`,
	RunE: runDimension,
}

func init() {
	rootCmd.AddCommand(dimensionCmd)

	dimensionCmd.Flags().StringVar(&dimInput, "input", "", "путь к локальному JSON/GeoJSON береговой линии")
	dimensionCmd.Flags().StringVar(&dimSourceURL, "source-url", "", "явно включить удалённый GeoJSON-источник")
	dimensionCmd.Flags().BoolVar(&dimRefresh, "refresh", false, "принудительное обновление удалённого кэша")
	dimensionCmd.Flags().StringVar(&dimOutput, "output", "", "каталог для вывода (по умолчанию: ./output)")
}

func runDimension(_ *cobra.Command, _ []string) error {
	result, err := coastline.Load(coastline.LoadOptions{
		LocalPath: dimInput,
		RemoteURL: dimSourceURL,
		Refresh:   dimRefresh,
	})
	if err != nil {
		return fmt.Errorf("загрузка береговой линии: %w", err)
	}

	fmt.Printf("Загружено: %s (%d точек)\n", result.Source, len(result.Points))
	analysis := fractal.AnalyzeBoxCounting(result.Points)
	printObservedDimension(result.Points, analysis)

	outputMgr := cli.NewOutputPathManager(dimOutput)
	if err := cli.WriteDimensionSVG(result.Points, outputMgr, result.DatasetName, result.Source, result.Validation); err != nil {
		fmt.Printf("Предупреждение: не удалось создать SVG: %v\n", err)
	}

	return nil
}

// printObservedDimension выводит оценку box-counting для неизменённой
// наблюдаемой геометрии в табличном формате.
func printObservedDimension(points []geometry.LatLon, analysis fractal.BoxCountingAnalysis) {
	dimensionValue := "н/д"
	rSquared := "н/д"
	spread := "н/д"
	stable := "нет"
	if analysis.Valid {
		dimensionValue = fmt.Sprintf("%.5f", analysis.Dimension)
		rSquared = fmt.Sprintf("%.4f", analysis.RegressionRSquared)
		spread = fmt.Sprintf("%.4f", analysis.StabilitySpread)
		if analysis.StableAcrossScales {
			stable = "да"
		}
	}

	fmt.Println("\nФРАКТАЛЬНАЯ РАЗМЕРНОСТЬ НАБЛЮДАЕМОЙ ЛИНИИ (BOX-COUNTING)")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Данные\tТочки\tДлина, км\tD\tМасштабы\tR²\tРазброс\tУстойчива")
	fmt.Fprintln(w, "------\t-----\t---------\t-\t---------\t--\t-------\t---------")
	fmt.Fprintf(w, "Наблюдение\t%d\t%.0f\t%s\t%d\t%s\t%s\t%s\n",
		len(points), geometry.PolylineLength(points), dimensionValue, len(analysis.Samples), rSquared, spread, stable)
	w.Flush()
	fmt.Println("\nИнтерпретация: оценка относится только к загруженным наблюдениям и доступному диапазону масштабов; она не доказывает соответствие береговой линии кривой Коха.")
}
