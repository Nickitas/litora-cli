package cobra

import (
	"fmt"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"

	"github.com/spf13/cobra"
)

var (
	erosionInput             string
	erosionSourceURL         string
	erosionRefresh           bool
	erosionOutput            string
	erosionSteps             int
	erosionSeed              int64
	erosionStrength          float64
	erosionWaveDirection     float64
	erosionWindSpeed         float64
	erosionFetchSpread       float64
	erosionFetchSamples      int
	erosionMaxFetchKM        float64
	erosionDepthScale        float64
	erosionExposurePower     float64
	erosionBathymetry        string
	erosionLithology         string
	erosionEnableLithology   bool
	erosionTargetYears       int
	erosionYearsPerStep      float64
	erosionStormProbability  float64
	erosionStormIntensity    float64
	erosionSeaLevelRise      float64
	erosionEnableSeasonality bool
	erosionSeasonalPhase     float64
	erosionOutputCSV         string
	erosionCSVFormat         string
	erosionOutputGIF         string
	erosionGIFFPS            int
	erosionGIFSkip           int
)

var erosionCmd = &cobra.Command{
	Use:   "erosion",
	Short: "Моделирование волновой эрозии с использованием физики, основанной на выборке",
	Long: `Моделируйте эрозию побережья, вызванную волнами, используя расчеты энергии волн на основе выборки.
Включает поддержку:

- Воздействие волн на основе батиметрии
- Устойчивость к эрозии на основе литологии
- Временная динамика штормов
- Сезонные колебания
- Повышение уровня моря`,
	RunE: runErosion,
}

func init() {
	rootCmd.AddCommand(erosionCmd)

	// Input options
	erosionCmd.Flags().StringVar(&erosionInput, "input", coastline.DefaultCoastlineJSONPath, "путь к локальному JSON/GeoJSON береговой линии")
	erosionCmd.Flags().StringVar(&erosionSourceURL, "source-url", coastline.DefaultCoastlineGeoJSONURL, "URL удалённого GeoJSON")
	erosionCmd.Flags().BoolVar(&erosionRefresh, "refresh", false, "принудительное обновление удалённого кэша")
	erosionCmd.Flags().StringVar(&erosionOutput, "output", "", "каталог для вывода (по умолчанию: ./output)")

	// Erosion parameters
	erosionCmd.Flags().IntVar(&erosionSteps, "steps", 5, "количество шагов эрозии")
	erosionCmd.Flags().Int64Var(&erosionSeed, "seed", 42, "зерно генератора случайных чисел для моделирования")
	erosionCmd.Flags().Float64Var(&erosionStrength, "erosion-strength", 50, "сила эрозии в метрах")
	erosionCmd.Flags().Float64Var(&erosionWaveDirection, "wave-direction", 0, "направление волны в градусах от севера")
	erosionCmd.Flags().Float64Var(&erosionWindSpeed, "wind-speed", 12, "скорость ветра в м/с")
	erosionCmd.Flags().Float64Var(&erosionFetchSpread, "fetch-spread", 55, "разброс разгон в градусах")
	erosionCmd.Flags().IntVar(&erosionFetchSamples, "fetch-samples", 9, "количество выборок разгон")
	erosionCmd.Flags().Float64Var(&erosionMaxFetchKM, "max-fetch-km", 150, "максимальное расстояние разгон в км")
	erosionCmd.Flags().Float64Var(&erosionDepthScale, "depth-scale", 4000, "масштаб глубины в метрах")
	erosionCmd.Flags().Float64Var(&erosionExposurePower, "exposure-power", 1.5, "степень экспозиции")
	erosionCmd.Flags().StringVar(&erosionBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	erosionCmd.Flags().StringVar(&erosionLithology, "lithology", "", "путь к JSON литологии")
	erosionCmd.Flags().BoolVar(&erosionEnableLithology, "enable-lithology", false, "включить эрозию на основе литологии")

	// Temporal dynamics
	erosionCmd.Flags().IntVar(&erosionTargetYears, "target-years", 0, "целевая продолжительность моделирования в годах")
	erosionCmd.Flags().Float64Var(&erosionYearsPerStep, "years-per-step", 1.0, "лет на шаг эрозии")
	erosionCmd.Flags().Float64Var(&erosionStormProbability, "storm-probability", 0, "вероятность шторма на шаг [0-1]")
	erosionCmd.Flags().Float64Var(&erosionStormIntensity, "storm-intensity", 2.0, "множитель интенсивности шторма")
	erosionCmd.Flags().Float64Var(&erosionSeaLevelRise, "sea-level-rise", 0, "повышение уровня моря в метрах в год")
	erosionCmd.Flags().BoolVar(&erosionEnableSeasonality, "enable-seasonality", false, "включить сезонные колебания")
	erosionCmd.Flags().Float64Var(&erosionSeasonalPhase, "seasonal-phase", 0, "сезонная фаза в радианах")

	// Export options
	erosionCmd.Flags().StringVar(&erosionOutputCSV, "output-csv", "", "путь к CSV файлу для экспорта метрик")
	erosionCmd.Flags().StringVar(&erosionCSVFormat, "csv-format", "long", "формат CSV: long или wide")
	erosionCmd.Flags().StringVar(&erosionOutputGIF, "output-gif", "", "путь к GIF файлу для анимации")
	erosionCmd.Flags().IntVar(&erosionGIFFPS, "gif-fps", 10, "кадров в секунду для GIF")
	erosionCmd.Flags().IntVar(&erosionGIFSkip, "gif-skip", 1, "пропускать каждые N кадров")
}

func runErosion(cmd *cobra.Command, args []string) error {
	result, err := coastline.Load(coastline.LoadOptions{
		LocalPath: erosionInput,
		RemoteURL: erosionSourceURL,
		Refresh:   erosionRefresh,
	})
	if err != nil {
		return fmt.Errorf("загрузка береговой линии: %w", err)
	}

	fmt.Printf("Загружено: %s (%d точек)\n", result.Source, len(result.Points))

	modelInputs, err := loadModelInputs(erosionBathymetry, erosionLithology, erosionEnableLithology)
	if err != nil {
		return err
	}
	if modelInputs.BathymetryGrid != nil {
		fmt.Printf("✓ Батиметрия загружена: %s (%d точек)\n", modelInputs.BathymetryPath, len(modelInputs.BathymetryGrid.Points))
	}
	if modelInputs.LithologyProfile != nil {
		fmt.Printf("✓ Литология загружена: %s (%d точек)\n", modelInputs.LithologyPath, len(modelInputs.LithologyProfile.Points))
	}

	waveOptions := geometry.WaveErosionOptions{
		StrengthMeters:           erosionStrength,
		WindSourceDirectionDeg:   erosionWaveDirection,
		WindSpeedMetersPerSecond: erosionWindSpeed,
		FetchSpreadDeg:           erosionFetchSpread,
		FetchSamples:             erosionFetchSamples,
		MaxFetchMeters:           erosionMaxFetchKM * 1000,
		DepthScaleMeters:         erosionDepthScale,
		ExposurePower:            erosionExposurePower,
		YearsPerStep:             erosionYearsPerStep,
		BathymetryGrid:           modelInputs.BathymetryGrid,
		LithologyProfile:         modelInputs.LithologyProfile,
		EnableLithology:          modelInputs.LithologyEnabled,
	}

	// Check for temporal dynamics and reject silently ignored temporal flags.
	if temporalParametersRequested(erosionTargetYears, erosionYearsPerStep, erosionStormProbability, erosionStormIntensity, erosionSeaLevelRise, erosionEnableSeasonality, erosionSeasonalPhase) {
		if erosionTargetYears <= 0 {
			return fmt.Errorf("для временных параметров укажите --target-years больше нуля")
		}
		if erosionYearsPerStep <= 0 {
			return fmt.Errorf("--years-per-step должен быть больше нуля")
		}
	}
	useTemporal := erosionTargetYears > 0

	var snapshots [][]geometry.LatLon
	var temporalResult *geometry.TemporalResult

	if useTemporal {
		fmt.Printf("Запуск временного моделирования на %d лет...\n", erosionTargetYears)

		temporalParams := geometry.TemporalParameters{
			YearsPerStep:       erosionYearsPerStep,
			StormProbability:   erosionStormProbability,
			StormIntensityMult: erosionStormIntensity,
			SeaLevelRise:       erosionSeaLevelRise,
			Seasonality:        erosionEnableSeasonality,
			SeasonalPhase:      erosionSeasonalPhase,
		}

		result := geometry.SimulateErosionWithDurationSeed(
			result.Points, erosionTargetYears, temporalParams, waveOptions, erosionSeed)
		temporalResult = &result
		snapshots = result.Snapshots

		fmt.Println("\n  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("  ВОЛНОВАЯ ЭРОЗИЯ С ВРЕМЕННОЙ ДИНАМИКОЙ")
		fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("  ┌──────┬──────────┬───────────┬───────────┬─────────────┐")
		fmt.Println("  │ Шаг  │ Год      │ Точек     │ Длина км  │ Площадь км² │")
		fmt.Println("  ├──────┼──────────┼───────────┼───────────┼─────────────┤")

		for i, state := range snapshots {
			year := temporalResult.TemporalStates[i].Year
			length := geometry.PolylineLength(state)
			area := geometry.Area(state)

			fmt.Printf("  │ %-4d │ %-8.0f │ %-9d │ %-9.0f │ %-11.0f │\n",
				i, year, len(state), length, area)
		}
		fmt.Println("  └──────┴──────────┴───────────┴───────────┴─────────────┘")
	} else {
		snapshots = geometry.SimulateWaveErosionWithSeed(
			result.Points, erosionSteps, waveOptions, erosionSeed)

		fmt.Println("\n  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("  ВОЛНОВАЯ ЭРОЗИЯ")
		fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("  ┌──────┬───────────┬───────────┬─────────────┐")
		fmt.Println("  │ Шаг  │ Точек     │ Длина км  │ Площадь км² │")
		fmt.Println("  ├──────┼───────────┼───────────┼─────────────┤")

		for i, state := range snapshots {
			length := geometry.PolylineLength(state)
			area := geometry.Area(state)
			fmt.Printf("  │ %-4d │ %-9d │ %-9.0f │ %-11.0f │\n",
				i, len(state), length, area)
		}
		fmt.Println("  └──────┴───────────┴───────────┴─────────────┘")
	}

	fmt.Println("\n✓ Моделирование эрозии завершено")

	// Create SVG visualizations
	outputMgr := cli.NewOutputPathManager(erosionOutput)
	if err := outputMgr.EnsureDirectories(); err != nil {
		return fmt.Errorf("подготовка каталогов вывода: %w", err)
	}
	simulationSteps := len(snapshots) - 1
	if simulationSteps < 0 {
		simulationSteps = 0
	}
	if err := cli.WriteErosionSVGSeries(result.Points, snapshots, simulationSteps, erosionStrength, erosionSeed,
		waveOptions, outputMgr, result.DatasetName, result.Source, result.Validation); err != nil {
		fmt.Printf("Предупреждение: не удалось создать SVG: %v\n", err)
	}
	if err := exportErosionArtifacts(outputMgr, snapshots, temporalResult, erosionOutputCSV, erosionCSVFormat, erosionOutputGIF, erosionGIFFPS, erosionGIFSkip); err != nil {
		return err
	}
	return nil
}
