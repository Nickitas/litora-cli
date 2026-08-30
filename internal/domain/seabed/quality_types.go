package seabed

import "coastal-geometry/internal/domain/mesh"

const (
	// ReliefQualitySchemaVersion — версия машинного отчёта QA-02.
	ReliefQualitySchemaVersion = "lito-relief-quality/v1"
	// ReliefReferencePassportSchemaVersion — версия паспорта контрольной модели.
	ReliefReferencePassportSchemaVersion = "lito-relief-reference/v1"
)

// ReliefValidationClass описывает степень независимости контрольного набора.
type ReliefValidationClass string

const (
	// ReliefValidationIndependent означает независимые промеры или источник без
	// общих исходных данных с оцениваемой моделью.
	ReliefValidationIndependent ReliefValidationClass = "independent_measurements"
	// ReliefValidationInterproduct означает межпродуктовый контроль с возможной
	// общей частью исходных данных.
	ReliefValidationInterproduct ReliefValidationClass = "interproduct_control"
	// ReliefValidationHeldOut означает пространственно отложенную выборку того
	// же продукта; это тест реконструкции, но не независимая полевая валидация.
	ReliefValidationHeldOut ReliefValidationClass = "held_out_same_product"
)

// ReliefReferencePassport фиксирует происхождение, разрешение и
// неопределённость контрольной батиметрической модели.
type ReliefReferencePassport struct {
	SchemaVersion         string                `json:"schema_version"`
	Title                 string                `json:"title"`
	SourceProduct         string                `json:"source_product"`
	SourceVersion         string                `json:"source_version"`
	DatasetSHA256         string                `json:"dataset_sha256"`
	HorizontalResolutionM float64               `json:"horizontal_resolution_m"`
	VerticalUncertaintyM  float64               `json:"vertical_uncertainty_m"`
	VerticalReference     string                `json:"vertical_reference"`
	ValidationClass       ReliefValidationClass `json:"validation_class"`
	SamplingDesign        string                `json:"sampling_design"`
	ExcludedSources       []string              `json:"excluded_sources"`
	Limitations           []string              `json:"limitations"`
}

// ReliefQualityInputs содержит пути и контрольные суммы воспроизводимого
// запуска команды seabed validate.
type ReliefQualityInputs struct {
	EvaluatedMSH                string `json:"evaluated_msh"`
	EvaluatedMSHSHA256          string `json:"evaluated_msh_sha256"`
	EvaluatedMetadata           string `json:"evaluated_metadata"`
	EvaluatedMetadataSHA256     string `json:"evaluated_metadata_sha256"`
	ReferenceMSH                string `json:"reference_msh"`
	ReferenceMSHSHA256          string `json:"reference_msh_sha256"`
	ReferenceMetadata           string `json:"reference_metadata"`
	ReferenceMetadataSHA256     string `json:"reference_metadata_sha256"`
	ReferencePassport           string `json:"reference_passport"`
	ReferencePassportSHA256     string `json:"reference_passport_sha256"`
	TargetSizeField             string `json:"target_size_field"`
	TargetSizeFieldSHA256       string `json:"target_size_field_sha256"`
	TargetSizeFieldReport       string `json:"target_size_field_report"`
	TargetSizeFieldReportSHA256 string `json:"target_size_field_report_sha256"`
}

// ReliefQualityConfig задаёт контрольные изобаты, локальные ошибки и
// опорное поле размера. Размер и зоны индексируются как узлы модели.
type ReliefQualityConfig struct {
	IsobathsM           []float64
	WorstCellCount      int
	MaxNearestDistanceM float64
	TargetSizeM         []float64
	TargetZones         []string
	TargetZoneNames     map[string]string
}

// DepthErrorMetrics содержит точечные ошибки на центрах опорных ячеек.
type DepthErrorMetrics struct {
	EvaluationCellCount      int     `json:"evaluation_cell_count"`
	MissingCellCount         int     `json:"missing_cell_count"`
	NearestFallbackCellCount int     `json:"nearest_fallback_cell_count"`
	NearestFallbackPercent   float64 `json:"nearest_fallback_percent"`
	ReferenceMeanDepthM      float64 `json:"reference_mean_depth_m"`
	ReferenceP95DepthM       float64 `json:"reference_p95_depth_m"`
	EvaluatedMeanDepthM      float64 `json:"evaluated_mean_depth_m"`
	BiasM                    float64 `json:"bias_m"`
	MAEM                     float64 `json:"mae_m"`
	RMSEM                    float64 `json:"rmse_m"`
	AreaWeightedRMSEM        float64 `json:"area_weighted_rmse_m"`
	P95AbsoluteErrorM        float64 `json:"p95_absolute_error_m"`
}

// VolumePreservationMetrics описывает интегральный объём водной толщи.
type VolumePreservationMetrics struct {
	ReferenceKM3     float64 `json:"reference_km3"`
	EvaluatedKM3     float64 `json:"evaluated_km3"`
	DeviationKM3     float64 `json:"deviation_km3"`
	DeviationPercent float64 `json:"deviation_percent"`
}

// DepthBandPreservationMetrics описывает площадь между соседними изобатами.
// UpperDepthM=nil означает последнюю открытую глубоководную зону.
type DepthBandPreservationMetrics struct {
	LowerDepthM            float64  `json:"lower_depth_m"`
	UpperDepthM            *float64 `json:"upper_depth_m"`
	ReferenceAreaKM2       float64  `json:"reference_area_km2"`
	EvaluatedAreaKM2       float64  `json:"evaluated_area_km2"`
	AbsoluteDeviationKM2   float64  `json:"absolute_deviation_km2"`
	AbsoluteDeviationPct   float64  `json:"absolute_deviation_percent"`
	ResolutionTolerancePct float64  `json:"resolution_tolerance_percent"`
	Accepted               bool     `json:"accepted"`
}

// IsobathPreservationMetrics сравнивает геометрию одной изобаты в общей LAEA.
type IsobathPreservationMetrics struct {
	DepthM            float64 `json:"depth_m"`
	Comparable        bool    `json:"comparable"`
	ReferenceLengthKM float64 `json:"reference_length_km"`
	EvaluatedLengthKM float64 `json:"evaluated_length_km"`
	SampleCount       int     `json:"sample_count"`
	MeanDistanceM     float64 `json:"mean_distance_m"`
	P95DistanceM      float64 `json:"p95_distance_m"`
	MaxDistanceM      float64 `json:"max_distance_m"`
	Accepted          bool    `json:"accepted"`
	Reason            string  `json:"reason,omitempty"`
}

// SlopeErrorMetrics содержит ошибку угла наклона в контрольных ячейках.
type SlopeErrorMetrics struct {
	EvaluationCellCount int     `json:"evaluation_cell_count"`
	ReferenceP95Deg     float64 `json:"reference_p95_deg"`
	MAEDeg              float64 `json:"mae_deg"`
	RMSEDeg             float64 `json:"rmse_deg"`
}

// SamplingQualityMetrics фиксирует ближайшие замены во всей проверяемой модели.
type SamplingQualityMetrics struct {
	AssignedNodeCount  int     `json:"assigned_node_count"`
	NearestNodeCount   int     `json:"nearest_node_count"`
	NearestNodePercent float64 `json:"nearest_node_percent"`
	MaxSourceDistanceM float64 `json:"max_source_distance_m"`
}

// MeshRegionQualityMetrics агрегирует геометрическое качество ячеек по
// морфометрическим зонам BATHY-03.
type MeshRegionQualityMetrics struct {
	Region          CellRegion `json:"region"`
	CellCount       int        `json:"cell_count"`
	MeanQuadQuality float64    `json:"mean_quad_quality"`
	P05QuadQuality  float64    `json:"p05_quad_quality"`
}

// MeshReliefQualityMetrics объединяет форму ячеек и соответствие ADAPT-01.
type MeshReliefQualityMetrics struct {
	CellCount                   int                        `json:"cell_count"`
	MeanQuadQuality             float64                    `json:"mean_quad_quality"`
	P05QuadQuality              float64                    `json:"p05_quad_quality"`
	TargetSizeCompliancePercent float64                    `json:"target_size_compliance_percent"`
	Regions                     []MeshRegionQualityMetrics `json:"regions"`
	TargetZones                 []mesh.ZoneEdgeStatistics  `json:"target_zones"`
}

// WorstReliefCell хранит локальную контрольную точку с большой ошибкой.
type WorstReliefCell struct {
	ReferenceCellID int     `json:"reference_cell_id"`
	XM              float64 `json:"x_m"`
	YM              float64 `json:"y_m"`
	LongitudeDeg    float64 `json:"longitude_deg"`
	LatitudeDeg     float64 `json:"latitude_deg"`
	ReferenceDepthM float64 `json:"reference_depth_m"`
	EvaluatedDepthM float64 `json:"evaluated_depth_m"`
	AbsoluteErrorM  float64 `json:"absolute_error_m"`
}

// ReliefQualityThresholds фиксирует не только значения, но и формулы порогов.
type ReliefQualityThresholds struct {
	Method                     string  `json:"method"`
	DepthRMSEMaxM              float64 `json:"depth_rmse_max_m"`
	DepthP95AbsoluteErrorMaxM  float64 `json:"depth_p95_absolute_error_max_m"`
	VolumeDeviationMaxPercent  float64 `json:"volume_deviation_max_percent"`
	SlopeRMSEMaxDeg            float64 `json:"slope_rmse_max_deg"`
	IsobathP95DistanceMaxM     float64 `json:"isobath_p95_distance_max_m"`
	NearestNodeMaxPercent      float64 `json:"nearest_node_max_percent"`
	P05QuadQualityMin          float64 `json:"p05_quad_quality_min"`
	TargetSizeComplianceMinPct float64 `json:"target_size_compliance_min_percent"`
}

// ReliefQualityReport — полный машинный результат QA-02.
type ReliefQualityReport struct {
	SchemaVersion    string                         `json:"schema_version"`
	GeneratedAt      string                         `json:"generated_at"`
	Inputs           ReliefQualityInputs            `json:"inputs"`
	Reference        ReliefReferencePassport        `json:"reference"`
	Method           string                         `json:"method"`
	Depth            DepthErrorMetrics              `json:"depth"`
	Volume           VolumePreservationMetrics      `json:"volume"`
	DepthBands       []DepthBandPreservationMetrics `json:"depth_bands"`
	Isobaths         []IsobathPreservationMetrics   `json:"isobaths"`
	Slope            SlopeErrorMetrics              `json:"slope"`
	Sampling         SamplingQualityMetrics         `json:"sampling"`
	Mesh             MeshReliefQualityMetrics       `json:"mesh"`
	WorstCells       []WorstReliefCell              `json:"worst_cells"`
	Thresholds       ReliefQualityThresholds        `json:"thresholds"`
	MetricsAccepted  bool                           `json:"metrics_accepted"`
	PublicationReady bool                           `json:"publication_ready"`
	Reasons          []string                       `json:"reasons"`
	DurationSeconds  float64                        `json:"duration_seconds"`
}
