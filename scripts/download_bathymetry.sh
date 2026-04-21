#!/bin/bash
set -e

# Скрипт для автоматического скачивания и конвертации батиметрии GEBCO
# для региона Чёрного моря

echo "=== Загрузка батиметрии GEBCO для Чёрного моря ==="

# Конфигурация
GEOJSON_URL="https://www.bodc.ac.uk/data/open_download/gebco/gebco_2024/netcdf/gebco_2024_n40.5_s46.5_w27.5_e42.5_v2.nc"
OUTPUT_DIR="data"
TEMP_NC="${OUTPUT_DIR}/temp_bathymetry.nc"
FINAL_JSON="${OUTPUT_DIR}/black-sea-bathymetry.json"

# Границы Чёрного моря (минимальный прямоугольник)
MIN_LAT=40.5
MAX_LAT=46.5
MIN_LON=27.5
MAX_LON=42.5

# Разрешение сетки (0.01 градуса ≈ 1.1 км)
RESOLUTION=0.01

mkdir -p "${OUTPUT_DIR}"

# Проверяем, существует ли уже файл
if [ -f "${FINAL_JSON}" ]; then
    echo "✓ Файл уже существует: ${FINAL_JSON}"

    # Спрашиваем, нужно ли обновить
    read -p "Обновить данные? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Используем существующий файл."
        exit 0
    fi
    echo "Обновляем данные..."
fi

# Шаг 1: Скачивание NetCDF
echo ""
echo "[1/3] Скачивание данных GEBCO..."
echo "URL: ${GEOJSON_URL}"

if command -v curl &> /dev/null; then
    curl -L -o "${TEMP_NC}" "${GEOJSON_URL}"
elif command -v wget &> /dev/null; then
    wget -O "${TEMP_NC}" "${GEOJSON_URL}"
else
    echo "❌ Ошибка: нужны curl или wget"
    exit 1
fi

if [ ! -f "${TEMP_NC}" ]; then
    echo "❌ Ошибка: файл не скачался"
    exit 1
fi

echo "✓ Скачано: $(wc -c < "${TEMP_NC}") байт"

# Шаг 2: Проверка зависимостей Python
echo ""
echo "[2/3] Проверка зависимостей..."

if ! command -v python3 &> /dev/null; then
    echo "❌ Ошибка: нужен Python 3"
    exit 1
fi

# Установка зависимостей если нужно
if ! python3 -c "import xarray" 2>/dev/null; then
    echo "Установка xarray и netCDF4..."
    pip3 install xarray netCDF4
fi

# Шаг 3: Конвертация
echo ""
echo "[3/3] Конвертация в JSON формат..."

python3 << EOF
import json
import numpy as np
import sys

try:
    import xarray as xr
except ImportError:
    print("❌ Ошибка: не удалось импортировать xarray")
    print("Установите: pip3 install xarray netCDF4")
    sys.exit(1)

input_file = "${TEMP_NC}"
output_file = "${FINAL_JSON}"
min_lat, max_lat = ${MIN_LAT}, ${MAX_LAT}
min_lon, max_lon = ${MIN_LON}, ${MAX_LON}
resolution = ${RESOLUTION}

print(f"Загрузка {input_file}...")
try:
    ds = xr.open_dataset(input_file)
except Exception as e:
    print(f"❌ Ошибка открытия NetCDF: {e}")
    sys.exit(1)

# Определяем переменную с глубиной
depth_var = None
for var_name in ['elevation', 'depth', 'bathymetry', 'z']:
    if var_name in ds:
        depth_var = var_name
        break

if depth_var is None:
    print(f"❌ Ошибка: переменная глубины не найдена")
    print(f"Доступные переменные: {list(ds.data_vars)}")
    sys.exit(1)

print(f"Используется переменная: {depth_var}")

# Извлекаем данные
subset = ds.sel(
    lat=slice(min_lat, max_lat),
    lon=slice(min_lon, max_lon)
)

depths = subset[depth_var].values
lats = subset['lat'].values
lons = subset['lon'].values

print(f"Исходные данные: {depths.shape} точек")

# Создаём регулярную сетку
points = []
lat_steps = int((max_lat - min_lat) / resolution) + 1
lon_steps = int((max_lon - min_lon) / resolution) + 1

print(f"Создание сетки: {lat_steps}x{lon_steps} = {lat_steps*lon_steps} точек")

for i in range(lat_steps):
    lat = min_lat + i * resolution
    for j in range(lon_steps):
        lon = min_lon + j * resolution

        # Находим ближайшую точку
        lat_idx = np.abs(lats - lat).argmin()
        lon_idx = np.abs(lons - lon).argmin()

        try:
            depth = depths[lat_idx, lon_idx]

            # GEBCO: положительные значения = глубина ниже уровня моря
            # Наш формат: отрицательные значения = глубина
            if depth > 0:
                depth = -depth

            # Пропускаем точки на суше (depth >= 0 после конвертации)
            if depth >= 0:
                continue

            points.append({
                "lat": round(lat, 6),
                "lon": round(lon, 6),
                "depth": round(depth, 2)
            })
        except (IndexError, ValueError):
            continue

print(f"Создано {len(points)} подводных точек")

# Сохраняем в JSON
with open(output_file, 'w') as f:
    json.dump(points, f)

print(f"✓ Сохранено: {output_file}")

# Статистика
depths_only = [p['depth'] for p in points]
print(f"\nСтатистика:")
print(f"  Мин. глубина: {min(depths_only):.1f} м")
print(f"  Макс. глубина: {max(depths_only):.1f} м")
print(f"  Средняя глубина: {sum(depths_only)/len(depths_only):.1f} м")
print(f"  Размер файла: {len(json.dumps(points)) // 1024} KB")

EOF

# Очистка временного файла
rm "${TEMP_NC}"

echo ""
echo "=== Готово! ==="
echo "Батиметрия сохранена: ${FINAL_JSON}"
echo ""
echo "Использование:"
echo "  ./fraes model erosion --bathymetry ${FINAL_JSON} --output ./output/erosion"
