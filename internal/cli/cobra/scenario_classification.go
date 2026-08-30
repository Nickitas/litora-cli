package cobra

import (
	"fmt"

	"coastal-geometry/internal/domain/geometry"
)

// classifyLongshoreScenario формирует явный статус интерпретации расчёта.
// Автоматический набор Сочи остаётся демонстрационным независимо от того,
// завершилась ли численная модель без ошибок.
func classifyLongshoreScenario(sochiDemo bool, climate geometry.WaveClimate, inputs loadedModelInputs) geometry.ScenarioClassification {
	if !sochiDemo {
		return geometry.ScenarioClassification{
			ScenarioStatus: geometry.ScenarioStatusUnclassified,
			UsageLimitations: []string{
				"Lito не подтверждает исследовательскую пригодность пользовательского набора автоматически.",
			},
		}
	}

	totalHours := 0.0
	for _, condition := range climate.Conditions {
		totalHours += condition.DurationHours
	}
	bathymetryPoints := 0
	if inputs.BathymetryGrid != nil {
		bathymetryPoints = len(inputs.BathymetryGrid.Points)
	}
	bathymetryStatus := inputs.BathymetryStatus
	if bathymetryStatus == "" {
		bathymetryStatus = "не указан"
	}

	return geometry.ScenarioClassification{
		ScenarioStatus: geometry.ScenarioStatusDemo,
		UsageLimitations: []string{
			"Использован оперативный прогноз Open-Meteo Marine, а не архив наблюдений или реанализ.",
			fmt.Sprintf("Расчёт охватывает %.0f ч (%d состояний волн), поэтому не оценивает годовой размыв.", totalHours, len(climate.Conditions)),
			fmt.Sprintf("Батиметрия содержит %d точек; статус паспорта: %s.", bathymetryPoints, bathymetryStatus),
			"Результат нельзя использовать для калибровки, проектного решения или научной публикации.",
		},
	}
}

// printScenarioClassification выводит статус отдельно от сообщения об
// успешном завершении численного расчёта.
func printScenarioClassification(classification geometry.ScenarioClassification) {
	fmt.Printf("Статус сценария: %s\n", classification.ScenarioStatus)
	for _, limitation := range classification.UsageLimitations {
		fmt.Printf("Ограничение: %s\n", limitation)
	}
}
