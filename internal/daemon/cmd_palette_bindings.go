package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type cmdPaletteBindingsResolver struct {
	workspaces cmdPaletteWorkspaceResolver
	loadGlobal func() (compozyconfig.Config, error)
	catalog    func() cmdpalette.BindableCatalog
	logger     *slog.Logger
}

type cmdPaletteWorkspaceResolver interface {
	Resolve(context.Context, string) (workspacepkg.ResolvedWorkspace, error)
}

var _ cmdpalette.BindingsResolver = (*cmdPaletteBindingsResolver)(nil)
var _ cmdpalette.PersonalizationPolicy = (*cmdPaletteBindingsResolver)(nil)

func (r *cmdPaletteBindingsResolver) PersonalizationEnabled(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
) (bool, error) {
	paletteConfig, err := r.resolvePaletteConfig(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	return paletteConfig.Personalization, nil
}

func (r *cmdPaletteBindingsResolver) Bindings(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
) (map[cmdpalette.CommandID][]string, map[cmdpalette.CommandID]string, error) {
	if r == nil || r.workspaces == nil || r.catalog == nil {
		return nil, nil, errors.New("daemon: command palette binding resolver is unavailable")
	}
	resolved, err := r.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	globalConfig, err := r.currentGlobalConfig()
	if err != nil {
		return nil, nil, err
	}
	configPath := filepath.Join(resolved.RootDir, compozyconfig.DirName, compozyconfig.ConfigName)
	windowConfig, err := compozyconfig.ApplyWindowManagerOverlayFile(configPath, globalConfig.WindowManager)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: resolve command palette shortcuts: %w", err)
	}
	paletteConfig, err := compozyconfig.ApplyCmdPaletteOverlayFile(configPath, globalConfig.CmdPalette)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: resolve command palette aliases: %w", err)
	}
	catalog := r.catalog()
	if catalog == nil {
		return nil, nil, errors.New("daemon: command palette catalog is unavailable")
	}
	commandIDs, err := catalog.BindableIDs(ctx, workspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: list bindable command ids: %w", err)
	}
	ids := make([]string, 0, len(commandIDs))
	for _, commandID := range commandIDs {
		ids = append(ids, string(commandID))
	}
	bindable := windowmanager.NewBindableIDs(ids)
	effective, diagnostics, err := windowmanager.TolerantEffectiveKeymap(windowConfig.Shortcuts, bindable)
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: build effective command palette keymap: %w", err)
	}
	bindings := make(map[cmdpalette.CommandID][]string, len(effective))
	for commandID, chords := range effective {
		bindings[cmdpalette.CommandID(commandID)] = append([]string(nil), chords...)
	}
	aliases := make(map[cmdpalette.CommandID]string, len(paletteConfig.Aliases))
	for commandID, alias := range paletteConfig.Aliases {
		if _, exists := bindable[commandID]; !exists {
			r.logDeadBinding(workspaceID, commandID, "alias")
			continue
		}
		aliases[cmdpalette.CommandID(commandID)] = alias
	}
	for _, diagnostic := range diagnostics {
		r.logDeadBinding(workspaceID, diagnostic.CommandID, "shortcut")
	}
	return bindings, aliases, nil
}

func (r *cmdPaletteBindingsResolver) resolvePaletteConfig(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
) (compozyconfig.CmdPaletteConfig, error) {
	if r == nil || r.workspaces == nil {
		return compozyconfig.CmdPaletteConfig{}, errors.New(
			"daemon: command palette config resolver is unavailable",
		)
	}
	resolved, err := r.resolveWorkspace(ctx, workspaceID)
	if err != nil {
		return compozyconfig.CmdPaletteConfig{}, err
	}
	globalConfig, err := r.currentGlobalConfig()
	if err != nil {
		return compozyconfig.CmdPaletteConfig{}, err
	}
	configPath := filepath.Join(resolved.RootDir, compozyconfig.DirName, compozyconfig.ConfigName)
	paletteConfig, err := compozyconfig.ApplyCmdPaletteOverlayFile(configPath, globalConfig.CmdPalette)
	if err != nil {
		return compozyconfig.CmdPaletteConfig{}, fmt.Errorf(
			"daemon: resolve command palette config: %w",
			err,
		)
	}
	return paletteConfig, nil
}

func (r *cmdPaletteBindingsResolver) currentGlobalConfig() (compozyconfig.Config, error) {
	if r == nil || r.loadGlobal == nil {
		return compozyconfig.Config{}, errors.New(
			"daemon: command palette global config loader is unavailable",
		)
	}
	config, err := r.loadGlobal()
	if err != nil {
		return compozyconfig.Config{}, fmt.Errorf(
			"daemon: load command palette global config: %w",
			err,
		)
	}
	return config, nil
}

func (r *cmdPaletteBindingsResolver) resolveWorkspace(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
) (workspacepkg.ResolvedWorkspace, error) {
	if r == nil || r.workspaces == nil {
		return workspacepkg.ResolvedWorkspace{}, errors.New(
			"daemon: command palette workspace resolver is unavailable",
		)
	}
	resolved, err := r.workspaces.Resolve(ctx, string(workspaceID))
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, fmt.Errorf(
			"daemon: resolve command palette workspace %q: %w",
			workspaceID,
			err,
		)
	}
	if resolved.ID != string(workspaceID) {
		return workspacepkg.ResolvedWorkspace{}, fmt.Errorf(
			"daemon: command palette workspace %q resolved as %q",
			workspaceID,
			resolved.ID,
		)
	}
	return resolved, nil
}

func (r *cmdPaletteBindingsResolver) logDeadBinding(
	workspaceID cmdpalette.WorkspaceID,
	commandID string,
	kind string,
) {
	logger := r.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn(
		"dropping command palette config for an unregistered command",
		"workspace_id", strings.TrimSpace(string(workspaceID)),
		"command_id", strings.TrimSpace(commandID),
		"config_kind", kind,
	)
}
