package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if err := sdkcodegen.Generate(ctx, absolute); err != nil {
		cancel()
		log.Fatalf("sdkcodegen: %v", err)
	}
	cancel()
	fmt.Printf("sdkcodegen: generated files in %s\n", absolute)
}
