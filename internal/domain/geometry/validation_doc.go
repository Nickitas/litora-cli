package geometry

// Этот файл содержит документацию по использованию метрик качества модели

/*
# Метрики качества модели для научной валидации

## Обзор

Пакет предоставляет метрики качества модели для валидации результатов моделирования
береговой эрозии. Метрики позволяют оценить научную достоверность и стабильность модели.

## Основные метрики

### ModelQualityMetrics

Основная структура, содержащая все метрики качества модели:

- DimensionStability: Стабильность фрактальной размерности D во времени [0-1]
  * 1.0 = идеальная стабильность (размерность не меняется)
  * 0.0 = низкая стабильность (размерность сильно вариативна)
  * Рассчитывается как экспоненциальное затухание от коэффициента вариации

- MassBalance: Баланс массы (eroded - deposited) в абсолютных значениях
  * 0.0 = идеальный баланс массы
  * > 0.15 = плохой баланс (допуск 15%)
  * Основан на SedimentBudget из transport наносов

- SpatialAutocorr: Пространственная автокорреляция соседних участков [-1, 1]
  * 1.0 = идеальная положительная корреляция (кластеризация)
  * 0.0 = отсутствие корреляции (случайные значения)
  * -1.0 = идеальная отрицательная корреляция (чересполосица)
  * Рассчитывается через лаг-1 автокорреляцию

- ConvergenceRate: Скорость изменения метрик (сходимость) [0-1]
  * 1.0 = полная сходимость (изменения замедляются)
  * 0.5 = умеренная сходимость
  * 0.0 = отсутствие сходимости (изменения ускоряются)

## Использование

### Базовое использование

```go
// Получаем метрики эрозии из временного моделирования
temporalResult := SimulateErosionWithDuration(points, targetYears, params, options)
erosionMetrics := CalculateErosionMetrics(temporalResult)

// Рассчитываем транспорт наносов
sedimentResult := CalculateSedimentTransport(points, erosionRates, waveData, lithology, params)

// Вычисляем метрики качества модели
qualityMetrics := CalculateModelQualityMetrics(erosionMetrics, sedimentResult)

// Проверяем валидность модели
if qualityMetrics.IsValidModel {
    fmt.Println("Model is scientifically valid")
} else {
    fmt.Println("Model validation failed:")
    for _, warning := range qualityMetrics.Warnings {
        fmt.Println(" -", warning)
    }
}

// Получаем сводку метрик
summary := GetQualityMetricsSummary(qualityMetrics)
```

### Временной анализ

```go
// Рассчитываем временные ряды метрик
timeSeries := CalculateTimeSeriesMetrics(erosionMetrics)

// Проверяем сходимость модели
isConvergent, warnings := ValidateModelConvergence(timeSeries)
if isConvergent {
    fmt.Println("Model converges successfully")
} else {
    fmt.Println("Model shows divergence:")
    for _, warning := range warnings {
        fmt.Println(" -", warning)
    }
}
```

## Критерии валидности

Модель считается валидной, если выполняются все условия:

1. DimensionStability > 0.7
   - Размерность береговой линии не должна сильно меняться
   - Допускаются малые вариации около стабильного значения

2. |MassBalance| < 0.15
   - Баланс массы должен сохраняться
   - Допуск 15% от эродированного объёма

3. -0.3 ≤ SpatialAutocorr ≤ 0.8
   - Пространственная корреляция в разумных пределах
   - Избегаем экстремальной кластеризации или чересполосицы

4. ConvergenceRate > 0.5
   - Модель должна сходиться (изменения замедляются)
   - Экспоненциальное затухание изменений

## Предупреждения

Система генерирует предупреждения для следующих проблем:

- "Low dimension stability" - размерность вариативна
- "Poor mass balance" - нарушение баланса массы
- "Negative/High spatial autocorrelation" - аномальные пространственные паттерны
- "Low convergence rate" - модель не сходится
- "High dimension variance" - нестабильная геометрия
- "Strong mass balance trend" - дрейф в балансе массы

## Интеграция с симуляцией

Метрики рассчитываются на каждом шаге симуляции:

```go
// Пошаговая симуляция с метриками
result := SimulateErosionWithDuration(points, years, temporalParams, options)

// Метрики для каждого шага
metrics := CalculateErosionMetrics(result)

// Финальная оценка качества
sedimentResult := CalculateSedimentTransport(...)
quality := CalculateModelQualityMetrics(metrics, sedimentResult)

// Вывод в отчёт
fmt.Printf("Dimension Stability: %.2f\n", quality.DimensionStability)
fmt.Printf("Mass Balance: %.4f\n", quality.MassBalance)
fmt.Printf("Spatial Autocorrelation: %.2f\n", quality.SpatialAutocorr)
fmt.Printf("Convergence Rate: %.2f\n", quality.ConvergenceRate)
```

## Научная интерпретация

### DimensionStability

Фрактальная размерность D характеризует сложность береговой линии:
- D ≈ 1.0: гладкая линия (минимальная сложность)
- D ≈ 1.3: умеренная извилистость
- D ≈ 1.5: высокая сложность (фрактальный берег)
- D → 2.0: предельно заполненная плоскость

Стабильность D во времени указывает на устойчивость динамики.

### MassBalance

Баланс массы - фундаментальный закон сохранения:
- Eroded = Deposited + Transport
- Нарушение указывает на ошибки в алгоритме
- Учитывается литология и скорость транспорта

### SpatialAutocorr

Пространственная автокорреляция выявляет паттерны:
- Высокая положительная: кластеризация (однородные участки)
- Близкая к нулю: случайность (шум)
- Отрицательная: чередование (structured pattern)

### ConvergenceRate

Сходимость указывает на стабильность решения:
- Высокая: решение stabilises
- Низкая: решение diverges или oscillates
- Используется для определения времени моделирования

## References

- Mandelbrot, B.B. (1982): The Fractal Geometry of Nature
- CERC (1984): Shore Protection Manual
- Komar, P.D. (1998): Beach Processes and Sedimentation

## Performance

- O(n) для расчёта базовых метрик
- O(n²) для пространственной корреляции (n = число точек)
- O(n) для сходимости (n = число шагов)
- Рекомендуется: n < 10,000 точек для interactive performance
*/
