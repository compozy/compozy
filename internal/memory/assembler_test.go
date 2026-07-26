package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestAssemblerAssemble(t *testing.T) {
	t.Parallel()

	t.Run("Should global index only", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Global](global.md) - global note")

		got := env.assemble(t)
		if !strings.Contains(got, "## Global MEMORY.md Index") {
			t.Fatalf("assembled prompt missing global section: %q", got)
		}
		if strings.Contains(got, "## Workspace MEMORY.md Index") {
			t.Fatalf("assembled prompt unexpectedly included workspace section: %q", got)
		}
	})

	t.Run("Should workspace index only", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeWorkspaceIndex(t, "- [Workspace](workspace.md) - workspace note")

		got := env.assemble(t)
		if !strings.Contains(got, "## Workspace MEMORY.md Index") {
			t.Fatalf("assembled prompt missing workspace section: %q", got)
		}
		if strings.Contains(got, "## Global MEMORY.md Index") {
			t.Fatalf("assembled prompt unexpectedly included global section: %q", got)
		}
	})

	t.Run("Should both indexes", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Global](global.md) - global note")
		env.writeWorkspaceIndex(t, "- [Workspace](workspace.md) - workspace note")

		got := env.assemble(t)
		if !strings.Contains(got, "## Global MEMORY.md Index") ||
			!strings.Contains(got, "## Workspace MEMORY.md Index") {
			t.Fatalf("assembled prompt missing expected sections: %q", got)
		}
	})

	t.Run("Should empty indexes returns original prompt", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		got := env.assemble(t)
		if got != env.agent.Prompt {
			t.Fatalf("assembled prompt = %q, want original prompt %q", got, env.agent.Prompt)
		}
	})

	t.Run("Should includes taxonomy instructions", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Global](global.md) - global note")

		got := env.assemble(t)
		for _, want := range []string{"## Memory Taxonomy", "`user`", "`feedback`", "`project`", "`reference`"} {
			if !strings.Contains(got, want) {
				t.Fatalf("assembled prompt missing taxonomy content %q: %q", want, got)
			}
		}
	})

	t.Run("Should includes agh memory command reference", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Global](global.md) - global note")

		got := env.assemble(t)
		for _, want := range []string{
			"## Memory Commands",
			"`agh memory list`",
			"`agh memory search <query>`",
			"`agh memory show <filename>`",
			"`agh memory reindex`",
			"`agh memory write --name <name> --type <type> --description <desc> --content <content>`",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("assembled prompt missing command reference %q: %q", want, got)
			}
		}
	})

	t.Run("Should includes staleness policy", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Global](global.md) - global note")

		got := env.assemble(t)
		if !strings.Contains(got, "## Staleness Policy") ||
			!strings.Contains(got, "Memories older than 1 day should be verified") {
			t.Fatalf("assembled prompt missing staleness policy: %q", got)
		}
	})

	t.Run("Should memory context before agent prompt", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Global](global.md) - global note")

		got := env.assemble(t)
		memoryIndex := strings.Index(got, "# Persistent Memory")
		agentIndex := strings.Index(got, env.agent.Prompt)
		if memoryIndex < 0 || agentIndex < 0 {
			t.Fatalf("assembled prompt missing expected components: %q", got)
		}
		if memoryIndex >= agentIndex {
			t.Fatalf(
				"memory context index = %d, agent prompt index = %d, want memory before agent prompt",
				memoryIndex,
				agentIndex,
			)
		}
	})
}

func TestAssemblerPromptSection(t *testing.T) {
	t.Parallel()

	t.Run("Should returns memory block for global and workspace indexes only", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Global](global.md) - global note")
		env.writeWorkspaceIndex(t, "- [Workspace](workspace.md) - workspace note")

		got := env.promptSection(context.Background(), t)
		for _, want := range []string{
			memoryPromptIntro,
			"AGH memory snapshot v1 blocks=2 hash=",
			"## Global MEMORY.md Index\n\n- [Global](global.md) - global note",
			"## Workspace MEMORY.md Index\n\n- [Workspace](workspace.md) - workspace note",
			memoryTaxonomySection,
			memoryCommandsSection,
			memoryStalenessSection,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("PromptSection() missing %q:\n%s", want, got)
			}
		}
		if strings.Contains(got, strings.TrimSpace(env.agent.Prompt)) {
			t.Fatalf("PromptSection() unexpectedly included base prompt: %q", got)
		}
	})

	t.Run("Should returns empty string when indexes are missing", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)

		got := env.promptSection(context.Background(), t)
		if got != "" {
			t.Fatalf("PromptSection() = %q, want empty string", got)
		}
	})

	t.Run("Should respects context cancellation", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Global](global.md) - global note")

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := env.assembler.PromptSection(ctx, resolvedWorkspacePtr(env.workspace))
		if err != context.Canceled {
			t.Fatalf("PromptSection() error = %v, want %v", err, context.Canceled)
		}
	})
}

func TestAssemblerCheckpointSummaryIsolation(t *testing.T) {
	t.Parallel()

	t.Run("Should never inject a workspace checkpoint into another workspace", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		workspaceA := env.workspace
		workspaceB := filepath.Join(filepath.Dir(workspaceA), "workspace-b")
		if err := os.MkdirAll(workspaceB, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspace-b) error = %v", err)
		}
		document, err := renderCheckpointSummaryDocument(
			checkpointSummaryFixture("Workspace A retains the cobalt decision."),
			checkpointSummaryHeader(time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC), nil),
		)
		if err != nil {
			t.Fatalf("renderCheckpointSummaryDocument() error = %v", err)
		}
		if err := env.store.ForWorkspace(workspaceA).Write(
			memcontract.ScopeWorkspace,
			CheckpointSummaryFilename,
			document,
		); err != nil {
			t.Fatalf("Write(workspace A checkpoint) error = %v", err)
		}

		sectionA, err := env.assembler.PromptSection(context.Background(), resolvedWorkspacePtr(workspaceA))
		if err != nil {
			t.Fatalf("PromptSection(workspace A) error = %v", err)
		}
		sectionB, err := env.assembler.PromptSection(context.Background(), resolvedWorkspacePtr(workspaceB))
		if err != nil {
			t.Fatalf("PromptSection(workspace B) error = %v", err)
		}
		if !strings.Contains(sectionA, "<agh_checkpoint_summary>") ||
			!strings.Contains(sectionA, "cobalt decision") {
			t.Fatalf("workspace A section missing checkpoint summary:\n%s", sectionA)
		}
		if strings.Contains(sectionB, "cobalt decision") || strings.Contains(sectionB, "<agh_checkpoint_summary>") {
			t.Fatalf("workspace B section leaked workspace A checkpoint:\n%s", sectionB)
		}
	})
}

func TestAssemblerAssembleRegressionMatchesPromptSectionAndBasePrompt(t *testing.T) {
	t.Parallel()

	env := newAssemblerTestEnv(t)
	env.agent.Prompt = "  You are a coding assistant.\n"
	env.writeGlobalIndex(t, "- [Global](global.md) - global note")
	env.writeWorkspaceIndex(t, "- [Workspace](workspace.md) - workspace note")

	section := env.promptSection(context.Background(), t)
	got := env.assemble(t)
	want := section + "\n\n" + strings.TrimSpace(env.agent.Prompt)

	if got != want {
		t.Fatalf("Assemble() regression mismatch\nwant:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestSnapshotServiceCapture(t *testing.T) {
	t.Parallel()

	t.Run("Should freeze startup memory and recapture only at next boot boundary", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Original](global.md) - old note")
		service := NewSnapshotService(env.store, WithSnapshotClock(fixedSnapshotNow))

		first, err := service.Capture(context.Background(), PromptSnapshotRequest{SessionID: "sess-1"})
		if err != nil {
			t.Fatalf("Capture(first) error = %v", err)
		}
		env.writeGlobalIndex(t, "- [Updated](global.md) - new note")
		generation := service.InvalidateNextBoot()

		if !strings.Contains(first.Section, "old note") || strings.Contains(first.Section, "new note") {
			t.Fatalf("first snapshot mutated after write/reload: %s", first.Section)
		}
		second, err := service.Capture(context.Background(), PromptSnapshotRequest{SessionID: "sess-2"})
		if err != nil {
			t.Fatalf("Capture(second) error = %v", err)
		}
		if second.Generation != generation {
			t.Fatalf("second generation = %d, want %d", second.Generation, generation)
		}
		if !strings.Contains(second.Section, "new note") {
			t.Fatalf("second snapshot missing recaptured memory: %s", second.Section)
		}
	})

	t.Run("Should keep one session prefix byte stable until a committed memory write", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Original](global.md) - stable prefix note")
		service := NewSnapshotService(env.store, WithSnapshotClock(fixedSnapshotNow))
		request := PromptSnapshotRequest{SessionID: "sess-prefix-stability"}

		hashes := make([]string, 0, 6)
		var initialGeneration uint64
		for turn := range 3 {
			snapshot, err := service.Capture(context.Background(), request)
			if err != nil {
				t.Fatalf("Capture(before write, turn %d) error = %v", turn+1, err)
			}
			if turn == 0 {
				initialGeneration = snapshot.Generation
			}
			hashes = append(hashes, hashText(snapshot.Section))
		}

		if err := env.store.Write(
			memcontract.ScopeGlobal,
			"user_prefix_change.md",
			mustMemoryContent(t, testMemoryMeta{
				Name:        "Prefix Change",
				Description: "Committed memory revision",
				Type:        memcontract.TypeUser,
			}, "Remember the committed prefix transition.\n"),
		); err != nil {
			t.Fatalf("Store.Write(prefix mutation) error = %v", err)
		}

		for turn := range 3 {
			snapshot, err := service.Capture(context.Background(), request)
			if err != nil {
				t.Fatalf("Capture(after write, turn %d) error = %v", turn+1, err)
			}
			if snapshot.Generation != initialGeneration+1 {
				t.Fatalf(
					"Capture(after write, turn %d).Generation = %d, want %d",
					turn+1,
					snapshot.Generation,
					initialGeneration+1,
				)
			}
			hashes = append(hashes, hashText(snapshot.Section))
		}

		transitions := 0
		for index := 1; index < len(hashes); index++ {
			if hashes[index] != hashes[index-1] {
				transitions++
			}
		}
		if transitions != 1 {
			t.Fatalf("prefix hashes = %#v, want exactly one transition", hashes)
		}
	})

	t.Run("Should compose scope blocks least specific first", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Global](global.md) - global note")
		env.writeWorkspaceIndex(t, "- [Workspace](workspace.md) - workspace note")
		env.writeAgentIndex(t, memcontract.AgentTierGlobal, "- [Agent Global](feedback_agent_global.md) - agent global")
		env.writeAgentIndex(
			t,
			memcontract.AgentTierWorkspace,
			"- [Agent Workspace](feedback_agent_ws.md) - agent workspace",
		)

		snapshot, err := env.assembler.snapshots.Capture(context.Background(), PromptSnapshotRequest{
			WorkspaceID:   "ws-alpha",
			WorkspaceRoot: env.workspace,
			AgentName:     "reviewer",
			SessionType:   session.SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Capture() error = %v", err)
		}

		wantOrder := []string{
			"## Global MEMORY.md Index",
			"## Workspace MEMORY.md Index",
			"## Agent Global MEMORY.md Index",
			"## Agent Workspace MEMORY.md Index",
		}
		assertSnapshotOrder(t, snapshot.Section, wantOrder)
		if snapshot.ControllerMode != SnapshotControllerWritable {
			t.Fatalf("ControllerMode = %q, want writable", snapshot.ControllerMode)
		}
	})

	t.Run("Should enforce prompt caps and preserve freshness warnings", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Very Old](global.md) - "+strings.Repeat("memory ", 100))
		oldTime := fixedSnapshotNow().Add(-72 * time.Hour)
		globalFile := filepath.Join(env.store.globalDir, "global.md")
		if err := os.Chtimes(globalFile, oldTime, oldTime); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", globalFile, err)
		}
		service := NewSnapshotService(
			env.store,
			WithSnapshotClock(fixedSnapshotNow),
			WithSnapshotMaxCharacters(800),
		)

		snapshot, err := service.Capture(context.Background(), PromptSnapshotRequest{SessionID: "sess-1"})
		if err != nil {
			t.Fatalf("Capture() error = %v", err)
		}
		if !strings.Contains(snapshot.Section, "This memory index is") {
			t.Fatalf("snapshot missing freshness warning: %s", snapshot.Section)
		}
		if strings.Contains(snapshot.Section, strings.Repeat("memory ", 80)) {
			t.Fatalf("snapshot cap did not trim long content: %s", snapshot.Section)
		}
		if !strings.Contains(snapshot.Section, "Index truncated to fit prompt limits.") {
			t.Fatalf("snapshot missing truncation marker: %s", snapshot.Section)
		}
	})

	t.Run("Should let subagents inherit parent snapshots as read only", func(t *testing.T) {
		t.Parallel()

		env := newAssemblerTestEnv(t)
		env.writeGlobalIndex(t, "- [Parent](global.md) - parent note")
		service := NewSnapshotService(env.store, WithSnapshotClock(fixedSnapshotNow))
		parent, err := service.Capture(context.Background(), PromptSnapshotRequest{
			SessionID:   "parent",
			AgentName:   "reviewer",
			SessionType: session.SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Capture(parent) error = %v", err)
		}
		env.writeAgentIndex(t, memcontract.AgentTierWorkspace, "- [Child Private](feedback_child.md) - child private")

		child, err := service.Capture(context.Background(), PromptSnapshotRequest{
			SessionID:      "child",
			WorkspaceID:    "ws-alpha",
			WorkspaceRoot:  env.workspace,
			AgentName:      "worker",
			SessionType:    session.SessionTypeSpawned,
			ParentSnapshot: &parent,
		})
		if err != nil {
			t.Fatalf("Capture(child) error = %v", err)
		}
		if child.ControllerMode != SnapshotControllerReadOnly {
			t.Fatalf("child ControllerMode = %q, want read_only", child.ControllerMode)
		}
		if child.InheritedFrom != parent.ID {
			t.Fatalf("child inherited_from = %q, want %q", child.InheritedFrom, parent.ID)
		}
		if child.Section != parent.Section {
			t.Fatalf(
				"child section rebuilt instead of inherited\nparent:\n%s\nchild:\n%s",
				parent.Section,
				child.Section,
			)
		}
		if strings.Contains(child.Section, "child private") {
			t.Fatalf("child inherited snapshot included private sub-agent memory: %s", child.Section)
		}
	})

	t.Run("Should render provider snapshot blocks through the same deterministic path", func(t *testing.T) {
		t.Parallel()

		provider := &snapshotProviderStub{
			results: map[string]memcontract.SnapshotResult{
				"global/": {
					Markdown: "- [Provider Global](global.md) - provider global",
					AgeMs:    int64((48 * time.Hour) / time.Millisecond),
				},
				"workspace/": {Markdown: "- [Provider Workspace](workspace.md) - provider workspace"},
				"agent/global": {
					Markdown: "- [Provider Agent Global](agent_global.md) - provider agent global",
				},
				"agent/workspace": {
					Markdown: "- [Provider Agent Workspace](agent_workspace.md) - provider agent workspace",
				},
			},
		}
		service := NewSnapshotService(nil, WithProviderSnapshotSource(provider), WithSnapshotClock(fixedSnapshotNow))

		snapshot, err := service.Capture(context.Background(), PromptSnapshotRequest{
			SessionID:     "sess-provider",
			WorkspaceID:   "ws-alpha",
			WorkspaceRoot: "/work/agh",
			AgentName:     "reviewer",
			SessionType:   session.SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Capture(provider) error = %v", err)
		}
		if len(provider.requests) != 4 {
			t.Fatalf("provider requests = %d, want 4", len(provider.requests))
		}
		for _, want := range []string{"provider global", "provider workspace", "provider agent global", "provider agent workspace"} {
			if !strings.Contains(snapshot.Section, want) {
				t.Fatalf("provider snapshot missing %q: %s", want, snapshot.Section)
			}
		}
	})
}

type assemblerTestEnv struct {
	store     *Store
	assembler *Assembler
	workspace string
	agent     aghconfig.AgentDef
}

func newAssemblerTestEnv(t *testing.T) assemblerTestEnv {
	t.Helper()

	baseDir := t.TempDir()
	workspace := filepath.Join(baseDir, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}

	store := newOpenTestStore(t, filepath.Join(baseDir, "home", "memory"))
	if err := store.EnsureDirs(); err != nil {
		t.Fatalf("Store.EnsureDirs() error = %v", err)
	}

	return assemblerTestEnv{
		store:     store,
		assembler: NewAssembler(store),
		workspace: workspace,
		agent: aghconfig.AgentDef{
			Name:     "coder",
			Provider: "claude",
			Prompt:   "You are a coding assistant.",
		},
	}
}

func (e assemblerTestEnv) assemble(t *testing.T) string {
	t.Helper()

	workspace := testResolvedWorkspace(e.workspace)
	got, err := e.assembler.Assemble(context.Background(), e.agent, &workspace)
	if err != nil {
		t.Fatalf("Assembler.Assemble() error = %v", err)
	}
	return got
}

func (e assemblerTestEnv) promptSection(ctx context.Context, t *testing.T) string {
	t.Helper()

	got, err := e.assembler.PromptSection(ctx, resolvedWorkspacePtr(e.workspace))
	if err != nil {
		t.Fatalf("Assembler.PromptSection() error = %v", err)
	}
	return got
}

func resolvedWorkspacePtr(root string) *workspacepkg.ResolvedWorkspace {
	workspace := testResolvedWorkspace(root)
	return &workspace
}

func (e assemblerTestEnv) writeGlobalIndex(t *testing.T, content string) {
	t.Helper()
	writeAssemblerFileForTest(t, filepath.Join(e.store.globalDir, indexFilename), content)
	e.writeIndexBackedDocuments(t, memcontract.ScopeGlobal, "", content)
}

func (e assemblerTestEnv) writeWorkspaceIndex(t *testing.T, content string) {
	t.Helper()
	writeAssemblerFileForTest(t, filepath.Join(e.store.ForWorkspace(e.workspace).workspaceDir, indexFilename), content)
	e.writeIndexBackedDocuments(t, memcontract.ScopeWorkspace, e.workspace, content)
}

func (e assemblerTestEnv) writeAgentIndex(
	t *testing.T,
	tier memcontract.AgentTier,
	content string,
) {
	t.Helper()

	target := e.store.ForWorkspace(e.workspace).ForAgent("ws-alpha", "reviewer", tier)
	if err := target.EnsureDirs(); err != nil {
		t.Fatalf("agent Store.EnsureDirs() error = %v", err)
	}
	dir := target.dirForScopeMust(t, memcontract.ScopeAgent)
	writeAssemblerFileForTest(t, filepath.Join(dir, indexFilename), content)
	writeIndexBackedDocumentsForStore(t, target, memcontract.ScopeAgent, content)
}

func writeAssemblerFileForTest(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func (e assemblerTestEnv) writeIndexBackedDocuments(
	t *testing.T,
	scope memcontract.Scope,
	workspace string,
	content string,
) {
	t.Helper()

	target := e.store
	if scope == memcontract.ScopeWorkspace {
		target = e.store.ForWorkspace(workspace)
	}

	writeIndexBackedDocumentsForStore(t, target, scope, content)
}

func writeIndexBackedDocumentsForStore(
	t *testing.T,
	target *Store,
	scope memcontract.Scope,
	content string,
) {
	t.Helper()

	for line := range strings.SplitSeq(content, "\n") {
		filename, ok := firstMarkdownLinkTarget(line)
		if !ok {
			continue
		}
		name := "Stub Memory"
		if start := strings.Index(line, "["); start >= 0 {
			if end := strings.Index(line[start+1:], "]"); end >= 0 {
				name = strings.TrimSpace(line[start+1 : start+1+end])
			}
		}
		description := "stub description"
		if _, after, ok0 := strings.Cut(line, " - "); ok0 {
			description = strings.TrimSpace(after)
		}
		doc := strings.Join([]string{
			"---",
			"name: " + name,
			"description: " + description,
			"type: feedback",
			"scope: " + string(scope.Normalize()),
		}, "\n")
		if scope.Normalize() == memcontract.ScopeAgent {
			doc += "\nagent: " + target.agentName
			doc += "\nagent_tier: " + string(target.agentTier.Normalize())
		}
		doc += "\n---\n\nstub body\n"
		if err := os.WriteFile(
			filepath.Join(target.dirForScopeMust(t, scope), filename),
			[]byte(doc),
			0o644,
		); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", filename, err)
		}
	}
}

func (s *Store) dirForScopeMust(t *testing.T, scope memcontract.Scope) string {
	t.Helper()
	dir, err := s.dirForScope(scope)
	if err != nil {
		t.Fatalf("dirForScope(%q) error = %v", scope, err)
	}
	return dir
}

func testResolvedWorkspace(root string) workspacepkg.ResolvedWorkspace {
	return workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "ws-alpha", RootDir: root},
	}
}

func fixedSnapshotNow() time.Time {
	return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
}

func assertSnapshotOrder(t *testing.T, rendered string, fragments []string) {
	t.Helper()

	last := -1
	for _, fragment := range fragments {
		index := strings.Index(rendered, fragment)
		if index < 0 {
			t.Fatalf("rendered snapshot missing %q:\n%s", fragment, rendered)
		}
		if index <= last {
			t.Fatalf("rendered snapshot order invalid for %q:\n%s", fragment, rendered)
		}
		last = index
	}
}

type snapshotProviderStub struct {
	results  map[string]memcontract.SnapshotResult
	requests []memcontract.SnapshotRequest
}

func (s *snapshotProviderStub) SystemPromptBlock(
	_ context.Context,
	req memcontract.SnapshotRequest,
) (memcontract.SnapshotResult, error) {
	s.requests = append(s.requests, req)
	result, ok := s.results[snapshotProviderKey(req)]
	if !ok {
		return memcontract.SnapshotResult{}, nil
	}
	return result, nil
}

func snapshotProviderKey(req memcontract.SnapshotRequest) string {
	return string(req.Scope.Normalize()) + "/" + string(req.AgentTier.Normalize())
}
