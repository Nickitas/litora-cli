package cobra

import (
	"testing"

	spfcobra "github.com/spf13/cobra"
)

func TestCLIExposesOnlyBlackSeaDomain(t *testing.T) {
	for _, command := range rootCmd.Commands() {
		if command.Name() == "waterbody" {
			t.Fatal("команда выбора других акваторий не должна присутствовать")
		}
	}

	for _, command := range []*spfcobra.Command{erosionCmd, allCmd, cercCalibrationCmd} {
		if command.Flags().Lookup("waterbody") != nil {
			t.Fatalf("команда %s не должна содержать флаг --waterbody", command.Name())
		}
	}
}
