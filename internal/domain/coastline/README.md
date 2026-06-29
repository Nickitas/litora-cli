# Package `coastline`

**Domain-модуль для загрузки, валидации, геометрического анализа и визуализации береговых линий.**

Модуль отвечает за полный цикл работы с береговой линией: от получения сырых данных (локальный JSON, удалённый GeoJSON, кэш) до расчёта метрик, валидации геометрии, обнаружения проблем и подготовки данных для рендеринга.

---

## Содержание

- [Архитектура модуля](#архитектура-модуля)
- [Основные типы данных](#основные-типы-данных)
- [Загрузка данных](#загрузка-данных)
  - [Источники данных](#источники-данных)
  - [Алгоритм разрешения источника](#алгоритм-разрешения-источника)
  - [Парсинг GeoJSON](#парсинг-geojson)
- [Валидация геометрии](#валидация-геометрии)
  - [Удаление дубликатов](#удаление-дубликатов)
  - [Выбор оптимального порядка обхода](#выбор-оптимального-порядка-обхода)
  - [Обнаружение самопересечений](#обнаружение-самопересечений)
  - [Предупреждения о длинных сегментах](#предупреждения-о-длинных-сегментах)
- [Геодезические вычисления](#геодезические-вычисления)
  - [Формула гаверсинуса](#формула-гаверсинуса)
  - [Длина полилинии](#длина-полилинии)
  - [Площадь полигона](#площадь-полигона)
- [Sanity Check](#sanity-check)
- [Упрощение геометрии](#упрощение-геометрии)
  - [Алгоритм Рамера — Дугласа — Пекера](#алгоритм-рамера--дугласа--пекера)
- [Эрозия](#эрозия)
  - [Модель Гауссовского сдвига](#модель-гауссовского-сдвига)
  - [Параллельное выполнение](#параллельное-выполнение)
  - [Замкнутые полилинии](#замкнутые-полилинии)
- [Валидация и визуализация](#валидация-и-визуализация)
  - [ValidationSummary](#validationsummary)
  - [SegmentHighlight](#segmenthighlight)
- [Источник и snapshot](#источник-и-snapshot)
  - [InspectSource](#inspectsource)
  - [Метаданные источника](#метаданные-источника)
  - [Snapshot](#snapshot)
- [Локации](#локации)

---

## Архитектура модуля

```
internal/domain/coastline/
├── source.go           # Загрузка из JSON/GeoJSON, HTTP, кэш
├── validation.go       # Валидация геометрии, self-intersection
├── validation_summary.go # Агрегация проблем валидации
├── visualization.go    # Подсветка проблемных сегментов для SVG
├── sanity.go           # Sanity check длины береговой линии
├── metrics.go          # Консольный вывод метрик
├── locations.go        # Справочник известных локаций
├── data.go             # Константы, GeoBounds, LoadOptions
├── data_test.go
├── source_test.go
├── validation_summary_test.go
└── visualization_test.go
```

Зависимости:
- `internal/domain/geometry` — примитивы (`LatLon`), гаверсинус, упрощение, эрозия, площадь

---

## Основные типы данных

### `geometry.LatLon`

Базовый тип точки с географическими координатами:

```go
type LatLon struct {
    Lat float64 `json:"lat"` // Широта, диапазон [-90, 90]
    Lon float64 `json:"lon"` // Долгота, диапазон [-180, 180]
}
```

### `GeoBounds`

Прямоугольная область для фильтрации GeoJSON:

```go
type GeoBounds struct {
    MinLat, MaxLat float64 // Границы по широте
    MinLon, MaxLon float64 // Границы по долготе
}
```

Метод `Contains(point LatLon) bool` проверяет, попадает ли точка в область.

### `ValidationReport`

Отчёт о валидации после загрузки:

```go
type ValidationReport struct {
    Fixes    []string // Применённые исправления (дедупликация, переупорядочивание)
    Warnings []string // Предупреждения (длинные сегменты, повторяющиеся локации)
}
```

### `LoadResult`

Результат загрузки береговой линии:

```go
type LoadResult struct {
    Points       []geometry.LatLon // Валидированные точки
    Validation   ValidationReport  // Отчёт валидации
    Source       string            // Фактический источник данных
    DatasetName  string            // Имя набора (из метаданных или файла)
    LoadWarnings []string          // Предупреждения при загрузке (fallback и т.д.)
}
```

---

## Загрузка данных

### Источники данных

Модуль поддерживает три уровня источников с приоритетом:

1. **Удалённый GeoJSON** — WFS-эндпоинт Marine Regions или произвольный URL
2. **Локальный кэш** — `data/cache/black-sea.geojson` или хэш URL
3. **Локальный fallback** — `data/black-sea.json`

Константы по умолчанию:

```go
const (
    DefaultCoastlineJSONPath   = "data/black-sea.json"
    DefaultCoastlineCacheDir   = "data/cache"
    marineRegionsWFSURL        = "https://geo.vliz.be/geoserver/MarineRegions/wfs"
    blackSeaMarineRegionID     = 3319
    defaultHTTPTimeout         = 12 * time.Second
)
```

URL по умолчанию для Чёрного моря формируется как WFS-запрос:

```
https://geo.vliz.be/geoserver/MarineRegions/wfs?
  service=WFS&version=1.0.0&request=GetFeature&
  typeName=iho&cql_filter=mrgid=3319&
  outputFormat=application/json
```

### Алгоритм разрешения источника

Функция `resolveSourcePayload()` реализует стратегию загрузки с fallback:

```
1. Если RemoteURL пуст → читать из LocalPath
2. Иначе:
   2a. Если Refresh=false и кэш существует → вернуть кэш
   2b. Попытаться скачать удалённый GeoJSON
       - Успех → обновить кэш, вернуть удалённый
       - Неудача → попробовать кэш
         - Кэш есть → вернуть с warning
         - Кэша нет → попробовать LocalPath
           - Файл есть → вернуть с warning
           - Файла нет → ошибка
```

### Парсинг GeoJSON

Функция `parseCoastlineData()` поддерживает несколько форматов:

**1. Массив точек** (`[{"lat":...,"lon":...}, ...]`)**

Прямая десериализация в `[]geometry.LatLon`.

**2. GeoJSON FeatureCollection**

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": {
        "type": "LineString",
        "coordinates": [[lon1, lat1], [lon2, lat2], ...]
      }
    }
  ]
}
```

Поддерживаемые типы геометрии:
- `LineString` — одна последовательность точек
- `MultiLineString` — несколько независимых линий
- `Polygon` — внешний контур + отверстия (каждое кольцо — отдельная последовательность)
- `MultiPolygon` — несколько полигонов
- `GeometryCollection` — вложенная коллекция геометрий

**Алгоритм извлечения координат:**

```
1. Определить корневой тип (FeatureCollection / Feature / Geometry)
2. Рекурсивно обойти все геометрии
3. Извлечь координатные последовательности
4. Отфильтровать по RemoteBounds (если заданы)
5. Выбрать лучшую последовательность:
   - Самая длинная по PolylineLength()
   - При равенстве — с наибольшим числом точек
```

Конвертация координат: GeoJSON хранит `[longitude, latitude]`, модуль преобразует в `LatLon{Lat, Lon}`.

---

## Валидация геометрии

Функция `validateAndNormalizePoints()` выполняет полный цикл валидации:

### Удаление дубликатов

```go
func removeDuplicateCoordinates(points []LatLon) ([]LatLon, int)
```

Использует `pointKey()` — строковое представление с точностью до 6 знаков после запятой:

```
pointKey(LatLon{44.123456789, 33.987654321}) = "44.123457|33.987654"
```

Все повторяющиеся точки удаляются с сохранением порядка первого вхождения.

### Выбор оптимального порядка обхода

```go
func chooseBestOrder(points []LatLon) []LatLon
```

**Проблема:** данные могут быть загружены в неправильном порядке, что приведёт к «скачкам» через всю береговую линию.

**Алгоритм:**

1. Генерируются кандидаты:
   - Прямой порядок точек
   - Обратный порядок
   - Жадный обход от начальных точек (мин/макс широта/долгота) — по 2 направления на каждую

2. Для каждого кандидата вычисляется `orderScore`:
   ```go
   type orderScore struct {
       intersections int      // Число самопересечений (меньше = лучше)
       longSegments  int      // Число сегментов > порога (меньше = лучше)
       maxSegmentKM  float64  // Максимальная длина сегмента
       totalLengthKM float64  // Общая длина полилинии
   }
   ```

3. Выбирается кандидат с минимальным score (лексикографическое сравнение)

**Жадный обход** (`greedyTraversal`): от стартовой точки на каждом шаге выбирается ближайшая ещё не использованная точка.

### Обнаружение самопересечений

```go
func findSelfIntersections(points []LatLon) []segmentIntersection
```

**Алгоритм:** проверка каждой пары несмежных сегментов на пересечение.

Для сегментов `(a, b)` и `(c, d)` проверяется:

**1. Ориентированная площадь треугольника** (orientation):

```
orientation(a, b, c) = (b.Lon - a.Lon) × (c.Lat - a.Lat) - (b.Lat - a.Lat) × (c.Lon - a.Lon)
```

Это удвоенная площадь треугольника `abc` со знаком:
- `> 0` — точка `c` слева от вектора `ab`
- `< 0` — точка `c` справа от вектора `ab`
- `= 0` — коллинеарность

**2. Критерий пересечения:**

Сегменты пересекаются, если:
- `orientation(a, b, c)` и `orientation(a, b, d)` имеют разные знаки, **И**
- `orientation(c, d, a)` и `orientation(c, d, b)` имеют разные знаки

**3. Специальные случаи** (коллинеарность):

Если `orientation = 0`, проверяется принадлежность точки отрезку через `onSegment()`:

```go
func onSegment(a, b, c) bool:
    return b.Lon ∈ [min(a.Lon, c.Lon), max(a.Lon, c.Lon)] ∧
           b.Lat ∈ [min(a.Lat, c.Lat), max(a.Lat, c.Lat)]
```

**Сложность:** `O(n²)` — приемлемо для типовых береговых линий (до ~10K точек).

### Предупреждения о длинных сегментах

```go
func longSegmentWarnings(points []LatLon, thresholdKM float64) []string
```

По умолчанию `thresholdKM = 450.0`. Каждый сегмент, превышающий порог, генерирует warning с указанием индексов и длины.

---

## Геодезические вычисления

### Формула гаверсинуса

Функция `Haversine(a, b LatLon) float64` вычисляет расстояние между двумя точками на сфере:

```
a = (lat₁, lon₁), b = (lat₂, lon₂)

Δlat = (lat₂ - lat₁) × π / 180
Δlon = (lon₂ - lon₁) × π / 180

h = sin²(Δlat/2) + sin²(Δlon/2) × cos(lat₁) × cos(lat₂)
c = 2 × atan2(√h, √(1-h))

distance = R × c
```

где `R = 6371.0` км — средний радиус Земли.

**Точность:** ~0.5% для расстояний до нескольких тысяч км.

### Длина полилинии

```go
func PolylineLength(points []LatLon) float64
```

Сумма гаверсинусных расстояний между последовательными точками:

```
L = Σ Haversine(points[i-1], points[i]), i = 1..n
```

### Площадь полигона

Функция `Area(points []LatLon) float64` вычисляет площадь в км² через **формулу Гаусса** (shoelace formula):

**1. Проекция координат в метры:**

```
refLat = mean(latᵢ), refLon = mean(lonᵢ)

metersPerDegLat = 111194.9
metersPerDegLon = metersPerDegLat × cos(refLat × π/180)

xᵢ = (lonᵢ - refLon) × metersPerDegLon
yᵢ = (latᵢ - refLat) × metersPerDegLat
```

**2. Shoelace formula:**

```
A = |Σ(xᵢ₋₁ × yᵢ - xᵢ × yᵢ₋₁)| / 2
```

Результат в м² конвертируется в км² делением на 1 000 000.

---

## Sanity Check

Функция `SanityCheck(dataset string, lengthKM float64)` проверяет корректность расчёта длины.

**Эталонные диапазоны** (для известных наборов данных):

```go
var knownCoastlineEstimates = map[string]coastlineEstimate{
    "black-sea.json": {MinKM: 4000, MaxKM: 4987},
}
```

**Допуск:** `sanityTolerance = 0.40` (±40%)

```
minAllowed = MinKM × (1 - 0.40) = MinKM × 0.60
maxAllowed = MaxKM × (1 + 0.40) = MaxKM × 1.40

Если lengthKM ∈ [minAllowed, maxAllowed] → Valid = true
Иначе → Valid = false + Warning
```

**Warning при ошибке** содержит возможные причины:
- Неправильный порядок точек
- Пропущенные участки береговой линии
- Сегменты, пересекающие море

---

## Упрощение геометрии

### Алгоритм Рамера — Дугласа — Пекера

Функция `SimplifyPolyline()` реализует классический алгоритм упрощения полилинии.

**Цель:** сократить число точек, сохранив форму кривой в пределах заданного допуска.

**Алгоритм:**

```
Вход: points[], maxPoints
Выход: simplified[], tolerance

1. Если len(points) ≤ maxPoints → вернуть без изменений

2. Определить диагональ bounding box projected точек:
   diagonal = √((maxX-minX)² + (maxY-minY)²)

3. Бинарный поиск tolerance ∈ [0, diagonal] (24 итерации):
   a. mid = (low + high) / 2
   b. Применить Douglas-Peucker с допуском mid
   c. Если len(simplified) > target → low = mid (нужно строже)
   d. Если len(simplified) < minPoints → high = mid (нужно мягче)
   e. Иначе → запомнить как лучший, high = mid

4. Для замкнутой полилинии:
   - Временно убрать замыкающую точку
   - Упростить
   - Добавить замыкающую точку обратно
```

**Рекурсивный шаг Douglas-Peucker:**

```
func markSimplifiedPoints(projected[], keep[], start, end, tol²):
    1. Найти точку index ∈ (start, end) с максимальным расстоянием
       до отрезка (projected[start], projected[end])
    
    2. Если расстояние > tol²:
       - keep[index] = true
       - Рекурсивно обработать (start, index) и (index, end)
```

**Расстояние от точки P до отрезка AB:**

```
t = ((P-A)·(B-A)) / |B-A|²

Если t ≤ 0:    distance = |P - A|
Если t ≥ 1:    distance = |P - B|
Иначе:         distance = |P - (A + t(B-A))|
```

где `·` — скалярное произведение.

---

## Эрозия

### Модель Гауссовского сдвига

Функция `Erode()` и `ErodeWithSeed()` применяют стохастическое смещение к каждой точке.

**Модель:**

```
Для каждой точки pᵢ = (latᵢ, lonᵢ):

  dx ~ N(0, σ)  // Сдвиг по долготе (в метрах)
  dy ~ N(0, σ)  // Сдвиг по широте (в метрах)

  pᵢ' = (latᵢ + dy/metersPerDegLat, lonᵢ + dx/metersPerDegLon)
```

где:
- `σ = strength` — стандартное отклонение в метрах
- `metersPerDegLat = 111194.9` — метров в одном градусе широты
- `metersPerDegLon = metersPerDegLat × cos(refLat × π/180)` — метров в градусе долготы (зависит от широты)

**Для замкнутых полилиний:** первая и последняя точки получают одинаковый сдвиг, чтобы сохранить замкнутость.

### Параллельное выполнение

Функция `erodeParallel()` разбивает точки на чанки для параллельной обработки:

```
1. Chunk size = 512 точек
2. Каждый чанк обрабатывается в отдельной горутине
3. Детерминизм через seed:

   localSeed = seed + step × 10000 + index
   rng = rand.New(rand.NewSource(localSeed))

   Это гарантирует одинаковый сдвиг для точки index независимо от порядка выполнения горутин.

4. Для замкнутых линий: сдвиг первой точки сохраняется под мьютексом
   и применяется к последней точке после завершения всех горутин.
```

### Многоступенчатая симуляция

```go
func SimulateErosionWithSeed(points []LatLon, steps, strength, seed) [][]LatLon
```

Возвращает `steps + 1` снимков (включая начальное состояние):

```
snapshots[0] = points              // Исходное состояние
snapshots[1] = Erode(points, σ)    // После 1-го шага
snapshots[2] = Erode(snapshots[1], σ)  // После 2-го шага
...
```

Каждый шаг использует новый `step` в seed, обеспечивая разные сдвиги на каждом этапе.

---

## Валидация и визуализация

### ValidationSummary

Функция `BuildValidationSummary()` агрегирует все проблемы валидации в структурированный отчёт:

```go
type ValidationSummary struct {
    Issues []ValidationIssueSummary     // Счётчики по типам
    DuplicateLocations []DuplicateLocationSummary  // Конкретные локации
}
```

**Типы проблем:**

| Тип | Константа | Описание |
|-----|-----------|----------|
| `long_segment` | `WarningTypeLongSegment` | Сегмент > 450 км |
| `duplicate_location` | `WarningTypeDuplicateLocation` | Один ориентир встретился > 1 раза |

**Определение локации:** функция `getLocationName()` использует справочник из 16 известных городов/ориентиров Чёрного моря с порогом `0.15°` (~16 км):

```
locationName(p) = argmin distance(p, lᵢ), где |p.Lat - lᵢ.Lat| < 0.15 ∧ |p.Lon - lᵢ.Lon| < 0.15
```

### SegmentHighlight

Функция `BuildVisualizationHints()` собирает подсказки для рендерера:

```go
type SegmentHighlight struct {
    StartIndex, EndIndex int      // Индексы точек
    Start, End           LatLon   // Координаты
    LengthKM             float64  // Длина сегмента
}

type VisualizationHints struct {
    LongSegments []SegmentHighlight  // Сегменты для подсветки в SVG
}
```

---

## Источник и snapshot

### InspectSource

Функция `InspectSource()` выполняет инспекцию источника без полного парсинга:

```go
type InspectOptions struct {
    LocalPath    string      // Путь к локальному файлу
    RemoteURL    string      // URL удалённого GeoJSON
    CachePath    string      // Путь к кэшу
    SnapshotPath string      // Куда сохранить snapshot
    Refresh      bool        // Принудительное обновление
    HTTPClient   *http.Client
}
```

### Метаданные источника

```go
type SourceMetadata struct {
    Name                string      // Имя набора (из GeoJSON properties)
    RegionID            string      // MRID региона
    Format              string      // "GeoJSON" или "point-array"
    RootType            string      // "FeatureCollection", "Feature", ...
    FeatureCount        int         // Число фич
    GeometryTypes       []string    // ["LineString", "Polygon", ...]
    CoastlinePointCount int         // Число точек после парсинга
    PayloadBytes        int         // Размер сырого payload
    Bounds              GeoBounds   // Bounding box всех точек
}
```

Извлечение метаданных из GeoJSON properties:
- `name` — имя региона
- `mrgid` — идентификатор Marine Regions

### Snapshot

Функция сохраняет копию сырого payload:

```
data/snapshots/
  black-sea-20250411-123456.geojson
```

**Формат имени:** `{slug}-{YYYYMMDD-HHMMSS}.{geojson|json}`

- `slugify()` конвертирует имя в ASCII lowercase с дефисами
- Расширение зависит от формата: `.geojson` для GeoJSON, `.json` для массива точек

---

## Локации

Модуль содержит справочник из 16 известных локаций Чёрного моря:

| Локация | Широта | Долгота |
|---------|--------|---------|
| Одесса, Украина | 46.48 | 30.73 |
| Евпатория, Крым | 45.33 | 32.49 |
| Алушта, Крым | 44.94 | 34.10 |
| Севастополь, Крым | 44.62 | 33.53 |
| Геленджик, Россия | 44.55 | 38.10 |
| Сочи, Россия | 43.70 | 39.75 |
| Адлер, Россия | 43.58 | 39.72 |
| Сухум, Абхазия | 42.00 | 41.58 |
| Поти, Грузия | 42.15 | 41.65 |
| Батуми, Грузия | 41.65 | 41.63 |
| Чорох (граница) | 41.55 | 41.57 |
| Трабзон, Турция | 41.02 | 40.27 |
| Орду, Турция | 41.00 | 39.65 |
| Синоп, Турция | 41.28 | 31.42 |
| Варна, Болгария | 43.00 | 28.00 |

Используется для:
- Идентификации ориентиров в консольном выводе
- Генерации warnings о повторяющихся локациях
- Контекстной информации в SVG-отчётах

---

## Публичный API

### Загрузка данных

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `LoadFromJSON(filename string)` | Загрузка из локального JSON-файла | `points, report, err` |
| `Load(options LoadOptions)` | Загрузка с полным набором опций (remote, cache, bounds) | `LoadResult, err` |
| `FetchCoastlineData(url string)` | Загрузка напрямую из удалённого URL | `points, err` |

### Инспекция источника

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `InspectSource(options InspectOptions)` | Метаданные + snapshot | `SourceInspection, err` |

### Валидация и анализ

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `SanityCheck(dataset, lengthKM)` | Проверка длины береговой линии | `SanityCheckResult` |
| `BuildValidationSummary(points)` | Структурированная сводка проблем | `ValidationSummary` |
| `BuildVisualizationHints(points)` | Подсказки для рендерера (подсветка) | `VisualizationHints` |
| `MainCalculation(coast, name, source)` | Консольный вывод полных метрик | `SanityCheckResult` |

### Константы и конфигурация

| Константа | Значение | Описание |
|-----------|----------|----------|
| `DefaultCoastlineJSONPath` | `"data/black-sea.json"` | Путь к локальному fallback |
| `DefaultCoastlineCacheDir` | `"data/cache"` | Директория кэша |
| `DefaultCoastlineSnapshotDir` | `"data/snapshots"` | Директория snapshot-ов |
| `EarthRadiusKM` | `6371.0` | Средний радиус Земли |
| `metersPerDegLat` | `111194.9` | Метров в градусе широты |
| `sanityTolerance` | `0.40` | Допуск sanity check (±40%) |
| `longSegmentWarningKM` | `450.0` | Порог предупреждения о длинном сегменте |
| `defaultHTTPTimeout` | `12s` | Таймаут HTTP-запроса |
| `erosionChunkSize` | `512` | Размер чанка для параллельной эрозии |
| `maxConsolePoints` | `30` | Макс. точек в консольном выводе |
| `locationThreshold` | `0.15°` | Порог привязки к ориентиру (~16 км) |

### Оценки береговых линий

| Набор данных | Min (км) | Max (км) |
|--------------|----------|----------|
| `black-sea.json` | 4000 | 4987 |

---

## Примеры использования API

### Базовая загрузка из JSON

```go
package main

import (
    "coastal-geometry/internal/domain/coastline"
)

func main() {
    points, report, err := coastline.LoadFromJSON("data/black-sea.json")
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Загружено %d точек\n", len(points))
    fmt.Printf("Исправления: %v\n", report.Fixes)
    fmt.Printf("Предупреждения: %v\n", report.Warnings)
}
```

### Загрузка с удалённого источника с кэшированием

```go
func main() {
    result, err := coastline.Load(coastline.LoadOptions{
        LocalPath:    "data/black-sea.json",
        RemoteURL:    coastline.DefaultCoastlineGeoJSONURL,
        RemoteBounds: coastline.DefaultBlackSeaBounds,
        CachePath:    "data/cache/black-sea.geojson",
        Refresh:      false, // использовать кэш если есть
    })
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Источник: %s\n", result.Source)
    fmt.Printf("Точек: %d\n", len(result.Points))
    fmt.Printf("Dataset: %s\n", result.DatasetName)
    for _, w := range result.LoadWarnings {
        fmt.Printf("  ⚠ %s\n", w)
    }
}
```

### Инспекция источника с сохранением snapshot

```go
func main() {
    inspection, err := coastline.InspectSource(coastline.InspectOptions{
        RemoteURL:    coastline.DefaultCoastlineGeoJSONURL,
        SnapshotPath: "./data/snapshots",
        Refresh:      true, // принудительно обновить кэш
    })
    if err != nil {
        panic(err)
    }
    
    meta := inspection.Metadata
    fmt.Printf("Название: %s\n", meta.Name)
    fmt.Printf("Регион ID: %s\n", meta.RegionID)
    fmt.Printf("Формат: %s\n", meta.Format)
    fmt.Printf("Корневой тип: %s\n", meta.RootType)
    fmt.Printf("Фич: %d\n", meta.FeatureCount)
    fmt.Printf("Геометрии: %v\n", meta.GeometryTypes)
    fmt.Printf("Точек: %d\n", meta.CoastlinePointCount)
    fmt.Printf("Размер: %d байт\n", meta.PayloadBytes)
    fmt.Printf("Bounds: [%.2f, %.2f] x [%.2f, %.2f]\n",
        meta.Bounds.MinLat, meta.Bounds.MaxLat,
        meta.Bounds.MinLon, meta.Bounds.MaxLon)
}
```

### Валидация и визуализация

```go
func main() {
    points, _, err := coastline.LoadFromJSON("data/black-sea.json")
    if err != nil {
        panic(err)
    }
    
    // Структурированная сводка проблем
    summary := coastline.BuildValidationSummary(points)
    for _, issue := range summary.Issues {
        fmt.Printf("%s: %d (порог: %.0f км)\n", 
            issue.WarningType, issue.Count, issue.ThresholdKM)
    }
    for _, dup := range summary.DuplicateLocations {
        fmt.Printf("  Дубликат: %s × %d\n", dup.Name, dup.Count)
    }
    
    // Подсказки для рендерера
    hints := coastline.BuildVisualizationHints(points)
    for _, seg := range hints.LongSegments {
        fmt.Printf("Длинный сегмент %d-%d: %.0f км\n",
            seg.StartIndex, seg.EndIndex, seg.LengthKM)
    }
}
```

### Полный расчёт и вывод в консоль

```go
func main() {
    result, err := coastline.Load(coastline.LoadOptions{
        LocalPath: "data/black-sea.json",
    })
    if err != nil {
        panic(err)
    }
    
    sanity := coastline.MainCalculation(
        result.Points, 
        result.DatasetName, 
        result.Source,
    )
    
    if !sanity.Valid {
        fmt.Println(sanity.Warning)
    }
}
```

---

## Обработка ошибок

### LoadFromJSON / Load

| Ошибка | Условие |
|--------|---------|
| `read coastline json "..."` | Файл не найден / нет прав на чтение |
| `parse coastline data "..."` | Невалидный JSON / неподдерживаемый формат |
| `empty coastline payload` | Пустой файл |
| `coastline data must contain at least 2 points` | Недостаточно точек |
| `coastline data has invalid latitude at index N: X` | Широта вне [-90, 90] |
| `coastline data has invalid longitude at index N: X` | Долгота вне [-180, 180] |
| `после удаления дубликатов осталось меньше 2 точек` | Все точки были дубликатами |
| `полилиния имеет self-intersection: пересекаются сегменты X и Y` | Невосстановимая ошибка геометрии |

### InspectSource

| Ошибка | Условие |
|--------|---------|
| `read coastline json "..."` | Локальный файл не найден |
| `inspect coastline source "..."` | Невозможно распарсить payload |
| `create snapshot directory "..."` | Нет прав на запись |
| `write snapshot "..."` | Ошибка записи snapshot |

### FetchCoastlineData

| Ошибка | Условие |
|--------|---------|
| `remote url is empty` | URL не передан |
| `build GET request for "..."` | Невалидный URL |
| `request coastline url "..."` | Сетевая ошибка |
| `request coastline url "..."` | HTTP-статус ≠ 200 |
| `read coastline response "..."` | Ошибка чтения body |

### Sanity Check

Sanity check **никогда не возвращает ошибку**. Вместо этого:
- `Checked = false` — набор данных неизвестен, проверка пропущена
- `Checked = true, Valid = false` — длина вне ожидаемого диапазона, `Warning` содержит детали

---

## Тестирование

Модуль покрыт unit-тестами. Запуск:

```bash
go test ./internal/domain/coastline/...
```

### Покрытие тестами

| Функция | Что тестируется |
|---------|-----------------|
| `LoadFromJSON` | ✅ Загрузка валидных данных<br>✅ Ошибка при невалидной широте<br>✅ Удаление дубликатов |
| `validateAndNormalizePoints` | ✅ Переупорядочивание точек при self-intersection<br>✅ Предупреждения о длинных сегментах и дубликатах |
| `findSelfIntersections` | ✅ Обнаружение пересекающихся сегментов |
| `SanityCheck` |✅ Warning для известного набора с некорректной длиной<br>✅ Пропуск для неизвестного набора |
| `FetchCoastlineData` | ✅ Парсинг GeoJSON Polygon с фильтрацией по bounds<br>✅ Сохранение замкнутого кольца |
| `Load` | ✅ Использование удалённого GeoJSON<br>✅ Сохранение замкнутого кольца<br>✅ Fallback на локальный JSON при ошибке remote<br>✅ Использование кэша без remote-запроса<br>✅ Обновление кэша при `Refresh=true`<br>✅ Использование stale-кэша при ошибке refresh |
| `InspectSource` | ✅ Сохранение snapshot + извлечение метаданных из GeoJSON<br>✅ Fallback на локальный + генерация `.json` snapshot |
| `BuildValidationSummary` | ✅ Включение длинных сегментов и дубликатов<br>✅ Стабильные строки с count=0 для чистой геометрии |
| `BuildVisualizationHints` | ✅ Обнаружение длинных сегментов с правильными индексами |

### Паттерны тестирования

- **httptest.Server** — мокирование WFS-эндпоинта для тестирования загрузки
- **t.TempDir()** — изолированные временные директории для кэша и snapshot-ов
- **atomic.Int32** — подсчёт HTTP-запросов для проверки кэширования
- **Table-driven test** — через отдельные Test-функции для каждой сценарной проверки

---

## Связанные модули

- [`../geometry`](../geometry) — `LatLon`, `Haversine`, `PolylineLength`, `Area`, `SimplifyPolyline`, `Erode`, `ErodeWithSeed`, `SimulateErosionWithSeed`
- [`../render`](../render) — генерация SVG-отчётов с использованием валидации и подсветки
- [`../../cmd/lito`](../../cmd/lito) — CLI-интерфейс, использующий все функции модуля
