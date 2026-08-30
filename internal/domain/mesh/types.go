package mesh

import (
	"fmt"
	"strings"
	"time"
)

// Algorithm идентифицирует алгоритм построения 2D-сетки в Gmsh.
type Algorithm string

const (
	// PhysicalCoastline помечает внешний берег Чёрного моря в MSH.
	PhysicalCoastline = 1
	// PhysicalIsland помечает берег островного отверстия в MSH.
	PhysicalIsland = 2
	// PhysicalOpenBoundary зарезервирован для открытой границы локального участка.
	PhysicalOpenBoundary = 3
	// PhysicalWaterSurface помечает двумерную поверхность акватории.
	PhysicalWaterSurface = 10

	// AlgorithmDelaunay использует классическую 2D-триангуляцию Delaunay.
	AlgorithmDelaunay Algorithm = "delaunay"
	// AlgorithmFrontalQuad использует Frontal-Delaunay for Quads.
	AlgorithmFrontalQuad Algorithm = "frontal-quad"
	// AlgorithmParallelograms использует упаковку параллелограммов.
	AlgorithmParallelograms Algorithm = "parallelograms"
)

// Algorithms возвращает стабильный порядок доступных алгоритмов сравнения.
func Algorithms() []Algorithm {
	return []Algorithm{AlgorithmDelaunay, AlgorithmFrontalQuad, AlgorithmParallelograms}
}

// ParseAlgorithm проверяет пользовательское имя алгоритма.
func ParseAlgorithm(value string) (Algorithm, error) {
	algorithm := Algorithm(strings.TrimSpace(strings.ToLower(value)))
	// Старое имя оставлено как совместимый псевдоним для ранних отчётов Lito.
	if algorithm == "delaunay-blossom" {
		algorithm = AlgorithmDelaunay
	}
	for _, candidate := range Algorithms() {
		if algorithm == candidate {
			return algorithm, nil
		}
	}
	return "", fmt.Errorf("неизвестный генератор 2D-сетки %q", value)
}

// RussianName возвращает название алгоритма для русскоязычного отчёта.
func (algorithm Algorithm) RussianName() string {
	switch algorithm {
	case AlgorithmDelaunay:
		return "Delaunay"
	case AlgorithmFrontalQuad:
		return "Frontal-Delaunay for Quads"
	case AlgorithmParallelograms:
		return "Упаковка параллелограммов"
	default:
		return string(algorithm)
	}
}

type algorithmOptions struct {
	MeshAlgorithm int
}

func (algorithm Algorithm) options() (algorithmOptions, error) {
	switch algorithm {
	case AlgorithmDelaunay:
		return algorithmOptions{MeshAlgorithm: 5}, nil
	case AlgorithmFrontalQuad:
		return algorithmOptions{MeshAlgorithm: 8}, nil
	case AlgorithmParallelograms:
		return algorithmOptions{MeshAlgorithm: 9}, nil
	default:
		return algorithmOptions{}, fmt.Errorf("неподдерживаемый алгоритм %q", algorithm)
	}
}

// Point задаёт координаты узла в локальной плоскости LAEA и WGS 84.
// GeographicCoordinatesSet отличает вычисленные географические координаты
// от нулевых значений старого плоского MSH.
type Point struct {
	X                        float64 `json:"x_m"`
	Y                        float64 `json:"y_m"`
	LongitudeDeg             float64 `json:"longitude_deg"`
	LatitudeDeg              float64 `json:"latitude_deg"`
	GeographicCoordinatesSet bool    `json:"geographic_coordinates_set"`
}

// Cell хранит индексы узлов одной поверхностной ячейки.
type Cell struct {
	Nodes     [4]int
	NodeCount int
}

// Mesh представляет прочитанную плоскую сетку Gmsh.
type Mesh struct {
	Nodes                []Point
	Cells                []Cell
	BoundaryEdges        [][2]int
	BoundaryPhysicalTags []int
	SurfacePhysicalTag   int
	TriangleCount        int
	QuadCount            int
}

// PreparedDomain содержит исходные и детализированные кольца в единой
// равноплощадной проекции.
type PreparedDomain struct {
	OriginalRings                [][]Point
	SimplifiedRings              [][]Point
	OriginalPointCount           int
	SimplifiedPointCount         int
	ReferenceAreaM2              float64
	SimplifiedAreaM2             float64
	BoundaryLengthM              float64
	CumulativeFeatureDeviationM2 float64
	// EffectiveBoundaryToleranceMeters фиксирует фактический допуск после
	// автоматического уменьшения ради сохранения топологии колец.
	EffectiveBoundaryToleranceMeters float64
	Projection                       EqualAreaProjection
	// ProjectionRoundTripMaxErrorMeters — максимальная ошибка
	// WGS 84 → LAEA → WGS 84 на всех исходных кольцах.
	ProjectionRoundTripMaxErrorMeters float64
}

// GenerationConfig задаёт один воспроизводимый запуск Gmsh.
type GenerationConfig struct {
	Algorithm        Algorithm
	TargetEdgeMeters float64
	GeoPath          string
	MeshPath         string
	LogPath          string
	GmshPath         string
	Timeout          time.Duration
}

// QualityMetrics описывает сохранение береговых форм и геометрию ячеек.
type QualityMetrics struct {
	ReferenceAreaKM2                  float64 `json:"reference_area_km2"`
	SimplifiedBoundaryAreaKM2         float64 `json:"simplified_boundary_area_km2"`
	MeshAreaKM2                       float64 `json:"mesh_area_km2"`
	CumulativeFeatureAreaDeviationKM2 float64 `json:"cumulative_feature_area_deviation_km2"`
	MeshAreaDeviationKM2              float64 `json:"mesh_area_deviation_km2"`
	CoastalAreaDeviationPercent       float64 `json:"coastal_area_deviation_percent"`
	BoundaryRMSMeters                 float64 `json:"boundary_rms_meters"`
	BoundaryHausdorffMeters           float64 `json:"boundary_hausdorff_meters"`
	CellCount                         int     `json:"cell_count"`
	QuadCount                         int     `json:"quad_count"`
	TriangleCount                     int     `json:"triangle_count"`
	QuadSharePercent                  float64 `json:"quad_share_percent"`
	MeanEdgeMeters                    float64 `json:"mean_edge_meters"`
	MinEdgeMeters                     float64 `json:"min_edge_meters"`
	MaxEdgeMeters                     float64 `json:"max_edge_meters"`
	MeanCellQuality                   float64 `json:"mean_cell_quality"`
	P05CellQuality                    float64 `json:"p05_cell_quality"`
	CompositeScore                    float64 `json:"composite_score"`
}
