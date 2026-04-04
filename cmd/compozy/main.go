package main

import (
	"os"

	"github.com/compozy/compozy/command"
)

func main() {
	if err := command.New().Execute(); err != nil {
		os.Exit(command.ExitCode(err))
	}
}
