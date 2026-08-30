"""Совместимость имён колонок CSV между версиями экспорта Lito."""

import pandas as pd


ALIASES = {
    "год": "year", "шаг": "step", "длина_км": "length_km",
    "площадь_км2": "area_km2", "эродировано_м3": "eroded_m3",
    "отложено_м3": "deposited_m3", "чистое_изменение_м3": "net_change_m3",
    "штормовое_событие": "storm_event", "уровень_моря_м": "sea_level_m",
}


def read_erosion_csv(path):
    """Читает CSV Lito и приводит русскую схему к внутренним именам скриптов."""
    frame = pd.read_csv(path)
    frame = frame.rename(columns={key: value for key, value in ALIASES.items()
                                  if key in frame.columns})
    if "storm_event" in frame.columns:
        frame["storm_event"] = (frame["storm_event"].astype(str).str.lower()
                                 .map({"истина": True, "правда": True, "true": True,
                                       "1": True, "ложь": False, "false": False,
                                       "0": False}).fillna(False))
    return frame
