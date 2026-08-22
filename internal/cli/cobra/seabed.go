package cobra

import (
	"encoding/json"
	"fmt"
	"math"
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
	seabedRenderVerticalScale  float64
	seabedRenderControlPoints  bool
)

var seabedCmd = &cobra.Command{
	Use:   "seabed",
	Short: "Работать с батиметрической моделью Чёрного моря",
	Long: `Команды подготовки, проверки и визуализации рельефа дна Чёрного моря.
Доступно повторное построение обзорной карты, увеличенных фрагментов,
3D-рельефа и профилей из принятого MSH lito-seabed/v1 без пересчёта глубин.`,
}

var seabedRenderCmd = &cobra.Command{
	Use:   "render",
	Short: "Построить карты, 3D-рельеф и профили",
	Long: `Читает батиметрический MSH и паспорт EXPORT-02, затем строит обзорную
SVG-карту с непрерывной цветовой шкалой глубины, реальными изобатами,
береговой линией, масштабом, источником, вертикальной системой и долей NoData.
Внутренние рёбра на обзорной карте не рисуются. Второй SVG показывает три
непрореженных фрагмента сетки. Дополнительно создаются 3D-рельеф с явно
подписанным вертикальным преувеличением и три профиля «берег → глубоководье».`,
	RunE: runSeabedRender,
}

func init() {
	rootCmd.AddCommand(seabedCmd)
	seabedCmd.AddCommand(seabedRenderCmd)

	seabedRenderCmd.Flags().StringVar(&seabedRenderInput, "input", "", "батиметрический MSH; по умолчанию output/seabed/black-sea-depth.msh")
	seabedRenderCmd.Flags().StringVar(&seabedRenderMetadata, "metadata", "", "паспорт EXPORT-02; по умолчанию рядом с входным MSH")
	seabedRenderCmd.Flags().StringVar(&seabedRenderSourceMetadata, "source-metadata", "data/black-sea-bathymetry-source.metadata.json", "паспорт исходного набора батиметрии")
	seabedRenderCmd.Flags().StringVar(&seabedRenderSource, "source", "", "явная атрибуция источника вместо --source-metadata")
	seabedRenderCmd.Flags().StringVar(&seabedRenderOutput, "output", "output", "корневой каталог результатов; карты сохраняются в seabed/svg")
	seabedRenderCmd.Flags().StringVar(&seabedRenderIsobaths, "isobaths", "20,50,100,200,500,1000,1500,2000", "положительные глубины изобат в метрах через запятую")
	seabedRenderCmd.Flags().Float64Var(&seabedRenderVerticalScale, "vertical-exaggeration", 40, "вертикальное преувеличение 3D-вида; 1 сохраняет натуральный метрический масштаб")
	seabedRenderCmd.Flags().BoolVar(&seabedRenderControlPoints, "control-points", true, "показывать на 3D-поверхности узловые выборки глубины")
}

type bathymetrySourcePassport struct {
	Attribution   string `json:"attribution"`
	DatasetSHA256 string `json:"dataset_sha256"`
}

type seabedRenderRunReport struct {
	SchemaVersion     string                             `json:"schema_version"`
	GeneratedAt       string                             `json:"generated_at"`
	InputMSH          string                             `json:"input_msh"`
	ExportMetadata    string                             `json:"export_metadata"`
	SourceMetadata    string                             `json:"source_metadata,omitempty"`
	OverviewSVG       string                             `json:"overview_svg"`
	MeshDetailsSVG    string                             `json:"mesh_details_svg"`
	Relief3DSVG       string                             `json:"relief_3d_svg"`
	ProfilesSVG       string                             `json:"profiles_svg"`
	ProfilesCSV       string                             `json:"profiles_csv"`
	Source            string                             `json:"source"`
	VerticalReference string                             `json:"vertical_reference"`
	VerticalCaveat    string                             `json:"vertical_caveat"`
	Overview          bathymetryrender.OverviewReport    `json:"overview"`
	MeshDetails       bathymetryrender.MeshDetailsReport `json:"mesh_details"`
	Relief3D          bathymetryrender.ReliefReport      `json:"relief_3d"`
	Profiles          bathymetryrender.ProfilesReport    `json:"profiles"`
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
	if math.IsNaN(seabedRenderVerticalScale) || math.IsInf(seabedRenderVerticalScale, 0) || seabedRenderVerticalScale < 1 {
		return fmt.Errorf("--vertical-exaggeration должно быть конечным числом не меньше 1")
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
	profiles, profileSelections, err := seabedmodel.SelectCoastToDeepProfiles(document.Model)
	if err != nil {
		return err
	}
	overviewSVGPath := filepath.Join(outputDir, "svg", "bathymetry-overview.svg")
	overview, err := bathymetryrender.WriteOverviewSVG(overviewSVGPath, document.Model, bathymetryrender.OverviewConfig{
		Title: "Батиметрия Чёрного моря", Source: source, SourceChecksum: checksum,
		Metadata: metadata, IsobathsM: isobaths,
	})
	if err != nil {
		return err
	}
	meshDetailsSVGPath := filepath.Join(outputDir, "svg", "mesh-details.svg")
	meshDetails, err := bathymetryrender.WriteMeshDetailsSVG(meshDetailsSVGPath, document.Model, bathymetryrender.MeshDetailsConfig{
		Title: "Фактическая сетка Чёрного моря: контрольные фрагменты", Source: source, SourceChecksum: checksum,
		Metadata: metadata, IsobathsM: isobaths,
	})
	if err != nil {
		return err
	}
	profilesCSVPath := filepath.Join(outputDir, "profiles.csv")
	if err := seabedmodel.WriteProfilesCSV(profilesCSVPath, document.Model, profiles, metadata); err != nil {
		return err
	}
	relief3DSVGPath := filepath.Join(outputDir, "svg", "seabed-3d.svg")
	relief3D, err := bathymetryrender.WriteReliefSVG(relief3DSVGPath, document.Model, bathymetryrender.ReliefConfig{
		Title: "3D-рельеф дна Чёрного моря", Source: source, SourceChecksum: checksum,
		Metadata: metadata, VerticalExaggeration: seabedRenderVerticalScale,
		ControlPoints: seabedRenderControlPoints, Profiles: profiles,
	})
	if err != nil {
		return err
	}
	profilesSVGPath := filepath.Join(outputDir, "svg", "profiles.svg")
	profilesReport, err := bathymetryrender.WriteProfilesSVG(profilesSVGPath, document.Model, bathymetryrender.ProfilesConfig{
		Title: "Профили рельефа дна Чёрного моря", Source: source, SourceChecksum: checksum,
		Metadata: metadata, Profiles: profiles, SelectionReports: profileSelections,
	})
	if err != nil {
		return err
	}

	runReport := seabedRenderRunReport{
		SchemaVersion: "lito-bathymetry-visualization/v3", GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		InputMSH: inputPath, ExportMetadata: metadataPath, SourceMetadata: sourceMetadataPath,
		OverviewSVG: overviewSVGPath, MeshDetailsSVG: meshDetailsSVGPath,
		Relief3DSVG: relief3DSVGPath, ProfilesSVG: profilesSVGPath, ProfilesCSV: profilesCSVPath,
		Source: source, VerticalReference: metadata.VerticalReference,
		VerticalCaveat: metadata.VerticalCaveat, Overview: overview, MeshDetails: meshDetails,
		Relief3D: relief3D, Profiles: profilesReport,
	}
	reportPath := filepath.Join(outputDir, "bathymetry-overview.json")
	if err := writeSeabedRenderReport(reportPath, runReport); err != nil {
		return err
	}

	if !quiet {
		fmt.Printf("Обзорная батиметрическая карта: %s\n", overviewSVGPath)
		fmt.Printf("Фрагменты фактической сетки: %s\n", meshDetailsSVGPath)
		fmt.Printf("3D-рельеф: %s\n", relief3DSVGPath)
		fmt.Printf("Профили рельефа: %s\n", profilesSVGPath)
		fmt.Printf("Таблица профилей: %s\n", profilesCSVPath)
		fmt.Printf("Отчёт построения: %s\n", reportPath)
		fmt.Printf("Глубина: 0–%.1f м; изобат: %d; NoData узлов: %.2f%%; NoData ячеек: %.2f%%\n",
			overview.MaxDepthM, len(overview.RenderedIsobathsM), overview.NoDataNodePercent, overview.NoDataCellPercent)
		for _, fragment := range meshDetails.Fragments {
			fmt.Printf("%s: ячеек %d; рёбра %.0f/%.0f/%.0f м; Qср=%.3f\n",
				fragment.Title, fragment.CellCount, fragment.EdgeMinM, fragment.EdgeMeanM, fragment.EdgeMaxM, fragment.QualityMean)
		}
		fmt.Printf("3D: вертикаль ×%.0f; контрольных узлов: %d; профильных трасс: %d\n",
			relief3D.VerticalExaggeration, relief3D.ControlPointCount, profilesReport.ProfileCount)
		for _, profile := range profilesReport.Profiles {
			fmt.Printf("%s: %.1f км; узлов %d; глубина в конце %.1f м\n",
				profile.Name, profile.LengthM/1000, profile.PointCount, profile.EndDepthM)
		}
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
		return fmt.Errorf("кодирование отчёта батиметрической визуализации: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога отчёта батиметрической визуализации: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта батиметрической визуализации %q: %w", path, err)
	}
	return nil
}
