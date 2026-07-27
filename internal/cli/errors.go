package cli

import "fmt"

func errUnsupportedCommand(command string) error {
	return fmt.Errorf("unsupported command %q", command)
}
