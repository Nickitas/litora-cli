package adaptive

const (
	// SchemaVersion задаёт версию машинного контракта отчёта ADAPT-01.
	SchemaVersion = "lito-adaptive-size-field/v1"
	// HorizontalUnit фиксирует метрическую единицу координат и расстояний.
	HorizontalUnit = "m"
)

// Config задаёт объяснимые пределы и чувствительность поля размера.
type Config struct {
	MinSizeM              float64 `json:"min_size_m"`
	StraightCoastSizeM    float64 `json:"straight_coast_size_m"`
	ShelfSizeM            float64 `json:"shelf_size_m"`
	DeepSizeM             float64 `json:"deep_size_m"`
	CoastInfluenceM       float64 `json:"coast_influence_m"`
	CurvatureReferenceDeg float64 `json:"curvature_reference_deg"`
	SlopeReferenceDeg     float64 `json:"slope_reference_deg"`
	FlatDeepMaxSlopeDeg   float64 `json:"flat_deep_max_slope_deg"`
	MaxNeighbourRatio     float64 `json:"max_neighbour_ratio"`
	MaxSizeGradientPerM   float64 `json:"max_size_gradient_per_m"`
}

// DefaultConfig возвращает базовую настройку ADAPT-01 для Чёрного моря:
// 200–300 м у берега, 500 м на шельфе и до 1000 м на ровном глубоководье.
func DefaultConfig() Config {
	return Config{
		MinSizeM:              200,
		StraightCoastSizeM:    300,
		ShelfSizeM:            500,
		DeepSizeM:             1000,
		CoastInfluenceM:       25_000,
		CurvatureReferenceDeg: 30,
		SlopeReferenceDeg:     10,
		FlatDeepMaxSlopeDeg:   1,
		MaxNeighbourRatio:     1.25,
		MaxSizeGradientPerM:   0.15,
	}
}

// NodeValue хранит исходные признаки, вклады формулы и итоговый размер в
// одном узле. Благодаря этому любое локальное уточнение можно объяснить без
// знания внутреннего состояния будущего генератора сетки.
type NodeValue struct {
	NodeID               int     `json:"node_id"`
	XM                   float64 `json:"x_m"`
	YM                   float64 `json:"y_m"`
	LongitudeDeg         float64 `json:"longitude_deg"`
	LatitudeDeg          float64 `json:"latitude_deg"`
	WaterDepthM          float64 `json:"water_depth_m"`
	DepthAvailable       bool    `json:"depth_available"`
	DistanceToCoastM     float64 `json:"distance_to_coast_m"`
	CoastCurvatureDeg    float64 `json:"coast_curvature_deg"`
	DepthGradientDeg     float64 `json:"depth_gradient_deg"`
	EffectiveGradientDeg float64 `json:"effective_gradient_deg"`
	GradientAvailable    bool    `json:"gradient_available"`
	BaseSizeM            float64 `json:"base_size_m"`
	DistanceRefinementM  float64 `json:"distance_refinement_m"`
	CurvatureRefinementM float64 `json:"curvature_refinement_m"`
	GradientRefinementM  float64 `json:"gradient_refinement_m"`
	RawTargetSizeM       float64 `json:"raw_target_size_m"`
	TargetSizeM          float64 `json:"target_size_m"`
	GrowthLimited        bool    `json:"growth_limited"`
	Zone                 string  `json:"zone"`
}

// SizeStatistics описывает распределение размера в метрах.
type SizeStatistics struct {
	MinM    float64 `json:"min_m"`
	P05M    float64 `json:"p05_m"`
	MedianM float64 `json:"median_m"`
	MeanM   float64 `json:"mean_m"`
	P95M    float64 `json:"p95_m"`
	MaxM    float64 `json:"max_m"`
}

// ZoneSummary объясняет итоговый диапазон для одной морфометрической зоны.
type ZoneSummary struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	NodeCount int            `json:"node_count"`
	Target    SizeStatistics `json:"target_size_m"`
}

// Summary содержит проверяемые критерии полноты и плавности поля.
type Summary struct {
	NodeCount                int            `json:"node_count"`
	CellCount                int            `json:"cell_count"`
	CoastlineSegmentCount    int            `json:"coastline_segment_count"`
	NoDataNodeCount          int            `json:"no_data_node_count"`
	GrowthLimitedNodeCount   int            `json:"growth_limited_node_count"`
	RawTarget                SizeStatistics `json:"raw_target_size_m"`
	Target                   SizeStatistics `json:"target_size_m"`
	RawMaxAdjacentRatio      float64        `json:"raw_max_adjacent_ratio"`
	FinalMaxAdjacentRatio    float64        `json:"final_max_adjacent_ratio"`
	RawMaxSizeGradientPerM   float64        `json:"raw_max_size_gradient_per_m"`
	FinalMaxSizeGradientPerM float64        `json:"final_max_size_gradient_per_m"`
	MaxDistanceToCoastM      float64        `json:"max_distance_to_coast_m"`
	MaxCoastCurvatureDeg     float64        `json:"max_coast_curvature_deg"`
	MaxDepthGradientDeg      float64        `json:"max_depth_gradient_deg"`
	Zones                    []ZoneSummary  `json:"zones"`
}

// Formula фиксирует смысл каждого шага расчёта в машинном отчёте.
type Formula struct {
	BaseSize       string `json:"base_size"`
	CoastDistance  string `json:"coast_distance"`
	CoastCurvature string `json:"coast_curvature"`
	DepthGradient  string `json:"depth_gradient"`
	RawTarget      string `json:"raw_target"`
	GrowthLimit    string `json:"growth_limit"`
}

// Report является отдельным отчётом ADAPT-01. Пути и источник заполняет CLI
// после расчёта, а числовые сводки формируются только доменным алгоритмом.
type Report struct {
	SchemaVersion         string  `json:"schema_version"`
	GeneratedAt           string  `json:"generated_at,omitempty"`
	InputMSH              string  `json:"input_msh,omitempty"`
	FieldCSV              string  `json:"field_csv,omitempty"`
	MapSVG                string  `json:"map_svg,omitempty"`
	BathymetrySource      string  `json:"bathymetry_source,omitempty"`
	BathymetrySHA256      string  `json:"bathymetry_sha256,omitempty"`
	HorizontalReference   string  `json:"horizontal_reference"`
	HorizontalUnit        string  `json:"horizontal_unit"`
	TargetSizeUnit        string  `json:"target_size_unit"`
	AdaptiveMeshGenerated bool    `json:"adaptive_mesh_generated"`
	Config                Config  `json:"config"`
	Formula               Formula `json:"formula"`
	Summary               Summary `json:"summary"`
}

// Field содержит отсортированные по идентификатору узловые значения и отчёт.
type Field struct {
	Nodes  []NodeValue `json:"-"`
	Report Report      `json:"report"`
}

// TargetSizeField — минимальное представление результата ADAPT-01, которое
// требуется генератору ADAPT-02. Индекс с нулём не используется, поэтому
// идентификатор узла совпадает с индексом в TargetSizeM и Zones.
type TargetSizeField struct {
	TargetSizeM []float64
	Zones       []string
	NodeCount   int
	MinSizeM    float64
	MaxSizeM    float64
}

// ZoneRussianName возвращает устойчивое русское название зоны ADAPT-01.
func ZoneRussianName(id string) string {
	return zoneName(id)
}
