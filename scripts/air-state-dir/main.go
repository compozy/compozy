package main

import (
	"fmt"
	"os"
	"path/filepath"

	aghconfig "github.com/compozy/agh/internal/config"
)

func main() {
	homeDir, err := aghconfig.ResolveHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve AGH home for Air state: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(filepath.Join(homeDir, ".dev", "air"))
}
