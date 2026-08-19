package cobra

import (
	"fmt"
	"math"
	"os"
	"text/tabwriter"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/fractal"
	"coastal-geometry/internal/domain/generators/koch"
	"coastal-geometry/internal/domain/geometry"

	"github.com/spf13/cobra"
)

var (
	kochDemoInput           string
	kochDemoSourceURL       string
	kochDemoRefresh         bool
	kochDemoOutput          string
	kochDemoIterations      int
	kochDemoSeed            int64
	kochDemoAngleJitter     float64
	kochDemoHeightJitter    float64
	kochDemoModelMaxPoints  int
	kochDemoDisableSimplify bool
)

var kochDemoCmd = &cobra.Command{
	Use:   "koch-demo",
	Short: "Показать учебную Кох-подобную генерацию",
	Long: `Учебный и демонстрационный сценарий: искусственно преобразовать базовую
полилинию органическим Кох-подобным генератором, показать рост геометрической
сложности и длины, а также проверить алгоритм box-counting.

Результаты относятся к синтетическим кривым и не являются научным выводом о
фрактальной размерности природной береговой линии. Для анализа наблюдений
используйте lito dimension.`,
	RunE: runKochDemo,
}

func init() {
	rootCmd.AddCommand(kochDemoCmd)

	kochDemoCmd.Flags().StringVar(&kochDemoInput, "input", "", "путь к локальному JSON/GeoJSON базовой линии")
	kochDemoCmd.Flags().StringVar(&kochDemoSourceURL, "source-url", "", "явно включить удалённый GeoJSON-источник")
	kochDemoCmd.Flags().BoolVar(&kochDemoRefresh, "refresh", false, "принудительное обновление удалённого кэша")
	kochDemoCmd.Flags().StringVar(&kochDemoOutput, "output", "", "каталог для вывода (по умолчанию: ./output)")
	kochDemoCmd.Flags().IntVar(&kochDemoIterations, "iterations", 5, "максимальное количество органических итераций Коха")
	kochDemoCmd.Flags().Int64Var(&kochDemoSeed, "seed", 42, "зерно генератора случайных чисел")
	kochDemoCmd.Flags().Float64Var(&kochDemoAngleJitter, "angle-jitter", 18, "максимальное случайное отклонение угла в градусах")
	kochDemoCmd.Flags().Float64Var(&kochDemoHeightJitter, "height-jitter", 0.25, "максимальное случайное отклонение высоты как отношение")
	kochDemoCmd.Flags().IntVar(&kochDemoModelMaxPoints, "model-max-points", 0, "максимальное количество точек базовой синтетической модели")
	kochDemoCmd.Flags().BoolVar(&kochDemoDisableSimplify, "no-model-simplify", false, "отключить упрощение базы синтетической модели")
}

func runKochDemo(_ *cobra.Command, _ []string) error {
	result, err := coastline.Load(coastline.LoadOptions{
		LocalPath: kochDemoInput,
		RemoteURL: kochDemoSourceURL,
		Refresh:   kochDemoRefresh,
	})
	if err != nil {
		return fmt.Errorf("загрузка базовой линии: %w", err)
	}

	modelBase := result.Points
	if kochDemoModelMaxPoints > 0 {
		modelBase = geometry.SimplifyPolyline(modelBase, geometry.SimplifyOptions{MaxPoints: kochDemoModelMaxPoints}).Points
	} else if !kochDemoDisableSimplify {
		modelBase = geometry.SimplifyPolyline(modelBase, geometry.SimplifyOptions{MaxPoints: 6000}).Points
	}
	opts := koch.OrganicOptions{
		Seed:            kochDemoSeed,
		AngleJitterDeg:  kochDemoAngleJitter,
		HeightJitterPct: kochDemoHeightJitter,
	}

	fmt.Printf("Загружена база: %s (%d точек; в синтетической модели %d)\n", result.Source, len(result.Points), len(modelBase))
	fmt.Println("\nВНИМАНИЕ: далее строятся синтетические Кох-подобные кривые; это демонстрация, а не анализ природной фрактальности.")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Итерация\tТочки\tДлина, км\tD\tМасштабы\tR²\tРазброс\tУстойчива")
	fmt.Fprintln(w, "--------\t-----\t---------\t-\t---------\t--\t-------\t---------")
	for iter := 0; iter <= kochDemoIterations; iter++ {
		curve := koch.OrganicKochCurve(modelBase, iter, opts)
		analysis := fractal.AnalyzeBoxCounting(curve)
		dimensionValue, rSquared, spread, stable := "н/д", "н/д", "н/д", "нет"
		if analysis.Valid {
			dimensionValue = fmt.Sprintf("%.5f", analysis.Dimension)
			rSquared = fmt.Sprintf("%.4f", analysis.RegressionRSquared)
			spread = fmt.Sprintf("%.4f", analysis.StabilitySpread)
			if analysis.StableAcrossScales {
				stable = "да"
			}
		}
		fmt.Fprintf(w, "%d\t%d\t%.0f\t%s\t%d\t%s\t%s\t%s\n",
			iter, len(curve), geometry.PolylineLength(curve), dimensionValue, len(analysis.Samples), rSquared, spread, stable)
	}
	w.Flush()
	fmt.Printf("\nСправочное значение классической кривой Коха: D = %.5f\n", math.Log(4)/math.Log(3))

	outputMgr := cli.NewOutputPathManager(kochDemoOutput)
	if err := cli.WriteKochDemoSVGSeries(result.Points, modelBase, kochDemoIterations, opts,
		outputMgr, result.DatasetName, result.Source, result.Validation); err != nil {
		fmt.Printf("Предупреждение: не удалось создать демонстрационные SVG: %v\n", err)
	}

	return nil
}
