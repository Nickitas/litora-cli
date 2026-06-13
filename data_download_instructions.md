# Инструкции по скачиванию данных для FRAES

## 🎯 Цель: Скачать данные для литологического профиля Чёрного моря

---

## 📥 1. EMODnet Geology (Primary Source)

### Методы доступа:

#### **Вариант A: Через Map Viewer (рекомендуется)**
1. Перейти: https://emodnet.ec.europa.eu/geoviewer/
2. Выбрать слой:
   - **Seabed Substrate** (дляsurficial sediments)
   - **Sea-floor Geology** (для bedrock)
   - **Coastal Behaviour** (для erosion/accretion)
3. Навигация к Black Sea (zoom in)
4. Export → Download as:
   - GeoJSON
   - ESRI Shapefile
   - WFS request

#### **Вариант B: Через WFS (программно)**
```bash
# WFS endpoint для seabed substrate
https://www.emodnet-geology.nl/data-services/wfs?

# Пример запроса для Black Sea:
https://www.emodnet-geology.nl/data-services/wfs?
service=WFS&
version=2.0.0&
request=GetFeature&
typename=geology:seabed_substrate&
bbox=27,40,42,47,EPSG:4326&
outputFormat=application/json
```

#### **Вариант C: Через R package**
```r
# Установить пакет
devtools::install_github("ropensci/emodnet.wfs")

# Скачать данные
library(emodnet.wfs)

# Seabed substrate
substrate <- emodnet_wfs(
  dataset = "seabed_substrate",
  bbox = c(27, 40, 42, 47),  # Black Sea bounds
  crs = EPSG:4326
)

# Coastal behaviour
behaviour <- emodnet_wfs(
  dataset = "coastal_behaviour",
  bbox = c(27, 40, 42, 47)
)
```

### Конвертация в FRAES формат:

```bash
# Если GeoJSON — конвертировать в наш формат
python scripts/convert_emodnet_to_lithology.py \
  --input emodnet_substrate_blacksea.geojson \
  --output data/black-sea-lithology.json
```

---

## 📥 2. DOORS Black Sea Data

### Доступ через Observation Tool:
1. Перейти: https://www.doorsblacksea.eu/observationtool
2. Выбрать слой:
   - Coastal Vulnerability
   - Geomorphology
   - Sediment Distribution
3. Download данных (если доступно)

### Published Articles:
- https://www.doorsblacksea.eu/publishedarticles
- Искать статьи с "lithology", "sediment", "coastal erosion"

---

## 📥 3. USGS Shorelines (Румыния)

### Прямая ссылка:
https://www.usgs.gov/data/satellite-derived-shorelines-romania-black-sea-coast-period-1984-2023-using-landsat

### Формат:
- Shapefile (.shp)
- CSV с метаданными
- Временной ряд: 1984-2023

### Использование:
```bash
# Скачать для валидации erosion rates
wget https://www.usgs.gov/.../romania_shorelines.zip
unzip romania_shorelines.zip

# Конвертировать в GeoJSON если нужно
ogr2ogr -f GeoJSON romania_shorelines.geojson romania_shorelines.shp
```

---

## 📥 4. GEBCO_2026 (Обновление батиметрии)

### Скачивание:
1. Перейти: https://www.gebco.net/data-products/gridded-bathymetry-data
2. Скачать GEBCO_2026 Grid:
   - Global file (если нужна вся область)
   - Tiles (для Black Sea region)
   - User-defined area

### Для Чёрного моря:
```bash
# User-defined area: 27°E-42°E, 40°N-47°N
# Скачать через GEBCO Download Tool

# Или использовать Python
pip install gefetch4

# Скачать область
gefetch --bbox 27 40 42 47 --output gebco_2026_blacksea.nc
```

### Конвертация в наш формат:
```python
# scripts/netcdf_to_lithology.py
import xarray as xr
import json

ds = xr.open_dataset('gebco_2026_blacksea.nc')
depth_data = ds['elevation'].values

# Конвертировать в наш формат
# (уже есть в cmd/download-bathymetry/main.go)
```

---

## 📥 5. Научные публикации (Literature Data)

### Ключевые статьи:

**Крым:**
1. HAL Science: https://hal.science/hal-01794787/file/HippolytePourHall_2018.pdf
2. GeoEcoMar: https://www.geoecomar.ro/website/publicatii/Nr.11-2005/3.pdf

**Румыния:**
3. MDPI Sustainability: https://www.mdpi.com/2071-1050/15/9/7651

**Общий анализ:**
4. ResearchGate: https://www.researchgate.net/publication/349225435

**Дунай:**
5. Frontiers: https://www.frontiersin.org/journals/marine-science/articles/10.3389/fmars.2023.1068065/full

### Метод извлечения данных:
1. Скачать PDF
2. Извлечь таблицы с lithology/erosion rates
3. Ручной ввод в JSON profile
4. Cross-reference с EMODnet данными

---

## 🗂️ Структура финального профиля

### Файл: `data/black-sea-lithology.json`

```json
{
  "metadata": {
    "name": "Black Sea Lithology Profile",
    "version": "1.0",
    "created": "2025-01-XX",
    "sources": [
      "EMODnet Geology",
      "DOORS Black Sea 2024",
      "Scientific publications (see sources)"
    ],
    "resolution": 0.5,
    "bounds": [27.0, 40.0, 42.0, 47.0]
  },
  
  "points": [
    {
      "lat": 45.2,
      "lon": 32.8,
      "region": "crimea",
      "lithology_class": "limestone",
      "resistance": 4.5,
      "color": "#6b6b6b",
      "description": "Sarmatian limestone, well-cemented",
      "confidence": "high",
      "source": "emodnet + literature"
    },
    // ... больше точек
  ],
  
  "classes": {
    // Классификация пород (как в data_search_results.md)
  },
  
  "erosion_baselines": {
    // Базовые скорости эрозии (как в data_search_results.md)
  },
  
  "coastal_behaviour": {
    // Из EMODnet Coastal Behaviour (если доступно)
    "erosion_zones": [...],
    "accretion_zones": [...],
    "stable_zones": [...]
  }
}
```

---

## ⚙️ Скрипты для конвертации

### scripts/convert_emodnet_to_lithology.py

```python
#!/usr/bin/env python3
"""
Конвертация EMODnet WFS данных в FRAES литологический профиль
"""

import json
import geojson
from typing import Dict, List

# Mapping EMODnet substrate codes к resistance values
SUBSTRATE_TO_RESISTANCE = {
    "bedrock": {
        "igneous": 8.0,
        "metamorphic": 6.0,
        "sedimentary": 4.0
    },
    "sediment": {
        "coarse": 3.0,
        "mixed": 2.0,
        "fine": 1.0
    }
}

def convert_emodnet_to_lithology(
    emodnet_file: str,
    output_file: str,
    resolution: float = 0.5
) -> None:
    """Конвертирует EMODnet GeoJSON в FRAES профиль"""
    
    with open(emodnet_file) as f:
        data = geojson.load(f)
    
    profile = {
        "metadata": {
            "name": "Black Sea Lithology Profile from EMODnet",
            "version": "1.0",
            "sources": ["EMODnet Geology WFS"]
        },
        "points": [],
        "classes": {}
    }
    
    # Обработка точек
    for feature in data['features']:
        coords = feature['geometry']['coordinates']
        properties = feature['properties']
        
        # Извлечение свойств
        lithology = properties.get('lithology', 'unknown')
        substrate = properties.get('substrate', 'unknown')
        
        # Mapping к resistance
        resistance = map_to_resistance(lithology, substrate)
        
        point = {
            "lat": coords[1],
            "lon": coords[0],
            "lithology_class": lithology,
            "resistance": resistance,
            "source": "emodnet"
        }
        
        profile['points'].append(point)
    
    # Сохранение
    with open(output_file, 'w') as f:
        json.dump(profile, f, indent=2)
    
    print(f"✓ Конвертировано {len(profile['points'])} точек")

def map_to_resistance(lithology: str, substrate: str) -> float:
    """Maps EMODnet коды к FRAES resistance"""
    # Детальная mapping логика
    # ...
    return 2.5  # default

if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser()
    parser.add_argument('--input', required=True)
    parser.add_argument('--output', default='data/black-sea-lithology.json')
    args = parser.parse_args()
    
    convert_emodnet_to_lithology(args.input, args.output)
```

---

## 📋 План действий (неделя 1)

### День 1-2: Поиск и скачивание
- [ ] EMODnet Geology (через Viewer или WFS)
- [ ] EMODnet Coastal Behaviour (если доступно)
- [ ] USGS Shorelines Румынии
- [ ] GEBCO_2026 update (опционально)

### День 3: Извлечение данных
- [ ] Конвертация EMODnet → GeoJSON
- [ ] Извлечение lithology из научных статей
- [ ] Cross-reference разных источников

### День 4: Создание профиля
- [ ] Генерация точек профиля
- [ ] Добавление confidence scores
- [ ] Валидация по известным erosion rates

### День 5: Тестирование
- [ ] Загрузка профиля в тестовый код
- [ ] Проверка интерполяции
- [ ] Сравнение с observed erosion

---

## ✅ Критерии успеха

1. **Покрытие:** Профиль покрывает весь Черноморский регион
2. **Точность:** Resistance values соответствуют literature
3. **Интерполяция:** Корректная интерполяция между точками
4. **Валидация:** Calculated erosion ≈ observed erosion

---

## 🚀 Начало работы

**Сейчас:**
1. Открыть EMODnet Geology Viewer
2. Экспериментировать с слоями
3. Скачать test данные

**Завтра:**
1. Написать конвертационный скрипт
2. Скачать полные dataset'ы
3. Начать создание профиля

**Готовы ли начать скачивание?**
