# EXPORT-02: VTU и табличный экспорт модели дна

## Назначение

EXPORT-02 создаёт согласованный набор для численной проверки и дальнейшей
визуализации рельефа Чёрного моря:

- `black-sea-depth.vtu` — поверхность VTK XML UnstructuredGrid для ParaView;
- `nodes.csv` — полный узловой слой;
- `cells.csv` — производные характеристики четырёхугольников;
- `profiles.csv` — явно выбранные последовательности узлов профилей;
- `export-metadata.json` — удобное представление общих пространственных
  метаданных и таблиц кодов.

Набор создаёт `seabed.WriteExportBundle`. Отдельные экспортёры доступны как
`WriteVTU`, `WriteNodesCSV`, `WriteCellsCSV` и `WriteProfilesCSV`.

## Пространственные метаданные

`seabed.NewExportMetadata` принимает центр сферической LAEA-проекции, название
вертикальной системы и обязательную оговорку её источника. Во всех форматах
фиксируются:

- `lito-seabed/v1`;
- WGS 84 / EPSG:4326, порядок `longitude,latitude`, единица `degree`;
- `spherical_laea`, центр проекции и горизонтальная единица `m`;
- название и необязательный EPSG вертикальной системы;
- вертикальная единица `m`, положительное направление `up`;
- отсутствие вертикального увеличения в данных;
- отрицательный NoData sentinel, пороги регионов и таблицы кодов.

Lito не подставляет универсальный вертикальный датум. Для GEBCO следует
передавать формулировку и оговорку из паспорта конкретного производного набора.

## Структура VTU

Файл соответствует VTK XML `UnstructuredGrid` версии 0.1 и записывается в
ASCII для прозрачной проверки. Каждая точка содержит:

```text
X = x_m, Y = y_m, Z = elevation_m
```

Connectivity использует нулевые индексы VTK, но массивы `PointData/id` и
`CellData/id` сохраняют идентификаторы Lito, начинающиеся с 1. Все ячейки имеют
тип `VTK_QUAD = 9`.

### PointData

| Массив | Тип | Единица |
|---|---|---|
| `id` | `Int32` | 1 |
| `longitude_deg`, `latitude_deg` | `Float64` | degree |
| `elevation_m`, `water_depth_m` | `Float64` | m |
| `sampling_method_code` | `Int32` | code |
| `source_distance_m` | `Float64` | m |
| `quality_code`, `boundary_code` | `Int32` | code |
| `is_boundary` | `UInt8` | 1 |

### CellData

| Массив | Тип | Единица |
|---|---|---|
| `id` | `Int32` | 1 |
| `area_m2` | `Float64` | m2 |
| `elevation_min_m`, `elevation_max_m`, `elevation_mean_m` | `Float64` | m |
| `water_depth_mean_m` | `Float64` | m |
| `slope_deg`, `aspect_deg` | `Float64` | degree |
| `roughness_m` | `Float64` | m |
| `region_code`, `cell_quality_code` | `Int32` | code |
| `quality_score` | `Float64` | 1 |

Единицы хранятся в стандартном `InformationKey` с именем `UNITS_LABEL` и
расположением `vtkDataArray`. Активным узловым скаляром является
`water_depth_m`, активным ячеечным — `water_depth_mean_m`.

### FieldData

Формат `DataArray` допускает только числовые типы. Поэтому полный UTF-8 JSON
пространственных метаданных записывается как массив байтов `UInt8` с именем
`lito_metadata_utf8_json`. Это не зависит от нестандартной поддержки строковых
массивов. Рёбра границы сохраняются отдельным `Int32`-массивом
`lito_boundary_edges` с тройками:

```text
zero_based_node_a, zero_based_node_b, boundary_code
```

Спецификация числовых `DataArray`, `PointData`, `CellData` и connectivity:
[официальная документация VTK XML](https://docs.vtk.org/en/latest/vtk_file_formats/vtkxml_file_format.html).
Формат `InformationKey` и `UNITS_LABEL`:
[официальная документация VTK](https://docs.vtk.org/en/latest/design_documents/IOXMLInformationFormat.html).

## Таблицы CSV

`nodes.csv` и `cells.csv` начинаются с канонических полей контракта
`lito-seabed/v1`. После них заголовок содержит `record_type` и колонки:

```text
schema_version
horizontal_source_crs, horizontal_source_epsg, horizontal_axis_order
horizontal_angular_unit, horizontal_mesh_crs
projection_reference_latitude_deg, projection_reference_longitude_deg
horizontal_linear_unit
vertical_reference, vertical_epsg, vertical_unit
elevation_positive_direction, vertical_exaggeration, vertical_caveat
```

Первая запись имеет `record_type=metadata`: канонические поля данных в ней
пусты, а метаданные заполнены. Следующие записи имеют `record_type=data`:
метаданные в них пусты, чтобы не раздувать многомиллионную таблицу повторением
одних строк. Таким образом, при отделении CSV от sidecar-файла проекция,
вертикальная система и единицы не исчезают.

## Профили

`profiles.csv` не выбирает трассу автоматически. `Profile` содержит ID,
название и упорядоченные `NodeIDs`; все ID проверяются до создания файлов.
Таблица хранит `point_index` от 1, накопленное `distance_m` по X/Y LAEA,
координаты, отметку, глубину, способ выборки и качество каждого узла.

Автоматическое построение содержательных поперечных профилей «берег →
глубоководная часть» относится к VIEW-03. EXPORT-02 намеренно не выдаёт
случайную последовательность узлов за геоморфологический разрез.

## NoData и проверки

Как и MSH, VTU и обогащённые таблицы создаются только для принятой модели со
100% покрытием. Nullable-поля `source_distance_m` и `aspect_deg` используют
объявленный отрицательный sentinel; обязательные глубины и отметки не могут
быть NoData.

`ReadVTU` проверяет XML, число tuples/components, типы массивов, единицы,
`VTK_QUAD`, connectivity, ID, таблицы кодов, `Z = elevation_m` и формулу
положительной глубины. Автоматический тест выполняет полный цикл
`Model → VTU → Model` и отдельно сверяет CSV с MSH и VTU по идентификаторам.

ASCII VTU выбран для аудита и совместимости. Для сеток с миллионами узлов в
будущем допустим бинарный appended-режим с тем же логическим контрактом.
