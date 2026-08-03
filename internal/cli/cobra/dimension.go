package cobra

import (
	"fmt"
	"math"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/fractal"
	"coastal-geometry/internal/domain/generators/koch"
	"coastal-geometry/internal/domain/geometry"

	"github.com/spf13/cobra"
)

var (
	dimInput           string
	dimSourceURL       string
	dimRefresh         bool
	dimOutput          string
	dimIterations      int
	dimSeed            int64
	dimAngleJitter     float64
	dimHeightJitter    float64
	dimErosionStrength float64
	dimModelMaxPoints  int
	dimDisableSimplify bool
)

var dimensionCmd = &cobra.Command{
	Use:   "dimension",
	Short: "Анализ фрактальной размерности с использованием подсчета блоков",
	Long: `Проанализируйте фрактальную размерность геометрии береговой линии,
используя метод подсчета квадратов. Этот научный анализ помогает охарактеризовать
сложность и неровности береговых объектов.`,
	RunE: runDimension,
}

func init() {
	rootCmd.AddCommand(dimensionCmd)

	dimensionCmd.Flags().StringVar(&dimInput, "input", coastline.DefaultCoastlineJSONPath, "путь к локальному JSON/GeoJSON береговой линии")
	dimensionCmd.Flags().StringVar(&dimSourceURL, "source-url", coastline.DefaultCoastlineGeoJSONURL, "URL удалённого GeoJSON")
	dimensionCmd.Flags().BoolVar(&dimRefresh, "refresh", false, "принудительное обновление удалённого кэша")
	dimensionCmd.Flags().StringVar(&dimOutput, "output", "", "каталог для вывода (по умолчанию: ./output)")
	dimensionCmd.Flags().IntVar(&dimIterations, "iterations", 5, "максимальное количество органических итераций Коха")
	dimensionCmd.Flags().Int64Var(&dimSeed, "seed", 42, "зерно генератора случайных чисел для органической генерации")
	dimensionCmd.Flags().Float64Var(&dimAngleJitter, "angle-jitter", 18, "максимальное случайное отклонение угла в градусах")
	dimensionCmd.Flags().Float64Var(&dimHeightJitter, "height-jitter", 0.25, "максимальное случайное отклонение высоты как отношение")
	dimensionCmd.Flags().Float64Var(&dimErosionStrength, "erosion-strength", 0, "сила гауссовой эрозии после роста")
	dimensionCmd.Flags().IntVar(&dimModelMaxPoints, "model-max-points", 0, "максимальное количество точек для базовой модели")
	dimensionCmd.Flags().BoolVar(&dimDisableSimplify, "no-model-simplify", false, "отключить упрощение базовой модели")
}

func runDimension(cmd *cobra.Command, args []string) error {
	result, err := coastline.Load(coastline.LoadOptions{
		LocalPath: dimInput,
		RemoteURL: dimSourceURL,
		Refresh:   dimRefresh,
	})
	if err != nil {
		return fmt.Errorf("загрузка береговой линии: %w", err)
	}

	fmt.Printf("Загружено: %s (%d точек)\n", result.Source, len(result.Points))

	// Prepare model base
	modelBase := result.Points
	if dimModelMaxPoints > 0 {
		simplified := geometry.SimplifyPolyline(modelBase, geometry.SimplifyOptions{MaxPoints: dimModelMaxPoints})
		modelBase = simplified.Points
	} else if !dimDisableSimplify {
		simplified := geometry.SimplifyPolyline(modelBase, geometry.SimplifyOptions{MaxPoints: 6000})
		modelBase = simplified.Points
	}

	opts := koch.OrganicOptions{
		Seed:            dimSeed,
		AngleJitterDeg:  dimAngleJitter,
		HeightJitterPct: dimHeightJitter,
	}

	theoreticalDimension := math.Log(4) / math.Log(3)

	fmt.Println("\n  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  ФРАКТАЛЬНАЯ РАЗМЕРНОСТЬ (BOX-COUNTING)")
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("  ┌──────┬───────────┬───────────┬─────────┬──────────┬──────────┬───────────┬───────────┬──────┐")
	fmt.Println("  │ Итер │ Точек     │ Длина км  │ D       │ Масштаб  │ R²       │ Разброс   │ Δ к пред  │ Стаб │")
	fmt.Println("  ├──────┼───────────┼───────────┼─────────┼──────────┼──────────┼───────────┼───────────┼──────┤")

	for iter := 0; iter <= dimIterations; iter++ {
		curve := koch.OrganicKochCurve(modelBase, iter, opts)
		length := geometry.PolylineLength(curve)
		analysis := fractal.AnalyzeBoxCounting(curve)

		delta := "—"
		if analysis.Valid && iter > 0 {
			// Simple delta tracking would require storing prev value
		}

		stable := "no"
		dimensionValue := "n/a"
		rSquared := "n/a"
		spread := "n/a"
		if analysis.Valid {
			dimensionValue = fmt.Sprintf("%.5f", analysis.Dimension)
			rSquared = fmt.Sprintf("%.4f", analysis.RegressionRSquared)
			spread = fmt.Sprintf("%.4f", analysis.StabilitySpread)
			if analysis.StableAcrossScales {
				stable = "yes"
			}
		}

		fmt.Printf("  │ %-4d │ %-9d │ %-9.0f │ %-7s │ %-8s │ %-8s │ %-9s │ %-9s │ %-4s │\n",
			iter, len(curve), length, dimensionValue, fmt.Sprint(len(analysis.Samples)), rSquared, spread, delta, stable)
	}

	fmt.Println("  └──────┴───────────┴───────────┴─────────┴──────────┴──────────┴───────────┴───────────┴──────┘")
	fmt.Printf("\n  Теоретическая (Koch): D = %.5f\n", theoreticalDimension)

	// Create SVG visualizations
	outputMgr := cli.NewOutputPathManager(dimOutput)
	if err := cli.WriteDimensionSVGSeries(modelBase, dimIterations, koch.OrganicOptions{
		Seed:            dimSeed,
		AngleJitterDeg:  dimAngleJitter,
		HeightJitterPct: dimHeightJitter,
	}, outputMgr); err != nil {
		fmt.Printf("Предупреждение: не удалось создать SVG: %v\n", err)
	}

	return nil
}
