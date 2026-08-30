#!/bin/bash
set -euo pipefail

# Загружает региональный NetCDF зафиксированного продукта GEBCO_2026 для
# Чёрного моря и создаёт производный JSON вместе с паспортом происхождения.

if [ "$#" -ne 2 ]; then
    echo "Использование: $0 URL_GEBCO_2026 ВЫХОДНОЙ_JSON"
    echo "Пример: $0 'URL_NETCDF_GEBCO_2026' output/source/black-sea-bathymetry-gebco2026-0.01deg-derived.json"
    exit 1
fi

SOURCE_URL="$1"
FINAL_JSON="$2"
SOURCE_DIR="output/source/netcdf"
SOURCE_NAME="$(basename "${SOURCE_URL%%\?*}")"
SOURCE_NETCDF="${SOURCE_DIR}/${SOURCE_NAME}"
FINAL_PASSPORT="${FINAL_JSON%.*}.metadata.json"
DOWNLOADED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

case "${SOURCE_URL}" in
    https://download.gebco.net/*|https://dap.ceda.ac.uk/*)
        ;;
    *)
        echo "Ошибка: URL должен указывать на официальный хост download.gebco.net или dap.ceda.ac.uk"
        exit 1
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
    --source-url "${SOURCE_URL}" \
    --downloaded-at "${DOWNLOADED_AT}"

echo "Готово. Исходный NetCDF не удалён: он необходим для воспроизведения результата."
