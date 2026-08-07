package cobra

import (
	"os"
	"path/filepath"
	"testing"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/geometry"
)

func TestExportErosionArtifactsWritesCSVAndGIF(t *testing.T) {
	t.Parallel()

	outputDir := filepath.Join(t.TempDir(), "output")
	manager := cli.NewOutputPathManager(outputDir)
	snapshots := [][]geometry.LatLon{
		{{Lat: 44, Lon: 30}, {Lat: 44.1, Lon: 30.1}, {Lat: 44.2, Lon: 30}},
		{{Lat: 44, Lon: 30}, {Lat: 44.1, Lon: 30.099}, {Lat: 44.2, Lon: 30}},
	}

	if err := exportErosionArtifacts(manager, snapshots, nil, "metrics.csv", "long", "animation.gif", 1, 1); err != nil {
		t.Fatalf("exportErosionArtifacts() error = %v", err)
	}

	for _, path := range []string{
		filepath.Join(outputDir, "csv", "metrics.csv"),
		filepath.Join(outputDir, "gif", "animation.gif"),
	} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("ожидался экспортированный файл %q: %v", path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("экспортированный файл %q пуст", path)
		}
	}
}
