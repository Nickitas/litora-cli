package cli

import (
	"coastal-geometry/internal/domain/coastline"
)

func runAllCommand(app *App) error {
	invalid := false

	// 1. Базовая валидация и метрики береговой линии
	sanity := coastline.MainCalculation(app.Base, app.Dataset, app.DataSource)
	if sanity.Checked && !sanity.Valid {
		invalid = true
	}
	if err := writeCoastlineSVG(app.Base, app.RenderBase, app.Config.OutputPath, "coastline.svg", newExportContext(app)); err != nil {
		return err
	}

	// 2. Анализ фрактальной размерности (научно ценный компонент)
	// Используем упрощенную органическую модель для анализа
	assessment, err := runDimensionMetrics(app.ModelBase, app.Config.Iterations, organicKochOptions(app))
	if err != nil {
		return err
	}
	if !assessment.Valid {
		invalid = true
	}

	// 3. Волновая эрозия с физически обоснованной моделью
	if err := runErosionCommand(app); err != nil {
		return err
	}

	if invalid {
		printInvalidResult()
	}
	return nil
}
