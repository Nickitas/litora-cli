package cobra

import (
	"fmt"
	"os"

	"coastal-geometry/internal/domain/geometry"
)

const (
	defaultBathymetryPath = "data/black-sea-bathymetry.json"
	defaultLithologyPath  = "data/black-sea-lithology.json"
)

type loadedModelInputs struct {
	BathymetryPath   string
	BathymetryGrid   *geometry.BathymetryGrid
	LithologyPath    string
	LithologyProfile *geometry.LithologyProfile
	LithologyEnabled bool
}

func temporalParametersRequested(targetYears int, yearsPerStep, stormProbability, stormIntensity, seaLevelRise float64, seasonality bool, seasonalPhase float64) bool {
	return targetYears > 0 || yearsPerStep != 1 || stormProbability != 0 || stormIntensity != 2 || seaLevelRise != 0 || seasonality || seasonalPhase != 0
}

// loadModelInputs загружает дополнительные входные данные модели и возвращает
// их в форме, пригодной для WaveErosionOptions.
func loadModelInputs(bathymetryPath, lithologyPath string, enableLithology bool) (loadedModelInputs, error) {
	inputs := loadedModelInputs{LithologyEnabled: enableLithology}

	if bathymetryPath == "" {
		if _, err := os.Stat(defaultBathymetryPath); err == nil {
			bathymetryPath = defaultBathymetryPath
		}
	}
	if bathymetryPath != "" {
		data, err := os.ReadFile(bathymetryPath)
		if err != nil {
			return loadedModelInputs{}, fmt.Errorf("чтение батиметрии %q: %w", bathymetryPath, err)
		}
		grid, err := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{Resolution: 0.01})
		if err != nil {
			return loadedModelInputs{}, fmt.Errorf("загрузка батиметрии %q: %w", bathymetryPath, err)
		}
		inputs.BathymetryPath = bathymetryPath
		inputs.BathymetryGrid = grid
	}

	if lithologyPath != "" {
		inputs.LithologyEnabled = true
	} else if enableLithology {
		if _, err := os.Stat(defaultLithologyPath); err == nil {
			lithologyPath = defaultLithologyPath
		} else {
			return loadedModelInputs{}, fmt.Errorf("профиль литологии не найден: %q", defaultLithologyPath)
		}
	}
	if lithologyPath != "" {
		profile, err := geometry.LoadLithologyProfileFromFile(lithologyPath)
		if err != nil {
			return loadedModelInputs{}, fmt.Errorf("загрузка литологии %q: %w", lithologyPath, err)
		}
		inputs.LithologyPath = lithologyPath
		inputs.LithologyProfile = profile
	}

	return inputs, nil
}
