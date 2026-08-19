package cobra

import (
	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/benchmark"
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"

	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	benchDir                string
	benchSiteID             string
	benchInput              string
	benchOutput             string
	benchBathymetry         string
	benchMaxDistance        float64
	benchValidationFraction float64
	spectrumSpread          float64
	benchStrength           float64
	benchWaveDir            float64
	// Bounds for extract
	boundsMinLat float64
	boundsMaxLat float64
	boundsMinLon float64
	boundsMaxLon float64
	// Site creation
	siteName        string
	siteRegion      string
	siteCountry     string
	siteDescription string
	siteLithology   string
	siteBounds      string
	siteCoastType   string
	siteQuality     string
	siteCoastline   string
	siteWaveHeight  float64
	siteWavePeriod  float64
	siteObsYearMin  int
	siteObsYearMax  int
	siteWaveDir     float64
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Анализ эталонов и историческая эвристическая калибровка",
	Long: `Контрольные подкоманды для анализа эталонов и исторической эвристической калибровки.

Эти подкоманды сохранены для сравнения со старыми экспериментами. Они не
являются калибровкой one-line CERC-модели; для физической калибровки используйте
команду lito calibrate-cerc с фактическим волновым рядом, батиметрией и
независимыми наблюдениями.

Subcommands:
  list           Перечислите все сайты-эталоны
  init           Инициализировать стандартные черноморские тестовые сайты
  show           Показывать подробную информацию о сайте
  calibrate      Откалибруйте параметры модели для сайта
  calibrate-all  Выполните калибровку всех узлов и подготовьте сводный отчет
  cross-validate Проверьте переносимость параметров между эталонными сайтами
  analyze        Полный анализ: калибровка + чувствительность + bootstrap CI
  hotspots       Выявление очагов эрозии вдоль побережья
  scenarios      Анализ климатических сценариев
  extract        Извлечение сегмента береговой линии по координатам
  create         Создайте новый эталонный сайт`,
	DisableFlagsInUseLine: true,
}

var benchListCmd = &cobra.Command{
	Use:   "list",
	Short: "Перечислите все сайты-эталоны",
	RunE:  runBenchmarkList,
}

var benchInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Инициализировать стандартные черноморские тестовые сайты",
	RunE:  runBenchmarkInit,
}

var benchShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Показывать подробную информацию о сайте",
	RunE:  runBenchmarkShow,
}

var benchCalibrateCmd = &cobra.Command{
	Use:   "calibrate",
	Short: "Исторически откалибровать эвристические параметры для сайта",
	RunE:  runBenchmarkCalibrate,
}

var benchCalibrateAllCmd = &cobra.Command{
	Use:   "calibrate-all",
	Short: "Выполните калибровку всех узлов и подготовьте сводный отчет",
	RunE:  runBenchmarkCalibrateAll,
}

var benchCrossValidateCmd = &cobra.Command{
	Use:   "cross-validate",
	Short: "Проверить параметры на полностью исключённом эталонном сайте",
	RunE:  runBenchmarkCrossValidate,
}

var benchAnalyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Полный анализ: калибровка + чувствительность + bootstrap CI + нулевая модель",
	RunE:  runBenchmarkAnalyze,
}

var benchHotspotsCmd = &cobra.Command{
	Use:   "hotspots",
	Short: "Выявление очагов эрозии вдоль побережья",
	RunE:  runBenchmarkHotspots,
}

var benchScenariosCmd = &cobra.Command{
	Use:   "scenarios",
	Short: "Анализ климатических сценариев",
	RunE:  runBenchmarkScenarios,
}

var benchExtractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Извлечение сегмента береговой линии по координатам",
	RunE:  runBenchmarkExtract,
}

var benchCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Создайте новый эталонный сайт",
	RunE:  runBenchmarkCreate,
}

func init() {
	rootCmd.AddCommand(benchmarkCmd)
	benchmarkCmd.AddGroup(&cobra.Group{ID: "main", Title: "Main Commands"})
	benchmarkCmd.AddGroup(&cobra.Group{ID: "analysis", Title: "Analysis Commands"})
	benchmarkCmd.AddGroup(&cobra.Group{ID: "management", Title: "Site Management"})

	// Main commands
	benchmarkCmd.AddCommand(benchListCmd)
	benchmarkCmd.AddCommand(benchInitCmd)
	benchmarkCmd.AddCommand(benchShowCmd)
	benchmarkCmd.AddCommand(benchCalibrateCmd)
	benchmarkCmd.AddCommand(benchCalibrateAllCmd)
	benchmarkCmd.AddCommand(benchCrossValidateCmd)

	// Analysis commands
	benchmarkCmd.AddCommand(benchAnalyzeCmd)
	benchmarkCmd.AddCommand(benchHotspotsCmd)
	benchmarkCmd.AddCommand(benchScenariosCmd)

	// Management
	benchmarkCmd.AddCommand(benchExtractCmd)
	benchmarkCmd.AddCommand(benchCreateCmd)

	// Common flags
	benchmarkCmd.PersistentFlags().StringVar(&benchDir, "dir", "data/benchmarks", "каталог для эталонных сайтов")

	// Flags for show/calibrate/analyze/hotspots/scenarios
	benchShowCmd.Flags().StringVar(&benchSiteID, "site", "", "ID сайта")
	benchCalibrateCmd.Flags().StringVar(&benchSiteID, "site", "", "ID сайта")
	benchAnalyzeCmd.Flags().StringVar(&benchSiteID, "site", "", "ID сайта")
	benchHotspotsCmd.Flags().StringVar(&benchSiteID, "site", "", "ID сайта")
	benchScenariosCmd.Flags().StringVar(&benchSiteID, "site", "", "ID сайта")

	benchShowCmd.Flags().StringVar(&benchBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	benchCalibrateCmd.Flags().StringVar(&benchBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	benchCalibrateCmd.Flags().Float64Var(&benchMaxDistance, "max-distance-km", 5, "максимальное расстояние наблюдения до сегмента береговой линии, км")
	benchCalibrateCmd.Flags().Float64Var(&benchValidationFraction, "validation-fraction", 0.25, "доля пространственно отложенных наблюдений для проверки")
	benchCalibrateCmd.Flags().StringVar(&benchOutput, "output", "", "путь к выходному файлу")
	benchAnalyzeCmd.Flags().StringVar(&benchBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	benchAnalyzeCmd.Flags().StringVar(&benchOutput, "output", "", "путь к выходному файлу")
	benchHotspotsCmd.Flags().StringVar(&benchBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	benchHotspotsCmd.Flags().Float64Var(&benchStrength, "erosion-strength", 0, "сила эрозии в метрах")
	benchHotspotsCmd.Flags().Float64Var(&benchWaveDir, "wave-direction", -1, "направление волны в градусах")
	benchHotspotsCmd.Flags().StringVar(&benchOutput, "output", "", "путь к выходному файлу")
	benchScenariosCmd.Flags().Float64Var(&benchStrength, "erosion-strength", 0, "сила эрозии в метрах")
	benchScenariosCmd.Flags().Float64Var(&benchWaveDir, "wave-direction", -1, "направление волны в градусах")
	benchScenariosCmd.Flags().StringVar(&benchBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	benchScenariosCmd.Flags().StringVar(&benchOutput, "output", "", "путь к выходному файлу")

	benchCalibrateAllCmd.Flags().StringVar(&benchBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	benchCalibrateAllCmd.Flags().Float64Var(&benchMaxDistance, "max-distance-km", 5, "максимальное расстояние наблюдения до сегмента береговой линии, км")
	benchCalibrateAllCmd.Flags().Float64Var(&benchValidationFraction, "validation-fraction", 0.25, "доля пространственно отложенных наблюдений для проверки")
	benchCalibrateAllCmd.Flags().StringVar(&benchOutput, "output", "", "каталог для вывода")
	benchCalibrateAllCmd.Flags().Float64Var(&spectrumSpread, "spectrum-spread", 0, "направленный разброс волнового спектра в градусах")

	benchCrossValidateCmd.Flags().StringVar(&benchBathymetry, "bathymetry", "", "путь к JSON батиметрии")
	benchCrossValidateCmd.Flags().Float64Var(&benchMaxDistance, "max-distance-km", 5, "максимальное расстояние наблюдения до сегмента береговой линии, км")
	benchCrossValidateCmd.Flags().Float64Var(&benchValidationFraction, "validation-fraction", 0.25, "доля пространственно отложенных наблюдений в локальной диагностике")
	benchCrossValidateCmd.Flags().Float64Var(&spectrumSpread, "spectrum-spread", 0, "направленный разброс волнового спектра в градусах")
	benchCrossValidateCmd.Flags().StringVar(&benchOutput, "output", "", "путь к JSON-отчёту")

	// Extract flags
	benchExtractCmd.Flags().StringVar(&benchInput, "input", coastline.DefaultCoastlineJSONPath, "путь к JSON береговой линии")
	benchExtractCmd.Flags().StringVar(&benchOutput, "output", "", "путь к выходному файлу")
	benchExtractCmd.Flags().Float64Var(&boundsMinLat, "bounds-min-lat", 0, "минимальная широта")
	benchExtractCmd.Flags().Float64Var(&boundsMaxLat, "bounds-max-lat", 0, "максимальная широта")
	benchExtractCmd.Flags().Float64Var(&boundsMinLon, "bounds-min-lon", 0, "минимальная долгота")
	benchExtractCmd.Flags().Float64Var(&boundsMaxLon, "bounds-max-lon", 0, "максимальная долгота")

	// Create flags
	benchCreateCmd.Flags().StringVar(&siteName, "name", "", "название сайта (обязательно)")
	benchCreateCmd.Flags().StringVar(&siteRegion, "region", "", "название региона")
	benchCreateCmd.Flags().StringVar(&siteCountry, "country", "", "название страны")
	benchCreateCmd.Flags().StringVar(&siteDescription, "description", "", "описание сайта")
	benchCreateCmd.Flags().StringVar(&siteBounds, "bounds", "", "границы: min_lat,max_lat,min_lon,max_lon (обязательно)")
	benchCreateCmd.Flags().StringVar(&siteCoastType, "coast-type", "mixed", "тип берега: sandy|cliff|rocky|muddy|mixed|artificial")
	benchCreateCmd.Flags().StringVar(&siteQuality, "quality", "medium", "качество данных: high|medium|low")
	benchCreateCmd.Flags().StringVar(&siteLithology, "lithology", "mixed", "доминирующая литология")
	benchCreateCmd.Flags().Float64Var(&siteWaveHeight, "mean-wave-height", 1.0, "средняя значительная высота волны в метрах")
	benchCreateCmd.Flags().Float64Var(&siteWavePeriod, "mean-wave-period", 5.0, "средний период волны в секундах")
	benchCreateCmd.Flags().Float64Var(&siteWaveDir, "wave-direction", 0, "среднее направление волны в градусах")
	benchCreateCmd.Flags().IntVar(&siteObsYearMin, "obs-year-min", 2000, "год начала наблюдений")
	benchCreateCmd.Flags().IntVar(&siteObsYearMax, "obs-year-max", 2024, "год окончания наблюдений")
	benchCreateCmd.Flags().StringVar(&siteCoastline, "coastline", "", "путь к GeoJSON/JSON береговой линии")
}

func runBenchmarkList(cmd *cobra.Command, args []string) error {
	repo := benchmark.NewRepository(benchDir)
	sites, err := repo.LoadAll()
	if err != nil {
		return fmt.Errorf("загрузка эталонных сайтов: %w", err)
	}

	if len(sites) == 0 {
		fmt.Println("Эталонные сайты не найдены.")
		fmt.Println("\nИнициализируйте стандартные сайты с помощью:")
		fmt.Println("  lito benchmark init")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tРегион\tСтрана\tТип\tКачество\tГоды")
	fmt.Fprintln(w, "----\t------\t-------\t----\t--------\t-----")

	for _, site := range sites {
		years := fmt.Sprintf("%.0f-%.0f", site.ObservationYears.Min, site.ObservationYears.Max)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			site.ID, site.Region, site.Country,
			site.CoastType, site.DataQuality, years)
	}
	w.Flush()

	return nil
}

func runBenchmarkInit(cmd *cobra.Command, args []string) error {
	fmt.Println("Инициализация стандартных эталонных сайтов...")
	fmt.Println()

	repo := benchmark.NewRepository(benchDir)

	loadResult, err := coastline.Load(coastline.LoadOptions{
		RemoteURL: coastline.DefaultCoastlineGeoJSONURL,
	})
	if err != nil {
		return fmt.Errorf("загрузка береговой линии: %w", err)
	}
	fmt.Printf("Загружена береговая линия: %s (%d точек)\n\n", loadResult.Source, len(loadResult.Points))

	sites := benchmark.StandardSites()
	for _, site := range sites {
		fmt.Printf("  %s: %s (%s)\n", site.ID, site.Name, site.Country)

		site.Coastline = benchmark.ExtractCoastline(loadResult.Points, site.Bounds.ToGeoBounds())

		if len(site.Coastline) < 2 {
			fmt.Printf("    ⚠️  Предупреждение: В границах не найдено точек береговой линии\n")
			continue
		}

		site.ObservedErosion = benchmark.ObservationsForSite(site.ID)

		if err := repo.Save(site); err != nil {
			return fmt.Errorf("сохранение сайта %s: %w", site.ID, err)
		}

		obsCount := len(site.ObservedErosion)
		if obsCount > 0 {
			fmt.Printf("    ✓ %d точек береговой линии, %d наблюдений эрозии\n",
				len(site.Coastline), obsCount)
		} else {
			fmt.Printf("    ✓ %d точек береговой линии извлечено\n", len(site.Coastline))
		}
	}

	fmt.Println("\nЭталонные сайты инициализированы!")
	fmt.Println("\nИспользуйте:")
	fmt.Println("  lito benchmark list    - список всех сайтов")
	fmt.Println("  lito benchmark show ID - подробности сайта")

	return nil
}

func runBenchmarkShow(cmd *cobra.Command, args []string) error {
	if benchSiteID == "" {
		return fmt.Errorf("укажите ID сайта с помощью --site")
	}

	repo := benchmark.NewRepository(benchDir)
	site, err := repo.Load(benchSiteID)
	if err != nil {
		return fmt.Errorf("загрузка эталонного сайта %q: %w", benchSiteID, err)
	}

	fmt.Printf("┌─ %s ─────────────────────────────────────\n", site.ID)
	fmt.Printf("│ Название:        %s\n", site.Name)
	fmt.Printf("│ Регион:          %s\n", site.Region)
	fmt.Printf("│ Страна:          %s\n", site.Country)
	fmt.Printf("│ Тип:             %s\n", site.CoastType)
	fmt.Printf("│ Литология:       %s\n", site.DominantLithology)
	fmt.Printf("│ Качество:        %s\n", site.DataQuality)
	fmt.Printf("│ Годы:            %.0f - %.0f\n", site.ObservationYears.Min, site.ObservationYears.Max)
	fmt.Printf("│\n")
	fmt.Printf("│ Границы:\n")
	fmt.Printf("│   Широта: %.3f до %.3f\n", site.Bounds.MinLat, site.Bounds.MaxLat)
	fmt.Printf("│   Долгота: %.3f до %.3f\n", site.Bounds.MinLon, site.Bounds.MaxLon)
	fmt.Printf("│\n")
	fmt.Printf("│ Волновые условия:\n")
	fmt.Printf("│   Высота:    %.1f м\n", site.MeanWaveHeight)
	fmt.Printf("│   Период:    %.1f с\n", site.MeanWavePeriod)
	fmt.Printf("│   Направление: %.0f°\n", site.MeanWaveDirection)
	fmt.Printf("│\n")
	fmt.Printf("│ Точек береговой линии: %d\n", len(site.Coastline))
	fmt.Printf("│ Наблюдений эрозии:     %d\n", len(site.ObservedErosion))
	fmt.Printf("│\n")
	fmt.Printf("│ Источник данных: %s\n", site.DataSource)
	if len(site.References) > 0 {
		fmt.Printf("│ Ссылки:\n")
		for _, ref := range site.References {
			fmt.Printf("│   - %s\n", ref)
		}
	}
	fmt.Printf("└────────────────────────────────────────────\n")

	return nil
}

func runBenchmarkCalibrate(cmd *cobra.Command, args []string) error {
	fmt.Println("Предупреждение: benchmark calibrate использует историческую эвристику; для CERC используйте lito calibrate-cerc.")
	if benchSiteID == "" {
		return fmt.Errorf("укажите ID сайта с помощью --site")
	}

	repo := benchmark.NewRepository(benchDir)
	site, err := repo.Load(benchSiteID)
	if err != nil {
		return fmt.Errorf("загрузка эталонного сайта %q: %w", benchSiteID, err)
	}

	if len(site.ObservedErosion) == 0 {
		return fmt.Errorf("сайт %q не имеет данных наблюдений эрозии для калибровки", benchSiteID)
	}

	fmt.Printf("Калибровка модели для сайта: %s\n", site.Name)
	fmt.Printf("Точек береговой линии: %d, наблюдений: %d\n", len(site.Coastline), len(site.ObservedErosion))

	if benchBathymetry == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			benchBathymetry = "data/black-sea-bathymetry.json"
		}
	}

	var bathymetry *geometry.BathymetryGrid
	if benchBathymetry != "" {
		data, err := os.ReadFile(benchBathymetry)
		if err != nil {
			return fmt.Errorf("чтение файла батиметрии: %w", err)
		}
		grid, err := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		if err != nil {
			return fmt.Errorf("загрузка батиметрии: %w", err)
		}
		bathymetry = grid
		fmt.Printf("Батиметрия: %s (%d точек сетки)\n", benchBathymetry, len(grid.Points))
	} else {
		fmt.Println("Батиметрия: НЕ загружена (выполните 'make bathymetry' для загрузки)")
	}
	fmt.Println()

	config := benchmark.DefaultCalibrationConfig()
	config.MaxDistanceKm = benchMaxDistance
	config.ValidationFraction = benchValidationFraction
	config.BathymetryGrid = bathymetry
	fmt.Printf("Пространство параметров: %d сил эрозии × %d направлений волн = %d запусков\n",
		len(config.ErosionStrengths), len(config.WaveDirections),
		len(config.ErosionStrengths)*len(config.WaveDirections))

	var results []benchmark.CalibrationResultItem
	if bathymetry != nil {
		results, err = benchmark.CalibrateWithBathymetry(*site, config, bathymetry)
	} else {
		results, err = benchmark.Calibrate(*site, config)
	}
	if err != nil {
		return fmt.Errorf("калибровка не удалась: %w", err)
	}

	if len(results) == 0 {
		return fmt.Errorf("результаты калибровки не получены")
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  ТОП-5 КОМБИНАЦИЙ: выбор только по обучающей выборке")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Ранг\tСила, м\tНапр. волн, °\tВзвешенный RMSE обучения\tВзвешенный RMSE проверки\tТочек обучение/проверка")
	fmt.Fprintln(table, "----\t--------\t--------------\t-------------------------\t-------------------------\t------------------------")

	topN := 5
	if len(results) < topN {
		topN = len(results)
	}
	for i := 0; i < topN; i++ {
		r := results[i]
		validationRMSE := "—"
		if r.ValidationMetrics.N > 0 {
			validationRMSE = fmt.Sprintf("%.3f", r.ValidationMetrics.WeightedRMSE)
		}
		fmt.Fprintf(table, "%d\t%.1f\t%.1f\t%.3f\t%s\t%d/%d\n",
			i+1, r.ErosionStrength, r.WaveDirection,
			r.TrainingMetrics.WeightedRMSE, validationRMSE,
			r.TrainingMetrics.N, r.ValidationMetrics.N)
	}
	table.Flush()
	fmt.Println()

	best := results[0]
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("  ЛУЧШАЯ ПОДГОНКА ПО ОБУЧАЮЩИМ ДАННЫМ: erosion-strength=%.1f м, wave-direction=%.0f°\n", best.ErosionStrength, best.WaveDirection)
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("  Наблюдений принято/исключено: %d/%d (предел %.2f км, максимум %.2f км)\n",
		best.Matching.Accepted, best.Matching.ExcludedByDistance+best.Matching.ExcludedInvalidPeriod+best.Matching.ExcludedInvalidUncertainty, best.Matching.MaxDistanceKm, best.Matching.MaximumMatchedKm)
	if len(best.Matching.Warnings) > 0 {
		fmt.Println("  Предупреждения контроля качества:")
		for _, warning := range best.Matching.Warnings {
			fmt.Printf("  • %s\n", warning)
		}
	}
	fmt.Printf("  Взвешенный RMSE обучения: %.3f м/год (%d точек)\n", best.TrainingMetrics.WeightedRMSE, best.TrainingMetrics.N)
	if best.ValidationMetrics.N > 0 {
		fmt.Printf("  Взвешенный RMSE проверки: %.3f м/год (%d точек)\n", best.ValidationMetrics.WeightedRMSE, best.ValidationMetrics.N)
	} else {
		fmt.Println("  Проверочная выборка не сформирована: после отсева недостаточно точек.")
	}
	if best.ValidationMetrics.InferenceAllowed {
		fmt.Printf("  Проверочное R²: %.3f; p с поправкой Бонферрони: %.4f\n", best.ValidationMetrics.RSquared, best.ValidationMetrics.AdjustedPValue)
	} else {
		fmt.Println("  R² и p-значение для проверки не интерпретируются: отложенная выборка слишком мала.")
	}
	fmt.Println("  Это оценка подгонки малой выборки, а не доказательство прогностической способности модели.")
	fmt.Println()

	if reportPath, diagnosticsPath, err := writeBenchmarkCalibrationArtifacts(*site, config, best, results, benchOutput); err != nil {
		return fmt.Errorf("сохранение отчёта калибровки: %w", err)
	} else {
		fmt.Printf("Отчёт о калибровке сохранён в: %s\n", reportPath)
		fmt.Printf("Диагностика сопоставления сохранена в: %s\n", diagnosticsPath)
	}

	return nil
}

// writeBenchmarkCalibrationArtifacts сохраняет отчёт и диагностическую таблицу
// каждой калибровки в стандартной структуре output/.
func writeBenchmarkCalibrationArtifacts(site benchmark.BenchmarkSite, config benchmark.CalibrationConfig, best benchmark.CalibrationResultItem, results []benchmark.CalibrationResultItem, requestedReportPath string) (string, string, error) {
	output := cli.NewOutputPathManager("")
	if err := output.EnsureDirectories(); err != nil {
		return "", "", err
	}

	topN := 5
	if len(results) < topN {
		topN = len(results)
	}
	report := struct {
		SiteID   string `json:"site_id"`
		SiteName string `json:"site_name"`
		Config   struct {
			YearsPerStep       float64 `json:"years_per_step"`
			LegacyTotalYears   int     `json:"legacy_total_years"`
			MaxDistanceKm      float64 `json:"max_distance_km"`
			ValidationFraction float64 `json:"validation_fraction"`
			UsesBathymetry     bool    `json:"uses_bathymetry"`
		} `json:"config"`
		BestFit             benchmark.CalibrationResultItem   `json:"best_fit"`
		TopResults          []benchmark.CalibrationResultItem `json:"top_results"`
		MatchingDiagnostics []benchmark.MatchingDiagnostic    `json:"matching_diagnostics"`
	}{
		SiteID: site.ID, SiteName: site.Name, BestFit: best,
		TopResults: results[:topN], MatchingDiagnostics: best.Matching.Diagnostics,
	}
	report.Config.YearsPerStep = config.YearsPerStep
	report.Config.LegacyTotalYears = config.TotalYears
	report.Config.MaxDistanceKm = config.MaxDistanceKm
	report.Config.ValidationFraction = config.ValidationFraction
	report.Config.UsesBathymetry = config.BathymetryGrid != nil
	reportPath := output.MetricsPath(fmt.Sprintf("benchmark-calibration-%s.json", site.ID))
	if requestedReportPath != "" {
		reportPath = output.ResolveUserPath(requestedReportPath, "metrics")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("сериализация JSON: %w", err)
	}
	if err := os.WriteFile(reportPath, data, 0o644); err != nil {
		return "", "", fmt.Errorf("запись JSON: %w", err)
	}

	diagnosticsPath := output.CSVPath(fmt.Sprintf("benchmark-calibration-%s-diagnostics.tsv", site.ID))
	file, err := os.Create(diagnosticsPath)
	if err != nil {
		return "", "", fmt.Errorf("создание таблицы диагностики: %w", err)
	}
	defer file.Close()
	table := tabwriter.NewWriter(file, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Индекс\tШирота\tДолгота\tНачало\tКонец\tПериод, лет\tНеопределённость, м/год\tДистанция, км\tСегмент\tПоложение на сегменте\tВыборка\tСтатус\tПричина")
	fmt.Fprintln(table, "-----\t------\t-------\t------\t-----\t-----------\t------------------------\t-------------\t-------\t---------------------\t--------\t------\t-------")
	for _, diagnostic := range best.Matching.Diagnostics {
		fmt.Fprintf(table, "%d\t%.6f\t%.6f\t%s\t%s\t%.3f\t%.3f\t%.3f\t%d\t%.5f\t%s\t%s\t%s\n",
			diagnostic.ObservationIndex, diagnostic.LatLon.Lat, diagnostic.LatLon.Lon,
			diagnostic.StartDate, diagnostic.EndDate, diagnostic.ObservationYears, diagnostic.Uncertainty,
			diagnostic.DistanceToCoastKm, diagnostic.CoastSegment, diagnostic.SegmentPosition,
			diagnostic.Split, diagnostic.Status, diagnostic.Reason)
	}
	if err := table.Flush(); err != nil {
		return "", "", fmt.Errorf("запись таблицы диагностики: %w", err)
	}
	return reportPath, diagnosticsPath, nil
}

func runBenchmarkCalibrateAll(cmd *cobra.Command, args []string) error {
	repo := benchmark.NewRepository(benchDir)
	sites, err := repo.LoadAll()
	if err != nil {
		return err
	}
	if len(sites) == 0 {
		return fmt.Errorf("эталонные сайты не найдены, сначала выполните 'lito benchmark init'")
	}

	if benchBathymetry == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			benchBathymetry = "data/black-sea-bathymetry.json"
		}
	}

	var bathymetry *geometry.BathymetryGrid
	if benchBathymetry != "" {
		data, _ := os.ReadFile(benchBathymetry)
		grid, err := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		if err == nil {
			bathymetry = grid
			fmt.Printf("Батиметрия: %s (%d точек)\n", benchBathymetry, len(grid.Points))
		}
	}
	if spectrumSpread > 0 {
		fmt.Printf("Разброс волнового спектра: %.1f°\n", spectrumSpread)
	}
	fmt.Println()

	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  КАЛИБРОВКА ВСЕХ ЭТАЛОННЫХ САЙТОВ")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Сайт\tСила, м\tНапр. волн, °\tRMSE обучения\tMAE обучения\tMBE обучения\tR² обучения")
	fmt.Fprintln(table, "----\t--------\t--------------\t-------------\t------------\t------------\t------------")

	config := benchmark.DefaultCalibrationConfig()
	config.MaxDistanceKm = benchMaxDistance
	config.ValidationFraction = benchValidationFraction
	config.SpectrumSpreadDeg = spectrumSpread

	for _, site := range sites {
		var results []benchmark.CalibrationResultItem
		if bathymetry != nil {
			results, err = benchmark.CalibrateWithBathymetry(site, config, bathymetry)
		} else {
			results, err = benchmark.Calibrate(site, config)
		}
		if err != nil || len(results) == 0 {
			fmt.Fprintf(table, "%s\tОШИБКА: %v\n", site.ID, err)
			continue
		}

		best := results[0]
		fmt.Fprintf(table, "%s\t%.1f\t%.0f\t%.3f\t%.3f\t%+.3f\t%.3f\n",
			site.ID, best.ErosionStrength, best.WaveDirection,
			best.TrainingMetrics.RMSE, best.TrainingMetrics.MAE,
			best.TrainingMetrics.MBE, best.TrainingMetrics.RSquared)
	}
	table.Flush()

	fmt.Println()
	return nil
}

// runBenchmarkCrossValidate выполняет leave-one-site-out проверку: один сайт
// полностью исключается из выбора параметров и применяется только для оценки.
func runBenchmarkCrossValidate(cmd *cobra.Command, args []string) error {
	repo := benchmark.NewRepository(benchDir)
	sites, err := repo.LoadAll()
	if err != nil {
		return fmt.Errorf("загрузка эталонных сайтов: %w", err)
	}
	if len(sites) < 2 {
		return fmt.Errorf("для межсайтовой проверки нужны как минимум два эталонных сайта")
	}

	if benchBathymetry == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			benchBathymetry = "data/black-sea-bathymetry.json"
		}
	}
	var bathymetry *geometry.BathymetryGrid
	if benchBathymetry != "" {
		data, err := os.ReadFile(benchBathymetry)
		if err != nil {
			return fmt.Errorf("чтение файла батиметрии: %w", err)
		}
		bathymetry, err = geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		if err != nil {
			return fmt.Errorf("загрузка батиметрии: %w", err)
		}
	}

	config := benchmark.DefaultCalibrationConfig()
	config.MaxDistanceKm = benchMaxDistance
	config.ValidationFraction = benchValidationFraction
	config.SpectrumSpreadDeg = spectrumSpread
	config.BathymetryGrid = bathymetry

	fmt.Printf("Межсайтовая проверка: %d эталонов, %d комбинаций параметров\n", len(sites), len(config.ErosionStrengths)*len(config.WaveDirections))
	result, err := benchmark.CrossValidateSites(sites, config)
	if err != nil {
		return fmt.Errorf("межсайтовая проверка не удалась: %w", err)
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  МЕЖСАЙТОВАЯ ПРОВЕРКА: ИСКЛЮЧЁННЫЙ САЙТ НЕ УЧАСТВУЕТ В ПОДБОРЕ")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Исключённый сайт\tСила, м\tНапр. волн, °\tВзвешенный RMSE обучения\tВзвешенный RMSE внешней проверки\tТочек внешней проверки")
	fmt.Fprintln(table, "----------------\t--------\t--------------\t-------------------------\t---------------------------------\t------------------------")
	for _, fold := range result.Folds {
		fmt.Fprintf(table, "%s\t%.1f\t%.1f\t%.3f\t%.3f\t%d\n",
			fold.HeldOutSiteID, fold.ErosionStrength, fold.WaveDirection,
			fold.TrainingMetrics.WeightedRMSE, fold.ExternalMetrics.WeightedRMSE, fold.ExternalMetrics.N)
	}
	table.Flush()
	fmt.Printf("Общий взвешенный RMSE внешней проверки: %.3f м/год (%d точек)\n",
		result.PooledExternalMetrics.WeightedRMSE, result.PooledExternalMetrics.N)
	for _, warning := range result.Warnings {
		fmt.Printf("Предупреждение: %s\n", warning)
	}
	for _, skipped := range result.SkippedSites {
		fmt.Printf("Пропущен сайт %s: %s\n", skipped.SiteID, skipped.Reason)
	}

	path, err := writeCrossSiteValidationReport(result, config, benchOutput)
	if err != nil {
		return fmt.Errorf("сохранение отчёта межсайтовой проверки: %w", err)
	}
	fmt.Printf("Отчёт межсайтовой проверки сохранён в: %s\n", path)
	return nil
}

// writeCrossSiteValidationReport сохраняет воспроизводимый JSON-отчёт в output/metrics/.
func writeCrossSiteValidationReport(result benchmark.CrossSiteValidationResult, config benchmark.CalibrationConfig, requestedPath string) (string, error) {
	output := cli.NewOutputPathManager("")
	if err := output.EnsureDirectories(); err != nil {
		return "", err
	}
	report := struct {
		Config struct {
			MaxDistanceKm      float64 `json:"max_distance_km"`
			ValidationFraction float64 `json:"validation_fraction"`
			SpectrumSpreadDeg  float64 `json:"spectrum_spread_deg"`
			UsesBathymetry     bool    `json:"uses_bathymetry"`
		} `json:"config"`
		Result benchmark.CrossSiteValidationResult `json:"result"`
	}{Result: result}
	report.Config.MaxDistanceKm = config.MaxDistanceKm
	report.Config.ValidationFraction = config.ValidationFraction
	report.Config.SpectrumSpreadDeg = config.SpectrumSpreadDeg
	report.Config.UsesBathymetry = config.BathymetryGrid != nil
	path := output.MetricsPath("benchmark-cross-validation.json")
	if requestedPath != "" {
		path = output.ResolveUserPath(requestedPath, "metrics")
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("сериализация JSON: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("запись JSON: %w", err)
	}
	return path, nil
}

func runBenchmarkAnalyze(cmd *cobra.Command, args []string) error {
	if benchSiteID == "" {
		return fmt.Errorf("укажите ID сайта с помощью --site")
	}

	repo := benchmark.NewRepository(benchDir)
	site, err := repo.Load(benchSiteID)
	if err != nil {
		return fmt.Errorf("загрузка эталонного сайта %q: %w", benchSiteID, err)
	}

	fmt.Printf("Полный статистический анализ для: %s\n", site.Name)

	if benchBathymetry == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			benchBathymetry = "data/black-sea-bathymetry.json"
		}
	}

	var bathymetry *geometry.BathymetryGrid
	if benchBathymetry != "" {
		data, _ := os.ReadFile(benchBathymetry)
		grid, _ := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		bathymetry = grid
	}

	config := benchmark.DefaultCalibrationConfig()
	const bootstrapIter = 200

	var analysis benchmark.FullAnalysis
	if bathymetry != nil {
		analysis, err = benchmark.RunFullAnalysisWithBathymetry(*site, config, bathymetry, bootstrapIter)
	} else {
		analysis, err = benchmark.RunFullAnalysis(*site, config, bootstrapIter)
	}
	if err != nil {
		return fmt.Errorf("анализ не удался: %w", err)
	}

	best := analysis.BestFit
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  ПАРАМЕТРЫ, ВЫБРАННЫЕ НА ОБУЧАЮЩЕЙ ВЫБОРКЕ")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("  Сила эрозии:          %.2f м\n", best.ErosionStrength)
	fmt.Printf("  Направление волны:    %.1f°\n", best.WaveDirection)
	fmt.Printf("  Взвешенный RMSE обучения: %.3f м/год\n", best.TrainingMetrics.WeightedRMSE)
	if best.ValidationMetrics.N > 0 {
		fmt.Printf("  Взвешенный RMSE проверки: %.3f м/год\n", best.ValidationMetrics.WeightedRMSE)
	} else {
		fmt.Println("  Проверочная метрика недоступна: после отсева недостаточно точек.")
	}
	fmt.Println()

	if benchOutput != "" {
		data, _ := json.MarshalIndent(analysis, "", "  ")
		_ = os.WriteFile(benchOutput, data, 0o644)
		fmt.Printf("Полный анализ сохранён в %s\n", benchOutput)
	}

	return nil
}

func runBenchmarkHotspots(cmd *cobra.Command, args []string) error {
	if benchSiteID == "" {
		return fmt.Errorf("укажите ID сайта с помощью --site")
	}

	repo := benchmark.NewRepository(benchDir)
	site, err := repo.Load(benchSiteID)
	if err != nil {
		return fmt.Errorf("загрузка эталонного сайта %q: %w", benchSiteID, err)
	}

	if benchStrength <= 0 {
		benchStrength = 10.0
	}
	if benchWaveDir < 0 {
		benchWaveDir = site.MeanWaveDirection
	}

	if benchBathymetry == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			benchBathymetry = "data/black-sea-bathymetry.json"
		}
	}

	var bathymetry *geometry.BathymetryGrid
	if benchBathymetry != "" {
		data, _ := os.ReadFile(benchBathymetry)
		grid, _ := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		bathymetry = grid
	}

	config := benchmark.DefaultCalibrationConfig()
	config.BathymetryGrid = bathymetry

	fmt.Printf("Анализ горячих точек для: %s\n", site.Name)
	fmt.Printf("Параметры: сила=%.1f м, направление=%.0f°\n\n", benchStrength, benchWaveDir)

	rates := benchmark.SegmentRates(*site, config, benchStrength, benchWaveDir)
	hotspots := benchmark.FindHotspots(rates, site.Coastline, 5, 0.75)

	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  ТОП ГОРЯЧИХ ТОЧЕК ЭРОЗИИ")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("%-5s %-10s %-10s %-12s %-12s %-10s\n",
		"Ранг", "Шир", "Долг", "Средн (м/год)", "Макс (м/год)", "Длинакм")
	fmt.Println("───────────────────────────────────────────────────────────────────────")
	for _, h := range hotspots {
		fmt.Printf("%-5d %-10.4f %-10.4f %-12.2f %-12.2f %-10.2f\n",
			h.Rank, h.Center.Lat, h.Center.Lon,
			h.MeanRetreatRate, h.MaxRetreatRate, h.LengthKm)
	}
	fmt.Println()

	if benchOutput != "" {
		report := struct {
			SiteID    string              `json:"site_id"`
			Strength  float64             `json:"strength"`
			Direction float64             `json:"wave_direction"`
			Hotspots  []benchmark.Hotspot `json:"hotspots"`
		}{
			SiteID:    site.ID,
			Strength:  benchStrength,
			Direction: benchWaveDir,
			Hotspots:  hotspots,
		}
		data, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(benchOutput, data, 0o644)
		fmt.Printf("Отчёт о горячих точках сохранён в: %s\n", benchOutput)
	}

	return nil
}

func runBenchmarkScenarios(cmd *cobra.Command, args []string) error {
	if benchSiteID == "" {
		return fmt.Errorf("укажите ID сайта с помощью --site")
	}

	repo := benchmark.NewRepository(benchDir)
	site, err := repo.Load(benchSiteID)
	if err != nil {
		return fmt.Errorf("загрузка эталонного сайта %q: %w", benchSiteID, err)
	}

	if benchStrength <= 0 {
		benchStrength = 10.0
	}
	if benchWaveDir < 0 {
		benchWaveDir = site.MeanWaveDirection
	}

	if benchBathymetry == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			benchBathymetry = "data/black-sea-bathymetry.json"
		}
	}

	var bathymetry *geometry.BathymetryGrid
	if benchBathymetry != "" {
		data, _ := os.ReadFile(benchBathymetry)
		grid, _ := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		bathymetry = grid
	}

	fmt.Printf("Анализ сценариев для: %s\n", site.Name)
	fmt.Printf("Базовые параметры: сила=%.1f м, направление=%.0f°\n\n", benchStrength, benchWaveDir)

	scenarios := benchmark.DefaultScenarios(benchStrength, benchWaveDir)
	fmt.Printf("Выполнение %d сценариев...\n\n", len(scenarios))

	results, err := benchmark.RunScenarios(*site, scenarios, bathymetry)
	if err != nil {
		return fmt.Errorf("выполнение сценариев: %w", err)
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  РЕЗУЛЬТАТЫ СЦЕНАРИЕВ")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("%-15s %-8s %-8s %-8s %-8s %-10s %-8s\n",
		"Сценарий", "Средн", "Макс", "Эрод%", "ГорТч", "ЭродКм", "ΔДлинКм")
	fmt.Println("───────────────────────────────────────────────────────────────────────")
	for _, r := range results {
		fmt.Printf("%-15s %6.2f   %6.2f   %5.1f%%  %5d   %8.2f   %+6.2f\n",
			r.Config.Name, r.MeanRetreatRate, r.MaxRetreatRate,
			r.ErodingFraction*100, r.HotspotCount, r.TotalErodedKm, r.CoastChangeKm)
	}
	fmt.Println()

	if benchOutput != "" {
		report := struct {
			SiteID   string                     `json:"site_id"`
			Baseline benchmark.ScenarioResult   `json:"baseline"`
			Results  []benchmark.ScenarioResult `json:"results"`
		}{
			SiteID:   site.ID,
			Baseline: results[0],
			Results:  results,
		}
		data, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(benchOutput, data, 0o644)
		fmt.Printf("Отчёт о сценариях сохранён в: %s\n", benchOutput)
	}

	return nil
}

func runBenchmarkExtract(cmd *cobra.Command, args []string) error {
	bounds := coastline.GeoBounds{
		MinLat: boundsMinLat,
		MaxLat: boundsMaxLat,
		MinLon: boundsMinLon,
		MaxLon: boundsMaxLon,
	}

	if bounds.IsZero() {
		return fmt.Errorf("укажите границы с помощью --bounds-min-lat, --bounds-max-lat, --bounds-min-lon, --bounds-max-lon")
	}

	loadResult, err := coastline.Load(coastline.LoadOptions{
		RemoteURL: coastline.DefaultCoastlineGeoJSONURL,
	})
	if err != nil {
		return fmt.Errorf("загрузка береговой линии: %w", err)
	}

	segment := benchmark.ExtractCoastline(loadResult.Points, bounds)

	if len(segment) < 2 {
		return fmt.Errorf("в указанных границах не найдено точек береговой линии")
	}

	fmt.Printf("Загружено: %s (%d точек всего)\n", loadResult.Source, len(loadResult.Points))
	fmt.Printf("Извлечено %d точек из границ:\n", len(segment))
	fmt.Printf("  Широта: %.3f до %.3f\n", bounds.MinLat, bounds.MaxLat)
	fmt.Printf("  Долгота: %.3f до %.3f\n", bounds.MinLon, bounds.MaxLon)

	if benchOutput != "" {
		data, _ := json.MarshalIndent(segment, "", "  ")
		_ = os.WriteFile(benchOutput, data, 0o644)
		fmt.Printf("Сохранено в: %s\n", benchOutput)
	}

	return nil
}

func runBenchmarkCreate(cmd *cobra.Command, args []string) error {
	if siteName == "" {
		return fmt.Errorf("--name обязательно")
	}
	if siteBounds == "" {
		return fmt.Errorf("--bounds обязательно (формат: min_lat,max_lat,min_lon,max_lon)")
	}

	bounds, err := benchmark.ParseBounds(siteBounds)
	if err != nil {
		return fmt.Errorf("разбор границ: %w", err)
	}

	coastType, err := benchmark.PresetCoastType(siteCoastType)
	if err != nil {
		return err
	}
	quality, err := benchmark.PresetQuality(siteQuality)
	if err != nil {
		return err
	}

	spec := benchmark.SiteSpec{
		Name:              siteName,
		Region:            siteRegion,
		Country:           siteCountry,
		Description:       siteDescription,
		Bounds:            bounds,
		CoastType:         coastType,
		DominantLithology: siteLithology,
		MeanWaveHeight:    siteWaveHeight,
		MeanWavePeriod:    siteWavePeriod,
		MeanWaveDirection: siteWaveDir,
		DataQuality:       quality,
		ObservationYears: benchmark.Range{
			Min: float64(siteObsYearMin),
			Max: float64(siteObsYearMax),
		},
	}

	if err := spec.Validate(); err != nil {
		return fmt.Errorf("недопустимая спецификация: %w", err)
	}

	fmt.Printf("Создание эталонного сайта: %s (id=%s)\n", spec.Name, spec.ID)

	var fullCoastline []geometry.LatLon
	if siteCoastline != "" {
		data, err := os.ReadFile(siteCoastline)
		if err == nil {
			pts, _, err := coastline.LoadFromJSON(string(data))
			if err == nil {
				fullCoastline = pts
				fmt.Printf("Загружена береговая линия: %s (%d точек)\n", siteCoastline, len(pts))
			}
		}
	}
	if fullCoastline == nil {
		loadResult, err := coastline.Load(coastline.LoadOptions{
			RemoteURL: coastline.DefaultCoastlineGeoJSONURL,
		})
		if err == nil {
			fullCoastline = loadResult.Points
			fmt.Printf("Загружена береговая линия: %s (%d точек)\n", loadResult.Source, len(loadResult.Points))
		}
	}

	site := spec.Build(fullCoastline)
	fmt.Printf("Извлечено %d точек береговой линии в границах\n", len(site.Coastline))

	repo := benchmark.NewRepository(benchDir)
	if err := repo.Save(site); err != nil {
		return fmt.Errorf("сохранение сайта: %w", err)
	}

	fmt.Println("\n✓ Сайт создан!")
	fmt.Printf("  Сохранено в: %s/%s.json\n", benchDir, site.ID)
	fmt.Println("\nСледующие шаги:")
	fmt.Printf("  lito benchmark show --site=%s\n", site.ID)
	fmt.Printf("  lito benchmark calibrate --site=%s\n", site.ID)

	return nil
}
