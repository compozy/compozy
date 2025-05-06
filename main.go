package main

import (
	"os"

	compozy "github.com/compozy/compozy/cmd"
)

func main() {
	cmd := compozy.RootCmd()
	if err := cmd.Execute(); err != nil {
		// Exit with error code 1 if command execution fails
		os.Exit(1)
	}
}
