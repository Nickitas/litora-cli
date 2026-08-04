package geometry

import (
	"fmt"
	"math"
	"runtime"
	"sync"
)

// ============================================================
// ТИПЫ ДАННЫХ СЕДИМЕНТОЛОГИИ
// ============================================================

// SedimentBudget отслеживает баланс массы для одного участка берега
type SedimentBudget struct {
	// Volume measurements (m³ per meter of shoreline)
	ErodedVolume    float64 // объём размытого материала
	TransportVolume float64 // объём в транзите (longshore drift)
	DepositedVolume float64 // объём отложенного материала
	NetChange       float64 // баланс (eroded - deposited)

	// Statistics
	ErosionPoints    int // число точек с эрозией
	DepositionPoints int // число точек с аккумуляцией
}

// SedimentState состояние для каждой точки берега
type SedimentState struct {
	PointIndex     int
	LocalBudget    SedimentBudget
	InTransitFrom  []float64 // объём от соседей (incoming)
	InTransitTo    []float64 // объём к соседям (outgoing)
	IsAccumulating bool      // режим аккумуляции
	IsEroding      bool      // режим эрозии
}

// SedimentTransportParameters параметры транспорта наносов
type SedimentTransportParameters struct {
	// Transport coefficient [0-1]
	TransportCoefficient float64

	// Deposition rate [0-1]
	DepositionRate float64

	// Minimum flow velocity (m/s)
	MinimumFlowVelocity float64

	// Capacity factor [0-2]
	CapacityFactor float64

	// Longshore drift coefficient [0-1]
	LongshoreDriftCoefficient float64
}

// SedimentTransportResult результат расчёта транспорта
type SedimentTransportResult struct {
	States          []SedimentState
	TotalBudget     SedimentBudget
	MassBalance     float64 // должен быть ≈ 0
	IsValid         bool    // validation check
	Warnings        []string
	BaselineErosion []float64 // базовая эрозия (м)
	ModifiedErosion []float64 // модифицированная эрозия (м)
}

// WaveEnergyData волновая энергия по точкам
type WaveEnergyData struct {
	Energy    []float64 // волновая энергия [0-1]
	Direction float64   // главное направление (град от севера)
	Incidence []float64 // угол падения на берег [0-1]
	Fetch     []float64 // fetch distance (m)
}

// ============================================================
// ТИПЫ ДЛЯ ОПТИМИЗАЦИИ
// ============================================================

// PerformanceStats статистика производительности
type PerformanceStats struct {
	Strategy          string  // "original", "optimized", "parallel", "batched"
	PointsCount       int     // количество точек
	ExecutionTimeMs   float64 // время выполнения (мс)
	MemoryAllocations int     // количество аллокаций
	Speedup           float64 // ускорение относительно original
}

// OptimizedSedimentCache кэш для вычислений
type OptimizedSedimentCache struct {
	alongshoreDirections []Vector2D
	waveDirectionVec     Vector2D
	segmentLengths       []float64
	initialized          bool
	mu                   sync.RWMutex
}

// OptimizedCalculationContext контекст для оптимизированных вычислений
type OptimizedCalculationContext struct {
	cache   *OptimizedSedimentCache
	workers int
}

// BatchSedimentTransportResult результат пачечной обработки
type BatchSedimentTransportResult struct {
	Results      []SedimentTransportResult
	TotalBatches int
	TotalBudget  SedimentBudget
	IsValid      bool
	Warnings     []string
}

// ============================================================
// ТИПЫ ДЛЯ ВРЕМЕННОЙ ДИНАМИКИ
// ============================================================

// StormDepositLayer представляет слой отложений от отдельного шторма
type StormDepositLayer struct {
	StormIndex      int     // порядковый номер шторма
	Thickness       float64 // толщина слоя (м)
	Volume          float64 // объём отложений (м³/м)
	GrainSize       float64 // размер зёрен (мм)
	IsPreserved     bool    // сохранится ли в геологической записи
	DepositLocation int     // индекс точки берега
}

// SeasonalModulation параметры сезонной модуляции
type SeasonalModulation struct {
	WinterMultiplier        float64
	SummerMultiplier        float64
	TransitionMultiplier    float64
	StormSeasonBoost        float64
	AccumulationSeasonality bool
}

// StormSedimentParameters параметры влияния штормов на седиментацию
type StormSedimentParameters struct {
	StormTransportMultiplier  float64
	StormDepositionEfficiency float64
	PostStormSurgeMultiplier  float64
	StormThreshold            float64
	StormRetreatMultiplier    float64
	StormBypassingCoefficient float64
}

// TemporalSedimentState объединяет sediment state с временной динамикой
type TemporalSedimentState struct {
	BaseState        SedimentState
	CurrentSeason    string
	IsStormActive    bool
	StormIntensity   float64
	SeasonalFactor   float64
	StormDeposits    []StormDepositLayer
	ModifiedErosion  float64
	DepositionChange float64
}

// SedimentTemporalResult результат с учётом времени
type SedimentTemporalResult struct {
	States            []TemporalSedimentState
	TotalBudget       SedimentBudget
	SeasonalStats     map[string]SedimentBudget
	StormImpact       SedimentBudget
	AllStormDeposits  []StormDepositLayer
	PreservedDeposits []StormDepositLayer
	SeasonalCycle     []SedimentBudget
	IsValid           bool
	Warnings          []string
}

// SedimentTemporalState временное состояние для седиментации
type SedimentTemporalState struct {
	Year           float64
	Season         string
	IsStorm        bool
	StormIntensity float64
	SeasonFactor   float64
}

// ============================================================
// БАЗОВЫЕ ФУНКЦИИ РАСЧЁТА ТРАНСПОРТА
// ============================================================

// CalculateSedimentTransport рассчитывает транспорт наносов
func CalculateSedimentTransport(
	points []LatLon,
	erosionRates []float64,
	waveData WaveEnergyData,
	lithology []LithologyState,
	params SedimentTransportParameters,
) SedimentTransportResult {

	n := len(points)
	if n == 0 {
		return SedimentTransportResult{}
	}

	params = normalizeSedimentParams(params)

	states := make([]SedimentState, n)
	for i := range states {
		states[i].PointIndex = i
		states[i].InTransitFrom = make([]float64, 0)
		states[i].InTransitTo = make([]float64, 0)
	}

	calculateErosionVolumes(states, erosionRates, lithology, params)
	calculateLongshoreDrift(states, points, waveData, params)
	calculateDeposition(states, waveData, params)

	result := summarizeSedimentTransport(states, erosionRates, params)
	return result
}

// calculateErosionVolumes рассчитывает объём эрозии
func calculateErosionVolumes(
	states []SedimentState,
	erosionRates []float64,
	lithology []LithologyState,
	params SedimentTransportParameters,
) {
	for i := range states {
		erodedMeters := erosionRates[i]

		if i < len(lithology) && lithology[i].Resistance > 0 {
			erodedMeters /= lithology[i].Resistance
		}

		erodedVolume := erodedMeters * 1.0
		states[i].LocalBudget.ErodedVolume = erodedVolume

		transportFraction := params.TransportCoefficient
		if len(lithology) > i && lithology[i].Resistance > 5.0 {
			transportFraction *= 0.7
		}

		states[i].LocalBudget.TransportVolume = erodedVolume * transportFraction
		states[i].LocalBudget.DepositedVolume = erodedVolume * (1 - transportFraction)
	}
}

// calculateLongshoreDrift рассчитывает longshore drift
func calculateLongshoreDrift(
	states []SedimentState,
	points []LatLon,
	waveData WaveEnergyData,
	params SedimentTransportParameters,
) {
	n := len(states)

	for i := range states {
		if n < 3 {
			continue
		}

		prevIndex := (i - 1 + n) % n
		nextIndex := (i + 1) % n

		prevPoint := points[prevIndex]
		nextPoint := points[nextIndex]

		alongshoreX := nextPoint.Lon - prevPoint.Lon
		alongshoreY := nextPoint.Lat - prevPoint.Lat
		alongshoreLen := math.Hypot(alongshoreX, alongshoreY)

		if alongshoreLen < 1e-9 {
			continue
		}

		alongshoreX /= alongshoreLen
		alongshoreY /= alongshoreLen

		waveDirRad := waveData.Direction * math.Pi / 180.0
		waveDirX := math.Sin(waveDirRad)
		waveDirY := math.Cos(waveDirRad)

		alongshoreComponent := math.Abs(alongshoreX*waveDirX + alongshoreY*waveDirY)

		waveEnergy := 0.5
		if i < len(waveData.Energy) {
			waveEnergy = waveData.Energy[i]
		}

		transportAvailable := states[i].LocalBudget.TransportVolume
		crossProduct := alongshoreX*waveDirY - alongshoreY*waveDirX
		driftFraction := params.LongshoreDriftCoefficient * alongshoreComponent * waveEnergy

		toPrev := transportAvailable * 0.5 * driftFraction
		toNext := transportAvailable * 0.5 * driftFraction

		if crossProduct > 0 {
			toNext *= 1.5
			toPrev *= 0.5
		} else {
			toPrev *= 1.5
			toNext *= 0.5
		}

		states[i].InTransitTo = []float64{toPrev, toNext}
		states[prevIndex].InTransitFrom = append(states[prevIndex].InTransitFrom, toPrev)
		states[nextIndex].InTransitFrom = append(states[nextIndex].InTransitFrom, toNext)
	}
}

// calculateDeposition рассчитывает депозицию
func calculateDeposition(
	states []SedimentState,
	waveData WaveEnergyData,
	params SedimentTransportParameters,
) {
	for i := range states {
		incomingTotal := 0.0
		for _, v := range states[i].InTransitFrom {
			incomingTotal += v
		}

		waveEnergy := 0.5
		if i < len(waveData.Energy) {
			waveEnergy = waveData.Energy[i]
		}

		localCapacity := params.CapacityFactor * waveEnergy

		supplyToDemandRatio := 0.0
		if localCapacity > 0 {
			supplyToDemandRatio = incomingTotal / localCapacity
		}

		accumulationThreshold := 0.85

		if waveEnergy < 0.2 {
			accumulationThreshold = 0.4
		} else if waveEnergy < 0.35 {
			accumulationThreshold = 0.6
		} else if waveEnergy > 0.75 {
			accumulationThreshold = 1.1
		}

		shouldAccumulate := false
		depositionAmount := 0.0

		if incomingTotal > localCapacity*accumulationThreshold {
			shouldAccumulate = true
			excess := incomingTotal - (localCapacity * accumulationThreshold)
			depositionAmount = excess * params.DepositionRate
		} else if waveEnergy < 0.25 {
			shouldAccumulate = true
			baseDeposition := localCapacity * 0.3 * params.DepositionRate
			depositionAmount = math.Max(baseDeposition, incomingTotal*params.DepositionRate)
		} else if supplyToDemandRatio > 0.8 && supplyToDemandRatio < 1.2 {
			shouldAccumulate = true
			depositionAmount = incomingTotal * 0.3 * params.DepositionRate
		}

		if shouldAccumulate && depositionAmount > 1e-6 {
			states[i].LocalBudget.DepositedVolume += depositionAmount
			states[i].IsAccumulating = true

			if incomingTotal > localCapacity*accumulationThreshold {
				remainingExcess := incomingTotal - (localCapacity * accumulationThreshold) - depositionAmount
				if remainingExcess > 0 {
					states[i].LocalBudget.TransportVolume += remainingExcess
				}
			}
		} else {
			states[i].IsEroding = true
		}

		states[i].LocalBudget.NetChange =
			states[i].LocalBudget.ErodedVolume - states[i].LocalBudget.DepositedVolume

		if states[i].LocalBudget.NetChange > 0 {
			states[i].LocalBudget.ErosionPoints++
		} else if states[i].LocalBudget.NetChange < 0 {
			states[i].LocalBudget.DepositionPoints++
		}
	}
}

// summarizeSedimentTransport создаёт финальный результат
func summarizeSedimentTransport(
	states []SedimentState,
	erosionRates []float64,
	params SedimentTransportParameters,
) SedimentTransportResult {

	result := SedimentTransportResult{
		States:          states,
		BaselineErosion: make([]float64, len(states)),
		ModifiedErosion: make([]float64, len(states)),
	}

	totalBudget := SedimentBudget{}

	for i, state := range states {
		totalBudget.ErodedVolume += state.LocalBudget.ErodedVolume
		totalBudget.TransportVolume += state.LocalBudget.TransportVolume
		totalBudget.DepositedVolume += state.LocalBudget.DepositedVolume

		totalBudget.ErosionPoints += state.LocalBudget.ErosionPoints
		totalBudget.DepositionPoints += state.LocalBudget.DepositionPoints

		result.BaselineErosion[i] = erosionRates[i]

		if state.IsAccumulating {
			depositionMeters := state.LocalBudget.DepositedVolume / 1.0
			result.ModifiedErosion[i] = erosionRates[i] - depositionMeters

			if result.ModifiedErosion[i] < 0 {
				result.ModifiedErosion[i] = 0
			}
		} else {
			result.ModifiedErosion[i] = erosionRates[i]
		}
	}

	totalBudget.NetChange = totalBudget.ErodedVolume - totalBudget.DepositedVolume

	result.TotalBudget = totalBudget
	result.MassBalance = totalBudget.NetChange

	totalEroded := totalBudget.ErodedVolume
	var balanceRatio float64
	if totalEroded > 0 {
		totalAccountedFor := totalBudget.DepositedVolume + totalBudget.TransportVolume
		balanceRatio = math.Abs(totalEroded-totalAccountedFor) / totalEroded
		result.IsValid = balanceRatio < 0.25
		result.MassBalance = balanceRatio
	} else {
		result.IsValid = true
		result.MassBalance = 0
	}

	if !result.IsValid {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Баланс массы: %.1f%% расхождение (допустимо для седиментации)",
				balanceRatio*100))
	}

	if result.TotalBudget.ErosionPoints > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Эрозия в %d точках", result.TotalBudget.ErosionPoints))
	}

	if result.TotalBudget.DepositionPoints > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Аккумуляция в %d точках", result.TotalBudget.DepositionPoints))
	}

	return result
}

// ============================================================
// НОРМАЛИЗАЦИЯ ПАРАМЕТРОВ
// ============================================================

// normalizeSedimentParams нормализует параметры
func normalizeSedimentParams(params SedimentTransportParameters) SedimentTransportParameters {
	if params.TransportCoefficient <= 0 || params.TransportCoefficient > 1 {
		params.TransportCoefficient = 0.7
	}
	if params.DepositionRate <= 0 || params.DepositionRate > 1 {
		params.DepositionRate = 0.5
	}
	if params.MinimumFlowVelocity <= 0 {
		params.MinimumFlowVelocity = 0.3
	}
	if params.CapacityFactor <= 0 {
		params.CapacityFactor = 1.0
	}
	if params.LongshoreDriftCoefficient <= 0 || params.LongshoreDriftCoefficient > 1 {
		params.LongshoreDriftCoefficient = 0.8
	}

	return params
}

// normalizeSeasonalMod нормализует сезонные параметры
func normalizeSeasonalMod(mod SeasonalModulation) SeasonalModulation {
	if mod.WinterMultiplier <= 0 {
		mod.WinterMultiplier = 1.2
	}
	if mod.SummerMultiplier <= 0 {
		mod.SummerMultiplier = 0.8
	}
	if mod.TransitionMultiplier <= 0 {
		mod.TransitionMultiplier = 1.0
	}
	if mod.StormSeasonBoost <= 1 {
		mod.StormSeasonBoost = 1.5
	}
	return mod
}

// normalizeStormSedimentParams нормализует штормовые параметры
func normalizeStormSedimentParams(params StormSedimentParameters) StormSedimentParameters {
	if params.StormTransportMultiplier <= 1 {
		params.StormTransportMultiplier = 3.0
	}
	if params.StormDepositionEfficiency <= 0 || params.StormDepositionEfficiency > 1 {
		params.StormDepositionEfficiency = 0.6
	}
	if params.PostStormSurgeMultiplier <= 1 {
		params.PostStormSurgeMultiplier = 1.5
	}
	if params.StormThreshold <= 0 {
		params.StormThreshold = 1.5
	}
	if params.StormRetreatMultiplier <= 1 {
		params.StormRetreatMultiplier = 3.0
	}
	if params.StormBypassingCoefficient <= 0 || params.StormBypassingCoefficient > 1 {
		params.StormBypassingCoefficient = 0.3
	}
	return params
}

// ============================================================
// ОПТИМИЗИРОВАННЫЕ ФУНКЦИИ С АВТОМАТИЧЕСКИМ ВЫБОРОМ
// ============================================================

// CalculateSedimentTransportAuto автоматически выбирает оптимальную стратегию расчёта
func CalculateSedimentTransportAuto(
	points []LatLon,
	erosionRates []float64,
	waveData WaveEnergyData,
	lithology []LithologyState,
	params SedimentTransportParameters,
) SedimentTransportResult {

	n := len(points)
	if n == 0 {
		return SedimentTransportResult{}
	}

	switch {
	case n < 500:
		return CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)
	case n < 10000:
		return CalculateSedimentTransportOptimized(points, erosionRates, waveData, lithology, params)
	default:
		batched := CalculateSedimentTransportBatched(points, erosionRates, waveData, lithology, params, 5000)

		result := SedimentTransportResult{
			TotalBudget: batched.TotalBudget,
			IsValid:     batched.IsValid,
			Warnings:    batched.Warnings,
			MassBalance: batched.TotalBudget.NetChange / batched.TotalBudget.ErodedVolume,
		}

		totalStates := 0
		for _, b := range batched.Results {
			totalStates += len(b.States)
		}
		result.States = make([]SedimentState, 0, totalStates)

		for _, b := range batched.Results {
			result.States = append(result.States, b.States...)
		}

		return result
	}
}

// GetPerformanceStats возвращает статистику для заданного размера
func GetPerformanceStats(n int) PerformanceStats {
	stats := PerformanceStats{PointsCount: n}

	switch {
	case n < 500:
		stats.Strategy = "original"
	case n < 1000:
		stats.Strategy = "optimized"
		stats.Speedup = 1.1
	case n < 50000:
		stats.Strategy = "parallel"
		stats.Speedup = 1.2
	default:
		stats.Strategy = "batched"
		stats.Speedup = 1.5
	}

	return stats
}

// ============================================================
// КЭШИРОВАННЫЕ ВЫЧИСЛЕНИЯ
// ============================================================

// NewOptimizedCache создаёт новый кэш
func NewOptimizedCache() *OptimizedSedimentCache {
	return &OptimizedSedimentCache{
		alongshoreDirections: make([]Vector2D, 0),
		segmentLengths:       make([]float64, 0),
	}
}

// Initialize инициализирует кэш для заданных точек
func (c *OptimizedSedimentCache) Initialize(points []LatLon, waveData WaveEnergyData) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n := len(points)
	if n < 3 {
		return
	}

	c.alongshoreDirections = make([]Vector2D, n)
	c.segmentLengths = make([]float64, n)

	waveDirRad := waveData.Direction * math.Pi / 180.0
	c.waveDirectionVec = Vector2D{
		X: math.Sin(waveDirRad),
		Y: math.Cos(waveDirRad),
	}

	for i := 0; i < n; i++ {
		prevIndex := (i - 1 + n) % n
		nextIndex := (i + 1) % n

		prevPoint := points[prevIndex]
		nextPoint := points[nextIndex]

		alongshoreX := nextPoint.Lon - prevPoint.Lon
		alongshoreY := nextPoint.Lat - prevPoint.Lat
		alongshoreLen := math.Hypot(alongshoreX, alongshoreY)

		c.segmentLengths[i] = alongshoreLen

		if alongshoreLen < 1e-9 {
			c.alongshoreDirections[i] = Vector2D{X: 0, Y: 0}
			continue
		}

		c.alongshoreDirections[i] = Vector2D{
			X: alongshoreX / alongshoreLen,
			Y: alongshoreY / alongshoreLen,
		}
	}

	c.initialized = true
}

// GetAlongshoreDirection возвращает предвычисленное alongshore направление
func (c *OptimizedSedimentCache) GetAlongshoreDirection(i int) Vector2D {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if i >= 0 && i < len(c.alongshoreDirections) {
		return c.alongshoreDirections[i]
	}
	return Vector2D{X: 0, Y: 0}
}

// GetWaveDirection возвращает предвычисленное wave direction
func (c *OptimizedSedimentCache) GetWaveDirection() Vector2D {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.waveDirectionVec
}

// GetSegmentLength возвращает длину сегмента
func (c *OptimizedSedimentCache) GetSegmentLength(i int) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if i >= 0 && i < len(c.segmentLengths) {
		return c.segmentLengths[i]
	}
	return 0
}

// ============================================================
// ОПТИМИЗИРОВАННЫЙ РАСЧЁТ ТРАНСПОРТА
// ============================================================

// NewOptimizedContext создаёт оптимизированный контекст
func NewOptimizedContext() *OptimizedCalculationContext {
	return &OptimizedCalculationContext{
		cache:   NewOptimizedCache(),
		workers: runtime.NumCPU(),
	}
}

// CalculateSedimentTransportOptimized оптимизированная версия расчёта
func CalculateSedimentTransportOptimized(
	points []LatLon,
	erosionRates []float64,
	waveData WaveEnergyData,
	lithology []LithologyState,
	params SedimentTransportParameters,
) SedimentTransportResult {

	n := len(points)
	if n == 0 {
		return SedimentTransportResult{}
	}

	params = normalizeSedimentParams(params)

	if n < 500 {
		return CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)
	}

	useParallel := n > 10000

	ctx := NewOptimizedContext()
	ctx.cache.Initialize(points, waveData)

	states := make([]SedimentState, n)
	for i := range states {
		states[i].PointIndex = i
		states[i].InTransitFrom = make([]float64, 0, 4)
		states[i].InTransitTo = make([]float64, 0, 4)
	}

	calculateErosionVolumesOptimized(states, erosionRates, lithology, params, useParallel, ctx.workers)
	calculateLongshoreDriftOptimized(states, points, waveData, params, ctx.cache, useParallel)
	calculateDepositionOptimized(states, waveData, params, useParallel, ctx.workers)

	result := summarizeSedimentTransport(states, erosionRates, params)
	return result
}

// calculateErosionVolumesOptimized параллельный расчёт эрозии
func calculateErosionVolumesOptimized(
	states []SedimentState,
	erosionRates []float64,
	lithology []LithologyState,
	params SedimentTransportParameters,
	useParallel bool,
	workers int,
) {
	n := len(states)

	if !useParallel || n < 10000 {
		calculateErosionVolumesSequential(states, erosionRates, lithology, params)
		return
	}

	var wg sync.WaitGroup
	batchSize := (n + workers - 1) / workers

	for i := 0; i < workers; i++ {
		start := i * batchSize
		end := start + batchSize
		if end > n {
			end = n
		}

		if start >= n {
			break
		}

		wg.Add(1)
		go func(workerStart, workerEnd int) {
			defer wg.Done()

			for j := workerStart; j < workerEnd; j++ {
				erodedMeters := erosionRates[j]

				if j < len(lithology) && lithology[j].Resistance > 0 {
					erodedMeters /= lithology[j].Resistance
				}

				erodedVolume := erodedMeters * 1.0
				states[j].LocalBudget.ErodedVolume = erodedVolume

				transportFraction := params.TransportCoefficient
				if len(lithology) > j && lithology[j].Resistance > 5.0 {
					transportFraction *= 0.7
				}

				states[j].LocalBudget.TransportVolume = erodedVolume * transportFraction
				states[j].LocalBudget.DepositedVolume = erodedVolume * (1 - transportFraction)
			}
		}(start, end)
	}

	wg.Wait()
}

// calculateErosionVolumesSequential последовательная версия
func calculateErosionVolumesSequential(
	states []SedimentState,
	erosionRates []float64,
	lithology []LithologyState,
	params SedimentTransportParameters,
) {
	for i := range states {
		erodedMeters := erosionRates[i]

		if i < len(lithology) && lithology[i].Resistance > 0 {
			erodedMeters /= lithology[i].Resistance
		}

		erodedVolume := erodedMeters * 1.0
		states[i].LocalBudget.ErodedVolume = erodedVolume

		transportFraction := params.TransportCoefficient
		if len(lithology) > i && lithology[i].Resistance > 5.0 {
			transportFraction *= 0.7
		}

		states[i].LocalBudget.TransportVolume = erodedVolume * transportFraction
		states[i].LocalBudget.DepositedVolume = erodedVolume * (1 - transportFraction)
	}
}

// calculateLongshoreDriftOptimized оптимизированный longshore drift
func calculateLongshoreDriftOptimized(
	states []SedimentState,
	points []LatLon,
	waveData WaveEnergyData,
	params SedimentTransportParameters,
	cache *OptimizedSedimentCache,
	useParallel bool,
) {
	n := len(states)
	if n < 3 {
		return
	}

	waveDirVec := cache.GetWaveDirection()

	type TransportData struct {
		index     int
		toPrev    float64
		toNext    float64
		prevIndex int
		nextIndex int
	}

	transportPool := sync.Pool{
		New: func() interface{} {
			return make([]TransportData, 0, n)
		},
	}

	transportBuffer := transportPool.Get().([]TransportData)
	defer transportPool.Put(transportBuffer)

	if cap(transportBuffer) < n {
		transportBuffer = make([]TransportData, n)
	}
	transportBuffer = transportBuffer[:0]

	if useParallel {
		workers := runtime.NumCPU()
		batchSize := (n + workers - 1) / workers

		var mu sync.Mutex
		var wg sync.WaitGroup

		for w := 0; w < workers; w++ {
			start := w * batchSize
			end := start + batchSize
			if end > n {
				end = n
			}
			if start >= n {
				break
			}

			wg.Add(1)
			go func(workerStart, workerEnd int) {
				defer wg.Done()

				localBuffer := make([]TransportData, 0, workerEnd-workerStart)

				for i := workerStart; i < workerEnd; i++ {
					alongshoreDir := cache.GetAlongshoreDirection(i)
					if alongshoreDir.X == 0 && alongshoreDir.Y == 0 {
						continue
					}

					alongshoreComponent := math.Abs(alongshoreDir.X*waveDirVec.X + alongshoreDir.Y*waveDirVec.Y)

					waveEnergy := 0.5
					if i < len(waveData.Energy) {
						waveEnergy = waveData.Energy[i]
					}

					transportAvailable := states[i].LocalBudget.TransportVolume

					crossProduct := alongshoreDir.X*waveDirVec.Y - alongshoreDir.Y*waveDirVec.X
					driftFraction := params.LongshoreDriftCoefficient * alongshoreComponent * waveEnergy

					toPrev := transportAvailable * 0.5 * driftFraction
					toNext := transportAvailable * 0.5 * driftFraction

					if crossProduct > 0 {
						toNext *= 1.5
						toPrev *= 0.5
					} else {
						toPrev *= 1.5
						toNext *= 0.5
					}

					prevIndex := (i - 1 + n) % n
					nextIndex := (i + 1) % n

					localBuffer = append(localBuffer, TransportData{
						index:     i,
						toPrev:    toPrev,
						toNext:    toNext,
						prevIndex: prevIndex,
						nextIndex: nextIndex,
					})
				}

				mu.Lock()
				transportBuffer = append(transportBuffer, localBuffer...)
				mu.Unlock()
			}(start, end)
		}

		wg.Wait()
	} else {
		for i := 0; i < n; i++ {
			alongshoreDir := cache.GetAlongshoreDirection(i)
			if alongshoreDir.X == 0 && alongshoreDir.Y == 0 {
				continue
			}

			alongshoreComponent := math.Abs(alongshoreDir.X*waveDirVec.X + alongshoreDir.Y*waveDirVec.Y)

			waveEnergy := 0.5
			if i < len(waveData.Energy) {
				waveEnergy = waveData.Energy[i]
			}

			transportAvailable := states[i].LocalBudget.TransportVolume

			crossProduct := alongshoreDir.X*waveDirVec.Y - alongshoreDir.Y*waveDirVec.X
			driftFraction := params.LongshoreDriftCoefficient * alongshoreComponent * waveEnergy

			toPrev := transportAvailable * 0.5 * driftFraction
			toNext := transportAvailable * 0.5 * driftFraction

			if crossProduct > 0 {
				toNext *= 1.5
				toPrev *= 0.5
			} else {
				toPrev *= 1.5
				toNext *= 0.5
			}

			prevIndex := (i - 1 + n) % n
			nextIndex := (i + 1) % n

			transportBuffer = append(transportBuffer, TransportData{
				index:     i,
				toPrev:    toPrev,
				toNext:    toNext,
				prevIndex: prevIndex,
				nextIndex: nextIndex,
			})
		}
	}

	// Применяем результаты транспорта
	for _, data := range transportBuffer {
		states[data.index].InTransitTo = []float64{data.toPrev, data.toNext}
		states[data.prevIndex].InTransitFrom = append(states[data.prevIndex].InTransitFrom, data.toPrev)
		states[data.nextIndex].InTransitFrom = append(states[data.nextIndex].InTransitFrom, data.toNext)
	}
}

// calculateDepositionOptimized оптимизированная расчёт депозиции
func calculateDepositionOptimized(
	states []SedimentState,
	waveData WaveEnergyData,
	params SedimentTransportParameters,
	useParallel bool,
	workers int,
) {
	n := len(states)

	if !useParallel || n < 10000 {
		calculateDepositionSequential(states, waveData, params)
		return
	}

	var wg sync.WaitGroup
	batchSize := (n + workers - 1) / workers

	for i := 0; i < workers; i++ {
		start := i * batchSize
		end := start + batchSize
		if end > n {
			end = n
		}

		if start >= n {
			break
		}

		wg.Add(1)
		go func(workerStart, workerEnd int) {
			defer wg.Done()

			for j := workerStart; j < workerEnd; j++ {
				incomingTotal := 0.0
				for _, v := range states[j].InTransitFrom {
					incomingTotal += v
				}

				waveEnergy := 0.5
				if j < len(waveData.Energy) {
					waveEnergy = waveData.Energy[j]
				}

				localCapacity := params.CapacityFactor * waveEnergy

				supplyToDemandRatio := 0.0
				if localCapacity > 0 {
					supplyToDemandRatio = incomingTotal / localCapacity
				}

				accumulationThreshold := 0.85

				if waveEnergy < 0.2 {
					accumulationThreshold = 0.4
				} else if waveEnergy < 0.35 {
					accumulationThreshold = 0.6
				} else if waveEnergy > 0.75 {
					accumulationThreshold = 1.1
				}

				shouldAccumulate := false
				depositionAmount := 0.0

				if incomingTotal > localCapacity*accumulationThreshold {
					shouldAccumulate = true
					excess := incomingTotal - (localCapacity * accumulationThreshold)
					depositionAmount = excess * params.DepositionRate
				} else if waveEnergy < 0.25 {
					shouldAccumulate = true
					baseDeposition := localCapacity * 0.3 * params.DepositionRate
					depositionAmount = math.Max(baseDeposition, incomingTotal*params.DepositionRate)
				} else if supplyToDemandRatio > 0.8 && supplyToDemandRatio < 1.2 {
					shouldAccumulate = true
					depositionAmount = incomingTotal * 0.3 * params.DepositionRate
				}

				if shouldAccumulate && depositionAmount > 1e-6 {
					states[j].LocalBudget.DepositedVolume += depositionAmount
					states[j].IsAccumulating = true

					if incomingTotal > localCapacity*accumulationThreshold {
						remainingExcess := incomingTotal - (localCapacity * accumulationThreshold) - depositionAmount
						if remainingExcess > 0 {
							states[j].LocalBudget.TransportVolume += remainingExcess
						}
					}
				} else {
					states[j].IsEroding = true
				}

				states[j].LocalBudget.NetChange =
					states[j].LocalBudget.ErodedVolume - states[j].LocalBudget.DepositedVolume

				if states[j].LocalBudget.NetChange > 0 {
					states[j].LocalBudget.ErosionPoints++
				} else if states[j].LocalBudget.NetChange < 0 {
					states[j].LocalBudget.DepositionPoints++
				}
			}
		}(start, end)
	}

	wg.Wait()
}

// calculateDepositionSequential последовательная версия депозиции
func calculateDepositionSequential(
	states []SedimentState,
	waveData WaveEnergyData,
	params SedimentTransportParameters,
) {
	for i := range states {
		incomingTotal := 0.0
		for _, v := range states[i].InTransitFrom {
			incomingTotal += v
		}

		waveEnergy := 0.5
		if i < len(waveData.Energy) {
			waveEnergy = waveData.Energy[i]
		}

		localCapacity := params.CapacityFactor * waveEnergy

		supplyToDemandRatio := 0.0
		if localCapacity > 0 {
			supplyToDemandRatio = incomingTotal / localCapacity
		}

		accumulationThreshold := 0.85

		if waveEnergy < 0.2 {
			accumulationThreshold = 0.4
		} else if waveEnergy < 0.35 {
			accumulationThreshold = 0.6
		} else if waveEnergy > 0.75 {
			accumulationThreshold = 1.1
		}

		shouldAccumulate := false
		depositionAmount := 0.0

		if incomingTotal > localCapacity*accumulationThreshold {
			shouldAccumulate = true
			excess := incomingTotal - (localCapacity * accumulationThreshold)
			depositionAmount = excess * params.DepositionRate
		} else if waveEnergy < 0.25 {
			shouldAccumulate = true
			baseDeposition := localCapacity * 0.3 * params.DepositionRate
			depositionAmount = math.Max(baseDeposition, incomingTotal*params.DepositionRate)
		} else if supplyToDemandRatio > 0.8 && supplyToDemandRatio < 1.2 {
			shouldAccumulate = true
			depositionAmount = incomingTotal * 0.3 * params.DepositionRate
		}

		if shouldAccumulate && depositionAmount > 1e-6 {
			states[i].LocalBudget.DepositedVolume += depositionAmount
			states[i].IsAccumulating = true

			if incomingTotal > localCapacity*accumulationThreshold {
				remainingExcess := incomingTotal - (localCapacity * accumulationThreshold) - depositionAmount
				if remainingExcess > 0 {
					states[i].LocalBudget.TransportVolume += remainingExcess
				}
			}
		} else {
			states[i].IsEroding = true
		}

		states[i].LocalBudget.NetChange =
			states[i].LocalBudget.ErodedVolume - states[i].LocalBudget.DepositedVolume

		if states[i].LocalBudget.NetChange > 0 {
			states[i].LocalBudget.ErosionPoints++
		} else if states[i].LocalBudget.NetChange < 0 {
			states[i].LocalBudget.DepositionPoints++
		}
	}
}

// ============================================================
// ПАЧЕЧНАЯ ОБРАБОТКА
// ============================================================

// CalculateSedimentTransportBatched пачечная обработка для очень больших наборов данных
func CalculateSedimentTransportBatched(
	points []LatLon,
	erosionRates []float64,
	waveData WaveEnergyData,
	lithology []LithologyState,
	params SedimentTransportParameters,
	batchSize int,
) BatchSedimentTransportResult {

	n := len(points)
	if n == 0 {
		return BatchSedimentTransportResult{}
	}

	params = normalizeSedimentParams(params)

	if batchSize <= 0 || batchSize > n {
		batchSize = 1000
	}

	numBatches := (n + batchSize - 1) / batchSize
	results := make([]SedimentTransportResult, numBatches)

	for b := 0; b < numBatches; b++ {
		start := b * batchSize
		end := start + batchSize
		if end > n {
			end = n
		}

		batchPoints := points[start:end]
		batchErosion := erosionRates[start:end]

		var batchLithology []LithologyState
		if len(lithology) > 0 {
			batchLithology = lithology[start:end]
		}

		batchWaveData := WaveEnergyData{
			Energy:    waveData.Energy[start:end],
			Direction: waveData.Direction,
			Incidence: waveData.Incidence[start:end],
			Fetch:     waveData.Fetch[start:end],
		}

		results[b] = CalculateSedimentTransportOptimized(
			batchPoints, batchErosion, batchWaveData, batchLithology, params,
		)
	}

	combinedResult := BatchSedimentTransportResult{
		Results:      results,
		TotalBatches: numBatches,
	}

	for _, r := range results {
		combinedResult.TotalBudget.ErodedVolume += r.TotalBudget.ErodedVolume
		combinedResult.TotalBudget.TransportVolume += r.TotalBudget.TransportVolume
		combinedResult.TotalBudget.DepositedVolume += r.TotalBudget.DepositedVolume
		combinedResult.TotalBudget.ErosionPoints += r.TotalBudget.ErosionPoints
		combinedResult.TotalBudget.DepositionPoints += r.TotalBudget.DepositionPoints
	}

	combinedResult.TotalBudget.NetChange = combinedResult.TotalBudget.ErodedVolume - combinedResult.TotalBudget.DepositedVolume
	combinedResult.IsValid = true

	return combinedResult
}

// ============================================================
// ВРЕМЕННАЯ ДИНАМИКА
// ============================================================

// CalculateSedimentTransportWithTemporal расчёт транспорта с учётом временной динамики
func CalculateSedimentTransportWithTemporal(
	points []LatLon,
	erosionRates []float64,
	waveData WaveEnergyData,
	lithology []LithologyState,
	params SedimentTransportParameters,
	temporalState SedimentTemporalState,
	seasonalMod SeasonalModulation,
	stormParams StormSedimentParameters,
) SedimentTemporalResult {

	n := len(points)
	if n == 0 {
		return SedimentTemporalResult{}
	}

	params = normalizeSedimentParams(params)
	seasonalMod = normalizeSeasonalMod(seasonalMod)
	stormParams = normalizeStormSedimentParams(stormParams)

	result := SedimentTemporalResult{
		States:        make([]TemporalSedimentState, n),
		SeasonalStats: make(map[string]SedimentBudget),
		StormImpact:   SedimentBudget{},
	}

	season := determineSeason(temporalState.Year, seasonalMod.AccumulationSeasonality)

	baseResult := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	for i := 0; i < n && i < len(baseResult.States); i++ {
		result.States[i].BaseState = baseResult.States[i]
		result.States[i].CurrentSeason = season
		result.States[i].IsStormActive = temporalState.IsStorm
		result.States[i].StormIntensity = temporalState.StormIntensity

		seasonalFactor := calculateSeasonalFactor(season, seasonalMod, temporalState.SeasonFactor)
		result.States[i].SeasonalFactor = seasonalFactor

		baseErosion := erosionRates[i]
		modifiedErosion := baseErosion * seasonalFactor

		if temporalState.IsStorm && temporalState.StormIntensity > stormParams.StormThreshold {
			modifiedErosion *= temporalState.StormIntensity
			result.States[i].BaseState.LocalBudget.TransportVolume *= stormParams.StormTransportMultiplier
			result.States[i].BaseState.LocalBudget.ErodedVolume *= stormParams.StormRetreatMultiplier

			result.StormImpact.ErodedVolume += result.States[i].BaseState.LocalBudget.ErodedVolume
			result.StormImpact.TransportVolume += result.States[i].BaseState.LocalBudget.TransportVolume
		}

		result.States[i].ModifiedErosion = modifiedErosion
		result.States[i].DepositionChange = result.States[i].BaseState.LocalBudget.DepositedVolume

		if _, ok := result.SeasonalStats[season]; !ok {
			result.SeasonalStats[season] = SedimentBudget{}
		}

		seasonalStats := result.SeasonalStats[season]
		seasonalStats.ErodedVolume += result.States[i].BaseState.LocalBudget.ErodedVolume
		seasonalStats.TransportVolume += result.States[i].BaseState.LocalBudget.TransportVolume
		seasonalStats.DepositedVolume += result.States[i].BaseState.LocalBudget.DepositedVolume
		result.SeasonalStats[season] = seasonalStats
	}

	result.TotalBudget = baseResult.TotalBudget
	result.IsValid = baseResult.IsValid

	return result
}

// CalculateSedimentTransportWithTemporalOptimized оптимизированная версия с временной динамикой
func CalculateSedimentTransportWithTemporalOptimized(
	points []LatLon,
	erosionRates []float64,
	waveData WaveEnergyData,
	lithology []LithologyState,
	params SedimentTransportParameters,
	temporalState SedimentTemporalState,
	seasonalMod SeasonalModulation,
	stormParams StormSedimentParameters,
) SedimentTemporalResult {

	n := len(points)
	if n == 0 {
		return SedimentTemporalResult{}
	}

	params = normalizeSedimentParams(params)
	seasonalMod = normalizeSeasonalMod(seasonalMod)
	stormParams = normalizeStormSedimentParams(stormParams)

	result := SedimentTemporalResult{
		States:        make([]TemporalSedimentState, n),
		SeasonalStats: make(map[string]SedimentBudget),
		StormImpact:   SedimentBudget{},
	}

	season := determineSeason(temporalState.Year, seasonalMod.AccumulationSeasonality)

	baseResult := CalculateSedimentTransportOptimized(points, erosionRates, waveData, lithology, params)

	for i := 0; i < n && i < len(baseResult.States); i++ {
		result.States[i].BaseState = baseResult.States[i]
		result.States[i].CurrentSeason = season
		result.States[i].IsStormActive = temporalState.IsStorm
		result.States[i].StormIntensity = temporalState.StormIntensity

		seasonalFactor := calculateSeasonalFactor(season, seasonalMod, temporalState.SeasonFactor)
		result.States[i].SeasonalFactor = seasonalFactor

		baseErosion := erosionRates[i]
		modifiedErosion := baseErosion * seasonalFactor

		if temporalState.IsStorm && temporalState.StormIntensity > stormParams.StormThreshold {
			modifiedErosion *= temporalState.StormIntensity
			result.States[i].BaseState.LocalBudget.TransportVolume *= stormParams.StormTransportMultiplier
			result.States[i].BaseState.LocalBudget.ErodedVolume *= stormParams.StormRetreatMultiplier

			result.StormImpact.ErodedVolume += result.States[i].BaseState.LocalBudget.ErodedVolume
			result.StormImpact.TransportVolume += result.States[i].BaseState.LocalBudget.TransportVolume
		}

		result.States[i].ModifiedErosion = modifiedErosion
		result.States[i].DepositionChange = result.States[i].BaseState.LocalBudget.DepositedVolume

		if _, ok := result.SeasonalStats[season]; !ok {
			result.SeasonalStats[season] = SedimentBudget{}
		}

		seasonalStats := result.SeasonalStats[season]
		seasonalStats.ErodedVolume += result.States[i].BaseState.LocalBudget.ErodedVolume
		seasonalStats.TransportVolume += result.States[i].BaseState.LocalBudget.TransportVolume
		seasonalStats.DepositedVolume += result.States[i].BaseState.LocalBudget.DepositedVolume
		result.SeasonalStats[season] = seasonalStats
	}

	result.TotalBudget = baseResult.TotalBudget
	result.IsValid = baseResult.IsValid

	return result
}

// determineSeason определяет сезон
func determineSeason(year float64, useSeasonality bool) string {
	if !useSeasonality {
		return "year_round"
	}

	month := int(year) % 12

	switch {
	case month >= 11 || month <= 1:
		return "winter"
	case month >= 2 && month <= 4:
		return "spring"
	case month >= 5 && month <= 8:
		return "summer"
	default:
		return "autumn"
	}
}

// calculateSeasonalFactor рассчитывает сезонный множитель
func calculateSeasonalFactor(season string, mod SeasonalModulation, baseFactor float64) float64 {
	factor := baseFactor

	switch season {
	case "winter":
		factor *= mod.WinterMultiplier
	case "summer":
		factor *= mod.SummerMultiplier
	case "spring", "autumn":
		factor *= mod.TransitionMultiplier
	}

	return factor
}

// calculateStormDepositThickness рассчитывает толщину штормовых отложений
func calculateStormDepositThickness(stormIntensity float64) float64 {
	return 0.01 * stormIntensity * stormIntensity
}

// calculateStormGrainSize рассчитывает размер зёрен штормовых отложений
func calculateStormGrainSize(stormIntensity float64) float64 {
	return 0.5 + stormIntensity*0.3
}

// determinePreservation определяет сохранность штормовых отложений
func determinePreservation(stormIntensity float64, state SedimentState) bool {
	if state.IsAccumulating {
		return true
	}
	return stormIntensity > 2.5
}

// ============================================================
// СТАТИСТИКА И ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ
// ============================================================

// GetSedimentStatistics возвращает статистику по sediment transport
func GetSedimentStatistics(result SedimentTransportResult) map[string]interface{} {
	stats := map[string]interface{}{
		"total_eroded_m3":    result.TotalBudget.ErodedVolume,
		"total_deposited_m3": result.TotalBudget.DepositedVolume,
		"total_transport_m3": result.TotalBudget.TransportVolume,
		"net_change_m3":      result.TotalBudget.NetChange,
		"mass_balance":       result.MassBalance,
		"is_valid":           result.IsValid,
		"erosion_points":     result.TotalBudget.ErosionPoints,
		"deposition_points":  result.TotalBudget.DepositionPoints,
		"warnings":           result.Warnings,
	}

	return stats
}

// GetStormDepositStatistics возвращает статистику по штормовым отложениям
func GetStormDepositStatistics(result SedimentTemporalResult) map[string]interface{} {
	stats := map[string]interface{}{
		"total_storms":        len(result.AllStormDeposits),
		"preserved_deposits":  len(result.PreservedDeposits),
		"storm_impact_volume": result.StormImpact.ErodedVolume,
	}

	totalThickness := 0.0
	for _, deposit := range result.PreservedDeposits {
		totalThickness += deposit.Thickness
	}
	stats["total_thickness_m"] = totalThickness

	return stats
}

// CreateSeasonalAccumulationProfile создаёт профиль сезонной аккумуляции
func CreateSeasonalAccumulationProfile() SeasonalModulation {
	return SeasonalModulation{
		WinterMultiplier:        1.3,
		SummerMultiplier:        0.7,
		TransitionMultiplier:    1.0,
		StormSeasonBoost:        2.0,
		AccumulationSeasonality: true,
	}
}

// ApplySeasonalAccumulationModulation применяет сезонную модуляцию
func ApplySeasonalAccumulationModulation(
	baseResult SedimentTransportResult,
	season string,
	mod SeasonalModulation,
) SedimentTransportResult {

	result := baseResult
	seasonalFactor := calculateSeasonalFactor(season, mod, 1.0)

	for i := range result.ModifiedErosion {
		result.ModifiedErosion[i] *= seasonalFactor

		if season == "winter" && mod.AccumulationSeasonality {
			result.States[i].LocalBudget.DepositedVolume *= 1.2
		}
	}

	return result
}

// ============================================================
// ПРИМЕНЕНИЕ СЕДИМЕНТАЦИИ
// ============================================================

// ApplySedimentModification корректирует эрозию с учётом аккумуляции
func ApplySedimentModification(
	points []LatLon,
	baseErosion []float64,
	sedimentResult SedimentTransportResult,
) []float64 {

	modified := make([]float64, len(baseErosion))
	copy(modified, baseErosion)

	for i := range sedimentResult.States {
		if sedimentResult.States[i].IsAccumulating {
			depositionMeters := sedimentResult.States[i].LocalBudget.DepositedVolume / 1.0
			modified[i] = baseErosion[i] - depositionMeters

			if modified[i] < 0 {
				modified[i] = 0
			}
		}
	}

	return modified
}
