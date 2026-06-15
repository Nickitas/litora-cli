package cli

import (
	"coastal-geometry/internal/domain/coastline"
	"fmt"
	"io"
	"os"
)

func Run(args []string, stdout, stderr io.Writer) {
	cfg, err := parseConfig(args, stdout, stderr)
	if err != nil {
		if isHelp(err) {
			return
		}
		exitWithError(stderr, err)
	}

	// Show banner only if not quiet and not a help request
	if !cfg.Quiet && !isHelpRequest(args) {
		printBanner(stdout)
	}

	app, err := NewApp(cfg)
	if err != nil {
		exitWithError(stderr, err)
	}

	printValidationReport(stdout, app.Validation)

	if err := executeCommand(app); err != nil {
		exitWithError(stderr, err)
	}
}

func exitWithError(stderr io.Writer, err error) {
	fmt.Fprintf(stderr, "error: %v\n", err)
	os.Exit(1)
}

func isHelpRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}

	arg := args[0]
	switch arg {
	case "-h", "--help", "help":
		return true
	default:
		return false
	}
}

func printValidationReport(w io.Writer, report coastline.ValidationReport) {
	for _, fix := range report.Fixes {
		fmt.Fprintf(w, "fix: %s\n", fix)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warning)
	}
}

func printLoadNotes(w io.Writer, app *App) {
	if app == nil || app.DataSource == "" {
		return
	}

	label := "coastline source"
	if app.Config.Command == cmdSource {
		label = "dataset source"
	}

	fmt.Fprintf(w, "info: %s: %s\n", label, app.DataSource)
	for _, note := range app.LoadNotes {
		fmt.Fprintf(w, "warning: %s\n", note)
	}
}

func printProcessNotes(w io.Writer, app *App) {
	if app == nil {
		return
	}

	for _, note := range app.ProcessNotes {
		fmt.Fprintf(w, "info: %s\n", note)
	}
}

func printCommandUX(w io.Writer, command string) {
	ux := getCommandUX(command)
	if ux.Mode == "" {
		return
	}

	fmt.Fprintf(w, "info: canonical command: %s\n", canonicalCommandPath(command))
	fmt.Fprintf(w, "info: command mode: %s\n", ux.Mode)
	if ux.RuntimeNote != "" {
		fmt.Fprintf(w, "info: %s\n", ux.RuntimeNote)
	}
}
