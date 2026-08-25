package cobra

import (
	"strings"
	"testing"
)

func TestSeabedRenderCommandExposesReproducibleInputs(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"seabed", "render"})
	if err != nil {
		t.Fatal(err)
	}
	if command != seabedRenderCmd {
		t.Fatalf("найдена неверная команда: %s", command.CommandPath())
	}
	for _, flag := range []string{"input", "metadata", "source-metadata", "source", "output", "isobaths", "vertical-exaggeration", "control-points"} {
		if command.Flags().Lookup(flag) == nil {
			t.Fatalf("команда seabed render не содержит флаг --%s", flag)
		}
	}
	for _, marker := range []string{"непрореженных фрагмента сетки", "3D-рельеф", "профиля «берег → глубоководье»"} {
		if !strings.Contains(command.Long, marker) {
			t.Fatalf("описание seabed render не объясняет фрагмент %q", marker)
		}
	}
}

func TestHelpForSeabedRenderIsRussian(t *testing.T) {
	prepareRussianHelp()
	usage := seabedRenderCmd.UsageString()
	for _, marker := range []string{"Использование:", "Флаги:", "показать справку"} {
		if !strings.Contains(usage, marker) {
			t.Fatalf("русская справка не содержит %q:\n%s", marker, usage)
		}
	}
	for _, marker := range []string{"Usage:", "Flags:", "help for render", "[flags]", "(default "} {
		if strings.Contains(usage, marker) {
			t.Fatalf("в справке остался англоязычный legacy-текст %q:\n%s", marker, usage)
		}
	}
}

func TestSeabedAdaptCommandExposesExplainableFieldParameters(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"seabed", "adapt"})
	if err != nil {
		t.Fatal(err)
	}
	if command != seabedAdaptCmd {
		t.Fatalf("найдена неверная команда: %s", command.CommandPath())
	}
	for _, flag := range []string{
		"input", "output", "source-metadata", "source", "min-size", "coast-size",
		"shelf-size", "deep-size", "coast-influence", "curvature-reference",
		"slope-reference", "flat-deep-slope", "max-neighbour-ratio", "max-size-gradient",
	} {
		if command.Flags().Lookup(flag) == nil {
			t.Fatalf("команда seabed adapt не содержит флаг --%s", flag)
		}
	}
	for _, marker := range []string{"расстоянием до берега", "градиентом глубины", "не запускает", "ADAPT-02"} {
		if !strings.Contains(command.Long, marker) {
			t.Fatalf("описание seabed adapt не содержит %q", marker)
		}
	}
}

func TestHelpForSeabedAdaptIsRussian(t *testing.T) {
	prepareRussianHelp()
	usage := seabedAdaptCmd.UsageString()
	for _, marker := range []string{"Использование:", "Флаги:", "показать справку", "целевая длина"} {
		if !strings.Contains(usage, marker) {
			t.Fatalf("русская справка ADAPT-01 не содержит %q:\n%s", marker, usage)
		}
	}
	for _, marker := range []string{"Usage:", "Flags:", "help for adapt", "[flags]", "(default "} {
		if strings.Contains(usage, marker) {
			t.Fatalf("в справке ADAPT-01 остался англоязычный текст %q:\n%s", marker, usage)
		}
	}
}
