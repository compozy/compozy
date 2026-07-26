package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	skillbundled "github.com/compozy/agh/skills"
)

func TestComposedAssemblerAssemble(t *testing.T) {
	t.Parallel()

	t.Run("Should zero providers returns trimmed base prompt", func(t *testing.T) {
		t.Parallel()

		assembler := NewComposedAssembler()
		got := assemblePrompt(t, assembler, testPromptAgent("  Base prompt.\n"), t.TempDir())

		if got != "Base prompt." {
			t.Fatalf("Assemble() = %q, want %q", got, "Base prompt.")
		}
	})

	t.Run("Should prepend provider renders before base prompt", func(t *testing.T) {
		t.Parallel()

		assembler := NewComposedAssembler(
			WithPrependPromptProviders(staticPromptProvider("# Memory section")),
		)

		got := assemblePrompt(t, assembler, testPromptAgent("Base prompt."), t.TempDir())
		want := "# Memory section\n\nBase prompt."
		if got != want {
			t.Fatalf("Assemble() = %q, want %q", got, want)
		}
	})

	t.Run("Should append provider renders after base prompt", func(t *testing.T) {
		t.Parallel()

		assembler := NewComposedAssembler(
			WithAppendPromptProviders(staticPromptProvider("<available-skills />")),
		)

		got := assemblePrompt(t, assembler, testPromptAgent("Base prompt."), t.TempDir())
		want := "Base prompt.\n\n<available-skills />"
		if got != want {
			t.Fatalf("Assemble() = %q, want %q", got, want)
		}
	})

	t.Run("Should prepend and append providers preserve ordering", func(t *testing.T) {
		t.Parallel()

		assembler := NewComposedAssembler(
			WithPrependPromptProviders(staticPromptProvider("# Memory section")),
			WithAppendPromptProviders(staticPromptProvider("<available-skills />")),
		)

		got := assemblePrompt(t, assembler, testPromptAgent("Base prompt."), t.TempDir())
		want := "# Memory section\n\nBase prompt.\n\n<available-skills />"
		if got != want {
			t.Fatalf("Assemble() = %q, want %q", got, want)
		}
	})

	t.Run("Should nil providers are skipped", func(t *testing.T) {
		t.Parallel()

		var nilProvider session.PromptProvider
		assembler := NewComposedAssembler(
			WithPrependPromptProviders(nilProvider, staticPromptProvider("# Memory section")),
			WithAppendPromptProviders(nilProvider, staticPromptProvider("<available-skills />")),
		)

		got := assemblePrompt(t, assembler, testPromptAgent("Base prompt."), t.TempDir())
		want := "# Memory section\n\nBase prompt.\n\n<available-skills />"
		if got != want {
			t.Fatalf("Assemble() = %q, want %q", got, want)
		}
	})

	t.Run("Should provider errors are returned", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("boom")
		assembler := NewComposedAssembler(
			WithAppendPromptProviders(errorPromptProvider{err: wantErr}),
		)
		workspace := testResolvedWorkspace(t.TempDir())

		_, err := assembler.Assemble(
			context.Background(),
			testPromptAgent("Base prompt."),
			&workspace,
		)
		if !errors.Is(err, wantErr) {
			t.Fatalf("Assemble() error = %v, want error wrapping %v", err, wantErr)
		}
	})

	t.Run("Should empty provider sections do not add whitespace", func(t *testing.T) {
		t.Parallel()

		assembler := NewComposedAssembler(
			WithPrependPromptProviders(staticPromptProvider("   \n\t")),
			WithAppendPromptProviders(staticPromptProvider(" \n ")),
		)

		got := assemblePrompt(t, assembler, testPromptAgent("Base prompt."), t.TempDir())
		if got != "Base prompt." {
			t.Fatalf("Assemble() = %q, want %q", got, "Base prompt.")
		}
	})

	t.Run("Should workspace is passed to all providers", func(t *testing.T) {
		t.Parallel()

		prepend := &recordingPromptProvider{section: "# Memory section"}
		appendProvider := &recordingPromptProvider{section: "<available-skills />"}
		workspace := filepath.Join(t.TempDir(), "workspace")

		assembler := NewComposedAssembler(
			WithPrependPromptProviders(prepend),
			WithAppendPromptProviders(appendProvider),
		)

		got := assemblePrompt(t, assembler, testPromptAgent("Base prompt."), workspace)
		want := "# Memory section\n\nBase prompt.\n\n<available-skills />"
		if got != want {
			t.Fatalf("Assemble() = %q, want %q", got, want)
		}
		if len(prepend.workspaces) != 1 || prepend.workspaces[0] != workspace {
			t.Fatalf("prepend provider workspaces = %v, want [%q]", prepend.workspaces, workspace)
		}
		if len(appendProvider.workspaces) != 1 || appendProvider.workspaces[0] != workspace {
			t.Fatalf("append provider workspaces = %v, want [%q]", appendProvider.workspaces, workspace)
		}
	})

	t.Run("Should nil assembler returns trimmed base prompt", func(t *testing.T) {
		t.Parallel()

		var assembler *ComposedAssembler
		workspace := testResolvedWorkspace(t.TempDir())
		got, err := assembler.Assemble(
			context.Background(),
			testPromptAgent("  Base prompt.\n"),
			&workspace,
		)
		if err != nil {
			t.Fatalf("Assemble() error = %v", err)
		}
		if got != "Base prompt." {
			t.Fatalf("Assemble() = %q, want %q", got, "Base prompt.")
		}
	})

	t.Run("Should empty and nil options are ignored", func(t *testing.T) {
		t.Parallel()

		assembler := NewComposedAssembler(
			nil,
			WithPrependPromptProviders(),
			WithAppendPromptProviders(),
		)

		got := assemblePrompt(t, assembler, testPromptAgent("Base prompt."), t.TempDir())
		if got != "Base prompt." {
			t.Fatalf("Assemble() = %q, want %q", got, "Base prompt.")
		}
	})
}

func TestComposedAssemblerResumeContext(t *testing.T) {
	t.Parallel()

	t.Run("Should delegate selected resume sections in descriptor order", func(t *testing.T) {
		t.Parallel()

		first := &resumeContextPromptProvider{section: "<checkpoint>first</checkpoint>"}
		second := &resumeContextPromptProvider{section: "<recovery>second</recovery>"}
		assembler := NewComposedAssembler(WithPromptSectionDescriptors(
			PromptSectionDescriptor{
				Name:     "second",
				Position: PromptSectionPositionAppend,
				Order:    20,
				Provider: second,
			},
			PromptSectionDescriptor{
				Name:     "first",
				Position: PromptSectionPositionPrepend,
				Order:    10,
				Provider: first,
			},
		))
		startup := session.StartupPromptContext{
			SessionID:   "sess-alpha",
			WorkspaceID: "ws-alpha",
			Workspace:   "/workspace/alpha",
		}
		got, err := assembler.ResumeContextSection(testutil.Context(t), startup)
		if err != nil {
			t.Fatalf("ResumeContextSection() error = %v", err)
		}
		want := "<checkpoint>first</checkpoint>\n\n<recovery>second</recovery>"
		if got != want {
			t.Fatalf("ResumeContextSection() = %q, want %q", got, want)
		}
		if first.startup != startup || second.startup != startup {
			t.Fatalf(
				"ResumeContextSection() contexts = (%#v, %#v), want %#v",
				first.startup,
				second.startup,
				startup,
			)
		}
	})
}

func TestComposedAssemblerRegressionMatchesMemoryAssembler(t *testing.T) {
	t.Parallel()

	env := newComposedAssemblerMemoryEnv(t)
	env.writeGlobalIndex(t, "- [Global](global.md) - global note")
	env.writeWorkspaceIndex(t, "- [Workspace](workspace.md) - workspace note")

	memoryAssembler := memory.NewAssembler(env.store)
	composedAssembler := NewComposedAssembler(
		WithPrependPromptProviders(memoryAssembler),
	)

	workspace := testResolvedWorkspace(env.workspace)
	got, err := composedAssembler.Assemble(context.Background(), env.agent, &workspace)
	if err != nil {
		t.Fatalf("ComposedAssembler.Assemble() error = %v", err)
	}

	want, err := memoryAssembler.Assemble(context.Background(), env.agent, &workspace)
	if err != nil {
		t.Fatalf("memory.Assemble() error = %v", err)
	}

	if got != want {
		t.Fatalf("memory-only regression mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestComposedAssemblerAssembleStartupUsesEligibleSectionOrdering(t *testing.T) {
	t.Parallel()

	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		RuntimeIdentityPromptSectionEnabled: true,
		MemoryPromptSectionEnabled:          true,
		SkillsPromptSectionEnabled:          true,
	})
	assembler := NewComposedAssembler(
		WithSectionSelector(NewSectionSelector(resolver, nil)),
		WithPromptSectionDescriptors(
			PromptSectionDescriptor{
				Name:      string(HarnessPromptSectionRuntimeIdentity),
				Position:  PromptSectionPositionPrepend,
				Order:     10,
				Provider:  staticPromptProvider("runtime block"),
				Predicate: policyIncludesSection(HarnessPromptSectionRuntimeIdentity),
			},
			PromptSectionDescriptor{
				Name:     string(HarnessPromptSectionNetwork),
				Position: PromptSectionPositionAppend,
				Order:    200,
				Provider: staticPromptProvider("network block"),
				Predicate: policyIncludesSection(
					HarnessPromptSectionNetwork,
				),
			},
			PromptSectionDescriptor{
				Name:     string(HarnessPromptSectionSkills),
				Position: PromptSectionPositionAppend,
				Order:    100,
				Provider: staticPromptProvider("skills block"),
				Predicate: policyIncludesSection(
					HarnessPromptSectionSkills,
				),
			},
			PromptSectionDescriptor{
				Name:     string(HarnessPromptSectionMemory),
				Position: PromptSectionPositionPrepend,
				Order:    100,
				Provider: staticPromptProvider("memory block"),
				Predicate: policyIncludesSection(
					HarnessPromptSectionMemory,
				),
			},
		),
	)

	got := assembleStartupPrompt(
		t,
		assembler,
		session.StartupPromptContext{
			SessionType:          session.SessionTypeUser,
			NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
		},
		testPromptAgent("Base prompt."),
		t.TempDir(),
	)

	want := "runtime block\n\nmemory block\n\nBase prompt.\n\nskills block\n\nnetwork block"
	if got != want {
		t.Fatalf("AssembleStartup() = %q, want %q", got, want)
	}
}

func TestComposedAssemblerAssembleStartupIncludesSoulSection(t *testing.T) {
	t.Parallel()

	t.Run("Should render soul after base prompt and before append sections", func(t *testing.T) {
		t.Parallel()

		assembler := NewComposedAssembler(
			WithPromptSectionDescriptors(
				PromptSectionDescriptor{
					Name:             "soul",
					Position:         PromptSectionPositionAppend,
					Order:            50,
					Provider:         soulPromptSectionProvider{},
					StartupPredicate: startupHasSoulSnapshot,
				},
				PromptSectionDescriptor{
					Name:     "skills",
					Position: PromptSectionPositionAppend,
					Order:    100,
					Provider: staticPromptProvider("skills block"),
				},
			),
		)
		snapshot := testPromptSoulSnapshot(t, "Review implementation behavior.")

		got := assembleStartupPrompt(
			t,
			assembler,
			session.StartupPromptContext{SoulSnapshot: snapshot},
			testPromptAgent("Base prompt."),
			t.TempDir(),
		)

		assertPromptOrder(t, got, []string{"Base prompt.", "<agh-agent-soul>", "skills block"})
		for _, want := range []string{
			"# Agent Soul",
			"Snapshot ID: soul-test",
			"Digest: sha256:",
			"Role: Reviewer",
			"- direct",
			"Review implementation behavior.",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("AssembleStartup() = %q, want substring %q", got, want)
			}
		}
	})

	t.Run("Should omit soul section when startup has no snapshot", func(t *testing.T) {
		t.Parallel()

		assembler := NewComposedAssembler(
			WithPromptSectionDescriptors(
				PromptSectionDescriptor{
					Name:             "soul",
					Position:         PromptSectionPositionAppend,
					Order:            50,
					Provider:         soulPromptSectionProvider{},
					StartupPredicate: startupHasSoulSnapshot,
				},
				PromptSectionDescriptor{
					Name:     "skills",
					Position: PromptSectionPositionAppend,
					Order:    100,
					Provider: staticPromptProvider("skills block"),
				},
			),
		)

		got := assembleStartupPrompt(
			t,
			assembler,
			session.StartupPromptContext{},
			testPromptAgent("Base prompt."),
			t.TempDir(),
		)

		want := "Base prompt.\n\nskills block"
		if got != want {
			t.Fatalf("AssembleStartup() = %q, want %q", got, want)
		}
		if strings.Contains(got, "<agh-agent-soul>") {
			t.Fatalf("AssembleStartup() included soul section without snapshot: %q", got)
		}
	})

	t.Run("Should apply descriptor budget to soul startup section", func(t *testing.T) {
		t.Parallel()

		assembler := NewComposedAssembler(
			WithPromptSectionDescriptors(
				PromptSectionDescriptor{
					Name:             "soul",
					Position:         PromptSectionPositionAppend,
					Order:            50,
					Budget:           96,
					BudgetBehavior:   PromptSectionBudgetBehaviorTrim,
					Provider:         soulPromptSectionProvider{},
					StartupPredicate: startupHasSoulSnapshot,
				},
			),
		)
		snapshot := testPromptSoulSnapshot(t, strings.Repeat("long body ", 40)+"tail-marker")

		got := assembleStartupPrompt(
			t,
			assembler,
			session.StartupPromptContext{SoulSnapshot: snapshot},
			testPromptAgent("Base prompt."),
			t.TempDir(),
		)

		if !strings.Contains(got, "<agh-agent-soul>") {
			t.Fatalf("AssembleStartup() = %q, want trimmed soul section", got)
		}
		if strings.Contains(got, "tail-marker") {
			t.Fatalf("AssembleStartup() did not trim over-budget soul body: %q", got)
		}
	})
}

func TestComposedAssemblerAppliesBudgetPolicies(t *testing.T) {
	t.Parallel()

	assembler := NewComposedAssembler(
		WithPromptSectionDescriptors(
			PromptSectionDescriptor{
				Name:           "trimmed",
				Position:       PromptSectionPositionPrepend,
				Order:          10,
				Budget:         5,
				BudgetBehavior: PromptSectionBudgetBehaviorTrim,
				Provider:       staticPromptProvider("123456789"),
			},
			PromptSectionDescriptor{
				Name:           "omitted",
				Position:       PromptSectionPositionAppend,
				Order:          20,
				Budget:         4,
				BudgetBehavior: PromptSectionBudgetBehaviorOmit,
				Provider:       staticPromptProvider("abcdef"),
			},
			PromptSectionDescriptor{
				Name:     "empty",
				Position: PromptSectionPositionAppend,
				Order:    30,
				Provider: staticPromptProvider("   \n\t"),
			},
		),
	)

	got := assemblePrompt(t, assembler, testPromptAgent("Base prompt."), t.TempDir())
	want := "12345\n\nBase prompt."
	if got != want {
		t.Fatalf("Assemble() = %q, want %q", got, want)
	}
}

func TestComposedAssemblerDeduplicatesEligibleSectionNames(t *testing.T) {
	t.Parallel()

	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		MemoryPromptSectionEnabled: true,
		SkillsPromptSectionEnabled: true,
	})
	assembler := NewComposedAssembler(
		WithSectionSelector(NewSectionSelector(resolver, nil)),
		WithPromptSectionDescriptors(
			PromptSectionDescriptor{
				Name:     string(HarnessPromptSectionMemory),
				Position: PromptSectionPositionPrepend,
				Order:    100,
				Provider: staticPromptProvider("memory block"),
				Predicate: policyIncludesSection(
					HarnessPromptSectionMemory,
				),
			},
			PromptSectionDescriptor{
				Name:     string(HarnessPromptSectionNetwork),
				Position: PromptSectionPositionAppend,
				Order:    200,
				Provider: staticPromptProvider("network block"),
				Predicate: policyIncludesSection(
					HarnessPromptSectionNetwork,
				),
			},
			PromptSectionDescriptor{
				Name:     string(HarnessPromptSectionNetwork),
				Position: PromptSectionPositionAppend,
				Order:    210,
				Provider: staticPromptProvider("network block duplicate"),
				Predicate: policyIncludesSection(
					HarnessPromptSectionNetwork,
				),
			},
		),
	)

	got := assembleStartupPrompt(
		t,
		assembler,
		session.StartupPromptContext{
			SessionType:          session.SessionTypeUser,
			NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
		},
		testPromptAgent("Base prompt."),
		t.TempDir(),
	)

	if strings.Count(got, "network block") != 1 {
		t.Fatalf("network block occurrences = %d, want 1", strings.Count(got, "network block"))
	}
	if strings.Contains(got, "network block duplicate") {
		t.Fatalf("assembled prompt unexpectedly contains duplicate network block: %q", got)
	}
}

func TestComposedAssemblerAssembleStartupLoadsNetworkResponseRegisterSection(t *testing.T) {
	t.Parallel()

	t.Run("Should load compact network response register section", func(t *testing.T) {
		t.Parallel()

		resolver := NewHarnessContextResolver(HarnessRuntimeSignals{})
		assembler := NewComposedAssembler(
			WithSectionSelector(NewSectionSelector(resolver, nil)),
			WithPromptSectionDescriptors(defaultStartupPromptSectionDescriptors(nil, nil, nil)...),
		)

		got := assembleStartupPrompt(
			t,
			assembler,
			session.StartupPromptContext{
				SessionType:          session.SessionTypeUser,
				NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
			},
			testPromptAgent("Base prompt."),
			t.TempDir(),
		)

		networkSkill, err := skillbundled.LoadResource(bundledAghSkillName, bundledNetworkReference)
		if err != nil {
			t.Fatalf("LoadResource(%q, %q) error = %v", bundledAghSkillName, bundledNetworkReference, err)
		}
		if !strings.Contains(got, "# AGH Network Response Register") ||
			!strings.Contains(got, "Threads decide and discuss; actionable work is promoted to tasks") {
			t.Fatalf("AssembleStartup() = %q, want compact network response register", got)
		}
		if strings.Contains(got, strings.TrimSpace(networkSkill)) {
			t.Fatalf("AssembleStartup() loaded full network reference instead of compact register: %q", got)
		}
	})
}

func TestComposedAssemblerAssembleStartupLoadsBundledToolsSectionDescriptor(t *testing.T) {
	t.Parallel()

	t.Run("Should render tools guide only when tools section is selected", func(t *testing.T) {
		t.Parallel()

		resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
			ToolsPromptSectionEnabled: true,
		})
		assembler := NewComposedAssembler(
			WithSectionSelector(NewSectionSelector(resolver, nil)),
			WithPromptSectionDescriptors(defaultStartupPromptSectionDescriptors(nil, nil, nil)...),
		)

		got := assembleStartupPrompt(
			t,
			assembler,
			session.StartupPromptContext{
				SessionType: session.SessionTypeUser,
			},
			testPromptAgent("Base prompt."),
			t.TempDir(),
		)

		toolsGuide, err := skillbundled.LoadResource(bundledAghSkillName, bundledToolsReference)
		if err != nil {
			t.Fatalf("LoadResource(%q, %q) error = %v", bundledAghSkillName, bundledToolsReference, err)
		}
		toolsGuide = strings.TrimSpace(toolsGuide)
		nativeToolsGuide, err := skillbundled.LoadResource(bundledAghSkillName, bundledNativeToolsReference)
		if err != nil {
			t.Fatalf("LoadResource(%q, %q) error = %v", bundledAghSkillName, bundledNativeToolsReference, err)
		}
		nativeToolsGuide = strings.TrimSpace(nativeToolsGuide)
		networkSkill, err := skillbundled.LoadResource(bundledAghSkillName, bundledNetworkReference)
		if err != nil {
			t.Fatalf("LoadResource(%q, %q) error = %v", bundledAghSkillName, bundledNetworkReference, err)
		}
		networkSkill = strings.TrimSpace(networkSkill)
		if !strings.Contains(got, toolsGuide) {
			t.Fatalf("AssembleStartup() = %q, want bundled tools guide content", got)
		}
		if !strings.Contains(got, nativeToolsGuide) {
			t.Fatalf("AssembleStartup() = %q, want bundled native tools guide content", got)
		}
		if strings.Contains(got, networkSkill) {
			t.Fatalf("AssembleStartup() = %q, want no bundled network content without channel", got)
		}
	})
}

type recordingPromptProvider struct {
	section    string
	err        error
	workspaces []string
}

func (p *recordingPromptProvider) PromptSection(
	_ context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	if workspace != nil {
		p.workspaces = append(p.workspaces, workspace.RootDir)
	}
	if p.err != nil {
		return "", p.err
	}
	return p.section, nil
}

type errorPromptProvider struct {
	err error
}

func (p errorPromptProvider) PromptSection(context.Context, *workspacepkg.ResolvedWorkspace) (string, error) {
	return "", p.err
}

type staticPromptProvider string

func (p staticPromptProvider) PromptSection(context.Context, *workspacepkg.ResolvedWorkspace) (string, error) {
	return string(p), nil
}

type resumeContextPromptProvider struct {
	section string
	startup session.StartupPromptContext
}

func (p *resumeContextPromptProvider) PromptSection(
	context.Context,
	*workspacepkg.ResolvedWorkspace,
) (string, error) {
	return "", nil
}

func (p *resumeContextPromptProvider) ResumeContextSection(
	_ context.Context,
	startup session.StartupPromptContext,
) (string, error) {
	p.startup = startup
	return p.section, nil
}

type composedAssemblerMemoryEnv struct {
	store     *memory.Store
	globalDir string
	workspace string
	agent     aghconfig.AgentDef
}

func newComposedAssemblerMemoryEnv(t *testing.T) composedAssemblerMemoryEnv {
	t.Helper()

	baseDir := t.TempDir()
	workspace := filepath.Join(baseDir, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", workspace, err)
	}

	store := memory.NewStore(filepath.Join(baseDir, "home", "memory"))
	if err := store.EnsureDirs(); err != nil {
		t.Fatalf("Store.EnsureDirs() error = %v", err)
	}

	return composedAssemblerMemoryEnv{
		store:     store,
		globalDir: filepath.Join(baseDir, "home", "memory"),
		workspace: workspace,
		agent:     testPromptAgent("  You are a coding assistant.\n"),
	}
}

func (e composedAssemblerMemoryEnv) writeGlobalIndex(t *testing.T, content string) {
	t.Helper()
	writeComposedAssemblerFile(t, filepath.Join(e.globalDir, "MEMORY.md"), content)
}

func (e composedAssemblerMemoryEnv) writeWorkspaceIndex(t *testing.T, content string) {
	t.Helper()
	writeComposedAssemblerFile(t, filepath.Join(e.workspace, aghconfig.DirName, "memory", "MEMORY.md"), content)
}

func writeComposedAssemblerFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func assemblePrompt(t *testing.T, assembler *ComposedAssembler, agent aghconfig.AgentDef, workspace string) string {
	t.Helper()

	resolvedWorkspace := testResolvedWorkspace(workspace)
	got, err := assembler.Assemble(context.Background(), agent, &resolvedWorkspace)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	return got
}

func assembleStartupPrompt(
	t *testing.T,
	assembler *ComposedAssembler,
	startup session.StartupPromptContext,
	agent aghconfig.AgentDef,
	workspace string,
) string {
	t.Helper()

	resolvedWorkspace := testResolvedWorkspace(workspace)
	got, err := assembler.AssembleStartup(context.Background(), startup, agent, &resolvedWorkspace)
	if err != nil {
		t.Fatalf("AssembleStartup() error = %v", err)
	}
	return got
}

func testPromptAgent(prompt string) aghconfig.AgentDef {
	return aghconfig.AgentDef{
		Name:     "coder",
		Provider: "claude",
		Prompt:   prompt,
	}
}

func testResolvedWorkspace(root string) workspacepkg.ResolvedWorkspace {
	return workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{RootDir: root},
	}
}

func testPromptSoulSnapshot(t *testing.T, body string) *soul.Snapshot {
	t.Helper()

	cfg := aghconfig.DefaultSoulConfig()
	resolved, err := soul.Parse(context.Background(), soul.ParseRequest{
		SourcePath:    "/workspace/.agh/agents/coder/SOUL.md",
		WorkspaceRoot: "/workspace",
		Content: []byte(strings.Join([]string{
			"---",
			"role: Reviewer",
			"tone:",
			"  - direct",
			"principles:",
			"  - protect correctness",
			"---",
			body,
		}, "\n")),
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("soul.Parse() error = %v", err)
	}
	provenance, err := soul.NewConfigProvenance(cfg, "test")
	if err != nil {
		t.Fatalf("NewConfigProvenance() error = %v", err)
	}
	snapshot, err := soul.SnapshotFromResolved(
		"soul-test",
		"ws-1",
		"coder",
		&resolved,
		provenance,
		time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("SnapshotFromResolved() error = %v", err)
	}
	return &snapshot
}

func assertPromptOrder(t *testing.T, value string, tokens []string) {
	t.Helper()

	last := -1
	for _, token := range tokens {
		index := strings.Index(value, token)
		if index < 0 {
			t.Fatalf("missing token %q in %s", token, value)
		}
		if index <= last {
			t.Fatalf("token %q appeared out of order in %s", token, value)
		}
		last = index
	}
}
