package geometry

import "github.com/paulmach/orb"

// ToORB converts a LatLon to an orb.Point (longitude, latitude order).
func ToORB(ll LatLon) orb.Point {
	return orb.Point{ll.Lon, ll.Lat}
}

// FromORB converts an orb.Point to a LatLon.
func FromORB(p orb.Point) LatLon {
	return LatLon{Lat: p[1], Lon: p[0]}
}

// ToORBLineString converts a slice of LatLon to an orb.LineString.
func ToORBLineString(points []LatLon) orb.LineString {
	ls := make(orb.LineString, len(points))
	for i, p := range points {
		ls[i] = ToORB(p)
	}
	return ls
}

// FromORBLineString converts an orb.LineString to a slice of LatLon.
func FromORBLineString(ls orb.LineString) []LatLon {
	points := make([]LatLon, len(ls))
	for i, p := range ls {
		points[i] = FromORB(p)
	}
	return points
}
