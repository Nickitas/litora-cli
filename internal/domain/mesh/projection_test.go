package mesh

import (
	"math"
	"testing"

	"coastal-geometry/internal/domain/blacksea"
	"coastal-geometry/internal/domain/geometry"
)

func TestEqualAreaProjectionRoundTripAtBlackSeaBoundaries(t *testing.T) {
	t.Parallel()

	projection := EqualAreaProjection{ReferenceLat: 43.2765240426, ReferenceLon: 34.1751334089}
	points := map[string]geometry.LatLon{
		"запад":  {Lat: 43.2, Lon: blacksea.MinLongitude},
		"восток": {Lat: 43.2, Lon: blacksea.MaxLongitude},
		"север":  {Lat: blacksea.MaxLatitude, Lon: 34.0},
		"юг":     {Lat: blacksea.MinLatitude, Lon: 34.0},
	}

	const maximumAllowedErrorMeters = 0.001
	for name, original := range points {
		name, original := name, original
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			projected := projection.Project(original)
			recovered, err := projection.Unproject(projected)
			if err != nil {
				t.Fatal(err)
			}
			errorMeters := geometry.Haversine(original, recovered) * 1000
			if errorMeters > maximumAllowedErrorMeters {
				t.Fatalf("ошибка цикла %.9f м превышает %.3f м: исходная=%+v, восстановленная=%+v", errorMeters, maximumAllowedErrorMeters, original, recovered)
			}
		})
	}
}

func TestEqualAreaProjectionAssignsGeographicCoordinatesToEveryNode(t *testing.T) {
	t.Parallel()

	projection := EqualAreaProjection{ReferenceLat: 43.2765240426, ReferenceLon: 34.1751334089}
	nodes := []Point{
		{},
		{X: -1000, Y: -500},
		{X: 0, Y: 0},
		{X: 1000, Y: 500},
	}
	if err := projection.AssignGeographicCoordinates(nodes); err != nil {
		t.Fatal(err)
	}
	for nodeID := 1; nodeID < len(nodes); nodeID++ {
		point := nodes[nodeID]
		if !point.GeographicCoordinatesSet {
			t.Fatalf("узел %d не получил WGS 84", nodeID)
		}
		if math.IsNaN(point.LatitudeDeg) || math.IsNaN(point.LongitudeDeg) ||
			point.LatitudeDeg < blacksea.MinLatitude || point.LatitudeDeg > blacksea.MaxLatitude ||
			point.LongitudeDeg < blacksea.MinLongitude || point.LongitudeDeg > blacksea.MaxLongitude {
			t.Fatalf("узел %d получил координаты вне Чёрного моря: %+v", nodeID, point)
		}
	}
}

func TestEqualAreaProjectionRejectsPointOutsideProjectionDisc(t *testing.T) {
	t.Parallel()

	projection := EqualAreaProjection{ReferenceLat: 43, ReferenceLon: 34}
	_, err := projection.Unproject(Point{X: 2*equalAreaEarthRadiusMeters + 1, Y: 0})
	if err == nil {
		t.Fatal("ожидалась ошибка для точки вне допустимого круга LAEA")
	}
}
