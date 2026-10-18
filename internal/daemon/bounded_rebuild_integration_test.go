//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const rebuildFirstUser = "Preserve the original bounded rebuild request"

// IT-018 and IT-019 own the real ACP process/SQLite rebuild contract.
func TestDaemonBoundedRebuild(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		native     bool
	}{
		{"Should bound replay when native load is unsupported", "resume", false},
		{"Should bound replay when replacing the runtime", "replace", false},
		{"Should bound replay when the accepted account route changes", "account", true},
		{"Should load native history without sending replay", "resume", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, deps, resolved, diagnostics, daemon := newBoundedRebuildFixture(t, tc.native, false)
			if tc.path == "account" {
				provider := resolved.Config.Providers[acpmock.ProviderName]
				resolved.Agents[0].FallbackChain = []compozyconfig.RoleFallback{
					{Provider: acpmock.ProviderName, Model: "rebuild-model", Command: provider.Command},
				}
				provider.Command = "/missing/bounded-rebuild-primary-account"
				resolved.Config.Providers[acpmock.ProviderName] = provider
			}
			manager := newBoundedRebuildManager(t, home, deps, resolved, daemon)
			active, err := manager.Create(
				t.Context(),
				session.CreateOpts{AgentName: "rebuild-agent", Workspace: resolved.ID},
			)
			if err != nil {
				t.Fatal(err)
			}
			seedBoundedRebuildTurns(t, home, active)
			promptBoundedRebuild(t, manager, active.ID, "bind runtime")
			originalACP := active.Info().ACPSessionID
			if tc.path == "account" {
				if route := active.Meta().AcceptedRoute; route == nil || route.Attempt != 1 {
					t.Fatalf("accepted account route = %#v, want fallback attempt 1", route)
				}
			}
			switch tc.path {
			case "replace":
				if _, err := manager.SetRuntimeSelection(
					t.Context(),
					active.ID,
					session.RuntimeSelection{Provider: "acpmock-replacement"},
					0,
				); err != nil {
					t.Fatal(err)
				}
			case "resume", "account":
				if err := manager.Stop(t.Context(), active.ID); err != nil {
					t.Fatal(err)
				}
				if tc.path == "account" {
					// A second real account command changes the accepted fingerprint while the provider remains the same.
					if err := manager.Shutdown(t.Context()); err != nil {
						t.Fatal(err)
					}
					accountDiagnostics := filepath.Join(t.TempDir(), "second-account.jsonl")
					driverPath := acpmock.RequireDriver(t)
					fixturePath := writeBoundedRebuildAgent(t, true, false)
					provider := resolved.Config.Providers[acpmock.ProviderName]
					provider.Command = acpmock.BuildCommand(
						driverPath,
						fixturePath,
						"rebuild-agent",
						accountDiagnostics,
					)
					resolved.Config.Providers[acpmock.ProviderName] = provider
					resolved.Agents[0].FallbackChain = nil
					manager = newBoundedRebuildManager(t, home, deps, resolved, daemon)
					diagnostics = accountDiagnostics
				}
				if _, err := manager.Resume(t.Context(), active.ID); err != nil {
					t.Fatal(err)
				}
			}
			promptBoundedRebuild(t, manager, active.ID, "continue after rebuild")
			records, err := acpmock.ReadDiagnostics(diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			prompts := acpmock.PromptDiagnostics(records)
			if len(prompts) == 0 {
				t.Fatal("no completed ACP prompts")
			}
			received := prompts[len(prompts)-1].Prompt
			if tc.native && tc.path == "resume" {
				if strings.Contains(received, "<compozy_context_replay>") {
					t.Fatal("native load received replay")
				}
				if got, ok := manager.Get(active.ID); !ok || got.Info().ACPSessionID != originalACP {
					t.Fatal("native load did not retain ACP session identity")
				}
				loaded := false
				for _, record := range records {
					loaded = loaded || record.LifecycleEvent == "session_load"
				}
				if !loaded {
					t.Fatal("no actual session/load request")
				}
			} else {
				assertBoundedRebuildPrompt(t, received, active.ID)
				current, ok := manager.Get(active.ID)
				if !ok || current.Info().ACPSessionID == "" {
					t.Fatal("rebuild did not bind a new runtime")
				}
				if tc.path == "replace" && current.Info().Provider != "acpmock-replacement" {
					t.Fatal("replacement did not change provider")
				}
				if tc.path == "account" {
					for _, record := range records {
						if record.LifecycleEvent == "session_load" {
							t.Fatal("foreign account received native load")
						}
					}
				}
			}
		})
	}
}

// IT-020 owns durable history and lineage under provider pressure updates.
func TestDaemonBoundedRebuildPressure(t *testing.T) {
	t.Run("Should preserve raw history and the root session through ten high usage turns", func(t *testing.T) {
		home, deps, resolved, diagnostics, daemon := newBoundedRebuildFixture(t, false, true)
		manager := newBoundedRebuildManager(t, home, deps, resolved, daemon)
		active, err := manager.Create(
			t.Context(),
			session.CreateOpts{AgentName: "rebuild-agent", Workspace: resolved.ID},
		)
		if err != nil {
			t.Fatal(err)
		}
		for turn := range 10 {
			stream, err := manager.Prompt(t.Context(), active.ID, fmt.Sprintf("pressure turn %d", turn))
			if err != nil {
				t.Fatal(err)
			}
			observed := false
			for event := range stream {
				if event.Type == acp.EventTypeError {
					t.Fatalf("pressure prompt failed: %#v", event)
				}
				if event.Type == acp.EventTypeUsage && event.Usage != nil && event.Usage.ContextUsed != nil &&
					event.Usage.ContextSize != nil {
					if *event.Usage.ContextUsed != 95 || *event.Usage.ContextSize != 100 {
						t.Fatalf("pressure usage = %#v, want used=95 size=100", event.Usage)
					}
					observed = true
				}
			}
			if !observed {
				t.Fatalf("turn %d did not deliver its 95 percent usage update", turn)
			}
		}
		sessions, err := manager.ListAll(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(sessions) != 1 || sessions[0].ID != active.ID {
			t.Fatalf("sessions = %#v, want only the root", sessions)
		}
		events, err := manager.Events(t.Context(), active.ID, store.EventQuery{Archive: store.EventArchiveAll})
		if err != nil {
			t.Fatal(err)
		}
		usageUpdates, userTurns := 0, 0
		for _, event := range events {
			if event.Archived {
				t.Fatalf("event %s was archived", event.ID)
			}
			if event.Type == "session.compaction_fired" {
				t.Fatal("pressure triggered CompozyOS compaction")
			}
			if event.Type == acp.EventTypeUserMessage {
				userTurns++
			}
			if strings.Contains(event.Content, "usage_update") || event.Type == acp.EventTypeUsage {
				usageUpdates++
			}
		}
		if userTurns != 10 {
			t.Fatalf("user turns = %d, want 10", userTurns)
		}
		if usageUpdates < 10 {
			t.Fatalf("usage updates = %d, want at least 10", usageUpdates)
		}
		records, err := acpmock.ReadDiagnostics(diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(acpmock.PromptDiagnostics(records)); got != 10 {
			t.Fatalf("completed prompts = %d, want 10", got)
		}
	})
}

func newBoundedRebuildFixture(
	t *testing.T,
	native, pressure bool,
) (compozyconfig.HomePaths, SessionManagerDeps, workspacepkg.ResolvedWorkspace, string, *Daemon) {
	t.Helper()
	driverPath := acpmock.RequireDriver(t)
	fixturePath := writeBoundedRebuildAgent(t, native, pressure)
	diagnostics := filepath.Join(t.TempDir(), "rebuild-diagnostics.jsonl")
	command := acpmock.BuildCommand(driverPath, fixturePath, "rebuild-agent", diagnostics)
	home := integrationHomePaths(t)
	cfg := testConfig(t, home)
	cfg.Roles.AutoTitle.Enabled = false
	cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(command)
	cfg.Providers["acpmock-replacement"] = acpmock.ProviderConfig(command)
	resolved := newHarnessIntegrationWorkspace(t, home, cfg, filepath.Join(home.HomeDir, "workspace"))
	resolved.Agents = []compozyconfig.AgentDef{
		{
			Name:     "rebuild-agent",
			Provider: acpmock.ProviderName,
			Model:    "rebuild-model",
			Prompt:   "Exercise bounded session rebuilds.",
			Tools:    []string{"compozy__session_history"},
		},
	}
	daemon, deps := bootHarnessPolicyDaemon(t, home, &cfg)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := daemon.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown daemon: %v", err)
		}
	})
	return home, deps, resolved, diagnostics, daemon
}

func newBoundedRebuildManager(
	t *testing.T,
	home compozyconfig.HomePaths,
	deps SessionManagerDeps,
	resolved workspacepkg.ResolvedWorkspace,
	daemon *Daemon,
) *session.Manager {
	t.Helper()
	manager := newHarnessIntegrationManager(
		t,
		home,
		deps,
		resolved,
		session.NewACPDriverAdapter(acp.New(acp.WithProviderPreStarter(daemon.providerPreStarter))),
		session.WithHostedMCPLauncher(deps.HostedMCP),
	)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sessions, err := manager.ListAll(ctx)
		if err != nil {
			t.Errorf("ListAll cleanup: %v", err)
		}
		for _, active := range sessions {
			if _, ok := manager.Get(active.ID); ok {
				if err := manager.Stop(ctx, active.ID); err != nil {
					t.Errorf("Stop cleanup: %v", err)
				}
			}
		}
		if err := manager.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown manager: %v", err)
		}
	})
	return manager
}

func writeBoundedRebuildAgent(t *testing.T, native, pressure bool) string {
	t.Helper()
	steps := []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: "bounded rebuild acknowledged"}}
	if pressure {
		steps = append(
			steps,
			acpmock.Step{Kind: acpmock.StepKindDriverControl, DriverControl: &acpmock.DriverControlStep{
				Action:     acpmock.DriverControlWriteRawJSONRPC,
				RawJSONRPC: `{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"rebuild-agent-session-1","update":{"sessionUpdate":"usage_update","used":95,"size":100}}}`,
			}},
		)
	}
	fixture := acpmock.Fixture{Version: acpmock.FixtureVersion, Agents: []acpmock.AgentFixture{
		{
			Name:        "rebuild-agent",
			Provider:    acpmock.ProviderName,
			LoadSession: new(native),
			ConfigOptions: []acpmock.SessionConfigOptionFixture{{
				ID: "model", Name: "Model", Current: "rebuild-model",
				Values: []acpmock.SessionConfigOptionValueFixture{{Value: "rebuild-model", Label: "Rebuild model"}},
			}},
			Turns: []acpmock.TurnFixture{
				{Name: "user", Match: acpmock.TurnMatch{TurnSource: acp.PromptTurnSourceUser}, Steps: steps},
			},
		},
	}}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bounded-rebuild.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func seedBoundedRebuildTurns(t *testing.T, home compozyconfig.HomePaths, active *session.Session) {
	t.Helper()
	meta := active.Meta()
	owner, err := meta.DatabaseOwner()
	if err != nil {
		t.Fatal(err)
	}
	db, err := sessiondb.OpenSessionDB(
		t.Context(),
		owner,
		store.SessionDBFile(filepath.Join(home.SessionsDir, active.ID)),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(context.Background()); err != nil {
			t.Errorf("Close seed DB: %v", err)
		}
	}()
	events := make([]store.SessionEvent, 0, 800)
	epoch := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for turn := range 400 {
		text := fmt.Sprintf("persisted user turn %03d", turn)
		if turn == 0 {
			text = rebuildFirstUser
		}
		for _, message := range []struct{ kind, text string }{{acp.EventTypeUserMessage, text}, {acp.EventTypeAgentMessage, fmt.Sprintf("answer %03d ", turn) + strings.Repeat("x", 1500)}} {
			events = append(
				events,
				store.SessionEvent{
					ID:        fmt.Sprintf("seed-%d-%s", turn, message.kind),
					SessionID: active.ID,
					TurnID:    fmt.Sprintf("seed-turn-%d", turn),
					Type:      message.kind,
					AgentName: "rebuild-agent",
					Content:   message.text,
					Timestamp: epoch.Add(time.Duration(turn) * time.Second),
				},
			)
		}
	}
	if _, err := db.RecordPersistedBatch(t.Context(), events); err != nil {
		t.Fatal(err)
	}
}

func promptBoundedRebuild(t *testing.T, manager *session.Manager, id, message string) {
	t.Helper()
	events, err := manager.Prompt(t.Context(), id, message)
	if err != nil {
		t.Fatal(err)
	}
	for event := range events {
		if event.Type == acp.EventTypeError {
			t.Fatalf("ACP prompt error: %#v", event)
		}
	}
}

func assertBoundedRebuildPrompt(t *testing.T, prompt, sourceID string) {
	t.Helper()
	_, after, ok := strings.Cut(prompt, "<compozy_context_replay>")
	if !ok {
		t.Fatal("rebuild prompt has no replay block")
	}
	payload, _, ok := strings.Cut(after, "</compozy_context_replay>")
	if !ok {
		t.Fatal("rebuild replay block is not closed")
	}
	payload = strings.TrimSpace(payload)
	if len(payload) > compozyconfig.DefaultSessionDeriveMaxReplayBytes {
		t.Fatalf("replay bytes = %d, want <= 131072", len(payload))
	}
	var messages []transcript.Message
	if err := json.Unmarshal([]byte(payload), &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages) < 2 || messages[0].Role != transcript.RoleUser || messages[0].Content != rebuildFirstUser {
		t.Fatal("replay did not pin the first user message")
	}
	if !strings.Contains(messages[1].Content, "earlier messages omitted to fit the context budget") {
		t.Fatal("replay has no omission note")
	}
	if !strings.Contains(prompt, "The files and git state in the workspace are authoritative;") {
		t.Fatal("replay has no workspace authority line")
	}
	pointer := "Earlier messages were omitted. Read them with the compozy__session_history tool (session_id: " + sourceID + ") when you need them."
	if !strings.Contains(prompt, pointer) {
		t.Fatal("replay has no source history pointer")
	}
}

func assertCompactionRequestSource(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID, promptID, source string,
) {
	t.Helper()
	rows, err := harness.SessionEvents(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	requested := 0
	for _, row := range rows.Events {
		if row.Type != "session.compaction.requested" || row.TurnID != promptID {
			continue
		}
		requested++
		decoded, err := transcript.UnmarshalAgentEvent(string(row.Content))
		if err != nil {
			t.Fatal(err)
		}
		var requestFact struct {
			RequestedBy string `json:"requested_by"`
		}
		if err := json.Unmarshal(decoded.Raw, &requestFact); err != nil {
			t.Fatal(err)
		}
		if requestFact.RequestedBy != source {
			t.Fatalf("requested_by = %q, want %q", requestFact.RequestedBy, source)
		}
	}
	if requested != 1 {
		t.Fatalf("requested facts = %d, want 1", requested)
	}
}

func assertMaintenanceCommandDiagnostics(t *testing.T, harness *e2etest.RuntimeHarness, agent string) {
	t.Helper()
	registration, ok := harness.MockAgentRegistration(agent)
	if !ok {
		t.Fatal("mock registration missing")
	}
	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatal(err)
	}
	prompts := acpmock.PromptDiagnostics(records)
	if len(prompts) != 8 {
		t.Fatalf("received prompts = %#v, want four baseline and four compact commands", prompts)
	}
	for _, index := range []int{1, 3, 5, 7} {
		if prompts[index].Prompt != "/compact" {
			t.Fatalf("received prompt[%d] = %q, want literal compact command", index, prompts[index].Prompt)
		}
	}
}

func assertReducedMaintenanceReplay(t *testing.T, prompt string, records []acpmock.DiagnosticsRecord) {
	t.Helper()
	_, fenced, found := strings.Cut(prompt, "<compozy_context_replay>")
	if !found {
		t.Fatal("ordinary continuation has no deferred transcript")
	}
	payload, _, found := strings.Cut(fenced, "</compozy_context_replay>")
	if !found || len(strings.TrimSpace(payload)) > 8192 {
		t.Fatalf("delivered replay array bytes = %d, want <= 8192", len(strings.TrimSpace(payload)))
	}
	var messages []transcript.Message
	if err := json.Unmarshal([]byte(payload), &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages) == 0 || messages[0].Content != rebuildFirstUser {
		t.Fatal("reduced replay lost the original request")
	}
	for _, message := range messages {
		encoded, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded) > 4096 || strings.Contains(message.Content, "Native compaction observed.") ||
			strings.Contains(message.Content, "Retain the agreed project constraints.") ||
			message.Content == "/compact" {
			t.Fatalf("replay crossed its current per-message budget or maintenance cut: %s", encoded)
		}
	}
	if strings.Contains(prompt, "Read them with the compozy__session_history tool") {
		t.Fatal("replay advertises an unavailable effective history tool")
	}
	creates := 0
	for _, record := range records {
		if record.LifecycleEvent == "session_new" {
			creates++
		}
	}
	if creates != 3 {
		t.Fatalf("session/new calls = %d, want initial, replacement and load-unsupported fallback", creates)
	}
}
