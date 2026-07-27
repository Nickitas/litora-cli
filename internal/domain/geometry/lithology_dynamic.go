package geometry

import (
	"fmt"
	"math"
	"time"
)

// WeatheringProfile профиль выветривания пород
type WeatheringProfile struct {
	// Базовая скорость выветривания (м/год)
	BaseRate float64

	// Коэффициент ускорения выветривания для разных пород
	// Карта: lithology_class -> multiplier
	WeatheringRates map[string]float64

	// Климатический множитель
	ClimateMultiplier float64

	// Глубина зоны выветривания (м)
	WeatheringDepth float64

	// Время стабилизации (лет)
	StabilizationTime float64
}

// DynamicLithologyState динамическое состояние литологии с учётом времени
type DynamicLithologyState struct {
	// Статические свойства
	Static LithologyState

	// Динамические свойства (меняются со временем)
	CurrentResistance  float64 // текущее сопротивление
	WeatheringProgress float64 // прогресс выветривания [0-1]
	AgeYears           float64 // возраст породы (лет)
	Thickness          float64 // толщина слоя (м)
	IsWeathered        bool    // флаг выветривания
	FractureDensity    float64 // плотность трещин [0-1]
	Porosity           float64 // пористость [0-1]
	Saturation         float64 // водонасыщенность [0-1]

	// История изменений
	ModificationHistory []LithologyModification
}

// LithologyModification запись об изменении литологии
type LithologyModification struct {
	Timestamp        time.Time
	OldResistance    float64
	NewResistance    float64
	ModificationType string  // "weathering", "erosion", "deposition", "storm"
	Cause            string  // описание причины
	Magnitude        float64 // величина изменения
}

// LithologyInteractionParams параметры взаимодействия литологии с процессами
type LithologyInteractionParams struct {
	// Erosion-lithology interaction
	ErosionResistanceFactor float64 // насколько сопротивление снижает эрозию [0-1]
	WeatheringErosionBoost  float64 // насколько выветривание ускоряет эрозию [0-1]

	// Deposition-lithology interaction
	DepositionAdhesionFactor    float64 // сцепление отложений с породой [0-1]
	LithologyTrappingEfficiency float64 // эффективность захвата наносов [0-1]

	// Storm impact on lithology
	StormFractureMultiplier float64 // множитель трещинообразования во время штормов
	StormErosionMultiplier  float64 // множитель эрозии во время штормов

	// Spatial variability
	SpatialAutocorrelation float64 // пространственная автокорреляция [0-1]
	HeterogeneityScale     float64 // масштаб неоднородности (км)
	NoiseLevel             float64 // уровень случайных вариаций [0-1]
}

// SpatialLithologyMap пространственная карта литологии
type SpatialLithologyMap struct {
	Points     []DynamicLithologyState
	Bounds     Bounds
	Resolution float64 // км между точками
	Params     LithologyInteractionParams
	Weathering WeatheringProfile
}

// LithologyEvolutionResult результат эволюции литологии
type LithologyEvolutionResult struct {
	InitialState         []DynamicLithologyState
	FinalState           []DynamicLithologyState
	TimeSpanYears        float64
	TotalWeatheringDepth float64
	ResistanceChanges    []float64
	ErosionImpact        []float64
	DepositionImpact     []float64
}

// ApplyWeathering применяет выветривание к породе
func ApplyWeathering(
	state LithologyState,
	years float64,
	weathering WeatheringProfile,
	climateFactor float64,
) DynamicLithologyState {

	dynamic := DynamicLithologyState{
		Static:              state,
		CurrentResistance:   state.Resistance,
		WeatheringProgress:  0.0,
		AgeYears:            years,
		FractureDensity:     0.1,
		Porosity:            0.05,
		Saturation:          0.0,
		ModificationHistory: make([]LithologyModification, 0),
	}

	if years <= 0 {
		return dynamic
	}

	// Базовая скорость выветривания для класса
	baseRate := weathering.BaseRate
	if rate, ok := weathering.WeatheringRates[state.Class]; ok {
		baseRate = rate
	}

	// Климатический множитель
	totalRate := baseRate * weathering.ClimateMultiplier * climateFactor

	// Расчёт прогресса выветривания
	// Экспоненциальная модель: progress = 1 - exp(-rate * time)
	dynamic.WeatheringProgress = 1.0 - math.Exp(-totalRate*years/weathering.StabilizationTime)

	// Снижение сопротивления пропорционально прогрессу
	resistanceLoss := state.Resistance * dynamic.WeatheringProgress * 0.7 // максимум 70% потери
	dynamic.CurrentResistance = state.Resistance - resistanceLoss

	if dynamic.CurrentResistance < 0.1 {
		dynamic.CurrentResistance = 0.1 // минимум
	}

	// Увеличение пористости и трещиноватости
	dynamic.Porosity = 0.05 + dynamic.WeatheringProgress*0.25      // до 30%
	dynamic.FractureDensity = 0.1 + dynamic.WeatheringProgress*0.4 // до 50%

	dynamic.IsWeathered = dynamic.WeatheringProgress > 0.3

	// Запись в историю
	dynamic.ModificationHistory = append(dynamic.ModificationHistory, LithologyModification{
		Timestamp:        time.Now(),
		OldResistance:    state.Resistance,
		NewResistance:    dynamic.CurrentResistance,
		ModificationType: "weathering",
		Cause:            fmt.Sprintf("weathering over %.1f years", years),
		Magnitude:        state.Resistance - dynamic.CurrentResistance,
	})

	return dynamic
}

// CalculateLithologyErosionInteraction рассчитывает влияние литологии на эрозию
func CalculateLithologyErosionInteraction(
	baseErosion float64,
	litho DynamicLithologyState,
	params LithologyInteractionParams,
	isStorm bool,
) float64 {

	modified := baseErosion

	// 1. Сопротивление снижает эрозию
	resistanceFactor := 1.0 / (litho.CurrentResistance * params.ErosionResistanceFactor)
	if resistanceFactor > 1.0 {
		resistanceFactor = 1.0 // ограничиваем влияние
	}
	modified *= resistanceFactor

	// 2. Выветривание увеличивает эрозию
	if litho.IsWeathered {
		modified *= (1.0 + params.WeatheringErosionBoost*litho.WeatheringProgress)
	}

	// 3. Штормовая эрозия с учётом трещиноватости
	if isStorm {
		// Трещиноватость усиливает штормовую эрозию
		stormBoost := params.StormErosionMultiplier * (1.0 + litho.FractureDensity)
		modified *= stormBoost
	}

	return modified
}

// CalculateLithologyDepositionInteraction рассчитывает влияние литологии на аккумуляцию
func CalculateLithologyDepositionInteraction(
	baseDeposition float64,
	litho DynamicLithologyState,
	params LithologyInteractionParams,
	sedimentSupply float64,
) float64 {

	modified := baseDeposition

	// 1. Сцепление с породой
	adhesionEffect := params.DepositionAdhesionFactor * (1.0 - litho.Porosity*0.5)
	// Более пористые породы хуже удерживают отложения

	// 2. Эффективность захвата наносов
	trappingEfficiency := params.LithologyTrappingEfficiency

	// 3. Выветрелые породы с высокой трещиноватостью лучше захватывают наносы
	if litho.IsWeathered && litho.FractureDensity > 0.3 {
		trappingEfficiency *= (1.0 + litho.FractureDensity*0.5)
	}

	// 4. Ограничение по предложению наносов
	if sedimentSupply < modified {
		modified = sedimentSupply
	}

	modified *= (adhesionEffect + trappingEfficiency) / 2.0

	return modified
}

// GenerateSpatialLithologyVariability создаёт пространственную вариабельность литологии
func GenerateSpatialLithologyVariability(
	basePoints []LithologyState,
	bounds Bounds,
	params LithologyInteractionParams,
	weathering WeatheringProfile,
) SpatialLithologyMap {

	n := len(basePoints)
	dynamicPoints := make([]DynamicLithologyState, n)

	// Базовая статистика
	meanResistance := 0.0
	for _, state := range basePoints {
		meanResistance += state.Resistance
	}
	meanResistance /= float64(n)

	stdResistance := 0.0
	for _, state := range basePoints {
		diff := state.Resistance - meanResistance
		stdResistance += diff * diff
	}
	stdResistance = math.Sqrt(stdResistance / float64(n))

	for i, state := range basePoints {
		// Добавляем пространственную корреляцию
		spatialNoise := generateSpatiallyCorrelatedNoise(
			i, n, params.SpatialAutocorrelation, params.NoiseLevel,
		)

		// Модифицируем сопротивление с учётом пространственной вариабельности
		resistanceVariation := spatialNoise * stdResistance * 0.3
		modifiedResistance := state.Resistance + resistanceVariation

		if modifiedResistance < 0.1 {
			modifiedResistance = 0.1
		}

		dynamicPoints[i] = DynamicLithologyState{
			Static: LithologyState{
				Class:       state.Class,
				Resistance:  modifiedResistance,
				Color:       state.Color,
				Description: state.Description,
			},
			CurrentResistance: modifiedResistance,
			FractureDensity:   0.1 + spatialNoise*0.2,
			Porosity:          0.05 + spatialNoise*0.15,
		}
	}

	return SpatialLithologyMap{
		Points:     dynamicPoints,
		Bounds:     bounds,
		Resolution: params.HeterogeneityScale,
		Params:     params,
		Weathering: weathering,
	}
}

// generateSpatiallyCorrelatedNoise генерирует пространственно коррелированный шум
func generateSpatiallyCorrelatedNoise(index, n int, autocorr, noiseLevel float64) float64 {
	// Простая модель авторегрессии AR(1)
	if n <= 1 {
		return 0.0
	}

	// Используем индекс как "пространственную" координату
	normalizedPos := float64(index) / float64(n)

	// Детерминированная компонента (пространственная волна)
	spatialComponent := 0.5*math.Sin(2*math.Pi*normalizedPos*3) +
		0.3*math.Sin(2*math.Pi*normalizedPos*7)

	// Случайная компонента с автокорреляцией
	// Упрощённо: используем хеш-подобную функцию от индекса
	hashLike := float64((index*2654435761)%1000000007) / 1000000007.0
	randomComponent := (hashLike - 0.5) * 2.0 // [-1, 1]

	// Комбинируем с учётом автокорреляции
	result := (spatialComponent*(1-autocorr) + randomComponent*autocorr) * noiseLevel

	// Ограничиваем диапазон [-1, 1]
	if result > 1.0 {
		result = 1.0
	}
	if result < -1.0 {
		result = -1.0
	}

	return result
}

// SimulateLithologyEvolution моделирует эволюцию литологии во времени
func SimulateLithologyEvolution(
	initialStates []LithologyState,
	timeSpanYears float64,
	weathering WeatheringProfile,
	params LithologyInteractionParams,
	erosionHistory []float64, // эрозия по шагам
	stormEvents []int, // индексы штормовых событий
) LithologyEvolutionResult {

	n := len(initialStates)
	dynamicStates := make([]DynamicLithologyState, n)

	// Инициализируем динамические состояния
	climateFactor := 1.0 // можно сделать параметром

	for i, state := range initialStates {
		dynamicStates[i] = ApplyWeathering(state, timeSpanYears, weathering, climateFactor)
	}

	result := LithologyEvolutionResult{
		InitialState:      make([]DynamicLithologyState, n),
		FinalState:        dynamicStates,
		TimeSpanYears:     timeSpanYears,
		ResistanceChanges: make([]float64, n),
		ErosionImpact:     make([]float64, n),
		DepositionImpact:  make([]float64, n),
	}

	for i := range initialStates {
		result.InitialState[i] = DynamicLithologyState{
			Static: initialStates[i],
		}
		result.ResistanceChanges[i] = initialStates[i].Resistance - dynamicStates[i].CurrentResistance
	}

	// Рассчитываем влияние на эрозию
	for i, state := range dynamicStates {
		if i < len(erosionHistory) {
			baseErosion := erosionHistory[i]
			isStorm := false

			// Проверяем, было ли это штормовое событие
			for _, stormIdx := range stormEvents {
				if i == stormIdx {
					isStorm = true
					break
				}
			}

			erosionImpact := CalculateLithologyErosionInteraction(baseErosion, state, params, isStorm)
			result.ErosionImpact[i] = erosionImpact

			// Модифицируем состояние на основе эрозии
			if baseErosion > 0 {
				// Эрозия снижает сопротивление дополнительно
				erosionFactor := baseErosion / 1000.0 // нормируем
				if erosionFactor > 0.1 {
					erosionFactor = 0.1
				}

				state.ModificationHistory = append(state.ModificationHistory, LithologyModification{
					Timestamp:        time.Now(),
					OldResistance:    state.CurrentResistance,
					NewResistance:    state.CurrentResistance * (1.0 - erosionFactor),
					ModificationType: "erosion",
					Cause:            fmt.Sprintf("erosion impact %.2f", baseErosion),
					Magnitude:        state.CurrentResistance * erosionFactor,
				})

				state.CurrentResistance *= (1.0 - erosionFactor)
				state.FractureDensity += erosionFactor * 0.5 // эрозия создаёт трещины
			}
		}
	}

	// Рассчитываем влияние на аккумуляцию
	for i, state := range dynamicStates {
		// Базовая аккумуляция (эмпирическая оценка)
		baseDeposition := 0.1
		if i < len(erosionHistory) {
			baseDeposition = erosionHistory[i] * 0.3
		}

		depositionImpact := CalculateLithologyDepositionInteraction(
			baseDeposition, state, params, baseDeposition*2.0,
		)
		result.DepositionImpact[i] = depositionImpact
	}

	// Общая глубина выветривания
	for _, state := range dynamicStates {
		if state.WeatheringProgress > 0 {
			result.TotalWeatheringDepth += weathering.WeatheringDepth * state.WeatheringProgress
		}
	}

	return result
}

// ApplyStormImpactOnLithology применяет воздействие шторма на литологию
func ApplyStormImpactOnLithology(
	state DynamicLithologyState,
	params LithologyInteractionParams,
	stormIntensity float64,
) DynamicLithologyState {

	// Шторм создаёт новые трещины
	fractureIncrease := params.StormFractureMultiplier * stormIntensity * 0.1
	state.FractureDensity += fractureIncrease

	if state.FractureDensity > 1.0 {
		state.FractureDensity = 1.0
	}

	// Увеличение пористости
	porosityIncrease := stormIntensity * 0.05
	state.Porosity += porosityIncrease

	if state.Porosity > 0.5 {
		state.Porosity = 0.5
	}

	// Снижение сопротивления
	resistanceDecrease := state.CurrentResistance * stormIntensity * 0.1
	oldResistance := state.CurrentResistance
	state.CurrentResistance -= resistanceDecrease

	if state.CurrentResistance < 0.1 {
		state.CurrentResistance = 0.1
	}

	// Запись в историю
	state.ModificationHistory = append(state.ModificationHistory, LithologyModification{
		Timestamp:        time.Now(),
		OldResistance:    oldResistance,
		NewResistance:    state.CurrentResistance,
		ModificationType: "storm",
		Cause:            fmt.Sprintf("storm intensity %.2f", stormIntensity),
		Magnitude:        resistanceDecrease,
	})

	return state
}

// GetLithologyStatistics возвращает статистику по динамической литологии
func GetLithologyStatistics(states []DynamicLithologyState) map[string]interface{} {
	stats := map[string]interface{}{
		"total_points": len(states),
	}

	if len(states) == 0 {
		return stats
	}

	// Resistance statistics
	minR := states[0].CurrentResistance
	maxR := states[0].CurrentResistance
	sumR := 0.0
	sumInitialR := 0.0

	weatheredCount := 0
	totalWeatheringProgress := 0.0
	totalFractureDensity := 0.0
	totalPorosity := 0.0

	for _, state := range states {
		if state.CurrentResistance < minR {
			minR = state.CurrentResistance
		}
		if state.CurrentResistance > maxR {
			maxR = state.CurrentResistance
		}
		sumR += state.CurrentResistance
		sumInitialR += state.Static.Resistance

		if state.IsWeathered {
			weatheredCount++
		}
		totalWeatheringProgress += state.WeatheringProgress
		totalFractureDensity += state.FractureDensity
		totalPorosity += state.Porosity
	}

	n := float64(len(states))

	stats["resistance_min"] = minR
	stats["resistance_max"] = maxR
	stats["resistance_mean"] = sumR / n
	stats["resistance_initial_mean"] = sumInitialR / n
	stats["weathered_fraction"] = float64(weatheredCount) / n
	stats["weathering_progress_mean"] = totalWeatheringProgress / n
	stats["fracture_density_mean"] = totalFractureDensity / n
	stats["porosity_mean"] = totalPorosity / n

	// Потеря сопротивления
	resistanceLoss := ((sumInitialR / n) - (sumR / n)) / (sumInitialR / n) * 100
	stats["resistance_loss_percent"] = resistanceLoss

	return stats
}

// CreateDefaultWeatheringProfile создаёт профиль выветривания по умолчанию
func CreateDefaultWeatheringProfile() WeatheringProfile {
	return WeatheringProfile{
		BaseRate: 0.1, // базовая скорость отн. единиц/год
		WeatheringRates: map[string]float64{
			"limestone":    0.2,  // известняк быстро выветривается
			"granite":      0.05, // гранит медленно
			"sandstone":    0.3,  // песчаник быстро
			"shale":        0.4,  // глины сланцы очень быстро
			"basalt":       0.03, // базальт медленно
			"conglomerate": 0.2,  // конгломераты средне
			"alluvium":     1.0,  // аллювий очень быстро
			"rock":         0.1,  // по умолчанию
		},
		ClimateMultiplier: 1.0,
		WeatheringDepth:   2.0,  // 2 м зона выветривания
		StabilizationTime: 50.0, // 50 лет характерное время выветривания
	}
}

// CreateDefaultLithologyInteractionParams создаёт параметры взаимодействия по умолчанию
func CreateDefaultLithologyInteractionParams() LithologyInteractionParams {
	return LithologyInteractionParams{
		ErosionResistanceFactor:     0.5,
		WeatheringErosionBoost:      0.3,
		DepositionAdhesionFactor:    0.7,
		LithologyTrappingEfficiency: 0.5,
		StormFractureMultiplier:     2.0,
		StormErosionMultiplier:      3.0,
		SpatialAutocorrelation:      0.7,
		HeterogeneityScale:          5.0, // 5 км
		NoiseLevel:                  0.2,
	}
}

// Validate validates the lithology interaction parameters
func (p *LithologyInteractionParams) Validate() error {
	if p.ErosionResistanceFactor < 0 || p.ErosionResistanceFactor > 1 {
		return fmt.Errorf("invalid erosion resistance factor: %.2f", p.ErosionResistanceFactor)
	}
	if p.WeatheringErosionBoost < 0 || p.WeatheringErosionBoost > 1 {
		return fmt.Errorf("invalid weathering erosion boost: %.2f", p.WeatheringErosionBoost)
	}
	if p.StormFractureMultiplier < 1 || p.StormFractureMultiplier > 10 {
		return fmt.Errorf("invalid storm fracture multiplier: %.2f", p.StormFractureMultiplier)
	}
	if p.SpatialAutocorrelation < 0 || p.SpatialAutocorrelation > 1 {
		return fmt.Errorf("invalid spatial autocorrelation: %.2f", p.SpatialAutocorrelation)
	}
	return nil
}

// DynamicWaveErosionOptions расширенные опции эрозии с динамической литологией
type DynamicWaveErosionOptions struct {
	// Базовые опции
	Base WaveErosionOptions

	// Динамическая литология
	DynamicStates []DynamicLithologyState

	// Параметры взаимодействия
	InteractionParams LithologyInteractionParams

	// Профиль выветривания
	Weathering WeatheringProfile

	// Текущее время симуляции (лет)
	SimulationYears float64

	// История штормов (индексы шагов)
	StormHistory []int
}

// ApplyDynamicLithologyToErosion применяет динамическую литологию к расчёту эрозии
func ApplyDynamicLithologyToErosion(
	points []LatLon,
	baseRetreat []float64,
	dynamicMap SpatialLithologyMap,
	simulationYears float64,
	isStormStep bool,
	stormIntensity float64,
) []float64 {

	if len(baseRetreat) == 0 {
		return nil
	}

	n := len(baseRetreat)
	modifiedRetreat := make([]float64, n)

	currentStates := dynamicMap.Points
	if simulationYears > 0 {
		for i := range currentStates {
			weatheredState := ApplyWeathering(
				currentStates[i].Static,
				simulationYears,
				dynamicMap.Weathering,
				1.0,
			)
			currentStates[i] = weatheredState
		}
	}

	for i := 0; i < n && i < len(currentStates); i++ {
		lithoState := currentStates[i]

		if isStormStep {
			lithoState = ApplyStormImpactOnLithology(
				lithoState,
				dynamicMap.Params,
				stormIntensity,
			)
		}

		modifiedRetreat[i] = CalculateLithologyErosionInteraction(
			baseRetreat[i],
			lithoState,
			dynamicMap.Params,
			isStormStep,
		)
	}

	return modifiedRetreat
}

// UpdateLithologyAfterErosion обновляет состояние литологии после эрозии
func UpdateLithologyAfterErosion(
	states []DynamicLithologyState,
	erosionAmounts []float64,
	params LithologyInteractionParams,
) []DynamicLithologyState {

	if len(states) == 0 {
		return states
	}

	updated := make([]DynamicLithologyState, len(states))
	copy(updated, states)

	for i := range updated {
		if i >= len(erosionAmounts) {
			continue
		}

		erosionMeters := erosionAmounts[i]
		if erosionMeters <= 0 {
			continue
		}

		normalizedErosion := erosionMeters / 10.0
		if normalizedErosion > 1.0 {
			normalizedErosion = 1.0
		}

		fractureIncrease := normalizedErosion * params.WeatheringErosionBoost * 0.3
		updated[i].FractureDensity += fractureIncrease
		if updated[i].FractureDensity > 1.0 {
			updated[i].FractureDensity = 1.0
		}

		porosityIncrease := normalizedErosion * 0.1
		updated[i].Porosity += porosityIncrease
		if updated[i].Porosity > 0.5 {
			updated[i].Porosity = 0.5
		}

		resistanceLoss := updated[i].CurrentResistance * normalizedErosion * 0.2
		updated[i].CurrentResistance -= resistanceLoss
		if updated[i].CurrentResistance < 0.1 {
			updated[i].CurrentResistance = 0.1
		}

		updated[i].ModificationHistory = append(updated[i].ModificationHistory, LithologyModification{
			Timestamp:        time.Now(),
			OldResistance:    states[i].CurrentResistance,
			NewResistance:    updated[i].CurrentResistance,
			ModificationType: "erosion_feedback",
			Cause:            fmt.Sprintf("erosion %.2fm", erosionMeters),
			Magnitude:        resistanceLoss,
		})
	}

	return updated
}

// UpdateLithologyAfterDeposition обновляет состояние литологии после аккумуляции
func UpdateLithologyAfterDeposition(
	states []DynamicLithologyState,
	depositionAmounts []float64,
	params LithologyInteractionParams,
) []DynamicLithologyState {

	if len(states) == 0 {
		return states
	}

	updated := make([]DynamicLithologyState, len(states))
	copy(updated, states)

	for i := range updated {
		if i >= len(depositionAmounts) {
			continue
		}

		depositionMeters := depositionAmounts[i]
		if depositionMeters <= 0 {
			continue
		}

		updated[i].Thickness += depositionMeters

		if depositionMeters > 0.5 {
			healingFactor := math.Min(depositionMeters/5.0, 0.3)
			updated[i].FractureDensity *= (1.0 - healingFactor)
			updated[i].Porosity *= (1.0 - healingFactor*0.5)
		}

		updated[i].ModificationHistory = append(updated[i].ModificationHistory, LithologyModification{
			Timestamp:        time.Now(),
			OldResistance:    states[i].CurrentResistance,
			NewResistance:    updated[i].CurrentResistance,
			ModificationType: "deposition",
			Cause:            fmt.Sprintf("deposition %.2fm", depositionMeters),
			Magnitude:        depositionMeters,
		})
	}

	return updated
}

// LithologyErosionStepResult результат одного шага эрозии с учётом литологии
type LithologyErosionStepResult struct {
	ModifiedErosion  []float64
	UpdatedStates    []DynamicLithologyState
	ResistanceBefore []float64
	ResistanceAfter  []float64
	FeedbackApplied  bool
}

// SimulateErosionWithLithologyFeedback моделирует эрозию с обратной связью
func SimulateErosionWithLithologyFeedback(
	points []LatLon,
	baseErosionRates []float64,
	initialStates []DynamicLithologyState,
	params LithologyInteractionParams,
	isStorm bool,
	stormIntensity float64,
) LithologyErosionStepResult {

	if len(points) == 0 || len(baseErosionRates) == 0 {
		return LithologyErosionStepResult{}
	}

	n := len(baseErosionRates)
	result := LithologyErosionStepResult{
		ModifiedErosion:  make([]float64, n),
		UpdatedStates:    make([]DynamicLithologyState, n),
		ResistanceBefore: make([]float64, n),
		ResistanceAfter:  make([]float64, n),
		FeedbackApplied:  true,
	}

	if len(initialStates) >= n {
		copy(result.UpdatedStates, initialStates)
	} else {
		for i := range result.UpdatedStates {
			if i < len(initialStates) {
				result.UpdatedStates[i] = initialStates[i]
			} else {
				result.UpdatedStates[i] = DynamicLithologyState{
					CurrentResistance: 3.0,
					FractureDensity:   0.1,
					Porosity:          0.05,
				}
			}
		}
	}

	for i, state := range result.UpdatedStates {
		result.ResistanceBefore[i] = state.CurrentResistance
	}

	for i := range result.ModifiedErosion {
		if i >= len(result.UpdatedStates) {
			result.ModifiedErosion[i] = baseErosionRates[i]
			continue
		}

		result.ModifiedErosion[i] = CalculateLithologyErosionInteraction(
			baseErosionRates[i],
			result.UpdatedStates[i],
			params,
			isStorm,
		)
	}

	result.UpdatedStates = UpdateLithologyAfterErosion(
		result.UpdatedStates,
		result.ModifiedErosion,
		params,
	)

	if isStorm {
		for i := range result.UpdatedStates {
			result.UpdatedStates[i] = ApplyStormImpactOnLithology(
				result.UpdatedStates[i],
				params,
				stormIntensity,
			)
		}
	}

	for i, state := range result.UpdatedStates {
		result.ResistanceAfter[i] = state.CurrentResistance
	}

	return result
}

// EstimateStormProbabilityByLithology оценивает вероятность штормового воздействия
func EstimateStormProbabilityByLithology(
	state DynamicLithologyState,
	baseStormProbability float64,
	params LithologyInteractionParams,
) float64 {

	probability := baseStormProbability

	if state.FractureDensity > 0.3 {
		boost := (state.FractureDensity - 0.3) * params.StormFractureMultiplier * 0.5
		probability += boost
	}

	if state.IsWeathered {
		probability *= (1.0 + state.WeatheringProgress*0.3)
	}

	if probability > 1.0 {
		probability = 1.0
	}

	return probability
}
