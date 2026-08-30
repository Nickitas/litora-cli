#!/usr/bin/env python3
"""Анализ актуальных артефактов, созданных командами Lito.

Скрипт не пересчитывает модель: он читает JSON/CSV из output и помогает
проверить интерпретируемость результата, устойчивость box-counting,
батиметрические профили и качество вариантов расчётной сетки.
"""

import argparse
import csv
import json
from pathlib import Path

import matplotlib.pyplot as plt


def read_json(path):
    with path.open(encoding="utf-8") as stream:
        return json.load(stream)


def find_first(root, names):
    for name in names:
        matches = sorted(root.rglob(name))
        if matches:
            return matches[0]
    return None


def plot_profiles(path, out):
    with path.open(encoding="utf-8", newline="") as stream:
        rows = list(csv.DictReader(stream))
    groups = {}
    for row in rows:
        groups.setdefault(row["profile_name"], []).append(row)
    fig, ax = plt.subplots(figsize=(10, 6))
    for name, values in groups.items():
        values.sort(key=lambda item: float(item["distance_m"]))
        ax.plot([float(v["distance_m"]) for v in values],
                [float(v["water_depth_m"]) for v in values], marker=".", label=name)
    ax.set(xlabel="Расстояние от берега, м", ylabel="Глубина, м",
           title="Профили от берега к глубоководью")
    ax.invert_yaxis()
    ax.grid(alpha=0.25)
    ax.legend()
    fig.tight_layout()
    fig.savefig(out, dpi=160)
    plt.close(fig)
    return len(groups), len(rows)


def plot_dimension(path, out):
    data = read_json(path)
    points = []
    for item in data.get("iterations", []):
        dimension = item.get("dimension") or {}
        if dimension.get("valid"):
            points.append((item["iteration"], dimension["dimension"],
                           dimension.get("regression_r_squared", 0)))
    if not points:
        return 0
    fig, ax = plt.subplots(figsize=(9, 5))
    ax.plot([p[0] for p in points], [p[1] for p in points], "o-")
    ax.set(xlabel="Итерация", ylabel="Фрактальная размерность D",
           title="Устойчивость оценки box-counting")
    ax.grid(alpha=0.25)
    fig.tight_layout()
    fig.savefig(out, dpi=160)
    plt.close(fig)
    return len(points)


def mesh_summary(path):
    data = read_json(path)
    rows = []
    for level in data.get("levels", []):
        for result in level.get("results", []):
            metrics = result.get("metrics", {})
            rows.append((level.get("target_edge_meters"), result.get("algorithm"),
                         metrics.get("composite_score"), metrics.get("mean_cell_quality"),
                         metrics.get("cell_count")))
    return rows


def main():
    parser = argparse.ArgumentParser(description="Анализ выходных артефактов Lito")
    parser.add_argument("output_dir", nargs="?", default="output", help="каталог output")
    parser.add_argument("--output", default=None, help="каталог для отчёта и графиков")
    args = parser.parse_args()
    root = Path(args.output_dir)
    destination = Path(args.output) if args.output else root / "analysis"
    destination.mkdir(parents=True, exist_ok=True)
    lines = ["ОТЧЁТ АНАЛИЗА ВЫХОДОВ LITO", "=" * 32, f"Каталог: {root}", ""]

    profiles = find_first(root, ["profiles.csv"])
    if profiles:
        count, records = plot_profiles(profiles, destination / "bathymetry_profiles.png")
        lines.append(f"Батиметрия: {count} профилей, {records} точек")
    else:
        lines.append("Батиметрия: profiles.csv не найден")

    dimension = find_first(root, ["dimension.metrics.json"])
    if dimension:
        count = plot_dimension(dimension, destination / "dimension_convergence.png")
        data = read_json(dimension)
        lines.append(f"Box-counting: {count} валидных итераций")
        if data.get("report_metadata", {}).get("usage_limitations"):
            lines.append("Ограничения: " + "; ".join(data["report_metadata"]["usage_limitations"]))
    else:
        lines.append("Box-counting: dimension.metrics.json не найден")

    mesh = find_first(root, ["mesh-comparison.json"])
    rows = mesh_summary(mesh) if mesh else []
    if rows:
        lines.append("Сетка: варианты (размер, алгоритм, score, качество, ячейки)")
        for edge, algorithm, score, quality, cells in rows:
            lines.append(f"  {edge:g} м, {algorithm}: score={score:.2f}, качество={quality:.3f}, ячейки={cells}")
    else:
        lines.append("Сетка: mesh-comparison.json не найден")

    report = destination / "analysis_report.txt"
    report.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print("\n".join(lines))
    print(f"\nРезультаты сохранены в: {destination}")


if __name__ == "__main__":
    main()
