package geometry

import (
	"math"
	"testing"
)

// TestCalculateSedimentTransport базовый тест
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
		TransportCoefficient:         0.7,
		DepositionRate:                0.5,
		MinimumFlowVelocity:           0.3,
		CapacityFactor:                1.0,
		LongshoreDriftCoefficient:     0.8,
	}

	// Расчёт
	result := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	// Проверки
	if len(result.States) != len(points) {
		t.Errorf("Expected %d states, got %d", len(points), len(result.States))
	}

	// Массовый баланс должен быть в разумных пределах
	totalEroded := result.TotalBudget.ErodedVolume
	totalDeposited := result.TotalBudget.DepositedVolume

	if totalEroded <= 0 {
		t.Errorf("Total eroded volume should be positive, got %.2f", totalEroded)
	}

	if totalDeposited <= 0 {
		t.Errorf("Total deposited volume should be positive, got %.2f", totalDeposited)
	}

	// Вывод результатов для отладки
	t.Logf("Total eroded: %.2f m³/m", totalEroded)
	t.Logf("Total deposited: %.2f m³/m", totalDeposited)
	t.Logf("Total transport: %.2f m³/m", result.TotalBudget.TransportVolume)
	t.Logf("Net change: %.2f m³/m", result.TotalBudget.NetChange)
	t.Logf("Mass balance: %.2f", result.MassBalance)
	t.Logf("Is valid: %v", result.IsValid)
	t.Logf("Warnings: %v", result.Warnings)
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
		TransportCoefficient:     0.7,
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

	t.Logf("Mass balance check:")
	t.Logf("  Eroded: %.2f m³/m", totalEroded)
	t.Logf("  Deposited + Transport: %.2f + %.2f = %.2f m³/m",
		totalDeposited, totalTransport, totalDeposited+totalTransport)
	t.Logf("  Balance ratio: %.2f%%", balanceRatio*100)

	if balanceRatio > 0.2 {
		t.Errorf("Mass balance violation: ratio %.2f > 20%%", balanceRatio*100)
	}

	// Проверка валидности
	if !result.IsValid && balanceRatio < 0.1 {
		t.Errorf("Result marked as invalid but balance ratio is good (%.2f)", balanceRatio)
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
		TransportCoefficient:     0.7,
		DepositionRate:            0.8, // высокая депозиция
		CapacityFactor:            0.5, // низкая ёмкость
		LongshoreDriftCoefficient: 0.8,
	}

	result := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	// Проверка: точка 2 должна иметь аккумуляцию
	if !result.States[2].IsAccumulating {
		t.Errorf("Point 2 should be accumulating (low wave energy), got IsAccumulating=false")
	}

	// Проверка: в точке аккумуляции модифицированная эрозия < базовой
	if result.ModifiedErosion[2] >= result.BaselineErosion[2] {
		t.Errorf("Accumulating point should have modified erosion < baseline, got %.2f >= %.2f",
			result.ModifiedErosion[2], result.BaselineErosion[2])
	}

	t.Logf("Accumulation test:")
	t.Logf("  Point 2 (low energy):")
	t.Logf("    Baseline erosion: %.2f m", result.BaselineErosion[2])
	t.Logf("    Modified erosion: %.2f m", result.ModifiedErosion[2])
	t.Logf("    Deposited volume: %.2f m³/m", result.States[2].LocalBudget.DepositedVolume)
	t.Logf("    Is accumulating: %v", result.States[2].IsAccumulating)
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
		{Class: "clay", Resistance: 1.0, Color: "#c4a484"},      // мягкая
		{Class: "serpentinite", Resistance: 9.0, Color: "#2d2d2d"}, // твёрдая
	}

	params := SedimentTransportParameters{
		TransportCoefficient:     0.7,
		DepositionRate:            0.5,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	result := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	// Объём эрозии для мягкой породы > для твёрдой
	erodedSoft := result.States[0].LocalBudget.ErodedVolume
	erodedHard := result.States[1].LocalBudget.ErodedVolume

	if erodedSoft <= erodedHard {
		t.Errorf("Soft rock should erode more than hard rock, got %.2f <= %.2f",
			erodedSoft, erodedHard)
	}

	t.Logf("Lithology effect:")
	t.Logf("  Clay (R=1.0): eroded %.2f m³/m", erodedSoft)
	t.Logf("  Serpentinite (R=9.0): eroded %.2f m³/m", erodedHard)
	t.Logf("  Ratio: %.2fx", erodedSoft/erodedHard)
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
		TransportCoefficient:     0.8,  // высокий транспорт
		DepositionRate:            0.3,  // низкая депозиция
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.9,  // сильный drift
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
		t.Error("Expected incoming sediment transport, got none")
	}

	if !hasOutgoingTransport {
		t.Error("Expected outgoing sediment transport, got none")
	}

	t.Logf("Longshore drift:")
	t.Logf("  Total transport: %.2f m³/m", result.TotalBudget.TransportVolume)
	t.Logf("  Total deposited: %.2f m³/m", result.TotalBudget.DepositedVolume)
	t.Logf("  Has incoming: %v", hasIncomingTransport)
	t.Logf("  Has outgoing: %v", hasOutgoingTransport)
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
		t.Errorf("Point 0 should not be modified, got %.2f != %.2f",
			modified[0], baseErosion[0])
	}

	// Точка 1: уменьшена на депозицию
	expectedModified := baseErosion[1] - 1.5
	if math.Abs(modified[1]-expectedModified) > 0.01 {
		t.Errorf("Point 1 should be modified to %.2f, got %.2f",
			expectedModified, modified[1])
	}

	t.Logf("Sediment modification:")
	t.Logf("  Point 0: %.2f → %.2f (no change)", baseErosion[0], modified[0])
	t.Logf("  Point 1: %.2f → %.2f (reduced by deposition)", baseErosion[1], modified[1])
}

// BenchmarkCalculateSedimentTransport бенчмарк для производительности
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
		TransportCoefficient:     0.7,
		DepositionRate:            0.5,
		CapacityFactor:            1.0,
		LongshoreDriftCoefficient: 0.8,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)
	}
}