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
	for _, flag := range []string{"input", "metadata", "source-metadata", "source", "output", "isobaths"} {
		if command.Flags().Lookup(flag) == nil {
			t.Fatalf("команда seabed render не содержит флаг --%s", flag)
		}
	}
	for _, marker := range []string{"сложный берег", "шельф", "крутой материковый склон"} {
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
