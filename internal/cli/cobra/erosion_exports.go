package cobra

import (
	"fmt"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/geometry"
)

func exportErosionArtifacts(
	outputManager *cli.OutputPathManager,
	snapshots [][]geometry.LatLon,
	temporalResult *geometry.TemporalResult,
	outputCSV, csvFormat, outputGIF string,
	gifFPS, gifSkip int,
) error {
	if outputCSV != "" {
		if err := cli.WriteErosionCSV(snapshots, temporalResult, outputCSV, csvFormat, outputManager); err != nil {
			return fmt.Errorf("экспорт CSV: %w", err)
		}
		fmt.Printf("CSV сохранён: %s\n", outputManager.ResolveUserPath(outputCSV, "csv"))
	}

	if outputGIF != "" {
		gifConfig := cli.DefaultGIFConfig()
		gifConfig.OutputPath = outputManager.ResolveUserPath(outputGIF, "gif")
		gifConfig.FPS = gifFPS
		gifConfig.SkipEvery = gifSkip
		if temporalResult != nil {
			gifConfig.TemporalStates = temporalResult.TemporalStates
		}
		if err := cli.GenerateErosionGIFWithConfig(snapshots, gifConfig); err != nil {
			return fmt.Errorf("экспорт GIF: %w", err)
		}
		fmt.Printf("GIF сохранён: %s\n", gifConfig.OutputPath)
	}

	return nil
}
