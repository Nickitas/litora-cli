package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// Output directory subdirectories
const (
	subdirSVG     = "svg"
	subdirMetrics = "metrics"
	subdirCSV     = "csv"
)

// OutputPathManager manages output directory structure and paths
type OutputPathManager struct {
	baseDir string
}

// NewOutputPathManager creates a new output path manager
func NewOutputPathManager(baseDir string) *OutputPathManager {
	if baseDir == "" {
		baseDir = defaultOutputDir
	}
	return &OutputPathManager{
		baseDir: baseDir,
	}
}

// EnsureDirectories creates all output subdirectories if they don't exist
func (opm *OutputPathManager) EnsureDirectories() error {
	dirs := []string{
		opm.SVGDir(),
		opm.MetricsDir(),
		opm.CSVDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return nil
}

// BaseDir returns the base output directory
func (opm *OutputPathManager) BaseDir() string {
	return opm.baseDir
}

// SVGDir returns the SVG output directory
func (opm *OutputPathManager) SVGDir() string {
	return filepath.Join(opm.baseDir, subdirSVG)
}

// MetricsDir returns the metrics output directory
func (opm *OutputPathManager) MetricsDir() string {
	return filepath.Join(opm.baseDir, subdirMetrics)
}

// CSVDir returns the CSV output directory
func (opm *OutputPathManager) CSVDir() string {
	return filepath.Join(opm.baseDir, subdirCSV)
}

// SVGPath returns the full path for an SVG file
func (opm *OutputPathManager) SVGPath(filename string) string {
	return filepath.Join(opm.SVGDir(), filename)
}

// MetricsPath returns the full path for a metrics file
func (opm *OutputPathManager) MetricsPath(filename string) string {
	return filepath.Join(opm.MetricsDir(), filename)
}

// CSVPath returns the full path for a CSV file
func (opm *OutputPathManager) CSVPath(filename string) string {
	return filepath.Join(opm.CSVDir(), filename)
}

// ResolveUserPath resolves a user-provided path to the appropriate subdirectory
// If the path is absolute, it uses it as-is
// If the path is relative and starts with a subdirectory name (svg/, metrics/, csv/),
// it places it in the appropriate subdirectory
// Otherwise, it uses the base directory
func (opm *OutputPathManager) ResolveUserPath(userPath string, fileType string) string {
	if userPath == "" {
		return ""
	}

	// If absolute path, use as-is
	if filepath.IsAbs(userPath) {
		return userPath
	}

	// If path already includes a subdirectory prefix, use as-is
	base := filepath.Base(userPath)
	dir := filepath.Dir(userPath)

	switch fileType {
	case "svg":
		if dir == subdirSVG {
			return filepath.Join(opm.baseDir, userPath)
		}
		return opm.SVGPath(base)
	case "metrics":
		if dir == subdirMetrics {
			return filepath.Join(opm.baseDir, userPath)
		}
		return opm.MetricsPath(base)
	case "csv":
		if dir == subdirCSV {
			return filepath.Join(opm.baseDir, userPath)
		}
		return opm.CSVPath(base)
	default:
		// Unknown file type, place in base directory
		return filepath.Join(opm.baseDir, userPath)
	}
}

// ParseFileType determines the file type from a filename/extension
func ParseFileType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".svg":
		return "svg"
	case ".json":
		return "metrics"
	case ".csv":
		return "csv"
	default:
		return "unknown"
	}
}
