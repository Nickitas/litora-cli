# Материалы к статье: выбор генератора адаптивной сетки

Этот каталог содержит воспроизводимые результаты для статьи
`research-notes/obsidian/articles/Статья 2.  Математико-алгоритмичное моделирование береговой линии.md`.
Все пути ниже заданы от корня проекта.

## Полученные серии

| Серия | Отчёт | Генераторы | Назначение |
|---|---|---|---|
| `reproduction-detailed/` | `seabed/adaptive/comparison/adaptive-generator-comparison.json` и TSV | Delaunay, Frontal-Delaunay for Quads | подробная сетка 200–1000 м |
| `reproduction-coarse/` | `seabed/adaptive/comparison/adaptive-generator-comparison.json` и TSV | Delaunay, Frontal-Delaunay for Quads | укрупнённая сетка 500–1000 м |
| `reference/accepted-adapt-03/` | принятые JSON/TSV и журналы | Packing of Parallelograms | фиксирует отказ: 10-минутный лимит превышен на обоих уровнях, MSH не создан |

Воспроизведённые отчёты подтверждают первое место Delaunay на обоих уровнях.
Числа времени и пиковой памяти могут незначительно меняться от запуска к
запуску; геометрические и батиметрические метрики фиксированы одними входами.

## Главный вывод статьи

По суммарному отклонению площадей существенных береговых особенностей и
итоговому многокритериальному баллу для модели всей акватории следует применять
**Delaunay, 500–1000 м**: $\delta_A=78,18\%$, $S=72,51$, 1 191 504 ячейки.
Если первична точность восстановления глубин, применяют **Delaunay, 200–1000
м**: RMSE глубины 0,333 м при 3 041 643 ячейках.

## Иллюстрации

| Файл | Место в статье |
|---|---|
| `figures/fig_01_normalized_coastline_box_counting.svg` | рисунок 1: нормализованный контур и покрытие box-counting |
| `figures/fig_02_box_counting_regression.svg` | рисунок 2: log-log регрессия box-counting |
| `figures/fig_03_bathymetry_overview.svg` | рисунок 3: обзорная батиметрическая карта |
| `figures/fig_04_adaptive_size_field.svg` | рисунок 4: поле требуемого размера |
| `figures/fig_05_actual_mesh_fragments.svg` | рисунок 5: фактические фрагменты сетки |
| `figures/fig_06a_delaunay_coarse_kizilirmak.svg` | рисунок 6а: Delaunay, 500–1000 м |
| `figures/fig_06b_frontal_quad_coarse_kizilirmak.svg` | рисунок 6б: Frontal-Delaunay for Quads, 500–1000 м |

Карточки были построены командой AI-01 из одних и тех же окон, берегов и
изобат. В каталогах `figures/detailed/` и `figures/coarse/` остаются закрытые
ключи соответствия; их не следует передавать независимым экспертам.

## Журналы

Все вызовы CLI сохранены в `logs/`; `09-dimension-full-black-sea.log` фиксирует
создание рисунков 1–2 по наблюдаемому контуру. Первая объединённая попытка
`01-compare-adaptive.log` ограничена временем оболочки после успешного
подробного уровня; поэтому в отдельные каталоги вынесены завершённые
воспроизводимые серии `reproduction-detailed/` и `reproduction-coarse/`.
