#!/usr/bin/env python3
"""
Визуализация динамики береговой линии для Litora-CLI

Генерирует профессиональные графики для анализа динамики эрозии.
"""

import argparse
import sys
from pathlib import Path
import pandas as pd
import matplotlib.pyplot as plt
import seaborn as sns
import numpy as np


class DynamicsPlotter:
    """Класс для визуализации динамики береговой линии"""

    def __init__(self, csv_path, style='default'):
        """
        Инициализация плоттера динамики

        Args:
            csv_path: Путь к CSV файлу с метриками эрозии
            style: Стиль графиков ('default', 'seaborn', 'ggplot')
        """
        self.csv_path = Path(csv_path)
        if not self.csv_path.exists():
            raise FileNotFoundError(f"CSV файл не найден: {csv_path}")

        # Установка стиля
        if style == 'seaborn':
            sns.set_style("whitegrid")
            plt.style.use('seaborn-v0_8-darkgrid')
        elif style == 'ggplot':
            plt.style.use('ggplot')
        else:
            plt.style.use('default')

        self.df = pd.read_csv(self.csv_path)
        self._validate_data()

    def _validate_data(self):
        """Проверка структуры данных"""
        required_columns = ['step', 'length_km']
        missing_columns = [col for col in required_columns if col not in self.df.columns]

        if missing_columns:
            raise ValueError(f"Отсутствуют обязательные колонки: {missing_columns}")

    def plot_coastline_dynamics(self, output_path='coastline_dynamics.png', figsize=(12, 8)):
        """
        График динамики береговой линии

        Args:
            output_path: Путь для сохранения графика
            figsize: Размер графика (ширина, высота)
        """
        fig, axes = plt.subplots(2, 2, figsize=figsize)
        fig.suptitle('Динамика береговой линии', fontsize=16, fontweight='bold')

        # 1. Длина берега по времени
        ax1 = axes[0, 0]
        if 'year' in self.df.columns:
            x_data = self.df['year']
            x_label = 'Год'
        else:
            x_data = self.df['step']
            x_label = 'Шаг'

        ax1.plot(x_data, self.df['length_km'], marker='o', linewidth=2, markersize=6, color='#1f77b4')
        ax1.set_xlabel(x_label, fontsize=11)
        ax1.set_ylabel('Длина берега (км)', fontsize=11)
        ax1.set_title('Изменение длины береговой линии', fontsize=12, fontweight='bold')
        ax1.grid(True, alpha=0.3)

        # Добавляем тренд
        if len(x_data) > 2:
            try:
                z = np.polyfit(x_data, self.df['length_km'], 1)
                p = np.poly1d(z)
                ax1.plot(x_data, p(x_data), "r--", alpha=0.8, linewidth=2, label='Тренд')
                ax1.legend()
            except (np.linalg.LinAlgError, RuntimeWarning):
                # Если не удается построить тренд, пропускаем
                pass

        # 2. Площадь по времени
        ax2 = axes[0, 1]
        if 'area_km2' in self.df.columns:
            ax2.plot(x_data, self.df['area_km2'], marker='s', linewidth=2, markersize=6, color='#ff7f0e')
            ax2.set_xlabel(x_label, fontsize=11)
            ax2.set_ylabel('Площадь (км²)', fontsize=11)
            ax2.set_title('Изменение площади', fontsize=12, fontweight='bold')
            ax2.grid(True, alpha=0.3)

        # 3. Темпы эрозии
        ax3 = axes[1, 0]
        if len(self.df['length_km']) > 1:
            erosion_rates = self.df['length_km'].diff().dropna()
            if 'year' in self.df.columns:
                rate_years = self.df['year'].iloc[1:].values
            else:
                rate_years = self.df['step'].iloc[1:].values

            ax3.bar(rate_years, -erosion_rates, color='#d62728', alpha=0.7, edgecolor='black')
            ax3.set_xlabel(x_label, fontsize=11)
            ax3.set_ylabel('Темп эрозии (км/шаг)', fontsize=11)
            ax3.set_title('Темп эрозии по шагам', fontsize=12, fontweight='bold')
            ax3.grid(True, alpha=0.3, axis='y')
            ax3.axhline(y=0, color='black', linestyle='-', linewidth=0.8)

        # 4. Штормовые события
        ax4 = axes[1, 1]
        if 'storm_event' in self.df.columns:
            storm_steps = self.df[self.df['storm_event'] == True]
            normal_steps = self.df[self.df['storm_event'] == False]

            # Use the appropriate column name for x-axis
            x_column = 'year' if 'year' in self.df.columns else 'step'

            ax4.scatter(normal_steps[x_column], normal_steps['length_km'],
                      c='green', marker='o', s=80, alpha=0.6, label='Обычные условия')
            ax4.scatter(storm_steps[x_column], storm_steps['length_km'],
                      c='red', marker='^', s=120, alpha=0.8, label='Штормовые события')

            ax4.set_xlabel(x_label, fontsize=11)
            ax4.set_ylabel('Длина берега (км)', fontsize=11)
            ax4.set_title('Влияние штормов на эрозию', fontsize=12, fontweight='bold')
            ax4.legend()
            ax4.grid(True, alpha=0.3)
        else:
            # Альтернатива: уровень моря
            if 'sea_level_m' in self.df.columns:
                ax4_twin = ax4.twinx()
                ax4.plot(x_data, self.df['length_km'], marker='o', linewidth=2, markersize=6, color='#1f77b4', label='Длина')
                ax4_twin.plot(x_data, self.df['sea_level_m'], marker='s', linewidth=2, markersize=6, color='#9467bd', label='Уровень моря')

                ax4.set_xlabel(x_label, fontsize=11)
                ax4.set_ylabel('Длина берега (км)', color='#1f77b4', fontsize=11)
                ax4_twin.set_ylabel('Уровень моря (м)', color='#9467bd', fontsize=11)
                ax4.set_title('Связь с уровнем моря', fontsize=12, fontweight='bold')
                ax4.grid(True, alpha=0.3)
                ax4.tick_params(axis='y', labelcolor='#1f77b4')
                ax4_twin.tick_params(axis='y', labelcolor='#9467bd')

                # Легенды
                lines1, labels1 = ax4.get_legend_handles_labels()
                lines2, labels2 = ax4_twin.get_legend_handles_labels()
                ax4.legend(lines1 + lines2, labels1 + labels2, loc='best')

        plt.tight_layout()

        # Сохранение
        output_file = Path(output_path)
        output_file.parent.mkdir(parents=True, exist_ok=True)
        plt.savefig(output_file, dpi=300, bbox_inches='tight')
        plt.close()

        return str(output_file)

    def plot_erosion_rates(self, output_path='erosion_rates.png', figsize=(12, 6)):
        """
        График темпов эрозии

        Args:
            output_path: Путь для сохранения графика
            figsize: Размер графика (ширина, высота)
        """
        if 'eroded_m3' not in self.df.columns:
            raise ValueError("Колонка 'eroded_m3' не найдена в данных")

        fig, axes = plt.subplots(1, 2, figsize=figsize)
        fig.suptitle('Анализ темпов эрозии', fontsize=16, fontweight='bold')

        # Данные для графика
        if 'year' in self.df.columns:
            x_data = self.df['year']
            x_label = 'Год'
        else:
            x_data = self.df['step']
            x_label = 'Шаг'

        # 1. Объем эрозии по шагам
        ax1 = axes[0]
        colors = ['#d62728' if x > 0 else '#2ca02c' for x in self.df['eroded_m3']]
        ax1.bar(x_data, self.df['eroded_m3'], color=colors, alpha=0.7, edgecolor='black')
        ax1.set_xlabel(x_label, fontsize=11)
        ax1.set_ylabel('Объем эрозии (м³)', fontsize=11)
        ax1.set_title('Объем эрозии по шагам', fontsize=12, fontweight='bold')
        ax1.grid(True, alpha=0.3, axis='y')
        ax1.axhline(y=0, color='black', linestyle='-', linewidth=0.8)

        # 2. Накопленная эрозия
        ax2 = axes[1]
        if 'net_change_m3' in self.df.columns:
            cumulative = self.df['net_change_m3'].cumsum()
            ax2.plot(x_data, cumulative, marker='o', linewidth=2, markersize=6, color='#d62728')
            ax2.fill_between(x_data, cumulative, alpha=0.3, color='#d62728')
            ax2.set_xlabel(x_label, fontsize=11)
            ax2.set_ylabel('Накопленная эрозия (м³)', fontsize=11)
            ax2.set_title('Накопленная эрозия', fontsize=12, fontweight='bold')
            ax2.grid(True, alpha=0.3)

        plt.tight_layout()

        # Сохранение
        output_file = Path(output_path)
        output_file.parent.mkdir(parents=True, exist_ok=True)
        plt.savefig(output_file, dpi=300, bbox_inches='tight')
        plt.close()

        return str(output_file)

    def plot_summary_dashboard(self, output_path='dashboard.png', figsize=(14, 10)):
        """
        Генерация комплексной панели дашборда

        Args:
            output_path: Путь для сохранения графика
            figsize: Размер графика (ширина, высота)
        """
        fig = plt.figure(figsize=figsize)
        gs = fig.add_gridspec(3, 3, hspace=0.3, wspace=0.3)

        fig.suptitle('ДАШБОРД АНАЛИЗА ЭРОЗИИ', fontsize=18, fontweight='bold')

        # Данные для графиков
        if 'year' in self.df.columns:
            x_data = self.df['year']
            x_label = 'Год'
        else:
            x_data = self.df['step']
            x_label = 'Шаг'

        # 1. Основной график длины берега
        ax1 = fig.add_subplot(gs[0, :])
        ax1.plot(x_data, self.df['length_km'], marker='o', linewidth=3, markersize=8, color='#1f77b4')
        ax1.set_xlabel(x_label, fontsize=12, fontweight='bold')
        ax1.set_ylabel('Длина берега (км)', fontsize=12, fontweight='bold')
        ax1.set_title('Динамика береговой линии', fontsize=14, fontweight='bold')
        ax1.grid(True, alpha=0.3)

        # Тренд
        if len(x_data) > 2:
            try:
                z = np.polyfit(x_data, self.df['length_km'], 1)
                p = np.poly1d(z)
                ax1.plot(x_data, p(x_data), "r--", alpha=0.8, linewidth=2, label=f'Тренд: {z[0]:.2f} км/шаг')
                ax1.legend(fontsize=11)
            except (np.linalg.LinAlgError, RuntimeWarning):
                # Если не удается построить тренд, пропускаем
                pass

        # 2. Площадь
        ax2 = fig.add_subplot(gs[1, 0])
        if 'area_km2' in self.df.columns:
            ax2.plot(x_data, self.df['area_km2'], marker='s', linewidth=2, markersize=6, color='#ff7f0e')
            ax2.set_xlabel(x_label, fontsize=10)
            ax2.set_ylabel('Площадь (км²)', fontsize=10)
            ax2.set_title('Площадь', fontsize=11, fontweight='bold')
            ax2.grid(True, alpha=0.3)

        # 3. Темпы эрозии
        ax3 = fig.add_subplot(gs[1, 1])
        if len(self.df['length_km']) > 1:
            erosion_rates = self.df['length_km'].diff().dropna()
            rate_years = x_data.iloc[1:].values
            colors = ['#d62728' if x < 0 else '#2ca02c' for x in erosion_rates]
            ax3.bar(rate_years, -erosion_rates, color=colors, alpha=0.7, edgecolor='black')
            ax3.set_xlabel(x_label, fontsize=10)
            ax3.set_ylabel('Темп (км/шаг)', fontsize=10)
            ax3.set_title('Темп эрозии', fontsize=11, fontweight='bold')
            ax3.grid(True, alpha=0.3, axis='y')
            ax3.axhline(y=0, color='black', linestyle='-', linewidth=0.8)

        # 4. Штормы
        ax4 = fig.add_subplot(gs[1, 2])
        if 'storm_event' in self.df.columns:
            storm_count = self.df['storm_event'].sum()
            normal_count = len(self.df) - storm_count

            sizes = [normal_count, storm_count]
            labels = ['Обычные', 'Штормы']
            colors = ['#2ca02c', '#d62728']
            explode = (0, 0.1)

            ax4.pie(sizes, explode=explode, labels=labels, colors=colors, autopct='%1.1f%%',
                   shadow=True, startangle=90, textprops={'fontsize': 11, 'fontweight': 'bold'})
            ax4.set_title('Распределение условий', fontsize=11, fontweight='bold')

        # 5. Объем эрозии
        ax5 = fig.add_subplot(gs[2, 0])
        if 'eroded_m3' in self.df.columns:
            erosion_colors = ['#d62728' if x > 0 else '#2ca02c' for x in self.df['eroded_m3']]
            ax5.bar(x_data, self.df['eroded_m3'], color=erosion_colors, alpha=0.7, edgecolor='black')
            ax5.set_xlabel(x_label, fontsize=10)
            ax5.set_ylabel('Эрозия (м³)', fontsize=10)
            ax5.set_title('Объем эрозии', fontsize=11, fontweight='bold')
            ax5.grid(True, alpha=0.3, axis='y')
            ax5.axhline(y=0, color='black', linestyle='-', linewidth=0.8)

        # 6. Накопленная эрозия
        ax6 = fig.add_subplot(gs[2, 1])
        if 'net_change_m3' in self.df.columns:
            cumulative = self.df['net_change_m3'].cumsum()
            ax6.plot(x_data, cumulative, marker='o', linewidth=2, markersize=6, color='#9467bd')
            ax6.fill_between(x_data, cumulative, alpha=0.3, color='#9467bd')
            ax6.set_xlabel(x_label, fontsize=10)
            ax6.set_ylabel('Накоплено (м³)', fontsize=10)
            ax6.set_title('Накопленная эрозия', fontsize=11, fontweight='bold')
            ax6.grid(True, alpha=0.3)

        # 7. Статистика
        ax7 = fig.add_subplot(gs[2, 2])
        ax7.axis('off')

        # Расчет статистики
        stats_text = "СТАТИСТИКА\\n"
        stats_text += f"Шагов: {len(self.df)}\\n"
        stats_text += f"Начальная длина: {self.df['length_km'].iloc[0]:.1f} км\\n"
        stats_text += f"Конечная длина: {self.df['length_km'].iloc[-1]:.1f} км\\n"

        if len(self.df['length_km']) > 1:
            length_change = self.df['length_km'].iloc[-1] - self.df['length_km'].iloc[0]
            stats_text += f"Изменение: {length_change:+.1f} км\\n"

        if 'eroded_m3' in self.df.columns:
            total_erosion = self.df['eroded_m3'].sum()
            stats_text += f"Общая эрозия: {total_erosion:.1f} м³\\n"

        if 'storm_event' in self.df.columns:
            storm_count = self.df['storm_event'].sum()
            stats_text += f"Штормов: {int(storm_count)}\\n"

        ax7.text(0.1, 0.5, stats_text, transform=ax7.transAxes, fontsize=11,
                verticalalignment='center', fontfamily='monospace',
                bbox=dict(boxstyle='round', facecolor='wheat', alpha=0.3))

        # Сохранение
        output_file = Path(output_path)
        output_file.parent.mkdir(parents=True, exist_ok=True)
        plt.savefig(output_file, dpi=300, bbox_inches='tight')
        plt.close()

        return str(output_file)


def main():
    """Главная функция для запуска из командной строки"""
    parser = argparse.ArgumentParser(
        description='Визуализация динамики береговой линии',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Примеры использования:
  python plot_dynamics.py output/csv/erosion_metrics.csv
  python plot_dynamics.py output/csv/erosion_metrics.csv --style seaborn
  python plot_dynamics.py output/csv/erosion_metrics.csv --output my_report
  python plot_dynamics.py output/csv/erosion_metrics.csv --plots my_analysis --dashboard
  python plot_dynamics.py output/csv/erosion_metrics.csv --dashboard --figsize 14 10
        '''
    )

    parser.add_argument('csv_file', help='Путь к CSV файлу с метриками эрозии')
    parser.add_argument('--style', choices=['default', 'seaborn', 'ggplot'], default='default',
                       help='Стиль графиков')
    parser.add_argument('--dashboard', '-d', action='store_true',
                       help='Генерировать комплексную панель дашборда')
    parser.add_argument('--output', '-o', help='Базовое имя для сохранения графиков в output/report/')
    parser.add_argument('--plots', '-p', help='Альтернатива --output: базовое имя для графиков в output/report/')
    parser.add_argument('--figsize', nargs=2, type=int, default=[12, 8],
                       help='Размер графиков (ширина высота)')

    args = parser.parse_args()

    try:
        # Создание плоттера
        plotter = DynamicsPlotter(args.csv_file, style=args.style)

        # Базовый путь для выходных файлов
        # Поддержка как --output, так и --plots для гибкости
        input_name = args.output or args.plots or 'dynamics_plots'

        # Извлекаем только basename из пути, если пользователь указал директории
        basename = Path(input_name).name

        # Всегда сохраняем в output/report/
        output_dir = Path('output/report')
        output_dir.mkdir(parents=True, exist_ok=True)

        base_path = output_dir / basename

        # Генерация графиков
        if args.dashboard:
            output_file = plotter.plot_summary_dashboard(
                output_path=base_path,
                figsize=tuple(args.figsize)
            )
            print(f"✓ Дашборд сохранен: {output_file}")
        else:
            # Обычные графики
            output1 = plotter.plot_coastline_dynamics(
                output_path=f"{base_path}_coastline.png",
                figsize=tuple(args.figsize)
            )
            print(f"✓ График динамики: {output1}")

            # График темпов эрозии (если есть данные)
            try:
                output2 = plotter.plot_erosion_rates(
                    output_path=f"{base_path}_rates.png",
                    figsize=tuple(args.figsize)
                )
                print(f"✓ График темпов: {output2}")
            except ValueError:
                print("⚠ График темпов эрозии недоступен (нет данных об объеме эрозии)")

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