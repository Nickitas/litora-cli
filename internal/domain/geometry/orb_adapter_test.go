package geometry

import (
	"github.com/paulmach/orb"
	"testing"
)

func TestToORB(t *testing.T) {
	ll := LatLon{Lat: 55.75, Lon: 37.61}
	point := ToORB(ll)

	if point[0] != 37.61 {
		t.Errorf("ToORB() longitude = %v, want %v", point[0], 37.61)
	}
	if point[1] != 55.75 {
		t.Errorf("ToORB() latitude = %v, want %v", point[1], 55.75)
	}
}

func TestFromORB(t *testing.T) {
	point := orb.Point{37.61, 55.75}
	ll := FromORB(point)

	if ll.Lat != 55.75 {
		t.Errorf("FromORB() latitude = %v, want %v", ll.Lat, 55.75)
	}
	if ll.Lon != 37.61 {
		t.Errorf("FromORB() longitude = %v, want %v", ll.Lon, 37.61)
	}
}

func TestToORBLineString(t *testing.T) {
	points := []LatLon{
		{Lat: 55.75, Lon: 37.61},
		{Lat: 55.76, Lon: 37.62},
		{Lat: 55.77, Lon: 37.63},
	}

	lineString := ToORBLineString(points)

	if len(lineString) != 3 {
		t.Errorf("ToORBLineString() length = %v, want %v", len(lineString), 3)
	}

	if lineString[0][0] != 37.61 || lineString[0][1] != 55.75 {
		t.Errorf("ToORBLineString() first point = %v, want [37.61, 55.75]", lineString[0])
	}
}

func TestFromORBLineString(t *testing.T) {
	lineString := orb.LineString{
		{37.61, 55.75},
		{37.62, 55.76},
		{37.63, 55.77},
	}

	points := FromORBLineString(lineString)

	if len(points) != 3 {
		t.Errorf("FromORBLineString() length = %v, want %v", len(points), 3)
	}

	if points[0].Lat != 55.75 || points[0].Lon != 37.61 {
		t.Errorf("FromORBLineString() first point = %v, want {Lat: 55.75, Lon: 37.61}", points[0])
	}
}
