package main

import (
	"fmt"
	"os"

	"github.com/compozy/compozy/pkg/release/cmd"
)

func main() {
	if err := cmd.InitCommands(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize commands: %v\n", err)
		os.Exit(1)
	}
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
