package compozy

import (
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

type filePathSetter interface {
	SetFilePath(string)
}

type cwdSetter interface {
	SetCWD(string) error
}

func loadYAML[T any](engine *Engine, path string) (T, string, error) {
	var zero T
	if engine == nil {
		return zero, "", fmt.Errorf("engine is nil")
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return zero, "", fmt.Errorf("path is required")
	}
	if engine.ctx == nil {
		return zero, "", fmt.Errorf("engine context is not set")
	}
	cfg := appconfig.FromContext(engine.ctx)
	if cfg == nil {
		return zero, "", fmt.Errorf("configuration unavailable")
	}
	info, err := os.Stat(trimmed)
	if err != nil {
		return zero, "", fmt.Errorf("stat %s: %w", trimmed, err)
	}
	if max := cfg.Limits.MaxConfigFileSize; max > 0 && info.Size() > int64(max) {
		return zero, "", fmt.Errorf("%s exceeds maximum size of %d bytes", trimmed, max)
	}
	data, err := os.ReadFile(trimmed)
	if err != nil {
		return zero, "", fmt.Errorf("read %s: %w", trimmed, err)
	}
	var value T
	if err := yamlv3.Unmarshal(data, &value); err != nil {
		return zero, "", fmt.Errorf("decode %s: %w", trimmed, err)
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		abs = trimmed
	}
	if setter, ok := any(value).(filePathSetter); ok && abs != "" {
		setter.SetFilePath(abs)
	}
	if setter, ok := any(value).(cwdSetter); ok && abs != "" {
		if err := setter.SetCWD(filepath.Dir(abs)); err != nil {
			log := logger.FromContext(engine.ctx)
			if log != nil {
				log.Error("failed to set cwd", "path", abs, "error", err)
			}
		}
	}
	return value, abs, nil
}

func (e *Engine) loadFromDir(dir string, loader func(string) error) error {
	if e == nil {
		return fmt.Errorf("engine is nil")
	}
	cleaned := strings.TrimSpace(dir)
	if cleaned == "" {
		return fmt.Errorf("directory is required")
	}
	entries, err := os.ReadDir(cleaned)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", cleaned, err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isYAMLExtension(entry.Name()) {
			continue
		}
		files = append(files, filepath.Join(cleaned, entry.Name()))
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil
	}
	var errs []error
	log := logger.FromContext(e.ctx)
	for _, file := range files {
		if err := loader(file); err != nil {
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
