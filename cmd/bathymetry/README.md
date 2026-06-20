# Батиметрические данные для Litora-CLI

Инструменты для работы с батиметрическими данными через CLI и Python скрипты.

## Быстрый старт

### Вариант 1: Через CLI команду (рекомендуется)

```bash
# Загрузка данных (интерактивный мастер)
go run cmd/bathymetry/main.go download

# Конвертация NetCDF в JSON
go run cmd/bathymetry/main.go convert \
  --input gebco_2024_n40.5_s46.5_w27.5_e42.5.nc \
  --output data/black-sea-bathymetry.json \
  --resolution 0.01 \
  --bounds 40.5 46.5 27.5 42.5
```

### Вариант 2: Через bash скрипт

```bash
cd cmd/bathymetry/convert
./download_bathymetry.sh
```

### Вариант 3: Ручная конвертация

```bash
# Установка зависимостей
pip install -r cmd/bathymetry/convert/requirements.txt

# Или через virtualenv
python3 -m venv scripts/venv
source scripts/venv/bin/activate
pip install -r cmd/bathymetry/convert/requirements.txt

# Конвертация
python cmd/bathymetry/convert/convert_bathymetry.py \
  --input gebco_2024_n40.5_s46.5_w27.5_e42.5.nc \
  --output data/black-sea-bathymetry.json \
  --resolution 0.01 \
  --bounds 40.5 46.5 27.5 42.5
```

## Структура директорий

```
bathymetry/
├── cmd/
│   └── bathymetry/
│       ├── main.go                 # CLI команда
│       ├── README.md              # Эта документация
│       └── convert/               # Скрипты конвертации
│           ├── convert_bathymetry.py
│           ├── download_bathymetry.sh
│           └── requirements.txt
└── data/
    └── black-sea-bathymetry.json  # Конвертированные данные
```

## Источники данных для Чёрного моря

### 1. GEBCO (рекомендуется)

**Плюсы:**
- Глобальное покрытие
- Бесплатный
- Высокое качество
- Регулярно обновляется

**Как скачать:**

```bash
# Вариант 1: CLI команда (автоматически откроет браузер)
go run cmd/bathymetry/main.go download

# Вариант 2: Вручную
# Зайдите на: https://www.gebco.net/data_and_products/gridded_bathymetry_data/
# Выберите регион: 40°N-47°N, 27°E-42°E

# Вариант 3: Скачать глобальный dataset
wget https://www.bodc.ac.uk/data/open_download/gebco/gebco_2024/zip/
unzip gebco_2024.zip
```

**Конвертация:**
```bash
# Через CLI
go run cmd/bathymetry/main.go convert \
  --input gebco_2024_n40.5_s46.5_w27.5_e42.5.nc \
  --output data/black-sea-bathymetry.json \
  --resolution 0.01 \
  --bounds 40.5 46.5 27.5 42.5

# Или через Python
pip install -r cmd/bathymetry/convert/requirements.txt
python cmd/bathymetry/convert/convert_bathymetry.py \
  --input gebco_2024_n40.5_s46.5_w27.5_e42.5.nc \
  --output data/black-sea-bathymetry.json \
  --resolution 0.01 \
  --bounds 40.5 46.5 27.5 42.5
```

### 2. EMODnet (наилучшее покрытие для Чёрного моря)

**Плюсы:**
- Специализирован на европейских морях
- Очень высокое разрешение
- Актуальные данные

**Как скачать:**

1. Зайдите на: https://www.emodnet-bathymetry.eu/data-products/
2. Выберите "Black Sea"
3. Скачайте GeoTIFF или NetCDF
4. Конвертируйте с помощью CLI или Python скрипта

### 3. ETOPO1 (альтернатива)

**Плюсы:**
- Простой формат
- Глобальное покрытие

**Как скачать:**
```bash
wget https://www.ngdc.noaa.gov/mgg/global/relief/ETOPO1/data/ice_surface/grid_registered/netcdf/ETOPO1_Bed_g_gmt4.nc
```

## Рекомендации по выбору разрешения

| Разрешение (градусы) | Метров (на широте 43°) | Использование |
|---------------------|------------------------|---------------|
| 0.001° | ~110 м | Детальные локальные модели |
| 0.01° | ~1.1 км | Региональные модели (рекомендуется) |
| 0.1° | ~11 км | Грубые оценки |

**Рекомендация:** Используйте 0.01° для Чёрного моря — это баланс между точностью и производительностью.

## Пример использования реальных данных

```bash
# После конвертации:
./lito model erosion \
  --steps 10 \
  --erosion-strength 50 \
  --wave-direction 0 \
  --wind-speed 12 \
  --bathymetry data/black-sea-bathymetry.json \
  --output ./output/black-sea-erosion
```

## Валидация данных

После конвертации проверьте JSON файл:

```bash
# Проверка валидности JSON
python -m json.tool data/black-sea-bathymetry.json > /dev/null

# Статистика по данным
python3 << 'EOF'
import json
with open('data/black-sea-bathymetry.json') as f:
    data = json.load(f)

depths = [p['depth'] for p in data]
print(f"Точек: {len(data)}")
print(f"Мин. глубина: {min(depths):.1f} м")
print(f"Макс. глубина: {max(depths):.1f} м")
print(f"Средняя глубина: {sum(depths)/len(depths):.1f} м")
EOF
```

## Troubleshooting

### Ошибка "outside grid bounds"
**Причина:** Береговая линия выходит за пределы батиметрической сетки
**Решение:** Увеличьте bounds при конвертации

### Ошибка "missing neighbor points"
**Причина:** Слишком высокое разрешение сетки
**Решение:** Увеличьте resolution до 0.01 или 0.02

### Медленная работа
**Причина:** Слишком много точек в сетке
**Решение:** Увеличьте resolution или уменьшите область покрытия

## Дополнительные ресурсы

- [Основная документация проекта](../../README.md)
- [Python скрипты анализа](../../scripts/README.md)
- Документация по формату: см. internal/domain/geometry/bathymetry.go

## Ссылки на источники данных

- GEBCO: https://www.gebco.net/
- EMODnet: https://www.emodnet-bathymetry.eu/
- ETOPO1: https://www.ngdc.noaa.gov/mgg/global/
