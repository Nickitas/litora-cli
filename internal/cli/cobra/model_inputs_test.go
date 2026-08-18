package cobra

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadModelInputsLoadsBathymetryAndLithology(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	bathymetryPath := filepath.Join(tempDir, "bathymetry.json")
	lithologyPath := filepath.Join(tempDir, "lithology.json")

	writeTestFile(t, bathymetryPath, `[{"lat":44,"lon":30,"depth":-100}]`)
	writeTestFile(t, lithologyPath, `{
  "metadata": {"name":"test profile","bounds":{"min_lat":40,"max_lat":50,"min_lon":20,"max_lon":40}},
  "points": [{"lat":44,"lon":30,"region":"test","lithology_class":"sand","resistance":1.5,"color":"#c2b280","description":"sand","confidence":"high","source":"test"}],
  "classes": {"sand":{"resistance":1.5,"color":"#c2b280","description":"sand"}}
}`)

	inputs, err := loadModelInputs(bathymetryPath, lithologyPath, false, 0.01)
	if err != nil {
		t.Fatalf("loadModelInputs() error = %v", err)
	}
	if inputs.BathymetryGrid == nil {
		t.Fatal("батиметрическая сетка не загружена")
	}
	if inputs.LithologyProfile == nil {
		t.Fatal("профиль литологии не загружен")
	}
	if !inputs.LithologyEnabled {
		t.Fatal("явно переданный профиль литологии не включён")
	}
}

func TestLoadModelInputsRejectsMissingExplicitFile(t *testing.T) {
	t.Parallel()

	_, err := loadModelInputs(filepath.Join(t.TempDir(), "missing.json"), "", false, 0.01)
	if err == nil {
		t.Fatal("ожидалась ошибка для отсутствующей батиметрии")
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("запись тестового файла %q: %v", path, err)
	}
}
