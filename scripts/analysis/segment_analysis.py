#!/usr/bin/env python3
"""
Анализ сегментов береговой линии для Litora-CLI

Классифицирует сегменты берега (бухты, мысы, прямые участки) и анализирует их уязвимость.
"""

import argparse
import sys
from pathlib import Path
import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
from matplotlib.patches import FancyBboxPatch, Circle
from dataclasses import dataclass
from typing import List, Tuple, Optional
import json


@dataclass
class SegmentInfo:
    """Информация о сегменте береговой линии"""
    index: int
    start_lat: float
    start_lon: float
    end_lat: float
    end_lon: float
    length_km: float
    segment_type: str  # 'bay', 'cape', 'straight', 'complex'
    exposure: str  # 'protected', 'exposed', 'semi_protected'
    erosion_rate: float  # км/шаг
    total_erosion: float  # км
    vulnerability_score: float  # 0-100
    curvature: float  # кривизна сегмента
    orientation: float  # ориентация в градусах


class SegmentAnalyzer:
    """Класс для анализа сегментов береговой линии"""

    def __init__(self, csv_path: str, geojson_path: Optional[str] = None):
        """
        Инициализация анализатора сегментов

        Args:
            csv_path: Путь к CSV файлу с метриками эрозии
            geojson_path: Опционально путь к GeoJSON с координатами береговой линии
        """
        self.csv_path = Path(csv_path)
        if not self.csv_path.exists():
            raise FileNotFoundError(f"CSV файл не найден: {csv_path}")

        self.df = pd.read_csv(self.csv_path)
        self.segments: List[SegmentInfo] = []
        self.geojson_path = geojson_path

        # Загружаем координаты если есть GeoJSON
        self.coastline_coords = None
        if geojson_path:
            self.coastline_coords = self._load_geojson(geojson_path)

    def _load_geojson(self, geojson_path: str) -> List[Tuple[float, float]]:
        """Загружает координаты из GeoJSON или простого массива точек"""
        try:
            with open(geojson_path, 'r') as f:
                data = json.load(f)

            coords = []

            # Если это простой массив точек (формат Litora)
            if isinstance(data, list):
                for point in data:
                    if isinstance(point, dict):
                        lat = point.get('Lat') or point.get('lat')
                        lon = point.get('Lon') or point.get('lon')
                        if lat is not None and lon is not None:
                            coords.append((float(lat), float(lon)))
                    elif isinstance(point, list) and len(point) >= 2:
                        coords.append((float(point[1]), float(point[0])))
                return coords if coords else None

            # GeoJSON формат
            if data.get('type') == 'FeatureCollection':
                features = data.get('features', [])
                for feature in features:
                    geom = feature.get('geometry', {})
                    if geom.get('type') == 'LineString':
                        coords = geom.get('coordinates', [])
                        return [(lat, lon) for lon, lat in coords]
            elif data.get('type') == 'LineString':
                coords = data.get('coordinates', [])
                return [(lat, lon) for lon, lat in coords]

            return None
        except Exception as e:
            print(f"⚠️  Предупреждение: не удалось загрузить GeoJSON: {e}")
            return None

    def classify_segments(self, window_size: int = 5) -> List[SegmentInfo]:
        """
        Классифицирует сегменты береговой линии

        Args:
            window_size: Размер окна для анализа кривизны
        """
        if self.coastline_coords is None or len(self.coastline_coords) < 3:
            print("⚠️  Недостаточно координат для классификации сегментов")
            return []

        segments = []
        coords = self.coastline_coords

        # Анализируем каждый сегмент
        for i in range(len(coords) - 1):
            segment = self._analyze_segment(coords, i, window_size)
            segments.append(segment)

        self.segments = segments
        return segments

    def _analyze_segment(self, coords: List[Tuple[float, float]],
                        index: int, window_size: int) -> SegmentInfo:
        """Анализирует отдельный сегмент"""
        start_lat, start_lon = coords[index]
        end_lat, end_lon = coords[index + 1]

        # Длина сегмента
        length_km = self._haversine_distance(start_lat, start_lon, end_lat, end_lon)

        # Кривизна (используем окно для анализа)
        curvature = self._calculate_curvature(coords, index, window_size)

        # Ориентация сегмента
        orientation = self._calculate_orientation(start_lat, start_lon, end_lat, end_lon)

        # Тип сегмента на основе кривизны
        segment_type = self._classify_by_curvature(curvature)

        # Экспозиция на основе ориентации и типа
        exposure = self._classify_exposure(segment_type, orientation)

        # Оценка уязвимости
        vulnerability = self._calculate_vulnerability(
            segment_type, exposure, curvature, length_km
        )

        # Темп эрозии (оценка)
        erosion_rate = self._estimate_erosion_rate(segment_type, exposure)

        return SegmentInfo(
            index=index,
            start_lat=start_lat,
            start_lon=start_lon,
            end_lat=end_lat,
            end_lon=end_lon,
            length_km=length_km,
            segment_type=segment_type,
            exposure=exposure,
            erosion_rate=erosion_rate,
            total_erosion=erosion_rate * 10,  # Предполагаем 10 шагов
            vulnerability_score=vulnerability,
            curvature=curvature,
            orientation=orientation
        )

    def _haversine_distance(self, lat1: float, lon1: float,
                           lat2: float, lon2: float) -> float:
        """Вычисляет расстояние между двумя точками (км)"""
        from math import radians, sin, cos, sqrt, asin

        R = 6371  # Радиус Земли в км

        dlat = radians(lat2 - lat1)
        dlon = radians(lon2 - lon1)
        a = sin(dlat/2)**2 + cos(radians(lat1)) * cos(radians(lat2)) * sin(dlon/2)**2
        c = 2 * asin(sqrt(a))

        return R * c

    def _calculate_curvature(self, coords: List[Tuple[float, float]],
                            index: int, window_size: int) -> float:
        """Вычисляет кривизну в точке"""
        n = len(coords)
        if n < 3:
            return 0.0

        # Берем точки вокруг текущей
        start_idx = max(0, index - window_size // 2)
        end_idx = min(n - 1, index + window_size // 2)

        if end_idx - start_idx < 2:
            return 0.0

        # Вычисляем угол отклонения
        points = coords[start_idx:end_idx + 1]
        if len(points) < 3:
            return 0.0

        # Упрощенная кривизна через углы
        angles = []
        for i in range(len(points) - 2):
            p1 = np.array(points[i])
            p2 = np.array(points[i + 1])
            p3 = np.array(points[i + 2])

            v1 = p2 - p1
            v2 = p3 - p2

            # Угол между векторами
            cos_angle = np.dot(v1, v2) / (np.linalg.norm(v1) * np.linalg.norm(v2) + 1e-10)
            cos_angle = np.clip(cos_angle, -1, 1)
            angle = np.arccos(cos_angle)
            angles.append(angle)

        if not angles:
            return 0.0

        return np.mean(angles)

    def _calculate_orientation(self, lat1: float, lon1: float,
                              lat2: float, lon2: float) -> float:
        """Вычисляет ориентацию сегмента в градусах"""
        dlat = lat2 - lat1
        dlon = lon2 - lon1

        # Азимут
        angle = np.arctan2(dlat, dlon)
        degrees = np.degrees(angle)

        # Нормализуем к 0-360
        return (degrees + 360) % 360

    def _classify_by_curvature(self, curvature: float) -> str:
        """Классифицирует сегмент по кривизне"""
        if curvature < 0.05:
            return 'straight'
        elif curvature < 0.15:
            return 'gentle_curve'
        elif curvature < 0.3:
            return 'bay' if self._is_concave(curvature) else 'cape'
        else:
            return 'complex'

    def _is_concave(self, curvature: float) -> bool:
        """Определяет, вогнута ли кривая (бухта) или выпуклая (мыс)"""
        # Упрощенно: используем знак кривизны
        return curvature > 0

    def _classify_exposure(self, segment_type: str, orientation: float) -> str:
        """Классифицирует экспозицию сегмента"""
        # Мысы обычно более экспонированы
        if segment_type == 'cape':
            return 'exposed'
        elif segment_type == 'bay':
            return 'protected'
        elif segment_type == 'straight':
            # Прямые участки: зависимость от ориентации
            # Если направлен на север/северо-восток (основные ветра)
            if 315 <= orientation <= 360 or 0 <= orientation <= 45:
                return 'exposed'
            else:
                return 'semi_protected'
        else:
            return 'semi_protected'

    def _calculate_vulnerability(self, segment_type: str, exposure: str,
                                curvature: float, length_km: float) -> float:
        """Вычисляет оценку уязвимости (0-100)"""
        score = 50.0  # Базовый score

        # Тип сегмента
        type_scores = {
            'cape': 20,
            'straight': 10,
            'gentle_curve': 5,
            'bay': -20,
            'complex': 0
        }
        score += type_scores.get(segment_type, 0)

        # Экспозиция
        exposure_scores = {
            'exposed': 25,
            'semi_protected': 0,
            'protected': -15
        }
        score += exposure_scores.get(exposure, 0)

        # Кривизна
        score += curvature * 30

        # Длина (длинные сегменты более уязвимы)
        if length_km > 10:
            score += 10
        elif length_km < 2:
            score -= 5

        return np.clip(score, 0, 100)

    def _estimate_erosion_rate(self, segment_type: str, exposure: str) -> float:
        """Оценивает темп эрозии для сегмента (км/шаг)"""
        base_rates = {
            'cape': 0.5,
            'straight': 0.3,
            'gentle_curve': 0.2,
            'bay': 0.05,
            'complex': 0.15
        }

        base_rate = base_rates.get(segment_type, 0.2)

        # Модификатор экспозиции
        exposure_mults = {
            'exposed': 2.0,
            'semi_protected': 1.0,
            'protected': 0.3
        }

        return base_rate * exposure_mults.get(exposure, 1.0)

    def analyze_erosion_by_segments(self) -> pd.DataFrame:
        """Анализирует эрозию по типам сегментов"""
        if not self.segments:
            return pd.DataFrame()

        data = []
        for seg in self.segments:
            data.append({
                'segment_index': seg.index,
                'type': seg.segment_type,
                'exposure': seg.exposure,
                'length_km': seg.length_km,
                'erosion_rate_km_step': seg.erosion_rate,
                'vulnerability_score': seg.vulnerability_score,
                'curvature': seg.curvature,
                'orientation_deg': seg.orientation
            })

        df = pd.DataFrame(data)

        # Добавляем сводную статистику
        summary = {
            'total_segments': len(df),
            'by_type': df.groupby('type').size().to_dict(),
            'by_exposure': df.groupby('exposure').size().to_dict(),
            'avg_vulnerability': df['vulnerability_score'].mean(),
            'high_vulnerability_count': len(df[df['vulnerability_score'] > 70]),
            'total_length_km': df['length_km'].sum()
        }

        return df, summary

    def get_problematic_zones(self, threshold: float = 70) -> List[SegmentInfo]:
        """Возвращает проблемные зоны (высокая уязвимость)"""
        return [s for s in self.segments if s.vulnerability_score > threshold]

    def compare_protected_unprotected(self) -> dict:
        """Сравнивает защищенные и незащищенные участки"""
        if not self.segments:
            return {}

        protected = [s for s in self.segments if s.exposure == 'protected']
        exposed = [s for s in self.segments if s.exposure == 'exposed']
        semi = [s for s in self.segments if s.exposure == 'semi_protected']

        def calc_stats(segments):
            if not segments:
                return {'count': 0, 'total_length': 0, 'avg_erosion': 0}
            return {
                'count': len(segments),
                'total_length': sum(s.length_km for s in segments),
                'avg_erosion': np.mean([s.erosion_rate for s in segments]),
                'avg_vulnerability': np.mean([s.vulnerability_score for s in segments])
            }

        return {
            'protected': calc_stats(protected),
            'exposed': calc_stats(exposed),
            'semi_protected': calc_stats(semi)
        }


class SegmentVisualizer:
    """Класс для визуализации сегментов береговой линии"""

    def __init__(self, analyzer: SegmentAnalyzer):
        self.analyzer = analyzer
        self.colors = {
            'bay': '#2ca02c',      # зеленый - защищенные бухты
            'cape': '#d62728',     # красный - опасные мысы
            'straight': '#1f77b4', # синий - прямые участки
            'gentle_curve': '#ff7f0e',  # оранжевый
            'complex': '#9467bd'   # фиолетовый
        }
        self.exposure_colors = {
            'protected': '#2ca02c',
            'exposed': '#d62728',
            'semi_protected': '#ff7f0e'
        }

    def create_segment_map(self, output_path: str = 'segment_map.png'):
        """Создает карту сегментов береговой линии"""
        if not self.analyzer.segments:
            print("⚠️  Нет сегментов для визуализации")
            return

        fig, ax = plt.subplots(figsize=(16, 12))

        # Собираем координаты
        lats = []
        lons = []
        colors = []

        for seg in self.analyzer.segments:
            lats.extend([seg.start_lat, seg.end_lat])
            lons.extend([seg.start_lon, seg.end_lon])
            colors.extend([self.colors.get(seg.segment_type, '#666666'),
                          self.colors.get(seg.segment_type, '#666666')])

        # Рисуем сегменты
        for i, seg in enumerate(self.analyzer.segments):
            color = self.colors.get(seg.segment_type, '#666666')
            linewidth = 2 + seg.vulnerability_score / 25  # Толщина по уязвимости

            ax.plot([seg.start_lon, seg.end_lon],
                   [seg.start_lat, seg.end_lat],
                   color=color, linewidth=linewidth, alpha=0.7, solid_capstyle='round')

            # Подписываем проблемные зоны
            if seg.vulnerability_score > 70:
                ax.scatter([(seg.start_lon + seg.end_lon) / 2],
                          [(seg.start_lat + seg.end_lat) / 2],
                          c='red', s=100, marker='X', zorder=5,
                          edgecolors='black', linewidths=1)

        # Легенда
        legend_elements = [
            plt.Line2D([0], [0], color=self.colors['bay'], lw=3, label='Бухта (защищенная)'),
            plt.Line2D([0], [0], color=self.colors['cape'], lw=3, label='Мыс (опасный)'),
            plt.Line2D([0], [0], color=self.colors['straight'], lw=3, label='Прямой участок'),
            plt.Line2D([0], [0], color=self.colors['gentle_curve'], lw=3, label='Плавный изгиб'),
            plt.Line2D([0], [0], color=self.colors['complex'], lw=3, label='Сложный участок'),
            plt.Line2D([0], [0], marker='X', color='w', markerfacecolor='red',
                      markersize=10, label='Проблемная зона', lw=0)
        ]
        ax.legend(handles=legend_elements, loc='upper right', fontsize=11)

        ax.set_xlabel('Долгота', fontsize=12, fontweight='bold')
        ax.set_ylabel('Широта', fontsize=12, fontweight='bold')
        ax.set_title('КАРТА СЕГМЕНТОВ БЕРЕГОВОЙ ЛИНИИ', fontsize=16, fontweight='bold')
        ax.grid(True, alpha=0.3)
        ax.set_aspect('equal', adjustable='box')

        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()

        return output_path

    def create_exposure_map(self, output_path: str = 'exposure_map.png'):
        """Создает карту экспозиции участков"""
        if not self.analyzer.segments:
            print("⚠️  Нет сегментов для визуализации")
            return

        fig, ax = plt.subplots(figsize=(16, 12))

        # Рисуем сегменты по экспозиции
        for seg in self.analyzer.segments:
            color = self.exposure_colors.get(seg.exposure, '#666666')
            linewidth = 2 + (100 - seg.vulnerability_score) / 30

            ax.plot([seg.start_lon, seg.end_lon],
                   [seg.start_lat, seg.end_lat],
                   color=color, linewidth=linewidth, alpha=0.7, solid_capstyle='round')

        # Легенда
        legend_elements = [
            plt.Line2D([0], [0], color=self.exposure_colors['protected'], lw=4,
                      label='Защищенный'),
            plt.Line2D([0], [0], color=self.exposure_colors['semi_protected'], lw=4,
                      label='Полузащищенный'),
            plt.Line2D([0], [0], color=self.exposure_colors['exposed'], lw=4,
                      label='Экспонированный')
        ]
        ax.legend(handles=legend_elements, loc='upper right', fontsize=12)

        ax.set_xlabel('Долгота', fontsize=12, fontweight='bold')
        ax.set_ylabel('Широта', fontsize=12, fontweight='bold')
        ax.set_title('КАРТА ЭКСПОЗИЦИИ БЕРЕГОВОЙ ЛИНИИ', fontsize=16, fontweight='bold')
        ax.grid(True, alpha=0.3)
        ax.set_aspect('equal', adjustable='box')

        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()

        return output_path

    def create_vulnerability_chart(self, output_path: str = 'vulnerability_chart.png'):
        """Создает график уязвимости сегментов"""
        if not self.analyzer.segments:
            print("⚠️  Нет сегментов для визуализации")
            return

        fig, axes = plt.subplots(2, 2, figsize=(16, 12))
        fig.suptitle('АНАЛИЗ УЯЗВИМОСТИ СЕГМЕНТОВ', fontsize=16, fontweight='bold')

        segments = self.analyzer.segments

        # 1. Распределение по типам
        ax1 = axes[0, 0]
        type_counts = {}
        for seg in segments:
            type_counts[seg.segment_type] = type_counts.get(seg.segment_type, 0) + 1

        if type_counts:
            colors = [self.colors.get(t, '#666666') for t in type_counts.keys()]
            ax1.pie(type_counts.values(), labels=type_counts.keys(), autopct='%1.1f%%',
                   colors=colors, startangle=90)
            ax1.set_title('Распределение по типам сегментов', fontweight='bold')

        # 2. Распределение по экспозиции
        ax2 = axes[0, 1]
        exposure_counts = {}
        for seg in segments:
            exposure_counts[seg.exposure] = exposure_counts.get(seg.exposure, 0) + 1

        if exposure_counts:
            exp_colors = [self.exposure_colors.get(e, '#666666') for e in exposure_counts.keys()]
            ax2.pie(exposure_counts.values(), labels=exposure_counts.keys(), autopct='%1.1f%%',
                   colors=exp_colors, startangle=90)
            ax2.set_title('Распределение по экспозиции', fontweight='bold')

        # 3. Уязвимость по сегментам
        ax3 = axes[1, 0]
        indices = [seg.index for seg in segments]
        vulnerabilities = [seg.vulnerability_score for seg in segments]
        bar_colors = [self.colors.get(seg.segment_type, '#666666') for seg in segments]

        ax3.bar(indices, vulnerabilities, color=bar_colors, alpha=0.7, edgecolor='black')
        ax3.axhline(y=70, color='red', linestyle='--', linewidth=2, label='Порог уязвимости')
        ax3.set_xlabel('Номер сегмента', fontsize=10)
        ax3.set_ylabel('Оценка уязвимости', fontsize=10)
        ax3.set_title('Уязвимость по сегментам', fontweight='bold')
        ax3.legend()
        ax3.grid(True, alpha=0.3, axis='y')

        # 4. Темпы эрозии по типам
        ax4 = axes[1, 1]
        type_erosion = {}
        for seg in segments:
            if seg.segment_type not in type_erosion:
                type_erosion[seg.segment_type] = []
            type_erosion[seg.segment_type].append(seg.erosion_rate)

        if type_erosion:
            data_to_plot = [type_erosion[t] for t in type_erosion.keys()]
            bp = ax4.boxplot(data_to_plot, tick_labels=list(type_erosion.keys()),
                            patch_artist=True)

            for patch, type_name in zip(bp['boxes'], type_erosion.keys()):
                patch.set_facecolor(self.colors.get(type_name, '#666666'))
                patch.set_alpha(0.7)

            ax4.set_xlabel('Тип сегмента', fontsize=10)
            ax4.set_ylabel('Темп эрозии (км/шаг)', fontsize=10)
            ax4.set_title('Темпы эрозии по типам', fontweight='bold')
            ax4.grid(True, alpha=0.3, axis='y')

        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()

        return output_path

    def create_comparison_dashboard(self, output_path: str = 'comparison_dashboard.png'):
        """Создает панель сравнения защищенных/незащищенных участков"""
        comparison = self.analyzer.compare_protected_unprotected()

        if not comparison:
            print("⚠️  Нет данных для сравнения")
            return

        fig, axes = plt.subplots(2, 2, figsize=(16, 12))
        fig.suptitle('СРАВНЕНИЕ ЗАЩИЩЕННЫХ/НЕЗАЩИЩЕННЫХ УЧАСТКОВ',
                    fontsize=16, fontweight='bold')

        categories = ['protected', 'semi_protected', 'exposed']
        category_labels = ['Защищенные', 'Полузащищенные', 'Экспонированные']

        # 1. Количество сегментов
        ax1 = axes[0, 0]
        counts = [comparison[cat]['count'] for cat in categories]
        colors = [self.exposure_colors[cat] for cat in categories]
        ax1.bar(category_labels, counts, color=colors, alpha=0.7, edgecolor='black')
        ax1.set_ylabel('Количество сегментов', fontsize=10)
        ax1.set_title('Количество сегментов', fontweight='bold')
        ax1.grid(True, alpha=0.3, axis='y')

        # 2. Общая длина
        ax2 = axes[0, 1]
        lengths = [comparison[cat]['total_length'] for cat in categories]
        ax2.bar(category_labels, lengths, color=colors, alpha=0.7, edgecolor='black')
        ax2.set_ylabel('Общая длина (км)', fontsize=10)
        ax2.set_title('Общая длина участков', fontweight='bold')
        ax2.grid(True, alpha=0.3, axis='y')

        # 3. Средняя эрозия
        ax3 = axes[1, 0]
        erosions = [comparison[cat]['avg_erosion'] for cat in categories]
        ax3.bar(category_labels, erosions, color=colors, alpha=0.7, edgecolor='black')
        ax3.set_ylabel('Средний темп эрозии (км/шаг)', fontsize=10)
        ax3.set_title('Средние темпы эрозии', fontweight='bold')
        ax3.grid(True, alpha=0.3, axis='y')

        # 4. Средняя уязвимость
        ax4 = axes[1, 1]
        vulnerabilities = [comparison[cat]['avg_vulnerability'] for cat in categories]
        ax4.bar(category_labels, vulnerabilities, color=colors, alpha=0.7, edgecolor='black')
        ax4.axhline(y=70, color='red', linestyle='--', linewidth=2, label='Порог')
        ax4.set_ylabel('Средняя уязвимость', fontsize=10)
        ax4.set_title('Средняя уязвимость', fontweight='bold')
        ax4.legend()
        ax4.grid(True, alpha=0.3, axis='y')

        plt.tight_layout()
        plt.savefig(output_path, dpi=300, bbox_inches='tight')
        plt.close()

        return output_path


def generate_report(analyzer: SegmentAnalyzer) -> str:
    """Генерирует текстовый отчет"""
    lines = []
    lines.append("=" * 80)
    lines.append("АНАЛИЗ СЕГМЕНТОВ БЕРЕГОВОЙ ЛИНИИ")
    lines.append("=" * 80)
    lines.append("")

    # Общая статистика
    df, summary = analyzer.analyze_erosion_by_segments()
    if df is not None and not df.empty:
        lines.append("ОБЩАЯ СТАТИСТИКА")
        lines.append("-" * 80)
        lines.append(f"  Всего сегментов:           {summary['total_segments']}")
        lines.append(f"  Общая длина:               {summary['total_length_km']:.1f} км")
        lines.append(f"  Средняя уязвимость:        {summary['avg_vulnerability']:.1f}/100")
        lines.append(f"  Проблемных зон (70+):      {summary['high_vulnerability_count']}")
        lines.append("")

        # Распределение по типам
        lines.append("РАСПРЕДЕЛЕНИЕ ПО ТИПАМ")
        lines.append("-" * 80)
        type_names = {
            'bay': 'Бухты',
            'cape': 'Мысы',
            'straight': 'Прямые участки',
            'gentle_curve': 'Плавные изгибы',
            'complex': 'Сложные участки'
        }
        for seg_type, count in summary['by_type'].items():
            name = type_names.get(seg_type, seg_type)
            pct = (count / summary['total_segments']) * 100
            lines.append(f"  {name:20} : {count:3d} ({pct:5.1f}%)")
        lines.append("")

        # Распределение по экспозиции
        lines.append("РАСПРЕДЕЛЕНИЕ ПО ЭКСПОЗИЦИИ")
        lines.append("-" * 80)
        exp_names = {
            'protected': 'Защищенные',
            'exposed': 'Экспонированные',
            'semi_protected': 'Полузащищенные'
        }
        for exp, count in summary['by_exposure'].items():
            name = exp_names.get(exp, exp)
            pct = (count / summary['total_segments']) * 100
            lines.append(f"  {name:20} : {count:3d} ({pct:5.1f}%)")
        lines.append("")

        # Сравнение защищенных/незащищенных
        comparison = analyzer.compare_protected_unprotected()
        if comparison:
            lines.append("СРАВНЕНИЕ ЗАЩИЩЕННЫХ/НЕЗАЩИЩЕННЫХ УЧАСТКОВ")
            lines.append("-" * 80)

            for cat in ['protected', 'semi_protected', 'exposed']:
                data = comparison[cat]
                if data['count'] > 0:
                    name = exp_names.get(cat, cat)
                    lines.append(f"  {name:20}:")
                    lines.append(f"    Сегментов:           {data['count']}")
                    lines.append(f"    Длина:                {data['total_length']:.1f} км")
                    lines.append(f"    Ср. темп эрозии:      {data['avg_erosion']:.3f} км/шаг")
                    lines.append(f"    Ср. уязвимость:       {data['avg_vulnerability']:.1f}/100")
            lines.append("")

        # Проблемные зоны
        problematic = analyzer.get_problematic_zones()
        if problematic:
            lines.append("ПРОБЛЕМНЫЕ ЗОНЫ (уязвимость > 70)")
            lines.append("-" * 80)
            for i, zone in enumerate(problematic[:10], 1):  # Максимум 10
                type_name = type_names.get(zone.segment_type, zone.segment_type)
                exp_name = exp_names.get(zone.exposure, zone.exposure)
                lines.append(f"  {i}. Сегмент #{zone.index}:")
                lines.append(f"     Тип:          {type_name}")
                lines.append(f"     Экспозиция:   {exp_name}")
                lines.append(f"     Уязвимость:   {zone.vulnerability_score:.1f}/100")
                lines.append(f"     Длина:        {zone.length_km:.2f} км")
                lines.append(f"     Темп эрозии:  {zone.erosion_rate:.3f} км/шаг")
                lines.append("")

    lines.append("=" * 80)

    return "\n".join(lines)


def main():
    """Главная функция"""
    parser = argparse.ArgumentParser(
        description='Анализ сегментов береговой линии',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Примеры использования:
  python segment_analysis.py output/csv/erosion_metrics.csv
  python segment_analysis.py output/csv/erosion_metrics.csv --geojson data/black-sea.json
  python segment_analysis.py output/csv/erosion_metrics.csv --threshold 65 --output segment_report
        '''
    )

    parser.add_argument('csv_file', help='Путь к CSV файлу с метриками эрозии')
    parser.add_argument('--geojson', '-g', help='Путь к GeoJSON файлу с координатами береговой линии')
    parser.add_argument('--threshold', '-t', type=float, default=70,
                       help='Порог уязвимости для проблемных зон (default: 70)')
    parser.add_argument('--output', '-o', help='Базовое имя для выходных файлов')
    parser.add_argument('--window-size', '-w', type=int, default=5,
                       help='Размер окна для анализа кривизны (default: 5)')
    parser.add_argument('--svg', action='store_true',
                       help='Генерировать SVG визуализации')
    parser.add_argument('--report-only', action='store_true',
                       help='Генерировать только текстовый отчет')

    args = parser.parse_args()

    try:
        # Создаем анализатор
        analyzer = SegmentAnalyzer(args.csv_file, args.geojson)

        # Классифицируем сегменты
        segments = analyzer.classify_segments(args.window_size)

        if not segments:
            print("⚠️  Не удалось классифицировать сегменты")
            print("💡 Укажите GeoJSON файл с координатами: --geojson data/black-sea.json")
            return 1

        print(f"✓ Классифицировано {len(segments)} сегментов")

        # Генерируем отчет
        report_text = generate_report(analyzer)
        print(report_text)

        # Базовый путь для выходных файлов
        output_base = args.output or 'segment_analysis'
        output_dir = Path('output/report')
        output_dir.mkdir(parents=True, exist_ok=True)

        # Сохраняем отчет
        report_path = output_dir / f'{output_base}_report.txt'
        with open(report_path, 'w', encoding='utf-8') as f:
            f.write(report_text)
        print(f"✓ Отчет сохранен: {report_path}")

        if args.report_only:
            return 0

        # Создаем визуализатор
        visualizer = SegmentVisualizer(analyzer)

        # Генерируем визуализации
        if args.svg:
            # SVG формат
            segment_map = visualizer.create_segment_map(
                output_dir / f'{output_base}_segments.svg')
            print(f"✓ Карта сегментов: {segment_map}")
        else:
            # PNG формат
            segment_map = visualizer.create_segment_map(
                output_dir / f'{output_base}_segments.png')
            print(f"✓ Карта сегментов: {segment_map}")

        exposure_map = visualizer.create_exposure_map(
            output_dir / f'{output_base}_exposure.png')
        print(f"✓ Карта экспозиции: {exposure_map}")

        vulnerability_chart = visualizer.create_vulnerability_chart(
            output_dir / f'{output_base}_vulnerability.png')
        print(f"✓ График уязвимости: {vulnerability_chart}")

        comparison_dashboard = visualizer.create_comparison_dashboard(
            output_dir / f'{output_base}_comparison.png')
        print(f"✓ Панель сравнения: {comparison_dashboard}")

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
