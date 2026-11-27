package daemon

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// Dependency node states (rule 12). Experimental.
const (
	dependencyStateMet          = "met"
	dependencyStateMissing      = "missing"
	dependencyStateDisabled     = "disabled"
	dependencyStateVersionUnmet = "version_unmet"
	// dependencyStateInstalledElsewhere marks a package installed for another scope or profile with bytes that
	// differ from what the dependency source provides; a dependency install never replaces it (D9).
	dependencyStateInstalledElsewhere = "installed_elsewhere"
	maxDependencyDepth                = 5
)

// dependencyNode is one extension name of the graph; diamonds collapse onto it.
type dependencyNode struct {
	payload    contract.ExtensionDependencyPayload
	declaredBy string
	// declaredRef is the ref before physical resolution, absolute for local dependencies.
	declaredRef         string
	declaredDir         string
	installed           bool
	installedGlobally   bool
	enabled             bool
	children            []extensionpkg.ExtensionDependency
	expandedDepth       int
	expandedGlobalDepth int
	// candidate is the acquired package of a missing node; whoever installs it takes ownership.
	candidate *preparedDaemonExtensionInstall
	// unverifiedErr is the trust refusal replayed when a consent-required node lacks allow_unverified.
	unverifiedErr error
	// installedElsewhere is set when the candidate would replace a package other cells use.
	installedElsewhere bool
	// sourceDir is this package's local source directory, the base of its relative local_path dependency refs.
	sourceDir string
}

// dependencyGraph lists nodes in topological order, dependencies first.
type dependencyGraph struct {
	nodes    map[string]*dependencyNode
	order    []*dependencyNode
	releases []func()
}

// Close releases acquired candidates that were not installed and the marketplace cache holds.
func (g *dependencyGraph) Close() error {
	if g == nil {
		return nil
	}
	var err error
	for _, node := range g.nodes {
		if node.candidate != nil {
			err = errors.Join(err, node.candidate.Close())
			node.candidate = nil
		}
	}
	for _, release := range g.releases {
		release()
	}
	g.releases = nil
	return err
}

func (g *dependencyGraph) payloads() []contract.ExtensionDependencyPayload {
	if g == nil || len(g.order) == 0 {
		return nil
	}
	items := make([]contract.ExtensionDependencyPayload, 0, len(g.order))
	for _, node := range g.order {
		items = append(items, node.payload)
	}
	return items
}

// gate refuses unmet dependencies without consent, and version_unmet or installed_elsewhere ones always (rule 12),
// before any dependency is installed.
func (g *dependencyGraph) gate(withDependencies bool) error {
	var unmet []contract.ExtensionDependencyPayload
	blocked := false
	for _, item := range g.payloads() {
		if item.State == dependencyStateMet {
			continue
		}
		unmet = append(unmet, item)
		blocked = blocked || item.State == dependencyStateVersionUnmet ||
			item.State == dependencyStateInstalledElsewhere
	}
	if len(unmet) == 0 || (withDependencies && !blocked) {
		return nil
	}
	return &extensionpkg.DependenciesRequiredError{Dependencies: unmet}
}

type dependencyResolver struct {
	service *daemonExtensionService
	target  extensionMutationTarget
	consent map[string]contract.ExtensionDependencyConsent
	actor   taskpkg.ActorContext
	graph   *dependencyGraph
}

// resolveDependencies walks root's transitive requires.extensions before any mutation (rules 11 and 12).
// A completed subtree is revisited only for a deeper path or to check a global parent's visibility.
func (s *daemonExtensionService) resolveDependencies(
	ctx context.Context, root *extensionpkg.Manifest, rootDir string, target extensionMutationTarget,
	consent map[string]contract.ExtensionDependencyConsent, actor taskpkg.ActorContext,
) (*dependencyGraph, error) {
	graph := &dependencyGraph{nodes: make(map[string]*dependencyNode)}
	if root == nil || len(root.Requires.Extensions) == 0 {
		return graph, nil
	}
	resolver := dependencyResolver{
		service: s, target: target, consent: consent, actor: actor, graph: graph,
	}
	if err := resolver.expand(
		ctx,
		root.Name,
		rootDir,
		root.Requires.Extensions,
		[]string{root.Name},
		false,
	); err != nil {
		return nil, errors.Join(err, graph.Close())
	}
	cell := extensionpkg.InstallationScope{ProfileID: target.profile.ID, WorkspaceID: target.scope.WorkspaceID}
	for _, node := range graph.order {
		node.resolveState()
		// An installed version_unmet dependency blocks in the dependent's cell; another cell's copy in its own.
		blocking := &cell
		if node.payload.State == dependencyStateInstalledElsewhere {
			blocking = nil
		}
		node.payload.RetryCommand = s.dependencyRetryCommand(ctx, node.payload, blocking)
	}
	return graph, nil
}

func (r *dependencyResolver) expand(
	ctx context.Context, parent, parentDir string, dependencies []extensionpkg.ExtensionDependency, path []string,
	globalOnly bool,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, dependency := range dependencies {
		ref, err := dependencyDeclarationRef(dependency, parentDir)
		if err != nil {
			return err
		}
		chain := append(slices.Clone(path), dependency.Name)
		if slices.Contains(path, dependency.Name) {
			return &extensionpkg.DependencyGraphInvalidError{
				Reason: extensionpkg.DependencyGraphReasonCycle,
				Path:   chain,
			}
		}
		if len(chain)-1 > maxDependencyDepth {
			return &extensionpkg.DependencyGraphInvalidError{
				Reason: extensionpkg.DependencyGraphReasonDepth,
				Path:   chain,
			}
		}
		depth := len(chain) - 1
		previous := r.graph.nodes[dependency.Name]
		if globalOnly && (previous == nil || !previous.installedGlobally) {
			if err := r.requireGlobalDependency(ctx, parent, dependency, ref, depth); err != nil {
				return err
			}
		}
		node, err := r.visit(ctx, parent, parentDir, ref, dependency, depth)
		if err != nil {
			return err
		}
		childGlobalOnly := globalOnly || (r.target.scope.WorkspaceID != "" && node.installedGlobally)
		expandedDepth := &node.expandedDepth
		if childGlobalOnly {
			expandedDepth = &node.expandedGlobalDepth
		}
		if *expandedDepth >= depth {
			continue
		}
		if err := r.expand(ctx, dependency.Name, node.sourceDir, node.children, chain, childGlobalOnly); err != nil {
			return err
		}
		*expandedDepth = depth
		if previous == nil {
			r.graph.order = append(r.graph.order, node)
		}
	}
	return nil
}

func (r *dependencyResolver) visit(
	ctx context.Context, parent, parentDir, ref string, dependency extensionpkg.ExtensionDependency, depth int,
) (*dependencyNode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if node, ok := r.graph.nodes[dependency.Name]; ok {
		matchingRef := node.declaredRef == ref
		if node.payload.Source == dependency.Source &&
			contract.InstallExtensionSource(dependency.Source) == contract.InstallExtensionSourceLocalPath {
			resolved, matches, err := node.matchLocalDependencyRef(ctx, parent, parentDir, dependency, ref)
			if err != nil {
				return nil, err
			}
			ref, matchingRef = resolved, matches
		}
		if node.payload.Source != dependency.Source || !matchingRef {
			return nil, &extensionpkg.DependencyGraphInvalidError{
				Reason: extensionpkg.DependencyGraphReasonConflictingSource,
				Path:   []string{dependency.Name},
				Parents: []contract.ExtensionDependencyParentPayload{
					{Name: node.declaredBy, Source: node.payload.Source, Ref: node.payload.Ref},
					{Name: parent, Source: dependency.Source, Ref: ref},
				},
			}
		}
		node.payload.MinVersion = extensionpkg.HigherVersion(node.payload.MinVersion, dependency.MinVersion)
		if !slices.Contains(node.payload.RequiredBy, parent) {
			node.payload.RequiredBy = append(node.payload.RequiredBy, parent)
		}
		node.payload.Depth = max(node.payload.Depth, depth)
		return node, nil
	}
	node := &dependencyNode{
		declaredBy: parent, declaredRef: ref, declaredDir: parentDir,
		payload: contract.ExtensionDependencyPayload{
			Name: dependency.Name, Source: dependency.Source, Ref: ref, MinVersion: dependency.MinVersion,
			RequiredBy: []string{parent}, Depth: depth,
		},
	}
	r.graph.nodes[dependency.Name] = node
	if err := r.inspect(ctx, node, dependency, parentDir); err != nil {
		return nil, err
	}
	return node, nil
}

// inspect reads an installation visible to the dependent's instance (D11), or acquires the package.
func (r *dependencyResolver) inspect(
	ctx context.Context, node *dependencyNode, dependency extensionpkg.ExtensionDependency, parentDir string,
) error {
	s, name := r.service, node.payload.Name
	installation, err := s.registry.ResolveInstallation(ctx, name, extensionpkg.InstallationScope{
		ProfileID: r.target.profile.ID, WorkspaceID: r.target.scope.WorkspaceID,
	})
	if errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		if contract.InstallExtensionSource(dependency.Source) == contract.InstallExtensionSourceLocalPath {
			resolved, resolveErr := resolveLocalDependencyRef(ctx, dependency, parentDir, node.declaredRef)
			if resolveErr != nil {
				return resolveErr
			}
			node.payload.Ref = resolved
		}
		return r.acquire(ctx, node)
	}
	if err != nil {
		return err
	}
	info, err := s.registry.GetContext(ctx, name)
	if err != nil {
		return err
	}
	if node.enabled, err = s.registry.IsEnabledForProfileContext(ctx, name, r.target.profile.ID); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	manifest, err := extensionpkg.LoadInstalledManifest(info)
	if err != nil {
		return fmt.Errorf("daemon: read installed dependency %q manifest: %w", name, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	node.installed, node.payload.InstalledVersion = true, info.Version
	node.installedGlobally = installation.Scope.WorkspaceID == ""
	node.children = manifest.Requires.Extensions
	source := info.Provenance.SourceURL
	if info.Provenance.InstalledFrom == extensionpkg.ExtensionInstalledFromLocalPath && filepath.IsAbs(source) {
		node.sourceDir = source
	}
	// The recorded tier, not past consent, says whether the installed package is registry-verified.
	node.payload.RegistryTier = cmp.Or(info.Provenance.RegistryTier, extensionpkg.ExtensionRegistryTierUnverified)
	return nil
}

// A workspace install cannot repair a missing dependency of an already-installed global parent.
func (r *dependencyResolver) requireGlobalDependency(
	ctx context.Context, parent string, dependency extensionpkg.ExtensionDependency, ref string, depth int,
) error {
	_, err := r.service.registry.ResolveInstallation(ctx, dependency.Name, extensionpkg.InstallationScope{
		ProfileID: r.target.profile.ID,
	})
	if !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		return err
	}
	required := &extensionpkg.DependenciesRequiredError{Dependencies: []contract.ExtensionDependencyPayload{{
		Name: dependency.Name, Source: dependency.Source, Ref: ref, MinVersion: dependency.MinVersion,
		RequiredBy: []string{parent}, Depth: depth, State: dependencyStateMissing,
	}}}
	return &extensionpkg.DependencyGateError{
		Dependency: dependency.Name,
		Err:        fmt.Errorf("install %s in the global scope for %s first: %w", dependency.Name, parent, required),
	}
}

// acquire downloads a missing dependency into staging so preview and install read its own manifest (D15).
// A refused unverified artifact is re-acquired to report consent_required; install still demands consent.
func (r *dependencyResolver) acquire(ctx context.Context, node *dependencyNode) error {
	s, name := r.service, node.payload.Name
	consent := r.consent[name]
	req := contract.InstallExtensionRequest{
		Source: contract.InstallExtensionSource(node.payload.Source), Ref: node.payload.Ref,
		Scope: extensionInstallationScopeGlobal, WorkspaceID: r.target.scope.WorkspaceID,
		ExpectedDigest: consent.ExpectedDigest,
	}
	if req.WorkspaceID != "" {
		req.Scope = resourceScopeWorkspaceKind
	}
	if s.marketplaceCache != nil && req.Source == contract.InstallExtensionSourceMarketplace {
		release, err := s.marketplaceCache.Hold(ctx)
		if err != nil {
			return fmt.Errorf("daemon: dependency %q marketplace cache hold: %w", name, err)
		}
		r.graph.releases = append(r.graph.releases, release)
	}
	if req.ExpectedDigest == "" && req.Source == contract.InstallExtensionSourceMarketplace {
		// The first preview has no reviewed digest yet: acquire the listed one so preview can report it.
		// Installing still requires the caller's own pin (checkDependencyDigestConsent).
		listed, err := s.listedMarketplaceDigest(ctx, node.payload.Ref)
		if err != nil {
			return &extensionpkg.DependencyGateError{Dependency: name, Err: err}
		}
		req.ExpectedDigest = listed
	}
	prepared, err := s.prepareExtensionInstall(ctx, req, r.actor, extensionInstalledBy(r.actor), r.target)
	if errors.Is(err, extensionpkg.ErrExtensionChecksumUnverified) {
		node.unverifiedErr, node.payload.ConsentRequired = err, true
		req.AllowUnverified = true
		prepared, err = s.prepareExtensionInstall(ctx, req, r.actor, extensionInstalledBy(r.actor), r.target)
	}
	if err != nil {
		return &extensionpkg.DependencyGateError{Dependency: name, Err: err}
	}
	node.candidate = &prepared
	if prepared.name != name {
		return &extensionpkg.DependencyGateError{Dependency: name, Err: fmt.Errorf(
			"%w: %s resolves to extension %q", extensionpkg.ErrManifestInvalid, node.payload.Ref, prepared.name,
		)}
	}
	node.payload.Version, node.payload.DigestSHA256 = prepared.manifest.Version, prepared.digest
	node.payload.RegistryTier = prepared.registryTier
	node.children, node.sourceDir = prepared.manifest.Requires.Extensions, prepared.sourceDir
	return r.checkInstalledElsewhere(ctx, node)
}

// checkInstalledElsewhere marks a missing node whose package is installed for another scope or profile with
// different bytes, so preview reports it and install refuses before installing any dependency.
func (r *dependencyResolver) checkInstalledElsewhere(ctx context.Context, node *dependencyNode) error {
	installed, err := r.service.registry.GetContext(ctx, node.payload.Name)
	if errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if node.candidate.published == nil || node.candidate.published.MatchesInstalled(installed) {
		return nil
	}
	node.installedElsewhere, node.payload.InstalledVersion = true, installed.Version
	return nil
}

// resolveState classifies the node against its effective (highest) min_version.
func (node *dependencyNode) resolveState() {
	item := &node.payload
	switch {
	case !node.installed && node.installedElsewhere:
		item.State = dependencyStateInstalledElsewhere
	case !node.installed:
		item.State = dependencyStateMissing
		if !extensionpkg.VersionSatisfies(item.Version, item.MinVersion) {
			item.State = dependencyStateVersionUnmet
		}
	case !extensionpkg.VersionSatisfies(item.InstalledVersion, item.MinVersion):
		item.State = dependencyStateVersionUnmet
	case !node.enabled:
		item.State = dependencyStateDisabled
	default:
		item.State, item.InstalledVersion = dependencyStateMet, ""
	}
}
