package bundles

import (
	"context"
	"errors"
	"fmt"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"

	"github.com/compozy/agh/internal/resources"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (s *Service) materializeBridge(
	ctx context.Context,
	activation Activation,
	bundleRecord resources.Record[BundleResourceSpec],
	bundle extensionpkg.BundleSpec,
	profile extensionpkg.BundleProfile,
	preset extensionpkg.BundleBridgePreset,
) (bridgepkg.BridgeInstance, error) {
	extensionName := strings.TrimSpace(preset.ExtensionName)
	if extensionName == "" {
		extensionName = strings.TrimSpace(activation.ExtensionName)
	}

	platform := strings.TrimSpace(preset.Platform)
	if platform == "" {
		switch {
		case strings.EqualFold(extensionName, activation.ExtensionName):
			platform = strings.TrimSpace(bundleRecord.Spec.OwnerBridgePlatform)
			if platform == "" {
				provider, err := s.loadExtension(ctx, extensionName)
				if err != nil {
					return bridgepkg.BridgeInstance{}, err
				}
				if provider != nil && provider.Manifest != nil {
					platform = strings.TrimSpace(provider.Manifest.Bridge.Platform)
				}
			}
		default:
			provider, err := s.loadExtension(ctx, extensionName)
			if err != nil {
				return bridgepkg.BridgeInstance{}, err
			}
			if provider == nil || provider.Manifest == nil {
				return bridgepkg.BridgeInstance{}, fmt.Errorf(
					"bundles: bridge provider %q is unavailable",
					extensionName,
				)
			}
			platform = strings.TrimSpace(provider.Manifest.Bridge.Platform)
		}
	}

	instance := bridgepkg.BridgeInstance{
		ID:               stableID("bri", activation.ID, preset.Name),
		Scope:            bridgeScopeFromActivation(activation.Scope),
		WorkspaceID:      activation.WorkspaceID,
		Platform:         platform,
		ExtensionName:    extensionName,
		DisplayName:      strings.TrimSpace(preset.DisplayName),
		Source:           bridgepkg.BridgeInstanceSourcePackage,
		Enabled:          false,
		Status:           bridgepkg.BridgeStatusDisabled,
		RoutingPolicy:    preset.RoutingPolicy,
		DeliveryDefaults: cloneRawMessage(preset.DeliveryDefaults),
		UpdatedAt:        s.now().UTC(),
	}
	if err := instance.Validate(); err != nil {
		return bridgepkg.BridgeInstance{}, fmt.Errorf(
			"bundles: materialize bridge %s/%s/%s/%s: %w",
			activation.ExtensionName,
			bundle.Name,
			profile.Name,
			preset.Name,
			err,
		)
	}
	return instance, nil
}

func (s *Service) resolveWorkspace(
	ctx context.Context,
	scope Scope,
	ref string,
	mode workspaceResolutionMode,
) (string, error) {
	if scope == ScopeGlobal {
		return "", nil
	}
	if s.workspaceResolver == nil {
		return "", errors.New("bundles: workspace resolver is required for workspace-scoped activations")
	}

	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", errors.New("bundles: workspace reference is required")
	}

	var (
		resolved workspacepkg.ResolvedWorkspace
		err      error
	)
	if isPathLikeWorkspaceRef(trimmed) {
		normalized, normalizeErr := aghconfig.ResolvePath(trimmed)
		if normalizeErr != nil {
			return "", normalizeErr
		}
		if mode == workspaceResolutionRegisterPaths {
			resolved, err = s.workspaceResolver.ResolveOrRegister(ctx, normalized)
		} else {
			resolved, err = s.workspaceResolver.Resolve(ctx, normalized)
		}
	} else {
		resolved, err = s.workspaceResolver.Resolve(ctx, trimmed)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resolved.ID), nil
}

func (s *Service) checkReady(ctx context.Context) error {
	if s == nil {
		return errors.New("bundles: service is required")
	}
	if ctx == nil {
		return errors.New("bundles: context is required")
	}
	if s.store == nil {
		return errors.New("bundles: store is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
