package cli

func executeCommand(app *App) error {
	switch app.Config.Command {
	case cmdSource:
		return runSourceCommand(app)
	case cmdAll:
		return runAllCommand(app)
	case cmdCoastline:
		return runCoastlineCommand(app)
	case cmdDimension:
		return runDimensionCommand(app)
	case cmdErosion:
		return runErosionCommand(app)
	default:
		return errUnsupportedCommand(app.Config.Command)
	}
}
