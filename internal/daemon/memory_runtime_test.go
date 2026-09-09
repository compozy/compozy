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

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/memory"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
	extractorpkg "github.com/compozy/compozy/internal/memory/extractor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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

	t.Run("Should route profile candidates without falling back on missing owners", func(t *testing.T) {
		t.Parallel()
		base := memory.NewStore(t.TempDir())
		scoped := base.ForProfile("profile-engineering", t.TempDir())
		sink := daemonMemoryProposalSink{
			base: base,
			profileStores: func(_ context.Context, id string) (*memory.Store, error) {
				if id != "profile-engineering" {
					return nil, errors.New("unknown profile")
				}
				return scoped, nil
			},
		}
		target, _, err := sink.targetStore(
			t.Context(),
			memcontract.Candidate{ProfileID: "profile-engineering", Scope: memcontract.ScopeProfile},
		)
		if err != nil {
			t.Fatal(err)
		}
		if target != scoped {
			t.Fatal("candidate selected another Profile store")
		}
		if _, _, err := sink.targetStore(
			t.Context(),
			memcontract.Candidate{ProfileID: "missing", Scope: memcontract.ScopeProfile},
		); err == nil || !strings.Contains(err.Error(), "unknown profile") {
			t.Fatalf("error = %v, want unknown profile", err)
		}
	})

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
			filepath.Join(baseDir, "profiles", store.DefaultProfileID, "memory"),
			memory.WithCatalogDatabasePath(filepath.Join(baseDir, "compozy.db")),
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
		events <- acp.AgentEvent{Type: acp.EventTypeDone, PromptStopReason: acp.PromptStopReasonEndTurn}
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

	for _, tc := range []struct {
		name     string
		terminal acp.AgentEvent
	}{
		{name: "Should retain output on provider error", terminal: acp.AgentEvent{Type: acp.EventTypeError, Error: "disconnected"}},
		{name: "Should reject a canceled terminal", terminal: acp.AgentEvent{Type: acp.EventTypeDone, StopReason: string(acp.PromptStopReasonCancelled)}},
		{name: "Should reject a truncated terminal", terminal: acp.AgentEvent{Type: acp.EventTypeDone, StopReason: "max_tokens"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			events := make(chan acp.AgentEvent, 2)
			events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "partial output"}
			events <- tc.terminal
			close(events)
			output, err := collectMemoryExtractorOutput(t.Context(), events)
			if tc.terminal.StopReason == string(acp.PromptStopReasonCancelled) &&
				(err == nil || !strings.Contains(err.Error(), string(acp.PromptStopReasonCancelled))) {
				t.Fatalf("error = %v, want cancellation reason", err)
			}
			if err == nil || output != "partial output" {
				t.Fatalf("collected=%q error=%v, want diagnostic output and failure", output, err)
			}
		})
	}

	t.Run("Should discard agent tier outside agent-scoped candidates", func(t *testing.T) {
		t.Parallel()

		turn := memcontract.TurnRecord{
			SessionID:   "sess-parent",
			AgentID:     "cto",
			WorkspaceID: "ws-test",
		}
		candidates, err := parseMemoryExtractorCandidates(
			`{"type":"reference","scope":"profile","agent_tier":"global","content":"Release artifacts use checksums.","evidence":"seq=7"}`,
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

func TestMemoryExtractorOutputContract(t *testing.T) {
	t.Parallel()
	candidate := `{"type":"user","content":"Pedro prefers concise updates."}`
	for _, tc := range []struct {
		name, output string
		count        int
		failure      bool
	}{
		{name: "Should accept explicit no candidates", output: `{"no_candidates":true}`},
		{name: "Should accept an empty array", output: `[]`},
		{name: "Should accept conventional no memory prose", output: "No memories to save."},
		{name: "Should accept parenthesized none", output: "(none)"},
		{name: "Should accept fenced candidates", output: "```jsonl\n" + candidate + "\n```", count: 1},
		{name: "Should preserve candidates around invalid JSON", output: candidate + "\n{broken\n" + candidate, count: 2, failure: true},
		{name: "Should preserve candidates around unknown prose", output: "Here are the candidates:\n" + candidate, count: 1, failure: true},
		{name: "Should reject unknown prose alone", output: "Unable to determine memories", failure: true},
		{name: "Should reject invalid candidate fields", output: `{"type":"user","content":""}`, failure: true},
		{name: "Should reject null", output: `null`, failure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			candidates, err := parseMemoryExtractorCandidates(
				tc.output,
				memcontract.TurnRecord{SessionID: "parent"},
				"",
				time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC),
			)
			if len(candidates) != tc.count || (err != nil) != tc.failure {
				t.Fatalf(
					"parse = %d candidates, %v; want %d candidates, failure=%t",
					len(candidates),
					err,
					tc.count,
					tc.failure,
				)
			}
		})
	}
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

	t.Run("Should retain the source Profile in background role resolution", func(t *testing.T) {
		t.Parallel()
		extractor := &forkedMemoryExtractor{
			sessions: &recordingMemoryExtractorSessions{},
			roles: roleResolverFunc(
				func(ctx context.Context, workspaceID string, _ compozyconfig.RoleName) (ResolvedRole, error) {
					if got := roleInvocationCorrelationFromContext(
						ctx,
						workspaceID,
					).ProfileID; got != "profile-engineering" {
						t.Errorf("ProfileID = %q", got)
					}
					return ResolvedRole{Enabled: false}, nil
				},
			),
		}
		if _, err := extractor.Extract(
			t.Context(),
			memcontract.TurnRecord{ProfileID: "profile-engineering", SessionID: "source", WorkspaceID: "ws-test"},
		); err != nil {
			t.Fatal(err)
		}
	})

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

	for _, tc := range []struct {
		name, output string
		cause        session.StopCause
		count        int
		promptErr    error
		incomplete   bool
	}{
		{name: "Should stop a parsed empty child without a failure report", output: `{"no_candidates":true}`, cause: session.CauseCompleted},
		{name: "Should preserve partial output and expose its failure", output: `{"type":"user","content":"Pedro prefers concise updates."}` + "\n{broken", cause: session.CauseFailed, count: 1},
		{name: "Should reject an interrupted candidate and retain diagnostic output", output: `{"type":"user","content":"partial fact"}`, incomplete: true, cause: session.CauseFailed},
		{name: "Should classify deadline failure separately from parsing", cause: session.CauseTimeout, promptErr: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			failuresDir := t.TempDir()
			sessions := &recordingMemoryExtractorSessions{
				output:     tc.output,
				promptErr:  tc.promptErr,
				incomplete: tc.incomplete,
			}
			extractor := &forkedMemoryExtractor{
				sessions:    sessions,
				roles:       resolvedRoleResolver(ResolvedRole{Enabled: true}),
				deadline:    time.Second,
				failuresDir: failuresDir,
			}
			candidates, err := extractor.Extract(
				t.Context(),
				memcontract.TurnRecord{SessionID: "parent", WorkspaceID: "workspace", AgentID: "agent"},
			)
			if (err != nil) != (tc.cause != session.CauseCompleted) || len(candidates) != tc.count {
				t.Fatalf("Extract=%#v, %v", candidates, err)
			}
			if sessions.stopCause != tc.cause || strings.Contains(sessions.stopDetail, "extractor completed") {
				t.Fatalf("stop=%v %q", sessions.stopCause, sessions.stopDetail)
			}
			inspector := &daemonMemoryExtractor{failuresDir: failuresDir}
			failures, listErr := inspector.ListFailures(t.Context())
			if listErr != nil {
				t.Fatal(listErr)
			}
			if err == nil {
				if len(failures) != 0 {
					t.Fatalf("unexpected failures: %#v", failures)
				}
				return
			}
			if len(failures) != 1 || failures[0].SessionID != "parent" || failures[0].WorkspaceID != "workspace" ||
				failures[0].AgentName != "agent" {
				t.Fatalf("failures=%#v", failures)
			}
			failure, readErr := readExtractorFailure(failures[0].Path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if tc.incomplete && !strings.Contains(failure.Report.Content, "partial fact") {
				t.Fatalf("incomplete output missing from diagnostic: %#v", failure)
			}
			if _, decodeErr := failure.Candidates(); decodeErr == nil {
				t.Fatal("raw extraction failure must not replay as normalized candidates")
			}
			if tc.promptErr != nil && !errors.Is(err, tc.promptErr) {
				t.Fatalf("error lost deadline identity: %v", err)
			}
		})
	}

	t.Run("Should pass the configured model to the extractor child spawn", func(t *testing.T) {
		t.Parallel()

		sessions := &recordingMemoryExtractorSessions{}
		extractor := &forkedMemoryExtractor{
			sessions: sessions,
			roles: resolvedRoleResolver(ResolvedRole{
				Role:    compozyconfig.RoleMemoryExtractor,
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
		if sessions.spawnOpts.NotifyCreator || !sessions.spawnOpts.NotifyCreatorSet {
			t.Fatalf(
				"spawn notify creator = %t (set=%t), want explicitly disabled",
				sessions.spawnOpts.NotifyCreator,
				sessions.spawnOpts.NotifyCreatorSet,
			)
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
	output     string
	promptErr  error
	stopCause  session.StopCause
	stopDetail string
	incomplete bool
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
	if s.promptErr != nil {
		return nil, s.promptErr
	}
	events := make(chan acp.AgentEvent, 2)
	events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: s.output}
	if !s.incomplete {
		events <- acp.AgentEvent{Type: acp.EventTypeDone, PromptStopReason: acp.PromptStopReasonEndTurn}
	}
	close(events)
	return events, nil
}

func (s *recordingMemoryExtractorSessions) StopWithCause(
	_ context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	s.stoppedID = id
	s.stopCause, s.stopDetail = cause, detail
	return nil
}
