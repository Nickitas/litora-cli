package geometry

import (
	"fmt"
	"math"
	"runtime"
	"sync"
)

// StormDepositLayer представляет слой отложений от отдельного шторма
type StormDepositLayer struct {
	StormIndex      int     // порядковый номер шторма
	Thickness       float64 // толщина слоя (м)
	Volume          float64 // объём отложений (м³/м)
	GrainSize       float64 // размер зёрен (мм) - индекс энергии
	IsPreserved     bool    // сохранится ли в геологической записи
	DepositLocation int     // индекс точки берега
}

// SeasonalModulation параметры сезонной модуляции
type SeasonalModulation struct {
	// Winter multiplier [0.5-2.0] - множитель для зимних месяцев (дек-фев)
	WinterMultiplier float64

	// Summer multiplier [0.5-2.0] - множитель для летних месяцев (июн-авг)
	SummerMultiplier float64

	// Transition multiplier [0.5-2.0] - для переходных сезонов
	TransitionMultiplier float64

	// Storm season boost [1.0-3.0] - дополнительный множитель во время штормового сезона
	StormSeasonBoost float64

	// Accumulation seasonality - сезонность аккумуляции
	// true: зима - больше эрозии, лето - больше аккумуляции
	// false: равномерное распределение
	AccumulationSeasonality bool
}

// StormSedimentParameters параметры влияния штормов на седиментацию
type StormSedimentParameters struct {
	// Enhanced transport multiplier [1.0-5.0]
	// Во сколько раз увеличивается транспорт во время шторма
	StormTransportMultiplier float64

	// Storm deposition efficiency [0.1-0.9]
	// Доля материала, отложенного во время шторма
	StormDepositionEfficiency float64

	// Post-storm surges [1.0-3.0]
	// Множитель для послештормовых нагонов
	PostStormSurgeMultiplier float64

	// Storm threshold - минимальная интенсивность для эффекта
	StormThreshold float64

	// Coastal retreat multiplier [1.0-10.0]
	// Отступание берега во время шторма
	StormRetreatMultiplier float64

	// Sediment bypassing - объём наносов, транспортируемых мимо мысов
	StormBypassingCoefficient float64
}

// TemporalSedimentState объединяет sediment state с временной динамикой
type TemporalSedimentState struct {
	BaseState         SedimentState
	CurrentSeason     string  // "winter", "spring", "summer", "autumn"
	IsStormActive     bool    // активный шторм в этом шаге
	StormIntensity    float64 // интенсивность шторма [1.0+]
	SeasonalFactor    float64 // сезонный множитель [0.5-1.5]
	StormDeposits     []StormDepositLayer
	ModifiedErosion   float64 // модифицированная эрозия с учётом сезона/шторма
	DepositionChange  float64 // изменение аккумуляции
}

// SedimentTemporalResult результат с учётом времени
type SedimentTemporalResult struct {
	States              []TemporalSedimentState
	TotalBudget         SedimentBudget
	SeasonalStats       map[string]SedimentBudget // статистика по сезонам
	StormImpact         SedimentBudget            // влияние штормов
	AllStormDeposits    []StormDepositLayer       // все штормовые отложения
	PreservedDeposits   []StormDepositLayer       // сохранённые отложения
	SeasonalCycle       []SedimentBudget          // годовой цикл
	IsValid             bool
	Warnings            []string
}

// CalculateSedimentTransportWithTemporal расчёт транспорта с учётом временной динамики
func CalculateSedimentTransportWithTemporal(
	points []LatLon,
	erosionRates []float64,
	waveData WaveEnergyData,
	lithology []LithologyState,
	params SedimentTransportParameters,
	temporalState TemporalState,
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
		States:       make([]TemporalSedimentState, n),
		SeasonalStats: make(map[string]SedimentBudget),
		StormImpact:  SedimentBudget{},
	}

	// Определяем текущий сезон
	season := determineSeason(temporalState.Year, seasonalMod.AccumulationSeasonality)

	// Расчёт базового транспорта
	baseResult := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

	// Применяем временную модуляцию
	for i := 0; i < n && i < len(baseResult.States); i++ {
		// Инициализируем временное состояние
		result.States[i].BaseState = baseResult.States[i]
		result.States[i].CurrentSeason = season
		result.States[i].IsStormActive = temporalState.IsStorm
		result.States[i].StormIntensity = temporalState.StormIntensity

		// Сезонный множитель
		seasonalFactor := calculateSeasonalFactor(season, seasonalMod, temporalState.SeasonalFactor)
		result.States[i].SeasonalFactor = seasonalFactor

		// Модифицируем эрозию с учётом сезонности
		baseErosion := erosionRates[i]
		modifiedErosion := baseErosion * seasonalFactor

		// Штормовая модуляция
		if temporalState.IsStorm && temporalState.StormIntensity > stormParams.StormThreshold {
			// Усиливаем эрозию во время шторма
			modifiedErosion *= temporalState.StormIntensity

			// Усиливаем транспорт
			result.States[i].BaseState.LocalBudget.TransportVolume *= stormParams.StormTransportMultiplier

			// Штормовое отступание
			result.States[i].BaseState.LocalBudget.ErodedVolume *= stormParams.StormRetreatMultiplier

			// Фиксируем влияние шторма
			result.StormImpact.ErodedVolume += result.States[i].BaseState.LocalBudget.ErodedVolume
			result.StormImpact.TransportVolume += result.States[i].BaseState.LocalBudget.TransportVolume
			result.StormImpact.ErosionPoints++
		}

		result.States[i].ModifiedErosion = modifiedErosion

		// Сезонная аккумуляция
		if season == "summer" || seasonalFactor < 1.0 {
			// Лето или низкая эрозия -> усиленная аккумуляция
			depositionBoost := (2.0 - seasonalFactor) * 0.5
			result.States[i].BaseState.LocalBudget.DepositedVolume *= (1.0 + depositionBoost)
			result.States[i].DepositionChange = result.States[i].BaseState.LocalBudget.DepositedVolume
		} else if season == "winter" {
			// Зима -> ослабленная аккумуляция
			result.States[i].BaseState.LocalBudget.DepositedVolume *= 0.7
			result.States[i].DepositionChange = -result.States[i].BaseState.LocalBudget.DepositedVolume * 0.3
		}

		// Штормовые отложения
		if temporalState.IsStorm && temporalState.StormIntensity > 1.5 {
			stormDeposit := StormDepositLayer{
				StormIndex:        int(temporalState.Step),
				Thickness:         calculateStormDepositThickness(temporalState.StormIntensity),
				Volume:            result.States[i].BaseState.LocalBudget.DepositedVolume,
				GrainSize:         calculateStormGrainSize(temporalState.StormIntensity),
				IsPreserved:       determinePreservation(temporalState.StormIntensity, result.States[i].BaseState),
				DepositLocation:   i,
			}
			result.States[i].StormDeposits = append(result.States[i].StormDeposits, stormDeposit)
			result.AllStormDeposits = append(result.AllStormDeposits, stormDeposit)

			if stormDeposit.IsPreserved {
				result.PreservedDeposits = append(result.PreservedDeposits, stormDeposit)
			}
		}

		// Баланс
		result.States[i].BaseState.LocalBudget.NetChange =
			result.States[i].BaseState.LocalBudget.ErodedVolume -
			result.States[i].BaseState.LocalBudget.DepositedVolume
	}

	// Сводная статистика
	result.TotalBudget = baseResult.TotalBudget

	// Сезонная статистика
	seasonBudget := SedimentBudget{}
	for _, state := range result.States {
		seasonBudget.ErodedVolume += state.BaseState.LocalBudget.ErodedVolume
		seasonBudget.TransportVolume += state.BaseState.LocalBudget.TransportVolume
		seasonBudget.DepositedVolume += state.BaseState.LocalBudget.DepositedVolume
	}
	result.SeasonalStats[season] = seasonBudget

	// Валидация
	result.IsValid = baseResult.IsValid
	result.Warnings = baseResult.Warnings

	if temporalState.IsStorm {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Storm event: intensity %.2f", temporalState.StormIntensity))
	}

	return result
}

// determineSeason определяет сезон по номеру года
func determineSeason(year float64, useSeasonality bool) string {
	if !useSeasonality {
		return "neutral"
	}

	// Год дробный, берём дробную часть
	yearFraction := year - math.Floor(year)

	// Сезоны:
	// 0.0-0.25: зима (декабрь-февраль) - высокая эрозия
	// 0.25-0.5: весна (март-май) - переход
	// 0.5-0.75: лето (июнь-август) - низкая эрозия, высокая аккумуляция
	// 0.75-1.0: осень (сентябрь-ноябрь) - переход
	if yearFraction < 0.25 || yearFraction >= 0.92 {
		return "winter"
	} else if yearFraction < 0.5 {
		return "spring"
	} else if yearFraction < 0.75 {
		return "summer"
	} else {
		return "autumn"
	}
}

// calculateSeasonalFactor вычисляет сезонный множитель
func calculateSeasonalFactor(season string, mod SeasonalModulation, baseFactor float64) float64 {
	factor := 1.0

	switch season {
	case "winter":
		factor = mod.WinterMultiplier
	case "spring", "autumn":
		factor = mod.TransitionMultiplier
	case "summer":
		factor = mod.SummerMultiplier
	default:
		factor = 1.0
	}

	// Комбинируем с базовым фактором
	return (factor + baseFactor) / 2.0
}

// calculateStormDepositThickness вычисляет толщину штормовых отложений
func calculateStormDepositThickness(stormIntensity float64) float64 {
	// Эмпирическая формула: интенсивность шторма коррелирует с толщиной отложений
	// Сильные штормы создают более мощные отложения (до 0.5м)
	baseThickness := 0.05 // базовая толщина 5 см
	thickness := baseThickness * (stormIntensity - 0.5)

	if thickness > 0.5 {
		thickness = 0.5 // максимум 50 см
	}
	if thickness < 0.01 {
		thickness = 0.01 // минимум 1 см
	}

	return thickness
}

// calculateStormGrainSize вычисляет размер зёрен в штормовых отложениях
func calculateStormGrainSize(stormIntensity float64) float64 {
	// Сильные штормы переносят более крупный материал
	// 1.0-2.0: песок (0.1-2 мм)
	// 2.0-3.0: гравий (2-10 мм)
	// 3.0+: галечник (10+ мм)
	baseSize := 0.5 // базовый размер песка 0.5 мм

	grainSize := baseSize * stormIntensity

	if grainSize > 20 {
		grainSize = 20 // максимум 20 мм
	}

	return grainSize
}

// determinePreservation определяет, сохранятся ли отложения в геологической записи
func determinePreservation(stormIntensity float64, state SedimentState) bool {
	// Критерии сохранения:
	// 1. Высокая интенсивность шторма (> 2.0) - лучше сохраняются
	// 2. Наличие аккумуляции - материал захоронен
	// 3. Низкая энергия в точке - меньше переупаковки

	preservationScore := 0.0

	if stormIntensity > 2.0 {
		preservationScore += 0.4
	}
	if stormIntensity > 3.0 {
		preservationScore += 0.2
	}

	if state.IsAccumulating {
		preservationScore += 0.3
	}

	if state.LocalBudget.DepositedVolume > 0.1 {
		preservationScore += 0.1
	}

	return preservationScore > 0.5
}

// normalizeSeasonalMod нормализует параметры сезонности
func normalizeSeasonalMod(mod SeasonalModulation) SeasonalModulation {
	if mod.WinterMultiplier <= 0 {
		mod.WinterMultiplier = 1.5 // зима - больше эрозии
	}
	if mod.SummerMultiplier <= 0 {
		mod.SummerMultiplier = 0.7 // лето - меньше эрозии
	}
	if mod.TransitionMultiplier <= 0 {
		mod.TransitionMultiplier = 1.0 // нейтрально
	}
	if mod.StormSeasonBoost <= 0 {
		mod.StormSeasonBoost = 1.5 // штормовой сезон
	}

	return mod
}

// normalizeStormSedimentParams нормализует параметры штормовой седиментации
func normalizeStormSedimentParams(params StormSedimentParameters) StormSedimentParameters {
	if params.StormTransportMultiplier <= 1.0 {
		params.StormTransportMultiplier = 2.5
	}
	if params.StormDepositionEfficiency <= 0 {
		params.StormDepositionEfficiency = 0.6
	}
	if params.PostStormSurgeMultiplier <= 1.0 {
		params.PostStormSurgeMultiplier = 1.5
	}
	if params.StormThreshold <= 0 {
		params.StormThreshold = 1.2
	}
	if params.StormRetreatMultiplier <= 1.0 {
		params.StormRetreatMultiplier = 3.0
	}
	if params.StormBypassingCoefficient < 0 || params.StormBypassingCoefficient > 1 {
		params.StormBypassingCoefficient = 0.3
	}

	return params
}

// ApplySeasonalAccumulationModulation модифицирует аккумуляцию с учётом сезонности
func ApplySeasonalAccumulationModulation(
	baseErosion []float64,
	depositionZones []bool, // true если зона аккумуляции
	season string,
	mod SeasonalModulation,
) []float64 {

	modified := make([]float64, len(baseErosion))
	copy(modified, baseErosion)

	seasonalFactor := 1.0
	switch season {
	case "winter":
		seasonalFactor = mod.WinterMultiplier
	case "summer":
		seasonalFactor = mod.SummerMultiplier
	default:
		seasonalFactor = mod.TransitionMultiplier
	}

	for i, isAccumulation := range depositionZones {
		if isAccumulation {
			// В зонах аккумуляции модификация меньше (защищённые участки)
			modified[i] *= seasonalFactor * 0.8
		} else {
			// В зонах эрозии - полная модификация
			modified[i] *= seasonalFactor
		}
	}

	return modified
}

// GetStormDepositStatistics возвращает статистику штормовых отложений
func GetStormDepositStatistics(result SedimentTemporalResult) map[string]interface{} {
	stats := map[string]interface{}{
		"total_storms":            len(result.AllStormDeposits),
		"preserved_layers":        len(result.PreservedDeposits),
		"total_thickness_m":       0.0,
		"avg_thickness_m":         0.0,
		"avg_grain_size_mm":       0.0,
		"storm_impact_eroded_m3":  result.StormImpact.ErodedVolume,
		"storm_impact_transport":  result.StormImpact.TransportVolume,
		"storm_impact_deposited":  result.StormImpact.DepositedVolume,
	}

	if len(result.AllStormDeposits) > 0 {
		totalThickness := 0.0
		totalGrainSize := 0.0

		for _, deposit := range result.AllStormDeposits {
			totalThickness += deposit.Thickness
			totalGrainSize += deposit.GrainSize
		}

		stats["total_thickness_m"] = totalThickness
		stats["avg_thickness_m"] = totalThickness / float64(len(result.AllStormDeposits))
		stats["avg_grain_size_mm"] = totalGrainSize / float64(len(result.AllStormDeposits))
	}

	return stats
}

// CreateSeasonalAccumulationProfile создаёт профиль аккумуляции по сезонам
func CreateSeasonalAccumulationProfile(
	yearlySteps int,
	points []LatLon,
	erosionRates []float64,
	waveData WaveEnergyData,
	lithology []LithologyState,
	params SedimentTransportParameters,
	seasonalMod SeasonalModulation,
	stormParams StormSedimentParameters,
) []SedimentTemporalResult {

	results := make([]SedimentTemporalResult, yearlySteps)

	for step := 0; step < yearlySteps; step++ {
		// Создаём виртуальное временное состояние для каждого сезона
		year := float64(step) / float64(yearlySteps)

		temporalState := TemporalState{
			Step:           step,
			Year:           year,
			SeasonalFactor: calculateSeasonalFactor(
				determineSeason(year, seasonalMod.AccumulationSeasonality),
				seasonalMod,
				1.0,
			),
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

		results[step] = result
	}

	return results
}

// CalculateSedimentTransportWithTemporalOptimized оптимизированная версия с параллельной обработкой
func CalculateSedimentTransportWithTemporalOptimized(
	points []LatLon,
	erosionRates []float64,
	waveData WaveEnergyData,
	lithology []LithologyState,
	params SedimentTransportParameters,
	temporalState TemporalState,
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
		States:       make([]TemporalSedimentState, n),
		SeasonalStats: make(map[string]SedimentBudget),
		StormImpact:  SedimentBudget{},
	}

	season := determineSeason(temporalState.Year, seasonalMod.AccumulationSeasonality)

	// Используем оптимизированный расчёт базового транспорта
	baseResult := CalculateSedimentTransportAuto(points, erosionRates, waveData, lithology, params)

	// Предвычисляем сезонный фактор (общий для всех точек)
	seasonalFactor := calculateSeasonalFactor(season, seasonalMod, temporalState.SeasonalFactor)

	// Параллельная обработка временной модуляции
	useParallel := n > 500

	if useParallel {
		applyTemporalModulationParallel(result, baseResult, erosionRates, temporalState,
			season, seasonalFactor, stormParams, n)
	} else {
		applyTemporalModulationSequential(result, baseResult, erosionRates, temporalState,
			season, seasonalFactor, stormParams, n)
	}

	// Сводная статистика (может быть параллельной)
	calculateTemporalStatistics(result, baseResult, season, temporalState)

	return result
}

// applyTemporalModulationParallel параллельная применяет временную модуляцию
func applyTemporalModulationParallel(
	result SedimentTemporalResult,
	baseResult SedimentTransportResult,
	erosionRates []float64,
	temporalState TemporalState,
	season string,
	seasonalFactor float64,
	stormParams StormSedimentParameters,
	n int,
) {
	workers := runtime.NumCPU()
	batchSize := (n + workers - 1) / workers

	var wg sync.WaitGroup
	var mu sync.Mutex

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

			localStormDeposits := make([]StormDepositLayer, 0)
			localStormImpact := SedimentBudget{}

			for j := workerStart; j < workerEnd; j++ {
				if j >= len(baseResult.States) {
					break
				}

				// Копируем базовое состояние
				result.States[j].BaseState = baseResult.States[j]
				result.States[j].CurrentSeason = season
				result.States[j].IsStormActive = temporalState.IsStorm
				result.States[j].StormIntensity = temporalState.StormIntensity
				result.States[j].SeasonalFactor = seasonalFactor

				baseErosion := erosionRates[j]
				modifiedErosion := baseErosion * seasonalFactor

				// Штормовая модуляция
				if temporalState.IsStorm && temporalState.StormIntensity > stormParams.StormThreshold {
					modifiedErosion *= temporalState.StormIntensity
					result.States[j].BaseState.LocalBudget.TransportVolume *= stormParams.StormTransportMultiplier
					result.States[j].BaseState.LocalBudget.ErodedVolume *= stormParams.StormRetreatMultiplier

					localStormImpact.ErodedVolume += result.States[j].BaseState.LocalBudget.ErodedVolume
					localStormImpact.TransportVolume += result.States[j].BaseState.LocalBudget.TransportVolume
					localStormImpact.ErosionPoints++
				}

				result.States[j].ModifiedErosion = modifiedErosion

				// Сезонная аккумуляция
				if season == "summer" || seasonalFactor < 1.0 {
					depositionBoost := (2.0 - seasonalFactor) * 0.5
					result.States[j].BaseState.LocalBudget.DepositedVolume *= (1.0 + depositionBoost)
					result.States[j].DepositionChange = result.States[j].BaseState.LocalBudget.DepositedVolume
				} else if season == "winter" {
					result.States[j].BaseState.LocalBudget.DepositedVolume *= 0.7
					result.States[j].DepositionChange = -result.States[j].BaseState.LocalBudget.DepositedVolume * 0.3
				}

				// Штормовые отложения
				if temporalState.IsStorm && temporalState.StormIntensity > 1.5 {
					stormDeposit := StormDepositLayer{
						StormIndex:      int(temporalState.Step),
						Thickness:       calculateStormDepositThickness(temporalState.StormIntensity),
						Volume:          result.States[j].BaseState.LocalBudget.DepositedVolume,
						GrainSize:       calculateStormGrainSize(temporalState.StormIntensity),
						IsPreserved:     determinePreservation(temporalState.StormIntensity, result.States[j].BaseState),
						DepositLocation: j,
					}
					result.States[j].StormDeposits = append(result.States[j].StormDeposits, stormDeposit)
					localStormDeposits = append(localStormDeposits, stormDeposit)
				}

				result.States[j].BaseState.LocalBudget.NetChange =
					result.States[j].BaseState.LocalBudget.ErodedVolume -
					result.States[j].BaseState.LocalBudget.DepositedVolume
			}

			// Агрегируем результаты
			mu.Lock()
			result.AllStormDeposits = append(result.AllStormDeposits, localStormDeposits...)
			result.StormImpact.ErodedVolume += localStormImpact.ErodedVolume
			result.StormImpact.TransportVolume += localStormImpact.TransportVolume
			result.StormImpact.ErosionPoints += localStormImpact.ErosionPoints
			mu.Unlock()
		}(start, end)
	}

	wg.Wait()

	// Пост-обработка: фильтруем сохранённые отложения
	for _, deposit := range result.AllStormDeposits {
		if deposit.IsPreserved {
			result.PreservedDeposits = append(result.PreservedDeposits, deposit)
		}
	}
}

// applyTemporalModulationSequential последовательная версия
func applyTemporalModulationSequential(
	result SedimentTemporalResult,
	baseResult SedimentTransportResult,
	erosionRates []float64,
	temporalState TemporalState,
	season string,
	seasonalFactor float64,
	stormParams StormSedimentParameters,
	n int,
) {
	for i := 0; i < n && i < len(baseResult.States); i++ {
		result.States[i].BaseState = baseResult.States[i]
		result.States[i].CurrentSeason = season
		result.States[i].IsStormActive = temporalState.IsStorm
		result.States[i].StormIntensity = temporalState.StormIntensity
		result.States[i].SeasonalFactor = seasonalFactor

		baseErosion := erosionRates[i]
		modifiedErosion := baseErosion * seasonalFactor

		if temporalState.IsStorm && temporalState.StormIntensity > stormParams.StormThreshold {
			modifiedErosion *= temporalState.StormIntensity
			result.States[i].BaseState.LocalBudget.TransportVolume *= stormParams.StormTransportMultiplier
			result.States[i].BaseState.LocalBudget.ErodedVolume *= stormParams.StormRetreatMultiplier

			result.StormImpact.ErodedVolume += result.States[i].BaseState.LocalBudget.ErodedVolume
			result.StormImpact.TransportVolume += result.States[i].BaseState.LocalBudget.TransportVolume
			result.StormImpact.ErosionPoints++
		}

		result.States[i].ModifiedErosion = modifiedErosion

		if season == "summer" || seasonalFactor < 1.0 {
			depositionBoost := (2.0 - seasonalFactor) * 0.5
			result.States[i].BaseState.LocalBudget.DepositedVolume *= (1.0 + depositionBoost)
			result.States[i].DepositionChange = result.States[i].BaseState.LocalBudget.DepositedVolume
		} else if season == "winter" {
			result.States[i].BaseState.LocalBudget.DepositedVolume *= 0.7
			result.States[i].DepositionChange = -result.States[i].BaseState.LocalBudget.DepositedVolume * 0.3
		}

		if temporalState.IsStorm && temporalState.StormIntensity > 1.5 {
			stormDeposit := StormDepositLayer{
				StormIndex:      int(temporalState.Step),
				Thickness:       calculateStormDepositThickness(temporalState.StormIntensity),
				Volume:          result.States[i].BaseState.LocalBudget.DepositedVolume,
				GrainSize:       calculateStormGrainSize(temporalState.StormIntensity),
				IsPreserved:     determinePreservation(temporalState.StormIntensity, result.States[i].BaseState),
				DepositLocation: i,
			}
			result.States[i].StormDeposits = append(result.States[i].StormDeposits, stormDeposit)
			result.AllStormDeposits = append(result.AllStormDeposits, stormDeposit)

			if stormDeposit.IsPreserved {
				result.PreservedDeposits = append(result.PreservedDeposits, stormDeposit)
			}
		}

		result.States[i].BaseState.LocalBudget.NetChange =
			result.States[i].BaseState.LocalBudget.ErodedVolume -
			result.States[i].BaseState.LocalBudget.DepositedVolume
	}
}

// calculateTemporalStatistics вычисляет статистику (может быть параллельной)
func calculateTemporalStatistics(
	result SedimentTemporalResult,
	baseResult SedimentTransportResult,
	season string,
	temporalState TemporalState,
) {
	result.TotalBudget = baseResult.TotalBudget

	seasonBudget := SedimentBudget{}
	for _, state := range result.States {
		seasonBudget.ErodedVolume += state.BaseState.LocalBudget.ErodedVolume
		seasonBudget.TransportVolume += state.BaseState.LocalBudget.TransportVolume
		seasonBudget.DepositedVolume += state.BaseState.LocalBudget.DepositedVolume
	}
	result.SeasonalStats[season] = seasonBudget

	result.IsValid = baseResult.IsValid
	result.Warnings = baseResult.Warnings

	if temporalState.IsStorm {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Storm event: intensity %.2f", temporalState.StormIntensity))
	}
}
