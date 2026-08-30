package cobra

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"coastal-geometry/internal/domain/geometry"
)

const (
	defaultLithologyPath = "data/black-sea-lithology.json"
)

type loadedModelInputs struct {
	BathymetryPath         string
	BathymetryGrid         *geometry.BathymetryGrid
	BathymetryPassportPath string
	BathymetryPassport     *geometry.BathymetryPassport
	BathymetrySHA256       string
	BathymetryStatus       string
	LithologyPath          string
	LithologyProfile       *geometry.LithologyProfile
	LithologyEnabled       bool
	Warnings               []string
}

func temporalParametersRequested(targetYears int, yearsPerStep, stormProbability, stormIntensity, seaLevelRise float64, seasonality bool, seasonalPhase float64) bool {
	return targetYears > 0 || yearsPerStep != 1 || stormProbability != 0 || stormIntensity != 2 || seaLevelRise != 0 || seasonality || seasonalPhase != 0
}

// loadModelInputs загружает дополнительные входные данные модели и возвращает
// их в форме, пригодной для WaveErosionOptions.
func loadModelInputs(bathymetryPath, lithologyPath string, enableLithology bool, bathymetryResolution float64) (loadedModelInputs, error) {
	inputs := loadedModelInputs{LithologyEnabled: enableLithology}

	if bathymetryPath != "" {
		data, err := os.ReadFile(bathymetryPath)
		if err != nil {
			return loadedModelInputs{}, fmt.Errorf("чтение батиметрии %q: %w", bathymetryPath, err)
		}

		passportPath := geometry.BathymetryPassportPath(bathymetryPath)
		passport, passportErr := geometry.LoadBathymetryPassportFromFile(passportPath)
		if passportErr != nil {
			if errors.Is(passportErr, os.ErrNotExist) {
				inputs.Warnings = append(inputs.Warnings, fmt.Sprintf("для батиметрии %q не найден паспорт %q", bathymetryPath, passportPath))
			} else {
				return loadedModelInputs{}, passportErr
			}
		} else {
			if err := passport.VerifyDataset(data); err != nil {
				return loadedModelInputs{}, fmt.Errorf("проверка батиметрии %q: %w", bathymetryPath, err)
			}
			if passport.DatasetFile != filepath.Base(bathymetryPath) {
				return loadedModelInputs{}, fmt.Errorf("паспорт %q относится к файлу %q, а загружается %q", passportPath, passport.DatasetFile, filepath.Base(bathymetryPath))
			}
			if bathymetryResolution > 0 && math.Abs(bathymetryResolution-passport.TargetResolutionDegrees) > 1e-12 {
				return loadedModelInputs{}, fmt.Errorf("шаг батиметрии %.12g° не совпадает с паспортом %.12g°", bathymetryResolution, passport.TargetResolutionDegrees)
			}
			if bathymetryResolution <= 0 {
				bathymetryResolution = passport.TargetResolutionDegrees
			}
			inputs.BathymetryPassportPath = passportPath
			inputs.BathymetryPassport = passport
			inputs.BathymetrySHA256 = passport.DatasetSHA256
			inputs.BathymetryStatus = passport.Status
			inputs.Warnings = append(inputs.Warnings, passport.ReproducibilityWarnings()...)
		}
		if inputs.BathymetrySHA256 == "" {
			inputs.BathymetrySHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
			inputs.BathymetryStatus = "passport_missing"
		}

		grid, err := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{Resolution: bathymetryResolution})
		if err != nil {
			return loadedModelInputs{}, fmt.Errorf("загрузка батиметрии %q: %w", bathymetryPath, err)
		}
		if passport != nil && passport.PointCount != len(grid.Points) {
			return loadedModelInputs{}, fmt.Errorf("паспорт батиметрии %q указывает %d точек, загружено %d", passportPath, passport.PointCount, len(grid.Points))
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
		inputs.Warnings = append(inputs.Warnings, profile.QualityWarnings()...)
	}

	return inputs, nil
}

// printModelInputWarnings выводит ограничения происхождения входных данных.
func printModelInputWarnings(inputs loadedModelInputs) {
	for _, warning := range inputs.Warnings {
		fmt.Printf("Предупреждение о входных данных: %s\n", warning)
	}
}
