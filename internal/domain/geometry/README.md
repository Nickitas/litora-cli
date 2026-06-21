# Package `geometry`

**Базовые геометрические примитивы: координаты, расстояния, длины, площади, упрощение и эрозия.**

Модуль предоставляет фундаментальные математические операции для работы с береговыми линиями: гаверсинусное расстояние, длина полилинии, площадь полигона, упрощение Рамера — Дугласа — Пекера и стохастическая эрозия.

---

## Содержание

- [Архитектура модуля](#архитектура-модуля)
- [Основные типы данных](#основные-типы-данных)
- [Гаверсинусное расстояние](#гаверсинусное-расстояние)
  - [Формула](#формула)
  - [Точность и ограничения](#точность-и-ограничения)
- [Длина полилинии](#длина-полилинии)
- [Площадь полигона](#площадь-полигона)
  - [Проекция координат](#проекция-координат)
  - [Формула Гаусса (shoelace)](#формула-гаусса-shoelace)
- [Упрощение геометрии](#упрощение-геометрии)
  - [Алгоритм Рамера — Дугласа — Пекера](#алгоритм-рамера--дугласа--пекера)
  - [Бинарный поиск допуска](#бинарный-поиск-допуска)
  - [Обработка замкнутых полилиний](#обработка-замкнутых-полилиний)
- [Волновая эрозия](#волновая-эрозия)
  - [Физическая модель волнового воздействия](#физическая-модель-волнового-воздействия)
  - [Fetch и экспозиция берега](#fetch-и-экспозиция-берега)
  - [Батиметрия и физическая глубина](#батиметрия-и-физическая-глубина)
  - [Параметры волновой эрозии](#параметры-волновой-эрозии)
- [Временная динамика эрозии](#временная-динамика-эрозии)
  - [Модель временных параметров](#модель-временных-параметров)
  - [Штормовые события](#штормовые-события)
  - [Сезонность](#сезонность)
  - [Подъём уровня моря](#подъём-уровня-моря)
  - [Интеграция с волновой эрозией](#интеграция-с-волновой-эрозией)
- [Транспорт наносов](#транспорт-наносов)
  - [Модель баланса массы](#модель-баланса-массы)
  - [Longshore drift](#longshore-drift)
  - [Аккумуляция и эрозия](#аккумуляция-и-эрозия)
  - [Интеграция с волновой эрозией](#интеграция-с-волновой-эрозией-1)
- [Литологический модуль](#литологический-модуль)
  - [Структура данных](#структура-данных)
  - [IDW интерполяция](#idw-интерполяция)
  - [Загрузка профиля](#загрузка-профиля)
  - [Интеграция с эрозией](#интеграция-с-эрозией)
  - [Формат данных](#формат-данных)
  - [Дефолтный профиль](#дефолтный-профиль)
- [Батиметрический модуль](#батиметрический-модуль)
  - [Структура данных](#структура-данных-1)
  - [Загрузка и интерполяция](#загрузка-и-интерполяция-1)
  - [Формат данных](#формат-данных-1)
- [Эрозия](#эрозия)
  - [Модель Гауссовского сдвига](#модель-гауссовского-сдвига)
  - [Параллельное выполнение](#параллельное-выполнение)
  - [Детерминизм через seed](#детерминизм-через-seed)
  - [Замкнутые полилинии](#замкнутые-полилинии)
  - [Многоступенчатая симуляция](#многоступенчатая-симуляция)
- [Константы и конфигурация](#константы-и-конфигурация)
- [Публичный API](#публичный-api)
- [Примеры использования](#примеры-использования)
- [Связанные модули](#связанные-модули)

---

## Архитектура модуля

```
internal/domain/geometry/
├── types.go        # Базовый тип LatLon
├── haversine.go    # Гаверсинусное расстояние
├── length.go       # Длина полилинии
├── area.go         # Площадь полигона (shoelace)
├── simplify.go     # Упрощение (Ramer-Douglas-Peucker)
├── erosion.go      # Стохастическая и волновая эрозия
├── temporal.go     # Временная динамика эрозии
├── sediment.go     # Транспорт наносов и аккумуляция
├── bathymetry.go   # Батиметрический модуль
├── lithology.go    # Литологический модуль
└── *_test.go       # Тесты всех модулей
```

Зависимости: **отсутствуют** (модуль самодостаточен)

---

## Основные типы данных

### `LatLon`

Базовый тип точки с географическими координатами:

```go
type LatLon struct {
    Lat float64 `json:"lat"` // Широта, диапазон [-90, 90]
    Lon float64 `json:"lon"` // Долгота, диапазон [-180, 180]
}
```

JSON-теги позволяют напрямую десериализовать массивы точек:

```json
[
  {"lat": 46.48, "lon": 30.73},
  {"lat": 41.65, "lon": 41.63}
]
```

### `SimplifyOptions`

```go
type SimplifyOptions struct {
    MaxPoints int // Целевое максимальное число точек (0 = без ограничений)
}
```

### `SimplifyResult`

```go
type SimplifyResult struct {
    Points           []LatLon // Упрощённые точки
    OriginalCount    int      // Исходное число точек
    SimplifiedCount  int      // Число точек после упрощения
    ToleranceMeters  float64 // Найденный допуск в метрах
    Applied          bool     // Было ли применено упрощение
    OriginalClosed   bool     // Была ли исходная замкнутой
    SimplifiedClosed bool     // Осталась ли замкнутой
}
```

---

## Гаверсинусное расстояние

### Формула

Функция `Haversine(a, b LatLon) float64` вычисляет расстояние между двумя точками на сфере по формуле гаверсинуса:

```
Δlat = (lat₂ - lat₁) × π/180
Δlon = (lon₂ - lon₁) × π/180

lat₁_rad = lat₁ × π/180
lat₂_rad = lat₂ × π/180

h = sin²(Δlat/2) + sin²(Δlon/2) × cos(lat₁_rad) × cos(lat₂_rad)
c = 2 × atan2(√h, √(1 - h))

distance = R × c
```

где:
- `R = 6371.0` км — средний радиус Земли
- `h` — гаверсинус центрального угла
- `c` — центральный угол в радианах

**Тождество:** `sin²(x) = sin(x) × sin(x)`

### Точность и ограничения

| Фактор | Влияние |
|--------|---------|
| Сферическая модель Земли | ~0.3% ошибки (Земля — эллипсоид) |
| Средний радиус | ~0.5% ошибки (полюса/экватор отличаются) |
| Малые расстояния (< 1 км) | Хорошая точность |
| Большие расстояния (> 5000 км) | Погрешность возрастает до ~1% |
| Антиподальные точки | Численная нестабильность (крайний случай) |

**Для береговых линий Чёрного моря** (расстояния до ~1000 км) точность ~0.5–1%, что достаточно для задач проекта.

---

## Длина полилинии

Функция `PolylineLength(points []LatLon) float64`:

```
L = Σ Haversine(points[i-1], points[i]),  i = 1..n

где n = len(points)
```

Если `len(points) < 2`, возвращает `0`.

**Пример:**

```go
coast := []LatLon{
    {Lat: 46.48, Lon: 30.73}, // Одесса
    {Lat: 41.65, Lon: 41.63}, // Батуми
}

length := PolylineLength(coast)
// ~1100 км (гаверсинусное расстояние)
```

---

## Площадь полигона

### Проекция координат

Перед вычислением площади географические координаты проецируются в локальную декартову систему (метры):

```go
func projectToMetersLocal(points []LatLon) []pointXY:
    refLat = mean(latᵢ)
    refLon = mean(lonᵢ)
    
    metersPerDegLat = 111194.9
    metersPerDegLon = metersPerDegLat × cos(refLat × π/180)
    
    // Защита от полюсов
    if |metersPerDegLon| < 1e-9:
        metersPerDegLon = metersPerDegLat
    
    for each point:
        x = (lon - refLon) × metersPerDegLon
        y = (lat - refLat) × metersPerDegLat
```

**Почему проекция?** Shoelace formula работает в декартовых координатах. Проекция использует среднюю широту как опорную для минимизации искажений.

### Формула Гаусса (shoelace)

Площадь полигона через координаты вершин:

```
A = |Σ(xᵢ₋₁ × yᵢ - xᵢ × yᵢ₋₁)| / 2

где сумма по i = 0..n-1, с циклическим переходом (x₋₁ = xₙ₋₁)
```

**Алгоритм:**

```go
func Area(points []LatLon) float64:
    if len(points) < 3 → return 0
    
    projected = projectToMetersLocal(points)
    
    // Убедиться в замкнутости
    if points[0] != points[last]:
        projected.append(projected[0])
    
    areaMeters2 = 0
    last = projected[last]
    for each p in projected:
        areaMeters2 += (last.X × p.Y - p.X × last.Y)
        last = p
    
    return |areaMeters2| / 2 / 1_000_000  // m² → km²
```

**Пример:**

```go
// Прямоугольник 1° × 1° на широте 43.5°
polygon := []LatLon{
    {Lat: 43.0, Lon: 35.0},
    {Lat: 44.0, Lon: 35.0},
    {Lat: 44.0, Lon: 36.0},
    {Lat: 43.0, Lon: 36.0},
}

area := Area(polygon)
// ~8000 км² (приблизительно)
```

---

## Упрощение геометрии

### Алгоритм Рамера — Дугласа — Пекера

Функция `SimplifyPolyline()` реализует классический алгоритм упрощения полилинии.

**Цель:** сократить число точек, сохранив форму кривой в пределах допуска.

**Идея:** рекурсивно находить точку с максимальным отклонением от отрезка между концами. Если отклонение > допуска — сохранить точку и рекурсивно обработать обе половины.

```
Исходная:    A────────────────────────────B

Шаг 1: найти точку P с max отклонением от AB

Если |P, AB| > tolerance:
    A────────P────────────B
    (сохранить P, рекурсия на AP и PB)

Иначе:
    A────────────────────B
    (удалить все промежуточные)
```

**Расстояние от точки P до отрезка AB:**

```
dx = B.x - A.x
dy = B.y - A.y
L² = dx² + dy²

if L² == 0:  // A и B совпадают
    return |P - A|²

// Проекция P на AB
t = ((P.x - A.x) × dx + (P.y - A.y) × dy) / L²

if t ≤ 0:    return |P - A|²   // Ближе к A
if t ≥ 1:    return |P - B|²   // Ближе к B
else:        return |P - (A + t·AB)|²  // Проекция на отрезок
```

где `t` — параметрическая координата проекции на отрезок.

### Бинарный поиск допуска

Пользователь задаёт `MaxPoints`, но алгоритм работает с допуском в метрах. Модуль автоматически находит подходящий допуск бинарным поиском:

```go
func SimplifyPolyline(points, options) SimplifyResult:
    if len(points) <= options.MaxPoints → без изменений
    
    projected = projectToMeters(points)
    diagonal = projectedDiagonal(projected)
    
    low = 0.0
    high = diagonal
    best = points
    bestTolerance = 0.0
    
    // 24 итерации бинарного поиска
    for i = 0..23:
        mid = (low + high) / 2
        simplified = simplifyWithTolerance(points, projected, mid)
        
        if len(simplified) > target:
            low = mid  // Нужно строже
        else if len(simplified) < minPoints:
            high = mid // Нужно мягче
        else:
            best = simplified
            bestTolerance = mid
            high = mid // Попробовать строже
    
    return {Points: best, ToleranceMeters: bestTolerance, Applied: true}
```

**Почему 24 итерации?**

```
precision = diagonal / 2^24 ≈ diagonal / 1.67 × 10⁷
```

Для диагонали 1000 км это ~60 метров точности — достаточно для практических целей.

### Обработка замкнутых полилиний

Для замкнутых полилиний (где первая и последняя точки совпадают):

```go
if isClosedPolyline(points):
    // Временно убрать замыкающую точку
    working = points[0..last-1]
    target = options.MaxPoints - 1  // Зарезервировать место для замыкания
    minPoints = 3  // Минимум треугольник
    
    result = SimplifyPolyline(working, target)
    
    // Добавить замыкающую точку обратно
    result.Points.append(result.Points[0])
```

**Специальные случаи:**
- `≤ 4 точки` → не упрощать (слишком мало)
- `target < minPoints` → ограничить до minPoints

---

## Волновая эрозия

### Физическая модель волнового воздействия

Модуль реализует физически обоснованную модель волновой эрозии береговой линии, которая учитывает:

1. **Направленность волн** — волны приходят от определённого направления (задаётся пользователем)
2. **Fetch расстояние** — длина открытой воды по направлению волны
3. **Экспозиция сегмента** — насколько открыт сегмент берега к волнению
4. **Батиметрия** — реальные глубины у берега влияют на энергию волн

В отличие от простой гауссовской эрозии, волновая модель сильнее размывает открытые мысы и слабее — защищённые бухты.

### Fetch и экспозиция берега

**Fetch** — расстояние от точки берега до противоположного берега в заданном направлении. Чем больше fetch, тем выше энергия волн.

**Расчёт fetch:**
```
Для каждого сегмента берега:
  1. Определить нормаль к берегу ( seaward normal )
  2. Выпустить лучи в секторе направлений волн
  3. Найти пересечение с противоположным берегом
  4. Fetch = расстояние до пересечения
```

**Экспозиция** — функция угла падения волны:
```
exposure = cos(θ)^power

где:
  θ — угол между нормалью к берегу и направлением волны
  power — показатель нелинейности (по умолчанию 1.5)
```

Сегменты, ориентированные перпендикулярно волнам, получают максимальную экспозицию.

### Батиметрия и физическая глубина

**Физический принцип:** Чем глубже вода у берега, тем больше энергии волна может передать берегу, тем сильнее эрозия.

**Расчёт depth factor:**

**Геометрический proxy (без батиметрии):**
```go
depthFactor = 1 - exp(-fetch / depthScale)

где fetch используется как proxy для глубины
```

**Физическая модель (с батиметрией):**
```go
effectiveDepth = max(0, -depthMeters)  // Глубина в метрах (положительная)
depthFactor = 1 - exp(-effectiveDepth / depthScale)

где depthMeters < 0 для подводных глубин
```

**Интерполяция батиметрии:**
Батиметрические данные интерполируются билинейным методом для получения глубины в любой точке берега.

### Параметры волновой эрозии

```go
type WaveErosionOptions struct {
    StrengthMeters           float64  // Базовая сила эрозии (м)
    WindSourceDirectionDeg   float64  // Направление ветра (градусы от севера)
    WindSpeedMetersPerSecond float64  // Скорость ветра (м/с)
    FetchSpreadDeg           float64  // Разброс направлений (градусы)
    FetchSamples             int      // Число лучей для fetch
    MaxFetchMeters           float64  // Максимальный fetch (м)
    DepthScaleMeters         float64  // Масштаб глубины (м)
    ExposurePower            float64  // Показатель экспозиции
    BathymetryGrid           *BathymetryGrid // Опциональная батиметрия
}
```

**Влияние параметров:**

| Параметр | Влияние на эрозию |
|----------|-------------------|
| `StrengthMeters` | Базовый отступ берега за шаг |
| `WindSpeedMetersPerSecond` | Энергия волн ∝ (скорость)² |
| `FetchSpreadDeg` | Широта сектора выборки |
| `FetchSamples` | Точность оценки экспозиции |
| `DepthScaleMeters` | Масштаб затухания по глубине |
| `ExposurePower` | Нелинейность угла падения |

**Алгоритм расчёта отступа:**
```go
// 1. Оценить fetch и экспозицию
fetch = meanFetchalongSector
exposure = weightedIncidence / samples

// 2. Определить depth factor
if bathymetry != nil:
    depth = interpolateDepth(lat, lon)
    depthFactor = 1 - exp(-depth / depthScale)
else:
    depthFactor = 1 - exp(-fetch / depthScale)

// 3. Рассчитать силу ветра
windFactor = clamp((windSpeed / 12)², 0.1, 4.0)

// 4. Базовый отступ
retreat = strength × windFactor × fetchFactor × exposure × (0.35 + 0.65 × depthFactor)

// 5. Поправка на форму берега
protrusion = насколько выступает мыс
bayShelter = насколько защищена бухта
retreat ×= clamp(0.55 + protrusion - 0.35 × bayShelter, 0.1, 1.75)

// 6. Сглаживание
smoothedRetreat = retreat + shapeCorrection × smoothAlpha
```

---

## Временная динамика эрозии

### Модель временных параметров

Модуль реализует временную динамику эрозии, позволяя моделировать изменение береговой линии на протяжении длительных периодов (десятилетия) с учётом нестационарных факторов: штормов, сезонности, подъёма уровня моря.

```go
type TemporalParameters struct {
    YearsPerStep          float64  // Лет за один шаг модели
    StormProbability     float64  // Вероятность шторма [0-1]
    StormIntensityMult   float64  // Множитель силы шторма [1.0-5.0]
    SeaLevelRise         float64  // Подъём уровня моря (м/год)
    Seasonality          bool     // Учитывать сезонность
    SeasonalPhase        float64  // Фаза сезонности (радианы)
    MinYearsPerStep      float64  // Минимальное значение для валидации
    MaxYearsPerStep      float64  // Максимальное значение для валидации
}

type TemporalState struct {
    Step            int       // Номер шага
    Year            float64   // Текущий год
    IsStorm         bool      // Штормовое событие
    StormIntensity  float64   // Интенсивность шторма [1.0+]
    SeasonalFactor  float64   // Сезонный множитель [0.5-1.5]
    SeaLevelOffset  float64   // Смещение уровня моря (м)
    EffectiveYears  float64   // Эффективное число лет для шага
}
```

### Штормовые события

**Вероятностная модель:** на каждом шаге моделирования проверяется вероятность шторма:

```go
if rng.Float64() < StormProbability:
    IsStorm = true
    StormIntensity = StormIntensityMult + variation
```

**Интенсивность шторма:** базовый множитель + случайная вариация (±50%).

| Вероятность | Частота штормов | Пример климата |
|-------------|-----------------|-----------------|
| 0.0 | Никогда | Идеально спокойный |
| 0.05 | 1 раз в 20 лет | Умеренный |
| 0.1 | 1 раз в 10 лет | Штормовой |
| 0.2+ | Частые штормы | Экстремальный |

### Сезонность

Модель учитывает годовые колебания волновой активности:

```go
seasonalFactor = 1.0 + 0.5 × sin(2π × year + phase)
```

**Интерпретация:**
- `seasonalFactor = 1.5` — пик активности (зимние штормы)
- `seasonalFactor = 0.5` — минимум активности (летний штиль)
- `phase` — сдвиг фазы (позволяет двигать пик)

**Типичные значения:**
- `phase = 0` — пик в начале года
- `phase = π` — пик в середине года
- `phase = 3π/2` — пик зимой (северное полушарие)

### Подъём уровня моря

**Линейная модель:** уровень моря растёт с постоянной скоростью:

```go
seaLevelOffset = year × SeaLevelRise
```

**Модуляция эрозии:** чем выше уровень моря, тем сильнее эрозия:

```go
seaLevelFactor = 1.0 + 0.1 × ln(1 + seaLevelOffset)
modulatedErosion = baseErosion × seaLevelFactor
```

Логарифмическая зависимость отражает убывающую эффективность: каждые дополнительные 10 м подъёма дают меньший эффект.

**Типичные значения (IPCC сценариии):**
| Сценарий | Подъём (м/год) | Через 50 лет |
|----------|----------------|--------------|
| RCP2.6 | 0.003 | +0.15 м |
| RCP4.5 | 0.006 | +0.30 м |
| RCP8.5 | 0.010 | +0.50 м |

### Интеграция с волновой эрозией

Временная динамика модулирует параметры волновой эрозии на каждом шаге:

```go
func SimulateErosionWithDurationSeed(
    points []LatLon,
    targetYears int,
    temporalParams TemporalParameters,
    waveOptions WaveErosionOptions,
    seed int64,
) TemporalResult:
```

**Алгоритм:**
1. Рассчитать число шагов = `ceil(targetYears / YearsPerStep)`
2. Для каждого шага:
   - Рассчитать `TemporalState` (шторм, сезонность, подъём уровня)
   - Модулировать силу эрозии: `strength × StormIntensity × SeasonalFactor × SeaLevelFactor`
   - Применить волновую эрозию с модулированной силой
3. Вернуть снапшоты и временные состояния

**Результат:**

```go
type TemporalResult struct {
    Snapshots           [][]LatLon       // Состояния береговой линии по шагам
    TemporalStates      []TemporalState  // Временные состояния по шагам
    TotalYears          float64          // Общее число промоделированных лет
    StormCount          int              // Число штормовых событий
    AccumulatedErosion  float64          // Накопленная эрозия (м)
    FinalSeaLevelRise   float64          // Итоговый подъём уровня моря (м)
}
```

### Метрики эрозии по шагам

```go
type ErosionMetrics struct {
    Step                 int      // Номер шага
    Year                 float64  // Год
    LengthKm             float64  // Длина береговой линии (км)
    AreaKm2              float64  // Площадь (км²)
    ErodedM3             float64  // Объём эрозии (м³)
    DepositedM3          float64  // Объём депозиции (м³)
    NetChangeM3          float64  // Баланс (м³)
    FractalDimension     float64  // Фрактальная размерность
    MeanRetreatMeters    float64  // Среднее отступание (м)
    MaxRetreatMeters     float64  // Максимальное отступание (м)
    IsStorm              bool     // Штормовое событие
    SeasonalFactor       float64  // Сезонный множитель
}
```

---

## Транспорт наносов

### Модель баланса массы

Модуль моделирует транспорт наносов (sediment transport) вдоль береговой линии, учитывая:
- Эрозию источника материала
- Longshore drift (транспорт вдоль берега)
- Депозицию в защитных зонах
- Баланс массы

```go
type SedimentBudget struct {
    ErodedVolume    float64  // Объём размытого материала (м³/м)
    TransportVolume float64  // Объём в транзите (longshore drift)
    DepositedVolume float64  // Объём отложенного материала
    NetChange       float64  // Баланс (eroded - deposited)
    ErosionPoints    int      // Число точек с эрозией
    DepositionPoints int     // Число точек с аккумуляцией
}

type SedimentState struct {
    PointIndex     int
    LocalBudget    SedimentBudget
    InTransitFrom  []float64  // Объём от соседей (incoming)
    InTransitTo    []float64  // Объём к соседям (outgoing)
    IsAccumulating bool       // Режим аккумуляции
    IsEroding      bool       // Режим эрозии
}
```

### Longshore drift

**Longshore drift** — транспорт наносов вдоль берега под действием волн.

```go
type SedimentTransportParameters struct {
    TransportCoefficient      float64  // [0-1] часть в транспорт
    DepositionRate            float64  // [0-1] скорость отложения
    MinimumFlowVelocity      float64  // Минимальная скорость для транспорта (м/с)
    CapacityFactor           float64  // [0-2] ёмкость аккумуляции
    LongshoreDriftCoefficient float64  // [0-1] alongshore транспорт
}
```

**Алгоритм расчёта:**

```go
// 1. Alongshore направление (от prev к next)
alongshoreVector = (next - prev) / |next - prev|

// 2. Wave direction
waveDir = (sin(Direction), cos(Direction))

// 3. Alongshore компонента
alongshoreComponent = |alongshoreVector · waveDir|

// 4. Drift распределение
driftFraction = LongshoreDriftCoefficient × alongshoreComponent × waveEnergy

toPrev = transportVolume × 0.5 × driftFraction
toNext  = transportVolume × 0.5 × driftFraction
```

**Направление drift:** зависит от cross product между alongshore и wave direction.

### Аккумуляция и эрозия

```go
type WaveEnergyData struct {
    Energy    []float64  // Волновая энергия [0-1] по точкам
    Direction float64     // Главное направление (град от севера)
    Incidence []float64  // Угол падения на берег [0-1]
    Fetch     []float64  // Fetch distance (м)
}
```

**Логика аккумуляции:**

```go
incomingTotal = sum(InTransitFrom)
localCapacity = CapacityFactor × waveEnergy

if incomingTotal > localCapacity OR waveEnergy < 0.3:
    excess = incomingTotal - localCapacity
    deposition = excess × DepositionRate
    IsAccumulating = true
else:
    IsEroding = true

NetChange = ErodedVolume - DepositedVolume
```

**Интерпретация:**
- Защищённые бухты (`waveEnergy < 0.3`) → аккумуляция независимо от incoming
- Высокая энергия волн → низкая ёмкость → эрозия
- Избыток наносов → отложение (accretion)

### Интеграция с волновой эрозией

```go
func CalculateSedimentTransport(
    points []LatLon,
    erosionRates []float64,
    waveData WaveEnergyData,
    lithology []LithologyState,
    params SedimentTransportParameters,
) SedimentTransportResult
```

**Результат:**

```go
type SedimentTransportResult struct {
    States          []SedimentState
    TotalBudget     SedimentBudget
    MassBalance     float64  // Должен быть ≈ 0
    IsValid         bool     // Проверка баланса массы
    Warnings        []string
    BaselineErosion []float64  // Базовая эрозия (м)
    ModifiedErosion []float64  // Модифицированная эрозия (м)
}
```

**Валидация:** баланс массы должен сохраняться (допуск 15%).

```go
ApplySedimentModification(points, baseErosion, sedimentResult) → []float64
// В точках аккумуляции: erosion -= depositedVolume
```

---

## Литологический модуль

### Структура данных

**Литологический профиль** содержит распределение пород по региону с их сопротивляемостью эрозии:

```go
type LithologyProfile struct {
    Metadata  LithologyMetadata         // Метаданные профиля
    Points    []LithologyPoint          // Точки замера lithology
    Classes   map[string]LithologyClass // Классы пород
    Baselines map[string]ErosionBaseline // Базовые скорости эрозии
}

type LithologyPoint struct {
    Lat              float64  // Широта
    Lon              float64  // Долгота
    Region           string   // Регион
    Lithology        string   // Класс породы
    Resistance       float64  // Сопротивление эрозии [0.1-10.0]
    Color            string   // Цвет для визуализации
    Description      string   // Описание
    Confidence       string   // Уверенность в данных
    Source           string   // Источник данных
    ErosionObserved  *float64 // Наблюдаемая эрозия (м/год)
    Dynamic          bool     // Динамическая литология (пляжи)
}

type LithologyClass struct {
    Resistance    float64    // Базовое сопротивление
    Color         string     // Цвет для SVG
    Description   string     // Описание породы
    ErosionRange  []float64  // Диапазон эрозии (м/год)
    Dynamic       bool       // Динамический класс
}

type LithologyState struct {
    Class       string  // Класс породы в точке
    Resistance  float64 // Интерполированное сопротивление
    Color       string  // Цвет
    Description string  // Описание
}
```

### IDW интерполяция

Модуль использует **Inverse Distance Weighting (IDW)** для интерполяции сопротивления пород между точками замера:

```
Для запрашиваемой точки (lat, lon):
  1. Найти N ближайших точек (по умолчанию N=6)
  2. Рассчитать веса:  wᵢ = 1 / distance²
  3. Нормировать:      Wᵢ = wᵢ / Σwⱼ
  4. Интерполировать:  R = Σ(Wᵢ × Resistanceᵢ)
  5. Класс и цвет — от точки с максимальным весом
```

**Физический смысл:** Точки ближе к точке замера наследуют её свойства, с плавным переходом между регионами.

### Загрузка профиля

```go
// Из JSON файла
profile, err := LoadLithologyProfileFromFile("data/black-sea-lithology.json")

// Из байтов
profile, err := LoadLithologyProfile(jsonData)

// Получение литологии в точке
state := profile.GetLithologyAt(lat, lon)
fmt.Printf("Порода: %s, Сопротивление: %.1f\n", state.Class, state.Resistance)

// Статистика профиля
stats := profile.GetStatistics()
// → num_points, num_classes, resistance_min/max/mean, confidence_distribution
```

### Интеграция с эрозией

Литология модулирует скорость отступа берега в волновой эрозии:

```go
options := WaveErosionOptions{
    StrengthMeters:  50,
    WindSpeed:       12,
    // ...
    LithologyProfile: profile,  // Загруженный профиль
    EnableLithology:  true,     // Включить модуляцию
}

// Внутри waveErodeStep для каждой точки:
if options.EnableLithology && options.LithologyProfile != nil {
    lithology := options.LithologyProfile.GetLithologyAt(lat, lon)
    retreatMeters /= lithology.Resistance  // Чем выше R, тем медленнее эрозия
}
```

**Формула модуляции:**
```
retreatActual = retreatBase / Resistance

где:
  retreatBase   — базовый отступ (м) от энергии волны
  Resistance    — сопротивление породы [0.1-10.0]
  retreatActual — фактический отступ с учётом литологии
```

| Сопротивление | Порода | Эрозия |
|---------------|--------|--------|
| 0.8-1.4 | Очень мягкие (глина, дельта) | Очень быстрая |
| 1.5-2.4 | Мягкие (ил, песок) | Быстрая |
| 2.5-3.9 | Средние (песчаник) | Значительная |
| 4.0-5.9 | Средне-твёрдые (известняк) | Умеренная |
| 6.0-7.9 | Твёрдые (вулканит) | Медленная |
| 8.0-10.0 | Очень твёрдые (серпентинит) | Очень медленная |

### Формат данных

**Пример lithology JSON:**
```json
{
  "metadata": {
    "name": "Black Sea Lithology Profile",
    "version": "1.0",
    "resolution": 0.5,
    "bounds": {"min_lat": 40.0, "max_lat": 47.0, "min_lon": 27.0, "max_lon": 42.0},
    "regions": ["crimea", "turkey", "bulgaria", "romania", "georgia"]
  },
  "points": [
    {
      "lat": 46.2, "lon": 33.0, "region": "crimea",
      "lithology_class": "limestone", "resistance": 4.8,
      "color": "#6b6b6b", "confidence": "high"
    }
  ],
  "classes": {
    "limestone": {
      "resistance": 4.5, "color": "#6b6b6b",
      "description": "Sarmatian limestone"
    },
    "clay": {
      "resistance": 1.2, "color": "#c4a484",
      "description": "Shales and clay"
    }
  }
}
```

### Дефолтный профиль

При отсутствии данных автоматически создаётся профиль для Чёрного моря:

```go
profile := CreateDefaultBlackSeaProfile()
// 5 точек по регионам: Crimea, Turkey, Bulgaria, Romania, Georgia
// 4 класса: limestone, volcanic, clay, sediment
// 6 baseline категорий erosion
```

---

## Батиметрический модуль

### Структура данных

```go
type BathymetryPoint struct {
    Lat   float64 `json:"lat"`   // Широта
    Lon   float64 `json:"lon"`   // Долгота
    Depth float64 `json:"depth"` // Глубина (м, отрицательная)
}

type BathymetryGrid struct {
    Points     map[string]BathymetryPoint // Регулярная сетка
    Resolution float64                    // Размер ячейки (градусы)
    bounds                              // Границы сетки
}
```

### Загрузка и интерполяция

**Загрузка из JSON:**
```go
func LoadBathymetryFromJSON(data []byte, options BathymetryLoadOptions) (*BathymetryGrid, error)
```

Формат JSON — массив точек с `{lat, lon, depth}`. Модуль автоматически строит регулярную сетку с заданным разрешением.

**Билинейная интерполяция:**
```go
func (g *BathymetryGrid) InterpolateDepth(lat, lon float64) (float64, error)
```

Интерполяция использует 4 соседние точки сетки:
```
P00 •------• P01
    |      |
    |  *   |  (*) — запрашиваемая точка
    |      |
P10 •------• P11

depth = (1-t)(1-u)×P00 + t(1-u)×P01 + (1-t)u×P10 + t×u×P11

где t, u — нормированные координаты в ячейке
```

### Формат данных

**Пример батиметрического JSON:**
```json
[
  {"lat": 45.0, "lon": 30.0, "depth": -100},
  {"lat": 45.0, "lon": 30.01, "depth": -150},
  {"lat": 45.01, "lon": 30.0, "depth": -120},
  {"lat": 45.01, "lon": 30.01, "depth": -180}
]
```

- **depth < 0** — подводная глубина (метров ниже уровня моря)
- **depth = 0** — уровень моря
- **depth > 0** — надводная высота (не используется для эрозии)

**Рекомендации по разрешению:**
- **0.01°** (~1.1 км) — для региональных моделей
- **0.001°** (~110 м) — для детальных локальных моделей

Чем выше разрешение, тем точнее интерполяция, но больше данных и медленнее расчёт.

---

## Эрозия

### Модель Гауссовского сдвига

Функции `Erode()` и `ErodeWithSeed()` применяют случайное смещение к каждой точке:

```
Для каждой точки pᵢ = (latᵢ, lonᵢ):

  dx ~ N(0, σ)  // Случайный сдвиг по долготе (метры)
  dy ~ N(0, σ)  // Случайный сдвиг по широте (метры)

  pᵢ' = (latᵢ + dy/metersPerDegLat, lonᵢ + dx/metersPerDegLon)
```

где:
- `σ = strength` — стандартное отклонение в метрах
- `N(0, σ)` — нормальное распределение с матожиданием 0
- `metersPerDegLat = 111194.9` — метров в градусе широты
- `metersPerDegLon = 111194.9 × cos(refLat × π/180)` — метров в градусе долготы

**Конвертация метров в градусы:**

```go
refLat = mean(latᵢ)
metersPerDegLon = 111194.9 × cos(refLat × π/180)

eroded[i] = LatLon{
    Lat: p.Lat + dy / metersPerDegLat,
    Lon: p.Lon + dx / metersPerDegLon,
}
```

**Интерпретация `strength`:**

| `strength` (м) | Эффект |
|-----------------|--------|
| `0` | Без изменений (возвращает копию) |
| `10` | Лёгкий «шум» — ±10 м |
| `100` | Заметная эрозия — ±100 м |
| `500` | Сильная эрозия — ±500 м |
| `1000` | Грубые деформации — ±1 км |

**Статистическая интерпретация:**

```
P(|dx| ≤ σ)  ≈ 68.3%  (1σ)
P(|dx| ≤ 2σ) ≈ 95.4%  (2σ)
P(|dx| ≤ 3σ) ≈ 99.7%  (3σ)
```

### Параллельное выполнение

Функция `erodeParallel()` разбивает точки на чанки для параллельной обработки:

```
┌───────────────┬───────────────┬───────────────┐
│   Chunk 0     │   Chunk 1     │   Chunk 2     │
│ точки 0..511  │ точки 512..   │ точки ...     │
│   горутина 0  │   горутина 1  │   горутина 2  │
└───────────────┴───────────────┴───────────────┘
```

```go
func erodeParallel(points, strength, seed, step) []LatLon:
    chunkSize = 512
    out = make([]LatLon, len(points))
    
    for start = 0; start < len(points); start += chunkSize:
        end = min(start + chunkSize, len(points))
        
        go func():
            for i = start; i < end:
                localSeed = seed + step × 10000 + i
                rng = rand.New(rand.NewSource(localSeed))
                
                dx = rng.NormFloat64() × strength
                dy = rng.NormFloat64() × strength
                
                out[i] = LatLon{
                    Lat: points[i].Lat + dy / metersPerDegLat,
                    Lon: points[i].Lon + dx / metersPerDegLon,
                }
            
        wg.Add(1)
    
    wg.Wait()
    return out
```

### Детерминизм через seed

Для воспроизводимости каждая точка получает уникальный seed, независящий от порядка выполнения горутин:

```
localSeed = seed + step × 10_000 + index

где:
  seed  — базовый seed пользователя
  step  — номер шага эрозии (1, 2, 3, ...)
  index — индекс точки в массиве
```

Это гарантирует, что **точка с индексом i** всегда получит **одинаковый сдвиг** при одинаковых `seed` и `step`, независимо от того, в какой горутине и в каком порядке она обрабатывается.

### Замкнутые полилинии

Для замкнутых полилиний (первая и последняя точки совпадают) необходимо, чтобы они получили одинаковый сдвиг:

```go
if closed && i == 0:
    mu.Lock()
    firstShiftLat = dy
    firstShiftLon = dx
    mu.Unlock()

// ... после завершения всех горутин ...

if closed && len(out) > 1:
    last = len(out) - 1
    out[last] = LatLon{
        Lat: points[last].Lat + firstShiftLat / metersPerDegLat,
        Lon: points[last].Lon + firstShiftLon / metersPerDegLon,
    }
```

Мьютекс нужен, потому что горутина, обрабатывающая первую точку, может выполниться в любом порядке относительно других.

### Многоступенчатая симуляция

```go
func SimulateErosionWithSeed(points, steps, strength, seed) [][]LatLon:
    snapshots = make([][]LatLon, steps + 1)
    
    snapshots[0] = clonePoints(points)  // Начальное состояние
    
    current = snapshots[0]
    for step = 1..steps:
        current = erodeParallel(current, strength, seed, step)
        snapshots[step] = current
    
    return snapshots
```

Каждый шаг эрозии применяется к результату предыдущего, накапливая смещения:

```
s₀ = исходная
s₁ = Erode(s₀)       // Шаг 1: сдвиг от s₀
s₂ = Erode(s₁)       // Шаг 2: сдвиг от s₁
s₃ = Erode(s₂)       // Шаг 3: сдвиг от s₂
```

**Накопленное смещение** растёт как `√step × σ` (случайное блуждание):

```
E[|sₙ - s₀|] ≈ √n × σ
```

---

## Константы и конфигурация

| Константа | Значение | Описание |
|-----------|----------|----------|
| `EarthRadiusKM` | `6371.0` | Средний радиус Земли (км) |
| `metersPerDegLat` | `111194.9` | Метров в одном градусе широты |
| `erosionChunkSize` | `512` | Размер чанка для параллельной эрозии |

**Формула `metersPerDegLat`:**

```
metersPerDegLat = 2π × R / 360 ≈ 111194.9 м

где R = 6371000 м — радиус Земли в метрах
```

---

## Публичный API

### Расстояния и длины

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `Haversine(a, b)` | Расстояние между двумя точками | `float64` (км) |
| `PolylineLength(points)` | Длина ломаной | `float64` (км) |
| `Area(points)` | Площадь полигона | `float64` (км²) |

### Упрощение

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `SimplifyPolyline(points, options)` | Упрощение с целевым числом точек | `SimplifyResult` |

### Волновая эрозия

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `SimulateWaveErosion(points, steps, options)` | Волновая эрозия (случайный seed) | `[][]LatLon` |
| `SimulateWaveErosionWithSeed(points, steps, options, seed)` | Волновая эрозия (детерминированная) | `[][]LatLon` |

### Батиметрия

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `LoadBathymetryFromJSON(data, options)` | Загрузка батиметрии из JSON | `*BathymetryGrid` |
| `BuildGrid(points, resolution)` | Построение сетки из точек | `*BathymetryGrid` |
| `(grid).InterpolateDepth(lat, lon)` | Интерполяция глубины | `float64` (м) |

### Литология

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `LoadLithologyProfile(data)` | Загрузка профиля из JSON | `*LithologyProfile` |
| `LoadLithologyProfileFromFile(path)` | Загрузка профиля из файла | `*LithologyProfile` |
| `(profile).GetLithologyAt(lat, lon)` | Интерполяция литологии (IDW) | `LithologyState` |
| `(profile).GetStatistics()` | Статистика профиля | `map[string]interface{}` |
| `CreateDefaultBlackSeaProfile()` | Дефолтный профиль Чёрного моря | `*LithologyProfile` |

### Временная динамика

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `SimulateErosionWithDuration(points, years, temporalParams, waveOptions)` | Эрозия с временными параметрами (случайный seed) | `TemporalResult` |
| `SimulateErosionWithDurationSeed(points, years, temporalParams, waveOptions, seed)` | Эрозия с временными параметрами (детерминированная) | `TemporalResult` |
| `CalculateErosionMetrics(result)` | Метрики эрозии по шагам | `[]ErosionMetrics` |
| `GetTemporalSummary(result)` | Сводка временной динамики | `map[string]interface{}` |
| `ValidateTemporalParameters(params)` | Валидация параметров | `[]string` (warnings) |

### Транспорт наносов

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)` | Расчёт транспорта наносов | `SedimentTransportResult` |
| `ApplySedimentModification(points, baseErosion, result)` | Коррекция эрозии с учётом аккумуляции | `[]float64` |
| `GetSedimentStatistics(result)` | Статистика транспорта | `map[string]interface{}` |

### Эрозия

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `Erode(points, strength)` | Гауссовская эрозия (случайный seed) | `[]LatLon` |
| `ErodeWithSeed(points, strength, seed)` | Гауссовская эрозия (фиксированный seed) | `[]LatLon` |
| `SimulateErosion(points, steps, strength)` | Многоступенчатая эрозия | `[][]LatLon` |
| `SimulateErosionWithSeed(points, steps, strength, seed)` | Многоступенчатая эрозия (детерминированная) | `[][]LatLon` |

---

## Примеры использования

### Расчёт расстояния и длины

```go
package main

import (
    "coastal-geometry/internal/domain/geometry"
    "fmt"
)

func main() {
    // Расстояние Одесса — Батуми
    odessa := geometry.LatLon{Lat: 46.48, Lon: 30.73}
    batumi := geometry.LatLon{Lat: 41.65, Lon: 41.63}
    
    distance := geometry.Haversine(odessa, batumi)
    fmt.Printf("Расстояние: %.0f км\n", distance) // ~1100 км
    
    // Длина береговой линии
    coast := []geometry.LatLon{
        {Lat: 46.48, Lon: 30.73},
        {Lat: 45.33, Lon: 32.49},
        {Lat: 44.62, Lon: 33.53},
        {Lat: 43.70, Lon: 39.75},
        {Lat: 41.65, Lon: 41.63},
    }
    
    length := geometry.PolylineLength(coast)
    fmt.Printf("Длина: %.0f км\n", length)
}
```

### Упрощение геометрии

```go
func main() {
    // Исходная линия с 1000 точек
    original := loadCoastline()
    
    result := geometry.SimplifyPolyline(original, geometry.SimplifyOptions{
        MaxPoints: 100,
    })
    
    fmt.Printf("Было: %d точек, стало: %d\n", 
        result.OriginalCount, result.SimplifiedCount)
    fmt.Printf("Допуск: %.0f м\n", result.ToleranceMeters)
    fmt.Printf("Применено: %v\n", result.Applied)
}
```

### Эрозия с воспроизводимым seed

```go
func main() {
    coast := loadCoastline()
    
    // Детерминированная эрозия
    eroded := geometry.ErodeWithSeed(coast, 100.0, 42)
    // σ = 100 м, seed = 42
    
    // Многоступенчатая симуляция
    snapshots := geometry.SimulateErosionWithSeed(coast, 10, 50.0, 42)
    // 10 шагов, σ = 50 м на каждом шаге, seed = 42
    
    for step, snap := range snapshots {
        length := geometry.PolylineLength(snap)
        fmt.Printf("Шаг %d: %d точек, длина = %.0f км\n",
            step, len(snap), length)
    }
}
```

### Площадь акватории

```go
func main() {
    // Полигон Чёрного моря (упрощённый)
    polygon := []geometry.LatLon{
        {Lat: 46.5, Lon: 30.5},
        {Lat: 46.5, Lon: 37.0},
        {Lat: 44.0, Lon: 42.0},
        {Lat: 41.0, Lon: 42.0},
        {Lat: 41.0, Lon: 28.0},
        {Lat: 43.0, Lon: 27.5},
    }
    
    area := geometry.Area(polygon)
    fmt.Printf("Площадь: %.0f км²\n", area)
}
```

### Волновая эрозия с батиметрией

```go
func main() {
    coast := loadCoastline()
    
    // Загрузка батиметрии
    bathyData := loadFile("depths.json")
    grid, err := geometry.LoadBathymetryFromJSON(bathyData, geometry.BathymetryLoadOptions{
        Resolution: 0.01, // ~1.1 км
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Параметры волновой эрозии
    options := geometry.WaveErosionOptions{
        StrengthMeters:           50,   // Базовый отступ
        WindSourceDirectionDeg:   0,    // Волны с севера
        WindSpeedMetersPerSecond: 12,   // Скорость ветра
        FetchSpreadDeg:           55,   // Сектор выборки
        FetchSamples:             9,    // Число лучей
        MaxFetchMeters:           150000, // 150 км
        DepthScaleMeters:         4000, // Масштаб глубины
        ExposurePower:            1.5,  // Нелинейность
        BathymetryGrid:           grid, // Батиметрия
    }
    
    // Симуляция
    snapshots := geometry.SimulateWaveErosionWithSeed(coast, 10, options, 42)
    
    for step, snap := range snapshots {
        length := geometry.PolylineLength(snap)
        fmt.Printf("Шаг %d: %d точек, длина = %.0f км\n", step, len(snap), length)
    }
}
```

### Волновая эрозия с литологией

```go
func main() {
    coast := loadCoastline()
    
    // Загрузка литологического профиля
    lithData := loadFile("black-sea-lithology.json")
    profile, err := geometry.LoadLithologyProfile(lithData)
    if err != nil {
        log.Fatal(err)
    }
    
    // Параметры с учётом литологии
    options := geometry.WaveErosionOptions{
        StrengthMeters:           50,
        WindSourceDirectionDeg:   0,
        WindSpeedMetersPerSecond: 12,
        // ...
        LithologyProfile: profile,  // Включить литологию
        EnableLithology:  true,
    }
    
    snapshots := geometry.SimulateWaveErosionWithSeed(coast, 10, options, 42)
    
    // Анализ по литологии
    for i, point := range coast {
        lith := profile.GetLithologyAt(point.Lat, point.Lon)
        fmt.Printf("Точка %d: %s (R=%.1f)\n", i, lith.Class, lith.Resistance)
    }
}
```

**Пример распределения пород Чёрного моря:**

| Регион | Доминирующая порода | Сопротивление | Эрозия |
|--------|---------------------|---------------|--------|
| Крым (юг) | Известняк | 4.5-4.8 | Умеренная |
| Турция (Pontic) | Вулканит/Серпентинит | 6.5-9.0 | Медленная |
| Болгария | Известняк | 4.0-4.2 | Умеренная |
| Румыния (дельта) | Глины/Ил | 0.8-1.5 | Очень быстрая |
| Краснодар | Пески/Ил | 1.0-2.5 | Быстрая |

### Волновая эрозия без батиметрии (геометрический proxy)

```go
func main() {
    coast := loadCoastline()
    
    // Без батиметрии — используется fetch как proxy для глубины
    options := geometry.WaveErosionOptions{
        StrengthMeters:           50,
        WindSourceDirectionDeg:   90,   // Волны с востока
        WindSpeedMetersPerSecond: 14,
        FetchSpreadDeg:           45,
        FetchSamples:             7,
        MaxFetchMeters:           5000,
        DepthScaleMeters:         1000,
        ExposurePower:            1.2,
        BathymetryGrid:           nil,  // Без батиметрии
    }
    
    snapshots := geometry.SimulateWaveErosion(coast, 5, options)
    // ... обработка результатов
}
```

### Временная динамика с учётом климатических факторов

```go
func main() {
    coast := loadCoastline()

    // Временные параметры
    temporalParams := geometry.TemporalParameters{
        YearsPerStep:          1.0,  // 1 год за шаг
        StormProbability:     0.1,  // 10% шанс шторма в год
        StormIntensityMult:   2.5,  // Шторм в 2.5 раз сильнее
        SeaLevelRise:         0.006, // 6 мм/год (RCP4.5)
        Seasonality:          true,  // Учитывать сезонность
        SeasonalPhase:        3.5,   // Пик штормов зимой
    }

    // Волновая эрозия
    waveOptions := geometry.WaveErosionOptions{
        StrengthMeters:         50,
        WindSourceDirectionDeg: 0,
        WindSpeedMetersPerSecond: 12,
        // ... другие параметры
    }

    // Моделирование на 50 лет
    result := geometry.SimulateErosionWithDurationSeed(
        coast, 50, temporalParams, waveOptions, 42,
    )

    fmt.Printf("Промоделировано лет: %.0f\n", result.TotalYears)
    fmt.Printf("Штормовых событий: %d\n", result.StormCount)
    fmt.Printf("Накопленная эрозия: %.0f м\n", result.AccumulatedErosion)
    fmt.Printf("Подъём уровня моря: %.2f м\n", result.FinalSeaLevelRise)

    // Метрики по шагам
    metrics := geometry.CalculateErosionMetrics(result)
    for _, m := range metrics {
        if m.IsStorm {
            fmt.Printf("Шаг %d (шторм): длина=%.0f км\n", m.Step, m.LengthKm)
        }
    }
}
```

### Транспорт наносов и аккумуляция

```go
func main() {
    coast := loadCoastline()

    // Загрузка литологии
    lithData := loadFile("black-sea-lithology.json")
    lithProfile, _ := geometry.LoadLithologyProfile(lithData)

    // Получить литологию для всех точек
    lithology := make([]geometry.LithologyState, len(coast))
    for i, point := range coast {
        lithology[i] = lithProfile.GetLithologyAt(point.Lat, point.Lon)
    }

    // Базовая эрозия от волн (м/шаг)
    erosionRates := []float64{5.0, 4.5, 6.0, 3.5, 2.0} // пример

    // Волновые данные
    waveData := geometry.WaveEnergyData{
        Energy:    []float64{0.7, 0.8, 0.6, 0.9, 0.5},
        Direction: 45.0, // Волны с северо-востока
    }

    // Параметры транспорта
    params := geometry.SedimentTransportParameters{
        TransportCoefficient:      0.7,  // 70% идёт в транспорт
        DepositionRate:            0.5,  // 50% избытка откладывается
        CapacityFactor:            1.2,  // Ёмкость аккумуляции
        LongshoreDriftCoefficient: 0.8, // Сильный alongshore drift
    }

    // Расчёт транспорта
    result := geometry.CalculateSedimentTransport(
        coast, erosionRates, waveData, lithology, params,
    )

    fmt.Printf("Баланс массы: %.2f м³\n", result.TotalBudget.NetChange)
    fmt.Printf("Эрозия точек: %d\n", result.TotalBudget.ErosionPoints)
    fmt.Printf("Аккумуляция точек: %d\n", result.TotalBudget.DepositionPoints)

    // Модифицированная эрозия с учётом аккумуляции
    modifiedErosion := geometry.ApplySedimentModification(
        coast, erosionRates, result,
    )

    for i, modified := range modifiedErosion {
        if modified < erosionRates[i] {
            fmt.Printf("Точка %d: аккумуляция %.2f м\n", i, erosionRates[i]-modified)
        }
    }
}
```

---

## Связанные модули

- [`../coastline`](../coastline) — загрузка, валидация и анализ береговых линий
- [`../fractal`](../fractal) — box-counting анализ фрактальной размерности
- [`../generators/koch`](../generators/koch) — генерация фрактальных кривых Коха