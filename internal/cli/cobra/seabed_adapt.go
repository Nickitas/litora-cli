package cobra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	adaptivemodel "coastal-geometry/internal/domain/adaptive"
	"coastal-geometry/internal/domain/seabed"
	adaptiverender "coastal-geometry/internal/render/adaptive"

	"github.com/spf13/cobra"
)

var (
	seabedAdaptInput              string
	seabedAdaptOutput             string
	seabedAdaptSourceMetadata     string
	seabedAdaptSource             string
	seabedAdaptMinSize            float64
	seabedAdaptCoastSize          float64
	seabedAdaptShelfSize          float64
	seabedAdaptDeepSize           float64
	seabedAdaptCoastInfluence     float64
	seabedAdaptCurvatureReference float64
	seabedAdaptSlopeReference     float64
	seabedAdaptFlatDeepSlope      float64
	seabedAdaptMaxNeighbourRatio  float64
	seabedAdaptMaxSizeGradient    float64
)

var seabedAdaptCmd = &cobra.Command{
	Use:   "adapt",
	Short: "Построить поле требуемого размера ячейки",
	Long: `Читает принятую батиметрическую модель Чёрного моря и рассчитывает
воспроизводимое узловое поле h(x,y) для будущей адаптивной четырёхугольной
сетки. Размер объясняется расстоянием до берега и островов, локальным поворотом
береговой линии и градиентом глубины. Переходы ограничиваются по фактическим
рёбрам исходного каркаса. Команда сохраняет CSV, JSON и SVG, но не запускает
Gmsh: построение новой сетки относится к ADAPT-02.`,
	RunE: runSeabedAdapt,
}

func init() {
	seabedCmd.AddCommand(seabedAdaptCmd)
	defaults := adaptivemodel.DefaultConfig()
	seabedAdaptCmd.Flags().StringVar(&seabedAdaptInput, "input", "", "батиметрический MSH; по умолчанию output/seabed/black-sea-depth.msh")
	seabedAdaptCmd.Flags().StringVar(&seabedAdaptOutput, "output", "output", "корневой каталог результатов")
	seabedAdaptCmd.Flags().StringVar(&seabedAdaptSourceMetadata, "source-metadata", "data/black-sea-bathymetry-source.metadata.json", "паспорт исходного набора батиметрии")
	seabedAdaptCmd.Flags().StringVar(&seabedAdaptSource, "source", "", "явная атрибуция источника вместо --source-metadata")
	seabedAdaptCmd.Flags().Float64Var(&seabedAdaptMinSize, "min-size", defaults.MinSizeM, "минимальная целевая длина ребра, м")
	seabedAdaptCmd.Flags().Float64Var(&seabedAdaptCoastSize, "coast-size", defaults.StraightCoastSizeM, "размер у прямолинейного берега, м")
	seabedAdaptCmd.Flags().Float64Var(&seabedAdaptShelfSize, "shelf-size", defaults.ShelfSizeM, "базовый размер на шельфе, м")
	seabedAdaptCmd.Flags().Float64Var(&seabedAdaptDeepSize, "deep-size", defaults.DeepSizeM, "размер на ровном глубоководье, м")
	seabedAdaptCmd.Flags().Float64Var(&seabedAdaptCoastInfluence, "coast-influence", defaults.CoastInfluenceM, "дальность экспоненциального влияния берега, м")
	seabedAdaptCmd.Flags().Float64Var(&seabedAdaptCurvatureReference, "curvature-reference", defaults.CurvatureReferenceDeg, "поворот берега, соответствующий максимальному уточнению, градусы")
	seabedAdaptCmd.Flags().Float64Var(&seabedAdaptSlopeReference, "slope-reference", defaults.SlopeReferenceDeg, "уклон дна, соответствующий максимальному уточнению, градусы")
	seabedAdaptCmd.Flags().Float64Var(&seabedAdaptFlatDeepSlope, "flat-deep-slope", defaults.FlatDeepMaxSlopeDeg, "максимальный уклон ровного глубоководья, градусы")
	seabedAdaptCmd.Flags().Float64Var(&seabedAdaptMaxNeighbourRatio, "max-neighbour-ratio", defaults.MaxNeighbourRatio, "максимальное отношение размеров в соседних узлах")
	seabedAdaptCmd.Flags().Float64Var(&seabedAdaptMaxSizeGradient, "max-size-gradient", defaults.MaxSizeGradientPerM, "максимальный рост размера на метр расстояния, м/м")
}

func runSeabedAdapt(_ *cobra.Command, _ []string) error {
	inputPath := strings.TrimSpace(seabedAdaptInput)
	if inputPath == "" {
		inputPath = filepath.Join(seabedAdaptOutput, "seabed", "black-sea-depth.msh")
	}
	document, err := seabed.ReadMSH2(inputPath)
	if err != nil {
		return fmt.Errorf("чтение модели дна для ADAPT-01: %w", err)
	}
	if document.Metadata.ModelKind != seabed.MSHModelSeabed || document.Metadata.SchemaVersion != seabed.SeabedMSHSchemaVersion {
		return fmt.Errorf("файл %q не является батиметрической моделью %s", inputPath, seabed.SeabedMSHSchemaVersion)
	}
	config := adaptivemodel.Config{
		MinSizeM: seabedAdaptMinSize, StraightCoastSizeM: seabedAdaptCoastSize,
		ShelfSizeM: seabedAdaptShelfSize, DeepSizeM: seabedAdaptDeepSize,
		CoastInfluenceM: seabedAdaptCoastInfluence, CurvatureReferenceDeg: seabedAdaptCurvatureReference,
		SlopeReferenceDeg: seabedAdaptSlopeReference, FlatDeepMaxSlopeDeg: seabedAdaptFlatDeepSlope,
		MaxNeighbourRatio: seabedAdaptMaxNeighbourRatio, MaxSizeGradientPerM: seabedAdaptMaxSizeGradient,
	}
	field, err := adaptivemodel.BuildSizeField(document.Model, config)
	if err != nil {
		return err
	}
	source, checksum, _, err := resolveBathymetryAttribution(seabedAdaptSource, seabedAdaptSourceMetadata)
	if err != nil {
		return err
	}
	adaptiveDir := filepath.Join(seabedAdaptOutput, "seabed", "adaptive")
	fieldCSVPath := filepath.Join(adaptiveDir, "size-field.csv")
	reportPath := filepath.Join(adaptiveDir, "size-field.json")
	mapPath := filepath.Join(seabedAdaptOutput, "seabed", "svg", "size-field.svg")
	field.Report.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	field.Report.InputMSH = inputPath
	field.Report.FieldCSV = fieldCSVPath
	field.Report.MapSVG = mapPath
	field.Report.BathymetrySource = source
	field.Report.BathymetrySHA256 = checksum
	if err := adaptivemodel.WriteFieldCSV(fieldCSVPath, field); err != nil {
		return err
	}
	if _, err := adaptiverender.WriteSizeFieldSVG(mapPath, document.Model, field, adaptiverender.SizeFieldMapConfig{
		Title: "Поле требуемого размера ячейки Чёрного моря", Source: source,
	}); err != nil {
		return err
	}
	if err := adaptivemodel.WriteReportJSON(reportPath, field.Report); err != nil {
		return err
	}
	if quiet {
		return nil
	}

	fmt.Printf("Поле размера: %s\n", fieldCSVPath)
	fmt.Printf("Отчёт ADAPT-01: %s\n", reportPath)
	fmt.Printf("Карта поля: %s\n", mapPath)
	summary := field.Report.Summary
	fmt.Printf("Размер h: %.0f–%.0f м; медиана %.0f м; скорректировано ростом %d из %d узлов\n",
		summary.Target.MinM, summary.Target.MaxM, summary.Target.MedianM,
		summary.GrowthLimitedNodeCount, summary.NodeCount)
	fmt.Printf("Плавность: соседнее отношение %.3f ≤ %.3f; градиент %.4f ≤ %.4f м/м\n",
		summary.FinalMaxAdjacentRatio, config.MaxNeighbourRatio,
		summary.FinalMaxSizeGradientPerM, config.MaxSizeGradientPerM)
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Зона\tУзлов\tМинимум, м\tСреднее, м\tМаксимум, м")
	fmt.Fprintln(writer, "----\t-----\t----------\t----------\t-----------")
	for _, zone := range summary.Zones {
		fmt.Fprintf(writer, "%s\t%d\t%.0f\t%.0f\t%.0f\n", zone.Name, zone.NodeCount, zone.Target.MinM, zone.Target.MeanM, zone.Target.MaxM)
	}
	return writer.Flush()
}
