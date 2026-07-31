package fractal

import (
	"context"
	"math"
	"runtime"
	"sync"

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

// ========== Parallel Optimized Versions ==========

// AnalyzeBoxCountingParallel performs box counting analysis with parallel processing
func AnalyzeBoxCountingParallel(ctx context.Context, points []geometry.LatLon) BoxCountingAnalysis {
	select {
	case <-ctx.Done():
		return BoxCountingAnalysis{}
	default:
	}

	if len(points) < 2 {
		return BoxCountingAnalysis{}
	}

	meters := make([]Point2D, len(points))

	// Convert points in parallel
	numConvWorkers := fractalMin(runtime.NumCPU(), 4)
	convChunkSize := (len(points) + numConvWorkers - 1) / numConvWorkers

	var convWg sync.WaitGroup
	for w := 0; w < numConvWorkers; w++ {
		start := w * convChunkSize
		end := fractalMin(start+convChunkSize, len(points))

		convWg.Add(1)
		go func(workerID int) {
			defer convWg.Done()

			for i := start; i < end; i++ {
				select {
				case <-ctx.Done():
					return
				default:
					meters[i] = latLonToMeters(points[i])
				}
			}
		}(w)
	}
	convWg.Wait()

	if ctx.Err() != nil {
		return BoxCountingAnalysis{}
	}

	// Calculate bounds in parallel
	minX, maxX, minY, maxY := bboxMetersParallel(ctx, meters)
	width := maxX - minX
	height := maxY - minY
	bboxSize := math.Max(width, height)
	if bboxSize < 1 {
		return BoxCountingAnalysis{}
	}

	// Process scale factors in parallel
	samples := make([]BoxCountingSample, 0, len(defaultScaleFactors))
	var samplesMu sync.Mutex

	scaleWorkers := fractalMin(runtime.NumCPU(), 8)
	scaleChunkSize := (len(defaultScaleFactors) + scaleWorkers - 1) / scaleWorkers

	var scaleWg sync.WaitGroup
	for w := 0; w < scaleWorkers; w++ {
		start := w * scaleChunkSize
		end := fractalMin(start+scaleChunkSize, len(defaultScaleFactors))

		scaleWg.Add(1)
		go func(workerID int) {
			defer scaleWg.Done()

			localSamples := make([]BoxCountingSample, 0)

			for idx := start; idx < end; idx++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				factor := defaultScaleFactors[idx]
				boxSize := bboxSize / factor
				if boxSize <= 0 {
					continue
				}

				boxes := boxesCoveredMetersAverageParallel(ctx, meters, boxSize, minX, minY, gridOffsets)
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
				localSamples = append(localSamples, sample)
			}

			samplesMu.Lock()
			samples = append(samples, localSamples...)
			samplesMu.Unlock()
		}(w)
	}
	scaleWg.Wait()

	if ctx.Err() != nil {
		return BoxCountingAnalysis{}
	}

	if len(samples) < minScaleSamples {
		return BoxCountingAnalysis{Samples: samples}
	}

	// Prepare log arrays
	logInvScale := make([]float64, len(samples))
	logBoxes := make([]float64, len(samples))
	for i, s := range samples {
		logInvScale[i] = s.LogInvScale
		logBoxes[i] = s.LogBoxes
	}

	window := bestRegressionWindowParallel(ctx, logInvScale, logBoxes)
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

// bboxMetersParallel computes bounding box in parallel
func bboxMetersParallel(ctx context.Context, points []Point2D) (minX, maxX, minY, maxY float64) {
	if len(points) == 0 {
		return 0, 0, 0, 0
	}

	numWorkers := fractalMin(runtime.NumCPU(), 4)
	chunkSize := (len(points) + numWorkers - 1) / numWorkers

	type boundsResult struct {
		minX, maxX, minY, maxY float64
	}

	results := make(chan boundsResult, numWorkers)

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := fractalMin(start+chunkSize, len(points))

		wg.Add(1)
		go func(workerID, s, e int) {
			defer wg.Done()

			if s >= len(points) {
				results <- boundsResult{}
				return
			}

			localMinX := points[s].X
			localMaxX := points[s].X
			localMinY := points[s].Y
			localMaxY := points[s].Y

			for i := s + 1; i < e && i < len(points); i++ {
				select {
				case <-ctx.Done():
					results <- boundsResult{}
					return
				default:
				}

				p := points[i]
				if p.X < localMinX {
					localMinX = p.X
				}
				if p.X > localMaxX {
					localMaxX = p.X
				}
				if p.Y < localMinY {
					localMinY = p.Y
				}
				if p.Y > localMaxY {
					localMaxY = p.Y
				}
			}

			results <- boundsResult{
				minX: localMinX,
				maxX: localMaxX,
				minY: localMinY,
				maxY: localMaxY,
			}
		}(w, start, end)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate results
	minX = points[0].X
	maxX = points[0].X
	minY = points[0].Y
	maxY = points[0].Y

	first := true
	for res := range results {
		if ctx.Err() != nil {
			return
		}
		if first {
			minX, maxX, minY, maxY = res.minX, res.maxX, res.minY, res.maxY
			first = false
		} else {
			if res.minX < minX {
				minX = res.minX
			}
			if res.maxX > maxX {
				maxX = res.maxX
			}
			if res.minY < minY {
				minY = res.minY
			}
			if res.maxY > maxY {
				maxY = res.maxY
			}
		}
	}

	return minX, maxX, minY, maxY
}

// boxesCoveredMetersAverageParallel computes average box coverage across offsets in parallel
func boxesCoveredMetersAverageParallel(ctx context.Context, points []Point2D, boxSize, minX, minY float64, offsets [][2]float64) float64 {
	if len(offsets) == 0 {
		offsets = [][2]float64{{0, 0}}
	}

	numOffsets := len(offsets)
	type offsetResult struct {
		covered map[[2]int]struct{}
		count   int
	}

	results := make(chan offsetResult, numOffsets)
	var wg sync.WaitGroup

	for oIdx, off := range offsets {
		wg.Add(1)
		go func(idx int, offset [2]float64) {
			defer wg.Done()

			covered := make(map[[2]int]struct{})
			localPoints := points

			for i := 1; i < len(localPoints); i++ {
				select {
				case <-ctx.Done():
					results <- offsetResult{}
					return
				default:
				}

				markSegmentBoxesOffset(covered, localPoints[i-1], localPoints[i],
					boxSize, minX, minY, offset[0], offset[1])
			}

			results <- offsetResult{
				covered: covered,
				count:   len(covered),
			}
		}(oIdx, off)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	sum := 0.0
	validCount := 0

	for res := range results {
		if ctx.Err() != nil {
			return 0
		}
		if res.covered != nil {
			sum += float64(res.count)
			validCount++
		}
	}

	if validCount == 0 {
		return 0
	}
	return sum / float64(validCount)
}

// bestRegressionWindowParallel finds best regression window using parallel evaluation
func bestRegressionWindowParallel(ctx context.Context, x, y []float64) *regressionWindow {
	n := len(x)
	if n < minScaleSamples || len(y) != n {
		return nil
	}

	type windowCandidate struct {
		window *regressionWindow
		stable bool
	}

	numWorkers := fractalMin(runtime.NumCPU(), 8)
	totalWindows := 0

	for start := 0; start <= n-minScaleSamples; start++ {
		for end := start + minScaleSamples - 1; end < n; end++ {
			totalWindows++
		}
	}

	chunkSize := (totalWindows + numWorkers - 1) / numWorkers

	candidates := make(chan windowCandidate, totalWindows)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		startWindow := w * chunkSize
		endWin := fractalMin(startWindow+chunkSize, totalWindows)

		wg.Add(1)
		go func(workerID, sWin, eWin int) {
			defer wg.Done()

			currentIdx := 0
			localBest := &windowCandidate{}

			for start := 0; start <= n-minScaleSamples; start++ {
				for end := start + minScaleSamples - 1; end < n; end++ {
					if currentIdx < sWin {
						currentIdx++
						continue
					}
					if currentIdx >= eWin {
						break
					}

					select {
					case <-ctx.Done():
						return
					default:
					}

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

					if betterWindow(localBest.window, candidate, stable) {
						localBest = &windowCandidate{window: candidate, stable: stable}
					}

					currentIdx++
				}
				if currentIdx >= eWin {
					break
				}
			}

			if localBest.window != nil {
				candidates <- *localBest
			}
		}(w, startWindow, endWin)
	}

	go func() {
		wg.Wait()
		close(candidates)
	}()

	var best *windowCandidate
	for cand := range candidates {
		if ctx.Err() != nil {
			return nil
		}
		if betterWindow(best.window, cand.window, cand.stable) {
			best = &cand
		}
	}

	return best.window
}

// FractalDimensionParallel computes fractal dimension using parallel box counting
func FractalDimensionParallel(ctx context.Context, points []geometry.LatLon) float64 {
	analysis := AnalyzeBoxCountingParallel(ctx, points)
	if !analysis.Valid {
		return 1.0
	}
	return analysis.Dimension
}

// valueSpreadParallel computes value spread in parallel
func valueSpreadParallel(ctx context.Context, values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	numWorkers := fractalMin(runtime.NumCPU(), 4)
	chunkSize := (len(values) + numWorkers - 1) / numWorkers

	type spreadResult struct {
		minVal float64
		maxVal float64
	}

	results := make(chan spreadResult, numWorkers)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := fractalMin(start+chunkSize, len(values))

		wg.Add(1)
		go func(workerID, s, e int) {
			defer wg.Done()

			if s >= len(values) {
				results <- spreadResult{}
				return
			}

			localMin := values[s]
			localMax := values[s]

			for i := s + 1; i < e && i < len(values); i++ {
				select {
				case <-ctx.Done():
					results <- spreadResult{}
					return
				default:
				}

				val := values[i]
				if val < localMin {
					localMin = val
				}
				if val > localMax {
					localMax = val
				}
			}

			results <- spreadResult{minVal: localMin, maxVal: localMax}
		}(w, start, end)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	minValue := values[0]
	maxValue := values[0]
	first := true

	for res := range results {
		if ctx.Err() != nil {
			return 0
		}
		if first {
			minValue = res.minVal
			maxValue = res.maxVal
			first = false
		} else {
			if res.minVal < minValue {
				minValue = res.minVal
			}
			if res.maxVal > maxValue {
				maxValue = res.maxVal
			}
		}
	}

	return maxValue - minValue
}

// Helper function for min (avoiding name conflicts)
func fractalMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
