package geometry

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ============================================================
// ТИПЫ ДАННЫХ
// ============================================================

// LithologyProfile представляет полный литологический профиль региона
type LithologyProfile struct {
	Metadata  LithologyMetadata          `json:"metadata"`
	Points    []LithologyPoint           `json:"points"`
	Classes   map[string]LithologyClass  `json:"classes"`
	Baselines map[string]ErosionBaseline `json:"erosion_baselines"`
}

// LithologyMetadata содержит метаданные профиля
type LithologyMetadata struct {
	Name                   string   `json:"name"`
	Version                string   `json:"version"`
	Status                 string   `json:"status,omitempty"` // уровень подтверждения профиля
	Created                string   `json:"created"`
	RepositoryFirstCommit  string   `json:"repository_first_commit,omitempty"` // первое появление файла в Git
	Sources                []string `json:"sources"`
	Resolution             float64  `json:"resolution"`
	Bounds                 Bounds   `json:"bounds"`
	Regions                []string `json:"regions"`
	EmpiricalValidation    string   `json:"empirical_validation,omitempty"`     // статус независимой проверки
	ContainsInferredValues bool     `json:"contains_inferred_values,omitempty"` // наличие гипотетических точек
	UsageLimitations       []string `json:"usage_limitations,omitempty"`        // ограничения научного применения
	Note                   string   `json:"note,omitempty"`
}

// Bounds представляет географические границы
type Bounds struct {
	MinLat float64 `json:"min_lat"`
	MaxLat float64 `json:"max_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLon float64 `json:"max_lon"`
}

// LithologyPoint представляет одну точку измерения литологии
type LithologyPoint struct {
	Lat             float64  `json:"lat"`
	Lon             float64  `json:"lon"`
	Region          string   `json:"region"`
	Lithology       string   `json:"lithology_class"`
	Resistance      float64  `json:"resistance"`
	Color           string   `json:"color"`
	Description     string   `json:"description"`
	Confidence      string   `json:"confidence"`
	Source          string   `json:"source"`
	ErosionObserved *float64 `json:"erosion_observed,omitempty"`
	Note            string   `json:"note,omitempty"`
	Dynamic         bool     `json:"dynamic,omitempty"`
}

// LithologyClass определяет класс горной породы
type LithologyClass struct {
	Resistance   float64   `json:"resistance"`
	Color        string    `json:"color"`
	Description  string    `json:"description"`
	ErosionRange []float64 `json:"erosion_range,omitempty"`
	Dynamic      bool      `json:"dynamic,omitempty"`
	Note         string    `json:"note,omitempty"`
}

// ErosionBaseline определяет базовые скорости эрозии для классов сопротивления
type ErosionBaseline struct {
	ResistanceRange [2]float64         `json:"resistance_range"`
	ErosionMYear    map[string]float64 `json:"erosion_m_year"`
	Description     string             `json:"description"`
	Note            string             `json:"note,omitempty"`
}

// LithologyState представляет состояние литологии в точке
type LithologyState struct {
	Class       string
	Resistance  float64
	Color       string
	Description string
}

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
	// Взаимодействие эрозии и литологии
	ErosionResistanceFactor float64 // насколько сопротивление снижает эрозию [0-1]
	WeatheringErosionBoost  float64 // насколько выветривание ускоряет эрозию [0-1]

	// Взаимодействие аккумуляции и литологии
	DepositionAdhesionFactor    float64 // сцепление отложений с породой [0-1]
	LithologyTrappingEfficiency float64 // эффективность захвата наносов [0-1]

	// Воздействие штормов на литологию
	StormFractureMultiplier float64 // множитель трещинообразования во время штормов
	StormErosionMultiplier  float64 // множитель эрозии во время штормов

	// Пространственная изменчивость
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

// LithologyErosionStepResult результат одного шага эрозии с учётом литологии
type LithologyErosionStepResult struct {
	ModifiedErosion  []float64
	UpdatedStates    []DynamicLithologyState
	ResistanceBefore []float64
	ResistanceAfter  []float64
	FeedbackApplied  bool
}

// pointDist представляет точку с её расстоянием от запрашиваемого местоположения
type pointDist struct {
	point *LithologyPoint
	dist  float64
}

// ============================================================
// ЗАГРУЗКА И ВАЛИДАЦИЯ
// ============================================================

// LoadLithologyProfile загружает литологический профиль из JSON-данных
func LoadLithologyProfile(data []byte) (*LithologyProfile, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("пустые литологические данные")
	}

	var profile LithologyProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("десериализация литологического профиля: %w", err)
	}

	// Валидация
	if err := validateLithologyProfile(&profile); err != nil {
		return nil, fmt.Errorf("валидация не пройдена: %w", err)
	}

	return &profile, nil
}

// LoadLithologyProfileFromFile загружает литологический профиль из файла
func LoadLithologyProfileFromFile(path string) (*LithologyProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение литологического файла %q: %w", path, err)
	}
	return LoadLithologyProfile(data)
}

// QualityWarnings возвращает ограничения качества литологического профиля,
// которые необходимо показывать перед использованием в расчёте.
func (p *LithologyProfile) QualityWarnings() []string {
	if p == nil {
		return nil
	}

	warnings := make([]string, 0, 3)
	version := strings.ToLower(p.Metadata.Version)
	status := strings.ToLower(p.Metadata.Status)
	validation := strings.ToLower(p.Metadata.EmpiricalValidation)
	if strings.Contains(version, "alpha") || status != "validated_empirical" {
		warnings = append(warnings, fmt.Sprintf("литологический профиль %q имеет статус %q и не является подтверждённой эмпирической картой", p.Metadata.Name, p.Metadata.Status))
	}
	if validation != "validated" {
		warnings = append(warnings, "литологические классы и коэффициенты сопротивления не прошли независимую эмпирическую валидацию")
	}

	inferredCount := 0
	for _, point := range p.Points {
		if strings.EqualFold(strings.TrimSpace(point.Source), "inferred") {
			inferredCount++
		}
	}
	if inferredCount > 0 || p.Metadata.ContainsInferredValues {
		warnings = append(warnings, fmt.Sprintf("профиль содержит inferred-значения: %d из %d точек", inferredCount, len(p.Points)))
	}
	return warnings
}

// validateLithologyProfile валидирует литологический профиль
func validateLithologyProfile(profile *LithologyProfile) error {
	// Проверка метаданных
	if profile.Metadata.Name == "" {
		return fmt.Errorf("отсутствует название профиля")
	}

	// Проверка границ
	if profile.Metadata.Bounds.MinLat >= profile.Metadata.Bounds.MaxLat {
		return fmt.Errorf("некорректные границы широты")
	}
	if profile.Metadata.Bounds.MinLon >= profile.Metadata.Bounds.MaxLon {
		return fmt.Errorf("некорректные границы долготы")
	}

	// Проверка точек
	for i, point := range profile.Points {
		if point.Lat < -90 || point.Lat > 90 {
			return fmt.Errorf("точка %d: некорректная широта %.4f", i, point.Lat)
		}
		if point.Lon < -180 || point.Lon > 180 {
			return fmt.Errorf("точка %d: некорректная долгота %.4f", i, point.Lon)
		}
		if point.Resistance <= 0 || point.Resistance > 20 {
			return fmt.Errorf("точка %d: некорректное сопротивление %.2f (ожидается 0-20)", i, point.Resistance)
		}
		if point.Lithology == "" {
			return fmt.Errorf("точка %d: отсутствует класс литологии", i)
		}
	}

	// Проверка классов
	for name, class := range profile.Classes {
		if class.Resistance <= 0 {
			return fmt.Errorf("класс %s: некорректное сопротивление", name)
		}
		if class.Color == "" {
			return fmt.Errorf("класс %s: отсутствует цвет", name)
		}
	}

	return nil
}

// ============================================================
// ПОЛУЧЕНИЕ ЛИТОЛОГИИ
// ============================================================

// GetLithologyAt возвращает состояние литологии в заданной точке с использованием IDW-интерполяции
func (p *LithologyProfile) GetLithologyAt(lat, lon float64) LithologyState {
	// Проверка границ
	if lat < p.Metadata.Bounds.MinLat || lat > p.Metadata.Bounds.MaxLat ||
		lon < p.Metadata.Bounds.MinLon || lon > p.Metadata.Bounds.MaxLon {
		// За пределами границ → возвращаем значение по умолчанию
		return p.getDefaultLithology()
	}

	// Если нет точек → значение по умолчанию
	if len(p.Points) == 0 {
		return p.getDefaultLithology()
	}

	// Для 1 точки используем её напрямую
	if len(p.Points) == 1 {
		point := &p.Points[0]
		if _, ok := p.Classes[point.Lithology]; !ok {
			return p.getDefaultLithology()
		}
		return LithologyState{
			Class:       point.Lithology,
			Resistance:  point.Resistance,
			Color:       point.Color,
			Description: point.Description,
		}
	}

	// IDW интерполяция по N ближайшим точкам
	return p.interpolateLithologyIDW(lat, lon)
}

// GetLithologyAtParallel использует параллельный поиск ближайших соседей с fallback
func (p *LithologyProfile) GetLithologyAtParallel(ctx context.Context, lat, lon float64) LithologyState {
	if lat < p.Metadata.Bounds.MinLat || lat > p.Metadata.Bounds.MaxLat ||
		lon < p.Metadata.Bounds.MinLon || lon > p.Metadata.Bounds.MaxLon {
		return p.getDefaultLithology()
	}

	if len(p.Points) == 0 {
		return p.getDefaultLithology()
	}

	if len(p.Points) == 1 {
		point := &p.Points[0]
		if _, ok := p.Classes[point.Lithology]; !ok {
			return p.getDefaultLithology()
		}
		return LithologyState{
			Class:       point.Lithology,
			Resistance:  point.Resistance,
			Color:       point.Color,
			Description: point.Description,
		}
	}

	const parallelThreshold = 100
	var nearby []*LithologyPoint

	if len(p.Points) >= parallelThreshold {
		nearby = p.findNearbyPointsParallel(ctx, lat, lon, 6)
	} else {
		nearby = p.findNearbyPoints(lat, lon, 6)
	}

	if len(nearby) == 0 {
		return p.getDefaultLithology()
	}

	if len(nearby) == 1 {
		point := nearby[0]
		if _, ok := p.Classes[point.Lithology]; !ok {
			return p.getDefaultLithology()
		}
		return LithologyState{
			Class:       point.Lithology,
			Resistance:  point.Resistance,
			Color:       point.Color,
			Description: point.Description,
		}
	}

	return p.interpolateLithologyIDWFromPoints(lat, lon, nearby)
}

// BatchGetLithologyAt получает литологию для нескольких точек эффективно
func (p *LithologyProfile) BatchGetLithologyAt(ctx context.Context, coords []struct{ Lat, Lon float64 }) []LithologyState {
	if len(coords) == 0 {
		return []LithologyState{}
	}

	results := make([]LithologyState, len(coords))

	numWorkers := lithologyMin(runtime.NumCPU(), 8)
	chunkSize := (len(coords) + numWorkers - 1) / numWorkers

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := lithologyMin(start+chunkSize, len(coords))

		wg.Add(1)
		go func(workerID, s, e int) {
			defer wg.Done()

			for i := s; i < e && i < len(coords); i++ {
				select {
				case <-ctx.Done():
					return
				default:
					results[i] = p.GetLithologyAt(coords[i].Lat, coords[i].Lon)
				}
			}
		}(w, start, end)
	}
	wg.Wait()

	return results
}

// interpolateLithologyIDW интерполирует литологию с использованием обратных расстояний (IDW)
func (p *LithologyProfile) interpolateLithologyIDW(lat, lon float64) LithologyState {
	const maxPoints = 6 // максимальное число точек для интерполяции
	const power = 2.0   // степень для IDW (стандартное значение)

	// Найти ближайшие точки
	nearby := p.findNearbyPoints(lat, lon, maxPoints)
	if len(nearby) == 0 {
		return p.getDefaultLithology()
	}

	// Если только одна точка в радиусе
	if len(nearby) == 1 {
		point := nearby[0]
		if _, ok := p.Classes[point.Lithology]; !ok {
			return p.getDefaultLithology()
		}
		return LithologyState{
			Class:       point.Lithology,
			Resistance:  point.Resistance,
			Color:       point.Color,
			Description: point.Description,
		}
	}

	return p.interpolateLithologyIDWFromPoints(lat, lon, nearby)
}

// interpolateLithologyIDWFromPoints выполняет IDW-интерполяцию из заранее выбранных ближайших точек
func (p *LithologyProfile) interpolateLithologyIDWFromPoints(lat, lon float64, nearby []*LithologyPoint) LithologyState {
	const power = 2.0

	weights := make([]float64, len(nearby))
	weightSum := 0.0

	for i, point := range nearby {
		dist := math.Sqrt(math.Pow(point.Lat-lat, 2) + math.Pow(point.Lon-lon, 2))

		if dist < 1e-6 {
			if _, ok := p.Classes[point.Lithology]; ok {
				return LithologyState{
					Class:       point.Lithology,
					Resistance:  point.Resistance,
					Color:       point.Color,
					Description: point.Description,
				}
			}
			return p.getDefaultLithology()
		}

		weights[i] = 1.0 / math.Pow(dist, power)
		weightSum += weights[i]
	}

	for i := range weights {
		weights[i] /= weightSum
	}

	interpolatedResistance := 0.0
	for i, point := range nearby {
		interpolatedResistance += weights[i] * point.Resistance
	}

	maxWeightIdx := 0
	for i := range weights {
		if weights[i] > weights[maxWeightIdx] {
			maxWeightIdx = i
		}
	}

	dominantPoint := nearby[maxWeightIdx]

	if _, ok := p.Classes[dominantPoint.Lithology]; !ok {
		return p.getDefaultLithology()
	}

	return LithologyState{
		Class:       dominantPoint.Lithology,
		Resistance:  interpolatedResistance,
		Color:       dominantPoint.Color,
		Description: dominantPoint.Description,
	}
}

// ============================================================
// ПОИСК БЛИЖАЙШИХ ТОЧЕК
// ============================================================

// findNearbyPoints находит N ближайших литологических точек к заданным координатам
func (p *LithologyProfile) findNearbyPoints(lat, lon float64, n int) []*LithologyPoint {
	if len(p.Points) == 0 {
		return nil
	}

	// Ограничиваем n числом доступных точек
	if n > len(p.Points) {
		n = len(p.Points)
	}

	pointDists := make([]pointDist, len(p.Points))
	for i := range p.Points {
		dist := math.Sqrt(
			math.Pow(p.Points[i].Lat-lat, 2) + math.Pow(p.Points[i].Lon-lon, 2),
		)
		pointDists[i] = pointDist{point: &p.Points[i], dist: dist}
	}

	// Полная сортировка по расстоянию
	for i := 0; i < len(pointDists)-1; i++ {
		minIdx := i
		for j := i + 1; j < len(pointDists); j++ {
			if pointDists[j].dist < pointDists[minIdx].dist {
				minIdx = j
			}
		}
		pointDists[i], pointDists[minIdx] = pointDists[minIdx], pointDists[i]
	}

	// Вернуть первые n точек
	result := make([]*LithologyPoint, n)
	for i := 0; i < n; i++ {
		result[i] = pointDists[i].point
	}

	return result
}

// findNearbyPointsVectorized использует SIMD-дружественную последовательную обработку для улучшения локализации кэша
func (p *LithologyProfile) findNearbyPointsVectorized(lat, lon float64, n int) []*LithologyPoint {
	if len(p.Points) == 0 {
		return nil
	}

	if n > len(p.Points) {
		n = len(p.Points)
	}

	pointDists := make([]pointDist, len(p.Points))

	// Последовательные вычисления с лучшей локализацией кэша
	for i := range p.Points {
		dLat := p.Points[i].Lat - lat
		dLon := p.Points[i].Lon - lon
		dist := dLat*dLat + dLon*dLon
		pointDists[i] = pointDist{
			point: &p.Points[i],
			dist:  dist, // Храним квадрат расстояния
		}
	}

	// Частичная сортировка
	nth := lithologyMin(n, len(pointDists))
	partialSort(pointDists, nth)

	result := make([]*LithologyPoint, nth)
	for i := 0; i < nth; i++ {
		result[i] = pointDists[i].point
	}

	return result
}

// findNearbyPointsParallel находит N ближайших литологических точек с использованием параллельных вычислений расстояний
func (p *LithologyProfile) findNearbyPointsParallel(ctx context.Context, lat, lon float64, n int) []*LithologyPoint {
	if len(p.Points) == 0 {
		return nil
	}

	if n > len(p.Points) {
		n = len(p.Points)
	}

	numWorkers := lithologyMin(runtime.NumCPU(), 8)
	chunkSize := (len(p.Points) + numWorkers - 1) / numWorkers

	type chunkResult struct {
		dists []pointDist
	}

	results := make(chan chunkResult, numWorkers)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := start + chunkSize
		if end > len(p.Points) {
			end = len(p.Points)
		}

		wg.Add(1)
		go func(startIdx, endIdx int) {
			defer wg.Done()

			if startIdx >= len(p.Points) {
				results <- chunkResult{}
				return
			}

			localDists := make([]pointDist, 0, endIdx-startIdx)

			for i := startIdx; i < endIdx && i < len(p.Points); i++ {
				select {
				case <-ctx.Done():
					results <- chunkResult{}
					return
				default:
				}

				dist := math.Sqrt(
					math.Pow(p.Points[i].Lat-lat, 2) + math.Pow(p.Points[i].Lon-lon, 2),
				)
				localDists = append(localDists, pointDist{
					point: &p.Points[i],
					dist:  dist,
				})
			}

			results <- chunkResult{dists: localDists}
		}(start, end)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	allDists := make([]pointDist, 0, len(p.Points))
	for res := range results {
		if ctx.Err() != nil {
			return nil
		}
		allDists = append(allDists, res.dists...)
	}

	if len(allDists) == 0 {
		return nil
	}

	// Частичная сортировка
	nth := n
	if nth > len(allDists) {
		nth = len(allDists)
	}
	quickselectLithology(allDists, nth)

	result := make([]*LithologyPoint, nth)
	for i := 0; i < nth; i++ {
		result[i] = allDists[i].point
	}

	return result
}

// findClosestPoint находит ближайшую литологическую точку к заданным координатам
func (p *LithologyProfile) findClosestPoint(lat, lon float64) *LithologyPoint {
	nearby := p.findNearbyPoints(lat, lon, 1)
	if len(nearby) == 0 {
		return nil
	}
	return nearby[0]
}

// ============================================================
// АЛГОРИТМЫ СОРТИРОВКИ
// ============================================================

// quickselectLithology выполняет частичную сортировку для нахождения n наименьших элементов
func quickselectLithology(arr []pointDist, n int) {
	if len(arr) <= 1 || n <= 0 {
		return
	}

	left, right := 0, len(arr)-1
	pivotIndex := partitionLithology(arr, left, right)

	if pivotIndex == n-1 {
		return
	} else if pivotIndex > n-1 {
		quickselectLithology(arr[:pivotIndex], n)
	} else {
		quickselectLithology(arr[pivotIndex+1:], n-pivotIndex-1)
	}
}

// partitionLithology разбивает массив вокруг опорного элемента и возвращает финальный индекс опорного элемента
func partitionLithology(arr []pointDist, left, right int) int {
	pivot := arr[right]
	i := left

	for j := left; j < right; j++ {
		if arr[j].dist <= pivot.dist {
			arr[i], arr[j] = arr[j], arr[i]
			i++
		}
	}

	arr[i], arr[right] = arr[right], arr[i]
	return i
}

// partialSort выполняет частичную сортировку на основе кучи
func partialSort(arr []pointDist, n int) {
	if n <= 0 || len(arr) == 0 {
		return
	}

	if n > len(arr) {
		n = len(arr)
	}

	// Строим max-кучу из первых n элементов
	heap := arr[:n]
	for i := n/2 - 1; i >= 0; i-- {
		heapify(heap, i, n-1, true)
	}

	// Для оставшихся элементов
	for i := n; i < len(arr); i++ {
		if arr[i].dist < heap[0].dist {
			heap[0] = arr[i]
			heapify(heap, 0, n-1, true)
		}
	}

	// Сортируем кучу
	for size := n - 1; size > 0; size-- {
		heap[0], heap[size] = heap[size], heap[0]
		heapify(heap[:size], 0, size-1, false)
	}
}

// heapify поддерживает свойство кучи
func heapify(arr []pointDist, root, last int, isMaxHeap bool) {
	for {
		left := 2*root + 1
		right := 2*root + 2
		candidate := root

		if isMaxHeap {
			// Max-куча: родитель больше детей
			if left <= last && arr[left].dist > arr[candidate].dist {
				candidate = left
			}
			if right <= last && arr[right].dist > arr[candidate].dist {
				candidate = right
			}
		} else {
			// Min-куча: родитель меньше детей
			if left <= last && arr[left].dist < arr[candidate].dist {
				candidate = left
			}
			if right <= last && arr[right].dist < arr[candidate].dist {
				candidate = right
			}
		}

		if candidate == root {
			break
		}

		arr[root], arr[candidate] = arr[candidate], arr[root]
		root = candidate
	}
}

// lithologyMin функция минимума для целых чисел
func lithologyMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================
// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ
// ============================================================

// getDefaultLithology возвращает состояние литологии по умолчанию
func (p *LithologyProfile) getDefaultLithology() LithologyState {
	defaultClass := p.getDefaultClass()
	return LithologyState{
		Class:       "limestone",
		Resistance:  defaultClass.Resistance,
		Color:       defaultClass.Color,
		Description: "литология по умолчанию",
	}
}

// getDefaultClass возвращает класс литологии по умолчанию
func (p *LithologyProfile) getDefaultClass() LithologyClass {
	// Попробовать найти limestone, иначе fallback
	if class, ok := p.Classes["limestone"]; ok {
		return class
	}

	// Fallback к разумным значениям по умолчанию
	return LithologyClass{
		Resistance:  2.5,
		Color:       "#8b8b8b",
		Description: "резервная литология",
	}
}

// ============================================================
// СТАТИСТИКА И СОЗДАНИЕ ПРОФИЛЕЙ
// ============================================================

// GetStatistics возвращает статистику о литологическом профиле
func (p *LithologyProfile) GetStatistics() map[string]interface{} {
	stats := map[string]interface{}{
		"name":          p.Metadata.Name,
		"version":       p.Metadata.Version,
		"num_points":    len(p.Points),
		"num_classes":   len(p.Classes),
		"num_baselines": len(p.Baselines),
		"resolution":    p.Metadata.Resolution,
		"bounds":        p.Metadata.Bounds,
		"regions":       p.Metadata.Regions,
	}

	// Статистика сопротивления
	if len(p.Points) > 0 {
		minR := p.Points[0].Resistance
		maxR := p.Points[0].Resistance
		sumR := 0.0

		for _, point := range p.Points {
			if point.Resistance < minR {
				minR = point.Resistance
			}
			if point.Resistance > maxR {
				maxR = point.Resistance
			}
			sumR += point.Resistance
		}

		stats["resistance_min"] = minR
		stats["resistance_max"] = maxR
		stats["resistance_mean"] = sumR / float64(len(p.Points))
	}

	// Распределение уверенности
	confidenceDist := make(map[string]int)
	for _, point := range p.Points {
		confidenceDist[point.Confidence]++
	}
	stats["confidence_distribution"] = confidenceDist

	return stats
}

// GetLithologyStatistics возвращает статистику по динамической литологии
func GetLithologyStatistics(states []DynamicLithologyState) map[string]interface{} {
	stats := map[string]interface{}{
		"total_points": len(states),
	}

	if len(states) == 0 {
		return stats
	}

	// Статистика сопротивления
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

// CreateDefaultBlackSeaProfile создаёт профиль литологии Чёрного моря по умолчанию
func CreateDefaultBlackSeaProfile() *LithologyProfile {
	return &LithologyProfile{
		Metadata: LithologyMetadata{
			Name:       "Литология Чёрного моря по умолчанию",
			Version:    "1.0-fallback",
			Created:    "автоматически сгенерировано",
			Sources:    []string{"резервный"},
			Resolution: 1.0,
			Bounds: Bounds{
				MinLat: 40.0,
				MaxLat: 47.0,
				MinLon: 27.0,
				MaxLon: 42.0,
			},
			Regions: []string{"crimea", "turkey", "bulgaria", "romania", "georgia", "russia"},
			Note:    "Резервный профиль при отсутствии данных литологии",
		},
		Points: []LithologyPoint{
			{Lat: 45.0, Lon: 34.5, Region: "crimea", Lithology: "limestone", Resistance: 4.0, Color: "#6b6b6b", Description: "Крымский известняк", Confidence: "low"},
			{Lat: 41.5, Lon: 40.0, Region: "turkey", Lithology: "volcanic", Resistance: 6.5, Color: "#4a4a4a", Description: "Понтийские вулканические породы", Confidence: "low"},
			{Lat: 42.5, Lon: 28.0, Region: "bulgaria", Lithology: "limestone", Resistance: 4.0, Color: "#6b6b6b", Description: "Болгарский известняк", Confidence: "low"},
			{Lat: 44.5, Lon: 29.0, Region: "romania", Lithology: "clay", Resistance: 1.2, Color: "#c4a484", Description: "Румынская глина", Confidence: "low"},
			{Lat: 42.5, Lon: 41.5, Region: "georgia", Lithology: "sedimentary", Resistance: 2.5, Color: "#8b8b8b", Description: "Осадочные породы Кавказа", Confidence: "low"},
		},
		Classes: map[string]LithologyClass{
			"limestone": {
				Resistance:  4.0,
				Color:       "#6b6b6b",
				Description: "Сарматский/неогеновый известняк",
			},
			"volcanic": {
				Resistance:  6.5,
				Color:       "#4a4a4a",
				Description: "Вулканические породы",
			},
			"clay": {
				Resistance:  1.2,
				Color:       "#c4a484",
				Description: "Глинистые образования",
			},
			"sedimentary": {
				Resistance:  2.5,
				Color:       "#8b8b8b",
				Description: "Осадочные породы",
			},
		},
		Baselines: map[string]ErosionBaseline{
			"very_soft": {
				ResistanceRange: [2]float64{0.8, 1.4},
				ErosionMYear: map[string]float64{
					"min": 5.0, "max": 12.0, "mean": 7.5,
				},
				Description: "Мягкие осадки — очень быстрая эрозия",
			},
			"soft": {
				ResistanceRange: [2]float64{1.5, 2.4},
				ErosionMYear: map[string]float64{
					"min": 2.0, "max": 5.0, "mean": 3.5,
				},
				Description: "Сцементированные осадки — быстрая эрозия",
			},
			"medium": {
				ResistanceRange: [2]float64{2.5, 3.9},
				ErosionMYear: map[string]float64{
					"min": 1.0, "max": 3.0, "mean": 2.0,
				},
				Description: "Песчаник, конгломерат — значительная эрозия",
			},
			"medium_hard": {
				ResistanceRange: [2]float64{4.0, 5.9},
				ErosionMYear: map[string]float64{
					"min": 0.5, "max": 2.0, "mean": 1.2,
				},
				Description: "Известняк, метаморфические — заметная эрозия",
			},
			"hard": {
				ResistanceRange: [2]float64{6.0, 7.9},
				ErosionMYear: map[string]float64{
					"min": 0.3, "max": 1.0, "mean": 0.6,
				},
				Description: "Вулканические породы — умеренная эрозия",
			},
			"very_hard": {
				ResistanceRange: [2]float64{8.0, 10.0},
				ErosionMYear: map[string]float64{
					"min": 0.1, "max": 0.5, "mean": 0.3,
				},
				Description: "Серпентинит, гранит — очень медленная эрозия",
			},
		},
	}
}

// CreateDefaultWeatheringProfile создаёт профиль выветривания по умолчанию
func CreateDefaultWeatheringProfile() WeatheringProfile {
	return WeatheringProfile{
		BaseRate: 0.1,
		WeatheringRates: map[string]float64{
			"limestone":    0.2,
			"granite":      0.05,
			"sandstone":    0.3,
			"shale":        0.4,
			"basalt":       0.03,
			"conglomerate": 0.2,
			"alluvium":     1.0,
			"rock":         0.1,
		},
		ClimateMultiplier: 1.0,
		WeatheringDepth:   2.0,
		StabilizationTime: 50.0,
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
		HeterogeneityScale:          5.0,
		NoiseLevel:                  0.2,
	}
}

// Validate валидирует параметры взаимодействия литологии
func (p *LithologyInteractionParams) Validate() error {
	if p.ErosionResistanceFactor < 0 || p.ErosionResistanceFactor > 1 {
		return fmt.Errorf("некорректный коэффициент сопротивления эрозии: %.2f", p.ErosionResistanceFactor)
	}
	if p.WeatheringErosionBoost < 0 || p.WeatheringErosionBoost > 1 {
		return fmt.Errorf("некорректный множитель ускорения эрозии при выветривании: %.2f", p.WeatheringErosionBoost)
	}
	if p.StormFractureMultiplier < 1 || p.StormFractureMultiplier > 10 {
		return fmt.Errorf("некорректный множитель трещинообразования при шторме: %.2f", p.StormFractureMultiplier)
	}
	if p.SpatialAutocorrelation < 0 || p.SpatialAutocorrelation > 1 {
		return fmt.Errorf("некорректная пространственная автокорреляция: %.2f", p.SpatialAutocorrelation)
	}
	return nil
}

// ============================================================
// ДИНАМИЧЕСКАЯ ЛИТОЛОГИЯ - ВЫВЕТРИВАНИЕ
// ============================================================

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
	dynamic.WeatheringProgress = 1.0 - math.Exp(-totalRate*years/weathering.StabilizationTime)

	// Снижение сопротивления
	resistanceLoss := state.Resistance * dynamic.WeatheringProgress * 0.7
	dynamic.CurrentResistance = state.Resistance - resistanceLoss

	if dynamic.CurrentResistance < 0.1 {
		dynamic.CurrentResistance = 0.1
	}

	// Увеличение пористости и трещиноватости
	dynamic.Porosity = 0.05 + dynamic.WeatheringProgress*0.25
	dynamic.FractureDensity = 0.1 + dynamic.WeatheringProgress*0.4

	dynamic.IsWeathered = dynamic.WeatheringProgress > 0.3

	// Запись в историю
	dynamic.ModificationHistory = append(dynamic.ModificationHistory, LithologyModification{
		Timestamp:        time.Now(),
		OldResistance:    state.Resistance,
		NewResistance:    dynamic.CurrentResistance,
		ModificationType: "weathering",
		Cause:            fmt.Sprintf("выветривание за %.1f лет", years),
		Magnitude:        state.Resistance - dynamic.CurrentResistance,
	})

	return dynamic
}

// ============================================================
// ДИНАМИЧЕСКАЯ ЛИТОЛОГИЯ - ВЗАИМОДЕЙСТВИЕ С ЭРОЗИЕЙ
// ============================================================

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
		resistanceFactor = 1.0
	}
	modified *= resistanceFactor

	// 2. Выветривание увеличивает эрозию
	if litho.IsWeathered {
		modified *= (1.0 + params.WeatheringErosionBoost*litho.WeatheringProgress)
	}

	// 3. Штормовая эрозия
	if isStorm {
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

	// 2. Эффективность захвата наносов
	trappingEfficiency := params.LithologyTrappingEfficiency

	// 3. Выветрелые породы лучше захватывают наносы
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
			Cause:            fmt.Sprintf("эрозия %.2fм", erosionMeters),
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
			Cause:            fmt.Sprintf("аккумуляция %.2fм", depositionMeters),
			Magnitude:        depositionMeters,
		})
	}

	return updated
}

// ============================================================
// ДИНАМИЧЕСКАЯ ЛИТОЛОГИЯ - ШТОРМЫ
// ============================================================

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
		Cause:            fmt.Sprintf("интенсивность шторма %.2f", stormIntensity),
		Magnitude:        resistanceDecrease,
	})

	return state
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

// ============================================================
// ДИНАМИЧЕСКАЯ ЛИТОЛОГИЯ - СИМУЛЯЦИЯ
// ============================================================

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

// ============================================================
// ДИНАМИЧЕСКАЯ ЛИТОЛОГИЯ - ПРОСТРАНСТВЕННАЯ ВАРИАБЕЛЬНОСТЬ
// ============================================================

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
		spatialNoise := generateSpatiallyCorrelatedNoise(
			i, n, params.SpatialAutocorrelation, params.NoiseLevel,
		)

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
	if n <= 1 {
		return 0.0
	}

	normalizedPos := float64(index) / float64(n)

	// Детерминированная компонента
	spatialComponent := 0.5*math.Sin(2*math.Pi*normalizedPos*3) +
		0.3*math.Sin(2*math.Pi*normalizedPos*7)

	// Случайная компонента
	hashLike := float64((index*2654435761)%1000000007) / 1000000007.0
	randomComponent := (hashLike - 0.5) * 2.0

	// Комбинируем
	result := (spatialComponent*(1-autocorr) + randomComponent*autocorr) * noiseLevel

	// Ограничиваем диапазон
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
	erosionHistory []float64,
	stormEvents []int,
) LithologyEvolutionResult {

	n := len(initialStates)
	dynamicStates := make([]DynamicLithologyState, n)

	climateFactor := 1.0

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

			for _, stormIdx := range stormEvents {
				if i == stormIdx {
					isStorm = true
					break
				}
			}

			erosionImpact := CalculateLithologyErosionInteraction(baseErosion, state, params, isStorm)
			result.ErosionImpact[i] = erosionImpact

			if baseErosion > 0 {
				erosionFactor := baseErosion / 1000.0
				if erosionFactor > 0.1 {
					erosionFactor = 0.1
				}

				state.ModificationHistory = append(state.ModificationHistory, LithologyModification{
					Timestamp:        time.Now(),
					OldResistance:    state.CurrentResistance,
					NewResistance:    state.CurrentResistance * (1.0 - erosionFactor),
					ModificationType: "erosion",
					Cause:            fmt.Sprintf("воздействие эрозии %.2f", baseErosion),
					Magnitude:        state.CurrentResistance * erosionFactor,
				})

				state.CurrentResistance *= (1.0 - erosionFactor)
				state.FractureDensity += erosionFactor * 0.5
			}
		}
	}

	// Рассчитываем влияние на аккумуляцию
	for i, state := range dynamicStates {
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
