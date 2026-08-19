package cobra

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestLoadModelInputsUsesVerifiedPassportResolution(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	bathymetryPath := filepath.Join(tempDir, "bathymetry.json")
	bathymetry := `[{"lat":44,"lon":30,"depth":-100}]`
	writeTestFile(t, bathymetryPath, bathymetry)
	passportPath := filepath.Join(tempDir, "bathymetry.metadata.json")
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(bathymetry)))
	writeTestFile(t, passportPath, fmt.Sprintf(`{
  "schema_version":"1.0",
  "title":"Производный тестовый набор",
  "status":"verified_derived",
  "dataset_file":"bathymetry.json",
  "dataset_sha256":"%s",
  "point_count":1,
  "target_resolution_degrees":0.02,
  "target_resolution_arc_seconds":72,
  "source_product":"GEBCO_2026",
  "source_downloaded_at":"2026-08-19T12:00:00Z",
  "source_netcdf":"source.nc",
  "source_netcdf_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "source_grid_interval_arc_seconds":15,
  "horizontal_reference":"WGS84",
  "vertical_reference":"Средний уровень моря по допущению GEBCO",
  "resampling_method":"ближайший сосед",
  "processing_script":"convert_bathymetry.py",
  "license":"Общественное достояние с атрибуцией"
}`, checksum))

	inputs, err := loadModelInputs(bathymetryPath, "", false, 0)
	if err != nil {
		t.Fatalf("loadModelInputs() error = %v", err)
	}
	if inputs.BathymetryGrid.Resolution != 0.02 {
		t.Fatalf("ожидался шаг из паспорта 0.02, получено %f", inputs.BathymetryGrid.Resolution)
	}
	if inputs.BathymetryPassportPath != passportPath {
		t.Fatalf("ожидался паспорт %q, получено %q", passportPath, inputs.BathymetryPassportPath)
	}
	if len(inputs.Warnings) != 0 {
		t.Fatalf("не ожидались предупреждения: %v", inputs.Warnings)
	}
	if _, err := loadModelInputs(bathymetryPath, "", false, 0.01); err == nil || !strings.Contains(err.Error(), "не совпадает с паспортом") {
		t.Fatalf("ожидалась ошибка для противоречащего паспорту шага, получено: %v", err)
	}
}

func TestLoadModelInputsWarnsWhenPassportIsMissing(t *testing.T) {
	t.Parallel()

	bathymetryPath := filepath.Join(t.TempDir(), "bathymetry.json")
	writeTestFile(t, bathymetryPath, `[{"lat":44,"lon":30,"depth":-100}]`)

	inputs, err := loadModelInputs(bathymetryPath, "", false, 0.01)
	if err != nil {
		t.Fatalf("loadModelInputs() error = %v", err)
	}
	if len(inputs.Warnings) != 1 || !strings.Contains(inputs.Warnings[0], "не найден паспорт") {
		t.Fatalf("ожидалось предупреждение об отсутствующем паспорте: %v", inputs.Warnings)
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("запись тестового файла %q: %v", path, err)
	}
}
