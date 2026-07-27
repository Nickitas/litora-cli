package cli

import (
	"coastal-geometry/internal/domain/benchmark"
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
)

func runBenchmarkCommand(app *App) error {
	subcommand := app.Config.BenchmarkSubcommand
	if subcommand == "" || subcommand == "--help" || subcommand == "-h" {
		return runBenchmarkHelp()
	}

	repo := benchmark.NewRepository(app.Config.BenchmarkDir)

	switch subcommand {
	case "list":
		return runBenchmarkList(repo)
	case "init":
		return runBenchmarkInit(repo)
	case "show":
		return runBenchmarkShow(repo, app.Config.BenchmarkSiteID)
	case "create":
		return runBenchmarkCreate(repo, app)
	case "calibrate":
		return runBenchmarkCalibrate(repo, app.Config.BenchmarkSiteID, app.Config.OutputPath, app.Config.BathymetryPath)
	case "calibrate-all":
		return runBenchmarkCalibrateAll(repo, app.Config.OutputPath, app.Config.BathymetryPath, app.Config.SpectrumSpread)
	case "analyze":
		return runBenchmarkAnalyze(repo, app.Config.BenchmarkSiteID, app.Config.OutputPath, app.Config.BathymetryPath)
	case "hotspots":
		return runBenchmarkHotspots(repo, app.Config.BenchmarkSiteID, app.Config.ErosionStrength, app.Config.WaveDirection, app.Config.OutputPath, app.Config.BathymetryPath)
	case "scenarios":
		return runBenchmarkScenarios(repo, app.Config.BenchmarkSiteID, app.Config.ErosionStrength, app.Config.WaveDirection, app.Config.OutputPath, app.Config.BathymetryPath)
	case "extract":
		return runBenchmarkExtract(app)
	default:
		return fmt.Errorf("unknown benchmark subcommand: %s", subcommand)
	}
}

// runBenchmarkScenarios runs predefined climate scenarios and compares them
func runBenchmarkScenarios(repo *benchmark.Repository, siteID string, strength, waveDir float64, outputPath string, bathymetryPath string) error {
	if siteID == "" {
		return fmt.Errorf("specify site ID with --site")
	}

	site, err := repo.Load(siteID)
	if err != nil {
		return fmt.Errorf("load site: %w", err)
	}

	if strength <= 0 {
		strength = 10.0
	}
	if waveDir < 0 {
		waveDir = site.MeanWaveDirection
	}

	// Load bathymetry
	if bathymetryPath == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			bathymetryPath = "data/black-sea-bathymetry.json"
		}
	}
	var bathymetry *geometry.BathymetryGrid
	if bathymetryPath != "" {
		data, _ := os.ReadFile(bathymetryPath)
		grid, _ := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		bathymetry = grid
	}

	fmt.Printf("Scenario analysis for: %s\n", site.Name)
	fmt.Printf("Baseline parameters: strength=%.1f m, direction=%.0f°\n\n", strength, waveDir)

	scenarios := benchmark.DefaultScenarios(strength, waveDir)
	fmt.Printf("Running %d scenarios...\n\n", len(scenarios))

	results, err := benchmark.RunScenarios(*site, scenarios, bathymetry)
	if err != nil {
		return fmt.Errorf("run scenarios: %w", err)
	}

	// Print results table
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  SCENARIO RESULTS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("%-15s %-8s %-8s %-8s %-8s %-10s %-8s\n",
		"Scenario", "Mean", "Max", "Erod%", "Hotsp", "ErodedKm", "ΔLenKm")
	fmt.Println("───────────────────────────────────────────────────────────────────────")
	for _, r := range results {
		fmt.Printf("%-15s %6.2f   %6.2f   %5.1f%%  %5d   %8.2f   %+6.2f\n",
			r.Config.Name,
			r.MeanRetreatRate,
			r.MaxRetreatRate,
			r.ErodingFraction*100,
			r.HotspotCount,
			r.TotalErodedKm,
			r.CoastChangeKm,
		)
	}
	fmt.Println()

	// Diff vs baseline
	diffs := benchmark.CompareScenarios(results, site.Coastline)
	baseline := results[0]

	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  IMPACT ANALYSIS (delta vs baseline)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("%-15s %-10s %-10s %-10s %-10s %-10s\n",
		"Scenario", "ΔMean", "ΔMax", "ΔErod%", "HotspotΔ", "HotShift")
	fmt.Println("───────────────────────────────────────────────────────────────────────")
	for _, d := range diffs {
		fmt.Printf("%-15s %+8.2f   %+8.2f   %+8.1f   %+5d      %6.1f km\n",
			d.Modified.Name,
			d.MeanRetreatDelta,
			d.MaxRetreatDelta,
			d.ErodingFractionDelta*100,
			d.NewHotspotCount-d.LostHotspotCount,
			d.HotspotShiftKm,
		)
	}
	fmt.Println()

	// Summary insights
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  KEY INSIGHTS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	for _, d := range diffs {
		pctChange := 0.0
		if baseline.MeanRetreatRate > 0 {
			pctChange = d.MeanRetreatDelta / baseline.MeanRetreatRate * 100
		}
		fmt.Printf("  • %s: %+5.1f%% retreat vs baseline", d.Modified.Name, pctChange)
		if d.HotspotShiftKm > 1 {
			fmt.Printf(", hotspot shifted %.1f km", d.HotspotShiftKm)
		}
		fmt.Println()
	}
	fmt.Println()

	// Save report
	if outputPath != "" {
		report := struct {
			SiteID   string                     `json:"site_id"`
			Baseline benchmark.ScenarioResult   `json:"baseline"`
			Results  []benchmark.ScenarioResult `json:"results"`
			Diffs    []benchmark.ScenarioDiff   `json:"diffs"`
		}{
			SiteID:   site.ID,
			Baseline: baseline,
			Results:  results,
			Diffs:    diffs,
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("Scenario report saved to: %s\n", outputPath)
	}

	return nil
}

// runBenchmarkHotspots identifies erosion hotspots for a site
func runBenchmarkHotspots(repo *benchmark.Repository, siteID string, strength, waveDir float64, outputPath string, bathymetryPath string) error {
	if siteID == "" {
		return fmt.Errorf("specify site ID with --site")
	}

	site, err := repo.Load(siteID)
	if err != nil {
		return fmt.Errorf("load site: %w", err)
	}

	if strength <= 0 {
		strength = 10.0
	}
	if waveDir < 0 {
		waveDir = site.MeanWaveDirection
	}

	// Load bathymetry if available
	if bathymetryPath == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			bathymetryPath = "data/black-sea-bathymetry.json"
		}
	}
	var bathymetry *geometry.BathymetryGrid
	if bathymetryPath != "" {
		data, _ := os.ReadFile(bathymetryPath)
		grid, err := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		if err == nil {
			bathymetry = grid
		}
	}

	config := benchmark.DefaultCalibrationConfig()
	config.BathymetryGrid = bathymetry

	fmt.Printf("Hotspot analysis for: %s\n", site.Name)
	fmt.Printf("Parameters: strength=%.1f m, direction=%.0f°\n\n", strength, waveDir)

	rates := benchmark.SegmentRates(*site, config, strength, waveDir)
	hotspots := benchmark.FindHotspots(rates, site.Coastline, 5, 0.75)

	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  TOP EROSION HOTSPOTS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("%-5s %-10s %-10s %-12s %-12s %-10s\n",
		"Rank", "Lat", "Lon", "Mean (m/yr)", "Max (m/yr)", "Length km")
	fmt.Println("───────────────────────────────────────────────────────────────────────")
	for _, h := range hotspots {
		fmt.Printf("%-5d %-10.4f %-10.4f %-12.2f %-12.2f %-10.2f\n",
			h.Rank, h.Center.Lat, h.Center.Lon,
			h.MeanRetreatRate, h.MaxRetreatRate, h.LengthKm)
	}
	fmt.Println()

	// Overall stats
	var sumRate, maxRate float64
	var erodingCount int
	for _, r := range rates {
		if r.RetreatRate > 0 {
			sumRate += r.RetreatRate
			erodingCount++
			if r.RetreatRate > maxRate {
				maxRate = r.RetreatRate
			}
		}
	}
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  OVERALL EROSION STATISTICS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("  Total segments:        %d\n", len(rates))
	fmt.Printf("  Eroding segments:      %d (%.1f%%)\n",
		erodingCount, float64(erodingCount)/float64(len(rates))*100)
	if erodingCount > 0 {
		fmt.Printf("  Mean retreat (eroding): %.2f m/year\n", sumRate/float64(erodingCount))
	}
	fmt.Printf("  Max retreat rate:      %.2f m/year\n", maxRate)
	fmt.Printf("  Hotspots found:        %d\n", len(hotspots))
	fmt.Println()

	if outputPath != "" {
		report := struct {
			SiteID    string                  `json:"site_id"`
			Strength  float64                 `json:"strength"`
			Direction float64                 `json:"wave_direction"`
			Hotspots  []benchmark.Hotspot     `json:"hotspots"`
			Segments  []benchmark.SegmentRate `json:"segments"`
		}{
			SiteID:    site.ID,
			Strength:  strength,
			Direction: waveDir,
			Hotspots:  hotspots,
			Segments:  rates,
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("Hotspot report saved to: %s\n", outputPath)
	}

	return nil
}

// runBenchmarkAnalyze runs the full statistical analysis: calibration + sensitivity + bootstrap CI
func runBenchmarkAnalyze(repo *benchmark.Repository, siteID string, outputPath string, bathymetryPath string) error {
	if siteID == "" {
		return fmt.Errorf("specify site ID with --site")
	}

	site, err := repo.Load(siteID)
	if err != nil {
		return fmt.Errorf("load benchmark site %q: %w", siteID, err)
	}

	fmt.Printf("Full statistical analysis for: %s\n", site.Name)
	fmt.Printf("Coastline points: %d, Observations: %d\n\n", len(site.Coastline), len(site.ObservedErosion))

	// Auto-discover bathymetry
	if bathymetryPath == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			bathymetryPath = "data/black-sea-bathymetry.json"
		}
	}

	var bathymetry *geometry.BathymetryGrid
	if bathymetryPath != "" {
		data, _ := os.ReadFile(bathymetryPath)
		grid, err := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		if err == nil {
			bathymetry = grid
			fmt.Printf("Bathymetry: %s (%d points)\n", bathymetryPath, len(grid.Points))
		}
	}

	config := benchmark.DefaultCalibrationConfig()
	const bootstrapIter = 200

	fmt.Println("Running: calibration + sensitivity + bootstrap (200 iter) + null model comparison...")
	fmt.Println()

	var analysis benchmark.FullAnalysis
	var analysisErr error
	if bathymetry != nil {
		analysis, analysisErr = benchmark.RunFullAnalysisWithBathymetry(*site, config, bathymetry, bootstrapIter)
	} else {
		analysis, analysisErr = benchmark.RunFullAnalysis(*site, config, bootstrapIter)
	}
	if analysisErr != nil {
		return fmt.Errorf("analysis failed: %w", analysisErr)
	}

	// 1. Best fit
	best := analysis.BestFit
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  BEST FIT PARAMETERS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("  Erosion strength:  %.2f m\n", best.ErosionStrength)
	fmt.Printf("  Wave direction:    %.1f°\n", best.WaveDirection)
	fmt.Printf("  RMSE:              %.3f m/year\n", best.ValidationMetrics.RMSE)
	fmt.Printf("  MAE:               %.3f m/year\n", best.ValidationMetrics.MAE)
	fmt.Printf("  MBE:               %+.3f m/year\n", best.ValidationMetrics.MBE)
	fmt.Printf("  R²:                %.3f\n", best.ValidationMetrics.RSquared)
	fmt.Printf("  P-value:           %.4f\n", best.ValidationMetrics.PValue)
	fmt.Println()

	// 2. Sensitivity analysis
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  SENSITIVITY ANALYSIS (one-at-a-time)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("%-25s %10s %10s %10s %10s\n",
		"Parameter", "Best", "Best RMSE", "Worst RMSE", "Score")
	fmt.Println("───────────────────────────────────────────────────────────────────────")
	for _, s := range analysis.Sensitivities {
		fmt.Printf("%-25s %10.2f %10.3f %10.3f %10.3f\n",
			s.Parameter, s.BestValue, s.BestRMSE, s.WorstRMSE, s.SensitivityScore)
	}
	fmt.Println()
	for _, s := range analysis.Sensitivities {
		fmt.Printf("  %s:\n", s.Parameter)
		fmt.Printf("    Range tested:    %.2f to %.2f\n", s.Values[0], s.Values[len(s.Values)-1])
		fmt.Printf("    Sensitivity:     %.3f (0=insensitive, 1=highly sensitive)\n", s.SensitivityScore)
		fmt.Printf("    Local derivative: %+.4f (RMSE change per unit parameter)\n", s.LocalSensitivity)
		fmt.Println()
	}

	// 3. Bootstrap confidence intervals
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  BOOTSTRAP CONFIDENCE INTERVALS (200 iterations)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	printCI("Erosion strength (m)", analysis.StrengthCI)
	printCI("Wave direction (°)", analysis.WaveDirectionCI)
	fmt.Println()

	// 4. Null model comparison
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  NULL MODEL COMPARISON")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	nm := analysis.NullModel
	fmt.Printf("  Model RMSE:           %.3f m/year\n", nm.ModelRMSE)
	fmt.Printf("  Null model RMSE:      %.3f m/year (predict mean observed)\n", nm.MeanModelRMSE)
	fmt.Printf("  Skill score:          %+.3f (1=perfect, 0=as good as null, neg=worse)\n", nm.SkillScore)
	fmt.Printf("  Improvement vs null:  %+.1f%%\n", nm.Improvement)
	if nm.SkillScore > 0.2 {
		fmt.Println("  → Model clearly outperforms null model ✓")
	} else if nm.SkillScore > 0 {
		fmt.Println("  → Model slightly better than null model")
	} else {
		fmt.Println("  → Model performs worse than simply predicting mean ⚠️")
	}
	fmt.Println()

	// Save JSON report
	if outputPath != "" {
		data, err := json.MarshalIndent(analysis, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal analysis: %w", err)
		}
		if err := os.WriteFile(outputPath, data, 0o644); err != nil {
			return fmt.Errorf("write analysis: %w", err)
		}
		fmt.Printf("Full analysis saved to: %s\n", outputPath)
	}

	return nil
}

func printCI(name string, ci benchmark.ConfidenceInterval) {
	fmt.Printf("  %s:\n", name)
	fmt.Printf("    Best fit:       %.2f\n", ci.BestFit)
	fmt.Printf("    Mean ± StdDev:  %.2f ± %.2f\n", ci.Mean, ci.StdDev)
	fmt.Printf("    Median:         %.2f\n", ci.Median)
	fmt.Printf("    68%% CI:         [%.2f, %.2f]\n", ci.Lower68, ci.Upper68)
	fmt.Printf("    95%% CI:         [%.2f, %.2f]\n", ci.Lower95, ci.Upper95)
}

// siteCalibrationResult bundles a site with its best calibration result
type siteCalibrationResult struct {
	Site    benchmark.BenchmarkSite
	BestFit benchmark.CalibrationResultItem
}

// runBenchmarkCalibrateAll runs calibration for all benchmark sites
// and produces a comparison report
func runBenchmarkCalibrateAll(repo *benchmark.Repository, outputDir string, bathymetryPath string, spectrumSpread float64) error {
	sites, err := repo.LoadAll()
	if err != nil {
		return fmt.Errorf("load sites: %w", err)
	}
	if len(sites) == 0 {
		return fmt.Errorf("no benchmark sites found, run './lito benchmark init' first")
	}

	// Auto-discover bathymetry file if not specified
	if bathymetryPath == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			bathymetryPath = "data/black-sea-bathymetry.json"
		}
	}

	var bathymetry *geometry.BathymetryGrid
	if bathymetryPath != "" {
		data, err := os.ReadFile(bathymetryPath)
		if err != nil {
			return fmt.Errorf("read bathymetry: %w", err)
		}
		grid, err := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		if err != nil {
			return fmt.Errorf("load bathymetry: %w", err)
		}
		bathymetry = grid
		fmt.Printf("Bathymetry: %s (%d points)\n", bathymetryPath, len(grid.Points))
	}
	if spectrumSpread > 0 {
		fmt.Printf("Wave spectrum spread: %.1f° (Gaussian)\n", spectrumSpread)
	}
	fmt.Println()

	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  CALIBRATION OF ALL BENCHMARK SITES")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("%-22s %-10s %-10s %-8s %-8s %-8s %-8s\n",
		"Site", "Strength", "WaveDir", "RMSE", "MAE", "MBE", "R²")
	fmt.Println("───────────────────────────────────────────────────────────────────────")

	var allResults []siteCalibrationResult

	config := benchmark.DefaultCalibrationConfig()
	config.SpectrumSpreadDeg = spectrumSpread

	for _, site := range sites {
		var results []benchmark.CalibrationResultItem
		var err error
		if bathymetry != nil {
			results, err = benchmark.CalibrateWithBathymetry(site, config, bathymetry)
		} else {
			results, err = benchmark.Calibrate(site, config)
		}
		if err != nil {
			fmt.Printf("%-22s ERROR: %v\n", site.ID, err)
			continue
		}
		if len(results) == 0 {
			continue
		}

		best := results[0]
		allResults = append(allResults, siteCalibrationResult{Site: site, BestFit: best})

		fmt.Printf("%-22s %-10.1f %-10.0f %-8.3f %-8.3f %+8.3f %-8.3f\n",
			site.ID, best.ErosionStrength, best.WaveDirection,
			best.ValidationMetrics.RMSE, best.ValidationMetrics.MAE,
			best.ValidationMetrics.MBE, best.ValidationMetrics.RSquared)
	}

	fmt.Println()

	if len(allResults) == 0 {
		return fmt.Errorf("no successful calibration results")
	}

	// Compute aggregate statistics
	var sumRMSE, sumMAE, sumRSq float64
	var countSig int
	for _, r := range allResults {
		sumRMSE += r.BestFit.ValidationMetrics.RMSE
		sumMAE += r.BestFit.ValidationMetrics.MAE
		sumRSq += r.BestFit.ValidationMetrics.RSquared
		if r.BestFit.ValidationMetrics.Significant {
			countSig++
		}
	}
	n := len(allResults)

	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  AGGREGATE STATISTICS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("  Sites calibrated:       %d\n", n)
	fmt.Printf("  Mean RMSE:              %.3f m/year\n", sumRMSE/float64(n))
	fmt.Printf("  Mean MAE:               %.3f m/year\n", sumMAE/float64(n))
	fmt.Printf("  Mean R²:                %.3f\n", sumRSq/float64(n))
	fmt.Printf("  Significant fits (p<0.05): %d / %d\n", countSig, n)
	fmt.Println()

	// Save detailed report if requested
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
		summaryPath := outputDir + "/calibration_summary.json"
		if err := saveAllCalibrationSummary(allResults, summaryPath); err != nil {
			return err
		}
		fmt.Printf("Summary saved: %s\n", summaryPath)

		// Save individual site reports
		for _, r := range allResults {
			sitePath := fmt.Sprintf("%s/%s_calibration.json", outputDir, r.Site.ID)
			siteResults, _ := benchmark.CalibrateWithBathymetry(r.Site, config, bathymetry)
			if err := saveCalibrationReport(&r.Site, siteResults, sitePath); err == nil {
				fmt.Printf("  %s\n", sitePath)
			}
		}
	}

	return nil
}

// saveAllCalibrationSummary saves summary of all calibrations
func saveAllCalibrationSummary(results []siteCalibrationResult, path string) error {
	type summaryItem struct {
		SiteID      string                      `json:"site_id"`
		SiteName    string                      `json:"site_name"`
		BestErosion float64                     `json:"best_erosion_strength_m"`
		BestWaveDir float64                     `json:"best_wave_direction_deg"`
		Metrics     benchmark.ValidationMetrics `json:"metrics"`
	}
	type summary struct {
		TotalSites int           `json:"total_sites"`
		Items      []summaryItem `json:"items"`
	}

	s := summary{TotalSites: len(results)}
	for _, r := range results {
		s.Items = append(s.Items, summaryItem{
			SiteID:      r.Site.ID,
			SiteName:    r.Site.Name,
			BestErosion: r.BestFit.ErosionStrength,
			BestWaveDir: r.BestFit.WaveDirection,
			Metrics:     r.BestFit.ValidationMetrics,
		})
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func runBenchmarkHelp() error {
	fmt.Println("Usage: ./lito benchmark [subcommand] [flags]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  list           List all benchmark sites")
	fmt.Println("  init           Initialize standard benchmark sites for Black Sea")
	fmt.Println("  show           Show detailed information about a site")
	fmt.Println("  calibrate      Calibrate model parameters for a site")
	fmt.Println("  calibrate-all  Calibrate all sites and produce summary report")
	fmt.Println("  analyze        Full analysis: calibration + sensitivity + bootstrap CI + null model")
	fmt.Println("  hotspots       Identify top erosion hotspots along coast")
	fmt.Println("  extract        Extract coastline segment by coordinates")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --dir string           Directory for benchmark sites (default: data/benchmarks)")
	fmt.Println("  --site string          Site ID for show/calibrate/analyze/hotspots")
	fmt.Println("  --input string         Path to coastline JSON for extract")
	fmt.Println("  --output string        Output file/dir path")
	fmt.Println("  --bathymetry string    Path to bathymetry JSON (auto-detected if omitted)")
	fmt.Println("  --spectrum-spread N    Wave directional spread in deg (0=single, 30=mild, 60=wide)")
	fmt.Println("  --erosion-strength N   Strength in m for hotspots")
	fmt.Println("  --wave-direction N     Wave direction in deg for hotspots")
	fmt.Println("  --bounds-min-lat ..    Min latitude for extract")
	fmt.Println("  --bounds-max-lat ..    Max latitude for extract")
	fmt.Println("  --bounds-min-lon ..    Min longitude for extract")
	fmt.Println("  --bounds-max-lon ..    Max longitude for extract")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ./lito benchmark list")
	fmt.Println("  ./lito benchmark init")
	fmt.Println("  ./lito benchmark show --site=odessa-coast-ua")
	fmt.Println("  ./lito benchmark calibrate --site=kobuleti-ge")
	fmt.Println("  ./lito benchmark calibrate-all --spectrum-spread=30")
	fmt.Println("  ./lito benchmark analyze --site=kobuleti-ge --output=analysis.json")
	fmt.Println("  ./lito benchmark hotspots --site=kobuleti-ge --erosion-strength=15 --wave-direction=22")
	return nil
}

func runBenchmarkList(repo *benchmark.Repository) error {
	sites, err := repo.LoadAll()
	if err != nil {
		return fmt.Errorf("load benchmark sites: %w", err)
	}

	if len(sites) == 0 {
		fmt.Println("No benchmark sites found.")
		fmt.Println("\nInitialize standard sites with:")
		fmt.Println("  ./lito benchmark init")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tRegion\tCountry\tType\tQuality\tYears")
	fmt.Fprintln(w, "----\t------\t-------\t----\t-------\t-----")

	for _, site := range sites {
		years := fmt.Sprintf("%.0f-%.0f", site.ObservationYears.Min, site.ObservationYears.Max)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			site.ID,
			site.Region,
			site.Country,
			site.CoastType,
			site.DataQuality,
			years,
		)
	}
	w.Flush()

	return nil
}

func runBenchmarkInit(repo *benchmark.Repository) error {
	fmt.Println("Initializing standard benchmark sites...")
	fmt.Println()

	// Load full coastline data using the same loader as other commands
	// (uses cached GeoJSON with ~11k points instead of simplified black-sea.json)
	loadResult, err := coastline.Load(coastline.LoadOptions{
		RemoteURL: coastline.DefaultCoastlineGeoJSONURL,
	})
	if err != nil {
		return fmt.Errorf("load coastline: %w", err)
	}
	fmt.Printf("Loaded coastline: %s (%d points)\n\n", loadResult.Source, len(loadResult.Points))

	sites := benchmark.StandardSites()
	for _, site := range sites {
		fmt.Printf("  %s: %s (%s)\n", site.ID, site.Name, site.Country)

		// Extract segment for this site
		site.Coastline = benchmark.ExtractCoastline(loadResult.Points, site.Bounds.ToGeoBounds())

		if len(site.Coastline) < 2 {
			fmt.Printf("    ⚠️  Warning: No coastline points found in bounds\n")
			continue
		}

		// Attach real-world erosion observations from scientific publications
		site.ObservedErosion = benchmark.ObservationsForSite(site.ID)

		// Save site
		if err := repo.Save(site); err != nil {
			return fmt.Errorf("save site %s: %w", site.ID, err)
		}

		obsCount := len(site.ObservedErosion)
		if obsCount > 0 {
			fmt.Printf("    ✓ %d coastline points, %d erosion observations\n",
				len(site.Coastline), obsCount)
		} else {
			fmt.Printf("    ✓ %d coastline points extracted\n", len(site.Coastline))
		}
	}

	fmt.Println()
	fmt.Println("Benchmark sites initialized!")
	fmt.Println("\nUse:")
	fmt.Println("  ./lito benchmark list    - list all sites")
	fmt.Println("  ./lito benchmark show ID - show site details")

	return nil
}

func runBenchmarkShow(repo *benchmark.Repository, siteID string) error {
	if siteID == "" {
		return fmt.Errorf("specify site ID with --site")
	}

	site, err := repo.Load(siteID)
	if err != nil {
		return fmt.Errorf("load benchmark site %q: %w", siteID, err)
	}

	// Print site information
	fmt.Printf("┌─ %s ─────────────────────────────────────\n", site.ID)
	fmt.Printf("│ Name:        %s\n", site.Name)
	fmt.Printf("│ Region:      %s\n", site.Region)
	fmt.Printf("│ Country:     %s\n", site.Country)
	fmt.Printf("│ Type:        %s\n", site.CoastType)
	fmt.Printf("│ Lithology:   %s\n", site.DominantLithology)
	fmt.Printf("│ Quality:     %s\n", site.DataQuality)
	fmt.Printf("│ Years:       %.0f - %.0f\n", site.ObservationYears.Min, site.ObservationYears.Max)
	fmt.Printf("│\n")
	fmt.Printf("│ Bounds:\n")
	fmt.Printf("│   Lat: %.3f to %.3f\n", site.Bounds.MinLat, site.Bounds.MaxLat)
	fmt.Printf("│   Lon: %.3f to %.3f\n", site.Bounds.MinLon, site.Bounds.MaxLon)
	fmt.Printf("│\n")
	fmt.Printf("│ Wave Conditions:\n")
	fmt.Printf("│   Height:    %.1f m\n", site.MeanWaveHeight)
	fmt.Printf("│   Period:    %.1f s\n", site.MeanWavePeriod)
	fmt.Printf("│   Direction: %.0f°\n", site.MeanWaveDirection)
	fmt.Printf("│\n")
	fmt.Printf("│ Coastline Points: %d\n", len(site.Coastline))
	fmt.Printf("│ Erosion Observations: %d\n", len(site.ObservedErosion))
	fmt.Printf("│\n")
	fmt.Printf("│ Data Source: %s\n", site.DataSource)
	if len(site.References) > 0 {
		fmt.Printf("│ References:\n")
		for _, ref := range site.References {
			fmt.Printf("│   - %s\n", ref)
		}
	}
	fmt.Printf("└────────────────────────────────────────────\n")

	return nil
}

func runBenchmarkCalibrate(repo *benchmark.Repository, siteID string, outputPath string, bathymetryPath string) error {
	if siteID == "" {
		return fmt.Errorf("specify site ID with --site")
	}

	site, err := repo.Load(siteID)
	if err != nil {
		return fmt.Errorf("load benchmark site %q: %w", siteID, err)
	}

	if len(site.ObservedErosion) == 0 {
		return fmt.Errorf("site %q has no observed erosion data for calibration", siteID)
	}

	fmt.Printf("Calibrating model for site: %s\n", site.Name)
	fmt.Printf("Coastline points: %d, Observations: %d\n", len(site.Coastline), len(site.ObservedErosion))

	// Auto-discover bathymetry file if not specified
	if bathymetryPath == "" {
		if _, err := os.Stat("data/black-sea-bathymetry.json"); err == nil {
			bathymetryPath = "data/black-sea-bathymetry.json"
		}
	}

	// Load bathymetry if available
	var bathymetry *geometry.BathymetryGrid
	if bathymetryPath != "" {
		data, err := os.ReadFile(bathymetryPath)
		if err != nil {
			return fmt.Errorf("read bathymetry file: %w", err)
		}
		grid, err := geometry.LoadBathymetryFromJSON(data, geometry.BathymetryLoadOptions{})
		if err != nil {
			return fmt.Errorf("load bathymetry: %w", err)
		}
		bathymetry = grid
		fmt.Printf("Bathymetry: %s (%d grid points)\n", bathymetryPath, len(grid.Points))
	} else {
		fmt.Println("Bathymetry: NOT loaded (run 'make bathymetry' to download)")
	}
	fmt.Println()

	config := benchmark.DefaultCalibrationConfig()
	fmt.Printf("Parameter space: %d erosion strengths × %d wave directions = %d runs\n\n",
		len(config.ErosionStrengths), len(config.WaveDirections),
		len(config.ErosionStrengths)*len(config.WaveDirections))

	var results []benchmark.CalibrationResultItem
	if bathymetry != nil {
		results, err = benchmark.CalibrateWithBathymetry(*site, config, bathymetry)
	} else {
		results, err = benchmark.Calibrate(*site, config)
	}
	if err != nil {
		return fmt.Errorf("calibration failed: %w", err)
	}

	if len(results) == 0 {
		return fmt.Errorf("no calibration results produced")
	}

	// Print top 5 results
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  TOP 5 PARAMETER COMBINATIONS (sorted by RMSE)")
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Println("  Rank  Strength(m)  WaveDir(°)   RMSE    MAE    MBE     R²     Sig?")
	fmt.Println("  ----  ----------  ----------  ------  -----  -----  -----  ------")

	topN := 5
	if len(results) < topN {
		topN = len(results)
	}
	for i := 0; i < topN; i++ {
		r := results[i]
		sig := "no"
		if r.ValidationMetrics.Significant {
			sig = "yes ✓"
		}
		fmt.Printf("  %4d   %8.1f    %8.1f   %5.2f  %5.2f  %5.2f  %5.3f  %s\n",
			i+1,
			r.ErosionStrength,
			r.WaveDirection,
			r.ValidationMetrics.RMSE,
			r.ValidationMetrics.MAE,
			r.ValidationMetrics.MBE,
			r.ValidationMetrics.RSquared,
			sig,
		)
	}
	fmt.Println()

	// Best result details
	best := results[0]
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("  BEST FIT: erosion-strength=%.1f m, wave-direction=%.0f°\n", best.ErosionStrength, best.WaveDirection)
	fmt.Println("═══════════════════════════════════════════════════════════════════════")
	fmt.Printf("  RMSE:            %.3f m/year\n", best.ValidationMetrics.RMSE)
	fmt.Printf("  MAE:             %.3f m/year\n", best.ValidationMetrics.MAE)
	fmt.Printf("  MBE:             %+.3f m/year  (positive = model overestimates erosion)\n", best.ValidationMetrics.MBE)
	fmt.Printf("  R²:              %.3f\n", best.ValidationMetrics.RSquared)
	fmt.Printf("  P-value:         %.4f (significant: %v)\n", best.ValidationMetrics.PValue, best.ValidationMetrics.Significant)
	fmt.Printf("  N observations:  %d\n", best.ValidationMetrics.N)
	fmt.Println()

	// Per-observation breakdown for best
	fmt.Println("  Per-observation comparison (best fit):")
	fmt.Println("  Lat       Lon         Observed  Modeled  Error    Distance")
	fmt.Println("  ----      -----       --------  -------  ------   --------")
	for _, c := range best.ComparisonPoints {
		err := c.Modeled - c.Observed
		fmt.Printf("  %.4f    %.4f    %6.2f    %6.2f   %+6.2f   %5.2f km\n",
			c.LatLon.Lat, c.LatLon.Lon,
			c.Observed, c.Modeled, err, c.DistanceToCoastKm)
	}
	fmt.Println()

	// Quality assessment
	quality := assessCalibrationQuality(best.ValidationMetrics)
	fmt.Printf("  Assessment: %s\n", quality)

	// Save JSON report if requested
	if outputPath != "" {
		if err := saveCalibrationReport(site, results, outputPath); err != nil {
			return fmt.Errorf("save calibration report: %w", err)
		}
		fmt.Printf("\nCalibration report saved to: %s\n", outputPath)
	}

	return nil
}

// saveCalibrationReport writes a JSON report with all calibration results
func saveCalibrationReport(site *benchmark.BenchmarkSite, results []benchmark.CalibrationResultItem, outputPath string) error {
	report := struct {
		SiteID     string                            `json:"site_id"`
		SiteName   string                            `json:"site_name"`
		BestFit    benchmark.CalibrationResultItem   `json:"best_fit"`
		TopResults []benchmark.CalibrationResultItem `json:"top_results"`
		AllResults []benchmark.CalibrationResultItem `json:"all_results"`
	}{
		SiteID:     site.ID,
		SiteName:   site.Name,
		BestFit:    results[0],
		TopResults: results,
		AllResults: results,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0o644)
}

// assessCalibrationQuality returns a human-readable quality rating
func assessCalibrationQuality(m benchmark.ValidationMetrics) string {
	if m.RMSE < 0.3 && m.RSquared > 0.5 {
		return "★★★ EXCELLENT — model reproduces observed erosion patterns well"
	}
	if m.RMSE < 0.5 && m.RSquared > 0.3 {
		return "★★☆ GOOD — model captures main erosion patterns with acceptable error"
	}
	if m.RMSE < 1.0 {
		return "★☆☆ MODERATE — model has systematic bias, needs improvement"
	}
	return "☆☆☆ POOR — model does not reproduce observed erosion"
}

func runBenchmarkExtract(app *App) error {
	// Extract coastline segment for a custom bounds
	bounds := app.Config.Bounds

	if bounds.IsZero() {
		return fmt.Errorf("specify bounds with --bounds-min-lat, --bounds-max-lat, --bounds-min-lon, --bounds-max-lon")
	}

	loadResult, err := coastline.Load(coastline.LoadOptions{
		RemoteURL: coastline.DefaultCoastlineGeoJSONURL,
	})
	if err != nil {
		return fmt.Errorf("load coastline: %w", err)
	}

	segment := benchmark.ExtractCoastline(loadResult.Points, bounds)

	if len(segment) < 2 {
		return fmt.Errorf("no coastline points found in specified bounds")
	}

	fmt.Printf("Loaded: %s (%d total points)\n", loadResult.Source, len(loadResult.Points))
	fmt.Printf("Extracted %d points from bounds:\n", len(segment))
	fmt.Printf("  Lat: %.3f to %.3f\n", bounds.MinLat, bounds.MaxLat)
	fmt.Printf("  Lon: %.3f to %.3f\n", bounds.MinLon, bounds.MaxLon)

	// Save to file if requested
	if app.Config.OutputPath != "" {
		data, err := json.MarshalIndent(segment, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal segment: %w", err)
		}

		if err := os.WriteFile(app.Config.OutputPath, data, 0o644); err != nil {
			return fmt.Errorf("write segment: %w", err)
		}

		fmt.Printf("Saved to: %s\n", app.Config.OutputPath)
	}

	return nil
}

// runBenchmarkCreate creates a new benchmark site from user-supplied parameters
func runBenchmarkCreate(repo *benchmark.Repository, app *App) error {
	cfg := app.Config

	if cfg.SiteName == "" {
		return fmt.Errorf("--name is required")
	}
	if cfg.BoundsString == "" {
		return fmt.Errorf("--bounds is required (format: min_lat,max_lat,min_lon,max_lon)")
	}

	bounds, err := benchmark.ParseBounds(cfg.BoundsString)
	if err != nil {
		return fmt.Errorf("parse bounds: %w", err)
	}

	coastType, err := benchmark.PresetCoastType(cfg.CoastTypeStr)
	if err != nil {
		return err
	}
	quality, err := benchmark.PresetQuality(cfg.QualityStr)
	if err != nil {
		return err
	}

	spec := benchmark.SiteSpec{
		Name:              cfg.SiteName,
		Region:            cfg.SiteRegion,
		Country:           cfg.SiteCountry,
		Description:       cfg.SiteDescription,
		Bounds:            bounds,
		CoastType:         coastType,
		DominantLithology: cfg.SiteLithology,
		MeanWaveHeight:    cfg.MeanWaveHeight,
		MeanWavePeriod:    cfg.MeanWavePeriod,
		MeanWaveDirection: cfg.WaveDirection,
		DataQuality:       quality,
		ObservationYears: benchmark.Range{
			Min: float64(cfg.ObsYearMin),
			Max: float64(cfg.ObsYearMax),
		},
	}

	if err := spec.Validate(); err != nil {
		return fmt.Errorf("invalid spec: %w", err)
	}

	fmt.Printf("Creating benchmark site: %s (id=%s)\n", spec.Name, spec.ID)
	fmt.Printf("Bounds: lat [%.3f, %.3f], lon [%.3f, %.3f]\n",
		bounds.MinLat, bounds.MaxLat, bounds.MinLon, bounds.MaxLon)

	// Try to load coastline
	var fullCoastline []geometry.LatLon
	if cfg.CoastlinePath != "" {
		data, err := os.ReadFile(cfg.CoastlinePath)
		if err == nil {
			pts, _, err := coastline.LoadFromJSON(string(data))
			if err == nil {
				fullCoastline = pts
				fmt.Printf("Loaded coastline: %s (%d points)\n", cfg.CoastlinePath, len(pts))
			}
		}
	}
	if fullCoastline == nil {
		loadResult, err := coastline.Load(coastline.LoadOptions{
			RemoteURL: coastline.DefaultCoastlineGeoJSONURL,
		})
		if err == nil {
			fullCoastline = loadResult.Points
			fmt.Printf("Loaded coastline: %s (%d points)\n", loadResult.Source, len(loadResult.Points))
		} else {
			fmt.Printf("⚠️  Could not load coastline: %v\n", err)
			fmt.Println("    Site will be created without coastline data.")
			fmt.Println("    Add coastline manually or use --coastline flag.")
		}
	}

	site := spec.Build(fullCoastline)
	fmt.Printf("Extracted %d coastline points within bounds\n", len(site.Coastline))

	if err := repo.Save(site); err != nil {
		return fmt.Errorf("save site: %w", err)
	}
	fmt.Println()
	fmt.Printf("✓ Site created: %s\n", site.ID)
	fmt.Printf("  Saved to: %s/%s.json\n", cfg.BenchmarkDir, site.ID)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  ./lito benchmark show --site=%s\n", site.ID)
	fmt.Printf("  ./lito benchmark calibrate --site=%s\n", site.ID)
	fmt.Println()
	fmt.Println("Note: Site has no observed erosion data yet.")
	fmt.Println("      Add observation data manually to the JSON file to enable calibration.")

	return nil
}
