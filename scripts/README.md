# Python Analysis Scripts

Коллекция Python скриптов для анализа CSV данных, генерируемых Litora-CLI.

## Структура

```
scripts/
├── README.md                  # Эта документация
├── requirements.txt           # Зависимости Python
└── analysis/                 # Скрипты анализа
    ├── analyze_erosion.py     # Базовый анализ эрозии
    ├── plot_dynamics.py       # Визуализация динамики
    ├── storm_analysis.py      # Анализ штормов
    ├── compare_scenarios.py   # Сравнение сценариев
    └── export_reports.py      # Генерация отчетов
```

## Установка

```bash
# Установка зависимостей
pip install -r scripts/requirements.txt

# Или используя virtualenv (рекомендуется)
python3 -m venv scripts/venv
source scripts/venv/bin/activate  
# На Windows: venv\Scripts\activate
pip install -r scripts/requirements.txt
```

## Использование

### Базовый анализ эрозии

**Задача:** Получить базовую статистику моделирования эрозии.

```bash
# Моделирование
./lito model erosion --steps 10 --target-years 25 --years-per-step 2.5

# Анализ одного CSV файла
python scripts/analysis/analyze_erosion.py output/csv/erosion_metrics.csv

# С сохранением результатов
python scripts/analysis/analyze_erosion.py output/csv/erosion_metrics.csv --output output/report/analysis.txt

# Только краткая сводка
python scripts/analysis/analyze_erosion.py output/csv/erosion_metrics.csv --summary
```

**Результат:**
- Текстовый отчет с полной статистикой
- Данные о темпах эрозии
- Временной анализ
- Краткая сводка при использовании --summary

### Визуализация динамики

**Задача:** Создать профессиональные графики для презентации.

```bash
# Моделирование
./lito model erosion --steps 12 --target-years 30 --years-per-step 2.5 \
  --storm-probability 0.25 --sea-level-rise 0.01

# Генерация графиков
python scripts/analysis/plot_dynamics.py output/csv/erosion_metrics.csv

# С настройкой стиля
python scripts/analysis/plot_dynamics.py output/csv/erosion_metrics.csv --style seaborn

# Комплексная панель дашборда
python scripts/analysis/plot_dynamics.py output/csv/erosion_metrics.csv \
  --dashboard --output presentation_dashboard --style seaborn

# С настройкой размера графиков
python scripts/analysis/plot_dynamics.py output/csv/erosion_metrics.csv \
  --figsize 12 8 --output custom_plots
```

**Результат:**
- `presentation_dashboard.png` — комплексная панель для презентации
- Все ключевые метрики на одном графике
- Профессиональное оформление
- Настройка размера и стиля графиков

### Анализ штормов

**Задача:** Исследовать влияние штормов на эрозию берега.

```bash
# Моделирование с частыми штормами
./lito model erosion --steps 20 --target-years 40 --years-per-step 2 \
  --storm-probability 0.4 --storm-intensity 2.5 --output-csv storm_analysis.csv

# Статистика штормовых событий
python scripts/analysis/storm_analysis.py output/csv/erosion_metrics.csv

# Детальный анализ
python scripts/analysis/storm_analysis.py output/csv/storm_analysis.csv \
  --detailed --plot --output output/report/storm_report
```

**Результат:**
- Детальный текстовый отчет о штормах
- Графики воздействия штормов
- Сравнение эффективности штормовых событий

### Сравнение сценариев

**Задача:** Сравнить влияние разных уровней подъема моря.

```bash
# Низкий подъем (RCP4.5)
./lito model erosion --steps 15 --target-years 50 --years-per-step 3.33 \
  --sea-level-rise 0.007 --output-csv rcp45_scenario.csv

# Высокий подъем (RCP8.5)
./lito model erosion --steps 15 --target-years 50 --years-per-step 3.33 \
  --sea-level-rise 0.015 --output-csv rcp85_scenario.csv

# Сравнение нескольких CSV файлов
python scripts/analysis/compare_scenarios.py output/csv/rcp45_scenario.csv output/csv/rcp85_scenario.csv

# С heatmap визуализацией
python scripts/analysis/compare_scenarios.py "output/csv/scenario_*.csv" \
  --heatmap --output climate_comparison

# С текстовым отчетом сравнения
python scripts/analysis/compare_scenarios.py output/csv/rcp*.csv \
  --report --output comparison_report.txt
```

**Результат:**
- `climate_comparison.png` — графики сравнения
- `climate_comparison_heatmap.png` — heatmap визуализация
- `comparison_report.txt` — текстовый отчет со статистикой
- Понимание различий между сценариями

### Генерация отчета

**Задача:** Создать профессиональные отчеты для научной публикации.

```bash
# Моделирование с комплексными параметрами
./lito model erosion --steps 20 --target-years 50 --years-per-step 2.5 \
  --storm-probability 0.2 --sea-level-rise 0.01 --enable-seasonality

# Все форматы отчетов
python scripts/analysis/export_reports.py output/csv/erosion_metrics.csv \
  --format all --output output/report/paper_analysis

# Указание директории для отчетов
python scripts/analysis/export_reports.py output/csv/erosion_metrics.csv \
  --format markdown --report-dir my_reports --output output/report/chapter1

# Только Markdown отчет
python scripts/analysis/export_reports.py output/csv/erosion_metrics.csv \
  --format markdown --output output/report/paper_summary
```

**Результат:**
- `reports/paper_analysis.md` — Markdown для документации
- `reports/paper_analysis.json` — JSON для автоматизации
- `reports/paper_analysis.tex` — LaTeX для научных статей
- Настройка директории сохранения через `--report-dir`

### Комплексный анализ

**Задача:** Полный анализ данных для диссертационного исследования.

```bash
# 1. Базовая модель
./lito all --iterations 6 --steps 15 --target-years 30 --years-per-step 2 \
  --storm-probability 0.2 --sea-level-rise 0.008

# 2. Анализ
python scripts/analysis/analyze_erosion.py output/csv/erosion_metrics.csv \
  --output analysis_results.txt

# 3. Визуализации  
python scripts/analysis/plot_dynamics.py output/csv/erosion_metrics.csv \
  --dashboard --output thesis_dashboard --style seaborn

# 4. Штормовый анализ
python scripts/analysis/storm_analysis.py output/csv/erosion_metrics.csv \
  --detailed --plot --output thesis_storms

# 5. Отчеты
python scripts/analysis/export_reports.py output/csv/erosion_metrics.csv \
  --format all --output thesis_chapter
```

**Результат:**
- Полный набор анализов для диссертации
- Профессиональные графики
- LaTeX код для включения в диссертацию
