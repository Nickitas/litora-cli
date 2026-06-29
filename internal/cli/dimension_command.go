package cli

import (
	"coastal-geometry/internal/domain/fractal"
	"coastal-geometry/internal/domain/generators/koch"
	"coastal-geometry/internal/domain/geometry"
	"fmt"
	"math"
)

const (
	theoryConvergenceTolerance = 0.05
	iterationConvergenceDelta  = 0.03
	minConvergedIterations     = 3
)

type dimensionIterationResult struct {
	Iteration int
	Analysis  fractal.BoxCountingAnalysis
}

type dimensionAssessment struct {
	Valid bool
}

func organicKochOptions(app *App) koch.OrganicOptions {
	return koch.OrganicOptions{
		Seed:            app.Config.Seed,
		AngleJitterDeg:  app.Config.AngleJitter,
		HeightJitterPct: app.Config.HeightJitter,
	}
}

func runDimensionCommand(app *App) error {
	opts := organicKochOptions(app)
	if err := writeOrganicKochSVGSeries(app.Base, app.ModelBase, app.Config.Iterations, app.Config.OutputPath, opts, app.Config.ErosionStrength, "dimension_iter", "dimension", true, newExportContext(app), app.OutputPaths); err != nil {
		return err
	}
	assessment, err := runDimensionMetrics(app.ModelBase, app.Config.Iterations, opts)
	if err != nil {
		return err
	}
	if !assessment.Valid {
		printInvalidResult()
	}
	return nil
}

func runDimensionMetrics(base []geometry.LatLon, maxIterations int, opts koch.OrganicOptions) (dimensionAssessment, error) {
	theoreticalDimension := math.Log(4) / math.Log(3)

	fmt.Println("\n  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  ФРАКТАЛЬНАЯ РАЗМЕРНОСТЬ (BOX-COUNTING)")
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	fmt.Println("  ┌──────┬───────────┬───────────┬─────────┬──────────┬──────────┬───────────┬───────────┬──────┐")
	fmt.Println("  │ Итер │ Точек     │ Длина км  │ D       │ Масштаб  │ R²       │ Разброс   │ Δ к пред  │ Стаб │")
	fmt.Println("  ├──────┼───────────┼───────────┼─────────┼──────────┼──────────┼───────────┼───────────┼──────┤")

	results := make([]dimensionIterationResult, 0, maxIterations+1)
	prevDimension := 0.0
	prevValid := false
	for iter := 0; iter <= maxIterations; iter++ {
		curve := koch.OrganicKochCurve(base, iter, opts)
		length := geometry.PolylineLength(curve)
		analysis := fractal.AnalyzeBoxCounting(curve)
		results = append(results, dimensionIterationResult{Iteration: iter, Analysis: analysis})

		delta := "—"
		if prevValid && analysis.Valid {
			delta = fmt.Sprintf("%+.5f", analysis.Dimension-prevDimension)
		}

		stable := "no"
		dimensionValue := "n/a"
		rSquared := "n/a"
		spread := "n/a"
		if analysis.Valid {
			dimensionValue = fmt.Sprintf("%.5f", analysis.Dimension)
			rSquared = fmt.Sprintf("%.4f", analysis.RegressionRSquared)
			spread = fmt.Sprintf("%.4f", analysis.StabilitySpread)
			if analysis.StableAcrossScales {
				stable = "yes"
			}
			prevDimension = analysis.Dimension
			prevValid = true
		} else {
			prevValid = false
		}

		fmt.Printf("  │ %-4d │ %-9d │ %-9.0f │ %-7s │ %-8s │ %-8s │ %-9s │ %-9s │ %-4s │\n",
			iter, len(curve), length, dimensionValue, fmt.Sprint(len(analysis.Samples)), rSquared, spread, delta, stable)
	}

	fmt.Println("  └──────┴───────────┴───────────┴─────────┴──────────┴──────────┴───────────┴───────────┴──────┘")
	fmt.Println()

	return printDimensionAssessment(results, theoreticalDimension), nil
}

func printDimensionAssessment(results []dimensionIterationResult, theoreticalDimension float64) dimensionAssessment {
	valid := make([]dimensionIterationResult, 0, len(results))
	for _, result := range results {
		if result.Analysis.Valid {
			valid = append(valid, result)
		}
	}

	if len(valid) < minConvergedIterations {
		return dimensionAssessment{Valid: false}
	}

	tail := valid[len(valid)-minConvergedIterations:]
	convergedAcrossIterations := true
	for i := 1; i < len(tail); i++ {
		if math.Abs(tail[i].Analysis.Dimension-tail[i-1].Analysis.Dimension) > iterationConvergenceDelta {
			convergedAcrossIterations = false
			break
		}
	}

	stableAcrossScales := true
	for _, result := range tail {
		if !result.Analysis.StableAcrossScales {
			stableAcrossScales = false
			break
		}
	}

	finalDimension := tail[len(tail)-1].Analysis.Dimension
	deltaTheory := math.Abs(finalDimension - theoreticalDimension)

	if convergedAcrossIterations && stableAcrossScales && deltaTheory <= theoryConvergenceTolerance {
		return dimensionAssessment{Valid: true}
	}

	return dimensionAssessment{Valid: false}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
