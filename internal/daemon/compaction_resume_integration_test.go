//go:build integration

package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	localprovider "github.com/compozy/agh/internal/memory/provider/local"
	localmemstore "github.com/compozy/agh/internal/memory/provider/local/memstore"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const compactionIntegrationFact = "cobalt-archive-fact"

func TestDaemonE2ECompactionResumeSafety(t *testing.T) {
	t.Run("Should preserve compacted facts through crash-style degraded resume without raw replay", func(t *testing.T) {
		homePaths := integrationHomePaths(t)
		cfg := testConfig(t, homePaths)
		workspaceRoot := filepath.Join(homePaths.HomeDir, "workspace")
		resolved := newHarnessIntegrationWorkspace(t, homePaths, cfg, workspaceRoot)
		writeDaemonMemoryIndex(t, cfg.Memory.GlobalDir, workspaceRoot)

		daemonInstance, deps := bootHarnessPolicyDaemon(t, homePaths, &cfg)
		t.Cleanup(func() {
			if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown(daemon) error = %v", err)
			}
		})
		runtime := newCompactionIntegrationRuntime(t, daemonInstance, resolved)
		driver := newHarnessIntegrationDriver()
		driver.promptHook = compactionIntegrationPromptHook()
		manager := newHarnessIntegrationManager(
			t,
			homePaths,
			deps,
			resolved,
			driver,
			session.WithSessionCompactionConfig(aghconfig.DefaultSessionCompactionConfig()),
			session.WithCompactionHandler(runtime),
		)
		created, err := manager.Create(testutil.Context(t), session.CreateOpts{
			AgentName: resolved.Agents[0].Name,
			Workspace: resolved.ID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		drainCompactionPrompt(t, manager, created.ID, "remember the archive fact")
		drainCompactionPrompt(t, manager, created.ID, "cross the pressure threshold")

		waitForCondition(t, "prior turn archived", func() bool {
			archived, queryErr := manager.Events(
				testutil.Context(t),
				created.ID,
				store.EventQuery{Archive: store.EventArchiveArchived},
			)
			return queryErr == nil && eventContentsContain(archived, compactionIntegrationFact)
		})
		history, err := manager.History(testutil.Context(t), created.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if !historyContains(history, compactionIntegrationFact) {
			t.Fatalf("History() = %#v, want archived fact retained", history)
		}
		unarchived, err := manager.Events(
			testutil.Context(t),
			created.ID,
			store.EventQuery{Archive: store.EventArchiveUnarchived},
		)
		if err != nil {
			t.Fatalf("Events(unarchived) error = %v", err)
		}
		if eventContentsContain(unarchived, compactionIntegrationFact) {
			t.Fatal("unarchived replay projection contains compacted raw fact")
		}
		checkpoint := readCompactionCheckpoint(t, workspaceRoot)
		if !strings.Contains(checkpoint, compactionIntegrationFact) ||
			!strings.Contains(checkpoint, "agh:checkpoint-compaction:v1") {
			t.Fatalf("checkpoint = %q, want fact and durable coverage", checkpoint)
		}

		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown(pre-crash manager) error = %v", err)
		}
		restartDriver := newHarnessIntegrationDriver()
		restartDriver.startHook = func(opts acp.StartOpts, _ int) error {
			if opts.ResumeSessionID != "" {
				return fmt.Errorf("%w: integration fixture", acp.ErrAgentDoesNotSupportSession)
			}
			return nil
		}
		restarted := newHarnessIntegrationManager(t, homePaths, deps, resolved, restartDriver)
		resumed, err := restarted.Resume(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Resume(load unsupported) error = %v", err)
		}
		drainCompactionPrompt(t, restarted, resumed.ID, "continue from durable context")
		if len(restartDriver.promptCalls) != 1 {
			t.Fatalf("restart prompt calls = %d, want 1", len(restartDriver.promptCalls))
		}
		prompt := restartDriver.promptCalls[0].Message
		if !strings.Contains(prompt, compactionIntegrationFact) {
			t.Fatalf("resumed prompt missing checkpoint fact: %q", prompt)
		}
		replay := compactionReplayPayload(t, prompt)
		if strings.Contains(replay, compactionIntegrationFact) {
			t.Fatalf("raw replay re-inflated archived fact: %s", replay)
		}
		if err := restarted.Stop(testutil.Context(t), resumed.ID); err != nil {
			t.Fatalf("Stop(restarted) error = %v", err)
		}
		if err := restarted.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown(restarted) error = %v", err)
		}
		if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop(pre-crash manager cleanup) error = %v", err)
		}
	})

	t.Run("Should leave raw replay readable when archive fails after summary coverage", func(t *testing.T) {
		homePaths := integrationHomePaths(t)
		cfg := testConfig(t, homePaths)
		workspaceRoot := filepath.Join(homePaths.HomeDir, "workspace")
		resolved := newHarnessIntegrationWorkspace(t, homePaths, cfg, workspaceRoot)
		writeDaemonMemoryIndex(t, cfg.Memory.GlobalDir, workspaceRoot)

		daemonInstance, deps := bootHarnessPolicyDaemon(t, homePaths, &cfg)
		t.Cleanup(func() {
			if err := daemonInstance.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown(daemon) error = %v", err)
			}
		})
		runtime := newCompactionIntegrationRuntime(t, daemonInstance, resolved)
		driver := newHarnessIntegrationDriver()
		driver.promptHook = compactionIntegrationPromptHook()
		archiveFailed := make(chan struct{})
		manager := newHarnessIntegrationManager(
			t,
			homePaths,
			deps,
			resolved,
			driver,
			session.WithSessionCompactionConfig(aghconfig.DefaultSessionCompactionConfig()),
			session.WithCompactionHandler(runtime),
			session.WithStore(func(
				ctx context.Context,
				sessionID string,
				path string,
			) (session.EventRecorder, error) {
				db, openErr := sessiondb.OpenSessionDB(ctx, sessionID, path)
				if openErr != nil {
					return nil, openErr
				}
				return &archiveFailureRecorder{SessionDB: db, failed: archiveFailed}, nil
			}),
		)
		created, err := manager.Create(testutil.Context(t), session.CreateOpts{
			AgentName: resolved.Agents[0].Name,
			Workspace: resolved.ID,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		drainCompactionPrompt(t, manager, created.ID, "remember the archive fact")
		drainCompactionPrompt(t, manager, created.ID, "cross the pressure threshold")
		<-archiveFailed

		checkpoint := readCompactionCheckpoint(t, workspaceRoot)
		if !strings.Contains(checkpoint, compactionIntegrationFact) ||
			!strings.Contains(checkpoint, "agh:checkpoint-compaction:v1") {
			t.Fatalf("checkpoint = %q, want coverage committed before archive failure", checkpoint)
		}
		archived, err := manager.Events(
			testutil.Context(t),
			created.ID,
			store.EventQuery{Archive: store.EventArchiveArchived},
		)
		if err != nil {
			t.Fatalf("Events(archived) error = %v", err)
		}
		if len(archived) != 0 {
			t.Fatalf("archived after injected failure = %#v, want none", archived)
		}
		unarchived, err := manager.Events(
			testutil.Context(t),
			created.ID,
			store.EventQuery{Archive: store.EventArchiveUnarchived},
		)
		if err != nil {
			t.Fatalf("Events(unarchived) error = %v", err)
		}
		if !eventContentsContain(unarchived, compactionIntegrationFact) {
			t.Fatal("archive failure removed the raw replay fact")
		}
		if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	})
}

func newCompactionIntegrationRuntime(
	t *testing.T,
	daemonInstance *Daemon,
	resolved workspacepkg.ResolvedWorkspace,
) *checkpointSummaryRuntime {
	t.Helper()
	resolver := &harnessIntegrationWorkspaceResolver{resolved: resolved}
	service := memory.NewCheckpointSummaryService(
		daemonInstance.memoryStore,
		resolver,
		compactionIntegrationSummarizer{},
	)
	provider := localprovider.New(localmemstore.New(daemonInstance.memoryStore))
	provider.SetPreCompressHandler(service)
	if err := provider.Initialize(testutil.Context(t), memcontract.ProviderInit{
		WorkspaceID:   resolved.ID,
		WorkspaceRoot: resolved.RootDir,
		Logger:        discardLogger(),
	}); err != nil {
		t.Fatalf("Initialize(local memory provider) error = %v", err)
	}
	t.Cleanup(func() {
		if err := provider.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("Shutdown(local memory provider) error = %v", err)
		}
	})
	registry := extensionpkg.NewMemoryProviderRegistry()
	if err := registry.Register(testutil.Context(t), extensionpkg.MemoryProviderRegistration{
		Name: localprovider.Name, Provider: provider, Bundled: true,
	}); err != nil {
		t.Fatalf("Register(local memory provider) error = %v", err)
	}
	if err := registry.SetActive(testutil.Context(t), "", localprovider.Name); err != nil {
		t.Fatalf("SetActive(local memory provider) error = %v", err)
	}
	return newCheckpointSummaryRuntime(
		&fakeSessionManager{},
		registry,
		time.Minute,
		time.Minute,
		discardLogger(),
		service,
	)
}

type compactionIntegrationSummarizer struct{}

func (compactionIntegrationSummarizer) Summarize(
	context.Context,
	memory.CheckpointSummaryRequest,
) (string, error) {
	return compactionCheckpointBody(compactionIntegrationFact), nil
}

func compactionCheckpointBody(fact string) string {
	return strings.Join([]string{
		"## Historical Task Snapshot", "None.",
		"## Goal", fact,
		"## Constraints & Preferences", "None.",
		"## Completed Actions", "1. Preserved the compacted fact.",
		"## Active State", "Idle.",
		"## Historical In-Progress State", "None.",
		"## Blocked", "None.",
		"## Key Decisions", fact,
		"## Resolved Questions", "None.",
		"## Historical Pending User Asks", "None.",
		"## Relevant Files", "None.",
		"## Historical Remaining Work", "None.",
		"## Critical Context", fact,
	}, "\n\n")
}

func compactionIntegrationPromptHook() func(
	context.Context,
	*session.AgentProcess,
	acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	var mu sync.Mutex
	ordinal := 0
	return func(
		_ context.Context,
		proc *session.AgentProcess,
		req acp.PromptRequest,
	) (<-chan acp.AgentEvent, error) {
		mu.Lock()
		ordinal++
		current := ordinal
		mu.Unlock()
		used, size := int64(50), int64(100)
		text := strings.Repeat("archive-padding ", 2048) + compactionIntegrationFact
		if current > 1 {
			used = 90
			text = "current turn remains raw"
		}
		at := time.Now().UTC()
		events := make(chan acp.AgentEvent, 2)
		events <- acp.AgentEvent{
			Type: acp.EventTypeAgentMessage, SessionID: proc.SessionID,
			TurnID: req.TurnID, Timestamp: at, Text: text,
		}
		events <- acp.AgentEvent{
			Type: acp.EventTypeDone, SessionID: proc.SessionID,
			TurnID: req.TurnID, Timestamp: at,
			StopReason:       string(acp.PromptStopReasonEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
			Usage: &acp.TokenUsage{
				TurnID: req.TurnID, ContextUsed: &used, ContextSize: &size, Timestamp: at,
			},
		}
		close(events)
		return events, nil
	}
}

type archiveFailureRecorder struct {
	*sessiondb.SessionDB
	once   sync.Once
	failed chan struct{}
}

func (r *archiveFailureRecorder) ArchiveEvents(
	context.Context,
	store.EventArchiveRequest,
) (store.EventArchiveResult, error) {
	r.once.Do(func() { close(r.failed) })
	return store.EventArchiveResult{}, errors.New("integration: crash before archive flag")
}

func drainCompactionPrompt(t *testing.T, manager *session.Manager, sessionID string, message string) {
	t.Helper()
	events, err := manager.Prompt(testutil.Context(t), sessionID, message)
	if err != nil {
		t.Fatalf("Prompt(%q) error = %v", message, err)
	}
	drainHarnessIntegrationEvents(events)
}

func eventContentsContain(events []store.SessionEvent, needle string) bool {
	for _, event := range events {
		if strings.Contains(event.Content, needle) {
			return true
		}
	}
	return false
}

func historyContains(history []store.TurnHistory, needle string) bool {
	for _, turn := range history {
		if eventContentsContain(turn.Events, needle) {
			return true
		}
	}
	return false
}

func readCompactionCheckpoint(t *testing.T, workspaceRoot string) string {
	t.Helper()
	path := filepath.Join(workspaceRoot, aghconfig.DirName, "memory", memory.CheckpointSummaryFilename)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return string(content)
}

func compactionReplayPayload(t *testing.T, prompt string) string {
	t.Helper()
	start := strings.Index(prompt, "<agh_context_replay>")
	end := strings.Index(prompt, "</agh_context_replay>")
	if start < 0 || end <= start {
		t.Fatalf("prompt missing replay block: %q", prompt)
	}
	return prompt[start:end]
}
