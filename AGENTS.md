# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## Language Rule

**This program is for Russian-speaking users.** All user-facing text, command descriptions, output messages, and comments must be in Russian. Do not translate or replace Russian text with English when modifying CLI commands, help text, or output messages.

## Table Output Format

When outputting data in tabular format, use tab-separated columns with the `text/tabwriter` package:

```go
w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
fmt.Fprintln(w, "Заголовок1\tЗаголовок2\tЗаголовок3")
fmt.Fprintln(w, "----------\t----------\t----------")
// data rows
w.Flush()
```

Key points:
- Use `\t` (tab character) to separate columns
- Use `tabwriter.Writer` for proper alignment
- Column headers should be in Russian
- Include a separator line with dashes after headers

## Output Rule

**All command execution results must be saved to the `output/` directory.** Each command should create a corresponding subdirectory (e.g., `output/dimension/`, `output/erosion/`, `output/source/`) and save:
- Generated files (SVG visualizations, CSV exports, GIF animations)
- Analysis results and metrics
- Logs and snapshots

Do not change output paths to other locations unless explicitly requested by the user.

## Documentation Rule

**Every code change requires documentation updates.** For any modification:
1. Add/update package documentation in the affected package (godoc comments)
2. Update the root `README.md` with relevant changes
3. Document new functions, types, or exported elements with Russian comments
4. **General documentation must be stored in `docs/` directory** (architecture guides, usage examples, installation instructions, etc.)

Do not skip documentation when implementing features or fixes.

## Project Overview

**Lito** is a Go CLI tool for coastal geomorphology modeling, specifically for analyzing Black Sea coastline geometry. The tool combines:
- Fractal analysis via box-counting dimension
- Wave erosion modeling with bathymetry support
- Benchmark calibration against observed erosion data

Module name: `coastal-geometry`

## Build and Test Commands

```bash
# Build
go build -o lito ./cmd/lito
# or
make build

# Run
./lito --help

# Tests
go test ./...
go test -v ./internal/domain/geometry/... -run TestSpecific

# Format check
gofmt -l .  # list unformatted files
gofmt -w .  # format in-place
```

## Architecture

### Entry Point
`cmd/lito/main.go` → `internal/cli/cobra.Execute()`

### CLI Layer (`internal/cli/cobra/`)
Commands use `github.com/spf13/cobra`:
- `source` - inspect and validate coastline data sources
- `dimension` - fractal box-counting analysis
- `erosion` - wave erosion simulation
- `all` - full pipeline (validation + fractal + erosion)
- `benchmark` - calibration sites and parameter optimization

Each command file follows the pattern: flag variables in `init()`, command logic in `run*()` function.

### Domain Layer (`internal/domain/`)

**coastline/** - Data loading and validation
- `source.go` - GeoJSON fetching, caching, snapshot generation
- `validation.go` - geometric and topological validation
- `data.go` - coordinate parsing from GeoJSON/JSON arrays
- Default data: Black Sea coastline from remote URL with local fallback

**geometry/** - Core geometric operations
- `erosion.go`, `sediment*.go` - wave erosion physics
- `length.go`, `area.go` - geodesic measurements
- `haversine.go` - great-circle distance calculations
- `simplify.go` - polyline simplification (Visvalingam)
- `bathymetry.go` - depth grid integration
- `lithology.go` - rock type erosion resistance
- `temporal.go` - storm dynamics and sea-level rise
- `types.go` - `LatLon{Lat, Lon float64}` is the core coordinate type

**fractal/** - Box-counting dimension analysis
- `dimension.go` - `AnalyzeBoxCounting()` returns fractal D with stability metrics

**benchmark/** - Model calibration
- `repository.go` - Black Sea calibration sites
- `calibration.go` - parameter search vs observed erosion
- `types.go` - `BenchmarkSite` with `ErosionObservation` records

**generators/koch/** - Organic Koch curve generation for fractal validation

### Render Layer (`internal/render/svg/`)
SVG visualization with multi-layer rendering, stat cards, charts, scale bars. Key function: `DrawSVG()`.

### Concurrency (`internal/pkg/concurrency/`)
Worker pool utilities for parallel geometric processing.

## Key Data Flows

1. **Coastline loading**: `coastline.Load()` → local file or remote URL with caching → `[]geometry.LatLon`
2. **Fractal analysis**: Base coastline → `koch.OrganicKochCurve()` (iterative growth) → `fractal.AnalyzeBoxCounting()`
3. **Erosion**: Coastline → `geometry.SimulateWaveErosionWithSeed()` → snapshots array with optional temporal dynamics

## Important Constants and Defaults

- Default coastline path: `data/black-sea-coastline.geojson`
- Default bathymetry: `data/black-sea-bathymetry.json`
- Koch theoretical D: `log(4)/log(3) ≈ 1.262`
- Default output: `./output/`

## Testing Patterns

Tests use standard Go testing (`_test.go` files). For geometry tests, fixtures are often small synthetic coastlines.
