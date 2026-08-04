package geometry

import (
	"math"
	"testing"
)

func TestCalculateDimensionStability(t *testing.T) {
	// Создаём тестовые метрики с стабильной размерностью
	metrics := []ErosionMetrics{
		{FractalDimension: 1.2},
		{FractalDimension: 1.21},
		{FractalDimension: 1.19},
		{FractalDimension: 1.2},
	}

	stability := calculateDimensionStability(metrics)

	// Стабильность должна быть высокой (> 0.9) для малых вариаций
	if stability < 0.9 {
		t.Errorf("Ожидаемая высокая стабильность > 0,9 для стабильных размеров, получено %.3f", stability)
	}

	// Тест с нестабильной размерностью
	unstableMetrics := []ErosionMetrics{
		{FractalDimension: 1.0},
		{FractalDimension: 1.5},
		{FractalDimension: 1.2},
		{FractalDimension: 1.8},
		{FractalDimension: 1.1},
		{FractalDimension: 1.7},
	}

	unstableStability := calculateDimensionStability(unstableMetrics)

	// Нестабильная размерность должна давать умеренную стабильность
	if unstableStability > 0.7 {
		t.Errorf("Ожидаемая умеренная стабильность < 0,7 для нестабильных размеров, получено %.3f", unstableStability)
	}

	// Но должна быть ниже стабильности стабильной размерности
	if unstableStability >= stability {
		t.Errorf("Нестабильная стабильность (%.3f) должна быть < стабильной (%.3f)", unstableStability, stability)
	}
}

func TestCalculateSpatialAutocorrelation(t *testing.T) {
	// Создаём тестовые метрики с позитивной корреляцией
	metrics := []ErosionMetrics{
		{LengthKm: 100.0},
		{LengthKm: 102.0},
		{LengthKm: 104.0},
		{LengthKm: 106.0},
		{LengthKm: 108.0},
	}

	autocorr := calculateSpatialAutocorrelation(metrics)

	// Позитивный тренд должен давать положительную корреляцию
	if autocorr < 0 {
		t.Errorf("Ожидаемая положительная автокорреляция для растущего тренда, %.3f", autocorr)
	}

	// Тест с осциллирующими значениями
	oscillatingMetrics := []ErosionMetrics{
		{LengthKm: 100.0},
		{LengthKm: 110.0},
		{LengthKm: 100.0},
		{LengthKm: 110.0},
		{LengthKm: 100.0},
	}

	oscillatingAutocorr := calculateSpatialAutocorrelation(oscillatingMetrics)

	// Осцилляция должна давать отрицательную корреляцию
	if oscillatingAutocorr > 0 {
		t.Errorf("Ожидаемая отрицательная автокорреляция для колеблющихся значений, получено %.3f", oscillatingAutocorr)
	}
}

func TestCalculateConvergenceRate(t *testing.T) {
	// Создаём метрики с сходящейся моделью (замедляющиеся изменения)
	convergingMetrics := []ErosionMetrics{
		{LengthKm: 100.0},
		{LengthKm: 95.0}, // изменение 5
		{LengthKm: 91.0}, // изменение 4
		{LengthKm: 88.0}, // изменение 3
		{LengthKm: 86.0}, // изменение 2
	}

	convergenceRate := calculateConvergenceRate(convergingMetrics)

	// Сходящаяся модель должна давать высокую конвергенцию
	if convergenceRate < 0.5 {
		t.Errorf("Ожидаемая высокая скорость сходимости > 0,5 для сходящейся модели, получено %.3f", convergenceRate)
	}

	// Тест с расходящейся моделью (экспоненциальные изменения)
	divergingMetrics := []ErosionMetrics{
		{LengthKm: 100.0},
		{LengthKm: 90.0}, // изменение 10
		{LengthKm: 70.0}, // изменение 20
		{LengthKm: 40.0}, // изменение 30
		{LengthKm: 0.0},  // изменение 40
	}

	divergenceRate := calculateConvergenceRate(divergingMetrics)

	// Расходящаяся модель должна дать более низкую конвергенцию, чем сходящаяся
	if divergenceRate >= convergenceRate {
		t.Errorf("Сходимость расходящейся модели (%.3f) должна быть < сходящаяся (%.3f)",
			divergenceRate, convergenceRate)
	}
}

func TestCalculateMoransI(t *testing.T) {
	// Тест с позитивной пространственной корреляцией
	clusteringMetrics := []ErosionMetrics{
		{LengthKm: 100.0},
		{LengthKm: 102.0},
		{LengthKm: 104.0},
		{LengthKm: 106.0},
		{LengthKm: 108.0},
	}

	moransI := calculateMoransI(clusteringMetrics)

	// Кластеризация (позитивный тренд) должна давать Moran's I > 0
	if moransI < 0 {
		t.Errorf("Ожидаемый положительный I по шкале Морана для модели кластеризации, получено %.3f", moransI)
	}

	// Moran's I должен быть в диапазоне [-1, 1]
	if moransI < -1 || moransI > 1 {
		t.Errorf("Показатель Морана I вне диапазона [-1, 1]: %.3f", moransI)
	}
}

func TestCalculateModelQualityMetrics(t *testing.T) {
	// Создаём тестовые данные эрозии
	erosionMetrics := []ErosionMetrics{
		{
			Step:             0,
			FractalDimension: 1.2,
			LengthKm:         100.0,
			NetChangeM3:      -50.0,
		},
		{
			Step:             1,
			FractalDimension: 1.21,
			LengthKm:         98.0,
			NetChangeM3:      -48.0,
		},
		{
			Step:             2,
			FractalDimension: 1.19,
			LengthKm:         96.0,
			NetChangeM3:      -46.0,
		},
	}

	// Создаём тестовые данные транспорта наносов
	sedimentResult := SedimentTransportResult{
		TotalBudget: SedimentBudget{
			ErodedVolume:    100.0,
			DepositedVolume: 95.0,
			NetChange:       5.0,
		},
		MassBalance: 0.05, // нормализованный баланс массы (5%)
	}

	// Рассчитываем метрики качества
	metrics := CalculateModelQualityMetrics(erosionMetrics, sedimentResult)

	// Проверяем, что все метрики рассчитаны
	if metrics.DimensionStability < 0 || metrics.DimensionStability > 1 {
		t.Errorf("Стабильность размеров вне диапазона [0, 1]: %.3f", metrics.DimensionStability)
	}

	// MassBalance может быть любым, но должен быть рассчитан
	if metrics.MassBalance == 0 && sedimentResult.TotalBudget.NetChange != 0 {
		t.Errorf("Неправильно рассчитан баланс массы: %.3f", metrics.MassBalance)
	}

	// SpatialAutocorr должен быть в диапазоне [-1, 1]
	if metrics.SpatialAutocorr < -1 || metrics.SpatialAutocorr > 1 {
		t.Errorf("Пространственная автокоррекция вне диапазона [-1, 1]: %.3f", metrics.SpatialAutocorr)
	}

	// ConvergenceRate должен быть в диапазоне [0, 1]
	if metrics.ConvergenceRate < 0 || metrics.ConvergenceRate > 1 {
		t.Errorf("Коэффициент сходимости вне диапазона [0, 1]: %.3f", metrics.ConvergenceRate)
	}
}

func TestValidateModelQuality(t *testing.T) {
	// Тест валидной модели
	validMetrics := ModelQualityMetrics{
		DimensionStability: 0.85,
		MassBalance:        0.05,
		SpatialAutocorr:    0.4,
		ConvergenceRate:    0.7,
	}

	isValid := validateModelQuality(validMetrics)
	if !isValid {
		t.Error("Ожидаемая валидность модели с хорошими показателями")
	}

	// Тест невалидной модели (низкая стабильность размерности)
	invalidMetrics := ModelQualityMetrics{
		DimensionStability: 0.5,
		MassBalance:        0.05,
		SpatialAutocorr:    0.4,
		ConvergenceRate:    0.7,
	}

	isValid = validateModelQuality(invalidMetrics)
	if isValid {
		t.Error("Ожидаемая модель будет недействительной из-за низкой стабильности размеров")
	}

	// Тест невалидной модели (плохой баланс массы)
	poorBalanceMetrics := ModelQualityMetrics{
		DimensionStability: 0.85,
		MassBalance:        0.2,
		SpatialAutocorr:    0.4,
		ConvergenceRate:    0.7,
	}

	isValid = validateModelQuality(poorBalanceMetrics)
	if isValid {
		t.Error("Ожидаемая модель будет недействительной из-за плохого баланса массы")
	}
}

func TestGenerateValidationWarnings(t *testing.T) {
	// Тест с хорошими метриками (без предупреждений)
	goodMetrics := ModelQualityMetrics{
		DimensionStability:    0.85,
		MassBalance:           0.05,
		SpatialAutocorr:       0.4,
		ConvergenceRate:       0.7,
		DimensionVariance:     0.01,
		MassBalanceTrend:      0.1,
		SedimentTransportRate: 50.0, // достаточно для избежания warning
		AccumulationIndex:     0.5,  // в допустимом диапазоне
	}

	warnings := generateValidationWarnings(goodMetrics)
	if len(warnings) > 0 {
		t.Errorf("Не ожидал никаких предупреждений о хороших показателях, получил %d: %v", len(warnings), warnings)
	}

	// Тест с проблемными метриками (много предупреждений)
	poorMetrics := ModelQualityMetrics{
		DimensionStability: 0.5,
		MassBalance:        0.2,
		SpatialAutocorr:    -0.5,
		ConvergenceRate:    0.3,
		DimensionVariance:  0.15,
		MassBalanceTrend:   1.5,
	}

	warnings = generateValidationWarnings(poorMetrics)
	if len(warnings) == 0 {
		t.Error("Ожидаемые предупреждения о плохих показателях")
	}

	// Проверяем наличие конкретных предупреждений
	hasDimensionWarning := false
	hasBalanceWarning := false
	hasConvergenceWarning := false

	for _, w := range warnings {
		if contains(w, "стабильность размер") {
			hasDimensionWarning = true
		}
		if contains(w, "баланс массы") {
			hasBalanceWarning = true
		}
		if contains(w, "сходимост") {
			hasConvergenceWarning = true
		}
	}

	if !hasDimensionWarning {
		t.Error("Ожидаемое предупреждение о низкой стабильности размеров")
	}
	if !hasBalanceWarning {
		t.Error("Ожидаемое предупреждение о плохом балансе массы")
	}
	if !hasConvergenceWarning {
		t.Error("Ожидаемое предупреждение о низкой скорости конвергенции")
	}
}

func TestCalculateTimeSeriesMetrics(t *testing.T) {
	// Создаём тестовые метрики
	erosionMetrics := []ErosionMetrics{
		{Step: 0, FractalDimension: 1.2, LengthKm: 100.0, NetChangeM3: -50.0},
		{Step: 1, FractalDimension: 1.21, LengthKm: 98.0, NetChangeM3: -48.0},
		{Step: 2, FractalDimension: 1.19, LengthKm: 96.0, NetChangeM3: -46.0},
		{Step: 3, FractalDimension: 1.2, LengthKm: 95.0, NetChangeM3: -45.0},
	}

	// Рассчитываем временные ряды
	ts := CalculateTimeSeriesMetrics(erosionMetrics)

	// Проверяем, что все массивы имеют правильную длину
	if len(ts.Steps) != len(erosionMetrics) {
		t.Errorf("Ожидаемые %d шаги, получено %d", len(erosionMetrics), len(ts.Steps))
	}

	if len(ts.Dimensions) != len(erosionMetrics) {
		t.Errorf("Ожидаемые размеры %d, полученные %d", len(erosionMetrics), len(ts.Dimensions))
	}

	if len(ts.MassBalances) != len(erosionMetrics) {
		t.Errorf("Ожидаемый %d баланс массы, полученный %d", len(erosionMetrics), len(ts.MassBalances))
	}

	// Проверяем, что значения корректны
	for i, step := range ts.Steps {
		if step != erosionMetrics[i].Step {
			t.Errorf("Несоответствие шага по индексу %d: ожидаемый %d, полученный %d",
				i, erosionMetrics[i].Step, step)
		}
	}
}

func TestValidateModelConvergence(t *testing.T) {
	// Тест сходящейся модели
	convergingTs := ValidationMetricsTimeSeries{
		ConvergenceRates: []float64{0.5, 0.6, 0.7, 0.75, 0.8},
		Dimensions:       []float64{1.2, 1.21, 1.19, 1.2, 1.2},
	}

	isConvergent, warnings := ValidateModelConvergence(convergingTs)
	if !isConvergent {
		t.Error("Ожидается, что модель будет конвергентной с увеличением скорости конвергенции")
	}
	if len(warnings) > 0 {
		t.Errorf("Не ожидал никаких предупреждений для конвергентной модели, получил %d: %v", len(warnings), warnings)
	}

	// Тест расходящейся модели
	divergingTs := ValidationMetricsTimeSeries{
		ConvergenceRates: []float64{0.9, 0.7, 0.5, 0.3, 0.1}, // сильное снижение
		Dimensions:       []float64{1.2, 1.3, 1.4, 1.5, 1.6}, // большая вариация
	}

	isConvergent, warnings = ValidateModelConvergence(divergingTs)
	if isConvergent {
		t.Error("Ожидаемая модель будет неконвергентной с уменьшающимися скоростями конвергенции")
	}
	if len(warnings) == 0 {
		t.Error("Ожидаемые предупреждения для расходящейся модели")
	}

	// Проверяем наличие предупреждений
	hasTrendWarning := false
	hasDimensionWarning := false

	for _, w := range warnings {
		if contains(w, "тренд") || contains(w, "расход") {
			hasTrendWarning = true
		}
		if contains(w, "размер") || contains(w, "нестабил") {
			hasDimensionWarning = true
		}
	}

	// Должно быть хотя бы одно предупреждение
	if !hasTrendWarning && !hasDimensionWarning {
		t.Error("Ожидается по крайней мере одно предупреждение о тренде или измерении")
	}
}

func TestGetQualityMetricsSummary(t *testing.T) {
	metrics := ModelQualityMetrics{
		DimensionStability: 0.85,
		MassBalance:        0.05,
		SpatialAutocorr:    0.4,
		ConvergenceRate:    0.7,
		DimensionVariance:  0.01,
		MassBalanceTrend:   0.1,
		SpatialCorrelation: 0.3,
		IsValidModel:       true,
		Warnings:           []string{"test warning"},
	}

	summary := GetQualityMetricsSummary(metrics)

	// Проверяем наличие всех полей
	expectedFields := []string{
		"dimension_stability",
		"mass_balance",
		"spatial_autocorr",
		"convergence_rate",
		"dimension_variance",
		"mass_balance_trend",
		"spatial_correlation_morans_i",
		"is_valid_model",
		"warnings",
	}

	for _, field := range expectedFields {
		if _, exists := summary[field]; !exists {
			t.Errorf("Missing field in summary: %s", field)
		}
	}

	// Проверяем значения
	if summary["dimension_stability"].(float64) != 0.85 {
		t.Errorf("несоответствие dimension_stability: ожидалось 0.85, получено %.3f",
			summary["dimension_stability"].(float64))
	}

	if !summary["is_valid_model"].(bool) {
		t.Error("is_valid_model должно быть true")
	}

	warnings, ok := summary["warnings"].([]string)
	if !ok || len(warnings) != 1 {
		t.Error("поле warnings отсутствует или некорректно")
	}
}

func TestCalculateLag1Autocorrelation(t *testing.T) {
	// Тест с постоянными значениями (корреляция = 1)
	constantValues := []float64{100.0, 100.0, 100.0, 100.0}
	autocorr := calculateLag1Autocorrelation(constantValues)
	if math.Abs(autocorr-1.0) >= 0.01 {
		t.Errorf("Ожидалась автокорреляция ≈ 1.0 для постоянных значений, получено %.3f", autocorr)
	}

	// Тест с линейным ростом (умеренная положительная корреляция)
	growingValues := []float64{100.0, 102.0, 104.0, 106.0, 108.0}
	autocorr = calculateLag1Autocorrelation(growingValues)
	if autocorr < 0.4 {
		t.Errorf("Ожидалась умеренная положительная автокорреляция для растущих значений, получено %.3f", autocorr)
	}

	// Тест с осцилляцией (отрицательная корреляция)
	oscillatingValues := []float64{100.0, 110.0, 100.0, 110.0, 100.0}
	autocorr = calculateLag1Autocorrelation(oscillatingValues)
	if autocorr > 0 {
		t.Errorf("Ожидалась отрицательная автокорреляция для колеблющихся значений, получено %.3f", autocorr)
	}
}

func TestCalculateLinearTrend(t *testing.T) {
	// Тест с линейным ростом (положительный тренд)
	growingValues := []float64{100.0, 102.0, 104.0, 106.0, 108.0}
	trend := calculateLinearTrend(growingValues)
	if trend <= 0 {
		t.Errorf("Ожидался положительный тренд для растущих значений, получено %.3f", trend)
	}

	// Тест с линейным спадом (отрицательный тренд)
	decliningValues := []float64{108.0, 106.0, 104.0, 102.0, 100.0}
	trend = calculateLinearTrend(decliningValues)
	if trend >= 0 {
		t.Errorf("Ожидался отрицательный тренд для убывающих значений, получено %.3f", trend)
	}

	// Тест с постоянными значениями (нулевой тренд)
	constantValues := []float64{100.0, 100.0, 100.0, 100.0}
	trend = calculateLinearTrend(constantValues)
	if math.Abs(trend) > 0.01 {
		t.Errorf("Ожидался нулевой тренд для постоянных значений, получено %.3f", trend)
	}
}

// Вспомогательная функция для проверки наличия подстроки
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
