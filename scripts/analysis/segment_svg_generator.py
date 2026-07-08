#!/usr/bin/env python3
"""
SVG генератор карт сегментов береговой линии для Litora-CLI

Создает интерактивные SVG карты с классификацией сегментов берега.
"""

import argparse
import sys
from pathlib import Path
import json
import math
from dataclasses import dataclass
from typing import List, Tuple, Optional


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
    erosion_rate: float
    vulnerability_score: float
    curvature: float
    orientation: float


class SVGSegmentGenerator:
    """Генератор SVG карт сегментов"""

    def __init__(self, width: int = 1200, height: int = 800):
        self.width = width
        self.height = height
        self.padding = 50

        # Цветовая схема
        self.colors = {
            'bay': '#2ca02c',
            'cape': '#d62728',
            'straight': '#1f77b4',
            'gentle_curve': '#ff7f0e',
            'complex': '#9467bd',
            'protected': '#2ca02c',
            'exposed': '#d62728',
            'semi_protected': '#ff7f0e',
            'water': '#e6f3ff',
            'land': '#f5e6d3',
            'border': '#333333',
            'text': '#000000',
            'grid': '#cccccc'
        }

        # Русские названия
        self.type_names = {
            'bay': 'Бухта',
            'cape': 'Мыс',
            'straight': 'Прямой участок',
            'gentle_curve': 'Плавный изгиб',
            'complex': 'Сложный участок'
        }

        self.exposure_names = {
            'protected': 'Защищенный',
            'exposed': 'Экспонированный',
            'semi_protected': 'Полузащищенный'
        }

    def load_geojson(self, geojson_path: str) -> List[Tuple[float, float]]:
        """Загружает координаты из GeoJSON или простого массива точек"""
        with open(geojson_path, 'r', encoding='utf-8') as f:
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
            return coords

        # GeoJSON формат
        if data.get('type') == 'FeatureCollection':
            features = data.get('features', [])
            for feature in features:
                geom = feature.get('geometry', {})
                if geom.get('type') == 'LineString':
                    coordinates = geom.get('coordinates', [])
                    coords = [(lat, lon) for lon, lat in coordinates]
                    break
        elif data.get('type') == 'LineString':
            coordinates = data.get('coordinates', [])
            coords = [(lat, lon) for lon, lat in coordinates]

        return coords

    def classify_segments(self, coords: List[Tuple[float, float]],
                         window_size: int = 5) -> List[SegmentInfo]:
        """Классифицирует сегменты береговой линии"""
        segments = []

        for i in range(len(coords) - 1):
            start_lat, start_lon = coords[i]
            end_lat, end_lon = coords[i + 1]

            # Длина сегмента
            length_km = self._haversine_distance(start_lat, start_lon, end_lat, end_lon)

            # Кривизна
            curvature = self._calculate_curvature(coords, i, window_size)

            # Ориентация
            orientation = self._calculate_orientation(start_lat, start_lon, end_lat, end_lon)

            # Тип сегмента
            segment_type = self._classify_by_curvature(curvature)

            # Экспозиция
            exposure = self._classify_exposure(segment_type, orientation)

            # Уязвимость
            vulnerability = self._calculate_vulnerability(
                segment_type, exposure, curvature, length_km
            )

            # Темп эрозии
            erosion_rate = self._estimate_erosion_rate(segment_type, exposure)

            segments.append(SegmentInfo(
                index=i,
                start_lat=start_lat,
                start_lon=start_lon,
                end_lat=end_lat,
                end_lon=end_lon,
                length_km=length_km,
                segment_type=segment_type,
                exposure=exposure,
                erosion_rate=erosion_rate,
                vulnerability_score=vulnerability,
                curvature=curvature,
                orientation=orientation
            ))

        return segments

    def _haversine_distance(self, lat1: float, lon1: float,
                           lat2: float, lon2: float) -> float:
        """Вычисляет расстояние между двумя точками (км)"""
        R = 6371
        dlat = math.radians(lat2 - lat1)
        dlon = math.radians(lon2 - lon1)
        a = (math.sin(dlat/2)**2 +
             math.cos(math.radians(lat1)) * math.cos(math.radians(lat2)) *
             math.sin(dlon/2)**2)
        c = 2 * math.asin(math.sqrt(a))
        return R * c

    def _calculate_curvature(self, coords: List[Tuple[float, float]],
                            index: int, window_size: int) -> float:
        """Вычисляет кривизну в точке"""
        n = len(coords)
        if n < 3:
            return 0.0

        start_idx = max(0, index - window_size // 2)
        end_idx = min(n - 1, index + window_size // 2)

        if end_idx - start_idx < 2:
            return 0.0

        points = coords[start_idx:end_idx + 1]
        if len(points) < 3:
            return 0.0

        angles = []
        for i in range(len(points) - 2):
            p1 = points[i]
            p2 = points[i + 1]
            p3 = points[i + 2]

            v1 = (p2[0] - p1[0], p2[1] - p1[1])
            v2 = (p3[0] - p2[0], p3[1] - p2[1])

            dot = v1[0] * v2[0] + v1[1] * v2[1]
            norm1 = math.sqrt(v1[0]**2 + v1[1]**2)
            norm2 = math.sqrt(v2[0]**2 + v2[1]**2)

            if norm1 > 0 and norm2 > 0:
                cos_angle = max(-1, min(1, dot / (norm1 * norm2)))
                angle = math.acos(cos_angle)
                angles.append(angle)

        return sum(angles) / len(angles) if angles else 0.0

    def _calculate_orientation(self, lat1: float, lon1: float,
                              lat2: float, lon2: float) -> float:
        """Вычисляет ориентацию сегмента в градусах"""
        dlat = lat2 - lat1
        dlon = lon2 - lon1
        angle = math.atan2(dlat, dlon)
        degrees = math.degrees(angle)
        return (degrees + 360) % 360

    def _classify_by_curvature(self, curvature: float) -> str:
        """Классифицирует сегмент по кривизне"""
        if curvature < 0.05:
            return 'straight'
        elif curvature < 0.15:
            return 'gentle_curve'
        elif curvature < 0.3:
            return 'bay'
        else:
            return 'complex'

    def _classify_exposure(self, segment_type: str, orientation: float) -> str:
        """Классифицирует экспозицию сегмента"""
        if segment_type == 'cape':
            return 'exposed'
        elif segment_type == 'bay':
            return 'protected'
        elif segment_type == 'straight':
            if 315 <= orientation <= 360 or 0 <= orientation <= 45:
                return 'exposed'
            else:
                return 'semi_protected'
        else:
            return 'semi_protected'

    def _calculate_vulnerability(self, segment_type: str, exposure: str,
                                curvature: float, length_km: float) -> float:
        """Вычисляет оценку уязвимости (0-100)"""
        score = 50.0

        type_scores = {
            'cape': 20,
            'straight': 10,
            'gentle_curve': 5,
            'bay': -20,
            'complex': 0
        }
        score += type_scores.get(segment_type, 0)

        exposure_scores = {
            'exposed': 25,
            'semi_protected': 0,
            'protected': -15
        }
        score += exposure_scores.get(exposure, 0)

        score += curvature * 30

        if length_km > 10:
            score += 10
        elif length_km < 2:
            score -= 5

        return max(0, min(100, score))

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

        exposure_mults = {
            'exposed': 2.0,
            'semi_protected': 1.0,
            'protected': 0.3
        }

        return base_rate * exposure_mults.get(exposure, 1.0)

    def generate_segment_map(self, segments: List[SegmentInfo],
                            output_path: str) -> str:
        """Генерирует SVG карту сегментов"""
        if not segments:
            raise ValueError("Нет сегментов для визуализации")

        # Вычисляем границы
        all_lats = [s.start_lat for s in segments] + [s.end_lat for s in segments]
        all_lons = [s.start_lon for s in segments] + [s.end_lon for s in segments]

        min_lat, max_lat = min(all_lats), max(all_lats)
        min_lon, max_lon = min(all_lons), max(all_lons)

        lat_span = max_lat - min_lat or 1
        lon_span = max_lon - min_lon or 1

        # Создаем SVG
        svg_content = self._create_svg_header()

        # Фон (вода)
        svg_content += f'<rect width="100%" height="100%" fill="{self.colors["water"]}"/>'

        # Сетка координат
        svg_content += self._create_grid(min_lat, max_lat, min_lon, max_lon)

        # Группы для сегментов
        svg_content += '<g id="segments">\n'

        for seg in segments:
            x1, y1 = self._geo_to_pixel(seg.start_lon, seg.start_lat,
                                          min_lon, max_lat, lon_span, lat_span)
            x2, y2 = self._geo_to_pixel(seg.end_lon, seg.end_lat,
                                          min_lon, max_lat, lon_span, lat_span)

            color = self.colors.get(seg.segment_type, '#666666')
            linewidth = 2 + seg.vulnerability_score / 25

            svg_content += f'  <line x1="{x1:.1f}" y1="{y1:.1f}" x2="{x2:.1f}" y2="{y2:.1f}" '
            svg_content += f'stroke="{color}" stroke-width="{linewidth:.1f}" '
            svg_content += f'stroke-linecap="round" opacity="0.8">\n'
            svg_content += f'    <title>Сегмент #{seg.index}\\n'
            svg_content += f'Тип: {self.type_names.get(seg.segment_type, seg.segment_type)}\\n'
            svg_content += f'Экспозиция: {self.exposure_names.get(seg.exposure, seg.exposure)}\\n'
            svg_content += f'Длина: {seg.length_km:.2f} км\\n'
            svg_content += f'Уязвимость: {seg.vulnerability_score:.1f}/100\\n'
            svg_content += f'Темп эрозии: {seg.erosion_rate:.3f} км/шаг</title>\n'
            svg_content += '  </line>\n'

            # Маркер для проблемных зон
            if seg.vulnerability_score > 70:
                mx, my = (x1 + x2) / 2, (y1 + y2) / 2
                svg_content += f'  <circle cx="{mx:.1f}" cy="{my:.1f}" r="6" '
                svg_content += f'fill="#d62728" stroke="#000" stroke-width="1" opacity="0.9">\n'
                svg_content += f'    <title>⚠️ Проблемная зона (уязвимость: {seg.vulnerability_score:.1f}/100)</title>\n'
                svg_content += '  </circle>\n'

        svg_content += '</g>\n'

        # Легенда
        svg_content += self._create_legend()

        # Заголовок
        svg_content += self._create_title()

        # Масштабная линейка
        svg_content += self._create_scale_bar(lat_span)

        svg_content += '</svg>'

        # Сохраняем
        output_file = Path(output_path)
        output_file.parent.mkdir(parents=True, exist_ok=True)

        with open(output_file, 'w', encoding='utf-8') as f:
            f.write(svg_content)

        return str(output_file)

    def generate_exposure_map(self, segments: List[SegmentInfo],
                             output_path: str) -> str:
        """Генерирует SVG карту экспозиции"""
        if not segments:
            raise ValueError("Нет сегментов для визуализации")

        all_lats = [s.start_lat for s in segments] + [s.end_lat for s in segments]
        all_lons = [s.start_lon for s in segments] + [s.end_lon for s in segments]

        min_lat, max_lat = min(all_lats), max(all_lats)
        min_lon, max_lon = min(all_lons), max(all_lons)

        lat_span = max_lat - min_lat or 1
        lon_span = max_lon - min_lon or 1

        svg_content = self._create_svg_header()
        svg_content += f'<rect width="100%" height="100%" fill="{self.colors["water"]}"/>'
        svg_content += self._create_grid(min_lat, max_lat, min_lon, max_lon)
        svg_content += '<g id="segments">\n'

        for seg in segments:
            x1, y1 = self._geo_to_pixel(seg.start_lon, seg.start_lat,
                                          min_lon, max_lat, lon_span, lat_span)
            x2, y2 = self._geo_to_pixel(seg.end_lon, seg.end_lat,
                                          min_lon, max_lat, lon_span, lat_span)

            color = self.colors.get(seg.exposure, '#666666')
            linewidth = 2 + (100 - seg.vulnerability_score) / 30

            svg_content += f'  <line x1="{x1:.1f}" y1="{y1:.1f}" x2="{x2:.1f}" y2="{y2:.1f}" '
            svg_content += f'stroke="{color}" stroke-width="{linewidth:.1f}" '
            svg_content += f'stroke-linecap="round" opacity="0.8">\n'
            svg_content += f'    <title>Сегмент #{seg.index}\\n'
            svg_content += f'Экспозиция: {self.exposure_names.get(seg.exposure, seg.exposure)}\\n'
            svg_content += f'Уязвимость: {seg.vulnerability_score:.1f}/100</title>\n'
            svg_content += '  </line>\n'

        svg_content += '</g>\n'
        svg_content += self._create_exposure_legend()
        svg_content += self._create_title(title='КАРТА ЭКСПОЗИЦИИ БЕРЕГОВОЙ ЛИНИИ')
        svg_content += self._create_scale_bar(lat_span)
        svg_content += '</svg>'

        output_file = Path(output_path)
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write(svg_content)

        return str(output_file)

    def _create_svg_header(self) -> str:
        """Создает заголовок SVG"""
        return f'''<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg"
     viewBox="0 0 {self.width} {self.height}"
     width="{self.width}" height="{self.height}">
<style>
  .segment {{ transition: stroke-width 0.2s; cursor: pointer; }}
  .segment:hover {{ stroke-width: 4px; }}
  text {{ font-family: Arial, sans-serif; }}
</style>
'''

    def _geo_to_pixel(self, lon: float, lat: float,
                     min_lon: float, max_lat: float,
                     lon_span: float, lat_span: float) -> Tuple[float, float]:
        """Конвертирует географические координаты в пиксели"""
        draw_width = self.width - 2 * self.padding
        draw_height = self.height - 2 * self.padding

        scale = min(draw_width / lon_span, draw_height / lat_span)
        content_width = lon_span * scale
        content_height = lat_span * scale

        origin_x = self.padding + (draw_width - content_width) / 2
        origin_y = self.padding + (draw_height - content_height) / 2

        x = origin_x + (lon - min_lon) * scale
        y = origin_y + content_height - (lat - (max_lat - lat_span)) * scale

        return x, y

    def _create_grid(self, min_lat: float, max_lat: float,
                    min_lon: float, max_lon: float) -> str:
        """Создает сетку координат"""
        grid = '<g id="grid" opacity="0.3">\n'

        # Вертикальные линии (долгота)
        lon_step = (max_lon - min_lon) / 5
        for i in range(6):
            lon = min_lon + lon_step * i
            x, _ = self._geo_to_pixel(lon, min_lat, min_lon, max_lat,
                                      max_lon - min_lon, max_lat - min_lat)
            grid += f'  <line x1="{x:.1f}" y1="{self.padding}" x2="{x:.1f}" '
            grid += f'y2="{self.height - self.padding}" stroke="{self.colors["grid"]}" stroke-dasharray="5,5"/>\n'

        # Горизонтальные линии (широта)
        lat_step = (max_lat - min_lat) / 5
        for i in range(6):
            lat = min_lat + lat_step * i
            _, y = self._geo_to_pixel(min_lon, lat, min_lon, max_lat,
                                      max_lon - min_lon, max_lat - min_lat)
            grid += f'  <line x1="{self.padding}" y1="{y:.1f}" x2="{self.width - self.padding}" '
            grid += f'y2="{y:.1f}" stroke="{self.colors["grid"]}" stroke-dasharray="5,5"/>\n'

        grid += '</g>\n'
        return grid

    def _create_legend(self) -> str:
        """Создает легенду типов сегментов"""
        legend = '<g id="legend" transform="translate(20, 20)">\n'
        legend += '  <rect x="0" y="0" width="200" height="140" fill="white" stroke="#333" opacity="0.9"/>\n'
        legend += '  <text x="10" y="20" font-weight="bold" font-size="14">Типы сегментов</text>\n'

        y = 40
        for seg_type, name in self.type_names.items():
            color = self.colors.get(seg_type, '#666666')
            legend += f'  <line x1="10" y1="{y}" x2="40" y2="{y}" stroke="{color}" stroke-width="3" stroke-linecap="round"/>\n'
            legend += f'  <text x="50" y="{y+4}" font-size="12">{name}</text>\n'
            y += 20

        # Проблемные зоны
        legend += f'  <circle cx="25" cy="{y}" r="5" fill="#d62728" stroke="#000" stroke-width="1"/>\n'
        legend += f'  <text x="50" y="{y+4}" font-size="12">Проблемная зона</text>\n'

        legend += '</g>\n'
        return legend

    def _create_exposure_legend(self) -> str:
        """Создает легенду экспозиции"""
        legend = '<g id="legend" transform="translate(20, 20)">\n'
        legend += '  <rect x="0" y="0" width="200" height="100" fill="white" stroke="#333" opacity="0.9"/>\n'
        legend += '  <text x="10" y="20" font-weight="bold" font-size="14">Экспозиция</text>\n'

        y = 40
        for exposure, name in self.exposure_names.items():
            color = self.colors.get(exposure, '#666666')
            legend += f'  <line x1="10" y1="{y}" x2="40" y2="{y}" stroke="{color}" stroke-width="4" stroke-linecap="round"/>\n'
            legend += f'  <text x="50" y="{y+4}" font-size="12">{name}</text>\n'
            y += 20

        legend += '</g>\n'
        return legend

    def _create_title(self, title: str = 'КАРТА СЕГМЕНТОВ БЕРЕГОВОЙ ЛИНИИ') -> str:
        """Создает заголовок"""
        return f'<text x="{self.width/2}" y="30" text-anchor="middle" font-weight="bold" font-size="18">{title}</text>\n'

    def _create_scale_bar(self, lat_span: float) -> str:
        """Создает масштабную линейку"""
        km_per_degree = 111.0
        span_km = lat_span * km_per_degree

        # Выбираем красивое число
        if span_km > 500:
            target_km = 100
        elif span_km > 200:
            target_km = 50
        elif span_km > 50:
            target_km = 10
        else:
            target_km = 5

        # Конвертируем в пиксели
        scale_km_per_px = span_km / (self.height - 2 * self.padding)
        bar_px = target_km / scale_km_per_px

        x = self.width - self.padding - bar_px - 80
        y = self.height - self.padding - 10

        scale_bar = f'<g id="scale-bar" transform="translate({x}, {y})">\n'
        scale_bar += f'  <line x1="0" y1="0" x2="{bar_px:.1f}" y2="0" stroke="#333" stroke-width="3"/>\n'
        scale_bar += f'  <line x1="0" y1="-5" x2="0" y2="5" stroke="#333" stroke-width="2"/>\n'
        scale_bar += f'  <line x1="{bar_px:.1f}" y1="-5" x2="{bar_px:.1f}" y2="5" stroke="#333" stroke-width="2"/>\n'
        scale_bar += f'  <text x="{bar_px/2:.1f}" y="-10" text-anchor="middle" font-size="12">{target_km} км</text>\n'
        scale_bar += '</g>\n'

        return scale_bar


def main():
    """Главная функция"""
    parser = argparse.ArgumentParser(
        description='Генерация SVG карт сегментов береговой линии',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Примеры использования:
  python segment_svg_generator.py data/black-sea.json
  python segment_svg_generator.py data/black-sea.json --output segment_map.svg
  python segment_svg_generator.py data/black-sea.json --exposure-map
        '''
    )

    parser.add_argument('geojson_file', help='Путь к GeoJSON файлу с координатами береговой линии')
    parser.add_argument('--output', '-o', help='Путь для сохранения SVG файла')
    parser.add_argument('--width', type=int, default=1200, help='Ширина изображения (default: 1200)')
    parser.add_argument('--height', type=int, default=800, help='Высота изображения (default: 800)')
    parser.add_argument('--window-size', '-w', type=int, default=5,
                       help='Размер окна для анализа кривизны (default: 5)')
    parser.add_argument('--exposure-map', action='store_true',
                       help='Генерировать карту экспозиции вместо карты сегментов')
    parser.add_argument('--both', action='store_true',
                       help='Генерировать обе карты (сегментов и экспозиции)')

    args = parser.parse_args()

    try:
        # Создаем генератор
        generator = SVGSegmentGenerator(width=args.width, height=args.height)

        # Загружаем координаты
        coords = generator.load_geojson(args.geojson_file)
        if not coords:
            print("❌ Не удалось загрузить координаты из GeoJSON")
            return 1

        print(f"✓ Загружено {len(coords)} точек")

        # Классифицируем сегменты
        segments = generator.classify_segments(coords, args.window_size)
        print(f"✓ Классифицировано {len(segments)} сегментов")

        # Определяем выходной путь
        output_dir = Path('output/report')
        output_dir.mkdir(parents=True, exist_ok=True)

        if args.output:
            output_path = output_dir / Path(args.output).name
        else:
            base_name = Path(args.geojson_file).stem
            if args.exposure_map:
                output_path = output_dir / f'{base_name}_exposure.svg'
            else:
                output_path = output_dir / f'{base_name}_segments.svg'

        if args.both:
            # Генерируем обе карты
            segment_path = output_dir / f'{Path(args.geojson_file).stem}_segments.svg'
            exposure_path = output_dir / f'{Path(args.geojson_file).stem}_exposure.svg'

            result1 = generator.generate_segment_map(segments, segment_path)
            print(f"✓ Карта сегментов: {result1}")

            result2 = generator.generate_exposure_map(segments, exposure_path)
            print(f"✓ Карта экспозиции: {result2}")
        elif args.exposure_map:
            result = generator.generate_exposure_map(segments, output_path)
            print(f"✓ Карта экспозиции: {result}")
        else:
            result = generator.generate_segment_map(segments, output_path)
            print(f"✓ Карта сегментов: {result}")

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
