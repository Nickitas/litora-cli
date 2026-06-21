# Геоморфологическое моделирование

Геоморфологическое моделирование представляет собой комплексную физически обоснованную систему симуляции эволюции береговой линии под воздействием волновой эрозии, транспорта наносов, литологических особенностей и временных факторов включая штормовые события, сезонность и климатические изменения.

---

## Содержание

- [Обзор методологии](#обзор-методологии)
- [Волновая эрозия](#волновая-эрозия)
  - [Физическая модель волнового воздействия](#физическая-модель-волнового-воздействия)
  - [Fetch-расстояния и экспозиция берега](#fetch-расстояния-и-экспозиция-берега)
  - [Алгоритм ray casting для fetch](#алгоритм-ray-casting-для-fetch)
  - [Расчёт отступания берега](#расчёт-отступания-берега)
  - [Сглаживание геометрии](#сглаживание-геометрии)
- [Батиметрическая интеграция](#батиметрическая-интеграция)
  - [Загрузка батиметрических данных](#загрузка-батиметрических-данных)
  - [Билинейная интерполяция глубин](#билинейная-интерполяция-глубин)
  - [Физическая глубина и energy factor](#физическая-глубина-и-energy-factor)
  - [Graceful degradation](#graceful-degradation)
- [Литологический модуль](#литологический-модуль)
  - [IDW-интерполяция сопротивления пород](#idw-интерполяция-сопротивления-пород)
  - [Модуляция эрозии по литологии](#модуляция-эрозии-по-литологии)
  - [Профиль Чёрного моря](#профиль-чёрного-моря)
- [Транспорт наносов](#транспорт-наносов)
  - [Баланс массы](#баланс-массы)
  - [Longshore drift](#longshore-drift)
  - [Аккумуляция и эрозия](#аккумуляция-и-эрозия)
  - [Интеграция с волновой эрозией](#интеграция-с-волновой-эрозией)
- [Временная динамика](#временная-динамика)
  - [Штормовые события](#штормовые-события)
  - [Сезонность](#сезонность)
  - [Климатические сценарии](#климатические-сценарии)
  - [Интеграция с волновой эрозией](#интеграция-с-волновой-эрозией-1)

---

## Обзор методологии

**Геоморфологическое моделирование** объединяет несколько физических процессов для реалистичной симуляции эволюции береговой линии:

```
┌─────────────────────────────────────────────────────────────┐
│              ГЕОМОРФОЛОГИЧЕСКАЯ МОДЕЛЬ                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐     │
│  │ Волновая     │   │ Батиметрия   │   │ Литология    │     │
│  │ эрозия       │←→ │ (глубины)    │   │ (породы)     │     │
│  └──────────────┘   └──────────────┘   └──────────────┘     │
│         ↓                    ↓                    ↓         │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐     │
│  │ Транспорт    │   │ Временная    │   │ Баланс       │     │
│  │ наносов      │←→ │ динамика     │   │ массы        │     │
│  └──────────────┘   └──────────────┘   └──────────────┘     │
│         ↓                    ↓                    ↓         │
│  ┌─────────────────────────────────────────────────────┐    │
│  │           ЭВОЛЮЦИЯ БЕРЕГОВОЙ ЛИНИИ                  │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

**Архитектура модулей:**

```
internal/domain/geometry/
├── erosion.go        # Волновая эрозия
├── temporal.go       # Временная динамика
├── sediment.go       # Транспорт наносов
├── lithology.go      # Литологический модуль
└── bathymetry.go     # Батиметрический модуль
```

---

## Волновая эрозия

Волновая эрозия является основным драйвером изменения береговой линии. Модель учитывает направленность волн, расстояние до противоположного берега (fetch), экспозицию сегментов берега и батиметрию.

### Физическая модель волнового воздействия

**Ключевая идея:** Энергия волны пропорциональна квадрату её скорости и зависит от расстояния, которое она проходит (fetch). Чем больше fetch, тем выше энергия волны и тем сильнее эрозия.

**Базовая формула отступания:**

```
retreat = strength × windFactor × fetchFactor × exposure × depthFactor × shapeCorrection
```

где:
- `strength` — базовая сила эрозии (м/шаг)
- `windFactor` — множитель скорости ветра
- `fetchFactor` — множитель расстояния fetch
- `exposure` — экспозиция сегмента к волнению
- `depthFactor` — множитель глубины
- `shapeCorrection` — поправка на форму берега

### Fetch-расстояния и экспозиция берега

**Fetch** — расстояние от точки берега до противоположного берега в заданном направлении. Чем больше fetch, тем выше энергия волн.

**Экспозиция** — функция угла падения волны на береговый сегмент. Сегменты, ориентированные перпендикулярно волнам, получают максимальную экспозицию.

**Структура данных:**

```go
type WaveErosionOptions struct {
    StrengthMeters           float64  // Базовая сила эрозии (м)
    WindSourceDirectionDeg   float64  // Направление ветра (градусы от севера)
    WindSpeedMetersPerSecond float64  // Скорость ветра (м/с)
    FetchSpreadDeg           float64  // Разброс направлений (градусы)
    FetchSamples             int      // Число лучей для fetch
    MaxFetchMeters           float64  // Максимальный fetch (м)
    DepthScaleMeters         float64  // Масштаб глубины (м)
    ExposurePower            float64  // Показатель нелинейности
    BathymetryGrid           *BathymetryGrid // Батиметрия
    LithologyProfile         *LithologyProfile // Литология
    EnableLithology          bool     // Включить литологию
}
```

**Дефолтные значения:**

| Параметр | Дефолт | Описание |
|----------|--------|----------|
| `WindSpeedMetersPerSecond` | 12 м/с | Скорость ветра |
| `FetchSpreadDeg` | 55° | Разброс направлений |
| `FetchSamples` | 9 | Число лучей выборки |
| `MaxFetchMeters` | 150 000 м | Максимальный fetch |
| `DepthScaleMeters` | 4 000 м | Масштаб глубины |
| `ExposurePower` | 1.5 | Нелинейность экспозиции |

### Алгоритм ray casting для fetch

**Ray casting** используется для определения fetch расстояния в заданном направлении:

```go
func rayFetchDistance(projected []pointXY, index int, direction pointXY, 
                     closed bool, probeDistance, maxFetch float64) float64 {
    // Начальная точка луча (с отступом от берега)
    origin := pointXY{
        X: projected[index].X + direction.X * probeDistance,
        Y: projected[index].Y + direction.Y * probeDistance,
    }
    
    limit := maxFetch - probeDistance
    best := limit
    
    // Перебор всех сегментов берега
    for segmentIndex := 0; segmentIndex < segmentCount; segmentIndex++ {
        if segmentTouchesVertex(segmentIndex, index, len(projected), closed) {
            continue // Пропускаем смежные сегменты
        }
        
        // Проверка пересечения луча с сегментом
        a := projected[segmentIndex]
        b := projected[(segmentIndex + 1) % len(projected)]
        
        distance, ok := raySegmentDistance(origin, direction, a, b)
        if ok && distance < best {
            best = distance // Найдено более близкое пересечение
        }
    }
    
    return probeDistance + best
}
```

**Пересечение луча с сегментом:**

```go
func raySegmentDistance(origin, direction, a, b pointXY) (float64, bool) {
    segment := {X: b.X - a.X, Y: b.Y - a.Y}
    denominator := cross(direction, segment)
    
    if abs(denominator) < 1e-9 {
        return 0, false // Параллельны
    }
    
    delta := {X: a.X - origin.X, Y: a.Y - origin.Y}
    t := cross(delta, segment) / denominator // расстояние до пересечения
    u := cross(delta, direction) / denominator // позиция на сегменте
    
    if t <= 1e-6 || u < -1e-6 || u > 1 + 1e-6 {
        return 0, false // Пересечение вне сегмента
    }
    
    return t, true
}
```

**Визуализация:**

```
                ┌─────────────────────┐
                │  Противоположный    │
                │  берег              │
                └─────────────────────┘
                     ↖
                      ↖   *пересечение*
                       ↖
                        ↖  ← ray
                         ↖
                          ● точка берега
```

### Расчёт отступания берега

**Экспозиция сегмента:**

Для каждой точки берега определяются два направления (левая и правая нормаль), затем выбирается seaward нормаль (направленная в сторону моря).

```go
// Касательный вектор
tangent := normalizeXY({
    X: next.X - prev.X,
    Y: next.Y - prev.Y,
})

// Нормали
leftNormal := {X: -tangent.Y, Y: tangent.X}
rightNormal := {X: tangent.Y, Y: -tangent.X}

// Выбор seaward нормали
seawardNormal := leftNormal
if dot(rightNormal, mainDirection) > dot(leftNormal, mainDirection) {
    seawardNormal = rightNormal // Больше экспозиция к волнам
}
```

**Расчёт экспозиции:**

```go
func sampleWaveSide(projected, index, normal, mainDirection, options) {
    weightedFetch := 0.0
    weightSum := 0.0
    
    // Выборка по сектору направлений
    for sample := 0; sample < options.FetchSamples; sample++ {
        direction := sampleWaveDirection(mainDirection, options.FetchSpreadDeg, 
                                        sample, options.FetchSamples)
        incidence := dot(normal, direction)
        
        if incidence <= 0 {
            continue // Волна не падает на этот сегмент
        }
        
        weight := pow(incidence, options.ExposurePower)
        fetch := rayFetchDistance(projected, index, direction, ...)
        
        weightedFetch += fetch * weight
        weightSum += weight
    }
    
    meanFetch := weightedFetch / weightSum
    exposure := clamp(weightSum / float64(options.FetchSamples), 0, 1)
    
    return {MeanFetch: meanFetch, Exposure: exposure}
}
```

**Множители:**

```go
// Ветровой множитель
windFactor := pow(windSpeed / 12.0, 2)
windFactor := clamp(windFactor, 0.1, 4.0)

// Fetch множитель
fetchFactor := sqrt(clamp(meanFetch / maxFetch, 0, 1))

// Глубинный множитель
depthFactor := 1 - exp(-normalFetch / depthScale)

// Итоговый score
score := fetchFactor * exposure * (0.35 + 0.65 * depthFactor)
```

**Поправка на форму берега:**

```go
// Вы protrusion (насколько выступает мыс)
shapeDelta := {
    X: (prev.X + next.X) / 2 - current.X,
    Y: (prev.Y + next.Y) / 2 - current.Y,
}
localScale := 0.5 * (distance(current, prev) + distance(current, next))

protrusion := clamp(-dot(shapeDelta, seawardNormal) / localScale, 0, 1.5)
bayShelter := clamp(dot(shapeDelta, seawardNormal) / localScale, 0, 1.2)

// Мысы эродируют быстрее, бухты — медленнее
shapeCorrection := clamp(0.55 + protrusion - 0.35 * bayShelter, 0.1, 1.75)
```

**Итоговое отступание:**

```go
retreatMeters := strength * windFactor * score * shapeCorrection

// Ограничение максимума
if maxRetreat > 0 {
    retreatMeters = min(retreatMeters, maxRetreat)
}

// Модуляция по литологии
if enableLithology && lithology != nil {
    retreatMeters /= lithology.Resistance
}
```

### Сглаживание геометрии

Для предотвращения Arteфakтов и поддержания гладкости береговой линии применяется сглаживание:

```go
// Коэффициент сглаживания
smoothingAlpha := min(retreatMeters / localScale, 0.5)

// Новая позиция с учётом сглаживания
out[i] = {
    X: current.X - seawardNormal.X * retreatMeters + shapeDelta.X * smoothingAlpha,
    Y: current.Y - seawardNormal.Y * retreatMeters + shapeDelta.Y * smoothingAlpha,
}
```

**Интерпретация:**
- `smoothingAlpha` → 0: минимальное сглаживание, берег становится более rugged
- `smoothingAlpha` → 0.5: умеренное сглаживание, баланс между детализацией и гладкостью

---

## Батиметрическая интеграция

Батиметрия (глубина моря) играет критическую роль в волновой эрозии: чем глубже вода у берега, тем больше энергии волна может передать берегу.

### Загрузка батиметрических данных

**Источник:** GEBCO (General Bathymetric Chart of the Oceans) — глобальный батиметрический датасет с разрешением 30 arc seconds.

**Структура данных:**

```go
type BathymetryPoint struct {
    Lat   float64 `json:"lat"`
    Lon   float64 `json:"lon"`
    Depth float64 `json:"depth"` // глубина в метрах (отрицательная)
}

type BathymetryGrid struct {
    Points     map[string]BathymetryPoint // Регулярная сетка
    Resolution float64                     // Размер ячейки (градусы)
    bounds                                 // Границы сетки
}
```

**Формат JSON:**

```json
[
  {"lat": 45.0, "lon": 30.0, "depth": -100},
  {"lat": 45.0, "lon": 30.01, "depth": -150},
  {"lat": 45.01, "lon": 30.0, "depth": -120}
]
```

- `depth < 0` — подводная глубина (метры ниже уровня моря)
- `depth = 0` — уровень моря
- `depth > 0` — надводная высота (не используется для эрозии)

### Билинейная интерполяция глубин

Для получения глубины в произвольной точке используется билинейная интерполяция:

```
P00 •------• P01
    |      |
    |  *   |  (*) — запрашиваемая точка
    |      |
P10 •------• P11

depth = (1-t)(1-u)×P00 + t(1-u)×P01 + (1-t)u×P10 + t×u×P11

где t, u — нормированные координаты в ячейке
```

**Алгоритм:**

```go
func (grid *BathymetryGrid) InterpolateDepth(lat, lon float64) (float64, error) {
    // Найти ячейку
    cellLat := math.Floor((lat - grid.minLat) / grid.Resolution)
    cellLon := math.Floor((lon - grid.minLon) / grid.Resolution)
    
    // Координаты углов ячейки
    P00 := grid.getPoint(cellLat, cellLon)
    P01 := grid.getPoint(cellLat, cellLon + 1)
    P10 := grid.getPoint(cellLat + 1, cellLon)
    P11 := grid.getPoint(cellLat + 1, cellLon + 1)
    
    // Нормированные координаты
    t := (lat - P00.Lat) / grid.Resolution
    u := (lon - P00.Lon) / grid.Resolution
    
    // Билинейная интерполяция
    depth := (1-t)*(1-u)*P00.Depth + t*(1-u)*P01.Depth + 
             (1-t)*u*P10.Depth + t*u*P11.Depth
    
    return depth, nil
}
```

### Физическая глубина и energy factor

**Физический принцип:** Чем глубже вода у берега, тем больше энергии волна может передать берегу, тем сильнее эрозия.

**Геометрический proxy (без батиметрии):**

```go
depthFactor = 1 - exp(-fetch / depthScale)
```

Здесь `fetch` используется как proxy для глубины: чем больше fetch, тем глубже вода предположительно.

**Физическая модель (с батиметрией):**

```go
effectiveDepth = max(0, -depthMeters) // Глубина в метрах (положительная)
depthFactor = 1 - exp(-effectiveDepth / depthScale)
```

**Интерпретация `depthScale`:**
- `depthScale = 4000 м` — характерная глубина Чёрного моря
- Малые `depthScale` → быстрое насыщение, глубокая вода не увеличивает эрозию сильно
- Большие `depthScale` → линейная зависимость, глубина важна

### Graceful degradation

**Обработка отсутствия батиметрии:**

```go
var depthFactor float64
if options.BathymetryGrid != nil {
    depth, err := options.BathymetryGrid.InterpolateDepth(lat, lon)
    if err == nil {
        depthFactor = physicalDepthFactor(depth, normalFetch, options.DepthScaleMeters)
    } else {
        // Graceful degradation: логируем предупреждение, но продолжаем
        // Используем geometric proxy как fallback
        depthFactor = 1 - math.Exp(-normalFetch / options.DepthScaleMeters)
    }
} else {
    // Без батиметрии — используется fetch как proxy
    depthFactor = 1 - math.Exp(-normalFetch / options.DepthScaleMeters)
}
```

**CLI интеграция:**

```go
// Автоматическая загрузка батиметрии
bathymetryPath := app.Config.BathymetryPath
if bathymetryPath == "" {
    if _, err := os.Stat(defaultBathymetryFile); err == nil {
        bathymetryPath = defaultBathymetryFile
        fmt.Println("✓ Батиметрия загружена:", defaultBathymetryFile)
    } else {
        fmt.Println("⚠️ Батиметрия не найдена, используем geometric proxy...")
        bathymetryPath = ""
    }
}
```

---

## Литологический модуль

Литологический модуль учитывает различную сопротивляемость горных пород эрозии, что существенно влияет на скорость изменения береговой линии в разных регионах.

### IDW-интерполяция сопротивления пород

**Inverse Distance Weighting (IDW)** используется для интерполяции сопротивления пород между точками замера:

```
Для запрашиваемой точки (lat, lon):
  1. Найти N ближайших точек (по умолчанию N=6)
  2. Рассчитать веса:  wᵢ = 1 / distance²
  3. Нормировать:      Wᵢ = wᵢ / Σwⱼ
  4. Интерполировать:  R = Σ(Wᵢ × Resistanceᵢ)
  5. Класс и цвет — от точки с максимальным весом
```

**Алгоритм:**

```go
func (profile *LithologyProfile) GetLithologyAt(lat, lon float64) LithologyState {
    // Найти ближайшие точки
    nearest := profile.findNearestPoints(lat, lon, 6)
    
    // Рассчитать веса
    weights := make([]float64, len(nearest))
    totalWeight := 0.0
    
    for i, point := range nearest {
        distance := haversineDistance(lat, lon, point.Lat, point.Lon)
        weights[i] = 1.0 / (distance * distance)
        totalWeight += weights[i]
    }
    
    // Нормировать веса
    for i := range weights {
        weights[i] /= totalWeight
    }
    
    // Интерполировать сопротивление
    resistance := 0.0
    maxWeightIndex := 0
    
    for i, point := range nearest {
        resistance += weights[i] * point.Resistance
        if weights[i] > weights[maxWeightIndex] {
            maxWeightIndex = i
        }
    }
    
    // Класс и цвет от точки с максимальным весом
    class := nearest[maxWeightIndex].Lithology
    color := profile.Classes[class].Color
    
    return LithologyState{
        Class:       class,
        Resistance:  resistance,
        Color:       color,
        Description: profile.Classes[class].Description,
    }
}
```

### Модуляция эрозии по литологии

**Физический принцип:** Чем выше сопротивление породы, тем медленнее она erodes.

**Формула модуляции:**

```go
retreatActual = retreatBase / Resistance

где:
  retreatBase  — базовый отступ (м) от энергии волны
  Resistance   — сопротивление породы [0.1-10.0]
  retreatActual — фактический отступ с учётом литологии
```

**Применение в волновой эрозии:**

```go
// Внутри waveErodeStep для каждой точки
if options.EnableLithology && options.LithologyProfile != nil {
    lithology := options.LithologyProfile.GetLithologyAt(lat, lon)
    retreatMeters /= lithology.Resistance
}
```

**Таблица сопротивлений:**

| Сопротивление | Порода | Эрозия | Примеры |
|---------------|--------|--------|---------|
| 0.8-1.4 | Очень мягкие | Очень быстрая | Глины, дельтовые отложения |
| 1.5-2.4 | Мягкие | Быстрая | Ил, песок |
| 2.5-3.9 | Средние | Значительная | Песчаник |
| 4.0-5.9 | Средне-твёрдые | Умеренная | Известняк |
| 6.0-7.9 | Твёрдые | Медленная | Вулканит |
| 8.0-10.0 | Очень твёрдые | Очень медленная | Серпентинит |

### Профиль Чёрного моря

**Дефолтный профиль создаётся автоматически при отсутствии данных:**

```go
func CreateDefaultBlackSeaProfile() *LithologyProfile {
    return &LithologyProfile{
        Metadata: LithologyMetadata{
            Name:       "Black Sea Default Profile",
            Version:    "1.0",
            Resolution: 0.5,
        },
        Points: []LithologyPoint{
            // Крым (юг) — известняк
            {Lat: 44.5, Lon: 34.0, Region: "crimea", Lithology: "limestone", Resistance: 4.8},
            
            // Турция (Pontic) — вулканит
            {Lat: 41.5, Lon: 37.5, Region: "turkey", Lithology: "volcanic", Resistance: 7.2},
            
            // Болгария — известняк
            {Lat: 43.0, Lon: 28.0, Region: "bulgaria", Lithology: "limestone", Resistance: 4.2},
            
            // Румыния (дельта) — глины
            {Lat: 45.0, Lon: 30.0, Region: "romania", Lithology: "clay", Resistance: 1.2},
            
            // Краснодар — пески
            {Lat: 44.0, Lon: 38.0, Region: "kuban", Lithology: "sediment", Resistance: 2.0},
        },
        Classes: map[string]LithologyClass{
            "limestone": {Resistance: 4.5, Color: "#6b6b6b", Description: "Sarmatian limestone"},
            "volcanic":  {Resistance: 7.0, Color: "#3b3b3b", Description: "Pontic volcanics"},
            "clay":      {Resistance: 1.2, Color: "#c4a484", Description: "Delta clays"},
            "sediment":  {Resistance: 2.0, Color: "#e8d8a8", Description: "Coastal sands"},
        },
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

---

## Транспорт наносов

Транспорт наносов (sediment transport) моделирует перемещение материала вдоль берега под действием волн, аккумуляцию в защищённых зонах и баланс массы.

### Баланс массы

**Система отслеживает баланс массы для каждого участка берега:**

```go
type SedimentBudget struct {
    ErodedVolume    float64 // объём размытого материала (м³/м)
    TransportVolume float64 // объём в транзите (longshore drift)
    DepositedVolume float64 // объём отложенного материала
    NetChange       float64 // баланс (eroded - deposited)
    ErosionPoints    int     // число точек с эрозией
    DepositionPoints int    // число точек с аккумуляцией
}
```

**Принцип сохранения массы:**

```
Eroded = Transport + LocalDeposition
NetChange = Eroded - Deposited
```

Для всей системы сумма `NetChange` должна быть близка к 0 (допуск 15%).

### Longshore drift

**Longshore drift** — транспорт наносов вдоль берега под действием волн.

```go
type SedimentTransportParameters struct {
    TransportCoefficient      float64 // [0-1] часть в транспорт
    DepositionRate            float64 // [0-1] скорость отложения
    MinimumFlowVelocity      float64 // Минимальная скорость для транспорта (м/с)
    CapacityFactor            float64 // [0-2] ёмкость аккумуляции
    LongshoreDriftCoefficient float64 // [0-1] alongshore транспорт
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

**Логика аккумуляции:**

```go
incomingTotal = sum(InTransitFrom)
localCapacity = CapacityFactor × waveEnergy

if incomingTotal > localCapacity OR waveEnergy < 0.3:
    // Избыток наносов или низкая энергия → аккумуляция
    excess = incomingTotal - localCapacity
    deposition = excess × DepositionRate
    IsAccumulating = true
else:
    // Высокая энергия → эрозия
    IsEroding = true

NetChange = ErodedVolume - DepositedVolume
```

**Интерпретация:**
- Защищённые бухты (`waveEnergy < 0.3`) → аккумуляция независимо от incoming
- Высокая энергия волн → низкая ёмкость → эрозия
- Избыток наносов → отложение (accretion)

### Интеграция с волновой эрозией

**Применение транспорта наносов:**

```go
func CalculateSedimentTransport(points, erosionRates, waveData, lithology, params) {
    // 1. Рассчитать объём эрозии
    calculateErosionVolumes(states, erosionRates, lithology, params)
    
    // 2. Longshore drift
    calculateLongshoreDrift(states, points, waveData, params)
    
    // 3. Депозиция
    calculateDeposition(states, waveData, params)
    
    // 4. Баланс массы
    result := summarizeSedimentTransport(states, erosionRates, params)
    
    return result
}
```

**Модификация эрозии с учётом аккумуляции:**

```go
func ApplySedimentModification(points, baseErosion, sedimentResult) []float64 {
    modified := make([]float64, len(baseErosion))
    
    for i := range modified {
        state := sedimentResult.States[i]
        
        if state.IsAccumulating {
            // В точках аккумуляции уменьшаем эрозию
            depositedVolume := state.LocalBudget.DepositedVolume
            modified[i] = baseErosion[i] - depositedVolume
            
            if modified[i] < 0 {
                modified[i] = 0 // Аккумуляция превышает эрозию
            }
        } else {
            modified[i] = baseErosion[i]
        }
    }
    
    return modified
}
```

**Валидация:** баланс массы должен сохраняться (допуск 15%).

```go
if abs(result.MassBalance) > 0.15 {
    result.Warnings = append(result.Warnings, 
        "Mass balance violation: more than 15% deviation")
}
```

---

## Временная динамика

Временная динамика моделирует изменение береговой линии на протяжении длительных периодов (десятилетия) с учётом нестационарных факторов: штормов, сезонности, подъёма уровня моря.

### Штормовые события

**Вероятностная модель:** На каждом шаге моделирования проверяется вероятность шторма:

```go
if rng.Float64() < StormProbability:
    IsStorm = true
    StormIntensity = StormIntensityMult + variation
```

**Интенсивность шторма:** базовый множитель + случайная вариация (±50%).

**Таблица частот:**

| Вероятность | Частота штормов | Пример климата |
|-------------|-----------------|----------------|
| 0.0 | Никогда | Идеально спокойный |
| 0.05 | 1 раз в 20 лет | Умеренный |
| 0.1 | 1 раз в 10 лет | Штормовой |
| 0.2+ | Частые штормы | Экстремальный |

### Сезонность

**Синусоидальная модель:** Учитывает годовые колебания волновой активности:

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

### Климатические сценарии

**Подъём уровня моря:** Линейная модель:

```go
seaLevelOffset = year × SeaLevelRise
```

**Модуляция эрозии:** Чем выше уровень моря, тем сильнее эрозия:

```go
seaLevelFactor = 1.0 + 0.1 × ln(1 + seaLevelOffset)
modulatedErosion = baseErosion × seaLevelFactor
```

Логарифмическая зависимость отражает убывающую эффективность: каждые дополнительные 10 м подъёма дают меньший эффект.

**IPCC сценарии:**

| Сценарий | Подъём (м/год) | Через 50 лет |
|----------|----------------|--------------|
| RCP2.6 | 0.003 | +0.15 м |
| RCP4.5 | 0.006 | +0.30 м |
| RCP8.5 | 0.010 | +0.50 м |

### Интеграция с волновой эрозией

**Временная модуляция параметров:**

```go
func SimulateErosionWithDurationSeed(points, targetYears, temporalParams, waveOptions, seed) {
    numSteps := ceil(targetYears / temporalParams.YearsPerStep)
    
    for step := 1; step <= numSteps; step++ {
        // Рассчитать временное состояние
        state := calculateTemporalState(step, temporalParams, rng)
        
        // Модулированная сила эрозии
        modulatedStrength := applyTemporalModulation(waveOptions.StrengthMeters, state)
        
        waveOptions.StrengthMeters = modulatedStrength
        
        // Шаг эрозии
        current = waveErodeStep(current, waveOptions, seed, step)
        snapshots[step] = current
    }
    
    return TemporalResult{Snapshots: snapshots, TemporalStates: states}
}
```

**Формула модуляции:**

```go
func applyTemporalModulation(baseErosion, state) float64 {
    modulated := baseErosion
    
    // Штормовая модуляция
    if state.IsStorm:
        modulated *= state.StormIntensity
    
    // Сезонная модуляция
    modulated *= state.SeasonalFactor
    
    // Модуляция от подъёма уровня моря
    if state.SeaLevelOffset > 0:
        seaLevelFactor = 1.0 + 0.1 × ln(1 + state.SeaLevelOffset)
        modulated *= seaLevelFactor
    
    return modulated
}
```

**Метрики по шагам:**

```go
type ErosionMetrics struct {
    Step                int      // номер шага
    Year                float64  // год
    LengthKm            float64  // длина береговой линии (км)
    AreaKm2             float64  // площадь (км²)
    ErodedM3            float64  // объём эрозии (м³)
    DepositedM3         float64  // объём депозиции (м³)
    NetChangeM3         float64  // баланс (м³)
    FractalDimension    float64  // фрактальная размерность
    MeanRetreatMeters   float64  // среднее отступание (м)
    MaxRetreatMeters    float64  // максимальное отступание (м)
    IsStorm             bool     // штормовое событие
    SeasonalFactor      float64  // сезонный множитель
}
```

**Пример симуляции:**

```
┌──────┬──────────┬───────────┬───────────┬─────────────┐
│ Шаг  │ Год      │ Точек     │ Длина км  │ Площадь км² │
├──────┼──────────┼───────────┼───────────┼─────────────┤
│ 0    │ 0.0      │ 847       │ 4235      │ 587432      │
│ 1    │ 1.0      │ 847       │ 4232      │ 587120      │
│ 2    │ 2.0      │ 847       │ 4228      │ 586547      │ ⛈️ шторм
│ 3    │ 3.0      │ 847       │ 4221      │ 585432      │
│ 4    │ 4.0      │ 847       │ 4215      │ 584521      │
│ 5    │ 5.0      │ 847       │ 4209      │ 583654      │
└──────┴──────────┴───────────┴───────────┴─────────────┘

📊 Статистика временной динамики:
   • Промоделировано лет: 5.0 из 5 (целевых)
   • Шагов моделирования: 5
   • Штормовых событий: 1 (частота 0.20)
   • Подъём уровня моря: 0.03 м
   • Накопленная эрозия: 26.0 м
   • Изменение длины берега: -26.0 км (-0.6%)
```

---

## Практическое применение

### CLI интеграция

**Автоматическая загрузка данных:**

```go
// Батиметрия
if _, err := os.Stat(defaultBathymetryFile); err == nil {
    bathymetryGrid = loadBathymetry(defaultBathymetryFile)
    fmt.Println("✓ Батиметрия загружена")
} else {
    fmt.Println("⚠️ Батиметрия не найдена, используем geometric proxy")
}

// Литология
if enableLithology {
    lithologyProfile = loadLithology(defaultLithologyFile)
    fmt.Println("✓ Литология включена")
}
```

### Настройка параметров

**Рекомендуемые диапазоны:**

| Параметр | Минимум | Дефолт | Максимум | Описание |
|----------|---------|--------|----------|----------|
| `StrengthMeters` | 5 | 30 | 100 | Базовая сила эрозии |
| `WindSpeed` | 3 | 12 | 25 | Скорость ветра (м/с) |
| `FetchSpread` | 15 | 55 | 120 | Разброс направлений (градусы) |
| `FetchSamples` | 3 | 9 | 21 | Число лучей |
| `MaxFetchMeters` | 10 000 | 150 000 | 500 000 | Максимальный fetch (м) |
| `DepthScale` | 500 | 4 000 | 15 000 | Масштаб глубины (м) |
| `ExposurePower` | 1.0 | 1.5 | 3.0 | Нелинейность |

**Валидация параметров:**

```go
warnings := ValidateTemporalParameters(params)
if len(warnings) > 0 {
    fmt.Println("⚠ Предупреждения:")
    for _, warning := range warnings {
        fmt.Printf("  • %s\n", warning)
    }
}
```

### Интерпретация результатов

**Что влияет на скорость эрозии:**

1. **Скорость ветра:** Квадратичная зависимость (`wind²`)
2. **Fetch расстояние:** Корневая зависимость (`√fetch`)
3. **Экспозиция:** Степенная зависимость (`incidence^power`)
4. **Глубина:** Экспоненциальное насыщение (`1 - exp(-depth/scale)`)
5. **Литология:** Обратная зависимость (`1/resistance`)
6. **Форма берега:** Мысы эродируют быстрее, бухты — медленнее

**Диагностика проблем:**

| Симптом | Возможная причина | Решение |
|---------|-------------------|---------|
| Слишком быстрая эрозия | Высокая скорость ветра, низкое сопротивление пород | Уменьшить `WindSpeed`, проверить литологию |
| Недостаточная детализация | Малый `StrengthMeters`, высокая аккумуляция | Увеличить силу, уменьшить `CapacityFactor` |
| Arteфакты на границах | Проблемы с замкнутостью полилинии | Проверить топологию |
| Несохранение массы | Ошибки в расчёте транспорта | Проверить баланс массы |

Геоморфологическое моделирование предоставляет комплексную и физически обоснованную систему для моделирования эволюции береговой линии, подходящую для широкого спектра исследований и практических приложений.
