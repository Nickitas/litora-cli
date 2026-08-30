package seabed

import (
	"fmt"
	"math"
	"sort"
)

// ModelDepthSample описывает выборку положительной глубины из готовой модели
// дна. Градиенты заданы в метрах глубины на метр LAEA.
type ModelDepthSample struct {
	WaterDepthM       float64
	GradientX         float64
	GradientY         float64
	GradientAvailable bool
	NearestFallback   bool
	SourceDistanceM   float64
}

// ModelDepthSampler индексирует четырёхугольную модель дна для многократной
// выборки в произвольных точках LAEA. Каждая ячейка делится одной и той же
// диагональю на два треугольника, а выход за покрытие остаётся явным fallback.
type ModelDepthSampler struct {
	nodes         []Node
	triangles     []modelDepthTriangle
	bins          [][]int
	columns, rows int
	minX, minY    float64
	maxX, maxY    float64
	stepX, stepY  float64
	nearest       *modelDepthKDNode
}

type modelDepthTriangle struct {
	nodeIDs    [3]int
	minX, minY float64
	maxX, maxY float64
	gradientX  float64
	gradientY  float64
}

// NewModelDepthSampler создаёт пространственный индекс принятой модели дна.
func NewModelDepthSampler(model Model) (*ModelDepthSampler, error) {
	if len(model.Nodes) <= 1 || len(model.Mesh.Cells) == 0 {
		return nil, fmt.Errorf("модель дна не содержит узлов или ячеек")
	}
	index := &ModelDepthSampler{
		nodes: model.Nodes,
		minX:  math.Inf(1), minY: math.Inf(1),
		maxX: math.Inf(-1), maxY: math.Inf(-1),
	}
	validNodeIDs := make([]int, 0, len(model.Nodes)-1)
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		if node.WaterDepthM == nil || !finite(*node.WaterDepthM) || *node.WaterDepthM < 0 {
			continue
		}
		validNodeIDs = append(validNodeIDs, nodeID)
		index.minX, index.maxX = math.Min(index.minX, node.XM), math.Max(index.maxX, node.XM)
		index.minY, index.maxY = math.Min(index.minY, node.YM), math.Max(index.maxY, node.YM)
	}
	if len(validNodeIDs) == 0 || index.maxX <= index.minX || index.maxY <= index.minY {
		return nil, fmt.Errorf("модель дна не содержит пригодных глубин")
	}
	for _, cell := range model.Mesh.Cells {
		if cell.NodeCount != 4 {
			continue
		}
		for _, ids := range [][3]int{{cell.Nodes[0], cell.Nodes[1], cell.Nodes[2]}, {cell.Nodes[0], cell.Nodes[2], cell.Nodes[3]}} {
			triangle, ok := makeModelDepthTriangle(model.Nodes, ids)
			if ok {
				index.triangles = append(index.triangles, triangle)
			}
		}
	}
	if len(index.triangles) == 0 {
		return nil, fmt.Errorf("модель дна не содержит интерполируемых ячеек")
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
	index.nearest = buildModelDepthKDTree(model.Nodes, validNodeIDs, 0)
	return index, nil
}

// Sample возвращает интерполированную глубину или ближайший реальный узел в
// пределах maxNearestDistanceM. Ноль запрещает ближайшую замену.
func (index *ModelDepthSampler) Sample(x, y, maxNearestDistanceM float64) (ModelDepthSample, error) {
	if index == nil {
		return ModelDepthSample{}, fmt.Errorf("индекс модели дна не создан")
	}
	if !finite(x) || !finite(y) || !finite(maxNearestDistanceM) || maxNearestDistanceM < 0 {
		return ModelDepthSample{}, fmt.Errorf("координаты и радиус ближайшей замены должны быть конечными, радиус — неотрицательным")
	}
	column, row := index.binCoordinates(x, y)
	for _, triangleID := range index.bins[row*index.columns+column] {
		triangle := index.triangles[triangleID]
		if x < triangle.minX-1e-7 || x > triangle.maxX+1e-7 || y < triangle.minY-1e-7 || y > triangle.maxY+1e-7 {
			continue
		}
		if sample, ok := interpolateModelDepthTriangle(index.nodes, triangle, x, y); ok {
			return sample, nil
		}
	}
	if maxNearestDistanceM == 0 {
		return ModelDepthSample{}, fmt.Errorf("точка (%.3f, %.3f) находится вне интерполируемого покрытия модели", x, y)
	}
	nearestID := nearestModelDepthNode(index.nearest, index.nodes, x, y, 0, math.Inf(1))
	if nearestID == 0 || index.nodes[nearestID].WaterDepthM == nil {
		return ModelDepthSample{}, fmt.Errorf("для точки (%.3f, %.3f) не найдена глубина", x, y)
	}
	node := index.nodes[nearestID]
	distanceM := math.Hypot(node.XM-x, node.YM-y)
	if distanceM > maxNearestDistanceM {
		return ModelDepthSample{}, fmt.Errorf(
			"ближайшая глубина для точки (%.3f, %.3f) удалена на %.3f м при пределе %.3f м",
			x, y, distanceM, maxNearestDistanceM,
		)
	}
	return ModelDepthSample{
		WaterDepthM: *node.WaterDepthM, NearestFallback: true, SourceDistanceM: distanceM,
	}, nil
}

func makeModelDepthTriangle(nodes []Node, ids [3]int) (modelDepthTriangle, bool) {
	triangle := modelDepthTriangle{nodeIDs: ids, minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
	for _, nodeID := range ids {
		if nodeID <= 0 || nodeID >= len(nodes) || nodes[nodeID].WaterDepthM == nil || !finite(*nodes[nodeID].WaterDepthM) {
			return modelDepthTriangle{}, false
		}
		node := nodes[nodeID]
		triangle.minX, triangle.maxX = math.Min(triangle.minX, node.XM), math.Max(triangle.maxX, node.XM)
		triangle.minY, triangle.maxY = math.Min(triangle.minY, node.YM), math.Max(triangle.maxY, node.YM)
	}
	a, b, c := nodes[ids[0]], nodes[ids[1]], nodes[ids[2]]
	denominator := (b.XM-a.XM)*(c.YM-a.YM) - (c.XM-a.XM)*(b.YM-a.YM)
	if math.Abs(denominator) <= 1e-12 {
		return modelDepthTriangle{}, false
	}
	az, bz, cz := *a.WaterDepthM, *b.WaterDepthM, *c.WaterDepthM
	triangle.gradientX = ((bz-az)*(c.YM-a.YM) - (cz-az)*(b.YM-a.YM)) / denominator
	triangle.gradientY = ((b.XM-a.XM)*(cz-az) - (c.XM-a.XM)*(bz-az)) / denominator
	return triangle, true
}

func interpolateModelDepthTriangle(nodes []Node, triangle modelDepthTriangle, x, y float64) (ModelDepthSample, bool) {
	a, b, c := nodes[triangle.nodeIDs[0]], nodes[triangle.nodeIDs[1]], nodes[triangle.nodeIDs[2]]
	denominator := (b.YM-c.YM)*(a.XM-c.XM) + (c.XM-b.XM)*(a.YM-c.YM)
	if math.Abs(denominator) <= 1e-12 {
		return ModelDepthSample{}, false
	}
	wA := ((b.YM-c.YM)*(x-c.XM) + (c.XM-b.XM)*(y-c.YM)) / denominator
	wB := ((c.YM-a.YM)*(x-c.XM) + (a.XM-c.XM)*(y-c.YM)) / denominator
	wC := 1 - wA - wB
	if wA < -1e-9 || wB < -1e-9 || wC < -1e-9 {
		return ModelDepthSample{}, false
	}
	depthM := wA**a.WaterDepthM + wB**b.WaterDepthM + wC**c.WaterDepthM
	return ModelDepthSample{
		WaterDepthM: math.Max(0, depthM), GradientX: triangle.gradientX, GradientY: triangle.gradientY,
		GradientAvailable: true,
	}, true
}

func (index *ModelDepthSampler) binCoordinates(x, y float64) (int, int) {
	column := int(math.Floor((x - index.minX) / index.stepX))
	row := int(math.Floor((y - index.minY) / index.stepY))
	column = max(0, min(index.columns-1, column))
	row = max(0, min(index.rows-1, row))
	return column, row
}

type modelDepthKDNode struct {
	id, axis    int
	left, right *modelDepthKDNode
}

func buildModelDepthKDTree(nodes []Node, ids []int, depth int) *modelDepthKDNode {
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
	return &modelDepthKDNode{
		id: ids[middle], axis: axis,
		left: buildModelDepthKDTree(nodes, ids[:middle], depth+1), right: buildModelDepthKDTree(nodes, ids[middle+1:], depth+1),
	}
}

func nearestModelDepthNode(node *modelDepthKDNode, points []Node, x, y float64, bestID int, bestSquared float64) int {
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
	bestID = nearestModelDepthNode(near, points, x, y, bestID, bestSquared)
	if bestID != 0 {
		bestPoint := points[bestID]
		bestSquared = (bestPoint.XM-x)*(bestPoint.XM-x) + (bestPoint.YM-y)*(bestPoint.YM-y)
	}
	if delta*delta < bestSquared {
		bestID = nearestModelDepthNode(far, points, x, y, bestID, bestSquared)
	}
	return bestID
}
