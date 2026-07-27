# Litora-CLI | Моделирование эрозии прибрежных систем

**CLI-утилита для геоморфологического моделирования береговых систем, анализа фрактальных свойств береговой линии и физически обоснованного моделирования эрозионных процессов**

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go)](https://golang.org/)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nickitas/litora-cli)](https://goreportcard.com/report/github.com/Nickitas/litora-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/Nickitas/litora-cli/ci.yml?branch=main)](https://github.com/Nickitas/litora-cli/actions)

> Проект разрабатывается в рамках диссертационного исследования  
> **«Построение геометрических образов прибрежных систем»**  
> (на примере Черного моря)

## Содержание

- [Научная цель](#-научная-цель)
- [Научные возможности](#-научные-возможности)
- [CLI](#-cli)
- [Установка и запуск](#-установка-и-запуск)
- [Научные сценарии использования](#-научные-сценарии-использования)
- [Выходные данные](#-выходные-данные)
  - [SVG-отчёты](#svg-отчёты)
  - [Метрики JSON](#-метрики-json)
  - [CSV-отчёты](#csv-отчёты)
  - [GIF-анимация](#gif-анимация)
- [Методы аппроксимации и интерполяции](#-методы-аппроксимации-и-интерполяции)
- [Научные задачи](#-научные-задачи)
- [Области применения](#-области-применения)
- [Лицензия](#-лицензия)

---

## Научная цель

Разработка и верификация комплексной математико-алгоритмической модели геометрического образа береговой линии, сочетающей фрактальные свойства и динамику изменения под воздействием эрозионных процессов.

**Ключевые задачи:**
1. Валидация геометрии входных береговых линий с проверкой топологической корректности
2. Геодезически корректное измерение длины с учётом кривизны Земли
3. Анализ фрактальных свойств методом box-counting с повышенной статистической устойчивостью
4. Геоморфологическое моделирование волновой эрозии с учётом:
   - fetch-расстояний и экспозиции берега
   - реальной батиметрии (GEBCO данные)
   - литологического состава пород
   - транспорт наносов и баланса массы
   - временной динамики (штормы, сезонность, климатические сценарии)
5. Визуализация результатов с автоматическим созданием GIF-анимаций эрозионных процессов

---

## Возможности

### Базовый анализ
- **Валидация геометрии:** проверка топологической корректности, замкнутости контуров, пересечений сегментов
- **Геодезический расчёт длины:** учёт кривизны Земли через формулу Haversine
- **Метрики качества:** детальная диагностика геометрических проблем

### Фрактальный анализ
- **Box-counting размерность:** с усреднением по нескольким смещениям сетки
- **Статистическая устойчивость:** адаптивный выбор диапазона масштабов
- **Диагностика сходимости:** анализ изменения размерности по итерациям

### Геоморфологическое моделирование
- **Волновая эрозия:** направленность волн, fetch-расстояния, экспозиция берега
- **Батиметрическая интеграция:** загрузка GEBCO данных, билинейная интерполяция
- **Литологический модуль:** IDW-интерполяция сопротивления пород
- **Транспорт наносов:** баланс массы, longshore drift, аккумуляция
- **Временная динамика:** штормовые события, сезонность, климатические сценарии (RCP4.5, RCP8.5)

### Визуализация и экспорт
- **GIF-анимация:** автоматическое создание анимированной визуализации эрозионных процессов
- **Научные элементы:** масштабная линейка, временные метки, метрики кадра, географические метки
- **Цветовая кодировка:** дифференциация эрозии, аккумуляции и стабильных зон
- **Настройка качества:** баланс между размером файла и качеством изображения

### Метрики качества модели

**Научная валидация результатов моделирования**

Для обеспечения научной достоверности результатов система автоматически рассчитывает метрики качества модели на каждом шаге симуляции:

#### Основные метрики

**ModelQualityMetrics** - комплексная оценка качества модели:

```go
type ModelQualityMetrics struct {
    DimensionStability float64  // стабильность фрактальной размерности D во времени [0-1]
    MassBalance        float64  // баланс массы (eroded - deposited) с допуском 15%
    SpatialAutocorr    float64  // пространственная автокорреляция [-1, 1]
    ConvergenceRate    float64  // скорость сходимости модели [0-1]
}
```

#### Критерии валидности

Модель считается **научно валидной**, если выполняются все условия:

1. **DimensionStability > 0.7** — фрактальная размерность стабильна во времени
2. **|MassBalance| < 0.15** — баланс массы сохраняется (допуск 15%)
3. **-0.3 ≤ SpatialAutocorr ≤ 0.8** — разумные пространственные паттерны
4. **ConvergenceRate > 0.5** — модель сходится (изменения замедляются)

#### Интерпретация метрик

**DimensionStability** — стабильность фрактальной размерности D:
- `1.0` — идеальная стабильность (размерность не меняется)
- `0.7+` — хорошая стабильность (допустимые вариации)
- `0.5-0.7` — умеренная стабильность (требует внимания)
- `< 0.5` — низкая стабильность (нестабильная геометрия)

**MassBalance** — баланс массы (фундаментальный закон сохранения):
- `0.0` — идеальный баланс массы
- `< 0.15` — допустимый баланс (15% допуск)
- `≥ 0.15` — нарушение баланса массы (ошибки в алгоритме)

**SpatialAutocorr** — пространственная автокорреляция (Moran's I):
- `0.8-1.0` — сильная кластеризация (однородные участки)
- `0.3-0.8` — умеренная корреляция (нормальный паттерн)
- `-0.3-0.3` — слабая корреляция (случайные значения)
- `-1.0 - -0.3` — чередование (structured pattern)

**ConvergenceRate** — скорость сходимости модели:
- `1.0` — полная сходимость (изменения прекратились)
- `0.7+` — хорошая сходимость (изменения замедляются)
- `0.5-0.7` — умеренная сходимость
- `< 0.5` — отсутствие сходимости (модель diverges)

#### Использование в CLI

Метрики качества рассчитываются **автоматически** при выполнении команд `model erosion` и `all`:

```bash
# Метрики качества выводятся автоматически
./lito model erosion --steps 10 --erosion-strength 50
```

**Пример вывода:**

```
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  МЕТРИКИ КАЧЕСТВА МОДЕЛИ
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  ✓ Dimension Stability: 0.87 (стабильная геометрия)
  ✓ Mass Balance: 0.03 (сохранение массы)
  ✓ Spatial Autocorr: 0.42 (нормальный паттерн)
  ✓ Convergence Rate: 0.73 (модель сходится)

  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  ИТОГОВАЯ ОЦЕНКА: МОДЕЛЬ НАУЧНО ВАЛИДНА ✓
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**При проблемах с качеством:**

```
  ⚠️  МЕТРИКИ КАЧЕСТВА МОДЕЛИ
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  ✗ Dimension Stability: 0.52 (нестабильная геометрия)
  ⚠️  Mass Balance: 0.18 (нарушение баланса массы)
  ✓ Spatial Autocorr: 0.45 (нормальный паттерн)
  ✗ Convergence Rate: 0.31 (модель не сходится)

  ПРЕДУПРЕЖДЕНИЯ:
  • Low dimension stability: 0.52 (expected > 0.7)
  • Poor mass balance: 0.1800 (expected |balance| < 0.15)
  • Low convergence rate: 0.31 (model may not converge)

  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  ИТОГОВАЯ ОЦЕНКА: МОДЕЛЬ ТРЕБУЕТ ДОРАБОТКИ ✗
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

#### Экспорт метрик качества

Метрики качества автоматически включаются в JSON-отчёты:

```json
{
  "model_quality": {
    "dimension_stability": 0.87,
    "mass_balance": 0.03,
    "spatial_autocorr": 0.42,
    "convergence_rate": 0.73,
    "dimension_variance": 0.01,
    "mass_balance_trend": 0.1,
    "spatial_correlation_morans_i": 0.3,
    "is_valid_model": true,
    "warnings": []
  }
}
```

#### Научная значимость

Метрики качества обеспечивают:

- **Верификацию модели** — проверка соответствия физическим законам
- **Статистическую устойчивость** — достоверность численных результатов
- **Прогностическую способность** — надёжность будущих проекций
- **Сравнимость сценариев** — объективная оценка различных моделей

#### References

- Mandelbrot, B.B. (1982): *The Fractal Geometry of Nature*
- CERC (1984): *Shore Protection Manual*
- Komar, P.D. (1998): *Beach Processes and Sedimentation*

---

### Методы аппроксимации

#### TIN (Triangulated Irregular Network)

**Более точная аппроксимация сложной береговой линии**

```go
type ApproximationMesh struct {
    Type             string  // "regular" | "tin" | "adaptive"
    Resolution       float64 // для regular (градусы)
    MaxTriangleArea  float64 // для TIN (км²)
    ErrorTolerance   float64 // для adaptive (метры)
}

func BuildTINMesh(points []LatLon, opts ApproximationOptions) (*TINMesh, error)
```

**Применение:**
- **Плотная триангуляция** нерегулярно распределённых точек береговой линии
- **Интерполяция значений** (глубина, эрозия) через барицентрические координаты
- **Адаптивное уточнение** сетки в зонах сложной геометрии
- **Визуализация рельефа** через TIN mesh с экспортом в GeoJSON

**Алгоритм:** Delaunay триангуляция (Bowyer-Watson) с автоматическим удалением супер-треугольника

**Метрики качества:**
- Углы треугольников (min/max/avg) для оценки регулярности сетки
- Плотность треугольников на единицу площади
- Выявление вырожденных треугольников

#### Другие методы аппроксимации

- **IDW (Inverse Distance Weighting):** интерполяция литологических данных
- **Regular Grid:** батиметрическая сетка с заданным разрешением
- **Билинейная интерполяция:** оценка глубин в промежуточных точках
- **Linear Interpolation:** аппроксимация полилиний

---

## 🧩 CLI

**Каноническая структура:**

```bash
# Утилиты источника данных
lito source                    # проверка источника данных и сохранение snapshot

# Анализ реальных данных
lito real coastline           # валидация и метрики загруженной береговой линии

# Научные модели
lito model dimension          # фрактальный анализ (box-counting)
lito model erosion            # геоморфологическое моделирование эрозии

# Полный сценарий
lito all                       # валидация + фрактальный анализ + эрозия
```

**Основные флаги для моделей:**

```bash
# Базовые параметры
--output PATH                # директория для выходных файлов
--iterations N               # число итераций organic Koch (0-10)
--steps N                   # число шагов эрозии

# Фрактальный анализ
--seed INT                   # seed для воспроизводимости
--angle-jitter FLOAT         # максимальное отклонение угла (градусы)
--height-jitter FLOAT        # максимальное отклонение высоты (доля)

# Геоморфологическое моделирование
--steps N                    # число шагов эрозии
--erosion-strength FLOAT     # базовый отступ берега (метры)
--wave-direction FLOAT       # направление волн (градусы от севера)
--wind-speed FLOAT            # скорость ветра (м/с)
--fetch-spread FLOAT         # полуширина сектора fetch (градусы)
--bathymetry PATH            # путь к файлу батиметрии
--lithology PATH             # путь к файлу литологии
--enable-lithology           # включить модуляцию по породам

# Временная динамика
--target-years N             # целевой период моделирования (годы)
--years-per-step FLOAT       # лет за шаг моделирования
--storm-probability FLOAT    # вероятность шторма за шаг [0-1]
--storm-intensity FLOAT      # множитель силы шторма [1.0-10.0]
--sea-level-rise FLOAT       # подъём уровня моря (м/год)
--enable-seasonality         # включить сезонные колебания
--seasonal-phase FLOAT       # фаза сезонности [0-2π]

# CSV экспорт
--output-csv PATH            # путь к CSV файлу (по умолчанию: erosion_metrics.csv)
--csv-format FORMAT          # формат: 'long' (одна строка на шаг) или 'wide' (матрица)

# GIF анимация
--output-gif PATH            # путь к GIF файлу (пустая строка отключает экспорт)
--gif-fps INT                # кадров в секунду (1-30, по умолчанию: 10)
--gif-skip INT               # пропуск каждого N-го кадра (1 = не пропускать)
--gif-color-change           # цветовая кодировка по интенсивности изменений
--gif-show-initial           # показывать начальное состояние (серая линия)
--gif-show-scalebar          # показывать масштабную линейку
--gif-scalebar-km FLOAT      # длина масштабной линейки в км (0 = авто)
--gif-colorlegend-pos STRING # позиция легенды цветов: 'right', 'bottom', 'none'
--gif-colors INT             # количество цветов палитры (0 = авто 16, диапазон: 4-256)
--gif-compression STRING     # уровень сжатия: 'low', 'medium', 'high'
--gif-width INT              # ширина GIF в пикселях (0 = авто 1200)
--gif-height INT             # высота GIF в пикселях (0 = авто 800)
--gif-show-timestamp         # показывать временные метки (годы, штормы)
--gif-show-metrics           # показывать метрики кадра (длина, эрозия)
--gif-geo-labels STRING      # географические метки: 'none', 'major', 'all'
```

---

## 🛠 Установка и запуск

```bash
# Клонирование
git clone https://github.com/Nickitas/litora-cli.git
cd litora-cli

# Сборка
go build -o lito ./cmd/lito

# Или через Makefile
make build

# Запуск
./lito --help
```

### Автоматическая загрузка батиметрии

```bash
# Загрузка реальных данных GEBCO для Чёрного моря
make bathymetry

# Или напрямую
go run cmd/download-bathymetry/main.go
```

Данные сохраняются в `data/black-sea-bathymetry.json` и автоматически используются при моделировании эрозии.

---

## Сценарии использования

### 1. Базовая валидация и метрики

```bash
# Метрики загруженной береговой линии
./lito real coastline

# Принудительное обновление данных
./lito real coastline --refresh

# Использование локального файла
./lito real coastline --source-url '' --input data/black-sea.json
```

**Результаты:**
- `output/svg/coastline.svg` — визуализация береговой линии с подсветкой проблемных сегментов
- `output/metrics/coastline.metrics.json` — детальные метрики: длина, число точек, валидация

### 2. Фрактальный анализ

```bash
# Базовый анализ фрактальной размерности
./lito model dimension --iterations 6 --seed 42

# С сохранением в отдельную директорию
./lito model dimension --iterations 8 --output ./output/fractal
```

**Результаты:**
- `output/svg/dimension_iter_0.svg ... dimension_iter_6.svg` — визуализация сходимости
- `output/metrics/dimension.metrics.json` — метрики фрактальной размерности
- Анализ изменения фрактальной размерности по итерациям
- Диагностика статистической устойчивости оценки

### 3. Геоморфологическое моделирование эрозии

#### 3.1 Базовое моделирование

```bash
# Простая эрозия с геометрическим proxy
./lito model erosion --steps 10 --erosion-strength 50 --wave-direction 0
# Автоматически создаёт output/csv/erosion_metrics.csv

# С GIF-анимацией результатов
./lito model erosion \
  --steps 10 \
  --erosion-strength 50 \
  --wave-direction 0 \
  --output-gif erosion_basic.gif \
  --gif-fps 12 \
  --gif-show-scalebar
# → erosion_basic.gif + output/csv/erosion_metrics.csv
```

#### 3.2 С учётом батиметрии

```bash
# Загрузка реальных глубин Чёрного моря
make bathymetry

# Моделирование с батиметрией
./lito model erosion \
  --steps 15 \
  --erosion-strength 50 \
  --bathymetry data/black-sea-bathymetry.json \
  --wave-direction 45 \
  --wind-speed 14 \
  --output-gif erosion_bathymetry.gif \
  --gif-show-timestamp \
  --gif-geo-labels major
# → erosion_bathymetry.gif + output/csv/erosion_metrics.csv
```

#### 3.3 С учётом литологии

```bash
# Моделирование с учётом сопротивления пород
./lito model erosion \
  --steps 10 \
  --lithology data/black-sea-lithology.json \
  --enable-lithology \
  --output-csv erosion_lithology.csv
# → output/csv/erosion_lithology.csv
```

**Физический принцип:** `retreatActual = retreatBase / Resistance`

#### 3.4 Комплексная модель (батиметрия + литология)

```bash
# Полная физическая модель
./lito model erosion \
  --steps 15 \
  --erosion-strength 50 \
  --wave-direction 45 \
  --wind-speed 14 \
  --bathymetry data/black-sea-bathymetry.json \
  --lithology data/black-sea-lithology.json \
  --enable-lithology \
  --output-csv full_model.csv \
  --csv-format long
# → output/csv/full_model.csv
```

### 4. Временная динамика

#### 4.1 Базовое временное моделирование

```bash
# Моделирование на 10 лет (1 год за шаг)
./lito model erosion \
  --target-years 10 \
  --years-per-step 1 \
  --steps 10
# → output/csv/erosion_metrics.csv (с временными данными)
```

#### 4.2 Штормовый климат

```bash
# 30% вероятность штормов, интенсивность 2.5x
./lito model erosion \
  --target-years 20 \
  --years-per-step 2 \
  --storm-probability 0.3 \
  --storm-intensity 2.5 \
  --output-csv storm_analysis.csv
# → output/csv/storm_analysis.csv
```

#### 4.3 С сезонными колебаниями

```bash
# Пик эрозии зимой
./lito model erosion \
  --target-years 15 \
  --years-per-step 3 \
  --enable-seasonality \
  --seasonal-phase 3.14 \
  --csv-format wide
# → output/csv/erosion_metrics.csv (wide формат)
```

#### 4.4 Климатический сценарий RCP8.5

```bash
# Комплексный сценарий с учётом климатических изменений
./lito model erosion \
  --target-years 50 \
  --years-per-step 5 \
  --storm-probability 0.2 \
  --storm-intensity 2.5 \
  --sea-level-rise 0.01 \
  --enable-seasonality \
  --bathymetry data/black-sea-bathymetry.json \
  --lithology data/black-sea-lithology.json \
  --enable-lithology \
  --output-csv climate_rcp85.csv
# → output/csv/climate_rcp85.csv
```

### 5. Полный научный сценарий

```bash
# Все этапы: валидация + фрактальный анализ + эрозия
./lito all --output ./output
# Автоматически создаёт output/csv/erosion_metrics.csv

# С временными параметрами и GIF
./lito all \
  --iterations 4 \
  --steps 10 \
  --target-years 20 \
  --years-per-step 2 \
  --storm-probability 0.15 \
  --enable-seasonality \
  --output-gif full_analysis.gif \
  --gif-fps 10 \
  --gif-show-scalebar \
  --gif-show-timestamp \
  --gif-geo-labels major \
  --output ./output

# С кастомным CSV экспортом и оптимизированным GIF
./lito all \
  --steps 10 \
  --target-years 30 \
  --years-per-step 3 \
  --storm-probability 0.2 \
  --sea-level-rise 0.01 \
  --output-csv climate_analysis.csv \
  --csv-format long \
  --output-gif climate_optimized.gif \
  --gif-fps 8 \
  --gif-skip 2 \
  --gif-colors 12 \
  --gif-compression high
# → climate_analysis.csv + climate_optimized.gif
```

### 6. TIN Mesh аппроксимация береговой линии

#### 6.1 Базовая TIN триангуляция

```go
package main

import (
    "coastal-geometry/internal/domain/geometry"
    "log"
)

func main() {
    // Загрузка береговой линии
    points, _, err := coastline.LoadFromJSON("data/black-sea.json")
    if err != nil {
        log.Fatal(err)
    }

    // Параметры TIN mesh
    opts := geometry.DefaultApproximationOptions()
    opts.MeshType = "tin"
    opts.MaxTriangleArea = 10.0  // 10 км² максимальная площадь треугольника

    // Построение TIN mesh
    mesh, err := geometry.BuildTINMesh(points, opts)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("TIN Mesh: %d вершин, %d треугольников",
        mesh.Stats.VertexCount, mesh.Stats.TriangleCount)
    log.Printf("Площадь треугольников: avg=%.2f км², min=%.2f км², max=%.2f км²",
        mesh.Stats.AvgTriangleArea/1e6,
        mesh.Stats.MinTriangleArea/1e6,
        mesh.Stats.MaxTriangleArea/1e6)
}
```

#### 6.2 Адаптивное уточнение сетки

```go
// Адаптивная триангуляция с автоматическим уточнением
opts := geometry.DefaultApproximationOptions()
opts.MeshType = "adaptive"
opts.MaxTriangleArea = 5.0   // 5 км² - порог для разделения
opts.ErrorTolerance = 100.0  // 100 метров допустимая ошибка

mesh, _ := geometry.BuildTINMesh(points, opts)

log.Printf("Рефайнмент: %d итераций", mesh.Stats.RefinementSteps)
```

#### 6.3 Интерполяция значений через TIN

```go
// Интерполяция глубин в произвольной точке
depths := []float64{-10.5, -25.3, -15.8, ...} // для каждой вершины mesh

lat, lon := 44.5, 38.5
interpolatedDepth, err := mesh.InterpolateValue(lat, lon, depths)
if err != nil {
    log.Printf("Точка (%f, %f) вне mesh bounds", lat, lon)
} else {
    log.Printf("Глубина в точке: %.2f м", interpolatedDepth)
}
```

#### 6.4 Оценка качества TIN mesh

```go
// Метрики качества сетки
quality := mesh.GetMeshQuality()

log.Printf("Углы треугольников: min=%.1f°, max=%.1f°, avg=%.1f°",
    quality.MinAngle, quality.MaxAngle, quality.AvgAngle)

// Хороший TIN mesh: min угол > 20°, max угол < 120°
if quality.MinAngle < 20 {
    log.Println("Предупреждение: найдены узкие треугольники")
}
if quality.MaxAngle > 120 {
    log.Println("Предупреждение: найдены вытянутые треугольники")
}
```

#### 6.5 Экспорт TIN mesh в GeoJSON

```go
// Экспорт для визуализации в GIS системах
geojson, err := mesh.ExportGeoJSON()
if err != nil {
    log.Fatal(err)
}

err = os.WriteFile("output/tin_mesh.geojson", geojson, 0644)
if err != nil {
    log.Fatal(err)
}
```

**Пример использования в Python:**

```python
import geopandas as gpd
import matplotlib.pyplot as plt

# Загрузка TIN mesh
gdf = gpd.read_file("output/tin_mesh.geojson")

# Визуализация треугольников с цветовой кодировкой по площади
fig, ax = plt.subplots(figsize=(12, 8))
gdf.plot(column='area', cmap='viridis', legend=True, ax=ax)
ax.set_title('TIN Mesh береговой линии')
plt.show()
```

#### 6.6 Валидация TIN mesh

```go
// Проверка топологической корректности
errors := mesh.ValidateMesh()
if len(errors) > 0 {
    log.Println("Ошибки валидации TIN mesh:")
    for _, e := range errors {
        log.Printf("  - %s", e)
    }
} else {
    log.Println("TIN mesh валиден")
}

// Плотность сетки (треугольников на км²)
density := mesh.MeshDensity()
log.Printf("Плотность: %.2f треугольников/км²", density/1e6)
```

#### 6.7 Упрощение TIN mesh

```go
// Удаление мелких треугольников для снижения детализации
maxAreaKm2 := 50.0  // 50 км²
err := mesh.Simplify(maxAreaKm2 * 1e6)  // конвертация в м²
if err != nil {
    log.Fatal(err)
}

log.Printf("После упрощения: %d треугольников (было %d)",
    mesh.Stats.TriangleCount, originalCount)
```

---

## Выходные данные

### SVG-отчёты

**Файлы генерируемые при анализе:**

- `output/svg/coastline.svg` — исходная береговая линия с диагностикой геометрии
- `output/svg/dimension_iter_0.svg ... dimension_iter_N.svg` — фрактальный анализ по итерациям
- `output/svg/erosion_step_0.svg ... erosion_step_N.svg` — динамика эрозии во времени

**GIF-анимация:**
- `custom_name.gif` — анимированная визуализация эрозионных процессов (указывается через `--output-gif`)

**Информация в SVG:**
- Карта береговой линии с цветовой кодировкой
- Графики метрик (длина, площадь, фрактальная размерность)
- Таблицы с количественными данными
- Индикаторы временной динамики (⛈️ — штормовые события)

### Метрики JSON

**Файлы:**
- `output/metrics/coastline.metrics.json` — метрики береговой линии
- `output/metrics/erosion.metrics.json` — метрики эрозии
- `output/metrics/dimension.metrics.json` — метрики фрактального анализа

**Структура `*.metrics.json`:**

```json
{
  "length_km": 6391.234,
  "points": 9635,
  "area_km2": 419695.123,
  "validation": {
    "summary": {
      "total_warnings": 3,
      "self_intersections": 0,
      "long_segments": 2
    }
  },
  "fractal_analysis": {
    "dimension": 1.2678,
    "r_squared": 0.9876,
    "stable_across_scales": true
  }
}
```

### Временная статистика

**Таблица временной динамики:**

```
┌──────┬──────────┬───────────┬───────────┬─────────────┐
│ Шаг  │ Год      │ Точек     │ Длина км  │ Площадь км² │
├──────┼──────────┼───────────┼───────────┼─────────────┤
│ 0    │ 0        │ 9635      │ 6391      │ 419695      │
│ 1    │ 2        │ 9635      │ 6343      │ 419690      │
│ 2    │ 4        │ 9635      │ 6277      │ 419680      │⛈️
└──────┴──────────┴───────────┴───────────┴─────────────┘
```

**Сводка временной динамики:**
- Промоделировано лет: 20 из 50 (целевых)
- Штормовых событий: 3 (частота 0.15)
- Подъём уровня моря: 0.50 м
- Накопленная эрозия: 114.2 м
- Изменение длины берега: -44.1 км (-0.7%)

### CSV-отчёты

**Автоматический экспорт метрик для количественного анализа**

CSV-файлы создаются автоматически при запуске команд `all` и `model erosion`. По умолчанию сохраняются в `output/csv/erosion_metrics.csv`.

```bash
# Автоматическое создание CSV
./lito all --steps 5
# → output/csv/erosion_metrics.csv

# Кастомное имя файла
./lito model erosion --steps 5 --output-csv my_analysis.csv
# → output/csv/my_analysis.csv

# Wide формат (матрица метрик)
./lito model erosion --steps 5 --csv-format wide
# → output/csv/erosion_metrics.csv (wide)

# Отключение CSV экспорта
./lito model erosion --steps 5 --output-csv ""
# → CSV не создаётся
```

#### Long формат (по умолчанию)

**Структура:** одна строка на шаг моделирования

```csv
year,step,length_km,area_km2,eroded_m3,deposited_m3,net_change_m3,storm_event,sea_level_m
0.0,0,6391.1,419695.1,0.0,0.0,0.0,false,0.0000
3.0,1,6342.9,419689.8,48169.6,0.0,48169.6,false,0.0300
6.0,2,6276.3,419680.2,66576.6,0.0,66576.6,true,0.0600
9.0,3,6246.8,419674.4,29538.1,0.0,29538.1,false,0.0900
12.0,4,6185.0,419659.3,61794.0,0.0,61794.0,true,0.1200
```

**Преимущества:**
- Идеален для временных рядов и графиков
- Удобен для анализа динамики по шагам
- Подходит для линейной регрессии и трендов

#### Wide формат

**Структура:** матрица метрик по шагам

```csv
metric_name,step_0,step_1,step_2,step_3,step_4
year,0.0,3.0,6.0,9.0,12.0
length_km,6391.1,6342.9,6276.3,6246.8,6185.0
area_km2,419695.1,419689.8,419680.2,419674.4,419659.3
storm_event,false,false,true,true,false
sea_level_m,0.0000,0.0300,0.0600,0.0900,0.1200
```

**Преимущества:**
- Удобен для сравнения метрик между шагами
- Компактное представление данных
- Идеален для анализа корреляций

#### Встроенный анализ Python

Доступные скрипты для анализа:

- базовый анализ эрозии
- визуализация динамики
- анализ штормов
- сравнение сценариев
- генерация отчетов

Детальное описание представлено в [scripts](scripts/README.md)

### GIF-анимация

**Автоматическое создание анимированной визуализации эрозионных процессов**

GIF-файлы создаются автоматически при указании параметра `--output-gif` для команд `all` и `model erosion`. Анимация показывает динамику изменения береговой линии во времени с цветовой кодировкой эрозионных и аккумулятивных процессов.

```bash
# Базовое создание GIF
./lito model erosion --steps 10 --output-gif erosion_animation.gif
# → erosion_animation.gif

# Настройка качества и размера
./lito model erosion \
  --steps 15 \
  --output-gif high_quality.gif \
  --gif-fps 15 \
  --gif-width 1600 \
  --gif-height 900 \
  --gif-compression high
# → high_quality.gif (высокое качество)

# Оптимизация размера файла
./lito model erosion \
  --steps 20 \
  --output-gif optimized.gif \
  --gif-skip 2 \
  --gif-fps 8 \
  --gif-colors 8 \
  --gif-compression high
# → optimized.gif (маленький размер)

# Научная визуализация с масштабом
./lito model erosion \
  --steps 12 \
  --output-gif scientific.gif \
  --gif-show-scalebar \
  --gif-scalebar-km 100 \
  --gif-show-timestamp \
  --gif-show-metrics \
  --gif-geo-labels major
# → scientific.gif (с научными элементами)
```

#### Параметры визуализации

**Цветовая кодировка:**
- 🔴 **Красный** — эрозия (отступ берега)
- 🟢 **Зелёный** — аккумуляция (наносы)
- ⚪ **Серый** — начальная береговая линия
- 🔵 **Синий** — вода

**Элементы визуализации:**
- **Масштабная линейка** — реальный масштаб в километрах
- **Временные метки** — текущий год, индикаторы штормов (⛈️)
- **Метрики кадра** — длина берега, объём эрозии
- **Географические метки** — названия городов и ориентиров
- **Легенда цветов** — пояснение цветовой кодировки

#### Настройка качества и размера

**Баланс качества и размера файла:**

| Параметр | Маленький размер | Высокое качество |
|----------|------------------|------------------|
| `--gif-fps` | 8 | 15-30 |
| `--gif-skip` | 2-3 | 1 |
| `--gif-colors` | 8-16 | 32-64 |
| `--gif-compression` | high | low/medium |
| `--gif-width` | 800-1200 | 1600-2400 |

**Примеры оптимальных настроек:**

```bash
# Для презентаций (средний размер, хорошее качество)
--gif-fps 12 --gif-skip 1 --gif-colors 16 --gif-compression medium

# для веб-публикации (маленький размер)
--gif-fps 8 --gif-skip 2 --gif-colors 8 --gif-compression high

# Для научных публикаций (максимальное качество)
--gif-fps 15 --gif-skip 1 --gif-colors 32 --gif-compression low --gif-width 1920
```

#### Примеры использования в научных сценариях

**Временная динамика с GIF-визуализацией:**

```bash
# 50-летняя проекция с климатическими сценариями
./lito model erosion \
  --target-years 50 \
  --years-per-step 5 \
  --steps 10 \
  --storm-probability 0.2 \
  --storm-intensity 2.5 \
  --sea-level-rise 0.01 \
  --enable-seasonality \
  --bathymetry data/black-sea-bathymetry.json \
  --lithology data/black-sea-lithology.json \
  --enable-lithology \
  --output-gif climate_projection_50yr.gif \
  --gif-fps 10 \
  --gif-show-scalebar \
  --gif-show-timestamp \
  --gif-geo-labels major \
  --output-csv climate_50yr.csv
# → climate_projection_50yr.gif + climate_50yr.csv
```

**Сравнение сценариев:**

```bash
# Сценарий 1: умеренная эрозия
./lito model erosion \
  --steps 8 \
  --erosion-strength 30 \
  --wave-direction 45 \
  --output-gif scenario1_moderate.gif \
  --gif-width 1400 \
  --gif-compression medium

# Сценарий 2: интенсивная эрозия
./lito model erosion \
  --steps 8 \
  --erosion-strength 80 \
  --wave-direction 0 \
  --output-gif scenario2_intense.gif \
  --gif-width 1400 \
  --gif-compression medium

# Сценарий 3: с защитными мерами (низкая эрозия)
./lito model erosion \
  --steps 8 \
  --erosion-strength 15 \
  --wave-direction 90 \
  --output-gif scenario3_protected.gif \
  --gif-width 1400 \
  --gif-compression medium
```

---

## Методы аппроксимации и интерполяции

### IDW (Inverse Distance Weighting)

**Применение:** Литологическая интерполяция

```go
// Вес точки обратно пропорционален квадрату расстояния
weight = 1 / distance²

// Интерполируемое значение = взвешенная сумма
value = Σ(weight_i × value_i) / Σ(weight_i)
```

**Параметры:**
- Максимальное число точек: 6
- Степень расстояния: 2.0 (стандартное значение)

### Regular Grid

**Применение:** Батиметрическая сетка

```go
// Создание равномерной сетки с заданным разрешением
grid = BuildGrid(points, resolution)

// Билинейная интерполяция для промежуточных точек
depth = BilinearInterpolate(grid, lat, lon)
```

**Параметры:**
- Разрешение по умолчанию: 0.01° (~1.1 км)
- Автоматическое определение границ

### Линейная интерполяция

**Применение:** Аппроксимация полилиний

```go
// Интерполяция между двумя точками
point = lerp(point1, point2, t)
```

---

## Научные задачи проекта

1. **Сравнительный анализ** различных уровней представления береговой линии
2. **Исследование фрактальных свойств** природных и синтетических кривых
3. **Разработка физически обоснованной модели** волновой эрозии с учётом батиметрии и литологии
4. **Верификация модели** на реальных данных Черного моря
5. **Исследование временной динамики** береговых процессов с учётом климатических сценариев
6. **Создание открытого инструментария** для автоматизированного анализа прибрежных систем

---

## Области применения

- **Геоморфология** и динамика береговых зон
- **Фрактальная геометрия** и вычислительная геометрия
- **Экологическое прогнозирование** и оценка рисков эрозии
- **Климатические исследования** и сценарии потепления
- **Образовательные курсы** по геоморфологии и моделированию

---

## Лицензия

Проект распространяется под лицензией **MIT** — см. файл [LICENSE](LICENSE).
