package main

import (
	"fmt"
	"os"
	"path/filepath"

	config "github.com/compozy/compozy/internal/config"
)

func main() {
	homeDir, err := config.ResolveHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve CompozyOS home for Air state: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(filepath.Join(homeDir, ".dev", "air"))
}
