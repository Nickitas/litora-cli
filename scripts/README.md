# Анализ результатов Lito

Скрипты, работающие с актуальными артефактами CLI Lito.
Все команды выполняются из корня репозитория.

## Установка

```bash
python3 -m venv scripts/venv
source scripts/venv/bin/activate
python -m pip install -r scripts/requirements.txt
```

Графики используют backend `Agg` и подходят для серверов и CI.

## Сценарии использования

### Сводный анализ

`analyze_outputs.py` читает `profiles.csv`, `dimension.metrics.json` и
`mesh-comparison.json` из `output` и вложенных каталогов.

```bash
./lito seabed render --input output/seabed/black-sea-depth.msh
./lito dimension --iterations 5
python scripts/analysis/analyze_outputs.py output
```

Создаются `output/analysis/analysis_report.txt`,
`bathymetry_profiles.png` и `dimension_convergence.png`.

### Анализ эрозии

```bash
./lito erosion --black-sea-sochi --steps 20 \
  --output output --output-csv output/csv/erosion.csv
python scripts/analysis/analyze_erosion.py output/csv/erosion.csv \
  --output output/analysis/erosion_report.txt
```

### Графики динамики

```bash
./lito erosion --black-sea-sochi --steps 20 \
  --storm-probability 0.25 --output output \
  --output-csv output/csv/storm-erosion.csv
python scripts/analysis/plot_dynamics.py output/csv/storm-erosion.csv \
  --dashboard --output output/analysis/erosion_dashboard
```

### Анализ штормов

```bash
./lito erosion --black-sea-sochi --steps 30 \
  --storm-probability 0.3 --storm-intensity 2.5 \
  --output output --output-csv output/csv/storms.csv
python scripts/analysis/storm_analysis.py output/csv/storms.csv \
  --detailed --plot --output output/analysis/storms
```

### Сравнение сценариев

```bash
./lito erosion --black-sea-sochi --steps 20 --storm-probability 0.1 \
  --output output --output-csv output/csv/scenario-calm.csv
./lito erosion --black-sea-sochi --steps 20 --storm-probability 0.4 \
  --output output --output-csv output/csv/scenario-stormy.csv
python scripts/analysis/compare_scenarios.py \
  output/csv/scenario-calm.csv output/csv/scenario-stormy.csv \
  --report --heatmap --output scenario-comparison
```

Результаты сравнения сохраняются в `output/report/comparison/`.

### Экспорт отчётов

```bash
python scripts/analysis/export_reports.py output/csv/storms.csv \
  --format all --output output/analysis/erosion-report
```

Создаются Markdown, JSON и LaTeX-версии отчёта.

## Файлы каталога `analysis`

- `analyze_outputs.py` — сводка `seabed`, `dimension`, `mesh`;
- `analyze_erosion.py` — числовая статистика эрозии;
- `plot_dynamics.py` — графики динамики;
- `storm_analysis.py` — анализ штормовых состояний;
- `compare_scenarios.py` — параметрическое сравнение CSV;
- `export_reports.py` — отчёты Markdown/JSON/LaTeX;
- `cli_csv.py` — внутренний адаптер русских имён CSV.

Демонстрационный режим `--black-sea-sochi` не является калибровкой годового
размыва. Нулевые значения на графиках означают отсутствие изменения в
исходном запуске, а не неисправность построения.
