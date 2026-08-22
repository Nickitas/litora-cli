package seabed

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// WriteNodeDepthCSV сохраняет полный узловой слой BATHY-01 в порядке полей
// контракта lito-seabed/v1. Nil записывается пустым полем, а не нулём.
func WriteNodeDepthCSV(path string, model Model) error {
	file, err := createOutputFile(path)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	writeErr := writer.Write([]string{
		"id", "x_m", "y_m", "longitude_deg", "latitude_deg", "elevation_m",
		"water_depth_m", "sampling_method", "source_distance_m", "quality_flag",
		"is_boundary", "boundary_kind",
	})
	for nodeID := 1; writeErr == nil && nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		writeErr = writer.Write([]string{
			strconv.Itoa(node.ID),
			formatFloat(node.XM),
			formatFloat(node.YM),
			formatFloat(node.LongitudeDeg),
			formatFloat(node.LatitudeDeg),
			formatOptionalFloat(node.ElevationM),
			formatOptionalFloat(node.WaterDepthM),
			string(node.SamplingMethod),
			formatOptionalFloat(node.SourceDistanceM),
			string(node.QualityFlag),
			strconv.FormatBool(node.IsBoundary),
			string(node.BoundaryKind),
		})
	}
	writer.Flush()
	if writeErr == nil {
		writeErr = writer.Error()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись таблицы узлов %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие таблицы узлов %q: %w", path, closeErr)
	}
	return nil
}

// WriteCellsCSV сохраняет принятые производные характеристики BATHY-03.
// Ячейки с NoData не попадают в файл и отражаются в CellSummary.
func WriteCellsCSV(path string, model Model) error {
	file, err := createOutputFile(path)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	writeErr := writer.Write([]string{
		"id", "node_ids", "area_m2", "elevation_min_m", "elevation_max_m",
		"elevation_mean_m", "water_depth_mean_m", "slope_deg", "aspect_deg",
		"roughness_m", "region", "quality_flag", "quality_score",
	})
	for _, cell := range model.Cells {
		if writeErr != nil {
			break
		}
		writeErr = writer.Write([]string{
			strconv.Itoa(cell.ID),
			fmt.Sprintf("[%d,%d,%d,%d]", cell.NodeIDs[0], cell.NodeIDs[1], cell.NodeIDs[2], cell.NodeIDs[3]),
			formatFloat(cell.AreaM2),
			formatFloat(cell.ElevationMinM),
			formatFloat(cell.ElevationMaxM),
			formatFloat(cell.ElevationMeanM),
			formatFloat(cell.WaterDepthMeanM),
			formatFloat(cell.SlopeDeg),
			formatOptionalFloat(cell.AspectDeg),
			formatFloat(cell.RoughnessM),
			string(cell.Region),
			string(cell.QualityFlag),
			formatFloat(cell.QualityScore),
		})
	}
	writer.Flush()
	if writeErr == nil {
		writeErr = writer.Error()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись таблицы ячеек %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие таблицы ячеек %q: %w", path, closeErr)
	}
	return nil
}

// WriteCorrectionsCSV сохраняет построчный журнал всех коррекций BATHY-02.
func WriteCorrectionsCSV(path string, corrections []Correction) error {
	file, err := createOutputFile(path)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	writeErr := writer.Write([]string{
		"node_id", "kind", "original_elevation_m", "corrected_elevation_m", "adjustment_m", "reason",
	})
	for _, correction := range corrections {
		if writeErr != nil {
			break
		}
		writeErr = writer.Write([]string{
			strconv.Itoa(correction.NodeID),
			string(correction.Kind),
			formatOptionalFloat(correction.OriginalElevationM),
			formatOptionalFloat(correction.CorrectedElevationM),
			formatOptionalFloat(correction.AdjustmentM),
			correction.Reason,
		})
	}
	writer.Flush()
	if writeErr == nil {
		writeErr = writer.Error()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись журнала коррекций %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие журнала коррекций %q: %w", path, closeErr)
	}
	return nil
}

// WriteReconciliationJSON сохраняет диагностический блок согласования масок с
// агрегатами и полным журналом исправлений.
func WriteReconciliationJSON(path string, summary ReconciliationSummary) error {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта согласования: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога отчёта %q: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта согласования %q: %w", path, err)
	}
	return nil
}

// WriteCellDerivationJSON сохраняет методы, единицы, пороги и сводку BATHY-03.
func WriteCellDerivationJSON(path string, metadata CellDerivationMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта характеристик ячеек: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога отчёта %q: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта характеристик ячеек %q: %w", path, err)
	}
	return nil
}

func createOutputFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("создание каталога %q: %w", filepath.Dir(path), err)
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("создание файла %q: %w", path, err)
	}
	return file, nil
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', 15, 64)
}

func formatOptionalFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return formatFloat(*value)
}
