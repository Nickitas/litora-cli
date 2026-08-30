package adaptive

import (
	"container/heap"
	"fmt"
	"math"
	"sort"

	"coastal-geometry/internal/domain/seabed"
)

const comparisonTolerance = 1e-9

type coastSegment struct {
	a int
	b int
}

type fieldEdge struct {
	a      int
	b      int
	length float64
}

type neighbour struct {
	id     int
	length float64
}

type queueItem struct {
	nodeID int
	value  float64
}

type sizeQueue []queueItem

func (queue sizeQueue) Len() int           { return len(queue) }
func (queue sizeQueue) Less(i, j int) bool { return queue[i].value < queue[j].value }
func (queue sizeQueue) Swap(i, j int)      { queue[i], queue[j] = queue[j], queue[i] }
func (queue *sizeQueue) Push(value any)    { *queue = append(*queue, value.(queueItem)) }
func (queue *sizeQueue) Pop() any {
	old := *queue
	last := old[len(old)-1]
	*queue = old[:len(old)-1]
	return last
}

// BuildSizeField рассчитывает узловое поле размера по принятой модели дна.
// Функция детерминирована и не содержит случайных параметров генератора сетки.
func BuildSizeField(model seabed.Model, config Config) (Field, error) {
	if err := validateInputs(model, config); err != nil {
		return Field{}, err
	}
	edges, adjacency, err := buildFieldGraph(model)
	if err != nil {
		return Field{}, err
	}
	segments, coastAdjacency, err := collectCoastSegments(model)
	if err != nil {
		return Field{}, err
	}
	curvature := coastlineCurvature(model, coastAdjacency)
	gradient, gradientAvailable := nodeDepthGradients(model)

	nodes := make([]NodeValue, len(model.Nodes)-1)
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		distanceM, curvatureDeg := nearestCoastFeature(model, nodeID, segments, curvature)
		depthM, depthAvailable := 0.0, false
		if node.WaterDepthM != nil && finite(*node.WaterDepthM) && *node.WaterDepthM >= 0 {
			depthM, depthAvailable = *node.WaterDepthM, true
		}
		gradientDeg := gradient[nodeID]
		effectiveGradientDeg := gradientDeg
		if !gradientAvailable[nodeID] {
			// При NoData используется консервативное уточнение, но сам факт
			// замены остаётся явным в CSV и сводке.
			effectiveGradientDeg = config.SlopeReferenceDeg
		}
		base := baseSize(depthM, depthAvailable, model.CellDerivation.RegionThresholds, config)
		attenuation := math.Exp(-distanceM / config.CoastInfluenceM)
		distanceRefinement := math.Max(0, base-config.StraightCoastSizeM) * attenuation
		curvatureSignal := clamp(curvatureDeg/config.CurvatureReferenceDeg, 0, 1)
		curvatureRefinement := (config.StraightCoastSizeM - config.MinSizeM) * curvatureSignal * attenuation
		gradientSignal := clamp(effectiveGradientDeg/config.SlopeReferenceDeg, 0, 1)
		gradientRefinement := (config.ShelfSizeM - config.StraightCoastSizeM) * gradientSignal
		raw := clamp(base-distanceRefinement-curvatureRefinement-gradientRefinement, config.MinSizeM, config.DeepSizeM)
		nodes[nodeID-1] = NodeValue{
			NodeID: nodeID, XM: node.XM, YM: node.YM,
			LongitudeDeg: node.LongitudeDeg, LatitudeDeg: node.LatitudeDeg,
			WaterDepthM: depthM, DepthAvailable: depthAvailable,
			DistanceToCoastM: distanceM, CoastCurvatureDeg: curvatureDeg,
			DepthGradientDeg: gradientDeg, EffectiveGradientDeg: effectiveGradientDeg,
			GradientAvailable: gradientAvailable[nodeID],
			BaseSizeM:         base, DistanceRefinementM: distanceRefinement,
			CurvatureRefinementM: curvatureRefinement, GradientRefinementM: gradientRefinement,
			RawTargetSizeM: raw, TargetSizeM: raw,
			Zone: classifyZone(node, distanceM, depthM, depthAvailable, effectiveGradientDeg, model.CellDerivation.RegionThresholds, config),
		}
	}

	rawRatio, rawGradient := adjacencyMetrics(nodes, edges, false)
	limitGrowth(nodes, adjacency, config)
	finalRatio, finalGradient := adjacencyMetrics(nodes, edges, true)
	if finalRatio <= config.MaxNeighbourRatio+1e-12 {
		finalRatio = math.Min(finalRatio, config.MaxNeighbourRatio)
	}
	if finalGradient <= config.MaxSizeGradientPerM+1e-12 {
		finalGradient = math.Min(finalGradient, config.MaxSizeGradientPerM)
	}
	report := buildReport(model, nodes, len(segments), rawRatio, rawGradient, finalRatio, finalGradient, config)
	return Field{Nodes: nodes, Report: report}, nil
}

func validateInputs(model seabed.Model, config Config) error {
	if !model.Accepted {
		return fmt.Errorf("поле размера строится только для принятой модели lito-seabed/v1")
	}
	if len(model.Nodes) <= 1 || len(model.Mesh.Cells) == 0 {
		return fmt.Errorf("модель не содержит узлов и ячеек для поля размера")
	}
	values := []float64{
		config.MinSizeM, config.StraightCoastSizeM, config.ShelfSizeM, config.DeepSizeM,
		config.CoastInfluenceM, config.CurvatureReferenceDeg, config.SlopeReferenceDeg,
		config.FlatDeepMaxSlopeDeg, config.MaxNeighbourRatio, config.MaxSizeGradientPerM,
	}
	for _, value := range values {
		if !finite(value) || value <= 0 {
			return fmt.Errorf("все параметры поля размера должны быть конечными и положительными")
		}
	}
	if config.MinSizeM > config.StraightCoastSizeM || config.StraightCoastSizeM > config.ShelfSizeM || config.ShelfSizeM > config.DeepSizeM {
		return fmt.Errorf("размеры должны удовлетворять min ≤ coast ≤ shelf ≤ deep")
	}
	if config.MaxNeighbourRatio <= 1 {
		return fmt.Errorf("максимальное отношение соседних размеров должно быть больше 1")
	}
	thresholds := model.CellDerivation.RegionThresholds
	if thresholds.ShelfMaxDepthM <= thresholds.CoastMaxDepthM || thresholds.SlopeMaxDepthM <= thresholds.ShelfMaxDepthM {
		return fmt.Errorf("модель не содержит согласованных порогов морфометрических зон")
	}
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		if node.ID != nodeID || !finite(node.XM) || !finite(node.YM) {
			return fmt.Errorf("узел %d имеет некорректный идентификатор или координаты", nodeID)
		}
	}
	return nil
}

func buildFieldGraph(model seabed.Model) ([]fieldEdge, map[int][]neighbour, error) {
	unique := make(map[[2]int]bool)
	edges := make([]fieldEdge, 0, 2*len(model.Mesh.Cells))
	adjacency := make(map[int][]neighbour, len(model.Nodes)-1)
	for cellIndex, cell := range model.Mesh.Cells {
		if cell.NodeCount != 4 {
			return nil, nil, fmt.Errorf("ячейка %d содержит %d узлов: ADAPT-01 принимает четырёхугольный каркас", cellIndex+1, cell.NodeCount)
		}
		for side := 0; side < 4; side++ {
			a, b := cell.Nodes[side], cell.Nodes[(side+1)%4]
			if a <= 0 || a >= len(model.Nodes) || b <= 0 || b >= len(model.Nodes) || a == b {
				return nil, nil, fmt.Errorf("ячейка %d содержит некорректное ребро %d–%d", cellIndex+1, a, b)
			}
			key := normalizedEdge(a, b)
			if unique[key] {
				continue
			}
			unique[key] = true
			left, right := model.Nodes[a], model.Nodes[b]
			length := math.Hypot(right.XM-left.XM, right.YM-left.YM)
			if !finite(length) || length <= 0 {
				return nil, nil, fmt.Errorf("ребро %d–%d имеет нулевую или некорректную длину", a, b)
			}
			edges = append(edges, fieldEdge{a: a, b: b, length: length})
			adjacency[a] = append(adjacency[a], neighbour{id: b, length: length})
			adjacency[b] = append(adjacency[b], neighbour{id: a, length: length})
		}
	}
	return edges, adjacency, nil
}

func collectCoastSegments(model seabed.Model) ([]coastSegment, map[int][]int, error) {
	segments := make([]coastSegment, 0, len(model.BoundaryEdges))
	adjacency := make(map[int][]int)
	for _, edge := range model.BoundaryEdges {
		if edge.Kind != seabed.BoundaryCoastline && edge.Kind != seabed.BoundaryIsland {
			continue
		}
		a, b := edge.NodeIDs[0], edge.NodeIDs[1]
		if a <= 0 || a >= len(model.Nodes) || b <= 0 || b >= len(model.Nodes) || a == b {
			return nil, nil, fmt.Errorf("береговое ребро %d–%d некорректно", a, b)
		}
		segments = append(segments, coastSegment{a: a, b: b})
		adjacency[a] = appendUnique(adjacency[a], b)
		adjacency[b] = appendUnique(adjacency[b], a)
	}
	if len(segments) == 0 {
		return nil, nil, fmt.Errorf("модель не содержит физической границы берега или островов")
	}
	return segments, adjacency, nil
}

func coastlineCurvature(model seabed.Model, adjacency map[int][]int) map[int]float64 {
	result := make(map[int]float64, len(adjacency))
	for nodeID, neighbours := range adjacency {
		if len(neighbours) < 2 {
			continue
		}
		origin := model.Nodes[nodeID]
		maxDeflection := 0.0
		for first := 0; first < len(neighbours); first++ {
			for second := first + 1; second < len(neighbours); second++ {
				a, b := model.Nodes[neighbours[first]], model.Nodes[neighbours[second]]
				ax, ay := a.XM-origin.XM, a.YM-origin.YM
				bx, by := b.XM-origin.XM, b.YM-origin.YM
				denominator := math.Hypot(ax, ay) * math.Hypot(bx, by)
				if denominator <= 0 {
					continue
				}
				angle := math.Acos(clamp((ax*bx+ay*by)/denominator, -1, 1))
				deflection := (math.Pi - angle) * 180 / math.Pi
				maxDeflection = math.Max(maxDeflection, deflection)
			}
		}
		result[nodeID] = maxDeflection
	}
	return result
}

func nearestCoastFeature(model seabed.Model, nodeID int, segments []coastSegment, curvature map[int]float64) (float64, float64) {
	point := model.Nodes[nodeID]
	minimumSquared := math.Inf(1)
	nearestCurvature := 0.0
	for _, segment := range segments {
		a, b := model.Nodes[segment.a], model.Nodes[segment.b]
		distanceSquared, position := squaredPointSegmentDistance(point.XM, point.YM, a.XM, a.YM, b.XM, b.YM)
		localCurvature := curvature[segment.a]*(1-position) + curvature[segment.b]*position
		if distanceSquared < minimumSquared-comparisonTolerance || (math.Abs(distanceSquared-minimumSquared) <= comparisonTolerance && localCurvature > nearestCurvature) {
			minimumSquared = distanceSquared
			nearestCurvature = localCurvature
		}
	}
	if ownCurvature, ok := curvature[nodeID]; ok && minimumSquared <= comparisonTolerance {
		nearestCurvature = math.Max(nearestCurvature, ownCurvature)
	}
	return math.Sqrt(math.Max(0, minimumSquared)), nearestCurvature
}

func nodeDepthGradients(model seabed.Model) (map[int]float64, map[int]bool) {
	gradient := make(map[int]float64, len(model.Nodes)-1)
	available := make(map[int]bool, len(model.Nodes)-1)
	for _, cell := range model.Cells {
		if !finite(cell.SlopeDeg) || cell.SlopeDeg < 0 {
			continue
		}
		for _, nodeID := range cell.NodeIDs {
			gradient[nodeID] = math.Max(gradient[nodeID], cell.SlopeDeg)
			available[nodeID] = true
		}
	}
	return gradient, available
}

func baseSize(depthM float64, available bool, thresholds seabed.RegionThresholds, config Config) float64 {
	if !available {
		return config.ShelfSizeM
	}
	if depthM <= thresholds.ShelfMaxDepthM {
		return config.ShelfSizeM
	}
	if depthM >= thresholds.SlopeMaxDepthM {
		return config.DeepSizeM
	}
	ratio := (depthM - thresholds.ShelfMaxDepthM) / (thresholds.SlopeMaxDepthM - thresholds.ShelfMaxDepthM)
	return config.ShelfSizeM + ratio*(config.DeepSizeM-config.ShelfSizeM)
}

func limitGrowth(nodes []NodeValue, adjacency map[int][]neighbour, config Config) {
	queue := make(sizeQueue, 0, len(nodes))
	for index := range nodes {
		heap.Push(&queue, queueItem{nodeID: nodes[index].NodeID, value: nodes[index].TargetSizeM})
	}
	for queue.Len() > 0 {
		item := heap.Pop(&queue).(queueItem)
		current := &nodes[item.nodeID-1]
		if math.Abs(item.value-current.TargetSizeM) > comparisonTolerance {
			continue
		}
		for _, next := range adjacency[item.nodeID] {
			allowedByRatio := current.TargetSizeM * config.MaxNeighbourRatio
			allowedByDistance := current.TargetSizeM + config.MaxSizeGradientPerM*next.length
			allowed := math.Min(allowedByRatio, allowedByDistance)
			neighbourValue := &nodes[next.id-1]
			if neighbourValue.TargetSizeM <= allowed+comparisonTolerance {
				continue
			}
			neighbourValue.TargetSizeM = math.Max(config.MinSizeM, allowed)
			neighbourValue.GrowthLimited = true
			heap.Push(&queue, queueItem{nodeID: next.id, value: neighbourValue.TargetSizeM})
		}
	}
}

func buildReport(model seabed.Model, nodes []NodeValue, segmentCount int, rawRatio, rawGradient, finalRatio, finalGradient float64, config Config) Report {
	rawValues := make([]float64, 0, len(nodes))
	finalValues := make([]float64, 0, len(nodes))
	zones := make(map[string][]float64)
	summary := Summary{
		NodeCount: len(nodes), CellCount: len(model.Mesh.Cells), CoastlineSegmentCount: segmentCount,
		RawMaxAdjacentRatio: rawRatio, FinalMaxAdjacentRatio: finalRatio,
		RawMaxSizeGradientPerM: rawGradient, FinalMaxSizeGradientPerM: finalGradient,
	}
	for _, node := range nodes {
		rawValues = append(rawValues, node.RawTargetSizeM)
		finalValues = append(finalValues, node.TargetSizeM)
		zones[node.Zone] = append(zones[node.Zone], node.TargetSizeM)
		if !node.DepthAvailable {
			summary.NoDataNodeCount++
		}
		if node.GrowthLimited {
			summary.GrowthLimitedNodeCount++
		}
		summary.MaxDistanceToCoastM = math.Max(summary.MaxDistanceToCoastM, node.DistanceToCoastM)
		summary.MaxCoastCurvatureDeg = math.Max(summary.MaxCoastCurvatureDeg, node.CoastCurvatureDeg)
		summary.MaxDepthGradientDeg = math.Max(summary.MaxDepthGradientDeg, node.DepthGradientDeg)
	}
	summary.RawTarget = sizeStatistics(rawValues)
	summary.Target = sizeStatistics(finalValues)
	zoneOrder := []string{"coastline", "near_coast", "shelf", "slope", "basin_complex", "basin_flat", "no_data"}
	for _, id := range zoneOrder {
		values := zones[id]
		if len(values) == 0 {
			continue
		}
		summary.Zones = append(summary.Zones, ZoneSummary{ID: id, Name: zoneName(id), NodeCount: len(values), Target: sizeStatistics(values)})
	}
	return Report{
		SchemaVersion:       SchemaVersion,
		HorizontalReference: "X/Y LAEA без изменения из входного lito-seabed/v1",
		HorizontalUnit:      HorizontalUnit, TargetSizeUnit: "m",
		AdaptiveMeshGenerated: false, Config: config,
		Formula: Formula{
			BaseSize:       "shelf_size_m до границы шельфа; линейный рост shelf_size_m→deep_size_m на склоне; deep_size_m в котловине",
			CoastDistance:  "(h_base − straight_coast_size_m) · exp(−distance_to_coast / coast_influence_m)",
			CoastCurvature: "(straight_coast_size_m − min_size_m) · clamp(curvature / curvature_reference_deg, 0, 1) · coast_attenuation",
			DepthGradient:  "(shelf_size_m − straight_coast_size_m) · clamp(effective_gradient / slope_reference_deg, 0, 1)",
			RawTarget:      "clamp(h_base − distance_refinement − curvature_refinement − gradient_refinement, min_size_m, deep_size_m)",
			GrowthLimit:    "для каждого ребра h_j ≤ min(h_i · max_neighbour_ratio, h_i + edge_length · max_size_gradient_per_m)",
		},
		Summary: summary,
	}
}

func classifyZone(node seabed.Node, distanceM, depthM float64, depthAvailable bool, gradientDeg float64, thresholds seabed.RegionThresholds, config Config) string {
	if node.BoundaryKind == seabed.BoundaryCoastline || node.BoundaryKind == seabed.BoundaryIsland {
		return "coastline"
	}
	if distanceM <= config.CoastInfluenceM {
		return "near_coast"
	}
	if !depthAvailable {
		return "no_data"
	}
	if depthM <= thresholds.ShelfMaxDepthM {
		return "shelf"
	}
	if depthM < thresholds.SlopeMaxDepthM {
		return "slope"
	}
	if gradientDeg <= config.FlatDeepMaxSlopeDeg {
		return "basin_flat"
	}
	return "basin_complex"
}

func zoneName(id string) string {
	switch id {
	case "coastline":
		return "Берег и острова"
	case "near_coast":
		return "Прибрежная полоса"
	case "shelf":
		return "Шельф"
	case "slope":
		return "Материковый склон"
	case "basin_complex":
		return "Расчленённая котловина"
	case "basin_flat":
		return "Ровное глубоководье"
	default:
		return "Нет данных"
	}
}

func adjacencyMetrics(nodes []NodeValue, edges []fieldEdge, final bool) (float64, float64) {
	maxRatio, maxGradient := 1.0, 0.0
	for _, edge := range edges {
		left, right := nodes[edge.a-1], nodes[edge.b-1]
		a, b := left.RawTargetSizeM, right.RawTargetSizeM
		if final {
			a, b = left.TargetSizeM, right.TargetSizeM
		}
		ratio := math.Max(a/b, b/a)
		gradient := math.Abs(a-b) / edge.length
		maxRatio = math.Max(maxRatio, ratio)
		maxGradient = math.Max(maxGradient, gradient)
	}
	return maxRatio, maxGradient
}

func sizeStatistics(values []float64) SizeStatistics {
	if len(values) == 0 {
		return SizeStatistics{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	sum := 0.0
	for _, value := range sorted {
		sum += value
	}
	return SizeStatistics{
		MinM: sorted[0], P05M: percentile(sorted, 0.05), MedianM: percentile(sorted, 0.5),
		MeanM: sum / float64(len(sorted)), P95M: percentile(sorted, 0.95), MaxM: sorted[len(sorted)-1],
	}
}

func percentile(sorted []float64, probability float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	position := probability * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	weight := position - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

func squaredPointSegmentDistance(px, py, ax, ay, bx, by float64) (float64, float64) {
	dx, dy := bx-ax, by-ay
	denominator := dx*dx + dy*dy
	if denominator <= 0 {
		distanceX, distanceY := px-ax, py-ay
		return distanceX*distanceX + distanceY*distanceY, 0
	}
	position := clamp(((px-ax)*dx+(py-ay)*dy)/denominator, 0, 1)
	qx, qy := ax+position*dx, ay+position*dy
	distanceX, distanceY := px-qx, py-qy
	return distanceX*distanceX + distanceY*distanceY, position
}

func appendUnique(values []int, candidate int) []int {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func normalizedEdge(a, b int) [2]int {
	if a > b {
		a, b = b, a
	}
	return [2]int{a, b}
}

func clamp(value, minimum, maximum float64) float64 {
	return math.Max(minimum, math.Min(maximum, value))
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
