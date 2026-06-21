# Базовый анализ

Базовый анализ представляет собой комплексную систему проверки и измерения характеристик береговой линии, включающую три основных компонента: **валидацию геометрии**, **геодезический расчёт длины** и **метрики качества**. Эта система обеспечивает надёжность входных данных и их соответствие физическим и топологическим ограничениям.

---

## Содержание

- [Обзор архитектуры](#обзор-архитектуры)
- [Валидация геометрии](#валидация-геометрии)
  - [Этапы валидации](#этапы-валидации)
  - [Удаление дубликатов](#удаление-дубликатов)
  - [Автоматическое переупорядочивание](#автоматическое-переупорядочивание)
  - [Обнаружение самопересечений](#обнаружение-самопересечений)
  - [Проверки здравого смысла](#проверки-здравого-смысла)
- [Геодезический расчёт длины](#геодезический-расчёт-длины)
  - [Формула Haversine](#формула-haversine)
  - [Вычисление длины полилинии](#вычисление-длины-полилинии)
  - [Точность и ограничения](#точность-и-ограничения)
- [Метрики качества](#метрики-качества)
  - [Предупреждения о длинных сегментах](#предупреждения-о-длинных-сегментах)
  - [Обнаружение повторяющихся локаций](#обнаружение-повторяющихся-локаций)
  - [Отчёт о валидации](#отчёт-о-валидации)
  - [Суммарная статистика](#суммарная-статистика)
- [Интеграция с рабочим процессом](#интеграция-с-рабочим-процессом)

---

## Обзор архитектуры

```
internal/domain/coastline/
├── validation.go           # Основная логика валидации
├── validation_summary.go   # Суммарная статистика
├── sanity.go              # Проверки здравого смысла
├── locations.go           # Географические ориентиры
└── metrics.go             # Основные расчёты и вывод

internal/domain/geometry/
├── haversine.go           # Геодезические расстояния
├── length.go              # Длина полилиний
└── types.go               # Базовые типы данных
```

**Основные типы данных:**

```go
type LatLon struct {
    Lat float64 // Широта [-90, 90]
    Lon float64 // Долгота [-180, 180]
}

type ValidationReport struct {
    Fixes    []string // Автоматические исправления
    Warnings []string // Предупреждающие сообщения
}

type ValidationSummary struct {
    Issues             []ValidationIssueSummary
    DuplicateLocations []DuplicateLocationSummary
}
```

---

## Валидация геометрии

Валидация геометрии обеспечивает топологическую корректность береговой линии и автоматическое исправление обнаруженных проблем. Процесс валидации применяется к каждой загруженной береговой линии перед дальнейшим анализом.

### Этапы валидации

Процесс валидации состоит из пяти последовательных этапов:

1. **Удаление дубликатов** — исключение повторяющихся координат
2. **Автоматическое переупорядочивание** — оптимизация порядка обхода точек
3. **Обнаружение самопересечений** — проверка топологической корректности
4. **Сбор предупреждений** — идентификация потенциальных проблем
5. **Проверки здравого смысла** — сравнение с референсными значениями

### Удаление дубликатов

**Проблема:** Входные данные могут содержать дубликаты координат из-за ошибок оцифровки, объединения данных из разных источников или других артефактов.

**Решение:** Модуль автоматически идентифицирует и удаляет дубликаты с использованием хеш-таблицы:

```go
func removeDuplicateCoordinates(points []LatLon) ([]LatLon, int) {
    seen := make(map[string]struct{})
    result := make([]LatLon, 0)
    removed := 0
    
    for _, point := range points {
        key := fmt.Sprintf("%.6f|%.6f", point.Lat, point.Lon)
        if _, exists := seen[key]; exists {
            removed++
            continue
        }
        seen[key] = struct{}{}
        result = append(result, point)
    }
    
    return result, removed
}
```

**Ключевая точка:** Координаты округляются до 6 знаков после запятой (точность ~0.11 м) для идентификации дубликатов.

**Пример:**

```go
// Исходные данные (с дубликатами)
[
  {"lat": 46.48, "lon": 30.73},  // Одесса
  {"lat": 46.48, "lon": 30.73},  // Дубликат
  {"lat": 45.33, "lon": 32.49},  // Евпатория
]

// После удаления дубликатов
[
  {"lat": 46.48, "lon": 30.73},
  {"lat": 45.33, "lon": 32.49}
]

// Отчёт: "удалены повторяющиеся координаты: 1"
```

### Автоматическое переупорядочивание

**Проблема:** Порядок точек в исходных данных может не соответствовать естественному обходу контура, что приводит к большим расстояниям между соседними точками и самопересечениям.

**Решение:** Модуль генерирует несколько кандидатов упорядочивания и выбирает лучший по комплексному критерию качества:

```go
func chooseBestOrder(points []LatLon) []LatLon {
    candidates := [][]LatLon{
        slices.Clone(points),           // Исходный порядок
        reversePoints(points),          // Обратный порядок
    }
    
    // Жадный обход от разных начальных точек
    for _, start := range candidateStartIndices(points) {
        candidate := greedyTraversal(points, start)
        candidates = append(candidates, candidate)
        candidates = append(candidates, reversePoints(candidate))
    }
    
    // Выбор лучшего кандидата
    best := candidates[0]
    bestScore := scoreOrder(best)
    for _, candidate := range candidates[1:] {
        score := scoreOrder(candidate)
        if score.less(bestScore) {
            best = candidate
            bestScore = score
        }
    }
    
    return best
}
```

**Критерий качества (orderScore):**

```go
type orderScore struct {
    intersections int     // Число самопересечений
    longSegments  int     // Число длинных сегментов
    maxSegmentKM  float64 // Максимальная длина сегмента
    totalLengthKM float64 // Общая длина
}
```

**Правило сравнения:** Меньше пересечений → лучше; при равенстве — меньше длинных сегментов; при равенстве — меньше максимальная длина.

**Жадный обход (greedy traversal):**

```go
func greedyTraversal(points []LatLon, start int) []LatLon {
    used := make([]bool, len(points))
    result := make([]LatLon, 0)
    current := start
    
    for len(result) < len(points) {
        result = append(result, points[current])
        used[current] = true
        
        // Найти ближайшую неиспользованную точку
        next := -1
        bestDistance := math.MaxFloat64
        for i := range points {
            if used[i] {
                continue
            }
            distance := Haversine(points[current], points[i])
            if distance < bestDistance {
                bestDistance = distance
                next = i
            }
        }
        
        if next == -1 {
            break
        }
        current = next
    }
    
    return result
}
```

**Пример:**

```go
// Исходный порядок (плохой)
[A → C → B → D]  // Пропуски и большие расстояния

// После жадного обхода
[A → B → C → D]  // Естественный порядок

// Отчёт: "точки автоматически переупорядочены по обходу контура"
```

### Обнаружение самопересечений

**Критическая проверка:** Самопересечения делают полилинию топологически некорректной и приводят к ошибкам в дальнейших расчётах.

**Алгоритм:** Перебор всех пар несмежных сегментов с проверкой пересечения:

```go
func findSelfIntersections(points []LatLon) []segmentIntersection {
    var intersections []segmentIntersection
    
    for i := 0; i < len(points)-1; i++ {
        for j := i + 2; j < len(points)-1; j++ {
            if segmentsIntersect(points[i], points[i+1], points[j], points[j+1]) {
                intersections = append(intersections, segmentIntersection{
                    First:  i + 1,
                    Second: j + 1,
                })
            }
        }
    }
    
    return intersections
}
```

**Проверка пересечения сегментов (orientation test):**

```go
func segmentsIntersect(a, b, c, d LatLon) bool {
    // Ориентация троек точек
    o1 := orientation(a, b, c)
    o2 := orientation(a, b, d)
    o3 := orientation(c, d, a)
    o4 := orientation(c, d, b)
    
    // Общее пересечение (разные стороны)
    if o1*o2 < 0 && o3*o4 < 0 {
        return true
    }
    
    // Коллинеарные случаи (точка на сегменте)
    if math.Abs(o1) <= eps && onSegment(a, c, b) {
        return true
    }
    if math.Abs(o2) <= eps && onSegment(a, d, b) {
        return true
    }
    if math.Abs(o3) <= eps && onSegment(c, a, d) {
        return true
    }
    if math.Abs(o4) <= eps && onSegment(c, b, d) {
        return true
    }
    
    return false
}
```

**Результат:** Если обнаружены пересечения, валидация возвращает ошибку с указанием номеров сегментов.

### Проверки здравого смысла

**Санити-чек (sanity check):** Сравнение рассчитанной длины с референсными значениями для известных наборов данных.

```go
var knownCoastlineEstimates = map[string]coastlineEstimate{
    "black-sea.json": {
        MinKM: 4000,
        MaxKM: 4987,
    },
}

func SanityCheck(dataset string, lengthKM float64) SanityCheckResult {
    estimate, ok := knownCoastlineEstimates[dataset]
    if !ok {
        return SanityCheckResult{Checked: false}
    }
    
    minAllowed := estimate.MinKM * (1 - sanityTolerance)
    maxAllowed := estimate.MaxKM * (1 + sanityTolerance)
    
    if lengthKM >= minAllowed && lengthKM <= maxAllowed {
        return SanityCheckResult{
            Checked: true,
            Valid:   true,
        }
    }
    
    return SanityCheckResult{
        Checked: true,
        Valid:   false,
        Warning: "WARNING: coastline length likely incorrect...",
    }
}
```

**Допуск:** `sanityTolerance = 0.40` (40% отклонение от референса).

**Пример:**

```
Рассчитанная длина: 3000 км
Референсный диапазон: 4000-4987 км
Результат: WARNING - возможные проблемы с порядком точек
```

---

## Геодезический расчёт длины

Геодезический расчёт длины береговой линии основан на формуле Haversine для вычисления расстояний на сфере с учётом кривизны Земли.

### Формула Haversine

**Назначение:** Вычисление расстояния между двумя точками на сфере по их географическим координатам.

```go
func Haversine(a, b LatLon) float64 {
    dLat := (b.Lat - a.Lat) * math.Pi / 180
    dLon := (b.Lon - a.Lon) * math.Pi / 180
    lat1 := a.Lat * math.Pi / 180
    lat2 := b.Lat * math.Pi / 180
    
    h := math.Sin(dLat/2)*math.Sin(dLat/2) +
         math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1)*math.Cos(lat2)
    
    c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
    
    return EarthRadiusKM * c
}
```

**Где:**
- `EarthRadiusKM = 6371.0` — средний радиус Земли в километрах
- `h` — гаверсинус центрального угла
- `c` — центральный угол в радианах

**Математическая форма:**

```
distance = R × 2 × atan2(√h, √(1-h))

h = sin²(Δlat/2) + sin²(Δlon/2) × cos(lat1) × cos(lat2)

Δlat = (lat₂ - lat₁) × π/180
Δlon = (lon₂ - lon₁) × π/180
```

### Вычисление длины полилинии

**Функция `PolylineLength`:** Сумма гаверсинусных расстояний между последовательными точками:

```go
func PolylineLength(points []LatLon) float64 {
    if len(points) < 2 {
        return 0
    }
    
    var total float64
    for i := 1; i < len(points); i++ {
        total += Haversine(points[i-1], points[i])
    }
    
    return total
}
```

**Пример расчёта:**

```go
// Береговая линия Чёрного моря (упрощённая)
coast := []LatLon{
    {Lat: 46.48, Lon: 30.73}, // Одесса
    {Lat: 45.33, Lon: 32.49}, // Евпатория
    {Lat: 44.62, Lon: 33.53}, // Севастополь
    {Lat: 43.70, Lon: 39.75}, // Сочи
    {Lat: 41.65, Lon: 41.63}, // Батуми
}

length := PolylineLength(coast)
// Результат: ~1100 км
```

### Точность и ограничения

| Фактор | Влияние | Величина ошибки |
|--------|---------|-----------------|
| Сферическая модель | Земля — эллипсоид | ~0.3% |
| Средний радиус | Полюса/экватор | ~0.5% |
| Малые расстояния (< 1 км) | Хорошая точность | <0.1% |
| Средние расстояния (100-1000 км) | Приемлемая точность | ~0.5-1% |
| Большие расстояния (> 5000 км) | Погрешность возрастает | ~1% |

**Для Чёрного моря:** Максимальные расстояния ~1000 км, точность ~0.5-1% — вполне достаточно для задач проекта.

**Преимущества подхода:**
- ✅ Учёт кривизны Земли (в отличие от евклидовых расстояний)
- ✅ Простота и вычислительная эффективность
- ✅ Адекватная точность для региональных масштабов

---

## Метрики качества

Метрики качества предоставляют детальную диагностику состояния геометрии и выявляют потенциальные проблемы, которые могут влиять на точность дальнейшего анализа.

### Предупреждения о длинных сегментах

**Проблема:** Слишком длинные сегменты указывают на недостаточную детализацию береговой линии, пропуск важных изгибов или ошибки в данных.

**Порог:** `longSegmentWarningKM = 450.0` км

**Алгоритм обнаружения:**

```go
func longSegmentWarnings(points []LatLon, thresholdKM float64) []string {
    var warnings []string
    
    for i := 1; i < len(points); i++ {
        length := Haversine(points[i-1], points[i])
        if length > thresholdKM {
            warnings = append(warnings, 
                fmt.Sprintf("сегмент %d-%d имеет длину %.0f км, это больше порога %.0f км", 
                    i, i+1, length, thresholdKM))
        }
    }
    
    return warnings
}
```

**Пример предупреждения:**

```
сегмент 45-46 имеет длину 512 км, это больше порога 450 км
```

**Интерпретация:**
- Длинные сегменты могут указывать на пропуск промежуточных точек
- Могут возникать при неправильном порядке обхода
- Требуют проверки исходных данных

### Обнаружение повторяющихся локаций

**Проблема:** Множественные точки в одном географическом районе могут указывать на избыточную детализацию или ошибки оцифровки.

**Алгоритм:** Сопоставление точек с географическими ориентирами:

```go
func duplicateLocationWarnings(points []LatLon) []string {
    if len(points) > 200 {
        return nil // Слишком много точек для анализа
    }
    
    counts := map[string]int{}
    for _, point := range points {
        name := getLocationName(point)
        if name != "—" {
            counts[name]++
        }
    }
    
    var warnings []string
    for name, count := range counts {
        if count > 1 {
            warnings = append(warnings, 
                fmt.Sprintf("обнаружен повторяющийся ориентир %q: %d точек", name, count))
        }
    }
    
    return warnings
}
```

**Географические ориентири (Чёрное море):**

```go
locations := []location{
    {lat: 46.48, lon: 30.73, name: "Одесса, Украина"},
    {lat: 45.33, lon: 32.49, name: "Евпатория, Крым"},
    {lat: 44.62, lon: 33.53, name: "Севастополь, Крым"},
    {lat: 43.70, lon: 39.75, name: "Сочи, Россия"},
    {lat: 41.65, lon: 41.63, name: "Батуми, Грузия"},
    // ... и другие
}
```

**Порог:** 0.15 градусов (~16.5 км) для отнесения точки к ориентиру.

**Пример предупреждения:**

```
обнаружен повторяющийся ориентир "Сочи, Россия": 5 точек
```

### Отчёт о валидации

**Структура отчёта:**

```go
type ValidationReport struct {
    Fixes    []string // Автоматические исправления
    Warnings []string // Предупреждающие сообщения
}
```

**Пример отчёта:**

```go
ValidationReport{
    Fixes: [
        "удалены повторяющиеся координаты: 3",
        "точки автоматически переупорядочены по обходу контура"
    ],
    Warnings: [
        "сегмент 45-46 имеет длину 512 км, это больше порога 450 км",
        "обнаружен повторяющийся ориентир \"Сочи, Россия\": 5 точек"
    ]
}
```

### Суммарная статистика

**Назначение:** Агрегация проблем по категориям для экспорта и анализа.

```go
type ValidationSummary struct {
    Issues             []ValidationIssueSummary
    DuplicateLocations []DuplicateLocationSummary
}

type ValidationIssueSummary struct {
    WarningType string
    Count       int
    ThresholdKM float64
}
```

**Пример:**

```json
{
  "issues": [
    {
      "warning_type": "long_segment",
      "count": 2,
      "threshold_km": 450.0
    },
    {
      "warning_type": "duplicate_location",
      "count": 3
    }
  ],
  "duplicate_locations": [
    {
      "name": "Сочи, Россия",
      "count": 5
    },
    {
      "name": "Батуми, Грузия",
      "count": 3
    }
  ]
}
```

---

## Интеграция с рабочим процессом

Базовый анализ автоматически применяется при загрузке береговой линии:

```go
// Внутри процесса загрузки
func LoadCoastline(data []byte) ([]LatLon, ValidationReport, error) {
    // 1. Парсинг JSON
    points := parseJSON(data)
    
    // 2. Валидация и нормализация
    validated, report, err := validateAndNormalizePoints(points)
    if err != nil {
        return nil, report, err
    }
    
    // 3. Расчёт длины
    length := PolylineLength(validated)
    
    // 4. Санити-чек
    sanity := SanityCheck(datasetName, length)
    
    return validated, report, nil
}
```

**Результаты анализа:**
- ✅ Валидная геометрия без топологических ошибок
- ✅ Оптимальный порядок обхода точек
- ✅ Корректный учёт кривизны Земли
- ✅ Детальная диагностика потенциальных проблем
- ✅ Сравнение с референсными значениями

**Экспорт метрик:** Все результаты валидации экспортируются в JSON файлы вместе с результатами анализа для дальнейшей обработки и визуализации.