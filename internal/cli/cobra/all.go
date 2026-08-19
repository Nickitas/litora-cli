package cobra

import (
	"fmt"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/fractal"
	"coastal-geometry/internal/domain/geometry"

	"github.com/spf13/cobra"
)

var (
	allInput                  string
	allSourceURL              string
	allRefresh                bool
	allOutput                 string
	allErosionStrength        float64
	allSteps                  int
	allWaveDirection          float64
	allWindSpeed              float64
	allFetchSpread            float64
	allFetchSamples           int
	allMaxFetchKM             float64
	allDepthScale             float64
	allExposurePower          float64
	allBathymetry             string
	allBathymetryResolution   float64
	allLeftBoundaryTransport  float64
	allRightBoundaryTransport float64
	allSedimentSources        string
	allStructures             string
	allLithology              string
	allEnableLithology        bool
	allTargetYears            int
	allYearsPerStep           float64
	allStormProbability       float64
	allStormIntensity         float64
	allSeaLevelRise           float64
	allEnableSeasonality      bool
	allSeasonalPhase          float64
	allOutputCSV              string
	allCSVFormat              string
	allOutputGIF              string
	allGIFFPS                 int
	allGIFSkip                int
	allModelMaxPoints         int
	allDisableSimplify        bool
	allWaveInput              string
	allWaveSource             string
	allBreakingIndex          float64
	allBermHeight             float64
	allClosureDepth           float64
	allPorosity               float64
	allCERCCoefficient        float64
	allOffshoreDistance       float64
	allMaxChange              float64
	allMaxBathymetryGap       float64
	allBlackSeaSochi          bool
	allWaterbody              string
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Запустите полный конвейер проверки и моделирования",
	Long: `Запустите полный цикл проверки и моделирования, включая:

1. Проверка береговой линии и метрики
2. Анализ фрактальной размерности
3. Одномерное моделирование CERC по загруженному волновому ряду
4. Пространственный баланс наносов и экспорт результатов

Без файлов и без --waterbody команда запускает демонстрационный открытый
сценарий Сочи со статусом demo. Он не является оценкой годового размыва,
калибровкой или научным отчётом. Для другого водоёма нужны его фактические
входные данные.`,
	RunE: runAll,
}

func init() {
	rootCmd.AddCommand(allCmd)

	// Input options
	allCmd.Flags().StringVar(&allInput, "input", "", "путь к локальному JSON/GeoJSON береговой линии")
	allCmd.Flags().StringVar(&allSourceURL, "source-url", "", "явно включить удалённый GeoJSON-источник")
	allCmd.Flags().BoolVar(&allRefresh, "refresh", false, "принудительное обновление удалённого кэша")
	allCmd.Flags().StringVar(&allOutput, "output", "", "каталог для вывода (по умолчанию: ./output)")

	// Erosion options
	allCmd.Flags().IntVar(&allSteps, "steps", 0, "ограничить число первых состояний волнового ряда (0 — использовать все)")
	allCmd.Flags().StringVar(&allBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	allCmd.Flags().Float64Var(&allBathymetryResolution, "bathymetry-resolution", 0, "шаг регулярной батиметрической сетки в градусах (обязателен для выбранного вручную водоёма)")
	allCmd.Flags().StringVar(&allWaveInput, "wave-input", "", "путь к волновому ряду наблюдений, реанализа или прогноза в CSV/JSON (обязателен)")
	allCmd.Flags().StringVar(&allWaveSource, "wave-source", "", "источник волнового ряда, если он не указан в JSON")
	allCmd.Flags().Float64Var(&allBreakingIndex, "breaking-index", 0.78, "индекс разрушения H_b/h_b")
	allCmd.Flags().Float64Var(&allBermHeight, "berm-height", 2, "высота бермы активного профиля, м")
	allCmd.Flags().Float64Var(&allClosureDepth, "closure-depth", 8, "глубина замыкания активного профиля, м")
	allCmd.Flags().Float64Var(&allPorosity, "porosity", 0.4, "пористость наносов")
	allCmd.Flags().Float64Var(&allCERCCoefficient, "cerc-coefficient", 0.39, "безразмерный коэффициент CERC")
	allCmd.Flags().Float64Var(&allOffshoreDistance, "offshore-sample-distance", 300, "расстояние отбора глубины от берега, м")
	allCmd.Flags().Float64Var(&allMaxChange, "max-shoreline-change", 25, "максимальное смещение берега за одно состояние волн, м")
	allCmd.Flags().Float64Var(&allMaxBathymetryGap, "max-bathymetry-gap", 1500, "максимальная дистанция до реальной точки глубины, м")
	allCmd.Flags().Float64Var(&allLeftBoundaryTransport, "left-boundary-transport", 0, "поток наносов через левую границу, м³/с; положительный направлен внутрь сегмента")
	allCmd.Flags().Float64Var(&allRightBoundaryTransport, "right-boundary-transport", 0, "поток наносов через правую границу, м³/с; положительный направлен из сегмента")
	allCmd.Flags().StringVar(&allSedimentSources, "sediment-sources", "", "JSON внешних источников и стоков наносов по ячейкам")
	allCmd.Flags().StringVar(&allStructures, "structures", "", "JSON сооружений, изменяющих пропуск потока между ячейками")
	allCmd.Flags().BoolVar(&allBlackSeaSochi, "black-sea-sochi", false, "загрузить открытые данные Сочи для демонстрационного конвейера demo")
	allCmd.Flags().StringVar(&allWaterbody, "waterbody", "", "водоём РФ из lito waterbody list")

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
	// Пустой запуск должен быть воспроизводимым и не использовать обзорную
	// линию всего Чёрного моря как локальный инженерный участок. Поэтому
	// выбирается единственный готовый набор с открытыми входами — Сочи.
	if allWaterbody == "" && allInput == "" && allBathymetry == "" && allWaveInput == "" && !allBlackSeaSochi {
		allBlackSeaSochi = true
		fmt.Println("✓ Входные файлы не заданы: выбран демонстрационный набор Чёрного моря — Сочи (demo)")
	}
	if allWaterbody != "" {
		body, err := selectedWaterbody(allWaterbody)
		if err != nil {
			return err
		}
		if allBlackSeaSochi && body.ID != "black-sea-sochi" {
			return fmt.Errorf("--black-sea-sochi можно сочетать только с --waterbody black-sea-sochi")
		}
		// Автосценарий активируется только когда пользователь не передал свои данные.
		if body.ID == "black-sea-sochi" && allInput == "" && allBathymetry == "" && allWaveInput == "" {
			allBlackSeaSochi = true
		}
		fmt.Printf("✓ Выбран водоём: %s (%s)\n", body.Name, body.Availability)
	}
	// Ручной выбор обязан иметь собственные данные участка, а не значения по
	// умолчанию, предназначенные для другого бассейна.
	if allWaterbody != "" && !allBlackSeaSochi {
		if allInput == "" {
			return fmt.Errorf("для выбранного водоёма укажите локальный --input")
		}
		if allBathymetry == "" {
			return fmt.Errorf("для выбранного водоёма укажите локальную --bathymetry")
		}
		if allBathymetryResolution <= 0 {
			return fmt.Errorf("для выбранного водоёма укажите фактический --bathymetry-resolution")
		}
	}
	if allBlackSeaSochi {
		if allWaterbody == "" {
			allWaterbody = "black-sea-sochi"
		}
		paths, err := prepareBlackSeaSochiData(allRefresh)
		if err != nil {
			return fmt.Errorf("подготовка набора Сочи: %w", err)
		}
		allInput = paths.Coastline
		allBathymetry = paths.Bathymetry
		allWaveInput = paths.Waves
		allWaveSource = paths.WaveSource
		if !cmd.Flags().Changed("structures") {
			allStructures = paths.Structures
		}
		if !cmd.Flags().Changed("max-bathymetry-gap") {
			allMaxBathymetryGap = 3000
		}
		if !cmd.Flags().Changed("bathymetry-resolution") {
			allBathymetryResolution = 0.005
		}
		if allSteps == 0 {
			allSteps = 24
		}
		fmt.Printf("✓ Загружен стартовый набор Сочи: %s\n", blackSeaSochiDataDir)
		if paths.StructureWarning != "" {
			fmt.Printf("Предупреждение: %s\n", paths.StructureWarning)
		}
	}

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

	modelInputs, err := loadModelInputs(allBathymetry, allLithology, allEnableLithology, allBathymetryResolution)
	if err != nil {
		return err
	}
	printModelInputWarnings(modelInputs)
	if modelInputs.BathymetryGrid != nil {
		fmt.Printf("✓ Батиметрия загружена: %s (%d точек, шаг %.6f°)\n", modelInputs.BathymetryPath, len(modelInputs.BathymetryGrid.Points), modelInputs.BathymetryGrid.Resolution)
	}
	if modelInputs.LithologyProfile != nil {
		fmt.Printf("✓ Литология загружена: %s (%d точек)\n", modelInputs.LithologyPath, len(modelInputs.LithologyProfile.Points))
	}
	sedimentSources, err := loadLongshoreSedimentSources(allSedimentSources)
	if err != nil {
		return err
	}
	structures, err := loadLongshoreStructures(allStructures)
	if err != nil {
		return err
	}

	// Print validation report
	for _, fix := range result.Validation.Fixes {
		fmt.Printf("fix: %s\n", fix)
	}
	for _, warning := range result.Validation.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}

	// Анализ размерности выполняется только по неизменённым наблюдаемым данным.
	fmt.Println("\nВыполнение анализа фрактальной размерности наблюдаемой линии...")
	dimensionAnalysis := fractal.AnalyzeBoxCounting(result.Points)
	printObservedDimension(result.Points, dimensionAnalysis)

	// Simplify base for model if needed
	modelBase := result.Points
	if allModelMaxPoints > 0 {
		simplified := geometry.SimplifyPolyline(modelBase, geometry.SimplifyOptions{MaxPoints: allModelMaxPoints})
		modelBase = simplified.Points
	} else if !allDisableSimplify {
		simplified := geometry.SimplifyPolyline(modelBase, geometry.SimplifyOptions{MaxPoints: 2000})
		modelBase = simplified.Points
	}

	// Run erosion simulation
	fmt.Println("\nВыполнение моделирования волновой эрозии...")
	if allWaveInput == "" {
		return fmt.Errorf("укажите --wave-input: полный конвейер не использует вымышленные волновые условия")
	}
	climate, err := geometry.LoadWaveClimate(allWaveInput, allWaveSource)
	if err != nil {
		return err
	}
	if allSteps > 0 && allSteps < len(climate.Conditions) {
		climate.Conditions = climate.Conditions[:allSteps]
	}
	scenario := classifyLongshoreScenario(allBlackSeaSochi, climate, modelInputs)
	model, err := geometry.RunLongshoreCERC(modelBase, climate, geometry.LongshoreModelConfig{
		Scenario:                  scenario,
		Bathymetry:                modelInputs.BathymetryGrid,
		BathymetrySource:          modelInputs.BathymetryPath,
		BathymetrySHA256:          modelInputs.BathymetrySHA256,
		BathymetryPassport:        modelInputs.BathymetryPassportPath,
		BathymetryStatus:          modelInputs.BathymetryStatus,
		WaterbodyID:               allWaterbody,
		SedimentSources:           sedimentSources,
		Structures:                structures,
		LeftBoundaryTransportM3S:  allLeftBoundaryTransport,
		RightBoundaryTransportM3S: allRightBoundaryTransport,
		BreakingIndex:             allBreakingIndex,
		BermHeightMeters:          allBermHeight,
		ClosureDepthMeters:        allClosureDepth,
		Porosity:                  allPorosity,
		CERCCoefficient:           allCERCCoefficient,
		OffshoreSampleDistanceM:   allOffshoreDistance,
		MaxShorelineChangeMeters:  allMaxChange,
		MaxBathymetryGapMeters:    allMaxBathymetryGap,
	})
	if err != nil {
		return fmt.Errorf("расчёт вдольберегового транспорта: %w", err)
	}
	snapshots := model.Snapshots
	printLongshoreInputQuality(model.InputQuality)
	printScenarioClassification(model.ScenarioClassification)
	waveOptions := geometry.WaveErosionOptions{WindSourceDirectionDeg: climate.Conditions[0].DirectionFromDeg, BathymetryGrid: modelInputs.BathymetryGrid}
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

	// Отчёт dimension использует исходную наблюдаемую линию, а не упрощённую
	// базу эрозионной модели, и наследует статус внешнего сценария.
	if err := cli.WriteDimensionSVG(result.Points, outputMgr, result.DatasetName, result.Source, result.Validation, model.ScenarioClassification); err != nil {
		fmt.Printf("Предупреждение: не удалось создать SVG для размерности: %v\n", err)
	}

	// Erosion series SVG
	simulationSteps := len(snapshots) - 1
	if simulationSteps < 0 {
		simulationSteps = 0
	}
	if err := cli.WriteErosionSVGSeries(result.Points, snapshots, simulationSteps, 0, 0,
		waveOptions, outputMgr, result.DatasetName, result.Source, result.Validation, model.ScenarioClassification); err != nil {
		fmt.Printf("Предупреждение: не удалось создать SVG для эрозии: %v\n", err)
	}
	if err := cli.WriteLongshoreModelJSON(model, outputMgr); err != nil {
		return fmt.Errorf("сохранение баланса наносов: %w", err)
	}
	if err := exportErosionArtifacts(outputMgr, snapshots, nil, allOutputCSV, allCSVFormat, allOutputGIF, allGIFFPS, allGIFSkip); err != nil {
		return err
	}

	return nil
}
