package main

import (
	"context"
	"fmt"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	rw "github.com/radovskyb/watcher"

	"github.com/compozy/compozy/pkg/logger"
)

type SchemaWatcher struct {
	generator *SchemaGenerator
	debounce  time.Duration
}

func NewSchemaWatcher(generator *SchemaGenerator) *SchemaWatcher {
	return &SchemaWatcher{generator: generator, debounce: 500 * time.Millisecond}
}

func (w *SchemaWatcher) Watch(ctx context.Context, outDir string) error {
	log := logger.FromContext(ctx)
	watch := rw.New()
	if err := w.configureWatcher(watch); err != nil {
		return err
	}
	log.Info("Starting file watcher for config changes. Press Ctrl+C to exit.")
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	events := w.forwardWatcherEvents(ctx, watch, log)
	w.startWatcher(log, watch)
	w.consumeEvents(ctx, events, outDir, log)
	return nil
}

func (w *SchemaWatcher) configureWatcher(watch *rw.Watcher) error {
	watch.SetMaxEvents(1)
	watch.IgnoreHiddenFiles(true)
	if err := watch.AddRecursive("engine"); err != nil {
		return fmt.Errorf("failed to add recursive watch for engine directory: %w", err)
	}
	watch.FilterOps(rw.Write, rw.Create)
	goFileRegex := regexp.MustCompile(`\\.go$`)
	watch.AddFilterHook(rw.RegexFilterHook(goFileRegex, false))
	return nil
}

func (w *SchemaWatcher) forwardWatcherEvents(
	ctx context.Context,
	watch *rw.Watcher,
	log logger.Logger,
) <-chan struct{} {
	events := make(chan struct{}, 1)
	go func() {
		defer close(events)
		for {
			select {
			case event, ok := <-watch.Event:
				if !ok {
					return
				}
				if event.Op != rw.Write && event.Op != rw.Create {
					continue
				}
				log.Debug("Config file modified", "file", event.Path, "op", event.Op)
				select {
				case events <- struct{}{}:
				default:
				}
			case err, ok := <-watch.Error:
				if !ok {
					return
				}
				log.Error("File watcher error", "error", err)
			case <-ctx.Done():
				watch.Close()
				return
			}
		}
	}()
	return events
}

func (w *SchemaWatcher) startWatcher(log logger.Logger, watch *rw.Watcher) {
	go func() {
		if err := watch.Start(200 * time.Millisecond); err != nil {
			log.Error("Failed to start watcher", "error", err)
		}
	}()
}

func (w *SchemaWatcher) consumeEvents(
	ctx context.Context,
	events <-chan struct{},
	outDir string,
	log logger.Logger,
) {
	var debounceTimer *time.Timer
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(w.debounce, func() {
				w.runGeneration(ctx, outDir, log)
			})
		case <-ctx.Done():
			return
		}
	}
}

func (w *SchemaWatcher) runGeneration(ctx context.Context, outDir string, log logger.Logger) {
	log.Info("Regenerating schemas due to config changes")
	if err := w.generator.Generate(ctx, outDir); err != nil {
		log.Error("Error regenerating schemas", "error", err)
		return
	}
	log.Info("Schemas regenerated successfully")
}
