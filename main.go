package main

import (
	"os"

	"github.com/compozy/compozy/cli"
)

func main() {
	cmd := cli.RootCmd()
	if err := cmd.Execute(); err != nil {
		// Exit with error code 1 if command execution fails
		os.Exit(1)
	}
}
