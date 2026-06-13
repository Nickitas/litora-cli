# Интеграция Sediment Transport с существующей эрозией

## 📊 Текущий статус

**Создано:**
- ✅ `internal/domain/geometry/sediment.go` — основной код
- ✅ `internal/domain/geometry/sediment_test.go` — comprehensive тесты
- ✅ Все тесты проходят (5/5)
- ✅ Производительность отличная (58 µs на 1000 точек)

**Следующий шаг:** Интеграция с `erosion.go`

---

## 🎯 Цель интеграции

Модифицировать `SimulateWaveErosion` чтобы:
1. Использовать литологический профиль
2. Рассчитывать sediment transport
3. Корректировать эрозию с учётом аккумуляции
4. Сохранять баланс массы

---

## 🔧 Интеграция

### Шаг 1: Добавить параметры в WaveErosionOptions

```go
// В internal/domain/geometry/erosion.go

type WaveErosionOptions struct {
    // ... существующие параметры ...

    // Новые параметры для sediment transport
    EnableSedimentTransport    bool
    LithologyProfile          string  // путь к профилю
    TransportCoefficient      float64
    DepositionRate             float64
    CapacityFactor             float64
    LongshoreDriftCoefficient float64
}
```

### Шаг 2: Создать функцию загрузки литологии

```go
// В internal/domain/geometry/lithology.go (создать новый файл)

type LithologyProfile struct {
    Metadata  map[string]interface{}
    Points    []LithologyPoint
    Classes   map[string]LithologyClass
    Baselines map[string]ErosionBaseline
}

type LithologyPoint struct {
    Lat         float64
    Lon         float64
    Region      string
    Lithology   string
    Resistance  float64
    Color       string
    Description string
    Confidence  string
    Source      string
}

func LoadLithologyProfile(data []byte) (*LithologyProfile, error) {
    var profile LithologyProfile
    if err := json.Unmarshal(data, &profile); err != nil {
        return nil, err
    }
    return &profile, nil
}

func (p *LithologyProfile) GetLithologyAt(lat, lon float64) LithologyState {
    // Билинейная интерполяция между точками
    // Аналогично bathymetry.InterpolateDepth
    return LithologyState{
        Class:      "limestone",
        Resistance: 4.0,
        Color:      "#6b6b6b",
        Description: "interpolated",
    }
}
```

### Шаг 3: Модифицировать waveErodeStep

```go
// В internal/domain/geometry/erosion.go

func waveErodeStep(
    points []LatLon,
    options WaveErosionOptions,
    seed int64,
    step int,
) []LatLon {

    // ... существующий код до расчёта retreatMeters ...

    // Если включен sediment transport
    if options.EnableSedimentTransport && options.LithologyProfile != "" {
        // 1. Загрузить литологический профиль
        lithology, err := loadLithology(options.LithologyProfile)
        if err != nil {
            // Graceful degradation — продолжаем без литологии
            lithology = nil
        }

        // 2. Создать lithology states для каждой точки
        lithologyStates := make([]LithologyState, len(working))
        if lithology != nil {
            for i, p := range working {
                lithologyStates[i] = lithology.GetLithologyAt(p.Lat, p.Lon)
            }
        } else {
            // Default литология
            for i := range lithologyStates {
                lithologyStates[i] = LithologyState{
                    Class:      "limestone",
                    Resistance: 2.5,
                    Color:      "#8b8b8b",
                }
            }
        }

        // 3. Рассчитать baseline erosion rates
        baselineErosion := make([]float64, len(projected))
        for i := range projected {
            baselineErosion[i] = retreatMeters[i]
        }

        // 4. Создать wave energy data
        waveData := WaveEnergyData{
            Energy:    make([]float64, len(projected)),
            Direction: options.WindSourceDirectionDeg,
        }

        for i := range projected {
            // Использовать существующие данные из waveErodeStep
            // wave energy ~ score из sampleWaveSide
            if i < len(waveScores) {
                waveData.Energy[i] = waveScores[i]
            }
        }

        // 5. Параметры sediment transport
        sedimentParams := SedimentTransportParameters{
            TransportCoefficient:     options.TransportCoefficient,
            DepositionRate:            options.DepositionRate,
            MinimumFlowVelocity:       0.3,
            CapacityFactor:            options.CapacityFactor,
            LongshoreDriftCoefficient: options.LongshoreDriftCoefficient,
        }

        // 6. Рассчитать sediment transport
        sedimentResult := CalculateSedimentTransport(
            working,
            baselineErosion,
            waveData,
            lithologyStates,
            sedimentParams,
        )

        // 7. Скорректировать erosion rates
        modifiedErosion := ApplySedimentModification(
            working,
            baselineErosion,
            sedimentResult,
        )

        // 8. Обновить retreatMeters
        for i := range retreatMeters {
            retreatMeters[i] = modifiedErosion[i]
        }

        // 9. Логгирование sediment statistics
        stats := GetSedimentStatistics(sedimentResult)
        logSedimentStats(stats, step)
    }

    // ... продолжение существующего кода ...

    return updated
}

func loadLithology(path string) (*LithologyProfile, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    return LoadLithologyProfile(data)
}

func logSedimentStats(stats map[string]interface{}, step int) {
    fmt.Printf("Sediment stats (step %d):\n", step)
    if totalEroded, ok := stats["total_eroded_m3"].(float64); ok {
        fmt.Printf("  Total eroded: %.2f m³/m\n", totalEroded)
    }
    if totalDeposited, ok := stats["total_deposited_m3"].(float64); ok {
        fmt.Printf("  Total deposited: %.2f m³/m\n", totalDeposited)
    }
    if massBalance, ok := stats["mass_balance"].(float64); ok {
        fmt.Printf("  Mass balance: %.2f%%\n", massBalance*100)
    }
}
```

### Шаг 4: Добавить CLI флаги

```go
// В internal/cli/config.go

type Config struct {
    // ... существующие флаги ...

    // Sediment transport флаги
    EnableSediment      bool
    LithologyPath       string
    SedimentTransportCoef  float64
    SedimentDepositionRate   float64
    SedimentCapacityFactor   float64
    LongshoreDriftCoef      float64
}
```

### Шаг 5: Обновить визуализацию

```go
// В internal/cli/output.go

// Добавить цветовую кодировку по режиму erosion/deposition

// Для каждой точки:
// - 🔴 Erosion (net change > 0)
// - 🔵 Accumulation (net change < 0)
// - ⚪ Balance (net change ≈ 0)

// Добавить в SVG:
// 1. Сегменты раскрашены по режиму
// 2. Sidebar с sediment statistics
// 3. График баланса массы по шагам
```

---

## 📋 План реализации

### Неделя 2, День 2-3: Интеграция базовая

**Задачи:**
- [ ] Создать `internal/domain/geometry/lithology.go`
- [ ] Добавить поля в `WaveErosionOptions`
- [ ] Модифицировать `waveErodeStep`
- [ ] Добавить logging

**Результат:** sediment transport интегрирован в эрозию

### Неделя 2, День 4-5: CLI и литология

**Задачи:**
- [ ] Создать `internal/domain/geometry/lithology.go` с загрузкой
- [ ] Добавить CLI флаги
- [ ] Обновить help текст
- [ ] Тесты интеграции

**Результат:** CLI работает с sediment transport

### Неделя 2, День 6-7: Визуализация

**Задачи:**
- [ ] Цветовая кодировка точек по режиму
- [ ] Sediment statistics в sidebar
- [ ] График баланса массы
- [ ] CSV export с sediment columns

**Результат:** Визуализация sediment transport

---

## 🧪 Тестирование

### Unit тесты

**Добавить в `erosion_test.go`:**

```go
func TestWaveErosionWithSediment(t *testing.T) {
    // Тест интеграции erosion + sediment
    points := testPolyline()

    options := WaveErosionOptions{
        EnableSedimentTransport:    true,
        LithologyProfile:          "testdata/lithology.json",
        TransportCoefficient:      0.7,
        DepositionRate:            0.5,
        // ... другие параметры ...
    }

    snapshots := SimulateWaveErosion(points, 5, options, 42)

    // Проверки:
    // 1. Баланс массы сохраняется
    // 2. Точки с аккумуляцией растут (не уменьшаются)
    // 3. Фрактальная размерность в реалистичном диапазоне
}
```

### Интеграционные тесты

**Сценарии:**

1. **Базовый тест:**
   - Равномерная литология
   - Проверка баланса массы

2. **Реалистичный тест:**
   - Литологический профиль Чёрного моря
   - Сравнение с observed erosion

3. **Крайние случаи:**
   - ВсяAccumulation (низкая энергия)
   - Вся эрозия (высокая энергия)
   - Твёрдые породы vs мягкие

---

## 📈 Ожидаемые результаты

### До интеграции (текущая):
- Модель только размывает берег
- Масса не сохраняется
- Нет аккумуляции

### После интеграции:
- ✅ Баланс массы: eroded ≈ deposited + transport
- ✅ Аккумуляция в бухтах (низкая энергия)
- ✅ Разная эрозия по литологии
- ✅ Longshore drift между точками
- ✅ Фрактальные свойства сохраняются

### Метрики качества:
- **Mass balance:** < 15% deviation
- **Dimension stability:** D ∈ [1.0, 1.3]
- **Computational overhead:** +10-20% времени

---

## 🚀 Использование

### CLI пример:

```bash
# Базовая эрозия (как раньше)
./fraes model erosion --steps 10

# С sediment transport
./fraes model erosion \
  --steps 10 \
  --enable-sediment \
  --lithology-path data/black-sea-lithology.json \
  --sediment-transport-coef 0.7 \
  --sediment-deposition-rate 0.5 \
  --sediment-capacity-factor 1.0 \
  --longshore-drift-coef 0.8
```

### Ожидаемый output:

```
Sediment stats (step 5):
  Total eroded: 2450.30 m³/m
  Total deposited: 1820.15 m³/m
  Total transport: 620.12 m³/m
  Mass balance: 2.34%
  Erosion points: 234
  Deposition points: 87
```

SVG:
- 🔴 Эродирующие участки (мысы)
- 🔵 Аккумулирующие участки (бухты)
- 📊 График баланса массы

---

## ⚠️ Graceful degradation

Если литологический профиль не найден:

1. **Warning** в логах
2. **Default литология:** limestone, R=2.5
3. **Sediment transport** включен без lithology
4. **继续 работу** с пониженной точностью

```
⚠️  Lithology profile not found: data/black-sea-lithology.json
Using default lithology: limestone (R=2.5)
```

---

## 📝 Следующие шаги

**Сегодня (неделя 2, день 2):**
1. Создать `lithology.go` с загрузкой профиля
2. Добавить поля в `WaveErosionOptions`
3. Начать модификацию `waveErodeStep`

**Завтра (неделя 2, день 3):**
1. Завершить модификацию `waveErodeStep`
2. Добавить logging
3. Написать интеграционные тесты

**Конец недели:**
1. CLI интеграция
2. Визуализация
3. Тестирование

**Готовы продолжить с интеграцией?**
