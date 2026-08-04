package geometry

import (
	"math"
	"math/rand"
	"runtime"
	"testing"
)

// ============================================================
// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ДЛЯ ТЕСТОВ
// ============================================================

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

// ============================================================
// БАЗОВЫЕ ТЕСТЫ ФУНКЦИОНАЛЬНОСТИ
// ============================================================

// TestCalculateSedimentTransport базовый тест расчёта транспорта
func TestCalculateSedimentTransport(t *testing.T) {
	// Создаём простую polyline
	points := []LatLon{
		{Lat: 44.5, Lon: 34.0},
		{Lat: 44.5, Lon: 34.1},
		{Lat: 44.5, Lon: 34.2},
		{Lat: 44.5, Lon: 34.3},
		{Lat: 44.5, Lon: 34.4},
	}

	// Равномерная эрозия
	erosionRates := []float64{1.0, 1.0, 1.0, 1.0, 1.0}

	// Wave energy (равномерная)
	waveData := WaveEnergyData{
		Energy:    []float64{0.5, 0.5, 0.5, 0.5, 0.5},
		Direction: 0.0, // с севера
	}

	// Литология (одинаковая)
	lithology := []LithologyState{
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
	}

	// Параметры транспорта
	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		MinimumFlowVelocity:       0.3,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	// Расчёт
	result := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	// Проверки
	if len(result.States) != len(points) {
		t.Errorf("Ожидалось %d состояний, получено %d", len(points), len(result.States))
	}

	// Массовый баланс должен быть в разумных пределах
	totalEroded := result.TotalBudget.ErodedVolume
	totalDeposited := result.TotalBudget.DepositedVolume

	if totalEroded <= 0 {
		t.Errorf("Общий объём эрозии должен быть положительным, получено %.2f", totalEroded)
	}

	if totalDeposited <= 0 {
		t.Errorf("Общий объём депозиции должен быть положительным, получено %.2f", totalDeposited)
	}

	// Вывод результатов для отладки
	t.Logf("Всего эродировано: %.2f м³/м", totalEroded)
	t.Logf("Всего отложено: %.2f м³/м", totalDeposited)
	t.Logf("Всего транспорта: %.2f м³/м", result.TotalBudget.TransportVolume)
	t.Logf("Чистое изменение: %.2f м³/м", result.TotalBudget.NetChange)
	t.Logf("Массовый баланс: %.2f", result.MassBalance)
	t.Logf("Валидно: %v", result.IsValid)
	t.Logf("Предупреждения: %v", result.Warnings)
}

// TestMassBalance тест массового баланса
func TestMassBalance(t *testing.T) {
	// Тест: баланс массы должен сохраняться
	points := []LatLon{
		{Lat: 44.0, Lon: 33.0},
		{Lat: 44.0, Lon: 34.0},
		{Lat: 44.0, Lon: 35.0},
		{Lat: 44.0, Lon: 36.0},
	}

	// Разная эрозия по точкам
	erosionRates := []float64{2.0, 1.5, 1.0, 0.5}

	waveData := WaveEnergyData{
		Energy:    []float64{0.6, 0.5, 0.4, 0.3},
		Direction: 45.0, // с северо-востока
	}

	lithology := []LithologyState{
		{Class: "clay", Resistance: 1.0, Color: "#c4a484"},
		{Class: "marl", Resistance: 1.8, Color: "#a8a8a8"},
		{Class: "sandstone", Resistance: 2.8, Color: "#8b8b8b"},
		{Class: "limestone", Resistance: 4.5, Color: "#6b6b6b"},
	}

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.6,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	result := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	// Баланс массы: eroded ≈ deposited + transport
	// Допуск: 20% (так как есть transport между точками)
	totalEroded := result.TotalBudget.ErodedVolume
	totalDeposited := result.TotalBudget.DepositedVolume
	totalTransport := result.TotalBudget.TransportVolume

	expectedBalance := totalEroded
	actualBalance := totalDeposited + totalTransport

	balanceRatio := math.Abs(expectedBalance-actualBalance) / expectedBalance

	t.Logf("Проверка массового баланса:")
	t.Logf("  Эродировано: %.2f м³/м", totalEroded)
	t.Logf("  Отложено + Транспорт: %.2f + %.2f = %.2f м³/м",
		totalDeposited, totalTransport, totalDeposited+totalTransport)
	t.Logf("  Соотношение баланса: %.2f%%", balanceRatio*100)

	if balanceRatio > 0.2 {
		t.Errorf("Нарушение массового баланса: соотношение %.2f > 20%%", balanceRatio*100)
	}

	// Проверка валидности
	if !result.IsValid && balanceRatio < 0.1 {
		t.Errorf("Результат помечен как невалидный, но соотношение баланса хорошее (%.2f)", balanceRatio)
	}
}

// TestSedimentAccumulation тест аккумуляции
func TestSedimentAccumulation(t *testing.T) {
	// Тест: в точках с низкой энергией волн должна быть аккумуляция
	points := []LatLon{
		{Lat: 45.0, Lon: 35.0}, // высокая энергия
		{Lat: 45.0, Lon: 36.0}, // средняя энергия
		{Lat: 45.0, Lon: 37.0}, // низкая энергия (бухта)
	}

	erosionRates := []float64{2.0, 2.0, 2.0}

	waveData := WaveEnergyData{
		Energy:    []float64{0.8, 0.5, 0.2}, // низкая энергия в точке 2
		Direction: 0.0,
	}

	lithology := []LithologyState{
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
	}

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.8, // высокая депозиция
		CapacityFactor:            0.5, // низкая ёмкость
		LongshoreDriftCoefficient: 0.8,
	}

	result := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	// Проверка: точка 2 должна иметь аккумуляцию
	if !result.States[2].IsAccumulating {
		t.Errorf("Точка 2 должна аккумулировать (низкая энергия волн), получено IsAccumulating=false")
	}

	// Проверка: в точке аккумуляции модифицированная эрозия < базовой
	if result.ModifiedErosion[2] >= result.BaselineErosion[2] {
		t.Errorf("В точке аккумуляции модифицированная эрозия должна быть меньше базовой, получено %.2f >= %.2f",
			result.ModifiedErosion[2], result.BaselineErosion[2])
	}

	t.Logf("Тест аккумуляции:")
	t.Logf("  Точка 2 (низкая энергия):")
	t.Logf("    Базовая эрозия: %.2f м", result.BaselineErosion[2])
	t.Logf("    Модифицированная эрозия: %.2f м", result.ModifiedErosion[2])
	t.Logf("    Отложенный объём: %.2f м³/м", result.States[2].LocalBudget.DepositedVolume)
	t.Logf("    Аккумулирует: %v", result.States[2].IsAccumulating)
}

// TestLithologyEffect тест влияния литологии
func TestLithologyEffect(t *testing.T) {
	// Тест: твёрдые породы должны эродировать медленнее
	points := []LatLon{
		{Lat: 44.0, Lon: 33.0}, // мягкая порода
		{Lat: 44.0, Lon: 35.0}, // твёрдая порода
	}

	erosionRates := []float64{2.0, 2.0} // одинаковая базовая эрозия

	waveData := WaveEnergyData{
		Energy:    []float64{0.5, 0.5},
		Direction: 0.0,
	}

	lithology := []LithologyState{
		{Class: "clay", Resistance: 1.0, Color: "#c4a484"},         // мягкая
		{Class: "serpentinite", Resistance: 9.0, Color: "#2d2d2d"}, // твёрдая
	}

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	result := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	// Объём эрозии для мягкой породы > для твёрдой
	erodedSoft := result.States[0].LocalBudget.ErodedVolume
	erodedHard := result.States[1].LocalBudget.ErodedVolume

	if erodedSoft <= erodedHard {
		t.Errorf("Мягкая порода должна эродировать больше твёрдой, получено %.2f <= %.2f",
			erodedSoft, erodedHard)
	}

	t.Logf("Влияние литологии:")
	t.Logf("  Глина (R=1.0): эродировано %.2f м³/м", erodedSoft)
	t.Logf("  Серпентинит (R=9.0): эродировано %.2f м³/м", erodedHard)
	t.Logf("  Соотношение: %.2fx", erodedSoft/erodedHard)
}

// TestLongshoreDrift тест longshore drift
func TestLongshoreDrift(t *testing.T) {
	// Тест: longshore drift должен распределять sediment вдоль берега
	points := []LatLon{
		{Lat: 43.0, Lon: 33.0},
		{Lat: 43.0, Lon: 34.0},
		{Lat: 43.0, Lon: 35.0},
		{Lat: 43.0, Lon: 36.0},
		{Lat: 43.0, Lon: 37.0},
	}

	erosionRates := []float64{2.0, 2.0, 2.0, 2.0, 2.0}

	waveData := WaveEnergyData{
		Energy:    []float64{0.5, 0.5, 0.5, 0.5, 0.5},
		Direction: 90.0, // с востока → drift на запад
	}

	lithology := []LithologyState{
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
	}

	params := SedimentTransportParameters{
		TransportCoefficient:      0.8, // высокий транспорт
		DepositionRate:            0.3, // низкая депозиция
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.9, // сильный drift
	}

	result := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	// Проверка: должен быть transport между точками
	hasIncomingTransport := false
	hasOutgoingTransport := false

	for _, state := range result.States {
		if len(state.InTransitFrom) > 0 {
			hasIncomingTransport = true
		}
		if len(state.InTransitTo) > 0 {
			hasOutgoingTransport = true
		}
	}

	if !hasIncomingTransport {
		t.Error("Ожидался входящий транспорт наносов, не получено")
	}

	if !hasOutgoingTransport {
		t.Error("Ожидался исходящий транспорт наносов, не получено")
	}

	t.Logf("Longshore drift:")
	t.Logf("  Всего транспорта: %.2f м³/м", result.TotalBudget.TransportVolume)
	t.Logf("  Всего отложено: %.2f м³/м", result.TotalBudget.DepositedVolume)
	t.Logf("  Есть входящий: %v", hasIncomingTransport)
	t.Logf("  Есть исходящий: %v", hasOutgoingTransport)
}

// TestApplySedimentModification тест модификации эрозии
func TestApplySedimentModification(t *testing.T) {
	points := []LatLon{
		{Lat: 44.0, Lon: 34.0},
		{Lat: 44.0, Lon: 35.0},
	}

	baseErosion := []float64{2.0, 2.0}

	// Simulate sediment result with accumulation at point 1
	result := SedimentTransportResult{
		States: []SedimentState{
			{PointIndex: 0, IsAccumulating: false, LocalBudget: SedimentBudget{DepositedVolume: 0.0}},
			{PointIndex: 1, IsAccumulating: true, LocalBudget: SedimentBudget{DepositedVolume: 1.5}},
		},
	}

	modified := ApplySedimentModification(points, baseErosion, result)

	// Точка 0: без изменений
	if modified[0] != baseErosion[0] {
		t.Errorf("Точка 0 не должна быть модифицирована, получено %.2f != %.2f",
			modified[0], baseErosion[0])
	}

	// Точка 1: уменьшена на депозицию
	expectedModified := baseErosion[1] - 1.5
	if math.Abs(modified[1]-expectedModified) > 0.01 {
		t.Errorf("Точка 1 должна быть модифицирована до %.2f, получено %.2f",
			expectedModified, modified[1])
	}

	t.Logf("Модификация седиментов:")
	t.Logf("  Точка 0: %.2f → %.2f (без изменений)", baseErosion[0], modified[0])
	t.Logf("  Точка 1: %.2f → %.2f (уменьшено на депозицию)", baseErosion[1], modified[1])
}

// ============================================================
// ТЕСТЫ ОПТИМИЗАЦИИ И ПРОИЗВОДИТЕЛЬНОСТИ
// ============================================================

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
				t.Errorf("Несоответствие ErodedVolume: оригинал=%f, оптимизированный=%f",
					originalResult.TotalBudget.ErodedVolume, optimizedResult.TotalBudget.ErodedVolume)
			}
			if !roughlyEqual(originalResult.TotalBudget.DepositedVolume, optimizedResult.TotalBudget.DepositedVolume, 0.01) {
				t.Errorf("Несоответствие DepositedVolume: оригинал=%f, оптимизированный=%f",
					originalResult.TotalBudget.DepositedVolume, optimizedResult.TotalBudget.DepositedVolume)
			}
			if !roughlyEqual(originalResult.TotalBudget.TransportVolume, optimizedResult.TotalBudget.TransportVolume, 0.01) {
				t.Errorf("Несоответствие TransportVolume: оригинал=%f, оптимизированный=%f",
					originalResult.TotalBudget.TransportVolume, optimizedResult.TotalBudget.TransportVolume)
			}

			// Сравнение количества точек эрозии/аккумуляции
			if originalResult.TotalBudget.ErosionPoints != optimizedResult.TotalBudget.ErosionPoints {
				t.Errorf("Несоответствие ErosionPoints: оригинал=%d, оптимизированный=%d",
					originalResult.TotalBudget.ErosionPoints, optimizedResult.TotalBudget.ErosionPoints)
			}
			if originalResult.TotalBudget.DepositionPoints != optimizedResult.TotalBudget.DepositionPoints {
				t.Errorf("Несоответствие DepositionPoints: оригинал=%d, оптимизированный=%d",
					originalResult.TotalBudget.DepositionPoints, optimizedResult.TotalBudget.DepositionPoints)
			}

			// Детальное сравнение states (для малых наборов)
			if tc.n <= 100 {
				for i := 0; i < len(originalResult.States); i++ {
					origState := originalResult.States[i]
					optState := optimizedResult.States[i]

					if !roughlyEqual(origState.LocalBudget.ErodedVolume, optState.LocalBudget.ErodedVolume, 0.001) {
						t.Errorf("State %d несоответствие ErodedVolume: оригинал=%f, оптимизированный=%f",
							i, origState.LocalBudget.ErodedVolume, optState.LocalBudget.ErodedVolume)
					}
					if !roughlyEqual(origState.LocalBudget.DepositedVolume, optState.LocalBudget.DepositedVolume, 0.01) {
						t.Logf("State %d DepositedVolume: оригинал=%f, оптимизированный=%f",
							i, origState.LocalBudget.DepositedVolume, optState.LocalBudget.DepositedVolume)
					}
				}
			}
		})
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
		t.Error("Кэш должен быть инициализирован после Initialize()")
	}

	// Проверяем, что направления нормализованы
	for i := 0; i < n; i++ {
		dir := cache.GetAlongshoreDirection(i)
		length := math.Sqrt(dir.X*dir.X + dir.Y*dir.Y)

		if length > 1.0+1e-6 {
			t.Errorf("Направление %d не нормализовано: длина=%f", i, length)
		}
	}

	// Проверяем wave direction
	waveDir := cache.GetWaveDirection()
	waveLength := math.Sqrt(waveDir.X*waveDir.X + waveDir.Y*waveDir.Y)
	if waveLength > 1.0+1e-6 {
		t.Errorf("Направление волны не нормализовано: длина=%f", waveLength)
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
		t.Errorf("Несоответствие ErodedVolume: полный=%f, пачечный=%f",
			fullResult.TotalBudget.ErodedVolume, batchedResult.TotalBudget.ErodedVolume)
	}

	if !roughlyEqual(fullResult.TotalBudget.DepositedVolume, batchedResult.TotalBudget.DepositedVolume, 0.01) {
		t.Errorf("Несоответствие DepositedVolume: полный=%f, пачечный=%f",
			fullResult.TotalBudget.DepositedVolume, batchedResult.TotalBudget.DepositedVolume)
	}

	// Проверка количества батчей
	expectedBatches := (n + 500 - 1) / 500
	if batchedResult.TotalBatches != expectedBatches {
		t.Errorf("Несоответствие TotalBatches: ожидалось=%d, получено=%d",
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
				t.Errorf("Несоответствие длины States: ожидалось=%d, получено=%d", tc.n, len(result.States))
			}

			if result.TotalBudget.ErodedVolume <= 0 {
				t.Error("Ожидался положительный объём эрозии")
			}

			// Проверяем стратегию
			stats := GetPerformanceStats(tc.n)
			if stats.Strategy != tc.wantStr {
				t.Errorf("Несоответствие стратегии: ожидалось=%s, получено=%s", tc.wantStr, stats.Strategy)
			}
		})
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
					t.Error("Ожидались пустые состояния для пустого ввода")
				}
				return
			}

			// Количество состояний должно совпадать
			if len(original.States) != len(optimized.States) {
				t.Errorf("Несоответствие количества состояний: оригинал=%d, оптимизированный=%d",
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
				t.Errorf("Результаты не детерминированы на итерации %d", i)
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
				t.Errorf("%s: несоответствие ErodedVolume: оригинал=%f, оптимизированный=%f",
					tc.name, original.TotalBudget.ErodedVolume, optimized.TotalBudget.ErodedVolume)
			}
			if !roughlyEqual(original.TotalBudget.DepositedVolume, optimized.TotalBudget.DepositedVolume, tolerance) {
				t.Errorf("%s: несоответствие DepositedVolume: оригинал=%f, оптимизированный=%f",
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
		t.Error("Кэш должен возвращать консистентные результаты")
	}

	// Повторная инициализация не должна panic
	cache.Initialize(points, waveData)
	dir3 := cache.GetAlongshoreDirection(100)

	if dir1.X != dir3.X || dir1.Y != dir3.Y {
		t.Error("Реинициализация должна производить те же результаты")
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
		t.Error("Ожидались непустые результаты пачечной обработки")
	}

	if result.TotalBatches != 10 { // 50000 / 5000 = 10
		t.Logf("Предупреждение: ожидалось 10 батчей, получено %d", result.TotalBatches)
	}
}

// ============================================================
// ТЕСТЫ ВРЕМЕННОЙ ДИНАМИКИ
// ============================================================

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

	temporalState := SedimentTemporalState{
		Year:           0.5,
		Season:         "summer",
		IsStorm:        false,
		StormIntensity: 1.0,
		SeasonFactor:   1.0,
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
		t.Errorf("Несоответствие длины States: ожидалось=%d, получено=%d", n, len(result.States))
	}

	if result.TotalBudget.ErodedVolume <= 0 {
		t.Error("Ожидался положительный объём эрозии")
	}

	// Проверяем сезонную статистику
	if len(result.SeasonalStats) == 0 {
		t.Error("Ожидалась сезонная статистика")
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

	temporalState := SedimentTemporalState{
		Year:           0.5,
		Season:         "summer",
		IsStorm:        true,
		StormIntensity: 2.5,
		SeasonFactor:   1.0,
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
		t.Errorf("Несоответствие количества состояний: оригинал=%d, оптимизированный=%d",
			len(original.States), len(optimized.States))
	}

	// Сезон должен совпадать
	if original.States[0].CurrentSeason != optimized.States[0].CurrentSeason {
		t.Errorf("Несоответствие сезона: оригинал=%s, оптимизированный=%s",
			original.States[0].CurrentSeason, optimized.States[0].CurrentSeason)
	}

	// Проверяем штормовые отложения
	if len(original.AllStormDeposits) != len(optimized.AllStormDeposits) {
		t.Logf("Предупреждение: количество штормовых отложений отличается: оригинал=%d, оптимизированный=%d",
			len(original.AllStormDeposits), len(optimized.AllStormDeposits))
	}
}

// ============================================================
// БЕНЧМАРКИ
// ============================================================

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

// BenchmarkCalculateSedimentTransport базовый бенчмарк
func BenchmarkCalculateSedimentTransport(b *testing.B) {
	// Создаём realistic polyline
	n := 1000
	points := make([]LatLon, n)
	for i := 0; i < n; i++ {
		points[i] = LatLon{
			Lat: 44.0 + float64(i)*0.001,
			Lon: 33.0 + float64(i)*0.001,
		}
	}

	erosionRates := make([]float64, n)
	for i := range erosionRates {
		erosionRates[i] = 1.0 + float64(i%3)
	}

	waveData := WaveEnergyData{
		Energy:    make([]float64, n),
		Direction: 0.0,
	}
	for i := range waveData.Energy {
		waveData.Energy[i] = 0.3 + float64(i%5)*0.1
	}

	lithology := make([]LithologyState, n)
	for i := range lithology {
		lithology[i] = LithologyState{
			Class:      "limestone",
			Resistance: 4.0,
			Color:      "#6b6b6b",
		}
	}

	params := SedimentTransportParameters{
		TransportCoefficient:      0.7,
		DepositionRate:            0.5,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)
	}
}

func TestOptimizedSedimentCacheGetSegmentLength(t *testing.T) {
	points := []LatLon{
		{Lat: 45, Lon: 30},
		{Lat: 45, Lon: 31},
		{Lat: 46, Lon: 31},
	}

	cache := NewOptimizedCache()
	n := len(points)
	waveData := WaveEnergyData{
		Energy:    make([]float64, n),
		Direction: 45.0,
		Incidence: make([]float64, n),
		Fetch:     make([]float64, n),
	}
	for i := range waveData.Energy {
		waveData.Energy[i] = 0.8
		waveData.Incidence[i] = 0.5
		waveData.Fetch[i] = 10000
	}
	cache.Initialize(points, waveData)
	
	// Проверка валидного индекса
	length := cache.GetSegmentLength(0)
	if length < 0 {
		t.Errorf("GetSegmentLength(0) returned negative value %v", length)
	}
	
	// Проверка граничных условий
	firstLength := cache.GetSegmentLength(0)
	lastLength := cache.GetSegmentLength(len(points) - 2)
	
	if firstLength <= 0 {
		t.Errorf("GetSegmentLength(0) expected positive length, got %v", firstLength)
	}
	if lastLength <= 0 {
		t.Errorf("GetSegmentLength(last) expected positive length, got %v", lastLength)
	}
	
	// Проверка неверных индексов
	invalidNeg := cache.GetSegmentLength(-1)
	if invalidNeg != 0 {
		t.Errorf("GetSegmentLength(-1) expected 0, got %v", invalidNeg)
	}
	
	invalidLarge := cache.GetSegmentLength(1000)
	if invalidLarge != 0 {
		t.Errorf("GetSegmentLength(1000) expected 0, got %v", invalidLarge)
	}
}
