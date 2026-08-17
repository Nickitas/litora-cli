package cobra

import (
	"fmt"
	"os"
	"text/tabwriter"

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
	erosionWaveInput         string
	erosionWaveSource        string
	erosionBreakingIndex     float64
	erosionBermHeight        float64
	erosionClosureDepth      float64
	erosionPorosity          float64
	erosionCERCCoefficient   float64
	erosionOffshoreDistance  float64
	erosionMaxChange         float64
	erosionMaxBathymetryGap  float64
	erosionBlackSeaSochi     bool
	erosionWaterbody         string
)

var erosionCmd = &cobra.Command{
	Use:   "erosion",
	Short: "Одномерная вдольбереговая модель транспорта наносов CERC",
	Long: `Выполняет инженерную one-line модель CERC по фактическому волновому ряду.
Расчёт включает дисперсию, рефракцию, shoaling, разрушение волн и
уравнение неразрывности вдольберегового транспорта. Батиметрия и волновой ряд
обязательны; без них расчёт не запускается.`,
	RunE: runErosion,
}

func init() {
	rootCmd.AddCommand(erosionCmd)

	// Input options
	erosionCmd.Flags().StringVar(&erosionInput, "input", "", "путь к локальному JSON/GeoJSON береговой линии")
	erosionCmd.Flags().StringVar(&erosionSourceURL, "source-url", "", "явно включить удалённый GeoJSON-источник")
	erosionCmd.Flags().BoolVar(&erosionRefresh, "refresh", false, "принудительное обновление удалённого кэша")
	erosionCmd.Flags().StringVar(&erosionOutput, "output", "", "каталог для вывода (по умолчанию: ./output)")

	// Erosion parameters
	erosionCmd.Flags().IntVar(&erosionSteps, "steps", 0, "ограничить число первых состояний волнового ряда (0 — использовать все)")
	erosionCmd.Flags().StringVar(&erosionBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	erosionCmd.Flags().StringVar(&erosionWaveInput, "wave-input", "", "путь к фактическому волновому ряду CSV или JSON (обязателен)")
	erosionCmd.Flags().StringVar(&erosionWaveSource, "wave-source", "", "источник волнового ряда, если он не указан в JSON")
	erosionCmd.Flags().Float64Var(&erosionBreakingIndex, "breaking-index", 0.78, "индекс разрушения H_b/h_b")
	erosionCmd.Flags().Float64Var(&erosionBermHeight, "berm-height", 2, "высота бермы активного профиля, м")
	erosionCmd.Flags().Float64Var(&erosionClosureDepth, "closure-depth", 8, "глубина замыкания активного профиля, м")
	erosionCmd.Flags().Float64Var(&erosionPorosity, "porosity", 0.4, "пористость наносов")
	erosionCmd.Flags().Float64Var(&erosionCERCCoefficient, "cerc-coefficient", 0.39, "безразмерный коэффициент CERC")
	erosionCmd.Flags().Float64Var(&erosionOffshoreDistance, "offshore-sample-distance", 300, "расстояние отбора глубины от берега, м")
	erosionCmd.Flags().Float64Var(&erosionMaxChange, "max-shoreline-change", 25, "максимальное смещение берега за одно состояние волн, м")
	erosionCmd.Flags().Float64Var(&erosionMaxBathymetryGap, "max-bathymetry-gap", 1500, "максимальная дистанция до реальной точки глубины, м")
	erosionCmd.Flags().BoolVar(&erosionBlackSeaSochi, "black-sea-sochi", false, "самостоятельно загрузить открытые данные Сочи и выполнить стартовый расчёт")
	erosionCmd.Flags().StringVar(&erosionWaterbody, "waterbody", "", "водоём РФ из lito waterbody list")

	// Export options
	erosionCmd.Flags().StringVar(&erosionOutputCSV, "output-csv", "", "путь к CSV файлу для экспорта метрик")
	erosionCmd.Flags().StringVar(&erosionCSVFormat, "csv-format", "long", "формат CSV: long или wide")
	erosionCmd.Flags().StringVar(&erosionOutputGIF, "output-gif", "", "путь к GIF файлу для анимации")
	erosionCmd.Flags().IntVar(&erosionGIFFPS, "gif-fps", 10, "кадров в секунду для GIF")
	erosionCmd.Flags().IntVar(&erosionGIFSkip, "gif-skip", 1, "пропускать каждые N кадров")
}

func runErosion(cmd *cobra.Command, args []string) error {
	if erosionWaterbody != "" {
		body, err := selectedWaterbody(erosionWaterbody)
		if err != nil {
			return err
		}
		if erosionBlackSeaSochi && body.ID != "black-sea-sochi" {
			return fmt.Errorf("--black-sea-sochi можно сочетать только с --waterbody black-sea-sochi")
		}
		// Если входные файлы не заданы, готовый сценарий Сочи загружается сам.
		if body.ID == "black-sea-sochi" && erosionInput == "" && erosionBathymetry == "" && erosionWaveInput == "" {
			erosionBlackSeaSochi = true
		}
		fmt.Printf("✓ Выбран водоём: %s (%s)\n", body.Name, body.Availability)
	}
	// Для ручного выбора запрещаем незаметно подставить батиметрию Чёрного моря
	// или обзорную береговую линию другого бассейна.
	if erosionWaterbody != "" && !erosionBlackSeaSochi {
		if erosionInput == "" {
			return fmt.Errorf("для выбранного водоёма укажите локальный --input")
		}
		if erosionBathymetry == "" {
			return fmt.Errorf("для выбранного водоёма укажите локальную --bathymetry")
		}
	}
	if erosionBlackSeaSochi {
		if erosionWaterbody == "" {
			erosionWaterbody = "black-sea-sochi"
		}
		paths, err := prepareBlackSeaSochiData()
		if err != nil {
			return fmt.Errorf("подготовка набора Сочи: %w", err)
		}
		erosionInput = paths.Coastline
		erosionBathymetry = paths.Bathymetry
		erosionWaveInput = paths.Waves
		erosionWaveSource = paths.WaveSource
		if !cmd.Flags().Changed("max-bathymetry-gap") {
			erosionMaxBathymetryGap = 3000
		}
		if erosionSteps == 0 {
			erosionSteps = 24
		}
		fmt.Printf("✓ Загружен стартовый набор Сочи: %s\n", blackSeaSochiDataDir)
	}

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

	if erosionWaveInput == "" {
		return fmt.Errorf("укажите --wave-input: модель не использует вымышленные волновые условия")
	}
	climate, err := geometry.LoadWaveClimate(erosionWaveInput, erosionWaveSource)
	if err != nil {
		return err
	}
	if erosionSteps > 0 && erosionSteps < len(climate.Conditions) {
		climate.Conditions = climate.Conditions[:erosionSteps]
	}
	model, err := geometry.RunLongshoreCERC(result.Points, climate, geometry.LongshoreModelConfig{
		Bathymetry:               modelInputs.BathymetryGrid,
		BathymetrySource:         modelInputs.BathymetryPath,
		WaterbodyID:              erosionWaterbody,
		BreakingIndex:            erosionBreakingIndex,
		BermHeightMeters:         erosionBermHeight,
		ClosureDepthMeters:       erosionClosureDepth,
		Porosity:                 erosionPorosity,
		CERCCoefficient:          erosionCERCCoefficient,
		OffshoreSampleDistanceM:  erosionOffshoreDistance,
		MaxShorelineChangeMeters: erosionMaxChange,
		MaxBathymetryGapMeters:   erosionMaxBathymetryGap,
	})
	if err != nil {
		return fmt.Errorf("расчёт вдольберегового транспорта: %w", err)
	}
	snapshots := model.Snapshots
	firstCondition := climate.Conditions[0]
	waveOptions := geometry.WaveErosionOptions{WindSourceDirectionDeg: firstCondition.DirectionFromDeg, BathymetryGrid: modelInputs.BathymetryGrid}

	fmt.Printf("Волновой ряд: %s (%d состояний)\n", climate.Source, len(climate.Conditions))
	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Шаг\tHs, м\tTp, с\tРазмыв, м³\tАккумуляция, м³\tНевязка, м³")
	fmt.Fprintln(table, "---\t-----\t-----\t----------\t----------------\t-----------")
	for i, step := range model.Steps {
		fmt.Fprintf(table, "%d\t%.2f\t%.2f\t%.3f\t%.3f\t%.9f\n", i+1, step.Condition.SignificantWaveHeightM, step.Condition.PeakPeriodSeconds, step.ErodedVolumeM3, step.DepositedVolumeM3, step.MassBalanceM3)
	}
	table.Flush()

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
	if err := cli.WriteErosionSVGSeries(result.Points, snapshots, simulationSteps, 0, 0,
		waveOptions, outputMgr, result.DatasetName, result.Source, result.Validation); err != nil {
		fmt.Printf("Предупреждение: не удалось создать SVG: %v\n", err)
	}
	if err := cli.WriteLongshoreModelJSON(model, outputMgr); err != nil {
		return fmt.Errorf("сохранение баланса наносов: %w", err)
	}
	if err := exportErosionArtifacts(outputMgr, snapshots, nil, erosionOutputCSV, erosionCSVFormat, erosionOutputGIF, erosionGIFFPS, erosionGIFSkip); err != nil {
		return err
	}
	return nil
}
