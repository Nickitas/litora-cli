# Временные метки (Time Stamps) для GIF анимации ✅

## Обзор

Реализованы временные метки для GIF анимации с поддержкой временной динамики. Теперь на кадрах отображается год, индикаторы штормов и климатические параметры.

## Что добавлено

### 📅 Временные метки на кадрах
- **Year: X** - текущий год моделирования
- **⛈️ Storm** - индикатор штормовых событий
- **SLR: X m/yr** - подъём уровня моря (если включен)
- **Сезонные множители** - для сезонной динамики

### 🎛️ Новые CLI флаги
- `--gif-show-timestamp` - показывать временные метки на кадрах (по умолчанию: true)

## Использование

### С временной динамикой
```bash
./lito model erosion \
  --steps 8 \
  --target-years 20 \
  --years-per-step 2.5 \
  --storm-probability 0.3 \
  --sea-level-rise 0.005 \
  --output-gif temporal.gif \
  --gif-show-timestamp
```

### С сезонностью
```bash
./lito model erosion \
  --steps 6 \
  --target-years 15 \
  --years-per-step 2.5 \
  --enable-seasonality \
  --seasonal-phase 1.5 \
  --output-gif seasonal.gif \
  --gif-show-timestamp
```

### Комплексный пример с всеми функциями
```bash
./lito model erosion \
  --steps 10 \
  --target-years 25 \
  --years-per-step 2.5 \
  --storm-probability 0.25 \
  --sea-level-rise 0.003 \
  --enable-seasonality \
  --output-gif complete.gif \
  --gif-show-timestamp \
  --gif-geo-labels all \
  --gif-colorlegend-pos right \
  --gif-scalebar-km 50
```

### Отключить временные метки
```bash
./lito model erosion \
  --steps 5 \
  --output-gif no_timestamp.gif \
  --gif-show-timestamp=false
```

## Данные TemporalState

Временные метки используют структуру `TemporalState` из пакета geometry:

```go
type TemporalState struct {
    Step            int       // номер шага
    Year            float64   // текущий год
    IsStorm         bool      // штормовое событие
    StormIntensity  float64   // интенсивность шторма [1.0+]
    SeasonalFactor  float64   // сезонный множитель [0.5-1.5]
    SeaLevelOffset  float64   // смещение уровня моря (м)
    EffectiveYears  float64   // эффективное число лет для шага
}
```

## Технические детали

### Интеграция с временной динамикой
- Временные метки показываются только когда включена временная динамика (`target-years > 0`)
- Данные берутся из `TemporalResult.TemporalStates`
- Каждый кадр получает соответствующее состояние по индексу

### Позиционирование меток
- Левый верхний угол изображения
- Позиция: (10px, 10px) с отступами
- Межстрочный интервал: 14px

### Цветовая кодировка
- Год и текст: белый (цвет 13)
- Шторм: красный (цвет 10)
- Параметры: белый (цвет 13)

## Преимущества для научных публикаций

1. **Временная привязка:** чётко видно, какой год моделирования показывается
2. **Индикация событий:** штормы, климатические изменения визуально выделены
3. **Научная строгость:** параметры моделирования прозрачны и воспроизводимы
4. **Комплексный анализ:** совмещение пространственной и временной динамики

## Статус реализации

✅ **РЕАЛИЗОВАНО** - Июнь 2026
- Временные метки с годом
- Индикаторы штормовых событий ⛈️
- Отображение подъёма уровня моря
- Интеграция с сезонной динамикой
- Полная совместимость с существующими функциями GIF

## Пример для научной статьи

```bash
# GIF с временными метками для статьи
./lito model erosion \
  --steps 12 \
  --target-years 30 \
  --years-per-step 2.5 \
  --storm-probability 0.2 \
  --sea-level-rise 0.004 \
  --enable-seasonality \
  --output-gif figure5_temporal_dynamics.gif \
  --gif-show-timestamp \
  --gif-geo-labels major \
  --gif-colorlegend-pos bottom \
  --gif-scalebar-km 100 \
  --output-csv table2_temporal_states.csv
```

Результаты:
- **figure5_temporal_dynamics.gif** - анимация с временными метками
- **table2_temporal_states.csv** - количественные данные временных состояний
- Идеально для Figure + Table в научной статье

Временные метки значительно улучшают научную ценность GIF анимации! 🎓⏱️
