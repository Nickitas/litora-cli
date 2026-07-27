package geometry

import (
	"math"
	"math/rand"
	"runtime"
	"testing"
)

// generateTestPoints генерирует тестовые точки
func generateTestPoints(n int) []LatLon {
	points := make([]LatLon, n)
	for i := 0; i < n; i++ {
		// Простая линия вдоль побережья
		points[i] = LatLon{
			Lat: 43.0 + float64(i)*0.001,
			Lon: 39.0 + float64(i)*0.001,
		}
	}
	return points
}

// generateTestErosionRates генерирует тестовые скорости эрозии
func generateTestErosionRates(n int) []float64 {
	rates := make([]float64, n)
	for i := range rates {
		rates[i] = 0.1 + rand.Float64()*0.5
	}
	return rates
}

// generateTestWaveData генерирует тестовые волновые данные
func generateTestWaveData(n int) WaveEnergyData {
	energy := make([]float64, n)
	incidence := make([]float64, n)
	fetch := make([]float64, n)

	for i := range energy {
		energy[i] = rand.Float64()
		incidence[i] = rand.Float64()
		fetch[i] = 1000 + rand.Float64()*5000
	}

	return WaveEnergyData{
		Energy:    energy,
		Direction: 45 + rand.Float64()*90, // NE to SE
		Incidence: incidence,
		Fetch:     fetch,
	}
}

// generateTestLithology генерирует тестовую литологию
func generateTestLithology(n int) []LithologyState {
	lithology := make([]LithologyState, n)
	classes := []string{"sand", "clay", "gravel", "rock", "silt"}

	for i := range lithology {
		class := classes[rand.Intn(len(classes))]
		resistance := 1.0
		switch class {
		case "rock":
			resistance = 8.0
		case "gravel":
			resistance = 4.0
		case "sand":
			resistance = 1.0
		case "silt":
			resistance = 0.5
		case "clay":
			resistance = 0.3
		}

		lithology[i] = LithologyState{
			Class:       class,
			Resistance:  resistance,
			Color:       "#888888",
			Description: class,
		}
	}
	return lithology
}

// TestOptimizedVsOriginal сравнивает результаты оптимизированной и оригинальной версий
func TestOptimizedVsOriginal(t *testing.T) {
	testCases := []struct {
		name string
		n    int
	}{
		{"small", 50},
		{"medium", 500},
		{"large", 5000},
	}

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			points := generateTestPoints(tc.n)
			erosionRates := generateTestErosionRates(tc.n)
			waveData := generateTestWaveData(tc.n)
			lithology := generateTestLithology(tc.n)

			// Оригинальная версия
			originalResult := CalculateSedimentTransport(
				points,
				erosionRates,
				waveData,
				lithology,
				params,
			)

			// Оптимизированная версия
			optimizedResult := CalculateSedimentTransportOptimized(
				points,
				erosionRates,
				waveData,
				lithology,
				params,
			)

			// Сравнение общих бюджетов
			if !roughlyEqual(originalResult.TotalBudget.ErodedVolume, optimizedResult.TotalBudget.ErodedVolume, 0.01) {
				t.Errorf("ErodedVolume mismatch: original=%f, optimized=%f",
					originalResult.TotalBudget.ErodedVolume, optimizedResult.TotalBudget.ErodedVolume)
			}
			if !roughlyEqual(originalResult.TotalBudget.DepositedVolume, optimizedResult.TotalBudget.DepositedVolume, 0.01) {
				t.Errorf("DepositedVolume mismatch: original=%f, optimized=%f",
					originalResult.TotalBudget.DepositedVolume, optimizedResult.TotalBudget.DepositedVolume)
			}
			if !roughlyEqual(originalResult.TotalBudget.TransportVolume, optimizedResult.TotalBudget.TransportVolume, 0.01) {
				t.Errorf("TransportVolume mismatch: original=%f, optimized=%f",
					originalResult.TotalBudget.TransportVolume, optimizedResult.TotalBudget.TransportVolume)
			}

			// Сравнение количества точек эрозии/аккумуляции
			if originalResult.TotalBudget.ErosionPoints != optimizedResult.TotalBudget.ErosionPoints {
				t.Errorf("ErosionPoints mismatch: original=%d, optimized=%d",
					originalResult.TotalBudget.ErosionPoints, optimizedResult.TotalBudget.ErosionPoints)
			}
			if originalResult.TotalBudget.DepositionPoints != optimizedResult.TotalBudget.DepositionPoints {
				t.Errorf("DepositionPoints mismatch: original=%d, optimized=%d",
					originalResult.TotalBudget.DepositionPoints, optimizedResult.TotalBudget.DepositionPoints)
			}

			// Детальное сравнение states (для малых наборов)
			if tc.n <= 100 {
				for i := 0; i < len(originalResult.States); i++ {
					origState := originalResult.States[i]
					optState := optimizedResult.States[i]

					if !roughlyEqual(origState.LocalBudget.ErodedVolume, optState.LocalBudget.ErodedVolume, 0.001) {
						t.Errorf("State %d ErodedVolume mismatch: original=%f, optimized=%f",
							i, origState.LocalBudget.ErodedVolume, optState.LocalBudget.ErodedVolume)
					}
					if !roughlyEqual(origState.LocalBudget.DepositedVolume, optState.LocalBudget.DepositedVolume, 0.01) {
						t.Logf("State %d DepositedVolume: original=%f, optimized=%f",
							i, origState.LocalBudget.DepositedVolume, optState.LocalBudget.DepositedVolume)
					}
				}
			}
		})
	}
}

// roughlyEqual проверяет приблизительное равенство
func roughlyEqual(a, b, tolerance float64) bool {
	if a == 0 && b == 0 {
		return true
	}
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	avg := (a + b) / 2
	if avg == 0 {
		return diff < tolerance
	}
	return (diff / avg) < tolerance
}

// BenchmarkOriginal бенчмарк оригинальной версии
func BenchmarkOriginal(b *testing.B) {
	n := 10000
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)
	}
}

// BenchmarkOptimized бенчмарк оптимизированной версии
func BenchmarkOptimized(b *testing.B) {
	n := 10000
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateSedimentTransportOptimized(points, erosionRates, waveData, lithology, params)
	}
}

// BenchmarkOptimizedSmall бенчмарк для малых наборов
func BenchmarkOptimizedSmall(b *testing.B) {
	n := 100
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateSedimentTransportOptimized(points, erosionRates, waveData, lithology, params)
	}
}

// BenchmarkOptimizedLarge бенчмарк для больших наборов
func BenchmarkOptimizedLarge(b *testing.B) {
	n := 50000
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateSedimentTransportOptimized(points, erosionRates, waveData, lithology, params)
	}
}

// BenchmarkBatched бенчмарк пачечной обработки
func BenchmarkBatched(b *testing.B) {
	n := 50000
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateSedimentTransportBatched(points, erosionRates, waveData, lithology, params, 2000)
	}
}

// TestCacheCorrectness проверка корректности кэша
func TestCacheCorrectness(t *testing.T) {
	n := 100
	points := generateTestPoints(n)
	waveData := generateTestWaveData(n)

	cache := NewOptimizedCache()
	cache.Initialize(points, waveData)

	// Проверяем, что кэш инициализирован
	if !cache.initialized {
		t.Error("Cache should be initialized after Initialize()")
	}

	// Проверяем, что направления нормализованы
	for i := 0; i < n; i++ {
		dir := cache.GetAlongshoreDirection(i)
		length := math.Sqrt(dir.X*dir.X + dir.Y*dir.Y)

		if length > 1.0+1e-6 {
			t.Errorf("Direction %d not normalized: length=%f", i, length)
		}
	}

	// Проверяем wave direction
	waveDir := cache.GetWaveDirection()
	waveLength := math.Sqrt(waveDir.X*waveDir.X + waveDir.Y*waveDir.Y)
	if waveLength > 1.0+1e-6 {
		t.Errorf("Wave direction not normalized: length=%f", waveLength)
	}
}

// TestBatchedCorrectness проверка пачечной обработки
func TestBatchedCorrectness(t *testing.T) {
	n := 5000
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	// Полная обработка
	fullResult := CalculateSedimentTransportOptimized(
		points, erosionRates, waveData, lithology, params,
	)

	// Пачечная обработка
	batchedResult := CalculateSedimentTransportBatched(
		points, erosionRates, waveData, lithology, params, 500,
	)

	// Сравнение общих бюджетов
	if !roughlyEqual(fullResult.TotalBudget.ErodedVolume, batchedResult.TotalBudget.ErodedVolume, 0.01) {
		t.Errorf("ErodedVolume mismatch: full=%f, batched=%f",
			fullResult.TotalBudget.ErodedVolume, batchedResult.TotalBudget.ErodedVolume)
	}

	if !roughlyEqual(fullResult.TotalBudget.DepositedVolume, batchedResult.TotalBudget.DepositedVolume, 0.01) {
		t.Errorf("DepositedVolume mismatch: full=%f, batched=%f",
			fullResult.TotalBudget.DepositedVolume, batchedResult.TotalBudget.DepositedVolume)
	}

	// Проверка количества батчей
	expectedBatches := (n + 500 - 1) / 500
	if batchedResult.TotalBatches != expectedBatches {
		t.Errorf("TotalBatches mismatch: expected=%d, got=%d",
			expectedBatches, batchedResult.TotalBatches)
	}
}

// TestCalculateSedimentTransportAuto проверка автоматического выбора стратегии
func TestCalculateSedimentTransportAuto(t *testing.T) {
	testCases := []struct {
		name    string
		n       int
		wantStr string
	}{
		{"small", 100, "original"},
		{"medium", 750, "optimized"},
		{"large", 15000, "parallel"},
		{"huge", 60000, "batched"},
	}

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			points := generateTestPoints(tc.n)
			erosionRates := generateTestErosionRates(tc.n)
			waveData := generateTestWaveData(tc.n)
			lithology := generateTestLithology(tc.n)

			result := CalculateSedimentTransportAuto(
				points, erosionRates, waveData, lithology, params,
			)

			if len(result.States) != tc.n {
				t.Errorf("States length mismatch: expected=%d, got=%d", tc.n, len(result.States))
			}

			if result.TotalBudget.ErodedVolume <= 0 {
				t.Error("Expected positive eroded volume")
			}

			// Проверяем стратегию
			stats := GetPerformanceStats(tc.n)
			if stats.Strategy != tc.wantStr {
				t.Errorf("Strategy mismatch: expected=%s, got=%s", tc.wantStr, stats.Strategy)
			}
		})
	}
}

// TestTemporalOptimized проверка временной динамики с оптимизацией
func TestTemporalOptimized(t *testing.T) {
	n := 100
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	seasonalMod := SeasonalModulation{
		WinterMultiplier:        1.5,
		SummerMultiplier:        0.7,
		TransitionMultiplier:    1.0,
		StormSeasonBoost:        1.5,
		AccumulationSeasonality: true,
	}

	stormParams := StormSedimentParameters{
		StormTransportMultiplier:  2.5,
		StormDepositionEfficiency: 0.6,
		PostStormSurgeMultiplier:  1.5,
		StormThreshold:            1.2,
		StormRetreatMultiplier:    3.0,
		StormBypassingCoefficient: 0.3,
	}

	temporalState := TemporalState{
		Step:           0,
		Year:           0.5, // Summer
		IsStorm:        false,
		StormIntensity: 1.0,
		SeasonalFactor: 1.0,
	}

	result := CalculateSedimentTransportWithTemporal(
		points,
		erosionRates,
		waveData,
		lithology,
		params,
		temporalState,
		seasonalMod,
		stormParams,
	)

	if len(result.States) != n {
		t.Errorf("States length mismatch: expected=%d, got=%d", n, len(result.States))
	}

	if result.TotalBudget.ErodedVolume <= 0 {
		t.Error("Expected positive eroded volume")
	}

	// Проверяем сезонную статистику
	if len(result.SeasonalStats) == 0 {
		t.Error("Expected seasonal statistics")
	}
}

// TestTemporalOptimizedVsOriginal сравнение оригинальной и оптимизированной temporal версий
func TestTemporalOptimizedVsOriginal(t *testing.T) {
	n := 1000
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	seasonalMod := SeasonalModulation{
		WinterMultiplier:        1.5,
		SummerMultiplier:        0.7,
		TransitionMultiplier:    1.0,
		StormSeasonBoost:        1.5,
		AccumulationSeasonality: true,
	}

	stormParams := StormSedimentParameters{
		StormTransportMultiplier:  2.5,
		StormDepositionEfficiency: 0.6,
		PostStormSurgeMultiplier:  1.5,
		StormThreshold:            1.2,
		StormRetreatMultiplier:    3.0,
		StormBypassingCoefficient: 0.3,
	}

	temporalState := TemporalState{
		Step:           0,
		Year:           0.5,
		IsStorm:        true,
		StormIntensity: 2.5,
		SeasonalFactor: 1.0,
	}

	// Оригинальная версия
	original := CalculateSedimentTransportWithTemporal(
		points, erosionRates, waveData, lithology, params,
		temporalState, seasonalMod, stormParams,
	)

	// Оптимизированная версия
	optimized := CalculateSedimentTransportWithTemporalOptimized(
		points, erosionRates, waveData, lithology, params,
		temporalState, seasonalMod, stormParams,
	)

	// Проверяем, что результаты похожи
	if len(original.States) != len(optimized.States) {
		t.Errorf("States count mismatch: original=%d, optimized=%d",
			len(original.States), len(optimized.States))
	}

	// Сезон должен совпадать
	if original.States[0].CurrentSeason != optimized.States[0].CurrentSeason {
		t.Errorf("Season mismatch: original=%s, optimized=%s",
			original.States[0].CurrentSeason, optimized.States[0].CurrentSeason)
	}

	// Проверяем штормовые отложения
	if len(original.AllStormDeposits) != len(optimized.AllStormDeposits) {
		t.Logf("Warning: Storm deposits count differs: original=%d, optimized=%d",
			len(original.AllStormDeposits), len(optimized.AllStormDeposits))
	}
}

// BenchmarkErosionVolumesParallel бенчмарк параллельного расчёта эрозии
func BenchmarkErosionVolumesParallel(b *testing.B) {
	n := 100000
	erosionRates := generateTestErosionRates(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	states := make([]SedimentState, n)
	for i := range states {
		states[i].PointIndex = i
		states[i].InTransitFrom = make([]float64, 0)
		states[i].InTransitTo = make([]float64, 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateErosionVolumesOptimized(states, erosionRates, lithology, params, true, runtime.NumCPU())
	}
}

// TestEdgeCases проверка граничных случаев
func TestEdgeCases(t *testing.T) {
	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	testCases := []struct {
		name string
		n    int
	}{
		{"empty", 0},
		{"single", 1},
		{"two", 2},
		{"three", 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			points := generateTestPoints(tc.n)
			erosionRates := generateTestErosionRates(tc.n)
			waveData := generateTestWaveData(tc.n)
			lithology := generateTestLithology(tc.n)

			// Не должно panic
			original := CalculateSedimentTransport(
				points, erosionRates, waveData, lithology, params,
			)
			optimized := CalculateSedimentTransportOptimized(
				points, erosionRates, waveData, lithology, params,
			)

			// Для пустых данных оба должны вернуть пустой результат
			if tc.n == 0 {
				if len(original.States) != 0 || len(optimized.States) != 0 {
					t.Error("Expected empty states for empty input")
				}
				return
			}

			// Количество состояний должно совпадать
			if len(original.States) != len(optimized.States) {
				t.Errorf("States count mismatch: original=%d, optimized=%d",
					len(original.States), len(optimized.States))
			}
		})
	}
}

// TestDeterministicResults проверка детерминированности результатов
func TestDeterministicResults(t *testing.T) {
	n := 1000
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	// Запускаем несколько раз
	var firstResult SedimentTransportResult
	for i := 0; i < 10; i++ {
		result := CalculateSedimentTransportOptimized(
			points, erosionRates, waveData, lithology, params,
		)

		if i == 0 {
			firstResult = result
		} else {
			// Результаты должны быть идентичны
			if firstResult.TotalBudget.ErodedVolume != result.TotalBudget.ErodedVolume ||
				firstResult.TotalBudget.DepositedVolume != result.TotalBudget.DepositedVolume ||
				firstResult.TotalBudget.TransportVolume != result.TotalBudget.TransportVolume {
				t.Errorf("Results are not deterministic on iteration %d", i)
			}
		}
	}
}

// TestConsistencyAcrossParameters проверка консистентности для разных параметров
func TestConsistencyAcrossParameters(t *testing.T) {
	n := 500
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	paramVariations := []struct {
		name   string
		params SedimentTransportParameters
	}{
		{
			"default",
			SedimentTransportParameters{
				TransportCoefficient:      0.7,
				DepositionRate:            0.5,
				MinimumFlowVelocity:       0.3,
				CapacityFactor:            1.0,
				LongshoreDriftCoefficient: 0.8,
			},
		},
		{
			"high_transport",
			SedimentTransportParameters{
				TransportCoefficient:      0.9,
				DepositionRate:            0.3,
				MinimumFlowVelocity:       0.5,
				CapacityFactor:            0.8,
				LongshoreDriftCoefficient: 0.9,
			},
		},
		{
			"low_transport",
			SedimentTransportParameters{
				TransportCoefficient:      0.3,
				DepositionRate:            0.8,
				MinimumFlowVelocity:       0.2,
				CapacityFactor:            1.5,
				LongshoreDriftCoefficient: 0.4,
			},
		},
	}

	for _, tc := range paramVariations {
		t.Run(tc.name, func(t *testing.T) {
			original := CalculateSedimentTransport(
				points, erosionRates, waveData, lithology, tc.params,
			)
			optimized := CalculateSedimentTransportOptimized(
				points, erosionRates, waveData, lithology, tc.params,
			)

			// Проверяем, что бюджеты примерно равны
			tolerance := 0.05 // 5%
			if !roughlyEqual(original.TotalBudget.ErodedVolume, optimized.TotalBudget.ErodedVolume, tolerance) {
				t.Errorf("%s: ErodedVolume mismatch: original=%f, optimized=%f",
					tc.name, original.TotalBudget.ErodedVolume, optimized.TotalBudget.ErodedVolume)
			}
			if !roughlyEqual(original.TotalBudget.DepositedVolume, optimized.TotalBudget.DepositedVolume, tolerance) {
				t.Errorf("%s: DepositedVolume mismatch: original=%f, optimized=%f",
					tc.name, original.TotalBudget.DepositedVolume, optimized.TotalBudget.DepositedVolume)
			}
		})
	}
}

// TestCacheReusability проверка переиспользования кэша
func TestCacheReusability(t *testing.T) {
	n := 1000
	points := generateTestPoints(n)
	waveData := generateTestWaveData(n)

	cache := NewOptimizedCache()
	cache.Initialize(points, waveData)

	// Получаем направления
	dir1 := cache.GetAlongshoreDirection(100)
	dir2 := cache.GetAlongshoreDirection(100)

	if dir1.X != dir2.X || dir1.Y != dir2.Y {
		t.Error("Cache should return consistent results")
	}

	// Повторная инициализация не должна panic
	cache.Initialize(points, waveData)
	dir3 := cache.GetAlongshoreDirection(100)

	if dir1.X != dir3.X || dir1.Y != dir3.Y {
		t.Error("Re-initialization should produce same results")
	}
}

// TestMemoryEfficiency проверка эффективности использования памяти
func TestMemoryEfficiency(t *testing.T) {
	// Этот тест проверяет, что для больших наборов не происходит экспоненциального роста аллокаций
	n := 50000
	points := generateTestPoints(n)
	erosionRates := generateTestErosionRates(n)
	waveData := generateTestWaveData(n)
	lithology := generateTestLithology(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	// Batched версия должна работать без OOM
	result := CalculateSedimentTransportBatched(
		points, erosionRates, waveData, lithology, params, 5000,
	)

	if len(result.Results) == 0 {
		t.Error("Expected non-empty batched results")
	}

	if result.TotalBatches != 10 { // 50000 / 5000 = 10
		t.Logf("Warning: expected 10 batches, got %d", result.TotalBatches)
	}
}

// BenchmarkLongshoreDriftOptimized бенчмарк оптимизированного longshore drift
func BenchmarkLongshoreDriftOptimized(b *testing.B) {
	n := 10000
	points := generateTestPoints(n)
	waveData := generateTestWaveData(n)

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	// Инициализируем states
	states := make([]SedimentState, n)
	erosionRates := generateTestErosionRates(n)
	lithology := generateTestLithology(n)
	calculateErosionVolumesSequential(states, erosionRates, lithology, params)

	cache := NewOptimizedCache()
	cache.Initialize(points, waveData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateLongshoreDriftOptimized(states, points, waveData, params, cache, true)
	}
}
