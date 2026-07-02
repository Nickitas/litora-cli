package cli

import (
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/generators/koch"
	"flag"
	"fmt"
	"io"
	"strings"
)

const (
	defaultOutputDir = "output"
	cmdReal          = "real"
	cmdModel         = "model"
	cmdSource        = "source"
	cmdAll           = "all"
	cmdCoastline     = "coastline"
	cmdDimension     = "dimension"
	cmdErosion       = "erosion"
)

type config struct {
	Command         string
	InputPath       string
	SourceURL       string
	Refresh         bool
	OutputPath      string
	Iterations      int
	Steps           int
	Seed            int64
	AngleJitter     float64
	HeightJitter    float64
	ErosionStrength float64
	WaveDirection   float64
	WindSpeed       float64
	FetchSpread     float64
	FetchSamples    int
	MaxFetchKM      float64
	DepthScale      float64
	ExposurePower   float64
	BathymetryPath  string
	LithologyPath   string
	EnableLithology bool
	ModelMaxPoints  int
	DisableSimplify bool
	Quiet           bool
	// Temporal dynamics parameters
	TargetYears            int
	YearsPerStep           float64
	StormProbability       float64
	StormIntensityMult     float64
	SeaLevelRise           float64
	EnableSeasonality      bool
	SeasonalPhase          float64
	// CSV export parameters
	OutputCSV              string
	CSVFormat              string
	// GIF animation parameters
	OutputGIF              string
	GIFFPS                 int
	GIFSkip                int
	GIFColorByChange       bool
	GIFShowInitial         bool
	GIFShowMetrics         bool
	GIFShowScaleBar        bool
	GIFShowColorLegend     bool
	GIFScaleBarKM          float64
	GIFColorLegendPos      string
	GIFGeoLabels           string
	GIFShowTimeStamp      bool     // показывать временные метки на кадрах
	GIFWidth             int      // ширина GIF в пикселях (0 = auto 1200)
	GIFHeight            int      // высота GIF в пикселях (0 = auto 800)
	GIFColors           int      // количество цветов в палитре (0 = auto 16)
	GIFCompression      string   // уровень сжатия (low|medium|high)
	// Enhanced SVG options
	EnableEnhanced       bool     // включить enhanced SVG с дополнительными элементами
	ShowGrid             bool     // показать координатную сетку
	ShowCompass          bool     // показать компас/розу ветров
	ShowMarkers          bool     // показать маркеры ключевых точек
	ShowIsolines         bool     // показать изолинии глубин
	CompassStyle         string   // стиль компаса (modern|classic|minimal)
	GridStep             float64  // шаг координатной сетки в градусах
	CompassSize          int      // размер компаса в пикселях
	CompassWindDir       float64  // направление ветра для компаса
}

func parseConfig(args []string, stdout, stderr io.Writer) (config, error) {
	if len(args) == 0 {
		printBanner(stdout)
		printRootUsage(stdout)
		return config{}, flag.ErrHelp
	}

	if isHelpToken(args[0]) {
		printBanner(stdout)
		printRootUsage(stdout)
		return config{}, flag.ErrHelp
	}

	command, commandArgs, err := resolveCommand(args, stdout, stderr)
	if err != nil {
		return config{}, err
	}

	cfg := config{Command: command}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(stderr)

	// Общие флаги для всех команд
	fs.BoolVar(&cfg.Quiet, "quiet", false, "suppress startup banner")

	switch command {
	case cmdSource:
		fs.StringVar(&cfg.InputPath, "input", coastline.DefaultCoastlineJSONPath, "path to local coastline JSON/GeoJSON fallback file")
		fs.StringVar(&cfg.SourceURL, "source-url", coastline.DefaultCoastlineGeoJSONURL, "remote GeoJSON URL for coastline data; empty string disables HTTP loading")
		fs.BoolVar(&cfg.Refresh, "refresh", false, "force refresh of the remote GeoJSON cache before saving a snapshot")
		fs.StringVar(&cfg.OutputPath, "output", "", "snapshot file or directory (default: ./data/snapshots)")
		fs.Usage = func() { printBanner(stdout); printCommandUsage(stdout, command) }
	case cmdAll:
		fs.StringVar(&cfg.InputPath, "input", coastline.DefaultCoastlineJSONPath, "path to local coastline JSON/GeoJSON fallback file")
		fs.StringVar(&cfg.SourceURL, "source-url", coastline.DefaultCoastlineGeoJSONURL, "remote GeoJSON URL for coastline data; empty string disables HTTP loading")
		fs.BoolVar(&cfg.Refresh, "refresh", false, "force refresh of the remote GeoJSON cache before running")
		fs.StringVar(&cfg.OutputPath, "output", "", "output directory for generated visualizations (default: ./output)")
		fs.IntVar(&cfg.Iterations, "iterations", 5, fmt.Sprintf("maximum organic Koch iterations (0-%d)", koch.MaxIterations))
		fs.Int64Var(&cfg.Seed, "seed", 42, "random seed for organic coastline generation")
		fs.Float64Var(&cfg.AngleJitter, "angle-jitter", 18, "maximum random angle deviation in degrees")
		fs.Float64Var(&cfg.HeightJitter, "height-jitter", 0.25, "maximum random height deviation as a ratio")
		fs.Float64Var(&cfg.ErosionStrength, "erosion-strength", 0, "Gaussian erosion strength in meters; applied after fractal growth (0 disables)")
		fs.IntVar(&cfg.ModelMaxPoints, "model-max-points", 0, "max points for model base (0 keeps default budget); higher preserves details")
		fs.BoolVar(&cfg.DisableSimplify, "no-model-simplify", false, "disable model base simplification before fractal growth")
		// Волновая эрозия
		fs.IntVar(&cfg.Steps, "steps", 5, "number of wave erosion steps (0+)")
		fs.Float64Var(&cfg.WaveDirection, "wave-direction", 0, "direction waves come from, in degrees clockwise from north")
		fs.Float64Var(&cfg.WindSpeed, "wind-speed", 12, "wind speed driving wave energy, in m/s")
		fs.Float64Var(&cfg.FetchSpread, "fetch-spread", 55, "half-width of the offshore sector sampled for fetch, in degrees")
		fs.IntVar(&cfg.FetchSamples, "fetch-samples", 9, "number of ray directions used to estimate fetch/exposure")
		fs.Float64Var(&cfg.MaxFetchKM, "max-fetch-km", 150, "upper clamp for fetch distance in kilometers")
		fs.Float64Var(&cfg.DepthScale, "depth-scale", 4000, "nearshore openness scale used as a depth proxy, in meters")
		fs.Float64Var(&cfg.ExposurePower, "exposure-power", 1.5, "nonlinear weight for wave-incidence angle")
		fs.StringVar(&cfg.BathymetryPath, "bathymetry", "", "path to bathymetry JSON file with lat,lon,depth points (empty uses automatic)")
		fs.StringVar(&cfg.LithologyPath, "lithology", "", "path to lithology JSON file with rock resistance data (empty uses default)")
		fs.BoolVar(&cfg.EnableLithology, "enable-lithology", false, "enable lithology-based erosion modulation (retreat /= resistance)")
		// Temporal dynamics flags
		fs.IntVar(&cfg.TargetYears, "target-years", 0, "target simulation duration in years (0 uses steps)")
		fs.Float64Var(&cfg.YearsPerStep, "years-per-step", 1.0, "years per erosion step (requires target-years)")
		fs.Float64Var(&cfg.StormProbability, "storm-probability", 0, "probability of storm event per step [0-1]")
		fs.Float64Var(&cfg.StormIntensityMult, "storm-intensity", 2.0, "storm intensity multiplier [1.0-10.0]")
		fs.Float64Var(&cfg.SeaLevelRise, "sea-level-rise", 0, "sea level rise in meters per year")
		fs.BoolVar(&cfg.EnableSeasonality, "enable-seasonality", false, "enable seasonal erosion variations")
		fs.Float64Var(&cfg.SeasonalPhase, "seasonal-phase", 0, "seasonal phase offset in radians [0-2π]")
		// CSV export flags
		fs.StringVar(&cfg.OutputCSV, "output-csv", "erosion_metrics.csv", "path to CSV file for erosion metrics export (default: erosion_metrics.csv)")
		fs.StringVar(&cfg.CSVFormat, "csv-format", "long", "CSV format: 'long' (one row per step) or 'wide' (one row with step columns)")
		// GIF animation flags
		fs.StringVar(&cfg.OutputGIF, "output-gif", "", "path to GIF file for erosion animation (empty disables GIF export)")
		fs.IntVar(&cfg.GIFFPS, "gif-fps", 10, "GIF animation frames per second (1-30)")
		fs.IntVar(&cfg.GIFSkip, "gif-skip", 1, "skip every N frames to reduce GIF size (1 = don't skip)")
		fs.BoolVar(&cfg.GIFColorByChange, "gif-color-change", true, "enable color coding by erosion/deposition intensity")
		fs.BoolVar(&cfg.GIFShowInitial, "gif-show-initial", true, "show initial coastline state (gray line)")
			fs.BoolVar(&cfg.GIFShowScaleBar, "gif-show-scalebar", true, "show scale bar on GIF (important for scientific publications)")
		fs.Float64Var(&cfg.GIFScaleBarKM, "gif-scalebar-km", 0, "scale bar length in km (0 = auto-detect)" )
			fs.StringVar(&cfg.GIFColorLegendPos, "gif-colorlegend-pos", "right", "color legend position (right|bottom|none)")
			fs.IntVar(&cfg.GIFColors, "gif-colors", 0, "number of palette colors (0 = auto 16, 4-256)")
			fs.StringVar(&cfg.GIFCompression, "gif-compression", "medium", "compression level (low|medium|high)")
			fs.IntVar(&cfg.GIFWidth, "gif-width", 1200, "GIF width in pixels (0 = auto 1200)")
			fs.IntVar(&cfg.GIFHeight, "gif-height", 800, "GIF height in pixels (0 = auto 800)")
			fs.BoolVar(&cfg.GIFShowTimeStamp, "gif-show-timestamp", true, "show time stamps (years, storms) on GIF frames")
			fs.StringVar(&cfg.GIFGeoLabels, "gif-geo-labels", "major", "geographic labels (none|major|all)")
		fs.BoolVar(&cfg.GIFShowMetrics, "gif-show-metrics", true, "show frame metrics (length, erosion)")
			// Enhanced SVG flags
			fs.BoolVar(&cfg.EnableEnhanced, "enhanced", true, "enable enhanced SVG with cartographic elements")
			fs.BoolVar(&cfg.ShowGrid, "show-grid", true, "show coordinate grid on maps")
			fs.BoolVar(&cfg.ShowCompass, "show-compass", true, "show compass/wind rose")
			fs.BoolVar(&cfg.ShowMarkers, "show-markers", true, "show key point markers")
			fs.BoolVar(&cfg.ShowIsolines, "show-isolines", false, "show depth contour lines (requires bathymetry data)")
			fs.StringVar(&cfg.CompassStyle, "compass-style", "modern", "compass style: modern, classic, or minimal")
			fs.Float64Var(&cfg.GridStep, "grid-step", 0.2, "coordinate grid step in degrees")
			fs.IntVar(&cfg.CompassSize, "compass-size", 32, "compass size in pixels")
			fs.Float64Var(&cfg.CompassWindDir, "compass-wind-dir", 315, "wind direction for compass arrow (degrees from north)")
			fs.Usage = func() { printBanner(stdout); printCommandUsage(stdout, command) }
	case cmdDimension:
		fs.StringVar(&cfg.InputPath, "input", coastline.DefaultCoastlineJSONPath, "path to local coastline JSON/GeoJSON fallback file")
		fs.StringVar(&cfg.SourceURL, "source-url", coastline.DefaultCoastlineGeoJSONURL, "remote GeoJSON URL for coastline data; empty string disables HTTP loading")
		fs.BoolVar(&cfg.Refresh, "refresh", false, "force refresh of the remote GeoJSON cache before running")
		fs.StringVar(&cfg.OutputPath, "output", "", "output directory for generated visualizations (default: ./output)")
		fs.IntVar(&cfg.Iterations, "iterations", 5, fmt.Sprintf("maximum organic Koch iterations (0-%d)", koch.MaxIterations))
		fs.Int64Var(&cfg.Seed, "seed", 42, "random seed for organic coastline generation")
		fs.Float64Var(&cfg.AngleJitter, "angle-jitter", 18, "maximum random angle deviation in degrees")
		fs.Float64Var(&cfg.HeightJitter, "height-jitter", 0.25, "maximum random height deviation as a ratio")
		fs.Float64Var(&cfg.ErosionStrength, "erosion-strength", 0, "Gaussian erosion strength in meters; applied after fractal growth (0 disables)")
		fs.IntVar(&cfg.ModelMaxPoints, "model-max-points", 0, "max points for model base (0 keeps default budget); higher preserves details")
		fs.BoolVar(&cfg.DisableSimplify, "no-model-simplify", false, "disable model base simplification before fractal growth")
		fs.Usage = func() { printBanner(stdout); printCommandUsage(stdout, command) }
	case cmdErosion:
		fs.StringVar(&cfg.InputPath, "input", coastline.DefaultCoastlineJSONPath, "path to local coastline JSON/GeoJSON fallback file")
		fs.StringVar(&cfg.SourceURL, "source-url", coastline.DefaultCoastlineGeoJSONURL, "remote GeoJSON URL for coastline data; empty string disables HTTP loading")
		fs.BoolVar(&cfg.Refresh, "refresh", false, "force refresh of the remote GeoJSON cache before running")
		fs.StringVar(&cfg.OutputPath, "output", "", "output directory for generated visualizations (default: ./output)")
		fs.IntVar(&cfg.Steps, "steps", 5, "number of erosion steps (0+)")
		fs.Int64Var(&cfg.Seed, "seed", 42, "random seed for erosion simulation")
		fs.Float64Var(&cfg.ErosionStrength, "erosion-strength", 50, "base shoreline retreat in meters per step before fetch/exposure scaling (0 disables)")
		fs.Float64Var(&cfg.WaveDirection, "wave-direction", 0, "direction waves come from, in degrees clockwise from north")
		fs.Float64Var(&cfg.WindSpeed, "wind-speed", 12, "wind speed driving wave energy, in m/s")
		fs.Float64Var(&cfg.FetchSpread, "fetch-spread", 55, "half-width of the offshore sector sampled for fetch, in degrees")
		fs.IntVar(&cfg.FetchSamples, "fetch-samples", 9, "number of ray directions used to estimate fetch/exposure")
		fs.Float64Var(&cfg.MaxFetchKM, "max-fetch-km", 150, "upper clamp for fetch distance in kilometers")
		fs.Float64Var(&cfg.DepthScale, "depth-scale", 4000, "nearshore openness scale used as a depth proxy, in meters")
		fs.Float64Var(&cfg.ExposurePower, "exposure-power", 1.5, "nonlinear weight for wave-incidence angle")
		fs.StringVar(&cfg.BathymetryPath, "bathymetry", "", "path to bathymetry JSON file with lat,lon,depth points (empty uses geometric proxy)")
		fs.StringVar(&cfg.LithologyPath, "lithology", "", "path to lithology JSON file with rock resistance data (empty uses default)")
		fs.BoolVar(&cfg.EnableLithology, "enable-lithology", false, "enable lithology-based erosion modulation (retreat /= resistance)")
		// Temporal dynamics flags
		fs.IntVar(&cfg.TargetYears, "target-years", 0, "target simulation duration in years (0 uses steps)")
		fs.Float64Var(&cfg.YearsPerStep, "years-per-step", 1.0, "years per erosion step (requires target-years)")
		fs.Float64Var(&cfg.StormProbability, "storm-probability", 0, "probability of storm event per step [0-1]")
		fs.Float64Var(&cfg.StormIntensityMult, "storm-intensity", 2.0, "storm intensity multiplier [1.0-10.0]")
		fs.Float64Var(&cfg.SeaLevelRise, "sea-level-rise", 0, "sea level rise in meters per year")
		fs.BoolVar(&cfg.EnableSeasonality, "enable-seasonality", false, "enable seasonal erosion variations")
		fs.Float64Var(&cfg.SeasonalPhase, "seasonal-phase", 0, "seasonal phase offset in radians [0-2π]")
		// CSV export flags
		fs.StringVar(&cfg.OutputCSV, "output-csv", "erosion_metrics.csv", "path to CSV file for erosion metrics export (default: erosion_metrics.csv)")
		fs.StringVar(&cfg.CSVFormat, "csv-format", "long", "CSV format: 'long' (one row per step) or 'wide' (one row with step columns)")
		// GIF animation flags
		fs.StringVar(&cfg.OutputGIF, "output-gif", "", "path to GIF file for erosion animation (empty disables GIF export)")
		fs.BoolVar(&cfg.GIFShowColorLegend, "gif-show-colorlegend", true, "show color legend on GIF")
			fs.StringVar(&cfg.GIFColorLegendPos, "gif-colorlegend-pos", "right", "color legend position (right|bottom|none)")
			fs.StringVar(&cfg.GIFGeoLabels, "gif-geo-labels", "major", "geographic labels (none|major|all)")
			fs.BoolVar(&cfg.GIFShowScaleBar, "gif-show-scalebar", true, "show scale bar on GIF (important for scientific publications)")
		fs.IntVar(&cfg.GIFFPS, "gif-fps", 10, "GIF animation frames per second (1-30)")
		fs.IntVar(&cfg.GIFSkip, "gif-skip", 1, "skip every N frames to reduce GIF size (1 = don't skip)")
		fs.Float64Var(&cfg.GIFScaleBarKM, "gif-scalebar-km", 0, "scale bar length in km (0 = auto-detect)")
		fs.BoolVar(&cfg.GIFColorByChange, "gif-color-change", true, "enable color coding by erosion/deposition intensity")
		fs.BoolVar(&cfg.GIFShowInitial, "gif-show-initial", true, "show initial coastline state (gray line)")
		fs.IntVar(&cfg.GIFColors, "gif-colors", 0, "number of palette colors (0 = auto 16, 4-256)")
		fs.StringVar(&cfg.GIFCompression, "gif-compression", "medium", "compression level (low|medium|high)")
		fs.BoolVar(&cfg.GIFShowMetrics, "gif-show-metrics", true, "show frame metrics (length, erosion)")
		fs.BoolVar(&cfg.GIFShowTimeStamp, "gif-show-timestamp", true, "show time stamps (years, storms) on GIF frames")
		fs.IntVar(&cfg.GIFWidth, "gif-width", 1200, "GIF width in pixels (0 = auto 1200)")
		fs.IntVar(&cfg.GIFHeight, "gif-height", 800, "GIF height in pixels (0 = auto 800)")
			// Enhanced SVG flags
			fs.BoolVar(&cfg.EnableEnhanced, "enhanced", true, "enable enhanced SVG with cartographic elements")
			fs.BoolVar(&cfg.ShowGrid, "show-grid", true, "show coordinate grid on maps")
			fs.BoolVar(&cfg.ShowCompass, "show-compass", true, "show compass/wind rose")
			fs.BoolVar(&cfg.ShowMarkers, "show-markers", true, "show key point markers")
			fs.BoolVar(&cfg.ShowIsolines, "show-isolines", false, "show depth contour lines (requires bathymetry data)")
			fs.StringVar(&cfg.CompassStyle, "compass-style", "modern", "compass style: modern, classic, or minimal")
			fs.Float64Var(&cfg.GridStep, "grid-step", 0.2, "coordinate grid step in degrees")
			fs.IntVar(&cfg.CompassSize, "compass-size", 32, "compass size in pixels")
			fs.Float64Var(&cfg.CompassWindDir, "compass-wind-dir", 315, "wind direction for compass arrow (degrees from north)")
		fs.Usage = func() { printBanner(stdout); printCommandUsage(stdout, command) }
	}

	if err := fs.Parse(commandArgs); err != nil {
		if err == flag.ErrHelp {
			return config{}, err
		}
		return config{}, err
	}

	if fs.NArg() > 0 {
		fs.Usage()
		return config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	if commandUsesIterations(command) && (cfg.Iterations < 0 || cfg.Iterations > koch.MaxIterations) {
		return config{}, fmt.Errorf("iterations must be between 0 and %d", koch.MaxIterations)
	}
	if command == cmdAll || command == cmdDimension {
		if cfg.AngleJitter < 0 {
			return config{}, fmt.Errorf("angle-jitter must be non-negative")
		}
		if cfg.HeightJitter < 0 {
			return config{}, fmt.Errorf("height-jitter must be non-negative")
		}
	}
	if cfg.ErosionStrength < 0 {
		return config{}, fmt.Errorf("erosion-strength must be non-negative")
	}
	if command == cmdErosion && cfg.Steps < 0 {
		return config{}, fmt.Errorf("steps must be non-negative")
	}
	if command == cmdErosion && cfg.WindSpeed <= 0 {
		return config{}, fmt.Errorf("wind-speed must be positive")
	}
	if command == cmdErosion && cfg.FetchSpread < 0 {
		return config{}, fmt.Errorf("fetch-spread must be non-negative")
	}
	if command == cmdErosion && cfg.FetchSamples <= 0 {
		return config{}, fmt.Errorf("fetch-samples must be positive")
	}
	if command == cmdErosion && cfg.MaxFetchKM <= 0 {
		return config{}, fmt.Errorf("max-fetch-km must be positive")
	}
	if command == cmdErosion && cfg.DepthScale <= 0 {
		return config{}, fmt.Errorf("depth-scale must be positive")
	}
	if command == cmdErosion && cfg.ExposurePower <= 0 {
		return config{}, fmt.Errorf("exposure-power must be positive")
	}
	if cfg.ModelMaxPoints < 0 {
		return config{}, fmt.Errorf("model-max-points must be non-negative")
	}
	if cfg.OutputCSV != "" && cfg.CSVFormat != "long" && cfg.CSVFormat != "wide" {
		return config{}, fmt.Errorf("csv-format must be 'long' or 'wide'")
	}
	if cfg.GIFFPS < 0 || cfg.GIFFPS > 30 {
		return config{}, fmt.Errorf("gif-fps must be between 0 and 30")
	}
	if cfg.GIFSkip < 1 {
		return config{}, fmt.Errorf("gif-skip must be >= 1")
	}
	if cfg.GIFColors != 0 && (cfg.GIFColors < 4 || cfg.GIFColors > 256) {
		return config{}, fmt.Errorf("gif-colors must be between 4 and 256 (0 for auto)")
	}
	if cfg.GIFCompression != "low" && cfg.GIFCompression != "medium" && cfg.GIFCompression != "high" {
		return config{}, fmt.Errorf("gif-compression must be low, medium, or high")
	}

	return cfg, nil
}

func commandNeedsCoastline(command string) bool {
	switch command {
	case cmdAll, cmdCoastline, cmdDimension, cmdErosion:
		return true
	default:
		return false
	}
}

func commandUsesIterations(command string) bool {
	switch command {
	case cmdAll, cmdDimension:
		return true
	default:
		return false
	}
}

func isHelp(err error) bool {
	return err == flag.ErrHelp
}

func resolveCommand(args []string, stdout, stderr io.Writer) (string, []string, error) {
	switch args[0] {
	case cmdReal:
		return resolveGroupedCommand(cmdReal, args[1:], stdout, stderr)
	case cmdModel:
		return resolveGroupedCommand(cmdModel, args[1:], stdout, stderr)
	case cmdSource, cmdAll:
		return args[0], args[1:], nil
	default:
		printRootUsage(stderr)
		return "", nil, fmt.Errorf("unknown command %q", args[0])
	}
}

func resolveGroupedCommand(group string, args []string, stdout, stderr io.Writer) (string, []string, error) {
	if len(args) == 0 || isHelpToken(args[0]) {
		printBanner(stdout)
		printGroupUsage(stdout, group)
		return "", nil, flag.ErrHelp
	}

	command := args[0]
	if !commandBelongsToGroup(command, group) {
		printGroupUsage(stderr, group)
		return "", nil, fmt.Errorf("unknown %s command %q", group, command)
	}

	return command, args[1:], nil
}

func commandBelongsToGroup(command, group string) bool {
	switch group {
	case cmdReal:
		return command == cmdCoastline
	case cmdModel:
		switch command {
		case cmdDimension, cmdErosion:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isHelpToken(arg string) bool {
	switch arg {
	case "-h", "--help", "help":
		return true
	default:
		return false
	}
}
