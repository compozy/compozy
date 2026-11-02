//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	mentionIntegrationAgent    = "mention-agent"
	mentionIntegrationHoldText = "hold the turn"
	// mentionDeniedHistoryAgent runs the same fixture with an operator-authored deny_tools
	// entry for compozy__session_history.
	mentionDeniedHistoryAgent = "mention-denied-agent"
)

// mentionIntegrationDaemon is a booted daemon whose session manager carries the real
// session-references renderer and native tool registry.
type mentionIntegrationDaemon struct {
	daemon      *Daemon
	manager     *session.Manager
	workspace   string
	root        string
	diagnostics string
}

func newMentionIntegrationDaemon(t *testing.T, toolsEnabled bool) mentionIntegrationDaemon {
	t.Helper()
	home := testHomePaths(t)
	fixture := acpmock.Fixture{
		Version: 2,
		Agents: []acpmock.AgentFixture{{
			Name:        mentionIntegrationAgent,
			Provider:    acpmock.ProviderName,
			Permissions: "approve-all",
			Prompt:      "Acknowledge every prompt.",
			Turns: []acpmock.TurnFixture{
				{
					Name:  "hold",
					Match: acpmock.TurnMatch{UserText: mentionIntegrationHoldText},
					Steps: []acpmock.Step{
						{
							Kind: acpmock.StepKindDriverControl,
							DriverControl: &acpmock.DriverControlStep{
								Action:  acpmock.DriverControlDelay,
								DelayMS: 1500,
							},
						},
						{Kind: acpmock.StepKindAssistant, Text: "held"},
					},
				},
				{
					Name:  "acknowledge",
					Match: acpmock.TurnMatch{TurnSource: acp.PromptTurnSourceUser},
					Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: "acknowledged"}},
				},
			},
		}},
	}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fixturePath := filepath.Join(home.HomeDir, "mentions-fixture.json")
	if err := os.WriteFile(fixturePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	diagnostics := filepath.Join(t.TempDir(), "mentions-diagnostics.jsonl")
	registration, err := acpmock.Register(home, acpmock.RegisterOptions{
		FixturePath:     fixturePath,
		FixtureAgent:    mentionIntegrationAgent,
		ProviderName:    acpmock.ProviderName,
		DiagnosticsPath: diagnostics,
	})
	if err != nil {
		t.Fatal(err)
	}
	registerMentionDeniedHistoryAgent(t, home, fixturePath, diagnostics)
	cfg := e2etest.SeedConfig(t, home, e2etest.ConfigSeedOptions{
		SocketPath:      home.DaemonSocket,
		DefaultAgent:    mentionIntegrationAgent,
		DefaultProvider: acpmock.ProviderName,
		Providers: map[string]compozyconfig.ProviderConfig{
			acpmock.ProviderName: acpmock.ProviderConfig(registration.Command),
		},
		PermissionMode: compozyconfig.PermissionModeApproveAll,
		Mutate: func(cfg *compozyconfig.Config) {
			cfg.Automation.Enabled = false
			cfg.Roles.AutoTitle.Enabled = false
			cfg.ModelCatalog.Sources.ModelsDev.Enabled = new(false)
		},
	})
	d := newTestDaemon(t, home, &cfg)
	if err := d.boot(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})
	root := t.TempDir()
	if !toolsEnabled {
		// Workspace config disables tools for its sessions (reason source_disabled) while the
		// daemon keeps hosted MCP for agents that declare tools.
		writeDaemonFile(t, filepath.Join(root, compozyconfig.DirName, "config.toml"), "[tools]\nenabled = false\n")
	}
	workspace, err := d.workspaceResolver.ResolveOrRegister(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	manager, ok := d.sessions.(*session.Manager)
	if !ok {
		t.Fatal("real session manager missing")
	}
	return mentionIntegrationDaemon{
		daemon: d, manager: manager, workspace: workspace.ID, root: root, diagnostics: diagnostics,
	}
}

// registerMentionDeniedHistoryAgent writes a second agent for the shared fixture whose
// definition denies compozy__session_history, the way an operator narrows an agent.
func registerMentionDeniedHistoryAgent(
	t *testing.T,
	home compozyconfig.HomePaths,
	fixturePath string,
	diagnostics string,
) {
	t.Helper()
	if _, err := acpmock.Register(home, acpmock.RegisterOptions{
		FixturePath:     fixturePath,
		FixtureAgent:    mentionIntegrationAgent,
		AgentName:       mentionDeniedHistoryAgent,
		ProviderName:    acpmock.ProviderName,
		DiagnosticsPath: diagnostics,
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home.AgentsDir, mentionDeniedHistoryAgent, "AGENT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header, body, ok := strings.Cut(strings.TrimPrefix(string(data), "---\n"), "---\n")
	if !ok {
		t.Fatalf("agent definition %s lacks frontmatter", path)
	}
	denied := "---\n" + header + "deny_tools:\n  - 'compozy__session_history'\n---\n" + body
	if err := os.WriteFile(path, []byte(denied), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (h mentionIntegrationDaemon) create(t *testing.T, name string) *session.Session {
	t.Helper()
	return h.createWithAgent(t, mentionIntegrationAgent, name)
}

func (h mentionIntegrationDaemon) createWithAgent(t *testing.T, agentName, name string) *session.Session {
	t.Helper()
	created, err := h.manager.Create(t.Context(), new(session.CreateOpts{
		AgentName: agentName, Workspace: h.workspace, Name: name,
	}))
	if err != nil {
		t.Fatal(err)
	}
	return created
}

// send admits one prompt and drains its turn when it dispatches immediately.
func (h mentionIntegrationDaemon) send(
	t *testing.T,
	sessionID string,
	message string,
	caller session.PromptCaller,
	mode session.BusyInputMode,
) session.SendPromptResult {
	t.Helper()
	result, err := h.manager.SendPrompt(t.Context(), sessionID, session.SendPromptOpts{
		Message: message, Caller: caller, Mode: mode,
	})
	if err != nil {
		t.Fatalf("SendPrompt(%s) error = %v", sessionID, err)
	}
	if result.Events != nil {
		for range result.Events {
			continue
		}
	}
	return result
}

// lastMentions waits for the delivery manifest count of sessionID to reach want and
// returns the newest manifest mentions plus the text the agent received last.
func (h mentionIntegrationDaemon) lastDelivery(
	t *testing.T,
	sessionID string,
	want int,
) (*acp.DeliveredMentions, string) {
	t.Helper()
	var deliveries []session.DeliveryEventEnvelope
	waitForRuntimeCondition(t, "prompt delivery for "+sessionID, 30*time.Second, func() bool {
		var err error
		deliveries, err = h.manager.Deliveries(t.Context(), sessionID)
		return err == nil && len(deliveries) >= want
	})
	records, err := acpmock.ReadDiagnostics(h.diagnostics)
	if err != nil {
		t.Fatalf("ReadDiagnostics() error = %v", err)
	}
	prompt := ""
	for _, record := range acpmock.DiagnosticsForCompozySession(records, sessionID) {
		if record.Prompt != "" {
			prompt = record.Prompt
		}
	}
	return deliveries[len(deliveries)-1].Manifest.Mentions, prompt
}

func humanMentionCaller() session.PromptCaller {
	return session.PromptCaller{Kind: "human", ID: "operator", Source: "test"}
}

func agentMentionCaller(id string) session.PromptCaller {
	return session.PromptCaller{Kind: "agent", ID: id, Source: "test"}
}

func assertMentionReferencesDelivery(
	t *testing.T,
	mentions *acp.DeliveredMentions,
	form session.MentionReferenceForm,
	sessionIDs []string,
) {
	t.Helper()
	if mentions == nil || mentions.ReferencesForm != string(form) || mentions.Sessions != len(sessionIDs) {
		t.Fatalf("manifest mentions = %#v, want form %s with %d sessions", mentions, form, len(sessionIDs))
	}
	if len(mentions.Blocks) == 0 || mentions.Blocks[0].Kind != acp.DeliveredMentionBlockSessionReferences ||
		!slices.Equal(mentions.Blocks[0].SessionIDs, sessionIDs) {
		t.Fatalf("manifest mention blocks = %#v, want references block for %v", mentions.Blocks, sessionIDs)
	}
}

// IT-002: compozy__session_prompt from session C to session B with @session:A delivers the
// reference form when B can call compozy__session_history.
func TestSessionMentionReferencesNativePromptIT002(t *testing.T) {
	acpmock.RequireDriver(t)
	h := newMentionIntegrationDaemon(t, true)
	source := h.create(t, "Fix login bug")
	target := h.create(t, "Review")
	caller := h.create(t, "Coordinator")

	t.Run("Should deliver the reference form for an agent-origin native prompt", func(t *testing.T) {
		input, err := json.Marshal(map[string]any{
			"session_id": target.ID, "message": "Compare with @session:" + source.ID,
			"message_id": "msg-it002", "idempotency_key": "idem-it002", "wait": true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.daemon.toolRegistry.Call(t.Context(), toolspkg.Scope{
			SessionID: caller.ID, WorkspaceID: h.workspace, AgentName: mentionIntegrationAgent,
			ProfileID: caller.ProfileID,
		}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDSessionPrompt, ToolCallID: "call-it002", Input: input,
		}); err != nil {
			t.Fatalf("compozy__session_prompt error = %v", err)
		}

		mentions, prompt := h.lastDelivery(t, target.ID, 1)
		assertMentionReferencesDelivery(t, mentions, session.MentionReferenceFormReference, []string{source.ID})
		if !strings.Contains(prompt, "you MUST call compozy__session_history with its session_id") ||
			!strings.Contains(prompt, "@session:"+source.ID+` · title "Fix login bug"`) {
			t.Fatalf("delivered references block = %q, want the reference form for %s", prompt, source.ID)
		}
	})
}

// IT-003: with tools disabled for the receiving session, a human prompt inlines the referenced
// session's history and an agent-origin prompt gets the unavailable form without history.
func TestSessionMentionReferencesToolsDisabledIT003(t *testing.T) {
	acpmock.RequireDriver(t)
	h := newMentionIntegrationDaemon(t, false)
	source := h.create(t, "Fix login bug")
	h.send(t, source.ID, "The login form returns 500 after refresh.", humanMentionCaller(), "")
	if _, prompt := h.lastDelivery(t, source.ID, 1); prompt == "" {
		t.Fatal("source session history prompt was not delivered")
	}

	t.Run("Should deny session history with source_disabled in the tools-disabled workspace", func(t *testing.T) {
		registry, ok := h.daemon.toolRegistry.(mentionToolDecisionRegistry)
		if !ok {
			t.Fatal("daemon tool registry lacks diagnostic decisions")
		}
		view, err := registry.DiagnosticGet(t.Context(), toolspkg.Scope{
			ProfileID: source.ProfileID, WorkspaceID: h.workspace, SessionID: source.ID,
			AgentName: mentionIntegrationAgent,
		}, toolspkg.ToolIDSessionHistory)
		if err != nil {
			t.Fatalf("DiagnosticGet() error = %v", err)
		}
		if reasons := view.Decision.ReasonCodes; view.Decision.Callable || len(reasons) != 1 ||
			reasons[0] != toolspkg.ReasonSourceDisabled {
			t.Fatalf("session history decision = %#v, want denied by source_disabled", view.Decision)
		}
	})

	t.Run("Should inline recent history for a human prompt", func(t *testing.T) {
		target := h.create(t, "Human review")
		h.send(t, target.ID, "Compare with @session:"+source.ID, humanMentionCaller(), "")

		mentions, prompt := h.lastDelivery(t, target.ID, 1)
		assertMentionReferencesDelivery(t, mentions, session.MentionReferenceFormInline, []string{source.ID})
		if !strings.Contains(prompt, "<recent_history>") ||
			!strings.Contains(prompt, "[user] The login form returns 500 after refresh.") ||
			!strings.Contains(prompt, "[assistant] acknowledged") {
			t.Fatalf("delivered references block = %q, want the source history inline", prompt)
		}
	})

	t.Run("Should withhold history from an agent-origin prompt", func(t *testing.T) {
		target := h.create(t, "Agent review")
		h.send(t, target.ID, "Compare with @session:"+source.ID, agentMentionCaller("coordinator"), "")

		mentions, prompt := h.lastDelivery(t, target.ID, 1)
		assertMentionReferencesDelivery(t, mentions, session.MentionReferenceFormUnavailable, []string{source.ID})
		if !strings.Contains(prompt, "Session history is not available to you; ask the operator if you need it.") ||
			strings.Contains(prompt, "<recent_history>") || strings.Contains(prompt, "returns 500") {
			t.Fatalf("delivered references block = %q, want the unavailable form without history", prompt)
		}
	})
}

// IT-003 (policy half): when the receiving agent's tool policy denies compozy__session_history,
// even a human prompt gets the unavailable form and no history.
func TestSessionMentionReferencesPolicyDeniedIT003(t *testing.T) {
	acpmock.RequireDriver(t)
	h := newMentionIntegrationDaemon(t, true)
	source := h.create(t, "Fix login bug")
	h.send(t, source.ID, "The login form returns 500 after refresh.", humanMentionCaller(), "")
	if _, prompt := h.lastDelivery(t, source.ID, 1); prompt == "" {
		t.Fatal("source session history prompt was not delivered")
	}
	target := h.createWithAgent(t, mentionDeniedHistoryAgent, "Denied review")

	t.Run("Should deny session history with policy_denied for the deny-listed agent", func(t *testing.T) {
		registry, ok := h.daemon.toolRegistry.(mentionToolDecisionRegistry)
		if !ok {
			t.Fatal("daemon tool registry lacks diagnostic decisions")
		}
		view, err := registry.DiagnosticGet(t.Context(), toolspkg.Scope{
			ProfileID: target.ProfileID, WorkspaceID: h.workspace, SessionID: target.ID,
			AgentName: mentionDeniedHistoryAgent,
		}, toolspkg.ToolIDSessionHistory)
		if err != nil {
			t.Fatalf("DiagnosticGet() error = %v", err)
		}
		if reasons := view.Decision.ReasonCodes; view.Decision.Callable || len(reasons) != 1 ||
			reasons[0] != toolspkg.ReasonPolicyDenied {
			t.Fatalf("session history decision = %#v, want denied by policy_denied", view.Decision)
		}
	})

	t.Run("Should deliver the unavailable form without history to a human prompt", func(t *testing.T) {
		h.send(t, target.ID, "Compare with @session:"+source.ID, humanMentionCaller(), "")

		mentions, prompt := h.lastDelivery(t, target.ID, 1)
		assertMentionReferencesDelivery(t, mentions, session.MentionReferenceFormUnavailable, []string{source.ID})
		if !strings.Contains(prompt, "Session history is not available to you; ask the operator if you need it.") ||
			strings.Contains(prompt, "<recent_history>") || strings.Contains(prompt, "returns 500") ||
			strings.Contains(prompt, "[assistant] acknowledged") {
			t.Fatalf("delivered references block = %q, want the unavailable form without history", prompt)
		}
	})
}

// IT-004 (daemon half): a queued session mention whose target is deleted before dispatch is left
// out of the references block, and the manifest counts only the sessions still rendered.
func TestSessionMentionReferencesQueuedDeletedTargetIT004(t *testing.T) {
	acpmock.RequireDriver(t)
	h := newMentionIntegrationDaemon(t, true)
	gone := h.create(t, "Deleted later")
	kept := h.create(t, "Kept")
	target := h.create(t, "Busy target")

	t.Run("Should drop the deleted session from the dispatched block", func(t *testing.T) {
		held, err := h.manager.SendPrompt(t.Context(), target.ID, session.SendPromptOpts{
			Message: mentionIntegrationHoldText, Caller: humanMentionCaller(),
		})
		if err != nil {
			t.Fatal(err)
		}
		queued := h.send(
			t, target.ID, "Compare @session:"+gone.ID+" with @session:"+kept.ID,
			humanMentionCaller(), session.BusyInputModeQueue,
		)
		if queued.QueueEntryID == "" {
			t.Fatalf("queued prompt result = %#v, want a queue entry while the turn is busy", queued)
		}
		if err := h.manager.Delete(t.Context(), gone.ID); err != nil {
			t.Fatalf("Delete(%s) error = %v", gone.ID, err)
		}
		for range held.Events {
			continue
		}

		mentions, prompt := h.lastDelivery(t, target.ID, 2)
		assertMentionReferencesDelivery(t, mentions, session.MentionReferenceFormReference, []string{kept.ID})
		if strings.Contains(prompt, gone.ID) || !strings.Contains(prompt, "@session:"+kept.ID) {
			t.Fatalf("delivered references block = %q, want only %s", prompt, kept.ID)
		}
	})
}
