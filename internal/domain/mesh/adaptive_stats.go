package mesh

import (
	"fmt"
	"math"
	"sort"
)

// AdaptivePreflight содержит оценку объёма до запуска Gmsh.
type AdaptivePreflight struct {
	EstimatedCellCount   int64  `json:"estimated_cell_count"`
	EstimatedMemoryBytes int64  `json:"estimated_memory_bytes"`
	EstimatedDiskBytes   int64  `json:"estimated_disk_bytes"`
	Method               string `json:"method"`
}

// EstimateAdaptiveSize интегрирует локальную площадь опорных ячеек по h².
// Коэффициенты памяти и диска консервативны для текстового MSH и full-quad
// постобработки и служат предохранителем, а не обещанием точного потребления.
func EstimateAdaptiveSize(support Mesh, targetSizeM []float64) (AdaptivePreflight, error) {
	if len(support.Nodes) <= 1 || len(support.Cells) == 0 || len(targetSizeM) != len(support.Nodes) {
		return AdaptivePreflight{}, fmt.Errorf("опорная сетка и поле размера не согласованы")
	}
	weighted := 0.0
	for cellIndex, cell := range support.Cells {
		if cell.NodeCount != 4 {
			return AdaptivePreflight{}, fmt.Errorf("опорная ячейка %d не является четырёхугольником", cellIndex+1)
		}
		area2, target := 0.0, 0.0
		for side := 0; side < 4; side++ {
			a, b := cell.Nodes[side], cell.Nodes[(side+1)%4]
			if a <= 0 || a >= len(support.Nodes) || b <= 0 || b >= len(support.Nodes) || !finitePositive(targetSizeM[a]) {
				return AdaptivePreflight{}, fmt.Errorf("опорная ячейка %d содержит некорректный узел", cellIndex+1)
			}
			left, right := support.Nodes[a], support.Nodes[b]
			area2 += left.X*right.Y - right.X*left.Y
			target += targetSizeM[a]
		}
		target /= 4
		weighted += math.Abs(area2) / 2 / (target * target)
	}
	cells := int64(math.Ceil(1.5 * weighted))
	return AdaptivePreflight{
		EstimatedCellCount:   cells,
		EstimatedMemoryBytes: cells * 320,
		EstimatedDiskBytes:   cells * 160,
		Method:               "ceil(1.5 · Σ(S_ячейки / mean(h_узлов)²)); память 320 Б/ячейку; текстовый MSH 160 Б/ячейку",
	}, nil
}

// ZoneEdgeStatistics сравнивает фактические рёбра с ближайшим узловым
// значением поля ADAPT-01. Внутренние рёбра учитываются со стороны каждой
// соседней ячейки, что позволяет считать статистику без многомиллионной карты
// уникальных рёбер.
type ZoneEdgeStatistics struct {
	Zone                 string  `json:"zone"`
	Name                 string  `json:"name"`
	EdgeObservationCount int64   `json:"edge_observation_count"`
	TargetMinM           float64 `json:"target_min_m"`
	TargetMeanM          float64 `json:"target_mean_m"`
	TargetMaxM           float64 `json:"target_max_m"`
	ActualMinM           float64 `json:"actual_min_m"`
	ActualP05M           float64 `json:"actual_p05_m"`
	ActualMeanM          float64 `json:"actual_mean_m"`
	ActualP95M           float64 `json:"actual_p95_m"`
	ActualMaxM           float64 `json:"actual_max_m"`
	RatioP05             float64 `json:"ratio_p05"`
	RatioMean            float64 `json:"ratio_mean"`
	RatioP95             float64 `json:"ratio_p95"`
	WithinTolerancePct   float64 `json:"within_tolerance_percent"`
}

type edgeAccumulator struct {
	count, within                   int64
	targetMin, targetSum, targetMax float64
	actualMin, actualSum, actualMax float64
	ratioSum                        float64
	actualHistogram                 [2001]int64
	ratioHistogram                  [1001]int64
}

// EvaluateAdaptiveEdges возвращает статистику по зонам с постоянным объёмом
// памяти для гистограмм. Квантили имеют шаг 5 м и 0,005 соответственно.
func EvaluateAdaptiveEdges(generated, support Mesh, targetSizeM []float64, zones []string, zoneNames map[string]string) ([]ZoneEdgeStatistics, error) {
	if len(targetSizeM) != len(support.Nodes) || len(zones) != len(support.Nodes) {
		return nil, fmt.Errorf("опорное поле для статистики рёбер не согласовано")
	}
	tree := buildKDTree(support.Nodes)
	if tree == nil {
		return nil, fmt.Errorf("опорное поле не содержит узлов")
	}
	accumulators := make(map[string]*edgeAccumulator)
	for cellIndex, cell := range generated.Cells {
		if cell.NodeCount != 4 {
			return nil, fmt.Errorf("ячейка %d не является четырёхугольником", cellIndex+1)
		}
		for side := 0; side < 4; side++ {
			a, b := cell.Nodes[side], cell.Nodes[(side+1)%4]
			left, right := generated.Nodes[a], generated.Nodes[b]
			nodeID := nearestKD(tree, support.Nodes, (left.X+right.X)/2, (left.Y+right.Y)/2, 0, math.Inf(1))
			target := targetSizeM[nodeID]
			zone := zones[nodeID]
			actual := math.Hypot(right.X-left.X, right.Y-left.Y)
			if !finitePositive(target) || !finitePositive(actual) || zone == "" {
				return nil, fmt.Errorf("не удалось сопоставить ребро ячейки %d с полем размера", cellIndex+1)
			}
			acc := accumulators[zone]
			if acc == nil {
				acc = &edgeAccumulator{targetMin: math.Inf(1), actualMin: math.Inf(1)}
				accumulators[zone] = acc
			}
			ratio := actual / target
			acc.count++
			acc.targetMin, acc.targetMax, acc.targetSum = math.Min(acc.targetMin, target), math.Max(acc.targetMax, target), acc.targetSum+target
			acc.actualMin, acc.actualMax, acc.actualSum = math.Min(acc.actualMin, actual), math.Max(acc.actualMax, actual), acc.actualSum+actual
			acc.ratioSum += ratio
			if ratio >= 0.5 && ratio <= 1.5 {
				acc.within++
			}
			actualBin := int(math.Min(float64(len(acc.actualHistogram)-1), math.Floor(actual/5)))
			ratioBin := int(math.Min(float64(len(acc.ratioHistogram)-1), math.Floor(ratio/0.005)))
			acc.actualHistogram[actualBin]++
			acc.ratioHistogram[ratioBin]++
		}
	}
	ids := make([]string, 0, len(accumulators))
	for id := range accumulators {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]ZoneEdgeStatistics, 0, len(ids))
	for _, id := range ids {
		acc := accumulators[id]
		name := zoneNames[id]
		if name == "" {
			name = id
		}
		result = append(result, ZoneEdgeStatistics{
			Zone: id, Name: name, EdgeObservationCount: acc.count,
			TargetMinM: acc.targetMin, TargetMeanM: acc.targetSum / float64(acc.count), TargetMaxM: acc.targetMax,
			ActualMinM: acc.actualMin, ActualP05M: histogramQuantile(acc.actualHistogram[:], acc.count, 0.05, 5),
			ActualMeanM: acc.actualSum / float64(acc.count), ActualP95M: histogramQuantile(acc.actualHistogram[:], acc.count, 0.95, 5), ActualMaxM: acc.actualMax,
			RatioP05: histogramQuantile(acc.ratioHistogram[:], acc.count, 0.05, 0.005), RatioMean: acc.ratioSum / float64(acc.count),
			RatioP95: histogramQuantile(acc.ratioHistogram[:], acc.count, 0.95, 0.005), WithinTolerancePct: 100 * float64(acc.within) / float64(acc.count),
		})
	}
	return result, nil
}

type kdNode struct {
	id, axis    int
	left, right *kdNode
}

func buildKDTree(nodes []Point) *kdNode {
	ids := make([]int, 0, len(nodes)-1)
	for id := 1; id < len(nodes); id++ {
		ids = append(ids, id)
	}
	var build func([]int, int) *kdNode
	build = func(values []int, depth int) *kdNode {
		if len(values) == 0 {
			return nil
		}
		axis := depth % 2
		sort.Slice(values, func(i, j int) bool {
			if axis == 0 {
				return nodes[values[i]].X < nodes[values[j]].X
			}
			return nodes[values[i]].Y < nodes[values[j]].Y
		})
		middle := len(values) / 2
		return &kdNode{id: values[middle], axis: axis, left: build(values[:middle], depth+1), right: build(values[middle+1:], depth+1)}
	}
	return build(ids, 0)
}

func nearestKD(node *kdNode, points []Point, x, y float64, bestID int, bestSquared float64) int {
	if node == nil {
		return bestID
	}
	point := points[node.id]
	distance := (point.X-x)*(point.X-x) + (point.Y-y)*(point.Y-y)
	if distance < bestSquared {
		bestSquared, bestID = distance, node.id
	}
	delta := point.X - x
	if node.axis == 1 {
		delta = point.Y - y
	}
	near, far := node.left, node.right
	if delta < 0 {
		near, far = node.right, node.left
	}
	bestID = nearestKD(near, points, x, y, bestID, bestSquared)
	if bestID != 0 {
		bestPoint := points[bestID]
		bestSquared = (bestPoint.X-x)*(bestPoint.X-x) + (bestPoint.Y-y)*(bestPoint.Y-y)
	}
	if delta*delta < bestSquared {
		bestID = nearestKD(far, points, x, y, bestID, bestSquared)
	}
	return bestID
}

func histogramQuantile(histogram []int64, count int64, probability, step float64) float64 {
	if count == 0 {
		return 0
	}
	target := int64(math.Ceil(probability * float64(count)))
	if target < 1 {
		target = 1
	}
	seen := int64(0)
	for index, value := range histogram {
		seen += value
		if seen >= target {
			return (float64(index) + 0.5) * step
		}
	}
	return float64(len(histogram)-1) * step
}
