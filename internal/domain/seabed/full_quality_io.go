package seabed

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// WriteFullBlackSeaQualityJSON сохраняет полный машинный отчёт QA-03.
func WriteFullBlackSeaQualityJSON(path string, report FullBlackSeaQualityReport) error {
	if report.SchemaVersion != FullBlackSeaQualitySchemaVersion {
		return fmt.Errorf("неподдерживаемая схема отчёта QA-03 %q", report.SchemaVersion)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта QA-03: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога отчёта QA-03: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта QA-03 %q: %w", path, err)
	}
	return nil
}

// WriteFullBlackSeaQualityTSV сохраняет плоскую таблицу ключевых шлюзов QA-03
// с русскими заголовками для аудита без разбора вложенного JSON.
func WriteFullBlackSeaQualityTSV(path string, report FullBlackSeaQualityReport) error {
	if report.SchemaVersion != FullBlackSeaQualitySchemaVersion {
		return fmt.Errorf("неподдерживаемая схема отчёта QA-03 %q", report.SchemaVersion)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога TSV QA-03: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("создание TSV QA-03 %q: %w", path, err)
	}
	writer := csv.NewWriter(file)
	writer.Comma = '\t'
	rows := [][]string{{"Раздел", "Показатель", "Значение", "Единица", "Принято"}}
	add := func(section, metric string, value float64, unit string, accepted bool) {
		rows = append(rows, []string{section, metric, strconv.FormatFloat(value, 'f', 6, 64), unit, strconv.FormatBool(accepted)})
	}
	add("Охват", "Максимальное отклонение границ", report.Extent.MaxDeviationDeg, "градус", report.Extent.Accepted)
	add("Топология", "Компоненты ячеек", float64(report.Topology.CellComponentCount), "шт.", report.Topology.Accepted)
	add("Топология", "Неожиданные граничные рёбра", float64(report.Topology.UnexpectedBoundaryEdgeCount), "шт.", report.Topology.Accepted)
	add("Топология", "Самопересечения ячеек", float64(report.Topology.SelfIntersectingCellCount), "шт.", report.Topology.Accepted)
	add("Топология", "Пересечения границы", float64(report.Topology.BoundaryIntersectionCount), "шт.", report.Topology.Accepted)
	add("Сетка", "Средняя длина ребра", report.EdgeSize.MeanEdgeM, "м", report.EdgeSize.Accepted)
	add("Сетка", "P05 длины ребра", report.EdgeSize.P05EdgeM, "м", report.EdgeSize.Accepted)
	add("Сетка", "P95 длины ребра", report.EdgeSize.P95EdgeM, "м", report.EdgeSize.Accepted)
	add("Глубина", "NoData узлов", float64(report.Depth.NoDataNodeCount), "шт.", report.Depth.Accepted)
	add("Глубина", "NoData ячеек", float64(report.Depth.NoDataCellCount), "шт.", report.Depth.Accepted)
	add("Глубина", "Положительные отметки", float64(report.Depth.PositiveElevationCount), "шт.", report.Depth.Accepted)
	add("Глубина", "Доля ближайших замен", report.Depth.NearestFallbackPercent, "%", report.Depth.NearestFallbackPercent <= report.Depth.NearestFallbackMaxPercent)
	add("Глубина", "Дальние ближайшие замены", float64(report.Depth.LongFallbackNodeCount), "шт.", report.Depth.LongFallbackNodeCount == 0)
	add("Глубина", "Максимальное расстояние до источника", report.Depth.MaxSourceDistanceM, "м", report.Depth.LongFallbackNodeCount == 0)
	add("Глубина", "Максимальная глубина", report.Depth.MaxWaterDepthM, "м", report.Depth.Accepted)
	add("Интегралы", "Площадь", report.Integrals.AreaKM2, "км²", report.Integrals.CoastlineAreaAccepted)
	add("Интегралы", "Объём", report.Integrals.VolumeKM3, "км³", publishedComparisonsAccepted(report.PublishedComparisons))
	add("Интегралы", "Средняя глубина", report.Integrals.MeanDepthM, "м", publishedComparisonsAccepted(report.PublishedComparisons))
	for _, comparison := range report.PublishedComparisons {
		add("Опубликованный ориентир "+comparison.Reference.ID, "Отклонение площади", comparison.AreaDeviationPercent, "%", comparison.AreaAccepted)
		add("Опубликованный ориентир "+comparison.Reference.ID, "Отклонение объёма", comparison.VolumeDeviationPercent, "%", comparison.VolumeAccepted)
		if comparison.Reference.MaxDepthM > 0 {
			add("Опубликованный ориентир "+comparison.Reference.ID, "Отклонение максимальной глубины", comparison.DepthDeviationPercent, "%", comparison.DepthAccepted)
		}
	}
	add("Ресурсы", "Общая длительность", report.Resources.DurationSeconds, "с", true)
	add("Ресурсы", "Пиковая память Go heap", float64(report.Resources.PeakHeapInUseBytes), "байт", true)
	add("Ресурсы", "Пиковая системная память Go", float64(report.Resources.PeakSystemBytes), "байт", true)
	for _, stage := range report.Resources.Stages {
		add("Ресурсы", "Этап "+stage.Title, stage.DurationSeconds, "с", true)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			_ = file.Close()
			return fmt.Errorf("запись TSV QA-03: %w", err)
		}
	}
	writer.Flush()
	writeErr := writer.Error()
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись TSV QA-03: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие TSV QA-03 %q: %w", path, closeErr)
	}
	return nil
}
