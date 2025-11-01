package compozy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	yamlv3 "gopkg.in/yaml.v3"
)

var yamlExtensions = []string{".yaml", ".yml"}

const defaultFilteredFilesCapacity = 8

func requireContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	return contextError(ctx)
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("operation canceled: %w", err)
	}
	return nil
}

type filePathSetter interface {
	SetFilePath(string)
}

type cwdSetter interface {
	SetCWD(string) error
}

func loadYAML[T any](ctx context.Context, engine *Engine, path string) (T, string, error) {
	var zero T
	log := logger.FromContext(ctx)
	logFailure := func(target string, err error) {
		if err == nil || log == nil {
			return
		}
		log.Error("failed to load YAML", "path", target, "error", err)
	}
	requestedPath := strings.TrimSpace(path)
	if err := requireContext(ctx); err != nil {
		logFailure(requestedPath, err)
		return zero, "", err
	}
	trimmed, cfg, err := prepareLoadContext(engine, path)
	if err != nil {
		logFailure(requestedPath, err)
		return zero, "", err
	}
	if err := contextError(ctx); err != nil {
		logFailure(trimmed, err)
		return zero, "", err
	}
	data, abs, err := readYAMLFile(trimmed, cfg.Limits.MaxConfigFileSize)
	if err != nil {
		logFailure(trimmed, err)
		return zero, "", err
	}
	if err := contextError(ctx); err != nil {
		logFailure(trimmed, err)
		return zero, "", err
	}
	value, err := decodeYAML[T](data, trimmed)
	if err != nil {
		logFailure(trimmed, err)
		return zero, "", err
	}
	if err := contextError(ctx); err != nil {
		logFailure(trimmed, err)
		return zero, "", err
	}
	applyYAMLMetadata(engine.ctx, value, abs)
	return value, abs, nil
}

func prepareLoadContext(engine *Engine, rawPath string) (string, *appconfig.Config, error) {
	if engine == nil {
		return "", nil, fmt.Errorf("engine is nil")
	}
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", nil, fmt.Errorf("path is required")
	}
	if engine.ctx == nil {
		return "", nil, fmt.Errorf("engine context is not set")
	}
	cfg := appconfig.FromContext(engine.ctx)
	if cfg == nil {
		return "", nil, fmt.Errorf("configuration unavailable")
	}
	return trimmed, cfg, nil
}

func readYAMLFile(path string, maxSize int) ([]byte, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", fmt.Errorf("stat %s: %w", path, err)
	}
	if limit := int64(maxSize); limit > 0 && info.Size() > limit {
		return nil, "", fmt.Errorf("%s exceeds maximum size of %d bytes", path, maxSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return data, abs, nil
}

func decodeYAML[T any](data []byte, path string) (T, error) {
	var value T
	if err := yamlv3.Unmarshal(data, &value); err != nil {
		return value, fmt.Errorf("decode %s: %w", path, err)
	}
	return value, nil
}

func applyYAMLMetadata(ctx context.Context, value any, abs string) {
	if abs == "" {
		return
	}
	if setter, ok := value.(filePathSetter); ok {
		setter.SetFilePath(abs)
	}
	if setter, ok := value.(cwdSetter); ok {
		if err := setter.SetCWD(filepath.Dir(abs)); err != nil {
			log := logger.FromContext(ctx)
			if log != nil {
				log.Error("failed to set cwd", "path", abs, "error", err)
			}
		}
	}
}

func filteredYAMLFiles(ctx context.Context, dir string) ([]string, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}
	filesCap := len(entries)
	if filesCap < defaultFilteredFilesCapacity {
		filesCap = defaultFilteredFilesCapacity
	}
	files := make([]string, 0, filesCap)
	for _, entry := range entries {
		if err := contextError(ctx); err != nil {
			return nil, err
		}
		if entry.IsDir() {
			continue
		}
		if !isYAMLExtension(entry.Name()) {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func (e *Engine) loadFromDir(ctx context.Context, dir string, loader func(context.Context, string) error) error {
	if e == nil {
		return fmt.Errorf("engine is nil")
	}
	if err := requireContext(ctx); err != nil {
		return err
	}
	cleaned := strings.TrimSpace(dir)
	if cleaned == "" {
		return fmt.Errorf("directory is required")
	}
	files, err := filteredYAMLFiles(ctx, cleaned)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	var errs []error
	log := logger.FromContext(e.ctx)
	for _, file := range files {
		if err := contextError(ctx); err != nil {
			return err
		}
		if err := loader(ctx, file); err != nil {
			wrapped := fmt.Errorf("%s: %w", file, err)
			if log != nil {
				log.Error("failed to load yaml file", "path", file, "error", err)
			}
			errs = append(errs, wrapped)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func isYAMLExtension(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range yamlExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
