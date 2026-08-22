package seabed

import (
	"container/heap"
	"fmt"
	"math"

	"coastal-geometry/internal/domain/mesh"
)

// Build назначает глубины узлам геопривязанной сетки и согласует их с
// береговой маской. Отсутствующие или отвергнутые значения остаются nil;
// неполная модель возвращается для диагностики с Accepted = false.
func Build(source mesh.Mesh, sampler ElevationSampler, config BuildConfig) (Model, error) {
	if sampler == nil {
		return Model{}, fmt.Errorf("источник батиметрических выборок не задан")
	}
	if len(source.Nodes) <= 1 {
		return Model{}, fmt.Errorf("сетка не содержит узлов")
	}
	if config.MaxSourceDistanceM < 0 {
		return Model{}, fmt.Errorf("максимальное расстояние до источника не может быть отрицательным")
	}
	if config.CoastTransitionWidthM < 0 {
		return Model{}, fmt.Errorf("ширина прибрежного перехода не может быть отрицательной")
	}
	regionThresholds, err := normalizeRegionThresholds(config.RegionThresholds)
	if err != nil {
		return Model{}, err
	}

	activeNodes, graph, meanEdgeM, err := buildNodeGraph(source)
	if err != nil {
		return Model{}, err
	}
	boundaries, err := classifyBoundaries(source, config.BoundaryOverrides)
	if err != nil {
		return Model{}, err
	}
	transitionWidthM := config.CoastTransitionWidthM
	if transitionWidthM == 0 && meanEdgeM > 0 {
		transitionWidthM = 2 * meanEdgeM
	}

	model := Model{
		Mesh:  source,
		Nodes: make([]Node, len(source.Nodes)),
		Reconciliation: ReconciliationSummary{
			TransitionWidthM: transitionWidthM,
			Corrections:      make([]Correction, 0),
		},
	}
	for nodeID := 1; nodeID < len(source.Nodes); nodeID++ {
		point := source.Nodes[nodeID]
		kind := boundaries.nodeKinds[nodeID]
		if kind == "" {
			kind = BoundaryNone
		}
		node := Node{
			ID:             nodeID,
			XM:             point.X,
			YM:             point.Y,
			LongitudeDeg:   point.LongitudeDeg,
			LatitudeDeg:    point.LatitudeDeg,
			SamplingMethod: SamplingNotSampled,
			QualityFlag:    QualityNoData,
			IsBoundary:     kind != BoundaryNone,
			BoundaryKind:   kind,
		}
		model.Nodes[nodeID] = node
		if activeNodes[nodeID] && !point.GeographicCoordinatesSet {
			return Model{}, fmt.Errorf("узел %d не содержит координаты WGS 84; сначала выполните GEO-01", nodeID)
		}
		incrementBoundaryCount(&model.Reconciliation.BoundaryCounts, kind)

		if !activeNodes[nodeID] {
			model.Nodes[nodeID].QualityFlag = QualityRejected
			model.addCorrection(Correction{
				NodeID: nodeID,
				Kind:   CorrectionOutsideAquatoryRejected,
				Reason: "узел не входит ни в одну расчётную ячейку акватории",
			})
			continue
		}

		if kind == BoundaryCoastline || kind == BoundaryIsland {
			model.assignCoastlineZero(nodeID, sampler, config.MaxSourceDistanceM)
			continue
		}
		model.sampleNode(nodeID, sampler, config.MaxSourceDistanceM)
	}

	if transitionWidthM > 0 {
		distances := coastDistances(graph, boundaries.nodeKinds, transitionWidthM)
		model.applyCoastTransition(distances, transitionWidthM)
	}
	if err := model.deriveCells(regionThresholds); err != nil {
		return Model{}, err
	}
	model.finishSummaries()
	return model, nil
}

func (model *Model) assignCoastlineZero(nodeID int, sampler ElevationSampler, maxSourceDistanceM float64) {
	node := &model.Nodes[nodeID]
	var original *float64
	if sample, err := sampler.SampleElevation(node.LatitudeDeg, node.LongitudeDeg, maxSourceDistanceM); err == nil && finite(sample.ElevationM) {
		original = floatPointer(sample.ElevationM)
	}
	zero := 0.0
	node.ElevationM = &zero
	node.WaterDepthM = floatPointer(0)
	node.SamplingMethod = SamplingCoastlineConstraint
	node.SourceDistanceM = nil
	node.QualityFlag = QualityConstrained
	correction := Correction{
		NodeID:              nodeID,
		Kind:                CorrectionCoastlineZero,
		OriginalElevationM:  original,
		CorrectedElevationM: floatPointer(0),
		Reason:              "уровень моря явно назначен береговому узлу",
	}
	if original != nil {
		correction.AdjustmentM = floatPointer(-*original)
	}
	model.addCorrection(correction)
}

func (model *Model) sampleNode(nodeID int, sampler ElevationSampler, maxSourceDistanceM float64) {
	node := &model.Nodes[nodeID]
	sample, err := sampler.SampleElevation(node.LatitudeDeg, node.LongitudeDeg, maxSourceDistanceM)
	if err != nil {
		node.QualityFlag = QualityNoData
		return
	}
	if !validSampleMethod(sample.Method) || !finite(sample.ElevationM) ||
		(sample.SourceDistanceSet && (!finite(sample.SourceDistanceM) || sample.SourceDistanceM < 0)) {
		node.QualityFlag = QualityRejected
		return
	}
	if sample.ElevationM > 0 {
		node.QualityFlag = QualityRejected
		model.addCorrection(Correction{
			NodeID:             nodeID,
			Kind:               CorrectionPositiveInsideRejected,
			OriginalElevationM: floatPointer(sample.ElevationM),
			Reason:             "положительная отметка внутри расчётной акватории отвергнута без подстановки нуля",
		})
		return
	}
	node.ElevationM = floatPointer(sample.ElevationM)
	node.WaterDepthM = floatPointer(math.Max(0, -sample.ElevationM))
	node.SamplingMethod = sample.Method
	if sample.SourceDistanceSet {
		node.SourceDistanceM = floatPointer(sample.SourceDistanceM)
	}
	if sample.Method == SamplingNearest {
		node.QualityFlag = QualityFallback
	} else {
		node.QualityFlag = QualityVerified
	}
}

func (model *Model) applyCoastTransition(distances []float64, widthM float64) {
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := &model.Nodes[nodeID]
		if node.BoundaryKind != BoundaryNone || node.ElevationM == nil || *node.ElevationM >= 0 {
			continue
		}
		distance := distances[nodeID]
		if !finite(distance) || distance <= 0 || distance >= widthM {
			continue
		}
		ratio := distance / widthM
		factor := ratio * ratio * (3 - 2*ratio)
		original := *node.ElevationM
		corrected := original * factor
		if corrected < original-1e-12 {
			model.Reconciliation.DeepenedNodeCount++
		}
		if math.Abs(corrected-original) <= 1e-12 {
			continue
		}
		node.ElevationM = floatPointer(corrected)
		node.WaterDepthM = floatPointer(math.Max(0, -corrected))
		node.QualityFlag = QualityConstrained
		model.addCorrection(Correction{
			NodeID:              nodeID,
			Kind:                CorrectionCoastTransition,
			OriginalElevationM:  floatPointer(original),
			CorrectedElevationM: floatPointer(corrected),
			AdjustmentM:         floatPointer(corrected - original),
			Reason:              fmt.Sprintf("плавный переход к береговому нулю на расстоянии %.3f м", distance),
		})
	}
}

func (model *Model) addCorrection(correction Correction) {
	model.Reconciliation.Corrections = append(model.Reconciliation.Corrections, correction)
	model.Reconciliation.TotalCorrections++
	switch correction.Kind {
	case CorrectionCoastlineZero:
		model.Reconciliation.CorrectionCounts.CoastlineZero++
	case CorrectionCoastTransition:
		model.Reconciliation.CorrectionCounts.CoastTransition++
	case CorrectionPositiveInsideRejected:
		model.Reconciliation.CorrectionCounts.PositiveInsideRejected++
	case CorrectionOutsideAquatoryRejected:
		model.Reconciliation.CorrectionCounts.OutsideAquatoryRejected++
	}
	if correction.AdjustmentM != nil && math.Abs(*correction.AdjustmentM) > model.Reconciliation.MaxAbsAdjustmentM {
		model.Reconciliation.MaxAbsAdjustmentM = math.Abs(*correction.AdjustmentM)
	}
}

func (model *Model) finishSummaries() {
	summary := SamplingSummary{TotalNodeCount: len(model.Nodes) - 1}
	distanceSum := 0.0
	distanceCount := 0
	maxDistance := 0.0
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		incrementMethodCount(&summary.MethodCounts, node.SamplingMethod)
		if node.ElevationM == nil || node.WaterDepthM == nil {
			summary.NoDataNodeCount++
			continue
		}
		summary.AssignedNodeCount++
		if node.SourceDistanceM != nil {
			distanceSum += *node.SourceDistanceM
			distanceCount++
			if *node.SourceDistanceM > maxDistance {
				maxDistance = *node.SourceDistanceM
			}
		}
	}
	if summary.TotalNodeCount > 0 {
		summary.CoveragePercent = 100 * float64(summary.AssignedNodeCount) / float64(summary.TotalNodeCount)
	}
	if distanceCount > 0 {
		summary.MeanSourceDistanceM = floatPointer(distanceSum / float64(distanceCount))
		summary.MaxSourceDistanceM = floatPointer(maxDistance)
	}
	model.Sampling = summary
	model.Accepted = summary.NoDataNodeCount == 0 &&
		model.CellDerivation.Summary.NoDataCellCount == 0 &&
		model.Reconciliation.DeepenedNodeCount == 0
	if summary.NoDataNodeCount > 0 {
		model.Reasons = append(model.Reasons, fmt.Sprintf("%d узлов не имеют принятой отметки", summary.NoDataNodeCount))
	}
	if model.Reconciliation.DeepenedNodeCount > 0 {
		model.Reasons = append(model.Reasons, "береговая коррекция создала более глубокие значения")
	}
	if model.CellDerivation.Summary.NoDataCellCount > 0 {
		model.Reasons = append(model.Reasons, fmt.Sprintf("%d ячеек исключены из производных характеристик из-за NoData", model.CellDerivation.Summary.NoDataCellCount))
	}
}

type graphEdge struct {
	to       int
	distance float64
}

func buildNodeGraph(source mesh.Mesh) ([]bool, [][]graphEdge, float64, error) {
	active := make([]bool, len(source.Nodes))
	graph := make([][]graphEdge, len(source.Nodes))
	uniqueEdges := make(map[[2]int]bool)
	totalEdgeM := 0.0
	for cellIndex, cell := range source.Cells {
		if cell.NodeCount < 3 || cell.NodeCount > 4 {
			return nil, nil, 0, fmt.Errorf("ячейка %d содержит недопустимое число узлов %d", cellIndex+1, cell.NodeCount)
		}
		for index := 0; index < cell.NodeCount; index++ {
			a := cell.Nodes[index]
			b := cell.Nodes[(index+1)%cell.NodeCount]
			if a <= 0 || a >= len(source.Nodes) || b <= 0 || b >= len(source.Nodes) || a == b {
				return nil, nil, 0, fmt.Errorf("ячейка %d содержит некорректное ребро %d–%d", cellIndex+1, a, b)
			}
			active[a], active[b] = true, true
			key := normalizedEdge(a, b)
			if uniqueEdges[key] {
				continue
			}
			uniqueEdges[key] = true
			distance := pointDistance(source.Nodes[a], source.Nodes[b])
			if distance <= 0 || !finite(distance) {
				return nil, nil, 0, fmt.Errorf("ребро %d–%d имеет некорректную длину", a, b)
			}
			graph[a] = append(graph[a], graphEdge{to: b, distance: distance})
			graph[b] = append(graph[b], graphEdge{to: a, distance: distance})
			totalEdgeM += distance
		}
	}
	if len(uniqueEdges) == 0 {
		return nil, nil, 0, fmt.Errorf("сетка не содержит рёбер расчётных ячеек")
	}
	return active, graph, totalEdgeM / float64(len(uniqueEdges)), nil
}

func pointDistance(a, b mesh.Point) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

type distanceItem struct {
	nodeID   int
	distance float64
}

type distanceQueue []distanceItem

func (queue distanceQueue) Len() int           { return len(queue) }
func (queue distanceQueue) Less(i, j int) bool { return queue[i].distance < queue[j].distance }
func (queue distanceQueue) Swap(i, j int)      { queue[i], queue[j] = queue[j], queue[i] }
func (queue *distanceQueue) Push(value any)    { *queue = append(*queue, value.(distanceItem)) }
func (queue *distanceQueue) Pop() any {
	old := *queue
	last := old[len(old)-1]
	*queue = old[:len(old)-1]
	return last
}

func coastDistances(graph [][]graphEdge, nodeKinds map[int]BoundaryKind, maxDistanceM float64) []float64 {
	distances := make([]float64, len(graph))
	for index := range distances {
		distances[index] = math.Inf(1)
	}
	queue := &distanceQueue{}
	heap.Init(queue)
	for nodeID, kind := range nodeKinds {
		if kind != BoundaryCoastline && kind != BoundaryIsland {
			continue
		}
		distances[nodeID] = 0
		heap.Push(queue, distanceItem{nodeID: nodeID})
	}
	for queue.Len() > 0 {
		current := heap.Pop(queue).(distanceItem)
		if current.distance != distances[current.nodeID] || current.distance >= maxDistanceM {
			continue
		}
		for _, edge := range graph[current.nodeID] {
			candidate := current.distance + edge.distance
			if candidate >= distances[edge.to] || candidate > maxDistanceM {
				continue
			}
			distances[edge.to] = candidate
			heap.Push(queue, distanceItem{nodeID: edge.to, distance: candidate})
		}
	}
	return distances
}

func incrementMethodCount(counts *MethodCounts, method SamplingMethod) {
	switch method {
	case SamplingExact:
		counts.Exact++
	case SamplingBilinear:
		counts.Bilinear++
	case SamplingNearest:
		counts.Nearest++
	case SamplingIrregular:
		counts.Irregular++
	case SamplingCoastlineConstraint:
		counts.CoastlineConstraint++
	default:
		counts.NotSampled++
	}
}

func incrementBoundaryCount(counts *BoundaryCounts, kind BoundaryKind) {
	switch kind {
	case BoundaryCoastline:
		counts.Coastline++
	case BoundaryIsland:
		counts.Island++
	case BoundaryOpen:
		counts.OpenBoundary++
	}
}

func validSampleMethod(method SamplingMethod) bool {
	return method == SamplingExact || method == SamplingBilinear || method == SamplingNearest || method == SamplingIrregular
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func floatPointer(value float64) *float64 {
	copy := value
	return &copy
}
