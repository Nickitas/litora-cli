package seabed

import (
	"fmt"
	"math"

	"coastal-geometry/internal/domain/mesh"
)

type boundaryClassification struct {
	edgeKinds map[[2]int]BoundaryKind
	nodeKinds map[int]BoundaryKind
}

type boundaryComponent struct {
	edges  [][2]int
	closed bool
	area   float64
}

func classifyBoundaries(source mesh.Mesh, overrides []BoundaryOverride) (boundaryClassification, error) {
	classification := boundaryClassification{
		edgeKinds: make(map[[2]int]BoundaryKind, len(source.BoundaryEdges)),
		nodeKinds: make(map[int]BoundaryKind),
	}
	if len(source.BoundaryEdges) == 0 {
		return classification, nil
	}

	edges := make(map[[2]int][2]int, len(source.BoundaryEdges))
	adjacency := make(map[int][]int)
	for _, edge := range source.BoundaryEdges {
		if err := validateBoundaryEdge(source, edge); err != nil {
			return boundaryClassification{}, err
		}
		key := normalizedEdge(edge[0], edge[1])
		if _, exists := edges[key]; exists {
			return boundaryClassification{}, fmt.Errorf("граничное ребро %d–%d повторяется", edge[0], edge[1])
		}
		edges[key] = edge
		adjacency[edge[0]] = append(adjacency[edge[0]], edge[1])
		adjacency[edge[1]] = append(adjacency[edge[1]], edge[0])
	}
	for nodeID, neighbours := range adjacency {
		if len(neighbours) > 2 {
			return boundaryClassification{}, fmt.Errorf("граничный узел %d имеет степень %d: неманифолдная граница", nodeID, len(neighbours))
		}
	}

	components, err := collectBoundaryComponents(source, edges, adjacency)
	if err != nil {
		return boundaryClassification{}, err
	}
	largestClosed := -1
	largestArea := -1.0
	for index, component := range components {
		if component.closed && math.Abs(component.area) > largestArea {
			largestClosed = index
			largestArea = math.Abs(component.area)
		}
	}
	for index, component := range components {
		kind := BoundaryOpen
		if component.closed {
			kind = BoundaryIsland
			if index == largestClosed {
				kind = BoundaryCoastline
			}
		}
		for _, edge := range component.edges {
			classification.edgeKinds[normalizedEdge(edge[0], edge[1])] = kind
		}
	}

	for _, override := range overrides {
		if override.Kind != BoundaryCoastline && override.Kind != BoundaryIsland && override.Kind != BoundaryOpen {
			return boundaryClassification{}, fmt.Errorf("для ребра %d–%d задан неизвестный тип границы %q", override.NodeA, override.NodeB, override.Kind)
		}
		key := normalizedEdge(override.NodeA, override.NodeB)
		if _, exists := edges[key]; !exists {
			return boundaryClassification{}, fmt.Errorf("переопределение %d–%d не относится к граничному ребру", override.NodeA, override.NodeB)
		}
		classification.edgeKinds[key] = override.Kind
	}

	for key, kind := range classification.edgeKinds {
		classification.nodeKinds[key[0]] = strongerBoundaryKind(classification.nodeKinds[key[0]], kind)
		classification.nodeKinds[key[1]] = strongerBoundaryKind(classification.nodeKinds[key[1]], kind)
	}
	return classification, nil
}

func collectBoundaryComponents(source mesh.Mesh, edges map[[2]int][2]int, adjacency map[int][]int) ([]boundaryComponent, error) {
	visited := make(map[[2]int]bool, len(edges))
	components := make([]boundaryComponent, 0)
	for key := range edges {
		if visited[key] {
			continue
		}
		componentEdges := make([][2]int, 0)
		componentNodes := make(map[int]bool)
		queue := []int{key[0]}
		seenNodes := map[int]bool{key[0]: true}
		for len(queue) > 0 {
			nodeID := queue[0]
			queue = queue[1:]
			componentNodes[nodeID] = true
			for _, neighbour := range adjacency[nodeID] {
				edgeKey := normalizedEdge(nodeID, neighbour)
				if !visited[edgeKey] {
					visited[edgeKey] = true
					componentEdges = append(componentEdges, edges[edgeKey])
				}
				if !seenNodes[neighbour] {
					seenNodes[neighbour] = true
					queue = append(queue, neighbour)
				}
			}
		}
		nodes := make([]int, 0, len(componentNodes))
		closed := true
		start := 0
		for nodeID := range componentNodes {
			nodes = append(nodes, nodeID)
			degree := len(adjacency[nodeID])
			if degree != 2 {
				closed = false
			}
			if degree == 1 {
				start = nodeID
			}
		}
		component := boundaryComponent{edges: componentEdges, closed: closed}
		if closed {
			start = nodes[0]
			ordered, orderErr := orderBoundaryComponent(start, adjacency, len(componentNodes), true)
			if orderErr != nil {
				return nil, orderErr
			}
			component.area = boundaryArea(source.Nodes, ordered)
		} else {
			if start == 0 {
				return nil, fmt.Errorf("открытая граница не имеет конечного узла")
			}
			if _, orderErr := orderBoundaryComponent(start, adjacency, len(componentNodes), false); orderErr != nil {
				return nil, orderErr
			}
		}
		components = append(components, component)
	}
	return components, nil
}

func orderBoundaryComponent(start int, adjacency map[int][]int, nodeCount int, closed bool) ([]int, error) {
	ordered := make([]int, 0, nodeCount)
	previous, current := 0, start
	for len(ordered) < nodeCount {
		ordered = append(ordered, current)
		neighbours := adjacency[current]
		next := 0
		for _, candidate := range neighbours {
			if candidate != previous {
				next = candidate
				break
			}
		}
		if next == 0 {
			if !closed && len(ordered) == nodeCount {
				break
			}
			return nil, fmt.Errorf("не удалось упорядочить граничную компоненту от узла %d", start)
		}
		previous, current = current, next
		if current == start {
			if closed && len(ordered) == nodeCount {
				break
			}
			return nil, fmt.Errorf("граничная компонента преждевременно замкнулась в узле %d", start)
		}
	}
	return ordered, nil
}

func boundaryArea(nodes []mesh.Point, ordered []int) float64 {
	area := 0.0
	for index, nodeID := range ordered {
		nextID := ordered[(index+1)%len(ordered)]
		area += nodes[nodeID].X*nodes[nextID].Y - nodes[nextID].X*nodes[nodeID].Y
	}
	return area / 2
}

func validateBoundaryEdge(source mesh.Mesh, edge [2]int) error {
	if edge[0] <= 0 || edge[0] >= len(source.Nodes) || edge[1] <= 0 || edge[1] >= len(source.Nodes) || edge[0] == edge[1] {
		return fmt.Errorf("некорректное граничное ребро %d–%d", edge[0], edge[1])
	}
	return nil
}

func normalizedEdge(a, b int) [2]int {
	if a > b {
		a, b = b, a
	}
	return [2]int{a, b}
}

func strongerBoundaryKind(current, candidate BoundaryKind) BoundaryKind {
	if boundaryPriority(candidate) > boundaryPriority(current) {
		return candidate
	}
	return current
}

func boundaryPriority(kind BoundaryKind) int {
	switch kind {
	case BoundaryOpen:
		return 3
	case BoundaryIsland:
		return 2
	case BoundaryCoastline:
		return 1
	default:
		return 0
	}
}
