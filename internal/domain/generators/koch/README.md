# Package `koch`

**Генераторы фрактальных кривых: классическая и органическая кривая Коха поверх произвольной базовой полилинии.**

Модуль реализует рекурсивное построение фрактальных аппроксимаций, преобразующих каждый сегмент базовой линии в набор новых сегментов по правилам Коха. Органическая версия добавляет стохастический шум для имитации природных форм.

---

## Содержание

- [Архитектура модуля](#архитектура-модуля)
- [Классическая кривая Коха](#классическая-кривая-коха)
  - [Математическая основа](#математическая-основа)
  - [Алгоритм разбиения сегмента](#алгоритм-разбиения-сегмента)
  - [Рекурсивная структура](#рекурсивная-структура)
  - [Теоретическая проверка](#теоретическая-проверка)
- [Органическая кривая Коха](#органическая-кривая-коха)
  - [Стохастическая модель](#стохастическая-модель)
  - [Алгоритм органического разбиения](#алгоритм-органического-разбиения)
  - [Влияние параметров](#влияние-параметров)
- [Константы и конфигурация](#константы-и-конфигурация)
- [Публичный API](#публичный-api)
- [Примеры использования](#примеры-использования)
- [Тестирование](#тестирование)
- [Обработка ошибок](#обработка-ошибок)
- [Научная интерпретация](#научная-интерпретация)
- [Связанные модули](#связанные-модули)

---

## Архитектура модуля

```
internal/domain/generators/koch/
├── koch.go           # Классическая кривая Коха + теоретическая проверка
├── organic.go        # Органическая кривая Коха со стохастическим шумом
└── koch_test.go      # Тесты корректности реализации
```

Зависимости:
- `internal/domain/geometry` — `LatLon`, `PolylineLength`

---

## Классическая кривая Коха

### Математическая основа

Кривая Коха — классический пример математического фрактала. На каждой итерации каждый прямолинейный сегмент заменяется на 4 сегмента, каждый в 3 раза короче исходного:

```
Исходный сегмент:     A──────────B

После 1 итерации:     A────P────P────B
                            ╱  ╲
                           ╱    ╲
                          ╱______╲
                            P
```

**Свойства:**
- Число сегментов растёт: `N(n) = 4ⁿ × N₀`
- Длина каждого сегмента: `l(n) = l₀ / 3ⁿ`
- Общая длина: `L(n) = L₀ × (4/3)ⁿ`
- При `n → ∞`: длина стремится к бесконечности, но кривая остаётся в ограниченной области

**Фрактальная размерность:**

```
D = log(4) / log(3) ≈ 1.26186
```

Вывод: каждый сегмент заменяется на 4 новых, каждый в 3 раза меньше. Размерность Хаусдорфа:

```
D = log(N) / log(1/r) = log(4) / log(3)
```

где `N = 4` — число новых сегментов, `r = 1/3` — коэффициент уменьшения.

### Алгоритм разбиения сегмента

Функция `kochSegment(a, b)` создаёт 4 точки из одного сегмента:

```go
func kochSegment(a, b LatLon) []LatLon {
    // Вектор сегмента
    vx = b.Lon - a.Lon
    vy = b.Lat - a.Lat

    // Точка на 1/3 от начала
    thirdX = vx / 3.0
    thirdY = vy / 3.0

    p1 = (a.Lon + thirdX, a.Lat + thirdY)  // 1/3

    // Точка на 2/3 от начала
    p3 = (a.Lon + 2*thirdX, a.Lat + 2*thirdY)  // 2/3

    // Вершина равностороннего треугольника (поворот на 60°)
    dx = thirdX
    dy = thirdY
    
    cos60 = 0.5
    sin60 = √3 / 2
    
    // Вектор от p1, повёрнутый на 60° против часовой стрелки
    p2x = dx * cos60 - dy * sin60
    p2y = dx * sin60 + dy * cos60
    
    p2 = (p1.Lon + p2x, p1.Lat + p2y)  // Вершина

    return [a, p1, p2, p3]
}
```

**Геометрическая интерпретация:**

```
     p2
    /  \
   /    \
 p1──────p3
 /        \
a          b
```

1. `p1` — точка на 1/3 расстояния от `a` к `b`
2. `p3` — точка на 2/3 расстояния от `a` к `b`
3. `p2` — вершина равностороннего треугольника, построенного на отрезке `[p1, p3]`

**Поворот вектора на 60°:**

```
[vx', vy'] = [vx·cos(60°) - vy·sin(60°), vx·sin(60°) + vy·cos(60°)]

cos(60°) = 1/2
sin(60°) = √3/2
```

### Рекурсивная структура

```go
func KochCurve(base []LatLon, iterations int) []LatLon:
    if iterations < 0 → iterations = 0
    if iterations > MaxIterations (10):
        warning, iterations = 10
    
    if iterations == 0 → copy(base)
    
    return kochRecursive(base, iterations)

func kochRecursive(points, depth):
    if depth == 1 → kochIteration(points)
    return kochIteration(kochRecursive(points, depth - 1))
```

**Полная итерация** применяет `kochSegment` ко каждому сегменту:

```go
func kochIteration(points []LatLon) []LatLon:
    newPoints = []
    for i = 0..len(points)-2:
        newPoints.append(kochSegment(points[i], points[i+1]))
    newPoints.append(points[last])  // Замыкающая точка
    return newPoints
```

**Рост числа точек:**

| Итерация `n` | Число точек | Длина относительно L₀ |
|--------------|-------------|------------------------|
| 0 | N₀ | 1.000× |
| 1 | 3N₀ - 2 | 1.333× |
| 2 | 9N₀ - 8 | 1.778× |
| 3 | 27N₀ - 26 | 2.370× |
| 4 | 81N₀ - 80 | 3.160× |
| 5 | 243N₀ - 242 | 4.214× |
| n | N₀ × 3ⁿ - (3ⁿ - 1) | (4/3)ⁿ |

### Теоретическая проверка

Функция `CheckTheoryConsistency()` проверяет корректность реализации:

```go
func CheckTheoryConsistency(base []LatLon, maxIterations int) TheoryCheckReport:
    baseLength = PolylineLength(base)
    report = {Valid: true}
    
    for iter = 0..maxIterations:
        curve = KochCurve(base, iter)
        measured = PolylineLength(curve)
        theoretical = baseLength × (4/3)^iter
        
        error = |measured - theoretical|
        errorPct = error / theoretical × 100
        
        report.Samples.append({
            Iteration: iter,
            PointsCount: len(curve),
            MeasuredLengthKM: measured,
            TheoreticalKM: theoretical,
            ErrorKM: error,
            ErrorPercent: errorPct,
        })
        
        if errorPct > 2.0%:
            report.Valid = false
    
    return report
```

**Порог:** `maxTheoryErrorPct = 2.0%`

Если ошибка превышает 2% на любой итерации — реализация считается некорректной.

**Теоретическая длина:**

```
L(n) = L₀ × (4/3)ⁿ

где L₀ — длина базовой полилинии
      n — номер итерации
```

---

## Органическая кривая Коха

### Стохастическая модель

Органическая кривая Коха модифицирует классический алгоритм, добавляя случайный шум к двум параметрам:

1. **Угол отклонения** — вместо фиксированных 60° используется `60° ± jitter`
2. **Высота треугольника** — вместо единичного коэффициента используется `1.0 ± jitter`

```go
type OrganicOptions struct {
    Seed            int64   // Seed для воспроизводимости
    AngleJitterDeg  float64 // Макс. отклонение угла в градусах (±)
    HeightJitterPct float64 // Макс. отклонение высоты в долях (±)
}
```

### Алгоритм органического разбиения

```go
func organicKochSegment(a, b, rng, opts) []LatLon:
    // Те же p1, p3 что и в классическом Кохе
    thirdX = vx / 3.0
    thirdY = vy / 3.0
    p1 = (a.Lon + thirdX, a.Lat + thirdY)
    p3 = (a.Lon + 2*thirdX, a.Lat + 2*thirdY)

    // Случайный угол: 60° ± AngleJitterDeg
    angle = (60.0 + randomSigned(rng, opts.AngleJitterDeg)) × π/180

    // Случайный масштаб высоты: 1.0 ± HeightJitterPct
    heightScale = 1.0 + randomSigned(rng, opts.HeightJitterPct)

    // Поворот с случайным углом и масштабом
    dx = thirdX, dy = thirdY
    rotX = dx × cos(angle) - dy × sin(angle)
    rotY = dx × sin(angle) + dy × cos(angle)

    p2 = (p1.Lon + rotX × heightScale, p1.Lat + rotY × heightScale)

    return [a, p1, p2, p3]
```

**Генерация случайного отклонения:**

```go
func randomSigned(rng, amplitude) float64:
    if amplitude <= 0 → return 0
    return (rng.Float64() × 2 - 1) × amplitude
```

Равномерное распределение в диапазоне `[-amplitude, +amplitude]`.

### Влияние параметров

#### `AngleJitterDeg`

| Значение | Эффект |
|----------|--------|
| `0°` | Идеальные 60°, классический Кох |
| `±5°` | Лёгкие отклонения, угол ∈ [55°, 65°] |
| `±18°` | Заметные искажения, угол ∈ [42°, 78°] |
| `±30°` | Сильные искажения, угол ∈ [30°, 90°] |
| `±60°` | Хаотичные направления, угол ∈ [0°, 120°] |

#### `HeightJitterPct`

| Значение | Эффект |
|----------|--------|
| `0.0` | Идеальная высота 1.0 |
| `±0.1` | Высота ∈ [0.9, 1.1] — лёгкие неровности |
| `±0.25` | Высота ∈ [0.75, 1.25] — заметная органичность |
| `±0.5` | Высота ∈ [0.5, 1.5] — сильные вариации |
| `±1.0` | Высота ∈ [0.0, 2.0] — может схлопываться или удваиваться |

#### Типовые значения для природных береговых линий

```go
OrganicKochCurve(base, iterations, OrganicOptions{
    Seed:            42,       // Воспроизводимый seed
    AngleJitterDeg:  18.0,     // ±18° отклонение угла
    HeightJitterPct: 0.25,     // ±25% высоты
})
```

**Свойства органической кривой:**
- ❌ Не обладает строгой самоподобностью
- ✅ Выглядит ближе к природной береговой линии
- ✅ Фрактальная размерность варьируется (не фиксирована)
- ✅ Воспроизводима при фиксированном seed

---

## Константы и конфигурация

| Константа | Значение | Описание |
|-----------|----------|----------|
| `MaxIterations` | `10` | Максимальное число итераций (ограничение из-за экспоненциального роста) |
| `maxTheoryErrorPct` | `2.0` | Макс. допустимая ошибка в % от теории |

**Пороговые значения для предупреждений:**
- При `iterations > 10` → автоматическое ограничение до 10 + warning
- При `errorPct > 2.0%` → предупреждение о несоответствии теории

---

## Публичный API

### Классический Кох

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `KochCurve(base, iterations)` | Построение кривой Коха | `[]LatLon` |
| `TheoreticalLength(baseLength, iterations)` | Расчёт теоретической длины | `float64` |
| `TheoryError(measured, theoretical)` | Абсолютная ошибка | `float64` |
| `TheoryErrorPercent(measured, theoretical)` | Ошибка в процентах | `float64` |
| `CheckTheoryConsistency(base, maxIter)` | Проверка корректности | `TheoryCheckReport` |
| `Demonstrate(base, maxIter)` | Консольная демонстрация | `TheoryCheckReport` |

### Органический Кох

| Функция | Описание | Возвращает |
|---------|----------|------------|
| `OrganicKochCurve(base, iterations, opts)` | Органическая кривая Коха | `[]LatLon` |
| `DemonstrateOrganic(base, maxIter, opts)` | Консольная демонстрация | `void` |

### Типы данных

```go
type OrganicOptions struct {
    Seed            int64   // Seed для rand.Rand (0 = не используется)
    AngleJitterDeg  float64 // Макс. отклонение угла в градусах
    HeightJitterPct float64 // Макс. отклонение высоты в долях
}

type TheoryCheckSample struct {
    Iteration        int     // Номер итерации
    PointsCount      int     // Число точек после итерации
    MeasuredLengthKM float64 // Измеренная длина
    TheoreticalKM    float64 // Теоретическая длина
    ErrorKM          float64 // Абсолютная ошибка
    ErrorPercent     float64 // Ошибка в %
}

type TheoryCheckReport struct {
    Samples []TheoryCheckSample
    Valid   bool  // Все ошибки ≤ 2%
}
```

---

## Примеры использования

### Классическая кривая Коха

```go
package main

import (
    "coastal-geometry/internal/domain/coastline"
    "coastal-geometry/internal/domain/generators/koch"
)

func main() {
    points, _, err := coastline.LoadFromJSON("data/black-sea.json")
    if err != nil {
        panic(err)
    }
    
    // Построить кривую Коха с 4 итерациями
    curve := koch.KochCurve(points, 4)
    
    fmt.Printf("Исходных точек: %d\n", len(points))
    fmt.Printf("Точек после 4 итераций: %d\n", len(curve))
    
    // Проверить теоретическую согласованность
    report := koch.CheckTheoryConsistency(points, 4)
    fmt.Printf("Валидно: %v\n", report.Valid)
    
    for _, sample := range report.Samples {
        fmt.Printf("Итерация %d: %d точек, длина=%.0f км, теория=%.0f км, ошибка=%.2f%%\n",
            sample.Iteration, sample.PointsCount,
            sample.MeasuredLengthKM, sample.TheoreticalKM,
            sample.ErrorPercent)
    }
}
```

### Органическая кривая Коха

```go
func main() {
    points, _, err := coastline.LoadFromJSON("data/black-sea.json")
    if err != nil {
        panic(err)
    }
    
    curve := koch.OrganicKochCurve(points, 4, koch.OrganicOptions{
        Seed:            42,
        AngleJitterDeg:  18.0,
        HeightJitterPct: 0.25,
    })
    
    fmt.Printf("Исходных точек: %d\n", len(points))
    fmt.Printf("Точек после organic-Кох: %d\n", len(curve))
}
```

### Серия итераций для визуализации

```go
func main() {
    points, _, _ := coastline.LoadFromJSON("data/black-sea.json")
    
    for iter := 0; iter <= 5; iter++ {
        curve := koch.KochCurve(points, iter)
        length := geometry.PolylineLength(curve)
        theory := koch.TheoreticalLength(geometry.PolylineLength(points), iter)
        
        fmt.Printf("Iter %d: %6d точек, %.0f км (теория: %.0f км)\n",
            iter, len(curve), length, theory)
    }
}
```

### Полная демонстрация

```go
func main() {
    points, _, _ := coastline.LoadFromJSON("data/black-sea.json")
    
    // Классический Кох с таблицей
    report := koch.Demonstrate(points, 5)
    
    // Органический Кох
    koch.DemonstrateOrganic(points, 5, koch.OrganicOptions{
        Seed:            42,
        AngleJitterDeg:  18.0,
        HeightJitterPct: 0.25,
    })
}
```

---

## Тестирование

Запуск:

```bash
go test ./internal/domain/generators/koch/...
```

### Покрытие тестами

| Тест | Что проверяется |
|------|-----------------|
| `TestTheoreticalLength` | ✅ `L₀ × (4/3)² = 90 × 16/9 = 160` |
| `TestTheoryErrorPercent` | ✅ `\|98 - 100\| / 100 × 100 = 2.0%` |
| `TestKochCurveMatchesTheoryForSingleSegment` | ✅ Ошибка реализации ≤ 2% для одной итерации на одном сегменте |

---

## Обработка ошибок

Модуль **не возвращает ошибки**. Вместо этого:

| Ситуация | Поведение |
|----------|-----------|
| `iterations < 0` | Автоматически → 0 |
| `iterations > 10` | Warning в stdout, автоматически → 10 |
| `< 2 точек в base` | Возвращает копию входных данных (для Коха) или пустой (для органического) |
| `errorPct > 2%` | `TheoryCheckReport.Valid = false` |
| `AngleJitterDeg ≤ 0` | Классический угол 60° без шума |
| `HeightJitterPct ≤ 0` | Классическая высота 1.0 без шума |

---

## Научная интерпретация

### Парадокс береговой линии

Кривая Коха наглядно демонстрирует парадокс:

```
lim L(n) = ∞  при n → ∞
n→∞
```

Но кривая остаётся в ограниченной области плоскости. Это показывает, что **длина береговой линии зависит от масштаба измерения**.

### Фрактальная размерность

| Кривая | D | Интерпретация |
|--------|---|---------------|
| Прямая | 1.0 | Евклидова, не фрактал |
| Кох классический | log(4)/log(3) ≈ 1.2619 | Идеальный фрактал |
| Кох органический | ~1.15–1.35 | Приближено к природе |
| Реальная береговая линия | ~1.10–1.30 | Природный фрактал |

### Органическая модель

Органический Кох нарушает строгую самоподобность, но создаёт более реалистичные формы:

- **Классический Кох:** идеальная симметрия, неестественно «правильные» формы
- **Органический Кох:** стохастические вариации, ближе к реальным береговым линиям
- **Реальная береговая линия:** результат эрозии, геологии, биологии — ещё менее регулярна

---

## Связанные модули

- [`../geometry`](../geometry) — `LatLon`, `PolylineLength`, `SimplifyPolyline`
- [`../fractal`](../fractal) — расчёт фрактальной размерности box-counting
- [`../coastline`](../coastline) — загрузка и валидация береговых линий
- [`../../cmd/lito`](../../cmd/lito) — CLI-интерфейс, вызывающий генераторы
