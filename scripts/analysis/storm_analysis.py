#!/usr/bin/env python3
"""
Анализ штормовых событий для Litora-CLI

Детальный анализ влияния штормов на эрозию береговой линии.
"""

import argparse
import sys
from pathlib import Path
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
import numpy as np


class StormAnalyzer:
    """Класс для анализа штормовых событий"""

    def __init__(self, csv_path):
        """
        Инициализация анализатора штормов

        Args:
            csv_path: Путь к CSV файлу с метриками эрозии
        """
        self.csv_path = Path(csv_path)
        if not self.csv_path.exists():
            raise FileNotFoundError(f"CSV файл не найден: {csv_path}")

        self.df = pd.read_csv(self.csv_path)
        self._validate_data()

    def _validate_data(self):
        """Проверка структуры данных"""
        if 'storm_event' not in self.df.columns:
            raise ValueError("Колонка 'storm_event' не найдена в данных. "
                           "Убедитесь, что моделирование проводилось с --storm-probability параметром.")

    def analyze_storms(self):
        """Провести полный анализ штормовых событий"""
        stats = {
            'basic_stats': self._basic_statistics(),
            'comparison': self._storm_vs_normal(),
            'temporal_analysis': self._temporal_analysis(),
            'impact_analysis': self._impact_analysis(),
        }

        return StormReport(self.df, stats)

    def _basic_statistics(self):
        """Базовая статистика штормов"""
        total_steps = len(self.df)
        storm_steps = self.df[self.df['storm_event'] == True]
        normal_steps = self.df[self.df['storm_event'] == False]

        return {
            'total_steps': total_steps,
            'storm_count': len(storm_steps),
            'normal_count': len(normal_steps),
            'storm_frequency': len(storm_steps) / total_steps if total_steps > 0 else 0,
        }

    def _storm_vs_normal(self):
        """Сравнение штормовых и обычных условий"""
        storm_steps = self.df[self.df['storm_event'] == True]
        normal_steps = self.df[self.df['storm_event'] == False]

        comparison = {}

        # Сравнение длины берега
        if 'length_km' in self.df.columns:
            storm_mean_length = storm_steps['length_km'].mean() if len(storm_steps) > 0 else 0
            normal_mean_length = normal_steps['length_km'].mean() if len(normal_steps) > 0 else 0

            comparison['length_km'] = {
                'storm_mean': storm_mean_length,
                'normal_mean': normal_mean_length,
                'difference': storm_mean_length - normal_mean_length,
            }

        # Сравнение эрозии
        if 'net_change_m3' in self.df.columns:
            storm_erosion = storm_steps['net_change_m3'].sum() if len(storm_steps) > 0 else 0
            normal_erosion = normal_steps['net_change_m3'].sum() if len(normal_steps) > 0 else 0

            comparison['erosion_m3'] = {
                'storm_total': storm_erosion,
                'normal_total': normal_erosion,
                'storm_per_event': storm_erosion / len(storm_steps) if len(storm_steps) > 0 else 0,
                'normal_per_step': normal_erosion / len(normal_steps) if len(normal_steps) > 0 else 0,
                'efficiency_ratio': (storm_erosion / len(storm_steps)) / (normal_erosion / len(normal_steps))
                                if len(storm_steps) > 0 and len(normal_steps) > 0 and normal_erosion > 0 else 0,
            }

        return comparison

    def _temporal_analysis(self):
        """Временной анализ штормов"""
        if 'year' not in self.df.columns:
            return None

        storm_steps = self.df[self.df['storm_event'] == True]
        normal_steps = self.df[self.df['storm_event'] == False]

        analysis = {}

        # Распределение штормов по времени
        if len(storm_steps) > 0:
            storm_years = storm_steps['year'].values
            analysis['storm_years'] = storm_years.tolist()
            analysis['storm_interval_mean'] = np.diff(storm_years).mean() if len(storm_years) > 1 else 0
            analysis['storm_interval_std'] = np.diff(storm_years).std() if len(storm_years) > 1 else 0

        # Сезонность (если есть данные)
        if 'seasonal_factor' in self.df.columns:
            analysis['storm_seasonal_mean'] = storm_steps['seasonal_factor'].mean() if len(storm_steps) > 0 else 0
            analysis['normal_seasonal_mean'] = normal_steps['seasonal_factor'].mean() if len(normal_steps) > 0 else 0

        return analysis

    def _impact_analysis(self):
        """Анализ воздействия штормов"""
        storm_steps = self.df[self.df['storm_event'] == True]

        analysis = {}

        # Максимальное воздействие за один шторм
        if 'net_change_m3' in self.df.columns and len(storm_steps) > 0:
            analysis['max_single_storm_erosion'] = storm_steps['net_change_m3'].max()
            analysis['min_single_storm_erosion'] = storm_steps['net_change_m3'].min()
            analysis['mean_single_storm_erosion'] = storm_steps['net_change_m3'].mean()

        # Влияние на длину берега
        if 'length_km' in self.df.columns and len(storm_steps) > 0:
            analysis['storm_length_change_mean'] = storm_steps['length_km'].diff().mean()
            analysis['storm_length_change_std'] = storm_steps['length_km'].diff().std()

        # Доля штормовой эрозии в общей
        if 'net_change_m3' in self.df.columns:
            total_erosion = self.df['net_change_m3'].sum()
            storm_erosion = storm_steps['net_change_m3'].sum() if len(storm_steps) > 0 else 0
            analysis['storm_contribution_pct'] = (storm_erosion / total_erosion * 100) if total_erosion != 0 else 0

        return analysis


class StormReport:
    """Класс для генерации отчета об анализе штормов"""

    def __init__(self, df, stats):
        """
        Инициализация генератора отчетов о штормах

        Args:
            df: DataFrame с данными
            stats: Словарь со статистикой
        """
        self.df = df
        self.stats = stats

    def generate_detailed_report(self):
        """Генерация детального текстового отчета"""
        lines = []
        lines.append("=" * 70)
        lines.append("ДЕТАЛЬНЫЙ АНАЛИЗ ШТОРМОВЫХ СОБЫТИЙ")
        lines.append("=" * 70)
        lines.append("")

        # Базовая статистика
        lines.append("БАЗОВАЯ СТАТИСТИКА")
        lines.append("-" * 70)
        basic = self.stats['basic_stats']
        lines.append(f"  Общее количество шагов:          {basic['total_steps']}")
        lines.append(f"  Штормовых событий:              {basic['storm_count']}")
        lines.append(f"  Обычных условий:                {basic['normal_count']}")
        lines.append(f"  Частота штормов:                {basic['storm_frequency']:.3f}")
        lines.append("")

        # Сравнение штормовых и обычных условий
        lines.append("СРАВНЕНИЕ ШТОРМОВЫХ И ОБЫЧНЫХ УСЛОВИЙ")
        lines.append("-" * 70)
        comparison = self.stats['comparison']

        if 'length_km' in comparison:
            length_comp = comparison['length_km']
            lines.append("  Длина береговой линии:")
            lines.append(f"    Средняя во время штормов:     {length_comp['storm_mean']:.2f} км")
            lines.append(f"    Средняя в обычное время:      {length_comp['normal_mean']:.2f} км")
            lines.append(f"    Разница:                      {length_comp['difference']:+.2f} км")
            lines.append("")

        if 'erosion_m3' in comparison:
            erosion_comp = comparison['erosion_m3']
            lines.append("  Эрозия:")
            lines.append(f"    Всего во время штормов:       {erosion_comp['storm_total']:+.1f} м³")
            lines.append(f"    Всего в обычное время:        {erosion_comp['normal_total']:+.1f} м³")
            lines.append(f"    В среднем за шторм:           {erosion_comp['storm_per_event']:+.1f} м³/шторм")
            lines.append(f"    В среднем за обычный шаг:     {erosion_comp['normal_per_step']:+.1f} м³/шаг")
            if erosion_comp['efficiency_ratio'] > 0:
                lines.append(f"    Коэффициент эффективности:  {erosion_comp['efficiency_ratio']:.2f}x")
            lines.append("")

        # Временной анализ
        if self.stats['temporal_analysis']:
            lines.append("ВРЕМЕННОЙ АНАЛИЗ")
            lines.append("-" * 70)
            temporal = self.stats['temporal_analysis']

            if 'storm_years' in temporal and temporal['storm_years']:
                lines.append(f"  Годы штормов:                  {', '.join(map(str, temporal['storm_years']))}")
                lines.append(f"  Средний интервал:              {temporal['storm_interval_mean']:.1f} лет")
                lines.append(f"  Стандартное отклонение:        {temporal['storm_interval_std']:.1f} лет")

            if 'storm_seasonal_mean' in temporal:
                lines.append(f"  Сезонный множитель (штормы):   {temporal['storm_seasonal_mean']:.3f}")
                lines.append(f"  Сезонный множитель (норма):    {temporal['normal_seasonal_mean']:.3f}")
            lines.append("")

        # Анализ воздействия
        lines.append("АНАЛИЗ ВОЗДЕЙСТВИЯ")
        lines.append("-" * 70)
        impact = self.stats['impact_analysis']

        if 'max_single_storm_erosion' in impact:
            lines.append(f"  Макс. эрозия за шторм:         {impact['max_single_storm_erosion']:+.1f} м³")
            lines.append(f"  Мин. эрозия за шторм:          {impact['min_single_storm_erosion']:+.1f} м³")
            lines.append(f"  Средняя эрозия за шторм:       {impact['mean_single_storm_erosion']:+.1f} м³")

        if 'storm_length_change_mean' in impact:
            lines.append(f"  Среднее изменение длины:       {impact['storm_length_change_mean']:+.3f} км/шаг")
            lines.append(f"  Стандартное отклонение:        {impact['storm_length_change_std']:.3f} км")

        if 'storm_contribution_pct' in impact:
            lines.append(f"  Доля штормов в общей эрозии:   {impact['storm_contribution_pct']:.1f}%")
        lines.append("")

        lines.append("=" * 70)

        return "\n".join(lines)

    def save_report(self, output_path):
        """
        Сохранить отчет в файл

        Args:
            output_path: Путь к выходному файлу
        """
        report_text = self.generate_detailed_report()

        output_file = Path(output_path)
        output_file.parent.mkdir(parents=True, exist_ok=True)

        with open(output_file, 'w', encoding='utf-8') as f:
            f.write(report_text)

    def plot_storm_impact(self, output_path='storm_impact.png', figsize=(12, 8)):
        """
        Визуализация воздействия штормов

        Args:
            output_path: Путь для сохранения графика
            figsize: Размер графика (ширина, высота)
        """
        fig, axes = plt.subplots(2, 2, figsize=figsize)
        fig.suptitle('Анализ воздействия штормов', fontsize=16, fontweight='bold')

        # Данные для графиков
        if 'year' in self.df.columns:
            x_data = self.df['year']
            x_label = 'Год'
        else:
            x_data = self.df['step']
            x_label = 'Шаг'

        storm_steps = self.df[self.df['storm_event'] == True]
        normal_steps = self.df[self.df['storm_event'] == False]

        # 1. График эрозии с выделением штормов
        ax1 = axes[0, 0]
        if 'net_change_m3' in self.df.columns:
            ax1.plot(x_data, self.df['net_change_m3'].cumsum(), marker='o',
                    linewidth=2, markersize=6, color='blue', label='Накопленная эрозия')

            if len(storm_steps) > 0:
                storm_x = storm_steps[x_label].values
                storm_y = self.df.loc[self.df['storm_event'] == True, 'net_change_m3'].cumsum()
                ax1.scatter(storm_x, storm_y, color='red', s=100, zorder=5, label='Штормы', marker='^')

            ax1.set_xlabel(x_label, fontsize=11)
            ax1.set_ylabel('Накопленная эрозия (м³)', fontsize=11)
            ax1.set_title('Накопленная эрозия', fontsize=12, fontweight='bold')
            ax1.legend()
            ax1.grid(True, alpha=0.3)

        # 2. Сравнение эрозии
        ax2 = axes[0, 1]
        if 'net_change_m3' in self.df.columns:
            categories = ['Штормовые\\nусловия', 'Обычные\\nусловия']
            values = [
                storm_steps['net_change_m3'].sum() if len(storm_steps) > 0 else 0,
                normal_steps['net_change_m3'].sum() if len(normal_steps) > 0 else 0
            ]
            colors = ['#d62728', '#2ca02c']

            ax2.bar(categories, values, color=colors, alpha=0.7, edgecolor='black')
            ax2.set_ylabel('Общая эрозия (м³)', fontsize=11)
            ax2.set_title('Сравнение эрозии', fontsize=12, fontweight='bold')
            ax2.grid(True, alpha=0.3, axis='y')

            # Добавляем значения над столбцами
            for i, v in enumerate(values):
                ax2.text(i, v + (max(values) * 0.02), f'{v:.1f}',
                        ha='center', va='bottom', fontweight='bold')

        # 3. Распределение по времени
        ax3 = axes[1, 0]
        if len(storm_steps) > 0:
            storm_years = storm_steps[x_label].values
            storm_counts = np.bincount(storm_years.astype(int), minlength=len(x_data))

            ax3.bar(x_data, storm_counts, color='#d62728', alpha=0.7, edgecolor='black')
            ax3.set_xlabel(x_label, fontsize=11)
            ax3.set_ylabel('Количество штормов', fontsize=11)
            ax3.set_title('Распределение штормов по времени', fontsize=12, fontweight='bold')
            ax3.grid(True, alpha=0.3, axis='y')

        # 4. Эффективность штормов
        ax4 = axes[1, 1]
        comparison = self.stats['comparison']

        if 'erosion_m3' in comparison:
            erosion_comp = comparison['erosion_m3']
            categories = ['За шторм', 'За обычный шаг']
            efficiencies = [
                erosion_comp['storm_per_event'],
                erosion_comp['normal_per_step']
            ]
            colors = ['#d62728', '#2ca02c']

            ax4.bar(categories, efficiencies, color=colors, alpha=0.7, edgecolor='black')
            ax4.set_ylabel('Эрозия (м³)', fontsize=11)
            ax4.set_title('Средняя эффективность', fontsize=12, fontweight='bold')
            ax4.grid(True, alpha=0.3, axis='y')

            # Добавляем значения над столбцами
            for i, v in enumerate(efficiencies):
                ax4.text(i, v + (max(efficiencies) * 0.02), f'{v:.1f}',
                        ha='center', va='bottom', fontweight='bold')

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
        description='Анализ штормовых событий в данных эрозии',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Примеры использования:
  python storm_analysis.py output/csv/erosion_metrics.csv
  python storm_analysis.py output/csv/erosion_metrics.csv --detailed --output storm_report.txt
  python storm_analysis.py output/csv/erosion_metrics.csv --plot
        '''
    )

    parser.add_argument('csv_file', help='Путь к CSV файлу с метриками эрозии')
    parser.add_argument('--detailed', '-d', action='store_true',
                       help='Показать детальный отчет')
    parser.add_argument('--output', '-o', help='Путь к выходному файлу для отчета')
    parser.add_argument('--plot', '-p', action='store_true',
                       help='Генерировать графики анализа штормов')

    args = parser.parse_args()

    try:
        # Анализ данных
        analyzer = StormAnalyzer(args.csv_file)
        report_data = analyzer.analyze_storms()

        # Вывод результатов
        if args.detailed:
            print(report_data.generate_detailed_report())
        else:
            # Краткая сводка
            basic = report_data.stats['basic_stats']
            comparison = report_data.stats['comparison']

            print("⛈️ АНАЛИЗ ШТОРМОВ")
            print("=" * 50)
            print(f"Штормовых событий:     {basic['storm_count']} из {basic['total_steps']}")
            print(f"Частота штормов:       {basic['storm_frequency']:.3f}")

            if 'erosion_m3' in comparison:
                erosion_comp = comparison['erosion_m3']
                print(f"Эрозия штормов:        {erosion_comp['storm_total']:+.1f} м³")
                print(f"Эрозия обычная:        {erosion_comp['normal_total']:+.1f} м³")
                if erosion_comp['efficiency_ratio'] > 0:
                    print(f"Коэффициент:          {erosion_comp['efficiency_ratio']:.2f}x")

        # Сохранение отчета
        if args.output:
            report_data.save_report(args.output)
            print(f"\n✓ Отчет сохранен: {args.output}")

        # Генерация графиков
        if args.plot:
            plot_path = report_data.plot_storm_impact()
            print(f"✓ График сохранен: {plot_path}")

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