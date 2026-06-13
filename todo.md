# Доработать до научности

Текущая эрозия — это **геометрическая генерация**, а не **геоморфологическая модель**.

## Прогресс (Январь 2026)

| Задача | Статус | Приоритет |
|--------|--------|-----------|
| 1. Волновая эрозия | ✅ ГОТОВО | P0 |
| 2. Транспорт наносов | ✅ ГОТОВО | P0 |
| 3. Литология | ✅ ГОТОВО | P0 |
| 4. Временная динамика | ❌ | P1 |
| 5. CSV отчёты | ❌ | P1 |
| 6. GIF анимация | ❌ | P2 |
| 7. Метрики качества | ❌ | P2 |
| 8. Профили регионов | ❌ | P2 |
| 9. WASM веб-версия | ❌ | P3 |
| 10. Документация API | ✅ ЧАСТИЧНО | P3 |

**P0 (критично для научности): 3/3 выполнено** ✅

---

## Детали задач

### 1. **Волновая эрозия** (Wave Action) ✅

**Физика:**
- Волны воздействуют на берег с энергией, зависящей от fetch (расстояния открытой воды), глубины, ветра
- Экспозиция берега к волнам определяется углом падения волн

**Реализация:** ✅
- ✅ Рассчитан **fetch** для каждой точки (ray casting)
- ✅ Определена **экспозиция** (мысы vs бухты)
- ✅ Применена **направленная эрозия** — открытые мысы размываются сильнее
- ✅ Добавлена **реальная батиметрия** Чёрного моря (GEBCO)
- ✅ **Graceful degradation:** если батиметрии нет, используется geometric proxy

**Код:** `internal/domain/geometry/erosion.go`, `bathymetry.go`

---

### 2. **Транспорт наносов** (Sediment Transport) ✅

**Физика:**
- Размытый материал не исчезает — он переносится вдоль берега (longshore drift)
- Осадки оседают в зонах затишья (бухты, мысы)
- Баланс эрозии/аккумуляции определяет форму берега

**Реализация:** ✅
- ✅ Создан `internal/domain/geometry/sediment.go`
- ✅ Реализован `SedimentBudget` — баланс массы (eroded, transport, deposited)
- ✅ **Longshore drift** — транспорт вдоль берега по касательной
- ✅ **Депозиция** — аккумуляция в бухтах при низкой скорости потока
- ✅ **Модуляция по литологии** — твёрдые породы меньше дают в транспорт
- ✅ **Валидация массы** — mass balance check (eroded ≈ deposited + transport)

**Код:** `internal/domain/geometry/sediment.go`, `sediment_test.go`

**Сложность:** ⭐⭐⭐ | **Время:** 2-3 недели | **Приоритет:** P0 | **Статус:** ГОТОВО

---

### 3. **Литология** (Lithology / Rock Resistance) ✅

**Физика:**
- Разные породы размываются с разной скоростью:
  - **Известняк/песчаник** — мягкие, быстрая эрозия
  - **Гранит/базальт** — твёрдые, медленная эрозия
  - **Глинистые берега** — очень быстрая эрозия
  - **Галечные пляжи** — динамическое равновесие

**Реализация:** ✅
- ✅ Создан `internal/domain/geometry/lithology.go`
- ✅ **IDW интерполяция** — inverse distance weighting для 6 ближайших точек
- ✅ **Профиль Чёрного моря** — `data/black-sea-lithology.json` (38 точек, 21 класс)
- ✅ **Дефолтный профиль** — `CreateDefaultBlackSeaProfile()` как fallback
- ✅ **Интеграция в erosion.go** — `retreatMeters /= lithology.Resistance`
- ✅ **WaveErosionOptions** — добавлены поля `LithologyProfile` и `EnableLithology`
- ✅ **21 класс пород** — от serpentinite (R=9.0) до deltaic_sediment (R=0.8)
- ✅ **Документация** — обновлены README.md и internal/domain/geometry/README.md
- ✅ **Пример использования** — `examples/lithology_demo.go`

**Код:** `internal/domain/geometry/lithology.go`, `lithology_test.go`

**Сложность:** ⭐⭐ | **Время:** 1-2 недели | **Приоритет:** P0 | **Статус:** ГОТОВО

---

### 4. **Временная динамика** (Temporal Dynamics) ❌

**Физика:**
- Эрозия — процесс во времени (годы/десятилетия)
- Штормовые события vs фоновая эрозия
- Sea-level rise (подъём уровня моря) усиливает эрозию
- Сезонные колебания (зимние штормы vs летний штиль)

**Реализация:**
```go
// Создать: internal/domain/geometry/temporal.go
type TemporalParameters struct {
    YearsPerStep        float64  // лет за один шаг модели
    StormProbability    float64  // вероятность шторма за шаг
    StormIntensityMult  float64  // множитель силы шторма
    SeaLevelRise        float64  // м/год
    Seasonality         bool     // учитывать сезонность
}

// Конвертация шагов в годы
func SimulateErosionWithDuration(
    points []LatLon,
    targetYears int,
    params TemporalParameters,
) [][]LatLon

// Сезонная модель
seasonalFactor = 1.0 + 0.5×sin(2π × year + phase)
```

**Сложность:** ⭐⭐⭐ | **Время:** 2-3 недели | **Приоритет:** P1

---

## 📊 ВАЖНО ДЛЯ ИНСТРУМЕНТОВ

### 5. **CSV отчёты** ❌

**Зачем:** Анализ в Excel/Python/R, построение графиков

**Реализация:**
```go
func writeErosionCSV(metrics []ErosionStepMetrics, path string) error
```

**Формат:**
```csv
year,step,length_km,area_km2,eroded_m3,deposited_m3,net_change_m3,dimension
0,0,2345.6,12345.0,0,0,0,1.12
1,1,2344.8,12344.2,1200.5,800.3,-400.2,1.13
...
```

**CLI:**
```bash
./fraes model erosion --output-csv results.csv
```

**Сложность:** ⭐ | **Время:** 3-4 дня | **Приоритет:** P1

---

### 6. **GIF анимация** ❌

**Зачем:** Презентации, визуализация динамики

**Реализация:**
```go
// Использовать библиотеку image/gif
func generateErosionGIF(
    snapshots [][]LatLon,
    outputPath string,
    fps int,
    skipEvery int,
) error
```

**CLI:**
```bash
./fraes model erosion --output-gif erosion.gif --gif-fps 10 --gif-skip 2
```

**Сложность:** ⭐⭐ | **Время:** 1 неделя | **Приоритет:** P2

---

### 7. **Метрики качества модели** ❌

**Зачем:** Научная валидация

**Метрики:**
```go
type ModelQualityMetrics struct {
    MassBalance        float64  // eroded - deposited ≈ 0?
    DimensionStability float64  // изменение D во времени
    SpatialAutocorr    float64  // корреляция соседних участков
    ConvergenceRate    float64  // скорость изменения метрик
}
```

**Сложность:** ⭐⭐ | **Время:** 1 неделя | **Приоритет:** P2

---

## 🌐 ПОЛЕЗНО ДЛЯ ДОСТУПНОСТИ

### 8. **Профили регионов** ❌

**Зачем:** Quick start для разных морей

**Структура:**
```
data/profiles/
  black-sea.json
  mediterranean.json
  caspian.json
```

**Формат:**
```json
{
  "name": "Чёрное море",
  "bounds": [40, 47, 27, 42],
  "bathymetry": "data/black-sea-bathymetry.json",
  "lithology": "data/black-sea-lithology.json",
  "typical_params": {
    "wave_direction": 0,
    "wind_speed": 12,
    "erosion_strength": 30
  }
}
```

**CLI:**
```bash
./fraes model erosion --profile black-sea
```

**Сложность:** ⭐⭐ | **Время:** 1 неделя | **Приоритет:** P2

---

### 9. **WASM веб-версия** ❌

**Зачем:** Демо, доступность без установки

**Архитектура:**
```
[React Frontend] → [JS Bridge] → [Go Code (WASM)]
```

**Этапы:**
1. Рефакторинг для чистого Go (убрать `os`-зависимости)
2. Компиляция: `GOOS=js GOARCH=wasm go build`
3. JavaScript bridge
4. UI: React компоненты

**Ограничения:**
- Нет файловой системы
- Батиметрия embedded в бинарник
- Производительность ниже нативной

**Сложность:** ⭐⭐⭐⭐ | **Время:** 4-6 недель | **Приоритет:** P3

---

### 10. **Документация API** ✅ (частично)

**Зачем:** Интеграция с Python (через cgo или subprocess)

**Реализация:** ✅
- ✅ Обновлён `internal/domain/geometry/README.md` — полная документация модулей
- ✅ Добавлен раздел **"Литологический модуль"** с формулами и примерами
- ✅ Публичный API документирован с таблицами функций
- ✅ Обновлён `README.md` с примерами использования литологии
- ✅ Создан `examples/lithology_demo.go` — рабочий пример кода
- ❌ Godoc комментарии (частично)
- ❌ Python wrapper (не реализовано)

**Сложность:** ⭐ | **Время:** 3-4 дня | **Приоритет:** P3 | **Статус:** ЧАСТИЧНО ГОТОВО

---

## 📚 БИБЛИОГРАФИЯ

### Coastal Geomorphology
1. Komar, P. D. (1998). *Beach Processes and Sedimentation*.
2. Davidson-Arnott, R. (2010). *Introduction to Coastal Processes and Geomorphology*.

### Sediment Transport
3. Soulsby, R. (1997). *Dynamics of Marine Sands*.
4. Van Rijn, L. C. (1993). *Principles of Sediment Transport in Rivers, Estuaries and Coastal Seas*.

### Wave Mechanics
5. US Army Corps of Engineers (1984). *Shore Protection Manual*.
6. Holthuijsen, L. H. (2007). *Waves in Oceanic and Coastal Waters*.

### Fractal Analysis
7. Falconer, K. (2013). *Fractal Geometry: Mathematical Foundations and Applications*.
8. Mandelbrot, B. B. (1982). *The Fractal Geometry of Nature*.

---

Книга по фрактальной геометрии 
https://www.ma.imperial.ac.uk/~jswlamb/M345PA46/Fractal%20Geometry_%20Mathematical%20Foundations%20and%20Applications%20-%20Kenneth%20Falconer.pdf
