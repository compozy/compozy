//go:build integration

package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/memory"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
	localprovider "github.com/compozy/compozy/internal/memory/provider/local"
	localmemstore "github.com/compozy/compozy/internal/memory/provider/local/memstore"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const compactionIntegrationFact = "cobalt-archive-fact"

func TestDaemonE2EFactoryPressureCompaction(t *testing.T) {
	t.Run(
		"Should preserve factory pressure coverage and resume without enabling background memory",
		func(t *testing.T) {
			acpmock.RequireDriver(t)
			fixture, err := acpmock.LoadFixture(mockFixturePath(t, "agent_roles_fixture.json"))
			if err != nil {
				t.Fatal(err)
			}
			loadSession := false
			fixture.Agents[0].LoadSession = &loadSession
			fixture.Agents[0].Turns = []acpmock.TurnFixture{
				{
					Name:  "fact",
					Match: acpmock.TurnMatch{UserText: "remember the archive fact"},
					Steps: []acpmock.Step{
						{
							Kind: acpmock.StepKindAssistant,
							Text: strings.Repeat("archive-padding ", 2048) + compactionIntegrationFact,
						},
					},
				},
				{
					Name:  "pressure",
					Match: acpmock.TurnMatch{UserText: "cross the pressure threshold"},
					Steps: []acpmock.Step{
						{Kind: acpmock.StepKindAssistant, Text: "current turn remains raw"},
						{
							Kind: acpmock.StepKindDriverControl,
							DriverControl: &acpmock.DriverControlStep{
								Action:     acpmock.DriverControlWriteRawJSONRPC,
								RawJSONRPC: `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"role-agent-session-1","update":{"sessionUpdate":"usage_update","used":90,"size":100}}}`,
							},
						},
					},
				},
				{
					Name:  "summary",
					Match: acpmock.TurnMatch{TurnSource: "synthetic"},
					Steps: []acpmock.Step{
						{Kind: acpmock.StepKindAssistant, Text: compactionCheckpointBody(compactionIntegrationFact)},
					},
				},
				{
					Name: "resume",
					Match: acpmock.TurnMatch{
						UserText:            "recover the archive fact",
						RawUserTextContains: "<compozy_context_replay>",
					},
					Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: "recovered"}},
				},
				{
					Name:  "lost-runtime",
					Match: acpmock.TurnMatch{UserText: "recover the archive fact"},
					Steps: []acpmock.Step{
						{Kind: acpmock.StepKindAssistant, Text: "recovering context"},
						{
							Kind:          acpmock.StepKindDriverControl,
							DriverControl: &acpmock.DriverControlStep{Action: acpmock.DriverControlDisconnect},
						},
					},
				},
			}
			data, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			fixturePath := filepath.Join(t.TempDir(), "pressure.json")
			if err := os.WriteFile(fixturePath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
				ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
					cfg.Memory.Enabled = compozyconfig.DefaultMemoryConfig(compozyconfig.HomePaths{}).Enabled
					cfg.Roles.Dream.Enabled = compozyconfig.DefaultRolesConfig().Dream.Enabled
					cfg.Roles.AutoTitle.Enabled = false
					cfg.Roles.CheckpointSummary.Provider = acpmock.ProviderName
					cfg.Roles.CheckpointSummary.Agent = "pressure-agent"
				}},
				MockAgents: []e2etest.MockAgentSpec{
					{FixturePath: fixturePath, FixtureAgent: "role-agent", AgentName: "pressure-agent"},
				},
			})
			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			defer cancel()
			root := createFixtureBackedSession(t, ctx, harness, "pressure-agent", "")
			defer func() {
				captureCtx, captureCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer captureCancel()
				if err := harness.CaptureSessionEvents(captureCtx, root.ID); err != nil {
					t.Errorf("capture pressure events: %v", err)
				}
				if registration, ok := harness.MockAgentRegistration("pressure-agent"); ok {
					if err := harness.CaptureProviderCallsFile(
						registration.DiagnosticsPath,
						"application/x-ndjson",
					); err != nil {
						t.Errorf("capture pressure provider calls: %v", err)
					}
				}
				logs, err := os.ReadFile(harness.HomePaths.LogFile)
				if err != nil {
					t.Errorf("read pressure daemon log: %v", err)
					return
				}
				if err := os.WriteFile(
					filepath.Join(harness.Artifacts.RootDir(), "daemon.log"),
					logs,
					0o600,
				); err != nil {
					t.Errorf("capture pressure daemon log: %v", err)
				}
			}()
			if _, err := harness.PromptSession(ctx, root.ID, "remember the archive fact"); err != nil {
				t.Fatal(err)
			}
			for _, info := range readWorkspaceRoleSessions(t, ctx, harness) {
				if info.ID != root.ID {
					t.Fatalf("unexpected idle child: %#v", info)
				}
			}
			db, err := sql.Open("sqlite", store.SessionDBFile(filepath.Join(harness.HomePaths.SessionsDir, root.ID)))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, err := harness.PromptSession(ctx, root.ID, "cross the pressure threshold"); err != nil {
				t.Fatal(err)
			}
			waitForRuntimeCondition(t, "factory pressure archives covered facts", 20*time.Second, func() bool {
				var count int
				err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE type = 'agent_message' AND archived = 1 AND content LIKE ?", "%"+compactionIntegrationFact+"%").
					Scan(&count)
				if err != nil {
					t.Fatalf("read compacted events: %v", err)
				}
				return count > 0
			})
			checkpoint := readCompactionCheckpoint(t, harness.WorkspaceRoot)
			if !strings.Contains(checkpoint, compactionIntegrationFact) ||
				!strings.Contains(checkpoint, "compozy:checkpoint-compaction:v1") {
				t.Fatalf("checkpoint lacks covered fact: %q", checkpoint)
			}
			var raw, history int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM(CASE WHEN archived = 0 THEN 1 ELSE 0 END), 0) FROM events WHERE type = 'agent_message' AND content LIKE ?", "%"+compactionIntegrationFact+"%").
				Scan(&history, &raw); err != nil {
				t.Fatal(err)
			}
			if history == 0 || raw != 0 {
				t.Fatalf("fact history=%d replay=%d, want retained history and compacted replay", history, raw)
			}
			if _, err := harness.PromptSession(ctx, root.ID, "recover the archive fact"); err != nil {
				t.Fatal(err)
			}
			registration, ok := harness.MockAgentRegistration("pressure-agent")
			if !ok {
				t.Fatal("missing pressure agent registration")
			}
			records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
			if err != nil {
				t.Fatal(err)
			}
			parentRecords := acpmock.DiagnosticsForCompozySession(records, root.ID)
			var calls, starts int
			for _, record := range parentRecords {
				if record.ProtocolMethod == "session/prompt" {
					calls++
				}
				if record.LifecycleEvent == "session_new" {
					starts++
				}
			}
			prompts := acpmock.PromptDiagnostics(parentRecords)
			if len(prompts) != 3 || calls != 4 || starts != 2 {
				t.Fatalf(
					"parent completed=%d calls=%d starts=%d, want 3 completions, 4 attempts and 2 runtimes",
					len(prompts),
					calls,
					starts,
				)
			}
			last := prompts[2].Prompt
			if !strings.Contains(last, compactionIntegrationFact) ||
				strings.Contains(compactionReplayPayload(t, last), compactionIntegrationFact) {
				t.Fatal("degraded resume lost checkpoint coverage or replayed archived raw facts")
			}
			children := 0
			for _, info := range readWorkspaceRoleSessions(t, ctx, harness) {
				if info.ID == root.ID {
					continue
				}
				if info.Name != checkpointSummarySessionName {
					t.Fatalf("unexpected background session: %#v", info)
				}
				children++
			}
			if children != 1 {
				t.Fatalf("checkpoint children=%d, want pressure-only child", children)
			}
		},
	)
}

func TestDaemonE2ECompactionResumeSafety(t *testing.T) {
	t.Run(
		"Should keep compacted facts durable when a dead session continues through a child without raw replay",
		func(t *testing.T) {
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
				session.WithSessionCompactionConfig(compozyconfig.DefaultSessionCompactionConfig()),
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
				!strings.Contains(checkpoint, "compozy:checkpoint-compaction:v1") {
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
			if _, err := restarted.Resume(
				testutil.Context(t),
				created.ID,
			); !errors.Is(
				err,
				store.ErrSessionNotAttachable,
			) {
				t.Fatalf("Resume(dead session) error = %v, want ErrSessionNotAttachable", err)
			}
			child, err := restarted.Create(testutil.Context(t), session.CreateOpts{
				AgentName: resolved.Agents[0].Name,
				Workspace: resolved.ID,
				Lineage:   &store.SessionLineage{ParentSessionID: created.ID},
			})
			if err != nil {
				t.Fatalf("Create(child session) error = %v", err)
			}
			if child.Info().Lineage == nil || child.Info().Lineage.ParentSessionID != created.ID {
				t.Fatalf("child lineage = %#v, want parent %q", child.Info().Lineage, created.ID)
			}
			drainCompactionPrompt(t, restarted, child.ID, "continue from durable context")
			if len(restartDriver.promptCalls) != 1 {
				t.Fatalf("child prompt calls = %d, want 1", len(restartDriver.promptCalls))
			}
			prompt := restartDriver.promptCalls[0].Message
			if strings.Contains(prompt, compactionIntegrationFact) {
				t.Fatalf("child prompt inherited archived parent fact: %q", prompt)
			}
			if strings.Contains(prompt, "<compozy_context_replay>") {
				t.Fatalf("child prompt inherited parent replay: %q", prompt)
			}
			if err := restarted.Stop(testutil.Context(t), child.ID); err != nil {
				t.Fatalf("Stop(restarted) error = %v", err)
			}
			if err := restarted.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("Shutdown(restarted) error = %v", err)
			}
			if err := manager.Stop(testutil.Context(t), created.ID); err != nil {
				t.Fatalf("Stop(pre-crash manager cleanup) error = %v", err)
			}
		},
	)

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
			session.WithSessionCompactionConfig(compozyconfig.DefaultSessionCompactionConfig()),
			session.WithCompactionHandler(runtime),
			session.WithStore(func(
				ctx context.Context,
				owner store.SessionDBOwner,
				path string,
			) (session.EventRecorder, error) {
				db, openErr := sessiondb.OpenSessionDB(ctx, owner, path)
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
			!strings.Contains(checkpoint, "compozy:checkpoint-compaction:v1") {
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
	path := filepath.Join(workspaceRoot, compozyconfig.DirName, "memory", memory.CheckpointSummaryFilename)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	return string(content)
}

func compactionReplayPayload(t *testing.T, prompt string) string {
	t.Helper()
	start := strings.Index(prompt, "<compozy_context_replay>")
	end := strings.Index(prompt, "</compozy_context_replay>")
	if start < 0 || end <= start {
		t.Fatalf("prompt missing replay block: %q", prompt)
	}
	return prompt[start:end]
}
