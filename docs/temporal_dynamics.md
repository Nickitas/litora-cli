# Временная динамика

## Обзор

Модуль временной динамики (`internal/domain/geometry/temporal.go`) добавляет временной масштаб к моделированию береговой эрозии, преобразуя геометрическую генерацию в ** geomorphологическую модель**.

## Основные концепции

### 1. Временной масштаб

- **YearsPerStep**: количество лет, моделируемых за один шаг
- **TargetYears**: общая продолжительность моделирования
- Автоматический расчёт числа шагов: `ceil(targetYears / yearsPerStep)`

### 2. Штормовые события

- **StormProbability**: вероятность шторма за шаг [0-1]
- **StormIntensityMult**: множитель силы шторма [1.0-10.0]
- Стохастическая модель с вариацией интенсивности

### 3. Сезонность

- **Seasonality**: включить/выключить сезонные колебания
- **SeasonalPhase**: фаза сезонности [0-2π]
- Формула: `seasonalFactor = 1.0 + 0.5×sin(2π × year + phase)`
- Результат: множитель [0.5, 1.5]

### 4. Подъём уровня моря

- **SeaLevelRise**: скорость подъёма (м/год)
- Эмпирическая модуляция: `seaLevelFactor = 1.0 + 0.1×log(1 + seaLevelOffset)`
- Накопительный эффект со временем

## API

### Основные типы

```go
type TemporalParameters struct {
    YearsPerStep       float64  // лет за шаг
    StormProbability   float64  // вероятность шторма [0-1]
    StormIntensityMult float64  // множитель силы шторма [1.0-10.0]
    SeaLevelRise       float64  // м/год
    Seasonality        bool     // сезонность
    SeasonalPhase      float64  // фаза [0-2π]
}

type TemporalState struct {
    Step            int      // номер шага
    Year            float64  // текущий год
    IsStorm         bool     // штормовое событие
    StormIntensity  float64  // интенсивность шторма
    SeasonalFactor  float64  // сезонный множитель
    SeaLevelOffset  float64  // смещение уровня моря (м)
    EffectiveYears  float64  // эффективное число лет
}

type TemporalResult struct {
    Snapshots          [][]LatLon      // состояния береговой линии
    TemporalStates     []TemporalState // временные состояния
    TotalYears         float64         // общее число лет
    StormCount         int             // число штормов
    AccumulatedErosion float64         // накопленная эрозия (м)
    FinalSeaLevelRise  float64         // итоговый подъём уровня (м)
}
```

### Основные функции

```go
// Моделирование с временными параметрами
func SimulateErosionWithDuration(
    points []LatLon,
    targetYears int,
    params TemporalParameters,
    options WaveErosionOptions,
) TemporalResult

// Детерминистическая версия с seed
func SimulateErosionWithDurationSeed(...) TemporalResult

// Расчёт метрик по шагам
func CalculateErosionMetrics(result TemporalResult) []ErosionMetrics

// Сводка результатов
func GetTemporalSummary(result TemporalResult) map[string]interface{}

// Валидация параметров
func ValidateTemporalParameters(params TemporalParameters) []string
```

## Использование в CLI

### Базовое моделирование

```bash
# 10 лет с шагом 1 год
./lito erosion --target-years 10 --years-per-step 1
```

### Со штормами

```bash
# 20 лет, штормы с вероятностью 15%
./lito erosion --target-years 20 --years-per-step 2 \
  --storm-probability 0.15 --storm-intensity 2.0
```

### С сезонностью

```bash
# 15 лет с сезонными колебаниями
./lito erosion --target-years 15 --years-per-step 3 \
  --enable-seasonality --seasonal-phase 3.14
```

### Комплексный сценарий

```bash
# Все факторы (RCP8.5 климатический сценарий)
./lito erosion --target-years 50 --years-per-step 5 \
  --storm-probability 0.2 --storm-intensity 2.5 \
  --enable-seasonality --sea-level-rise 0.01 \
  --enable-lithology --lithology data/black-sea-lithology.json
```

## Физические ограничения

### Реалистичные диапазоны параметров

- **YearsPerStep**: 0.1-10 лет
  - 0.1-1 год: детальные исследования
  - 1-5 лет: баланс точности и скорости
  - 5-10 лет: долгосрочные тренды

- **StormProbability**: 0-0.5
  - 0.05-0.15: морской климат
  - 0.15-0.3: умеренный климат
  - 0.3-0.5: тропический циклогенез

- **StormIntensityMult**: 1.5-5.0
  - 1.5-2.5: обычные штормы
  - 2.5-4.0: сильные штормы
  - 4.0-5.0: экстремальные события

- **SeaLevelRise**: 0-0.01 м/год
  - 0.003 м/год: RCP4.5 (средний сценарий)
  - 0.01 м/год: RCP8.5 (экстремальный сценарий)

## Результаты

### Таблица состояний

```
┌──────┬──────────┬───────────┬───────────┬─────────────┐
│ Шаг  │ Год      │ Точек     │ Длина км  │ Площадь км² │
├──────┼──────────┼───────────┼───────────┼─────────────┤
│ 0    │ 0        │ 9635      │ 6391      │ 419695      │
│ 1    │ 2        │ 9635      │ 6343      │ 419690      │
│ 2    │ 4        │ 9635      │ 6277      │ 419680      │⛈️
└──────┴──────────┴───────────┴───────────┴─────────────┘
```

⛈️ - индикатор штормового события

### Статистика

- **Промоделировано лет**: общее время моделирования
- **Штормовых событий**: число и частота
- **Подъём уровня моря**: накопленный подъём
- **Накопленная эрозия**: общий объем размыва
- **Изменение длины берега**: линейное изменение (км, %)

## Интеграция с другими модулями

### Литология

```go
// Временная динамика автоматически учитывает литологию
if options.EnableLithology && options.LithologyProfile != nil {
    // Эрозия модулируется по сопротивлению пород
    // В сочетании с временной динамикой:
    // retreat = (base / resistance) × stormFactor × seasonalFactor × seaLevelFactor
}
```

### Транспорт наносов

```go
// Временная модуляция применяется к transported volume
transportedVolume = baseEroded × transportCoefficient × temporalFactor
```

## Валидация

Модуль автоматически проверяет параметры на реалистичность:

- Предупреждение при `StormProbability > 0.5`
- Предупреждение при `SeaLevelRise > 0.01` (превышает RCP8.5)
- Проверка диапазонов `YearsPerStep`

## Примеры кода

Смотрите `examples/temporal_dynamics_example.go` для полного примера использования API.

## Рекомендации

1. **Выбор временного шага**:
   - Для детальных исследований: 0.5-1 год
   - Для долгосрочных трендов: 2-5 лет
   - Для геологических масштабов: 5-10 лет

2. **Штормовые параметры**:
   - П_use базовые литологические данные для точности
   - Калибруйте stormProbability по историческим данным
   - Используйте stormIntensityMult 2.0-3.0 для реалистичных сценариев

3. **Климатические сценарии**:
   - Исторический: seaLevelRise 0.001-0.003 м/год
   - RCP4.5: seaLevelRise 0.003-0.006 м/год
   - RCP8.5: seaLevelRise 0.006-0.01 м/год

4. **Сезонность**:
   - Важна для средних широт (40-60°)
   - Используйте phase=π для максимум зимой
   - Используйте phase=0 для максимум летом
