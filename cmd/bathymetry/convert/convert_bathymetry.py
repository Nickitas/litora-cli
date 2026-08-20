#!/usr/bin/env python3
"""Создание воспроизводимого производного набора батиметрии из NetCDF."""

import argparse
import hashlib
import json
import platform
import sys
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlparse

import numpy as np


BLACK_SEA_BOUNDS = (40.5, 47.5, 27.0, 42.5)
SOURCE_PRODUCT = "GEBCO_2026 Grid"
SOURCE_PRODUCT_DOI = "10.5285/4f68d5c7-45eb-f999-e063-7086abc036fa"
SOURCE_GRID_INTERVAL_ARC_SECONDS = 15
HORIZONTAL_REFERENCE = "WGS 84 по допущению GEBCO"
VERTICAL_REFERENCE = "Средний уровень моря по допущению GEBCO"
VERTICAL_REFERENCE_CAVEAT = (
    "В мелководных районах исходные данные GEBCO могут иметь иную "
    "вертикальную систему отсчёта."
)
LICENSE = (
    "Общественное достояние; обязательны атрибуция, отсутствие ложного "
    "официального статуса и соблюдение отказа от гарантий GEBCO."
)
LICENSE_URL = (
    "https://www.gebco.net/data-products/gridded-bathymetry-data/"
    "gebco2026-grid#terms-of-use-and-disclaimer"
)
ATTRIBUTION = (
    "GEBCO Bathymetric Compilation Group 2026 (2026). The GEBCO_2026 Grid — "
    "a continuous terrain model for oceans and land at 15 arc-second "
    f"intervals. doi:{SOURCE_PRODUCT_DOI}"
)
OFFICIAL_SOURCE_HOSTS = {"download.gebco.net", "dap.ceda.ac.uk"}

try:
    import xarray as xr
except ImportError:
    xr = None


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


def validate_source_url(value):
    """Принимает только официальный URL зафиксированного источника."""
    parsed = urlparse(value)
    if parsed.scheme != "https" or parsed.hostname not in OFFICIAL_SOURCE_HOSTS:
        hosts = ", ".join(sorted(OFFICIAL_SOURCE_HOSTS))
        raise argparse.ArgumentTypeError(
            f"ожидается HTTPS URL GEBCO_2026 с официального хоста: {hosts}"
        )
    return value


def convert_netcdf_to_json(args):
    """Ресэмплирует GEBCO ближайшим соседом и создаёт JSON с паспортом."""
    if xr is None:
        print(
            "Установите зависимости: "
            "pip install -r cmd/bathymetry/convert/requirements.txt"
        )
        sys.exit(1)
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
        min_lat, max_lat, min_lon, max_lon = BLACK_SEA_BOUNDS
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
            f"Ресэмплированный производный региональный набор {SOURCE_PRODUCT} "
            f"для Чёрного моря ({args.resolution:g}°)"
        ),
        "status": "verified_derived",
        "dataset_file": output_path.name,
        "dataset_sha256": dataset_checksum,
        "created_at": created_at,
        "point_count": len(points),
        "bounds": {
            "min_lat": BLACK_SEA_BOUNDS[0],
            "max_lat": BLACK_SEA_BOUNDS[1],
            "min_lon": BLACK_SEA_BOUNDS[2],
            "max_lon": BLACK_SEA_BOUNDS[3],
        },
        "target_resolution_degrees": args.resolution,
        "target_resolution_arc_seconds": args.resolution * 3600,
        "source_product": SOURCE_PRODUCT,
        "source_product_doi": SOURCE_PRODUCT_DOI,
        "source_url": args.source_url,
        "source_downloaded_at": args.downloaded_at,
        "source_netcdf": input_path.as_posix(),
        "source_netcdf_sha256": source_checksum,
        "source_grid_interval_arc_seconds": SOURCE_GRID_INTERVAL_ARC_SECONDS,
        "horizontal_reference": HORIZONTAL_REFERENCE,
        "vertical_reference": VERTICAL_REFERENCE,
        "vertical_reference_caveat": VERTICAL_REFERENCE_CAVEAT,
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
        "license": LICENSE,
        "license_url": LICENSE_URL,
        "attribution": ATTRIBUTION,
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
        "--source-url",
        required=True,
        type=validate_source_url,
        help="Точный официальный URL загрузки GEBCO_2026",
    )
    parser.add_argument(
        "--downloaded-at",
        required=True,
        type=parse_downloaded_at,
        help="Дата загрузки в ISO 8601 с часовым поясом",
    )
    return parser


if __name__ == "__main__":
    arguments = build_parser().parse_args()
    if arguments.resolution <= 0:
        raise ValueError("Целевой шаг сетки должен быть положительным")
    convert_netcdf_to_json(arguments)
