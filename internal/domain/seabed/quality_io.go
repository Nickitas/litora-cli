package seabed

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"coastal-geometry/internal/domain/mesh"
)

// ReadReliefReferencePassport читает и проверяет паспорт контрольной модели.
func ReadReliefReferencePassport(path string) (ReliefReferencePassport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReliefReferencePassport{}, fmt.Errorf("чтение паспорта контрольной модели %q: %w", path, err)
	}
	var passport ReliefReferencePassport
	if err := json.Unmarshal(data, &passport); err != nil {
		return ReliefReferencePassport{}, fmt.Errorf("разбор паспорта контрольной модели %q: %w", path, err)
	}
	if err := validateReliefReferencePassport(passport); err != nil {
		return ReliefReferencePassport{}, fmt.Errorf("проверка паспорта контрольной модели %q: %w", path, err)
	}
	if passport.ExcludedSources == nil {
		passport.ExcludedSources = []string{}
	}
	if passport.Limitations == nil {
		passport.Limitations = []string{}
	}
	return passport, nil
}

func validateReliefReferencePassport(passport ReliefReferencePassport) error {
	if passport.SchemaVersion != ReliefReferencePassportSchemaVersion {
		return fmt.Errorf("неподдерживаемая схема паспорта %q", passport.SchemaVersion)
	}
	if strings.TrimSpace(passport.Title) == "" || strings.TrimSpace(passport.SourceProduct) == "" || strings.TrimSpace(passport.SourceVersion) == "" {
		return fmt.Errorf("название, продукт и версия контрольного источника обязательны")
	}
	checksum, err := hex.DecodeString(strings.TrimSpace(passport.DatasetSHA256))
	if err != nil || len(checksum) != sha256.Size {
		return fmt.Errorf("dataset_sha256 должен содержать 64 шестнадцатеричных символа")
	}
	if !finite(passport.HorizontalResolutionM) || passport.HorizontalResolutionM <= 0 ||
		!finite(passport.VerticalUncertaintyM) || passport.VerticalUncertaintyM <= 0 {
		return fmt.Errorf("горизонтальное разрешение и вертикальная неопределённость должны быть конечными положительными числами")
	}
	if strings.TrimSpace(passport.VerticalReference) == "" || strings.TrimSpace(passport.SamplingDesign) == "" {
		return fmt.Errorf("вертикальная система и схема контрольной выборки обязательны")
	}
	switch passport.ValidationClass {
	case ReliefValidationIndependent, ReliefValidationInterproduct, ReliefValidationHeldOut:
	default:
		return fmt.Errorf("неподдерживаемый класс проверки %q", passport.ValidationClass)
	}
	return nil
}

// WriteReliefQualityJSON сохраняет нормализованный машинный отчёт QA-02.
func WriteReliefQualityJSON(path string, report ReliefQualityReport) error {
	if report.SchemaVersion != ReliefQualitySchemaVersion {
		return fmt.Errorf("неподдерживаемая схема отчёта QA-02 %q", report.SchemaVersion)
	}
	normalizeReliefQualityReport(&report)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта QA-02: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога отчёта QA-02: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта QA-02 %q: %w", path, err)
	}
	return nil
}

// WriteReliefQualityTSV сохраняет плоскую таблицу ключевых метрик и порогов.
func WriteReliefQualityTSV(path string, report ReliefQualityReport) error {
	if report.SchemaVersion != ReliefQualitySchemaVersion {
		return fmt.Errorf("неподдерживаемая схема отчёта QA-02 %q", report.SchemaVersion)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога TSV QA-02: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("создание TSV QA-02 %q: %w", path, err)
	}
	writer := csv.NewWriter(file)
	writer.Comma = '\t'
	writer.UseCRLF = false
	write := func(section, metric, unit string, value, threshold float64, accepted bool) {
		status := "не принято"
		if accepted {
			status = "принято"
		}
		_ = writer.Write([]string{section, metric, unit, qualityFloat(value), qualityFloat(threshold), status})
	}
	_ = writer.Write([]string{"Раздел", "Показатель", "Единица", "Значение", "Порог", "Статус"})
	write("Глубина", "RMSE", "м", report.Depth.RMSEM, report.Thresholds.DepthRMSEMaxM, report.Depth.RMSEM <= report.Thresholds.DepthRMSEMaxM)
	write("Глубина", "MAE", "м", report.Depth.MAEM, math.NaN(), true)
	write("Глубина", "Смещение", "м", report.Depth.BiasM, math.NaN(), true)
	write("Глубина", "P95 абсолютной ошибки", "м", report.Depth.P95AbsoluteErrorM, report.Thresholds.DepthP95AbsoluteErrorMaxM, report.Depth.P95AbsoluteErrorM <= report.Thresholds.DepthP95AbsoluteErrorMaxM)
	write("Объём", "Абсолютное отклонение", "%", math.Abs(report.Volume.DeviationPercent), report.Thresholds.VolumeDeviationMaxPercent, math.Abs(report.Volume.DeviationPercent) <= report.Thresholds.VolumeDeviationMaxPercent)
	write("Уклон", "RMSE", "градус", report.Slope.RMSEDeg, report.Thresholds.SlopeRMSEMaxDeg, report.Slope.RMSEDeg <= report.Thresholds.SlopeRMSEMaxDeg)
	write("Выборка", "Ближайшие замены узлов", "%", report.Sampling.NearestNodePercent, report.Thresholds.NearestNodeMaxPercent, report.Sampling.NearestNodePercent <= report.Thresholds.NearestNodeMaxPercent)
	write("Сетка", "P05 качества четырёхугольников", "доля", report.Mesh.P05QuadQuality, report.Thresholds.P05QuadQualityMin, report.Mesh.P05QuadQuality >= report.Thresholds.P05QuadQualityMin)
	write("Сетка", "Соответствие целевому размеру", "%", report.Mesh.TargetSizeCompliancePercent, report.Thresholds.TargetSizeComplianceMinPct, report.Mesh.TargetSizeCompliancePercent >= report.Thresholds.TargetSizeComplianceMinPct)
	for _, band := range report.DepthBands {
		name := fmt.Sprintf("Площадь %.0f–∞ м", band.LowerDepthM)
		if band.UpperDepthM != nil {
			name = fmt.Sprintf("Площадь %.0f–%.0f м", band.LowerDepthM, *band.UpperDepthM)
		}
		write("Глубинные зоны", name, "%", band.AbsoluteDeviationPct, band.ResolutionTolerancePct, band.Accepted)
	}
	for _, isobath := range report.Isobaths {
		write("Изобаты", fmt.Sprintf("P95 расстояния %.0f м", isobath.DepthM), "м", isobath.P95DistanceM, report.Thresholds.IsobathP95DistanceMaxM, isobath.Accepted)
	}
	writer.Flush()
	writeErr := writer.Error()
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись TSV QA-02 %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие TSV QA-02 %q: %w", path, closeErr)
	}
	return nil
}

func normalizeReliefQualityReport(report *ReliefQualityReport) {
	if report.Reference.ExcludedSources == nil {
		report.Reference.ExcludedSources = []string{}
	}
	if report.Reference.Limitations == nil {
		report.Reference.Limitations = []string{}
	}
	if report.DepthBands == nil {
		report.DepthBands = []DepthBandPreservationMetrics{}
	}
	if report.Isobaths == nil {
		report.Isobaths = []IsobathPreservationMetrics{}
	}
	if report.Mesh.Regions == nil {
		report.Mesh.Regions = []MeshRegionQualityMetrics{}
	}
	if report.Mesh.TargetZones == nil {
		report.Mesh.TargetZones = []mesh.ZoneEdgeStatistics{}
	}
	if report.WorstCells == nil {
		report.WorstCells = []WorstReliefCell{}
	}
	if report.Reasons == nil {
		report.Reasons = []string{}
	}
}

func qualityFloat(value float64) string {
	if math.IsNaN(value) {
		return ""
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}
