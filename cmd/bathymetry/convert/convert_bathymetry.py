#!/usr/bin/env python3
"""Создание воспроизводимого производного набора батиметрии из NetCDF."""

import argparse
import hashlib
import json
import platform
import sys
from datetime import datetime, timezone
from pathlib import Path

import numpy as np

try:
    import xarray as xr
except ImportError:
    print("Установите зависимости: pip install -r cmd/bathymetry/convert/requirements.txt")
    sys.exit(1)


def file_sha256(path):
    """Возвращает SHA-256 файла без загрузки всего содержимого в память."""
    digest = hashlib.sha256()
    with open(path, "rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def find_name(dataset, candidates, entity):
    """Находит имя координаты или переменной по известным вариантам."""
    for name in candidates:
        if name in dataset:
            return name
    raise ValueError(f"Не найдено поле {entity}; доступны: {list(dataset.variables)}")


def parse_downloaded_at(value):
    """Проверяет, что дата загрузки задана в ISO 8601 с часовым поясом."""
    parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if parsed.tzinfo is None:
        raise argparse.ArgumentTypeError("дата загрузки должна содержать часовой пояс")
    return value


def convert_netcdf_to_json(args):
    """Ресэмплирует GEBCO ближайшим соседом и создаёт JSON с паспортом."""
    input_path = Path(args.input)
    output_path = Path(args.output)
    passport_path = (
        Path(args.passport)
        if args.passport
        else output_path.with_name(f"{output_path.stem}.metadata.json")
    )
    if output_path.exists() or passport_path.exists():
        raise FileExistsError(
            "Производный набор или паспорт уже существует; выберите новое имя"
        )
    output_path.parent.mkdir(parents=True, exist_ok=True)
    passport_path.parent.mkdir(parents=True, exist_ok=True)

    print(f"Чтение исходного NetCDF: {input_path}")
    with xr.open_dataset(input_path) as dataset:
        latitude_name = find_name(dataset, ["lat", "latitude"], "широты")
        longitude_name = find_name(dataset, ["lon", "longitude"], "долготы")
        elevation_name = find_name(
            dataset, ["elevation", "depth", "bathymetry", "z"], "высоты"
        )

        elevation = dataset[elevation_name].squeeze(drop=True)
        if latitude_name not in elevation.dims or longitude_name not in elevation.dims:
            raise ValueError(
                f"Переменная {elevation_name} не образует сетку "
                f"{latitude_name} × {longitude_name}: {elevation.dims}"
            )
        if elevation.ndim != 2:
            raise ValueError(
                f"После удаления единичных измерений ожидалась двумерная сетка, "
                f"получено {elevation.ndim}"
            )

        elevation = elevation.sortby(latitude_name).sortby(longitude_name)
        min_lat, max_lat, min_lon, max_lon = args.bounds
        target_latitudes = np.round(
            np.arange(min_lat, max_lat + args.resolution / 2, args.resolution), 10
        )
        target_longitudes = np.round(
            np.arange(min_lon, max_lon + args.resolution / 2, args.resolution), 10
        )
        sampled = elevation.interp(
            {
                latitude_name: target_latitudes,
                longitude_name: target_longitudes,
            },
            method="nearest",
        ).transpose(latitude_name, longitude_name)
        sampled_values = sampled.values

    points = []
    for latitude_index, latitude in enumerate(target_latitudes):
        for longitude_index, longitude in enumerate(target_longitudes):
            elevation_meters = float(sampled_values[latitude_index, longitude_index])
            if not np.isfinite(elevation_meters) or elevation_meters >= 0:
                continue
            points.append(
                {
                    "lat": round(float(latitude), 6),
                    "lon": round(float(longitude), 6),
                    "depth": round(elevation_meters, 2),
                }
            )

    if not points:
        raise ValueError("После ресэмплинга и удаления суши не осталось подводных точек")

    print(f"Запись производного набора: {output_path} ({len(points)} точек)")
    with open(output_path, "w", encoding="utf-8") as output:
        json.dump(points, output, ensure_ascii=False, indent=2)
        output.write("\n")

    source_checksum = file_sha256(input_path)
    dataset_checksum = file_sha256(output_path)
    created_at = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    passport = {
        "schema_version": "1.0",
        "title": (
            f"Ресэмплированный производный региональный набор {args.source_product} "
            f"для Чёрного моря ({args.resolution:g}°)"
        ),
        "status": "verified_derived",
        "dataset_file": output_path.name,
        "dataset_sha256": dataset_checksum,
        "created_at": created_at,
        "point_count": len(points),
        "bounds": {
            "min_lat": args.bounds[0],
            "max_lat": args.bounds[1],
            "min_lon": args.bounds[2],
            "max_lon": args.bounds[3],
        },
        "target_resolution_degrees": args.resolution,
        "target_resolution_arc_seconds": args.resolution * 3600,
        "source_product": args.source_product,
        "source_product_doi": args.source_product_doi,
        "source_url": args.source_url,
        "source_downloaded_at": args.downloaded_at,
        "source_netcdf": input_path.as_posix(),
        "source_netcdf_sha256": source_checksum,
        "source_grid_interval_arc_seconds": args.source_grid_interval_arc_seconds,
        "horizontal_reference": args.horizontal_reference,
        "vertical_reference": args.vertical_reference,
        "vertical_reference_caveat": args.vertical_reference_caveat,
        "resampling_method": "ближайший сосед по центрам ячеек исходной сетки",
        "land_filter": (
            "Сохранены только конечные значения elevation < 0; знак GEBCO не инвертируется."
        ),
        "processing_script": "cmd/bathymetry/convert/convert_bathymetry.py",
        "processing_software": {
            "python": platform.python_version(),
            "numpy": np.__version__,
            "xarray": xr.__version__,
        },
        "license": args.license,
        "license_url": args.license_url,
        "attribution": args.attribution,
        "limitations": [
            "Целевой шаг сетки не является разрешением исходных измерений.",
            "Ближайший сосед не добавляет пространственной детализации.",
            "Набор не предназначен для навигации или задач безопасности на море.",
        ],
    }
    with open(passport_path, "w", encoding="utf-8") as passport_file:
        json.dump(passport, passport_file, ensure_ascii=False, indent=2)
        passport_file.write("\n")

    print(f"Паспорт сохранён: {passport_path}")
    print(f"SHA-256 исходного NetCDF: {source_checksum}")
    print(f"SHA-256 производного JSON: {dataset_checksum}")


def build_parser():
    """Создаёт парсер обязательных параметров воспроизводимой конвертации."""
    parser = argparse.ArgumentParser(
        description="Ресэмплинг NetCDF GEBCO в JSON Lito с паспортом происхождения"
    )
    parser.add_argument("--input", required=True, help="Путь к сохранённому NetCDF")
    parser.add_argument("--output", required=True, help="Путь к производному JSON")
    parser.add_argument("--passport", help="Путь к паспорту; по умолчанию рядом с JSON")
    parser.add_argument(
        "--resolution",
        type=float,
        default=0.01,
        help="Целевой шаг сетки в градусах (по умолчанию 0.01)",
    )
    parser.add_argument(
        "--bounds",
        nargs=4,
        type=float,
        required=True,
        metavar=("MIN_LAT", "MAX_LAT", "MIN_LON", "MAX_LON"),
        help="Границы производного регионального набора",
    )
    parser.add_argument("--source-product", required=True, help="Продукт и версия")
    parser.add_argument("--source-product-doi", required=True, help="DOI продукта")
    parser.add_argument("--source-url", required=True, help="Точный URL загрузки")
    parser.add_argument(
        "--downloaded-at",
        required=True,
        type=parse_downloaded_at,
        help="Дата загрузки в ISO 8601 с часовым поясом",
    )
    parser.add_argument(
        "--source-grid-interval-arc-seconds",
        required=True,
        type=float,
        help="Шаг исходной сетки в угловых секундах",
    )
    parser.add_argument("--horizontal-reference", required=True)
    parser.add_argument("--vertical-reference", required=True)
    parser.add_argument("--vertical-reference-caveat", default="")
    parser.add_argument("--license", required=True)
    parser.add_argument("--license-url", required=True)
    parser.add_argument("--attribution", required=True)
    return parser


if __name__ == "__main__":
    arguments = build_parser().parse_args()
    if arguments.resolution <= 0:
        raise ValueError("Целевой шаг сетки должен быть положительным")
    min_latitude, max_latitude, min_longitude, max_longitude = arguments.bounds
    if min_latitude >= max_latitude or min_longitude >= max_longitude:
        raise ValueError("Границы должны быть заданы в возрастающем порядке")
    convert_netcdf_to_json(arguments)
