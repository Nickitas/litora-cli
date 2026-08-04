package geometry

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
)

// ApproximationMesh определяет тип и параметры сетки для аппроксимации береговой линии.
type ApproximationMesh struct {
	Type            string  // "regular" | "tin" | "adaptive"
	Resolution      float64 // Разрешение для регулярной сетки (градусы)
	MaxTriangleArea float64 // Максимальная площадь треугольника для TIN (км²)
	ErrorTolerance  float64 // Допуск ошибки для адаптивной аппроксимации
}

// ApproximationOptions управляет процессом построения сетки.
type ApproximationOptions struct {
	MeshType        string  // "regular" | "tin" | "adaptive"
	Resolution      float64 // Разрешение сетки в градусах
	MaxTriangleArea float64 // Максимальная площадь треугольника в км² для TIN
	ErrorTolerance  float64 // Максимальная ошибка для адаптивной в метрах
	MinPoints       int     // Минимум точек для триангуляции
	RefineFactor    float64 // Коэффициент уточнения для адаптивной сетки
}

// Triangle представляет один треугольник в TIN-сети.
type Triangle struct {
	V0, V1, V2   int // Индексы вершин
	Area         float64
	Normal       [3]float64 // Вектор нормали (для 3D высоты)
	Circumcenter Point2D
	Circumradius float64
}

// Point2D представляет 2D-точку в спроецированных координатах.
type Point2D struct {
	X, Y float64
}

// TINMesh представляет триангулированную нерегулярную сеть (TIN).
type TINMesh struct {
	Vertices       []LatLon             // Исходные вершины lat/lon
	Projected      []Point2D            // Спроецированные координаты в метрах
	Triangles      []Triangle           // Треугольники Делоне
	Bounds         MeshBounds           // Границы сетки
	Options        ApproximationOptions // Параметры
	Stats          MeshStats            // Статистика
	edgeCountCache map[[2]int]int       // Кэшированное количество рёбер для O(1) поиска границ
}

// MeshBounds представляет пространственную протяжённость сетки.
type MeshBounds struct {
	MinLat, MaxLat float64
	MinLon, MaxLon float64
	MinX, MaxX     float64 // Спроецированные границы (метры)
	MinY, MaxY     float64 // Спроецированные границы (метры)
}

// MeshStats содержит статистику о генерируемой сетке.
type MeshStats struct {
	VertexCount     int
	TriangleCount   int
	AvgTriangleArea float64
	MaxTriangleArea float64
	MinTriangleArea float64
	EdgeCount       int
	HullVertexCount int
	RefinementSteps int
}

// DefaultApproximationOptions возвращает разумные значения по умолчанию для генерации сетки.
func DefaultApproximationOptions() ApproximationOptions {
	return ApproximationOptions{
		MeshType:        "tin",
		Resolution:      0.01,  // ~1.1 км
		MaxTriangleArea: 10.0,  // 10 км²
		ErrorTolerance:  100.0, // 100 метров
		MinPoints:       3,
		RefineFactor:    0.5,
	}
}

// BuildTINMesh создаёт TIN-сеть из набора точек с помощью триангуляции Делоне.
func BuildTINMesh(points []LatLon, opts ApproximationOptions) (*TINMesh, error) {
	if len(points) < opts.MinPoints {
		return nil, fmt.Errorf("требуется минимум %d точек для триангуляции, получено %d",
			opts.MinPoints, len(points))
	}

	// Клонируем точки, чтобы избежать модификации входных данных
	vertices := clonePoints(points)

	// Проецируем в локальную систему координат для точной геометрии
	projected, _ := projectToMetersWithRef(vertices)

	// Инициализируем сеть
	mesh := &TINMesh{
		Vertices:  vertices,
		Projected: projected,
		Options:   opts,
	}

	// Вычисляем границы
	mesh.calculateBounds(projected)

	// Строим триангуляцию Делоне
	if err := mesh.buildDelaunay(); err != nil {
		return nil, fmt.Errorf("не удалось выполнить триангуляцию Делоне: %w", err)
	}

	// Применяем уточнение сетки, если нужно
	if opts.MeshType == "adaptive" {
		mesh.refineAdaptive()
	}

	// Вычисляем статистику
	mesh.calculateStats()

	return mesh, nil
}

// buildDelaunay реализует алгоритм Бауэра-Ватсона для триангуляции Делоне.
func (m *TINMesh) buildDelaunay() error {
	// Создаём супер-треугольник, охватывающий все точки
	superTri, superPoints := m.createSuperTriangle()
	triangles := []Triangle{superTri}

	// Рабочая копия спроецированных точек с добавленными вершинами супер-треугольника
	workingPoints := make([]Point2D, len(superPoints)+len(m.Projected))
	copy(workingPoints, superPoints)
	copy(workingPoints[len(superPoints):], m.Projected)

	// Смещение для индексов вершин (вершины супер-треугольника идут первыми)
	offset := len(superPoints)

	// Инкрементально вставляем каждую точку
	for i := 0; i < len(m.Projected); i++ {
		triangles = m.insertPoint(i+offset, workingPoints[i+offset], triangles, workingPoints)
	}

	// Удаляем треугольники, имеющие общие вершины с супер-треугольником
	m.Triangles = m.removeSuperTriangle(triangles, offset)

	if len(m.Triangles) == 0 {
		return fmt.Errorf("триангуляция не дала валидных треугольников")
	}

	// Корректируем индексы вершин для удаления смещения супер-треугольника
	for i := range m.Triangles {
		m.Triangles[i].V0 -= offset
		m.Triangles[i].V1 -= offset
		m.Triangles[i].V2 -= offset
	}

	return nil
}

// createSuperTriangle создаёт большой треугольник, охватывающий все точки.
func (m *TINMesh) createSuperTriangle() (Triangle, []Point2D) {
	b := &m.Bounds

	// Создаём треугольник намного больше ограничивающей рамки
	scale := 10.0
	dx := (b.MaxX - b.MinX) * scale
	dy := (b.MaxY - b.MinY) * scale
	cx := (b.MinX + b.MaxX) / 2
	cy := (b.MinY + b.MaxY) / 2

	// Вершины супер-треугольника
	v0 := Point2D{cx - dx, cy - dy}
	v1 := Point2D{cx + dx, cy - dy}
	v2 := Point2D{cx, cy + dy*2}

	superPoints := []Point2D{v0, v1, v2}

	tri := Triangle{
		V0: 0,
		V1: 1,
		V2: 2,
	}
	tri.calculateCircumcircle(superPoints)
	tri.Area = triangleArea(v0, v1, v2)

	return tri, superPoints
}

// insertPoint добавляет одну точку в триангуляцию с помощью алгоритма Бауэра-Ватсона.
func (m *TINMesh) insertPoint(idx int, p Point2D, triangles []Triangle, points []Point2D) []Triangle {
	var badTriangles []Triangle

	// Находим треугольники, чья описанная окружность содержит точку
	for i, tri := range triangles {
		if m.pointInCircumcircle(p, tri, points) {
			badTriangles = append(badTriangles, triangles[i])
		}
	}

	// Находим границу полигонального отверстия
	polygon := m.findBoundary(badTriangles)

	// Удаляем плохие треугольники
	triangles = m.removeTriangles(triangles, badTriangles)

	// Ре-триангулируем отверстие
	seen := make(map[[3]int]bool)
	for _, edge := range polygon {
		triKey := orderedTriangle(edge[0], edge[1], idx)
		if seen[triKey] {
			continue
		}
		seen[triKey] = true

		newTri := Triangle{
			V0: edge[0],
			V1: edge[1],
			V2: idx,
		}
		newTri.calculateCircumcircle(points)
		newTri.Area = triangleArea(points[newTri.V0], points[newTri.V1], points[newTri.V2])
		triangles = append(triangles, newTri)
	}

	return triangles
}

// pointInCircumcircle проверяет, находится ли точка внутри описанной окружности треугольника.
func (m *TINMesh) pointInCircumcircle(p Point2D, tri Triangle, points []Point2D) bool {
	// Пересчитываем центр описанной окружности для этого треугольника
	v0 := points[tri.V0]
	v1 := points[tri.V1]
	v2 := points[tri.V2]

	// Сначала простая проверка по ограничивающей рамке
	minX := v0.X
	if v1.X < minX {
		minX = v1.X
	}
	if v2.X < minX {
		minX = v2.X
	}
	maxX := v0.X
	if v1.X > maxX {
		maxX = v1.X
	}
	if v2.X > maxX {
		maxX = v2.X
	}
	minY := v0.Y
	if v1.Y < minY {
		minY = v1.Y
	}
	if v2.Y < minY {
		minY = v2.Y
	}
	maxY := v0.Y
	if v1.Y > maxY {
		maxY = v1.Y
	}
	if v2.Y > maxY {
		maxY = v2.Y
	}

	if p.X < minX-1 || p.X > maxX+1 || p.Y < minY-1 || p.Y > maxY+1 {
		return false
	}

	dx := p.X - tri.Circumcenter.X
	dy := p.Y - tri.Circumcenter.Y
	distSq := dx*dx + dy*dy
	return distSq < tri.Circumradius*tri.Circumradius
}

// findBoundary находит граничные рёбра плохих треугольников (полигональное отверстие).
func (m *TINMesh) findBoundary(badTriangles []Triangle) [][]int {
	edgeCount := make(map[[2]int]int)

	// Подсчитываем количество появлений рёбер (хранятся с упорядоченными вершинами для согласованности)
	for _, tri := range badTriangles {
		edges := [][2]int{
			orderedEdge(tri.V0, tri.V1),
			orderedEdge(tri.V1, tri.V2),
			orderedEdge(tri.V2, tri.V0),
		}
		for _, e := range edges {
			edgeCount[e]++
		}
	}

	// Граничные рёбра появляются ровно один раз
	var polygon [][]int
	for edge, count := range edgeCount {
		if count == 1 {
			polygon = append(polygon, []int{edge[0], edge[1]})
		}
	}

	return polygon
}

// orderedEdge возвращает ребро с вершинами в согласованного порядке.
func orderedEdge(v0, v1 int) [2]int {
	if v0 < v1 {
		return [2]int{v0, v1}
	}
	return [2]int{v1, v0}
}

// orderedTriangle возвращает вершины треугольника в согласованного порядке.
// Упорядочивает v0, v1, v2 так, что v0 < v1 < v2 для согласованного хеширования.
func orderedTriangle(v0, v1, v2 int) [3]int {
	verts := [3]int{v0, v1, v2}
	if verts[0] > verts[1] {
		verts[0], verts[1] = verts[1], verts[0]
	}
	if verts[1] > verts[2] {
		verts[1], verts[2] = verts[2], verts[1]
	}
	if verts[0] > verts[1] {
		verts[0], verts[1] = verts[1], verts[0]
	}
	return verts
}

// removeTriangles удаляет треугольники из среза.
func (m *TINMesh) removeTriangles(triangles, toRemove []Triangle) []Triangle {
	removeSet := make(map[[3]int]bool, len(toRemove))
	for _, tri := range toRemove {
		key := orderedTriangle(tri.V0, tri.V1, tri.V2)
		removeSet[key] = true
	}

	result := make([]Triangle, 0, len(triangles)-len(toRemove))
	for _, tri := range triangles {
		key := orderedTriangle(tri.V0, tri.V1, tri.V2)
		if !removeSet[key] {
			result = append(result, tri)
		}
	}
	return result
}

// removeSuperTriangle удаляет треугольники, имеющие общие вершины с супер-треугольником.
func (m *TINMesh) removeSuperTriangle(triangles []Triangle, offset int) []Triangle {
	// Вершины супер-треугольника находятся по индексам 0, 1, 2 в рабочем массиве точек
	// Реальные точки имеют индексы >= offset
	superVerts := map[int]bool{0: true, 1: true, 2: true}

	var result []Triangle
	for _, tri := range triangles {
		// Проверяем, является ли любая вершина вершиной супер-треугольника (0, 1 или 2)
		if !superVerts[tri.V0] && !superVerts[tri.V1] && !superVerts[tri.V2] {
			result = append(result, tri)
		}
	}

	return result
}

// refineAdaptive уточняет сеть на основе допуска ошибки.
func (m *TINMesh) refineAdaptive() {
	maxArea := m.Options.MaxTriangleArea * 1e6 // Конвертируем км² в м²
	if maxArea <= 0 {
		return
	}

	iterations := 0
	maxIterations := 5 // Ограничиваем итерации для предотвращения взрыва

	for iterations < maxIterations {
		refined := false
		var newTriangles []Triangle

		for _, tri := range m.Triangles {
			if tri.Area > maxArea {
				// Разбиваем треугольник на более мелкие
				split := m.splitTriangle(tri)
				newTriangles = append(newTriangles, split...)
				refined = true
			} else {
				newTriangles = append(newTriangles, tri)
			}
		}

		if !refined {
			break
		}

		m.Triangles = newTriangles
		iterations++
	}

	m.Stats.RefinementSteps = iterations
}

// splitTriangle разбивает треугольник на 4 меньших треугольника.
func (m *TINMesh) splitTriangle(tri Triangle) []Triangle {
	// Вычисляем средние точки
	v0 := m.Projected[tri.V0]
	v1 := m.Projected[tri.V1]
	v2 := m.Projected[tri.V2]

	mid01 := Point2D{(v0.X + v1.X) / 2, (v0.Y + v1.Y) / 2}
	mid12 := Point2D{(v1.X + v2.X) / 2, (v1.Y + v2.Y) / 2}
	mid20 := Point2D{(v2.X + v0.X) / 2, (v2.Y + v0.Y) / 2}

	// Добавляем средние точки как новые вершины
	m.Projected = append(m.Projected, mid01, mid12, mid20)
	n := len(m.Projected)

	// Конвертируем обратно в lat/lon примерно
	refPoint, _ := m.getReferencePoint()
	m.Vertices = append(m.Vertices,
		m.projectToLatLon(mid01, refPoint),
		m.projectToLatLon(mid12, refPoint),
		m.projectToLatLon(mid20, refPoint),
	)

	// Создаём 4 новых треугольника
	newTris := []Triangle{
		{V0: tri.V0, V1: n - 3, V2: n - 1},
		{V0: n - 3, V1: tri.V1, V2: n - 2},
		{V0: n - 1, V1: n - 2, V2: tri.V2},
		{V0: n - 3, V1: n - 2, V2: n - 1},
	}

	for i := range newTris {
		newTris[i].calculateCircumcircle(m.Projected)
		newTris[i].Area = triangleArea(
			m.Projected[newTris[i].V0],
			m.Projected[newTris[i].V1],
			m.Projected[newTris[i].V2],
		)
	}

	return newTris
}

// calculateCircumcircle вычисляет центр и радиус описанной окружности треугольника.
func (t *Triangle) calculateCircumcircle(points []Point2D) {
	p0 := points[t.V0]
	p1 := points[t.V1]
	p2 := points[t.V2]

	ax, ay := p0.X, p0.Y
	bx, by := p1.X, p1.Y
	cx, cy := p2.X, p2.Y

	D := 2 * (ax*(by-cy) + bx*(cy-ay) + cx*(ay-by))
	if math.Abs(D) < 1e-12 {
		// Вырожденный треугольник - используем центроид
		t.Circumcenter = Point2D{(ax + bx + cx) / 3, (ay + by + cy) / 3}
		t.Circumradius = math.MaxFloat64
		return
	}

	ux := ((ax*ax+ay*ay)*(by-cy) + (bx*bx+by*by)*(cy-ay) + (cx*cx+cy*cy)*(ay-by)) / D
	uy := ((ax*ax+ay*ay)*(cx-bx) + (bx*bx+by*by)*(ax-cx) + (cx*cx+cy*cy)*(bx-ax)) / D

	t.Circumcenter = Point2D{ux, uy}
	t.Circumradius = math.Sqrt((ux-ax)*(ux-ax) + (uy-ay)*(uy-ay))
}

// triangleArea вычисляет площадь треугольника в спроецированных координатах.
func triangleArea(p0, p1, p2 Point2D) float64 {
	return math.Abs((p1.X-p0.X)*(p2.Y-p0.Y)-(p1.Y-p0.Y)*(p2.X-p0.X)) / 2
}

// calculateBounds вычисляет ограничивающую рамку сетки.
func (m *TINMesh) calculateBounds(projected []Point2D) {
	if len(projected) == 0 {
		return
	}

	m.Bounds.MinLat = m.Vertices[0].Lat
	m.Bounds.MaxLat = m.Vertices[0].Lat
	m.Bounds.MinLon = m.Vertices[0].Lon
	m.Bounds.MaxLon = m.Vertices[0].Lon

	m.Bounds.MinX = projected[0].X
	m.Bounds.MaxX = projected[0].X
	m.Bounds.MinY = projected[0].Y
	m.Bounds.MaxY = projected[0].Y

	for i := 1; i < len(projected); i++ {
		p := projected[i]
		v := m.Vertices[i]

		if p.X < m.Bounds.MinX {
			m.Bounds.MinX = p.X
		}
		if p.X > m.Bounds.MaxX {
			m.Bounds.MaxX = p.X
		}
		if p.Y < m.Bounds.MinY {
			m.Bounds.MinY = p.Y
		}
		if p.Y > m.Bounds.MaxY {
			m.Bounds.MaxY = p.Y
		}

		if v.Lat < m.Bounds.MinLat {
			m.Bounds.MinLat = v.Lat
		}
		if v.Lat > m.Bounds.MaxLat {
			m.Bounds.MaxLat = v.Lat
		}
		if v.Lon < m.Bounds.MinLon {
			m.Bounds.MinLon = v.Lon
		}
		if v.Lon > m.Bounds.MaxLon {
			m.Bounds.MaxLon = v.Lon
		}
	}
}

// calculateStats вычисляет статистику сети.
func (m *TINMesh) calculateStats() {
	m.Stats.VertexCount = len(m.Vertices)
	m.Stats.TriangleCount = len(m.Triangles)

	if len(m.Triangles) == 0 {
		return
	}

	minArea := math.MaxFloat64
	maxArea := 0.0
	totalArea := 0.0

	for _, tri := range m.Triangles {
		if tri.Area < minArea {
			minArea = tri.Area
		}
		if tri.Area > maxArea {
			maxArea = tri.Area
		}
		totalArea += tri.Area
	}

	m.Stats.MinTriangleArea = minArea
	m.Stats.MaxTriangleArea = maxArea
	m.Stats.AvgTriangleArea = totalArea / float64(len(m.Triangles))

	// Оцениваем количество рёбер (3 ребра на треугольник, разделяемые)
	m.Stats.EdgeCount = (len(m.Triangles)*3 + len(m.Vertices)) / 2

	// Количество вершин выпуклой оболочки (упрощённая оценка)
	m.Stats.HullVertexCount = m.calculateConvexHullCount()

	// Строим кэш количества рёбер для O(1) поиска граничных рёбер
	m.edgeCountCache = m.buildEdgeCountCache()
}

// buildEdgeCountCache строит кэш количества рёбер для O(1) поиска границ.
func (m *TINMesh) buildEdgeCountCache() map[[2]int]int {
	edgeCount := make(map[[2]int]int, len(m.Triangles)*3)
	for _, tri := range m.Triangles {
		edges := [][2]int{
			orderedEdge(tri.V0, tri.V1),
			orderedEdge(tri.V1, tri.V2),
			orderedEdge(tri.V2, tri.V0),
		}
		for _, e := range edges {
			edgeCount[e]++
		}
	}
	return edgeCount
}

// calculateConvexHullCount оценивает количество вершин выпуклой оболочки.
func (m *TINMesh) calculateConvexHullCount() int {
	// Используем граничные треугольники
	hullSet := make(map[int]bool)
	for _, tri := range m.Triangles {
		// Проверяем, является ли каждое ребро граничным
		edges := [][2]int{{tri.V0, tri.V1}, {tri.V1, tri.V2}, {tri.V2, tri.V0}}
		for _, edge := range edges {
			if m.isBoundaryEdge(edge[0], edge[1]) {
				hullSet[edge[0]] = true
				hullSet[edge[1]] = true
			}
		}
	}
	return len(hullSet)
}

// isBoundaryEdge проверяет, является ли ребро частью выпуклой оболочки.
func (m *TINMesh) isBoundaryEdge(v0, v1 int) bool {
	if m.edgeCountCache != nil {
		key := orderedEdge(v0, v1)
		return m.edgeCountCache[key] == 1
	}
	// Fallback на O(n) метод, если кэш не построен
	count := 0
	for _, tri := range m.Triangles {
		verts := []int{tri.V0, tri.V1, tri.V2}
		for i := 0; i < 3; i++ {
			if (verts[i] == v0 && verts[(i+1)%3] == v1) ||
				(verts[i] == v1 && verts[(i+1)%3] == v0) {
				count++
			}
		}
	}
	return count == 1
}

// projectToMetersWithRef проецирует lat/lon в метры с опорной точкой.
func projectToMetersWithRef(points []LatLon) ([]Point2D, LatLon) {
	if len(points) == 0 {
		return nil, LatLon{}
	}

	// Вычисляем центроид
	var latSum, lonSum float64
	for _, p := range points {
		latSum += p.Lat
		lonSum += p.Lon
	}
	ref := LatLon{latSum / float64(len(points)), lonSum / float64(len(points))}

	// Коэффициенты конверсии на опорной широте
	metersPerDegLat := 111194.9
	metersPerDegLon := metersPerDegLat * math.Cos(ref.Lat*math.Pi/180)
	if math.Abs(metersPerDegLon) < 1e-9 {
		metersPerDegLon = metersPerDegLat
	}

	projected := make([]Point2D, len(points))
	for i, p := range points {
		projected[i] = Point2D{
			X: (p.Lon - ref.Lon) * metersPerDegLon,
			Y: (p.Lat - ref.Lat) * metersPerDegLat,
		}
	}

	return projected, ref
}

// getReferencePoint возвращает опорную точку для проекции.
func (m *TINMesh) getReferencePoint() (LatLon, LatLon) {
	if len(m.Vertices) == 0 {
		return LatLon{}, LatLon{}
	}

	var latSum, lonSum float64
	for _, p := range m.Vertices {
		latSum += p.Lat
		lonSum += p.Lon
	}
	ref := LatLon{latSum / float64(len(m.Vertices)), lonSum / float64(len(m.Vertices))}

	// Приблизительное начало координат
	origin := LatLon{m.Bounds.MinLat, m.Bounds.MinLon}
	return ref, origin
}

// projectToLatLon конвертирует спроецированные координаты обратно в lat/lon.
func (m *TINMesh) projectToLatLon(p Point2D, ref LatLon) LatLon {
	metersPerDegLat := 111194.9
	metersPerDegLon := metersPerDegLat * math.Cos(ref.Lat*math.Pi/180)
	if math.Abs(metersPerDegLon) < 1e-9 {
		metersPerDegLon = metersPerDegLat
	}

	return LatLon{
		Lat: ref.Lat + p.Y/metersPerDegLat,
		Lon: ref.Lon + p.X/metersPerDegLon,
	}
}

// InterpolateValue интерполирует значение в заданной lat/lon с помощью барицентрических координат.
func (m *TINMesh) InterpolateValue(lat, lon float64, values []float64) (float64, error) {
	if len(values) != len(m.Vertices) {
		return 0, fmt.Errorf("количество значений %d не совпадает с количеством вершин %d",
			len(values), len(m.Vertices))
	}

	// Находим содержащий треугольник
	target := Point2D{}
	refPoint, _ := m.getReferencePoint()
	metersPerDegLat := 111194.9
	metersPerDegLon := metersPerDegLat * math.Cos(refPoint.Lat*math.Pi/180)

	target.X = (lon - refPoint.Lon) * metersPerDegLon
	target.Y = (lat - refPoint.Lat) * metersPerDegLat

	for _, tri := range m.Triangles {
		v0 := m.Projected[tri.V0]
		v1 := m.Projected[tri.V1]
		v2 := m.Projected[tri.V2]

		if m.pointInTriangle(target, v0, v1, v2) {
			// Барицентрическая интерполяция
			w := m.barycentricCoords(target, v0, v1, v2)
			result := w[0]*values[tri.V0] + w[1]*values[tri.V1] + w[2]*values[tri.V2]
			return result, nil
		}
	}

	return 0, fmt.Errorf("точка (%f, %f) находится за пределами границ сети", lat, lon)
}

// pointInTriangle проверяет, находится ли точка внутри треугольника с помощью барицентрических координат.
func (m *TINMesh) pointInTriangle(p, v0, v1, v2 Point2D) bool {
	w := m.barycentricCoords(p, v0, v1, v2)
	return w[0] >= 0 && w[1] >= 0 && w[2] >= 0
}

// barycentricCoords вычисляет барицентрические координаты для точки в треугольнике.
func (m *TINMesh) barycentricCoords(p, v0, v1, v2 Point2D) [3]float64 {
	v0v1 := Point2D{v1.X - v0.X, v1.Y - v0.Y}
	v0v2 := Point2D{v2.X - v0.X, v2.Y - v0.Y}
	v0p := Point2D{p.X - v0.X, p.Y - v0.Y}

	d00 := v0v1.X*v0v1.X + v0v1.Y*v0v1.Y
	d01 := v0v1.X*v0v2.X + v0v1.Y*v0v2.Y
	d11 := v0v2.X*v0v2.X + v0v2.Y*v0v2.Y
	d20 := v0p.X*v0v1.X + v0p.Y*v0v1.Y
	d21 := v0p.X*v0v2.X + v0p.Y*v0v2.Y

	denom := d00*d11 - d01*d01
	if math.Abs(denom) < 1e-12 {
		return [3]float64{1, 0, 0}
	}

	v := (d11*d20 - d01*d21) / denom
	w := (d00*d21 - d01*d20) / denom
	u := 1 - v - w

	return [3]float64{u, v, w}
}

// GetTriangles возвращает все треугольники в сети.
func (m *TINMesh) GetTriangles() []Triangle {
	return m.Triangles
}

// GetTriangleAreas возвращает площади всех треугольников в км².
func (m *TINMesh) GetTriangleAreas() []float64 {
	areas := make([]float64, len(m.Triangles))
	for i, tri := range m.Triangles {
		// Конвертируем из м² в км²
		areas[i] = tri.Area / 1e6
	}
	return areas
}

// GetMeshQuality возвращает метрики качества для сети.
func (m *TINMesh) GetMeshQuality() MeshQuality {
	var minAngle, maxAngle float64 = math.MaxFloat64, 0
	var angleSum float64

	for _, tri := range m.Triangles {
		angles := m.triangleAngles(tri)
		for _, a := range angles {
			if a < minAngle {
				minAngle = a
			}
			if a > maxAngle {
				maxAngle = a
			}
			angleSum += a
		}
	}

	avgAngle := angleSum / float64(len(m.Triangles)*3)

	return MeshQuality{
		MinAngle:      minAngle,
		MaxAngle:      maxAngle,
		AvgAngle:      avgAngle,
		TriangleCount: len(m.Triangles),
		VertexCount:   len(m.Vertices),
	}
}

// MeshQuality содержит метрики качества для TIN-сети.
type MeshQuality struct {
	MinAngle      float64
	MaxAngle      float64
	AvgAngle      float64
	TriangleCount int
	VertexCount   int
}

// triangleAngles вычисляет внутренние углы треугольника в градусах.
func (m *TINMesh) triangleAngles(tri Triangle) [3]float64 {
	p0 := m.Projected[tri.V0]
	p1 := m.Projected[tri.V1]
	p2 := m.Projected[tri.V2]

	// Вычисляем квадраты длин рёбер
	a2 := distSq(p1, p2) // напротив p0
	b2 := distSq(p0, p2) // напротив p1
	c2 := distSq(p0, p1) // напротив p2

	// Закон косинусов
	a := math.Sqrt(a2)
	b := math.Sqrt(b2)
	c := math.Sqrt(c2)

	angle0 := math.Acos((b2+c2-a2)/(2*b*c)) * 180 / math.Pi
	angle1 := math.Acos((a2+c2-b2)/(2*a*c)) * 180 / math.Pi
	angle2 := 180 - angle0 - angle1

	return [3]float64{angle0, angle1, angle2}
}

func distSq(a, b Point2D) float64 {
	dx := b.X - a.X
	dy := b.Y - a.Y
	return dx*dx + dy*dy
}

// Simplify упрощает TIN-сеть, удаляя вершины на основе порога площади.
func (m *TINMesh) Simplify(maxArea float64) error {
	if maxArea <= 0 {
		return nil
	}

	var newTriangles []Triangle
	for _, tri := range m.Triangles {
		if tri.Area <= maxArea {
			newTriangles = append(newTriangles, tri)
		}
	}

	if len(newTriangles) == 0 {
		return fmt.Errorf("упрощение удалит все треугольники")
	}

	m.Triangles = newTriangles
	m.calculateStats()
	return nil
}

// ExportGeoJSON экспортирует сеть в формате GeoJSON.
func (m *TINMesh) ExportGeoJSON() ([]byte, error) {
	// Это упрощённый экспорт GeoJSON
	// Полная реализация включала бы правильный формат FeatureCollection
	template := `{"type":"FeatureCollection","features":[`
	features := ``
	for _, tri := range m.Triangles {
		coords := fmt.Sprintf(`[[[%f,%f],[%f,%f],[%f,%f],[%f,%f]]]`,
			m.Vertices[tri.V0].Lon, m.Vertices[tri.V0].Lat,
			m.Vertices[tri.V1].Lon, m.Vertices[tri.V1].Lat,
			m.Vertices[tri.V2].Lon, m.Vertices[tri.V2].Lat,
			m.Vertices[tri.V0].Lon, m.Vertices[tri.V0].Lat)
		features += fmt.Sprintf(`{"type":"Feature","geometry":{"type":"Polygon","coordinates":%s},"properties":{"area":%f}},`, coords, tri.Area/1e6)
	}
	if len(features) > 0 {
		features = features[:len(features)-1] // Удаляем завершающую запятую
	}
	template += features + `]}`
	return []byte(template), nil
}

// ValidateMesh проверяет сеть на топологические ошибки.
func (m *TINMesh) ValidateMesh() []string {
	var errors []string

	if len(m.Triangles) == 0 {
		errors = append(errors, "сеть не содержит треугольников")
	}

	if len(m.Vertices) < 3 {
		errors = append(errors, "в сети меньше 3 вершин")
	}

	// Проверка на вырожденные треугольники
	degenerateCount := 0
	for _, tri := range m.Triangles {
		if tri.Area < 1e-6 {
			degenerateCount++
		}
	}
	if degenerateCount > 0 {
		errors = append(errors, fmt.Sprintf("найдено %d вырожденных треугольников", degenerateCount))
	}

	// Проверка валидности границ сети
	if m.Bounds.MaxLat < m.Bounds.MinLat || m.Bounds.MaxLon < m.Bounds.MinLon {
		errors = append(errors, "неверные границы сети")
	}

	return errors
}

// MeshDensity возвращает плотность треугольников на единицу площади.
func (m *TINMesh) MeshDensity() float64 {
	if len(m.Triangles) == 0 {
		return 0
	}

	meshArea := (m.Bounds.MaxX - m.Bounds.MinX) * (m.Bounds.MaxY - m.Bounds.MinY)
	if meshArea <= 0 {
		return 0
	}

	return float64(len(m.Triangles)) / meshArea
}

// insertPointParallel выполняет параллельную вставку точки с использованием конкурентных проверок описанных окружностей
func (m *TINMesh) insertPointParallel(ctx context.Context, idx int, p Point2D, triangles []Triangle, points []Point2D) []Triangle {
	type badTriangle struct {
		triangle Triangle
		index    int
	}

	badTriangles := make([]Triangle, 0, len(triangles)/4)
	var badMu sync.Mutex

	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}
	chunkSize := (len(triangles) + numWorkers - 1) / numWorkers

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > len(triangles) {
			end = len(triangles)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			localBad := make([]badTriangle, 0)

			for i := start; i < end; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				tri := triangles[i]
				if m.pointInCircumcircleOptimized(p, tri, points) {
					localBad = append(localBad, badTriangle{triangle: tri, index: i})
				}
			}

			badMu.Lock()
			for _, bt := range localBad {
				badTriangles = append(badTriangles, bt.triangle)
			}
			badMu.Unlock()
		}()
	}
	wg.Wait()

	if len(badTriangles) == 0 {
		return triangles
	}

	polygon := m.findBoundary(badTriangles)
	triangles = m.removeTriangles(triangles, badTriangles)

	seen := make(map[[3]int]bool)
	newTriangles := make([]Triangle, 0, len(polygon))

	for _, edge := range polygon {
		triKey := orderedTriangle(edge[0], edge[1], idx)
		if seen[triKey] {
			continue
		}
		seen[triKey] = true

		newTri := Triangle{
			V0: edge[0],
			V1: edge[1],
			V2: idx,
		}
		newTri.calculateCircumcircle(points)
		newTri.Area = triangleArea(points[newTri.V0], points[newTri.V1], points[newTri.V2])
		newTriangles = append(newTriangles, newTri)
	}

	triangles = append(triangles, newTriangles...)
	return triangles
}

// pointInCircumcircleOptimized использует оптимизации с ранним выходом для более быстрых проверок
func (m *TINMesh) pointInCircumcircleOptimized(p Point2D, tri Triangle, points []Point2D) bool {
	v0 := points[tri.V0]
	v1 := points[tri.V1]
	v2 := points[tri.V2]

	minX := v0.X
	if v1.X < minX {
		minX = v1.X
	}
	if v2.X < minX {
		minX = v2.X
	}
	maxX := v0.X
	if v1.X > maxX {
		maxX = v1.X
	}
	if v2.X > maxX {
		maxX = v2.X
	}
	minY := v0.Y
	if v1.Y < minY {
		minY = v1.Y
	}
	if v2.Y < minY {
		minY = v2.Y
	}
	maxY := v0.Y
	if v1.Y > maxY {
		maxY = v1.Y
	}
	if v2.Y > maxY {
		maxY = v2.Y
	}

	bboxPadding := tri.Circumradius + 1
	if p.X < minX-bboxPadding || p.X > maxX+bboxPadding ||
		p.Y < minY-bboxPadding || p.Y > maxY+bboxPadding {
		return false
	}

	dx := p.X - tri.Circumcenter.X
	dy := p.Y - tri.Circumcenter.Y
	distSq := dx*dx + dy*dy

	return distSq < tri.Circumradius*tri.Circumradius
}

// buildEdgeCountCacheParallel строит кэш количества рёбер с использованием параллельной обработки
func (m *TINMesh) buildEdgeCountCacheParallel(ctx context.Context) map[[2]int]int {
	if len(m.Triangles) == 0 {
		return make(map[[2]int]int)
	}

	edgeCount := make(map[[2]int]int, len(m.Triangles)*3)
	var mu sync.Mutex

	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}
	chunkSize := (len(m.Triangles) + numWorkers - 1) / numWorkers

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > len(m.Triangles) {
			end = len(m.Triangles)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			localCounts := make(map[[2]int]int)

			for i := start; i < end; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				tri := m.Triangles[i]
				edges := [][2]int{
					orderedEdge(tri.V0, tri.V1),
					orderedEdge(tri.V1, tri.V2),
					orderedEdge(tri.V2, tri.V0),
				}
				for _, e := range edges {
					localCounts[e]++
				}
			}

			mu.Lock()
			for edge, count := range localCounts {
				edgeCount[edge] += count
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	return edgeCount
}

// calculateBoundsParallel вычисляет границы сети параллельно
func (m *TINMesh) calculateBoundsParallel(ctx context.Context, projected []Point2D) {
	if len(projected) == 0 {
		return
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > 4 {
		numWorkers = 4
	}
	chunkSize := (len(projected) + numWorkers - 1) / numWorkers

	var boundsMu sync.Mutex
	var minLat, maxLat, minLon, maxLon, minX, maxX, minY, maxY float64

	minLat = m.Vertices[0].Lat
	maxLat = m.Vertices[0].Lat
	minLon = m.Vertices[0].Lon
	maxLon = m.Vertices[0].Lon
	minX = projected[0].X
	maxX = projected[0].X
	minY = projected[0].Y
	maxY = projected[0].Y

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > len(projected) {
			end = len(projected)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			localMinLat, localMaxLat, localMinLon, localMaxLon := minLat, maxLat, minLon, maxLon
			localMinX, localMaxX, localMinY, localMaxY := minX, maxX, minY, maxY

			for i := start; i < end && i < len(projected); i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				p := projected[i]
				v := m.Vertices[i]

				if p.X < localMinX {
					localMinX = p.X
				}
				if p.X > localMaxX {
					localMaxX = p.X
				}
				if p.Y < localMinY {
					localMinY = p.Y
				}
				if p.Y > localMaxY {
					localMaxY = p.Y
				}
				if v.Lat < localMinLat {
					localMinLat = v.Lat
				}
				if v.Lat > localMaxLat {
					localMaxLat = v.Lat
				}
				if v.Lon < localMinLon {
					localMinLon = v.Lon
				}
				if v.Lon > localMaxLon {
					localMaxLon = v.Lon
				}
			}

			boundsMu.Lock()
			if localMinX < minX {
				minX = localMinX
			}
			if localMaxX > maxX {
				maxX = localMaxX
			}
			if localMinY < minY {
				minY = localMinY
			}
			if localMaxY > maxY {
				maxY = localMaxY
			}
			if localMinLat < minLat {
				minLat = localMinLat
			}
			if localMaxLat > maxLat {
				maxLat = localMaxLat
			}
			if localMinLon < minLon {
				minLon = localMinLon
			}
			if localMaxLon > maxLon {
				maxLon = localMaxLon
			}
			boundsMu.Unlock()
		}()
	}
	wg.Wait()

	m.Bounds.MinLat = minLat
	m.Bounds.MaxLat = maxLat
	m.Bounds.MinLon = minLon
	m.Bounds.MaxLon = maxLon
	m.Bounds.MinX = minX
	m.Bounds.MaxX = maxX
	m.Bounds.MinY = minY
	m.Bounds.MaxY = maxY
}

// calculateStatsParallel вычисляет статистику сети параллельно
func (m *TINMesh) calculateStatsParallel(ctx context.Context) {
	m.Stats.VertexCount = len(m.Vertices)
	m.Stats.TriangleCount = len(m.Triangles)

	if len(m.Triangles) == 0 {
		return
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > 4 {
		numWorkers = 4
	}
	chunkSize := (len(m.Triangles) + numWorkers - 1) / numWorkers

	var statsMu sync.Mutex
	var minArea, maxArea, totalArea float64
	minArea = math.MaxFloat64
	maxArea = 0

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > len(m.Triangles) {
			end = len(m.Triangles)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			localMinArea := math.MaxFloat64
			localMaxArea := 0.0
			localTotal := 0.0

			for i := start; i < end && i < len(m.Triangles); i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				area := m.Triangles[i].Area
				localTotal += area

				if area < localMinArea {
					localMinArea = area
				}
				if area > localMaxArea {
					localMaxArea = area
				}
			}

			statsMu.Lock()
			if localMinArea < minArea {
				minArea = localMinArea
			}
			if localMaxArea > maxArea {
				maxArea = localMaxArea
			}
			totalArea += localTotal
			statsMu.Unlock()
		}()
	}
	wg.Wait()

	m.Stats.MinTriangleArea = minArea
	m.Stats.MaxTriangleArea = maxArea
	m.Stats.AvgTriangleArea = totalArea / float64(len(m.Triangles))

	m.Stats.EdgeCount = (len(m.Triangles)*3 + len(m.Vertices)) / 2
	m.Stats.HullVertexCount = m.calculateConvexHullCount()
	m.edgeCountCache = m.buildEdgeCountCacheParallel(ctx)
}

// BuildTINMeshParallel создаёт TIN-сеть с использованием параллельной триангуляции Делоне
func BuildTINMeshParallel(ctx context.Context, points []LatLon, opts ApproximationOptions) (*TINMesh, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if len(points) < opts.MinPoints {
		return nil, fmt.Errorf("требуется минимум %d точек для триангуляции, получено %d",
			opts.MinPoints, len(points))
	}

	vertices := clonePoints(points)
	projected, _ := projectToMetersWithRef(vertices)

	mesh := &TINMesh{
		Vertices:  vertices,
		Projected: projected,
		Options:   opts,
	}

	mesh.calculateBoundsParallel(ctx, projected)

	if err := mesh.buildDelaunayParallel(ctx); err != nil {
		return nil, fmt.Errorf("не удалось выполнить триангуляцию Делоне: %w", err)
	}

	if opts.MeshType == "adaptive" {
		mesh.refineAdaptiveParallel(ctx)
	}

	mesh.calculateStatsParallel(ctx)

	return mesh, nil
}

// buildDelaunayParallel реализует параллельный алгоритм Бауэра-Ватсона
func (m *TINMesh) buildDelaunayParallel(ctx context.Context) error {
	superTri, superPoints := m.createSuperTriangle()
	triangles := []Triangle{superTri}

	workingPoints := make([]Point2D, len(superPoints)+len(m.Projected))
	copy(workingPoints, superPoints)
	copy(workingPoints[len(superPoints):], m.Projected)

	offset := len(superPoints)

	for i := 0; i < len(m.Projected); i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		triangles = m.insertPointParallel(ctx, i+offset, workingPoints[i+offset], triangles, workingPoints)
	}

	m.Triangles = m.removeSuperTriangle(triangles, offset)

	if len(m.Triangles) == 0 {
		return fmt.Errorf("триангуляция не дала валидных треугольников")
	}

	for i := range m.Triangles {
		m.Triangles[i].V0 -= offset
		m.Triangles[i].V1 -= offset
		m.Triangles[i].V2 -= offset
	}

	return nil
}

// refineAdaptiveParallel выполняет уточнение сети параллельно
func (m *TINMesh) refineAdaptiveParallel(ctx context.Context) {
	maxArea := m.Options.MaxTriangleArea * 1e6
	if maxArea <= 0 {
		return
	}

	iterations := 0
	maxIterations := 5

	for iterations < maxIterations {
		select {
		case <-ctx.Done():
			return
		default:
		}

		refined := false
		var newTriangles []Triangle
		var mu sync.Mutex

		numWorkers := runtime.NumCPU()
		if numWorkers > 4 {
			numWorkers = 4
		}
		chunkSize := (len(m.Triangles) + numWorkers - 1) / numWorkers

		var wg sync.WaitGroup
		for w := 0; w < numWorkers; w++ {
			start := w * chunkSize
			end := start + chunkSize
			if end > len(m.Triangles) {
				end = len(m.Triangles)
			}

			wg.Add(1)
			go func() {
				defer wg.Done()

				localTriangles := make([]Triangle, 0)
				localRefined := false

				for i := start; i < end; i++ {
					select {
					case <-ctx.Done():
						return
					default:
					}

					tri := m.Triangles[i]
					if tri.Area > maxArea {
						split := m.splitTriangle(tri)
						localTriangles = append(localTriangles, split...)
						localRefined = true
					} else {
						localTriangles = append(localTriangles, tri)
					}
				}

				mu.Lock()
				if localRefined {
					refined = true
				}
				newTriangles = append(newTriangles, localTriangles...)
				mu.Unlock()
			}()
		}
		wg.Wait()

		if !refined {
			break
		}

		m.Triangles = newTriangles
		iterations++
	}

	m.Stats.RefinementSteps = iterations
}
