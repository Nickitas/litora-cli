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

	// Автоматическая загрузка батиметрии
	bathymetryPath := app.Config.BathymetryPath
	if bathymetryPath == "" {
		// Проверяем файл по умолчанию
		if _, err := os.Stat(defaultBathymetryFile); err == nil {
			bathymetryPath = defaultBathymetryFile
		} else {
			// Файл не найден - используем геометрический proxy
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

	snapshots := geometry.SimulateWaveErosionWithSeed(app.ModelBase, steps, waveOptions, seed)

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


	return writeErosionSVGSeries(app.Base, app.ModelBase, snapshots, steps, strength, seed, waveOptions, app.Config.OutputPath, newExportContext(app))
}
