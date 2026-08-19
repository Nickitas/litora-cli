package cobra

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"

	"github.com/spf13/cobra"
)

var (
	cercCalibrationInput                  string
	cercCalibrationBathymetry             string
	cercCalibrationBathymetryResolution   float64
	cercCalibrationWaveInput              string
	cercCalibrationWaveSource             string
	cercCalibrationObservations           string
	cercCalibrationCoefficients           string
	cercCalibrationMaxDistance            float64
	cercCalibrationMinWaveHours           float64
	cercCalibrationOutput                 string
	cercCalibrationWaterbody              string
	cercCalibrationBreakingIndex          float64
	cercCalibrationBermHeight             float64
	cercCalibrationClosureDepth           float64
	cercCalibrationPorosity               float64
	cercCalibrationOffshoreDistance       float64
	cercCalibrationMaxBathymetryGap       float64
	cercCalibrationLeftBoundaryTransport  float64
	cercCalibrationRightBoundaryTransport float64
	cercCalibrationSedimentSources        string
	cercCalibrationStructures             string
)

// cercCalibrationCmd калибрует только параметры физической CERC-модели по
// независимым годовым скоростям смещения берега.
var cercCalibrationCmd = &cobra.Command{
	Use:   "calibrate-cerc",
	Short: "Калибровать коэффициент CERC по наблюдениям береговой линии",
	Long: `Подбирает коэффициент CERC по независимым наблюдаемым годовым скоростям.
Обязательны локальный сегмент, батиметрия с фактическим шагом, волновой ряд не
короче репрезентативного года и файл наблюдений. Краткий прогноз волн для
калибровки намеренно не принимается.`,
	RunE: runCERCCalibration,
}

func init() {
	rootCmd.AddCommand(cercCalibrationCmd)
	cercCalibrationCmd.Flags().StringVar(&cercCalibrationInput, "input", "", "путь к локальному JSON/GeoJSON береговой линии (обязателен)")
	cercCalibrationCmd.Flags().StringVar(&cercCalibrationBathymetry, "bathymetry", "", "путь к JSON батиметрии (обязателен)")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationBathymetryResolution, "bathymetry-resolution", 0, "фактический шаг регулярной батиметрической сетки в градусах (обязателен)")
	cercCalibrationCmd.Flags().StringVar(&cercCalibrationWaveInput, "wave-input", "", "путь к фактическому волновому ряду (обязателен)")
	cercCalibrationCmd.Flags().StringVar(&cercCalibrationWaveSource, "wave-source", "", "происхождение волнового ряда, если оно отсутствует в JSON")
	cercCalibrationCmd.Flags().StringVar(&cercCalibrationObservations, "observations", "", "JSON наблюдаемых годовых скоростей береговой линии (обязателен)")
	cercCalibrationCmd.Flags().StringVar(&cercCalibrationCoefficients, "cerc-coefficients", "0.2,0.3,0.39,0.5,0.6", "проверяемые коэффициенты CERC через запятую")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationMaxDistance, "max-observation-distance", 500, "максимальная дистанция наблюдения до сегмента, м")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationMinWaveHours, "min-wave-hours", 8766, "минимальное покрытие волнового ряда для годовой калибровки, ч")
	cercCalibrationCmd.Flags().StringVar(&cercCalibrationWaterbody, "waterbody", "", "водоём РФ из lito waterbody list")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationBreakingIndex, "breaking-index", 0.78, "индекс разрушения H_b/h_b")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationBermHeight, "berm-height", 2, "высота бермы активного профиля, м")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationClosureDepth, "closure-depth", 8, "глубина замыкания активного профиля, м")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationPorosity, "porosity", 0.4, "пористость наносов")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationOffshoreDistance, "offshore-sample-distance", 300, "расстояние отбора глубины от берега, м")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationMaxBathymetryGap, "max-bathymetry-gap", 1500, "максимальная дистанция до реальной точки глубины, м")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationLeftBoundaryTransport, "left-boundary-transport", 0, "поток через левую границу, м³/с; положительный направлен внутрь сегмента")
	cercCalibrationCmd.Flags().Float64Var(&cercCalibrationRightBoundaryTransport, "right-boundary-transport", 0, "поток через правую границу, м³/с; положительный направлен из сегмента")
	cercCalibrationCmd.Flags().StringVar(&cercCalibrationSedimentSources, "sediment-sources", "", "JSON внешних источников и стоков наносов по ячейкам")
	cercCalibrationCmd.Flags().StringVar(&cercCalibrationStructures, "structures", "", "JSON сооружений, изменяющих пропуск потока между ячейками")
	cercCalibrationCmd.Flags().StringVar(&cercCalibrationOutput, "output", "", "каталог для вывода (по умолчанию: ./output)")
}

func runCERCCalibration(cmd *cobra.Command, args []string) error {
	if cercCalibrationWaterbody != "" {
		body, err := selectedWaterbody(cercCalibrationWaterbody)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Выбран водоём: %s\n", body.Name)
	}
	for _, required := range []struct {
		name  string
		value string
	}{
		{"--input", cercCalibrationInput},
		{"--bathymetry", cercCalibrationBathymetry},
		{"--wave-input", cercCalibrationWaveInput},
		{"--observations", cercCalibrationObservations},
	} {
		if required.value == "" {
			return fmt.Errorf("для калибровки обязателен %s", required.name)
		}
	}
	if cercCalibrationBathymetryResolution <= 0 {
		return fmt.Errorf("для калибровки обязателен фактический --bathymetry-resolution")
	}
	coefficients, err := parseCERCCoefficients(cercCalibrationCoefficients)
	if err != nil {
		return err
	}
	coast, err := coastline.Load(coastline.LoadOptions{LocalPath: cercCalibrationInput})
	if err != nil {
		return fmt.Errorf("загрузка береговой линии: %w", err)
	}
	inputs, err := loadModelInputs(cercCalibrationBathymetry, "", false, cercCalibrationBathymetryResolution)
	if err != nil {
		return err
	}
	printModelInputWarnings(inputs)
	climate, err := geometry.LoadWaveClimate(cercCalibrationWaveInput, cercCalibrationWaveSource)
	if err != nil {
		return err
	}
	observations, err := geometry.LoadShorelineRateObservations(cercCalibrationObservations)
	if err != nil {
		return err
	}
	sedimentSources, err := loadLongshoreSedimentSources(cercCalibrationSedimentSources)
	if err != nil {
		return err
	}
	structures, err := loadLongshoreStructures(cercCalibrationStructures)
	if err != nil {
		return err
	}
	modelConfig := geometry.LongshoreModelConfig{
		Bathymetry:                inputs.BathymetryGrid,
		BathymetrySource:          inputs.BathymetryPath,
		BathymetrySHA256:          inputs.BathymetrySHA256,
		BathymetryPassport:        inputs.BathymetryPassportPath,
		BathymetryStatus:          inputs.BathymetryStatus,
		WaterbodyID:               cercCalibrationWaterbody,
		SedimentSources:           sedimentSources,
		Structures:                structures,
		BreakingIndex:             cercCalibrationBreakingIndex,
		BermHeightMeters:          cercCalibrationBermHeight,
		ClosureDepthMeters:        cercCalibrationClosureDepth,
		Porosity:                  cercCalibrationPorosity,
		OffshoreSampleDistanceM:   cercCalibrationOffshoreDistance,
		MaxBathymetryGapMeters:    cercCalibrationMaxBathymetryGap,
		LeftBoundaryTransportM3S:  cercCalibrationLeftBoundaryTransport,
		RightBoundaryTransportM3S: cercCalibrationRightBoundaryTransport,
	}
	results, err := geometry.CalibrateCERCCoefficient(coast.Points, climate, observations, geometry.CERCCalibrationConfig{
		Model:                modelConfig,
		CERCCoefficients:     coefficients,
		MaxDistanceMeters:    cercCalibrationMaxDistance,
		MinWaveDurationHours: cercCalibrationMinWaveHours,
	})
	if err != nil {
		return fmt.Errorf("калибровка CERC: %w", err)
	}
	printCERCCalibrationResults(results)
	return writeCERCCalibrationReport(cli.NewOutputPathManager(cercCalibrationOutput), coast.Source, climate.Source, cercCalibrationObservations, modelConfig, cercCalibrationBathymetryResolution, results)
}

func parseCERCCoefficients(raw string) ([]float64, error) {
	parts := strings.Split(raw, ",")
	coefficients := make([]float64, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || value <= 0 || value > 1 {
			return nil, fmt.Errorf("некорректный коэффициент CERC %q; ожидается значение в диапазоне (0; 1]", part)
		}
		coefficients = append(coefficients, value)
	}
	return coefficients, nil
}

func printCERCCalibrationResults(results []geometry.CERCCalibrationResult) {
	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "Ранг\tКоэффициент CERC\tRMSE, м/год\tMAE, м/год\tMBE, м/год\tR²\tТочек")
	fmt.Fprintln(table, "----\t----------------\t-----------\t----------\t----------\t--\t------")
	for i, result := range results {
		fmt.Fprintf(table, "%d\t%.4f\t%.4f\t%.4f\t%.4f\t%.4f\t%d\n", i+1, result.CERCCoefficient, result.Metrics.RMSEMPerYear, result.Metrics.MAEMPerYear, result.Metrics.MBEMPerYear, result.Metrics.RSquared, result.Metrics.Count)
	}
	table.Flush()
}

func writeCERCCalibrationReport(output *cli.OutputPathManager, coastlineSource, waveSource, observationsSource string, model geometry.LongshoreModelConfig, bathymetryResolution float64, results []geometry.CERCCalibrationResult) error {
	if err := output.EnsureDirectories(); err != nil {
		return fmt.Errorf("подготовка каталогов вывода: %w", err)
	}
	report := struct {
		CoastlineSource        string                             `json:"coastline_source"`
		WaveSource             string                             `json:"wave_source"`
		ObservationsSource     string                             `json:"observations_source"`
		WaterbodyID            string                             `json:"waterbody_id,omitempty"`
		BathymetrySource       string                             `json:"bathymetry_source"`
		BathymetrySHA256       string                             `json:"bathymetry_sha256"`
		BathymetryPassport     string                             `json:"bathymetry_passport,omitempty"`
		BathymetryStatus       string                             `json:"bathymetry_status"`
		BathymetryResolution   float64                            `json:"bathymetry_resolution_degrees"`
		BreakingIndex          float64                            `json:"breaking_index"`
		BermHeightMeters       float64                            `json:"berm_height_meters"`
		ClosureDepthMeters     float64                            `json:"closure_depth_meters"`
		Porosity               float64                            `json:"porosity"`
		LeftBoundaryTransport  float64                            `json:"left_boundary_transport_m3_s"`
		RightBoundaryTransport float64                            `json:"right_boundary_transport_m3_s"`
		SedimentSources        []geometry.LongshoreSedimentSource `json:"sediment_sources,omitempty"`
		Structures             []geometry.LongshoreStructure      `json:"structures,omitempty"`
		Results                []geometry.CERCCalibrationResult   `json:"results"`
	}{
		CoastlineSource: coastlineSource, WaveSource: waveSource, ObservationsSource: observationsSource,
		WaterbodyID: model.WaterbodyID, BathymetrySource: model.BathymetrySource,
		BathymetrySHA256: model.BathymetrySHA256, BathymetryPassport: model.BathymetryPassport,
		BathymetryStatus: model.BathymetryStatus, BathymetryResolution: bathymetryResolution,
		BreakingIndex: model.BreakingIndex, BermHeightMeters: model.BermHeightMeters, ClosureDepthMeters: model.ClosureDepthMeters,
		Porosity: model.Porosity, LeftBoundaryTransport: model.LeftBoundaryTransportM3S, RightBoundaryTransport: model.RightBoundaryTransportM3S,
		SedimentSources: model.SedimentSources,
		Structures:      model.Structures,
		Results:         results,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("сериализация отчёта калибровки: %w", err)
	}
	path := output.ErosionPath("cerc-calibration.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта калибровки: %w", err)
	}
	fmt.Printf("✓ Отчёт калибровки сохранён: %s\n", path)
	return nil
}
