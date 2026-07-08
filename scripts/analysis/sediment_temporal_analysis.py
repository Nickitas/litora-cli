#!/usr/bin/env python3
"""
Анализ временной динамики седиментации для Litora-CLI

Анализирует влияние штормов, сезонности и аккумуляции на транспорт наносов.
"""

import argparse
import sys
from pathlib import Path
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import seaborn as sns
from matplotlib.patches import Rectangle
from dataclasses import dataclass
from typing import List, Dict, Optional, Tuple
import json


@dataclass
class StormEvent:
    """Информация о штормовом событии"""
    index: int
    year: float
    intensity: float
    duration: float  # длительность в шагах
    erosion_impact: float
    deposition_volume: float
    transport_increase: float
    is_preserved: bool


@dataclass
class SeasonalStats:
    """Статистика по сезону"""
    season: str  # 'winter', 'spring', 'summer', 'autumn'
    erosion_mean: float
    erosion_std: float
    deposition_mean: float
    deposition_std: float
    transport_mean: float
    storm_count: int
    net_change: float


@dataclass
class StormDeposit:
    """Штормовое отложение"""
    storm_index: int
    thickness: float  # м
    volume: float  # м³/м
    grain_size: float  # мм
    location: int  # индекс точки берега
    is_preserved: bool


class SedimentTemporalAnalyzer:
    """Анализатор временной динамики седиментации"""

    def __init__(self, csv_path: str):
        """
        Инициализация анализатора

        Args:
            csv_path: Путь к CSV файлу с метриками эрозии
        """
        self.csv_path = Path(csv_path)
        if not self.csv_path.exists():
            raise FileNotFoundError(f"CSV файл не найден: {csv_path}")

        self.df = pd.read_csv(self.csv_path)
        self.storm_events: List[StormEvent] = []
        self.seasonal_stats: Dict[str, SeasonalStats] = {}
        self.storm_deposits: List[StormDeposit] = []
        self._validate_data()

    def _validate_data(self):
        """Проверка структуры данных"""
        required_columns = ['step', 'length_km']
        missing_columns = [col for col in required_columns if col not in self.df.columns]

        if missing_columns:
            raise ValueError(f"Отсутствуют обязательные колонки: {missing_columns}")

    def detect_storm_events(self, erosion_threshold: float = None,
                           probability_column: str = 'storm_event') -> List[StormEvent]:
        """
        Обнаруживает штормовые события в данных

        Args:
            erosion_threshold: Порог эрозии для определения шторма (м³)
            probability_column: Колонка с индикатором шторма
        """
        self.storm_events = []

        if probability_column not in self.df.columns:
            # Эвристическое определение по резким скачкам эрозии
            if 'eroded_m3' in self.df.columns:
                erosion_values = self.df['eroded_m3'].values
                if erosion_threshold is None:
                    erosion_threshold = np.percentile(erosion_values, 75) * 2

                for idx, row in self.df.iterrows():
                    if row['eroded_m3'] > erosion_threshold:
                        storm = StormEvent(
                            index=idx,
                            year=row.get('year', idx),
                            intensity=self._estimate_storm_intensity(row['eroded_m3']),
                            duration=1.0,
                            erosion_impact=row['eroded_m3'],
                            deposition_volume=row.get('deposited_m3', 0),
                            transport_increase=row['eroded_m3'] * 0.7,
                            is_preserved=row['eroded_m3'] > erosion_threshold * 1.5
                        )
                        self.storm_events.append(storm)
        else:
            # Используем индикатор шторма
            for idx, row in self.df.iterrows():
                if row[probability_column] == True or row[probability_column] == 'true':
                    intensity = 2.0  # базовая интенсивность
                    if 'eroded_m3' in self.df.columns:
                        intensity = self._estimate_storm_intensity(row['eroded_m3'])

                    storm = StormEvent(
                        index=idx,
                        year=row.get('year', idx),
                        intensity=intensity,
                        duration=1.0,
                        erosion_impact=row.get('eroded_m3', 1000),
                        deposition_volume=row.get('deposited_m3', 0),
                        transport_increase=row.get('eroded_m3', 1000) * 0.7,
                        is_preserved=intensity > 2.0
                    )
                    self.storm_events.append(storm)

        return self.storm_events

    def _estimate_storm_intensity(self, erosion_value: float) -> float:
        """Оценивает интенсивность шторма по объёму эрозии"""
        if erosion_value <= 0:
            return 1.0

        # Нормализуем к диапазону 1.0-5.0
        max_erosion = self.df['eroded_m3'].max() if 'eroded_m3' in self.df.columns else erosion_value
        if max_erosion > 0:
            normalized = erosion_value / max_erosion
            return 1.0 + normalized * 4.0
        return 1.0

    def calculate_seasonal_statistics(self, year_column: str = 'year') -> Dict[str, SeasonalStats]:
        """
        Рассчитывает статистику по сезонам

        Args:
            year_column: Колонка с годами
        """
        if year_column not in self.df.columns:
            self.df['_sim_year'] = range(len(self.df))
            year_column = '_sim_year'

        self.seasonal_stats = {}

        for season in ['winter', 'spring', 'summer', 'autumn']:
            season_mask = self._get_season_mask(season, year_column)
            season_data = self.df[season_mask]

            if len(season_data) == 0:
                continue

            erosion_mean = season_data.get('eroded_m3', pd.Series([0])).mean()
            erosion_std = season_data.get('eroded_m3', pd.Series([0])).std()
            deposition_mean = season_data.get('deposited_m3', pd.Series([0])).mean()
            deposition_std = season_data.get('deposited_m3', pd.Series([0])).std()
            transport_mean = erosion_mean * 0.7  # оценка транспорта

            storm_count = sum(1 for s in self.storm_events
                            if self._get_season_from_year(s.year) == season)

            net_change = erosion_mean - deposition_mean

            self.seasonal_stats[season] = SeasonalStats(
                season=season,
                erosion_mean=erosion_mean,
                erosion_std=erosion_std,
                deposition_mean=deposition_mean,
                deposition_std=deposition_std,
                transport_mean=transport_mean,
                storm_count=storm_count,
                net_change=net_change
            )

        return self.seasonal_stats

    def _get_season_mask(self, season: str, year_column: str) -> pd.Series:
        """Возвращает маску для сезона"""
        if year_column not in self.df.columns:
            return pd.Series([False] * len(self.df))

        years = self.df[year_column].values
        mask = pd.Series([False] * len(self.df))

        for i, year in enumerate(years):
            year_fraction = year - int(year)
            current_season = self._get_season_from_year(year)

            if current_season == season:
                mask.iloc[i] = True

        return mask

    def _get_season_from_year(self, year: float) -> str:
        """Определяет сезон по дробной части года"""
        year_fraction = year - int(year)

        if year_fraction < 0.25 or year_fraction >= 0.92:
            return 'winter'
        elif year_fraction < 0.5:
            return 'spring'
        elif year_fraction < 0.75:
            return 'summer'
        else:
            return 'autumn'

    def simulate_storm_deposits(self, num_points: int = 100) -> List[StormDeposit]:
        """
        Моделирует штормовые отложения

        Args:
            num_points: Количество точек береговой линии
        """
        self.storm_deposits = []

        for storm in self.storm_events:
            # Количество отложений пропорционально интенсивности шторма
            num_deposits = int(storm.intensity * 2)

            for i in range(num_deposits):
                location = np.random.randint(0, num_points)

                # Толщина отложения зависит от интенсивности
                thickness = 0.05 * (storm.intensity - 0.5)
                thickness = np.clip(thickness, 0.01, 0.5)

                # Размер зёрен
                grain_size = 0.5 * storm.intensity
                grain_size = np.clip(grain_size, 0.1, 20.0)

                deposit = StormDeposit(
                    storm_index=storm.index,
                    thickness=thickness,
                    volume=storm.deposition_volume / num_deposits,
                    grain_size=grain_size,
                    location=location,
                    is_preserved=storm.is_preserved and np.random.random() > 0.3
                )
                self.storm_deposits.append(deposit)

        return self.storm_deposits

    def analyze_sediment_budget(self) -> Dict[str, float]:
        """Анализирует баланс седиментов"""
        total_eroded = 0.0
        total_deposited = 0.0
        total_transport = 0.0

        if 'eroded_m3' in self.df.columns:
            total_eroded = self.df['eroded_m3'].sum()
            total_transport = total_eroded * 0.7

        if 'deposited_m3' in self.df.columns:
            total_deposited = self.df['deposited_m3'].sum()

        # Штормовый вклад
        storm_eroded = sum(s.erosion_impact for s in self.storm_events)
        storm_deposited = sum(s.deposition_volume for s in self.storm_events)
        storm_transport = sum(s.transport_increase for s in self.storm_events)

        return {
            'total_eroded_m3': total_eroded,
            'total_deposited_m3': total_deposited,
            'total_transport_m3': total_transport,
            'net_balance_m3': total_eroded - total_deposited,
            'storm_contribution_eroded': storm_eroded,
            'storm_contribution_deposited': storm_deposited,
            'storm_contribution_transport': storm_transport,
            'storm_impact_pct': (storm_eroded / total_eroded * 100) if total_eroded > 0 else 0
        }


class SedimentTemporalVisualizer:
    """Визуализатор временной динамики седиментации"""

    def __init__(self, analyzer: SedimentTemporalAnalyzer):
        self.analyzer = analyzer
        self.season_colors = {
            'winter': '#1f77b4',  # синий - холод
            'spring': '#2ca02c',  # зеленый - рост
            'summer': '#ff7f0e',  # оранжевый - тепло
            'autumn': '#d62728',  # красный - листопад
        }

    def create_seasonal_cycle_plot(self, output_path: str = 'seasonal_cycle.png'):
        """Создаёт график сезонного цикла"""
        if not self.analyzer.seasonal_stats:
            print("⚠️  Нет данных о сезонной статистике")
            return

        fig, axes = plt.subplots(2, 2, figsize=(14, 10))
        fig.suptitle('СЕЗОННАЯ ДИНАМИКА СЕДИМЕНТАЦИИ', fontsize=16, fontweight='bold')

        seasons = ['winter', 'spring', 'summer', 'autumn']
        season_names = ['Зима', 'Весна', 'Лето', 'Осень']

        # 1. Эрозия по сезонам
        ax1 = axes[0, 0]
        erosion_values = [self.analyzer.seasonal_stats[s].erosion_mean
                         for s in seasons if s in self.analyzer.seasonal_stats]
        erosion_colors = [self.season_colors[s] for s in seasons
                         if s in self.analyzer.seasonal_stats]

        ax1.bar(range(len(erosion_values)), erosion_values, color=erosion_colors, alpha=0.7)
        ax1.set_xticks(range(len(erosion_values)))
        ax1.set_xticklabels([season_names[seasons.index(s)] for s in seasons
                            if s in self.analyzer.seasonal_stats])
        ax1.set_ylabel('Эрозия (м³)', fontsize=11)
        ax1.set_title('Средняя эрозия по сезонам', fontweight='bold')
        ax1.grid(True, alpha=0.3, axis='y')

        # 2. Аккумуляция по сезонам
        ax2 = axes[0, 1]
        deposition_values = [self.analyzer.seasonal_stats[s].deposition_mean
                            for s in seasons if s in self.analyzer.seasonal_stats]

        ax2.bar(range(len(deposition_values)), deposition_values, color=erosion_colors, alpha=0.7)
        ax2.set_xticks(range(len(deposition_values)))
        ax2.set_xticklabels([season_names[seasons.index(s)] for s in seasons
                            if s in self.analyzer.seasonal_stats])
        ax2.set_ylabel('Аккумуляция (м³)', fontsize=11)
        ax2.set_title('Средняя аккумуляция по сезонам', fontweight='bold')
        ax2.grid(True, alpha=0.3, axis='y')

        # 3. Штормы по сезонам
        ax3 = axes[1, 0]
        storm_counts = [self.analyzer.seasonal_stats[s].storm_count
                        for s in seasons if s in self.analyzer.seasonal_stats]

        ax3.bar(range(len(storm_counts)), storm_counts, color=erosion_colors, alpha=0.7)
        ax3.set_xticks(range(len(storm_counts)))
        ax3.set_xticklabels([season_names[seasons.index(s)] for s in seasons
                            if s in self.analyzer.seasonal_stats])
        ax3.set_ylabel('Количество штормов', fontsize=11)
        ax3.set_title('Штормовая активность по сезонам', fontweight='bold')
        ax3.grid(True, alpha=0.3, axis='y')

        # 4. Баланс по сезонам
        ax4 = axes[1, 1]
        net_changes = [self.analyzer.seasonal_stats[s].net_change
                      for s in seasons if s in self.analyzer.seasonal_stats]
        bar_colors = ['#d62728' if x > 0 else '#2ca02c' for x in net_changes]

        ax4.bar(range(len(net_changes)), net_changes, color=bar_colors, alpha=0.7)
        ax4.axhline(y=0, color='black', linestyle='-', linewidth=0.8)
        ax4.set_xticks(range(len(net_changes)))
        ax4.set_xticklabels([season_names[seasons.index(s)] for s in seasons
                            if s in self.analyzer.seasonal_stats])
        ax4.set_ylabel('Чистое изменение (м³)', fontsize=11)
        ax4.set_title('Баланс эрозия-аккумуляция', fontweight='bold')
        ax4.grid(True, alpha=0.3, axis='y')

        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()

        return output_path

    def create_storm_impact_plot(self, output_path: str = 'storm_impact.png'):
        """Создаёт график влияния штормов"""
        if not self.analyzer.storm_events:
            print("⚠️  Нет данных о штормах")
            return

        fig, axes = plt.subplots(2, 2, figsize=(14, 10))
        fig.suptitle('АНАЛИЗ ВЛИЯНИЯ ШТОРМОВ', fontsize=16, fontweight='bold')

        # 1. Интенсивность штормов
        ax1 = axes[0, 0]
        storm_indices = [s.index for s in self.analyzer.storm_events]
        intensities = [s.intensity for s in self.analyzer.storm_events]
        colors = ['#d62728' if s.intensity > 2.5 else '#ff7f0e'
                 for s in self.analyzer.storm_events]

        ax1.bar(storm_indices, intensities, color=colors, alpha=0.7)
        ax1.axhline(y=2.0, color='red', linestyle='--', linewidth=2, label='Сильный шторм')
        ax1.set_xlabel('Номер события', fontsize=11)
        ax1.set_ylabel('Интенсивность', fontsize=11)
        ax1.set_title('Интенсивность штормов', fontweight='bold')
        ax1.legend()
        ax1.grid(True, alpha=0.3, axis='y')

        # 2. Эрозия от штормов
        ax2 = axes[0, 1]
        erosion_impacts = [s.erosion_impact for s in self.analyzer.storm_events]

        ax2.scatter(storm_indices, erosion_impacts, c=intensities,
                  cmap='YlOrRd', s=100, alpha=0.7, edgecolors='black')
        ax2.set_xlabel('Номер события', fontsize=11)
        ax2.set_ylabel('Эрозия (м³)', fontsize=11)
        ax2.set_title('Эрозионное воздействие', fontweight='bold')
        ax2.grid(True, alpha=0.3)

        # 3. Сохранённые отложения
        ax3 = axes[1, 0]
        preserved = [s for s in self.analyzer.storm_events if s.is_preserved]
        not_preserved = [s for s in self.analyzer.storm_events if not s.is_preserved]

        ax3.pie([len(preserved), len(not_preserved)],
               labels=['Сохранённые', 'Переработанные'],
               colors=['#2ca02c', '#d62728'],
               autopct='%1.1f%%', startangle=90)
        ax3.set_title('Сохранение штормовых отложений', fontweight='bold')

        # 4. Распределение по интенсивности
        ax4 = axes[1, 1]
        intensity_bins = [1.0, 2.0, 3.0, 4.0, 5.0]
        intensity_counts = []

        for i in range(len(intensity_bins) - 1):
            count = sum(1 for s in self.analyzer.storm_events
                       if intensity_bins[i] <= s.intensity < intensity_bins[i+1])
            intensity_counts.append(count)

        intensity_labels = ['1-2', '2-3', '3-4', '4-5']
        bar_colors = ['#ff7f0e', '#d62728', '#9467bd', '#000000']

        ax4.bar(range(len(intensity_counts)), intensity_counts, color=bar_colors, alpha=0.7)
        ax4.set_xticks(range(len(intensity_counts)))
        ax4.set_xticklabels(intensity_labels)
        ax4.set_xlabel('Интенсивность', fontsize=11)
        ax4.set_ylabel('Количество штормов', fontsize=11)
        ax4.set_title('Распределение по интенсивности', fontweight='bold')
        ax4.grid(True, alpha=0.3, axis='y')

        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()

        return output_path

    def create_sediment_budget_plot(self, output_path: str = 'sediment_budget.png'):
        """Создаёт график баланса седиментов"""
        budget = self.analyzer.analyze_sediment_budget()

        fig, axes = plt.subplots(1, 2, figsize=(14, 6))
        fig.suptitle('БАЛАНС СЕДИМЕНТОВ', fontsize=16, fontweight='bold')

        # 1. Основной баланс
        ax1 = axes[0]
        categories = ['Эрозия', 'Аккумуляция', 'Транспорт']
        values = [
            budget['total_eroded_m3'],
            budget['total_deposited_m3'],
            budget['total_transport_m3']
        ]
        colors = ['#d62728', '#2ca02c', '#1f77b4']

        ax1.bar(categories, values, color=colors, alpha=0.7)
        ax1.set_ylabel('Объём (м³)', fontsize=11)
        ax1.set_title('Баланс седиментов', fontweight='bold')
        ax1.grid(True, alpha=0.3, axis='y')

        # 2. Штормовый вклад
        ax2 = axes[1]
        storm_categories = ['Штормовая\nэрозия', 'Штормовая\nаккумуляция',
                          'Штормовый\nтранспорт']
        storm_values = [
            budget['storm_contribution_eroded'],
            budget['storm_contribution_deposited'],
            budget['storm_contribution_transport']
        ]

        ax2.bar(storm_categories, storm_values, color=colors, alpha=0.7)
        ax2.set_ylabel('Объём (м³)', fontsize=11)
        ax2.set_title('Штормовый вклад', fontweight='bold')
        ax2.grid(True, alpha=0.3, axis='y')

        # Добавляем процент штормового вклада
        pct_text = f"Штормовый вклад: {budget['storm_impact_pct']:.1f}% от общей эрозии"
        fig.text(0.5, 0.02, pct_text, ha='center', fontsize=12,
                bbox=dict(boxstyle='round', facecolor='wheat', alpha=0.3))

        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()

        return output_path


def generate_report(analyzer: SedimentTemporalAnalyzer) -> str:
    """Генерирует текстовый отчёт"""
    lines = []
    lines.append("=" * 80)
    lines.append("АНАЛИЗ ВРЕМЕННОЙ ДИНАМИКИ СЕДИМЕНТАЦИИ")
    lines.append("=" * 80)
    lines.append("")

    # Штормовые события
    lines.append("ШТОРМОВЫЕ СОБЫТИЯ")
    lines.append("-" * 80)
    if analyzer.storm_events:
        lines.append(f"  Всего штормов:              {len(analyzer.storm_events)}")

        if analyzer.storm_events:
            intensities = [s.intensity for s in analyzer.storm_events]
            lines.append(f"  Средняя интенсивность:     {np.mean(intensities):.2f}")
            lines.append(f"  Макс. интенсивность:       {np.max(intensities):.2f}")
            lines.append(f"  Мин. интенсивность:        {np.min(intensities):.2f}")

        preserved = sum(1 for s in analyzer.storm_events if s.is_preserved)
        lines.append(f"  Сохранённых отложений:     {preserved}/{len(analyzer.storm_events)}")
    else:
        lines.append("  Штормовые события не обнаружены")
    lines.append("")

    # Сезонная статистика
    lines.append("СЕЗОННАЯ СТАТИСТИКА")
    lines.append("-" * 80)

    season_names = {
        'winter': 'Зима',
        'spring': 'Весна',
        'summer': 'Лето',
        'autumn': 'Осень'
    }

    for season, stats in analyzer.seasonal_stats.items():
        name = season_names.get(season, season)
        lines.append(f"  {name}:")
        lines.append(f"    Эрозия:           {stats.erosion_mean:.1f} ± {stats.erosion_std:.1f} м³")
        lines.append(f"    Аккумуляция:       {stats.deposition_mean:.1f} ± {stats.deposition_std:.1f} м³")
        lines.append(f"    Транспорт:         {stats.transport_mean:.1f} м³")
        lines.append(f"    Штормов:           {stats.storm_count}")
        lines.append(f"    Баланс:            {stats.net_change:+.1f} м³")
        lines.append("")

    # Баланс седиментов
    lines.append("БАЛАНС СЕДИМЕНТОВ")
    lines.append("-" * 80)

    budget = analyzer.analyze_sediment_budget()
    lines.append(f"  Общая эрозия:             {budget['total_eroded_m3']:.1f} м³")
    lines.append(f"  Общая аккумуляция:        {budget['total_deposited_m3']:.1f} м³")
    lines.append(f"  Общий транспорт:          {budget['total_transport_m3']:.1f} м³")
    lines.append(f"  Чистый баланс:            {budget['net_balance_m3']:+.1f} м³")
    lines.append("")
    lines.append(f"  Штормовая эрозия:         {budget['storm_contribution_eroded']:.1f} м³")
    lines.append(f"  Штормовая аккумуляция:    {budget['storm_contribution_deposited']:.1f} м³")
    lines.append(f"  Штормовый транспорт:     {budget['storm_contribution_transport']:.1f} м³")
    lines.append(f"  Штормовый вклад:          {budget['storm_impact_pct']:.1f}% от общей эрозии")
    lines.append("")

    # Штормовые отложения
    if analyzer.storm_deposits:
        lines.append("ШТОРМОВЫЕ ОТЛОЖЕНИЯ")
        lines.append("-" * 80)
        lines.append(f"  Всего отложений:          {len(analyzer.storm_deposits)}")

        if analyzer.storm_deposits:
            thicknesses = [d.thickness for d in analyzer.storm_deposits]
            grain_sizes = [d.grain_size for d in analyzer.storm_deposits]

            lines.append(f"  Средняя толщина:         {np.mean(thicknesses)*100:.1f} см")
            lines.append(f"  Макс. толщина:           {np.max(thicknesses)*100:.1f} см")
            lines.append(f"  Средний размер зёрен:    {np.mean(grain_sizes):.1f} мм")

        preserved = sum(1 for d in analyzer.storm_deposits if d.is_preserved)
        lines.append(f"  Сохранится в записи:      {preserved}/{len(analyzer.storm_deposits)}")
        lines.append("")

    lines.append("=" * 80)

    return "\n".join(lines)


def main():
    """Главная функция"""
    parser = argparse.ArgumentParser(
        description='Анализ временной динамики седиментации',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Примеры использования:
  python sediment_temporal_analysis.py output/csv/erosion_metrics.csv
  python sediment_temporal_analysis.py output/csv/storm_analysis.csv --num-points 50
  python sediment_temporal_analysis.py output/csv/erosion_metrics.csv --output sediment_report
        '''
    )

    parser.add_argument('csv_file', help='Путь к CSV файлу с метриками эрозии')
    parser.add_argument('--output', '-o', help='Базовое имя для выходных файлов')
    parser.add_argument('--num-points', '-n', type=int, default=100,
                       help='Количество точек береговой линии (default: 100)')
    parser.add_argument('--erosion-threshold', '-e', type=float,
                       help='Порог эрозии для определения шторма')

    args = parser.parse_args()

    try:
        # Создаём анализатор
        analyzer = SedimentTemporalAnalyzer(args.csv_file)

        # Обнаруживаем штормовые события
        storms = analyzer.detect_storm_events(args.erosion_threshold)
        print(f"✓ Обнаружено {len(storms)} штормовых событий")

        # Рассчитываем сезонную статистику
        seasonal_stats = analyzer.calculate_seasonal_statistics()
        print(f"✓ Рассчитана статистика для {len(seasonal_stats)} сезонов")

        # Моделируем штормовые отложения
        deposits = analyzer.simulate_storm_deposits(args.num_points)
        print(f"✓ Смоделировано {len(deposits)} штормовых отложений")

        # Генерируем отчёт
        report_text = generate_report(analyzer)
        print(report_text)

        # Базовый путь для выходных файлов
        output_base = args.output or 'sediment_temporal'
        output_dir = Path('output/report')
        output_dir.mkdir(parents=True, exist_ok=True)

        # Сохраняем отчёт
        report_path = output_dir / f'{output_base}_report.txt'
        with open(report_path, 'w', encoding='utf-8') as f:
            f.write(report_text)
        print(f"✓ Отчёт сохранен: {report_path}")

        # Создаём визуализатор
        visualizer = SedimentTemporalVisualizer(analyzer)

        # Генерируем визуализации
        if seasonal_stats:
            seasonal_plot = visualizer.create_seasonal_cycle_plot(
                output_dir / f'{output_base}_seasonal.png')
            print(f"✓ Сезонный цикл: {seasonal_plot}")

        if storms:
            storm_plot = visualizer.create_storm_impact_plot(
                output_dir / f'{output_base}_storms.png')
            print(f"✓ Влияние штормов: {storm_plot}")

        budget_plot = visualizer.create_sediment_budget_plot(
            output_dir / f'{output_base}_budget.png')
        print(f"✓ Баланс седиментов: {budget_plot}")

        return 0

    except FileNotFoundError as e:
        print(f"❌ Ошибка: {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"❌ Неожиданная ошибка: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    sys.exit(main())
