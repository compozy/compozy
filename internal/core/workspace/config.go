package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	toml "github.com/pelletier/go-toml/v2"
)

func Resolve(ctx context.Context, startDir string) (Context, error) {
	root, err := Discover(ctx, startDir)
	if err != nil {
		return Context{}, err
	}

	cfg, configPath, err := LoadConfig(ctx, root)
	if err != nil {
		return Context{}, err
	}

	return Context{
		Root:       root,
		CompozyDir: model.CompozyDir(root),
		ConfigPath: configPath,
		Config:     cfg,
	}, nil
}

func Discover(ctx context.Context, startDir string) (string, error) {
	if err := context.Cause(ctx); err != nil {
		return "", fmt.Errorf("discover workspace: %w", err)
	}

	resolvedStart := strings.TrimSpace(startDir)
	if resolvedStart == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		resolvedStart = cwd
	}

	absStart, err := filepath.Abs(resolvedStart)
	if err != nil {
		return "", fmt.Errorf("resolve workspace start dir: %w", err)
	}

	realStart, err := filepath.EvalSymlinks(absStart)
	if err != nil {
		return "", fmt.Errorf("resolve workspace start dir symlinks: %w", err)
	}

	current := realStart
	for {
		if err := context.Cause(ctx); err != nil {
			return "", fmt.Errorf("discover workspace: %w", err)
		}

		candidate := filepath.Join(current, model.WorkflowRootDirName)
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return current, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat workspace marker %s: %w", candidate, err)
		}

		parent := filepath.Dir(current)
		if parent == current {
			return realStart, nil
		}
		current = parent
	}
}

func LoadConfig(ctx context.Context, workspaceRoot string) (ProjectConfig, string, error) {
	if err := context.Cause(ctx); err != nil {
		return ProjectConfig{}, "", fmt.Errorf("load workspace config: %w", err)
	}

	configPath := model.ConfigPathForWorkspace(workspaceRoot)
	content, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProjectConfig{}, configPath, nil
		}
		return ProjectConfig{}, configPath, fmt.Errorf("read workspace config: %w", err)
	}

	var cfg ProjectConfig
	decoder := toml.NewDecoder(bytes.NewReader(content)).DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return ProjectConfig{}, configPath, fmt.Errorf("decode workspace config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return ProjectConfig{}, configPath, err
	}

	return cfg, configPath, nil
}
