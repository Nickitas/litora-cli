package geometry

import (
	"os"
	"testing"
)

func TestValidationRejectsInvalidData(t *testing.T) {
	// Создаём временный файл с некорректными данными
	tmpFile := "test_invalid_temp.json"
	defer os.Remove(tmpFile)

	invalidData := `[
		{"lat": 48.0, "lon": 30.0, "depth": -100},
		{"lat": 45.0, "lon": 50.0, "depth": -150},
		{"lat": 40.0, "lon": 30.0, "depth": 100}
	]`

	if err := os.WriteFile(tmpFile, []byte(invalidData), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	data, _ := os.ReadFile(tmpFile)
	_, err := LoadBathymetryFromJSON(data, BathymetryLoadOptions{Resolution: 0.01})

	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	// Проверяем, что сообщение об ошибке информативное
	errMsg := err.Error()
	if !contains(errMsg, "outside Black Sea") && !contains(errMsg, "positive depth") {
		t.Errorf("error message not informative: %s", errMsg)
	}
}

func TestValidationAcceptsValidData(t *testing.T) {
	validData := `[
		{"lat": 45.0, "lon": 30.0, "depth": -100},
		{"lat": 44.0, "lon": 31.0, "depth": -150},
		{"lat": 43.0, "lon": 32.0, "depth": -200}
	]`

	tmpFile := "test_valid_temp.json"
	defer os.Remove(tmpFile)

	if err := os.WriteFile(tmpFile, []byte(validData), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	data, _ := os.ReadFile(tmpFile)
	grid, err := LoadBathymetryFromJSON(data, BathymetryLoadOptions{Resolution: 0.01})

	if err != nil {
		t.Fatalf("unexpected error for valid data: %v", err)
	}

	if grid == nil {
		t.Fatal("grid is nil")
	}

	if len(grid.Points) != 3 {
		t.Errorf("expected 3 points, got %d", len(grid.Points))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
