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
	for _, node := range graph.order {
		err := precheckDependencyConsent(node, req.Dependencies[node.payload.Name])
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
			err = s.enableDependency(ctx, name, prepared.target, actor)
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

// Settle every missing dependency's trust and digest consent before the first mutation.
func precheckDependencyConsent(node *dependencyNode, consent contract.ExtensionDependencyConsent) error {
	if node.payload.State != dependencyStateMissing {
		return nil
	}
	if node.payload.ConsentRequired && !consent.AllowUnverified {
		return node.unverifiedErr
	}
	if node.candidate == nil {
		return fmt.Errorf("daemon: dependency %q has no acquired package", node.payload.Name)
	}
	return checkDependencyDigestConsent(node.payload, consent)
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
	ctx context.Context, name string, target extensionMutationTarget,
	actor taskpkg.ActorContext,
) error {
	err := s.lifecycle.withName(ctx, name, func() error {
		if target.profile.ID == extensionDefaultProfileLens().ID {
			_, err := s.enableTargetUnlocked(ctx, name, actor, target)
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
