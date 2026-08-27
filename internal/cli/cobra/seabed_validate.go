package cobra

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	adaptivemodel "coastal-geometry/internal/domain/adaptive"
	"coastal-geometry/internal/domain/seabed"

	"github.com/spf13/cobra"
)

var (
	seabedValidateInput             string
	seabedValidateMetadata          string
	seabedValidateReference         string
	seabedValidateReferenceMetadata string
	seabedValidateReferencePassport string
	seabedValidateSizeField         string
	seabedValidateSizeFieldReport   string
	seabedValidateOutput            string
	seabedValidateIsobaths          string
	seabedValidateWorstCells        int
	seabedValidateMaxNearest        float64
)

var seabedValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Проверить сохранение рельефа по отдельной опорной модели",
	Long: `Сравнивает принятую модель дна с отдельной опорной моделью в общей
LAEA-проекции. Центры опорных ячеек используются как отложенные точки.
Отчёт содержит MAE/RMSE/смещение/P95 глубины, объём, площади глубинных зон,
расстояния изобат, ошибку уклона, ближайшие замены, качество ячеек и
соответствие полю размера. Пороговые значения выводятся из разрешения и
вертикальной неопределённости, записанных в паспорте контрольного источника.
Межпродуктовый контроль явно отделяется от независимой публикационной проверки.`,
	RunE: runSeabedValidate,
}

func init() {
	seabedCmd.AddCommand(seabedValidateCmd)
	seabedValidateCmd.Flags().StringVar(&seabedValidateInput, "input", "", "проверяемый батиметрический MSH; по умолчанию output/seabed/black-sea-depth.msh")
	seabedValidateCmd.Flags().StringVar(&seabedValidateMetadata, "metadata", "", "паспорт EXPORT-02 проверяемой модели; по умолчанию рядом с MSH")
	seabedValidateCmd.Flags().StringVar(&seabedValidateReference, "reference", "", "отдельный опорный MSH lito-seabed/v1 (обязательный флаг)")
	seabedValidateCmd.Flags().StringVar(&seabedValidateReferenceMetadata, "reference-metadata", "", "паспорт EXPORT-02 опорной модели; по умолчанию рядом с опорным MSH")
	seabedValidateCmd.Flags().StringVar(&seabedValidateReferencePassport, "reference-passport", "", "паспорт происхождения и неопределённости опорной модели (обязательный флаг)")
	seabedValidateCmd.Flags().StringVar(&seabedValidateSizeField, "size-field", "", "CSV поля ADAPT-01; по умолчанию output/seabed/adaptive/size-field.csv")
	seabedValidateCmd.Flags().StringVar(&seabedValidateSizeFieldReport, "size-field-report", "", "JSON поля ADAPT-01; по умолчанию output/seabed/adaptive/size-field.json")
	seabedValidateCmd.Flags().StringVar(&seabedValidateOutput, "output", "output", "корневой каталог результатов; отчёты сохраняются в seabed/quality")
	seabedValidateCmd.Flags().StringVar(&seabedValidateIsobaths, "isobaths", "20,200,1000,2000", "положительные контрольные глубины изобат в метрах через запятую")
	seabedValidateCmd.Flags().IntVar(&seabedValidateWorstCells, "worst-cells", 20, "число худших локальных ошибок в JSON")
	seabedValidateCmd.Flags().Float64Var(&seabedValidateMaxNearest, "max-nearest-distance", 0, "предел ближайшей замены, м; 0 означает два шага опорного источника")
}

type seabedValidatePaths struct {
	inputMSH, inputMetadata         string
	referenceMSH, referenceMetadata string
	referencePassport               string
	sizeField, sizeFieldReport      string
}

func runSeabedValidate(_ *cobra.Command, _ []string) error {
	paths, err := resolveSeabedValidatePaths()
	if err != nil {
		return err
	}
	if seabedValidateWorstCells <= 0 {
		return fmt.Errorf("--worst-cells должно быть положительным")
	}
	if math.IsNaN(seabedValidateMaxNearest) || math.IsInf(seabedValidateMaxNearest, 0) || seabedValidateMaxNearest < 0 {
		return fmt.Errorf("--max-nearest-distance должно быть конечным неотрицательным числом")
	}
	isobaths, err := parsePositiveFloatList(seabedValidateIsobaths, "глубины изобат QA-02")
	if err != nil {
		return err
	}

	evaluatedDocument, err := seabed.ReadMSH2(paths.inputMSH)
	if err != nil {
		return fmt.Errorf("чтение проверяемой модели QA-02: %w", err)
	}
	referenceDocument, err := seabed.ReadMSH2(paths.referenceMSH)
	if err != nil {
		return fmt.Errorf("чтение опорной модели QA-02: %w", err)
	}
	for name, document := range map[string]seabed.MSHDocument{"проверяемый": evaluatedDocument, "опорный": referenceDocument} {
		if document.Metadata.ModelKind != seabed.MSHModelSeabed || document.Metadata.SchemaVersion != seabed.SeabedMSHSchemaVersion || !document.Model.Accepted {
			return fmt.Errorf("%s файл не является принятой моделью %s", name, seabed.SeabedMSHSchemaVersion)
		}
	}
	evaluatedMetadata, err := seabed.ReadExportMetadataJSON(paths.inputMetadata)
	if err != nil {
		return err
	}
	referenceMetadata, err := seabed.ReadExportMetadataJSON(paths.referenceMetadata)
	if err != nil {
		return err
	}
	passport, err := seabed.ReadReliefReferencePassport(paths.referencePassport)
	if err != nil {
		return err
	}
	fieldReport, err := adaptivemodel.ReadReportJSON(paths.sizeFieldReport)
	if err != nil {
		return err
	}
	field, err := adaptivemodel.ReadTargetSizeFieldCSV(paths.sizeField, evaluatedDocument.Model)
	if err != nil {
		return err
	}
	if err := field.ValidateAgainstReport(fieldReport); err != nil {
		return fmt.Errorf("CSV и JSON поля ADAPT-01 не согласованы: %w", err)
	}
	zoneNames := make(map[string]string, len(fieldReport.Summary.Zones))
	for _, zone := range fieldReport.Summary.Zones {
		zoneNames[zone.ID] = zone.Name
	}
	report, err := seabed.EvaluateReliefQuality(
		referenceDocument.Model, referenceMetadata,
		evaluatedDocument.Model, evaluatedMetadata,
		passport,
		seabed.ReliefQualityConfig{
			IsobathsM: isobaths, WorstCellCount: seabedValidateWorstCells,
			MaxNearestDistanceM: seabedValidateMaxNearest,
			TargetSizeM:         field.TargetSizeM, TargetZones: field.Zones, TargetZoneNames: zoneNames,
		},
	)
	if err != nil {
		return err
	}
	report.Inputs, err = buildSeabedValidateInputs(paths)
	if err != nil {
		return err
	}
	outputDirectory := filepath.Join(seabedValidateOutput, "seabed", "quality")
	jsonPath := filepath.Join(outputDirectory, "relief-quality.json")
	tsvPath := filepath.Join(outputDirectory, "relief-quality.tsv")
	if err := seabed.WriteReliefQualityJSON(jsonPath, report); err != nil {
		return err
	}
	if err := seabed.WriteReliefQualityTSV(tsvPath, report); err != nil {
		return err
	}
	if !quiet {
		printSeabedValidateSummary(report)
		fmt.Printf("Отчёты QA-02: %s, %s\n", jsonPath, tsvPath)
	}
	if !report.MetricsAccepted {
		return fmt.Errorf("QA-02 не прошла пороги качества; причины сохранены в %s", jsonPath)
	}
	return nil
}

func resolveSeabedValidatePaths() (seabedValidatePaths, error) {
	paths := seabedValidatePaths{}
	paths.inputMSH = strings.TrimSpace(seabedValidateInput)
	if paths.inputMSH == "" {
		paths.inputMSH = filepath.Join(seabedValidateOutput, "seabed", "black-sea-depth.msh")
	}
	paths.inputMetadata = strings.TrimSpace(seabedValidateMetadata)
	if paths.inputMetadata == "" {
		paths.inputMetadata = filepath.Join(filepath.Dir(paths.inputMSH), "export-metadata.json")
	}
	paths.referenceMSH = strings.TrimSpace(seabedValidateReference)
	if paths.referenceMSH == "" {
		return seabedValidatePaths{}, fmt.Errorf("для QA-02 обязателен --reference с отдельной опорной моделью")
	}
	paths.referenceMetadata = strings.TrimSpace(seabedValidateReferenceMetadata)
	if paths.referenceMetadata == "" {
		paths.referenceMetadata = filepath.Join(filepath.Dir(paths.referenceMSH), "export-metadata.json")
	}
	paths.referencePassport = strings.TrimSpace(seabedValidateReferencePassport)
	if paths.referencePassport == "" {
		return seabedValidatePaths{}, fmt.Errorf("для QA-02 обязателен --reference-passport с разрешением и неопределённостью источника")
	}
	paths.sizeField = strings.TrimSpace(seabedValidateSizeField)
	if paths.sizeField == "" {
		paths.sizeField = filepath.Join(seabedValidateOutput, "seabed", "adaptive", "size-field.csv")
	}
	paths.sizeFieldReport = strings.TrimSpace(seabedValidateSizeFieldReport)
	if paths.sizeFieldReport == "" {
		paths.sizeFieldReport = filepath.Join(seabedValidateOutput, "seabed", "adaptive", "size-field.json")
	}
	return paths, nil
}

func buildSeabedValidateInputs(paths seabedValidatePaths) (seabed.ReliefQualityInputs, error) {
	files := []string{
		paths.inputMSH, paths.inputMetadata, paths.referenceMSH, paths.referenceMetadata,
		paths.referencePassport, paths.sizeField, paths.sizeFieldReport,
	}
	checksums := make([]string, len(files))
	for index, path := range files {
		checksum, err := adaptiveFileSHA256(path)
		if err != nil {
			return seabed.ReliefQualityInputs{}, err
		}
		checksums[index] = checksum
	}
	return seabed.ReliefQualityInputs{
		EvaluatedMSH: paths.inputMSH, EvaluatedMSHSHA256: checksums[0],
		EvaluatedMetadata: paths.inputMetadata, EvaluatedMetadataSHA256: checksums[1],
		ReferenceMSH: paths.referenceMSH, ReferenceMSHSHA256: checksums[2],
		ReferenceMetadata: paths.referenceMetadata, ReferenceMetadataSHA256: checksums[3],
		ReferencePassport: paths.referencePassport, ReferencePassportSHA256: checksums[4],
		TargetSizeField: paths.sizeField, TargetSizeFieldSHA256: checksums[5],
		TargetSizeFieldReport: paths.sizeFieldReport, TargetSizeFieldReportSHA256: checksums[6],
	}, nil
}

func printSeabedValidateSummary(report seabed.ReliefQualityReport) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Показатель\tЗначение\tПорог")
	fmt.Fprintln(writer, "----------\t--------\t------")
	fmt.Fprintf(writer, "RMSE глубины\t%.3f м\t≤ %.3f м\n", report.Depth.RMSEM, report.Thresholds.DepthRMSEMaxM)
	fmt.Fprintf(writer, "P95 ошибки глубины\t%.3f м\t≤ %.3f м\n", report.Depth.P95AbsoluteErrorM, report.Thresholds.DepthP95AbsoluteErrorMaxM)
	fmt.Fprintf(writer, "Отклонение объёма\t%.3f%%\t≤ %.3f%% по модулю\n", report.Volume.DeviationPercent, report.Thresholds.VolumeDeviationMaxPercent)
	fmt.Fprintf(writer, "RMSE уклона\t%.3f°\t≤ %.3f°\n", report.Slope.RMSEDeg, report.Thresholds.SlopeRMSEMaxDeg)
	fmt.Fprintf(writer, "Ближайшие замены\t%.3f%%\t≤ %.3f%%\n", report.Sampling.NearestNodePercent, report.Thresholds.NearestNodeMaxPercent)
	fmt.Fprintf(writer, "P05 качества ячеек\t%.3f\t≥ %.3f\n", report.Mesh.P05QuadQuality, report.Thresholds.P05QuadQualityMin)
	fmt.Fprintf(writer, "Рёбра в целевом допуске\t%.3f%%\t≥ %.3f%%\n", report.Mesh.TargetSizeCompliancePercent, report.Thresholds.TargetSizeComplianceMinPct)
	_ = writer.Flush()
	status := "не принято"
	if report.MetricsAccepted {
		status = "принято"
	}
	publication := "нет"
	if report.PublicationReady {
		publication = "да"
	}
	fmt.Printf("Метрики: %s; класс контроля: %s; независимая публикационная проверка: %s\n", status, report.Reference.ValidationClass, publication)
}
