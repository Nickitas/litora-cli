package main

import (
	"coastal-geometry/internal/cli"
	"os"
)

func main() {
	cli.Run(os.Args[1:], os.Stdout, os.Stderr)
}
