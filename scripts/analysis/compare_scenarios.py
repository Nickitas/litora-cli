#!/usr/bin/env python3
"""
Сравнение климатических сценариев для Litora-CLI

Сравнивает несколько CSV файлов с разными сценариями моделирования.
"""

import argparse
import sys
from pathlib import Path
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
import numpy as np
import glob


class ScenarioComparator:
    """Класс для сравнения нескольких сценариев"""

    def __init__(self, csv_paths):
        """
        Инициализация компаратора сценариев

        Args:
            csv_paths: Список путей к CSV файлам для сравнения
        """
        self.csv_paths = [Path(p) for p in csv_paths]

        # Проверка существования файлов
        for csv_path in self.csv_paths:
            if not csv_path.exists():
                raise FileNotFoundError(f"CSV файл не найден: {csv_path}")

        # Загрузка данных
        self.scenarios = {}
        for csv_path in self.csv_paths:
            scenario_name = csv_path.stem  # Имя файла без расширения
            self.scenarios[scenario_name] = pd.read_csv(csv_path)

        self._validate_data()

    def _validate_data(self):
        """Проверка структуры данных"""
        required_columns = ['step', 'length_km']

        for name, df in self.scenarios.items():
            missing_columns = [col for col in required_columns if col not in df.columns]
            if missing_columns:
                raise ValueError(f"Сценарий '{name}' отсутствует колонки: {missing_columns}")

    def compare_scenarios(self):
        """Провести сравнение сценариев"""
        comparison = {
            'overview': self._overview_comparison(),
            'final_metrics': self._final_metrics_comparison(),
            'erosion_comparison': self._erosion_comparison(),
            'temporal_comparison': self._temporal_comparison(),
        }

        return ScenarioReport(self.scenarios, comparison)

    def _overview_comparison(self):
        """Обзор сравнения сценариев"""
        overview = {}

        for name, df in self.scenarios.items():
            overview[name] = {
                'total_steps': len(df),
                'initial_length': df['length_km'].iloc[0],
                'final_length': df['length_km'].iloc[-1],
                'length_change': df['length_km'].iloc[-1] - df['length_km'].iloc[0],
            }

        return overview

    def _final_metrics_comparison(self):
        """Сравнение финальных метрик"""
        final_metrics = {}

        for name, df in self.scenarios.items():
            final_metrics[name] = {
                'length_km': df['length_km'].iloc[-1],
                'area_km2': df['area_km2'].iloc[-1] if 'area_km2' in df.columns else None,
                'total_erosion_m3': df['eroded_m3'].sum() if 'eroded_m3' in df.columns else None,
                'storm_count': df['storm_event'].sum() if 'storm_event' in df.columns else 0,
            }

        return final_metrics

    def _erosion_comparison(self):
        """Сравнение эрозии между сценариями"""
        erosion_data = {}

        for name, df in self.scenarios.items():
            if 'eroded_m3' in df.columns and 'net_change_m3' in df.columns:
                erosion_data[name] = {
                    'total_eroded': df['eroded_m3'].sum(),
                    'net_change': df['net_change_m3'].sum(),
                    'mean_annual_erosion': df['eroded_m3'].mean(),
                    'max_single_step': df['eroded_m3'].max(),
                }

        return erosion_data

    def _temporal_comparison(self):
        """Временное сравнение сценариев"""
        temporal_data = {}

        for name, df in self.scenarios.items():
            if 'year' in df.columns:
                temporal_data[name] = {
                    'total_years': df['year'].diff().sum(),
                    'length_change_pct': ((df['length_km'].iloc[-1] - df['length_km'].iloc[0]) /
                                        df['length_km'].iloc[0] * 100),
                    'mean_annual_change_km': (df['length_km'].iloc[-1] - df['length_km'].iloc[0]) /
                                            df['year'].diff().sum() if df['year'].diff().sum() > 0 else 0,
                }

        return temporal_data


class ScenarioReport:
    """Класс для генерации отчета о сравнении сценариев"""

    def __init__(self, scenarios, comparison):
        """
        Инициализация генератора отчетов о сравнении

        Args:
            scenarios: Словарь с DataFrame сценариев
            comparison: Словарь со сравнительными данными
        """
        self.scenarios = scenarios
        self.comparison = comparison

    def generate_comparison_report(self):
        """Генерация текстового отчета о сравнении"""
        lines = []
        lines.append("=" * 70)
        lines.append("СРАВНЕНИЕ КЛИМАТИЧЕСКИХ СЦЕНАРИЕВ")
        lines.append("=" * 70)
        lines.append("")

        # Обзор
        lines.append("ОБЗОР СЦЕНАРИЕВ")
        lines.append("-" * 70)
        for name, metrics in self.comparison['overview'].items():
            lines.append(f"  {name}:")
            lines.append(f"    Шагов:                  {metrics['total_steps']}")
            lines.append(f"    Начальная длина:        {metrics['initial_length']:.2f} км")
            lines.append(f"    Конечная длина:         {metrics['final_length']:.2f} км")
            lines.append(f"    Изменение длины:        {metrics['length_change']:+.2f} км")
            lines.append("")

        # Финальные метрики
        lines.append("ФИНАЛЬНЫЕ МЕТРИКИ")
        lines.append("-" * 70)

        # Создаем таблицу
        headers = ["Сценарий", "Длина (км)", "Площадь (км²)", "Эрозия (м³)", "Штормы"]
        rows = []

        for name in self.scenarios.keys():
            if name in self.comparison['final_metrics']:
                metrics = self.comparison['final_metrics'][name]
                row = [
                    name,
                    f"{metrics['length_km']:.1f}",
                    f"{metrics['area_km2']:.1f}" if metrics['area_km2'] else "N/A",
                    f"{metrics['total_erosion_m3']:.0f}" if metrics['total_erosion_m3'] else "N/A",
                    f"{int(metrics['storm_count'])}" if metrics['storm_count'] is not None else "N/A",
                ]
                rows.append(row)

        # Добавляем строки в отчет
        lines.append("  ".join(headers))
        lines.append("  " + "-" * 60)
        for row in rows:
            lines.append("  ".join(row))
        lines.append("")

        # Временное сравнение
        if self.comparison['temporal_comparison']:
            lines.append("ВРЕМЕННОЕ СРАВНЕНИЕ")
            lines.append("-" * 70)
            for name, metrics in self.comparison['temporal_comparison'].items():
                lines.append(f"  {name}:")
                lines.append(f"    Период:                {metrics['total_years']:.1f} лет")
                lines.append(f"    Изменение (%):         {metrics['length_change_pct']:+.2f}%")
                lines.append(f"    Среднегодовое изменение: {metrics['mean_annual_change_km']:+.3f} км/год")
                lines.append("")

        lines.append("=" * 70)

        return "\n".join(lines)

    def save_report(self, output_path):
        """Сохранение отчета в файл"""
        output_file = Path(output_path)
        output_file.parent.mkdir(parents=True, exist_ok=True)

        report_content = self.generate_comparison_report()
        output_file.write_text(report_content, encoding='utf-8')

        return str(output_file)

    def plot_comparison_dashboard(self, output_path='scenario_comparison.png', figsize=(14, 10)):
        """
        Генерация панели сравнения сценариев

        Args:
            output_path: Путь для сохранения графика
            figsize: Размер графика (ширина, высота)
        """
        fig, axes = plt.subplots(2, 2, figsize=figsize)
        fig.suptitle('СРАВНЕНИЕ СЦЕНАРИЕВ', fontsize=16, fontweight='bold')

        scenario_names = list(self.scenarios.keys())

        # 1. График изменения длины берега
        ax1 = axes[0, 0]
        for name in scenario_names:
            df = self.scenarios[name]
            if 'year' in df.columns:
                x_data = df['year']
            else:
                x_data = df['step']

            ax1.plot(x_data, df['length_km'], marker='o', linewidth=2, markersize=6, label=name)

        ax1.set_xlabel('Время', fontsize=11)
        ax1.set_ylabel('Длина берега (км)', fontsize=11)
        ax1.set_title('Динамика длины береговой линии', fontsize=12, fontweight='bold')
        ax1.legend()
        ax1.grid(True, alpha=0.3)

        # 2. График накопленной эрозии
        ax2 = axes[0, 1]
        for name in scenario_names:
            df = self.scenarios[name]
            if 'net_change_m3' in df.columns:
                if 'year' in df.columns:
                    x_data = df['year']
                else:
                    x_data = df['step']

                cumulative = df['net_change_m3'].cumsum()
                ax2.plot(x_data, cumulative, marker='s', linewidth=2, markersize=6, label=name)

        ax2.set_xlabel('Время', fontsize=11)
        ax2.set_ylabel('Накопленная эрозия (м³)', fontsize=11)
        ax2.set_title('Накопленная эрозия', fontsize=12, fontweight='bold')
        ax2.legend()
        ax2.grid(True, alpha=0.3)

        # 3. Сравнение финальных метрик
        ax3 = axes[1, 0]

        final_lengths = []
        final_erosions = []
        storm_counts = []

        for name in scenario_names:
            if name in self.comparison['final_metrics']:
                metrics = self.comparison['final_metrics'][name]
                final_lengths.append(metrics['length_km'])
                final_erosions.append(metrics['total_erosion_m3'] if metrics['total_erosion_m3'] else 0)
                storm_counts.append(metrics['storm_count'] if metrics['storm_count'] is not None else 0)

        x = np.arange(len(scenario_names))
        width = 0.35

        ax3.bar(x - width/2, final_lengths, width, label='Длина (км)', alpha=0.7)
        ax3.set_xticks(x)
        ax3.set_xticklabels(scenario_names, rotation=45, ha='right')
        ax3.set_ylabel('Длина (км)', fontsize=11)
        ax3.set_title('Сравнение финальных метрик', fontsize=12, fontweight='bold')
        ax3.legend()
        ax3.grid(True, alpha=0.3, axis='y')

        # 4. Сравнение эрозии
        ax4 = axes[1, 1]

        if any(erosion > 0 for erosion in final_erosions):
            colors = ['#d62728' if e > 0 else '#2ca02c' for e in final_erosions]
            ax4.bar(x, final_erosions, color=colors, alpha=0.7, edgecolor='black')
            ax4.set_xticks(x)
            ax4.set_xticklabels(scenario_names, rotation=45, ha='right')
            ax4.set_ylabel('Общая эрозия (м³)', fontsize=11)
            ax4.set_title('Сравнение общей эрозии', fontsize=12, fontweight='bold')
            ax4.grid(True, alpha=0.3, axis='y')
            ax4.axhline(y=0, color='black', linestyle='-', linewidth=0.8)
        else:
            ax4.text(0.5, 0.5, 'Нет данных об эрозии',
                    ha='center', va='center', transform=ax4.transAxes,
                    fontsize=12, style='italic')

        plt.tight_layout()

        # Сохранение
        output_file = Path(output_path)
        output_file.parent.mkdir(parents=True, exist_ok=True)
        plt.savefig(output_file, dpi=300, bbox_inches='tight')
        plt.close()

        return str(output_file)

    def plot_heatmap_comparison(self, output_path='scenario_heatmap.png', figsize=(12, 8)):
        """
        Генерация heatmap сравнения сценариев

        Args:
            output_path: Путь для сохранения графика
            figsize: Размер графика (ширина, высота)
        """
        fig, axes = plt.subplots(2, 2, figsize=figsize)
        fig.suptitle('HEATMAP СРАВНЕНИЯ СЦЕНАРИЕВ', fontsize=16, fontweight='bold')

        scenario_names = list(self.scenarios.keys())

        # Подготовка данных для heatmap
        metrics_data = {}

        for name in scenario_names:
            df = self.scenarios[name]

            # Нормализованные данные для каждого шага
            if len(df) > 0:
                initial_length = df['length_km'].iloc[0]
                normalized_length = (df['length_km'] / initial_length * 100) if initial_length > 0 else df['length_km']
                metrics_data[name] = normalized_length.values

        # Максимальная длина для выравнивания
        max_length = max(len(values) for values in metrics_data.values()) if metrics_data else 0

        # Создаем DataFrame для heatmap
        heatmap_df = pd.DataFrame(index=scenario_names)

        for step in range(max_length):
            step_data = []
            for name in scenario_names:
                if name in metrics_data and step < len(metrics_data[name]):
                    step_data.append(metrics_data[name][step])
                else:
                    step_data.append(np.nan)

            heatmap_df[f'Step {step}'] = step_data

        # 1. Heatmap длины берега (%)
        ax1 = axes[0, 0]
        sns.heatmap(heatmap_df, annot=True, fmt='.1f', cmap='RdYlGn_r',
                   cbar_kws={'label': 'Длина (%) от начальной'},
                   ax=ax1, linewidths=0.5)
        ax1.set_title('Относительная длина береговой линии', fontsize=12, fontweight='bold')
        ax1.set_xlabel('Шаг', fontsize=10)
        ax1.set_ylabel('Сценарий', fontsize=10)

        # 2. Heatmap абсолютной длины
        ax2 = axes[0, 1]

        # Создаем DataFrame для абсолютных длин
        abs_length_data = {}
        for name in scenario_names:
            df = self.scenarios[name]
            abs_length_data[name] = df['length_km'].values

        # Находим максимальное количество шагов
        max_steps = max(len(values) for values in abs_length_data.values())

        # Создаем DataFrame с правильными размерами
        abs_length_df = pd.DataFrame(index=range(max_steps), columns=scenario_names)

        for name in scenario_names:
            abs_length_df[name] = abs_length_data[name]

        # Транспонируем для лучшей визуализации (сценарии как строки)
        abs_length_df = abs_length_df.T

        sns.heatmap(abs_length_df, annot=True, fmt='.0f', cmap='viridis',
                   cbar_kws={'label': 'Длина (км)'},
                   ax=ax2, linewidths=0.5)
        ax2.set_title('Абсолютная длина береговой линии', fontsize=12, fontweight='bold')
        ax2.set_xlabel('Шаг', fontsize=10)
        ax2.set_ylabel('Сценарий', fontsize=10)

        # 3. Сравнение финальных метрик
        ax3 = axes[1, 0]
        ax3.axis('off')

        comparison_text = "ФИНАЛЬНЫЕ МЕТРИКИ\\n\\n"
        for name in scenario_names:
            if name in self.comparison['final_metrics']:
                metrics = self.comparison['final_metrics'][name]
                comparison_text += f"{name}:\\n"
                comparison_text += f"  Длина: {metrics['length_km']:.1f} км\\n"

                if metrics['total_erosion_m3']:
                    comparison_text += f"  Эрозия: {metrics['total_erosion_m3']:.0f} м³\\n"

                if metrics['storm_count'] is not None and metrics['storm_count'] > 0:
                    comparison_text += f"  Штормы: {int(metrics['storm_count'])}\\n"

                comparison_text += "\\n"

        ax3.text(0.1, 0.5, comparison_text, transform=ax3.transAxes,
                fontsize=10, verticalalignment='center', fontfamily='monospace',
                bbox=dict(boxstyle='round', facecolor='wheat', alpha=0.3))

        # 4. Графики сравнения по годам (если есть)
        ax4 = axes[1, 1]

        has_year_data = any('year' in df.columns for df in self.scenarios.values())

        if has_year_data:
            for name in scenario_names:
                df = self.scenarios[name]
                if 'year' in df.columns and 'net_change_m3' in df.columns:
                    cumulative = df['net_change_m3'].cumsum()
                    ax4.plot(df['year'], cumulative, marker='o', linewidth=2, markersize=6, label=name)

            ax4.set_xlabel('Год', fontsize=11)
            ax4.set_ylabel('Накопленная эрозия (м³)', fontsize=11)
            ax4.set_title('Временная эволюция эрозии', fontsize=12, fontweight='bold')
            ax4.legend()
            ax4.grid(True, alpha=0.3)
        else:
            ax4.text(0.5, 0.5, 'Нет временных данных\\nдля визуализации',
                    ha='center', va='center', transform=ax4.transAxes,
                    fontsize=12, style='italic')

        plt.tight_layout()

        # Сохранение
        output_file = Path(output_path)
        output_file.parent.mkdir(parents=True, exist_ok=True)
        plt.savefig(output_file, dpi=300, bbox_inches='tight')
        plt.close()

        return str(output_file)


def main():
    """Главная функция для запуска из командной строки"""
    parser = argparse.ArgumentParser(
        description='Сравнение нескольких сценариев эрозии',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Примеры использования:
  python compare_scenarios.py output/csv/scenario_*.csv
  python compare_scenarios.py output/csv/rcp45_scenario.csv output/csv/rcp85_scenario.csv
  python compare_scenarios.py "output/csv/scenario_*.csv" --heatmap --output climate_comparison
  python compare_scenarios.py output/csv/rcp*.csv --report --output comparison_report
        '''
    )

    parser.add_argument('csv_files', nargs='+', help='Пути к CSV файлам для сравнения (можно использовать glob паттерны)')
    parser.add_argument('--heatmap', action='store_true', help='Генерировать heatmap визуализацию')
    parser.add_argument('--output', '-o', help='Базовое имя для сохранения графиков в output/report/comparison/')
    parser.add_argument('--plots', '-p', help='Альтернатива --output: базовое имя для графиков в output/report/comparison/')
    parser.add_argument('--report', '-r', action='store_true', help='Генерировать текстовый отчет сравнения')

    args = parser.parse_args()

    try:
        # Расширяем glob паттерны
        csv_files = []
        for file_pattern in args.csv_files:
            if '*' in file_pattern or '?' in file_pattern:
                expanded_files = glob.glob(file_pattern)
                csv_files.extend(expanded_files)
            else:
                csv_files.append(file_pattern)

        if not csv_files:
            print("❌ Не найдено CSV файлов по указанному паттерну")
            return 1

        # Сравнение сценариев
        comparator = ScenarioComparator(csv_files)
        comparison = comparator.compare_scenarios()

        # Создаем директорию output/report/comparison/
        output_dir = Path('output/report/comparison')
        output_dir.mkdir(parents=True, exist_ok=True)

        # Базовый путь для выходных файлов
        input_name = args.output or args.plots or 'scenario_comparison'

        # Извлекаем только basename из пути, если пользователь указал директории
        basename = Path(input_name).name

        # Вывод отчета
        if args.report:
            print(comparison.generate_comparison_report())

            report_path = output_dir / f'{basename}.txt'
            comparison.save_report(report_path)
            print(f"\n✓ Отчет сохранен: {report_path}")

        # Генерация графиков
        if args.heatmap:
            plot_path = output_dir / f'{basename}_heatmap.png'
            plot_file = comparison.plot_heatmap_comparison(output_path=plot_path)
            print(f"✓ Heatmap сохранен: {plot_file}")
        else:
            plot_path = output_dir / f'{basename}_dashboard.png'
            plot_file = comparison.plot_comparison_dashboard(output_path=plot_path)
            print(f"✓ График сравнения сохранен: {plot_file}")

        return 0

    except FileNotFoundError as e:
        print(f"❌ Ошибка: {e}", file=sys.stderr)
        return 1
    except ValueError as e:
        print(f"❌ Ошибка данных: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"❌ Неожиданная ошибка: {e}", file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())