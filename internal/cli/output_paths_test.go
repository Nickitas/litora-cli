package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOutputPathManagerBenchmark проверяет каталог и разрешение путей
// параметрических отчётов benchmark.
func TestOutputPathManagerBenchmark(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewOutputPathManager(baseDir)

	if err := manager.EnsureDirectories(); err != nil {
		t.Fatalf("создание выходных каталогов: %v", err)
	}
	if _, err := os.Stat(manager.BenchmarkDir()); err != nil {
		t.Fatalf("каталог benchmark не создан: %v", err)
	}

	wantDefault := filepath.Join(baseDir, "benchmark", "parametric-scenarios.json")
	if got := manager.ResolveBenchmarkPath("", "parametric-scenarios.json"); got != wantDefault {
		t.Errorf("путь по умолчанию = %q, требуется %q", got, wantDefault)
	}

	wantNamed := filepath.Join(baseDir, "benchmark", "custom.json")
	if got := manager.ResolveBenchmarkPath("reports/custom.json", "parametric-scenarios.json"); got != wantNamed {
		t.Errorf("относительный путь = %q, требуется %q", got, wantNamed)
	}

	absolute := filepath.Join(t.TempDir(), "explicit.json")
	if got := manager.ResolveBenchmarkPath(absolute, "parametric-scenarios.json"); got != absolute {
		t.Errorf("абсолютный путь = %q, требуется %q", got, absolute)
	}
}
