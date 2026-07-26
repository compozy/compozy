package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	extractorpkg "github.com/compozy/agh/internal/memory/extractor"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestDaemonMemoryExtractorSkipsSubagentWorkspaceRootCache(t *testing.T) {
	t.Parallel()

	workspaceRoots := &sync.Map{}
	extractor := &daemonMemoryExtractor{
		runtime:        &extractorpkg.Runtime{},
		workspaceRoots: workspaceRoots,
	}
	payload := hookspkg.SessionMessagePersistedPayload{
		SessionContext: hookspkg.SessionContext{
			SessionID: "sess-auto-title", Workspace: t.TempDir(), WorkspaceID: "ws-1",
		},
		ParentSessionID: "sess-parent",
		ActorKind:       "agent_subagent",
	}
	if err := extractor.HandleSessionMessagePersisted(testutil.Context(t), payload); err != nil {
		t.Fatalf("HandleSessionMessagePersisted() error = %v", err)
	}
	if _, loaded := workspaceRoots.Load(payload.SessionID); loaded {
		t.Fatalf("workspaceRoots[%q] retained a subagent entry", payload.SessionID)
	}
}

func TestDaemonMemoryProposalSinkTargetStore(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize workspace-root candidates to the stable workspace identity", func(t *testing.T) {
		t.Parallel()

		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("os.MkdirAll() error = %v", err)
		}
		identity, err := workspacepkg.EnsureIdentity(testutil.Context(t), workspaceRoot)
		if err != nil {
			t.Fatalf("EnsureIdentity() error = %v", err)
		}
		memoryStore := memory.NewStore(
			filepath.Join(baseDir, "global", "memory"),
			memory.WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
		)
		openDaemonMemoryCatalog(t, memoryStore)
		sink := daemonMemoryProposalSink{base: memoryStore}
		candidate := memcontract.Candidate{
			WorkspaceID: "ws-registration",
			Scope:       memcontract.ScopeWorkspace,
			Frontmatter: memcontract.Header{
				Scope: memcontract.ScopeWorkspace,
				Type:  memcontract.TypeProject,
			},
			Metadata: map[string]string{
				"workspace_root": workspaceRoot,
			},
		}

		_, normalized, err := sink.targetStore(testutil.Context(t), candidate)
		if err != nil {
			t.Fatalf("targetStore() error = %v", err)
		}
		if normalized.WorkspaceID != identity.WorkspaceID {
			t.Fatalf(
				"candidate workspace id = %q, want stable identity %q",
				normalized.WorkspaceID,
				identity.WorkspaceID,
			)
		}
	})
}

func TestCollectMemoryExtractorOutput(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve streamed JSONL chunks without synthetic newlines", func(t *testing.T) {
		t.Parallel()

		events := make(chan acp.AgentEvent, 4)
		events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "```jsonl\n{\"type\":\"reference\",\"scope"}
		events <- acp.AgentEvent{
			Type: acp.EventTypeAgentMessage,
			Text: "\":\"workspace\",\"agent_tier\":\"workspace\",\"content\":\"Channel marketing already exists.\",\"evidence\":\"seq=52\"",
		}
		events <- acp.AgentEvent{
			Type: acp.EventTypeAgentMessage,
			Text: ",\"entity\":\"ws-test\",\"attribute\":\"network_channel_marketing_exists\"}\n```",
		}
		close(events)

		output, err := collectMemoryExtractorOutput(testutil.Context(t), events)
		if err != nil {
			t.Fatalf("collectMemoryExtractorOutput() error = %v", err)
		}
		if strings.Contains(output, "scope\n") {
			t.Fatalf("output = %q, want no synthetic newline inside streamed JSON key", output)
		}

		turn := memcontract.TurnRecord{
			SessionID:       "sess-parent",
			RootSessionID:   "sess-parent",
			AgentID:         "cto",
			ActorKind:       "agent_root",
			WorkspaceID:     "ws-test",
			SinceMessageSeq: 32,
			UntilMessageSeq: 52,
			Snapshot: memcontract.TranscriptSnapshot{
				Messages: []memcontract.TranscriptMessage{{
					Sequence: 52,
					Role:     "assistant",
					Content:  "Channel marketing exists.",
					At:       time.Date(2026, 5, 26, 21, 1, 52, 0, time.UTC),
				}},
			},
			Trigger: memcontract.TriggerPostMessage,
		}
		candidates, err := parseMemoryExtractorCandidates(
			output,
			turn,
			"/workspace/test",
			time.Date(2026, 5, 26, 21, 2, 0, 0, time.UTC),
		)
		if err != nil {
			t.Fatalf("parseMemoryExtractorCandidates() error = %v", err)
		}
		if len(candidates) != 1 {
			t.Fatalf("candidates = %#v, want one parsed candidate", candidates)
		}
		candidate := candidates[0]
		if candidate.Scope != memcontract.ScopeWorkspace || candidate.Frontmatter.Type != memcontract.TypeReference {
			t.Fatalf(
				"candidate scope/type = %s/%s, want workspace/reference",
				candidate.Scope,
				candidate.Frontmatter.Type,
			)
		}
		if !strings.Contains(candidate.Content, "Channel marketing already exists") {
			t.Fatalf("candidate content = %q, want streamed JSON content", candidate.Content)
		}
		if candidate.Metadata["workspace_root"] != "/workspace/test" {
			t.Fatalf("candidate metadata = %#v, want workspace_root", candidate.Metadata)
		}
		if candidate.AgentTier != "" || candidate.Frontmatter.AgentTier != "" {
			t.Fatalf("candidate agent tiers = %q/%q, want empty outside agent scope",
				candidate.AgentTier,
				candidate.Frontmatter.AgentTier,
			)
		}
	})

	t.Run("Should discard agent tier outside agent-scoped candidates", func(t *testing.T) {
		t.Parallel()

		turn := memcontract.TurnRecord{
			SessionID:   "sess-parent",
			AgentID:     "cto",
			WorkspaceID: "ws-test",
		}
		candidates, err := parseMemoryExtractorCandidates(
			`{"type":"reference","scope":"global","agent_tier":"global","content":"Release artifacts use checksums.","evidence":"seq=7"}`,
			turn,
			"/workspace/test",
			time.Date(2026, 7, 24, 11, 25, 0, 0, time.UTC),
		)
		if err != nil {
			t.Fatalf("parseMemoryExtractorCandidates() error = %v", err)
		}
		if len(candidates) != 1 {
			t.Fatalf("candidates = %#v, want one parsed candidate", candidates)
		}
		if candidates[0].AgentTier != "" || candidates[0].Frontmatter.AgentTier != "" {
			t.Fatalf("candidate agent tiers = %q/%q, want empty outside agent scope",
				candidates[0].AgentTier,
				candidates[0].Frontmatter.AgentTier,
			)
		}
	})
}

func TestDaemonMemoryProviderService(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve missing provider sentinels for settings validation", func(t *testing.T) {
		t.Parallel()

		registry := extensionpkg.NewMemoryProviderRegistry()
		service := daemonMemoryProviderService{registry: registry}

		_, err := service.Get(testutil.Context(t), "", "missing-provider")
		if err == nil {
			t.Fatal("Get(missing-provider) error = nil, want not found")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Get(missing-provider) error = %v, want os.ErrNotExist", err)
		}
		if !errors.Is(err, extensionpkg.ErrMemoryProviderNotFound) {
			t.Fatalf("Get(missing-provider) error = %v, want ErrMemoryProviderNotFound", err)
		}
	})
}

func TestForkedMemoryExtractor(t *testing.T) {
	t.Parallel()

	t.Run("Should skip the child session when the role is disabled", func(t *testing.T) {
		t.Parallel()

		sessions := &recordingMemoryExtractorSessions{}
		extractor := &forkedMemoryExtractor{
			sessions: sessions,
			roles:    resolvedRoleResolver(ResolvedRole{Enabled: false}),
		}
		candidates, err := extractor.Extract(testutil.Context(t), memcontract.TurnRecord{WorkspaceID: "ws-test"})
		if err != nil {
			t.Fatalf("Extract() error = %v", err)
		}
		if candidates != nil || sessions.spawnCalls != 0 {
			t.Fatalf("Extract() = %#v with %d spawn calls, want skipped", candidates, sessions.spawnCalls)
		}
	})

	t.Run("Should pass the configured model to the extractor child spawn", func(t *testing.T) {
		t.Parallel()

		sessions := &recordingMemoryExtractorSessions{}
		extractor := &forkedMemoryExtractor{
			sessions: sessions,
			roles: resolvedRoleResolver(ResolvedRole{
				Role:    aghconfig.RoleMemoryExtractor,
				Enabled: true,
				Inherit: true,
				Model:   "claude-haiku-memory",
			}),
			deadline: time.Second,
			now: func() time.Time {
				return time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
			},
		}
		turn := memcontract.TurnRecord{
			SessionID:       "sess-parent",
			RootSessionID:   "sess-parent",
			AgentID:         "memory-agent",
			ActorKind:       "agent_root",
			WorkspaceID:     "ws-test",
			SinceMessageSeq: 1,
			UntilMessageSeq: 1,
			Snapshot: memcontract.TranscriptSnapshot{
				Messages: []memcontract.TranscriptMessage{{
					Sequence: 1,
					Role:     "assistant",
					Content:  "Pedro prefers concise updates.",
					At:       time.Date(2026, 5, 27, 9, 59, 0, 0, time.UTC),
				}},
			},
			Trigger: memcontract.TriggerPostMessage,
		}

		candidates, err := extractor.Extract(testutil.Context(t), turn)
		if err != nil {
			t.Fatalf("Extract() error = %v", err)
		}
		if len(candidates) != 0 {
			t.Fatalf("candidates = %#v, want none for empty child output", candidates)
		}
		if sessions.spawnOpts.AgentName != "memory-agent" {
			t.Fatalf("spawn agent = %q, want default memory agent", sessions.spawnOpts.AgentName)
		}
		if sessions.spawnOpts.Model != "claude-haiku-memory" {
			t.Fatalf("spawn model = %q, want configured extractor model", sessions.spawnOpts.Model)
		}
		if sessions.stoppedID != "sess-memory-child" {
			t.Fatalf("stopped child = %q, want spawned extractor child stopped", sessions.stoppedID)
		}
	})
}

type recordingMemoryExtractorSessions struct {
	spawnCalls int
	spawnOpts  session.SpawnOpts
	stoppedID  string
}

func (s *recordingMemoryExtractorSessions) Spawn(
	_ context.Context,
	opts session.SpawnOpts,
) (*session.Session, error) {
	s.spawnCalls++
	s.spawnOpts = opts
	return &session.Session{ID: "sess-memory-child"}, nil
}

func (s *recordingMemoryExtractorSessions) PromptSynthetic(
	_ context.Context,
	_ string,
	_ session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	events := make(chan acp.AgentEvent)
	close(events)
	return events, nil
}

func (s *recordingMemoryExtractorSessions) StopWithCause(
	_ context.Context,
	id string,
	_ session.StopCause,
	_ string,
) error {
	s.stoppedID = id
	return nil
}
