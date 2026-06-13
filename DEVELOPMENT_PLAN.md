# План доработки FRAES до научной геоморфологической модели

## Текущий статус анализа

**Уже реализовано:**
- ✅ Волновая эрозия с fetch distance и экспозицией
- ✅ Батиметрическая интеграция (GEBCO данные Чёрного моря)
- ✅ Направленная эрозия (мысы vs бухты)
- ✅ Временные шаги (штормы можно эмулировать силой ветра)

**Требует доработки:**
- ❌ Транспорт наносов (longshore drift)
- ❌ Литология (сопротивление пород)
- ❌ Баланс эрозия/аккумуляция
- ❌ Временная шкала (годы вместо шагов)
- ❌ GIF/CSV отчёты
- ❌ WASM веб-версия

---

## Приоритизация задач

### Приоритет 1: Научная валидность (критично)

#### 1.1 Транспорт наносов (Sediment Transport)

**Физическая модель:**
- Размытый материал не исчезает — переоткладывается вдоль берега
- Longshore drift: поток вдоль берега по градиенту волновой энергии
- Зоны аккумуляции: бухты, зашищённые участки

**Математическая модель:**
```go
// SedimentBudget отслеживает баланс материала
type SedimentBudget struct {
    Eroded    float64  // объём размытого материала (м³/м)
    Transport float64  // объём в транзите (м³/м)
    Deposited float64  // объём отложенного (м³/м)
}

// Longshore drift по касательной к берегу
driftVolume = waveEnergy × sin(waveIncidenceAngle) × transportCoefficient

// Депозиция когда скорость потока падает
if flowVelocity < threshold:
    depositedVolume = min(inTransit, capacity)
```

**Реализация:**
- Создать `internal/domain/geometry/sediment.go`
- Интегрировать в `SimulateWaveErosion`
- Добавить параметры: `TransportCoeff`, `DepositionRate`
- Визуализация: цветовая кодировка зон эрозии/аккумуляции

**Оценка сложности:** ⭐⭐⭐ (3/5)
**Время:** 2-3 недели
**Зависимости:** Нет

#### 1.2 Литологический профиль (Lithology)

**Физическая модель:**
- Разные породы имеют разную resistance к эрозии
- RockResistance модулирует скорость retreat

**Классификация пород (Чёрное море):**
```go
type LithologyClass struct {
    Name         string
    Resistance   float64  // относительная сопротивляемость [0.1-10]
    Color        string   // для визуализации
    Description  string
}

var blackSeaLithology = []LithologyClass{
    {Name: "Granite", Resistance: 8.0, Color: "#4a4a4a"},       // очень твёрдый
    {Name: "Basalt", Resistance: 7.0, Color: "#2d2d2d"},        // твёрдый
    {Name: "Limestone", Resistance: 3.0, Color: "#b8b8b8"},     // средний
    {Name: "Sandstone", Resistance: 2.0, Color: "#c4a484"},     // мягкий
    {Name: "Clay", Resistance: 1.0, Color: "#8b7355"},          // очень мягкий
    {Name: "GravelBeach", Resistance: 4.0, Color: "#9e9e9e"},   // динамический
}
```

**Математическая модель:**
```go
// Скорость эрозии обратно пропорциональна сопротивлению
retreatMeters = (strength / rockResistance) × otherFactors

// Для гравийных пляжей — динамическое равновесие
if lithology == GravelBeach:
    retreat = max(0, retreat - autoReplenishmentRate)
```

**Реализация:**
- Создать `internal/domain/geometry/lithology.go`
- Загрузка профиля из JSON: `data/black-sea-lithology.json`
- Интерполяция между точками замера
- Интеграция в `waveErodeStep`
- SVG: сегменты раскрашены по литологии

**Оценка сложности:** ⭐⭐ (2/5)
**Время:** 1-2 недели
**Зависимости:** Нет

#### 1.3 Временная динамика (Temporal Scaling)

**Проблема:** Сейчас шаги абстрактные, нет привязки к реальному времени

**Решение:**
```go
type TemporalParameters struct {
    YearsPerStep        float64  // лет за один шаг модели
    StormProbability    float64  // вероятность шторма за шаг
    StormIntensityMult  float64  // множитель силы шторма
    SeaLevelRise       float64  // м/год (глобальное потепление)
    Seasonality         bool     // учитывать сезонность
}

// Конвертация шагов в годы
func SimulateErosionWithDuration(
    points []LatLon,
    targetYears int,
    params TemporalParameters,
    options WaveErosionOptions,
) [][]LatLon
```

**Сезонная модель:**
```go
// Зима: сильные штормы, лето: штиль
seasonalFactor = 1.0 + 0.5×sin(2π × year + phase)

// Штормовые события
if rng.Float64() < stormProbability:
    windSpeed ×= stormIntensityMult
```

**Реализация:**
- Создать `internal/domain/geometry/temporal.go`
- Параметры: `--years`, `--years-per-step`, `--storm-prob`, `--sea-level-rise`
- Метрики: шаг → год с timestamp
- CSV отчёт: год, длина, площадь, объём эрозии

**Оценка сложности:** ⭐⭐⭐ (3/5)
**Время:** 2-3 недели
**Зависимости:** Нет

---

### Приоритет 2: Улучшение инструментов (важно)

#### 2.1 GIF анимация

**Зачем:** Визуализация динамики эрозии во времени

**Реализация:**
```go
// Использовать библиотеку imgo или ee_gif
func generateErosionGIF(
    snapshots [][]LatLon,
    outputPath string,
    fps int,
    skipEvery int,  // пропускать кадры
) error
```

**Параметры:**
```bash
./fraes model erosion \
  --steps 100 \
  --output-gif \
  --gif-fps 10 \
  --gif-skip 2  # каждый 2-й кадр
```

**Оценка сложности:** ⭐⭐ (2/5)
**Время:** 1 неделя
**Зависимости:** Go image библиотеки

#### 2.2 CSV отчёты

**Зачем:** Количественный анализ, графики в Excel/Python

**Структура CSV:**
```csv
year,step,length_km,area_km2,eroded_m3,deposited_m3,net_change_m3
0,0,2345.6,12345.0,0,0,0
1,1,2344.8,12344.2,1200.5,800.3,-400.2
...
```

**Реализация:**
```go
func writeErosionCSV(
    metrics []ErosionStepMetrics,
    outputPath string,
) error
```

**Параметры:**
```bash
./fraes model erosion \
  --output-csv \
  --csv-format long  # long | wide
```

**Оценка сложности:** ⭐ (1/5)
**Время:** 3-4 дня
**Зависимости:**encoding/csv

#### 2.3 Метрики качества модели

**Зачем:** Научная валидация

**Метрики:**
```go
type ModelQualityMetrics struct {
    // Фрактальная устойчивость
    DimensionStability    float64  // изменение D во времени

    // Баланс массы
    MassBalance          float64  // eroded - deposited

    // Пространственная когерентность
    SpatialAutocorr      float64  // корреляция соседних участков

    // Сходимость
    ConvergenceRate      float64  // скорость изменения метрик
}
```

**Реализация:**
- Создать `internal/domain/geometry/validation.go`
- Вычислять на каждом шаге симуляции
- Вывести в финальный отчёт

**Оценка сложности:** ⭐⭐ (2/5)
**Время:** 1 неделя
**Зависимости:** Нет

---

### Приоритет 3: Доступность и UX (полезно)

#### 3.1 WASM веб-версия

**Зачем:** Демо, доступность без установки

**Архитектура:**
```
[Web Frontend] → [WASM Bridge] → [Go Code (compiled to WASM)]
```

**Этапы:**
1. Рефакторинг для чистого Go (убрать `os`-зависимости)
2. Компиляция в WASM: `GOOS=js GOARCH=wasm go build`
3. JavaScript bridge для API
4. UI: React/Vue компоненты

**Функциональность:**
- Загрузка GeoJSON файла
- Базовый анализ: длина, фрактальная размерность
- Простая эрозия (без батиметрии)
- SVG экспорт

**Ограничения WASM:**
- Нет файловой системы
- Батиметрия должна быть embedded в бинарник
- Производительность ниже нативной

**Оценка сложности:** ⭐⭐⭐⭐ (4/5)
**Время:** 4-6 недель
**Зависимости:** Go WASM, JavaScript фронтенд

#### 3.2 Профили региональные

**Зачем:** Быстрый старт для разных морей

**Структура:**
```
data/profiles/
  black-sea.json          # литология, батиметрия
  mediterranean.json
  caspian.json
  north-sea.json
```

**Формат профиля:**
```json
{
  "name": "Чёрное море",
  "bounds": [40, 47, 27, 42],
  "default_bathymetry": "data/black-sea-bathymetry.json",
  "lithology_profile": "data/black-sea-lithology.json",
  "typical_params": {
    "wave_direction": 0,
    "wind_speed": 12,
    "erosion_strength": 30
  }
}
```

**Использование:**
```bash
./fraes model erosion --profile black-sea
```

**Оценка сложности:** ⭐⭐ (2/5)
**Время:** 1 неделя
**Зависимости:** Нет

#### 3.3 Документация API

**Зачем:** Интеграция с другими системами

**Формат:**
- Пакетная документация (godoc)
- Примеры использования
- Python wrapper (через cgo или subprocess)

**Оценка сложности:** ⭐ (1/5)
**Время:** 3-4 дня
**Зависимости:** Нет

---

## Детальный план реализации

### Фаза 1: Научное ядро (6-8 недель)

**Цель:** Превратить из геометрической генерации в геоморфологическую модель

| Неделя | Задача | Результат |
|--------|--------|-----------|
| 1-2 | Транспорт наносов | `sediment.go`, баланс массы |
| 3-4 | Литология | `lithology.go`, профиль Чёрного моря |
| 5-6 | Временная динамика | `temporal.go`, годы вместо шагов |
| 7-8 | Метрики качества | `validation.go`, научная валидация |

**Мilestone 1:** FRAES может моделировать реальную geomorphology с балансом массы

### Фаза 2: Инструменты (3-4 недели)

**Цель:** Улучшить юзабилити для научных исследований

| Неделя | Задача | Результат |
|--------|--------|-----------|
| 1 | CSV отчёты | Экспорт метрик |
| 2 | GIF анимация | Визуализация динамики |
| 3 | Профили регионов | Quick start для разных морей |
| 4 | Документация API | Интеграционная документация |

**Milestone 2:** Удобный инструмент для научной работы

### Фаза 3: Веб-версия (4-6 недель)

**Цель:** Демо и доступность

| Неделя | Задача | Результат |
|--------|--------|-----------|
| 1-2 | WASM компиляция | Go → WASM bridge |
| 3-4 | Frontend UI | React/Вью компоненты |
| 5-6 | Полировка | Оптимизация, тестирование |

**Milestone 3:** Работающее веб-демо

---

## Критерии успеха

### Научная валидность
- ✅ Баланс массы (эрозия ≈ аккумуляция)
- ✅ Фрактальная размерность в реалистичном диапазоне (1.05-1.30)
- ✅ Временная динамика соответствует наблюдаемым скоростям

### Практическая полезность
- ✅ CSV экспорт для анализа в Python/R
- ✅ GIF для презентаций
- ✅ Профили для разных регионов

### Техническое качество
- ✅ Тесты覆盖 всех основных функций
- ✅ Documentation README + API docs
- ✅ WASM версия работает в браузере

---

## Обратная совместимость

**Сохраняется:**
- CLI интерфейс без изменений
- Все текущие флаги и команды
- SVG/JSON форматы выходных данных

**Добавляется:**
- Новые флаги (opt-in)
- Новые команды (opt-in)
- Расширенные метрики

---

## Риски и митигация

### Риск 1: Слишком высокая сложность

**Митигация:**
- Разбить на независимые модули
- Каждый feature можно disable флагом
- Минимум зависимостей между модулями

### Риск 2: Плохая производительность

**Митигация:**
- Профилирование перед оптимизацией
- Параллельность там, где возможно
- Lazy loading тяжелых данных (батиметрия)

### Риск 3: Научная неверифицируемость

**Митигация:**
- Честная документация ограничений
- Ссылки на литературу для всех методов
- Quality metrics как часть output

---

## Библиография для реализации

### Coastal Geomorphology
1. **Komar, P. D.** (1998). *Beach Processes and Sedimentation*. Prentice Hall.
2. **Masselink, G., & Hughes, M. G.** (2003). *Introduction to Coastal Geomorphology*. Arnold.
3. **Davidson-Arnott, R.** (2010). *Introduction to Coastal Processes and Geomorphology*. Cambridge University Press.

### Sediment Transport
4. **Soulsby, R.** (1997). *Dynamics of Marine Sands*. Thomas Telford.
5. **Van Rijn, L. C.** (1993). *Principles of Sediment Transport in Rivers, Estuaries and Coastal Seas*. Aqua Publications.

### Wave Mechanics
6. **US Army Corps of Engineers.** (1984). *Shore Protection Manual*. US Government Printing Office.
7. **Holthuijsen, L. H.** (2007). *Waves in Oceanic and Coastal Waters*. Cambridge University Press.

### Fractal Analysis
8. **Falconer, K.** (2013). *Fractal Geometry: Mathematical Foundations and Applications*. Wiley.
9. **Mandelbrot, B. B.** (1982). *The Fractal Geometry of Nature*. W. H. Freeman.

---

## Следующие шаги

**Сейчас:**
1. Определить приоритеты с точки зрения научной ценности
2. Выбрать одну задачу для prototyping
3. Создать branch для разработки

**Вопросы для обсуждения:**
- Какая задача наиболее важна для вашей диссертации?
- Есть ли доступ к литологическим данным Чёрного моря?
- Нужен ли WASM/веб-версия или достаточно CLI?
- Какая временная шкала моделирования важнее (годы, десятилетия)?
