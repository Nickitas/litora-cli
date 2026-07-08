# Оптимизация Sediment Transport

## Обзор

В `sediment_optimized.go` реализованы три уровня оптимизации для расчёта транспорта наносов:

## 1. Кэширование вычислений

### `OptimizedSedimentCache`

Кэширует дорогостоящие векторные вычисления:
- Alongshore направления для каждой точки
- Wave direction вектор
- Длины сегментов между точками

**Выгода**: Избегает повторных вычислений тригонометрических функций.

## 2. Параллельная обработка

### Автоматический выбор стратегии

`CalculateSedimentTransportAuto` выбирает оптимальный подход:

| Размер набора | Стратегия | Причина |
|---------------|-----------|---------|
| < 500 точек | Original | Minimize overhead |
| 500-10K | Optimized (cache only) | Кэш выгоден, goroutines - нет |
| 10K-50K | Optimized (parallel) | Параллелизм даёт выгоду |
| > 50K | Batched | Memory efficiency |

### Параллельные функции

- `calculateErosionVolumesOptimized`: независимые вычисления по точкам
- `calculateLongshoreDriftOptimized`: с промежуточным буфером для избежания race conditions
- `calculateDepositionOptimized`: параллельная обработка аккумуляции

## 3. Пачечная обработка

### `CalculateSedimentTransportBatched`

Для очень больших линий (>50K точек):
- Разбивает на батчи (default: 2000-5000 точек)
- Обрабатывает каждый батч независимо
- Агрегирует результаты

**Выгода**: ~2x ускорение, lower memory pressure.

## Результаты бенчмарков

### 10K точек
```
Original:   745K ns/op, 16.9MB allocs
Optimized:  653K ns/op, 2.7MB allocs
Ускорение:  12% быстрее, 84% меньше памяти
```

### 50K точек
```
Optimized:  6.6M ns/op, 15.7MB allocs
Batched:    3.3M ns/op, 13.9MB allocs
Ускорение:  2x быстрее
```

## Использование

```go
// Автоматический выбор стратегии
result := CalculateSedimentTransportAuto(
    points,
    erosionRates,
    waveData,
    lithology,
    params,
)

// Принудительное использование стратегии
result := CalculateSedimentTransportOptimized(...)
result := CalculateSedimentTransportBatched(...)

// Получение статистики производительности
stats := GetPerformanceStats(len(points))
fmt.Printf("Strategy: %s, Speedup: %.1fx\n", stats.Strategy, stats.Speedup)
```

## Тесты

```bash
# Тесты корректности
go test ./internal/domain/geometry -run TestOptimized

# Бенчмарки
go test ./internal/domain/geometry -bench="^Benchmark" -benchmem
```

## Технические детали

### Thread-safe кэш

`OptimizedSedimentCache` использует `sync.RWMutex` для безопасного параллельного чтения.

### Memory pool

`calculateLongshoreDriftOptimized` использует `sync.Pool` для переиспользования буферов.

### Pre-allocation

Все слайцы выделяются с known capacity для минимизации аллокаций:

```go
states[i].InTransitFrom = make([]float64, 0, 4) // Pre-allocate
```

## Future improvements

1. GPU/offload для very large datasets
2. Incremental updates для изменяющихся coastline
3. Spatial indexing для regional calculations
