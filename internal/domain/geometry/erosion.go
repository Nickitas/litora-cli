package geometry

import (
	"context"
	"log/slog"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const (
	erosionChunkSize     = 512
	metersPerDegLat      = 111194.9
	erosionWorkerCount   = 8 // Number of parallel workers for erosion
	fetchWorkerCount     = 4 // Number of parallel workers for fetch distance calculation
	fetchCacheSize       = 256 // Size of LRU cache for fetch distances
)

type WaveErosionOptions struct {
	StrengthMeters           float64
	WindSourceDirectionDeg   float64
	WindSpeedMetersPerSecond float64
	FetchSpreadDeg           float64
	FetchSamples             int
	MaxFetchMeters           float64
	DepthScaleMeters         float64
	ExposurePower            float64
	MaxRetreatMeters         float64
	ProbeDistanceMeters      float64
	Irregularity             float64
	BathymetryGrid           *BathymetryGrid
	LithologyProfile         *LithologyProfile
	EnableLithology          bool

	// Динамическая литология (более реалистичная модель)
	DynamicLithologyMap        *SpatialLithologyMap
	EnableDynamicLithology     bool
	LithologyInteractionParams LithologyInteractionParams
	WeatheringProfile          WeatheringProfile
	SimulationYears            float64 // время симуляции для выветривания
}

type waveSideResponse struct {
	Score       float64
	MeanFetch   float64
	FetchFactor float64
	Exposure    float64
	DepthFactor float64
}

type projectionReference struct {
	RefLat          float64
	RefLon          float64
	MetersPerDegLon float64
}

// Erode applies a Gaussian-distributed random displacement to every point.
// strength is the standard deviation of the displacement in meters; zero or
// negative values return a clone of the input without changes.
func Erode(points []LatLon, strength float64) []LatLon {
	return erodeWithRand(points, strength, rand.New(rand.NewSource(time.Now().UnixNano())))
}

// ErodeWithSeed mirrors Erode but allows a fixed seed for reproducible output.
func ErodeWithSeed(points []LatLon, strength float64, seed int64) []LatLon {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return erodeWithRand(points, strength, rand.New(rand.NewSource(seed)))
}

// SimulateErosion runs multiple erosion steps and returns snapshot after each step,
// including the initial state at index 0.
func SimulateErosion(points []LatLon, steps int, strength float64) [][]LatLon {
	return SimulateErosionWithSeed(points, steps, strength, time.Now().UnixNano())
}

// SimulateErosionWithSeed is deterministic for a fixed seed.
func SimulateErosionWithSeed(points []LatLon, steps int, strength float64, seed int64) [][]LatLon {
	if steps < 0 {
		steps = 0
	}

	snapshots := make([][]LatLon, steps+1)

	current := clonePoints(points)
	snapshots[0] = current

	for i := 1; i <= steps; i++ {
		current = erodeParallel(current, strength, seed, i)
		snapshots[i] = current
	}
	return snapshots
}

// SimulateWaveErosion runs a directional wave-driven shoreline smoothing model.
func SimulateWaveErosion(points []LatLon, steps int, options WaveErosionOptions) [][]LatLon {
	return SimulateWaveErosionWithSeed(points, steps, options, time.Now().UnixNano())
}

// SimulateWaveErosionWithSeed mirrors SimulateWaveErosion but keeps the output reproducible.
func SimulateWaveErosionWithSeed(points []LatLon, steps int, options WaveErosionOptions, seed int64) [][]LatLon {
	if steps < 0 {
		steps = 0
	}
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	options = normalizeWaveErosionOptions(options)
	snapshots := make([][]LatLon, steps+1)
	current := clonePoints(points)
	snapshots[0] = current

	for i := 1; i <= steps; i++ {
		current = waveErodeStep(current, options, seed, i)
		snapshots[i] = current
	}

	return snapshots
}

func erodeWithRand(points []LatLon, strength float64, rng *rand.Rand) []LatLon {
	if len(points) == 0 {
		return nil
	}
	if strength <= 0 || rng == nil {
		return clonePoints(points)
	}

	// Use mean latitude to approximate meters-to-degrees conversion.
	refLat := 0.0
	for _, p := range points {
		refLat += p.Lat
	}
	refLat /= float64(len(points))

	metersPerDegLat := 111194.9
	metersPerDegLon := metersPerDegLat * math.Cos(refLat*math.Pi/180)
	if math.Abs(metersPerDegLon) < 1e-9 {
		metersPerDegLon = metersPerDegLat
	}

	eroded := make([]LatLon, len(points))
	firstShiftLat := 0.0
	firstShiftLon := 0.0
	closed := isClosedPolyline(points)

	for i, p := range points {
		dx := rng.NormFloat64() * strength
		dy := rng.NormFloat64() * strength

		if closed {
			if i == 0 {
				firstShiftLat = dy
				firstShiftLon = dx
			}
			if i == len(points)-1 {
				dy = firstShiftLat
				dx = firstShiftLon
			}
		}

		eroded[i] = LatLon{
			Lat: p.Lat + dy/metersPerDegLat,
			Lon: p.Lon + dx/metersPerDegLon,
		}
	}

	return eroded
}

func erodeParallel(points []LatLon, strength float64, seed int64, step int) []LatLon {
	if len(points) == 0 || strength <= 0 {
		return clonePoints(points)
	}

	closed := isClosedPolyline(points)
	refLat := 0.0
	for _, p := range points {
		refLat += p.Lat
	}
	refLat /= float64(len(points))

	metersPerDegLon := metersPerDegLat * math.Cos(refLat*math.Pi/180)
	if math.Abs(metersPerDegLon) < 1e-9 {
		metersPerDegLon = metersPerDegLat
	}

	out := make([]LatLon, len(points))

	var wg sync.WaitGroup
	var mu sync.Mutex
	firstShiftLat := 0.0
	firstShiftLon := 0.0

	chunkSize := erosionChunkSize
	for start := 0; start < len(points); start += chunkSize {
		end := start + chunkSize
		if end > len(points) {
			end = len(points)
		}

		startIdx := start
		endIdx := end
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := startIdx; i < endIdx; i++ {
				p := points[i]
				localSeed := seed + int64(step)*10_000 + int64(i)
				rng := rand.New(rand.NewSource(localSeed))
				dx := rng.NormFloat64() * strength
				dy := rng.NormFloat64() * strength

				if closed && i == 0 {
					mu.Lock()
					firstShiftLat = dy
					firstShiftLon = dx
					mu.Unlock()
				}

				out[i] = LatLon{
					Lat: p.Lat + dy/metersPerDegLat,
					Lon: p.Lon + dx/metersPerDegLon,
				}
			}
		}()
	}
	wg.Wait()

	if closed && len(out) > 1 {
		last := len(out) - 1
		p := points[last]
		out[last] = LatLon{
			Lat: p.Lat + firstShiftLat/metersPerDegLat,
			Lon: p.Lon + firstShiftLon/metersPerDegLon,
		}
	}

	return out
}

func waveErodeStep(points []LatLon, options WaveErosionOptions, seed int64, step int) []LatLon {
	if len(points) < 3 || options.StrengthMeters <= 0 {
		return clonePoints(points)
	}

	closed := isClosedPolyline(points)
	working := clonePoints(points)
	if closed {
		working = clonePoints(points[:len(points)-1])
	}
	if len(working) < 3 {
		return clonePoints(points)
	}

	projected, ref := projectToMetersWithReference(working)
	out := make([]pointXY, len(projected))
	copy(out, projected)

	mainDirection := directionFromNorthClockwise(options.WindSourceDirectionDeg)

	for i := range projected {
		if !closed && (i == 0 || i == len(projected)-1) {
			continue
		}

		prevIndex, nextIndex := waveNeighborIndexes(i, len(projected), closed)
		current := projected[i]
		prev := projected[prevIndex]
		next := projected[nextIndex]

		tangent := normalizeXY(pointXY{
			X: next.X - prev.X,
			Y: next.Y - prev.Y,
		})
		if vectorLength(tangent) == 0 {
			continue
		}

		leftNormal := pointXY{X: -tangent.Y, Y: tangent.X}
		rightNormal := pointXY{X: tangent.Y, Y: -tangent.X}

		lat := working[i].Lat
		lon := working[i].Lon

		leftResponse := sampleWaveSide(projected, i, leftNormal, mainDirection, closed, options, lat, lon)
		rightResponse := sampleWaveSide(projected, i, rightNormal, mainDirection, closed, options, lat, lon)

		seawardNormal := leftNormal
		response := leftResponse
		if rightResponse.Score > leftResponse.Score || (almostEqual(rightResponse.Score, leftResponse.Score) && dotXY(rightNormal, mainDirection) > dotXY(leftNormal, mainDirection)) {
			seawardNormal = rightNormal
			response = rightResponse
		}
		if response.Score <= 0 {
			continue
		}

		shapeDelta := pointXY{
			X: (prev.X+next.X)/2 - current.X,
			Y: (prev.Y+next.Y)/2 - current.Y,
		}
		localScale := 0.5 * (distanceXY(current, prev) + distanceXY(current, next))
		if localScale <= 1e-6 {
			continue
		}

		protrusion := clamp(-dotXY(shapeDelta, seawardNormal)/localScale, 0, 1.5)
		bayShelter := clamp(dotXY(shapeDelta, seawardNormal)/localScale, 0, 1.2)

		windFactor := math.Pow(options.WindSpeedMetersPerSecond/12.0, 2)
		windFactor = clamp(windFactor, 0.1, 4.0)

		retreatMeters := options.StrengthMeters * windFactor * response.Score
		retreatMeters *= clamp(0.55+protrusion-bayShelter*0.35, 0.1, 1.75)
		if options.MaxRetreatMeters > 0 {
			retreatMeters = math.Min(retreatMeters, options.MaxRetreatMeters)
		}

		// Lithology modulation: resistance reduces retreat rate
		if options.EnableLithology && options.LithologyProfile != nil {
			lithology := options.LithologyProfile.GetLithologyAt(lat, lon)
			if lithology.Resistance > 0 {
				// Higher resistance = slower retreat (inverse relationship)
				// Resistance=1.0 is baseline, Resistance>1.0 slows erosion
				retreatMeters /= lithology.Resistance
			}
		}

		// Dynamic lithology: more sophisticated modulation with weathering
		if options.EnableDynamicLithology && options.DynamicLithologyMap != nil {
			// Find nearest point in spatial map
			if i < len(options.DynamicLithologyMap.Points) {
				dynamicState := options.DynamicLithologyMap.Points[i]

				// Apply weathering if simulation time is set
				if options.SimulationYears > 0 {
					weatheredState := ApplyWeathering(
						dynamicState.Static,
						options.SimulationYears,
						options.WeatheringProfile,
						1.0, // climate factor
					)
					retreatMeters = CalculateLithologyErosionInteraction(
						retreatMeters,
						weatheredState,
						options.LithologyInteractionParams,
						false, // isStorm
					)
				} else {
					// Use current resistance directly
					if dynamicState.CurrentResistance > 0 {
						retreatMeters /= dynamicState.CurrentResistance
					}
				}
			}
		}

		if retreatMeters <= 0 {
			continue
		}

		smoothingAlpha := math.Min(retreatMeters/localScale, 0.5)

		if options.Irregularity > 0 {
			rng := rand.New(rand.NewSource(seed + int64(step)*10_000 + int64(i)))
			retreatMeters *= clamp(1+rng.NormFloat64()*options.Irregularity, 0.7, 1.3)
		}

		out[i] = pointXY{
			X: current.X - seawardNormal.X*retreatMeters + shapeDelta.X*smoothingAlpha,
			Y: current.Y - seawardNormal.Y*retreatMeters + shapeDelta.Y*smoothingAlpha,
		}
	}

	updated := make([]LatLon, len(out))
	for i, point := range out {
		updated[i] = projectFromMeters(point, ref)
	}
	if closed {
		updated = append(updated, updated[0])
	}
	return updated
}

func sampleWaveSide(projected []pointXY, index int, normal, mainDirection pointXY, closed bool, options WaveErosionOptions, lat, lon float64) waveSideResponse {
	normal = normalizeXY(normal)
	if vectorLength(normal) == 0 {
		return waveSideResponse{}
	}

	weightedFetch := 0.0
	weightSum := 0.0
	for sample := 0; sample < options.FetchSamples; sample++ {
		direction := sampleWaveDirection(mainDirection, options.FetchSpreadDeg, sample, options.FetchSamples)
		incidence := dotXY(normal, direction)
		if incidence <= 0 {
			continue
		}

		weight := math.Pow(incidence, options.ExposurePower)
		fetch := rayFetchDistance(projected, index, direction, closed, options.ProbeDistanceMeters, options.MaxFetchMeters)
		weightedFetch += fetch * weight
		weightSum += weight
	}

	if weightSum == 0 {
		return waveSideResponse{}
	}

	meanFetch := weightedFetch / weightSum
	normalFetch := rayFetchDistance(projected, index, normal, closed, options.ProbeDistanceMeters, options.MaxFetchMeters)
	fetchFactor := math.Sqrt(clamp(meanFetch/options.MaxFetchMeters, 0, 1))
	exposure := clamp(weightSum/float64(options.FetchSamples), 0, 1)

	var depthFactor float64
	if options.BathymetryGrid != nil {
		depth, err := options.BathymetryGrid.InterpolateDepth(lat, lon)
		if err == nil {
			depthFactor = physicalDepthFactor(depth, normalFetch, options.DepthScaleMeters)
		} else {
			// Graceful degradation: логируем предупреждение, но продолжаем
			// Используем geometric proxy как fallback
			depthFactor = 1 - math.Exp(-normalFetch/options.DepthScaleMeters)
		}
	} else {
		depthFactor = 1 - math.Exp(-normalFetch/options.DepthScaleMeters)
	}

	score := fetchFactor * exposure * (0.35 + 0.65*depthFactor)

	return waveSideResponse{
		Score:       score,
		MeanFetch:   meanFetch,
		FetchFactor: fetchFactor,
		Exposure:    exposure,
		DepthFactor: depthFactor,
	}
}

func sampleWaveDirection(mainDirection pointXY, spreadDeg float64, sampleIndex, sampleCount int) pointXY {
	if sampleCount <= 1 {
		return normalizeXY(mainDirection)
	}

	fraction := float64(sampleIndex) / float64(sampleCount-1)
	offsetDeg := -spreadDeg + fraction*2*spreadDeg
	return rotateXY(normalizeXY(mainDirection), offsetDeg*math.Pi/180)
}

func rayFetchDistance(projected []pointXY, index int, direction pointXY, closed bool, probeDistance, maxFetch float64) float64 {
	direction = normalizeXY(direction)
	if len(projected) < 2 || vectorLength(direction) == 0 || maxFetch <= 0 {
		return 0
	}
	if probeDistance <= 0 {
		probeDistance = 25
	}
	if probeDistance >= maxFetch {
		probeDistance = maxFetch * 0.1
	}
	if probeDistance <= 0 {
		probeDistance = 1
	}

	origin := pointXY{
		X: projected[index].X + direction.X*probeDistance,
		Y: projected[index].Y + direction.Y*probeDistance,
	}

	limit := maxFetch - probeDistance
	if limit <= 0 {
		return maxFetch
	}

	best := limit
	segmentCount := len(projected) - 1
	if closed {
		segmentCount = len(projected)
	}

	for segmentIndex := 0; segmentIndex < segmentCount; segmentIndex++ {
		if segmentTouchesVertex(segmentIndex, index, len(projected), closed) {
			continue
		}

		a := projected[segmentIndex]
		b := projected[(segmentIndex+1)%len(projected)]
		if !closed {
			b = projected[segmentIndex+1]
		}

		distance, ok := raySegmentDistance(origin, direction, a, b)
		if ok && distance < best {
			best = distance
		}
	}

	return probeDistance + best
}

func raySegmentDistance(origin, direction, a, b pointXY) (float64, bool) {
	segment := pointXY{X: b.X - a.X, Y: b.Y - a.Y}
	denominator := crossXY(direction, segment)
	if math.Abs(denominator) < 1e-9 {
		return 0, false
	}

	delta := pointXY{X: a.X - origin.X, Y: a.Y - origin.Y}
	t := crossXY(delta, segment) / denominator
	u := crossXY(delta, direction) / denominator
	if t <= 1e-6 || u < -1e-6 || u > 1+1e-6 {
		return 0, false
	}

	return t, true
}

func segmentTouchesVertex(segmentIndex, pointIndex, pointCount int, closed bool) bool {
	if closed {
		prevSegment := (pointIndex - 1 + pointCount) % pointCount
		nextSegment := pointIndex % pointCount
		return segmentIndex == prevSegment || segmentIndex == nextSegment
	}

	return segmentIndex == pointIndex-1 || segmentIndex == pointIndex
}

func waveNeighborIndexes(index, pointCount int, closed bool) (int, int) {
	if closed {
		return (index - 1 + pointCount) % pointCount, (index + 1) % pointCount
	}
	return index - 1, index + 1
}

func normalizeWaveErosionOptions(options WaveErosionOptions) WaveErosionOptions {
	if options.WindSpeedMetersPerSecond <= 0 {
		options.WindSpeedMetersPerSecond = 12
	}
	if options.FetchSpreadDeg <= 0 {
		options.FetchSpreadDeg = 55
	}
	if options.FetchSamples <= 0 {
		options.FetchSamples = 9
	}
	if options.MaxFetchMeters <= 0 {
		options.MaxFetchMeters = 150_000
	}
	if options.DepthScaleMeters <= 0 {
		options.DepthScaleMeters = 4_000
	}
	if options.ExposurePower <= 0 {
		options.ExposurePower = 1.5
	}
	if options.MaxRetreatMeters <= 0 {
		options.MaxRetreatMeters = options.StrengthMeters
	}
	if options.ProbeDistanceMeters <= 0 {
		options.ProbeDistanceMeters = 25
	}
	if options.Irregularity < 0 {
		options.Irregularity = 0
	}

	options.WindSourceDirectionDeg = math.Mod(options.WindSourceDirectionDeg, 360)
	if options.WindSourceDirectionDeg < 0 {
		options.WindSourceDirectionDeg += 360
	}

	return options
}

func projectToMetersWithReference(points []LatLon) ([]pointXY, projectionReference) {
	if len(points) == 0 {
		return nil, projectionReference{}
	}

	ref := projectionReference{}
	for _, point := range points {
		ref.RefLat += point.Lat
		ref.RefLon += point.Lon
	}
	ref.RefLat /= float64(len(points))
	ref.RefLon /= float64(len(points))
	ref.MetersPerDegLon = metersPerDegLat * math.Cos(ref.RefLat*math.Pi/180)
	if math.Abs(ref.MetersPerDegLon) < 1e-9 {
		ref.MetersPerDegLon = metersPerDegLat
	}

	projected := make([]pointXY, len(points))
	for i, point := range points {
		projected[i] = pointXY{
			X: (point.Lon - ref.RefLon) * ref.MetersPerDegLon,
			Y: (point.Lat - ref.RefLat) * metersPerDegLat,
		}
	}

	return projected, ref
}

func projectFromMeters(point pointXY, ref projectionReference) LatLon {
	return LatLon{
		Lat: ref.RefLat + point.Y/metersPerDegLat,
		Lon: ref.RefLon + point.X/ref.MetersPerDegLon,
	}
}

func directionFromNorthClockwise(deg float64) pointXY {
	rad := deg * math.Pi / 180
	return pointXY{
		X: math.Sin(rad),
		Y: math.Cos(rad),
	}
}

func rotateXY(point pointXY, angleRad float64) pointXY {
	cosA := math.Cos(angleRad)
	sinA := math.Sin(angleRad)
	return pointXY{
		X: point.X*cosA - point.Y*sinA,
		Y: point.X*sinA + point.Y*cosA,
	}
}

func normalizeXY(point pointXY) pointXY {
	length := vectorLength(point)
	if length == 0 {
		return pointXY{}
	}
	return pointXY{
		X: point.X / length,
		Y: point.Y / length,
	}
}

func vectorLength(point pointXY) float64 {
	return math.Hypot(point.X, point.Y)
}

func dotXY(a, b pointXY) float64 {
	return a.X*b.X + a.Y*b.Y
}

func crossXY(a, b pointXY) float64 {
	return a.X*b.Y - a.Y*b.X
}

func distanceXY(a, b pointXY) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}

// erosionJob represents a single point processing job
type erosionJob struct {
	index          int
	point          pointXY
	prev           pointXY
	next           pointXY
	lat            float64
	lon            float64
	leftNormal     pointXY
	rightNormal    pointXY
	mainDirection  pointXY
	projected      []pointXY
	closed         bool
	options        WaveErosionOptions
	seed           int64
	step           int
	localScale     float64
}

// erosionResult represents the result of processing a single point
type erosionResult struct {
	index int
	point pointXY
	valid bool
}

// fetchDistanceCache implements a thread-safe LRU cache for fetch distances
type fetchDistanceCache struct {
	mu      sync.RWMutex
	entries map[uint64]float64
	keys    []uint64
	maxSize int
	hits    atomic.Uint64
	misses  atomic.Uint64
}

func newFetchDistanceCache(size int) *fetchDistanceCache {
	return &fetchDistanceCache{
		entries: make(map[uint64]float64, size),
		keys:    make([]uint64, 0, size),
		maxSize: size,
	}
}

func (c *fetchDistanceCache) get(index int, directionKey uint64) (float64, bool) {
	key := uint64(index)<<32 | directionKey
	c.mu.RLock()
	val, ok := c.entries[key]
	c.mu.RUnlock()
	if ok {
		c.hits.Add(1)
		return val, true
	}
	c.misses.Add(1)
	return 0, false
}

func (c *fetchDistanceCache) put(index int, directionKey uint64, value float64) {
	key := uint64(index)<<32 | directionKey
	c.mu.Lock()
	if len(c.entries) >= c.maxSize {
		if len(c.keys) > 0 {
			delete(c.entries, c.keys[0])
			c.keys = c.keys[1:]
		}
	}
	c.entries[key] = value
	c.keys = append(c.keys, key)
	c.mu.Unlock()
}

// waveErodeStepParallel is the optimized version of waveErodeStep using concurrency
func waveErodeStepParallel(points []LatLon, options WaveErosionOptions, seed int64, step int) []LatLon {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	return waveErodeStepWithContext(ctx, points, options, seed, step)
}

// waveErodeStepWithContext performs wave erosion with context support
func waveErodeStepWithContext(ctx context.Context, points []LatLon, options WaveErosionOptions, seed int64, step int) []LatLon {
	if len(points) < 3 || options.StrengthMeters <= 0 {
		return clonePoints(points)
	}

	closed := isClosedPolyline(points)
	working := clonePoints(points)
	if closed {
		working = clonePoints(points[:len(points)-1])
	}
	if len(working) < 3 {
		return clonePoints(points)
	}

	projected, ref := projectToMetersWithReference(working)
	out := make([]pointXY, len(projected))
	copy(out, projected)

	mainDirection := directionFromNorthClockwise(options.WindSourceDirectionDeg)

	numWorkers := erosionWorkerCount
	if numCPUs := runtime.NumCPU(); numCPUs > 0 {
		if numCPUs > numWorkers {
			numWorkers = numCPUs
		}
	}
	if len(projected) < numWorkers*4 {
		if numWorkers := len(projected) / 4; numWorkers > 1 {
			numWorkers = numWorkers
		} else {
			numWorkers = 1
		}
	}

	jobs := make(chan erosionJob, len(projected))
	results := make(chan erosionResult, len(projected))

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	done := ctx.Done()

	for w := 0; w < numWorkers; w++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				case job, ok := <-jobs:
					if !ok {
						return
					}
					result := processErosionJob(job)
					results <- result
				}
			}
		}()
	}

	numSkipped := atomic.Int32{}
	for i := range projected {
		if !closed && (i == 0 || i == len(projected)-1) {
			continue
		}

		prevIndex, nextIndex := waveNeighborIndexes(i, len(projected), closed)
		current := projected[i]
		prev := projected[prevIndex]
		next := projected[nextIndex]

		tangent := normalizeXY(pointXY{X: next.X - prev.X, Y: next.Y - prev.Y})
		if vectorLength(tangent) == 0 {
			numSkipped.Add(1)
			continue
		}

		leftNormal := pointXY{X: -tangent.Y, Y: tangent.X}
		rightNormal := pointXY{X: tangent.Y, Y: -tangent.X}

		lat := working[i].Lat
		lon := working[i].Lon

		localScale := 0.5 * (distanceXY(current, prev) + distanceXY(current, next))
		if localScale <= 1e-6 {
			numSkipped.Add(1)
			continue
		}

		job := erosionJob{
			index:         i,
			point:         current,
			prev:          prev,
			next:          next,
			lat:           lat,
			lon:           lon,
			leftNormal:    leftNormal,
			rightNormal:   rightNormal,
			mainDirection: mainDirection,
			projected:     projected,
			closed:        closed,
			options:       options,
			seed:          seed,
			step:          step,
			localScale:    localScale,
		}

		select {
		case <-done:
			break
		case jobs <- job:
		}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		if result.valid {
			out[result.index] = result.point
		}
		select {
		case <-done:
			break
		default:
		}
	}

	if skipped := numSkipped.Load(); skipped > 0 {
		slog.Debug("skipped erosion points", "count", skipped)
	}

	updated := make([]LatLon, len(out))
	for i, point := range out {
		updated[i] = projectFromMeters(point, ref)
	}
	if closed {
		updated = append(updated, updated[0])
	}
	return updated
}

// processErosionJob processes a single erosion job
func processErosionJob(job erosionJob) erosionResult {
	type sideResult struct {
		response waveSideResponse
	}

	leftCh := make(chan sideResult, 1)
	rightCh := make(chan sideResult, 1)

	go func() {
		resp := sampleWaveSide(job.projected, job.index, job.leftNormal, job.mainDirection,
			job.closed, job.options, job.lat, job.lon)
		leftCh <- sideResult{response: resp}
	}()

	go func() {
		resp := sampleWaveSide(job.projected, job.index, job.rightNormal, job.mainDirection,
			job.closed, job.options, job.lat, job.lon)
		rightCh <- sideResult{response: resp}
	}()

	leftResp := <-leftCh
	rightResp := <-rightCh

	seawardNormal := job.leftNormal
	response := leftResp.response
	if rightResp.response.Score > leftResp.response.Score ||
		(almostEqual(rightResp.response.Score, leftResp.response.Score) &&
			dotXY(job.rightNormal, job.mainDirection) > dotXY(job.leftNormal, job.mainDirection)) {
		seawardNormal = job.rightNormal
		response = rightResp.response
	}

	if response.Score <= 0 {
		return erosionResult{index: job.index, valid: false}
	}

	shapeDelta := pointXY{
		X: (job.prev.X+job.next.X)/2 - job.point.X,
		Y: (job.prev.Y+job.next.Y)/2 - job.point.Y,
	}

	protrusion := clamp(-dotXY(shapeDelta, seawardNormal)/job.localScale, 0, 1.5)
	bayShelter := clamp(dotXY(shapeDelta, seawardNormal)/job.localScale, 0, 1.2)

	windFactor := math.Pow(job.options.WindSpeedMetersPerSecond/12.0, 2)
	windFactor = clamp(windFactor, 0.1, 4.0)

	retreatMeters := job.options.StrengthMeters * windFactor * response.Score
	retreatMeters *= clamp(0.55+protrusion-bayShelter*0.35, 0.1, 1.75)
	if job.options.MaxRetreatMeters > 0 {
		retreatMeters = math.Min(retreatMeters, job.options.MaxRetreatMeters)
	}

	if job.options.EnableLithology && job.options.LithologyProfile != nil {
		lithology := job.options.LithologyProfile.GetLithologyAt(job.lat, job.lon)
		if lithology.Resistance > 0 {
			retreatMeters /= lithology.Resistance
		}
	}

	if job.options.EnableDynamicLithology && job.options.DynamicLithologyMap != nil {
		if job.index < len(job.options.DynamicLithologyMap.Points) {
			dynamicState := job.options.DynamicLithologyMap.Points[job.index]

			if job.options.SimulationYears > 0 {
				weatheredState := ApplyWeathering(
					dynamicState.Static,
					job.options.SimulationYears,
					job.options.WeatheringProfile,
					1.0,
				)
				retreatMeters = CalculateLithologyErosionInteraction(
					retreatMeters,
					weatheredState,
					job.options.LithologyInteractionParams,
					false,
				)
			} else {
				if dynamicState.CurrentResistance > 0 {
					retreatMeters /= dynamicState.CurrentResistance
				}
			}
		}
	}

	if retreatMeters <= 0 {
		return erosionResult{index: job.index, valid: false}
	}

	smoothingAlpha := math.Min(retreatMeters/job.localScale, 0.5)

	if job.options.Irregularity > 0 {
		rng := rand.New(rand.NewSource(job.seed + int64(job.step)*10_000 + int64(job.index)))
		retreatMeters *= clamp(1+rng.NormFloat64()*job.options.Irregularity, 0.7, 1.3)
	}

	newPoint := pointXY{
		X: job.point.X - seawardNormal.X*retreatMeters + shapeDelta.X*smoothingAlpha,
		Y: job.point.Y - seawardNormal.Y*retreatMeters + shapeDelta.Y*smoothingAlpha,
	}

	return erosionResult{index: job.index, point: newPoint, valid: true}
}

// sampleWaveSideParallel is an optimized version using parallel fetch sampling
func sampleWaveSideParallel(projected []pointXY, index int, normal, mainDirection pointXY,
	closed bool, options WaveErosionOptions, lat, lon float64, cache *fetchDistanceCache) waveSideResponse {

	normal = normalizeXY(normal)
	if vectorLength(normal) == 0 {
		return waveSideResponse{}
	}

	numSamples := options.FetchSamples
	if numSamples <= 0 {
		numSamples = 9
	}

	type sampleResult struct {
		fetch float64
		weight float64
		valid bool
	}

	results := make([]sampleResult, numSamples)
	var wg sync.WaitGroup
	wg.Add(numSamples)

	for sample := 0; sample < numSamples; sample++ {
		go func(s int) {
			defer wg.Done()

			direction := sampleWaveDirection(mainDirection, options.FetchSpreadDeg, s, numSamples)
			incidence := dotXY(normal, direction)

			if incidence <= 0 {
				results[s] = sampleResult{valid: false}
				return
			}

			weight := math.Pow(incidence, options.ExposurePower)

			directionKey := math.Float64bits(direction.X) ^ math.Float64bits(direction.Y)
			cached, hit := cache.get(index, directionKey)

			var fetch float64
			if hit {
				fetch = cached
			} else {
				fetch = rayFetchDistance(projected, index, direction, closed,
					options.ProbeDistanceMeters, options.MaxFetchMeters)
				cache.put(index, directionKey, fetch)
			}

			results[s] = sampleResult{
				fetch:  fetch,
				weight: weight,
				valid:  true,
			}
		}(sample)
	}

	wg.Wait()

	weightedFetch := 0.0
	weightSum := 0.0

	for _, r := range results {
		if !r.valid {
			continue
		}
		weightedFetch += r.fetch * r.weight
		weightSum += r.weight
	}

	if weightSum == 0 {
		return waveSideResponse{}
	}

	meanFetch := weightedFetch / weightSum

	normalKey := math.Float64bits(normal.X) ^ math.Float64bits(normal.Y)
	normalFetch, _ := cache.get(index, normalKey)
	if normalFetch == 0 {
		normalFetch = rayFetchDistance(projected, index, normal, closed,
			options.ProbeDistanceMeters, options.MaxFetchMeters)
		cache.put(index, normalKey, normalFetch)
	}

	fetchFactor := math.Sqrt(clamp(meanFetch/options.MaxFetchMeters, 0, 1))
	exposure := clamp(weightSum/float64(numSamples), 0, 1)

	var depthFactor float64
	if options.BathymetryGrid != nil {
		depth, err := options.BathymetryGrid.InterpolateDepth(lat, lon)
		if err == nil {
			depthFactor = physicalDepthFactor(depth, normalFetch, options.DepthScaleMeters)
		} else {
			depthFactor = 1 - math.Exp(-normalFetch/options.DepthScaleMeters)
		}
	} else {
		depthFactor = 1 - math.Exp(-normalFetch/options.DepthScaleMeters)
	}

	score := fetchFactor * exposure * (0.35 + 0.65*depthFactor)

	return waveSideResponse{
		Score:       score,
		MeanFetch:   meanFetch,
		FetchFactor: fetchFactor,
		Exposure:    exposure,
		DepthFactor: depthFactor,
	}
}

// rayFetchDistanceParallel computes fetch distance with parallel segment checking
func rayFetchDistanceParallel(ctx context.Context, projected []pointXY, index int,
	direction pointXY, closed bool, probeDistance, maxFetch float64) float64 {

	direction = normalizeXY(direction)
	if len(projected) < 2 || vectorLength(direction) == 0 || maxFetch <= 0 {
		return 0
	}
	if probeDistance <= 0 {
		probeDistance = 25
	}
	if probeDistance >= maxFetch {
		probeDistance = maxFetch * 0.1
	}
	if probeDistance <= 0 {
		probeDistance = 1
	}

	origin := pointXY{
		X: projected[index].X + direction.X*probeDistance,
		Y: projected[index].Y + direction.Y*probeDistance,
	}

	limit := maxFetch - probeDistance
	if limit <= 0 {
		return maxFetch
	}

	segmentCount := len(projected) - 1
	if closed {
		segmentCount = len(projected)
	}

	var bestDistance uint64 = math.Float64bits(limit)
	var bestDistanceMu sync.Mutex

	var wg sync.WaitGroup
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8
	}
	chunkSize := (segmentCount + numWorkers - 1) / numWorkers

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > segmentCount {
			end = segmentCount
		}

		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			localBest := limit

			for segmentIndex := start; segmentIndex < end; segmentIndex++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if segmentTouchesVertex(segmentIndex, index, len(projected), closed) {
					continue
				}

				a := projected[segmentIndex]
				b := projected[(segmentIndex+1)%len(projected)]
				if !closed {
					b = projected[segmentIndex+1]
				}

				distance, ok := raySegmentDistance(origin, direction, a, b)
				if ok && distance < localBest {
					localBest = distance
				}
			}

			if localBest < math.Float64frombits(atomic.LoadUint64(&bestDistance)) {
				bestDistanceMu.Lock()
				currentBest := math.Float64frombits(atomic.LoadUint64(&bestDistance))
				if localBest < currentBest {
					atomic.StoreUint64(&bestDistance, math.Float64bits(localBest))
				}
				bestDistanceMu.Unlock()
			}
		}(w)
	}

	wg.Wait()

	return probeDistance + math.Float64frombits(atomic.LoadUint64(&bestDistance))
}
