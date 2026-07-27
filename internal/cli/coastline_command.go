package cli

import "coastal-geometry/internal/domain/coastline"

func runCoastlineCommand(app *App) error {
	sanity := coastline.MainCalculation(app.Base, app.Dataset, app.DataSource)
	if sanity.Checked && !sanity.Valid {
		printInvalidResult()
	}
	return writeCoastlineSVG(app.Base, app.RenderBase, app.Config.OutputPath, "coastline.svg", newExportContext(app), app.OutputPaths)
}
