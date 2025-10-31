package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/sdk/v2/internal/sdkcodegen"
)

func main() {
	outDir := flag.String("out", "", "output directory for generated files")
	flag.Parse()
	if strings.TrimSpace(*outDir) == "" {
		log.Fatal("sdkcodegen: -out directory is required")
	}
	absolute, err := filepath.Abs(*outDir)
	if err != nil {
		log.Fatalf("sdkcodegen: resolve output path: %v", err)
	}
	if err := sdkcodegen.Generate(absolute); err != nil {
		log.Fatalf("sdkcodegen: %v", err)
	}
	fmt.Printf("sdkcodegen: generated files in %s\n", absolute)
}
