#!/bin/bash
set -euo pipefail

# Загружает выбранный пользователем региональный NetCDF GEBCO, сохраняет
# исходник и создаёт производный JSON вместе с паспортом происхождения.

if [ "$#" -ne 4 ]; then
    echo "Использование: $0 URL ПРОДУКТ DOI ВЫХОДНОЙ_JSON"
    echo "Пример: $0 'URL_ИЗ_GEBCO' GEBCO_2026 '10.5285/4f68d5c7-45eb-f999-e063-7086abc036fa' output/source/black-sea-bathymetry-gebco2026-0.01deg-derived.json"
    exit 1
fi

SOURCE_URL="$1"
SOURCE_PRODUCT="$2"
SOURCE_DOI="$3"
FINAL_JSON="$4"
PRODUCT_LOWER="${SOURCE_PRODUCT,,}"
PRODUCT_SLUG="${PRODUCT_LOWER/_/}"
TERMS_URL="https://www.gebco.net/data-products-gridded-bathymetry-data/${PRODUCT_SLUG}-grid"
SOURCE_DIR="output/source/netcdf"
SOURCE_NAME="$(basename "${SOURCE_URL%%\?*}")"
SOURCE_NETCDF="${SOURCE_DIR}/${SOURCE_NAME}"
FINAL_PASSPORT="${FINAL_JSON%.*}.metadata.json"
DOWNLOADED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

case "${SOURCE_PRODUCT}" in
    GEBCO_2026)
        ATTRIBUTION="GEBCO Bathymetric Compilation Group 2026 (2026). The GEBCO_2026 Grid — a continuous terrain model for oceans and land at 15 arc-second intervals. doi:${SOURCE_DOI}"
        ;;
    GEBCO_2024)
        ATTRIBUTION="GEBCO Compilation Group (2024). GEBCO 2024 Grid. doi:${SOURCE_DOI}"
        ;;
    *)
        ATTRIBUTION="${SOURCE_PRODUCT}. doi:${SOURCE_DOI}"
        ;;
esac

if [ -z "${SOURCE_NAME}" ] || [ "${SOURCE_NAME}" = "/" ]; then
    echo "Ошибка: URL не содержит имени NetCDF"
    exit 1
fi

mkdir -p "${SOURCE_DIR}" "$(dirname "${FINAL_JSON}")"

if [ -e "${SOURCE_NETCDF}" ]; then
    echo "Ошибка: исходный файл уже существует и не будет перезаписан: ${SOURCE_NETCDF}"
    exit 1
fi
if [ -e "${FINAL_JSON}" ] || [ -e "${FINAL_PASSPORT}" ]; then
    echo "Ошибка: производный набор или паспорт уже существует; выберите новое имя"
    exit 1
fi

echo "Загрузка исходного NetCDF: ${SOURCE_URL}"
DOWNLOAD_TEMP="$(mktemp "${SOURCE_DIR}/.bathymetry-download.XXXXXX")"
trap 'rm -f "${DOWNLOAD_TEMP}"' EXIT
curl --fail --location --output "${DOWNLOAD_TEMP}" "${SOURCE_URL}"
mv "${DOWNLOAD_TEMP}" "${SOURCE_NETCDF}"
trap - EXIT
echo "Исходный NetCDF сохранён: ${SOURCE_NETCDF}"

python3 cmd/bathymetry/convert/convert_bathymetry.py \
    --input "${SOURCE_NETCDF}" \
    --output "${FINAL_JSON}" \
    --resolution 0.01 \
    --bounds 40.5 46.5 27.5 42.5 \
    --source-product "${SOURCE_PRODUCT}" \
    --source-product-doi "${SOURCE_DOI}" \
    --source-url "${SOURCE_URL}" \
    --downloaded-at "${DOWNLOADED_AT}" \
    --source-grid-interval-arc-seconds 15 \
    --horizontal-reference "WGS84 по допущению GEBCO" \
    --vertical-reference "Средний уровень моря по допущению GEBCO" \
    --vertical-reference-caveat "В мелководных районах исходные данные GEBCO могут иметь иную вертикальную систему отсчёта." \
    --license "Общественное достояние; обязательны атрибуция, отсутствие ложного официального статуса и соблюдение отказа от гарантий GEBCO." \
    --license-url "${TERMS_URL}" \
    --attribution "${ATTRIBUTION}"

echo "Готово. Исходный NetCDF не удалён: он необходим для воспроизведения результата."
