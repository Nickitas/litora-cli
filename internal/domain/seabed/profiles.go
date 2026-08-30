package seabed

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
)

const deepProfileFraction = 0.9

// ProfileSelectionReport фиксирует происхождение автоматически выбранной
// трассы «берег → глубоководье» и её метрические характеристики.
type ProfileSelectionReport struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	SelectionBasis string  `json:"selection_basis"`
	StartNodeID    int     `json:"start_node_id"`
	EndNodeID      int     `json:"end_node_id"`
	PointCount     int     `json:"point_count"`
	LengthM        float64 `json:"length_m"`
	StartDepthM    float64 `json:"start_depth_m"`
	EndDepthM      float64 `json:"end_depth_m"`
	MaxDepthM      float64 `json:"max_depth_m"`
}

type profileAnchorSpec struct {
	id         string
	name       string
	basis      string
	coordinate func(Node) float64
	maximum    bool
}

type profileGraphEdge struct {
	nodeID int
	weight float64
}

type profileQueueItem struct {
	nodeID   int
	distance float64
}

type profilePriorityQueue []*profileQueueItem

func (queue profilePriorityQueue) Len() int { return len(queue) }
func (queue profilePriorityQueue) Less(left, right int) bool {
	if queue[left].distance == queue[right].distance {
		return queue[left].nodeID < queue[right].nodeID
	}
	return queue[left].distance < queue[right].distance
}
func (queue profilePriorityQueue) Swap(left, right int) {
	queue[left], queue[right] = queue[right], queue[left]
}
func (queue *profilePriorityQueue) Push(value any) {
	*queue = append(*queue, value.(*profileQueueItem))
}
func (queue *profilePriorityQueue) Pop() any {
	old := *queue
	last := old[len(old)-1]
	old[len(old)-1] = nil
	*queue = old[:len(old)-1]
	return last
}

// SelectCoastToDeepProfiles выбирает западный, восточный и северный береговые
// узлы, находит ближайшие к ним узлы глубоководного ядра и соединяет пары
// оптимальными путями по фактическим рёбрам четырёхугольной сетки. Вес пути
// штрафует повторный выход на берег и уменьшение глубины. Одинаковая модель
// всегда даёт одинаковые трассы и порядок узлов.
func SelectCoastToDeepProfiles(model Model) ([]Profile, []ProfileSelectionReport, error) {
	if err := validateMSHModel(model); err != nil {
		return nil, nil, fmt.Errorf("проверка модели для автоматических профилей: %w", err)
	}
	coastNodes := externalCoastNodeIDs(model)
	if len(coastNodes) < 3 {
		return nil, nil, fmt.Errorf("для профилей нужны минимум три узла внешнего берега")
	}
	deepNodes, _ := deepCoreNodeIDs(model)
	if len(deepNodes) == 0 {
		return nil, nil, fmt.Errorf("модель не содержит глубоководного ядра для профилей")
	}
	graph := buildProfileGraph(model)
	if len(graph) == 0 {
		return nil, nil, fmt.Errorf("модель не содержит рёбер для профильных трасс")
	}

	specs := []profileAnchorSpec{
		{id: "profile-west", name: "A · западный берег → глубоководье", basis: "крайний западный узел внешнего берега", coordinate: func(node Node) float64 { return node.LongitudeDeg }},
		{id: "profile-east", name: "B · восточный берег → глубоководье", basis: "крайний восточный узел внешнего берега", coordinate: func(node Node) float64 { return node.LongitudeDeg }, maximum: true},
		{id: "profile-north", name: "C · северный берег → глубоководье", basis: "крайний северный узел внешнего берега", coordinate: func(node Node) float64 { return node.LatitudeDeg }, maximum: true},
	}
	usedAnchors := make(map[int]bool, len(specs))
	usedTargets := make(map[int]bool, len(specs))
	profiles := make([]Profile, 0, len(specs))
	reports := make([]ProfileSelectionReport, 0, len(specs))
	for _, spec := range specs {
		anchor := selectProfileAnchor(model, coastNodes, usedAnchors, spec)
		if anchor == 0 {
			return nil, nil, fmt.Errorf("не удалось выбрать независимый береговой узел для %s", spec.name)
		}
		usedAnchors[anchor] = true
		target := nearestDeepNode(model, anchor, deepNodes, usedTargets)
		if target == 0 {
			target = nearestDeepNode(model, anchor, deepNodes, nil)
		}
		if target == 0 {
			return nil, nil, fmt.Errorf("не удалось выбрать глубоководный узел для %s", spec.name)
		}
		usedTargets[target] = true
		path, err := shortestProfilePath(model, graph, anchor, target)
		if err != nil {
			return nil, nil, fmt.Errorf("построение %s: %w", spec.name, err)
		}
		profile := Profile{ID: spec.id, Name: spec.name, NodeIDs: path}
		profiles = append(profiles, profile)
		reports = append(reports, summarizeSelectedProfile(model, profile, spec.basis))
	}
	if err := validateProfiles(model, profiles); err != nil {
		return nil, nil, err
	}
	return profiles, reports, nil
}

func externalCoastNodeIDs(model Model) []int {
	unique := make(map[int]bool)
	for _, edge := range model.BoundaryEdges {
		if edge.Kind != BoundaryCoastline {
			continue
		}
		for _, nodeID := range edge.NodeIDs {
			if nodeID > 0 && nodeID < len(model.Nodes) && model.Nodes[nodeID].WaterDepthM != nil {
				unique[nodeID] = true
			}
		}
	}
	result := make([]int, 0, len(unique))
	for nodeID := range unique {
		result = append(result, nodeID)
	}
	sort.Ints(result)
	return result
}

func deepCoreNodeIDs(model Model) ([]int, float64) {
	maxDepthM := 0.0
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		if depth := model.Nodes[nodeID].WaterDepthM; depth != nil && *depth > maxDepthM {
			maxDepthM = *depth
		}
	}
	threshold := deepProfileFraction * maxDepthM
	result := make([]int, 0, len(model.Nodes)/8)
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		if depth := model.Nodes[nodeID].WaterDepthM; depth != nil && *depth >= threshold {
			result = append(result, nodeID)
		}
	}
	return result, maxDepthM
}

func selectProfileAnchor(model Model, candidates []int, used map[int]bool, spec profileAnchorSpec) int {
	bestID := 0
	bestValue := math.Inf(1)
	if spec.maximum {
		bestValue = math.Inf(-1)
	}
	for _, nodeID := range candidates {
		if used[nodeID] {
			continue
		}
		value := spec.coordinate(model.Nodes[nodeID])
		better := value < bestValue
		if spec.maximum {
			better = value > bestValue
		}
		if better || (value == bestValue && (bestID == 0 || nodeID < bestID)) {
			bestID, bestValue = nodeID, value
		}
	}
	return bestID
}

func nearestDeepNode(model Model, anchorID int, candidates []int, excluded map[int]bool) int {
	anchor := model.Nodes[anchorID]
	bestID := 0
	bestDistance := math.Inf(1)
	for _, nodeID := range candidates {
		if excluded != nil && excluded[nodeID] {
			continue
		}
		node := model.Nodes[nodeID]
		distance := math.Hypot(node.XM-anchor.XM, node.YM-anchor.YM)
		if distance < bestDistance || (distance == bestDistance && (bestID == 0 || nodeID < bestID)) {
			bestID, bestDistance = nodeID, distance
		}
	}
	return bestID
}

func buildProfileGraph(model Model) map[int][]profileGraphEdge {
	type edgeKey struct{ a, b int }
	unique := make(map[edgeKey]bool, len(model.Cells)*2)
	graph := make(map[int][]profileGraphEdge, len(model.Nodes)-1)
	for _, cell := range model.Cells {
		for index, nodeID := range cell.NodeIDs {
			nextID := cell.NodeIDs[(index+1)%4]
			if nodeID <= 0 || nextID <= 0 || nodeID >= len(model.Nodes) || nextID >= len(model.Nodes) {
				continue
			}
			a, b := nodeID, nextID
			if a > b {
				a, b = b, a
			}
			key := edgeKey{a: a, b: b}
			if unique[key] {
				continue
			}
			unique[key] = true
			left, right := model.Nodes[a], model.Nodes[b]
			weight := math.Hypot(right.XM-left.XM, right.YM-left.YM)
			if weight <= 0 {
				continue
			}
			graph[a] = append(graph[a], profileGraphEdge{nodeID: b, weight: weight})
			graph[b] = append(graph[b], profileGraphEdge{nodeID: a, weight: weight})
		}
	}
	for nodeID := range graph {
		sort.Slice(graph[nodeID], func(left, right int) bool { return graph[nodeID][left].nodeID < graph[nodeID][right].nodeID })
	}
	return graph
}

func shortestProfilePath(model Model, graph map[int][]profileGraphEdge, startID, targetID int) ([]int, error) {
	distance := make([]float64, len(model.Nodes))
	previous := make([]int, len(model.Nodes))
	_, modelMaxDepthM := deepCoreNodeIDs(model)
	for index := range distance {
		distance[index] = math.Inf(1)
	}
	distance[startID] = 0
	queue := profilePriorityQueue{&profileQueueItem{nodeID: startID, distance: 0}}
	heap.Init(&queue)
	for queue.Len() > 0 {
		current := heap.Pop(&queue).(*profileQueueItem)
		if current.distance > distance[current.nodeID] {
			continue
		}
		if current.nodeID == targetID {
			break
		}
		for _, edge := range graph[current.nodeID] {
			weight := profileTraversalWeight(model, current.nodeID, edge, startID, targetID, modelMaxDepthM)
			candidate := current.distance + weight
			if candidate < distance[edge.nodeID] || (candidate == distance[edge.nodeID] && current.nodeID < previous[edge.nodeID]) {
				distance[edge.nodeID] = candidate
				previous[edge.nodeID] = current.nodeID
				heap.Push(&queue, &profileQueueItem{nodeID: edge.nodeID, distance: candidate})
			}
		}
	}
	if math.IsInf(distance[targetID], 1) {
		return nil, fmt.Errorf("узлы %d и %d лежат в разных компонентах сетки", startID, targetID)
	}
	path := []int{targetID}
	for current := targetID; current != startID; {
		current = previous[current]
		if current == 0 {
			return nil, fmt.Errorf("не удалось восстановить путь %d → %d", startID, targetID)
		}
		path = append(path, current)
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path, nil
}

func profileTraversalWeight(model Model, currentID int, edge profileGraphEdge, startID, targetID int, maxDepthM float64) float64 {
	weight := edge.weight
	next := model.Nodes[edge.nodeID]
	if edge.nodeID != startID && edge.nodeID != targetID && (next.BoundaryKind == BoundaryCoastline || next.BoundaryKind == BoundaryIsland) {
		weight *= 25
	}
	currentDepth, nextDepth := 0.0, 0.0
	if model.Nodes[currentID].WaterDepthM != nil {
		currentDepth = *model.Nodes[currentID].WaterDepthM
	}
	if next.WaterDepthM != nil {
		nextDepth = *next.WaterDepthM
	}
	if maxDepthM > 0 && nextDepth < currentDepth {
		weight *= 1 + 30*(currentDepth-nextDepth)/maxDepthM
	}
	return weight
}

func summarizeSelectedProfile(model Model, profile Profile, basis string) ProfileSelectionReport {
	report := ProfileSelectionReport{
		ID: profile.ID, Name: profile.Name, SelectionBasis: basis,
		StartNodeID: profile.NodeIDs[0], EndNodeID: profile.NodeIDs[len(profile.NodeIDs)-1],
		PointCount: len(profile.NodeIDs),
	}
	for index, nodeID := range profile.NodeIDs {
		node := model.Nodes[nodeID]
		depth := *node.WaterDepthM
		if index == 0 {
			report.StartDepthM = depth
		} else {
			previous := model.Nodes[profile.NodeIDs[index-1]]
			report.LengthM += math.Hypot(node.XM-previous.XM, node.YM-previous.YM)
		}
		report.MaxDepthM = math.Max(report.MaxDepthM, depth)
	}
	report.EndDepthM = *model.Nodes[report.EndNodeID].WaterDepthM
	return report
}
