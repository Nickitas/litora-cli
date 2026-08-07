package cobra

import (
	"fmt"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/generators/koch"
	"coastal-geometry/internal/domain/geometry"

	"github.com/spf13/cobra"
)

var (
	allInput             string
	allSourceURL         string
	allRefresh           bool
	allOutput            string
	allIterations        int
	allSeed              int64
	allAngleJitter       float64
	allHeightJitter      float64
	allErosionStrength   float64
	allSteps             int
	allWaveDirection     float64
	allWindSpeed         float64
	allFetchSpread       float64
	allFetchSamples      int
	allMaxFetchKM        float64
	allDepthScale        float64
	allExposurePower     float64
	allBathymetry        string
	allLithology         string
	allEnableLithology   bool
	allTargetYears       int
	allYearsPerStep      float64
	allStormProbability  float64
	allStormIntensity    float64
	allSeaLevelRise      float64
	allEnableSeasonality bool
	allSeasonalPhase     float64
	allOutputCSV         string
	allCSVFormat         string
	allOutputGIF         string
	allGIFFPS            int
	allGIFSkip           int
	allModelMaxPoints    int
	allDisableSimplify   bool
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Запустите полный конвейер проверки и моделирования",
	Long: `Запустите полный цикл проверки и моделирования, включая:

1. Проверка береговой линии и метрики
2. Анализ фрактальной размерности
3. Моделирование волновой эрозии с учетом временной динамики
4. Оценка качества модели

Это рекомендуемая команда для всестороннего анализа.`,
	RunE: runAll,
}

func init() {
	rootCmd.AddCommand(allCmd)

	// Input options
	allCmd.Flags().StringVar(&allInput, "input", coastline.DefaultCoastlineJSONPath, "путь к локальному JSON/GeoJSON береговой линии")
	allCmd.Flags().StringVar(&allSourceURL, "source-url", coastline.DefaultCoastlineGeoJSONURL, "URL удалённого GeoJSON")
	allCmd.Flags().BoolVar(&allRefresh, "refresh", false, "принудительное обновление удалённого кэша")
	allCmd.Flags().StringVar(&allOutput, "output", "", "каталог для вывода (по умолчанию: ./output)")

	// Koch fractal options
	allCmd.Flags().IntVar(&allIterations, "iterations", 5, "максимальное количество органических итераций Коха")
	allCmd.Flags().Int64Var(&allSeed, "seed", 42, "зерно генератора случайных чисел для органической генерации")
	allCmd.Flags().Float64Var(&allAngleJitter, "angle-jitter", 18, "максимальное случайное отклонение угла в градусах")
	allCmd.Flags().Float64Var(&allHeightJitter, "height-jitter", 0.25, "максимальное случайное отклонение высоты как отношение")

	// Erosion options
	allCmd.Flags().IntVar(&allSteps, "steps", 5, "количество шагов волновой эрозии")
	allCmd.Flags().Float64Var(&allErosionStrength, "erosion-strength", 30, "сила эрозии в метрах")
	allCmd.Flags().Float64Var(&allWaveDirection, "wave-direction", 0, "направление волны в градусах от севера")
	allCmd.Flags().Float64Var(&allWindSpeed, "wind-speed", 12, "скорость ветра в м/с")
	allCmd.Flags().Float64Var(&allFetchSpread, "fetch-spread", 55, "разброс разгон в градусах")
	allCmd.Flags().IntVar(&allFetchSamples, "fetch-samples", 9, "количество выборок разгон")
	allCmd.Flags().Float64Var(&allMaxFetchKM, "max-fetch-km", 150, "максимальное расстояние разгон в км")
	allCmd.Flags().Float64Var(&allDepthScale, "depth-scale", 4000, "масштаб глубины в метрах")
	allCmd.Flags().Float64Var(&allExposurePower, "exposure-power", 1.5, "степень экспозиции")
	allCmd.Flags().StringVar(&allBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	allCmd.Flags().StringVar(&allLithology, "lithology", "", "путь к JSON литологии")
	allCmd.Flags().BoolVar(&allEnableLithology, "enable-lithology", false, "включить эрозию на основе литологии")

	// Temporal dynamics
	allCmd.Flags().IntVar(&allTargetYears, "target-years", 0, "целевая продолжительность моделирования в годах")
	allCmd.Flags().Float64Var(&allYearsPerStep, "years-per-step", 1.0, "лет на шаг эрозии")
	allCmd.Flags().Float64Var(&allStormProbability, "storm-probability", 0, "вероятность шторма на шаг [0-1]")
	allCmd.Flags().Float64Var(&allStormIntensity, "storm-intensity", 2.0, "множитель интенсивности шторма")
	allCmd.Flags().Float64Var(&allSeaLevelRise, "sea-level-rise", 0, "повышение уровня моря в метрах в год")
	allCmd.Flags().BoolVar(&allEnableSeasonality, "enable-seasonality", false, "включить сезонные колебания")
	allCmd.Flags().Float64Var(&allSeasonalPhase, "seasonal-phase", 0, "сезонная фаза в радианах")

	// Export options
	allCmd.Flags().StringVar(&allOutputCSV, "output-csv", "", "путь к CSV файлу для экспорта метрик")
	allCmd.Flags().StringVar(&allCSVFormat, "csv-format", "long", "формат CSV: long или wide")
	allCmd.Flags().StringVar(&allOutputGIF, "output-gif", "", "путь к GIF файлу для анимации")
	allCmd.Flags().IntVar(&allGIFFPS, "gif-fps", 10, "кадров в секунду для GIF")
	allCmd.Flags().IntVar(&allGIFSkip, "gif-skip", 1, "пропускать каждые N кадров")

	// Model options
	allCmd.Flags().IntVar(&allModelMaxPoints, "model-max-points", 0, "максимальное количество точек для базовой модели")
	allCmd.Flags().BoolVar(&allDisableSimplify, "no-model-simplify", false, "отключить упрощение базовой модели")
}

func runAll(cmd *cobra.Command, args []string) error {
	// Load coastline
	result, err := coastline.Load(coastline.LoadOptions{
		LocalPath: allInput,
		RemoteURL: allSourceURL,
		Refresh:   allRefresh,
	})
	if err != nil {
		return fmt.Errorf("загрузка береговой линии: %w", err)
	}

	fmt.Printf("Загружено: %s (%d точек)\n", result.Source, len(result.Points))

	modelInputs, err := loadModelInputs(allBathymetry, allLithology, allEnableLithology)
	if err != nil {
		return err
	}
	if modelInputs.BathymetryGrid != nil {
		fmt.Printf("✓ Батиметрия загружена: %s (%d точек)\n", modelInputs.BathymetryPath, len(modelInputs.BathymetryGrid.Points))
	}
	if modelInputs.LithologyProfile != nil {
		fmt.Printf("✓ Литология загружена: %s (%d точек)\n", modelInputs.LithologyPath, len(modelInputs.LithologyProfile.Points))
	}

	// Print validation report
	for _, fix := range result.Validation.Fixes {
		fmt.Printf("fix: %s\n", fix)
	}
	for _, warning := range result.Validation.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}

	// Run dimension analysis
	fmt.Println("\nВыполнение анализа фрактальной размерности...")
	opts := koch.OrganicOptions{
		Seed:            allSeed,
		AngleJitterDeg:  allAngleJitter,
		HeightJitterPct: allHeightJitter,
	}

	// Simplify base for model if needed
	modelBase := result.Points
	if allModelMaxPoints > 0 {
		simplified := geometry.SimplifyPolyline(modelBase, geometry.SimplifyOptions{MaxPoints: allModelMaxPoints})
		modelBase = simplified.Points
	} else if !allDisableSimplify {
		simplified := geometry.SimplifyPolyline(modelBase, geometry.SimplifyOptions{MaxPoints: 2000})
		modelBase = simplified.Points
	}

	// Run Koch iterations
	for iter := 0; iter <= allIterations; iter++ {
		curve := koch.OrganicKochCurve(modelBase, iter, opts)
		fmt.Printf("  Итерация %d: %d точек, длина %.0f км\n", iter, len(curve), geometry.PolylineLength(curve))
	}

	// Run erosion simulation
	fmt.Println("\nВыполнение моделирования волновой эрозии...")
	if temporalParametersRequested(allTargetYears, allYearsPerStep, allStormProbability, allStormIntensity, allSeaLevelRise, allEnableSeasonality, allSeasonalPhase) {
		if allTargetYears <= 0 {
			return fmt.Errorf("для временных параметров укажите --target-years больше нуля")
		}
		if allYearsPerStep <= 0 {
			return fmt.Errorf("--years-per-step должен быть больше нуля")
		}
	}
	waveOptions := geometry.WaveErosionOptions{
		StrengthMeters:           allErosionStrength,
		WindSourceDirectionDeg:   allWaveDirection,
		WindSpeedMetersPerSecond: allWindSpeed,
		FetchSpreadDeg:           allFetchSpread,
		FetchSamples:             allFetchSamples,
		MaxFetchMeters:           allMaxFetchKM * 1000,
		DepthScaleMeters:         allDepthScale,
		ExposurePower:            allExposurePower,
		BathymetryGrid:           modelInputs.BathymetryGrid,
		LithologyProfile:         modelInputs.LithologyProfile,
		EnableLithology:          modelInputs.LithologyEnabled,
		YearsPerStep:             allYearsPerStep,
	}

	var snapshots [][]geometry.LatLon
	var temporalResult *geometry.TemporalResult
	if allTargetYears > 0 {
		fmt.Printf("Временная динамика: %d лет, %.2f лет на шаг\n", allTargetYears, allYearsPerStep)
		temporalParams := geometry.TemporalParameters{
			YearsPerStep:       allYearsPerStep,
			StormProbability:   allStormProbability,
			StormIntensityMult: allStormIntensity,
			SeaLevelRise:       allSeaLevelRise,
			Seasonality:        allEnableSeasonality,
			SeasonalPhase:      allSeasonalPhase,
		}
		result := geometry.SimulateErosionWithDurationSeed(modelBase, allTargetYears, temporalParams, waveOptions, allSeed)
		temporalResult = &result
		snapshots = result.Snapshots
	} else {
		snapshots = geometry.SimulateWaveErosionWithSeed(modelBase, allSteps, waveOptions, allSeed)
	}
	for i, state := range snapshots {
		fmt.Printf("  Шаг %d: %d точек, длина %.0f км, площадь %.0f км²\n",
			i, len(state), geometry.PolylineLength(state), geometry.Area(state))
	}

	fmt.Println("\n✓ Все анализы завершены")

	// Create SVG visualizations
	outputMgr := cli.NewOutputPathManager(allOutput)
	if err := outputMgr.EnsureDirectories(); err != nil {
		return fmt.Errorf("подготовка каталогов вывода: %w", err)
	}

	// Dimension series SVG
	if err := cli.WriteDimensionSVGSeries(modelBase, allIterations, koch.OrganicOptions{
		Seed:            allSeed,
		AngleJitterDeg:  allAngleJitter,
		HeightJitterPct: allHeightJitter,
	}, outputMgr, result.DatasetName, result.Source, result.Validation); err != nil {
		fmt.Printf("Предупреждение: не удалось создать SVG для размерности: %v\n", err)
	}

	// Erosion series SVG
	simulationSteps := len(snapshots) - 1
	if simulationSteps < 0 {
		simulationSteps = 0
	}
	if err := cli.WriteErosionSVGSeries(result.Points, snapshots, simulationSteps, allErosionStrength, allSeed,
		waveOptions, outputMgr, result.DatasetName, result.Source, result.Validation); err != nil {
		fmt.Printf("Предупреждение: не удалось создать SVG для эрозии: %v\n", err)
	}
	if err := exportErosionArtifacts(outputMgr, snapshots, temporalResult, allOutputCSV, allCSVFormat, allOutputGIF, allGIFFPS, allGIFSkip); err != nil {
		return err
	}

	return nil
}
