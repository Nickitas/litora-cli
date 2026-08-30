package seabed

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"coastal-geometry/internal/domain/blacksea"
	"coastal-geometry/internal/domain/mesh"
)

// ExportMetadata описывает системы координат, вертикальную систему, единицы и
// таблицы кодов для VTU и табличных артефактов lito-seabed/v1.
type ExportMetadata struct {
	SchemaVersion                   string           `json:"schema_version"`
	HorizontalSourceCRS             string           `json:"horizontal_source_crs"`
	HorizontalSourceEPSG            int              `json:"horizontal_source_epsg"`
	HorizontalAxisOrder             string           `json:"horizontal_axis_order"`
	HorizontalAngularUnit           string           `json:"horizontal_angular_unit"`
	HorizontalMeshCRS               string           `json:"horizontal_mesh_crs"`
	ProjectionReferenceLatitudeDeg  float64          `json:"projection_reference_latitude_deg"`
	ProjectionReferenceLongitudeDeg float64          `json:"projection_reference_longitude_deg"`
	HorizontalLinearUnit            string           `json:"horizontal_linear_unit"`
	VerticalReference               string           `json:"vertical_reference"`
	VerticalEPSG                    *int             `json:"vertical_epsg"`
	VerticalUnit                    string           `json:"vertical_unit"`
	ElevationPositiveDirection      string           `json:"elevation_positive_direction"`
	VerticalExaggeration            float64          `json:"vertical_exaggeration"`
	VerticalCaveat                  string           `json:"vertical_caveat"`
	NoDataSentinel                  float64          `json:"no_data_sentinel"`
	RegionThresholds                RegionThresholds `json:"region_thresholds"`
	CodeTables                      MSHCodeTables    `json:"code_tables"`
}

// NewExportMetadata создаёт метаданные экспорта для сферической LAEA-проекции
// Чёрного моря. Название и оговорка вертикальной системы берутся из паспорта
// батиметрии и не подменяются универсальным датумом Lito.
func NewExportMetadata(projection mesh.EqualAreaProjection, verticalReference, verticalCaveat string) ExportMetadata {
	return ExportMetadata{
		SchemaVersion:                   SeabedMSHSchemaVersion,
		HorizontalSourceCRS:             "WGS 84",
		HorizontalSourceEPSG:            4326,
		HorizontalAxisOrder:             "longitude,latitude",
		HorizontalAngularUnit:           "degree",
		HorizontalMeshCRS:               "spherical_laea",
		ProjectionReferenceLatitudeDeg:  projection.ReferenceLat,
		ProjectionReferenceLongitudeDeg: projection.ReferenceLon,
		HorizontalLinearUnit:            "m",
		VerticalReference:               strings.TrimSpace(verticalReference),
		VerticalUnit:                    "m",
		ElevationPositiveDirection:      "up",
		VerticalExaggeration:            1,
		VerticalCaveat:                  strings.TrimSpace(verticalCaveat),
		NoDataSentinel:                  mshNoDataSentinel,
		RegionThresholds:                DefaultRegionThresholds(),
		CodeTables:                      DefaultMSHCodeTables(),
	}
}

// WriteExportMetadataJSON сохраняет сопутствующее описание CSV/VTU. Те же
// данные продублированы внутри VTU и в metadata-записи CSV, поэтому sidecar служит
// удобным человекочитаемым паспортом, а не единственным носителем метаданных.
func WriteExportMetadataJSON(path string, metadata ExportMetadata) error {
	if err := validateExportMetadata(metadata); err != nil {
		return err
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование метаданных экспорта: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога метаданных %q: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("сохранение метаданных экспорта %q: %w", path, err)
	}
	return nil
}

// ReadExportMetadataJSON читает и проверяет паспорт пространственных данных,
// созданный EXPORT-02. Неполная вертикальная система, другая акватория или
// несовместимая версия схемы отклоняются до визуализации.
func ReadExportMetadataJSON(path string) (ExportMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExportMetadata{}, fmt.Errorf("чтение метаданных экспорта %q: %w", path, err)
	}
	var metadata ExportMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return ExportMetadata{}, fmt.Errorf("разбор метаданных экспорта %q: %w", path, err)
	}
	if err := validateExportMetadata(metadata); err != nil {
		return ExportMetadata{}, fmt.Errorf("проверка метаданных экспорта %q: %w", path, err)
	}
	return metadata, nil
}

func normalizeExportMetadata(metadata ExportMetadata, model Model) (ExportMetadata, error) {
	if metadata.SchemaVersion == "" {
		metadata.SchemaVersion = SeabedMSHSchemaVersion
	}
	if metadata.HorizontalSourceCRS == "" {
		metadata.HorizontalSourceCRS = "WGS 84"
	}
	if metadata.HorizontalSourceEPSG == 0 {
		metadata.HorizontalSourceEPSG = 4326
	}
	if metadata.HorizontalAxisOrder == "" {
		metadata.HorizontalAxisOrder = "longitude,latitude"
	}
	if metadata.HorizontalAngularUnit == "" {
		metadata.HorizontalAngularUnit = "degree"
	}
	if metadata.HorizontalMeshCRS == "" {
		metadata.HorizontalMeshCRS = "spherical_laea"
	}
	if metadata.HorizontalLinearUnit == "" {
		metadata.HorizontalLinearUnit = "m"
	}
	if metadata.VerticalUnit == "" {
		metadata.VerticalUnit = "m"
	}
	if metadata.ElevationPositiveDirection == "" {
		metadata.ElevationPositiveDirection = "up"
	}
	if metadata.VerticalExaggeration == 0 {
		metadata.VerticalExaggeration = 1
	}
	if metadata.NoDataSentinel == 0 {
		metadata.NoDataSentinel = mshNoDataSentinel
	}
	if metadata.RegionThresholds == (RegionThresholds{}) {
		metadata.RegionThresholds = model.CellDerivation.RegionThresholds
	}
	if emptyMSHCodeTables(metadata.CodeTables) {
		metadata.CodeTables = DefaultMSHCodeTables()
	}
	if metadata.RegionThresholds != model.CellDerivation.RegionThresholds {
		return ExportMetadata{}, fmt.Errorf("пороги регионов метаданных не совпадают с моделью")
	}
	if err := validateExportMetadata(metadata); err != nil {
		return ExportMetadata{}, err
	}
	return metadata, nil
}

func validateExportMetadata(metadata ExportMetadata) error {
	if metadata.SchemaVersion != SeabedMSHSchemaVersion {
		return fmt.Errorf("неподдерживаемая схема экспорта %q", metadata.SchemaVersion)
	}
	if metadata.HorizontalSourceCRS != "WGS 84" || metadata.HorizontalSourceEPSG != 4326 ||
		metadata.HorizontalAxisOrder != "longitude,latitude" || metadata.HorizontalAngularUnit != "degree" {
		return fmt.Errorf("исходная горизонтальная система должна быть WGS 84 / EPSG:4326 с порядком longitude,latitude")
	}
	if metadata.HorizontalMeshCRS != "spherical_laea" || metadata.HorizontalLinearUnit != "m" {
		return fmt.Errorf("сетка должна быть описана как spherical_laea в метрах")
	}
	if !blacksea.Contains(metadata.ProjectionReferenceLatitudeDeg, metadata.ProjectionReferenceLongitudeDeg) {
		return fmt.Errorf("центр LAEA %.6f°, %.6f° находится вне области Чёрного моря",
			metadata.ProjectionReferenceLatitudeDeg, metadata.ProjectionReferenceLongitudeDeg)
	}
	if strings.TrimSpace(metadata.VerticalReference) == "" || strings.TrimSpace(metadata.VerticalCaveat) == "" {
		return fmt.Errorf("вертикальная система и её оговорка должны быть явно заданы")
	}
	if metadata.VerticalEPSG != nil && *metadata.VerticalEPSG <= 0 {
		return fmt.Errorf("EPSG вертикальной системы должен быть положительным")
	}
	if metadata.VerticalUnit != "m" || metadata.ElevationPositiveDirection != "up" || metadata.VerticalExaggeration != 1 {
		return fmt.Errorf("VTU/CSV должны хранить отметку в метрах, вверх, без вертикального увеличения")
	}
	if !finite(metadata.NoDataSentinel) || metadata.NoDataSentinel >= 0 {
		return fmt.Errorf("NoData sentinel должен быть конечным отрицательным числом")
	}
	if _, err := normalizeRegionThresholds(metadata.RegionThresholds); err != nil {
		return fmt.Errorf("пороги регионов экспорта: %w", err)
	}
	if err := validateMSHCodeTables(metadata.CodeTables); err != nil {
		return err
	}
	return nil
}

func validateMSHCodeTables(actual MSHCodeTables) error {
	expected := DefaultMSHCodeTables()
	if !sameCodeMap(actual.SamplingMethod, expected.SamplingMethod) ||
		!sameCodeMap(actual.QualityFlag, expected.QualityFlag) ||
		!sameCodeMap(actual.BoundaryKind, expected.BoundaryKind) ||
		!sameCodeMap(actual.CellRegion, expected.CellRegion) ||
		!sameCodeMap(actual.CellQuality, expected.CellQuality) {
		return fmt.Errorf("таблицы кодов не соответствуют схеме %s", SeabedMSHSchemaVersion)
	}
	return nil
}

func sameCodeMap[K comparable](actual, expected map[K]int) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, value := range expected {
		if actual[key] != value {
			return false
		}
	}
	return true
}

func emptyMSHCodeTables(tables MSHCodeTables) bool {
	return len(tables.SamplingMethod) == 0 && len(tables.QualityFlag) == 0 &&
		len(tables.BoundaryKind) == 0 && len(tables.CellRegion) == 0 && len(tables.CellQuality) == 0
}

func exportMetadataCSVHeader() []string {
	return []string{
		"schema_version", "horizontal_source_crs", "horizontal_source_epsg",
		"horizontal_axis_order", "horizontal_angular_unit", "horizontal_mesh_crs",
		"projection_reference_latitude_deg", "projection_reference_longitude_deg",
		"horizontal_linear_unit", "vertical_reference", "vertical_epsg",
		"vertical_unit", "elevation_positive_direction", "vertical_exaggeration",
		"vertical_caveat",
	}
}

func exportMetadataCSVValues(metadata ExportMetadata) []string {
	verticalEPSG := ""
	if metadata.VerticalEPSG != nil {
		verticalEPSG = fmt.Sprintf("%d", *metadata.VerticalEPSG)
	}
	return []string{
		metadata.SchemaVersion,
		metadata.HorizontalSourceCRS,
		fmt.Sprintf("%d", metadata.HorizontalSourceEPSG),
		metadata.HorizontalAxisOrder,
		metadata.HorizontalAngularUnit,
		metadata.HorizontalMeshCRS,
		formatFloat(metadata.ProjectionReferenceLatitudeDeg),
		formatFloat(metadata.ProjectionReferenceLongitudeDeg),
		metadata.HorizontalLinearUnit,
		metadata.VerticalReference,
		verticalEPSG,
		metadata.VerticalUnit,
		metadata.ElevationPositiveDirection,
		formatFloat(metadata.VerticalExaggeration),
		metadata.VerticalCaveat,
	}
}

func validFiniteNonNegative(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0
}
