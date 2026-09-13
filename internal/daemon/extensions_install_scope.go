package daemon

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type extensionInstallTarget struct {
	scope   extensionpkg.InstallationScope
	profile extensionpkg.ProfileLens
}

func (target extensionInstallTarget) key(name string) extensionpkg.InstanceKey {
	return extensionpkg.InstanceKey{Name: name, ProfileID: target.scope.ProfileID, WorkspaceID: target.scope.WorkspaceID}
}

func (s *daemonExtensionService) resolveExtensionInstallTarget(
	ctx context.Context, req contract.InstallExtensionRequest, actor taskpkg.ActorContext,
) (extensionInstallTarget, error) {
	target := extensionInstallTarget{profile: extensionDefaultProfileLens()}
	scope, workspaceID := strings.TrimSpace(req.Scope), strings.TrimSpace(req.WorkspaceID)
	if scope != "" && scope != "global" && scope != "workspace" {
		return target, extensionInstallSelectorError("scope", "must be global or workspace")
	}
	if scope == "global" && workspaceID != "" {
		return target, extensionInstallSelectorError("workspace_id", "global scope cannot select a workspace")
	}
	if scope == "workspace" && workspaceID == "" {
		workspaceID = strings.TrimSpace(actor.Scope.WorkspaceID)
		if workspaceID == "" {
			return target, extensionInstallSelectorError("workspace_id", "workspace scope requires a workspace")
		}
	}
	if !actor.Scope.Operator && scope == "" && workspaceID == "" {
		workspaceID = strings.TrimSpace(actor.Scope.WorkspaceID)
	}
	if !actor.Scope.Operator && workspaceID != strings.TrimSpace(actor.Scope.WorkspaceID) {
		return target, taskpkg.ErrPermissionDenied
	}
	if workspaceID != "" {
		scopedActor := actor
		scopedActor.Scope.WorkspaceID = workspaceID
		resolved, err := s.scopedDevelopmentWorkspaceID(ctx, scopedActor)
		if err != nil {
			return target, err
		}
		target.scope.WorkspaceID = resolved
	}
	return s.resolveExtensionInstallProfile(ctx, target, req.Profile, actor)
}

func (s *daemonExtensionService) resolveExtensionInstallProfile(
	ctx context.Context, target extensionInstallTarget, name string, actor taskpkg.ActorContext,
) (extensionInstallTarget, error) {
	name = strings.TrimSpace(name)
	if name != "" {
		if s.profiles == nil {
			if name != daemonDefaultProfileName {
				return target, extensionInstallSelectorError("profile", "profile manager is required")
			}
		} else {
			profile, err := s.profiles.GetByName(ctx, name)
			if err != nil {
				return target, extensionInstallSelectorError("profile", err.Error())
			}
			profileName, err := s.profiles.ProfileName(ctx, profile.ID)
			if err != nil {
				return target, extensionInstallSelectorError("profile", err.Error())
			}
			target.profile = extensionpkg.ProfileLens{ID: profile.ID, Name: profileName}
		}
		target.scope.ProfileID = target.profile.ID
	} else if !actor.Scope.Operator {
		profile, err := s.extensionReadProfile(ctx, actor)
		if err != nil {
			return target, err
		}
		target.profile, target.scope.ProfileID = profile, profile.ID
	}
	if !actor.Scope.Operator && !actor.ReadScope.Matches(target.profile.ID) {
		return target, taskpkg.ErrPermissionDenied
	}
	return target, nil
}

func extensionInstallSelectorError(field, message string) error {
	return &extensionpkg.ManifestValidationError{Field: field, Message: message}
}

func (s *daemonExtensionService) installedTargetStatus(
	ctx context.Context, name string, target extensionInstallTarget,
) (contract.ExtensionPayload, error) {
	if target.scope == (extensionpkg.InstallationScope{}) {
		return s.Status(ctx, name)
	}
	key := target.key(name)
	if _, err := s.registry.ResolveInstallation(ctx, name, extensionpkg.InstallationScope{
		ProfileID: target.profile.ID, WorkspaceID: key.WorkspaceID,
	}); err != nil {
		return contract.ExtensionPayload{}, err
	}
	if runtime, ok := s.runtime.(extensionDevRuntime); ok {
		ext, err := s.projectExtensionReadProfile(ctx, runtime, key, target.profile)
		if err != nil {
			return contract.ExtensionPayload{}, err
		}
		return s.payloadFromExtension(ctx, ext, target.profile)
	}
	info, err := s.registry.Get(name)
	if err != nil {
		return contract.ExtensionPayload{}, err
	}
	ext := &extensionpkg.Extension{Info: *info, Status: extensionpkg.ExtensionStatus{
		Name: name, Version: info.Version, Source: info.Source, Enabled: info.Enabled, WorkspaceID: key.WorkspaceID,
	}}
	populateExtensionManifest(s.logger, ext)
	return s.payloadFromExtension(ctx, ext, target.profile)
}

func (s *daemonExtensionService) applyManifestInstallScope(
	ctx context.Context, target extensionInstallTarget, request contract.InstallExtensionRequest,
	actor taskpkg.ActorContext, manifest *extensionpkg.Manifest,
) (extensionInstallTarget, error) {
	if strings.TrimSpace(request.Scope) != "" || strings.TrimSpace(request.WorkspaceID) != "" || !actor.Scope.Operator {
		return target, nil
	}
	defaultScope := ""
	for _, server := range manifest.Resources.MCPServers {
		scope := server.DefaultScope
		if scope == "" {
			scope = "global"
		}
		if defaultScope != "" && scope != defaultScope {
			return target, extensionInstallSelectorError("scope", "servers have different defaults; select global or workspace")
		}
		defaultScope = scope
	}
	if defaultScope != "workspace" {
		return target, nil
	}
	request.Scope = defaultScope
	return s.resolveExtensionInstallTarget(ctx, request, actor)
}
