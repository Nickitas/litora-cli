package koch

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
	"math"
	"math/rand"
)

const MaxIterations = 10

type OrganicOptions struct {
	Seed            int64
	AngleJitterDeg  float64
	HeightJitterPct float64
}

func OrganicKochCurve(base []geometry.LatLon, iterations int, opts OrganicOptions) []geometry.LatLon {
	if iterations < 0 {
		iterations = 0
	}
	if iterations > MaxIterations {
		fmt.Printf("Предупреждение: слишком много итераций (%d). Ограничено до %d\n", iterations, MaxIterations)
		iterations = MaxIterations
	}

	result := make([]geometry.LatLon, len(base))
	copy(result, base)

	if iterations == 0 {
		return result
	}

	rng := rand.New(rand.NewSource(opts.Seed))
	for i := 0; i < iterations; i++ {
		result = organicKochIteration(result, rng, opts)
	}
	return result
}

func organicKochIteration(points []geometry.LatLon, rng *rand.Rand, opts OrganicOptions) []geometry.LatLon {
	if len(points) < 2 {
		return points
	}

	newPoints := make([]geometry.LatLon, 0, len(points)*4)
	for i := 0; i < len(points)-1; i++ {
		segment := organicKochSegment(points[i], points[i+1], rng, opts)
		newPoints = append(newPoints, segment...)
	}
	newPoints = append(newPoints, points[len(points)-1])
	return newPoints
}

func organicKochSegment(a, b geometry.LatLon, rng *rand.Rand, opts OrganicOptions) []geometry.LatLon {
	vx := b.Lon - a.Lon
	vy := b.Lat - a.Lat

	thirdX := vx / 3.0
	thirdY := vy / 3.0

	p1 := geometry.LatLon{Lat: a.Lat + thirdY, Lon: a.Lon + thirdX}
	p3 := geometry.LatLon{Lat: a.Lat + 2*thirdY, Lon: a.Lon + 2*thirdX}

	angle := (60.0 + randomSigned(rng, opts.AngleJitterDeg)) * math.Pi / 180.0
	heightScale := 1.0 + randomSigned(rng, opts.HeightJitterPct)

	dx := thirdX
	dy := thirdY
	rotX := dx*math.Cos(angle) - dy*math.Sin(angle)
	rotY := dx*math.Sin(angle) + dy*math.Cos(angle)

	p2 := geometry.LatLon{
		Lat: p1.Lat + rotY*heightScale,
		Lon: p1.Lon + rotX*heightScale,
	}

	return []geometry.LatLon{a, p1, p2, p3}
}

func randomSigned(rng *rand.Rand, amplitude float64) float64 {
	if amplitude <= 0 {
		return 0
	}
	return (rng.Float64()*2 - 1) * amplitude
}
