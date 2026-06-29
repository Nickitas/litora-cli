package geometry

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// TemporalParameters определяет временные параметры для модели эрозии
type TemporalParameters struct {
	// YearsPerStep - количество лет за один шаг модели
	// Определяет временной масштаб моделирования
	YearsPerStep float64

	// StormProbability - вероятность штормового события за один шаг [0-1]
	// Штормы усиливают эрозию в краткосрочном периоде
	StormProbability float64

	// StormIntensityMult - множитель силы шторма [1.0-5.0]
	// Определяет насколько шторм сильнее обычной эрозии
	StormIntensityMult float64

	// SeaLevelRise - подъём уровня моря (м/год)
	// Положительное значение = уровень моря растёт, усиливая эрозию
	SeaLevelRise float64

	// Seasonality - учитывать сезонность колебаний
	// Зимние штормы vs летний штиль
	Seasonality bool

	// SeasonalPhase - фаза сезонности (радианы)
	// Позволяет сдвинуть пик штормов (зима vs лето)
	SeasonalPhase float64

	// MinYearsPerStep - минимальное значение для валидации
	MinYearsPerStep float64

	// MaxYearsPerStep - максимальное значение для валидации
	MaxYearsPerStep float64
}

// TemporalState состояние временной динамики для одного шага
type TemporalState struct {
	Step           int       // номер шага
	Year           float64   // текущий год
	IsStorm        bool      // штормовое событие
	StormIntensity float64   // интенсивность шторма [1.0+]
	SeasonalFactor float64   // сезонный множитель [0.5-1.5]
	SeaLevelOffset float64   // смещение уровня моря (м)
	EffectiveYears float64   // эффективное число лет для этого шага
}

// TemporalResult результат моделирования с временной динамикой
type TemporalResult struct {
	Snapshots       [][]LatLon       // состояния береговой линии по шагам
	TemporalStates  []TemporalState  // временные состояния по шагам
	TotalYears      float64          // общее число промоделированных лет
	StormCount      int              // число штормовых событий
	AccumulatedErosion float64       // накопленная эрозия (м)
	FinalSeaLevelRise float64        // итоговый подъём уровня моря (м)
}

// ErosionMetrics метрики эрозии для каждого шага
type ErosionMetrics struct {
	Step                int      // номер шага
	Year                float64  // год
	LengthKm            float64  // длина береговой линии (км)
	AreaKm2             float64  // площадь (км²)
	ErodedM3            float64  // объём эрозии (м³)
	DepositedM3         float64  // объём депозиции (м³)
	NetChangeM3         float64  // баланс (м³)
	FractalDimension    float64  // фрактальная размерность
	MeanRetreatMeters   float64  // среднее отступание (м)
	MaxRetreatMeters    float64  // максимальное отступание (м)
	IsStorm             bool     // штормовое событие
	SeasonalFactor       float64  // сезонный множитель
}

// normalizeTemporalParameters нормализует временные параметры
func normalizeTemporalParameters(params TemporalParameters) TemporalParameters {
	if params.YearsPerStep <= 0 {
		params.YearsPerStep = 1.0 // default: 1 год за шаг
	}
	if params.StormProbability < 0 {
		params.StormProbability = 0
	}
	if params.StormProbability > 1 {
		params.StormProbability = 1
	}
	if params.StormIntensityMult <= 1.0 {
		params.StormIntensityMult = 2.0 // default: шторм в 2 раза сильнее
	}
	if params.StormIntensityMult > 10.0 {
		params.StormIntensityMult = 10.0 // reasonable cap
	}
	if params.SeaLevelRise < 0 {
		params.SeaLevelRise = 0 // no sea level drop
	}
	if params.SeasonalPhase < 0 {
		params.SeasonalPhase = 0
	}
	if params.SeasonalPhase > 2*math.Pi {
		params.SeasonalPhase = 2 * math.Pi
	}

	// Валидация диапазона YearsPerStep
	if params.MinYearsPerStep <= 0 {
		params.MinYearsPerStep = 0.1 // minimum: 0.1 года
	}
	if params.MaxYearsPerStep <= params.MinYearsPerStep {
		params.MaxYearsPerStep = 10.0 // maximum: 10 лет за шаг
	}

	return params
}

// calculateSeasonalFactor рассчитывает сезонный множитель
// Формула: seasonalFactor = 1.0 + 0.5×sin(2π × year + phase)
// Результат: [0.5, 1.5] - сезонные колебания эрозии
func calculateSeasonalFactor(year float64, phase float64) float64 {
	// Нормализуем фазу
	phase = math.Mod(phase, 2*math.Pi)

	// Сезонная компонента: sin(2π × year + phase)
	// year определяет позицию в годовом цикле
	seasonalComponent := math.Sin(2*math.Pi*year + phase)

	// Конвертируем в множитель [0.5, 1.5]
	seasonalFactor := 1.0 + 0.5*seasonalComponent

	return seasonalFactor
}

// calculateTemporalState рассчитывает временное состояние для шага
func calculateTemporalState(step int, params TemporalParameters, rng *rand.Rand) TemporalState {
	state := TemporalState{
		Step: step,
		Year: float64(step) * params.YearsPerStep,
	}

	// Эффективное число лет (может варьироваться)
	state.EffectiveYears = params.YearsPerStep

	// Штормовое событие
	if params.StormProbability > 0 {
		// Проверка шторма
		if rng.Float64() < params.StormProbability {
			state.IsStorm = true

			// Интенсивность шторма: базовый множитель + вариация
			baseIntensity := params.StormIntensityMult
			variation := 0.5 * rng.NormFloat64() // ±50% вариация

			state.StormIntensity = math.Max(1.0, baseIntensity+variation)
		}
	}

	// Сезонность
	if params.Seasonality {
		state.SeasonalFactor = calculateSeasonalFactor(state.Year, params.SeasonalPhase)
	} else {
		state.SeasonalFactor = 1.0
	}

	// Подъём уровня моря
	if params.SeaLevelRise > 0 {
		state.SeaLevelOffset = state.Year * params.SeaLevelRise
	}

	return state
}

// applyTemporalModulation применяет временную модуляцию к эрозии
func applyTemporalModulation(
	baseErosion float64,
	state TemporalState,
) float64 {
	modulated := baseErosion

	// Штормовая модуляция
	if state.IsStorm {
		modulated *= state.StormIntensity
	}

	// Сезонная модуляция
	modulated *= state.SeasonalFactor

	// Модуляция от подъёма уровня моря
	// Чем выше уровень моря, тем сильнее эрозия
	if state.SeaLevelOffset > 0 {
		// Эмпирическая формула: 1 + 0.1 × логарифм от подъёма
		seaLevelFactor := 1.0 + 0.1*math.Log(1.0+state.SeaLevelOffset)
		modulated *= seaLevelFactor
	}

	return modulated
}

// SimulateErosionWithDuration моделирует эрозию с временными параметрами
func SimulateErosionWithDuration(
	points []LatLon,
	targetYears int,
	params TemporalParameters,
	options WaveErosionOptions,
) TemporalResult {
	return SimulateErosionWithDurationSeed(points, targetYears, params, options, time.Now().UnixNano())
}

// SimulateErosionWithDurationSeed детерминистская версия с seed
func SimulateErosionWithDurationSeed(
	points []LatLon,
	targetYears int,
	params TemporalParameters,
	options WaveErosionOptions,
	seed int64,
) TemporalResult {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	// Нормализация
	params = normalizeTemporalParameters(params)

	// Расчёт числа шагов
	numSteps := int(math.Ceil(float64(targetYears) / params.YearsPerStep))
	if numSteps < 1 {
		numSteps = 1
	}

	// Инициализация
	result := TemporalResult{
		Snapshots:      make([][]LatLon, numSteps+1),
		TemporalStates: make([]TemporalState, numSteps+1),
		TotalYears:     float64(numSteps) * params.YearsPerStep,
	}

	// Начальное состояние
	current := clonePoints(points)
	result.Snapshots[0] = current
	result.TemporalStates[0] = TemporalState{Step: 0, Year: 0, EffectiveYears: 0}

	// Генератор случайных чисел
	rng := rand.New(rand.NewSource(seed))

	// Моделирование по шагам
	for step := 1; step <= numSteps; step++ {
		// Рассчитать временное состояние
		state := calculateTemporalState(step, params, rng)
		result.TemporalStates[step] = state

		// Статистика штормов
		if state.IsStorm {
			result.StormCount++
		}

		// Модулированная эрозия
		modulatedOptions := options

		// Применить временную модуляцию к силе эрозии
		modulatedStrength := applyTemporalModulation(options.StrengthMeters, state)
		modulatedOptions.StrengthMeters = modulatedStrength

		// Добавить эффект уровня моря (опционально можно менять базовую линию)
		if state.SeaLevelOffset > 0 {
			// Можно модифицировать батиметрию или другие параметры
			// Здесь простая модуляция через силу эрозии уже учтена
		}

		// Шаг эрозии
		current = waveErodeStep(current, modulatedOptions, seed, step)
		result.Snapshots[step] = current

		// Накопленная эрозия (упрощённая оценка)
		if step > 0 {
			prevLen := PolylineLength(result.Snapshots[step-1])
			currLen := PolylineLength(current)
			result.AccumulatedErosion += (prevLen - currLen) * 1000 // км → м
		}
	}

	// Итоговый подъём уровня моря
	result.FinalSeaLevelRise = result.TotalYears * params.SeaLevelRise

	return result
}

// CalculateErosionMetrics рассчитывает метрики эрозии для всех шагов
func CalculateErosionMetrics(result TemporalResult) []ErosionMetrics {
	metrics := make([]ErosionMetrics, len(result.Snapshots))

	for i, snapshot := range result.Snapshots {
		state := TemporalState{}
		if i < len(result.TemporalStates) {
			state = result.TemporalStates[i]
		}

		metric := ErosionMetrics{
			Step:          i,
			Year:          state.Year,
			LengthKm:      PolylineLength(snapshot),
			AreaKm2:       Area(snapshot),
			IsStorm:       state.IsStorm,
			SeasonalFactor: state.SeasonalFactor,
		}

		// Фрактальная размерность (если есть достаточно точек)
		if len(snapshot) > 10 {
			metric.FractalDimension = fractalDimensionBoxCounting(snapshot, 10)
		}

		// Среднее и максимальное отступание
		if i > 0 && len(result.Snapshots[i-1]) > 0 && len(snapshot) > 0 {
			retreats := calculateRetreatMeters(result.Snapshots[i-1], snapshot)
			if len(retreats) > 0 {
				metric.MeanRetreatMeters = mean(retreats)
				metric.MaxRetreatMeters = max(retreats)
			}
		}

		metrics[i] = metric
	}

	return metrics
}

// calculateRetreatMeters рассчитывает отступание для каждой точки
func calculateRetreatMeters(prev, current []LatLon) []float64 {
	if len(prev) != len(current) || len(prev) == 0 {
		return nil
	}

	retreats := make([]float64, len(prev))

	for i := range prev {
		// Расстояние между соответствующими точками
		dist := Haversine(prev[i], current[i])
		retreats[i] = dist
	}

	return retreats
}

// GetTemporalSummary возвращает сводку временной динамики
func GetTemporalSummary(result TemporalResult) map[string]interface{} {
	summary := make(map[string]interface{})

	summary["total_years"] = result.TotalYears
	summary["total_steps"] = len(result.Snapshots) - 1
	summary["storm_count"] = result.StormCount
	summary["storm_frequency"] = float64(result.StormCount) / float64(len(result.Snapshots)-1)
	summary["accumulated_erosion_m"] = result.AccumulatedErosion
	summary["sea_level_rise_m"] = result.FinalSeaLevelRise

	// Метрики начального и конечного состояния
	if len(result.Snapshots) > 0 {
		initialLen := PolylineLength(result.Snapshots[0])
		finalLen := PolylineLength(result.Snapshots[len(result.Snapshots)-1])
		summary["initial_length_km"] = initialLen
		summary["final_length_km"] = finalLen
		summary["length_change_km"] = finalLen - initialLen
		summary["length_change_percent"] = ((finalLen - initialLen) / initialLen) * 100
	}

	return summary
}

// ValidateTemporalParameters валидирует временные параметры
func ValidateTemporalParameters(params TemporalParameters) []string {
	warnings := []string{}

	if params.YearsPerStep < params.MinYearsPerStep {
		warnings = append(warnings,
			fmt.Sprintf("YearsPerStep %.2f < minimum %.2f",
				params.YearsPerStep, params.MinYearsPerStep))
	}

	if params.YearsPerStep > params.MaxYearsPerStep {
		warnings = append(warnings,
			fmt.Sprintf("YearsPerStep %.2f > maximum %.2f",
				params.YearsPerStep, params.MaxYearsPerStep))
	}

	if params.StormProbability > 0.5 {
		warnings = append(warnings,
			fmt.Sprintf("High storm probability %.2f (unrealistic for most climates)",
				params.StormProbability))
	}

	if params.SeaLevelRise > 0.01 {
		warnings = append(warnings,
			fmt.Sprintf("High sea level rise %.4f m/year (exceeds IPCC RCP8.5)",
				params.SeaLevelRise))
	}

	return warnings
}

// Utility functions
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func max(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	maximum := values[0]
	for _, v := range values {
		if v > maximum {
			maximum = v
		}
	}
	return maximum
}

// fractalDimensionBoxCounting calculates fractal dimension using box-counting
func fractalDimensionBoxCounting(points []LatLon, maxScales int) float64 {
	if len(points) < 4 {
		return 1.0 // minimum dimension for line
	}

	// Simple implementation: calculate dimension using scale variation
	// This is a simplified approach for performance
	scales := []int{2, 4, 8, 16, 32}
	if len(scales) > maxScales {
		scales = scales[:maxScales]
	}

	logScales := make([]float64, 0)
	logCounts := make([]float64, 0)

	for _, scale := range scales {
		count := countBoxes(points, scale)
		if count > 0 {
			logScales = append(logScales, math.Log(float64(scale)))
			logCounts = append(logCounts, math.Log(float64(count)))
		}
	}

	if len(logScales) < 2 {
		return 1.0
	}

	// Linear regression to estimate dimension
	n := float64(len(logScales))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i := range logScales {
		sumX += logScales[i]
		sumY += logCounts[i]
		sumXY += logScales[i] * logCounts[i]
		sumX2 += logScales[i] * logScales[i]
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	dimension := -slope // D = -slope

	return math.Max(1.0, math.Min(dimension, 2.0)) // constrain to [1, 2]
}

func countBoxes(points []LatLon, scale int) int {
	if len(points) < 2 {
		return 0
	}

	// Find bounds
	minLat, maxLat := points[0].Lat, points[0].Lat
	minLon, maxLon := points[0].Lon, points[0].Lon

	for _, p := range points {
		if p.Lat < minLat {
			minLat = p.Lat
		}
		if p.Lat > maxLat {
			maxLat = p.Lat
		}
		if p.Lon < minLon {
			minLon = p.Lon
		}
		if p.Lon > maxLon {
			maxLon = p.Lon
		}
	}

	// Simple box counting
	latRange := maxLat - minLat
	lonRange := maxLon - minLon

	if latRange <= 0 || lonRange <= 0 {
		return 0
	}

	// Calculate box size
	boxSizeLat := latRange / float64(scale)
	boxSizeLon := lonRange / float64(scale)

	// Count occupied boxes
	occupied := make(map[string]bool)
	for _, p := range points {
		boxLat := int((p.Lat - minLat) / boxSizeLat)
		boxLon := int((p.Lon - minLon) / boxSizeLon)
		key := fmt.Sprintf("%d_%d", boxLat, boxLon)
		occupied[key] = true
	}

	return len(occupied)
}
