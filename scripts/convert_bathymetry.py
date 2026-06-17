#!/usr/bin/env python3
"""
Конвертация батиметрических данных в формат Litora-CLI.

Пример использования:
    python scripts/convert_bathymetry.py \
        --input gebco_2024_n46.0_n43.0_w28.0_e38.0.nc \
        --output data/black-sea-bathymetry.json \
        --resolution 0.01 \
        --bounds 43 47 28 38
"""

import json
import sys

import numpy as np

try:
    import xarray as xr
except ImportError:
    print("Установите xarray: pip install xarray netCDF4")
    sys.exit(1)


def convert_netcdf_to_json(input_file, output_file, bounds, resolution):
    """
    Конвертирует NetCDF файл с батиметрией в JSON формат Litora-CLI.

    Args:
        input_file: путь к NetCDF файлу
        output_file: путь к выходному JSON файлу
        bounds: (min_lat, max_lat, min_lon, max_lon)
        resolution: размер ячейки в градусах
    """
    print(f"Загрузка {input_file}...")
    ds = xr.open_dataset(input_file)

    # Определяем имена переменных (зависит от источника)
    depth_var = None
    for var_name in ["elevation", "depth", "bathymetry", "z"]:
        if var_name in ds:
            depth_var = var_name
            break

    if depth_var is None:
        print("Доступные переменные:", list(ds.data_vars))
        raise ValueError("Не найдена переменная с глубиной")

    print(f"Используется переменная: {depth_var}")

    # Извлекаем данные для заданного региона
    min_lat, max_lat, min_lon, max_lon = bounds

    print(f"Извлечение данных для региона: {bounds}")
    subset = ds.sel(lat=slice(min_lat, max_lat), lon=slice(min_lon, max_lon))

    depths = subset[depth_var].values
    lats = subset["lat"].values
    lons = subset["lon"].values

    # Создаём регулярную сетку с заданным разрешением
    print(f"Создание сетки с разрешением {resolution}°...")

    points = []
    lat_steps = int((max_lat - min_lat) / resolution) + 1
    lon_steps = int((max_lon - min_lon) / resolution) + 1

    for i in range(lat_steps):
        lat = min_lat + i * resolution
        for j in range(lon_steps):
            lon = min_lon + j * resolution

            # Находим ближайшую точку в исходных данных
            lat_idx = np.abs(lats - lat).argmin()
            lon_idx = np.abs(lons - lon).argmin()

            try:
                depth = depths[lat_idx, lon_idx]

                # GEBCO использует положительные значения для глубины ниже уровня моря
                # Наш формат использует отрицательные значения
                if depth > 0:
                    depth = -depth

                points.append(
                    {
                        "lat": round(lat, 6),
                        "lon": round(lon, 6),
                        "depth": round(depth, 2),
                    }
                )
            except IndexError:
                continue

    print(f"Создано {len(points)} точек")

    # Сохраняем в JSON
    print(f"Сохранение в {output_file}...")
    with open(output_file, "w") as f:
        json.dump(points, f, indent=2)

    print("Готово!")


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(
        description="Конвертация батиметрии в формат Litora-CLI"
    )
    parser.add_argument("--input", required=True, help="Путь к NetCDF файлу")
    parser.add_argument("--output", required=True, help="Путь к выходному JSON файлу")
    parser.add_argument(
        "--resolution",
        type=float,
        default=0.01,
        help="Разрешение сетки в градусах (по умолчанию 0.01)",
    )
    parser.add_argument(
        "--bounds",
        nargs=4,
        type=float,
        required=True,
        metavar=("MIN_LAT", "MAX_LAT", "MIN_LON", "MAX_LON"),
        help="Границы региона (для Чёрного моря: 40.5 46.5 27.5 42.5)",
    )

    args = parser.parse_args()

    convert_netcdf_to_json(args.input, args.output, args.bounds, args.resolution)
