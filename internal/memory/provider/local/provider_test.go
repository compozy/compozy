package local_test

import (
	"context"
	"errors"
	"go/build"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	localprovider "github.com/compozy/agh/internal/memory/provider/local"
	"github.com/compozy/agh/internal/memory/provider/local/memstore"
	"github.com/compozy/agh/internal/testutil"
	"github.com/goccy/go-yaml"
)

func TestProviderLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should initialize no-op lifecycle hooks and shutdown deterministically", func(t *testing.T) {
		t.Parallel()

		provider := localprovider.New(memstore.New(memory.NewStore(filepath.Join(t.TempDir(), "agh-home", "memory"))))
		ctx := testutil.Context(t)
		if _, err := provider.SystemPromptBlock(
			ctx,
			memcontract.SnapshotRequest{Scope: memcontract.ScopeGlobal},
		); err == nil {
			t.Fatal("SystemPromptBlock(before Initialize) error = nil, want error")
		}
		if err := provider.Initialize(ctx, memcontract.ProviderInit{
			WorkspaceID: "ws-alpha",
			Config:      map[string]any{"mode": "local"},
		}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		if err := provider.Prefetch(ctx, memcontract.PrefetchRequest{SessionID: "sess-1"}); err != nil {
			t.Fatalf("Prefetch() error = %v", err)
		}
		if err := provider.SyncTurn(ctx, memcontract.TurnRecord{SessionID: "sess-1"}); err != nil {
			t.Fatalf("SyncTurn() error = %v", err)
		}
		if err := provider.OnSessionEnd(ctx, memcontract.SessionEndRecord{SessionID: "sess-1"}); err != nil {
			t.Fatalf("OnSessionEnd() error = %v", err)
		}
		if err := provider.OnSessionSwitch(ctx, memcontract.SessionSwitchRecord{
			FromSession: "sess-1",
			ToSession:   "sess-2",
		}); err != nil {
			t.Fatalf("OnSessionSwitch() error = %v", err)
		}
		if _, err := provider.OnPreCompress(ctx, memcontract.PreCompressRequest{SessionID: "sess-1"}); err != nil {
			t.Fatalf("OnPreCompress() error = %v", err)
		}
		if err := provider.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		if err := provider.Prefetch(ctx, memcontract.PrefetchRequest{SessionID: "sess-2"}); err == nil {
			t.Fatal("Prefetch(after Shutdown) error = nil, want error")
		}
	})

	t.Run("Should delegate session end records to the configured handler", func(t *testing.T) {
		t.Parallel()

		provider := localprovider.New(memstore.New(memory.NewStore(filepath.Join(t.TempDir(), "memory"))))
		ctx := testutil.Context(t)
		if err := provider.Initialize(ctx, memcontract.ProviderInit{WorkspaceID: "ws-alpha"}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		wantErr := errors.New("checkpoint failed")
		var captured memcontract.SessionEndRecord
		provider.SetSessionEndHandler(sessionEndHandlerFunc(func(
			_ context.Context,
			record memcontract.SessionEndRecord,
		) error {
			captured = record
			return wantErr
		}))
		input := memcontract.SessionEndRecord{
			WorkspaceID: "ws-alpha",
			SessionID:   "sess-alpha",
			AgentName:   "coder",
		}
		err := provider.OnSessionEnd(ctx, input)
		if !errors.Is(err, wantErr) {
			t.Fatalf("OnSessionEnd() error = %v, want wrapped %v", err, wantErr)
		}
		if captured.WorkspaceID != input.WorkspaceID ||
			captured.SessionID != input.SessionID ||
			captured.AgentName != input.AgentName {
			t.Fatalf("OnSessionEnd() record = %#v, want %#v", captured, input)
		}
	})

	t.Run("Should delegate pre-compress exactly once with the covered range", func(t *testing.T) {
		t.Parallel()

		provider := localprovider.New(memstore.New(memory.NewStore(filepath.Join(t.TempDir(), "memory"))))
		ctx := testutil.Context(t)
		if err := provider.Initialize(ctx, memcontract.ProviderInit{WorkspaceID: "ws-alpha"}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		wantHint := memcontract.PreCompressHint{Markdown: "covered"}
		var calls int
		var captured memcontract.PreCompressRequest
		provider.SetPreCompressHandler(preCompressHandlerFunc(func(
			_ context.Context,
			request memcontract.PreCompressRequest,
		) (memcontract.PreCompressHint, error) {
			calls++
			captured = request
			return wantHint, nil
		}))
		input := memcontract.PreCompressRequest{
			WorkspaceID:  "ws-alpha",
			SessionID:    "sess-alpha",
			FromSequence: 1,
			ToSequence:   4,
		}
		hint, err := provider.OnPreCompress(ctx, input)
		if err != nil {
			t.Fatalf("OnPreCompress() error = %v", err)
		}
		if calls != 1 || captured.WorkspaceID != input.WorkspaceID || captured.SessionID != input.SessionID ||
			captured.FromSequence != input.FromSequence || captured.ToSequence != input.ToSequence ||
			hint.Markdown != wantHint.Markdown {
			t.Fatalf("OnPreCompress() calls/input/hint = %d / %#v / %#v", calls, captured, hint)
		}
	})
}

type sessionEndHandlerFunc func(context.Context, memcontract.SessionEndRecord) error

func (f sessionEndHandlerFunc) OnSessionEnd(
	ctx context.Context,
	record memcontract.SessionEndRecord,
) error {
	return f(ctx, record)
}

type preCompressHandlerFunc func(
	context.Context,
	memcontract.PreCompressRequest,
) (memcontract.PreCompressHint, error)

func (f preCompressHandlerFunc) OnPreCompress(
	ctx context.Context,
	request memcontract.PreCompressRequest,
) (memcontract.PreCompressHint, error) {
	return f(ctx, request)
}

func TestProviderBackendContract(t *testing.T) {
	t.Run("Should use the contract backend for prompt recall and write hooks", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)
		backend := &stubBackend{
			prompt: "## Memory\n- Contract backend\n",
			headers: []memcontract.Header{{
				Name:    "Contract backend",
				Type:    memcontract.TypeProject,
				Scope:   memcontract.ScopeGlobal,
				ModTime: now.Add(-time.Minute),
			}},
			packaged: memcontract.Packaged{Blocks: []memcontract.Block{{
				Scope: memcontract.ScopeGlobal,
				Entries: []memcontract.PackagedEntry{{
					ID:    "global/project.md",
					Title: "Contract backend",
					Body:  "Contract backend recall",
				}},
			}}},
		}
		provider := localprovider.New(
			backend,
			localprovider.WithClock(func() time.Time { return now }),
			localprovider.WithLogger(slog.Default()),
		)
		ctx := testutil.Context(t)
		if err := provider.Initialize(ctx, memcontract.ProviderInit{WorkspaceID: "ws-contract"}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		snapshot, err := provider.SystemPromptBlock(ctx, memcontract.SnapshotRequest{Scope: memcontract.ScopeGlobal})
		if err != nil {
			t.Fatalf("SystemPromptBlock() error = %v", err)
		}
		if snapshot.Markdown != backend.prompt {
			t.Fatalf("SystemPromptBlock().Markdown = %q, want backend prompt", snapshot.Markdown)
		}
		if snapshot.AgeMs <= 0 {
			t.Fatalf("SystemPromptBlock().AgeMs = %d, want positive age", snapshot.AgeMs)
		}
		recalled, err := provider.Recall(ctx, memcontract.RecallRequest{
			Query: memcontract.Query{QueryText: "contract backend"},
		})
		if err != nil {
			t.Fatalf("Recall() error = %v", err)
		}
		if backend.recallQuery.WorkspaceID != "ws-contract" {
			t.Fatalf("Recall() workspace = %q, want initialized workspace", backend.recallQuery.WorkspaceID)
		}
		if len(recalled.Blocks) != 1 {
			t.Fatalf("Recall() blocks = %d, want 1", len(recalled.Blocks))
		}
		decision := memcontract.Decision{
			ID:             "dec_contract",
			CandidateHash:  "hash_contract",
			IdempotencyKey: "key_contract",
			Op:             memcontract.OpNoop,
			Confidence:     1,
			Source:         memcontract.SourceRule,
			DecidedAt:      now,
		}
		if err := provider.OnMemoryWrite(ctx, memcontract.WriteRecord{Decision: decision}); err != nil {
			t.Fatalf("OnMemoryWrite() error = %v", err)
		}
		if backend.appliedDecision.ID != "dec_contract" {
			t.Fatalf("applied decision = %q, want dec_contract", backend.appliedDecision.ID)
		}
	})
}

func TestProviderSystemPromptBlock(t *testing.T) {
	t.Run("Should read prompt block from local store", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		now := time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC)
		store := memory.NewStore(filepath.Join(t.TempDir(), "agh-home", "memory"))
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if err := store.Write(memcontract.ScopeGlobal, "project_auth.md", memoryPayload(t, memoryPayloadMeta{
			Name:        "Auth Runtime",
			Description: "Auth session rules",
			Type:        memcontract.TypeProject,
		}, "Remember auth session migration rules.\n")); err != nil {
			t.Fatalf("Store.Write() error = %v", err)
		}

		provider := localprovider.New(memstore.New(store), localprovider.WithClock(func() time.Time { return now }))
		if err := provider.Initialize(ctx, memcontract.ProviderInit{}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		result, err := provider.SystemPromptBlock(ctx, memcontract.SnapshotRequest{Scope: memcontract.ScopeGlobal})
		if err != nil {
			t.Fatalf("SystemPromptBlock() error = %v", err)
		}
		if !strings.Contains(result.Markdown, "Auth Runtime") {
			t.Fatalf("SystemPromptBlock().Markdown = %q, want memory index", result.Markdown)
		}
		if result.AgeMs < 0 {
			t.Fatalf("SystemPromptBlock().AgeMs = %d, want non-negative", result.AgeMs)
		}
	})

	t.Run("Should use initialized workspace root for workspace snapshots", func(t *testing.T) {
		t.Parallel()

		workspaceBackend := &stubBackend{
			prompt: "## Memory\n- Workspace-bound prompt\n",
		}
		rootBackend := &stubBackend{workspaceBackend: workspaceBackend}
		provider := localprovider.New(rootBackend)
		ctx := testutil.Context(t)
		if err := provider.Initialize(ctx, memcontract.ProviderInit{
			WorkspaceID:   "ws-alpha",
			WorkspaceRoot: "/workspaces/alpha",
		}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}

		result, err := provider.SystemPromptBlock(ctx, memcontract.SnapshotRequest{
			Scope: memcontract.ScopeWorkspace,
		})
		if err != nil {
			t.Fatalf("SystemPromptBlock(workspace) error = %v", err)
		}
		if result.Markdown != workspaceBackend.prompt {
			t.Fatalf("SystemPromptBlock(workspace).Markdown = %q, want initialized workspace prompt", result.Markdown)
		}
		if len(rootBackend.workspaceRequests) != 1 || rootBackend.workspaceRequests[0].root != "/workspaces/alpha" {
			t.Fatalf("workspace requests = %#v, want initialized workspace root", rootBackend.workspaceRequests)
		}
	})
}

func TestProviderRecall(t *testing.T) {
	t.Run("Should delegate recall to deterministic store pipeline", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		store := memory.NewStore(
			filepath.Join(baseDir, "agh-home", "memory"),
			memory.WithCatalogDatabasePath(filepath.Join(baseDir, "agh.db")),
		).ForWorkspace(workspaceRoot)
		openProviderTestCatalog(t, store)
		if err := store.EnsureDirs(); err != nil {
			t.Fatalf("Store.EnsureDirs() error = %v", err)
		}
		if err := store.Write(memcontract.ScopeWorkspace, "project_auth.md", memoryPayload(t, memoryPayloadMeta{
			Name:        "Workspace Auth",
			Description: "Workspace auth migration",
			Type:        memcontract.TypeProject,
		}, "Workspace auth migration sessions should be recalled.\n")); err != nil {
			t.Fatalf("Store.Write() error = %v", err)
		}

		provider := localprovider.New(memstore.New(store))
		if err := provider.Initialize(ctx, memcontract.ProviderInit{}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		result, err := provider.Recall(ctx, memcontract.RecallRequest{
			Query:   memcontract.Query{QueryText: "auth migration sessions"},
			Options: memcontract.RecallOptions{TopK: 5},
		})
		if err != nil {
			t.Fatalf("Recall() error = %v", err)
		}
		entries := recallEntries(result.Packaged)
		if len(entries) != 1 {
			t.Fatalf("Recall() entries = %d, want 1", len(entries))
		}
		if !strings.Contains(entries[0].Body, "Workspace auth migration sessions") {
			t.Fatalf("Recall() entry body = %q, want stored memory", entries[0].Body)
		}
	})
}

func TestProviderOnMemoryWrite(t *testing.T) {
	t.Run("Should apply write decisions through the local store", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		store := memory.NewStore(
			filepath.Join(t.TempDir(), "agh-home", "memory"),
			memory.WithCatalogDatabasePath(filepath.Join(t.TempDir(), "agh.db")),
		)
		openProviderTestCatalog(t, store)
		provider := localprovider.New(memstore.New(store))
		if err := provider.Initialize(ctx, memcontract.ProviderInit{}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}

		frontmatter := memcontract.Header{
			Name:  "Provider Write",
			Type:  memcontract.TypeProject,
			Scope: memcontract.ScopeGlobal,
		}
		content := string(memoryPayload(t, memoryPayloadMeta{
			Name: "Provider Write",
			Type: memcontract.TypeProject,
		}, "Provider write decisions use the store seam.\n"))
		decision := memcontract.Decision{
			ID:              "dec_provider_write",
			CandidateHash:   "candidate_hash",
			IdempotencyKey:  "provider-write-key",
			Op:              memcontract.OpAdd,
			TargetFilename:  "project_provider_write.md",
			Frontmatter:     frontmatter,
			PostContent:     content,
			PostContentHash: "post_hash",
			Confidence:      1,
			Source:          memcontract.SourceRule,
			Reason:          "provider write test",
			DecidedAt:       time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC),
		}
		if err := provider.OnMemoryWrite(ctx, memcontract.WriteRecord{Decision: decision}); err != nil {
			t.Fatalf("OnMemoryWrite() error = %v", err)
		}
		got, err := store.Read(memcontract.ScopeGlobal, "project_provider_write.md")
		if err != nil {
			t.Fatalf("Store.Read() error = %v", err)
		}
		if !strings.Contains(string(got), "Provider write decisions use the store seam.") {
			t.Fatalf("Store.Read() = %q, want applied decision content", string(got))
		}
	})

	t.Run("Should route agent-scoped decisions through an agent backend", func(t *testing.T) {
		t.Parallel()

		agentBackend := &stubBackend{}
		rootBackend := &stubBackend{agentBackend: agentBackend}
		provider := localprovider.New(rootBackend)
		ctx := testutil.Context(t)
		if err := provider.Initialize(ctx, memcontract.ProviderInit{WorkspaceID: "ws-alpha"}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		decision := memcontract.Decision{
			ID:             "dec_agent",
			CandidateHash:  "hash_agent",
			IdempotencyKey: "key_agent",
			Op:             memcontract.OpNoop,
			Frontmatter: memcontract.Header{
				Name:      "Agent rule",
				Type:      memcontract.TypeFeedback,
				Scope:     memcontract.ScopeAgent,
				AgentName: "reviewer",
			},
			Confidence: 1,
			Source:     memcontract.SourceRule,
			DecidedAt:  time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC),
		}
		if err := provider.OnMemoryWrite(ctx, memcontract.WriteRecord{Decision: decision}); err != nil {
			t.Fatalf("OnMemoryWrite(agent) error = %v", err)
		}
		if len(rootBackend.agentRequests) != 1 {
			t.Fatalf("agent backend requests = %d, want 1", len(rootBackend.agentRequests))
		}
		request := rootBackend.agentRequests[0]
		if request.workspaceID != "ws-alpha" || request.agentName != "reviewer" ||
			request.tier != memcontract.AgentTierWorkspace {
			t.Fatalf("agent backend request = %#v, want initialized workspace and default workspace tier", request)
		}
		if agentBackend.appliedDecision.ID != "dec_agent" {
			t.Fatalf("agent applied decision = %q, want dec_agent", agentBackend.appliedDecision.ID)
		}
	})

	t.Run("Should route workspace-scoped decisions through initialized workspace backend", func(t *testing.T) {
		t.Parallel()

		workspaceBackend := &stubBackend{}
		rootBackend := &stubBackend{workspaceBackend: workspaceBackend}
		provider := localprovider.New(rootBackend)
		ctx := testutil.Context(t)
		if err := provider.Initialize(ctx, memcontract.ProviderInit{
			WorkspaceID:   "ws-alpha",
			WorkspaceRoot: "/workspaces/alpha",
		}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		decision := memcontract.Decision{
			ID:             "dec_workspace",
			CandidateHash:  "hash_workspace",
			IdempotencyKey: "key_workspace",
			Op:             memcontract.OpNoop,
			Frontmatter: memcontract.Header{
				Name:  "Workspace rule",
				Type:  memcontract.TypeProject,
				Scope: memcontract.ScopeWorkspace,
			},
			Confidence: 1,
			Source:     memcontract.SourceRule,
			DecidedAt:  time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC),
		}
		if err := provider.OnMemoryWrite(ctx, memcontract.WriteRecord{Decision: decision}); err != nil {
			t.Fatalf("OnMemoryWrite(workspace) error = %v", err)
		}
		if rootBackend.appliedDecision.ID != "" {
			t.Fatalf("root applied decision = %q, want no root write", rootBackend.appliedDecision.ID)
		}
		if workspaceBackend.appliedDecision.ID != "dec_workspace" {
			t.Fatalf("workspace applied decision = %q, want dec_workspace", workspaceBackend.appliedDecision.ID)
		}
		if len(rootBackend.workspaceRequests) != 1 || rootBackend.workspaceRequests[0].root != "/workspaces/alpha" {
			t.Fatalf("workspace requests = %#v, want initialized workspace root", rootBackend.workspaceRequests)
		}
	})

	t.Run("Should reject initialized workspace writes without workspace root", func(t *testing.T) {
		t.Parallel()

		provider := localprovider.New(&stubBackend{})
		ctx := testutil.Context(t)
		if err := provider.Initialize(ctx, memcontract.ProviderInit{WorkspaceID: "ws-alpha"}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		decision := memcontract.Decision{
			ID:             "dec_workspace_missing_root",
			CandidateHash:  "hash_workspace_missing_root",
			IdempotencyKey: "key_workspace_missing_root",
			Op:             memcontract.OpNoop,
			Frontmatter: memcontract.Header{
				Name:  "Workspace rule",
				Type:  memcontract.TypeProject,
				Scope: memcontract.ScopeWorkspace,
			},
			Confidence: 1,
			Source:     memcontract.SourceRule,
			DecidedAt:  time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC),
		}
		if err := provider.OnMemoryWrite(ctx, memcontract.WriteRecord{Decision: decision}); err == nil {
			t.Fatal("OnMemoryWrite(workspace without root) error = nil, want error")
		}
	})

	t.Run("Should reject agent-scoped decisions without agent name", func(t *testing.T) {
		t.Parallel()

		provider := localprovider.New(&stubBackend{})
		ctx := testutil.Context(t)
		if err := provider.Initialize(ctx, memcontract.ProviderInit{WorkspaceID: "ws-alpha"}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		decision := memcontract.Decision{
			ID:             "dec_agent_missing_name",
			CandidateHash:  "hash_agent_missing_name",
			IdempotencyKey: "key_agent_missing_name",
			Op:             memcontract.OpNoop,
			Frontmatter: memcontract.Header{
				Name:  "Agent rule",
				Type:  memcontract.TypeFeedback,
				Scope: memcontract.ScopeAgent,
			},
			Confidence: 1,
			Source:     memcontract.SourceRule,
			DecidedAt:  time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC),
		}
		if err := provider.OnMemoryWrite(ctx, memcontract.WriteRecord{Decision: decision}); err == nil {
			t.Fatal("OnMemoryWrite(agent without name) error = nil, want error")
		}
	})
}

func openProviderTestCatalog(t *testing.T, store *memory.Store) {
	t.Helper()
	if err := store.OpenCatalog(testutil.Context(t)); err != nil {
		t.Fatalf("Store.OpenCatalog() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.CloseCatalog(testutil.Context(t)); err != nil {
			t.Errorf("Store.CloseCatalog() error = %v", err)
		}
	})
}

func TestProviderValidationErrors(t *testing.T) {
	t.Run("Should reject invalid lifecycle and snapshot inputs", func(t *testing.T) {
		t.Parallel()

		provider := localprovider.New(&stubBackend{})
		ctx := testutil.Context(t)
		if err := localprovider.New(nil).Initialize(ctx, memcontract.ProviderInit{}); err == nil {
			t.Fatal("Initialize(nil backend) error = nil, want error")
		}
		if err := provider.Initialize(ctx, memcontract.ProviderInit{}); err != nil {
			t.Fatalf("Initialize(valid) error = %v", err)
		}
		if _, err := provider.SystemPromptBlock(ctx, memcontract.SnapshotRequest{
			Scope: memcontract.Scope("invalid"),
		}); err == nil {
			t.Fatal("SystemPromptBlock(invalid scope) error = nil, want error")
		}
		if _, err := provider.SystemPromptBlock(ctx, memcontract.SnapshotRequest{
			Scope: memcontract.ScopeAgent,
		}); err == nil {
			t.Fatal("SystemPromptBlock(agent without name) error = nil, want error")
		}
		if _, err := provider.SystemPromptBlock(ctx, memcontract.SnapshotRequest{
			Scope:     memcontract.ScopeAgent,
			AgentName: "reviewer",
			AgentTier: memcontract.AgentTier("invalid"),
		}); err == nil {
			t.Fatal("SystemPromptBlock(invalid agent tier) error = nil, want error")
		}
	})

	t.Run("Should propagate backend failures", func(t *testing.T) {
		t.Parallel()

		boom := errors.New("boom")
		ctx := testutil.Context(t)
		if err := localprovider.New(&stubBackend{ensureErr: boom}).
			Initialize(ctx, memcontract.ProviderInit{}); err == nil {
			t.Fatal("Initialize(backend error) error = nil, want error")
		}
		backend := &stubBackend{loadErr: boom}
		provider := localprovider.New(backend)
		if err := provider.Initialize(ctx, memcontract.ProviderInit{}); err != nil {
			t.Fatalf("Initialize(valid) error = %v", err)
		}
		if _, err := provider.SystemPromptBlock(
			ctx,
			memcontract.SnapshotRequest{Scope: memcontract.ScopeGlobal},
		); err == nil {
			t.Fatal("SystemPromptBlock(load error) error = nil, want error")
		}
		backend.loadErr = nil
		backend.listErr = boom
		if _, err := provider.SystemPromptBlock(
			ctx,
			memcontract.SnapshotRequest{Scope: memcontract.ScopeGlobal},
		); err == nil {
			t.Fatal("SystemPromptBlock(list error) error = nil, want error")
		}
		backend.listErr = nil
		backend.recallErr = boom
		if _, err := provider.Recall(
			ctx,
			memcontract.RecallRequest{Query: memcontract.Query{QueryText: "boom"}},
		); err == nil {
			t.Fatal("Recall(backend error) error = nil, want error")
		}
		backend.recallErr = nil
		backend.applyErr = boom
		if err := provider.OnMemoryWrite(ctx, memcontract.WriteRecord{Decision: memcontract.Decision{
			ID:             "dec_error",
			CandidateHash:  "hash_error",
			IdempotencyKey: "key_error",
			Op:             memcontract.OpNoop,
			Confidence:     1,
			Source:         memcontract.SourceRule,
			DecidedAt:      time.Date(2026, 5, 5, 8, 0, 0, 0, time.UTC),
		}}); err == nil {
			t.Fatal("OnMemoryWrite(backend error) error = nil, want error")
		}
	})
}

func TestProviderImportBoundary(t *testing.T) {
	t.Run("Should not import controller or recall internals directly", func(t *testing.T) {
		t.Parallel()

		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("runtime.Caller() failed")
		}
		pkg, err := build.ImportDir(filepath.Dir(filename), build.IgnoreVendor)
		if err != nil {
			t.Fatalf("build.ImportDir() error = %v", err)
		}
		for _, imported := range pkg.Imports {
			if strings.HasSuffix(imported, "/internal/memory/controller") ||
				strings.HasSuffix(imported, "/internal/memory/recall") {
				t.Fatalf("local provider imports runtime-private package %q", imported)
			}
			if strings.HasPrefix(imported, "github.com/compozy/agh/internal/") &&
				imported != "github.com/compozy/agh/internal/memory/contract" {
				t.Fatalf("local provider imports non-contract internal package %q", imported)
			}
		}
	})
}

type memoryPayloadMeta struct {
	Name        string
	Description string
	Type        memcontract.Type
}

func memoryPayload(t *testing.T, meta memoryPayloadMeta, body string) []byte {
	t.Helper()

	payload, err := yaml.Marshal(map[string]any{
		"name":        meta.Name,
		"description": meta.Description,
		"type":        meta.Type,
	})
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	return []byte("---\n" + string(payload) + "---\n" + body)
}

func recallEntries(packaged memcontract.Packaged) []memcontract.PackagedEntry {
	entries := make([]memcontract.PackagedEntry, 0)
	for _, block := range packaged.Blocks {
		entries = append(entries, block.Entries...)
	}
	return entries
}

func TestProviderRejectsCanceledContext(t *testing.T) {
	t.Run("Should reject canceled lifecycle contexts", func(t *testing.T) {
		t.Parallel()

		provider := localprovider.New(memstore.New(memory.NewStore(filepath.Join(t.TempDir(), "agh-home", "memory"))))
		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		if err := provider.Initialize(ctx, memcontract.ProviderInit{}); err == nil {
			t.Fatal("Initialize(canceled) error = nil, want error")
		}
	})
}

func TestProviderAgentSnapshot(t *testing.T) {
	t.Run("Should read agent-scoped snapshots through agent store binding", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		baseDir := t.TempDir()
		workspaceRoot := filepath.Join(baseDir, "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
		}
		store := memory.NewStore(filepath.Join(baseDir, "agh-home", "memory")).ForWorkspace(workspaceRoot)
		agentStore := store.ForAgent("ws-alpha", "reviewer", memcontract.AgentTierWorkspace)
		if err := agentStore.EnsureDirs(); err != nil {
			t.Fatalf("agentStore.EnsureDirs() error = %v", err)
		}
		if err := agentStore.Write(memcontract.ScopeAgent, "feedback_style.md", agentPayload(t)); err != nil {
			t.Fatalf("agentStore.Write() error = %v", err)
		}

		provider := localprovider.New(memstore.New(store))
		if err := provider.Initialize(ctx, memcontract.ProviderInit{WorkspaceID: "ws-alpha"}); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		result, err := provider.SystemPromptBlock(ctx, memcontract.SnapshotRequest{
			Scope:       memcontract.ScopeAgent,
			AgentName:   "reviewer",
			AgentTier:   memcontract.AgentTierWorkspace,
			WorkspaceID: "ws-alpha",
		})
		if err != nil {
			t.Fatalf("SystemPromptBlock(agent) error = %v", err)
		}
		if !strings.Contains(result.Markdown, "Reviewer Style") {
			t.Fatalf("SystemPromptBlock(agent).Markdown = %q, want agent memory", result.Markdown)
		}
	})
}

func agentPayload(t *testing.T) []byte {
	t.Helper()

	payload, err := yaml.Marshal(map[string]any{
		"name":       "Reviewer Style",
		"type":       memcontract.TypeFeedback,
		"scope":      memcontract.ScopeAgent,
		"agent":      "reviewer",
		"agent_tier": memcontract.AgentTierWorkspace,
	})
	if err != nil {
		t.Fatalf("yaml.Marshal(agent) error = %v", err)
	}
	return []byte("---\n" + string(payload) + "---\nPrefer concrete review findings.\n")
}

type stubBackend struct {
	ensureErr error
	loadErr   error
	listErr   error
	recallErr error
	applyErr  error

	prompt            string
	headers           []memcontract.Header
	packaged          memcontract.Packaged
	recallQuery       memcontract.Query
	recallOptions     memcontract.RecallOptions
	appliedDecision   memcontract.Decision
	agentBackend      *stubBackend
	workspaceBackend  *stubBackend
	agentRequests     []agentBackendRequest
	workspaceRequests []workspaceBackendRequest
}

type agentBackendRequest struct {
	workspaceID string
	agentName   string
	tier        memcontract.AgentTier
}

type workspaceBackendRequest struct {
	root string
}

func (b *stubBackend) EnsureDirs() error {
	return b.ensureErr
}

func (b *stubBackend) LoadPromptIndex(
	memcontract.Scope,
) (content string, truncated bool, err error) {
	if b.loadErr != nil {
		return "", false, b.loadErr
	}
	return b.prompt, false, nil
}

func (b *stubBackend) List(memcontract.Scope) ([]memcontract.Header, error) {
	if b.listErr != nil {
		return nil, b.listErr
	}
	return append([]memcontract.Header(nil), b.headers...), nil
}

func (b *stubBackend) Recall(
	_ context.Context,
	query memcontract.Query,
	opts memcontract.RecallOptions,
) (memcontract.Packaged, error) {
	if b.recallErr != nil {
		return memcontract.Packaged{}, b.recallErr
	}
	b.recallQuery = query
	b.recallOptions = opts
	return b.packaged, nil
}

func (b *stubBackend) ApplyDecision(_ context.Context, decision memcontract.Decision) error {
	if b.applyErr != nil {
		return b.applyErr
	}
	b.appliedDecision = decision
	return nil
}

func (b *stubBackend) ForWorkspace(workspaceRoot string) localprovider.Backend {
	b.workspaceRequests = append(b.workspaceRequests, workspaceBackendRequest{root: workspaceRoot})
	if b.workspaceBackend != nil {
		return b.workspaceBackend
	}
	return b
}

func (b *stubBackend) ForAgent(
	workspaceID string,
	agentName string,
	tier memcontract.AgentTier,
) localprovider.Backend {
	b.agentRequests = append(b.agentRequests, agentBackendRequest{
		workspaceID: workspaceID,
		agentName:   agentName,
		tier:        tier,
	})
	if b.agentBackend != nil {
		return b.agentBackend
	}
	return b
}
