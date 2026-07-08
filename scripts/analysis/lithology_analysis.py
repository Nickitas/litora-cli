#!/usr/bin/env python3
"""
Анализ литологической вариабельности и выветривания для Litora-CLI

Анализирует пространственную вариабельность литологии, выветривание пород
и взаимодействие с процессами эрозии/аккумуляции.
"""

import argparse
import sys
from pathlib import Path
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import seaborn as sns
from matplotlib.colors import LinearSegmentedColormap
from dataclasses import dataclass
from typing import List, Dict, Optional, Tuple
import json


@dataclass
class LithologyPoint:
    """Точка литологической съемки"""
    index: int
    lat: float
    lon: float
    lithology_class: str
    resistance: float
    porosity: float
    fracture_density: float
    weathering_progress: float
    age_years: float
    is_weathered: bool


@dataclass
class LithologyStats:
    """Статистика по литологическому классу"""
    class_name: str
    count: int
    mean_resistance: float
    std_resistance: float
    mean_porosity: float
    mean_fracture_density: float
    weathered_fraction: float
    color: str


class LithologyAnalyzer:
    """Анализатор литологической вариабельности"""

    def __init__(self, num_points: int = 100):
        """
        Инициализация анализатора

        Args:
            num_points: Количество точек береговой линии
        """
        self.num_points = num_points
        self.points: List[LithologyPoint] = []
        self.stats_by_class: Dict[str, LithologyStats] = {}

        # Литологические классы с характеристиками
        self.lithology_classes = {
            'limestone': {
                'resistance': 2.5,
                'color': '#8b8b8b',
                'weathering_rate': 0.002,
                'description': 'Известняк'
            },
            'granite': {
                'resistance': 8.0,
                'color': '#ff6b6b',
                'weathering_rate': 0.0005,
                'description': 'Гранит'
            },
            'sandstone': {
                'resistance': 1.5,
                'color': '#ffd93d',
                'weathering_rate': 0.003,
                'description': 'Песчаник'
            },
            'shale': {
                'resistance': 0.8,
                'color': '#6bcb77',
                'weathering_rate': 0.004,
                'description': 'Глинистый сланец'
            },
            'basalt': {
                'resistance': 6.0,
                'color': '#4d96ff',
                'weathering_rate': 0.0003,
                'description': 'Базальт'
            },
            'conglomerate': {
                'resistance': 2.0,
                'color': '#9b59b6',
                'weathering_rate': 0.002,
                'description': 'Конгломерат'
            },
            'alluvium': {
                'resistance': 0.3,
                'color': '#f39c12',
                'weathering_rate': 0.01,
                'description': 'Аллювий'
            }
        }

    def generate_spatial_lithology(self,
                                  autocorrelation: float = 0.7,
                                  noise_level: float = 0.2,
                                  seed: Optional[int] = None) -> List[LithologyPoint]:
        """
        Генерирует пространственную вариабельность литологии

        Args:
            autocorrelation: Пространственная автокорреляция [0-1]
            noise_level: Уровень шума [0-1]
            seed: Сид для воспроизводимости
        """
        if seed is not None:
            np.random.seed(seed)

        self.points = []

        # Генерируем пространственную структуру
        positions = np.linspace(0, 1, self.num_points)

        # Детерминированная компонента (пространственная волна)
        spatial_component = (0.5 * np.sin(2 * np.pi * positions * 3) +
                           0.3 * np.sin(2 * np.pi * positions * 7))

        # Случайная компонента с автокорреляцией
        random_component = np.zeros(self.num_points)
        current_value = np.random.randn() * noise_level

        for i in range(self.num_points):
            random_component[i] = current_value
            # AR(1) процесс для автокорреляции
            current_value = (autocorrelation * current_value +
                          np.sqrt(1 - autocorrelation**2) * np.random.randn() * noise_level)

        # Комбинируем компоненты
        combined_signal = (spatial_component * (1 - autocorrelation) +
                          random_component * autocorrelation)

        # Нормируем к [0, 1]
        combined_signal = (combined_signal - combined_signal.min()) / (combined_signal.max() - combined_signal.min())

        # Распределяем литологические классы
        for i in range(self.num_points):
            # Выбираем класс на основе пространственной позиции
            class_prob = combined_signal[i]

            # Разбиваем на интервалы для разных классов
            if class_prob < 0.15:
                litho_class = 'alluvium'
            elif class_prob < 0.3:
                litho_class = 'shale'
            elif class_prob < 0.45:
                litho_class = 'sandstone'
            elif class_prob < 0.6:
                litho_class = 'limestone'
            elif class_prob < 0.75:
                litho_class = 'conglomerate'
            elif class_prob < 0.9:
                litho_class = 'basalt'
            else:
                litho_class = 'granite'

            class_data = self.lithology_classes[litho_class]

            # Добавляем вариабельность сопротивления
            base_resistance = class_data['resistance']
            resistance_variation = (combined_signal[i] - 0.5) * base_resistance * 0.5
            resistance = max(0.1, base_resistance + resistance_variation)

            # Пористость обратно пропорциональна сопротивлению
            porosity = 0.35 - (resistance / 10.0) * 0.3
            porosity = max(0.05, min(0.5, porosity))

            # Трещиноватость
            fracture_density = 0.1 + (1.0 - resistance / 8.0) * 0.4

            point = LithologyPoint(
                index=i,
                lat=43.0 + positions[i] * 3.0,  // Примерные координаты Черного моря
                lon=27.0 + positions[i] * 15.0,
                lithology_class=litho_class,
                resistance=resistance,
                porosity=porosity,
                fracture_density=fracture_density,
                weathering_progress=0.0,
                age_years=0.0,
                is_weathered=False
            )

            self.points.append(point)

        return self.points

    def apply_weathering(self, years: float, climate_factor: float = 1.0) -> None:
        """
        Применяет выветривание к породам

        Args:
            years: Время выветривания (лет)
            climate_factor: Климатический множитель
        """
        for point in self.points:
            if years <= 0:
                continue

            # Базовая скорость выветривания для класса
            weathering_rate = self.lithology_classes[point.lithology_class]['weathering_rate']

            # Климатический множитель
            total_rate = weathering_rate * climate_factor

            # Прогресс выветривания: 1 - exp(-rate * time)
            stabilization_time = 1000.0
            point.weathering_progress = 1.0 - np.exp(-total_rate * years / stabilization_time)

            # Снижение сопротивления
            resistance_loss = point.resistance * point.weathering_progress * 0.7
            point.resistance = max(0.1, point.resistance - resistance_loss)

            # Увеличение пористости и трещиноватости
            point.porosity = min(0.5, point.porosity + point.weathering_progress * 0.25)
            point.fracture_density = min(1.0, point.fracture_density + point.weathering_progress * 0.4)

            point.age_years = years
            point.is_weathered = point.weathering_progress > 0.3

    def apply_storm_impact(self, storm_indices: List[int],
                          storm_intensity: float = 2.5) -> None:
        """
        Применяет воздействие штормов на литологию

        Args:
            storm_indices: Индексы точек, подверженных штормам
            storm_intensity: Интенсивность шторма
        """
        for idx in storm_indices:
            if idx >= len(self.points):
                continue

            point = self.points[idx]

            # Шторм создаёт новые трещины
            fracture_increase = 2.0 * storm_intensity * 0.1
            point.fracture_density = min(1.0, point.fracture_density + fracture_increase)

            # Увеличение пористости
            porosity_increase = storm_intensity * 0.05
            point.porosity = min(0.5, point.porosity + porosity_increase)

            # Снижение сопротивления
            resistance_decrease = point.resistance * storm_intensity * 0.1
            point.resistance = max(0.1, point.resistance - resistance_decrease)

    def calculate_statistics(self) -> Dict[str, LithologyStats]:
        """Рассчитывает статистику по литологическим классам"""
        self.stats_by_class = {}

        for class_name, class_data in self.lithology_classes.items():
            class_points = [p for p in self.points if p.lithology_class == class_name]

            if not class_points:
                continue

            resistances = [p.resistance for p in class_points]
            porosities = [p.porosity for p in class_points]
            fractures = [p.fracture_density for p in class_points]
            weathered = [p for p in class_points if p.is_weathered]

            stats = LithologyStats(
                class_name=class_name,
                count=len(class_points),
                mean_resistance=np.mean(resistances),
                std_resistance=np.std(resistances),
                mean_porosity=np.mean(porosities),
                mean_fracture_density=np.mean(fractures),
                weathered_fraction=len(weathered) / len(class_points),
                color=class_data['color']
            )

            self.stats_by_class[class_name] = stats

        return self.stats_by_class

    def analyze_erosion_interaction(self, base_erosion: np.ndarray,
                                   storm_mask: Optional[np.ndarray] = None) -> Dict[str, float]:
        """
        Анализирует взаимодействие литологии с эрозией

        Args:
            base_erosion: Базовая эрозия по точкам
            storm_mask: Маска штормовых событий
        """
        if len(base_erosion) != len(self.points):
            raise ValueError("Длина массива эрозии не совпадает с количеством точек")

        total_modified_erosion = 0.0
        total_base_erosion = base_erosion.sum()

        lithology_factor_sum = 0.0
        weathering_boost_sum = 0.0

        for i, point in enumerate(self.points):
            base = base_erosion[i]

            # Сопротивление снижает эрозию
            resistance_factor = 1.0 / (point.resistance * 0.5)
            resistance_factor = min(1.0, resistance_factor)

            # Выветривание увеличивает эрозию
            weathering_boost = 1.0
            if point.is_weathered:
                weathering_boost = 1.0 + 0.3 * point.weathering_progress

            modified = base * resistance_factor * weathering_boost

            if storm_mask is not None and i < len(storm_mask) and storm_mask[i]:
                # Штормовая эрозия с учётом трещиноватости
                storm_boost = 3.0 * (1.0 + point.fracture_density)
                modified *= storm_boost

            total_modified_erosion += modified
            lithology_factor_sum += resistance_factor
            weathering_boost_sum += weathering_boost

        n = len(self.points)

        return {
            'base_erosion_total': total_base_erosion,
            'modified_erosion_total': total_modified_erosion,
            'lithology_effect': total_modified_erosion / total_base_erosion if total_base_erosion > 0 else 1.0,
            'mean_resistance_factor': lithology_factor_sum / n,
            'mean_weathering_boost': weathering_boost_sum / n
        }


class LithologyVisualizer:
    """Визуализатор литологической вариабельности"""

    def __init__(self, analyzer: LithologyAnalyzer):
        self.analyzer = analyzer

    def create_lithology_map(self, output_path: str = 'lithology_map.png'):
        """Создаёт карту литологии"""
        fig, axes = plt.subplots(2, 2, figsize=(16, 12))
        fig.suptitle('ЛИТОЛОГИЧЕСКАЯ КАРТА', fontsize=16, fontweight='bold')

        points = self.analyzer.points
        positions = list(range(len(points)))

        # 1. Литологические классы
        ax1 = axes[0, 0]
        class_colors = [self.analyzer.lithology_classes[p.lithology_class]['color']
                       for p in points]

        for i in range(len(points) - 1):
            ax1.scatter([positions[i]], [0], c=[class_colors[i]], s=100,
                       edgecolors='black', alpha=0.8)

        ax1.set_xlabel('Позиция вдоль берега', fontsize=11)
        ax1.set_yticks([])
        ax1.set_title('Распределение литологических классов', fontweight='bold')

        # Легенда
        legend_elements = []
        for class_name, class_data in self.analyzer.lithology_classes.items():
            if any(p.lithology_class == class_name for p in points):
                legend_elements.append(plt.Rectangle((0, 0), 1, 1,
                                                     facecolor=class_data['color'],
                                                     label=class_data['description'],
                                                     edgecolor='black'))
        ax1.legend(handles=legend_elements, loc='upper right', fontsize=9)

        # 2. Сопротивление эрозии
        ax2 = axes[0, 1]
        resistances = [p.resistance for p in points]

        ax2.plot(positions, resistances, 'o-', linewidth=2, markersize=6, color='#1f77b4')
        ax2.fill_between(positions, resistances, alpha=0.3, color='#1f77b4')
        ax2.set_xlabel('Позиция вдоль берега', fontsize=11)
        ax2.set_ylabel('Сопротивление', fontsize=11)
        ax2.set_title('Сопротивление эрозии', fontweight='bold')
        ax2.grid(True, alpha=0.3)

        # 3. Пористость
        ax3 = axes[1, 0]
        porosities = [p.porosity * 100 for p in points]  # в процентах

        ax3.plot(positions, porosities, 'o-', linewidth=2, markersize=6, color='#ff7f0e')
        ax3.fill_between(positions, porosities, alpha=0.3, color='#ff7f0e')
        ax3.set_xlabel('Позиция вдоль берега', fontsize=11)
        ax3.set_ylabel('Пористость (%)', fontsize=11)
        ax3.set_title('Пористость пород', fontweight='bold')
        ax3.grid(True, alpha=0.3)

        # 4. Трещиноватость
        ax4 = axes[1, 1]
        fractures = [p.fracture_density * 100 for p in points]  # в процентах

        ax4.plot(positions, fractures, 'o-', linewidth=2, markersize=6, color='#2ca02c')
        ax4.fill_between(positions, fractures, alpha=0.3, color='#2ca02c')
        ax4.set_xlabel('Позиция вдоль берега', fontsize=11)
        ax4.set_ylabel('Трещиноватость (%)', fontsize=11)
        ax4.set_title('Трещиноватость', fontweight='bold')
        ax4.grid(True, alpha=0.3)

        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()

        return output_path

    def create_weathering_plot(self, output_path: str = 'weathering_effects.png'):
        """Создаёт график влияния выветривания"""
        if not self.analyzer.points:
            print("⚠️  Нет данных для визуализации")
            return

        fig, axes = plt.subplots(2, 2, figsize=(16, 12))
        fig.suptitle('ВЛИЯНИЕ ВЫВЕТРИВАНИЯ', fontsize=16, fontweight='bold')

        points = self.analyzer.points
        positions = list(range(len(points)))

        # 1. Прогресс выветривания
        ax1 = axes[0, 0]
        weathering_progress = [p.weathering_progress * 100 for p in points]

        weathered_colors = ['#d62728' if p.is_weathered else '#2ca02c' for p in points]
        ax1.bar(positions, weathering_progress, color=weathered_colors, alpha=0.7, edgecolor='black')
        ax1.axhline(y=30, color='blue', linestyle='--', linewidth=2, label='Порог выветривания')
        ax1.set_xlabel('Позиция вдоль берега', fontsize=11)
        ax1.set_ylabel('Прогресс выветривания (%)', fontsize=11)
        ax1.set_title('Прогресс выветривания', fontweight='bold')
        ax1.legend()
        ax1.grid(True, alpha=0.3, axis='y')

        # 2. Потеря сопротивления от выветривания
        ax2 = axes[0, 1]
        resistance_loss = []
        for point in points:
            initial_resistance = self.analyzer.lithology_classes[point.lithology_class]['resistance']
            loss_pct = (initial_resistance - point.resistance) / initial_resistance * 100
            resistance_loss.append(loss_pct)

        ax2.bar(positions, resistance_loss, color='#9467bd', alpha=0.7, edgecolor='black')
        ax2.set_xlabel('Позиция вдоль берега', fontsize=11)
        ax2.set_ylabel('Потеря сопротивления (%)', fontsize=11)
        ax2.set_title('Потеря сопротивления от выветривания', fontweight='bold')
        ax2.grid(True, alpha=0.3, axis='y')

        # 3. Распределение по классам
        ax3 = axes[1, 0]
        class_counts = {}
        for point in points:
            class_name = self.analyzer.lithology_classes[point.lithology_class]['description']
            class_counts[class_name] = class_counts.get(class_name, 0) + 1

        class_colors = [self.analyzer.lithology_classes[name]['color']
                       for name in self.analyzer.lithology_classes.keys()
                       if class_counts.get(self.analyzer.lithology_classes[name]['description'], 0) > 0]

        ax3.pie(class_counts.values(), labels=class_counts.keys(),
               colors=class_colors, autopct='%1.1f%%', startangle=90)
        ax3.set_title('Распределение литологических классов', fontweight='bold')

        # 4. Статистика по классам
        ax4 = axes[1, 1]
        ax4.axis('off')

        stats_text = "СТАТИСТИКА ПО КЛАССАМ\n\n"

        for class_name, class_data in self.analyzer.lithology_classes.items():
            class_points = [p for p in points if p.lithology_class == class_name]
            if not class_points:
                continue

            count = len(class_points)
            mean_resistance = np.mean([p.resistance for p in class_points])
            weathered_count = sum(1 for p in class_points if p.is_weathered)

            stats_text += f"{class_data['description']}:\n"
            stats_text += f"  Точек: {count}\n"
            stats_text += f"  Ср. сопротивление: {mean_resistance:.2f}\n"
            stats_text += f"  Выветрелых: {weathered_count}/{count}\n\n"

        ax4.text(0.1, 0.5, stats_text, transform=ax4.transAxes, fontsize=10,
                verticalalignment='center', fontfamily='monospace',
                bbox=dict(boxstyle='round', facecolor='wheat', alpha=0.3))

        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()

        return output_path

    def create_interaction_plot(self, output_path: str = 'lithology_interaction.png'):
        """Создаёт график взаимодействия с эрозией"""
        if not self.analyzer.points:
            print("⚠️  Нет данных для визуализации")
            return

        fig, axes = plt.subplots(2, 2, figsize=(16, 12))
        fig.suptitle('ВЗАИМОДЕЙСТВИЕ ЛИТОЛОГИИ С ЭРОЗИЕЙ', fontsize=16, fontweight='bold')

        points = self.analyzer.points
        positions = list(range(len(points)))

        # Базовая эрозия для демонстрации
        base_erosion = np.full(len(points), 100.0)

        # 1. Эффект сопротивления
        ax1 = axes[0, 0]
        resistance_factors = []
        for point in points:
            factor = 1.0 / (point.resistance * 0.5)
            resistance_factors.append(min(1.0, factor))

        ax1.plot(positions, resistance_factors, 'o-', linewidth=2, markersize=6, color='#1f77b4')
        ax1.fill_between(positions, resistance_factors, alpha=0.3, color='#1f77b4')
        ax1.set_xlabel('Позиция вдоль берега', fontsize=11)
        ax1.set_ylabel('Коэффициент снижения эрозии', fontsize=11)
        ax1.set_title('Влияние сопротивления на эрозию', fontweight='bold')
        ax1.grid(True, alpha=0.3)

        # 2. Эрозия с учётом литологии
        ax2 = axes[0, 1]
        modified_erosion = []
        for i, point in enumerate(points):
            base = base_erosion[i]
            factor = 1.0 / (point.resistance * 0.5)
            factor = min(1.0, factor)
            weathering_boost = 1.0 + (0.3 * point.weathering_progress if point.is_weathered else 0)
            modified = base * factor * weathering_boost
            modified_erosion.append(modified)

        ax2.plot(positions, base_erosion, '--', linewidth=2, color='#999999', label='Базовая эрозия')
        ax2.plot(positions, modified_erosion, 'o-', linewidth=2, markersize=6, color='#d62728', label='Модифицированная')
        ax2.fill_between(positions, modified_erosion, alpha=0.3, color='#d62728')
        ax2.set_xlabel('Позиция вдоль берега', fontsize=11)
        ax2.set_ylabel('Эрозия (условные единицы)', fontsize=11)
        ax2.set_title('Эрозия с учётом литологии', fontweight='bold')
        ax2.legend()
        ax2.grid(True, alpha=0.3)

        # 3. Зоны высокой/низкой эрозии
        ax3 = axes[1, 0]
        erosion_zones = ['Высокая' if e > 75 else 'Низкая' for e in modified_erosion]
        zone_colors = ['#d62728' if z == 'Высокая' else '#2ca02c' for z in erosion_zones]

        ax3.scatter(positions, modified_erosion, c=zone_colors, s=80, alpha=0.7, edgecolors='black')
        ax3.axhline(y=75, color='blue', linestyle='--', linewidth=2, label='Порог')
        ax3.set_xlabel('Позиция вдоль берега', fontsize=11)
        ax3.set_ylabel('Модифицированная эрозия', fontsize=11)
        ax3.set_title('Зоны эрозии', fontweight='bold')
        ax3.legend()
        ax3.grid(True, alpha=0.3)

        # 4. Корреляция свойств
        ax4 = axes[1, 1]
        resistances = [p.resistance for p in points]
        porosities = [p.porosity * 100 for p in points]

        scatter = ax4.scatter(resistances, porosities, c=[p.resistance for p in points],
                           cmap='RdYlGn_r', s=80, alpha=0.7, edgecolors='black')
        ax4.set_xlabel('Сопротивление', fontsize=11)
        ax4.set_ylabel('Пористость (%)', fontsize=11)
        ax4.set_title('Корреляция сопротивления и пористости', fontweight='bold')
        ax4.grid(True, alpha=0.3)

        plt.colorbar(scatter, ax=ax4, label='Сопротивление')

        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()

        return output_path


def generate_report(analyzer: LithologyAnalyzer) -> str:
    """Генерирует текстовый отчёт"""
    lines = []
    lines.append("=" * 80)
    lines.append("АНАЛИЗ ЛИТОЛОГИЧЕСКОЙ ВАРИАБЕЛЬНОСТИ")
    lines.append("=" * 80)
    lines.append("")

    # Общая статистика
    lines.append("ОБЩАЯ СТАТИСТИКА")
    lines.append("-" * 80)
    lines.append(f"  Всего точек:              {len(analyzer.points)}")

    if analyzer.points:
        resistances = [p.resistance for p in analyzer.points]
        porosities = [p.porosity for p in analyzer.points]
        fractures = [p.fracture_density for p in analyzer.points]
        weathered = [p for p in analyzer.points if p.is_weathered]

        lines.append(f"  Ср. сопротивление:       {np.mean(resistances):.2f}")
        lines.append(f"  Мин. сопротивление:       {np.min(resistances):.2f}")
        lines.append(f"  Макс. сопротивление:      {np.max(resistances):.2f}")
        lines.append(f"  Ср. пористость:          {np.mean(porosities)*100:.1f}%")
        lines.append(f"  Ср. трещиноватость:      {np.mean(fractures)*100:.1f}%")
        lines.append(f"  Выветрелых пород:         {len(weathered)}/{len(analyzer.points)}")
    lines.append("")

    # Статистика по классам
    stats = analyzer.calculate_statistics()
    if stats:
        lines.append("СТАТИСТИКА ПО КЛАССАМ")
        lines.append("-" * 80)

        class_names = {
            'limestone': 'Известняк',
            'granite': 'Гранит',
            'sandstone': 'Песчаник',
            'shale': 'Глинистый сланец',
            'basalt': 'Базальт',
            'conglomerate': 'Конгломерат',
            'alluvium': 'Аллювий'
        }

        for class_name, stat in stats.items():
            name = class_names.get(class_name, class_name)
            lines.append(f"  {name} ({stat.count} точек):")
            lines.append(f"    Ср. сопротивление:    {stat.mean_resistance:.2f} ± {stat.std_resistance:.2f}")
            lines.append(f"    Ср. пористость:       {stat.mean_porosity*100:.1f}%")
            lines.append(f"    Ср. трещиноватость:   {stat.mean_fracture_density*100:.1f}%")
            lines.append(f"    Выветрелых:           {stat.weathered_fraction*100:.1f}%")
            lines.append("")

    lines.append("=" * 80)

    return "\n".join(lines)


def main():
    """Главная функция"""
    parser = argparse.ArgumentParser(
        description='Анализ литологической вариабельности и выветривания',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Примеры использования:
  python lithology_analysis.py --points 150
  python lithology_analysis.py --autocorr 0.8 --weathering 1000
  python lithology_analysis.py --output lithology_report
        '''
    )

    parser.add_argument('--points', '-n', type=int, default=100,
                       help='Количество точек береговой линии (default: 100)')
    parser.add_argument('--autocorr', '-a', type=float, default=0.7,
                       help='Пространственная автокорреляция [0-1] (default: 0.7)')
    parser.add_argument('--noise', type=float, default=0.2,
                       help='Уровень шума [0-1] (default: 0.2)')
    parser.add_argument('--weathering', '-w', type=float, default=0,
                       help='Время выветривания в годах (default: 0)')
    parser.add_argument('--climate', '-c', type=float, default=1.0,
                       help='Климатический множитель (default: 1.0)')
    parser.add_argument('--storms', '-s', type=int, default=0,
                       help='Количество штормовых событий (default: 0)')
    parser.add_argument('--output', '-o', help='Базовое имя для выходных файлов')
    parser.add_argument('--seed', type=int, help='Сид для воспроизводимости')

    args = parser.parse_args()

    try:
        # Создаём анализатор
        analyzer = LithologyAnalyzer(num_points=args.points)

        # Генерируем пространственную литологию
        points = analyzer.generate_spatial_lithology(
            autocorrelation=args.autocorr,
            noise_level=args.noise,
            seed=args.seed
        )
        print(f"✓ Сгенерировано {len(points)} точек литологии")

        # Применяем выветривание
        if args.weathering > 0:
            analyzer.apply_weathering(args.weathering, args.climate)
            print(f"✓ Применено выветривание: {args.weathering} лет")

        # Применяем штормовые воздействия
        if args.storms > 0:
            storm_indices = np.random.choice(len(points),
                                           size=min(args.storms, len(points)),
                                           replace=False)
            analyzer.apply_storm_impact(storm_indices.tolist())
            print(f"✓ Применено {args.storms} штормовых воздействий")

        # Рассчитываем статистику
        stats = analyzer.calculate_statistics()
        print(f"✓ Рассчитана статистика для {len(stats)} классов")

        # Генерируем отчёт
        report_text = generate_report(analyzer)
        print(report_text)

        # Базовый путь для выходных файлов
        output_base = args.output or 'lithology_analysis'
        output_dir = Path('output/report')
        output_dir.mkdir(parents=True, exist_ok=True)

        # Сохраняем отчёт
        report_path = output_dir / f'{output_base}_report.txt'
        with open(report_path, 'w', encoding='utf-8') as f:
            f.write(report_text)
        print(f"✓ Отчёт сохранен: {report_path}")

        # Создаём визуализатор
        visualizer = LithologyVisualizer(analyzer)

        # Генерируем визуализации
        lithology_map = visualizer.create_lithology_map(
            output_dir / f'{output_base}_map.png')
        print(f"✓ Карта литологии: {lithology_map}")

        if args.weathering > 0 or args.storms > 0:
            weathering_plot = visualizer.create_weathering_plot(
                output_dir / f'{output_base}_weathering.png')
            print(f"✓ Влияние выветривания: {weathering_plot}")

        interaction_plot = visualizer.create_interaction_plot(
            output_dir / f'{output_base}_interaction.png')
        print(f"✓ Взаимодействие с эрозией: {interaction_plot}")

        return 0

    except Exception as e:
        print(f"❌ Неожиданная ошибка: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        return 1


if __name__ == '__main__':
    sys.exit(main())
