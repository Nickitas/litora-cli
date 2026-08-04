package geometry

import (
	"math"
	"testing"
)

func TestLoadBathymetryFromJSON_ValidInput_Success(t *testing.T) {
	data := []byte(`[
		{"lat": 45.0, "lon": 30.0, "depth": -100},
		{"lat": 45.0, "lon": 30.01, "depth": -150},
		{"lat": 45.01, "lon": 30.0, "depth": -120},
		{"lat": 45.01, "lon": 30.01, "depth": -180}
	]`)

	grid, err := LoadBathymetryFromJSON(data, BathymetryLoadOptions{Resolution: 0.01})
	if err != nil {
		t.Fatalf("ожидался успех, получена ошибка: %v", err)
	}

	if grid == nil {
		t.Fatal("ожидалась сетка, получен nil")
	}

	if grid.bounds.MinLat != 45.0 || grid.bounds.MaxLat != 45.01 {
		t.Fatalf("ожидались границы широты [45.0, 45.01], получено [%f, %f]", grid.bounds.MinLat, grid.bounds.MaxLat)
	}

	if grid.bounds.MinLon != 30.0 || grid.bounds.MaxLon != 30.01 {
		t.Fatalf("ожидались границы долготы [30.0, 30.01], получено [%f, %f]", grid.bounds.MinLon, grid.bounds.MaxLon)
	}
}

func TestLoadBathymetryFromJSON_EmptyArray_Error(t *testing.T) {
	data := []byte(`[]`)

	_, err := LoadBathymetryFromJSON(data, BathymetryLoadOptions{})
	if err == nil {
		t.Fatal("ожидалась ошибка для пустого массива, получен nil")
	}

	if !containsString(err.Error(), "пусты") {
		t.Fatalf("ожидалось слово 'пусты' в сообщении об ошибке, получено: %v", err)
	}
}

func TestLoadBathymetryFromJSON_InvalidCoordinates_Error(t *testing.T) {
	data := []byte(`[
		{"lat": 45.0, "lon": 30.0, "depth": -100},
		{"lat": 95.0, "lon": 30.0, "depth": -50}
	]`)

	_, err := LoadBathymetryFromJSON(data, BathymetryLoadOptions{})
	if err == nil {
		t.Fatal("ожидалась ошибка для недопустимой широты, получен nil")
	}

	if !containsString(err.Error(), "широт") {
		t.Fatalf("ожидалось слово 'широт' в сообщении об ошибке, получено: %v", err)
	}
}

func TestBuildGrid_ValidPoints_Success(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
		{Lat: 45.0, Lon: 30.01, Depth: -150},
		{Lat: 45.01, Lon: 30.0, Depth: -120},
		{Lat: 45.01, Lon: 30.01, Depth: -180},
	}

	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("ожидался успех, получена ошибка: %v", err)
	}

	if len(grid.Points) != 4 {
		t.Fatalf("ожидалось 4 точки в сетке, получено %d", len(grid.Points))
	}

	if grid.Resolution != 0.01 {
		t.Fatalf("ожидалось разрешение 0.01, получено %f", grid.Resolution)
	}
}

func TestBuildGrid_EmptyPoints_Error(t *testing.T) {
	points := []BathymetryPoint{}

	_, err := BuildGrid(points, 0.01)
	if err == nil {
		t.Fatal("ожидалась ошибка для пустых точек, получен nil")
	}
}

func TestBuildGrid_ZeroResolution_Error(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
	}

	_, err := BuildGrid(points, 0)
	if err == nil {
		t.Fatal("ожидалась ошибка для нулевого разрешения, получен nil")
	}
}

func TestInterpolateDepth_ExactMatch_ReturnsDepth(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
		{Lat: 45.0, Lon: 30.01, Depth: -150},
		{Lat: 45.01, Lon: 30.0, Depth: -120},
		{Lat: 45.01, Lon: 30.01, Depth: -180},
	}

	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("ошибка построения сетки: %v", err)
	}

	depth, err := grid.InterpolateDepth(45.0, 30.0)
	if err != nil {
		t.Fatalf("ожидался успех, получена ошибка: %v", err)
	}

	if math.Abs(depth+100) > 0.01 {
		t.Fatalf("ожидалась глубина -100, получено %f", depth)
	}
}

func TestInterpolateDepth_Bilinear_Interpolates(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
		{Lat: 45.0, Lon: 30.01, Depth: -150},
		{Lat: 45.01, Lon: 30.0, Depth: -120},
		{Lat: 45.01, Lon: 30.01, Depth: -180},
	}

	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("ошибка построения сетки: %v", err)
	}

	depth, err := grid.InterpolateDepth(45.005, 30.005)
	if err != nil {
		t.Fatalf("ожидался успех, получена ошибка: %v", err)
	}

	expected := -137.5
	if math.Abs(depth-expected) > 0.1 {
		t.Fatalf("ожидалась глубина ~%f, получено %f", expected, depth)
	}
}

func TestInterpolateDepth_OutsideBounds_Error(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
	}

	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("ошибка построения сетки: %v", err)
	}

	_, err = grid.InterpolateDepth(50.0, 30.0)
	if err == nil {
		t.Fatal("ожидалась ошибка для точки за пределами сетки, получен nil")
	}

	if !containsString(err.Error(), "границ сетки") {
		t.Fatalf("ожидалось 'границ сетки' в ошибке, получено: %v", err)
	}
}

func TestInterpolateDepth_MissingNeighbors_Error(t *testing.T) {
	points := []BathymetryPoint{
		{Lat: 45.0, Lon: 30.0, Depth: -100},
		{Lat: 45.0, Lon: 30.01, Depth: -150},
		{Lat: 45.0, Lon: 30.02, Depth: -200},
		{Lat: 45.01, Lon: 30.0, Depth: -120},
		{Lat: 45.01, Lon: 30.01, Depth: -180},
	}

	grid, err := BuildGrid(points, 0.01)
	if err != nil {
		t.Fatalf("ошибка построения сетки: %v", err)
	}

	_, err = grid.InterpolateDepth(45.005, 30.015)
	if err == nil {
		t.Fatal("ожидалась ошибка для отсутствующих соседей, получен nil")
	}

	if !containsString(err.Error(), "недостающие соседние") {
		t.Fatalf("ожидалось 'недостающие соседние' в ошибке, получено: %v", err)
	}
}

func TestPhysicalDepthFactor_DeepWater_HighFactor(t *testing.T) {
	depth := -100.0
	fetch := 500.0
	depthScale := 50.0

	factor := physicalDepthFactor(depth, fetch, depthScale)

	if factor <= 0.8 {
		t.Fatalf("ожидался высокий коэффициент (>0.8) для глубокой воды, получено %f", factor)
	}
}

func TestPhysicalDepthFactor_ShallowWater_LowFactor(t *testing.T) {
	depth := -5.0
	fetch := 500.0
	depthScale := 50.0

	factor := physicalDepthFactor(depth, fetch, depthScale)

	if factor >= 0.2 {
		t.Fatalf("ожидался низкий коэффициент (<0.2) для мелководья, получено %f", factor)
	}
}

func TestPhysicalDepthFactor_ZeroDepth_ZeroFactor(t *testing.T) {
	depth := 0.0
	fetch := 500.0
	depthScale := 50.0

	factor := physicalDepthFactor(depth, fetch, depthScale)

	if factor != 0 {
		t.Fatalf("ожидался нулевой коэффициент для нулевой глубины, получено %f", factor)
	}
}

func TestWaveErosionWithBathymetry_Integration(t *testing.T) {
	points := []LatLon{
		{Lat: 45.0, Lon: 30.0},
		{Lat: 45.01, Lon: 30.01},
		{Lat: 45.0, Lon: 30.02},
		{Lat: 45.0, Lon: 30.0},
	}

	bathyData := []byte(`[
		{"lat": 44.99, "lon": 29.99, "depth": -200},
		{"lat": 44.99, "lon": 30.0, "depth": -150},
		{"lat": 44.99, "lon": 30.01, "depth": -100},
		{"lat": 44.99, "lon": 30.02, "depth": -50},
		{"lat": 45.0, "lon": 29.99, "depth": -180},
		{"lat": 45.0, "lon": 30.0, "depth": -130},
		{"lat": 45.0, "lon": 30.01, "depth": -80},
		{"lat": 45.0, "lon": 30.02, "depth": -30},
		{"lat": 45.01, "lon": 29.99, "depth": -160},
		{"lat": 45.01, "lon": 30.0, "depth": -110},
		{"lat": 45.01, "lon": 30.01, "depth": -60},
		{"lat": 45.01, "lon": 30.02, "depth": -10}
	]`)

	grid, err := LoadBathymetryFromJSON(bathyData, BathymetryLoadOptions{Resolution: 0.01})
	if err != nil {
		t.Fatalf("ошибка загрузки батиметрии: %v", err)
	}

	options := WaveErosionOptions{
		StrengthMeters:           20,
		WindSourceDirectionDeg:   90,
		WindSpeedMetersPerSecond: 10,
		FetchSpreadDeg:           45,
		FetchSamples:             7,
		MaxFetchMeters:           5000,
		DepthScaleMeters:         1000,
		ExposurePower:            1.2,
		BathymetryGrid:           grid,
	}

	snapshots := SimulateWaveErosionWithSeed(points, 1, options, 42)
	if len(snapshots) != 2 {
		t.Fatalf("ожидалось 2 снимка, получено %d", len(snapshots))
	}

	eroded := snapshots[1]
	if len(eroded) != len(points) {
		t.Fatalf("ожидалось %d точек, получено %d", len(points), len(eroded))
	}

	if eroded[0] != eroded[len(eroded)-1] {
		t.Fatal("ожидалось, что замкнутое кольцо останется замкнутым")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
