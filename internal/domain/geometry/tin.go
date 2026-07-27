package geometry

import (
	"fmt"
	"math"
)

// ApproximationMesh defines the mesh type and parameters for coastline approximation.
type ApproximationMesh struct {
	Type            string  // "regular" | "tin" | "adaptive"
	Resolution      float64 // Resolution for regular grid (degrees)
	MaxTriangleArea float64 // Maximum triangle area for TIN (km²)
	ErrorTolerance  float64 // Error tolerance for adaptive approximation
}

// ApproximationOptions controls the mesh building process.
type ApproximationOptions struct {
	MeshType        string  // "regular" | "tin" | "adaptive"
	Resolution      float64 // Grid resolution in degrees
	MaxTriangleArea float64 // Max triangle area in km² for TIN
	ErrorTolerance  float64 // Max error for adaptive in meters
	MinPoints       int     // Minimum points for triangulation
	RefineFactor    float64 // Refinement factor for adaptive mesh
}

// Triangle represents a single triangle in the TIN mesh.
type Triangle struct {
	V0, V1, V2   int // Vertex indices
	Area         float64
	Normal       [3]float64 // Normal vector (for 3D elevation)
	Circumcenter Point2D
	Circumradius float64
}

// Point2D represents a 2D point in projected coordinates.
type Point2D struct {
	X, Y float64
}

// TINMesh represents a Triangulated Irregular Network mesh.
type TINMesh struct {
	Vertices       []LatLon             // Original lat/lon vertices
	Projected      []Point2D            // Projected coordinates in meters
	Triangles      []Triangle           // Delaunay triangles
	Bounds         MeshBounds           // Mesh bounding box
	Options        ApproximationOptions // Options
	Stats          MeshStats            // Stats
	edgeCountCache map[[2]int]int       // Cached edge counts for O(1) boundary lookup
}

// MeshBounds represents the spatial extent of the mesh.
type MeshBounds struct {
	MinLat, MaxLat float64
	MinLon, MaxLon float64
	MinX, MaxX     float64 // Projected bounds (meters)
	MinY, MaxY     float64 // Projected bounds (meters)
}

// MeshStats contains statistics about the generated mesh.
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

// DefaultApproximationOptions returns sensible defaults for mesh generation.
func DefaultApproximationOptions() ApproximationOptions {
	return ApproximationOptions{
		MeshType:        "tin",
		Resolution:      0.01,  // ~1.1 km
		MaxTriangleArea: 10.0,  // 10 km²
		ErrorTolerance:  100.0, // 100 meters
		MinPoints:       3,
		RefineFactor:    0.5,
	}
}

// BuildTINMesh creates a TIN mesh from a set of points using Delaunay triangulation.
func BuildTINMesh(points []LatLon, opts ApproximationOptions) (*TINMesh, error) {
	if len(points) < opts.MinPoints {
		return nil, fmt.Errorf("need at least %d points for triangulation, got %d",
			opts.MinPoints, len(points))
	}

	// Clone points to avoid modifying input
	vertices := clonePoints(points)

	// Project to local coordinate system for accurate geometry
	projected, _ := projectToMetersWithRef(vertices)

	// Initialize mesh
	mesh := &TINMesh{
		Vertices:  vertices,
		Projected: projected,
		Options:   opts,
	}

	// Calculate bounds
	mesh.calculateBounds(projected)

	// Build Delaunay triangulation
	if err := mesh.buildDelaunay(); err != nil {
		return nil, fmt.Errorf("delaunay triangulation failed: %w", err)
	}

	// Apply mesh refinement if needed
	if opts.MeshType == "adaptive" {
		mesh.refineAdaptive()
	}

	// Calculate statistics
	mesh.calculateStats()

	return mesh, nil
}

// buildDelaunay implements Bowyer-Watson algorithm for Delaunay triangulation.
func (m *TINMesh) buildDelaunay() error {
	// Create super-triangle that encompasses all points
	superTri, superPoints := m.createSuperTriangle()
	triangles := []Triangle{superTri}

	// Working copy of projected points with super-triangle vertices prepended
	workingPoints := make([]Point2D, len(superPoints)+len(m.Projected))
	copy(workingPoints, superPoints)
	copy(workingPoints[len(superPoints):], m.Projected)

	// Offset for vertex indices (super-triangle vertices come first)
	offset := len(superPoints)

	// Incrementally insert each point
	for i := 0; i < len(m.Projected); i++ {
		triangles = m.insertPoint(i+offset, workingPoints[i+offset], triangles, workingPoints)
	}

	// Remove triangles sharing vertices with super-triangle
	m.Triangles = m.removeSuperTriangle(triangles, offset)

	if len(m.Triangles) == 0 {
		return fmt.Errorf("triangulation produced no valid triangles")
	}

	// Adjust vertex indices to remove super-triangle offset
	for i := range m.Triangles {
		m.Triangles[i].V0 -= offset
		m.Triangles[i].V1 -= offset
		m.Triangles[i].V2 -= offset
	}

	return nil
}

// createSuperTriangle creates a large triangle encompassing all points.
func (m *TINMesh) createSuperTriangle() (Triangle, []Point2D) {
	b := &m.Bounds

	// Create a triangle much larger than the bounding box
	scale := 10.0
	dx := (b.MaxX - b.MinX) * scale
	dy := (b.MaxY - b.MinY) * scale
	cx := (b.MinX + b.MaxX) / 2
	cy := (b.MinY + b.MaxY) / 2

	// Super-triangle vertices
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

// insertPoint adds a single point to the triangulation using Bowyer-Watson.
func (m *TINMesh) insertPoint(idx int, p Point2D, triangles []Triangle, points []Point2D) []Triangle {
	var badTriangles []Triangle

	// Find triangles whose circumcircle contains the point
	for i, tri := range triangles {
		if m.pointInCircumcircle(p, tri, points) {
			badTriangles = append(badTriangles, triangles[i])
		}
	}

	// Find boundary of polygonal hole
	polygon := m.findBoundary(badTriangles)

	// Remove bad triangles
	triangles = m.removeTriangles(triangles, badTriangles)

	// Re-triangulate the hole
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

// pointInCircumcircle checks if a point is inside a triangle's circumcircle.
func (m *TINMesh) pointInCircumcircle(p Point2D, tri Triangle, points []Point2D) bool {
	// Recalculate circumcenter for this triangle
	v0 := points[tri.V0]
	v1 := points[tri.V1]
	v2 := points[tri.V2]

	// Simple bounding box check first
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

// findBoundary finds the boundary edges of bad triangles (polygon hole).
func (m *TINMesh) findBoundary(badTriangles []Triangle) [][]int {
	edgeCount := make(map[[2]int]int)

	// Count edge occurrences (store edges with sorted vertices for consistency)
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

	// Boundary edges appear exactly once
	var polygon [][]int
	for edge, count := range edgeCount {
		if count == 1 {
			polygon = append(polygon, []int{edge[0], edge[1]})
		}
	}

	return polygon
}

// orderedEdge returns an edge with vertices in consistent order.
func orderedEdge(v0, v1 int) [2]int {
	if v0 < v1 {
		return [2]int{v0, v1}
	}
	return [2]int{v1, v0}
}

// orderedTriangle returns triangle vertices in consistent order.
// Orders v0, v1, v2 such that v0 < v1 < v2 for consistent hashing.
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

// removeTriangles removes triangles from the slice.
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

// removeSuperTriangle removes triangles that share vertices with the super-triangle.
func (m *TINMesh) removeSuperTriangle(triangles []Triangle, offset int) []Triangle {
	// Super-triangle vertices are at indices 0, 1, 2 in the working points array
	// Real points have indices >= offset
	superVerts := map[int]bool{0: true, 1: true, 2: true}

	var result []Triangle
	for _, tri := range triangles {
		// Check if any vertex is a super-triangle vertex (0, 1, or 2)
		if !superVerts[tri.V0] && !superVerts[tri.V1] && !superVerts[tri.V2] {
			result = append(result, tri)
		}
	}

	return result
}

// refineAdaptive refines the mesh based on error tolerance.
func (m *TINMesh) refineAdaptive() {
	maxArea := m.Options.MaxTriangleArea * 1e6 // Convert km² to m²
	if maxArea <= 0 {
		return
	}

	iterations := 0
	maxIterations := 5 // Limit iterations to prevent explosion

	for iterations < maxIterations {
		refined := false
		var newTriangles []Triangle

		for _, tri := range m.Triangles {
			if tri.Area > maxArea {
				// Split triangle into smaller ones
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

// splitTriangle splits a triangle into 4 smaller triangles.
func (m *TINMesh) splitTriangle(tri Triangle) []Triangle {
	// Calculate midpoints
	v0 := m.Projected[tri.V0]
	v1 := m.Projected[tri.V1]
	v2 := m.Projected[tri.V2]

	mid01 := Point2D{(v0.X + v1.X) / 2, (v0.Y + v1.Y) / 2}
	mid12 := Point2D{(v1.X + v2.X) / 2, (v1.Y + v2.Y) / 2}
	mid20 := Point2D{(v2.X + v0.X) / 2, (v2.Y + v0.Y) / 2}

	// Add midpoints as new vertices
	m.Projected = append(m.Projected, mid01, mid12, mid20)
	n := len(m.Projected)

	// Convert back to lat/lon approximately
	refPoint, _ := m.getReferencePoint()
	m.Vertices = append(m.Vertices,
		m.projectToLatLon(mid01, refPoint),
		m.projectToLatLon(mid12, refPoint),
		m.projectToLatLon(mid20, refPoint),
	)

	// Create 4 new triangles
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

// calculateCircumcircle computes the circumcenter and circumradius of a triangle.
func (t *Triangle) calculateCircumcircle(points []Point2D) {
	p0 := points[t.V0]
	p1 := points[t.V1]
	p2 := points[t.V2]

	ax, ay := p0.X, p0.Y
	bx, by := p1.X, p1.Y
	cx, cy := p2.X, p2.Y

	D := 2 * (ax*(by-cy) + bx*(cy-ay) + cx*(ay-by))
	if math.Abs(D) < 1e-12 {
		// Degenerate triangle - use centroid
		t.Circumcenter = Point2D{(ax + bx + cx) / 3, (ay + by + cy) / 3}
		t.Circumradius = math.MaxFloat64
		return
	}

	ux := ((ax*ax+ay*ay)*(by-cy) + (bx*bx+by*by)*(cy-ay) + (cx*cx+cy*cy)*(ay-by)) / D
	uy := ((ax*ax+ay*ay)*(cx-bx) + (bx*bx+by*by)*(ax-cx) + (cx*cx+cy*cy)*(bx-ax)) / D

	t.Circumcenter = Point2D{ux, uy}
	t.Circumradius = math.Sqrt((ux-ax)*(ux-ax) + (uy-ay)*(uy-ay))
}

// triangleArea computes the area of a triangle in projected coordinates.
func triangleArea(p0, p1, p2 Point2D) float64 {
	return math.Abs((p1.X-p0.X)*(p2.Y-p0.Y)-(p1.Y-p0.Y)*(p2.X-p0.X)) / 2
}

// calculateBounds computes the mesh bounding box.
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

// calculateStats computes mesh statistics.
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

	// Estimate edge count (3 edges per triangle, shared)
	m.Stats.EdgeCount = (len(m.Triangles)*3 + len(m.Vertices)) / 2

	// Convex hull vertex count (simplified estimate)
	m.Stats.HullVertexCount = m.calculateConvexHullCount()

	// Build edge count cache for O(1) boundary edge lookups
	m.edgeCountCache = m.buildEdgeCountCache()
}

// buildEdgeCountCache builds a cache of edge counts for O(1) boundary lookup.
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

// calculateConvexHullCount estimates the convex hull vertex count.
func (m *TINMesh) calculateConvexHullCount() int {
	// Use boundary triangles
	hullSet := make(map[int]bool)
	for _, tri := range m.Triangles {
		// Check if each edge is a boundary edge
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

// isBoundaryEdge checks if an edge is on the convex hull.
func (m *TINMesh) isBoundaryEdge(v0, v1 int) bool {
	if m.edgeCountCache != nil {
		key := orderedEdge(v0, v1)
		return m.edgeCountCache[key] == 1
	}
	// Fallback to O(n) method if cache not built
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

// projectToMetersWithRef projects lat/lon to meters with reference point.
func projectToMetersWithRef(points []LatLon) ([]Point2D, LatLon) {
	if len(points) == 0 {
		return nil, LatLon{}
	}

	// Calculate centroid
	var latSum, lonSum float64
	for _, p := range points {
		latSum += p.Lat
		lonSum += p.Lon
	}
	ref := LatLon{latSum / float64(len(points)), lonSum / float64(len(points))}

	// Conversion factors at reference latitude
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

// getReferencePoint returns the reference point for projection.
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

	// Approximate origin
	origin := LatLon{m.Bounds.MinLat, m.Bounds.MinLon}
	return ref, origin
}

// projectToLatLon converts projected coordinates back to lat/lon.
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

// InterpolateValue interpolates a value at a given lat/lon using barycentric coordinates.
func (m *TINMesh) InterpolateValue(lat, lon float64, values []float64) (float64, error) {
	if len(values) != len(m.Vertices) {
		return 0, fmt.Errorf("values count %d doesn't match vertices count %d",
			len(values), len(m.Vertices))
	}

	// Find containing triangle
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
			// Barycentric interpolation
			w := m.barycentricCoords(target, v0, v1, v2)
			result := w[0]*values[tri.V0] + w[1]*values[tri.V1] + w[2]*values[tri.V2]
			return result, nil
		}
	}

	return 0, fmt.Errorf("point (%f, %f) is outside the mesh bounds", lat, lon)
}

// pointInTriangle checks if a point is inside a triangle using barycentric coordinates.
func (m *TINMesh) pointInTriangle(p, v0, v1, v2 Point2D) bool {
	w := m.barycentricCoords(p, v0, v1, v2)
	return w[0] >= 0 && w[1] >= 0 && w[2] >= 0
}

// barycentricCoords computes barycentric coordinates for a point in a triangle.
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

// GetTriangles returns all triangles in the mesh.
func (m *TINMesh) GetTriangles() []Triangle {
	return m.Triangles
}

// GetTriangleAreas returns the areas of all triangles in km².
func (m *TINMesh) GetTriangleAreas() []float64 {
	areas := make([]float64, len(m.Triangles))
	for i, tri := range m.Triangles {
		// Convert from m² to km²
		areas[i] = tri.Area / 1e6
	}
	return areas
}

// GetMeshQuality returns quality metrics for the mesh.
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

// MeshQuality contains quality metrics for the TIN mesh.
type MeshQuality struct {
	MinAngle      float64
	MaxAngle      float64
	AvgAngle      float64
	TriangleCount int
	VertexCount   int
}

// triangleAngles computes the internal angles of a triangle in degrees.
func (m *TINMesh) triangleAngles(tri Triangle) [3]float64 {
	p0 := m.Projected[tri.V0]
	p1 := m.Projected[tri.V1]
	p2 := m.Projected[tri.V2]

	// Compute squared edge lengths
	a2 := distSq(p1, p2) // opposite p0
	b2 := distSq(p0, p2) // opposite p1
	c2 := distSq(p0, p1) // opposite p2

	// Law of cosines
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

// Simplify simplifies the TIN mesh by removing vertices based on area threshold.
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
		return fmt.Errorf("simplification would remove all triangles")
	}

	m.Triangles = newTriangles
	m.calculateStats()
	return nil
}

// ExportGeoJSON exports the mesh as GeoJSON.
func (m *TINMesh) ExportGeoJSON() ([]byte, error) {
	// This is a simplified GeoJSON export
	// Full implementation would include proper FeatureCollection format
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
		features = features[:len(features)-1] // Remove trailing comma
	}
	template += features + `]}`
	return []byte(template), nil
}

// ValidateMesh checks the mesh for topological errors.
func (m *TINMesh) ValidateMesh() []string {
	var errors []string

	if len(m.Triangles) == 0 {
		errors = append(errors, "mesh contains no triangles")
	}

	if len(m.Vertices) < 3 {
		errors = append(errors, "mesh has fewer than 3 vertices")
	}

	// Check for degenerate triangles
	degenerateCount := 0
	for _, tri := range m.Triangles {
		if tri.Area < 1e-6 {
			degenerateCount++
		}
	}
	if degenerateCount > 0 {
		errors = append(errors, fmt.Sprintf("found %d degenerate triangles", degenerateCount))
	}

	// Check mesh bounds validity
	if m.Bounds.MaxLat < m.Bounds.MinLat || m.Bounds.MaxLon < m.Bounds.MinLon {
		errors = append(errors, "invalid mesh bounds")
	}

	return errors
}

// MeshDensity returns the triangle density per unit area.
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
