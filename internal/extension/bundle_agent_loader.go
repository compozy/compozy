package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/soul"
)

func loadBundleAgent(ctx context.Context, rootDir string, path string) (BundleAgent, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return BundleAgent{}, fmt.Errorf("%w: profile agent path is required", ErrBundleInvalid)
	}
	if filepath.IsAbs(trimmed) {
		return BundleAgent{}, fmt.Errorf("%w: profile agent path %q must be relative", ErrBundleInvalid, trimmed)
	}
	agentDir, err := resolveBundlePathWithinRoot(rootDir, trimmed, "profile agent")
	if err != nil {
		return BundleAgent{}, err
	}
	info, err := os.Stat(agentDir)
	if err != nil {
		return BundleAgent{}, fmt.Errorf("extension: stat bundle agent %q: %w", trimmed, err)
	}
	if !info.IsDir() {
		return BundleAgent{}, fmt.Errorf("%w: profile agent path %q must be a directory", ErrBundleInvalid, trimmed)
	}

	agentPath := filepath.Join(agentDir, "AGENT.md")
	agent, err := aghconfig.LoadAgentDefFile(agentPath)
	if err != nil {
		return BundleAgent{}, fmt.Errorf("%w: load profile agent %q: %w", ErrBundleInvalid, trimmed, err)
	}
	loaded := BundleAgent{
		Path:  filepath.ToSlash(filepath.Clean(trimmed)),
		Agent: aghconfig.CloneAgentDef(agent),
	}
	if loaded.Soul, err = loadBundleAgentSoulSidecar(ctx, agentDir, loaded.Path); err != nil {
		return BundleAgent{}, err
	}
	if loaded.Heartbeat, err = loadBundleAgentHeartbeatSidecar(ctx, agentDir, loaded.Path); err != nil {
		return BundleAgent{}, err
	}
	return loaded, nil
}

func loadBundleAgentSoulSidecar(
	ctx context.Context,
	agentDir string,
	agentRelPath string,
) (*BundleAgentSidecar, error) {
	sourcePath := filepath.ToSlash(filepath.Join(agentRelPath, soul.FileName))
	body, err := os.ReadFile(filepath.Join(agentDir, soul.FileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: read profile agent %s: %w", ErrBundleInvalid, sourcePath, err)
	}
	if _, err := soul.Parse(ctx, soul.ParseRequest{
		SourcePath: sourcePath,
		Content:    body,
		Config:     aghconfig.DefaultSoulConfig(),
	}); err != nil {
		return nil, fmt.Errorf("%w: profile agent %s: %w", ErrBundleInvalid, sourcePath, err)
	}
	return &BundleAgentSidecar{SourcePath: sourcePath, Body: string(body)}, nil
}

func loadBundleAgentHeartbeatSidecar(
	ctx context.Context,
	agentDir string,
	agentRelPath string,
) (*BundleAgentSidecar, error) {
	sourcePath := filepath.ToSlash(filepath.Join(agentRelPath, heartbeat.FileName))
	body, err := os.ReadFile(filepath.Join(agentDir, heartbeat.FileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: read profile agent %s: %w", ErrBundleInvalid, sourcePath, err)
	}
	if _, err := heartbeat.Parse(ctx, heartbeat.ParseRequest{
		SourcePath: sourcePath,
		Content:    body,
		Config:     aghconfig.DefaultHeartbeatConfig(),
	}); err != nil {
		return nil, fmt.Errorf("%w: profile agent %s: %w", ErrBundleInvalid, sourcePath, err)
	}
	return &BundleAgentSidecar{SourcePath: sourcePath, Body: string(body)}, nil
}
