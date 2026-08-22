package cobra

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	seabedmodel "coastal-geometry/internal/domain/seabed"
	bathymetryrender "coastal-geometry/internal/render/bathymetry"

	"github.com/spf13/cobra"
)

var (
	seabedRenderInput          string
	seabedRenderMetadata       string
	seabedRenderSourceMetadata string
	seabedRenderSource         string
	seabedRenderOutput         string
	seabedRenderIsobaths       string
)

var seabedCmd = &cobra.Command{
	Use:   "seabed",
	Short: "Работать с батиметрической моделью Чёрного моря",
	Long: `Команды подготовки, проверки и визуализации рельефа дна Чёрного моря.
На текущем этапе доступно повторное построение обзорной карты из принятого
MSH lito-seabed/v1 без пересчёта глубин.`,
}

var seabedRenderCmd = &cobra.Command{
	Use:   "render",
	Short: "Построить обзорную батиметрическую карту",
	Long: `Читает батиметрический MSH и паспорт EXPORT-02, затем строит обзорную
SVG-карту с непрерывной цветовой шкалой глубины, реальными изобатами,
береговой линией, масштабом, источником, вертикальной системой и долей NoData.
Внутренние рёбра сетки на обзорной карте не рисуются.`,
	RunE: runSeabedRender,
}

func init() {
	rootCmd.AddCommand(seabedCmd)
	seabedCmd.AddCommand(seabedRenderCmd)

	seabedRenderCmd.Flags().StringVar(&seabedRenderInput, "input", "", "батиметрический MSH; по умолчанию output/seabed/black-sea-depth.msh")
	seabedRenderCmd.Flags().StringVar(&seabedRenderMetadata, "metadata", "", "паспорт EXPORT-02; по умолчанию рядом с входным MSH")
	seabedRenderCmd.Flags().StringVar(&seabedRenderSourceMetadata, "source-metadata", "data/black-sea-bathymetry-source.metadata.json", "паспорт исходного набора батиметрии")
	seabedRenderCmd.Flags().StringVar(&seabedRenderSource, "source", "", "явная атрибуция источника вместо --source-metadata")
	seabedRenderCmd.Flags().StringVar(&seabedRenderOutput, "output", "output", "корневой каталог результатов; карта сохраняется в seabed/svg")
	seabedRenderCmd.Flags().StringVar(&seabedRenderIsobaths, "isobaths", "20,50,100,200,500,1000,1500,2000", "положительные глубины изобат в метрах через запятую")
}

type bathymetrySourcePassport struct {
	Attribution   string `json:"attribution"`
	DatasetSHA256 string `json:"dataset_sha256"`
}

type seabedRenderRunReport struct {
	SchemaVersion     string                          `json:"schema_version"`
	GeneratedAt       string                          `json:"generated_at"`
	InputMSH          string                          `json:"input_msh"`
	ExportMetadata    string                          `json:"export_metadata"`
	SourceMetadata    string                          `json:"source_metadata,omitempty"`
	SVG               string                          `json:"svg"`
	Source            string                          `json:"source"`
	VerticalReference string                          `json:"vertical_reference"`
	VerticalCaveat    string                          `json:"vertical_caveat"`
	Overview          bathymetryrender.OverviewReport `json:"overview"`
}

func runSeabedRender(_ *cobra.Command, _ []string) error {
	inputPath := strings.TrimSpace(seabedRenderInput)
	if inputPath == "" {
		inputPath = filepath.Join(seabedRenderOutput, "seabed", "black-sea-depth.msh")
	}
	metadataPath := strings.TrimSpace(seabedRenderMetadata)
	if metadataPath == "" {
		metadataPath = filepath.Join(filepath.Dir(inputPath), "export-metadata.json")
	}
	isobaths, err := parsePositiveFloatList(seabedRenderIsobaths, "глубины изобат")
	if err != nil {
		return err
	}

	document, err := seabedmodel.ReadMSH2(inputPath)
	if err != nil {
		return fmt.Errorf("чтение модели дна: %w", err)
	}
	if document.Metadata.ModelKind != seabedmodel.MSHModelSeabed || document.Metadata.SchemaVersion != seabedmodel.SeabedMSHSchemaVersion {
		return fmt.Errorf("файл %q не является батиметрической моделью %s", inputPath, seabedmodel.SeabedMSHSchemaVersion)
	}
	metadata, err := seabedmodel.ReadExportMetadataJSON(metadataPath)
	if err != nil {
		return err
	}
	if metadata.RegionThresholds != document.Model.CellDerivation.RegionThresholds {
		return fmt.Errorf("пороги морфометрических зон MSH и паспорта экспорта не совпадают")
	}

	source, checksum, sourceMetadataPath, err := resolveBathymetryAttribution()
	if err != nil {
		return err
	}
	outputDir := filepath.Join(seabedRenderOutput, "seabed")
	svgPath := filepath.Join(outputDir, "svg", "bathymetry-overview.svg")
	overview, err := bathymetryrender.WriteOverviewSVG(svgPath, document.Model, bathymetryrender.OverviewConfig{
		Title: "Батиметрия Чёрного моря", Source: source, SourceChecksum: checksum,
		Metadata: metadata, IsobathsM: isobaths,
	})
	if err != nil {
		return err
	}

	runReport := seabedRenderRunReport{
		SchemaVersion: "lito-bathymetry-overview/v1", GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		InputMSH: inputPath, ExportMetadata: metadataPath, SourceMetadata: sourceMetadataPath,
		SVG: svgPath, Source: source, VerticalReference: metadata.VerticalReference,
		VerticalCaveat: metadata.VerticalCaveat, Overview: overview,
	}
	reportPath := filepath.Join(outputDir, "bathymetry-overview.json")
	if err := writeSeabedRenderReport(reportPath, runReport); err != nil {
		return err
	}

	if !quiet {
		fmt.Printf("Обзорная батиметрическая карта: %s\n", svgPath)
		fmt.Printf("Отчёт построения: %s\n", reportPath)
		fmt.Printf("Глубина: 0–%.1f м; изобат: %d; NoData узлов: %.2f%%; NoData ячеек: %.2f%%\n",
			overview.MaxDepthM, len(overview.RenderedIsobathsM), overview.NoDataNodePercent, overview.NoDataCellPercent)
	}
	return nil
}

func resolveBathymetryAttribution() (source, checksum, metadataPath string, err error) {
	if explicit := strings.TrimSpace(seabedRenderSource); explicit != "" {
		return explicit, "", "", nil
	}
	metadataPath = strings.TrimSpace(seabedRenderSourceMetadata)
	if metadataPath == "" {
		return "", "", "", fmt.Errorf("нужно задать --source или --source-metadata")
	}
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return "", "", "", fmt.Errorf("чтение паспорта источника %q: %w", metadataPath, err)
	}
	var passport bathymetrySourcePassport
	if err := json.Unmarshal(data, &passport); err != nil {
		return "", "", "", fmt.Errorf("разбор паспорта источника %q: %w", metadataPath, err)
	}
	if strings.TrimSpace(passport.Attribution) == "" || strings.TrimSpace(passport.DatasetSHA256) == "" {
		return "", "", "", fmt.Errorf("паспорт источника %q не содержит атрибуцию или SHA-256", metadataPath)
	}
	return passport.Attribution, passport.DatasetSHA256, metadataPath, nil
}

func writeSeabedRenderReport(path string, report seabedRenderRunReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта обзорной карты: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога отчёта обзорной карты: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта обзорной карты %q: %w", path, err)
	}
	return nil
}
