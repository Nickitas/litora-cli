package seabed

import "coastal-geometry/internal/domain/mesh"

// SamplingMethod задаёт способ назначения отметки узлу расчётной сетки.
type SamplingMethod string

const (
	// SamplingNotSampled означает, что значение не назначено.
	SamplingNotSampled SamplingMethod = "not_sampled"
	// SamplingExact означает совпадение с исходной батиметрической точкой.
	SamplingExact SamplingMethod = "exact"
	// SamplingBilinear означает билинейную интерполяцию регулярной сетки.
	SamplingBilinear SamplingMethod = "bilinear"
	// SamplingNearest означает замену ближайшей исходной точкой.
	SamplingNearest SamplingMethod = "nearest"
	// SamplingIrregular означает интерполяцию нерегулярного набора.
	SamplingIrregular SamplingMethod = "irregular"
	// SamplingCoastlineConstraint означает явное береговое условие Z = 0.
	SamplingCoastlineConstraint SamplingMethod = "coastline_constraint"
)

// QualityFlag задаёт пригодность и происхождение узлового значения.
type QualityFlag string

const (
	// QualityRejected означает, что исходное значение отвергнуто.
	QualityRejected QualityFlag = "rejected"
	// QualityNoData означает отсутствие значения без подстановки нуля.
	QualityNoData QualityFlag = "no_data"
	// QualityFallback означает использование ближайшей замены.
	QualityFallback QualityFlag = "fallback"
	// QualityConstrained означает явную береговую или переходную коррекцию.
	QualityConstrained QualityFlag = "constrained"
	// QualityVerified означает принятую точную или интерполированную выборку.
	QualityVerified QualityFlag = "verified"
)

// BoundaryKind задаёт роль узла на границе расчётной области.
type BoundaryKind string

const (
	// BoundaryNone означает внутренний узел.
	BoundaryNone BoundaryKind = "none"
	// BoundaryCoastline означает внешний берег акватории.
	BoundaryCoastline BoundaryKind = "coastline"
	// BoundaryIsland означает берег островного отверстия.
	BoundaryIsland BoundaryKind = "island"
	// BoundaryOpen означает открытую расчётную границу локального участка.
	BoundaryOpen BoundaryKind = "open_boundary"
)

// CorrectionKind задаёт причину изменения или отклонения узловой отметки.
type CorrectionKind string

const (
	// CorrectionCoastlineZero фиксирует назначение нуля берегу или острову.
	CorrectionCoastlineZero CorrectionKind = "coastline_zero"
	// CorrectionCoastTransition фиксирует плавную прибрежную коррекцию.
	CorrectionCoastTransition CorrectionKind = "coast_transition"
	// CorrectionPositiveInsideRejected фиксирует положительную отметку в воде.
	CorrectionPositiveInsideRejected CorrectionKind = "positive_inside_rejected"
	// CorrectionOutsideAquatoryRejected фиксирует узел вне расчётных ячеек.
	CorrectionOutsideAquatoryRejected CorrectionKind = "outside_aquatory_rejected"
)

// Sample содержит одну выборку источника батиметрии.
type Sample struct {
	ElevationM        float64
	Method            SamplingMethod
	SourceDistanceM   float64
	SourceDistanceSet bool
}

// ElevationSampler отделяет построение модели дна от формата источника.
// Реальный источник GEBCO является регулярным, а интерфейс оставляет явную
// точку расширения для нерегулярных измерений без изменения алгоритма BATHY-02.
type ElevationSampler interface {
	SampleElevation(latitudeDeg, longitudeDeg, maxSourceDistanceM float64) (Sample, error)
}

// BoundaryOverride явно задаёт тип граничного ребра. Он нужен прежде всего для
// открытых сторон локальных моделей, которые топологически могут образовывать
// одно замкнутое кольцо вместе с берегом.
type BoundaryOverride struct {
	NodeA int
	NodeB int
	Kind  BoundaryKind
}

// BoundaryEdge хранит граничное ребро вместе с его физической ролью. Отдельная
// запись на уровне модели нужна для переноса групп берега, островов и открытых
// границ в MSH без неоднозначного восстановления по узловым признакам.
type BoundaryEdge struct {
	NodeIDs [2]int       `json:"node_ids"`
	Kind    BoundaryKind `json:"kind"`
}

// BuildConfig управляет привязкой глубины и береговой коррекцией.
type BuildConfig struct {
	MaxSourceDistanceM    float64
	CoastTransitionWidthM float64
	BoundaryOverrides     []BoundaryOverride
	RegionThresholds      RegionThresholds
}

// Node представляет узел батиметрической модели по контракту lito-seabed/v1.
// Указатели сохраняют различие между физическим нулём и отсутствием данных.
type Node struct {
	ID              int            `json:"id"`
	XM              float64        `json:"x_m"`
	YM              float64        `json:"y_m"`
	LongitudeDeg    float64        `json:"longitude_deg"`
	LatitudeDeg     float64        `json:"latitude_deg"`
	ElevationM      *float64       `json:"elevation_m"`
	WaterDepthM     *float64       `json:"water_depth_m"`
	SamplingMethod  SamplingMethod `json:"sampling_method"`
	SourceDistanceM *float64       `json:"source_distance_m"`
	QualityFlag     QualityFlag    `json:"quality_flag"`
	IsBoundary      bool           `json:"is_boundary"`
	BoundaryKind    BoundaryKind   `json:"boundary_kind"`
}

// Correction хранит полную запись одной коррекции для отдельного CSV-журнала.
type Correction struct {
	NodeID              int            `json:"node_id"`
	Kind                CorrectionKind `json:"kind"`
	OriginalElevationM  *float64       `json:"original_elevation_m"`
	CorrectedElevationM *float64       `json:"corrected_elevation_m"`
	AdjustmentM         *float64       `json:"adjustment_m"`
	Reason              string         `json:"reason"`
}

// MethodCounts подсчитывает способы назначения узловых значений.
type MethodCounts struct {
	Exact               int `json:"exact"`
	Bilinear            int `json:"bilinear"`
	Nearest             int `json:"nearest"`
	Irregular           int `json:"irregular"`
	CoastlineConstraint int `json:"coastline_constraint"`
	NotSampled          int `json:"not_sampled"`
}

// SamplingSummary описывает полноту и расстояния привязки батиметрии.
type SamplingSummary struct {
	TotalNodeCount      int          `json:"total_node_count"`
	AssignedNodeCount   int          `json:"assigned_node_count"`
	NoDataNodeCount     int          `json:"no_data_node_count"`
	CoveragePercent     float64      `json:"coverage_percent"`
	MethodCounts        MethodCounts `json:"method_counts"`
	MeanSourceDistanceM *float64     `json:"mean_source_distance_m"`
	MaxSourceDistanceM  *float64     `json:"max_source_distance_m"`
}

// BoundaryCounts подсчитывает узлы каждого вида границы.
type BoundaryCounts struct {
	Coastline    int `json:"coastline"`
	Island       int `json:"island"`
	OpenBoundary int `json:"open_boundary"`
}

// CorrectionCounts подсчитывает каждую причину исправления или отклонения.
type CorrectionCounts struct {
	CoastlineZero           int `json:"coastline_zero"`
	CoastTransition         int `json:"coast_transition"`
	PositiveInsideRejected  int `json:"positive_inside_rejected"`
	OutsideAquatoryRejected int `json:"outside_aquatory_rejected"`
}

// ReconciliationSummary описывает согласование береговой и батиметрической
// масок. DeepenedNodeCount обязан оставаться нулевым: коррекция может только
// приблизить отрицательную отметку к береговому нулю.
type ReconciliationSummary struct {
	TransitionWidthM  float64          `json:"transition_width_m"`
	BoundaryCounts    BoundaryCounts   `json:"boundary_counts"`
	CorrectionCounts  CorrectionCounts `json:"correction_counts"`
	TotalCorrections  int              `json:"total_correction_count"`
	DeepenedNodeCount int              `json:"deepened_node_count"`
	MaxAbsAdjustmentM float64          `json:"max_abs_adjustment_m"`
	Corrections       []Correction     `json:"corrections"`
}

// CellRegion задаёт морфометрический класс ячейки по её положительной глубине.
type CellRegion string

const (
	// RegionCoast означает береговую или мелководную ячейку.
	RegionCoast CellRegion = "coast"
	// RegionShelf означает ячейку континентального шельфа.
	RegionShelf CellRegion = "shelf"
	// RegionSlope означает ячейку материкового склона.
	RegionSlope CellRegion = "slope"
	// RegionBasin означает глубоководную ячейку котловины.
	RegionBasin CellRegion = "basin"
	// RegionUnclassified означает отсутствие принятой классификации.
	RegionUnclassified CellRegion = "unclassified"
)

// CellQualityFlag агрегирует качество четырёх узлов ячейки.
type CellQualityFlag string

const (
	// CellQualityVerified означает четыре проверенные узловые выборки.
	CellQualityVerified CellQualityFlag = "verified"
	// CellQualityMixed означает смесь проверенных, ограниченных или запасных значений.
	CellQualityMixed CellQualityFlag = "mixed"
	// CellQualityFallback означает четыре ближайшие запасные выборки.
	CellQualityFallback CellQualityFlag = "fallback"
)

// RegionThresholds задаёт верхние границы морфометрических зон по средней
// положительной глубине ячейки. Значения должны строго возрастать.
type RegionThresholds struct {
	CoastMaxDepthM float64 `json:"coast_max_depth_m"`
	ShelfMaxDepthM float64 `json:"shelf_max_depth_m"`
	SlopeMaxDepthM float64 `json:"slope_max_depth_m"`
}

// DefaultRegionThresholds возвращает исходные пороги для Чёрного моря:
// 20 м для прибрежной зоны, 200 м для шельфа и 2000 м для склона.
func DefaultRegionThresholds() RegionThresholds {
	return RegionThresholds{CoastMaxDepthM: 20, ShelfMaxDepthM: 200, SlopeMaxDepthM: 2000}
}

// Cell содержит производные характеристики принятой четырёхугольной ячейки.
type Cell struct {
	ID              int             `json:"id"`
	NodeIDs         [4]int          `json:"node_ids"`
	AreaM2          float64         `json:"area_m2"`
	ElevationMinM   float64         `json:"elevation_min_m"`
	ElevationMaxM   float64         `json:"elevation_max_m"`
	ElevationMeanM  float64         `json:"elevation_mean_m"`
	WaterDepthMeanM float64         `json:"water_depth_mean_m"`
	SlopeDeg        float64         `json:"slope_deg"`
	AspectDeg       *float64        `json:"aspect_deg"`
	RoughnessM      float64         `json:"roughness_m"`
	Region          CellRegion      `json:"region"`
	QualityFlag     CellQualityFlag `json:"quality_flag"`
	QualityScore    float64         `json:"quality_score"`
}

// RegionCounts подсчитывает принятые ячейки в каждой морфометрической зоне.
type RegionCounts struct {
	Coast        int `json:"coast"`
	Shelf        int `json:"shelf"`
	Slope        int `json:"slope"`
	Basin        int `json:"basin"`
	Unclassified int `json:"unclassified"`
}

// CellQualityCounts подсчитывает категории качества принятых ячеек.
type CellQualityCounts struct {
	Verified int `json:"verified"`
	Mixed    int `json:"mixed"`
	Fallback int `json:"fallback"`
}

// CellSummary описывает покрытие и диапазоны производных характеристик.
type CellSummary struct {
	TotalCellCount    int               `json:"total_cell_count"`
	AssignedCellCount int               `json:"assigned_cell_count"`
	NoDataCellCount   int               `json:"no_data_cell_count"`
	CoveragePercent   float64           `json:"coverage_percent"`
	RegionCounts      RegionCounts      `json:"region_counts"`
	QualityCounts     CellQualityCounts `json:"quality_counts"`
	MeanSlopeDeg      *float64          `json:"mean_slope_deg"`
	MaxSlopeDeg       *float64          `json:"max_slope_deg"`
	MeanRoughnessM    *float64          `json:"mean_roughness_m"`
	MaxRoughnessM     *float64          `json:"max_roughness_m"`
}

// CellDerivationMetadata фиксирует единицы, формулы и пороги BATHY-03.
type CellDerivationMetadata struct {
	AreaMethod        string           `json:"area_method"`
	ElevationMethod   string           `json:"elevation_method"`
	SlopeAspectMethod string           `json:"slope_aspect_method"`
	RoughnessMethod   string           `json:"roughness_method"`
	RegionMethod      string           `json:"region_method"`
	HorizontalUnit    string           `json:"horizontal_unit"`
	ElevationUnit     string           `json:"elevation_unit"`
	SlopeUnit         string           `json:"slope_unit"`
	AspectConvention  string           `json:"aspect_convention"`
	RegionThresholds  RegionThresholds `json:"region_thresholds"`
	Summary           CellSummary      `json:"summary"`
}

// Model содержит результат BATHY-01/BATHY-02/BATHY-03. Nodes сохраняет
// индексацию MSH: элемент 0 не используется, идентификатор узла совпадает с
// индексом. Cells содержит только ячейки с полным узловым покрытием.
type Model struct {
	Mesh           mesh.Mesh              `json:"-"`
	Nodes          []Node                 `json:"-"`
	Cells          []Cell                 `json:"-"`
	BoundaryEdges  []BoundaryEdge         `json:"-"`
	Sampling       SamplingSummary        `json:"sampling"`
	Reconciliation ReconciliationSummary  `json:"reconciliation"`
	CellDerivation CellDerivationMetadata `json:"cell_derivation"`
	Accepted       bool                   `json:"accepted"`
	Reasons        []string               `json:"reasons"`
}
