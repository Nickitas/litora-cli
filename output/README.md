# Локальные результаты запусков

Каталог предназначен для результатов локальных запусков Lito: SVG, CSV, GIF,
GEO/MSH-сеток, отчётов, метрик и журналов. Расчётные 2D-сетки, сценарии и
сравнение генераторов находятся в `output/mesh/`. Содержимое игнорируется Git и
может быть удалено перед новым экспериментом.

Батиметрические MSH, VTU/CSV и отчёты качества сохраняются только в
`output/seabed/`. Целевой файл EXPORT-01 — `output/seabed/black-sea-depth.msh`;
он должен иметь схему `lito-seabed/v1`, а не плоскую `lito-mesh/v1`.
EXPORT-02 добавляет рядом `black-sea-depth.vtu`, `nodes.csv`, `cells.csv`,
`profiles.csv` и `export-metadata.json`.
VIEW-01 создаёт `seabed/svg/bathymetry-overview.svg`; внутренние рёбра на
обзорной карте скрыты. VIEW-02 добавляет `seabed/svg/mesh-details.svg` с тремя
непрореженными фрагментами фактической сетки. Общий машинный отчёт
`seabed/bathymetry-overview.json` имеет схему
`lito-bathymetry-visualization/v3`. VIEW-03 создаёт
`seabed/svg/seabed-3d.svg`, `seabed/svg/profiles.svg` и обновляет
`seabed/profiles.csv` тремя автоматическими трассами «берег → глубоководье».
Отчёт содержит пути, вертикальное преувеличение и метрики всех визуализаций.
ADAPT-01 создаёт узловое поле будущего размера в
`seabed/adaptive/size-field.csv`, отчёт схемы `lito-adaptive-size-field/v1` в
`seabed/adaptive/size-field.json` и карту `seabed/svg/size-field.svg`.
Внутренние рёбра исходного каркаса на карте скрыты, а
`adaptive_mesh_generated=false` не позволяет принять поле за уже созданную
адаптивную сетку. Фактический адаптивный MSH относится к ADAPT-02.

Для публикации результата используйте отдельный согласованный каталог или
репозиторий данных; не добавляйте производные файлы в основной научный контур.
