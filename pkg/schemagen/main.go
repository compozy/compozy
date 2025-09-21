package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/pkg/logger"
)

func main() {
	outDir := flag.String("out", "./schemas", "output directory for generated schemas")
	watch := flag.Bool("watch", false, "watch config files and regenerate schemas on changes")
	logLevel := flag.String("log-level", "info", "log level (debug, info, warn, error)")
	logJSON := flag.Bool("log-json", false, "output logs in JSON format")
	logSource := flag.Bool("log-source", false, "include source code location in logs")
	flag.Parse()
	level := parseLogLevel(*logLevel)
	log := logger.SetupLogger(level, *logJSON, *logSource)
	ctx := logger.ContextWithLogger(context.Background(), log)
	absOutDir, err := filepath.Abs(*outDir)
	if err != nil {
		log.Error("Error converting path to absolute", "error", err)
		os.Exit(1)
	}
	generator := NewSchemaGenerator()
	if err := generator.Generate(ctx, absOutDir); err != nil {
		log.Error("Error generating schemas", "error", err)
		os.Exit(1)
	}
	if *watch {
		watcher := NewSchemaWatcher(generator)
		if err := watcher.Watch(ctx, absOutDir); err != nil {
			log.Error("Error watching config files", "error", err)
			os.Exit(1)
		}
	}
}

func parseLogLevel(value string) logger.LogLevel {
	switch value {
	case "debug":
		return logger.DebugLevel
	case "warn":
		return logger.WarnLevel
	case "error":
		return logger.ErrorLevel
	default:
		return logger.InfoLevel
	}
}
