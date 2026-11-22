package daemon

import (
	"cmp"
	"context"
	"errors"
	"fmt"
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
	maxDependencyDepth          = 5
)

// dependencyNode is one extension name of the graph; diamonds collapse onto it.
type dependencyNode struct {
	payload    contract.ExtensionDependencyPayload
	declaredBy string
	installed  bool
	enabled    bool
	children   []extensionpkg.ExtensionDependency
	// candidate is the acquired package of a missing node; whoever installs it takes ownership.
	candidate *preparedDaemonExtensionInstall
	// unverifiedErr is the trust refusal replayed when a consent-required node lacks allow_unverified.
	unverifiedErr error
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

// gate refuses unmet dependencies without consent, and version_unmet ones always (rule 12).
func (g *dependencyGraph) gate(withDependencies bool) error {
	var unmet []contract.ExtensionDependencyPayload
	versionUnmet := false
	for _, item := range g.payloads() {
		if item.State == dependencyStateMet {
			continue
		}
		unmet = append(unmet, item)
		versionUnmet = versionUnmet || item.State == dependencyStateVersionUnmet
	}
	if len(unmet) == 0 || (withDependencies && !versionUnmet) {
		return nil
	}
	return &extensionpkg.DependenciesRequiredError{Dependencies: unmet}
}

type dependencyResolver struct {
	service  *daemonExtensionService
	target   extensionMutationTarget
	consent  map[string]contract.ExtensionDependencyConsent
	actor    taskpkg.ActorContext
	graph    *dependencyGraph
	finished map[string]bool
}

// resolveDependencies walks root's transitive requires.extensions before any mutation (rules 11 and 12).
// Every path is expanded, so depth and cycles are checked on each chain; acquisitions happen once per name.
func (s *daemonExtensionService) resolveDependencies(
	ctx context.Context, root *extensionpkg.Manifest, target extensionMutationTarget,
	consent map[string]contract.ExtensionDependencyConsent, actor taskpkg.ActorContext,
) (*dependencyGraph, error) {
	graph := &dependencyGraph{nodes: make(map[string]*dependencyNode)}
	if root == nil || len(root.Requires.Extensions) == 0 {
		return graph, nil
	}
	resolver := dependencyResolver{
		service: s, target: target, consent: consent, actor: actor, graph: graph, finished: make(map[string]bool),
	}
	if err := resolver.expand(ctx, root.Name, root.Requires.Extensions, []string{root.Name}); err != nil {
		return nil, errors.Join(err, graph.Close())
	}
	for _, node := range graph.order {
		node.resolveState()
	}
	return graph, nil
}

func (r *dependencyResolver) expand(
	ctx context.Context, parent string, dependencies []extensionpkg.ExtensionDependency, path []string,
) error {
	for _, dependency := range dependencies {
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
		node, err := r.visit(ctx, parent, dependency, len(chain)-1)
		if err != nil {
			return err
		}
		if err := r.expand(ctx, dependency.Name, node.children, chain); err != nil {
			return err
		}
		if !r.finished[dependency.Name] {
			r.finished[dependency.Name] = true
			r.graph.order = append(r.graph.order, node)
		}
	}
	return nil
}

func (r *dependencyResolver) visit(
	ctx context.Context, parent string, dependency extensionpkg.ExtensionDependency, depth int,
) (*dependencyNode, error) {
	if node, ok := r.graph.nodes[dependency.Name]; ok {
		if node.payload.Source != dependency.Source || node.payload.Ref != dependency.Ref {
			return nil, &extensionpkg.DependencyGraphInvalidError{
				Reason: extensionpkg.DependencyGraphReasonConflictingSource,
				Path:   []string{dependency.Name},
				Parents: []contract.ExtensionDependencyParentPayload{
					{Name: node.declaredBy, Source: node.payload.Source, Ref: node.payload.Ref},
					{Name: parent, Source: dependency.Source, Ref: dependency.Ref},
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
		declaredBy: parent,
		payload: contract.ExtensionDependencyPayload{
			Name: dependency.Name, Source: dependency.Source, Ref: dependency.Ref, MinVersion: dependency.MinVersion,
			RequiredBy: []string{parent}, Depth: depth,
		},
	}
	r.graph.nodes[dependency.Name] = node
	if err := r.inspect(ctx, node); err != nil {
		return nil, err
	}
	return node, nil
}

// inspect reads an installation visible to the dependent's instance (D11), or acquires the package.
func (r *dependencyResolver) inspect(ctx context.Context, node *dependencyNode) error {
	s, name := r.service, node.payload.Name
	_, err := s.registry.ResolveInstallation(ctx, name, extensionpkg.InstallationScope{
		ProfileID: r.target.profile.ID, WorkspaceID: r.target.scope.WorkspaceID,
	})
	if errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		return r.acquire(ctx, node)
	}
	if err != nil {
		return err
	}
	info, err := s.registry.Get(name)
	if err != nil {
		return err
	}
	if node.enabled, err = s.registry.IsEnabledForProfile(name, r.target.profile.ID); err != nil {
		return err
	}
	manifest, err := extensionpkg.LoadInstalledManifest(info)
	if err != nil {
		return fmt.Errorf("daemon: read installed dependency %q manifest: %w", name, err)
	}
	node.installed, node.payload.InstalledVersion = true, info.Version
	node.children = manifest.Requires.Extensions
	// The recorded tier, not past consent, says whether the installed package is registry-verified.
	node.payload.RegistryTier = cmp.Or(info.Provenance.RegistryTier, extensionpkg.ExtensionRegistryTierUnverified)
	// Enabling repeats the gateway gate only when the installed confirmation does not cover the requirement.
	digest, required, err := candidateGatewayConfirmationRequirement(info, manifest)
	if err != nil {
		return fmt.Errorf("daemon: read installed dependency %q gateway requirement: %w", name, err)
	}
	if required {
		node.payload.GatewayRequirementDigest = digest
	}
	return nil
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
	digest, err := extensionpkg.GatewayRequirementDigest(prepared.manifest.Gateway)
	if err != nil {
		return &extensionpkg.DependencyGateError{Dependency: name, Err: err}
	}
	node.payload.Version, node.payload.DigestSHA256 = prepared.manifest.Version, prepared.digest
	node.payload.GatewayRequirementDigest, node.payload.RegistryTier = digest, prepared.registryTier
	node.children = prepared.manifest.Requires.Extensions
	return nil
}

// resolveState classifies the node against its effective (highest) min_version.
func (node *dependencyNode) resolveState() {
	item := &node.payload
	switch {
	case !node.installed:
		item.State = dependencyStateMissing
		if !extensionpkg.VersionSatisfies(item.Version, item.MinVersion) {
			item.State = dependencyStateVersionUnmet
		}
	case !extensionpkg.VersionSatisfies(item.InstalledVersion, item.MinVersion):
		item.State = dependencyStateVersionUnmet
		item.RetryCommand = "compozy extension update " + item.Name
	case !node.enabled:
		item.State = dependencyStateDisabled
	default:
		item.State, item.InstalledVersion = dependencyStateMet, ""
	}
}
