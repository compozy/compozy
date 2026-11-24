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
// dependent's scope and profile. Every consent is settled before the first mutation, so only a runtime failure
// stops the loop, keeping what it completed.
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
	// Root trust and digest consent were checked at acquisition; settle its gateway consent and every
	// dependency's own consent too, so a missing consent installs nothing. The commits re-check them.
	if err := s.precheckGatewayConfirmation(prepared, req.ConfirmGatewayDigest, actor); err != nil {
		return nil, err
	}
	for _, node := range graph.order {
		err := s.precheckDependencyConsent(node, req.Dependencies[node.payload.Name], prepared.target, actor)
		if err != nil {
			return nil, &extensionpkg.DependencyGateError{Dependency: node.payload.Name, Err: err}
		}
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

// precheckDependencyConsent mirrors one node's own consent gates without side effects (rule 13): a missing node's
// unverified and digest consent and its install gateway gate, or a disabled node's enable gateway gate, which only
// the default profile's enable path applies.
func (s *daemonExtensionService) precheckDependencyConsent(
	node *dependencyNode, consent contract.ExtensionDependencyConsent, target extensionMutationTarget,
	actor taskpkg.ActorContext,
) error {
	switch node.payload.State {
	case dependencyStateMissing:
		if node.payload.ConsentRequired && !consent.AllowUnverified {
			return node.unverifiedErr
		}
		if err := checkDependencyDigestConsent(node.payload, consent); err != nil {
			return err
		}
		if node.candidate == nil {
			return fmt.Errorf("daemon: dependency %q has no acquired package", node.payload.Name)
		}
		return s.precheckGatewayConfirmation(*node.candidate, consent.ConfirmGatewayDigest, actor)
	case dependencyStateDisabled:
		digest := node.payload.GatewayRequirementDigest
		if target.profile.ID == extensionDefaultProfileLens().ID && digest != "" &&
			strings.TrimSpace(consent.ConfirmGatewayDigest) != digest {
			return &extensionpkg.GatewayConfirmationRequiredError{CurrentDigest: digest}
		}
	}
	return nil
}

// precheckGatewayConfirmation mirrors the install commit's gateway gate without side effects: a new package
// needs the confirmed digest, and a reinstall needs it only when the installed confirmation does not cover it.
func (s *daemonExtensionService) precheckGatewayConfirmation(
	prepared preparedDaemonExtensionInstall, confirmDigest string, actor taskpkg.ActorContext,
) error {
	confirmed := strings.TrimSpace(confirmDigest)
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

// installDependencyNode commits the acquired candidate with its own per-dependency consent, which
// precheckDependencyConsent already settled. committed reports a package left installed even when its enable or
// post-commit notification failed (the retained-package contract).
func (s *daemonExtensionService) installDependencyNode(
	ctx context.Context, node *dependencyNode, consent contract.ExtensionDependencyConsent, actor taskpkg.ActorContext,
) (committed bool, _ error) {
	if node.candidate == nil {
		return false, fmt.Errorf("daemon: dependency %q has no acquired package", node.payload.Name)
	}
	prepared := *node.candidate
	node.candidate = nil
	// A dependency only attaches the package other cells already use (D9), whatever its source; a local bundle
	// with the installed bytes enrolls the attachment like a published one instead of refusing it.
	prepared.keepInstalled, prepared.enrollAttachment = true, true
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
// resolved refuses the commit instead of leaving the new dependent unmet. addRefusalRetryCommands names the retry
// commands once the locks are released.
func (s *daemonExtensionService) checkCommitDependenciesMet(
	ctx context.Context, prepared preparedDaemonExtensionInstall,
) error {
	if prepared.manifest == nil || len(prepared.manifest.Requires.Extensions) == 0 {
		return nil
	}
	cell := extensionpkg.InstallationScope{
		ProfileID: prepared.target.profile.ID, WorkspaceID: prepared.target.scope.WorkspaceID,
	}
	unmet, err := s.cellUnmetDependencies(ctx, prepared.name, prepared.manifest.Requires.Extensions, cell)
	if err != nil || len(unmet) == 0 {
		return err
	}
	return newDependencyRefusal(unmet, cell)
}

// dependencyReplacementError refuses a dependency install that would replace the package other cells use. The
// resolver reports this case before any mutation; the commit repeats it for a package installed meanwhile.
type dependencyReplacementError struct {
	item    contract.ExtensionDependencyPayload
	command string
}

func (e *dependencyReplacementError) Error() string {
	next := "reinstall it there from a source that provides the required version first"
	if e.command != "" {
		next = "run `" + e.command + "` first"
	}
	return fmt.Sprintf(
		"%s: %s %s is installed for another scope or profile and its source now provides %s; "+
			"dependencies are never updated, %s",
		extensionpkg.ErrExtensionExists.Error(), e.item.Name, e.item.InstalledVersion, e.item.Version, next,
	)
}

func (e *dependencyReplacementError) Unwrap() error { return extensionpkg.ErrExtensionExists }

func dependencyReplacementRefused(
	installed *extensionpkg.ExtensionInfo, candidate *extensionpkg.Manifest, req contract.InstallExtensionRequest,
) error {
	item := contract.ExtensionDependencyPayload{
		Name: installed.Name, Source: string(req.Source), Ref: req.Ref, InstalledVersion: installed.Version,
		State: dependencyStateInstalledElsewhere,
	}
	if candidate != nil {
		item.Version = candidate.Version
	}
	return &dependencyReplacementError{item: item}
}

// enableDependency enables a disabled dependency for the dependent's profile through the enable paths.
func (s *daemonExtensionService) enableDependency(
	ctx context.Context, name string, consent contract.ExtensionDependencyConsent, target extensionMutationTarget,
	actor taskpkg.ActorContext,
) error {
	err := s.lifecycle.withName(ctx, name, func() error {
		if target.profile.ID == extensionDefaultProfileLens().ID {
			_, err := s.enableTargetUnlocked(ctx, name, contract.EnableExtensionRequest{
				ConfirmGatewayDigest: consent.ConfirmGatewayDigest,
			}, actor, target)
			return err
		}
		return s.applyProfileEnablement(ctx, name, target.profile.ID, target.profile.Name, true, actor)
	})
	s.addRefusalRetryCommands(ctx, err)
	return err
}

// withDependenciesInstalled reports the dependencies a failed install already completed.
func withDependenciesInstalled(err error, installed []string) error {
	if err == nil || len(installed) == 0 {
		return err
	}
	return &extensionpkg.DependenciesInstalledError{Installed: installed, Err: err}
}
