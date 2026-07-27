package fractal

import (
	"math"

	"coastal-geometry/internal/domain/geometry"
)

const (
	minScaleSamples       = 4
	minStableLocalSlopes  = 3
	minRegressionRSquared = 0.98
	maxLocalSlopeSpread   = 0.18
)

// denser grid to reduce sensitivity to scale selection
var defaultScaleFactors = []float64{4, 6, 8, 12, 16, 24, 32, 48, 64, 96, 128, 192, 256}

var gridOffsets = [][2]float64{
	{0, 0},
	{0.5, 0},
	{0, 0.5},
	{0.5, 0.5},
}

type Point2D struct{ X, Y float64 }

type BoxCountingSample struct {
	ScaleFactor   float64
	RelativeScale float64
	BoxSizeMeters float64
	BoxesCovered  int
	LogInvScale   float64
	LogBoxes      float64
}

type BoxCountingAnalysis struct {
	Dimension          float64
	RegressionRSquared float64
	StableAcrossScales bool
	StabilitySpread    float64
	Samples            []BoxCountingSample
	LocalDimensions    []float64
	Valid              bool
}

func FractalDimension(points []geometry.LatLon) float64 {
	analysis := AnalyzeBoxCounting(points)
	if !analysis.Valid {
		return 1.0
	}
	return analysis.Dimension
}

func AnalyzeBoxCounting(points []geometry.LatLon) BoxCountingAnalysis {
	if len(points) < 2 {
		return BoxCountingAnalysis{}
	}

	meters := make([]Point2D, len(points))
	for i, p := range points {
		meters[i] = latLonToMeters(p)
	}

	minX, maxX, minY, maxY := bboxMeters(meters)
	width := maxX - minX
	height := maxY - minY
	bboxSize := math.Max(width, height)
	if bboxSize < 1 {
		return BoxCountingAnalysis{}
	}

	samples := make([]BoxCountingSample, 0, len(defaultScaleFactors))
	logInvScale := make([]float64, 0, len(defaultScaleFactors))
	logBoxes := make([]float64, 0, len(defaultScaleFactors))
	for _, factor := range defaultScaleFactors {
		boxSize := bboxSize / factor
		if boxSize <= 0 {
			continue
		}
		boxes := boxesCoveredMetersAverage(meters, boxSize, minX, minY, gridOffsets)
		if boxes <= 1 {
			continue
		}

		relativeScale := boxSize / bboxSize
		sample := BoxCountingSample{
			ScaleFactor:   factor,
			RelativeScale: relativeScale,
			BoxSizeMeters: boxSize,
			BoxesCovered:  int(math.Round(boxes)),
			LogInvScale:   math.Log(1.0 / relativeScale),
			LogBoxes:      math.Log(boxes),
		}

		samples = append(samples, sample)
		logInvScale = append(logInvScale, sample.LogInvScale)
		logBoxes = append(logBoxes, sample.LogBoxes)
	}

	if len(samples) < minScaleSamples {
		return BoxCountingAnalysis{Samples: samples}
	}

	window := bestRegressionWindow(logInvScale, logBoxes)
	if window == nil || window.length < minScaleSamples {
		return BoxCountingAnalysis{Samples: samples}
	}

	localDimensions := localSlopeSeries(window.x, window.y)
	spread := valueSpread(localDimensions)
	stable := len(localDimensions) >= minStableLocalSlopes &&
		window.rSquared >= minRegressionRSquared &&
		spread <= maxLocalSlopeSpread

	if window.slope < 0.5 || window.slope > 3.0 {
		return BoxCountingAnalysis{
			Samples:            samples,
			LocalDimensions:    localDimensions,
			RegressionRSquared: window.rSquared,
			StabilitySpread:    spread,
		}
	}

	return BoxCountingAnalysis{
		Dimension:          window.slope,
		RegressionRSquared: window.rSquared,
		StableAcrossScales: stable,
		StabilitySpread:    spread,
		Samples:            samples,
		LocalDimensions:    localDimensions,
		Valid:              true,
	}
}

func latLonToMeters(p geometry.LatLon) Point2D {
	const (
		refLat          = 43.5
		metersPerDegLat = 111194.9
		metersPerDegLon = 87300.0
	)

	dLat := (p.Lat - refLat) * metersPerDegLat
	dLon := (p.Lon - 35.0) * metersPerDegLon

	return Point2D{X: dLon, Y: dLat}
}

func bboxMeters(points []Point2D) (minX, maxX, minY, maxY float64) {
	if len(points) == 0 {
		return 0, 0, 0, 0
	}
	minX, minY = points[0].X, points[0].Y
	maxX, maxY = points[0].X, points[0].Y
	for _, p := range points {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	return
}

func boxesCoveredMetersAverage(points []Point2D, boxSize, minX, minY float64, offsets [][2]float64) float64 {
	if len(offsets) == 0 {
		offsets = [][2]float64{{0, 0}}
	}

	sum := 0.0
	for _, off := range offsets {
		covered := make(map[[2]int]struct{})
		for i := 1; i < len(points); i++ {
			markSegmentBoxesOffset(covered, points[i-1], points[i], boxSize, minX, minY, off[0], off[1])
		}
		sum += float64(len(covered))
	}

	return sum / float64(len(offsets))
}

func markSegmentBoxesOffset(covered map[[2]int]struct{}, a, b Point2D, boxSize, minX, minY, offsetX, offsetY float64) {
	dx := b.X - a.X
	dy := b.Y - a.Y
	distance := math.Hypot(dx, dy)
	steps := 1
	if boxSize > 0 {
		steps = int(math.Ceil(distance/(boxSize/2))) + 1
	}
	if steps < 2 {
		steps = 2
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := a.X + dx*t
		y := a.Y + dy*t
		row := int(math.Floor((y - minY + offsetY*boxSize) / boxSize))
		col := int(math.Floor((x - minX + offsetX*boxSize) / boxSize))
		covered[[2]int{row, col}] = struct{}{}
	}
}

type regressionWindow struct {
	start     int
	end       int
	length    int
	slope     float64
	intercept float64
	rSquared  float64
	spread    float64
	x         []float64
	y         []float64
}

func bestRegressionWindow(x, y []float64) *regressionWindow {
	n := len(x)
	if n < minScaleSamples || len(y) != n {
		return nil
	}

	var best *regressionWindow
	for start := 0; start <= n-minScaleSamples; start++ {
		for end := start + minScaleSamples - 1; end < n; end++ {
			xs := x[start : end+1]
			ys := y[start : end+1]
			slope, intercept := linearRegression(xs, ys)
			r2 := regressionRSquared(xs, ys, slope, intercept)
			locals := localSlopeSeries(xs, ys)
			spread := valueSpread(locals)
			stable := len(locals) >= minStableLocalSlopes && r2 >= minRegressionRSquared && spread <= maxLocalSlopeSpread

			candidate := &regressionWindow{
				start:     start,
				end:       end,
				length:    end - start + 1,
				slope:     slope,
				intercept: intercept,
				rSquared:  r2,
				spread:    spread,
				x:         xs,
				y:         ys,
			}

			if betterWindow(best, candidate, stable) {
				best = candidate
			}
		}
	}

	return best
}

func betterWindow(current, candidate *regressionWindow, candidateStable bool) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}

	currentStable := current.rSquared >= minRegressionRSquared && current.spread <= maxLocalSlopeSpread && current.length >= minScaleSamples

	// Prefer stable windows
	if candidateStable != currentStable {
		return candidateStable
	}
	// Prefer longer windows
	if candidate.length != current.length {
		return candidate.length > current.length
	}
	// Then higher R^2
	if math.Abs(candidate.rSquared-current.rSquared) > 1e-9 {
		return candidate.rSquared > current.rSquared
	}
	// If R^2 close, prefer higher slope (captures finer detail)
	if math.Abs(candidate.rSquared-current.rSquared) <= 1e-3 {
		if math.Abs(candidate.slope-current.slope) > 1e-6 {
			return candidate.slope > current.slope
		}
	}
	// Then lower spread
	return candidate.spread < current.spread
}

func localSlopeSeries(x, y []float64) []float64 {
	if len(x) != len(y) || len(x) < 2 {
		return nil
	}

	slopes := make([]float64, 0, len(x)-1)
	for i := 1; i < len(x); i++ {
		denominator := x[i] - x[i-1]
		if math.Abs(denominator) < 1e-12 {
			continue
		}
		slopes = append(slopes, (y[i]-y[i-1])/denominator)
	}
	return slopes
}

func valueSpread(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	minValue := values[0]
	maxValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue - minValue
}

func linearRegression(x, y []float64) (slope, intercept float64) {
	n := float64(len(x))
	var sumX, sumY, sumXY, sumX2 float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}
	denominator := n*sumX2 - sumX*sumX
	if math.Abs(denominator) < 1e-12 {
		return 0, 0
	}
	slope = (n*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / n
	return slope, intercept
}

func regressionRSquared(x, y []float64, slope, intercept float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	var meanY float64
	for _, value := range y {
		meanY += value
	}
	meanY /= float64(len(y))

	var ssTot, ssRes float64
	for i := range x {
		predicted := slope*x[i] + intercept
		residual := y[i] - predicted
		total := y[i] - meanY
		ssRes += residual * residual
		ssTot += total * total
	}

	if ssTot < 1e-12 {
		return 1
	}
	return 1 - ssRes/ssTot
}
