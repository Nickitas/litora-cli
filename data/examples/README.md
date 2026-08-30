# Готовый стартовый пример CERC для Сочи

Файлы этого каталога позволяют выполнить технический запуск для участка
побережья Сочи без регистрации в сторонних сервисах. Они не являются данными
для инженерного заключения.

- `sochi-local-segment.geojson` — локальный незамкнутый сегмент береговой
  линии OpenStreetMap (way 59530506, выгрузка 17 августа 2026 года, ODbL);
- `sochi-emodnet-bathymetry.json` — семь фактических глубин, запрошенных
  17 августа 2026 года из EMODnet Bathymetry REST по данным GEBCO 2024;
- `sochi-open-meteo-marine-2026-08-17.json` — семидневный почасовой прогноз
  высоты, периода и направления волн, выгруженный 17 августа 2026 года из
  Open-Meteo Marine для ячейки 43.625° с. ш., 39.70834° в. д.

Для запуска выполните:

```bash
./lito erosion \
  --input data/examples/sochi-local-segment.geojson \
  --bathymetry data/examples/sochi-emodnet-bathymetry.json \
  --wave-input data/examples/sochi-open-meteo-marine-2026-08-17.json \
  --wave-source "Open-Meteo Marine, точка 43.625N 39.70834E, выгрузка 2026-08-17" \
  --max-bathymetry-gap 3000 \
  --steps 24 \
  --output output/sochi-demo
```

Выходной баланс расположен в `output/sochi-demo/erosion/longshore-cerc.json`.
Радиус 3000 м допустим только для этой грубой демонстрационной сетки; для
исследования подготовьте регулярную прибрежную батиметрию и оставьте порог не
более обоснованной дистанции её разрешения.
