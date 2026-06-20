package geometry

import (
	"fmt"
	"math"
)

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
	IsAccumulating bool       // режим аккумуляции
	IsEroding      bool       // режим эрозии
}

// SedimentTransportParameters параметры транспорта наносов
type SedimentTransportParameters struct {
	// Transport coefficient [0-1]
	// Какая часть размытого материала идёт в транспорт
	TransportCoefficient float64

	// Deposition rate [0-1]
	// Какая часть избытка откладывается
	DepositionRate float64

	// Minimum flow velocity (m/s)
	// Минимальная скорость для транспорта
	MinimumFlowVelocity float64

	// Capacity factor [0-2]
	// Ёмкость берега для аккумуляции
	CapacityFactor float64

	// Longshore drift coefficient [0-1]
	// Сила alongshore транспорта
	LongshoreDriftCoefficient float64
}

// LithologyState литологическое состояние точки
type LithologyState struct {
	Class       string  // класс породы
	Resistance  float64 // сопротивление эрозии [0.1-10.0]
	Color       string  // для SVG
	Description string  // описание
}

// SedimentTransportResult результат расчёта транспорта
type SedimentTransportResult struct {
	States         []SedimentState
	TotalBudget    SedimentBudget
	MassBalance    float64  // должен быть ≈ 0
	IsValid        bool     // validation check
	Warnings       []string
	BaselineErosion []float64 // базовая эрозия (м)
	ModifiedErosion []float64 // модифицированная эрозия (м)
}

// WaveEnergyData волновая энергия по точкам
type WaveEnergyData struct {
	Energy      []float64 // волновая энергия [0-1]
	Direction   float64   // главное направление (град от севера)
	Incidence   []float64 // угол падения на берег [0-1]
	Fetch       []float64 // fetch distance (m)
}

// CalculateSedimentTransport рассчитывает транспорт наносов
func CalculateSedimentTransport(
	points []LatLon,
	erosionRates []float64, // скорость эрозии (м/шаг)
	waveData WaveEnergyData,
	lithology []LithologyState,
	params SedimentTransportParameters,
) SedimentTransportResult {

	n := len(points)
	if n == 0 {
		return SedimentTransportResult{}
	}

	// Нормализация параметров
	params = normalizeSedimentParams(params)

	// Инициализация states
	states := make([]SedimentState, n)
	for i := range states {
		states[i].PointIndex = i
		states[i].InTransitFrom = make([]float64, 0)
		states[i].InTransitTo = make([]float64, 0)
	}

	// Этап 1: Рассчитать объём эрозии для каждой точки
	calculateErosionVolumes(states, erosionRates, lithology, params)

	// Этап 2: Longshore drift — транспорт вдоль берега
	calculateLongshoreDrift(states, points, waveData, params)

	// Этап 3: Депозиция и баланс массы
	calculateDeposition(states, waveData, params)

	// Этап 4: Резюме и валидация
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
		// Базовая эрозия в метрах
		erodedMeters := erosionRates[i]

		// Модуляция по литологии
		if i < len(lithology) && lithology[i].Resistance > 0 {
			erodedMeters /= lithology[i].Resistance
		}

		// Конвертация линейной эрозии в объём (на метр берега)
		// Предполагаем depth erosion = 1м (можно параметризовать)
		erodedVolume := erodedMeters * 1.0

		states[i].LocalBudget.ErodedVolume = erodedVolume

		// Часть идёт в транспорт, часть откладывается локально
		transportFraction := params.TransportCoefficient
		if len(lithology) > i && lithology[i].Resistance > 5.0 {
			// Твёрдые породы — меньше транспорта
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

		// Соседи (с замыканием для closed polylines)
		prevIndex := (i - 1 + n) % n
		nextIndex := (i + 1) % n

		// Вектор alongshore (от prev к next)
		prevPoint := points[prevIndex]
		nextPoint := points[nextIndex]

		alongshoreX := nextPoint.Lon - prevPoint.Lon
		alongshoreY := nextPoint.Lat - prevPoint.Lat
		alongshoreLen := math.Hypot(alongshoreX, alongshoreY)

		if alongshoreLen < 1e-9 {
			continue
		}

		// Нормированный alongshore direction
		alongshoreX /= alongshoreLen
		alongshoreY /= alongshoreLen

		// Wave direction (в радианах от севера)
		waveDirRad := waveData.Direction * math.Pi / 180.0
		waveDirX := math.Sin(waveDirRad)
		waveDirY := math.Cos(waveDirRad)

		// Alongshore компонента wave energy
		// dot product: чем более alongshore волна, тем больше drift
		alongshoreComponent := math.Abs(alongshoreX*waveDirX + alongshoreY*waveDirY)

		// Wave energy на точке
		waveEnergy := 0.5
		if i < len(waveData.Energy) {
			waveEnergy = waveData.Energy[i]
		}

		// Transport объём зависит от:
		// 1. Alongshore component
		// 2. Wave energy
		// 3. Longshore drift coefficient
		transportAvailable := states[i].LocalBudget.TransportVolume

		// Если wave пришёл с лева → drift вправо, и наоборот
		crossProduct := alongshoreX*waveDirY - alongshoreY*waveDirX
		driftFraction := params.LongshoreDriftCoefficient * alongshoreComponent * waveEnergy

		// Распределение между соседями
		toPrev := transportAvailable * 0.5 * driftFraction
		toNext := transportAvailable * 0.5 * driftFraction

		// Если crossProduct > 0, drift направлен в одну сторону
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
		// Подсчёт incoming sediment
		incomingTotal := 0.0
		for _, v := range states[i].InTransitFrom {
			incomingTotal += v
		}

		// Local capacity для аккумуляции
		waveEnergy := 0.5
		if i < len(waveData.Energy) {
			waveEnergy = waveData.Energy[i]
		}

		localCapacity := params.CapacityFactor * waveEnergy

		// Логика аккумуляции:
		// 1. Если incoming > capacity → избыток откладывается
		// 2. Если wave energy очень низкая (< 0.3) → аккумуляция независимо от incoming
		if incomingTotal > localCapacity || waveEnergy < 0.3 {
			var excess float64
			if incomingTotal > localCapacity {
				excess = incomingTotal - localCapacity
			} else {
				// Низкая энергия волн → аккумуляция даже без избытка
				excess = localCapacity * 0.5 // фракция от ёмкости
			}

			deposition := excess * params.DepositionRate

			states[i].LocalBudget.DepositedVolume += deposition
			states[i].IsAccumulating = true

			// Остаток идёт дальше (только если был избыток)
			if incomingTotal > localCapacity {
				states[i].LocalBudget.TransportVolume += (excess - deposition)
			}
		} else {
			// Недостаток — erosion mode
			states[i].IsEroding = true
		}

		// Баланс массы
		states[i].LocalBudget.NetChange =
			states[i].LocalBudget.ErodedVolume -
			states[i].LocalBudget.DepositedVolume

		// Статистика
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
		States:         states,
		BaselineErosion: make([]float64, len(states)),
		ModifiedErosion: make([]float64, len(states)),
	}

	totalBudget := SedimentBudget{}

	// Сводка по точкам
	for i, state := range states {
		totalBudget.ErodedVolume += state.LocalBudget.ErodedVolume
		totalBudget.TransportVolume += state.LocalBudget.TransportVolume
		totalBudget.DepositedVolume += state.LocalBudget.DepositedVolume

		totalBudget.ErosionPoints += state.LocalBudget.ErosionPoints
		totalBudget.DepositionPoints += state.LocalBudget.DepositionPoints

		result.BaselineErosion[i] = erosionRates[i]

		// Модифицированная эрозия с учётом аккумуляции
		if state.IsAccumulating {
			depositionMeters := state.LocalBudget.DepositedVolume / 1.0 // /depth
			result.ModifiedErosion[i] = erosionRates[i] - depositionMeters

			if result.ModifiedErosion[i] < 0 {
				result.ModifiedErosion[i] = 0 // аккумуляция = рост берега
			}
		} else {
			result.ModifiedErosion[i] = erosionRates[i]
		}
	}

	totalBudget.NetChange = totalBudget.ErodedVolume - totalBudget.DepositedVolume

	result.TotalBudget = totalBudget
	result.MassBalance = totalBudget.NetChange

	// Валидация: баланс массы должен сохраняться
	// Eroded ≈ Deposited + Transport
	totalEroded := totalBudget.ErodedVolume
	if totalEroded > 0 {
		totalAccountedFor := totalBudget.DepositedVolume + totalBudget.TransportVolume
		balanceRatio := math.Abs(totalEroded-totalAccountedFor) / totalEroded
		result.IsValid = balanceRatio < 0.15 // допуск 15% (реалистично для sediment transport)
		result.MassBalance = balanceRatio
	} else {
		result.IsValid = true
		result.MassBalance = 0
	}

	// Warnings
	if !result.IsValid {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Poor mass balance: %.2f vs %.2f",
				totalBudget.ErodedVolume, totalBudget.DepositedVolume))
	}

	if result.TotalBudget.ErosionPoints > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Erosion at %d points", result.TotalBudget.ErosionPoints))
	}

	if result.TotalBudget.DepositionPoints > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Deposition at %d points", result.TotalBudget.DepositionPoints))
	}

	return result
}

// normalizeSedimentParams нормализует параметры
func normalizeSedimentParams(params SedimentTransportParameters) SedimentTransportParameters {
	if params.TransportCoefficient <= 0 || params.TransportCoefficient > 1 {
		params.TransportCoefficient = 0.7 // default
	}
	if params.DepositionRate <= 0 || params.DepositionRate > 1 {
		params.DepositionRate = 0.5 // default
	}
	if params.MinimumFlowVelocity <= 0 {
		params.MinimumFlowVelocity = 0.3 // default
	}
	if params.CapacityFactor <= 0 {
		params.CapacityFactor = 1.0 // default
	}
	if params.LongshoreDriftCoefficient <= 0 || params.LongshoreDriftCoefficient > 1 {
		params.LongshoreDriftCoefficient = 0.8 // default
	}

	return params
}

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
			// В точках аккумуляции эрозия компенсируется
			depositionMeters := sedimentResult.States[i].LocalBudget.DepositedVolume / 1.0
			modified[i] = baseErosion[i] - depositionMeters

			// Не может быть отрицательной
			if modified[i] < 0 {
				modified[i] = 0
			}
		}
	}

	return modified
}

// GetSedimentStatistics возвращает статистику по sediment transport
func GetSedimentStatistics(result SedimentTransportResult) map[string]interface{} {
	stats := map[string]interface{}{
		"total_eroded_m3":    result.TotalBudget.ErodedVolume,
		"total_deposited_m3": result.TotalBudget.DepositedVolume,
		"total_transport_m3": result.TotalBudget.TransportVolume,
		"net_change_m3":     result.TotalBudget.NetChange,
		"mass_balance":       result.MassBalance,
		"is_valid":          result.IsValid,
		"erosion_points":    result.TotalBudget.ErosionPoints,
		"deposition_points": result.TotalBudget.DepositionPoints,
		"warnings":          result.Warnings,
	}

	return stats
}