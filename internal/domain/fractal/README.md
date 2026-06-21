# Package `fractal`

**Модуль расчёта фрактальной размерности методом box-counting с усреднением по сеткам и адаптивной регрессией.**

Модуль анализирует эмпирическую фрактальную размерность произвольной береговой линии, используя метод подсчёта ячеек (box-counting) с пониженной чувствительностью к положению решётки и произвольному выбору масштабов.

---

## Содержание

- [Архитектура модуля](#архитектура-модуля)
- [Основные типы данных](#основные-типы-данных)
- [Метод box-counting](#метод-box-counting)
  - [Математическая основа](#математическая-основа)
  - [Проекция координат](#проекция-координат)
  - [Усреднение по смещениям сетки](#усреднение-по-смещениям-сетки)
  - [Алгоритм покрытия сегмента](#алгоритм-покрытия-сегмента)
- [Адаптивная регрессия](#адаптивная-регрессия)
  - [Логарифмические координаты](#логарифмические-координаты)
  - [Поиск оптимального окна](#поиск-оптимального-окна)
  - [Критерии стабильности](#критерии-стабильности)
  - [Локальные размерности](#локальные-размерности)
- [Полный алгоритм](#полный-алгоритм)
- [Константы и конфигурация](#константы-и-конфигурация)
- [Примеры использования](#примеры-использования)
- [Обработка ошибок](#обработка-ошибок)
- [Связанные модули](#связанные-модули)

---

## Архитектура модуля

```
internal/domain/fractal/
└── dimension.go          # Box-counting анализ
```

Зависимости:
- `internal/domain/geometry` — `LatLon`, `Haversine`
- `internal/domain/generators/koch` — генерация тестовых кривых Коха

---

## Основные типы данных

### `Point2D`

Точка в декартовой системе координат (метры):

```go
type Point2D struct {
    X, Y float64 // Координаты в метрах относительно опорной точки
}
```

### `BoxCountingSample`

Одна точка данных для анализа box-counting:

```go
type BoxCountingSample struct {
    ScaleFactor   float64 // Масштабный фактор (bboxSize / boxSize)
    RelativeScale float64 // Относительный размер ячейки (boxSize / bboxSize)
    BoxSizeMeters float64 // Физический размер ячейки в метрах
    BoxesCovered  int     // Среднее число покрытых ячеек (усреднено по смещениям)
    LogInvScale   float64 // ln(1 / relativeScale) — ось X для регрессии
    LogBoxes      float64 // ln(boxesCovered) — ось Y для регрессии
}
```

### `BoxCountingAnalysis`

Полный результат анализа фрактальной размерности:

```go
type BoxCountingAnalysis struct {
    Dimension          float64  // Эмпирическая фрактальная размерность D
    RegressionRSquared float64  // R² линейной регрессии (качество аппроксимации)
    StableAcrossScales bool     // Стабильна ли D across масштабов
    StabilitySpread    float64  // Разброс локальных размерностей (max - min)
    Samples            []BoxCountingSample  // Все измеренные точки
    LocalDimensions    []float64 // Локальные наклоны между соседними точками
    Valid              bool      // Можно ли доверять результату
}
```

### Публичные функции

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `FractalDimension(points)` | Быстрый расчёт D | `float64` (1.0 если невалидно) |
| `AnalyzeBoxCounting(points)` | Полный анализ с диагностикой | `BoxCountingAnalysis` |

---

## Метод box-counting

### Математическая основа

Фрактальная размерность `D` определяется через степенной закон:

```
N(ε) ∝ ε^(-D)
```

где:
- `N(ε)` — число ячеек размера `ε`, необходимых для покрытия кривой
- `ε` — размер ячейки (scale)

Логарифмируя:

```
ln(N(ε)) = D × ln(1/ε) + C

D = d[ln(N)] / d[ln(1/ε)]
```

Для идеальной фрактальной кривой `D` постоянна. Для природных кривых `D` варьируется в зависимости от масштаба.

**Интерпретация:**
- `D = 1.0` — гладкая линия (евклидова)
- `D ≈ 1.26` — кривая Коха (теоретическое значение)
- `D ≈ 1.15–1.25` — типичные береговые линии
- `D → 2.0` — кривая заполняет плоскость

### Проекция координат

Географические координаты проецируются в декартову систему (метры) относительно опорной точки Чёрного моря:

```go
func latLonToMeters(p LatLon) Point2D {
    refLat = 43.5°           // Опорная широта (центр Чёрного моря)
    refLon = 35.0°           // Опорная долгота

    metersPerDegLat = 111194.9
    metersPerDegLon = 87300.0  // ~111194.9 × cos(43.5°)

    return Point2D{
        X: (p.Lon - refLon) × metersPerDegLon,
        Y: (p.Lat - refLat) × metersPerDegLat,
    }
}
```

**Bounding box** вычисляется как min/max по всем точкам:

```
width  = maxX - minX
height = maxY - minY
bboxSize = max(width, height)
```

### Масштабный ряд

Используется плотный набор масштабных факторов для снижения чувствительности к выбору диапазона регрессии:

```go
var defaultScaleFactors = []float64{
    4, 6, 8, 12, 16, 24, 32, 48, 64, 96, 128, 192, 256,
}
```

Для каждого фактора `s`:

```
boxSize = bboxSize / s
relativeScale = 1 / s = boxSize / bboxSize
```

Диапазон: от `bboxSize/256` (мелкие ячейки) до `bboxSize/4` (крупные).

### Усреднение по смещениям сетки

Для снижения чувствительности к положению решётки используется 4 смещения:

```go
var gridOffsets = [][2]float64{
    {0, 0},     // Базовая сетка
    {0.5, 0},   // Сдвиг по X на пол-ячейки
    {0, 0.5},   // Сдвиг по Y на пол-ячейки
    {0.5, 0.5}, // Сдвиг по обоим осям
}
```

Для каждого смещения считается число покрытых ячеек, затем усредняется:

```
N_avg(ε) = Σ N(ε, offsetᵢ) / 4,  i = 0..3
```

Это уменьшает артефакты, возникающие при неудачном положении сетки относительно кривой.

### Алгоритм покрытия сегмента

Для каждого отрезка `(a, b)` определяется множество ячеек, которые он пересекает:

```go
func markSegmentBoxesOffset(covered, a, b, boxSize, minX, minY, offsetX, offsetY):
    dx = b.X - a.X
    dy = b.Y - a.Y
    distance = √(dx² + dy²)
    
    // Число шагов дискретизации (полу-ячейка для надёжности)
    steps = ceil(distance / (boxSize/2)) + 1
    steps = max(steps, 2)
    
    for i = 0..steps:
        t = i / steps
        x = a.X + dx × t
        y = a.Y + dy × t
        
        row = floor((y - minY + offsetX × boxSize) / boxSize)
        col = floor((x - minX + offsetY × boxSize) / boxSize)
        
        covered[(row, col)] = {}
```

**Ключевые моменты:**
- Дискретизация с шагом `boxSize/2` гарантирует, что ни одна ячейка не будет пропущена
- Использование `map[(row, col)]struct{}` автоматически устраняет дубликаты
- Смещение `offsetX × boxSize`, `offsetY × boxSize` реализует сдвиг сетки

---

## Адаптивная регрессия

### Логарифмические координаты

Данные преобразуются для линейной регрессии:

```
X = ln(1 / relativeScale) = ln(s)
Y = ln(N(s))
```

Наклон регрессии `Y = a × X + b` даёт оценку размерности: `D = a`.

### Поиск оптимального окна

Вместо использования всех точек модуль ищет **наилучшее окно регрессии** перебором:

```go
func bestRegressionWindow(x, y []float64) *regressionWindow:
    n = len(x)
    
    for start = 0..(n - minScaleSamples):
        for end = (start + minScaleSamples - 1)..(n - 1):
            xs = x[start..end+1]
            ys = y[start..end+1]
            
            slope, intercept = linearRegression(xs, ys)
            r² = regressionRSquared(xs, ys, slope, intercept)
            localDims = localSlopeSeries(xs, ys)
            spread = valueSpread(localDims)
            stable = len(localDims) ≥ 3 ∧ r² ≥ 0.98 ∧ spread ≤ 0.18
            
            candidate = {start, end, length, slope, r², spread, stable}
            
            if betterWindow(best, candidate):
                best = candidate
    
    return best
```

### Критерии выбора окна

Функция `betterWindow()` сравнивает кандидатов по приоритетам:

```
1. Стабильность: стабильное окно > нестабильное
2. Длина: длинное окно > короткое (больше данных)
3. R²: более высокое R² > низкое
4. Наклон: больший наклон > меньший (при близких R²)
5. Разброс: меньший spread > больший
```

**Критерии стабильности:**
```
stable ⟺ len(localDimensions) ≥ 3 
       ∧ r² ≥ 0.98 
       ∧ spread ≤ 0.18
```

### Линейная регрессия

Метод наименьших квадратов:

```
n × Σ(xy) - Σx × Σy
slope = ─────────────────
        n × Σ(x²) - (Σx)²

         Σy - slope × Σx
intercept = ─────────────
                 n
```

**Коэффициент детерминации R²:**

```
         Σ(yᵢ - ŷᵢ)²
R² = 1 - ───────────
         Σ(yᵢ - ȳ)²

где ŷᵢ = slope × xᵢ + intercept
      ȳ = mean(y)
```

- `R² = 1.0` — идеальная линейная зависимость
- `R² < 0.98` — результат может быть ненадёжным

### Локальные размерности

Наклон между каждой парой соседних точек в окне:

```go
func localSlopeSeries(x, y []float64) []float64:
    slopes = []
    for i = 1..len(x)-1:
        if |x[i] - x[i-1]| < 1e-12:
            continue  // пропускаем вырожденные
        slopes.append((y[i] - y[i-1]) / (x[i] - x[i-1]))
    return slopes
```

Разброс локальных размерностей:

```
spread = max(localDimensions) - min(localDimensions)
```

Малый spread (< 0.18) означает, что размерность стабильна across масштабов.

---

## Полный алгоритм

```
AnalyzeBoxCounting(points []LatLon) → BoxCountingAnalysis
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

1. Валидация:
   if len(points) < 2 → return {}

2. Проекция:
   meters[] = [latLonToMeters(p) for p in points]
   minX, maxX, minY, maxY = bboxMeters(meters)
   bboxSize = max(maxX - minX, maxY - minY)
   if bboxSize < 1 → return {}

3. Измерение box-counting:
   samples = []
   for factor in defaultScaleFactors:
       boxSize = bboxSize / factor
       boxes = boxesCoveredMetersAverage(meters, boxSize, minX, minY, gridOffsets)
       if boxes ≤ 1 → continue
       
       samples.append(BoxCountingSample{
           ScaleFactor:   factor,
           RelativeScale: boxSize / bboxSize,
           BoxSizeMeters: boxSize,
           BoxesCovered:  round(boxes),
           LogInvScale:   ln(1 / relativeScale),
           LogBoxes:      ln(boxes),
       })
   
   if len(samples) < 4 → return {Samples: samples}

4. Поиск окна регрессии:
   window = bestRegressionWindow(logInvScale[], logBoxes[])
   if window == nil || window.length < 4 → return {Samples: samples}

5. Диагностика:
   localDimensions = localSlopeSeries(window.x, window.y)
   spread = valueSpread(localDimensions)
   stable = len(localDimensions) ≥ 3 
          ∧ window.r² ≥ 0.98 
          ∧ spread ≤ 0.18

6. Валидация наклона:
   if window.slope < 0.5 || window.slope > 3.0:
       return {Samples, LocalDimensions, r², spread}  // Valid = false

7. Результат:
   return BoxCountingAnalysis{
       Dimension:          window.slope,
       RegressionRSquared: window.rSquared,
       StableAcrossScales: stable,
       StabilitySpread:    spread,
       Samples:            samples,
       LocalDimensions:    localDimensions,
       Valid:              true,
   }
```

**Сложность:**
- Box-counting: `O(n × m × k)`, где `n` — число точек, `m` — число масштабных факторов (13), `k` — число смещений сетки (4)
- Регрессия: `O(s³)`, где `s` — число samples (до 13) — пренебрежимо мало
- Итого: линейная по числу точек входной кривой

---

## Константы и конфигурация

| Константа | Значение | Описание |
|-----------|----------|----------|
| `minScaleSamples` | `4` | Мин. число точек в окне регрессии |
| `minStableLocalSlopes` | `3` | Мин. число локальных наклонов для стабильности |
| `minRegressionRSquared` | `0.98` | Мин. R² для признания стабильным |
| `maxLocalSlopeSpread` | `0.18` | Макс. разброс локальных размерностей |
| `defaultScaleFactors` | `[4, 6, 8, 12, 16, 24, 32, 48, 64, 96, 128, 192, 256]` | Набор масштабных факторов (13 штук) |
| `gridOffsets` | `[(0,0), (0.5,0), (0,0.5), (0.5,0.5)]` | Смещения сетки для усреднения |

**Опорные координаты проекции:**
| Параметр | Значение | Описание |
|----------|----------|----------|
| `refLat` | `43.5°` | Опорная широта (центр Чёрного моря) |
| `refLon` | `35.0°` | Опорная долгота |
| `metersPerDegLat` | `111194.9` | Метров в градусе широты |
| `metersPerDegLon` | `87300.0` | Метров в градусе долготы на 43.5° |

---

## Примеры использования

### Быстрый расчёт размерности

```go
package main

import (
    "coastal-geometry/internal/domain/coastline"
    "coastal-geometry/internal/domain/fractal"
)

func main() {
    points, _, err := coastline.LoadFromJSON("data/black-sea.json")
    if err != nil {
        panic(err)
    }
    
    d := fractal.FractalDimension(points)
    fmt.Printf("Фрактальная размерность: %.4f\n", d)
}
```

### Полный анализ с диагностикой

```go
func main() {
    points, _, err := coastline.LoadFromJSON("data/black-sea.json")
    if err != nil {
        panic(err)
    }
    
    analysis := fractal.AnalyzeBoxCounting(points)
    
    fmt.Printf("D = %.4f\n", analysis.Dimension)
    fmt.Printf("R² = %.4f\n", analysis.RegressionRSquared)
    fmt.Printf("Стабильна: %v\n", analysis.StableAcrossScales)
    fmt.Printf("Разброс: %.4f\n", analysis.StabilitySpread)
    fmt.Printf("Валидна: %v\n", analysis.Valid)
    
    // Локальные размерности между соседними масштабами
    fmt.Println("\nЛокальные размерности:")
    for i, ld := range analysis.LocalDimensions {
        fmt.Printf("  %d: %.4f\n", i, ld)
    }
    
    // Все измеренные точки
    fmt.Println("\nМасштабные точки:")
    for _, s := range analysis.Samples {
        fmt.Printf("  s=%d: box=%.0fм, boxes=%d, ln(1/ε)=%.2f, ln(N)=%.2f\n",
            s.ScaleFactor, s.BoxSizeMeters, s.BoxesCovered,
            s.LogInvScale, s.LogBoxes)
    }
}
```

### Сравнение с теоретическим значением Коха

```go
import (
    "coastal-geometry/internal/domain/generators/koch"
    "coastal-geometry/internal/domain/fractal"
    "math"
)

func main() {
    base := []geometry.LatLon{
        {Lat: 0, Lon: 0},
        {Lat: 0, Lon: 0.2},
    }
    
    curve := koch.KochCurve(base, 5)
    analysis := fractal.AnalyzeBoxCounting(curve)
    theoretical := math.Log(4) / math.Log(3) // ≈ 1.26186
    
    fmt.Printf("Теоретическая D (Кох): %.5f\n", theoretical)
    fmt.Printf("Измеренная D:          %.5f\n", analysis.Dimension)
    fmt.Printf("Отклонение:            %.5f\n", 
        math.Abs(analysis.Dimension - theoretical))
}
```

---

## Обработка ошибок

Модуль **никогда не возвращает ошибку**. Вместо этого:

| Ситуация | Результат |
|----------|-----------|
| `< 2 точек` | `BoxCountingAnalysis{}` (пустой, `Valid = false`) |
| `bboxSize < 1 м` | `BoxCountingAnalysis{}` (пустой) |
| `< 4 масштабных точек` | `{Samples: samples}` (недостаточно данных) |
| Не найдено стабильное окно | `{Samples: samples}` (без размерности) |
| Наклон вне `[0.5, 3.0]` | `{Samples, LocalDimensions, r², spread}` (нереалистичное значение) |

Для быстрого получения размерности:

```go
d := fractal.FractalDimension(points)
// Если невалидно → возвращает 1.0 (евклидова размерность)
```

---

## Научная интерпретация

### Что означает `StableAcrossScales`

| `StableAcrossScales` | Значение |
|----------------------|----------|
| `true` | Размерность стабильна across всего выбранного диапазона масштабов — результат надёжен |
| `false` | Размерность зависит от масштаба — результат следует интерпретировать с осторожностью |

### Что означает `StabilitySpread`

| Spread | Интерпретация |
|--------|---------------|
| `< 0.10` | Отличная стабильность — D почти не зависит от масштаба |
| `0.10–0.18` | Хорошая стабильность — D умеренно зависит от масштаба |
| `> 0.18` | Низкая стабильность — D сильно варьируется, результат может быть артефактом |

### Сравнение с известными значениями

| Кривая | Теоретическая D | Ожидаемый диапазон |
|--------|-----------------|--------------------|
| Прямая линия | 1.0000 | 1.0–1.1 |
| Береговая линия (типичная) | — | 1.10–1.30 |
| Кривая Коха | 1.2619 | 1.20–1.35 |
| Кривая Гильберта | 2.0000 | 1.90–2.00 |

---

## Связанные модули

- [`../geometry`](../geometry) — `LatLon`, `Haversine`, `PolylineLength`
- [`../generators/koch`](../generators/koch) — генерация кривых Коха для тестирования
- [`../coastline`](../coastline) — загрузка и валидация береговых линий
- [`../render`](../render) — визуализация результатов box-counting в SVG