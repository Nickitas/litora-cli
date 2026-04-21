package cli

import (
	"coastal-geometry/internal/domain/coastline"
	"fmt"
	"strings"
)

func runAllCommand(app *App) error {
	invalid := false

	sanity := coastline.MainCalculation(app.Base, app.Dataset, app.DataSource)
	if sanity.Checked && !sanity.Valid {
		invalid = true
	}
	if err := writeCoastlineSVG(app.Base, app.RenderBase, app.Config.OutputPath, "coastline.svg", newExportContext(app)); err != nil {
		return err
	}

	runParadoxCommand(app)

	// Классическая фрактальная аппроксимация (Koch)
	if err := writeKochSVGSeries(app.Base, app.ModelBase, app.Config.Iterations, app.Config.OutputPath, app.Config.ErosionStrength, app.Config.Seed, newExportContext(app)); err != nil {
		return err
	}

	// Органическая фрактальная модель
	runKochOrganicMetrics(app.ModelBase, app.Config.Iterations, organicKochOptions(app))
	if err := writeOrganicKochSVGSeries(app.Base, app.ModelBase, app.Config.Iterations, app.Config.OutputPath, organicKochOptions(app), app.Config.ErosionStrength, "koch-organic_iter", "koch-organic", false, newExportContext(app)); err != nil {
		return err
	}

	// Анализ фрактальной размерности органической модели
	if err := writeOrganicKochSVGSeries(app.Base, app.ModelBase, app.Config.Iterations, app.Config.OutputPath, organicKochOptions(app), app.Config.ErosionStrength, "dimension-organic_iter", "dimension-organic", true, newExportContext(app)); err != nil {
		return err
	}

	assessment, err := runDimensionMetrics(app.ModelBase, app.Config.Iterations, organicKochOptions(app))
	if err != nil {
		return err
	}
	if !assessment.Valid {
		invalid = true
	}

	// Волновая эрозия с автоматическим использованием батиметрии
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ЭТАП 5: ВОЛНОВАЯ ЭРОЗИЯ С БАТИМЕТРИЕЙ")
	fmt.Println(strings.Repeat("=", 80))

	if err := runErosionCommand(app); err != nil {
		return err
	}

	if invalid {
		printInvalidResult()
	}
	return nil
}
