package adaptive

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// WriteFieldCSV сохраняет полное узловое поле и все слагаемые формулы. CSV
// предназначен для проверки ADAPT-01 и последующей передачи в Gmsh в ADAPT-02.
func WriteFieldCSV(path string, field Field) error {
	if len(field.Nodes) == 0 || field.Report.SchemaVersion != SchemaVersion {
		return fmt.Errorf("поле размера пусто или имеет неподдерживаемую схему")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога поля размера %q: %w", filepath.Dir(path), err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("создание поля размера %q: %w", path, err)
	}
	writer := csv.NewWriter(file)
	writeErr := writer.Write([]string{
		"schema_version", "node_id", "x_m", "y_m", "longitude_deg", "latitude_deg",
		"water_depth_m", "depth_available", "distance_to_coast_m", "coast_curvature_deg",
		"depth_gradient_deg", "effective_gradient_deg", "gradient_available", "base_size_m", "distance_refinement_m",
		"curvature_refinement_m", "gradient_refinement_m", "raw_target_size_m",
		"target_size_m", "growth_limited", "zone",
	})
	for _, node := range field.Nodes {
		if writeErr != nil {
			break
		}
		writeErr = writer.Write([]string{
			SchemaVersion, strconv.Itoa(node.NodeID), formatFloat(node.XM), formatFloat(node.YM),
			formatFloat(node.LongitudeDeg), formatFloat(node.LatitudeDeg), formatFloat(node.WaterDepthM),
			strconv.FormatBool(node.DepthAvailable), formatFloat(node.DistanceToCoastM),
			formatFloat(node.CoastCurvatureDeg), formatFloat(node.DepthGradientDeg),
			formatFloat(node.EffectiveGradientDeg), strconv.FormatBool(node.GradientAvailable), formatFloat(node.BaseSizeM),
			formatFloat(node.DistanceRefinementM), formatFloat(node.CurvatureRefinementM),
			formatFloat(node.GradientRefinementM), formatFloat(node.RawTargetSizeM),
			formatFloat(node.TargetSizeM), strconv.FormatBool(node.GrowthLimited), node.Zone,
		})
	}
	writer.Flush()
	if writeErr == nil {
		writeErr = writer.Error()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись поля размера %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие поля размера %q: %w", path, closeErr)
	}
	return nil
}

// WriteReportJSON сохраняет конфигурацию, формулы и критерии плавности поля.
func WriteReportJSON(path string, report Report) error {
	if report.SchemaVersion != SchemaVersion {
		return fmt.Errorf("отчёт поля размера имеет неподдерживаемую схему %q", report.SchemaVersion)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта поля размера: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога отчёта поля размера %q: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта поля размера %q: %w", path, err)
	}
	return nil
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', 15, 64)
}
