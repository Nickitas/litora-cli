package cli

import (
	"coastal-geometry/internal/domain/geometry"
	"math"
)

// hashFloat генерирует детерминированное случайное число [0-1] из координат
func hashFloat(x float64) float64 {
	// Простой hash на основе дробной части
	abs := math.Abs(x)
	frac := abs - math.Floor(abs)
	// Используем синус для дополнительного перемешивания
	hash := math.Sin(frac * 1e6)   // масштабируем для хорошей гранулярности
	return hash - math.Floor(hash) // возвращаем дробную часть [0-1]
}

// calculateSedimentTransportForValidation подготавливает данные и рассчитывает
// sediment transport для валидации метрик качества модели
func calculateSedimentTransportForValidation(
	snapshots [][]geometry.LatLon,
	bathymetryGrid *geometry.BathymetryGrid,
	lithologyProfile *geometry.LithologyProfile,
	waveOptions geometry.WaveErosionOptions,
) *geometry.SedimentTransportResult {

	if len(snapshots) < 2 {
		return nil
	}

	// Используем последний и предпоследний snapshots для расчета erosion rates
	prevPoints := snapshots[len(snapshots)-2]
	currentPoints := snapshots[len(snapshots)-1]

	n := len(currentPoints)
	if n != len(prevPoints) {
		return nil
	}

	// Рассчитываем erosion rates (в метрах) для каждой точки
	erosionRates := make([]float64, n)
	for i := 0; i < n; i++ {
		// Расстояние между соответствующими точками
		dist := geometry.Haversine(prevPoints[i], currentPoints[i])
		erosionRates[i] = dist
	}

	// Подготавливаем WaveEnergyData
	waveData := prepareWaveEnergyData(currentPoints, bathymetryGrid, waveOptions)

	// Подготавливаем LithologyState для каждой точки
	lithology := prepareLithologyStates(currentPoints, lithologyProfile)

	// Параметры sediment transport (калиброванные для лучшего баланса)
	params := geometry.SedimentTransportParameters{
		TransportCoefficient:      0.8, // 80% эродированного материала идёт в транспорт (увеличен)
		DepositionRate:            0.5, // 50% избытка откладывается (сбалансировано)
		MinimumFlowVelocity:       0.2, // минимальная скорость потока (снижена)
		CapacityFactor:            1.2, // ёмкость берега для аккумуляции (уменьшена для реализма)
		LongshoreDriftCoefficient: 0.9, // сила alongshore транспорта (увеличена)
	}

	// Рассчитываем sediment transport с автоматическим выбором оптимальной стратегии
	result := geometry.CalculateSedimentTransportAuto(
		currentPoints, erosionRates, waveData, lithology, params,
	)

	return &result
}

// prepareWaveEnergyData подготавливает данные о волновой энергии
func prepareWaveEnergyData(
	points []geometry.LatLon,
	bathymetryGrid *geometry.BathymetryGrid,
	waveOptions geometry.WaveErosionOptions,
) geometry.WaveEnergyData {

	n := len(points)
	waveData := geometry.WaveEnergyData{
		Energy:    make([]float64, n),
		Incidence: make([]float64, n),
		Fetch:     make([]float64, n),
		Direction: float64(waveOptions.WindSourceDirectionDeg),
	}

	// Если есть батиметрия, используем её для более точного расчёта
	if bathymetryGrid != nil && len(bathymetryGrid.Points) > 0 {
		for i, point := range points {
			// Получаем глубину в точке
			depth, _ := bathymetryGrid.InterpolateDepth(point.Lat, point.Lon)

			// Fetch distance
			fetch := waveOptions.MaxFetchMeters
			waveData.Fetch[i] = fetch

			// Wave energy зависит от глубины и fetch
			// На мелководье энергия меньше
			depthFactor := 1.0
			if depth > 0 {
				// На глубине > 0 энергия уменьшается
				depthFactor = math.Exp(-depth / 50.0) // 50m - характерная глубина
			}

			// Fetch factor: чем больше fetch, тем больше энергия
			fetchFactor := math.Min(1.0, fetch/100000.0) // 100km - характерный fetch

			// Базовая энергия от ветра
			windSpeedFactor := math.Min(1.0, waveOptions.WindSpeedMetersPerSecond/20.0) // 20 m/s - сильный шторм

			// Итоговая энергия [0-1]
			waveData.Energy[i] = depthFactor * fetchFactor * windSpeedFactor
			waveData.Energy[i] = math.Max(0.0, math.Min(1.0, waveData.Energy[i]))

			// 📍 УЛУЧШЕНИЕ 1: Геометрическая вариативность энергии
			// Рассчитываем локальную кривизну береговой линии
			curvatureFactor := 1.0
			shelteringEffect := 1.0

			if n >= 3 {
				// Берём предыдущую и следующую точку для расчёта угла
				prevIdx := (i - 1 + n) % n
				nextIdx := (i + 1) % n

				prevPoint := points[prevIdx]
				nextPoint := points[nextIdx]

				// Вектор от prev к next (направление берега)
				coastX := nextPoint.Lon - prevPoint.Lon
				coastY := nextPoint.Lat - prevPoint.Lat
				coastLen := math.Hypot(coastX, coastY)

				if coastLen > 1e-9 {
					// Нормированное направление берега
					coastDirX := coastX / coastLen
					coastDirY := coastY / coastLen

					// Вектор от prev к current (инцидентность)
					incidentX := point.Lon - prevPoint.Lon
					incidentY := point.Lat - prevPoint.Lat
					incidentLen := math.Hypot(incidentX, incidentY)

					if incidentLen > 1e-9 {
						// Dot product для определения угла
						dot := (incidentX*coastDirX + incidentY*coastDirY) / incidentLen

						// Если dot близко к 1, точка на прямой линии (large curvature)
						// Если dot близко к 0, точка на выступе (headland)
						absDot := math.Abs(dot)

						// Headlands (высокая кривизна) → sheltered → low energy
						// Bays (низкая кривизна) → exposed → high energy
						if absDot < 0.6 { // Headland/convex
							curvatureFactor = 0.6 // Сниженная энергия
						} else if absDot > 0.95 { // Bay/concave
							curvatureFactor = 1.3 // Повышенная энергия
						}

						// Sheltering от соседних участков
						if i > 0 && i < n-1 {
							// Проверяем, есть ли "защита" от соседних точек
							nextDist := geometry.Haversine(point, nextPoint)
							prevDist := geometry.Haversine(point, prevPoint)

							// Близкие соседи создают sheltering эффект
							if nextDist < 2.0 || prevDist < 2.0 { // < 2km
								shelteringEffect = 0.7 // 30% снижение энергии
							}
						}
					}
				}
			}

			// Применяем геометрические факторы
			waveData.Energy[i] *= curvatureFactor * shelteringEffect

			// Добавляем случайную вариацию для естественности
			energyVariation := (hashFloat(point.Lat+point.Lon) - 0.5) * 0.3 // ±15% вместо ±20%
			waveData.Energy[i] = math.Max(0.05, math.Min(1.0, waveData.Energy[i]+energyVariation))

			// Incidence (угол падения) - упрощённо, считаем что волны приходят спереди
			waveData.Incidence[i] = 0.5 // среднее значение
		}
	} else {
		// Без батиметрии используем упрощённые значения
		for i := range points {
			waveData.Energy[i] = 0.5    // средняя энергия
			waveData.Fetch[i] = 50000.0 // 50km
			waveData.Incidence[i] = 0.5
		}
	}

	return waveData
}

// calculateFetchDistance рассчитывает fetch distance до берега
func calculateFetchDistance(
	point geometry.LatLon,
	samplePoints []geometry.LatLon,
	bathymetryGrid *geometry.BathymetryGrid,
) float64 {

	// Упрощённый расчёт: расстояние до ближайшей точки берега
	minDist := math.Inf(1)
	for _, sample := range samplePoints {
		dist := geometry.Haversine(point, sample)
		if dist < minDist {
			minDist = dist
		}
	}

	if minDist == math.Inf(1) {
		return 50000.0 // 50km по умолчанию
	}

	return minDist
}

// prepareLithologyStates подготавливает литологию для каждой точки
func prepareLithologyStates(
	points []geometry.LatLon,
	lithologyProfile *geometry.LithologyProfile,
) []geometry.LithologyState {

	n := len(points)
	lithology := make([]geometry.LithologyState, n)

	// Если есть профиль литологии, используем его
	if lithologyProfile != nil && len(lithologyProfile.Points) > 0 {
		for i, point := range points {
			// Находим литологию в точке
			lithInfo := lithologyProfile.GetLithologyAt(point.Lat, point.Lon)
			lithology[i] = geometry.LithologyState{
				Class:       lithInfo.Class,
				Resistance:  lithInfo.Resistance,
				Color:       lithInfo.Color,
				Description: lithInfo.Description,
			}
		}
	} else {
		// Используем дефолтную литологию (средняя сопротивляемость)
		for i := range lithology {
			lithology[i] = geometry.LithologyState{
				Class:       "sediment",
				Resistance:  2.0, // средняя сопротивляемость
				Color:       "#d4a574",
				Description: "Medium resistance sediment",
			}
		}
	}

	return lithology
}
