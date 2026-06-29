#!/usr/bin/env python3
"""
Базовый анализ эрозии для Litora-CLI

Анализирует CSV файлы с метриками эрозии и генерирует статистический отчет.
"""

import argparse
import sys
from pathlib import Path
import pandas as pd
import numpy as np


class ErosionAnalyzer:
    """Класс для анализа метрик эрозии"""

    def __init__(self, csv_path):
        """
        Инициализация анализатора эрозии

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
        required_columns = ['step', 'length_km', 'area_km2']
        missing_columns = [col for col in required_columns if col not in self.df.columns]

        if missing_columns:
            raise ValueError(f"Отсутствуют обязательные колонки: {missing_columns}")

    def analyze(self):
        """Провести полный анализ данных"""
        stats = {
            'basic_stats': self._basic_statistics(),
            'temporal_stats': self._temporal_analysis(),
            'erosion_stats': self._erosion_analysis(),
            'storm_stats': self._storm_analysis() if 'storm_event' in self.df.columns else None,
        }

        return ErosionReport(self.df, stats)

    def _basic_statistics(self):
        """Базовая статистика"""
        return {
            'total_steps': len(self.df),
            'initial_length_km': self.df['length_km'].iloc[0],
            'final_length_km': self.df['length_km'].iloc[-1],
            'initial_area_km2': self.df['area_km2'].iloc[0],
            'final_area_km2': self.df['area_km2'].iloc[-1],
            'total_points': self.df['step'].max() if len(self.df) > 0 else 0,
        }

    def _temporal_analysis(self):
        """Временной анализ"""
        if 'year' not in self.df.columns:
            return None

        years = self.df['year'].diff().sum()
        length_change = self.df['length_km'].iloc[-1] - self.df['length_km'].iloc[0]
        length_change_pct = (length_change / self.df['length_km'].iloc[0]) * 100

        return {
            'simulated_years': years,
            'length_change_km': length_change,
            'length_change_percent': length_change_pct,
            'mean_annual_change_km': length_change / years if years > 0 else 0,
        }

    def _erosion_analysis(self):
        """Анализ эрозии"""
        if 'eroded_m3' not in self.df.columns or 'net_change_m3' not in self.df.columns:
            return None

        total_eroded = self.df['eroded_m3'].sum()
        total_deposited = self.df['deposited_m3'].sum() if 'deposited_m3' in self.df.columns else 0
        net_change = self.df['net_change_m3'].sum()

        # Расчет средних показателей
        erosion_rates = self.df['eroded_m3'].diff().dropna()

        return {
            'total_eroded_m3': total_eroded,
            'total_deposited_m3': total_deposited,
            'net_change_m3': net_change,
            'mean_annual_erosion_m3': total_eroded / len(self.df) if len(self.df) > 0 else 0,
            'max_erosion_step_m3': self.df['eroded_m3'].max() if len(self.df) > 0 else 0,
            'min_erosion_step_m3': self.df['eroded_m3'].min() if len(self.df) > 0 else 0,
        }

    def _storm_analysis(self):
        """Анализ штормов"""
        if 'storm_event' not in self.df.columns:
            return None

        storm_count = self.df['storm_event'].sum()
        total_steps = len(self.df)
        storm_frequency = storm_count / total_steps if total_steps > 0 else 0

        # Эрозия во время штормов vs обычная эрозия
        if 'net_change_m3' in self.df.columns:
            storm_erosion = self.df[self.df['storm_event'] == True]['net_change_m3'].sum()
            normal_erosion = self.df[self.df['storm_event'] == False]['net_change_m3'].sum()
        else:
            storm_erosion = None
            normal_erosion = None

        return {
            'storm_events': int(storm_count),
            'storm_frequency': storm_frequency,
            'storm_erosion_m3': storm_erosion,
            'normal_erosion_m3': normal_erosion,
        }


class ErosionReport:
    """Класс для генерации отчета об анализе"""

    def __init__(self, df, stats):
        """
        Инициализация генератора отчетов

        Args:
            df: DataFrame с данными
            stats: Словарь со статистикой
        """
        self.df = df
        self.stats = stats

    def generate_text_report(self):
        """Генерация текстового отчета"""
        lines = []
        lines.append("=" * 70)
        lines.append("АНАЛИЗ ЭРОЗИИ БЕРЕГОВОЙ ЛИНИИ")
        lines.append("=" * 70)
        lines.append("")

        # Базовая статистика
        lines.append("ОБЩАЯ СТАТИСТИКА")
        lines.append("-" * 70)
        basic = self.stats['basic_stats']
        lines.append(f"  Количество шагов:         {basic['total_steps']}")
        lines.append(f"  Начальная длина:           {basic['initial_length_km']:.2f} км")
        lines.append(f"  Конечная длина:            {basic['final_length_km']:.2f} км")
        lines.append(f"  Начальная площадь:         {basic['initial_area_km2']:.2f} км²")
        lines.append(f"  Конечная площадь:          {basic['final_area_km2']:.2f} км²")
        lines.append("")

        # Временной анализ
        if self.stats['temporal_stats']:
            lines.append("ВРЕМЕННАЯ ДИНАМИКА")
            lines.append("-" * 70)
            temporal = self.stats['temporal_stats']
            lines.append(f"  Промоделировано лет:      {temporal['simulated_years']:.1f} лет")
            lines.append(f"  Изменение длины:          {temporal['length_change_km']:+.2f} км")
            lines.append(f"  Изменение длины (%):      {temporal['length_change_percent']:+.2f}%")
            lines.append(f"  Среднегодовое изменение:   {temporal['mean_annual_change_km']:+.3f} км/год")
            lines.append("")

        # Анализ эрозии
        if self.stats['erosion_stats']:
            lines.append("АНАЛИЗ ЭРОЗИИ")
            lines.append("-" * 70)
            erosion = self.stats['erosion_stats']
            lines.append(f"  Общая эрозия:             {erosion['total_eroded_m3']:+.1f} м³")
            lines.append(f"  Общая депозиция:          {erosion['total_deposited_m3']:+.1f} м³")
            lines.append(f"  Баланс (net change):      {erosion['net_change_m3']:+.1f} м³")
            lines.append(f"  Средняя эрозия/шаг:       {erosion['mean_annual_erosion_m3']:+.1f} м³/шаг")
            lines.append(f"  Макс. эрозия за шаг:      {erosion['max_erosion_step_m3']:+.1f} м³")
            lines.append(f"  Мин. эрозия за шаг:       {erosion['min_erosion_step_m3']:+.1f} м³")
            lines.append("")

        # Анализ штормов
        if self.stats['storm_stats']:
            lines.append("АНАЛИЗ ШТОРМОВ")
            lines.append("-" * 70)
            storm = self.stats['storm_stats']
            lines.append(f"  Штормовых событий:        {storm['storm_events']}")
            lines.append(f"  Частота штормов:          {storm['storm_frequency']:.3f}")
            if storm['storm_erosion_m3'] is not None:
                lines.append(f"  Эрозия во время штормов: {storm['storm_erosion_m3']:+.1f} м³")
                lines.append(f"  Эрозия в обычное время:  {storm['normal_erosion_m3']:+.1f} м³")
                efficiency = (storm['storm_erosion_m3'] / storm['storm_events']) if storm['storm_events'] > 0 else 0
                normal_efficiency = (storm['normal_erosion_m3'] / (len(self.df) - storm['storm_events'])) if (len(self.df) - storm['storm_events']) > 0 else 0
                lines.append(f"  Эффективность шторма:     {efficiency:+.1f} м³/шторм")
                lines.append(f"  Эффективность норма:      {normal_efficiency:+.1f} м³/шаг")
            lines.append("")

        lines.append("=" * 70)

        return "\n".join(lines)

    def save_report(self, output_path):
        """
        Сохранить отчет в файл

        Args:
            output_path: Путь к выходному файлу
        """
        report_text = self.generate_text_report()

        output_file = Path(output_path)
        output_file.parent.mkdir(parents=True, exist_ok=True)

        with open(output_file, 'w', encoding='utf-8') as f:
            f.write(report_text)

    def summary(self):
        """Краткая сводка результатов"""
        summary_lines = []

        basic = self.stats['basic_stats']
        summary_lines.append(f"📊 Анализ эрозии: {basic['total_steps']} шагов")

        if self.stats['temporal_stats']:
            temporal = self.stats['temporal_stats']
            summary_lines.append(f"📈 Период: {temporal['simulated_years']:.1f} лет")
            summary_lines.append(f"📏 Изменение длины: {temporal['length_change_km']:+.2f} км ({temporal['length_change_percent']:+.1f}%)")

        if self.stats['erosion_stats']:
            erosion = self.stats['erosion_stats']
            summary_lines.append(f"🌊 Общая эрозия: {erosion['total_eroded_m3']:+.1f} м³")

        if self.stats['storm_stats']:
            storm = self.stats['storm_stats']
            summary_lines.append(f"⛈️ Штормы: {storm['storm_events']} событий (частота: {storm['storm_frequency']:.2f})")

        return "\n".join(summary_lines)


def main():
    """Главная функция для запуска из командной строки"""
    parser = argparse.ArgumentParser(
        description='Анализ CSV файлов с метриками эрозии',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Примеры использования:
  python analyze_erosion.py output/csv/erosion_metrics.csv
  python analyze_erosion.py output/csv/erosion_metrics.csv --output analysis_report.txt
  python analyze_erosion.py output/csv/erosion_metrics.csv --summary
        '''
    )

    parser.add_argument('csv_file', help='Путь к CSV файлу с метриками эрозии')
    parser.add_argument('--output', '-o', help='Путь к выходному файлу для отчета')
    parser.add_argument('--summary', '-s', action='store_true', help='Показать только краткую сводку')

    args = parser.parse_args()

    try:
        # Анализ данных
        analyzer = ErosionAnalyzer(args.csv_file)
        report_data = analyzer.analyze()

        # Вывод результатов
        if args.summary:
            print(report_data.summary())
        else:
            print(report_data.generate_text_report())

        # Сохранение отчета
        if args.output:
            report_data.save_report(args.output)
            print(f"\n✓ Отчет сохранен: {args.output}")

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