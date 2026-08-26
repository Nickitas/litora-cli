package geometry

import (
	"math"
	"strings"
	"testing"
)

const (
	qualityGridBaseLatitude  = 43.0
	qualityGridBaseLongitude = 34.0
	qualityGridResolution    = 0.01
)

func TestBathymetryInterpolationReferenceSurfaces(t *testing.T) {
	t.Run("наклонная плоскость восстанавливается точно", func(t *testing.T) {
		plane := func(latitude, longitude float64) float64 {
			return -50 - 4_000*(latitude-qualityGridBaseLatitude) - 6_000*(longitude-qualityGridBaseLongitude)
		}
		grid := buildQualityGrid(t, 2, 2, plane, nil)
		for _, position := range [][2]float64{{0.1, 0.2}, {0.25, 0.75}, {0.5, 0.5}, {0.9, 0.4}} {
			latitude := qualityGridBaseLatitude + position[0]*qualityGridResolution
			longitude := qualityGridBaseLongitude + position[1]*qualityGridResolution
			details, err := grid.SampleDepthDetailed(latitude, longitude, 2_000)
			if err != nil {
				t.Fatalf("выборка плоскости в (%.6f, %.6f): %v", latitude, longitude, err)
			}
			expected := plane(latitude, longitude)
			if !details.Interpolated || math.Abs(details.ElevationM-expected) > 1e-8 {
				t.Fatalf("плоскость восстановлена неверно: ожидалось %.9f, получено %+v", expected, details)
			}
		}
	})

	t.Run("квадратичная чаша сохраняет симметрию и диапазон", func(t *testing.T) {
		bowl := func(latitude, longitude float64) float64 {
			dLatitude := latitude - (qualityGridBaseLatitude + qualityGridResolution)
			dLongitude := longitude - (qualityGridBaseLongitude + qualityGridResolution)
			return -50 - 1_000_000*(dLatitude*dLatitude+dLongitude*dLongitude)
		}
		grid := buildQualityGrid(t, 3, 3, bowl, nil)
		center, err := grid.SampleDepthDetailed(
			qualityGridBaseLatitude+qualityGridResolution,
			qualityGridBaseLongitude+qualityGridResolution,
			2_000,
		)
		if err != nil {
			t.Fatal(err)
		}
		if !center.Exact || math.Abs(center.ElevationM+50) > 1e-9 {
			t.Fatalf("центр чаши должен быть точным исходным узлом -50 м: %+v", center)
		}

		southwest := sampleQualityGrid(t, grid, 0.5, 0.5)
		northeast := sampleQualityGrid(t, grid, 1.5, 1.5)
		if math.Abs(southwest-northeast) > 1e-8 {
			t.Fatalf("симметричные точки чаши различаются: %.9f и %.9f", southwest, northeast)
		}
		if southwest < -250-1e-8 || southwest > -50+1e-8 || southwest >= center.ElevationM {
			t.Fatalf("интерполяция чаши нарушила диапазон или монотонность: центр %.3f, склон %.3f", center.ElevationM, southwest)
		}
	})

	t.Run("ступень не создаёт выброс и смену знака", func(t *testing.T) {
		step := func(_ float64, longitude float64) float64 {
			if longitude <= qualityGridBaseLongitude+1e-12 {
				return -20
			}
			return -200
		}
		grid := buildQualityGrid(t, 2, 2, step, nil)
		for _, testCase := range []struct {
			fraction float64
			expected float64
		}{{0.25, -65}, {0.5, -110}, {0.75, -155}} {
			actual := sampleQualityGrid(t, grid, 0.5, testCase.fraction)
			if math.Abs(actual-testCase.expected) > 1e-9 {
				t.Fatalf("ступень при доле %.2f: ожидалось %.3f, получено %.3f", testCase.fraction, testCase.expected, actual)
			}
			assertFiniteElevationInRange(t, actual, -200, -20)
		}
	})

	t.Run("NoData не подменяется нулём", func(t *testing.T) {
		missing := map[[2]int]bool{{1, 1}: true}
		grid := buildQualityGrid(t, 2, 2, func(latitude, longitude float64) float64 {
			return -20 - 1_000*(latitude-qualityGridBaseLatitude) - 2_000*(longitude-qualityGridBaseLongitude)
		}, missing)
		latitude := qualityGridBaseLatitude + 0.5*qualityGridResolution
		longitude := qualityGridBaseLongitude + 0.5*qualityGridResolution
		if _, err := grid.SampleDepthDetailed(latitude, longitude, 0); err == nil {
			t.Fatal("ячейка с NoData без разрешённой ближайшей замены не должна возвращать скрытый ноль")
		}
		details, err := grid.SampleDepthDetailed(latitude, longitude, 2_000)
		if err != nil {
			t.Fatal(err)
		}
		if details.Exact || details.Interpolated || details.SourceDistanceMeters <= 0 || details.ElevationM == 0 {
			t.Fatalf("NoData должен дать явно различимую ближайшую замену реальным значением: %+v", details)
		}
		assertFiniteElevationInRange(t, details.ElevationM, -50, -20)
	})
}

func TestBuildGridRejectsInvalidPhysicalValues(t *testing.T) {
	valid := BathymetryPoint{Lat: qualityGridBaseLatitude, Lon: qualityGridBaseLongitude, Depth: -100}
	testCases := []struct {
		name       string
		point      BathymetryPoint
		resolution float64
		errorPart  string
	}{
		{name: "NaN широты", point: BathymetryPoint{Lat: math.NaN(), Lon: valid.Lon, Depth: valid.Depth}, resolution: qualityGridResolution, errorPart: "широта"},
		{name: "Inf долготы", point: BathymetryPoint{Lat: valid.Lat, Lon: math.Inf(1), Depth: valid.Depth}, resolution: qualityGridResolution, errorPart: "долгота"},
		{name: "NaN глубины", point: BathymetryPoint{Lat: valid.Lat, Lon: valid.Lon, Depth: math.NaN()}, resolution: qualityGridResolution, errorPart: "глубина"},
		{name: "Inf глубины", point: BathymetryPoint{Lat: valid.Lat, Lon: valid.Lon, Depth: math.Inf(-1)}, resolution: qualityGridResolution, errorPart: "глубина"},
		{name: "смена знака", point: BathymetryPoint{Lat: valid.Lat, Lon: valid.Lon, Depth: 1}, resolution: qualityGridResolution, errorPart: "положительная"},
		{name: "ниже диапазона", point: BathymetryPoint{Lat: valid.Lat, Lon: valid.Lon, Depth: -3000.01}, resolution: qualityGridResolution, errorPart: "предела"},
		{name: "NaN разрешения", point: valid, resolution: math.NaN(), errorPart: "разрешение"},
		{name: "Inf разрешения", point: valid, resolution: math.Inf(1), errorPart: "разрешение"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := BuildGrid([]BathymetryPoint{testCase.point}, testCase.resolution)
			if err == nil || !strings.Contains(err.Error(), testCase.errorPart) {
				t.Fatalf("ожидалась ошибка, содержащая %q, получено: %v", testCase.errorPart, err)
			}
		})
	}
}

func TestBilinearInterpolationPreservesFiniteInputRange(t *testing.T) {
	grid := buildQualityGrid(t, 2, 2, func(latitude, longitude float64) float64 {
		latitudeIndex := math.Round((latitude - qualityGridBaseLatitude) / qualityGridResolution)
		longitudeIndex := math.Round((longitude - qualityGridBaseLongitude) / qualityGridResolution)
		return -3_000 + 1_000*latitudeIndex + 2_000*longitudeIndex
	}, nil)
	for latitudeStep := 1; latitudeStep < 20; latitudeStep++ {
		for longitudeStep := 1; longitudeStep < 20; longitudeStep++ {
			actual := sampleQualityGrid(t, grid, float64(latitudeStep)/20, float64(longitudeStep)/20)
			assertFiniteElevationInRange(t, actual, -3_000, 0)
		}
	}
}

func FuzzBilinearInterpolationPreservesFiniteRange(f *testing.F) {
	f.Add(0.0, 10.0, 100.0, 3_000.0, 0.5, 0.5)
	f.Add(3_000.0, 2_000.0, 1_000.0, 0.0, 0.1, 0.9)
	f.Add(12.5, 12.5, 12.5, 12.5, 0.25, 0.75)
	f.Fuzz(func(t *testing.T, d00, d01, d10, d11, rawLatitudeFraction, rawLongitudeFraction float64) {
		values := []float64{d00, d01, d10, d11, rawLatitudeFraction, rawLongitudeFraction}
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				t.Skip()
			}
		}
		depths := [4]float64{
			-normalizeQualityDepth(d00),
			-normalizeQualityDepth(d01),
			-normalizeQualityDepth(d10),
			-normalizeQualityDepth(d11),
		}
		points := []BathymetryPoint{
			{Lat: qualityGridBaseLatitude, Lon: qualityGridBaseLongitude, Depth: depths[0]},
			{Lat: qualityGridBaseLatitude, Lon: qualityGridBaseLongitude + qualityGridResolution, Depth: depths[1]},
			{Lat: qualityGridBaseLatitude + qualityGridResolution, Lon: qualityGridBaseLongitude, Depth: depths[2]},
			{Lat: qualityGridBaseLatitude + qualityGridResolution, Lon: qualityGridBaseLongitude + qualityGridResolution, Depth: depths[3]},
		}
		grid, err := BuildGrid(points, qualityGridResolution)
		if err != nil {
			t.Fatal(err)
		}
		latitudeFraction := 0.001 + 0.998*normalizeQualityFraction(rawLatitudeFraction)
		longitudeFraction := 0.001 + 0.998*normalizeQualityFraction(rawLongitudeFraction)
		actual := sampleQualityGrid(t, grid, latitudeFraction, longitudeFraction)
		minimum, maximum := depths[0], depths[0]
		for _, depth := range depths[1:] {
			minimum = math.Min(minimum, depth)
			maximum = math.Max(maximum, depth)
		}
		assertFiniteElevationInRange(t, actual, minimum, maximum)
	})
}

func buildQualityGrid(t testing.TB, latitudeCount, longitudeCount int, elevation func(float64, float64) float64, missing map[[2]int]bool) *BathymetryGrid {
	t.Helper()
	points := make([]BathymetryPoint, 0, latitudeCount*longitudeCount)
	for latitudeIndex := 0; latitudeIndex < latitudeCount; latitudeIndex++ {
		for longitudeIndex := 0; longitudeIndex < longitudeCount; longitudeIndex++ {
			if missing[[2]int{latitudeIndex, longitudeIndex}] {
				continue
			}
			latitude := qualityGridBaseLatitude + float64(latitudeIndex)*qualityGridResolution
			longitude := qualityGridBaseLongitude + float64(longitudeIndex)*qualityGridResolution
			points = append(points, BathymetryPoint{Lat: latitude, Lon: longitude, Depth: elevation(latitude, longitude)})
		}
	}
	grid, err := BuildGrid(points, qualityGridResolution)
	if err != nil {
		t.Fatal(err)
	}
	return grid
}

func sampleQualityGrid(t testing.TB, grid *BathymetryGrid, latitudeFraction, longitudeFraction float64) float64 {
	t.Helper()
	latitude := qualityGridBaseLatitude + latitudeFraction*qualityGridResolution
	longitude := qualityGridBaseLongitude + longitudeFraction*qualityGridResolution
	actual, err := grid.InterpolateDepth(latitude, longitude)
	if err != nil {
		t.Fatalf("интерполяция в (%.6f, %.6f): %v", latitude, longitude, err)
	}
	return actual
}

func assertFiniteElevationInRange(t testing.TB, actual, minimum, maximum float64) {
	t.Helper()
	tolerance := 1e-9 * math.Max(1, math.Max(math.Abs(minimum), math.Abs(maximum)))
	if math.IsNaN(actual) || math.IsInf(actual, 0) {
		t.Fatalf("интерполяция вернула неконечную отметку: %v", actual)
	}
	if actual < minimum-tolerance || actual > maximum+tolerance {
		t.Fatalf("отметка %.12f вышла за диапазон исходных значений [%.12f, %.12f]", actual, minimum, maximum)
	}
	if actual > tolerance {
		t.Fatalf("подводная отметка сменила знак и стала положительной: %.12f", actual)
	}
}

func normalizeQualityDepth(value float64) float64 {
	return math.Mod(math.Abs(value), 3_000.0000001)
}

func normalizeQualityFraction(value float64) float64 {
	return math.Mod(math.Abs(value), 1)
}
