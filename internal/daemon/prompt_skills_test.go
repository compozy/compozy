package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	commandpkg "github.com/compozy/compozy/internal/command"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestNewSkillsCatalogAugmenterUsesCurrentRegistryStatePerPrompt(t *testing.T) {
	t.Run("Should compact unchanged catalog after the first prompt", func(t *testing.T) {
		t.Parallel()

		registry, augmenter := newPromptSkillsAugmenterForTest(t, []*skillspkg.Skill{
			{
				Meta: skillspkg.SkillMeta{
					Name:        "qa-marker-skill",
					Description: "Shows up while enabled.",
				},
				Enabled: true,
			},
		})
		_ = registry
		sess := newPromptSkillsSession("sess-compact")

		first, err := augmenter(context.Background(), sess, "list current skills")
		if err != nil {
			t.Fatalf("augmenter(first) error = %v", err)
		}
		if !strings.Contains(first, `name="qa-marker-skill"`) {
			t.Fatalf("first prompt = %q, want enabled skill entry", first)
		}

		second, err := augmenter(context.Background(), sess, "list current skills again")
		if err != nil {
			t.Fatalf("augmenter(second) error = %v", err)
		}
		if !strings.Contains(second, `<catalog-state unchanged="true">`) {
			t.Fatalf("second prompt = %q, want unchanged catalog marker", second)
		}
		if strings.Contains(second, `name="qa-marker-skill"`) {
			t.Fatalf("second prompt = %q, want compact marker without repeated skill entries", second)
		}
		if !strings.Contains(second, "resolve canonical `compozy__skill_view` for full skill/resource instructions") {
			t.Fatalf("second prompt = %q, want compact harness-agnostic skill_view guidance", second)
		}
		if !strings.Contains(
			second,
			"Do not invoke `compozy skill view` or read skill files directly from a managed session",
		) {
			t.Fatalf("second prompt = %q, want native-only managed skill guidance", second)
		}
		if !strings.Contains(second, "`compozy skill view` is an operator-shell command only") {
			t.Fatalf("second prompt = %q, want operator-only CLI guidance", second)
		}
		if !strings.HasSuffix(second, "list current skills again") {
			t.Fatalf("second prompt = %q, want original prompt preserved", second)
		}
	})

	t.Run("Should emit refreshed full catalog when registry state changes", func(t *testing.T) {
		t.Parallel()

		registry, augmenter := newPromptSkillsAugmenterForTest(t, []*skillspkg.Skill{
			{
				Meta: skillspkg.SkillMeta{
					Name:        "qa-marker-skill",
					Description: "Shows up while enabled.",
				},
				Enabled: true,
			},
		})
		sess := newPromptSkillsSession("sess-refresh")

		first, err := augmenter(context.Background(), sess, "list current skills")
		if err != nil {
			t.Fatalf("augmenter(first) error = %v", err)
		}
		if !strings.Contains(first, `name="qa-marker-skill"`) {
			t.Fatalf("first prompt = %q, want enabled skill entry", first)
		}

		registry.skillsByAgent["general"] = []*skillspkg.Skill{
			{
				Meta: skillspkg.SkillMeta{
					Name:        "qa-marker-skill",
					Description: "Now disabled.",
				},
				Enabled: false,
			},
			{
				Meta: skillspkg.SkillMeta{
					Name:        "replacement-skill",
					Description: "Still enabled after refresh.",
				},
				Enabled: true,
			},
		}

		second, err := augmenter(context.Background(), sess, "list current skills")
		if err != nil {
			t.Fatalf("augmenter(second) error = %v, want live refresh", err)
		}
		if strings.Contains(second, `name="qa-marker-skill"`) {
			t.Fatalf("second prompt = %q, want disabled skill removed from current catalog", second)
		}
		if !strings.Contains(second, `name="replacement-skill"`) {
			t.Fatalf("second prompt = %q, want refreshed enabled skill in current catalog", second)
		}
		if !strings.Contains(
			second,
			"The <current-available-skills> block above is the authoritative current skill state for this turn.",
		) {
			t.Fatalf("second prompt = %q, want authoritative current-catalog guidance", second)
		}
		if strings.Contains(second, `<catalog-state unchanged="true">`) {
			t.Fatalf("second prompt = %q, want full refreshed catalog instead of unchanged marker", second)
		}
		if !strings.HasSuffix(second, "list current skills") {
			t.Fatalf("second prompt = %q, want original prompt preserved", second)
		}
	})

	t.Run("Should replay the full catalog after the ACP session changes", func(t *testing.T) {
		t.Parallel()

		registry, augmenter := newPromptSkillsAugmenterForTest(t, []*skillspkg.Skill{
			{
				Meta: skillspkg.SkillMeta{
					Name:        "qa-marker-skill",
					Description: "Shows up while enabled.",
				},
				Enabled: true,
			},
		})
		_ = registry
		sess := newPromptSkillsSession("sess-resume")
		sess.ACPSessionID = "acp-1"

		first, err := augmenter(context.Background(), sess, "list current skills")
		if err != nil {
			t.Fatalf("augmenter(first) error = %v", err)
		}
		if !strings.Contains(first, `name="qa-marker-skill"`) {
			t.Fatalf("first prompt = %q, want enabled skill entry", first)
		}

		second, err := augmenter(context.Background(), sess, "list current skills again")
		if err != nil {
			t.Fatalf("augmenter(second) error = %v", err)
		}
		if !strings.Contains(second, `<catalog-state unchanged="true">`) {
			t.Fatalf("second prompt = %q, want unchanged catalog marker", second)
		}

		sess.ACPSessionID = "acp-2"
		third, err := augmenter(context.Background(), sess, "list current skills after resume")
		if err != nil {
			t.Fatalf("augmenter(third) error = %v", err)
		}
		if strings.Contains(third, `<catalog-state unchanged="true">`) {
			t.Fatalf("third prompt = %q, want full catalog after ACP session reset", third)
		}
		if !strings.Contains(third, `name="qa-marker-skill"`) {
			t.Fatalf("third prompt = %q, want enabled skill entry after ACP session reset", third)
		}
		if !strings.HasSuffix(third, "list current skills after resume") {
			t.Fatalf("third prompt = %q, want original prompt preserved", third)
		}
	})

	t.Run("Should activate a tool-gated skill on the next prompt without restart", func(t *testing.T) {
		t.Parallel()

		userDir := t.TempDir()
		skillDir := filepath.Join(userDir, "tool-gated")
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		content := strings.Join([]string{
			"---",
			"name: tool-gated",
			"description: Needs the canonical skill tool.",
			"metadata:",
			"  compozy:",
			"    when:",
			"      requires_tools: [compozy__skill_view]",
			"---",
			"body",
		}, "\n")
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		var toolAvailable atomic.Bool
		registry := skillspkg.NewRegistry(
			skillspkg.RegistryConfig{UserSkillsDir: userDir},
			skillspkg.WithActivationContextProvider(func(
				_ context.Context,
				target skillspkg.ActivationTarget,
			) (skillspkg.ActivationContext, error) {
				if target.SessionID != "sess-tool-gate" {
					return skillspkg.ActivationContext{}, fmt.Errorf("session id = %q", target.SessionID)
				}
				tools := make([]string, 0)
				if toolAvailable.Load() {
					tools = append(tools, "compozy__skill_view")
				}
				return skillspkg.ActivationContext{Platform: "linux", Tools: tools}, nil
			}),
		)
		if err := registry.LoadAll(t.Context()); err != nil {
			t.Fatalf("LoadAll() error = %v", err)
		}
		resolver := &stubPromptSkillsWorkspaceResolver{resolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: "/tmp/ws-1"},
		}}
		augmenter := newSkillsCatalogAugmenter(registry, func() promptSkillsWorkspaceResolver { return resolver })
		sess := newPromptSkillsSession("sess-tool-gate")
		sess.AgentName = "coordinator"

		first, err := augmenter(t.Context(), sess, "first")
		if err != nil {
			t.Fatalf("augmenter(first) error = %v", err)
		}
		if strings.Contains(first, "tool-gated") {
			t.Fatalf("first prompt = %q, want inactive skill omitted", first)
		}

		toolAvailable.Store(true)
		second, err := augmenter(t.Context(), sess, "second")
		if err != nil {
			t.Fatalf("augmenter(second) error = %v", err)
		}
		if !strings.Contains(second, `name="tool-gated"`) {
			t.Fatalf("second prompt = %q, want active skill advertised", second)
		}
		if len(second) <= len(first) {
			t.Fatalf("prompt bytes inactive=%d active=%d, want measured catalog growth", len(first), len(second))
		}
	})

	t.Run("Should refresh authored agents by name while preserving package-owned snapshots", func(t *testing.T) {
		t.Parallel()

		registry, _ := newPromptSkillsAugmenterForTest(t, nil)
		augmenter := &skillsCatalogAugmenter{registry: registry}
		workspace := &workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: "/tmp/ws-1"},
		}
		if _, err := augmenter.skillsForSessionAgent(
			t.Context(),
			workspace,
			compozyconfig.AgentDef{Name: "authored", SourcePath: "/tmp/ws-1/.compozy/agents/authored/AGENT.md"},
			"sess-authored",
		); err != nil {
			t.Fatalf("skillsForSessionAgent(authored) error = %v", err)
		}
		if _, err := augmenter.skillsForSessionAgent(
			t.Context(),
			workspace,
			compozyconfig.AgentDef{Name: "packaged"},
			"sess-packaged",
		); err != nil {
			t.Fatalf("skillsForSessionAgent(packaged) error = %v", err)
		}
		if got, want := registry.resolvedNames, []string{"authored"}; !slices.Equal(got, want) {
			t.Fatalf("resolved agent names = %#v, want %#v", got, want)
		}
		if got, want := registry.concreteNames, []string{"packaged"}; !slices.Equal(got, want) {
			t.Fatalf("concrete agent names = %#v, want %#v", got, want)
		}
	})
}

func TestSessionCommandServiceProjectsAndRevalidatesExactSkillSources(t *testing.T) {
	t.Parallel()
	t.Run("Should project and revalidate exact skill sources", func(t *testing.T) {
		t.Parallel()

		workspaceSkill := &skillspkg.Skill{
			Meta:   skillspkg.SkillMeta{Name: "review", Description: "Workspace review"},
			Source: skillspkg.SourceWorkspace, CommandScope: "workspace", Enabled: true,
			FilePath: "/workspace/review/SKILL.md",
		}
		extensionSkill := &skillspkg.Skill{
			Meta:   skillspkg.SkillMeta{Name: "review", Description: "Extension review"},
			Source: skillspkg.SourceBundled, CommandScope: "workspace", Enabled: true,
			InstalledFromExtension: "ops", FilePath: "/extension/review/SKILL.md",
		}
		registry := &stubSessionCommandSkills{
			candidates: []skillspkg.CommandCandidate{
				{
					Skill: workspaceSkill, SourceKind: "workspace", Scope: "workspace", Available: true,
				},
				{
					Skill: extensionSkill, SourceKind: "extension", SourceID: "ops", SourceKey: "ops",
					Scope: "workspace", Qualified: true, Available: true,
				},
			},
			content: map[string]string{
				workspaceSkill.FilePath: "Workspace instructions.",
				extensionSkill.FilePath: "Extension instructions.",
			},
		}
		resolver := &stubPromptSkillsWorkspaceResolver{resolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: "ws-command", RootDir: "/workspace"},
		}}
		service := newSessionCommandService(registry, nil, func() promptSkillsWorkspaceResolver { return resolver })
		info := &session.Info{
			ID: "sess-command", AgentName: "coder", WorkspaceID: "ws-command", Workspace: "/workspace",
			AdvertisedCommands: []store.SessionAdvertisedCommand{{Name: "compact", Description: "Compact context"}},
		}
		agent := compozyconfig.AgentDef{Name: "coder"}
		catalog, err := service.Catalog(t.Context(), info, agent)
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		tokens := make([]string, 0, len(catalog.Commands))
		for _, descriptor := range catalog.Commands {
			tokens = append(tokens, descriptor.CanonicalToken)
		}
		if got, want := strings.Join(tokens, ","), "/goal,/compact,/ops:review,/review"; got != want {
			t.Fatalf("Catalog() tokens = %q, want %q", got, want)
		}
		invocations, err := commandpkg.ParseSkillInvocations("Please /ops:review this", catalog)
		if err != nil {
			t.Fatalf("ParseSkillInvocations() error = %v", err)
		}
		expanded, err := service.Expand(t.Context(), info, agent, invocations, "Please /ops:review this")
		if err != nil {
			t.Fatalf("Expand() error = %v", err)
		}
		if !strings.Contains(expanded, "Extension instructions.") ||
			!strings.HasSuffix(expanded, "Please /ops:review this") {
			t.Fatalf("Expand() = %q, want verified instructions plus authored text", expanded)
		}

		registry.candidates = registry.candidates[:1]
		_, err = service.Expand(t.Context(), info, agent, invocations, "Please /ops:review this")
		if !errors.Is(err, commandpkg.ErrUnavailable) {
			t.Fatalf("Expand(stale source) error = %v, want ErrUnavailable", err)
		}
	})
}

type stubSessionCommandSkills struct {
	candidates []skillspkg.CommandCandidate
	content    map[string]string
}

func (s *stubSessionCommandSkills) CommandCandidatesForAgentSession(
	context.Context,
	*workspacepkg.ResolvedWorkspace,
	string,
	string,
) ([]skillspkg.CommandCandidate, error) {
	return append([]skillspkg.CommandCandidate(nil), s.candidates...), nil
}

func (s *stubSessionCommandSkills) CommandCandidatesForAgentDefSession(
	context.Context,
	*workspacepkg.ResolvedWorkspace,
	compozyconfig.AgentDef,
	string,
) ([]skillspkg.CommandCandidate, error) {
	return append([]skillspkg.CommandCandidate(nil), s.candidates...), nil
}

func (s *stubSessionCommandSkills) LoadContent(_ context.Context, skill *skillspkg.Skill) (string, error) {
	content, ok := s.content[skill.FilePath]
	if !ok {
		return "", fmt.Errorf("missing content for %s", skill.FilePath)
	}
	return content, nil
}

func BenchmarkSkillsCatalogAugmenterCatalogReplayModes(b *testing.B) {
	registry, augmenter := newPromptSkillsAugmenterForTest(b, []*skillspkg.Skill{
		{
			Meta: skillspkg.SkillMeta{
				Name:        "qa-marker-skill",
				Description: "Shows up while enabled.",
			},
			Enabled: true,
		},
	})
	_ = registry
	sess := newPromptSkillsSession("sess-bench")

	full, err := augmenter(context.Background(), sess, "network note")
	if err != nil {
		b.Fatalf("augmenter(full) error = %v", err)
	}
	compact, err := augmenter(context.Background(), sess, "network note")
	if err != nil {
		b.Fatalf("augmenter(compact) error = %v", err)
	}
	fullBytes := len(full)
	compactBytes := len(compact)
	promptSuffixBytes := len("\n\nnetwork note")
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := augmenter(context.Background(), sess, "network note"); err != nil {
			b.Fatalf("augmenter(repeated) error = %v", err)
		}
	}
	b.ReportMetric(float64(fullBytes-promptSuffixBytes), "full-catalog-bytes/op")
	b.ReportMetric(float64(compactBytes-promptSuffixBytes), "compact-catalog-bytes/op")
	b.ReportMetric(float64(fullBytes-compactBytes), "saved-catalog-bytes/op")
	b.ReportMetric(float64(fullBytes), "full-bytes/op")
	b.ReportMetric(float64(compactBytes), "compact-bytes/op")
	b.ReportMetric(float64(fullBytes-compactBytes), "saved-prompt-bytes/op")
}

func newPromptSkillsAugmenterForTest(
	t testing.TB,
	skills []*skillspkg.Skill,
) (*stubPromptSkillsRegistry, session.PromptInputAugmenter) {
	t.Helper()

	registry := &stubPromptSkillsRegistry{
		skillsByAgent: map[string][]*skillspkg.Skill{
			"general": skills,
		},
	}
	resolver := &stubPromptSkillsWorkspaceResolver{
		resolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      "ws-1",
				RootDir: "/tmp/ws-1",
			},
		},
	}
	augmenter := newSkillsCatalogAugmenter(registry, func() promptSkillsWorkspaceResolver {
		return resolver
	})
	if augmenter == nil {
		t.Fatal("newSkillsCatalogAugmenter() = nil, want augmenter")
	}
	return registry, augmenter
}

func newPromptSkillsSession(sessionID string) *session.Session {
	return &session.Session{
		ID:          sessionID,
		AgentName:   "general",
		WorkspaceID: "ws-1",
		Workspace:   "/tmp/ws-1",
	}
}

type stubPromptSkillsRegistry struct {
	skillsByAgent map[string][]*skillspkg.Skill
	resolvedNames []string
	concreteNames []string
}

func (s *stubPromptSkillsRegistry) ForWorkspace(
	_ context.Context,
	_ *workspacepkg.ResolvedWorkspace,
) ([]*skillspkg.Skill, error) {
	return nil, nil
}

func (s *stubPromptSkillsRegistry) ForAgentDefSession(
	_ context.Context,
	_ *workspacepkg.ResolvedWorkspace,
	agent compozyconfig.AgentDef,
	_ string,
) ([]*skillspkg.Skill, error) {
	if s == nil {
		return nil, nil
	}
	s.concreteNames = append(s.concreteNames, agent.Name)
	return append([]*skillspkg.Skill(nil), s.skillsByAgent[agent.Name]...), nil
}

func (s *stubPromptSkillsRegistry) ForAgentSession(
	_ context.Context,
	_ *workspacepkg.ResolvedWorkspace,
	agentName string,
	_ string,
) ([]*skillspkg.Skill, error) {
	if s == nil {
		return nil, nil
	}
	s.resolvedNames = append(s.resolvedNames, agentName)
	return append([]*skillspkg.Skill(nil), s.skillsByAgent[agentName]...), nil
}

type stubPromptSkillsWorkspaceResolver struct {
	resolved workspacepkg.ResolvedWorkspace
}

func (s *stubPromptSkillsWorkspaceResolver) Resolve(
	_ context.Context,
	_ string,
) (workspacepkg.ResolvedWorkspace, error) {
	if s == nil {
		return workspacepkg.ResolvedWorkspace{}, nil
	}
	return s.resolved, nil
}
