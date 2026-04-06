package main

import (
	"os"

	"github.com/compozy/compozy/command"
	"github.com/compozy/compozy/internal/version"
)

func main() {
	cmd := command.New()
	cmd.Version = version.String()
	if err := cmd.Execute(); err != nil {
		os.Exit(command.ExitCode(err))
	}
}
