package cli

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"coastal-geometry/internal/domain/geometry"
)

// WriteErosionCSV экспортирует метрики эрозии в CSV через менеджер выходных путей.
func WriteErosionCSV(
	snapshots [][]geometry.LatLon,
	temporalResult *geometry.TemporalResult,
	outputPath string,
	format string,
	outputPathManager *OutputPathManager,
) error {
	if outputPathManager == nil {
		return fmt.Errorf("менеджер выходных путей не задан")
	}
	if err := outputPathManager.EnsureDirectories(); err != nil {
		return err
	}
	return writeErosionCSV(snapshots, temporalResult, outputPath, format, outputPathManager)
}

// writeErosionCSV экспортирует метрики эрозии в формат CSV
// Поддерживает два формата: "long" (одна строка на шаг) и "wide" (одна строка с колонками шагов)
func writeErosionCSV(
	snapshots [][]geometry.LatLon,
	temporalResult *geometry.TemporalResult,
	outputPath string,
	format string,
	outputPathManager *OutputPathManager,
) error {
	if outputPath == "" {
		return fmt.Errorf("путь к выходному CSV файлу не может быть пустым")
	}

	// Разрешаем путь вывода с помощью OutputPathManager
	resolvedPath := outputPathManager.ResolveUserPath(outputPath, "csv")
	if resolvedPath == "" {
		resolvedPath = outputPathManager.CSVPath(outputPath)
	}

	file, err := os.Create(resolvedPath)
	if err != nil {
		return fmt.Errorf("создать CSV файл %q: %w", resolvedPath, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	switch format {
	case "long":
		return writeLongFormatCSV(writer, snapshots, temporalResult)
	case "wide":
		return writeWideFormatCSV(writer, snapshots, temporalResult)
	default:
		return fmt.Errorf("неподдерживаемый формат CSV: %s", format)
	}
}

// writeLongFormatCSV создаёт CSV с одной строкой на шаг
// Колонки: year,step,length_km,area_km2,eroded_m3,deposited_m3,net_change_m3,storm_event,sea_level_m
func writeLongFormatCSV(
	writer *csv.Writer,
	snapshots [][]geometry.LatLon,
	temporalResult *geometry.TemporalResult,
) error {
	// Записываем заголовок
	header := []string{
		"год", "шаг", "длина_км", "площадь_км2",
		"эродировано_м3", "отложено_м3", "чистое_изменение_м3",
		"штормовое_событие", "уровень_моря_м",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("запись заголовка CSV: %w", err)
	}

	// Записываем строки данных
	for i, snapshot := range snapshots {
		// Получаем временное состояние если доступно
		var state geometry.TemporalState
		var hasTemporalState bool
		if temporalResult != nil && i < len(temporalResult.TemporalStates) {
			state = temporalResult.TemporalStates[i]
			hasTemporalState = true
		}

		// Вычисляем метрики
		lengthKm := geometry.PolylineLength(snapshot)
		areaKm2 := geometry.Area(snapshot)

		// Вычисляем объёмы эрозии/аккумуляции (упрощённо)
		var erodedM3, depositedM3, netChangeM3 float64
		if i > 0 && len(snapshots[i-1]) > 0 && len(snapshot) > 0 {
			// Простая оценка на основе изменения длины
			prevLength := geometry.PolylineLength(snapshots[i-1])
			lengthChange := prevLength - lengthKm

			// Конвертируем в объём (очень грубое приближение)
			// Предполагаем средний отступ береговой линии на 1м глубины
			erodedM3 = lengthChange * 1000 * 1 // км в м, предполагая 1м глубины

			// Для простоты предполагаем отсутствие аккумуляции в этой модели
			depositedM3 = 0
			netChangeM3 = erodedM3 - depositedM3
		}

		// Индикатор штормового события
		stormEvent := "ложь"
		if hasTemporalState && state.IsStorm {
			stormEvent = "истина"
		}

		// Уровень моря
		seaLevelM := 0.0
		if hasTemporalState && state.SeaLevelOffset > 0 {
			seaLevelM = state.SeaLevelOffset
		}

		// Получаем год
		year := 0.0
		if hasTemporalState {
			year = state.Year
		}

		// Записываем строку
		row := []string{
			fmt.Sprintf("%.1f", year),
			strconv.Itoa(i),
			fmt.Sprintf("%.1f", lengthKm),
			fmt.Sprintf("%.1f", areaKm2),
			fmt.Sprintf("%.1f", erodedM3),
			fmt.Sprintf("%.1f", depositedM3),
			fmt.Sprintf("%.1f", netChangeM3),
			stormEvent,
			fmt.Sprintf("%.4f", seaLevelM),
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("запись строки CSV %d: %w", i, err)
		}
	}

	return nil
}

// writeWideFormatCSV создаёт CSV с одной строкой и колонками для каждого шага
func writeWideFormatCSV(
	writer *csv.Writer,
	snapshots [][]geometry.LatLon,
	temporalResult *geometry.TemporalResult,
) error {
	// Определяем количество колонок метрик на шаг
	numSteps := len(snapshots)

	// Формируем заголовок: metric_name,step_0,step_1,...,step_N
	header := []string{"метрика"}
	for i := 0; i < numSteps; i++ {
		header = append(header, fmt.Sprintf("шаг_%d", i))
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("запись заголовка CSV: %w", err)
	}

	// Подготавливаем массивы данных
	years := make([]string, numSteps)
	lengths := make([]string, numSteps)
	areas := make([]string, numSteps)
	storms := make([]string, numSteps)
	seaLevels := make([]string, numSteps)

	// Извлекаем данные для каждого шага
	for i, snapshot := range snapshots {
		// Получаем временное состояние если доступно
		var state geometry.TemporalState
		var hasTemporalState bool
		if temporalResult != nil && i < len(temporalResult.TemporalStates) {
			state = temporalResult.TemporalStates[i]
			hasTemporalState = true
		}

		// Год
		if hasTemporalState {
			years[i] = fmt.Sprintf("%.1f", state.Year)
		} else {
			years[i] = fmt.Sprintf("%d", i)
		}

		// Длина и площадь
		lengths[i] = fmt.Sprintf("%.1f", geometry.PolylineLength(snapshot))
		areas[i] = fmt.Sprintf("%.1f", geometry.Area(snapshot))

		// Индикатор шторма
		if hasTemporalState && state.IsStorm {
			storms[i] = "истина"
		} else {
			storms[i] = "ложь"
		}

		// Уровень моря
		if hasTemporalState && state.SeaLevelOffset > 0 {
			seaLevels[i] = fmt.Sprintf("%.4f", state.SeaLevelOffset)
		} else {
			seaLevels[i] = "0.0000"
		}
	}

	// Записываем строки для каждой метрики
	metrics := []struct {
		name   string
		values []string
	}{
		{"год", years},
		{"длина_км", lengths},
		{"площадь_км2", areas},
		{"штормовое_событие", storms},
		{"уровень_моря_м", seaLevels},
	}

	for _, metric := range metrics {
		row := append([]string{metric.name}, metric.values...)
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("запись метрики CSV %s: %w", metric.name, err)
		}
	}

	return nil
}
