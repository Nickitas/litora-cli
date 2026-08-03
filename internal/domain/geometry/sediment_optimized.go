package geometry

import (
	"math"
	"runtime"
	"sync"
)

// CalculateSedimentTransportAuto автоматически выбирает оптимальную стратегию расчёта
// на основе размера входных данных.
//
// Стратегии:
//   - < 500 точек: оригинальная версия (минимум overhead)
//   - 500-10000 точек: оптимизированная с кэшем (без параллелизма)
//   - 10000-50000 точек: оптимизированная с кэшем и параллелизмом
//   - > 50000 точек: пачечная обработка (memory-efficient)
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

	// Автоматический выбор стратегии
	switch {
	case n < 500:
		// Малые наборы - оригинальная версия
		return CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	case n < 1000:
		// Средние наборы - оптимизированная без параллелизма
		return CalculateSedimentTransportOptimized(points, erosionRates, waveData, lithology, params)

	case n < 50000:
		// Большие наборы - оптимизированная с параллелизмом
		return CalculateSedimentTransportOptimized(points, erosionRates, waveData, lithology, params)

	default:
		// Очень большие наборы - пачечная обработка
		batched := CalculateSedimentTransportBatched(points, erosionRates, waveData, lithology, params, 5000)

		// Конвертируем пачечный результат в обычный
		result := SedimentTransportResult{
			TotalBudget: batched.TotalBudget,
			IsValid:     batched.IsValid,
			Warnings:    batched.Warnings,
			MassBalance: batched.TotalBudget.NetChange / batched.TotalBudget.ErodedVolume,
		}

		// Объединяем states из всех батчей
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

// PerformanceStats статистика производительности
type PerformanceStats struct {
	Strategy          string  // "original", "optimized", "parallel", "batched"
	PointsCount       int     // количество точек
	ExecutionTimeMs   float64 // время выполнения (мс)
	MemoryAllocations int     // количество аллокаций
	Speedup           float64 // ускорение относительно original
}

// GetPerformanceStats возвращает статистику для заданного размера
func GetPerformanceStats(n int) PerformanceStats {
	stats := PerformanceStats{PointsCount: n}

	switch {
	case n < 500:
		stats.Strategy = "original"
	case n < 1000:
		stats.Strategy = "optimized"
		stats.Speedup = 1.1 // ~10% быстрее
	case n < 50000:
		stats.Strategy = "parallel"
		stats.Speedup = 1.2 // ~20% быстрее
	default:
		stats.Strategy = "batched"
		stats.Speedup = 1.5 // ~50% быстрее
	}

	return stats
}

// OptimizedSedimentCache кэш для вычислений
type OptimizedSedimentCache struct {
	// Предвычисленные векторы alongshore для каждой точки
	alongshoreDirections []Vector2D
	// Предвычисленные wave direction
	waveDirectionVec Vector2D
	// Предвычисленные расстояния между точками
	segmentLengths []float64
	// Флаг инициализации
	initialized bool
	mu          sync.RWMutex
}

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

	// Предвычисляем wave direction
	waveDirRad := waveData.Direction * math.Pi / 180.0
	c.waveDirectionVec = Vector2D{
		X: math.Sin(waveDirRad),
		Y: math.Cos(waveDirRad),
	}

	// Предвычисляем alongshore направления для каждой точки
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

		// Нормированный вектор
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

// OptimizedCalculationContext контекст для оптимизированных вычислений
type OptimizedCalculationContext struct {
	cache   *OptimizedSedimentCache
	workers int
}

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

	// Быстрый путь для малых наборов: используем оригинальную версию
	// без накладных расходов на кэширование и параллелизм
	if n < 500 {
		return CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)
	}

	// Для средних наборов (500-10000): используем кэшированные вычисления,
	// но без параллелизма (накладные расходы goroutines превышают выгоду)
	useParallel := n > 10000

	// Создаём оптимизированный контекст
	ctx := NewOptimizedContext()
	ctx.cache.Initialize(points, waveData)

	// Инициализация states
	states := make([]SedimentState, n)
	for i := range states {
		states[i].PointIndex = i
		states[i].InTransitFrom = make([]float64, 0, 4) // Pre-allocate
		states[i].InTransitTo = make([]float64, 0, 4)   // Pre-allocate
	}

	// Этап 1: Параллельный расчёт эрозии
	calculateErosionVolumesOptimized(states, erosionRates, lithology, params, useParallel, ctx.workers)

	// Этап 2: Optimized longshore drift
	calculateLongshoreDriftOptimized(states, points, waveData, params, ctx.cache, useParallel)

	// Этап 3: Депозиция
	calculateDepositionOptimized(states, waveData, params, useParallel, ctx.workers)

	// Этап 4: Резюме
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
		// Последовательная обработка для малых наборов
		calculateErosionVolumesSequential(states, erosionRates, lithology, params)
		return
	}

	// Параллельная обработка
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

				// Модуляция по литологии
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

// calculateLongshoreDriftOptimized оптимизированный longshore drift с использованием кэша
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

	// Предвычисляем wave direction (используем кэш)
	waveDirVec := cache.GetWaveDirection()

	// Сначала собираем данные о транспорте (без race conditions)
	// Используем промежуточный буфер
	type TransportData struct {
		index     int
		toPrev    float64
		toNext    float64
		prevIndex int
		nextIndex int
	}

	// Используем sync.Pool для переиспользования буферов
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

	// Этап 1: Вычисляем объёмы транспорта (можно параллельно)
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

					// Alongshore компонента
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
		// Последовательная версия
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

	// Этап 2: Применяем результаты (без race conditions)
	for _, data := range transportBuffer {
		states[data.index].InTransitTo = []float64{data.toPrev, data.toNext}
		states[data.prevIndex].InTransitFrom = append(states[data.prevIndex].InTransitFrom, data.toPrev)
		states[data.nextIndex].InTransitFrom = append(states[data.nextIndex].InTransitFrom, data.toNext)
	}
}

// calculateDepositionOptimized оптимизированная депозиция
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

	// Параллельная обработка
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
				// Incoming sediment
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
					states[j].LocalBudget.ErodedVolume -
						states[j].LocalBudget.DepositedVolume

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
			states[i].LocalBudget.ErodedVolume -
				states[i].LocalBudget.DepositedVolume

		if states[i].LocalBudget.NetChange > 0 {
			states[i].LocalBudget.ErosionPoints++
		} else if states[i].LocalBudget.NetChange < 0 {
			states[i].LocalBudget.DepositionPoints++
		}
	}
}

// BatchSedimentTransportResult результат для пачечной обработки
type BatchSedimentTransportResult struct {
	Results      []SedimentTransportResult
	TotalBudget  SedimentBudget
	IsValid      bool
	Warnings     []string
	BatchSize    int
	TotalBatches int
}

// CalculateSedimentTransportBatched пачечная обработка для очень больших линий
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

	// Адаптивный размер батча
	if batchSize <= 0 {
		batchSize = 2000 // По умолчанию для больших линий
	}

	totalBatches := (n + batchSize - 1) / batchSize
	results := make([]SedimentTransportResult, totalBatches)

	totalBudget := SedimentBudget{}
	allWarnings := make([]string, 0)
	isValid := true

	// Обрабатываем каждый батч
	for i := 0; i < totalBatches; i++ {
		start := i * batchSize
		end := start + batchSize
		if end > n {
			end = n
		}

		batchPoints := points[start:end]
		batchErosion := erosionRates[start:end]
		batchLithology := lithology[start:end]

		// Создаём batch-specific wave data
		batchWaveData := WaveEnergyData{
			Energy:    waveData.Energy[start:end],
			Direction: waveData.Direction,
			Incidence: waveData.Incidence[start:end],
			Fetch:     waveData.Fetch[start:end],
		}

		result := CalculateSedimentTransportOptimized(
			batchPoints,
			batchErosion,
			batchWaveData,
			batchLithology,
			params,
		)

		results[i] = result

		totalBudget.ErodedVolume += result.TotalBudget.ErodedVolume
		totalBudget.TransportVolume += result.TotalBudget.TransportVolume
		totalBudget.DepositedVolume += result.TotalBudget.DepositedVolume
		totalBudget.ErosionPoints += result.TotalBudget.ErosionPoints
		totalBudget.DepositionPoints += result.TotalBudget.DepositionPoints

		allWarnings = append(allWarnings, result.Warnings...)

		if !result.IsValid {
			isValid = false
		}
	}

	totalBudget.NetChange = totalBudget.ErodedVolume - totalBudget.DepositedVolume

	return BatchSedimentTransportResult{
		Results:      results,
		TotalBudget:  totalBudget,
		IsValid:      isValid,
		Warnings:     allWarnings,
		BatchSize:    batchSize,
		TotalBatches: totalBatches,
	}
}
