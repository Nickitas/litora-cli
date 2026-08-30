package geometry

import "github.com/paulmach/orb"

// ToORB преобразует LatLon в orb.Point (порядок долготы и широты).
func ToORB(ll LatLon) orb.Point {
	return orb.Point{ll.Lon, ll.Lat}
}

// FromORB преобразует orb.Point в прямоугольник.
func FromORB(p orb.Point) LatLon {
	return LatLon{Lat: p[1], Lon: p[0]}
}

// ToORBLineString преобразует фрагмент LatLon в orb.LineString.
func ToORBLineString(points []LatLon) orb.LineString {
	ls := make(orb.LineString, len(points))
	for i, p := range points {
		ls[i] = ToORB(p)
	}
	return ls
}

// FromORBLineString преобразует orb.LineString в фрагмент латлона.
func FromORBLineString(ls orb.LineString) []LatLon {
	points := make([]LatLon, len(ls))
	for i, p := range ls {
		points[i] = FromORB(p)
	}
	return points
}
