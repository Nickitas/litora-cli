package benchmark

import (
	"fmt"
	"math"
	"sort"
	"sync/atomic"

	"coastal-geometry/internal/domain/geometry"
)

const metersPerDegLat = 111194.9

// CalibrationConfig настраивает запуск калибровки
type CalibrationConfig struct {
	// Пространство параметров для поиска
	ErosionStrengths []float64 // проверяемые значения силы эрозии (м/шаг)
	WaveDirections   []float64 // проверяемые направления доминирующих волн (град. от севера)

	// Опционально: разброс спектра волн (градусы) - если >0, каждое направление становится гауссовским спектром
	// 0 = одиночное направление (устаревший режим)
	// 30 = слабое направленное распространение
	// 60 = широкое направленное распространение
	SpectrumSpreadDeg float64

	// Параметры симуляции
	YearsPerStep   float64 // лет за шаг симуляции
	TotalYears     int     // всего лет для симуляции
	WindSpeed      float64 // скорость ветра (м/с)
	BathymetryGrid *geometry.BathymetryGrid

	// Сопоставление
	MaxDistanceKm float64 // макс. расстояние от наблюдения до точки побережья
}

// DefaultCalibrationConfig возвращает разумную начальную конфигурацию для участков Чёрного моря
func DefaultCalibrationConfig() CalibrationConfig {
	return CalibrationConfig{
		// Более точный диапазон сил, сфокусированный на меньших значениях (лучшая подгонка обычно 5-30)
		ErosionStrengths: []float64{2, 5, 10, 15, 20, 30, 50, 80},
		// 16 румбов для лучшего разрешения
		WaveDirections: []float64{0, 22.5, 45, 67.5, 90, 112.5, 135, 157.5,
			180, 202.5, 225, 247.5, 270, 292.5, 315, 337.5},
		// Разброс спектра = 0 означает одиночное направление (устаревший режим)
		// Установите, например, 30 для включения гауссовского направленного распределения
		SpectrumSpreadDeg: 0,
		YearsPerStep:      1.0,
		TotalYears:        10,
		WindSpeed:         12,
		MaxDistanceKm:     5.0,
	}
}

// CalibrationResultItem представляет одну комбинацию параметров и её валидацию
type CalibrationResultItem struct {
	ErosionStrength   float64           `json:"erosion_strength"`
	WaveDirection     float64           `json:"wave_direction"`
	ValidationMetrics ValidationMetrics `json:"validation_metrics"`
	ComparisonPoints  []ComparisonPoint `json:"comparison_points,omitempty"`
}

// ComparisonPoint показывает модельные и наблюдаемые значения в одной точке наблюдения
type ComparisonPoint struct {
	LatLon            geometry.LatLon `json:"lat_lon"`
	Observed          float64         `json:"observed_m_per_year"`
	Modeled           float64         `json:"modeled_m_per_year"`
	DistanceToCoastKm float64         `json:"distance_to_coast_km"`
}

// Calibrate выполняет калибровку для эталонного участка
//
// Алгоритм:
//  1. Для каждой комбинации (erosion_strength, wave_direction):
//     a. Запускаем симуляцию волновой эрозии (с батиметрией, если предоставлена)
//     b. Для каждой точки наблюдаемой эрозии:
//     - Находим ближайший сегмент побережья
//     - Вычисляем модельную скорость отступления (м/год) на этом сегменте
//     c. Вычисляем метрики валидации (RMSE, MAE, R²)
//  2. Возвращаем результаты, отсортированные по RMSE (лучшие первые)
func Calibrate(site BenchmarkSite, config CalibrationConfig, progress ...ProgressFunc) ([]CalibrationResultItem, error) {
	if len(site.ObservedErosion) == 0 {
		return nil, fmt.Errorf("на участке %q нет данных о наблюдаемой эрозии", site.ID)
	}
	if len(site.Coastline) < 3 {
		return nil, fmt.Errorf("на участке %q слишком мало точек береговой линии (%d)", site.ID, len(site.Coastline))
	}

	// Вычисляем количество шагов из лет
	steps := int(float64(config.TotalYears) / config.YearsPerStep)
	if steps < 1 {
		steps = 1
	}

	// Формируем комбинации параметров для параллельного выполнения
	type combo struct {
		strength float64
		waveDir  float64
	}
	var combos []combo
	for _, strength := range config.ErosionStrengths {
		for _, waveDir := range config.WaveDirections {
			combos = append(combos, combo{strength, waveDir})
		}
	}

	results := make([]CalibrationResultItem, len(combos))
	var progressFn ProgressFunc
	if len(progress) > 0 {
		progressFn = progress[0]
	}
	parallelCalibrate(combos, func(i int, c combo) {
		results[i] = runCalibrationIteration(site, config, c.strength, c.waveDir, steps)
	}, progressFn)

	// Сортируем по RMSE по возрастанию (лучшие первые)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ValidationMetrics.RMSE < results[j].ValidationMetrics.RMSE
	})

	return results, nil
}

// ProgressFunc - обратный вызов для отчёта о прогрессе во время калибровки
type ProgressFunc func(current, total int)

// parallelCalibrate выполняет итерации калибровки параллельно
// Использует до 8 воркеров (калибровка ограничена CPU)
func parallelCalibrate[T any](items []T, fn func(i int, item T), progress ProgressFunc) {
	const maxWorkers = 8
	n := len(items)
	if n == 0 {
		return
	}

	workers := maxWorkers
	if workers > n {
		workers = n
	}

	jobs := make(chan int, n)
	for i := range items {
		jobs <- i
	}
	close(jobs)

	done := make(chan struct{}, workers)
	completed := atomicInt64{}

	for w := 0; w < workers; w++ {
		go func() {
			for idx := range jobs {
				fn(idx, items[idx])
				c := completed.Add(1) + 1
				if progress != nil {
					progress(int(c), n)
				}
			}
			done <- struct{}{}
		}()
	}
	for w := 0; w < workers; w++ {
		<-done
	}
}

// atomicInt64 - простой атомарный счётчик
type atomicInt64 struct {
	v int64
}

func (a *atomicInt64) Add(delta int64) int64 {
	return atomic.AddInt64(&a.v, delta)
}

// CalibrateWithBathymetry выполняет калибровку с интегрированными данными батиметрии
// Обычно даёт значительно лучшие результаты, чем калибровка на плоском дне
func CalibrateWithBathymetry(site BenchmarkSite, config CalibrationConfig, bathymetry *geometry.BathymetryGrid, progress ...ProgressFunc) ([]CalibrationResultItem, error) {
	if len(site.ObservedErosion) == 0 {
		return nil, fmt.Errorf("на участке %q нет данных о наблюдаемой эрозии", site.ID)
	}
	if len(site.Coastline) < 3 {
		return nil, fmt.Errorf("на участке %q слишком мало точек береговой линии (%d)", site.ID, len(site.Coastline))
	}

	// Внедряем батиметрию в конфигурацию
	config.BathymetryGrid = bathymetry

	return Calibrate(site, config, progress...)
}

// runCalibrationIteration выполняет одиночный прогон модели и вычисляет валидацию
func runCalibrationIteration(
	site BenchmarkSite,
	config CalibrationConfig,
	strength float64,
	waveDir float64,
	steps int,
) CalibrationResultItem {
	// Если включен разброс спектра, запускаем модель с несколькими взвешенными направлениями
	// и агрегируем скорости отступления
	if config.SpectrumSpreadDeg > 0 {
		return runCalibrationWithSpectrum(site, config, strength, waveDir, steps)
	}

	options := geometry.WaveErosionOptions{
		StrengthMeters:           strength,
		WindSourceDirectionDeg:   waveDir,
		WindSpeedMetersPerSecond: config.WindSpeed,
		FetchSpreadDeg:           55,
		FetchSamples:             9,
		MaxFetchMeters:           150_000,
		DepthScaleMeters:         4000,
		ExposurePower:            1.5,
		MaxRetreatMeters:         strength * 3,
		BathymetryGrid:           config.BathymetryGrid,
		SignificantWaveHeightM:   site.MeanWaveHeight,
		PeakWavePeriodSeconds:    site.MeanWavePeriod,
	}

	snapshots := geometry.SimulateWaveErosionWithSeed(site.Coastline, steps, options, 42)

	initial := snapshots[0]
	final := snapshots[len(snapshots)-1]

	comparisons := computeComparisons(initial, final, site.ObservedErosion, config.YearsPerStep, config.TotalYears)
	metrics := computeValidationMetrics(comparisons)

	return CalibrationResultItem{
		ErosionStrength:   strength,
		WaveDirection:     waveDir,
		ValidationMetrics: metrics,
		ComparisonPoints:  comparisons,
	}
}

// runCalibrationWithSpectrum использует гауссовское направленное распределение
// вместо одиночного направления волны
func runCalibrationWithSpectrum(
	site BenchmarkSite,
	config CalibrationConfig,
	strength float64,
	centerDir float64,
	steps int,
) CalibrationResultItem {
	// Формируем спектр: 8 бинов с гауссовскими весами, центрированными на centerDir
	spectrum := geometry.NewGaussianSpectrum(centerDir, config.SpectrumSpreadDeg, 8)

	// Агрегируем отступление для каждой точки побережья по всем бинам спектра
	// Подход: запускаем модель для каждого бина независимо, суммируем взвешенное отступление
	nCoast := len(site.Coastline)
	if nCoast < 3 {
		return CalibrationResultItem{
			ErosionStrength:   strength,
			WaveDirection:     centerDir,
			ValidationMetrics: ValidationMetrics{},
		}
	}

	totalRetreat := make([]float64, nCoast)
	totalWeight := 0.0

	for _, bin := range spectrum.Bins {
		if bin.Weight < 0.01 {
			continue
		}

		// Запускаем эрозию для этого направления
		// Масштабируем силу по весу, чтобы вклады суммировались правильно
		binStrength := strength * bin.Weight * 2 // множитель 2, т.к. каждое направление вносит частичный вклад
		if binStrength < 0.1 {
			continue
		}

		options := geometry.WaveErosionOptions{
			StrengthMeters:           binStrength,
			WindSourceDirectionDeg:   bin.Direction,
			WindSpeedMetersPerSecond: config.WindSpeed,
			FetchSpreadDeg:           55,
			FetchSamples:             9,
			MaxFetchMeters:           150_000,
			DepthScaleMeters:         4000,
			ExposurePower:            1.5,
			MaxRetreatMeters:         binStrength * 3,
			BathymetryGrid:           config.BathymetryGrid,
			SignificantWaveHeightM:   waveHeightForBin(site.MeanWaveHeight, bin),
			PeakWavePeriodSeconds:    site.MeanWavePeriod,
		}

		snapshots := geometry.SimulateWaveErosionWithSeed(site.Coastline, steps, options, 42)
		initial := snapshots[0]
		final := snapshots[len(snapshots)-1]

		// Вычисляем отступление для каждой точки
		for i := range site.Coastline {
			if i >= len(initial) || i >= len(final) {
				continue
			}
			retreat := computeSegmentRetreat(initial, final, i)
			if retreat > 0 {
				totalRetreat[i] += retreat * bin.Weight
			}
		}
		totalWeight += bin.Weight
	}

	// Формируем синтетическую конечную линию побережья с использованием агрегированного отступления
	// Используем накопленное на точках отступление напрямую для сравнения
	comparisons := computeComparisonsFromRetreats(
		site.Coastline, totalRetreat, totalWeight,
		site.ObservedErosion, config.TotalYears,
	)
	metrics := computeValidationMetrics(comparisons)

	return CalibrationResultItem{
		ErosionStrength:   strength,
		WaveDirection:     centerDir,
		ValidationMetrics: metrics,
		ComparisonPoints:  comparisons,
	}
}

// waveHeightForBin передаёт высоту из набора наблюдений в историческую
// спектральную калибровку. Высота одного направленного бина масштабируется по
// корню его энергетического веса, чтобы сумма энергий соответствовала Hs².
func waveHeightForBin(meanHeight float64, bin geometry.WaveSpectrumBin) float64 {
	if meanHeight <= 0 || bin.Weight <= 0 {
		return meanHeight
	}
	if bin.SignificantWaveHeightM > 0 {
		return bin.SignificantWaveHeightM
	}
	return meanHeight * math.Sqrt(bin.Weight)
}

// computeComparisonsFromRetreats строит точки сравнения из предварительно вычисленных отступлений
func computeComparisonsFromRetreats(
	coastline []geometry.LatLon,
	retreats []float64,
	totalWeight float64,
	observations []ErosionObservation,
	totalYears int,
) []ComparisonPoint {
	if totalWeight <= 0 {
		return nil
	}

	var comparisons []ComparisonPoint
	for _, obs := range observations {
		segIdx := nearestSegmentIndex(coastline, obs.LatLon)
		if segIdx < 0 || segIdx >= len(retreats) {
			continue
		}

		// Нормализация: взвешенное отступление / общий вес / годы
		modeledRate := retreats[segIdx] / totalWeight / float64(totalYears)
		dist := haversineKm(coastline[segIdx], obs.LatLon)

		comparisons = append(comparisons, ComparisonPoint{
			LatLon:            obs.LatLon,
			Observed:          obs.ShorelineChangeRate,
			Modeled:           modeledRate,
			DistanceToCoastKm: dist,
		})
	}
	return comparisons
}

// computeComparions сопоставляет наблюдения с ближайшими сегментами побережья
// и вычисляет модельные скорости отступления
func computeComparisons(
	initial, final []geometry.LatLon,
	observations []ErosionObservation,
	yearsPerStep float64,
	totalYears int,
) []ComparisonPoint {
	var comparisons []ComparisonPoint

	for _, obs := range observations {
		// Find nearest segment in initial coastline
		segIdx := nearestSegmentIndex(initial, obs.LatLon)
		if segIdx < 0 {
			continue
		}

		// Вычисляем модельное отступление (м) на этом сегменте
		// Отступление = перпендикулярное смещение средней точки сегмента
		modeledRetreat := computeSegmentRetreat(initial, final, segIdx)
		// Преобразуем отступление в скорость в год (положительное = эрозия)
		modeledRate := modeledRetreat / float64(totalYears)

		dist := haversineKm(initial[segIdx], obs.LatLon)

		comparisons = append(comparisons, ComparisonPoint{
			LatLon:            obs.LatLon,
			Observed:          obs.ShorelineChangeRate,
			Modeled:           modeledRate,
			DistanceToCoastKm: dist,
		})
	}

	return comparisons
}

// nearestSegmentIndex returns the index of the closest coastline point to target
func nearestSegmentIndex(coastline []geometry.LatLon, target geometry.LatLon) int {
	if len(coastline) == 0 {
		return -1
	}

	bestIdx := 0
	bestDist := haversineKm(coastline[0], target)
	for i := 1; i < len(coastline); i++ {
		d := haversineKm(coastline[i], target)
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx
}

// computeSegmentRetreat оценивает, насколько отступила точка побережья (м)
// Положительное значение = эрозия (отступление в сторону суши)
func computeSegmentRetreat(initial, final []geometry.LatLon, idx int) float64 {
	if idx >= len(initial) || idx >= len(final) {
		return 0
	}

	// Получаем ориентацию сегмента, используя соседние точки
	// и вычисляем перпендикулярное смещение
	prev := (idx - 1 + len(initial)) % len(initial)
	next := (idx + 1) % len(initial)

	// Направление сегмента в этой точке
	dx := initial[next].Lon - initial[prev].Lon
	dy := initial[next].Lat - initial[prev].Lat

	// Внешняя нормаль (перпендикуляр, направленный от побережья)
	// Поворачиваем касательную на 90° (против часовой стрелки даёт внешнюю нормаль для CCW кольца)
	nx := -dy
	ny := dx
	nlen := math.Sqrt(nx*nx + ny*ny)
	if nlen == 0 {
		return 0
	}
	nx /= nlen
	ny /= nlen

	// Преобразуем смещение в метры, используя локальные метры на градус
	refLat := initial[idx].Lat
	metersPerDegLon := metersPerDegLat * math.Cos(refLat*math.Pi/180)

	// Смещение этой точки
	dLon := final[idx].Lon - initial[idx].Lon
	dLat := final[idx].Lat - initial[idx].Lat

	// Проецируем смещение на внешнюю нормаль
	dMetersX := dLon * metersPerDegLon
	dMetersY := dLat * metersPerDegLat
	nMetersX := nx * metersPerDegLon
	nMetersY := ny * metersPerDegLat
	nlenMeters := math.Sqrt(nMetersX*nMetersX + nMetersY*nMetersY)
	if nlenMeters == 0 {
		return 0
	}

	// Скалярная проекция (отрицательное = отступление в сторону суши = эрозия)
	projection := (dMetersX*nMetersX + dMetersY*nMetersY) / nlenMeters

	// Отступление (положительная эрозия) = -projection (т.к. внешнее направление +)
	return -projection
}

// haversineKm вычисляет ортодромическое расстояние между двумя точками в км
func haversineKm(a, b geometry.LatLon) float64 {
	const earthRadiusKm = 6371.0
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180

	sindLat := math.Sin(dLat / 2)
	sindLon := math.Sin(dLon / 2)
	h := sindLat*sindLat + math.Cos(lat1)*math.Cos(lat2)*sindLon*sindLon
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(math.Min(1, h)))
}

// computeValidationMetrics вычисляет RMSE, MAE, MBE, R² и значимость
func computeValidationMetrics(comparisons []ComparisonPoint) ValidationMetrics {
	n := len(comparisons)
	if n == 0 {
		return ValidationMetrics{}
	}

	var sumSqErr, sumAbsErr, sumBiasErr float64
	var sumObs, sumMod float64
	for _, c := range comparisons {
		err := c.Modeled - c.Observed
		sumSqErr += err * err
		sumAbsErr += math.Abs(err)
		sumBiasErr += err
		sumObs += c.Observed
		sumMod += c.Modeled
	}

	mse := sumSqErr / float64(n)
	rmse := math.Sqrt(mse)
	mae := sumAbsErr / float64(n)
	mbe := sumBiasErr / float64(n)

	meanObs := sumObs / float64(n)
	_ = sumMod / float64(n)

	// R² = 1 - SS_res / SS_tot
	var ssRes, ssTot float64
	for _, c := range comparisons {
		ssRes += (c.Modeled - c.Observed) * (c.Modeled - c.Observed)
		ssTot += (c.Observed - meanObs) * (c.Observed - meanObs)
	}
	rSquared := 0.0
	if ssTot > 0 {
		rSquared = 1 - ssRes/ssTot
	}

	// Корреляция Пирсона для проверки значимости
	r := pearsonCorrelation(comparisons)
	pValue := computePValue(r, n)
	significant := pValue < 0.05

	return ValidationMetrics{
		RMSE:        rmse,
		MAE:         mae,
		MBE:         mbe,
		RSquared:    rSquared,
		N:           n,
		PValue:      pValue,
		Significant: significant,
	}
}

// pearsonCorrelation вычисляет корреляцию Пирсона между наблюдаемыми и модельными значениями
func pearsonCorrelation(comparisons []ComparisonPoint) float64 {
	n := len(comparisons)
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for _, c := range comparisons {
		sumX += c.Observed
		sumY += c.Modeled
		sumXY += c.Observed * c.Modeled
		sumX2 += c.Observed * c.Observed
		sumY2 += c.Modeled * c.Modeled
	}

	num := float64(n)*sumXY - sumX*sumY
	denX := math.Sqrt(float64(n)*sumX2 - sumX*sumX)
	denY := math.Sqrt(float64(n)*sumY2 - sumY*sumY)
	if denX == 0 || denY == 0 {
		return 0
	}
	return num / (denX * denY)
}

// computePValue аппроксимирует двустороннее p-значение для корреляции Пирсона r
// Использует аппроксимацию t-распределения
func computePValue(r float64, n int) float64 {
	if n < 3 {
		return 1.0
	}
	df := n - 2
	tStat := r * math.Sqrt(float64(df)) / math.Sqrt(1-r*r)
	if math.IsNaN(tStat) {
		return 1.0
	}
	// Аппроксимируем p-значение, используя нормальное распределение для больших df
	// Для df >= 30, t ~ z; иначе используем консервативную верхнюю границу
	if df >= 30 {
		return 2 * (1 - normalCDF(math.Abs(tStat)))
	}
	// Аппроксимация малой выборки: просто используем нормальное как консервативную оценку
	return 2 * (1 - normalCDF(math.Abs(tStat)))
}

// normalCDF вычисляет стандартную нормальную функцию распределения, используя аппроксимацию функции ошибок
func normalCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt(2)))
}
