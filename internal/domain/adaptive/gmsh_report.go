package adaptive

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"coastal-geometry/internal/domain/mesh"
)

const (
	// GmshReportSchemaVersion задаёт машинный контракт результата ADAPT-02.
	GmshReportSchemaVersion = "lito-adaptive-gmsh/v1"
)

// BackgroundFieldReport фиксирует способ передачи h(x,y) в Gmsh.
type BackgroundFieldReport struct {
	Format              string  `json:"format"`
	FieldType           string  `json:"field_type"`
	Interpolation       string  `json:"interpolation"`
	UseClosest          bool    `json:"use_closest"`
	SupportNodeCount    int     `json:"support_node_count"`
	SupportCellCount    int     `json:"support_cell_count"`
	MinimumTargetSizeM  float64 `json:"minimum_target_size_m"`
	MaximumTargetSizeM  float64 `json:"maximum_target_size_m"`
	PreSubdivisionScale float64 `json:"pre_subdivision_scale"`
}

// GmshArtifacts перечисляет воспроизводимые входы и результаты ADAPT-02.
type GmshArtifacts struct {
	BackgroundPOS string `json:"background_pos"`
	Geo           string `json:"geo"`
	MSH           string `json:"msh"`
	Log           string `json:"log"`
	EdgeTSV       string `json:"edge_tsv"`
}

// GmshReport объединяет предзапусковую оценку, фактические ресурсы,
// топологическую проверку и зональную статистику рёбер.
type GmshReport struct {
	SchemaVersion                     string                       `json:"schema_version"`
	GeneratedAt                       string                       `json:"generated_at"`
	InputMSH                          string                       `json:"input_msh"`
	InputMSHSHA256                    string                       `json:"input_msh_sha256"`
	ExportMetadata                    string                       `json:"export_metadata"`
	ExportMetadataSHA256              string                       `json:"export_metadata_sha256"`
	SizeFieldCSV                      string                       `json:"size_field_csv"`
	SizeFieldCSVSHA256                string                       `json:"size_field_csv_sha256"`
	SizeFieldReport                   string                       `json:"size_field_report"`
	SizeFieldReportSHA256             string                       `json:"size_field_report_sha256"`
	Coastline                         string                       `json:"coastline"`
	CoastlineSHA256                   string                       `json:"coastline_sha256"`
	BoundaryDetailMeters              float64                      `json:"boundary_detail_meters"`
	EffectiveBoundaryDetailMeters     float64                      `json:"effective_boundary_detail_meters"`
	Projection                        string                       `json:"projection"`
	ProjectionRoundTripMaxErrorMeters float64                      `json:"projection_round_trip_max_error_meters"`
	Algorithm                         mesh.Algorithm               `json:"algorithm"`
	GmshPath                          string                       `json:"gmsh_path"`
	GmshVersion                       string                       `json:"gmsh_version"`
	BackgroundField                   BackgroundFieldReport        `json:"background_field"`
	Preflight                         mesh.AdaptivePreflight       `json:"preflight"`
	MaximumCellCount                  int64                        `json:"maximum_cell_count"`
	LargeRunAllowed                   bool                         `json:"large_run_allowed"`
	Resources                         mesh.GenerationResourceStats `json:"resources"`
	Topology                          mesh.TopologyValidation      `json:"topology"`
	EdgeSamplingMethod                string                       `json:"edge_sampling_method"`
	Zones                             []mesh.ZoneEdgeStatistics    `json:"zones"`
	Artifacts                         GmshArtifacts                `json:"artifacts"`
	Accepted                          bool                         `json:"accepted"`
	Reasons                           []string                     `json:"reasons"`
}

// WriteGmshReportJSON сохраняет полный машинный отчёт ADAPT-02.
func WriteGmshReportJSON(path string, report GmshReport) error {
	if report.SchemaVersion != GmshReportSchemaVersion {
		return fmt.Errorf("отчёт ADAPT-02 имеет неподдерживаемую схему %q", report.SchemaVersion)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта ADAPT-02: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога отчёта ADAPT-02: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение отчёта ADAPT-02 %q: %w", path, err)
	}
	return nil
}

// WriteGmshEdgeTSV потоково сохраняет зональную статистику фактических рёбер.
func WriteGmshEdgeTSV(path string, zones []mesh.ZoneEdgeStatistics) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога статистики рёбер: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("создание статистики рёбер %q: %w", path, err)
	}
	buffered := bufio.NewWriterSize(file, 256*1024)
	writer := tabwriter.NewWriter(buffered, 0, 0, 2, ' ', 0)
	writeErr := func() error {
		if _, err := fmt.Fprintln(writer, "Зона\tНаблюдений рёбер\tЦель min, м\tЦель mean, м\tЦель max, м\tФакт min, м\tФакт P05, м\tФакт mean, м\tФакт P95, м\tФакт max, м\tMean факт/цель\tВ допуске 0.5–1.5, %"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(writer, "----\t-----------------\t-----------\t------------\t-----------\t-----------\t-----------\t------------\t-----------\t-----------\t---------------\t---------------------"); err != nil {
			return err
		}
		for _, zone := range zones {
			if _, err := fmt.Fprintf(writer, "%s\t%d\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.3f\t%.2f\n",
				zone.Name, zone.EdgeObservationCount, zone.TargetMinM, zone.TargetMeanM, zone.TargetMaxM,
				zone.ActualMinM, zone.ActualP05M, zone.ActualMeanM, zone.ActualP95M, zone.ActualMaxM,
				zone.RatioMean, zone.WithinTolerancePct); err != nil {
				return err
			}
		}
		return writer.Flush()
	}()
	if writeErr == nil {
		writeErr = buffered.Flush()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись статистики рёбер %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие статистики рёбер %q: %w", path, closeErr)
	}
	return nil
}
