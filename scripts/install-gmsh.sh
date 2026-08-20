#!/usr/bin/env sh
set -eu

# Скрипт загружает официальный SDK Gmsh внутрь output/, не меняя системную
# установку. Версию можно переопределить переменной GMSH_VERSION.
gmsh_version="${GMSH_VERSION:-4.15.2}"
target_dir="${1:-output/mesh/tools}"
system_name="$(uname -s)"
machine_name="$(uname -m)"

case "${system_name}/${machine_name}" in
  Darwin/arm64)
    platform_dir="macOS"
    archive="gmsh-${gmsh_version}-MacOSARM-sdk.tgz"
    ;;
  Darwin/x86_64)
    platform_dir="macOS"
    archive="gmsh-${gmsh_version}-MacOSX-sdk.tgz"
    ;;
  Linux/x86_64)
    platform_dir="Linux"
    archive="gmsh-${gmsh_version}-Linux64-sdk.tgz"
    ;;
  *)
    echo "Неподдерживаемая платформа: ${system_name}/${machine_name}" >&2
    echo "Скачайте Gmsh вручную и передайте путь через lito mesh --gmsh." >&2
    exit 1
    ;;
esac

mkdir -p "${target_dir}"
archive_path="${target_dir}/${archive}"
download_url="https://gmsh.info/bin/${platform_dir}/${archive}"

echo "Загрузка Gmsh ${gmsh_version}: ${download_url}"
curl -L --fail --show-error "${download_url}" -o "${archive_path}"
tar -xzf "${archive_path}" -C "${target_dir}"

binary_path="${target_dir}/${archive%.tgz}/bin/gmsh"
if [ ! -x "${binary_path}" ]; then
  echo "Исполняемый файл Gmsh не найден после распаковки: ${binary_path}" >&2
  exit 1
fi

echo "Gmsh установлен локально: ${binary_path}"
"${binary_path}" --version
