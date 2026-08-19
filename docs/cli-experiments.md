# Эксперименты через CLI

## Входные данные модели

Команды `erosion` и `all` загружают переданные через `--bathymetry` и
`--lithology` файлы до запуска модели. Ошибка чтения или валидации теперь
останавливает эксперимент, чтобы результат не был принят за расчёт с другими
входными данными.

Если указан `--lithology`, профиль включается автоматически. Флаг
`--enable-lithology` включает профиль по умолчанию
`data/black-sea-lithology.json`, если путь не задан. Этот профиль имеет статус
`alpha_inferred`, не является подтверждённой эмпирической картой и вызывает
явное предупреждение. Батиметрия без явного пути больше не подставляется.
Рядом с переданным JSON ожидается паспорт `.metadata.json`; его SHA-256 и
число точек проверяются до расчёта.

## Временная динамика в `all`

При `--target-years N` и положительном `--years-per-step` полный конвейер
использует временную модель. В неё передаются вероятность и сила штормов,
подъём уровня моря, сезонность и фаза сезонности. Без `--target-years` команда
использует обычные дискретные шаги `--steps`.

Если передан любой временной параметр (например, `--enable-seasonality` или
`--storm-probability`), но не задан положительный `--target-years`, команда
завершается ошибкой: временные настройки не теряются незаметно.

## CSV и GIF

Флаги `--output-csv`, `--csv-format`, `--output-gif`, `--gif-fps` и
`--gif-skip` подключены к обеим командам `erosion` и `all`:

```bash
./lito erosion \
  --bathymetry output/source/black-sea-bathymetry-gebco2026-0.01deg-derived.json \
  --lithology data/black-sea-lithology.json \
  --output-csv erosion.csv \
  --output-gif erosion.gif

./lito all \
  --target-years 20 \
  --storm-probability 0.2 \
  --enable-seasonality \
  --output-csv all.csv \
  --output-gif all.gif
```

Относительные имена сохраняются в `output/csv/` и `output/gif/` выбранного
каталога `--output`. Абсолютные пути используются без изменения.
