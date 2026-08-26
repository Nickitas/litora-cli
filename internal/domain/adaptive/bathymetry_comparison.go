package adaptive

import (
	"fmt"
	"math"
	"sort"
	"time"

	"coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

// BathymetryComparisonConfig задаёт контрольные изобаты и число сохраняемых
// худших локальных ошибок внутренней проверки ADAPT-03.
type BathymetryComparisonConfig struct {
	IsobathsM      []float64
	WorstCellCount int
}

// DefaultBathymetryComparisonConfig возвращает единый набор контрольных
// глубин для всех генераторов и уровней детализации Чёрного моря.
func DefaultBathymetryComparisonConfig() BathymetryComparisonConfig {
	return BathymetryComparisonConfig{IsobathsM: []float64{200, 1000, 2000}, WorstCellCount: 10}
}

// IsobathAreaError описывает сохранение площади глубже заданной изобаты.
type IsobathAreaError struct {
	DepthM                   float64 `json:"depth_m"`
	ReferenceAreaKM2         float64 `json:"reference_area_km2"`
	ReconstructedAreaKM2     float64 `json:"reconstructed_area_km2"`
	AbsoluteDeviationKM2     float64 `json:"absolute_deviation_km2"`
	AbsoluteDeviationPercent float64 `json:"absolute_deviation_percent"`
}

// WorstBathymetryCell хранит одну контрольную точку с максимальной ошибкой.
type WorstBathymetryCell struct {
	CellID              int     `json:"cell_id"`
	XM                  float64 `json:"x_m"`
	YM                  float64 `json:"y_m"`
	LongitudeDeg        float64 `json:"longitude_deg"`
	LatitudeDeg         float64 `json:"latitude_deg"`
	ReferenceDepthM     float64 `json:"reference_depth_m"`
	ReconstructedDepthM float64 `json:"reconstructed_depth_m"`
	AbsoluteErrorM      float64 `json:"absolute_error_m"`
}

// BathymetryPreservationMetrics измеряет ошибку восстановления исходного
// поля BATHY-03 в центрах новых ячеек по значениям в их четырёх вершинах.
// Это внутренняя сравнительная проверка, а не внешняя валидация QA-02.
type BathymetryPreservationMetrics struct {
	Method                      string                `json:"method"`
	EvaluationCellCount         int                   `json:"evaluation_cell_count"`
	ReferenceMeanDepthM         float64               `json:"reference_mean_depth_m"`
	ReferenceP95DepthM          float64               `json:"reference_p95_depth_m"`
	DepthBiasM                  float64               `json:"depth_bias_m"`
	DepthMAEM                   float64               `json:"depth_mae_m"`
	DepthRMSEM                  float64               `json:"depth_rmse_m"`
	DepthP95AbsoluteErrorM      float64               `json:"depth_p95_absolute_error_m"`
	ReferenceWaterVolumeKM3     float64               `json:"reference_water_volume_km3"`
	ReconstructedWaterVolumeKM3 float64               `json:"reconstructed_water_volume_km3"`
	WaterVolumeDeviationPercent float64               `json:"water_volume_deviation_percent"`
	SlopeEvaluationCellCount    int                   `json:"slope_evaluation_cell_count"`
	SlopeMAEDeg                 float64               `json:"slope_mae_deg"`
	SlopeRMSEDeg                float64               `json:"slope_rmse_deg"`
	NearestFallbackNodeCount    int                   `json:"nearest_fallback_node_count"`
	NearestFallbackNodePercent  float64               `json:"nearest_fallback_node_percent"`
	MeanIsobathAreaDeviationPct float64               `json:"mean_isobath_area_deviation_percent"`
	Isobaths                    []IsobathAreaError    `json:"isobaths"`
	WorstCells                  []WorstBathymetryCell `json:"worst_cells"`
	DurationSeconds             float64               `json:"duration_seconds"`
}

type depthSample struct {
	depthM      float64
	gradientX   float64
	gradientY   float64
	gradientSet bool
	fallback    bool
}

type depthTriangle struct {
	nodeIDs    [3]int
	minX, minY float64
	maxX, maxY float64
	gradientX  float64
	gradientY  float64
}

type depthGridIndex struct {
	nodes         []seabed.Node
	triangles     []depthTriangle
	bins          [][]int
	columns, rows int
	minX, minY    float64
	maxX, maxY    float64
	stepX, stepY  float64
	nearest       *depthKDNode
}

// EvaluateBathymetryPreservation переносит глубины опорной модели на узлы
// новой сетки, а затем использует центры ячеек как независимые от её узлов
// контрольные точки. Все генераторы проходят одну и ту же процедуру.
func EvaluateBathymetryPreservation(reference seabed.Model, generated mesh.Mesh, config BathymetryComparisonConfig) (BathymetryPreservationMetrics, error) {
	started := time.Now()
	if len(generated.Nodes) <= 1 || len(generated.Cells) == 0 {
		return BathymetryPreservationMetrics{}, fmt.Errorf("сравниваемая сетка не содержит узлов или ячеек")
	}
	if len(config.IsobathsM) == 0 {
		config.IsobathsM = DefaultBathymetryComparisonConfig().IsobathsM
	}
	if config.WorstCellCount <= 0 {
		config.WorstCellCount = DefaultBathymetryComparisonConfig().WorstCellCount
	}
	index, err := newDepthGridIndex(reference)
	if err != nil {
		return BathymetryPreservationMetrics{}, err
	}

	nodeDepths := make([]float64, len(generated.Nodes))
	metrics := BathymetryPreservationMetrics{
		Method: "глубины узлов интерполируются по full-quad каркасу BATHY-03; центры новых ячеек являются контрольными точками; восстановление в центре равно среднему четырёх вершин",
	}
	for nodeID := 1; nodeID < len(generated.Nodes); nodeID++ {
		point := generated.Nodes[nodeID]
		sample := index.sample(point.X, point.Y)
		nodeDepths[nodeID] = sample.depthM
		if sample.fallback {
			metrics.NearestFallbackNodeCount++
		}
	}
	metrics.NearestFallbackNodePercent = 100 * float64(metrics.NearestFallbackNodeCount) / float64(len(generated.Nodes)-1)

	isobaths := append([]float64(nil), config.IsobathsM...)
	sort.Float64s(isobaths)
	referenceIsobathArea := make([]float64, len(isobaths))
	reconstructedIsobathArea := make([]float64, len(isobaths))
	errorHistogram := make([]int64, 20_001) // шаг 0,1 м, последний интервал ≥ 2000 м.
	depthHistogram := make([]int64, 5_001)  // шаг 1 м, последний интервал ≥ 5000 м.
	var errorSum, absoluteErrorSum, squaredErrorSum float64
	var referenceDepthSum, referenceVolumeM3, reconstructedVolumeM3 float64
	var slopeAbsoluteErrorSum, slopeSquaredErrorSum float64

	for cellIndex, cell := range generated.Cells {
		if cell.NodeCount != 4 {
			return BathymetryPreservationMetrics{}, fmt.Errorf("ячейка %d не является четырёхугольником", cellIndex+1)
		}
		center := mesh.Point{}
		reconstructedDepthM := 0.0
		longitudeDeg, latitudeDeg := 0.0, 0.0
		for corner := 0; corner < 4; corner++ {
			nodeID := cell.Nodes[corner]
			if nodeID <= 0 || nodeID >= len(generated.Nodes) {
				return BathymetryPreservationMetrics{}, fmt.Errorf("ячейка %d содержит некорректный узел", cellIndex+1)
			}
			point := generated.Nodes[nodeID]
			center.X += point.X
			center.Y += point.Y
			longitudeDeg += point.LongitudeDeg
			latitudeDeg += point.LatitudeDeg
			reconstructedDepthM += nodeDepths[nodeID]
		}
		center.X /= 4
		center.Y /= 4
		longitudeDeg /= 4
		latitudeDeg /= 4
		reconstructedDepthM /= 4
		referenceSample := index.sample(center.X, center.Y)
		referenceDepthM := referenceSample.depthM
		areaM2 := quadArea(generated.Nodes, cell)
		if areaM2 <= 0 {
			continue
		}
		errorM := reconstructedDepthM - referenceDepthM
		absoluteErrorM := math.Abs(errorM)
		metrics.EvaluationCellCount++
		errorSum += errorM
		absoluteErrorSum += absoluteErrorM
		squaredErrorSum += errorM * errorM
		referenceDepthSum += referenceDepthM
		referenceVolumeM3 += areaM2 * referenceDepthM
		reconstructedVolumeM3 += areaM2 * reconstructedDepthM
		errorBin := int(math.Min(float64(len(errorHistogram)-1), math.Floor(absoluteErrorM/0.1)))
		errorHistogram[errorBin]++
		depthBin := int(math.Min(float64(len(depthHistogram)-1), math.Floor(referenceDepthM)))
		depthHistogram[depthBin]++
		for index, isobathM := range isobaths {
			if referenceDepthM >= isobathM {
				referenceIsobathArea[index] += areaM2
			}
			if reconstructedDepthM >= isobathM {
				reconstructedIsobathArea[index] += areaM2
			}
		}
		if referenceSample.gradientSet {
			if gradientX, gradientY, ok := quadDepthGradient(generated.Nodes, cell, nodeDepths); ok {
				referenceSlopeDeg := math.Atan(math.Hypot(referenceSample.gradientX, referenceSample.gradientY)) * 180 / math.Pi
				reconstructedSlopeDeg := math.Atan(math.Hypot(gradientX, gradientY)) * 180 / math.Pi
				slopeErrorDeg := math.Abs(reconstructedSlopeDeg - referenceSlopeDeg)
				metrics.SlopeEvaluationCellCount++
				slopeAbsoluteErrorSum += slopeErrorDeg
				slopeSquaredErrorSum += slopeErrorDeg * slopeErrorDeg
			}
		}
		metrics.WorstCells = insertWorstCell(metrics.WorstCells, WorstBathymetryCell{
			CellID: cellIndex + 1, XM: center.X, YM: center.Y,
			LongitudeDeg: longitudeDeg, LatitudeDeg: latitudeDeg,
			ReferenceDepthM: referenceDepthM, ReconstructedDepthM: reconstructedDepthM, AbsoluteErrorM: absoluteErrorM,
		}, config.WorstCellCount)
	}
	if metrics.EvaluationCellCount == 0 {
		return BathymetryPreservationMetrics{}, fmt.Errorf("не найдено контрольных ячеек для сравнения батиметрии")
	}
	count := float64(metrics.EvaluationCellCount)
	metrics.ReferenceMeanDepthM = referenceDepthSum / count
	metrics.ReferenceP95DepthM = fixedHistogramQuantile(depthHistogram, int64(metrics.EvaluationCellCount), 0.95, 1)
	metrics.DepthBiasM = errorSum / count
	metrics.DepthMAEM = absoluteErrorSum / count
	metrics.DepthRMSEM = math.Sqrt(squaredErrorSum / count)
	metrics.DepthP95AbsoluteErrorM = fixedHistogramQuantile(errorHistogram, int64(metrics.EvaluationCellCount), 0.95, 0.1)
	metrics.ReferenceWaterVolumeKM3 = referenceVolumeM3 / 1e9
	metrics.ReconstructedWaterVolumeKM3 = reconstructedVolumeM3 / 1e9
	if referenceVolumeM3 > 0 {
		metrics.WaterVolumeDeviationPercent = 100 * (reconstructedVolumeM3 - referenceVolumeM3) / referenceVolumeM3
	}
	if metrics.SlopeEvaluationCellCount > 0 {
		slopeCount := float64(metrics.SlopeEvaluationCellCount)
		metrics.SlopeMAEDeg = slopeAbsoluteErrorSum / slopeCount
		metrics.SlopeRMSEDeg = math.Sqrt(slopeSquaredErrorSum / slopeCount)
	}
	for index, isobathM := range isobaths {
		referenceAreaM2 := referenceIsobathArea[index]
		reconstructedAreaM2 := reconstructedIsobathArea[index]
		deviationM2 := math.Abs(reconstructedAreaM2 - referenceAreaM2)
		deviationPercent := 0.0
		if referenceAreaM2 > 0 {
			deviationPercent = 100 * deviationM2 / referenceAreaM2
		}
		metrics.Isobaths = append(metrics.Isobaths, IsobathAreaError{
			DepthM: isobathM, ReferenceAreaKM2: referenceAreaM2 / 1e6,
			ReconstructedAreaKM2: reconstructedAreaM2 / 1e6,
			AbsoluteDeviationKM2: deviationM2 / 1e6, AbsoluteDeviationPercent: deviationPercent,
		})
		metrics.MeanIsobathAreaDeviationPct += deviationPercent
	}
	if len(metrics.Isobaths) > 0 {
		metrics.MeanIsobathAreaDeviationPct /= float64(len(metrics.Isobaths))
	}
	metrics.DurationSeconds = time.Since(started).Seconds()
	return metrics, nil
}

func newDepthGridIndex(reference seabed.Model) (*depthGridIndex, error) {
	if len(reference.Nodes) <= 1 || len(reference.Mesh.Cells) == 0 {
		return nil, fmt.Errorf("опорная модель BATHY-03 не содержит узлов или ячеек")
	}
	index := &depthGridIndex{nodes: reference.Nodes, minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	validNodeIDs := make([]int, 0, len(reference.Nodes)-1)
	for nodeID := 1; nodeID < len(reference.Nodes); nodeID++ {
		node := reference.Nodes[nodeID]
		if node.WaterDepthM == nil || math.IsNaN(*node.WaterDepthM) || math.IsInf(*node.WaterDepthM, 0) || *node.WaterDepthM < 0 {
			continue
		}
		validNodeIDs = append(validNodeIDs, nodeID)
		index.minX, index.maxX = math.Min(index.minX, node.XM), math.Max(index.maxX, node.XM)
		index.minY, index.maxY = math.Min(index.minY, node.YM), math.Max(index.maxY, node.YM)
	}
	if len(validNodeIDs) == 0 || index.maxX <= index.minX || index.maxY <= index.minY {
		return nil, fmt.Errorf("опорная модель BATHY-03 не содержит пригодных глубин")
	}
	for _, cell := range reference.Mesh.Cells {
		if cell.NodeCount != 4 {
			continue
		}
		for _, ids := range [][3]int{{cell.Nodes[0], cell.Nodes[1], cell.Nodes[2]}, {cell.Nodes[0], cell.Nodes[2], cell.Nodes[3]}} {
			triangle, ok := makeDepthTriangle(reference.Nodes, ids)
			if ok {
				index.triangles = append(index.triangles, triangle)
			}
		}
	}
	if len(index.triangles) == 0 {
		return nil, fmt.Errorf("опорная модель BATHY-03 не содержит интерполируемых ячеек")
	}
	resolution := int(math.Ceil(math.Sqrt(float64(len(index.triangles)))))
	if resolution < 16 {
		resolution = 16
	}
	index.columns, index.rows = resolution, resolution
	index.stepX = (index.maxX - index.minX) / float64(index.columns)
	index.stepY = (index.maxY - index.minY) / float64(index.rows)
	index.bins = make([][]int, index.columns*index.rows)
	for triangleID, triangle := range index.triangles {
		minColumn, minRow := index.binCoordinates(triangle.minX, triangle.minY)
		maxColumn, maxRow := index.binCoordinates(triangle.maxX, triangle.maxY)
		for row := minRow; row <= maxRow; row++ {
			for column := minColumn; column <= maxColumn; column++ {
				binID := row*index.columns + column
				index.bins[binID] = append(index.bins[binID], triangleID)
			}
		}
	}
	index.nearest = buildDepthKDTree(reference.Nodes, validNodeIDs, 0)
	return index, nil
}

func makeDepthTriangle(nodes []seabed.Node, ids [3]int) (depthTriangle, bool) {
	triangle := depthTriangle{nodeIDs: ids, minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	for _, nodeID := range ids {
		if nodeID <= 0 || nodeID >= len(nodes) || nodes[nodeID].WaterDepthM == nil {
			return depthTriangle{}, false
		}
		node := nodes[nodeID]
		triangle.minX, triangle.maxX = math.Min(triangle.minX, node.XM), math.Max(triangle.maxX, node.XM)
		triangle.minY, triangle.maxY = math.Min(triangle.minY, node.YM), math.Max(triangle.maxY, node.YM)
	}
	a, b, c := nodes[ids[0]], nodes[ids[1]], nodes[ids[2]]
	denominator := (b.XM-a.XM)*(c.YM-a.YM) - (c.XM-a.XM)*(b.YM-a.YM)
	if math.Abs(denominator) <= 1e-12 {
		return depthTriangle{}, false
	}
	az, bz, cz := *a.WaterDepthM, *b.WaterDepthM, *c.WaterDepthM
	triangle.gradientX = ((bz-az)*(c.YM-a.YM) - (cz-az)*(b.YM-a.YM)) / denominator
	triangle.gradientY = ((b.XM-a.XM)*(cz-az) - (c.XM-a.XM)*(bz-az)) / denominator
	return triangle, true
}

func (index *depthGridIndex) sample(x, y float64) depthSample {
	column, row := index.binCoordinates(x, y)
	for _, triangleID := range index.bins[row*index.columns+column] {
		triangle := index.triangles[triangleID]
		if x < triangle.minX-1e-7 || x > triangle.maxX+1e-7 || y < triangle.minY-1e-7 || y > triangle.maxY+1e-7 {
			continue
		}
		if sample, ok := interpolateDepthTriangle(index.nodes, triangle, x, y); ok {
			return sample
		}
	}
	nearestID := nearestDepthNode(index.nearest, index.nodes, x, y, 0, math.Inf(1))
	if nearestID == 0 || index.nodes[nearestID].WaterDepthM == nil {
		return depthSample{fallback: true}
	}
	return depthSample{depthM: *index.nodes[nearestID].WaterDepthM, fallback: true}
}

func (index *depthGridIndex) binCoordinates(x, y float64) (int, int) {
	column := int(math.Floor((x - index.minX) / index.stepX))
	row := int(math.Floor((y - index.minY) / index.stepY))
	column = max(0, min(index.columns-1, column))
	row = max(0, min(index.rows-1, row))
	return column, row
}

func interpolateDepthTriangle(nodes []seabed.Node, triangle depthTriangle, x, y float64) (depthSample, bool) {
	a, b, c := nodes[triangle.nodeIDs[0]], nodes[triangle.nodeIDs[1]], nodes[triangle.nodeIDs[2]]
	denominator := (b.YM-c.YM)*(a.XM-c.XM) + (c.XM-b.XM)*(a.YM-c.YM)
	if math.Abs(denominator) <= 1e-12 {
		return depthSample{}, false
	}
	wA := ((b.YM-c.YM)*(x-c.XM) + (c.XM-b.XM)*(y-c.YM)) / denominator
	wB := ((c.YM-a.YM)*(x-c.XM) + (a.XM-c.XM)*(y-c.YM)) / denominator
	wC := 1 - wA - wB
	if wA < -1e-9 || wB < -1e-9 || wC < -1e-9 {
		return depthSample{}, false
	}
	depthM := wA**a.WaterDepthM + wB**b.WaterDepthM + wC**c.WaterDepthM
	return depthSample{depthM: math.Max(0, depthM), gradientX: triangle.gradientX, gradientY: triangle.gradientY, gradientSet: true}, true
}

type depthKDNode struct {
	id, axis    int
	left, right *depthKDNode
}

func buildDepthKDTree(nodes []seabed.Node, ids []int, depth int) *depthKDNode {
	if len(ids) == 0 {
		return nil
	}
	axis := depth % 2
	sort.Slice(ids, func(left, right int) bool {
		if axis == 0 {
			return nodes[ids[left]].XM < nodes[ids[right]].XM
		}
		return nodes[ids[left]].YM < nodes[ids[right]].YM
	})
	middle := len(ids) / 2
	return &depthKDNode{
		id: ids[middle], axis: axis,
		left: buildDepthKDTree(nodes, ids[:middle], depth+1), right: buildDepthKDTree(nodes, ids[middle+1:], depth+1),
	}
}

func nearestDepthNode(node *depthKDNode, points []seabed.Node, x, y float64, bestID int, bestSquared float64) int {
	if node == nil {
		return bestID
	}
	point := points[node.id]
	distance := (point.XM-x)*(point.XM-x) + (point.YM-y)*(point.YM-y)
	if distance < bestSquared {
		bestSquared, bestID = distance, node.id
	}
	delta := point.XM - x
	if node.axis == 1 {
		delta = point.YM - y
	}
	near, far := node.left, node.right
	if delta < 0 {
		near, far = node.right, node.left
	}
	bestID = nearestDepthNode(near, points, x, y, bestID, bestSquared)
	if bestID != 0 {
		bestPoint := points[bestID]
		bestSquared = (bestPoint.XM-x)*(bestPoint.XM-x) + (bestPoint.YM-y)*(bestPoint.YM-y)
	}
	if delta*delta < bestSquared {
		bestID = nearestDepthNode(far, points, x, y, bestID, bestSquared)
	}
	return bestID
}

func quadArea(nodes []mesh.Point, cell mesh.Cell) float64 {
	area2 := 0.0
	for side := 0; side < 4; side++ {
		a, b := nodes[cell.Nodes[side]], nodes[cell.Nodes[(side+1)%4]]
		area2 += a.X*b.Y - b.X*a.Y
	}
	return math.Abs(area2) / 2
}

func quadDepthGradient(nodes []mesh.Point, cell mesh.Cell, depths []float64) (float64, float64, bool) {
	centerX, centerY, centerDepth := 0.0, 0.0, 0.0
	for corner := 0; corner < 4; corner++ {
		nodeID := cell.Nodes[corner]
		centerX += nodes[nodeID].X
		centerY += nodes[nodeID].Y
		centerDepth += depths[nodeID]
	}
	centerX, centerY, centerDepth = centerX/4, centerY/4, centerDepth/4
	var xx, xy, yy, xz, yz float64
	for corner := 0; corner < 4; corner++ {
		nodeID := cell.Nodes[corner]
		dx, dy, dz := nodes[nodeID].X-centerX, nodes[nodeID].Y-centerY, depths[nodeID]-centerDepth
		xx += dx * dx
		xy += dx * dy
		yy += dy * dy
		xz += dx * dz
		yz += dy * dz
	}
	determinant := xx*yy - xy*xy
	if math.Abs(determinant) <= 1e-12 {
		return 0, 0, false
	}
	return (xz*yy - yz*xy) / determinant, (yz*xx - xz*xy) / determinant, true
}

func insertWorstCell(values []WorstBathymetryCell, candidate WorstBathymetryCell, limit int) []WorstBathymetryCell {
	values = append(values, candidate)
	sort.SliceStable(values, func(left, right int) bool {
		if values[left].AbsoluteErrorM == values[right].AbsoluteErrorM {
			return values[left].CellID < values[right].CellID
		}
		return values[left].AbsoluteErrorM > values[right].AbsoluteErrorM
	})
	if len(values) > limit {
		values = values[:limit]
	}
	return values
}

func fixedHistogramQuantile(histogram []int64, count int64, probability, step float64) float64 {
	threshold := int64(math.Ceil(float64(count) * probability))
	if threshold < 1 {
		threshold = 1
	}
	seen := int64(0)
	for index, value := range histogram {
		seen += value
		if seen >= threshold {
			return (float64(index) + 0.5) * step
		}
	}
	return float64(len(histogram)-1) * step
}
