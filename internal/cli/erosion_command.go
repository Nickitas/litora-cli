package cli

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
	"os"
)

const (
	defaultBathymetryFile = "data/black-sea-bathymetry.json"
	defaultLithologyFile = "data/black-sea-lithology.json"
)

func runErosionCommand(app *App) error {
	steps := app.Config.Steps
	strength := app.Config.ErosionStrength
	seed := app.Config.Seed
	// Для волновой эрозии используем дефолтное значение, если не задано
	if strength == 0 && app.Config.Command == "all" {
		strength = 30.0 // Дефолтное значение для команды all
	}

	// Проверка на использование временной динамики
	useTemporalDynamics := app.Config.TargetYears > 0 && app.Config.YearsPerStep > 0

	var temporalParams geometry.TemporalParameters
	if useTemporalDynamics {
		temporalParams = geometry.TemporalParameters{
			YearsPerStep:       app.Config.YearsPerStep,
			StormProbability:   app.Config.StormProbability,
			StormIntensityMult: app.Config.StormIntensityMult,
			SeaLevelRise:       app.Config.SeaLevelRise,
			Seasonality:        app.Config.EnableSeasonality,
			SeasonalPhase:      app.Config.SeasonalPhase,
		}

		// Валидация временных параметров
		warnings := geometry.ValidateTemporalParameters(temporalParams)
		if len(warnings) > 0 {
			fmt.Println("  ⚠ Предупреждения временной динамики:")
			for _, warning := range warnings {
				fmt.Printf("    • %s\n", warning)
			}
			fmt.Println()
		}

		// Подавление предупреждений если включена seasonality
		if app.Config.EnableSeasonality {
			fmt.Println("✓ Сезонность включена: эрозия варьируется по сезонам")
		}
		if app.Config.StormProbability > 0 {
			fmt.Printf("✓ Штормовые события: вероятность %.2f за шаг, интенсивность %.1fx\n",
				app.Config.StormProbability, app.Config.StormIntensityMult)
		}
		if app.Config.SeaLevelRise > 0 {
			fmt.Printf("✓ Подъём уровня моря: %.4f м/год\n", app.Config.SeaLevelRise)
		}
		fmt.Printf("✓ Временная шкала: %.1f лет за шаг, цель %d лет\n",
			app.Config.YearsPerStep, app.Config.TargetYears)
		fmt.Println()
	}

	// Автоматическая загрузка батиметрии
	bathymetryPath := app.Config.BathymetryPath
	if bathymetryPath == "" {
		// Проверяем файл по умолчанию
		if _, err := os.Stat(defaultBathymetryFile); err == nil {
			bathymetryPath = defaultBathymetryFile
			fmt.Printf("✓ Батиметрия загружена: %s\n", defaultBathymetryFile)
		} else {
			// Файл не найден - предлагаем скачать
			fmt.Printf("⚠️  Батиметрия не найдена: %s\n", defaultBathymetryFile)
			fmt.Println("Для загрузки выполните:")
			fmt.Println("  make bathymetry")
			fmt.Println("  # или напрямую:")
			fmt.Println("  go run cmd/download-bathymetry/main.go")
			fmt.Println("Используем геометрический proxy...\n")
			bathymetryPath = ""
		}
	}

	var bathymetryGrid *geometry.BathymetryGrid
	if bathymetryPath != "" {
		data, err := os.ReadFile(bathymetryPath)
		if err != nil {
			return fmt.Errorf("ошибка чтения файла батиметрии %q: %w", bathymetryPath, err)
		}
		bathymetryGrid, err = geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{
			Resolution: 0.01,
		})
		if err != nil {
			return fmt.Errorf("ошибка загрузки батиметрии из %q: %w", bathymetryPath, err)
		}
	}

	// Загрузка литологии
	lithologyPath := app.Config.LithologyPath
	enableLithology := app.Config.EnableLithology

	var lithologyProfile *geometry.LithologyProfile
	if lithologyPath != "" || enableLithology {
		if lithologyPath == "" {
			lithologyPath = defaultLithologyFile
		}
		data, err := os.ReadFile(lithologyPath)
		if err != nil {
			if enableLithology {
				return fmt.Errorf("ошибка чтения файла литологии %q: %w", lithologyPath, err)
			}
			lithologyProfile = geometry.CreateDefaultBlackSeaProfile()
		} else {
			lithologyProfile, err = geometry.LoadLithologyProfile(data)
			if err != nil {
				if enableLithology {
					return fmt.Errorf("ошибка загрузки литологии из %q: %w", lithologyPath, err)
				}
				lithologyProfile = geometry.CreateDefaultBlackSeaProfile()
			}
		}
	}

	// Если включена литология но профиль не был загружен явно
	if enableLithology && lithologyProfile == nil {
		lithologyProfile = geometry.CreateDefaultBlackSeaProfile()
	}

	waveOptions := geometry.WaveErosionOptions{
		StrengthMeters:           strength,
		WindSourceDirectionDeg:   app.Config.WaveDirection,
		WindSpeedMetersPerSecond: app.Config.WindSpeed,
		FetchSpreadDeg:           app.Config.FetchSpread,
		FetchSamples:             app.Config.FetchSamples,
		MaxFetchMeters:           app.Config.MaxFetchKM * 1000,
		DepthScaleMeters:         app.Config.DepthScale,
		ExposurePower:            app.Config.ExposurePower,
		BathymetryGrid:           bathymetryGrid,
		LithologyProfile:         lithologyProfile,
		EnableLithology:          enableLithology,
	}

	if enableLithology && lithologyProfile != nil {
		fmt.Println("✓ Литология включена: эрозия модулируется по сопротивлению пород")
	}

	var snapshots [][]geometry.LatLon
	var temporalResult geometry.TemporalResult

	if useTemporalDynamics {
		// Используем временную динамику
		temporalResult = geometry.SimulateErosionWithDurationSeed(
			app.ModelBase,
			app.Config.TargetYears,
			temporalParams,
			waveOptions,
			seed,
		)
		snapshots = temporalResult.Snapshots

		fmt.Println("\n  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("  ВОЛНОВАЯ ЭРОЗИЯ С ВРЕМЕННОЙ ДИНАМИКОЙ")
		fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		fmt.Println("  ┌──────┬──────────┬───────────┬───────────┬─────────────┐")
		fmt.Println("  │ Шаг  │ Год      │ Точек     │ Длина км  │ Площадь км² │")
		fmt.Println("  ├──────┼──────────┼───────────┼───────────┼─────────────┤")

		for i, state := range snapshots {
			year := temporalResult.TemporalStates[i].Year
			length := geometry.PolylineLength(state)
			area := geometry.Area(state)

			// Добавляем индикаторы шторма и сезонности
			stormIndicator := ""
			if temporalResult.TemporalStates[i].IsStorm {
				stormIndicator = "⛈️ "
			}

			fmt.Printf("  │ %-4d │ %-8.0f │ %-9d │ %-9.0f │ %-11.0f │%s\n",
				i, year, len(state), length, area, stormIndicator)
		}

		fmt.Println("  └──────┴──────────┴───────────┴───────────┴─────────────┘")
		fmt.Println()

		// Вывод сводки временной динамики
		summary := geometry.GetTemporalSummary(temporalResult)
		fmt.Println("  📊 Статистика временной динамики:")
		fmt.Printf("     • Промоделировано лет: %.1f из %d (целевых)\n", summary["total_years"], app.Config.TargetYears)
		fmt.Printf("     • Шагов моделирования: %d\n", summary["total_steps"])
		fmt.Printf("     • Штормовых событий: %d (частота %.2f)\n",
			summary["storm_count"], summary["storm_frequency"])

		if summary["sea_level_rise_m"].(float64) > 0 {
			fmt.Printf("     • Подъём уровня моря: %.2f м\n", summary["sea_level_rise_m"])
		}

		if summary["accumulated_erosion_m"].(float64) > 0 {
			fmt.Printf("     • Накопленная эрозия: %.1f м\n", summary["accumulated_erosion_m"])
		}

		fmt.Printf("     • Изменение длины берега: %.1f км (%.1f%%)\n",
			summary["length_change_km"], summary["length_change_percent"])
		fmt.Println()

	} else {
		// Обычная эрозия без временной динамики
		snapshots = geometry.SimulateWaveErosionWithSeed(app.ModelBase, steps, waveOptions, seed)

		fmt.Println("\n  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("  ВОЛНОВАЯ ЭРОЗИЯ")
		fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		fmt.Println("  ┌──────┬───────────┬───────────┬─────────────┐")
		fmt.Println("  │ Шаг  │ Точек     │ Длина км  │ Площадь км² │")
		fmt.Println("  ├──────┼───────────┼───────────┼─────────────┤")

		for i, state := range snapshots {
			length := geometry.PolylineLength(state)
			area := geometry.Area(state)
			fmt.Printf("  │ %-4d │ %-9d │ %-9.0f │ %-11.0f │\n", i, len(state), length, area)
		}

		fmt.Println("  └──────┴───────────┴───────────┴─────────────┘")
		fmt.Println()
	}

	// Export CSV if requested
	if app.Config.OutputCSV != "" {
		var temporalResultPtr *geometry.TemporalResult
		if useTemporalDynamics {
			temporalResultPtr = &temporalResult
		}

		csvPath := app.OutputPaths.ResolveUserPath(app.Config.OutputCSV, "csv")
		fmt.Printf("  📄 Экспорт CSV метрик: %s\n", csvPath)
		if err := writeErosionCSV(snapshots, temporalResultPtr, app.Config.OutputCSV, app.Config.CSVFormat, app.OutputPaths); err != nil {
			fmt.Printf("  ⚠️  Ошибка экспорта CSV: %v\n", err)
		} else {
			fmt.Printf("  ✓ CSV успешно экспортирован (формат: %s)\n", app.Config.CSVFormat)
		}
		fmt.Println()
	}

	return writeErosionSVGSeries(app.Base, app.ModelBase, snapshots, steps, strength, seed, waveOptions, app.Config.OutputPath, newExportContext(app), app.OutputPaths)
}
