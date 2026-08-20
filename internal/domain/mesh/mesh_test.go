package mesh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"coastal-geometry/internal/domain/geometry"
)

func TestPrepareDomainAccountsForDroppedIsland(t *testing.T) {
	outer := []geometry.LatLon{
		{Lat: 43.0, Lon: 34.0}, {Lat: 43.0, Lon: 34.1},
		{Lat: 43.1, Lon: 34.1}, {Lat: 43.1, Lon: 34.0}, {Lat: 43.0, Lon: 34.0},
	}
	hole := []geometry.LatLon{
		{Lat: 43.0499, Lon: 34.0499}, {Lat: 43.0499, Lon: 34.0501},
		{Lat: 43.0501, Lon: 34.0501}, {Lat: 43.0501, Lon: 34.0499}, {Lat: 43.0499, Lon: 34.0499},
	}

	domain, err := PrepareDomain(outer, [][]geometry.LatLon{hole}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(domain.OriginalRings) != 2 {
		t.Fatalf("ожидались внешнее и островное кольца, получено %d", len(domain.OriginalRings))
	}
	if len(domain.SimplifiedRings) != 1 {
		t.Fatalf("малый остров должен быть исключён на масштабе 100 м, колец %d", len(domain.SimplifiedRings))
	}
	if domain.CumulativeFeatureDeviationM2 <= 0 {
		t.Fatal("площадь исключённого острова должна входить в отклонение")
	}
}

func TestValidDomainTopologyRejectsCrossingRing(t *testing.T) {
	bowTie := []Point{{X: 0, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}, {X: 10, Y: 0}, {X: 0, Y: 0}}
	if validDomainTopology([][]Point{bowTie}) {
		t.Fatal("самопересекающееся кольцо не должно передаваться генератору сетки")
	}
	square := []Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}, {X: 0, Y: 0}}
	if !validDomainTopology([][]Point{square}) {
		t.Fatal("простое замкнутое кольцо должно считаться корректным")
	}
}

func TestSubdivideToFullQuads(t *testing.T) {
	source := Mesh{
		Nodes:         []Point{{}, {X: 0, Y: 0}, {X: 2, Y: 0}, {X: 0, Y: 2}},
		Cells:         []Cell{{Nodes: [4]int{1, 2, 3}, NodeCount: 3}},
		BoundaryEdges: [][2]int{{1, 2}, {2, 3}, {3, 1}},
		TriangleCount: 1,
	}

	result := SubdivideToFullQuads(source)
	if result.TriangleCount != 0 || result.QuadCount != 3 || len(result.Cells) != 3 {
		t.Fatalf("треугольник должен превратиться в три четырёхугольника: %+v", result)
	}
	if len(result.BoundaryEdges) != 6 {
		t.Fatalf("каждое граничное ребро должно разделиться надвое, получено %d", len(result.BoundaryEdges))
	}
	for _, cell := range result.Cells {
		if cell.NodeCount != 4 {
			t.Fatalf("получена нечетырёхугольная ячейка: %+v", cell)
		}
	}
}

func TestBuildGeoContainsIndependentAlgorithmAndHole(t *testing.T) {
	domain := PreparedDomain{SimplifiedRings: [][]Point{
		{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}, {X: 0, Y: 0}},
		{{X: 4, Y: 4}, {X: 4, Y: 6}, {X: 6, Y: 6}, {X: 6, Y: 4}, {X: 4, Y: 4}},
	}}
	data, err := buildGeo(domain, AlgorithmFrontalQuad, 2)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, expected := range []string{"Plane Surface(1) = {1, 2};", "Mesh.Algorithm = 8;", "Mesh.RecombineAll = 0;"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("GEO не содержит %q\n%s", expected, text)
		}
	}
}

func TestReadMSH2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.msh")
	content := `$MeshFormat
2.2 0 8
$EndMeshFormat
$Nodes
4
1 0 0 0
2 1 0 0
3 1 1 0
4 0 1 0
$EndNodes
$Elements
3
1 1 0 1 2
2 2 0 1 2 3
3 3 0 1 2 3 4
$EndElements
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ReadMSH2(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.BoundaryEdges) != 1 || result.TriangleCount != 1 || result.QuadCount != 1 {
		t.Fatalf("неверно прочитаны элементы: %+v", result)
	}
}

func TestWriteMSH2PreservesFullQuadMesh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "full-quad.msh")
	source := Mesh{
		Nodes:         []Point{{}, {X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}},
		Cells:         []Cell{{Nodes: [4]int{1, 2, 3, 4}, NodeCount: 4}},
		BoundaryEdges: [][2]int{{1, 2}, {2, 3}, {3, 4}, {4, 1}},
		QuadCount:     1,
	}
	if err := WriteMSH2(path, source); err != nil {
		t.Fatal(err)
	}
	result, err := ReadMSH2(path)
	if err != nil {
		t.Fatal(err)
	}
	if result.TriangleCount != 0 || result.QuadCount != 1 || len(result.BoundaryEdges) != 4 {
		t.Fatalf("итоговая сетка повреждена при записи: %+v", result)
	}
}
