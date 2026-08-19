# Калибровка и верификация модели эрозии

> Внимание: команды `lito benchmark calibrate` и связанные с ними исторические
> сценарии используют прежнюю параметрическую эвристику. Они полезны для
> воспроизведения старых сравнений, но не являются калибровкой физической
> one-line CERC-модели. Для нового расчётного конвейера используйте
> `lito calibrate-cerc` с фактическим волновым рядом, батиметрией и
> независимыми наблюдениями скорости линии берега; формат описан в
> [`cerc-one-line-model.md`](cerc-one-line-model.md). Даже после описанных ниже
> проверок этот исторический подбор на 4–6 наблюдениях не является основанием
> заявлять высокую прогностическую способность модели.

**Краткое руководство по научному использованию Litora-CLI для моделирования береговой эрозии.**

## Содержание

- [Обзор](#обзор)
- [Быстрый старт](#быстрый-старт)
- [Benchmark sites](#benchmark-sites)
- [Калибровка модели](#калибровка-модели)
- [Межсайтовая проверка](#межсайтовая-проверка)
- [Sensitivity analysis](#sensitivity-analysis)
- [Bootstrap confidence intervals](#bootstrap-confidence-intervals)
- [Hotspot analysis](#hotspot-analysis)
- [Climate scenarios](#climate-scenarios)
- [Метрики качества](#метрики-качества)
- [Рекомендуемый workflow](#рекомендуемый-workflow)

---

## Обзор

Litora-CLI реализует исследовательский пайплайн подбора параметров модели волновой эрозии. Пайплайн включает:

1. **Benchmark sites** — 5 участков Чёрного моря с измеренными скоростями эрозии
2. **Grid search** — перебор 128 параметрических комбинаций (8 strengths × 16 directions)
3. **Пространственную отложенную проверку** — взвешенный RMSE, RMSE, MAE, MBE и R² только при достаточном числе точек
4. **Sensitivity analysis** — оценка влияния каждого параметра
5. **Bootstrap CI** — доверительные интервалы для параметров
6. **Hotspot analysis** — топ участков максимальной эрозии
7. **Climate scenarios** — RCP4.5/8.5 для 2050/2100, штормовые события

## Быстрый старт

```bash
# 1. Инициализация стандартных benchmark sites
./lito benchmark init

# 2. Калибровка всех сайтов с bathymetry + wave spectrum
./lito benchmark calibrate-all --spectrum-spread=30 --output=/tmp/calibration

# 3. Полный анализ одного сайта
./lito benchmark analyze --site=kobuleti-ge --output=analysis.json

# 4. Анализ hotspots
./lito benchmark hotspots --site=kobuleti-ge --erosion-strength=15 --wave-direction=22

# 5. Climate scenarios
./lito benchmark scenarios --site=kobuleti-ge --erosion-strength=15 --wave-direction=22
```

## Benchmark sites

Стандартные сайты Чёрного моря с реальными observation данными:

| Site | Country | Type | Wave dir | Years |
|------|---------|------|----------|-------|
| odessa-coast-ua | Ukraine | sandy | E (90°) | 1975-2020 |
| kobuleti-ge | Georgia | sandy | ESE (120°) | 1990-2023 |
| balchik-bg | Bulgaria | mixed | ENE (60°) | 1985-2022 |
| samsun-tr | Turkey | muddy | NE (45°) | 1990-2021 |
| anapa-ru | Russia | sandy | E (100°) | 1960-2020 |

**Команды:**

```bash
# Список сайтов
./lito benchmark list

# Детальная информация
./lito benchmark show --site=kobuleti-ge

# Извлечение сегмента по координатам
./lito benchmark extract \
  --bounds-min-lat=41.7 --bounds-max-lat=41.9 \
  --bounds-min-lon=41.6 --bounds-max-lon=42.0 \
  --output=custom_segment.json
```

## Калибровка модели

### Базовая калибровка

```bash
./lito benchmark calibrate --site=kobuleti-ge
```

Вывод:
- Топ-5 параметрических комбинаций, выбранных по взвешенному RMSE обучения
- Отдельный взвешенный RMSE пространственно отложенной выборки
- Число принятых и исключённых по `MaxDistanceKm` наблюдений
- Сравнение «модель—наблюдение» с проекцией на сегмент, дистанцией и неопределённостью
- JSON-отчёт и TSV-диагностика каждого исходного наблюдения в `output/metrics/` и `output/csv/`

Наблюдение сопоставляется с ближайшей точкой **сегмента** береговой линии, а
не с ближайшей вершиной. Точки дальше 5 км по умолчанию исключаются до поиска
параметров; для команды предел задаётся флагом `--max-distance-km` (и полем
`CalibrationConfig.MaxDistanceKm` в коде). Ошибка для
выбора параметров взвешивается как `1/σ²`, где `σ` — неопределённость
наблюдения. Отложенная выборка формируется детерминированно вдоль береговой
линии; на очень малом числе точек (обычно одна точка проверки) R² и p-значение
намеренно не интерпретируются.

Даже при точном расчёте t-критерия статистический вывод включается только для
10 и более наблюдений; при меньшем числе p-значение остаётся технической
диагностикой и не выводится как значимость.

Модель также использует продолжительность из `start_date` и `end_date` каждой
точки. Она рассчитывается до самого длинного принятого периода, а для остальных
дат берётся интерполированный снимок. Наблюдение без корректного периода не
получает подставной горизонт в 10 лет, а исключается с явной причиной в
диагностике. Это синхронизирует длительность модельной и наблюдаемой скорости,
но не заменяет фактический временной ряд волн для соответствующего периода.

До запуска выполняется контроль качества: исключаются точки с нулевой или
отсутствующей неопределённостью скорости, а в отчёте появляются предупреждения
о менее чем четырёх принятых точках, совпадающих сегментах и дистанциях свыше
1 км. При таких предупреждениях результат допустим только как диагностическая
подгонка.

Используйте `--validation-fraction` для доли пространственно отложенных точек
(по умолчанию 0.25). Значение `0` допустимо только для диагностического
воспроизведения старых расчётов: в таком режиме команда не даёт независимой
проверочной метрики.

### Диагностика сопоставления

После каждого `benchmark calibrate` команда сохраняет JSON-отчёт в
`output/metrics/benchmark-calibration-<site>.json` и TSV-таблицу в
`output/csv/benchmark-calibration-<site>-diagnostics.tsv`. Таблица содержит
каждое исходное наблюдение, его срок, дистанцию до профиля, номер и положение
сегмента, принадлежность к обучению/проверке и причину исключения. Путь JSON
можно переопределить флагом `--output`; TSV всегда остаётся в `output/csv/`.

### С bathymetry

```bash
# Явное указание
./lito benchmark calibrate --site=kobuleti-ge \
  --bathymetry=data/black-sea-bathymetry.json

# Или автоматически (если файл существует)
./lito benchmark calibrate --site=kobuleti-ge
```

### С wave spectrum

Для учёта нескольких направлений волн вместо одного:

```bash
# 30° directional spread (умеренный, рекомендуется для Чёрного моря)
./lito benchmark calibrate-all --spectrum-spread=30

# 60° directional spread (широкий, для открытых побережий)
./lito benchmark calibrate-all --spectrum-spread=60
```

### Калибровка всех сайтов

```bash
./lito benchmark calibrate-all \
  --spectrum-spread=30 \
  --output=/tmp/calibration_results
```

Выводит агрегированную статистику + сохраняет individual reports.

## Межсайтовая проверка

Команда leave-one-site-out проверяет, переносится ли одна комбинация параметров
на независимый эталон. Для каждого сайта она выбирает силу эрозии и направление
волны только по всем **остальным** пригодным сайтам, а затем вычисляет ошибку на
исключённом сайте. Его наблюдения не участвуют в выборе параметров.

```bash
./lito benchmark cross-validate \
  --max-distance-km=1 \
  --output=cross-validation.json
```

В отчёте для каждого исключённого сайта указываются параметры, взвешенная ошибка
обучения и внешняя ошибка. JSON по умолчанию сохраняется в
`output/metrics/benchmark-cross-validation.json`. Эталоны без пригодных точек
не прерывают весь эксперимент, а перечисляются как пропущенные с причиной.

Это более строгая проверка, чем локальная пространственная отложенная выборка,
но она всё ещё не доказывает универсальность модели: сайты различаются
литологией, волновым климатом и качеством наблюдений.

## Sensitivity analysis

Локальный OAT (one-at-a-time) анализ:

```bash
./lito benchmark analyze --site=kobuleti-ge --output=analysis.json
```

Выводит для каждого параметра:
- **Sensitivity score** (0=insensitive, 1=highly sensitive)
- **Local derivative** (RMSE change per unit parameter)
- **Best value** (минимальный RMSE)
- **Worst value** (максимальный RMSE)

Пример:
```
erosion_strength_m:
  Range tested:    1.00 to 30.00
  Sensitivity:     0.874 (highly sensitive)
  Local derivative: +0.0267 (RMSE change per unit parameter)
```

## Bootstrap confidence intervals

200 итераций resampling с возвращением:

```bash
./lito benchmark analyze --site=samsun-tr --output=analysis.json
```

Вывод:
- **Best fit**: одно значение (как в обычной калибровке)
- **Mean ± StdDev**: распределение bootstrap
- **Median**: робастная оценка центра
- **68% CI**: ~1 sigma диапазон
- **95% CI**: 2.5-97.5 перцентили

Пример (Samsun):
```
Erosion strength (m):
  Best fit:       5.00
  Mean ± StdDev:  5.51 ± 1.67
  95% CI:         [5.00, 10.00]
Wave direction (°):
  Best fit:       292.50
  Mean ± StdDev:  289.12 ± 21.84
  95% CI:         [267.75, 315.00]
```

## Hotspot analysis

Топ-N участков с максимальной эрозией:

```bash
./lito benchmark hotspots \
  --site=kobuleti-ge \
  --erosion-strength=15 \
  --wave-direction=22 \
  --output=hotspots.json
```

Вывод:
- Top 5 hotspots с координатами, mean retreat, max retreat, length
- Общая статистика (% eroding, mean/max retreat)

## Climate scenarios

```bash
./lito benchmark scenarios \
  --site=kobuleti-ge \
  --erosion-strength=15 \
  --wave-direction=22 \
  --output=scenarios.json
```

5 предопределённых сценариев:
- **baseline**: текущие условия
- **rcp45_2050**: RCP4.5 для 2050 (+8% wind, +5mm/yr SLR)
- **rcp85_2050**: RCP8.5 для 2050 (+17% wind, +8mm/yr SLR)
- **rcp85_2100**: RCP8.5 для 2100 (+33% wind, +12mm/yr SLR)
- **storm_surge**: шторм 1-in-100 лет (25 m/s wind)

Выводит delta от baseline и hotspot shifts.

## Метрики качества

| Метрика | Описание | Хорошее значение |
|---------|----------|------------------|
| **Взвешенный RMSE** | RMSE с весами `1/σ²` | Сравнивать только в пределах одного набора наблюдений |
| **RMSE** | Среднеквадратичная ошибка | Описательная метрика, не критерий доказательства |
| **MAE** | Mean Absolute Error | < 0.4 m/year |
| **MBE** | Mean Bias Error (positive = overestimate) | |MBE| < 0.2 |
| **R²** | Коэффициент детерминации | Только описательно при малой проверочной выборке |
| **p-value** | Корреляция Пирсона с точным t-распределением и поправкой Бонферрони | Не публиковать как доказательство при малой выборке |
| **Skill score** | Улучшение vs null model | > 0.2 |

### Интерпретация результатов

```
Метрики исторической калибровки нельзя превращать в шкалу «excellent/good»:
перебор 128 комбинаций создаёт оптимистическое смещение, а 4–6 наблюдений не
дают устойчивой внешней оценки. Для научного вывода нужны независимые участки
или временные периоды и физический CERC-конвейер с фактическим волновым рядом.
```

## Рекомендуемый workflow

### Для научной публикации

```bash
# 1. Калибровка всех сайтов
./lito benchmark calibrate-all --spectrum-spread=30 \
  --output=results/calibration

# 2. Полный анализ лучшего сайта
./lito benchmark analyze --site=kobuleti-ge \
  --output=results/kobuleti_analysis.json

# 3. Hotspot analysis для prioritization
./lito benchmark hotspots --site=kobuleti-ge \
  --erosion-strength=15 --wave-direction=22 \
  --output=results/kobuleti_hotspots.json

# 4. Climate projections
./lito benchmark scenarios --site=kobuleti-ge \
  --erosion-strength=15 --wave-direction=22 \
  --output=results/kobuleti_scenarios.json
```

### Для практического применения

```bash
# Identify vulnerable coast
./lito benchmark hotspots --site=YOUR_SITE \
  --erosion-strength=10 --wave-direction=90

# Estimate climate risk
./lito benchmark scenarios --site=YOUR_SITE \
  --erosion-strength=10 --wave-direction=90

# Plan mitigation
# Use hotspot locations + climate projections for coastal protection prioritization
```

## Источники данных

### Observation data

Реальные observation данные (4-6 точек на сайт) основаны на научных публикациях:

- **Odessa**: Zhytar (2021), Koltunov et al. (2019)
- **Kobuleti**: Kiknadze et al. (2017), Georgian NEA
- **Balchik**: Valchev et al. (2018), Bulgarian Academy
- **Samsun**: Gürol & Çefne (2018), Turkish SMS
- **Anapa**: Kosyan & Krylenko (2018, 2022), Shirshov Institute

### Coastline data

- Источник: Marine Regions (marineregions.org)
- 9635 точек для Чёрного моря
- Автокэширование в `data/cache/black-sea.geojson`

### Bathymetry

- Источник: GEBCO
- 458,571 точек сетки
- Разрешение: ~0.01° (~1.1 км)

## Цитирование

Если используете Litora-CLI в научной работе:

```bibtex
@software{litora-cli,
  title={Litora-CLI: Geomorphological modeling of coastal systems},
  author={Nickitas},
  year={2026},
  url={https://github.com/Nickitas/litora-cli}
}
```

## Ссылки

- [Development plan](../todo/DEVELOPMENT_PLAN.md)
- [Main README](../README.md)
- [Wave spectrum API](../internal/domain/geometry/wave_spectrum.go)
- [Calibration API](../internal/domain/benchmark/calibration.go)
