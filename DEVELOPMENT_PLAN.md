# План развития Litora-CLI — Научная утилита для геоморфологического моделирования

## Текущий статус (Июнь 2026)

### ✅ Выполнено (Phase 1 — Научное ядро)

- ✅ Волновая эрозия с fetch distance и экспозицией
- ✅ Батиметрическая интеграция (GEBCO данные Чёрного моря)
- ✅ Направленная эрозия (мысы vs бухты)
- ✅ Транспорт наносов (longshore drift, баланс массы)
- ✅ Литологический модуль (сопротивление пород, IDW интерполяция)
- ✅ Временная динамика (штормы, сезонность, подъём уровня моря)
- ✅ Фрактальный анализ (box-counting с усреднением по сеткам)
- ✅ Методы аппроксимации (IDW, Regular Grid, билинейная интерполяция)
- ✅ **РЕФАКТОРИНГ:** Удалены демонстрационные команды (paradox, koch, koch-organic)
- ✅ **РЕФАКТОРИНГ:** CLI ориентирован на научные сценарии
- ✅ **РЕФАКТОРИНГ:** README обновлён для научной аудитории
- ✅ **РЕФАКТОРИНГ:** Полностью удалены legacy aliases и демонстрационные возможности

### 🔨 Требует доработки (Phase 2 — Улучшение инструментов)

- ❌ CSV отчёты (Приоритет 1)
- ❌ GIF анимация (Приоритет 1)
- ❌ Метрики качества модели (Приоритет 1)
- ❌ TIN approximation mesh (Приоритет 2)

### 📋 Отложено (Phase 3 — Доступность)

- ❌ WASM веб-версия (низкий приоритет для научной утилиты)

---

## Детальный план реализации

### ✅ Phase 1: Научное ядро (ЗАВЕРШЕНО)

**Цель:** Превратить из геометрической генерации в geomorphological модель

| Неделя | Задача | Статус | Результат |
|--------|--------|--------|-----------|
| 1-2 | Транспорт наносов | ✅ | `sediment.go`, баланс массы |
| 3-4 | Литология | ✅ | `lithology.go`, профиль Чёрного моря |
| 5-6 | Временная динамика | ✅ | `temporal.go`, годы вместо шагов |
| 7-8 | Метрики качества | ⏳ | `validation.go`, научная валидация |
| 9-10 | Рефакторинг CLI | ✅ | Удалены demo команды, научный фокус |
| 11-12 | Обновление документации | ✅ | README.md для научной аудитории |

**Milestone 1:** ✅ Litora-CLI может моделировать реальную geomorphology с балансом массы

---

### 🔨 Phase 2: Улучшение инструментов (3-4 недели)

**Цель:** Улучшить юзабилити для научных исследований

#### 2.1 CSV отчёты (Приоритет 1)

**Зачем:** Количественный анализ, графики в Excel/Python

**Структура CSV для временной динамики:**
```csv
year,step,length_km,area_km2,eroded_m3,deposited_m3,net_change_m3,storm_event,sea_level_m
0,0,2345.6,12345.0,0,0,0,false,0.0
1,1,2344.8,12344.2,1200.5,800.3,-400.2,true,0.02
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
./lito model erosion \
  --output-csv erosion_metrics.csv \
  --csv-format long  # long | wide
```

#### 2.2 GIF анимация (Приоритет 1)

**Зачем:** Визуализация динамики эрозии во времени

**Реализация:**
```go
// Использовать библиотеку imgo или ee_gif
func generateErosionGIF(
    snapshots [][]LatLon,
    outputPath string,
    fps int,
    skipEvery int,
) error
```

**Параметры:**
```bash
./lito model erosion \
  --steps 100 \
  --output-gif \
  --gif-fps 10 \
  --gif-skip 2
```

#### 2.3 Метрики качества модели (Приоритет 1)

**Зачем:** Научная валидация

**Метрики:**
```go
type ModelQualityMetrics struct {
    DimensionStability float64  // изменение D во времени
    MassBalance        float64  // eroded - deposited
    SpatialAutocorr    float64  // корреляция соседних участков
    ConvergenceRate    float64  // скорость изменения метрик
}
```

**Реализация:**
- Создать `internal/domain/geometry/validation.go`
- Вычислять на каждом шаге симуляции
- Вывести в финальный отчёт

#### 2.4 TIN approximation mesh (Приоритет 2)

**Зачем:** Более точная аппроксимация сложной береговой линии

**Реализация:**
```go
type ApproximationMesh struct {
    Type             string  // "regular" | "tin" | "adaptive"
    Resolution       float64 // для regular
    MaxTriangleArea  float64 // для TIN
    ErrorTolerance   float64 // для adaptive
}

func BuildTINMesh(points []LatLon, opts ApproximationOptions) (*TINMesh, error)
```

**Milestone 2:** Удобный инструмент для научной работы

---

### 📋 Phase 3: WASM веб-версия (Отложено)

**Статус:** Низкий приоритет для научной утилиты

**Причина:** CLI больше подходит для научных исследований, чем веб-версия

**Возможное возобновление:** Если будет запрос от сообщества

---

## Архитектурные решения

### Удалённые демонстрационные компоненты

**Файлы удалены:**
- `cmd/generate-demo-bathymetry/` — генерация демо данных
- `cmd/lithology-example/` — пример литологии
- `cmd/temporal-example/` — пример временной динамики
- `examples/` — все демонстрационные примеры

**Команды CLI удалены:**
- `lito model paradox` — чисто образовательная демонстрация
- `lito model koch` — классическая Кох (математическая демонстрация)
- `lito model koch-organic` — органическая Кох (демонстрация)

**Сохранённые научные команды:**
- `lito source` — загрузка данных
- `lito real coastline` — базовая валидация и метрики
- `lito model dimension` — анализ фрактальной размерности
- `lito model erosion` — geomorphological моделирование
- `lito all` — полный научный сценарий

### Научная ориентация

**README.md обновлён:**
- Удалены демонстрационные примеры
- Добавлены научные сценарии использования
- Подробное описание временной динамики
- Секция "Методы аппроксимации и интерполяции"
- Фокус на geomorphological modelling

---

## Риски и митигация

### Риск 1: Слишком высокая сложность

**Митигация:**
- ✅ Разбить на независимые модули
- ✅ Каждый feature можно отключить флагом
- ✅ Минимум зависимостей между модулями

### Риск 2: Плохая производительность

**Митигация:**
- ✅ Параллельность вычислений эрозии
- ✅ Lazy loading тяжелых данных (батиметрия)
- ⏳ Профилирование перед оптимизацией

### Риск 3: Научная неверифицируемость

**Митигация:**
- ✅ Честная документация ограничений
- ✅ Ссылки на литературу для всех методов
- ⏳ Quality metrics как часть output

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

## Приоритеты на ближайшее время

### Немедленно (критично для научности):

1. ✅ ~~Удалить демонстрационные команды и файлы~~ (ВЫПОЛНЕНО)
2. ⏳ **Реализовать CSV экспорт метрик**
3. ⏳ **Добавить метрики качества модели**

### Важно (для научной работы):

4. ⏳ **Реализовать GIF анимацию**
5. ⏳ **Добавить TIN approximation mesh**
6. ⏳ **Профилирование и оптимизация**

### Желательно (для полноты):

7. ⏳ Дополнительная валидация на реальных данных
8. ⏳ Расширенная документация API
9. ⏳ Интеграция с Python/R для анализа

---

**Статус проекта:** ✅ Научное ядро завершено, Phase 2 в разработке
**Следующие шаги:** CSV экспорт → Метрики качества → GIF анимация
