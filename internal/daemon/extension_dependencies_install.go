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

const (
	dependencyInstallOutcomeInstalled = "installed"
	dependencyInstallOutcomeFailed    = "failed"
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
	graph, err := s.resolveDependencies(ctx, prepared.manifest, prepared.sourceDir, prepared.target, req.Dependencies,
		actor)
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
		committed := false
		switch node.payload.State {
		case dependencyStateMissing:
			committed, err = s.installDependencyNode(ctx, node, consent, actor)
		case dependencyStateDisabled:
			err = s.enableDependency(ctx, name, consent, prepared.target, actor)
			committed = err == nil
		default:
			continue
		}
		outcome := dependencyInstallOutcomeInstalled
		if err != nil {
			outcome = dependencyInstallOutcomeFailed
		}
		s.logger.Info("extension dependency install", "dependent", strings.Join(node.payload.RequiredBy, ","),
			"dependency", name, "source", node.payload.Source, "state", node.payload.State, "outcome", outcome)
		// A package committed before its activation failed stays on disk, so it is reported as installed.
		if committed {
			installed = append(installed, name)
		}
		if err != nil {
			return installed, &extensionpkg.DependencyGateError{Dependency: name, Err: err}
		}
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

// installDependencyNode commits the acquired candidate with its own per-dependency consent. committed reports a
// package left installed even when its enable or post-commit notification failed (the retained-package contract).
func (s *daemonExtensionService) installDependencyNode(
	ctx context.Context, node *dependencyNode, consent contract.ExtensionDependencyConsent, actor taskpkg.ActorContext,
) (committed bool, _ error) {
	if node.payload.ConsentRequired && !consent.AllowUnverified {
		return false, node.unverifiedErr
	}
	if err := checkDependencyDigestConsent(node.payload, consent); err != nil {
		return false, err
	}
	if node.candidate == nil {
		return false, fmt.Errorf("daemon: dependency %q has no acquired package", node.payload.Name)
	}
	prepared := *node.candidate
	node.candidate = nil
	prepared.keepInstalled = true
	req := contract.InstallExtensionRequest{
		Enable: true, Source: contract.InstallExtensionSource(node.payload.Source), Ref: node.payload.Ref,
		ExpectedDigest: consent.ExpectedDigest, AllowUnverified: consent.AllowUnverified,
		ConfirmGatewayDigest: consent.ConfirmGatewayDigest,
	}
	item, err := s.installPrepared(ctx, prepared, req, actor, extensionpkg.LifecycleEvent{
		Type: eventspkg.ExtensionInstallFailed, ExtensionName: prepared.name, SourceKind: node.payload.Source,
	})
	committed = item.Name != ""
	if err != nil {
		return committed, err
	}
	if item.InstallEnableError != nil {
		return committed, fmt.Errorf("daemon: dependency installed but not enabled: %s", item.InstallEnableError.Error)
	}
	return committed, nil
}

// checkDependencyDigestConsent requires the reviewed digest a marketplace artifact needs (rule 13). Preview
// acquires with the listed digest, so the install must not trust that acquisition without the caller's pin.
func checkDependencyDigestConsent(
	item contract.ExtensionDependencyPayload,
	consent contract.ExtensionDependencyConsent,
) error {
	if contract.InstallExtensionSource(item.Source) == contract.InstallExtensionSourceMarketplace &&
		strings.TrimSpace(consent.ExpectedDigest) == "" {
		return &extensionpkg.ManifestValidationError{
			Field:   "expected_digest",
			Message: "a listed digest is required for marketplace plugins",
		}
	}
	return extensionpkg.CheckExpectedDigest(consent.ExpectedDigest, item.DigestSHA256)
}

// dependencyNames lists the direct requires.extensions names of a manifest.
func dependencyNames(manifest *extensionpkg.Manifest) []string {
	if manifest == nil {
		return nil
	}
	names := make([]string, 0, len(manifest.Requires.Extensions))
	for _, dependency := range manifest.Requires.Extensions {
		names = append(names, dependency.Name)
	}
	return names
}

// checkCommitDependenciesMet re-checks, under the lifecycle locks of the package and its direct dependencies,
// that every dependency is still met in the target cell: a dependency removed or disabled after the graph was
// resolved refuses the commit instead of leaving the new dependent unmet.
func (s *daemonExtensionService) checkCommitDependenciesMet(
	ctx context.Context, prepared preparedDaemonExtensionInstall,
) error {
	if prepared.manifest == nil || len(prepared.manifest.Requires.Extensions) == 0 {
		return nil
	}
	unmet, err := s.cellUnmetDependencies(ctx, prepared.name, prepared.manifest.Requires.Extensions,
		extensionpkg.InstallationScope{
			ProfileID: prepared.target.profile.ID, WorkspaceID: prepared.target.scope.WorkspaceID,
		})
	if err != nil || len(unmet) == 0 {
		return err
	}
	return &extensionpkg.DependenciesRequiredError{Dependencies: unmet}
}

// dependencyReplacementRefused refuses a dependency install that would replace the package other cells use. The
// resolver reports this case before any mutation; the commit repeats it for a package installed meanwhile.
func (s *daemonExtensionService) dependencyReplacementRefused(
	ctx context.Context, installed *extensionpkg.ExtensionInfo, candidate *extensionpkg.Manifest,
	req contract.InstallExtensionRequest,
) error {
	version := ""
	if candidate != nil {
		version = candidate.Version
	}
	next := "reinstall it there from a source that provides the required version first"
	if command := s.dependencyRetryCommand(ctx, contract.ExtensionDependencyPayload{
		Name: installed.Name, Source: string(req.Source), Ref: req.Ref, Version: version,
		State: dependencyStateInstalledElsewhere,
	}, nil); command != "" {
		next = "run `" + command + "` first"
	}
	return fmt.Errorf(
		"%w: %s %s is installed for another scope or profile and its source now provides %s; "+
			"dependencies are never updated, %s",
		extensionpkg.ErrExtensionExists, installed.Name, installed.Version, version, next,
	)
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
