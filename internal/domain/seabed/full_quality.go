package seabed

import (
	"fmt"
	"math"
	"sort"
	"time"

	"coastal-geometry/internal/domain/mesh"
)

// FullBlackSeaQualitySchemaVersion задаёт машинный контракт отчёта QA-03.
const FullBlackSeaQualitySchemaVersion = "lito-black-sea-full-quality/v1"

// GeographicBounds задаёт географический охват полного контура в WGS 84.
type GeographicBounds struct {
	MinLatitudeDeg  float64 `json:"min_latitude_deg"`
	MaxLatitudeDeg  float64 `json:"max_latitude_deg"`
	MinLongitudeDeg float64 `json:"min_longitude_deg"`
	MaxLongitudeDeg float64 `json:"max_longitude_deg"`
}

// PublishedBasinReference хранит опубликованные интегральные ориентиры моря.
// Primary означает источник, относительно которого вычисляется основной
// научный шлюз QA-03; остальные источники сохраняются для объяснения разброса.
type PublishedBasinReference struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Citation  string  `json:"citation"`
	URL       string  `json:"url"`
	AreaKM2   float64 `json:"area_km2"`
	VolumeKM3 float64 `json:"volume_km3"`
	MaxDepthM float64 `json:"max_depth_m,omitempty"`
	Primary   bool    `json:"primary"`
}

// DefaultBlackSeaPublishedReferences возвращает зафиксированные ориентиры без
// Азовского моря. Значения намеренно не усредняются: отчёт показывает
// отклонение от каждого определения акватории отдельно.
func DefaultBlackSeaPublishedReferences() []PublishedBasinReference {
	return []PublishedBasinReference{
		{
			ID:        "black-sea-commission-geography",
			Title:     "География Чёрного моря",
			Citation:  "Commission on the Protection of the Black Sea Against Pollution: Area of Water Surface 432 000 km²; Water volume 547 000 km³",
			URL:       "https://www.blackseacommission.org/The%20Black%20Sea/Geography/",
			AreaKM2:   432_000,
			VolumeKM3: 547_000,
			MaxDepthM: 2_212,
			Primary:   true,
		},
		{
			ID:        "boudreau-2021-pa004242",
			Title:     "Inverse Modeling of the Net Water Balance of the Black Sea",
			Citation:  "Boudreau et al. (2021), Paleoceanography and Paleoclimatology, doi:10.1029/2021PA004242",
			URL:       "https://doi.org/10.1029/2021PA004242",
			AreaKM2:   423_000,
			VolumeKM3: 547_000,
			MaxDepthM: 2_210,
		},
	}
}

// FullBlackSeaQualityConfig задаёт воспроизводимые пороги полного прогона.
type FullBlackSeaQualityConfig struct {
	TargetEdgeM                       float64
	MeanEdgeTolerancePercent          float64
	ExpectedBounds                    GeographicBounds
	ExtentToleranceDeg                float64
	CoastlineReferenceAreaKM2         float64
	CoastlineAreaTolerancePercent     float64
	NearestFallbackMaxPercent         float64
	LongFallbackWarningM              float64
	PublishedAreaTolerancePercent     float64
	PublishedVolumeTolerancePercent   float64
	PublishedDepthTolerancePercent    float64
	PublishedReferences               []PublishedBasinReference
	IndependentReliefValidationPassed bool
}

// DefaultFullBlackSeaQualityConfig создаёт научные шлюзы QA-03 для сетки
// 1000 м. Допуски площади и объёма учитывают различие определений границы,
// береговое упрощение и разрешение глобального батиметрического продукта.
func DefaultFullBlackSeaQualityConfig() FullBlackSeaQualityConfig {
	return FullBlackSeaQualityConfig{
		TargetEdgeM:                     1_000,
		MeanEdgeTolerancePercent:        35,
		ExtentToleranceDeg:              0.05,
		CoastlineAreaTolerancePercent:   1.5,
		NearestFallbackMaxPercent:       5,
		LongFallbackWarningM:            5_000,
		PublishedAreaTolerancePercent:   5,
		PublishedVolumeTolerancePercent: 10,
		PublishedDepthTolerancePercent:  10,
		PublishedReferences:             DefaultBlackSeaPublishedReferences(),
	}
}

// FullBlackSeaInput фиксирует путь, размер и контрольную сумму одного входа.
type FullBlackSeaInput struct {
	Role        string `json:"role"`
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	SizeBytes   int64  `json:"size_bytes"`
	DataVersion string `json:"data_version,omitempty"`
}

// FullBlackSeaRunStage хранит длительность и наблюдавшийся пик памяти этапа.
type FullBlackSeaRunStage struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	DurationSeconds float64 `json:"duration_seconds"`
	HeapInUseBytes  uint64  `json:"heap_in_use_bytes"`
	SystemBytes     uint64  `json:"system_bytes"`
}

// FullBlackSeaResources описывает программную среду и ресурсы полного запуска.
type FullBlackSeaResources struct {
	StartedAt          string                 `json:"started_at"`
	FinishedAt         string                 `json:"finished_at"`
	DurationSeconds    float64                `json:"duration_seconds"`
	PeakHeapInUseBytes uint64                 `json:"peak_heap_in_use_bytes"`
	PeakSystemBytes    uint64                 `json:"peak_system_bytes"`
	GoVersion          string                 `json:"go_version"`
	LitoRevision       string                 `json:"lito_revision,omitempty"`
	GmshVersion        string                 `json:"gmsh_version"`
	Stages             []FullBlackSeaRunStage `json:"stages"`
}

// FullBlackSeaArtifacts перечисляет проверяемые результаты одного запуска.
type FullBlackSeaArtifacts struct {
	Directory          string `json:"directory"`
	BathymetricMSH     string `json:"bathymetric_msh"`
	VTU                string `json:"vtu"`
	NodesCSV           string `json:"nodes_csv"`
	CellsCSV           string `json:"cells_csv"`
	ProfilesCSV        string `json:"profiles_csv"`
	ExportMetadataJSON string `json:"export_metadata_json"`
	ReconciliationJSON string `json:"reconciliation_json"`
	CorrectionsCSV     string `json:"corrections_csv"`
	OverviewSVG        string `json:"overview_svg"`
	MeshDetailsSVG     string `json:"mesh_details_svg"`
	Relief3DSVG        string `json:"relief_3d_svg"`
	ProfilesSVG        string `json:"profiles_svg"`
	VisualizationJSON  string `json:"visualization_json"`
	QualityJSON        string `json:"quality_json"`
	QualityTSV         string `json:"quality_tsv"`
}

// FullBlackSeaExtentReport проверяет, что модель покрывает тот же полный
// географический контур, что и исходный полигон MarineRegions.
type FullBlackSeaExtentReport struct {
	ActualBounds           GeographicBounds `json:"actual_bounds"`
	ExpectedBounds         GeographicBounds `json:"expected_bounds"`
	ToleranceDeg           float64          `json:"tolerance_deg"`
	MaxDeviationDeg        float64          `json:"max_deviation_deg"`
	InvalidCoordinateCount int              `json:"invalid_coordinate_count"`
	Accepted               bool             `json:"accepted"`
}

// FullBlackSeaTopologyReport описывает целостность четырёхугольного каркаса.
type FullBlackSeaTopologyReport struct {
	NodeCount                   int  `json:"node_count"`
	CellCount                   int  `json:"cell_count"`
	QuadCount                   int  `json:"quad_count"`
	TriangleCount               int  `json:"triangle_count"`
	BoundaryEdgeCount           int  `json:"boundary_edge_count"`
	CoastlineComponentCount     int  `json:"coastline_component_count"`
	IslandComponentCount        int  `json:"island_component_count"`
	OpenBoundaryEdgeCount       int  `json:"open_boundary_edge_count"`
	CellComponentCount          int  `json:"cell_component_count"`
	UnexpectedBoundaryEdgeCount int  `json:"unexpected_boundary_edge_count"`
	MissingBoundaryEdgeCount    int  `json:"missing_boundary_edge_count"`
	DuplicateBoundaryEdgeCount  int  `json:"duplicate_boundary_edge_count"`
	NonManifoldEdgeCount        int  `json:"non_manifold_edge_count"`
	DegenerateCellCount         int  `json:"degenerate_cell_count"`
	SelfIntersectingCellCount   int  `json:"self_intersecting_cell_count"`
	BoundaryIntersectionCount   int  `json:"boundary_intersection_count"`
	Accepted                    bool `json:"accepted"`
}

// FullBlackSeaEdgeReport подтверждает фактический масштаб контрольной сетки.
type FullBlackSeaEdgeReport struct {
	TargetEdgeM             float64 `json:"target_edge_m"`
	UniqueEdgeCount         int     `json:"unique_edge_count"`
	MinEdgeM                float64 `json:"min_edge_m"`
	P05EdgeM                float64 `json:"p05_edge_m"`
	MeanEdgeM               float64 `json:"mean_edge_m"`
	P95EdgeM                float64 `json:"p95_edge_m"`
	MaxEdgeM                float64 `json:"max_edge_m"`
	MeanDeviationPercent    float64 `json:"mean_deviation_percent"`
	AllowedDeviationPercent float64 `json:"allowed_deviation_percent"`
	Accepted                bool    `json:"accepted"`
}

// FullBlackSeaDepthReport проверяет полноту, конечность и знак глубин.
type FullBlackSeaDepthReport struct {
	AssignedNodeCount         int     `json:"assigned_node_count"`
	NoDataNodeCount           int     `json:"no_data_node_count"`
	NoDataCellCount           int     `json:"no_data_cell_count"`
	NonFiniteValueCount       int     `json:"non_finite_value_count"`
	PositiveElevationCount    int     `json:"positive_elevation_count"`
	NegativeWaterDepthCount   int     `json:"negative_water_depth_count"`
	InconsistentSignCount     int     `json:"inconsistent_sign_count"`
	NearestFallbackNodeCount  int     `json:"nearest_fallback_node_count"`
	NearestFallbackPercent    float64 `json:"nearest_fallback_percent"`
	NearestFallbackMaxPercent float64 `json:"nearest_fallback_max_percent"`
	MaxSourceDistanceM        float64 `json:"max_source_distance_m"`
	LongFallbackWarningM      float64 `json:"long_fallback_warning_m"`
	LongFallbackNodeCount     int     `json:"long_fallback_node_count"`
	LongFallbackPercent       float64 `json:"long_fallback_percent"`
	MinElevationM             float64 `json:"min_elevation_m"`
	MaxElevationM             float64 `json:"max_elevation_m"`
	MaxWaterDepthM            float64 `json:"max_water_depth_m"`
	Accepted                  bool    `json:"accepted"`
}

// FullBlackSeaIntegralReport хранит площадь, объём и среднюю глубину модели.
type FullBlackSeaIntegralReport struct {
	AreaKM2                       float64 `json:"area_km2"`
	VolumeKM3                     float64 `json:"volume_km3"`
	MeanDepthM                    float64 `json:"mean_depth_m"`
	CoastlineReferenceAreaKM2     float64 `json:"coastline_reference_area_km2"`
	CoastlineAreaDeviationPercent float64 `json:"coastline_area_deviation_percent"`
	CoastlineAreaTolerancePercent float64 `json:"coastline_area_tolerance_percent"`
	CoastlineAreaAccepted         bool    `json:"coastline_area_accepted"`
}

// PublishedBasinComparison сохраняет отклонение от одного опубликованного
// ориентира без усреднения взаимоисключающих определений границы моря.
type PublishedBasinComparison struct {
	Reference              PublishedBasinReference `json:"reference"`
	AreaDeviationPercent   float64                 `json:"area_deviation_percent"`
	VolumeDeviationPercent float64                 `json:"volume_deviation_percent"`
	DepthDeviationPercent  float64                 `json:"depth_deviation_percent,omitempty"`
	AreaAccepted           bool                    `json:"area_accepted"`
	VolumeAccepted         bool                    `json:"volume_accepted"`
	DepthAccepted          bool                    `json:"depth_accepted"`
	Accepted               bool                    `json:"accepted"`
}

// FullBlackSeaQualityReport объединяет научные и инженерные проверки QA-03.
// Accepted относится к воспроизводимости модели; PublicationReady дополнительно
// требует независимой проверки рельефа из QA-02.
type FullBlackSeaQualityReport struct {
	SchemaVersion        string                     `json:"schema_version"`
	GeneratedAt          string                     `json:"generated_at"`
	Scope                string                     `json:"scope"`
	Inputs               []FullBlackSeaInput        `json:"inputs"`
	Extent               FullBlackSeaExtentReport   `json:"extent"`
	Topology             FullBlackSeaTopologyReport `json:"topology"`
	EdgeSize             FullBlackSeaEdgeReport     `json:"edge_size"`
	Depth                FullBlackSeaDepthReport    `json:"depth"`
	Integrals            FullBlackSeaIntegralReport `json:"integrals"`
	PublishedComparisons []PublishedBasinComparison `json:"published_comparisons"`
	Resources            FullBlackSeaResources      `json:"resources"`
	Artifacts            FullBlackSeaArtifacts      `json:"artifacts"`
	Accepted             bool                       `json:"accepted"`
	PublicationReady     bool                       `json:"publication_ready"`
	Reasons              []string                   `json:"reasons"`
	PublicationReasons   []string                   `json:"publication_reasons"`
	Warnings             []string                   `json:"warnings"`
}

type fullEdgeUsage struct {
	count     int
	firstCell int
}

// EvaluateFullBlackSeaQuality выполняет проверки QA-03 по полной модели без
// чтения файлов и визуальной субъективности.
func EvaluateFullBlackSeaQuality(model Model, config FullBlackSeaQualityConfig) (FullBlackSeaQualityReport, error) {
	config, err := normalizeFullBlackSeaQualityConfig(config)
	if err != nil {
		return FullBlackSeaQualityReport{}, err
	}
	report := FullBlackSeaQualityReport{
		SchemaVersion:      FullBlackSeaQualitySchemaVersion,
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		Scope:              "полный контур Чёрного моря без Азовского моря",
		Reasons:            make([]string, 0),
		PublicationReasons: make([]string, 0),
		Warnings:           make([]string, 0),
	}
	report.Extent = evaluateFullExtent(model, config)
	report.Topology, report.EdgeSize = evaluateFullTopology(model, config)
	report.Depth = evaluateFullDepth(model, config)
	report.Integrals = evaluateFullIntegrals(model, config)
	for _, reference := range config.PublishedReferences {
		report.PublishedComparisons = append(report.PublishedComparisons, comparePublishedReference(report, reference, config))
	}
	report.Accepted = report.Extent.Accepted && report.Topology.Accepted && report.EdgeSize.Accepted &&
		report.Depth.Accepted && report.Integrals.CoastlineAreaAccepted && publishedComparisonsAccepted(report.PublishedComparisons)
	if !model.Accepted {
		report.Accepted = false
		report.Reasons = append(report.Reasons, model.Reasons...)
	}
	appendFullQualityReasons(&report)
	report.PublicationReady = report.Accepted && config.IndependentReliefValidationPassed
	if !config.IndependentReliefValidationPassed {
		report.PublicationReasons = append(report.PublicationReasons,
			"не приложена независимая проверка рельефа класса independent_measurements из QA-02")
	}
	return report, nil
}

func normalizeFullBlackSeaQualityConfig(config FullBlackSeaQualityConfig) (FullBlackSeaQualityConfig, error) {
	defaults := DefaultFullBlackSeaQualityConfig()
	if config.TargetEdgeM == 0 {
		config.TargetEdgeM = defaults.TargetEdgeM
	}
	if config.MeanEdgeTolerancePercent == 0 {
		config.MeanEdgeTolerancePercent = defaults.MeanEdgeTolerancePercent
	}
	if config.ExtentToleranceDeg == 0 {
		config.ExtentToleranceDeg = defaults.ExtentToleranceDeg
	}
	if config.CoastlineAreaTolerancePercent == 0 {
		config.CoastlineAreaTolerancePercent = defaults.CoastlineAreaTolerancePercent
	}
	if config.NearestFallbackMaxPercent == 0 {
		config.NearestFallbackMaxPercent = defaults.NearestFallbackMaxPercent
	}
	if config.LongFallbackWarningM == 0 {
		config.LongFallbackWarningM = defaults.LongFallbackWarningM
	}
	if config.PublishedAreaTolerancePercent == 0 {
		config.PublishedAreaTolerancePercent = defaults.PublishedAreaTolerancePercent
	}
	if config.PublishedVolumeTolerancePercent == 0 {
		config.PublishedVolumeTolerancePercent = defaults.PublishedVolumeTolerancePercent
	}
	if config.PublishedDepthTolerancePercent == 0 {
		config.PublishedDepthTolerancePercent = defaults.PublishedDepthTolerancePercent
	}
	if len(config.PublishedReferences) == 0 {
		config.PublishedReferences = defaults.PublishedReferences
	}
	values := []float64{config.TargetEdgeM, config.MeanEdgeTolerancePercent, config.ExtentToleranceDeg,
		config.CoastlineReferenceAreaKM2, config.CoastlineAreaTolerancePercent,
		config.NearestFallbackMaxPercent, config.LongFallbackWarningM,
		config.PublishedAreaTolerancePercent, config.PublishedVolumeTolerancePercent, config.PublishedDepthTolerancePercent}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return FullBlackSeaQualityConfig{}, fmt.Errorf("параметры QA-03 должны быть конечными положительными числами")
		}
	}
	if config.ExpectedBounds.MinLatitudeDeg >= config.ExpectedBounds.MaxLatitudeDeg ||
		config.ExpectedBounds.MinLongitudeDeg >= config.ExpectedBounds.MaxLongitudeDeg {
		return FullBlackSeaQualityConfig{}, fmt.Errorf("ожидаемые границы полного контура QA-03 некорректны")
	}
	for _, reference := range config.PublishedReferences {
		if reference.ID == "" || reference.URL == "" || reference.AreaKM2 <= 0 || reference.VolumeKM3 <= 0 {
			return FullBlackSeaQualityConfig{}, fmt.Errorf("опубликованный ориентир QA-03 заполнен не полностью")
		}
	}
	return config, nil
}

func evaluateFullExtent(model Model, config FullBlackSeaQualityConfig) FullBlackSeaExtentReport {
	report := FullBlackSeaExtentReport{ExpectedBounds: config.ExpectedBounds, ToleranceDeg: config.ExtentToleranceDeg}
	report.ActualBounds = GeographicBounds{
		MinLatitudeDeg: math.Inf(1), MaxLatitudeDeg: math.Inf(-1),
		MinLongitudeDeg: math.Inf(1), MaxLongitudeDeg: math.Inf(-1),
	}
	for nodeID := 1; nodeID < len(model.Mesh.Nodes); nodeID++ {
		point := model.Mesh.Nodes[nodeID]
		if !point.GeographicCoordinatesSet || !finite(point.LatitudeDeg) || !finite(point.LongitudeDeg) ||
			point.LatitudeDeg < -90 || point.LatitudeDeg > 90 || point.LongitudeDeg < -180 || point.LongitudeDeg > 180 {
			report.InvalidCoordinateCount++
			continue
		}
		report.ActualBounds.MinLatitudeDeg = math.Min(report.ActualBounds.MinLatitudeDeg, point.LatitudeDeg)
		report.ActualBounds.MaxLatitudeDeg = math.Max(report.ActualBounds.MaxLatitudeDeg, point.LatitudeDeg)
		report.ActualBounds.MinLongitudeDeg = math.Min(report.ActualBounds.MinLongitudeDeg, point.LongitudeDeg)
		report.ActualBounds.MaxLongitudeDeg = math.Max(report.ActualBounds.MaxLongitudeDeg, point.LongitudeDeg)
	}
	if math.IsInf(report.ActualBounds.MinLatitudeDeg, 0) {
		report.InvalidCoordinateCount++
		return report
	}
	deviations := []float64{
		math.Abs(report.ActualBounds.MinLatitudeDeg - config.ExpectedBounds.MinLatitudeDeg),
		math.Abs(report.ActualBounds.MaxLatitudeDeg - config.ExpectedBounds.MaxLatitudeDeg),
		math.Abs(report.ActualBounds.MinLongitudeDeg - config.ExpectedBounds.MinLongitudeDeg),
		math.Abs(report.ActualBounds.MaxLongitudeDeg - config.ExpectedBounds.MaxLongitudeDeg),
	}
	for _, deviation := range deviations {
		report.MaxDeviationDeg = math.Max(report.MaxDeviationDeg, deviation)
	}
	report.Accepted = report.InvalidCoordinateCount == 0 && report.MaxDeviationDeg <= report.ToleranceDeg
	return report
}

func evaluateFullTopology(model Model, config FullBlackSeaQualityConfig) (FullBlackSeaTopologyReport, FullBlackSeaEdgeReport) {
	topology := FullBlackSeaTopologyReport{
		NodeCount: len(model.Mesh.Nodes) - 1, CellCount: len(model.Mesh.Cells),
		QuadCount: model.Mesh.QuadCount, TriangleCount: model.Mesh.TriangleCount,
		BoundaryEdgeCount: len(model.Mesh.BoundaryEdges),
	}
	edges := make(map[[2]int]fullEdgeUsage, 2*len(model.Mesh.Cells))
	parent := make([]int, len(model.Mesh.Cells))
	for index := range parent {
		parent[index] = index
	}
	for cellIndex, cell := range model.Mesh.Cells {
		if cell.NodeCount != 4 || invalidCellNodes(cell, len(model.Mesh.Nodes)) {
			topology.DegenerateCellCount++
			continue
		}
		points := [4]mesh.Point{}
		for index, nodeID := range cell.Nodes {
			points[index] = model.Mesh.Nodes[nodeID]
		}
		if polygonAreaM2(points[:]) <= 0 {
			topology.DegenerateCellCount++
		}
		if segmentsCross(points[0], points[1], points[2], points[3]) || segmentsCross(points[1], points[2], points[3], points[0]) {
			topology.SelfIntersectingCellCount++
		}
		for edgeIndex := 0; edgeIndex < 4; edgeIndex++ {
			key := normalizedEdge(cell.Nodes[edgeIndex], cell.Nodes[(edgeIndex+1)%4])
			usage := edges[key]
			if usage.count == 0 {
				usage.firstCell = cellIndex
			} else {
				unionCells(parent, usage.firstCell, cellIndex)
			}
			usage.count++
			edges[key] = usage
		}
	}
	expectedBoundary := make(map[[2]int]int, len(model.Mesh.BoundaryEdges))
	for _, edge := range model.Mesh.BoundaryEdges {
		key := normalizedEdge(edge[0], edge[1])
		expectedBoundary[key]++
		if expectedBoundary[key] > 1 {
			topology.DuplicateBoundaryEdgeCount++
		}
	}
	edgeLengths := make([]float64, 0, len(edges))
	for key, usage := range edges {
		if usage.count == 1 {
			if expectedBoundary[key] == 0 {
				topology.UnexpectedBoundaryEdgeCount++
			}
		} else if usage.count > 2 {
			topology.NonManifoldEdgeCount++
		}
		if key[0] > 0 && key[1] > 0 && key[0] < len(model.Mesh.Nodes) && key[1] < len(model.Mesh.Nodes) {
			edgeLengths = append(edgeLengths, pointDistance(model.Mesh.Nodes[key[0]], model.Mesh.Nodes[key[1]]))
		}
	}
	for key := range expectedBoundary {
		if usage, ok := edges[key]; !ok || usage.count != 1 {
			topology.MissingBoundaryEdgeCount++
		}
	}
	topology.CellComponentCount = countCellComponents(parent)
	topology.CoastlineComponentCount = countBoundaryComponents(model.BoundaryEdges, BoundaryCoastline)
	topology.IslandComponentCount = countBoundaryComponents(model.BoundaryEdges, BoundaryIsland)
	for _, edge := range model.BoundaryEdges {
		if edge.Kind == BoundaryOpen {
			topology.OpenBoundaryEdgeCount++
		}
	}
	topology.BoundaryIntersectionCount = countBoundaryIntersections(model.Mesh)
	topology.Accepted = topology.NodeCount > 0 && topology.CellCount > 0 && topology.QuadCount == topology.CellCount &&
		topology.TriangleCount == 0 && topology.CoastlineComponentCount == 1 && topology.OpenBoundaryEdgeCount == 0 &&
		topology.CellComponentCount == 1 && topology.UnexpectedBoundaryEdgeCount == 0 &&
		topology.MissingBoundaryEdgeCount == 0 && topology.DuplicateBoundaryEdgeCount == 0 &&
		topology.NonManifoldEdgeCount == 0 && topology.DegenerateCellCount == 0 &&
		topology.SelfIntersectingCellCount == 0 && topology.BoundaryIntersectionCount == 0

	edgeReport := summarizeFullEdges(edgeLengths, config)
	return topology, edgeReport
}

func summarizeFullEdges(lengths []float64, config FullBlackSeaQualityConfig) FullBlackSeaEdgeReport {
	report := FullBlackSeaEdgeReport{TargetEdgeM: config.TargetEdgeM, UniqueEdgeCount: len(lengths), AllowedDeviationPercent: config.MeanEdgeTolerancePercent}
	if len(lengths) == 0 {
		return report
	}
	sort.Float64s(lengths)
	sum := 0.0
	for _, length := range lengths {
		sum += length
	}
	report.MinEdgeM = lengths[0]
	report.P05EdgeM = quantileSorted(lengths, 0.05)
	report.MeanEdgeM = sum / float64(len(lengths))
	report.P95EdgeM = quantileSorted(lengths, 0.95)
	report.MaxEdgeM = lengths[len(lengths)-1]
	report.MeanDeviationPercent = 100 * math.Abs(report.MeanEdgeM-config.TargetEdgeM) / config.TargetEdgeM
	report.Accepted = report.MeanDeviationPercent <= report.AllowedDeviationPercent
	return report
}

func evaluateFullDepth(model Model, config FullBlackSeaQualityConfig) FullBlackSeaDepthReport {
	report := FullBlackSeaDepthReport{
		NoDataCellCount:           model.CellDerivation.Summary.NoDataCellCount,
		MinElevationM:             math.Inf(1),
		MaxElevationM:             math.Inf(-1),
		NearestFallbackMaxPercent: config.NearestFallbackMaxPercent,
		LongFallbackWarningM:      config.LongFallbackWarningM,
	}
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		if node.ElevationM == nil || node.WaterDepthM == nil {
			report.NoDataNodeCount++
			continue
		}
		report.AssignedNodeCount++
		if !finite(*node.ElevationM) || !finite(*node.WaterDepthM) {
			report.NonFiniteValueCount++
			continue
		}
		if *node.ElevationM > 1e-9 {
			report.PositiveElevationCount++
		}
		if *node.WaterDepthM < -1e-9 {
			report.NegativeWaterDepthCount++
		}
		if math.Abs(*node.WaterDepthM-math.Max(0, -*node.ElevationM)) > 1e-8 {
			report.InconsistentSignCount++
		}
		if node.SamplingMethod == SamplingNearest {
			report.NearestFallbackNodeCount++
		}
		if node.SourceDistanceM != nil {
			report.MaxSourceDistanceM = math.Max(report.MaxSourceDistanceM, *node.SourceDistanceM)
			if *node.SourceDistanceM > report.LongFallbackWarningM {
				report.LongFallbackNodeCount++
			}
		}
		report.MinElevationM = math.Min(report.MinElevationM, *node.ElevationM)
		report.MaxElevationM = math.Max(report.MaxElevationM, *node.ElevationM)
		report.MaxWaterDepthM = math.Max(report.MaxWaterDepthM, *node.WaterDepthM)
	}
	if report.AssignedNodeCount > 0 {
		report.NearestFallbackPercent = 100 * float64(report.NearestFallbackNodeCount) / float64(report.AssignedNodeCount)
		report.LongFallbackPercent = 100 * float64(report.LongFallbackNodeCount) / float64(report.AssignedNodeCount)
	}
	if math.IsInf(report.MinElevationM, 0) {
		report.MinElevationM, report.MaxElevationM = 0, 0
	}
	report.Accepted = report.AssignedNodeCount > 0 && report.NoDataNodeCount == 0 && report.NoDataCellCount == 0 &&
		report.NonFiniteValueCount == 0 && report.PositiveElevationCount == 0 &&
		report.NegativeWaterDepthCount == 0 && report.InconsistentSignCount == 0 &&
		report.NearestFallbackPercent <= report.NearestFallbackMaxPercent
	return report
}

func evaluateFullIntegrals(model Model, config FullBlackSeaQualityConfig) FullBlackSeaIntegralReport {
	report := FullBlackSeaIntegralReport{
		CoastlineReferenceAreaKM2:     config.CoastlineReferenceAreaKM2,
		CoastlineAreaTolerancePercent: config.CoastlineAreaTolerancePercent,
	}
	volumeM3 := 0.0
	for _, cell := range model.Cells {
		report.AreaKM2 += cell.AreaM2 / 1e6
		volumeM3 += cell.AreaM2 * cell.WaterDepthMeanM
	}
	report.VolumeKM3 = volumeM3 / 1e9
	if report.AreaKM2 > 0 {
		report.MeanDepthM = volumeM3 / (report.AreaKM2 * 1e6)
	}
	report.CoastlineAreaDeviationPercent = percentDeviation(report.AreaKM2, config.CoastlineReferenceAreaKM2)
	report.CoastlineAreaAccepted = report.CoastlineAreaDeviationPercent <= report.CoastlineAreaTolerancePercent
	return report
}

func comparePublishedReference(report FullBlackSeaQualityReport, reference PublishedBasinReference, config FullBlackSeaQualityConfig) PublishedBasinComparison {
	comparison := PublishedBasinComparison{Reference: reference}
	comparison.AreaDeviationPercent = percentDeviation(report.Integrals.AreaKM2, reference.AreaKM2)
	comparison.VolumeDeviationPercent = percentDeviation(report.Integrals.VolumeKM3, reference.VolumeKM3)
	comparison.AreaAccepted = comparison.AreaDeviationPercent <= config.PublishedAreaTolerancePercent
	comparison.VolumeAccepted = comparison.VolumeDeviationPercent <= config.PublishedVolumeTolerancePercent
	comparison.DepthAccepted = true
	if reference.MaxDepthM > 0 {
		comparison.DepthDeviationPercent = percentDeviation(report.Depth.MaxWaterDepthM, reference.MaxDepthM)
		comparison.DepthAccepted = comparison.DepthDeviationPercent <= config.PublishedDepthTolerancePercent
	}
	// QA-03 по плану использует опубликованные площадь и интегральный объём как
	// блокирующий шлюз. Максимальная глубина остаётся отдельной локальной
	// диагностикой: единичный выброс нельзя скрыть, но он не заменяет интеграл.
	comparison.Accepted = comparison.AreaAccepted && comparison.VolumeAccepted
	return comparison
}

func appendFullQualityReasons(report *FullBlackSeaQualityReport) {
	if !report.Extent.Accepted {
		report.Reasons = append(report.Reasons, "географический охват модели не совпадает с полным исходным контуром")
	}
	if !report.Topology.Accepted {
		report.Reasons = append(report.Reasons, "четырёхугольный каркас содержит разрывы, пересечения или топологические несоответствия")
	}
	if !report.EdgeSize.Accepted {
		report.Reasons = append(report.Reasons, "средняя длина ребра не соответствует контрольному масштабу")
	}
	if !report.Depth.Accepted {
		report.Reasons = append(report.Reasons, "глубины содержат NoData, неконечные значения или неверный знак")
	}
	if report.Depth.LongFallbackNodeCount > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"%d узлов используют явную ближайшую замену дальше %.0f м; максимум %.0f м из-за несовпадения IHO-контура и отрицательной маски GEBCO у восточной границы",
			report.Depth.LongFallbackNodeCount, report.Depth.LongFallbackWarningM, report.Depth.MaxSourceDistanceM))
	}
	if !report.Integrals.CoastlineAreaAccepted {
		report.Reasons = append(report.Reasons, "площадь сетки не согласуется с площадью исходного контура")
	}
	for _, comparison := range report.PublishedComparisons {
		if !comparison.Accepted {
			report.Reasons = append(report.Reasons, fmt.Sprintf("интегральные характеристики не проходят ориентир %s", comparison.Reference.ID))
		}
		if !comparison.DepthAccepted {
			report.Warnings = append(report.Warnings, fmt.Sprintf(
				"максимальная глубина отличается от ориентира %s на %.2f%%; интегральный шлюз QA-03 использует площадь и объём, а локальный экстремум сохранён как диагностическое отклонение",
				comparison.Reference.ID, comparison.DepthDeviationPercent))
		}
	}
}

func publishedComparisonsAccepted(comparisons []PublishedBasinComparison) bool {
	primaryFound := false
	for _, comparison := range comparisons {
		if comparison.Reference.Primary {
			primaryFound = true
			if !comparison.Accepted {
				return false
			}
		}
	}
	return primaryFound
}

func invalidCellNodes(cell mesh.Cell, nodeCount int) bool {
	seen := make(map[int]bool, 4)
	for index := 0; index < cell.NodeCount; index++ {
		nodeID := cell.Nodes[index]
		if nodeID <= 0 || nodeID >= nodeCount || seen[nodeID] {
			return true
		}
		seen[nodeID] = true
	}
	return false
}

func unionCells(parent []int, left, right int) {
	leftRoot, rightRoot := findCellRoot(parent, left), findCellRoot(parent, right)
	if leftRoot != rightRoot {
		parent[rightRoot] = leftRoot
	}
}

func findCellRoot(parent []int, index int) int {
	for parent[index] != index {
		parent[index] = parent[parent[index]]
		index = parent[index]
	}
	return index
}

func countCellComponents(parent []int) int {
	if len(parent) == 0 {
		return 0
	}
	roots := make(map[int]struct{})
	for index := range parent {
		roots[findCellRoot(parent, index)] = struct{}{}
	}
	return len(roots)
}

func countBoundaryComponents(edges []BoundaryEdge, kind BoundaryKind) int {
	adjacency := make(map[int][]int)
	for _, edge := range edges {
		if edge.Kind != kind {
			continue
		}
		adjacency[edge.NodeIDs[0]] = append(adjacency[edge.NodeIDs[0]], edge.NodeIDs[1])
		adjacency[edge.NodeIDs[1]] = append(adjacency[edge.NodeIDs[1]], edge.NodeIDs[0])
	}
	visited := make(map[int]bool, len(adjacency))
	components := 0
	for start := range adjacency {
		if visited[start] {
			continue
		}
		components++
		stack := []int{start}
		visited[start] = true
		for len(stack) > 0 {
			node := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, next := range adjacency[node] {
				if !visited[next] {
					visited[next] = true
					stack = append(stack, next)
				}
			}
		}
	}
	return components
}

func countBoundaryIntersections(source mesh.Mesh) int {
	count := 0
	for leftIndex, left := range source.BoundaryEdges {
		if left[0] <= 0 || left[1] <= 0 || left[0] >= len(source.Nodes) || left[1] >= len(source.Nodes) {
			count++
			continue
		}
		for rightIndex := leftIndex + 1; rightIndex < len(source.BoundaryEdges); rightIndex++ {
			right := source.BoundaryEdges[rightIndex]
			if left[0] == right[0] || left[0] == right[1] || left[1] == right[0] || left[1] == right[1] {
				continue
			}
			if right[0] <= 0 || right[1] <= 0 || right[0] >= len(source.Nodes) || right[1] >= len(source.Nodes) {
				continue
			}
			if segmentsCross(source.Nodes[left[0]], source.Nodes[left[1]], source.Nodes[right[0]], source.Nodes[right[1]]) {
				count++
			}
		}
	}
	return count
}

func segmentsCross(a, b, c, d mesh.Point) bool {
	if math.Max(a.X, b.X)+1e-9 < math.Min(c.X, d.X) || math.Max(c.X, d.X)+1e-9 < math.Min(a.X, b.X) ||
		math.Max(a.Y, b.Y)+1e-9 < math.Min(c.Y, d.Y) || math.Max(c.Y, d.Y)+1e-9 < math.Min(a.Y, b.Y) {
		return false
	}
	o1, o2 := orientation(a, b, c), orientation(a, b, d)
	o3, o4 := orientation(c, d, a), orientation(c, d, b)
	return o1*o2 <= 0 && o3*o4 <= 0
}

func orientation(a, b, c mesh.Point) float64 {
	value := (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
	if math.Abs(value) <= 1e-9 {
		return 0
	}
	return value
}

func quantileSorted(values []float64, probability float64) float64 {
	if len(values) == 0 {
		return 0
	}
	position := probability * float64(len(values)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return values[lower]
	}
	fraction := position - float64(lower)
	return values[lower]*(1-fraction) + values[upper]*fraction
}

func percentDeviation(actual, reference float64) float64 {
	if reference <= 0 {
		return math.Inf(1)
	}
	return 100 * math.Abs(actual-reference) / reference
}
