package daemon

import (
	"context"
	"errors"
	"testing"

	commandpkg "github.com/compozy/compozy/internal/command"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// extensionOnlyAgentCommandSkills mimics the real skills.Registry behavior exercised by the
// bug: the name-based lookup (CommandCandidatesForAgentSession -> resolveAgentScope) only sees
// workspace-authored, builtin, and global authored agents, so it fails with
// skillspkg.ErrAgentNotFound for an agent that is published exclusively by an extension. The
// AgentDef-based lookup (CommandCandidatesForAgentDefSession) supports extension agents and
// succeeds whenever it is given the concrete definition. nameBasedCalls counts invocations of
// CommandCandidatesForAgentSession so tests can prove whether the name-based fallback ran.
type extensionOnlyAgentCommandSkills struct {
	candidates     []skillspkg.CommandCandidate
	nameBasedCalls int
}

// CommandCandidatesForAgentSession always fails, matching the real registry's inability to
// resolve an extension-only agent through the name-based path, and records the call so tests can
// assert whether the fallback ran.
func (s *extensionOnlyAgentCommandSkills) CommandCandidatesForAgentSession(
	context.Context,
	*workspacepkg.ResolvedWorkspace,
	string,
	string,
) ([]skillspkg.CommandCandidate, error) {
	s.nameBasedCalls++
	return nil, skillspkg.ErrAgentNotFound
}

// CommandCandidatesForAgentDefSession returns the stubbed candidates once given a concrete def.
func (s *extensionOnlyAgentCommandSkills) CommandCandidatesForAgentDefSession(
	context.Context,
	*workspacepkg.ResolvedWorkspace,
	compozyconfig.AgentDef,
	string,
) ([]skillspkg.CommandCandidate, error) {
	return append([]skillspkg.CommandCandidate(nil), s.candidates...), nil
}

// LoadContent is unused by these tests; it exists only to satisfy sessionCommandSkills.
func (s *extensionOnlyAgentCommandSkills) LoadContent(
	context.Context,
	*skillspkg.Skill,
) (string, error) {
	return "", nil
}

// stubExtensionAgentResolver mimics resourceAgentCatalog.ResolveAgent: it resolves the concrete
// AgentDef for an extension-published agent that has no on-disk SourcePath.
type stubExtensionAgentResolver struct {
	agents map[string]compozyconfig.AgentDef
}

// ResolveAgent resolves a stubbed agent by name, or reports it not found — mirroring
// resourceAgentCatalog.ResolveAgent's contract for callers of sessionCommandAgentResolver.
func (r *stubExtensionAgentResolver) ResolveAgent(
	name string,
	_ *workspacepkg.ResolvedWorkspace,
) (compozyconfig.AgentDef, error) {
	if def, ok := r.agents[name]; ok {
		return def, nil
	}
	return compozyconfig.AgentDef{}, skillspkg.ErrAgentNotFound
}

// TestSessionCommandServiceCommandSkillCandidatesExtensionAgent covers commandSkillCandidates'
// no-snapshot fallback chain for extension-published agents: catalog resolution first, the
// name-based lookup as a final fallback, and the lazy-provider timing the boot-order bug hit.
func TestSessionCommandServiceCommandSkillCandidatesExtensionAgent(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should resolve command skill candidates for an extension-published agent without a session snapshot",
		func(t *testing.T) {
			t.Parallel()
			reviewSkill := &skillspkg.Skill{
				Meta: skillspkg.SkillMeta{Name: "review", Description: "Review changes."},
			}
			registry := &extensionOnlyAgentCommandSkills{
				candidates: []skillspkg.CommandCandidate{
					{Skill: reviewSkill, SourceKind: "extension", SourceID: "dev-cycle", Available: true},
				},
			}
			resolver := &stubExtensionAgentResolver{
				agents: map[string]compozyconfig.AgentDef{
					"reviewer": {Name: "reviewer"},
				},
			}
			workspaceResolver := &stubPromptSkillsWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: "ws-ext", RootDir: "/workspace"},
				},
			}
			service := newSessionCommandService(
				registry,
				func() sessionCommandAgentResolver { return resolver },
				func() promptSkillsWorkspaceResolver { return workspaceResolver },
			)
			info := &session.Info{
				ID:          "sess-ext",
				AgentName:   "reviewer",
				WorkspaceID: "ws-ext",
				Workspace:   "/workspace",
			}
			// No agent snapshot on the session (agent.Name is empty) — this is the state a
			// session bound to an extension-published agent is in when its snapshot has not
			// been persisted, forcing the name-based fallback path. Drive the public Catalog()
			// entry point rather than the unexported commandSkillCandidates helper.
			catalog, err := service.Catalog(t.Context(), info, compozyconfig.AgentDef{})
			if err != nil {
				t.Fatalf("Catalog() error = %v", err)
			}
			var found *commandpkg.Descriptor
			for index := range catalog.Commands {
				if catalog.Commands[index].Skill != nil && catalog.Commands[index].Skill.Name == reviewSkill.Meta.Name {
					found = &catalog.Commands[index]
					break
				}
			}
			if found == nil {
				t.Fatalf(
					"Catalog() commands = %+v, want the extension-published %q command",
					catalog.Commands,
					reviewSkill.Meta.Name,
				)
			}
			if found.Lane != commandpkg.LaneSkill || found.Source.Kind != "extension" {
				t.Fatalf("Catalog() command = %+v, want lane=skill source.kind=extension", found)
			}
		},
	)

	t.Run(
		"Should fall back to the name-based lookup when the agent resolver has no match",
		func(t *testing.T) {
			t.Parallel()
			registry := &extensionOnlyAgentCommandSkills{}
			resolver := &stubExtensionAgentResolver{agents: map[string]compozyconfig.AgentDef{}}
			workspaceResolver := &stubPromptSkillsWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: "ws-ext", RootDir: "/workspace"},
				},
			}
			service := newSessionCommandService(
				registry,
				func() sessionCommandAgentResolver { return resolver },
				func() promptSkillsWorkspaceResolver { return workspaceResolver },
			)
			info := &session.Info{
				ID:          "sess-ext",
				AgentName:   "unknown-agent",
				WorkspaceID: "ws-ext",
				Workspace:   "/workspace",
			}
			_, err := service.commandSkillCandidates(t.Context(), info, compozyconfig.AgentDef{})
			if !errors.Is(err, skillspkg.ErrAgentNotFound) {
				t.Fatalf("commandSkillCandidates() error = %v, want wrapping %v", err, skillspkg.ErrAgentNotFound)
			}
			if registry.nameBasedCalls != 1 {
				t.Fatalf(
					"CommandCandidatesForAgentSession call count = %d, want 1 (the name-based fallback must run)",
					registry.nameBasedCalls,
				)
			}
		},
	)

	t.Run(
		"Should use the resolver once the lazily-provided dependency becomes available after boot",
		func(t *testing.T) {
			t.Parallel()
			// Regression for the boot-order bug: bootPromptProviders constructs
			// sessionCommandService (and evaluates any eagerly-captured resolver) before
			// bootRuntime populates state.agentCatalog, so a resolver captured by value at
			// construction time is permanently nil. newSessionCommandService instead takes a
			// provider func, evaluated at call time — this reproduces that timing in
			// miniature: the provider returns nil the first time (simulating construction
			// before boot finishes) and a working resolver thereafter (simulating boot
			// finishing before any session actually calls the service).
			reviewSkill := &skillspkg.Skill{
				Meta: skillspkg.SkillMeta{Name: "review", Description: "Review changes."},
			}
			registry := &extensionOnlyAgentCommandSkills{
				candidates: []skillspkg.CommandCandidate{
					{Skill: reviewSkill, SourceKind: "extension", SourceID: "dev-cycle", Available: true},
				},
			}
			resolver := &stubExtensionAgentResolver{
				agents: map[string]compozyconfig.AgentDef{
					"reviewer": {Name: "reviewer"},
				},
			}
			providerCalls := 0
			workspaceResolver := &stubPromptSkillsWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: "ws-ext", RootDir: "/workspace"},
				},
			}
			service := newSessionCommandService(
				registry,
				func() sessionCommandAgentResolver {
					providerCalls++
					if providerCalls == 1 {
						// The dependency has not finished booting yet on this call.
						return nil
					}
					return resolver
				},
				func() promptSkillsWorkspaceResolver { return workspaceResolver },
			)
			if providerCalls != 0 {
				t.Fatalf("provider calls at construction time = %d, want 0 (must not be evaluated eagerly)", providerCalls)
			}
			info := &session.Info{
				ID:          "sess-ext",
				AgentName:   "reviewer",
				WorkspaceID: "ws-ext",
				Workspace:   "/workspace",
			}
			// First call: provider still returns nil, must fall back to the name-based path.
			_, err := service.commandSkillCandidates(t.Context(), info, compozyconfig.AgentDef{})
			if !errors.Is(err, skillspkg.ErrAgentNotFound) {
				t.Fatalf("first commandSkillCandidates() error = %v, want wrapping %v", err, skillspkg.ErrAgentNotFound)
			}
			// Second call: the dependency is now available; must resolve via the catalog.
			candidates, err := service.commandSkillCandidates(t.Context(), info, compozyconfig.AgentDef{})
			if err != nil {
				t.Fatalf("second commandSkillCandidates() error = %v", err)
			}
			if len(candidates) != 1 || candidates[0].Skill != reviewSkill {
				t.Fatalf("second commandSkillCandidates() = %+v, want the extension-published candidate", candidates)
			}
			if providerCalls != 2 {
				t.Fatalf("provider calls = %d, want 2 (evaluated once per commandSkillCandidates call)", providerCalls)
			}
		},
	)
}
