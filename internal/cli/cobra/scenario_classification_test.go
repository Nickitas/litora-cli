package cobra

import (
	"strings"
	"testing"

	"coastal-geometry/internal/domain/geometry"
)

func TestClassifyLongshoreScenarioMarksSochiAsDemo(t *testing.T) {
	conditions := make([]geometry.WaveCondition, 24)
	for index := range conditions {
		conditions[index].DurationHours = 1
	}
	grid := &geometry.BathymetryGrid{Points: make(map[string]geometry.BathymetryPoint, 7)}
	for index := 0; index < 7; index++ {
		grid.Points[string(rune('a'+index))] = geometry.BathymetryPoint{Depth: -10}
	}

	classification := classifyLongshoreScenario(true, geometry.WaveClimate{Conditions: conditions}, loadedModelInputs{
		BathymetryGrid:   grid,
		BathymetryStatus: "passport_missing",
	})
	if classification.ScenarioStatus != geometry.ScenarioStatusDemo {
		t.Fatalf("статус сценария Сочи = %q, требуется demo", classification.ScenarioStatus)
	}
	joined := strings.Join(classification.UsageLimitations, " ")
	for _, fragment := range []string{"Open-Meteo", "24 ч", "7 точек", "passport_missing", "публикации"} {
		if !strings.Contains(joined, fragment) {
			t.Errorf("в ограничениях отсутствует %q: %s", fragment, joined)
		}
	}
}

func TestClassifyLongshoreScenarioDoesNotPromoteUserData(t *testing.T) {
	classification := classifyLongshoreScenario(false, geometry.WaveClimate{}, loadedModelInputs{})
	if classification.ScenarioStatus != geometry.ScenarioStatusUnclassified {
		t.Fatalf("пользовательский набор автоматически получил статус %q", classification.ScenarioStatus)
	}
}
