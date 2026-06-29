#!/usr/bin/env python3
"""
Генерация комплексных отчетов для Litora-CLI

Создает профессиональные отчеты в различных форматах на основе CSV данных.
"""

import argparse
import sys
from pathlib import Path
import pandas as pd
from datetime import datetime
import json


class ReportGenerator:
    """Класс для генерации комплексных отчетов"""

    def __init__(self, csv_path, output_dir=None):
        """
        Инициализация генератора отчетов

        Args:
            csv_path: Путь к CSV файлу с метриками эрозии
            output_dir: Директория для сохранения отчетов (по умолчанию output/report/)
        """
        self.csv_path = Path(csv_path)
        if not self.csv_path.exists():
            raise FileNotFoundError(f"CSV файл не найден: {csv_path}")

        self.df = pd.read_csv(self.csv_path)

        # Всегда используем output/report/ как базовую директорию
        if output_dir:
            self.report_dir = Path(output_dir)
        else:
            self.report_dir = Path('output/report')

        self.report_dir.mkdir(parents=True, exist_ok=True)

        # Метаданные отчета
        self.metadata = {
            'csv_file': str(self.csv_path),
            'generated_at': datetime.now().isoformat(),
            'total_records': len(self.df),
        }

    def generate_markdown_report(self, output_name='erosion_report.md'):
        """
        Генерация Markdown отчета

        Args:
            output_name: Имя выходного файла
        """
        lines = []

        # Заголовок
        lines.append("# Отчет по анализу эрозии береговой линии")
        lines.append("")
        lines.append(f"**Дата генерации:** {datetime.now().strftime('%Y-%m-%d %H:%M')}")
        lines.append(f"**Исходный файл:** `{self.csv_path.name}`")
        lines.append(f"**Количество записей:** {len(self.df)}")
        lines.append("---")
        lines.append("")

        # Резюме
        lines.append("## 📊 Резюме")
        lines.append("")

        if len(self.df) > 0:
            initial_length = self.df['length_km'].iloc[0]
            final_length = self.df['length_km'].iloc[-1]
            length_change = final_length - initial_length
            length_change_pct = (length_change / initial_length) * 100 if initial_length > 0 else 0

            lines.append(f"- **Начальная длина:** {initial_length:.2f} км")
            lines.append(f"- **Конечная длина:** {final_length:.2f} км")
            lines.append(f"- **Изменение длины:** {length_change:+.2f} км ({length_change_pct:+.1f}%)")
        lines.append("")

        if 'year' in self.df.columns:
            total_years = self.df['year'].diff().sum()
            lines.append(f"**Временной период:** {total_years:.1f} лет")
            lines.append("")

        if 'eroded_m3' in self.df.columns:
            total_erosion = self.df['eroded_m3'].sum()
            lines.append(f"**Общая эрозия:** {total_erosion:+.1f} м³")
            lines.append("")

        if 'storm_event' in self.df.columns:
            storm_count = self.df['storm_event'].sum()
            storm_freq = storm_count / len(self.df) if len(self.df) > 0 else 0
            lines.append(f"**Штормовые события:** {int(storm_count)} (частота: {storm_freq:.3f})")
            lines.append("")

        lines.append("---")
        lines.append("")

        # Детальный анализ
        lines.append("## 🔬 Детальный анализ")
        lines.append("")

        # Динамика длины
        lines.append("### Динамика длины береговой линии")
        lines.append("")
        lines.append("| Шаг | Год | Длина (км) | Площадь (км²) |")
        lines.append("|------|-----|------------|--------------|")

        for _, row in self.df.iterrows():
            step = row['step']
            year = f"{row['year']:.1f}" if 'year' in row else "N/A"
            length = f"{row['length_km']:.2f}"
            area = f"{row['area_km2']:.2f}" if 'area_km2' in row else "N/A"

            storm_indicator = " ⛈️" if 'storm_event' in row and row['storm_event'] else ""

            lines.append(f"| {step} | {year} | {length} | {area} |{storm_indicator}")

        lines.append("")

        # Анализ эрозии
        if 'eroded_m3' in self.df.columns and 'net_change_m3' in self.df.columns:
            lines.append("### Анализ эрозии")
            lines.append("")
            lines.append("| Шаг | Эрозия (м³) | Депозиция (м³) | Баланс (м³) |")
            lines.append("|------|--------------|-----------------|---------------|")

            for _, row in self.df.iterrows():
                step = row['step']
                eroded = f"{row['eroded_m3']:.1f}"
                deposited = f"{row['deposited_m3']:.1f}" if 'deposited_m3' in row else "N/A"
                net = f"{row['net_change_m3']:.1f}"

                lines.append(f"| {step} | {eroded} | {deposited} | {net} |")

            lines.append("")

        # Статистика штормов
        if 'storm_event' in self.df.columns:
            lines.append("### Статистика штормов")
            lines.append("")

            storm_steps = self.df[self.df['storm_event'] == True]
            normal_steps = self.df[self.df['storm_event'] == False]

            lines.append(f"- **Общее количество штормов:** {len(storm_steps)}")
            lines.append(f"- **Обычных шагов:** {len(normal_steps)}")
            lines.append(f"- **Частота штормов:** {len(storm_steps) / len(self.df):.3f}")
            lines.append("")

            if 'net_change_m3' in self.df.columns:
                storm_erosion = storm_steps['net_change_m3'].sum() if len(storm_steps) > 0 else 0
                normal_erosion = normal_steps['net_change_m3'].sum() if len(normal_steps) > 0 else 0

                lines.append("**Эрозия по типам условий:**")
                lines.append(f"- Во время штормов: {storm_erosion:+.1f} м³")
                lines.append(f"- В обычное время: {normal_erosion:+.1f} м³")
                lines.append("")

        # Выводы
        lines.append("## 📈 Выводы")
        lines.append("")

        conclusions = self._generate_conclusions()
        for i, conclusion in enumerate(conclusions, 1):
            lines.append(f"{i}. {conclusion}")

        lines.append("")
        lines.append("---")
        lines.append("*Отчет сгенерирован автоматически с помощью Litora-CLI*")

        # Сохранение
        report_path = self.report_dir / output_name
        with open(report_path, 'w', encoding='utf-8') as f:
            f.write("\n".join(lines))

        return str(report_path)

    def generate_json_report(self, output_name='erosion_report.json'):
        """
        Генерация JSON отчета

        Args:
            output_name: Имя выходного файла
        """
        report_data = {
            'metadata': self.metadata,
            'summary': self._generate_summary(),
            'analysis': self._generate_detailed_analysis(),
        }

        # Сохранение
        report_path = self.report_dir / output_name
        with open(report_path, 'w', encoding='utf-8') as f:
            json.dump(report_data, f, indent=2, ensure_ascii=False)

        return str(report_path)

    def generate_latex_report(self, output_name='erosion_report.tex'):
        """
        Генерация LaTeX отчета

        Args:
            output_name: Имя выходного файла
        """
        lines = []

        lines.append("\\documentclass{article}")
        lines.append("\\usepackage[utf8]{inputenc}")
        lines.append("\\usepackage[russian]{babel}")
        lines.append("\\usepackage{geometry}")
        lines.append("\\usepackage{booktabs}")
        lines.append("\\usepackage{graphicx}")
        lines.append("")
        lines.append("\\geometry{a4paper, margin=1in}")
        lines.append("")
        lines.append("\\title{Анализ эрозии береговой линии}")
        lines.append(f"\\author{{Сгенерировано: {datetime.now().strftime('%Y-%m-%d %H:%M')}}}")
        lines.append("\\date{}")
        lines.append("")
        lines.append("\\begin{document}")
        lines.append("\\maketitle")
        lines.append("")

        # Резюме
        lines.append("\\section{Резюме}")

        if len(self.df) > 0:
            initial_length = self.df['length_km'].iloc[0]
            final_length = self.df['length_km'].iloc[-1]
            length_change = final_length - initial_length
            length_change_pct = (length_change / initial_length) * 100 if initial_length > 0 else 0

            lines.append("\\begin{itemize}")
            lines.append(f"  \\item Начальная длина: {initial_length:.2f} км")
            lines.append(f"  \\item Конечная длина: {final_length:.2f} км")
            lines.append(f"  \\item Изменение длины: {length_change:+.2f} км ({length_change_pct:+.1f}\\%)")
            lines.append("\\end{itemize}")

        if 'year' in self.df.columns:
            total_years = self.df['year'].diff().sum()
            lines.append(f"Временной период: {total_years:.1f} лет\\\\")
            lines.append("")

        if 'eroded_m3' in self.df.columns:
            total_erosion = self.df['eroded_m3'].sum()
            lines.append(f"Общая эрозия: {total_erosion:+.1f} м$^3$\\\\")
            lines.append("")

        # Таблица данных
        lines.append("\\section{Детальные данные}")
        lines.append("")
        lines.append("\\begin{table}[h]")
        lines.append("\\centering")
        lines.append("\\begin{tabular}{|c|c|c|}")
        lines.append("\\hline")
        lines.append("Шаг & Длина (км) & Площадь (км$^2$ \\\\ ")
        lines.append("\\hline")

        for _, row in self.df.iterrows():
            step = row['step']
            length = f"{row['length_km']:.2f}"
            area = f"{row['area_km2']:.2f}" if 'area_km2' in row else "N/A"

            storm_indicator = " $\\hat{\\text{P}}$" if 'storm_event' in row and row['storm_event'] else ""

            lines.append(f"{step} & {length} & {area}{storm_indicator} \\\\ ")
            lines.append("\\hline")

        lines.append("\\end{tabular}")
        lines.append("\\caption{Динамика береговой линии по шагам}")
        lines.append("\\end{table}")
        lines.append("")
        lines.append("\\end{document}")

        # Сохранение
        report_path = self.report_dir / output_name
        with open(report_path, 'w', encoding='utf-8') as f:
            f.write("\n".join(lines))

        return str(report_path)

    def _generate_summary(self):
        """Генерация сводки данных"""
        summary = {
            'total_steps': len(self.df),
            'initial_length_km': float(self.df['length_km'].iloc[0]) if len(self.df) > 0 else 0,
            'final_length_km': float(self.df['length_km'].iloc[-1]) if len(self.df) > 0 else 0,
        }

        if 'year' in self.df.columns:
            summary['simulated_years'] = float(self.df['year'].diff().sum())

        if 'eroded_m3' in self.df.columns:
            summary['total_erosion_m3'] = float(self.df['eroded_m3'].sum())

        if 'storm_event' in self.df.columns:
            summary['storm_events'] = int(self.df['storm_event'].sum())

        return summary

    def _generate_detailed_analysis(self):
        """Генерация детального анализа"""
        analysis = {}

        # Анализ по каждому шагу
        steps_data = []
        for _, row in self.df.iterrows():
            step_data = {
                'step': int(row['step']),
                'length_km': float(row['length_km']),
            }

            if 'year' in row:
                step_data['year'] = float(row['year'])

            if 'area_km2' in row:
                step_data['area_km2'] = float(row['area_km2'])

            if 'eroded_m3' in row:
                step_data['eroded_m3'] = float(row['eroded_m3'])

            if 'storm_event' in row:
                step_data['storm_event'] = bool(row['storm_event'])

            steps_data.append(step_data)

        analysis['steps'] = steps_data

        return analysis

    def _generate_conclusions(self):
        """Генерация выводов"""
        conclusions = []

        if len(self.df) == 0:
            return ["Нет данных для анализа"]

        # Анализ изменения длины
        initial_length = self.df['length_km'].iloc[0]
        final_length = self.df['length_km'].iloc[-1]
        length_change = final_length - initial_length
        length_change_pct = (length_change / initial_length) * 100 if initial_length > 0 else 0

        if length_change < 0:
            conclusions.append(f"Береговая линия сократилась на {abs(length_change):.2f} км ({abs(length_change_pct):.1f}%) за период моделирования")
        else:
            conclusions.append(f"Береговая линия увеличилась на {length_change:.2f} км ({length_change_pct:.1f}%) за период моделирования")

        # Анализ эрозии
        if 'eroded_m3' in self.df.columns:
            total_erosion = self.df['eroded_m3'].sum()
            if total_erosion > 0:
                conclusions.append(f"Накопленная эрозия составила {total_erosion:.1f} м$^3$ материала")
            else:
                conclusions.append("Наблюдается чистая аккумуляция материала")

        # Анализ штормов
        if 'storm_event' in self.df.columns:
            storm_count = self.df['storm_event'].sum()
            if storm_count > 0:
                storm_freq = storm_count / len(self.df)
                conclusions.append(f"Зафиксировано {int(storm_count)} штормовых событий (частота: {storm_freq:.3f})")

                # Проверка эффективности штормов
                if 'net_change_m3' in self.df.columns:
                    storm_erosion = self.df[self.df['storm_event'] == True]['net_change_m3'].sum()
                    normal_erosion = self.df[self.df['storm_event'] == False]['net_change_m3'].sum()

                    if abs(storm_erosion) > abs(normal_erosion):
                        conclusions.append("Штормовые события вносят основной вклад в общую эрозию")

        # Временные тенденции
        if len(self.df['length_km']) > 2:
            # Простая линейная регрессия
            x = range(len(self.df))
            y = self.df['length_km'].values

            # Коэффициент наклона
            n = len(y)
            sum_x = sum(x)
            sum_y = sum(y)
            sum_xy = sum(xi * yi for xi, yi in zip(x, y))
            sum_x2 = sum(xi ** 2 for xi in x)

            slope = (n * sum_xy - sum_x * sum_y) / (n * sum_x2 - sum_x ** 2)

            if slope < -0.1:
                conclusions.append(f"Обнаружена устойчивая тенденция к сокращению береговой линии (тренд: {slope:.3f} км/шаг)")
            elif slope > 0.1:
                conclusions.append(f"Обнаружена устойчивая тенденция к росту береговой линии (тренд: {slope:.3f} км/шаг)")
            else:
                conclusions.append("Динамика береговой линии относительно стабильна")

        return conclusions


def main():
    """Главная функция для запуска из командной строки"""
    parser = argparse.ArgumentParser(
        description='Генерация комплексных отчетов по анализу эрозии',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Примеры использования:
  python export_reports.py output/csv/erosion_metrics.csv
  python export_reports.py output/csv/erosion_metrics.csv --format markdown
  python export_reports.py output/csv/erosion_metrics.csv --format all --output paper_analysis
  python export_reports.py output/csv/erosion_metrics.csv --format json --output chapter1
        '''
    )

    parser.add_argument('csv_file', help='Путь к CSV файлу с метриками эрозии')
    parser.add_argument('--format', '-f', choices=['markdown', 'json', 'latex', 'all'],
                       default='markdown', help='Формат отчета')
    parser.add_argument('--output', '-o', default='erosion_report',
                       help='Базовое имя для сохранения отчетов в output/report/')

    args = parser.parse_args()

    try:
        # Создание генератора отчетов
        generator = ReportGenerator(args.csv_file)

        # Извлекаем только basename из пути, если пользователь указал директории
        output_basename = Path(args.output).name

        if args.format == 'markdown' or args.format == 'all':
            md_path = generator.generate_markdown_report(f"{output_basename}.md")
            print(f"✓ Markdown отчет: {md_path}")

        if args.format == 'json' or args.format == 'all':
            json_path = generator.generate_json_report(f"{output_basename}.json")
            print(f"✓ JSON отчет: {json_path}")

        if args.format == 'latex' or args.format == 'all':
            latex_path = generator.generate_latex_report(f"{output_basename}.tex")
            print(f"✓ LaTeX отчет: {latex_path}")
            print(f"  Для компиляции: pdflatex {latex_path}")

        return 0

    except FileNotFoundError as e:
        print(f"❌ Ошибка: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"❌ Неожиданная ошибка: {e}", file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())