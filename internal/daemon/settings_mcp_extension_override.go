package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/extensionmcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
)

func (s settingsMCPExtensionDefinitions) UpdateMCPExtensionOverride(
	ctx context.Context, req settingspkg.MCPAuthTargetRequest, override extensionmcp.Override,
) (settingspkg.MCPExtensionDefinition, error) {
	if s.state == nil || s.state.extensionMCP == nil || s.state.toolMCPResources == nil {
		return settingspkg.MCPExtensionDefinition{}, errors.New("daemon: extension MCP publisher is unavailable")
	}
	service, ok := s.state.deps.Extensions.(*daemonExtensionService)
	if !ok || service.lifecycle == nil {
		return settingspkg.MCPExtensionDefinition{}, errors.New("daemon: extension lifecycle is unavailable")
	}
	allocation, err := s.extensionMCPOverrideAllocation(ctx, req)
	if err != nil {
		return settingspkg.MCPExtensionDefinition{}, err
	}
	var result settingspkg.MCPExtensionDefinition
	key := extensionpkg.ProfileInstanceKey(allocation.Extension, allocation.ProfileID, allocation.WorkspaceID)
	err = service.lifecycle.withInstance(ctx, key, func() error {
		current, err := s.extensionMCPOverrideAllocation(ctx, req)
		if err != nil {
			return err
		}
		target, _, found, err := s.ResolveMCPExtensionDefinition(ctx, req)
		if err != nil {
			return err
		}
		if !found {
			return extensionmcp.ErrNotFound
		}
		base, err := service.extensionMCPOverrideDeclaration(ctx, current.Target)
		if err != nil {
			return err
		}
		base.Owner, base.RuntimeName = target.Owner, current.RuntimeName
		server, err := override.Apply(base)
		if err != nil {
			return fmt.Errorf("%w: %w", settingspkg.ErrValidation, err)
		}
		if err := s.commitMCPExtensionOverride(ctx, target, current, override); err != nil {
			return err
		}
		result = settingspkg.MCPExtensionDefinition{Target: target, Server: server, Override: override}
		return nil
	})
	return result, err
}

func (s settingsMCPExtensionDefinitions) extensionMCPOverrideAllocation(
	ctx context.Context, req settingspkg.MCPAuthTargetRequest,
) (extensionmcp.Record, error) {
	name, owned := strings.CutPrefix(strings.TrimSpace(req.Owner), "extension:")
	if !owned {
		return extensionmcp.Record{}, fmt.Errorf("%w: extension owner is required", settingspkg.ErrValidation)
	}
	profileID := store.DefaultProfileID
	if req.Scope == settingspkg.ScopeProfile {
		if s.state.profiles == nil {
			return extensionmcp.Record{}, errors.New("daemon: MCP override profile catalog is unavailable")
		}
		var err error
		profileID, err = s.state.profiles.AvailableProfileID(ctx, req.ProfileName)
		if err != nil {
			return extensionmcp.Record{}, err
		}
	}
	records, err := s.state.extensionMCP.List(ctx, profileID, req.WorkspaceID)
	if err != nil {
		return extensionmcp.Record{}, err
	}
	for _, record := range records {
		if record.Extension == name && record.ServerName == req.Name {
			return record, nil
		}
	}
	return extensionmcp.Record{}, extensionmcp.ErrNotFound
}

func (s *daemonExtensionService) extensionMCPOverrideDeclaration(
	ctx context.Context, target extensionmcp.Target,
) (compozyconfig.MCPServer, error) {
	key := extensionpkg.ProfileInstanceKey(target.Extension, target.ProfileID, target.WorkspaceID)
	var ext *extensionpkg.Extension
	var err error
	if runtime, ok := s.runtime.(interface {
		GetForInstance(extensionpkg.InstanceKey) (*extensionpkg.Extension, error)
	}); ok {
		ext, err = runtime.GetForInstance(key)
	} else if key.WorkspaceID == "" && s.runtime != nil {
		ext, err = s.runtime.Get(key.Name)
	} else {
		return compozyconfig.MCPServer{}, errors.New("daemon: extension MCP instance runtime is unavailable")
	}
	if err != nil {
		return compozyconfig.MCPServer{}, err
	}
	if ext == nil || ext.Manifest == nil {
		return compozyconfig.MCPServer{}, extensionmcp.ErrNotFound
	}
	inputs, err := s.inputReader().load(ctx, extensioninput.Instance{
		Extension: target.Extension, ProfileID: target.ProfileID, WorkspaceID: target.WorkspaceID,
	}, ext.Manifest)
	if err != nil {
		return compozyconfig.MCPServer{}, err
	}
	servers, err := extensionpkg.ResolveManifestMCPServerResources(ext.RootDir, ext.Manifest, inputs, s.getenv)
	if err != nil {
		return compozyconfig.MCPServer{}, err
	}
	for _, server := range servers {
		if server.Name == target.ServerName {
			return server.MCPServer, nil
		}
	}
	return compozyconfig.MCPServer{}, extensionmcp.ErrNotFound
}

func (s settingsMCPExtensionDefinitions) commitMCPExtensionOverride(
	ctx context.Context, target mcpauth.Target, previous extensionmcp.Record, override extensionmcp.Override,
) error {
	if s.auth != nil {
		if err := s.auth.MCPAuthInvalidate(target); err != nil {
			return err
		}
	}
	if err := s.state.extensionMCP.Update(ctx, previous.Target, override); err != nil {
		return err
	}
	if err := s.state.toolMCPResources.Sync(ctx); err != nil {
		return s.rollbackMCPExtensionOverride(ctx, target, previous, err)
	}
	if s.auth != nil {
		if err := s.auth.MCPAuthDeleteState(ctx, target); err != nil {
			return s.rollbackMCPExtensionOverride(ctx, target, previous, err)
		}
	}
	return nil
}

func (s settingsMCPExtensionDefinitions) rollbackMCPExtensionOverride(
	ctx context.Context, target mcpauth.Target, previous extensionmcp.Record, cause error,
) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extensionSecretRollbackTimeout)
	defer cancel()
	restoreErr := s.state.extensionMCP.Update(rollbackCtx, previous.Target, previous.Override)
	if restoreErr == nil {
		restoreErr = s.state.toolMCPResources.Sync(rollbackCtx)
	}
	var invalidateErr error
	if s.auth != nil {
		invalidateErr = s.auth.MCPAuthInvalidate(target)
	}
	return errors.Join(cause, restoreErr, invalidateErr)
}
