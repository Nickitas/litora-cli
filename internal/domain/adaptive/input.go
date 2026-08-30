package adaptive

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"coastal-geometry/internal/domain/seabed"
)

// ReadReportJSON читает и проверяет машинный отчёт ADAPT-01. Отчёт с уже
// построенной сеткой не принимается как исходное поле следующего запуска.
func ReadReportJSON(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, fmt.Errorf("чтение отчёта поля размера %q: %w", path, err)
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return Report{}, fmt.Errorf("разбор отчёта поля размера %q: %w", path, err)
	}
	if report.SchemaVersion != SchemaVersion {
		return Report{}, fmt.Errorf("отчёт поля размера имеет схему %q вместо %q", report.SchemaVersion, SchemaVersion)
	}
	if report.AdaptiveMeshGenerated {
		return Report{}, fmt.Errorf("отчёт %q не является исходным результатом ADAPT-01", path)
	}
	if report.HorizontalUnit != HorizontalUnit || report.TargetSizeUnit != HorizontalUnit {
		return Report{}, fmt.Errorf("отчёт поля размера должен использовать метры")
	}
	if report.Summary.NodeCount <= 0 || report.Summary.Target.MinM <= 0 || report.Summary.Target.MaxM < report.Summary.Target.MinM {
		return Report{}, fmt.Errorf("отчёт поля размера содержит некорректную сводку")
	}
	return report, nil
}

// ReadTargetSizeFieldCSV потоково читает только столбцы, необходимые Gmsh,
// и одновременно проверяет привязку каждой строки к узлу исходной модели.
// В отличие от csv.ReadAll объём служебной памяти не зависит от числа строк.
func ReadTargetSizeFieldCSV(path string, model seabed.Model) (TargetSizeField, error) {
	if len(model.Nodes) <= 1 {
		return TargetSizeField{}, fmt.Errorf("батиметрическая модель не содержит узлов")
	}
	file, err := os.Open(path)
	if err != nil {
		return TargetSizeField{}, fmt.Errorf("открытие поля размера %q: %w", path, err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return TargetSizeField{}, fmt.Errorf("чтение заголовка поля размера %q: %w", path, err)
	}
	columns := make(map[string]int, len(header))
	for index, name := range header {
		columns[strings.TrimSpace(name)] = index
	}
	required := []string{"schema_version", "node_id", "x_m", "y_m", "target_size_m", "zone"}
	for _, name := range required {
		if _, ok := columns[name]; !ok {
			return TargetSizeField{}, fmt.Errorf("поле размера не содержит столбец %q", name)
		}
	}
	result := TargetSizeField{
		TargetSizeM: make([]float64, len(model.Nodes)),
		Zones:       make([]string, len(model.Nodes)),
		MinSizeM:    math.Inf(1),
	}
	seen := make([]bool, len(model.Nodes))
	for line := 2; ; line++ {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return TargetSizeField{}, fmt.Errorf("чтение поля размера %q, строка %d: %w", path, line, readErr)
		}
		value := func(name string) string {
			index := columns[name]
			if index >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[index])
		}
		if value("schema_version") != SchemaVersion {
			return TargetSizeField{}, fmt.Errorf("строка %d имеет неподдерживаемую схему %q", line, value("schema_version"))
		}
		nodeID, idErr := strconv.Atoi(value("node_id"))
		x, xErr := strconv.ParseFloat(value("x_m"), 64)
		y, yErr := strconv.ParseFloat(value("y_m"), 64)
		target, targetErr := strconv.ParseFloat(value("target_size_m"), 64)
		zone := value("zone")
		if idErr != nil || nodeID <= 0 || nodeID >= len(model.Nodes) || seen[nodeID] {
			return TargetSizeField{}, fmt.Errorf("строка %d содержит некорректный или повторный node_id", line)
		}
		if xErr != nil || yErr != nil || !finite(x) || !finite(y) || math.Abs(x-model.Nodes[nodeID].XM) > 1e-6 || math.Abs(y-model.Nodes[nodeID].YM) > 1e-6 {
			return TargetSizeField{}, fmt.Errorf("строка %d не совпадает с координатами узла %d исходного MSH", line, nodeID)
		}
		if targetErr != nil || !finite(target) || target <= 0 || zone == "" {
			return TargetSizeField{}, fmt.Errorf("строка %d содержит некорректный размер или зону", line)
		}
		seen[nodeID] = true
		result.TargetSizeM[nodeID] = target
		result.Zones[nodeID] = zone
		result.NodeCount++
		result.MinSizeM = math.Min(result.MinSizeM, target)
		result.MaxSizeM = math.Max(result.MaxSizeM, target)
	}
	if result.NodeCount != len(model.Nodes)-1 {
		return TargetSizeField{}, fmt.Errorf("поле размера содержит %d узлов вместо %d", result.NodeCount, len(model.Nodes)-1)
	}
	return result, nil
}

// ValidateAgainstReport проверяет, что CSV и JSON описывают один результат
// ADAPT-01, а не случайно смешанные запуски.
func (field TargetSizeField) ValidateAgainstReport(report Report) error {
	if field.NodeCount != report.Summary.NodeCount {
		return fmt.Errorf("CSV содержит %d узлов, отчёт — %d", field.NodeCount, report.Summary.NodeCount)
	}
	if math.Abs(field.MinSizeM-report.Summary.Target.MinM) > 1e-6 || math.Abs(field.MaxSizeM-report.Summary.Target.MaxM) > 1e-6 {
		return fmt.Errorf("диапазон CSV %.6g–%.6g м не совпадает с отчётом %.6g–%.6g м", field.MinSizeM, field.MaxSizeM, report.Summary.Target.MinM, report.Summary.Target.MaxM)
	}
	return nil
}
