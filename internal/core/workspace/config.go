package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/core/model"
	toml "github.com/pelletier/go-toml/v2"
)

var osUserHomeDir = os.UserHomeDir
var discoverWorkspaceRoot = discoverWorkspaceRootFromStart
var discoverCache sync.Map

type discoverCacheEntry struct {
	ready chan struct{}
	root  string
	err   error
}

type configPaths struct {
	workspaceRoot string
	globalRoot    string
	workspacePath string
	globalPath    string
	workspaceSeen bool
	globalSeen    bool
}

func Resolve(ctx context.Context, startDir string) (Context, error) {
	root, err := Discover(ctx, startDir)
	if err != nil {
		return Context{}, err
	}

	cfg, paths, err := loadEffectiveConfig(ctx, root)
	if err != nil {
		return Context{}, err
	}

	return Context{
		Root:                root,
		CompozyDir:          model.CompozyDir(root),
		ConfigPath:          paths.effectivePath(),
		WorkspaceConfigPath: paths.workspacePath,
		GlobalConfigPath:    paths.globalPath,
		Config:              cfg,
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

	entry := &discoverCacheEntry{ready: make(chan struct{})}
	actual, loaded := discoverCache.LoadOrStore(absStart, entry)
	cachedEntry, ok := actual.(*discoverCacheEntry)
	if !ok || cachedEntry == nil {
		return "", fmt.Errorf("discover workspace: unexpected cache entry %T", actual)
	}
	entry = cachedEntry
	if loaded {
		select {
		case <-entry.ready:
			if entry.err != nil {
				return "", entry.err
			}
			return entry.root, nil
		case <-ctx.Done():
			return "", fmt.Errorf("discover workspace: %w", context.Cause(ctx))
		}
	}

	entry.root, entry.err = discoverWorkspaceRoot(ctx, resolvedStart)
	close(entry.ready)
	if entry.err != nil {
		discoverCache.Delete(absStart)
		return "", entry.err
	}
	return entry.root, nil
}

func discoverWorkspaceRootFromStart(ctx context.Context, startDir string) (string, error) {
	absStart, err := filepath.Abs(startDir)
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
	cfg, paths, err := loadEffectiveConfig(ctx, workspaceRoot)
	if err != nil {
		return ProjectConfig{}, "", err
	}
	return cfg, paths.effectivePath(), nil
}

func LoadGlobalConfig(ctx context.Context) (ProjectConfig, string, error) {
	if err := context.Cause(ctx); err != nil {
		return ProjectConfig{}, "", fmt.Errorf("load global config: %w", err)
	}

	paths, err := resolveConfigPaths(".")
	if err != nil {
		return ProjectConfig{}, "", fmt.Errorf("resolve global config paths: %w", err)
	}

	cfg, _, err := loadConfigFile(ctx, paths.globalPath, globalConfigScope, paths.globalRoot)
	if err != nil {
		return ProjectConfig{}, "", err
	}
	return cfg, paths.globalPath, nil
}

func loadEffectiveConfig(ctx context.Context, workspaceRoot string) (ProjectConfig, configPaths, error) {
	if err := context.Cause(ctx); err != nil {
		return ProjectConfig{}, configPaths{}, fmt.Errorf("load workspace config: %w", err)
	}

	paths, err := resolveConfigPaths(workspaceRoot)
	if err != nil {
		return ProjectConfig{}, configPaths{}, fmt.Errorf("resolve config paths: %w", err)
	}

	globalCfg, globalSeen, err := loadConfigFile(ctx, paths.globalPath, globalConfigScope, paths.globalRoot)
	if err != nil {
		return ProjectConfig{}, configPaths{}, err
	}
	workspaceCfg, workspaceSeen, err := loadConfigFile(
		ctx,
		paths.workspacePath,
		workspaceConfigScope,
		paths.workspaceRoot,
	)
	if err != nil {
		return ProjectConfig{}, configPaths{}, err
	}

	paths.globalSeen = globalSeen
	paths.workspaceSeen = workspaceSeen

	cfg := buildEffectiveProjectConfig(globalCfg, workspaceCfg)
	if err := cfg.validate(effectiveConfigScope); err != nil {
		return ProjectConfig{}, configPaths{}, err
	}
	return cfg, paths, nil
}

func resolveConfigPaths(workspaceRoot string) (configPaths, error) {
	paths := configPaths{
		workspaceRoot: workspaceRoot,
		workspacePath: model.ConfigPathForWorkspace(workspaceRoot),
	}

	homeDir, err := osUserHomeDir()
	if err != nil {
		return configPaths{}, fmt.Errorf("lookup user home directory: %w", err)
	}
	resolvedHomeDir, err := resolveConfigBaseDir(homeDir)
	if err != nil {
		return configPaths{}, fmt.Errorf("resolve global config base dir: %w", err)
	}

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(resolvedHomeDir, compozyconfig.DirName))
	if err != nil {
		return configPaths{}, fmt.Errorf("resolve global config base dir: %w", err)
	}

	paths.globalRoot = resolvedHomeDir
	paths.globalPath = homePaths.ConfigFile
	return paths, nil
}

func loadConfigFile(
	ctx context.Context,
	configPath string,
	scope string,
	baseDir string,
) (ProjectConfig, bool, error) {
	if err := context.Cause(ctx); err != nil {
		return ProjectConfig{}, false, fmt.Errorf("load %s: %w", scope, err)
	}
	if strings.TrimSpace(configPath) == "" {
		return ProjectConfig{}, false, nil
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProjectConfig{}, false, nil
		}
		return ProjectConfig{}, false, fmt.Errorf("read %s: %w", scope, err)
	}

	var cfg ProjectConfig
	decoder := toml.NewDecoder(bytes.NewReader(content)).DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return ProjectConfig{}, true, fmt.Errorf("decode %s: %w", scope, err)
	}

	cfg, err = normalizeProjectConfigPaths(cfg, baseDir)
	if err != nil {
		return ProjectConfig{}, true, fmt.Errorf("normalize %s: %w", scope, err)
	}
	if err := cfg.validate(scope); err != nil {
		return ProjectConfig{}, true, err
	}
	return cfg, true, nil
}

func (p configPaths) effectivePath() string {
	if p.workspaceSeen {
		return p.workspacePath
	}
	if p.globalSeen {
		return p.globalPath
	}
	return p.workspacePath
}
