package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// installDependencies is the dependency gate between the requested artifact's trust gates and its input gate
// (rules 12 and 13). With consent it installs missing and enables disabled nodes, dependencies first, in the
// dependent's scope and profile, and stops at the first failure keeping what it completed.
func (s *daemonExtensionService) installDependencies(
	ctx context.Context, prepared preparedDaemonExtensionInstall, req contract.InstallExtensionRequest,
	actor taskpkg.ActorContext,
) (installed []string, resultErr error) {
	if prepared.manifest == nil || len(prepared.manifest.Requires.Extensions) == 0 {
		return nil, nil
	}
	graph, err := s.resolveDependencies(ctx, prepared.manifest, prepared.target, req.Dependencies, actor)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := graph.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("daemon: close dependency staging: %w", closeErr))
		}
	}()
	if err := graph.gate(req.WithDependencies); err != nil {
		return nil, err
	}
	// Root trust and digest consent were checked at acquisition; settle its gateway consent too, so a root
	// refusal installs no dependency. The commit re-checks it under the lifecycle lock.
	if err := s.precheckRootGatewayConfirmation(prepared, req, actor); err != nil {
		return nil, err
	}
	for _, node := range graph.order {
		name := node.payload.Name
		consent := req.Dependencies[name]
		switch node.payload.State {
		case dependencyStateMissing:
			err = s.installDependencyNode(ctx, node, consent, actor)
		case dependencyStateDisabled:
			err = s.enableDependency(ctx, name, consent, prepared.target, actor)
		default:
			continue
		}
		outcome := "installed"
		if err != nil {
			outcome = "failed"
		}
		s.logger.Info("extension dependency install", "dependent", strings.Join(node.payload.RequiredBy, ","),
			"dependency", name, "source", node.payload.Source, "state", node.payload.State, "outcome", outcome)
		if err != nil {
			return installed, &extensionpkg.DependencyGateError{Dependency: name, Err: err}
		}
		installed = append(installed, name)
	}
	return installed, nil
}

// precheckRootGatewayConfirmation mirrors the install commit's gateway gate without side effects: a new package
// needs the confirmed digest, and a reinstall needs it only when the installed confirmation does not cover it.
func (s *daemonExtensionService) precheckRootGatewayConfirmation(
	prepared preparedDaemonExtensionInstall, req contract.InstallExtensionRequest, actor taskpkg.ActorContext,
) error {
	confirmed := strings.TrimSpace(req.ConfirmGatewayDigest)
	installed, err := s.registry.Get(prepared.name)
	if prepared.published == nil || errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		_, err = s.prepareInstallGatewayConfirmation(prepared.manifest, confirmed, actor)
		return err
	}
	if err != nil {
		return err
	}
	digest, required, err := candidateGatewayConfirmationRequirement(installed, prepared.manifest)
	if err != nil || !required || confirmed == digest {
		return err
	}
	return &extensionpkg.GatewayConfirmationRequiredError{CurrentDigest: digest}
}

// installDependencyNode commits the acquired candidate with its own per-dependency consent.
func (s *daemonExtensionService) installDependencyNode(
	ctx context.Context, node *dependencyNode, consent contract.ExtensionDependencyConsent, actor taskpkg.ActorContext,
) error {
	if node.payload.ConsentRequired && !consent.AllowUnverified {
		return node.unverifiedErr
	}
	if node.candidate == nil {
		return fmt.Errorf("daemon: dependency %q has no acquired package", node.payload.Name)
	}
	prepared := *node.candidate
	node.candidate = nil
	req := contract.InstallExtensionRequest{
		Enable: true, Source: contract.InstallExtensionSource(node.payload.Source), Ref: node.payload.Ref,
		ExpectedDigest: consent.ExpectedDigest, AllowUnverified: consent.AllowUnverified,
		ConfirmGatewayDigest: consent.ConfirmGatewayDigest,
	}
	item, err := s.installPrepared(ctx, prepared, req, actor, extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionInstallFailed, ExtensionName: prepared.name, SourceKind: node.payload.Source,
	})
	if err != nil {
		return err
	}
	if item.InstallEnableError != nil {
		return fmt.Errorf("daemon: dependency installed but not enabled: %s", item.InstallEnableError.Error)
	}
	return nil
}

// enableDependency enables a disabled dependency for the dependent's profile through the enable paths.
func (s *daemonExtensionService) enableDependency(
	ctx context.Context, name string, consent contract.ExtensionDependencyConsent, target extensionMutationTarget,
	actor taskpkg.ActorContext,
) error {
	return s.lifecycle.withName(ctx, name, func() error {
		if target.profile.ID == extensionDefaultProfileLens().ID {
			_, err := s.enableTargetUnlocked(ctx, name, contract.EnableExtensionRequest{
				ConfirmGatewayDigest: consent.ConfirmGatewayDigest,
			}, actor, target)
			return err
		}
		return s.applyProfileEnablement(ctx, name, target.profile.ID, target.profile.Name, true, actor)
	})
}

// withDependenciesInstalled reports the dependencies a failed install already completed.
func withDependenciesInstalled(err error, installed []string) error {
	if err == nil || len(installed) == 0 {
		return err
	}
	return &extensionpkg.DependenciesInstalledError{Installed: installed, Err: err}
}
