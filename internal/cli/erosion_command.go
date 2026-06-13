package cli

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// Автоматическая загрузка батиметрии
	bathymetryPath := app.Config.BathymetryPath
	if bathymetryPath == "" {
		// Проверяем файл по умолчанию
		if _, err := os.Stat(defaultBathymetryFile); err == nil {
			bathymetryPath = defaultBathymetryFile
			fmt.Printf("\n📊 Используется батиметрия: %s\n", bathymetryPath)
		} else {
			// Файл не найден - предлагаем скачать
			fmt.Println("\n⚠️  Батиметрия не найдена")
			fmt.Println("Для лучшей точности рекомендуется использовать реальные данные.")
			fmt.Println("\nФайл можно получить:")
			fmt.Println("  1. Автоматически: go run cmd/download-bathymetry/main.go")
			fmt.Println("  2. Скриптом: bash scripts/download_bathymetry.sh")
			fmt.Println("  3. Вручную: см. scripts/BATHYMETRY_DATA.md")
			fmt.Println("\nИспользуется геометрический proxy (менее точно).")

			// Продолжаем без батиметрии
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

		absPath, _ := filepath.Abs(bathymetryPath)
		fmt.Printf("✓ Загружена батиметрия: %d точек, разрешение ~%.1f км\n",
			len(bathymetryGrid.Points), bathymetryGrid.Resolution*111)
		fmt.Printf("  Источник: %s\n", absPath)
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
			fmt.Printf("\n⚠️  Литология не найдена: %v (используется дефолтный профиль)\n", err)
			lithologyProfile = geometry.CreateDefaultBlackSeaProfile()
		} else {
			lithologyProfile, err = geometry.LoadLithologyProfile(data)
			if err != nil {
				if enableLithology {
					return fmt.Errorf("ошибка загрузки литологии из %q: %w", lithologyPath, err)
				}
				fmt.Printf("\n⚠️  Ошибка загрузки литологии, используется дефолтный профиль: %v\n", err)
				lithologyProfile = geometry.CreateDefaultBlackSeaProfile()
			} else {
				absPath, _ := filepath.Abs(lithologyPath)
				stats := lithologyProfile.GetStatistics()
				fmt.Printf("✓ Загружена литология: %s (%d точек, %d классов)\n",
					stats["name"], stats["num_points"], stats["num_classes"])
				fmt.Printf("  Источник: %s\n", absPath)
			}
		}
	}

	// Если включена литология но профиль не был загружен явно
	if enableLithology && lithologyProfile == nil {
		lithologyProfile = geometry.CreateDefaultBlackSeaProfile()
		fmt.Println("Используется дефолтный литологический профиль")
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

	snapshots := geometry.SimulateWaveErosionWithSeed(app.ModelBase, steps, waveOptions, seed)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("\tЭРОЗИЯ: ВОЛНОВАЯ СИМУЛЯЦИЯ")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Шаги=%d, базовый отступ=%.1f м, волны %.0f°, ветер %.1f м/с, fetch<=%.0f км, seed=%d\n\n",
		steps,
		strength,
		app.Config.WaveDirection,
		app.Config.WindSpeed,
		app.Config.MaxFetchKM,
		seed,
	)
	fmt.Printf("%-6s %-10s %-12s %-14s\n", "Шаг", "Точек", "Длина, км", "Площадь, км²")
	fmt.Println(strings.Repeat("-", 56))

	for i, state := range snapshots {
		length := geometry.PolylineLength(state)
		area := geometry.Area(state)
		fmt.Printf("%-6d %-10d %-12.0f %-14.0f\n", i, len(state), length, area)
	}

	return writeErosionSVGSeries(app.Base, app.ModelBase, snapshots, steps, strength, seed, waveOptions, app.Config.OutputPath, newExportContext(app))
}
