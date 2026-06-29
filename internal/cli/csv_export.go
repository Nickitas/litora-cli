package cli

import (
	"coastal-geometry/internal/domain/geometry"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

// writeErosionCSV exports erosion metrics to CSV format
// Supports two formats: "long" (one row per step) and "wide" (one row with step columns)
func writeErosionCSV(
	snapshots [][]geometry.LatLon,
	temporalResult *geometry.TemporalResult,
	outputPath string,
	format string,
	outputPathManager *OutputPathManager,
) error {
	if outputPath == "" {
		return fmt.Errorf("output CSV path cannot be empty")
	}

	// Resolve the output path using the OutputPathManager
	resolvedPath := outputPathManager.ResolveUserPath(outputPath, "csv")
	if resolvedPath == "" {
		resolvedPath = outputPathManager.CSVPath(outputPath)
	}

	file, err := os.Create(resolvedPath)
	if err != nil {
		return fmt.Errorf("create CSV file %q: %w", resolvedPath, err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	switch format {
	case "long":
		return writeLongFormatCSV(writer, snapshots, temporalResult)
	case "wide":
		return writeWideFormatCSV(writer, snapshots, temporalResult)
	default:
		return fmt.Errorf("unsupported CSV format: %s", format)
	}
}

// writeLongFormatCSV creates CSV with one row per step
// Columns: year,step,length_km,area_km2,eroded_m3,deposited_m3,net_change_m3,storm_event,sea_level_m
func writeLongFormatCSV(
	writer *csv.Writer,
	snapshots [][]geometry.LatLon,
	temporalResult *geometry.TemporalResult,
) error {
	// Write header
	header := []string{
		"year", "step", "length_km", "area_km2",
		"eroded_m3", "deposited_m3", "net_change_m3",
		"storm_event", "sea_level_m",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	// Write data rows
	for i, snapshot := range snapshots {
		// Get temporal state if available
		var state geometry.TemporalState
		var hasTemporalState bool
		if temporalResult != nil && i < len(temporalResult.TemporalStates) {
			state = temporalResult.TemporalStates[i]
			hasTemporalState = true
		}

		// Calculate metrics
		lengthKm := geometry.PolylineLength(snapshot)
		areaKm2 := geometry.Area(snapshot)

		// Calculate erosion/deposition volumes (simplified)
		var erodedM3, depositedM3, netChangeM3 float64
		if i > 0 && len(snapshots[i-1]) > 0 && len(snapshot) > 0 {
			// Simple estimation based on length change
			prevLength := geometry.PolylineLength(snapshots[i-1])
			lengthChange := prevLength - lengthKm

			// Convert to volume (very rough approximation)
			// Assume average coastline retreat of 1m depth
			erodedM3 = lengthChange * 1000 * 1 // km to m, assuming 1m depth

			// For simplicity, assume no deposition in this model
			depositedM3 = 0
			netChangeM3 = erodedM3 - depositedM3
		}

		// Storm event indicator
		stormEvent := "false"
		if hasTemporalState && state.IsStorm {
			stormEvent = "true"
		}

		// Sea level
		seaLevelM := 0.0
		if hasTemporalState && state.SeaLevelOffset > 0 {
			seaLevelM = state.SeaLevelOffset
		}

		// Get year
		year := 0.0
		if hasTemporalState {
			year = state.Year
		}

		// Write row
		row := []string{
			fmt.Sprintf("%.1f", year),
			strconv.Itoa(i),
			fmt.Sprintf("%.1f", lengthKm),
			fmt.Sprintf("%.1f", areaKm2),
			fmt.Sprintf("%.1f", erodedM3),
			fmt.Sprintf("%.1f", depositedM3),
			fmt.Sprintf("%.1f", netChangeM3),
			stormEvent,
			fmt.Sprintf("%.4f", seaLevelM),
		}

		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write CSV row %d: %w", i, err)
		}
	}

	return nil
}

// writeWideFormatCSV creates CSV with one row and columns for each step
func writeWideFormatCSV(
	writer *csv.Writer,
	snapshots [][]geometry.LatLon,
	temporalResult *geometry.TemporalResult,
) error {
	// Determine number of metrics columns per step
	numSteps := len(snapshots)

	// Build header: metric_name,step_0,step_1,...,step_N
	header := []string{"metric_name"}
	for i := 0; i < numSteps; i++ {
		header = append(header, fmt.Sprintf("step_%d", i))
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}

	// Prepare data arrays
	years := make([]string, numSteps)
	lengths := make([]string, numSteps)
	areas := make([]string, numSteps)
	storms := make([]string, numSteps)
	seaLevels := make([]string, numSteps)

	// Extract data for each step
	for i, snapshot := range snapshots {
		// Get temporal state if available
		var state geometry.TemporalState
		var hasTemporalState bool
		if temporalResult != nil && i < len(temporalResult.TemporalStates) {
			state = temporalResult.TemporalStates[i]
			hasTemporalState = true
		}

		// Year
		if hasTemporalState {
			years[i] = fmt.Sprintf("%.1f", state.Year)
		} else {
			years[i] = fmt.Sprintf("%d", i)
		}

		// Length and area
		lengths[i] = fmt.Sprintf("%.1f", geometry.PolylineLength(snapshot))
		areas[i] = fmt.Sprintf("%.1f", geometry.Area(snapshot))

		// Storm indicator
		if hasTemporalState && state.IsStorm {
			storms[i] = "true"
		} else {
			storms[i] = "false"
		}

		// Sea level
		if hasTemporalState && state.SeaLevelOffset > 0 {
			seaLevels[i] = fmt.Sprintf("%.4f", state.SeaLevelOffset)
		} else {
			seaLevels[i] = "0.0000"
		}
	}

	// Write rows for each metric
	metrics := []struct {
		name   string
		values []string
	}{
		{"year", years},
		{"length_km", lengths},
		{"area_km2", areas},
		{"storm_event", storms},
		{"sea_level_m", seaLevels},
	}

	for _, metric := range metrics {
		row := append([]string{metric.name}, metric.values...)
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("write CSV metric %s: %w", metric.name, err)
		}
	}

	return nil
}
