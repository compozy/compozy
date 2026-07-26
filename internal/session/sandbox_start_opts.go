package session

import (
	"fmt"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/acp"
	envpkg "github.com/compozy/agh/internal/sandbox"
)

func sandboxStartOpts(
	opts acp.StartOpts,
	prepared envpkg.Prepared,
	state envpkg.SessionState,
	localRoot string,
) (acp.StartOpts, error) {
	next := opts
	if command := strings.TrimSpace(prepared.Launch.Command); command != "" {
		next.Command = command
	}
	if prepared.Launch.Env != nil {
		next.Env = append([]string(nil), prepared.Launch.Env...)
	}
	cwd, err := sandboxRuntimeCWD(localRoot, opts.Cwd, prepared, state)
	if err != nil {
		return acp.StartOpts{}, err
	}
	next.Cwd = cwd
	next.AdditionalDirs = append([]string(nil), prepared.RuntimeAdditionalDirs...)
	if next.AdditionalDirs == nil {
		next.AdditionalDirs = append([]string(nil), state.RuntimeAdditionalDirs...)
	}
	next.Launcher = prepared.Launcher
	next.ToolHost = prepared.ToolHost
	return next, nil
}

func sandboxRuntimeCWD(
	localRoot string,
	requested string,
	prepared envpkg.Prepared,
	state envpkg.SessionState,
) (string, error) {
	runtimeRoot := strings.TrimSpace(prepared.RuntimeRootDir)
	if runtimeRoot == "" {
		runtimeRoot = strings.TrimSpace(state.RuntimeRootDir)
	}
	canonicalLocalRoot, err := canonicalDirectory(localRoot)
	if err != nil {
		return "", fmt.Errorf("%w: resolve sandbox workspace root: %w", ErrValidation, err)
	}
	canonicalRequested, err := resolveContainedDirectory(canonicalLocalRoot, requested)
	if err != nil {
		return "", fmt.Errorf("%w: resolve sandbox workspace cwd: %w", ErrValidation, err)
	}
	relative, err := filepath.Rel(canonicalLocalRoot, canonicalRequested)
	if err != nil {
		return "", fmt.Errorf("%w: map sandbox workspace cwd: %w", ErrValidation, err)
	}
	if state.Backend == envpkg.BackendLocal {
		canonicalRoot, err := canonicalDirectory(runtimeRoot)
		if err != nil {
			return "", fmt.Errorf("%w: resolve local sandbox root: %w", ErrValidation, err)
		}
		if relative == "." {
			return canonicalRoot, nil
		}
		cwd, err := resolveContainedDirectory(canonicalRoot, filepath.Join(canonicalRoot, relative))
		if err != nil {
			return "", fmt.Errorf("%w: resolve local sandbox cwd: %w", ErrValidation, err)
		}
		return cwd, nil
	}
	remoteRoot := pathpkg.Clean(runtimeRoot)
	if remoteRoot == "." || strings.TrimSpace(runtimeRoot) == "" {
		return "", fmt.Errorf("%w: remote sandbox root is required", ErrValidation)
	}
	if relative == "." {
		return remoteRoot, nil
	}
	return pathpkg.Join(remoteRoot, filepath.ToSlash(relative)), nil
}
