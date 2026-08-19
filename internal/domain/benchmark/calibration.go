package benchmark

import (
	"fmt"
	"math"
	"sort"
	"sync/atomic"
	"time"

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
	YearsPerStep float64 // лет за шаг симуляции
	// TotalYears сохранён для исторических сценариев; Calibrate использует
	// фактический срок каждого наблюдения из StartDate и EndDate.
	TotalYears     int
	WindSpeed      float64 // скорость ветра (м/с)
	BathymetryGrid *geometry.BathymetryGrid

	// Сопоставление
	MaxDistanceKm float64 // макс. расстояние от наблюдения до проекции на сегмент побережья

	// Проверка. Отложенная выборка формируется пространственно: завершающий
	// участок сопоставленных наблюдений по ходу береговой линии резервируется для проверки.
	ValidationFraction float64 // доля отложенной выборки; 0 отключает проверку
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
		SpectrumSpreadDeg:  0,
		YearsPerStep:       1.0,
		TotalYears:         10,
		WindSpeed:          12,
		MaxDistanceKm:      5.0,
		ValidationFraction: 0.25,
	}
}

// CalibrationResultItem представляет одну комбинацию параметров и её валидацию
type CalibrationResultItem struct {
	ErosionStrength float64 `json:"erosion_strength"`
	WaveDirection   float64 `json:"wave_direction"`
	// ValidationMetrics вычислены только на отложенной выборке и не используются
	// для выбора параметров. При слишком малом числе точек они содержат только
	// ошибки, без статистического вывода.
	ValidationMetrics ValidationMetrics `json:"validation_metrics"`
	TrainingMetrics   ValidationMetrics `json:"training_metrics"`
	Matching          MatchingSummary   `json:"matching"`
	ComparisonPoints  []ComparisonPoint `json:"comparison_points,omitempty"`
}

// ComparisonPoint показывает модельные и наблюдаемые значения в одной точке наблюдения
type ComparisonPoint struct {
	LatLon            geometry.LatLon `json:"lat_lon"`
	Observed          float64         `json:"observed_m_per_year"`
	Modeled           float64         `json:"modeled_m_per_year"`
	Uncertainty       float64         `json:"uncertainty_m_per_year"`
	ObservationYears  float64         `json:"observation_years"`
	DistanceToCoastKm float64         `json:"distance_to_coast_km"`
	CoastSegment      int             `json:"coast_segment"`
	SegmentPosition   float64         `json:"segment_position"`
	Split             string          `json:"split"`
	observationIndex  int
}

// MatchingSummary фиксирует, какие наблюдения были допущены к сравнению.
// Наблюдения дальше MaxDistanceKm исключаются до подбора параметров.
type MatchingSummary struct {
	Candidates                 int                  `json:"candidates"`
	Accepted                   int                  `json:"accepted"`
	ExcludedByDistance         int                  `json:"excluded_by_distance"`
	ExcludedInvalidPeriod      int                  `json:"excluded_invalid_period"`
	ExcludedInvalidUncertainty int                  `json:"excluded_invalid_uncertainty"`
	MaxDistanceKm              float64              `json:"max_distance_km"`
	MaximumMatchedKm           float64              `json:"maximum_matched_distance_km"`
	UniqueCoastSegments        int                  `json:"unique_coast_segments"`
	TrainingObservations       int                  `json:"training_observations"`
	ValidationObservations     int                  `json:"validation_observations"`
	Warnings                   []string             `json:"warnings,omitempty"`
	Diagnostics                []MatchingDiagnostic `json:"diagnostics"`
}

// MatchingDiagnostic описывает результат проверки одного исходного наблюдения.
// Запись включается в отчёт независимо от того, было ли наблюдение допущено.
type MatchingDiagnostic struct {
	ObservationIndex  int             `json:"observation_index"`
	LatLon            geometry.LatLon `json:"lat_lon"`
	StartDate         string          `json:"start_date"`
	EndDate           string          `json:"end_date"`
	ObservationYears  float64         `json:"observation_years"`
	Uncertainty       float64         `json:"uncertainty_m_per_year"`
	DistanceToCoastKm float64         `json:"distance_to_coast_km"`
	CoastSegment      int             `json:"coast_segment"`
	SegmentPosition   float64         `json:"segment_position"`
	Projected         bool            `json:"projected"`
	Split             string          `json:"split,omitempty"`
	Status            string          `json:"status"`
	Reason            string          `json:"reason,omitempty"`
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
	if config.MaxDistanceKm <= 0 {
		return nil, fmt.Errorf("максимальное расстояние до береговой линии должно быть больше нуля")
	}
	if config.YearsPerStep <= 0 || config.TotalYears <= 0 {
		return nil, fmt.Errorf("длительность шага и общий срок моделирования должны быть больше нуля")
	}
	if config.ValidationFraction < 0 || config.ValidationFraction >= 1 {
		return nil, fmt.Errorf("доля отложенной выборки должна быть в диапазоне от 0 до 1, не включая 1")
	}
	if len(config.ErosionStrengths) == 0 || len(config.WaveDirections) == 0 {
		return nil, fmt.Errorf("задайте хотя бы одну силу эрозии и одно направление волны")
	}

	// До запуска модели отсеиваем точки, не относящиеся к данному береговому
	// профилю. Это одновременно проверяет, что у калибровки есть данные.
	prepared, matching := prepareComparisonLocations(site.Coastline, site.ObservedErosion, config)
	if len(prepared) == 0 {
		return nil, fmt.Errorf("ни одно из %d наблюдений не находится не дальше %.2f км от береговой линии", matching.Candidates, config.MaxDistanceKm)
	}

	// Симуляция идёт до самого длинного допустимого периода наблюдения. Для
	// каждой точки далее выбирается собственный момент времени, а не общий
	// устаревший горизонт TotalYears.
	steps := calibrationSteps(matching.Diagnostics, config)

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

	// Выбор параметров происходит только по обучающей выборке. Отложенная
	// выборка служит для независимой оценки уже выбранной комбинации.
	sort.Slice(results, func(i, j int) bool {
		return results[i].TrainingMetrics.WeightedRMSE < results[j].TrainingMetrics.WeightedRMSE
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
	// Внутренние вызовы из анализа чувствительности могли передать старый
	// фиксированный горизонт. Никогда не сокращаем расчёт ниже самого длинного
	// периода принятых наблюдений.
	_, matching := prepareComparisonLocations(site.Coastline, site.ObservedErosion, config)
	if requiredSteps := calibrationSteps(matching.Diagnostics, config); steps < requiredSteps {
		steps = requiredSteps
	}

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

	comparisons, matching := computeComparisons(site.Coastline, snapshots, site.ObservedErosion, config)
	training, validation := splitComparisonPoints(comparisons)
	trainingMetrics := computeValidationMetricsWithTests(training, len(config.ErosionStrengths)*len(config.WaveDirections))
	validationMetrics := computeValidationMetricsWithTests(validation, len(config.ErosionStrengths)*len(config.WaveDirections))

	return CalibrationResultItem{
		ErosionStrength:   strength,
		WaveDirection:     waveDir,
		ValidationMetrics: validationMetrics,
		TrainingMetrics:   trainingMetrics,
		Matching:          matching,
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

	totalRetreatByStep := make([][]float64, steps+1)
	for step := range totalRetreatByStep {
		totalRetreatByStep[step] = make([]float64, nCoast)
	}
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
		// Сохраняем отступление для каждого снимка: разные наблюдения имеют
		// разные даты начала и окончания, поэтому единственного final недостаточно.
		for step, snapshot := range snapshots {
			for i := range site.Coastline {
				if i >= len(snapshot) {
					continue
				}
				retreat := computeSegmentRetreat(site.Coastline, snapshot, i)
				if retreat > 0 {
					totalRetreatByStep[step][i] += retreat * bin.Weight
				}
			}
		}
		totalWeight += bin.Weight
	}

	// Формируем синтетическую конечную линию побережья с использованием агрегированного отступления
	// Используем накопленное на точках отступление напрямую для сравнения
	comparisons, matching := computeComparisonsFromRetreats(
		site.Coastline, totalRetreatByStep, totalWeight,
		site.ObservedErosion, config,
	)
	training, validation := splitComparisonPoints(comparisons)
	trainingMetrics := computeValidationMetricsWithTests(training, len(config.ErosionStrengths)*len(config.WaveDirections))
	validationMetrics := computeValidationMetricsWithTests(validation, len(config.ErosionStrengths)*len(config.WaveDirections))

	return CalibrationResultItem{
		ErosionStrength:   strength,
		WaveDirection:     centerDir,
		ValidationMetrics: validationMetrics,
		TrainingMetrics:   trainingMetrics,
		Matching:          matching,
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
	retreatsByStep [][]float64,
	totalWeight float64,
	observations []ErosionObservation,
	config CalibrationConfig,
) ([]ComparisonPoint, MatchingSummary) {
	if totalWeight <= 0 {
		return nil, MatchingSummary{Candidates: len(observations), MaxDistanceKm: config.MaxDistanceKm}
	}

	comparisons, summary := prepareComparisonLocations(coastline, observations, CalibrationConfig{
		MaxDistanceKm: config.MaxDistanceKm, ValidationFraction: config.ValidationFraction,
	})
	for i := range comparisons {
		c := &comparisons[i]
		lower, upper, fraction := observationSnapshotPosition(c.ObservationYears, config.YearsPerStep, len(retreatsByStep))
		if lower < 0 || upper < 0 || c.CoastSegment+1 >= len(retreatsByStep[lower]) || c.CoastSegment+1 >= len(retreatsByStep[upper]) {
			continue
		}
		retreatAtLower := retreatOnSegment(retreatsByStep[lower], c.CoastSegment, c.SegmentPosition)
		retreatAtUpper := retreatOnSegment(retreatsByStep[upper], c.CoastSegment, c.SegmentPosition)
		retreat := retreatAtLower + fraction*(retreatAtUpper-retreatAtLower)
		c.Modeled = retreat / totalWeight / c.ObservationYears
	}
	return comparisons, summary
}

// computeComparisons сопоставляет наблюдения с проекцией на ближайший сегмент
// и вычисляет модельные скорости отступления
func computeComparisons(
	initial []geometry.LatLon,
	snapshots [][]geometry.LatLon,
	observations []ErosionObservation,
	config CalibrationConfig,
) ([]ComparisonPoint, MatchingSummary) {
	comparisons, summary := prepareComparisonLocations(initial, observations, CalibrationConfig{
		MaxDistanceKm: config.MaxDistanceKm, ValidationFraction: config.ValidationFraction,
	})
	retreatsByStep := make([][]float64, len(snapshots))
	for step, snapshot := range snapshots {
		retreatsByStep[step] = make([]float64, len(initial))
		for i := range initial {
			retreatsByStep[step][i] = computeSegmentRetreat(initial, snapshot, i)
		}
	}
	for i := range comparisons {
		c := &comparisons[i]
		lower, upper, fraction := observationSnapshotPosition(c.ObservationYears, config.YearsPerStep, len(retreatsByStep))
		if lower < 0 || upper < 0 {
			continue
		}
		retreatAtLower := retreatOnSegment(retreatsByStep[lower], c.CoastSegment, c.SegmentPosition)
		retreatAtUpper := retreatOnSegment(retreatsByStep[upper], c.CoastSegment, c.SegmentPosition)
		retreat := retreatAtLower + fraction*(retreatAtUpper-retreatAtLower)
		c.Modeled = retreat / c.ObservationYears
	}
	return comparisons, summary
}

func retreatOnSegment(retreats []float64, segment int, position float64) float64 {
	if segment < 0 || segment+1 >= len(retreats) {
		return 0
	}
	return retreats[segment]*(1-position) + retreats[segment+1]*position
}

// observationSnapshotPosition возвращает два соседних снимка и долю между
// ними для точной длительности наблюдения.
func observationSnapshotPosition(years, yearsPerStep float64, snapshotCount int) (lower, upper int, fraction float64) {
	if years <= 0 || yearsPerStep <= 0 || snapshotCount == 0 {
		return -1, -1, 0
	}
	position := years / yearsPerStep
	lower = int(math.Floor(position))
	upper = int(math.Ceil(position))
	if lower >= snapshotCount {
		return -1, -1, 0
	}
	if upper >= snapshotCount {
		upper = snapshotCount - 1
	}
	return lower, upper, position - float64(lower)
}

// nearestSegmentIndex возвращает индекс ближайшей вершины и сохранён для
// совместимости внутренних вызовов. Сопоставление калибровки использует
// nearestSegmentProjection, а не эту функцию.
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

// nearestSegmentProjection ищет ближайшую точку на ломаной береговой линии.
// Расчёт выполняется в локальной метрической проекции, что достаточно точно
// для допустимой дистанции сопоставления в несколько километров.
func nearestSegmentProjection(coastline []geometry.LatLon, target geometry.LatLon) (segment int, position, distanceKm float64, ok bool) {
	if len(coastline) < 2 {
		return 0, 0, 0, false
	}
	bestDistance := math.Inf(1)
	bestSegment, bestPosition := -1, 0.0
	metersPerDegLon := metersPerDegLat * math.Cos(target.Lat*math.Pi/180)
	for i := 0; i < len(coastline)-1; i++ {
		a, b := coastline[i], coastline[i+1]
		ax, ay := (a.Lon-target.Lon)*metersPerDegLon, (a.Lat-target.Lat)*metersPerDegLat
		bx, by := (b.Lon-target.Lon)*metersPerDegLon, (b.Lat-target.Lat)*metersPerDegLat
		dx, dy := bx-ax, by-ay
		denominator := dx*dx + dy*dy
		if denominator == 0 {
			continue
		}
		t := -(ax*dx + ay*dy) / denominator
		t = math.Max(0, math.Min(1, t))
		x, y := ax+t*dx, ay+t*dy
		distance := x*x + y*y
		if distance < bestDistance {
			bestDistance, bestSegment, bestPosition = distance, i, t
		}
	}
	if bestSegment < 0 {
		return 0, 0, 0, false
	}
	a, b := coastline[bestSegment], coastline[bestSegment+1]
	projected := geometry.LatLon{Lat: a.Lat + bestPosition*(b.Lat-a.Lat), Lon: a.Lon + bestPosition*(b.Lon-a.Lon)}
	return bestSegment, bestPosition, haversineKm(target, projected), true
}

// prepareComparisonLocations выполняет геометрическое сопоставление один раз
// для каждой комбинации и исключает наблюдения вне допустимой полосы берега.
func prepareComparisonLocations(coastline []geometry.LatLon, observations []ErosionObservation, config CalibrationConfig) ([]ComparisonPoint, MatchingSummary) {
	summary := MatchingSummary{Candidates: len(observations), MaxDistanceKm: config.MaxDistanceKm}
	comparisons := make([]ComparisonPoint, 0, len(observations))
	segments := make(map[int]struct{})
	for i, observation := range observations {
		diagnostic := MatchingDiagnostic{
			ObservationIndex: i, LatLon: observation.LatLon,
			StartDate: observation.StartDate, EndDate: observation.EndDate,
		}
		diagnostic.Uncertainty = observation.Uncertainty
		if observation.Uncertainty <= 0 {
			summary.ExcludedInvalidUncertainty++
			diagnostic.Status = "исключено"
			diagnostic.Reason = "неопределённость скорости должна быть больше нуля"
			summary.Diagnostics = append(summary.Diagnostics, diagnostic)
			continue
		}
		years, err := observationPeriodYears(observation)
		if err != nil {
			summary.ExcludedInvalidPeriod++
			diagnostic.Status, diagnostic.Reason = "исключено", err.Error()
			summary.Diagnostics = append(summary.Diagnostics, diagnostic)
			continue
		}
		diagnostic.ObservationYears = years
		segment, position, distanceKm, ok := nearestSegmentProjection(coastline, observation.LatLon)
		if !ok {
			summary.ExcludedByDistance++
			diagnostic.Status = "исключено"
			diagnostic.Reason = "невозможно спроецировать точку на береговую линию"
			summary.Diagnostics = append(summary.Diagnostics, diagnostic)
			continue
		}
		diagnostic.DistanceToCoastKm = distanceKm
		diagnostic.CoastSegment, diagnostic.SegmentPosition = segment, position
		diagnostic.Projected = true
		if distanceKm > config.MaxDistanceKm {
			summary.ExcludedByDistance++
			diagnostic.Status = "исключено"
			diagnostic.Reason = "дистанция до береговой линии превышает допустимый предел"
			summary.Diagnostics = append(summary.Diagnostics, diagnostic)
			continue
		}
		comparisons = append(comparisons, ComparisonPoint{
			LatLon: observation.LatLon, Observed: observation.ShorelineChangeRate,
			Uncertainty: observation.Uncertainty, DistanceToCoastKm: distanceKm,
			ObservationYears: years, CoastSegment: segment, SegmentPosition: position, observationIndex: i,
		})
		diagnostic.Status = "принято"
		summary.Diagnostics = append(summary.Diagnostics, diagnostic)
		segments[segment] = struct{}{}
		if distanceKm > summary.MaximumMatchedKm {
			summary.MaximumMatchedKm = distanceKm
		}
	}
	sort.Slice(comparisons, func(i, j int) bool {
		if comparisons[i].CoastSegment != comparisons[j].CoastSegment {
			return comparisons[i].CoastSegment < comparisons[j].CoastSegment
		}
		if comparisons[i].SegmentPosition != comparisons[j].SegmentPosition {
			return comparisons[i].SegmentPosition < comparisons[j].SegmentPosition
		}
		return comparisons[i].observationIndex < comparisons[j].observationIndex
	})
	validationCount := validationCount(len(comparisons), config.ValidationFraction)
	for i := range comparisons {
		if validationCount > 0 && i >= len(comparisons)-validationCount {
			comparisons[i].Split = "validation"
			summary.ValidationObservations++
		} else {
			comparisons[i].Split = "training"
			summary.TrainingObservations++
		}
	}
	for i := range summary.Diagnostics {
		if summary.Diagnostics[i].Status != "принято" {
			continue
		}
		for _, comparison := range comparisons {
			if comparison.observationIndex == summary.Diagnostics[i].ObservationIndex {
				summary.Diagnostics[i].Split = comparison.Split
				break
			}
		}
	}
	summary.Accepted, summary.UniqueCoastSegments = len(comparisons), len(segments)
	summary.Warnings = matchingWarnings(summary, config.ValidationFraction)
	return comparisons, summary
}

func matchingWarnings(summary MatchingSummary, validationFraction float64) []string {
	var warnings []string
	if summary.Accepted < 4 {
		warnings = append(warnings, "после контроля качества осталось меньше четырёх наблюдений: пространственная проверка не сформируется")
	}
	if summary.Accepted > 0 && summary.UniqueCoastSegments < summary.Accepted {
		warnings = append(warnings, "несколько наблюдений проецируются на один сегмент: они пространственно зависимы")
	}
	if summary.MaximumMatchedKm > 1 {
		warnings = append(warnings, "есть принятые наблюдения дальше 1 км от береговой линии; проверьте геопривязку и обоснованность --max-distance-km")
	}
	if validationFraction > 0 && summary.ValidationObservations == 0 {
		warnings = append(warnings, "отложенная проверка не сформирована из-за недостаточного числа принятых наблюдений")
	}
	return warnings
}

// observationPeriodYears возвращает длительность наблюдения по датам ISO 8601.
// Отсутствующая или обратная дата делает наблюдение непригодным для сравнения:
// подставлять произвольный срок моделирования нельзя.
func observationPeriodYears(observation ErosionObservation) (float64, error) {
	start, err := time.Parse("2006-01-02", observation.StartDate)
	if err != nil {
		return 0, fmt.Errorf("некорректная дата начала %q", observation.StartDate)
	}
	end, err := time.Parse("2006-01-02", observation.EndDate)
	if err != nil {
		return 0, fmt.Errorf("некорректная дата окончания %q", observation.EndDate)
	}
	if !end.After(start) {
		return 0, fmt.Errorf("дата окончания должна быть позже даты начала")
	}
	return end.Sub(start).Hours() / (24 * 365.2425), nil
}

func calibrationSteps(diagnostics []MatchingDiagnostic, config CalibrationConfig) int {
	maxYears := 0.0
	for _, diagnostic := range diagnostics {
		if diagnostic.Status == "принято" && diagnostic.ObservationYears > maxYears {
			maxYears = diagnostic.ObservationYears
		}
	}
	steps := int(math.Ceil(maxYears / config.YearsPerStep))
	if steps < 1 {
		return 1
	}
	return steps
}

func validationCount(n int, fraction float64) int {
	if n < 4 || fraction <= 0 {
		return 0
	}
	count := int(math.Ceil(float64(n) * fraction))
	if count >= n-2 {
		count = n - 3
	}
	if count < 1 {
		return 1
	}
	return count
}

func splitComparisonPoints(comparisons []ComparisonPoint) (training, validation []ComparisonPoint) {
	for _, comparison := range comparisons {
		if comparison.Split == "validation" {
			validation = append(validation, comparison)
		} else {
			training = append(training, comparison)
		}
	}
	return training, validation
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

// computeValidationMetrics вычисляет метрики без поправки на поиск параметров.
// Она сохранена для внутренних и внешних тестов; калибровка использует вариант
// computeValidationMetricsWithTests с числом проверенных комбинаций.
func computeValidationMetrics(comparisons []ComparisonPoint) ValidationMetrics {
	return computeValidationMetricsWithTests(comparisons, 1)
}

// computeValidationMetricsWithTests вычисляет обычные и взвешенные метрики.
// Веса берутся как 1/σ², поэтому менее точные наблюдения вносят меньший вклад
// в критерий выбора. Нулевая неопределённость трактуется как единичный вес,
// но не как ложная «идеальная» точность.
func computeValidationMetricsWithTests(comparisons []ComparisonPoint, tests int) ValidationMetrics {
	n := len(comparisons)
	if n == 0 {
		return ValidationMetrics{}
	}

	var sumSqErr, weightedSumSqErr, sumWeights, sumAbsErr, sumBiasErr float64
	var sumObs, sumMod float64
	for _, c := range comparisons {
		err := c.Modeled - c.Observed
		sumSqErr += err * err
		weight := 1.0
		if c.Uncertainty > 0 {
			weight = 1 / (c.Uncertainty * c.Uncertainty)
		}
		weightedSumSqErr += weight * err * err
		sumWeights += weight
		sumAbsErr += math.Abs(err)
		sumBiasErr += err
		sumObs += c.Observed
		sumMod += c.Modeled
	}

	mse := sumSqErr / float64(n)
	rmse := math.Sqrt(mse)
	weightedRMSE := rmse
	if sumWeights > 0 {
		weightedRMSE = math.Sqrt(weightedSumSqErr / sumWeights)
	}
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
	if tests < 1 {
		tests = 1
	}
	adjustedPValue := math.Min(1, pValue*float64(tests))
	// Точный t-критерий вычисляется и для меньшего n, но при менее чем десяти
	// наблюдениях его нельзя выдавать за надёжный статистический вывод.
	inferenceAllowed := n >= 10
	significant := inferenceAllowed && adjustedPValue < 0.05

	return ValidationMetrics{
		RMSE:             rmse,
		WeightedRMSE:     weightedRMSE,
		MAE:              mae,
		MBE:              mbe,
		RSquared:         rSquared,
		N:                n,
		PValue:           pValue,
		AdjustedPValue:   adjustedPValue,
		Significant:      significant,
		InferenceAllowed: inferenceAllowed,
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

// computePValue вычисляет двустороннее p-значение корреляции Пирсона по
// точному t-распределению Стьюдента. Для n < 3 тест неприменим.
func computePValue(r float64, n int) float64 {
	if n < 3 {
		return 1.0
	}
	if math.Abs(r) >= 1 {
		return 0
	}
	df := n - 2
	tStat := r * math.Sqrt(float64(df)) / math.Sqrt(1-r*r)
	if math.IsNaN(tStat) {
		return 1.0
	}
	x := float64(df) / (float64(df) + tStat*tStat)
	return math.Min(1, regularizedIncompleteBeta(0.5*float64(df), 0.5, x))
}

// regularizedIncompleteBeta вычисляет I_x(a,b) методом непрерывной дроби.
// Он нужен для точного малого-sample t-критерия без внешней зависимости.
func regularizedIncompleteBeta(a, b, x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	logGammaA, _ := math.Lgamma(a)
	logGammaB, _ := math.Lgamma(b)
	logGammaAB, _ := math.Lgamma(a + b)
	logBeta := logGammaA + logGammaB - logGammaAB
	front := math.Exp(a*math.Log(x) + b*math.Log1p(-x) - logBeta)
	if x < (a+1)/(a+b+2) {
		return front * betaContinuedFraction(a, b, x) / a
	}
	return 1 - front*betaContinuedFraction(b, a, 1-x)/b
}

func betaContinuedFraction(a, b, x float64) float64 {
	const iterations = 200
	const epsilon = 3e-14
	const minimum = 1e-300
	qab, qap, qam := a+b, a+1, a-1
	c := 1.0
	d := 1 - qab*x/qap
	if math.Abs(d) < minimum {
		d = minimum
	}
	d = 1 / d
	h := d
	for m := 1; m <= iterations; m++ {
		m2 := float64(2 * m)
		aa := float64(m) * (b - float64(m)) * x / ((qam + m2) * (a + m2))
		d = 1 + aa*d
		if math.Abs(d) < minimum {
			d = minimum
		}
		c = 1 + aa/c
		if math.Abs(c) < minimum {
			c = minimum
		}
		d = 1 / d
		h *= d * c
		aa = -(a + float64(m)) * (qab + float64(m)) * x / ((a + m2) * (qap + m2))
		d = 1 + aa*d
		if math.Abs(d) < minimum {
			d = minimum
		}
		c = 1 + aa/c
		if math.Abs(c) < minimum {
			c = minimum
		}
		d = 1 / d
		delta := d * c
		h *= delta
		if math.Abs(delta-1) < epsilon {
			break
		}
	}
	return h
}
