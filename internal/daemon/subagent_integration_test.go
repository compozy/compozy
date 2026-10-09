//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// Invariant: real daemon wiring reserves, launches ACP, persists a result and delivers one wake.
// Owner: daemon orchestration; canonical suite for the new subagent journey (IT-001/IT-013 queue and replay halves).
func TestSubagentDaemonIntegration(t *testing.T) {
	// IT-018: boot must publish the subagent service to both public transports.
	t.Run("Should expose subagent routes over HTTP and UDS after boot", func(t *testing.T) {
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "native_tool_delegate_fixture.json"),
				FixtureAgent: "subagent-delegator", AgentName: "subagent-delegator",
			}},
		})
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		parent := createFixtureBackedSession(t, ctx, harness, "subagent-delegator", "subagent route boot")
		path := "/api/workspaces/" + harness.WorkspaceID + "/sessions/" + parent.ID + "/subagents"
		for _, transport := range []struct {
			name   string
			client *http.Client
			target string
		}{
			{name: "HTTP", client: harness.HTTPClient, target: harness.HTTPURL(path)},
			{name: "UDS", client: harness.UDSClient, target: harness.UDSURL(path)},
		} {
			t.Run("Should return OK over "+transport.name, func(t *testing.T) {
				request, err := http.NewRequestWithContext(ctx, http.MethodGet, transport.target, nil)
				if err != nil {
					t.Fatal(err)
				}
				response, err := transport.client.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				body, readErr := io.ReadAll(response.Body)
				closeErr := response.Body.Close()
				if readErr != nil || closeErr != nil {
					t.Fatalf("read response: %v; close: %v", readErr, closeErr)
				}
				if response.StatusCode != http.StatusOK {
					t.Fatalf("GET subagents over %s = %d, want 200: %s", transport.name, response.StatusCode, body)
				}
			})
		}
	})
	t.Run("Should deliver one prioritized wake across providers", func(t *testing.T) {
		d, manager, workspace := newSubagentDaemonIntegration(t)
		ctx := t.Context()
		parent, err := manager.Create(ctx, session.CreateOpts{AgentName: "subagent-test", Workspace: workspace})
		if err != nil {
			t.Fatal(err)
		}
		// IT-027: inactive callers are rejected before capability discovery.
		if _, err := d.SubagentService().
			Capabilities(ctx, session.SubagentCaller{WorkspaceID: workspace, SessionID: parent.ID}); !errors.Is(
			err,
			session.ErrSubagentParentNotActive,
		) {
			t.Fatal(err)
		}
		prompt, err := manager.SendPrompt(ctx, parent.ID, session.SendPromptOpts{Message: "hold parent"})
		if err != nil {
			t.Fatal(err)
		}
		drained := make(chan struct{})
		go func() {
			for range prompt.Events {
				continue
			}
			close(drained)
		}()
		waitForRuntimeCondition(
			t,
			"parent ACP turn",
			10*time.Second,
			func() bool { return parent.IsPrompting() && parent.CurrentTurnID() != "" },
		)
		caller := session.SubagentCaller{
			WorkspaceID: parent.Info().WorkspaceID,
			SessionID:   parent.ID,
			TurnID:      parent.CurrentTurnID(),
			ToolCallID:  "delegate-one",
		}
		capabilities, err := d.SubagentService().Capabilities(ctx, caller)
		if err != nil || capabilities.ParentSessionID != parent.ID ||
			capabilities.Inherited.Provider != acpmock.ProviderName ||
			capabilities.Depth != 0 {
			t.Fatal(capabilities, err)
		}
		available := false
		for _, provider := range capabilities.Providers {
			if provider.Provider == acpmock.ProviderName {
				available = provider.CanDelegate
			}
		}
		if !available {
			t.Fatal("fixture provider is not available for delegation")
		}
		// IT-007: older user inputs retain FIFO but a subsequent wake dispatches first.
		for _, text := range []string{"user one", "user two"} {
			queued, err := manager.SendPrompt(
				ctx,
				parent.ID,
				session.SendPromptOpts{Message: text, Mode: session.BusyInputModeQueue},
			)
			if err != nil {
				t.Fatal(err)
			}
			if queued.Events != nil {
				go func() {
					for range queued.Events {
						continue
					}
				}()
			}
		}
		req := session.SubagentRequest{
			Caller: caller,
			Task:   "child work",
			Title:  "Child work",
			Target: session.SubagentTarget{Provider: "acpmock-other"},
		}
		row, err := d.SubagentService().Delegate(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		again, err := d.SubagentService().Delegate(ctx, req)
		if err != nil || again.ID != row.ID || again.ChildSessionID == nil ||
			*again.ChildSessionID != *row.ChildSessionID {
			t.Fatal(again, err)
		}
		waitForRuntimeCondition(t, "durable child result", 15*time.Second, func() bool {
			current, err := d.SubagentService().Get(ctx, caller.WorkspaceID, row.ID)
			return err == nil && current.Status == "completed" && current.Result != nil &&
				*current.Result == "child answer" &&
				current.Delivery == "claimed"
		})
		db := d.registry.(store.SubagentStore)
		durable, err := db.GetSubagent(ctx, caller.WorkspaceID, row.ID)
		if err != nil || durable.RuntimeProvider != "acpmock-other" || durable.PendingTask != nil ||
			durable.Delivery != "claimed" ||
			durable.WakeMessageID == nil {
			t.Fatal(durable, err)
		}
		child, err := manager.Status(ctx, *row.ChildSessionID)
		if err != nil || child.Lineage == nil || child.Lineage.TTLExpiresAt != nil || child.Lineage.NotifyCreator ||
			child.LastSettledRevision > child.LastSeenRevision {
			t.Fatal(child, err)
		}
		// IT-016: sweep real sessions with a clock beyond any ordinary spawn TTL.
		reaper := &spawnReaper{
			sessions: manager,
			logger:   discardLogger(),
			now:      func() time.Time { return time.Now().Add(365 * 24 * time.Hour) },
		}
		report, err := reaper.Sweep(ctx)
		if err != nil || report.Reaped != 0 {
			t.Fatal(report, err)
		}
		// IT-005/IT-006: joins retain one wake; status prunes only acknowledged members.
		rows := []session.Subagent{row}
		for _, key := range []string{"delegate-two", "delegate-three"} {
			next := req
			next.IdempotencyKey = key
			added, err := d.SubagentService().Delegate(ctx, next)
			if err != nil {
				t.Fatal(err)
			}
			rows = append(rows, added)
			waitForRuntimeCondition(t, "joined child settled", 15*time.Second, func() bool {
				current, err := db.GetSubagent(ctx, workspace, added.ID)
				return err == nil && current.Delivery == "claimed"
			})
		}
		for _, member := range rows[:2] {
			acknowledged, err := d.SubagentService().Status(ctx, caller, member.ID)
			if err != nil || acknowledged.Delivery != "acknowledged" {
				t.Fatal(acknowledged, err)
			}
		}
		wake, members, err := db.GetWake(ctx, *durable.WakeMessageID)
		if err != nil || len(members) != 1 || members[0].ID != rows[2].ID {
			t.Fatal(wake, members, err)
		}
		queue, err := d.registry.(store.SessionInputQueueStore).ListPendingSessionInputs(ctx, parent.ID)
		if err != nil || len(queue) != 3 || queue[0].Priority != 1 || strings.Contains(queue[0].Text, rows[0].ID) ||
			!strings.Contains(queue[0].Text, rows[2].ID) {
			t.Fatal(queue, err)
		}
		row = rows[2]
		if _, err := manager.CancelPromptWithCause(ctx, parent.ID, session.PromptCancelSyntheticAdmission); err != nil {
			t.Fatal(err)
		}
		select {
		case <-drained:
		case <-time.After(10 * time.Second):
			t.Fatal("parent prompt did not finish")
		}
		waitForRuntimeCondition(t, "wake delivered", 15*time.Second, func() bool {
			current, err := db.GetSubagent(ctx, caller.WorkspaceID, row.ID)
			return err == nil && current.Delivery == "delivered"
		})
		waitForRuntimeCondition(t, "user queue drained", 15*time.Second, func() bool {
			inputs, err := d.registry.(store.SessionInputQueueStore).ListPendingSessionInputs(ctx, parent.ID)
			return err == nil && len(inputs) == 0 && !parent.IsPrompting()
		})
		events, err := manager.Events(ctx, parent.ID, store.EventQuery{Limit: 200})
		if err != nil {
			t.Fatal(err)
		}
		wakeCount := 0
		var dispatchOrder []string
		for _, event := range events {
			if event.Type == acp.EventTypeSyntheticReentry {
				wakeCount++
				dispatchOrder = append(dispatchOrder, "wake")
			}
			if event.Type == acp.EventTypeUserMessage {
				decoded, err := transcript.UnmarshalAgentEvent(event.Content)
				if err != nil {
					t.Fatal(err)
				}
				if decoded.Text == "user one" || decoded.Text == "user two" {
					dispatchOrder = append(dispatchOrder, decoded.Text)
				}
			}
		}
		if strings.Join(dispatchOrder, ",") != "wake,user one,user two" {
			t.Fatal(dispatchOrder)
		}
		if wakeCount != 1 {
			t.Fatalf("synthetic wake count=%d", wakeCount)
		}
		page, err := d.SubagentService().
			List(ctx, store.SubagentListQuery{WorkspaceID: caller.WorkspaceID, ParentSessionID: parent.ID, Limit: 20})
		if err != nil || len(page.Items) != 3 {
			t.Fatal(page, err)
		}
	})
}

func newSubagentDaemonIntegration(t *testing.T, steer ...string) (*Daemon, *session.Manager, string) {
	t.Helper()
	return newSubagentDaemonConfigured(t, nil, steer...)
}

func newSubagentDaemonConfigured(
	t *testing.T,
	mutate func(*compozyconfig.Config),
	steer ...string,
) (*Daemon, *session.Manager, string) {
	t.Helper()
	home := testHomePaths(t)
	fixture := acpmock.Fixture{
		Version: 2,
		Agents: []acpmock.AgentFixture{
			{
				Name:        "subagent-test",
				Provider:    acpmock.ProviderName,
				Permissions: "approve-all",
				Prompt:      "Complete the task.",
				Turns: []acpmock.TurnFixture{
					{
						Name:  "parent",
						Match: acpmock.TurnMatch{UserText: "hold parent"},
						Steps: []acpmock.Step{
							{Kind: acpmock.StepKindThought, Text: "parent ready"},
							{
								Kind: acpmock.StepKindDriverControl,
								DriverControl: &acpmock.DriverControlStep{
									Action: acpmock.DriverControlBlockUntilCancel,
								},
							},
						},
					},
					{
						Name:  "child",
						Match: acpmock.TurnMatch{UserText: "child work"},
						Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: "child answer"}},
					},
					{
						Name:  "wake",
						Match: acpmock.TurnMatch{TurnSource: acp.PromptTurnSourceSynthetic},
						Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: "wake received"}},
					},
				},
			},
		},
	}
	fixture.Agents[0].Turns = append(
		fixture.Agents[0].Turns,
		acpmock.TurnFixture{Name: "slow-child", Match: acpmock.TurnMatch{UserText: "slow child"}, Steps: []acpmock.Step{
			{
				Kind:          acpmock.StepKindDriverControl,
				DriverControl: &acpmock.DriverControlStep{Action: acpmock.DriverControlDelay, DelayMS: 3000},
			},
			{Kind: acpmock.StepKindAssistant, Text: "slow answer"},
		}},
		acpmock.TurnFixture{
			Name:  "approval-child",
			Match: acpmock.TurnMatch{UserText: "approval child"},
			Steps: []acpmock.Step{
				{
					Kind:           acpmock.StepKindPermission,
					ToolCallID:     "approval",
					Title:          "Edit fixture",
					ToolKind:       "edit",
					Path:           "review.txt",
					ExpectDecision: "allow-once",
				},
				{Kind: acpmock.StepKindAssistant, Text: "approved answer"},
			},
		},
	)
	for _, text := range []string{"user one", "user two"} {
		fixture.Agents[0].Turns = append(
			fixture.Agents[0].Turns,
			acpmock.TurnFixture{
				Name:  text,
				Match: acpmock.TurnMatch{UserText: text},
				Steps: []acpmock.Step{{Kind: acpmock.StepKindAssistant, Text: "processed " + text}},
			},
		)
	}
	if len(steer) > 0 {
		fixture.Agents[0].SteerOutcome = steer[0]
	}
	nativeData, err := os.ReadFile(filepath.Join("..", "acp", "testdata", "claude_agent_subagent.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	// Real providers may announce a generic call before streaming its description.
	first, rest, _ := strings.Cut(strings.TrimSpace(string(nativeData)), "\n")
	initial := strings.Replace(first, `"title":"Review the diff (high effort)"`, `"title":"Task"`, 1)
	initial = strings.Replace(initial, `"description":"Review the diff (high effort)",`, "", 1)
	update := strings.Replace(first, `"sessionUpdate":"tool_call"`, `"sessionUpdate":"tool_call_update"`, 1)
	nativeData = []byte(initial + "\n" + update + "\n" + rest)
	nativeSteps := []acpmock.Step{}
	for frame := range strings.SplitSeq(strings.TrimSpace(string(nativeData)), "\n") {
		nativeSteps = append(
			nativeSteps,
			acpmock.Step{
				Kind: acpmock.StepKindDriverControl,
				DriverControl: &acpmock.DriverControlStep{
					Action:     acpmock.DriverControlWriteRawJSONRPC,
					RawJSONRPC: strings.ReplaceAll(frame, "session-native", "subagent-test-session-1"),
				},
			},
		)
	}
	fixture.Agents[0].Turns = append(
		fixture.Agents[0].Turns,
		acpmock.TurnFixture{Name: "native", Match: acpmock.TurnMatch{UserText: "native work"}, Steps: nativeSteps},
	)
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	fixturePath := filepath.Join(home.HomeDir, "subagents-fixture.json")
	if err := os.WriteFile(fixturePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	registration, err := acpmock.Register(
		home,
		acpmock.RegisterOptions{
			FixturePath:  fixturePath,
			FixtureAgent: "subagent-test",
			ProviderName: acpmock.ProviderName,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	cfg := e2etest.SeedConfig(
		t,
		home,
		e2etest.ConfigSeedOptions{
			SocketPath:      home.DaemonSocket,
			DefaultAgent:    "subagent-test",
			DefaultProvider: acpmock.ProviderName,
			Providers: map[string]compozyconfig.ProviderConfig{
				acpmock.ProviderName: acpmock.ProviderConfig(registration.Command),
				"acpmock-other":      acpmock.ProviderConfig(registration.Command),
			},
			PermissionMode: compozyconfig.PermissionModeApproveAll,
			Mutate: func(cfg *compozyconfig.Config) {
				cfg.Automation.Enabled = false
				cfg.Roles.AutoTitle.Enabled = false
				cfg.ModelCatalog.Sources.ModelsDev.Enabled = new(false)
				if mutate != nil {
					mutate(cfg)
				}
			},
		},
	)
	// SeedConfig intentionally persists a small overlay; this journey also owns hook declarations.
	if len(cfg.Hooks.Declarations) > 0 {
		declarations := make([]map[string]any, 0, len(cfg.Hooks.Declarations))
		for _, hook := range cfg.Hooks.Declarations {
			declarations = append(
				declarations,
				map[string]any{
					"name":    hook.Name,
					"event":   hook.Event,
					"mode":    hook.Mode,
					"command": hook.Command,
					"args":    hook.Args,
				},
			)
		}
		data, err := toml.Marshal(map[string]any{"hooks": map[string]any{"declarations": declarations}})
		if err != nil {
			t.Fatal(err)
		}
		existing, err := os.ReadFile(home.ConfigFile)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(home.ConfigFile, append(append(existing, '\n'), data...), 0600); err != nil {
			t.Fatal(err)
		}
	}
	d := newTestDaemon(t, home, &cfg)
	if err := d.boot(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})
	workspace, err := d.workspaceResolver.ResolveOrRegister(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager, ok := d.sessions.(*session.Manager)
	if !ok {
		t.Fatal("real session manager missing")
	}
	return d, manager, workspace.ID
}

// IT-020: committed ACP Agent/Task traffic creates readable native rows without creating child sessions or wakes.
func TestSubagentNativeDaemonIntegration(t *testing.T) {
	t.Run("Should persist native work without child sessions", func(t *testing.T) {
		d, manager, workspace := newSubagentDaemonIntegration(t)
		parent, err := manager.Create(t.Context(), session.CreateOpts{AgentName: "subagent-test", Workspace: workspace})
		if err != nil {
			t.Fatal(err)
		}
		response, err := manager.SendPrompt(t.Context(), parent.ID, session.SendPromptOpts{Message: "native work"})
		if err != nil {
			t.Fatal(err)
		}
		for range response.Events {
			continue
		}
		var row store.SessionSubagent
		waitForRuntimeCondition(t, "native terminal row", 10*time.Second, func() bool {
			page, err := d.SubagentService().
				List(t.Context(), store.SubagentListQuery{WorkspaceID: workspace, ParentSessionID: parent.ID, Limit: 20})
			if err != nil || len(page.Items) != 1 {
				return false
			}
			row = page.Items[0]
			return row.Status == "completed"
		})
		if row.Origin != "provider_native" || row.ChildSessionID != nil || row.ProviderToolCallID != "toolu_agent" ||
			row.RuntimeModel != "sonnet-5.5" ||
			row.Title != "Review the diff (high effort)" ||
			row.WakeMessageID != nil ||
			row.Delivery != "none" ||
			row.Result == nil ||
			*row.Result == "" {
			t.Fatal(row)
		}
		if _, err := d.SubagentService().
			Cancel(t.Context(), session.SubagentActor{Kind: "operator"}, row.ID, ""); !errors.Is(
			err,
			session.ErrSubagentNotCancelable,
		) {
			t.Fatal(err)
		}
	})
}

// IT-004: real ACP steering injects without canceling; a refusal queues once with the same wake ID.
func TestSubagentSteerDaemonIntegration(t *testing.T) {
	for _, outcome := range []string{"injected", "failed"} {
		t.Run("Should handle "+outcome, func(t *testing.T) {
			d, manager, workspace := newSubagentDaemonIntegration(t, outcome)
			parent, err := manager.Create(
				t.Context(),
				session.CreateOpts{AgentName: "subagent-test", Workspace: workspace},
			)
			if err != nil {
				t.Fatal(err)
			}
			response, err := manager.SendPrompt(t.Context(), parent.ID, session.SendPromptOpts{Message: "hold parent"})
			if err != nil {
				t.Fatal(err)
			}
			go func() {
				for range response.Events {
					continue
				}
			}()
			waitForRuntimeCondition(
				t,
				"parent active",
				10*time.Second,
				func() bool { return parent.IsPrompting() && parent.CurrentTurnID() != "" },
			)
			turn := parent.CurrentTurnID()
			req := session.SubagentRequest{
				Caller: session.SubagentCaller{
					WorkspaceID: workspace,
					SessionID:   parent.ID,
					TurnID:      turn,
					ToolCallID:  "steer-child",
				},
				Task:  "child work",
				Title: "Child",
			}
			row, err := d.SubagentService().Delegate(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			db := d.registry.(store.SubagentStore)
			var current store.SessionSubagent
			waitForRuntimeCondition(t, "steer delivery", 15*time.Second, func() bool {
				var err error
				current, err = db.GetSubagent(t.Context(), workspace, row.ID)
				if err != nil || current.WakeMessageID == nil {
					return false
				}
				wake, _, err := db.GetWake(t.Context(), *current.WakeMessageID)
				return err == nil &&
					((outcome == "injected" && current.Delivery == "delivered") || (outcome == "failed" && wake.SteerRequeued && wake.Route == "queue"))
			})
			if !parent.IsPrompting() || parent.CurrentTurnID() != turn {
				t.Fatal("steering interrupted parent")
			}
			wake, _, err := db.GetWake(t.Context(), *current.WakeMessageID)
			if err != nil {
				t.Fatal(err)
			}
			if outcome == "injected" && (wake.Route != "steer" || wake.InputEntryID != "") {
				t.Fatal(wake)
			}
		})
	}
}

// IT-002/IT-003: bounded waiting consumes a fast result without a wake; timeout upgrades late delivery.
func TestSubagentWaitDaemonIntegration(t *testing.T) {
	d, manager, workspace := newSubagentDaemonIntegration(t)
	parent, caller := startSubagentIntegrationParent(t, manager, workspace)
	for _, tc := range []struct {
		name, task string
		timeout    time.Duration
		timedOut   bool
	}{
		{"Should return settled result without wake", "child work", 10 * time.Second, false},
		{"Should upgrade timed out wait for late delivery", "slow child", time.Second, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := session.SubagentRequest{
				Caller:         caller,
				Task:           tc.task,
				Title:          tc.task,
				Mode:           "wait",
				Timeout:        tc.timeout,
				IdempotencyKey: tc.task,
			}
			row, err := d.SubagentService().Delegate(t.Context(), req)
			if err != nil || row.WaitTimedOut != tc.timedOut {
				t.Fatal(row, err)
			}
			if !tc.timedOut &&
				(row.Status != "completed" || row.Result == nil || *row.Result != "child answer" || row.WakeMessageID != nil) {
				t.Fatal(row)
			}
			if tc.timedOut {
				waitForRuntimeCondition(t, "late wake", 10*time.Second, func() bool {
					current, err := d.SubagentService().Get(t.Context(), workspace, row.ID)
					return err == nil && current.Status == "completed" && current.Delivery == "claimed" &&
						current.WakePolicy == "always"
				})
			}
		})
	}
	if !parent.IsPrompting() {
		t.Fatal("waiting changed parent turn")
	}
}

// IT-008/IT-014: a real recursive ACP tree stops synchronously depth first and disposes its deliveries.
func TestSubagentStopDaemonIntegration(t *testing.T) {
	t.Run("Should synchronously stop the delegated tree", func(t *testing.T) {
		d, manager, workspace := newSubagentDaemonIntegration(t)
		parent, caller := startSubagentIntegrationParent(t, manager, workspace)
		child, err := d.SubagentService().
			Delegate(t.Context(), session.SubagentRequest{Caller: caller, Task: "hold parent", Title: "Child"})
		if err != nil {
			t.Fatal(err)
		}
		childSession, ok := manager.Get(*child.ChildSessionID)
		if !ok {
			t.Fatal("child session missing")
		}
		waitForRuntimeCondition(
			t,
			"child active",
			10*time.Second,
			func() bool { return childSession.IsPrompting() && childSession.CurrentTurnID() != "" },
		)
		grandchild, err := d.SubagentService().
			Delegate(t.Context(), session.SubagentRequest{Caller: session.SubagentCaller{WorkspaceID: workspace, SessionID: childSession.ID, TurnID: childSession.CurrentTurnID(), ToolCallID: "grandchild"}, Task: "hold parent", Title: "Grandchild"})
		if err != nil {
			t.Fatal(err)
		}
		if err := manager.Stop(t.Context(), parent.ID); err != nil {
			t.Fatal(err)
		}
		for _, row := range []session.Subagent{child, grandchild} {
			info, err := manager.Status(t.Context(), *row.ChildSessionID)
			if err != nil || info.State != session.StateStopped {
				t.Fatal(info, err)
			}
			current, err := d.SubagentService().Get(t.Context(), workspace, row.ID)
			if err != nil || current.Status != "canceled" || current.Delivery != "disposed" ||
				current.WakeMessageID != nil {
				t.Fatal(current, err)
			}
		}
		if _, err := d.SubagentService().
			Delegate(t.Context(), session.SubagentRequest{Caller: caller, Task: "child work", Title: "Late"}); !errors.Is(
			err,
			session.ErrSubagentParentNotActive,
		) {
			t.Fatal(err)
		} // IT-010
	})
}

func startSubagentIntegrationParent(
	t *testing.T,
	manager *session.Manager,
	workspace string,
) (*session.Session, session.SubagentCaller) {
	t.Helper()
	parent, err := manager.Create(t.Context(), session.CreateOpts{AgentName: "subagent-test", Workspace: workspace})
	if err != nil {
		t.Fatal(err)
	}
	response, err := manager.SendPrompt(t.Context(), parent.ID, session.SendPromptOpts{Message: "hold parent"})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range response.Events {
			continue
		}
	}()
	waitForRuntimeCondition(
		t,
		"parent active",
		10*time.Second,
		func() bool { return parent.IsPrompting() && parent.CurrentTurnID() != "" },
	)
	return parent, session.SubagentCaller{
		WorkspaceID: workspace,
		SessionID:   parent.ID,
		TurnID:      parent.CurrentTurnID(),
		ToolCallID:  "child",
	}
}

// IT-023: permission waiting belongs to the child and does not return prematurely from wait mode.
func TestSubagentPermissionDaemonIntegration(t *testing.T) {
	t.Run("Should wait through child approval", func(t *testing.T) {
		d, manager, workspace := newSubagentDaemonIntegration(t)
		_, caller := startSubagentIntegrationParent(t, manager, workspace)
		type result struct {
			row session.Subagent
			err error
		}
		done := make(chan result, 1)
		go func() {
			row, err := d.SubagentService().
				Delegate(t.Context(), session.SubagentRequest{Caller: caller, Task: "approval child", Title: "Approval", Mode: "wait", Timeout: 30 * time.Second, PermissionMode: compozyconfig.PermissionModeApproveReads})
			done <- result{row, err}
		}()
		var row store.SessionSubagent
		waitForRuntimeCondition(t, "child waiting for approval", 15*time.Second, func() bool {
			page, err := d.SubagentService().
				List(t.Context(), store.SubagentListQuery{WorkspaceID: workspace, ParentSessionID: caller.SessionID, Limit: 10})
			if err != nil || len(page.Items) != 1 {
				return false
			}
			row = page.Items[0]
			return row.Status == "waiting"
		})
		select {
		case got := <-done:
			t.Fatalf("wait returned before approval: %+v", got)
		default:
		}
		interactions, err := manager.PendingInteractions(
			t.Context(),
			*row.ChildSessionID,
			[]string{store.PendingInteractionStatusPending},
		)
		if err != nil || len(interactions) != 1 {
			t.Fatal(interactions, err)
		}
		if _, err := manager.ApprovePermission(
			t.Context(),
			*row.ChildSessionID,
			acp.ApproveRequest{TurnID: interactions[0].TurnID, Decision: "allow-once"},
		); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-done:
			if got.err != nil || got.row.Status != "completed" || got.row.Result == nil ||
				*got.row.Result != "approved answer" ||
				got.row.WakeMessageID != nil {
				t.Fatal(got)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("wait did not finish after approval")
		}
	})
}

// IT-009/IT-026: interrupt disposes delivery and releases wait without stopping the child.
// Owner: daemon orchestration; the existing ACP journey suite owns this lifecycle boundary.
func TestSubagentInterruptDaemonIntegration(t *testing.T) {
	t.Run("Should dispose delivery without stopping the child", func(t *testing.T) {
		d, manager, workspace := newSubagentDaemonIntegration(t)
		parent, caller := startSubagentIntegrationParent(t, manager, workspace)
		type result struct {
			row session.Subagent
			err error
		}
		done := make(chan result, 1)
		go func() {
			row, err := d.SubagentService().
				Delegate(t.Context(), session.SubagentRequest{Caller: caller, Task: "slow child", Title: "Slow", Mode: "wait", Timeout: 30 * time.Second})
			done <- result{row, err}
		}()
		var row store.SessionSubagent
		waitForRuntimeCondition(t, "running child", 10*time.Second, func() bool {
			page, err := d.SubagentService().
				List(t.Context(), store.SubagentListQuery{WorkspaceID: workspace, ParentSessionID: parent.ID, Limit: 10})
			if err != nil || len(page.Items) != 1 {
				return false
			}
			row = page.Items[0]
			if row.ChildSessionID == nil {
				return false
			}
			child, ok := manager.Get(*row.ChildSessionID)
			return ok && child.IsPrompting()
		})
		if _, err := manager.CancelPromptWithCause(t.Context(), parent.ID, session.PromptCancelUser); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-done:
			if got.err != nil || got.row.Delivery != "disposed" {
				t.Fatal(got)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("interrupt did not release waiter")
		}
		waitForRuntimeCondition(t, "child finishes independently", 10*time.Second, func() bool {
			current, err := d.SubagentService().Get(t.Context(), workspace, row.ID)
			return err == nil && current.Status == "completed" && current.Result != nil &&
				*current.Result == "slow answer" &&
				current.Delivery == "disposed" &&
				current.WakeMessageID == nil
		})
		waitForRuntimeCondition(
			t,
			"parent cancellation completed",
			5*time.Second,
			func() bool { return !parent.IsPrompting() },
		)
		response, err := manager.SendPrompt(t.Context(), parent.ID, session.SendPromptOpts{Message: "hold parent"})
		if err != nil {
			t.Fatal(err)
		}
		go func() {
			for range response.Events {
				continue
			}
		}()
		caller.TurnID = parent.CurrentTurnID()
		got, err := d.SubagentService().Status(t.Context(), caller, row.ID)
		if err != nil || got.Result == nil || *got.Result != "slow answer" {
			t.Fatal(got, err)
		}
	})
}

// IT-011/IT-031: the boot reactor reconciles durable rows, admissions and missing wake inputs idempotently.
// The daemon and ACP I/O stay real; detaching callbacks models the event gap before boot reconciliation.
func TestSubagentRecoveryDaemonIntegration(t *testing.T) {
	t.Run("Should reconcile durable work through the boot reactor", func(t *testing.T) {
		d, manager, workspace := newSubagentDaemonIntegration(t)
		parent, caller := startSubagentIntegrationParent(t, manager, workspace)
		service := d.SubagentService()
		manager.SetSubagentService(nil)
		t.Cleanup(func() { manager.SetSubagentService(service) })
		row, err := service.Delegate(
			t.Context(),
			session.SubagentRequest{Caller: caller, Task: "slow child", Title: "Recovered"},
		)
		if err != nil {
			t.Fatal(err)
		}
		child, ok := manager.Get(*row.ChildSessionID)
		if !ok {
			t.Fatal("missing child")
		}
		waitForRuntimeCondition(
			t,
			"unobserved child completion",
			10*time.Second,
			func() bool { return !child.IsPrompting() && child.CurrentTurnID() == "" },
		)
		current, err := service.Get(t.Context(), workspace, row.ID)
		if err != nil || current.Status != "running" {
			t.Fatal(current, err)
		}
		state := &bootState{
			sessions:          manager,
			registry:          d.registry,
			workspaceResolver: d.workspaceResolver.(*workspacepkg.Resolver),
			hooks:             d.hooks,
		}
		if err := d.bootSubagents(t.Context(), state); err != nil {
			t.Fatal(err)
		}
		service = state.subagents
		current, err = service.Get(t.Context(), workspace, row.ID)
		if err != nil || current.Status != "completed" || current.WakeMessageID == nil {
			t.Fatal(current, err)
		}
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		inputs, err := d.registry.(store.SessionInputQueueStore).ListPendingSessionInputs(t.Context(), parent.ID)
		if err != nil || len(inputs) != 1 || inputs[0].MessageID != *current.WakeMessageID {
			t.Fatal(inputs, err)
		}
		if _, err := service.Status(t.Context(), caller, row.ID); err != nil {
			t.Fatal(err)
		}
		db := d.registry.(store.SubagentStore)
		missing := row.SessionSubagent
		missing.ID = "sub-recovery-missing"
		missing.IdempotencyKey = "recovery-missing"
		missing.RequestFingerprint = "recovery-missing"
		missing.ChildSessionID = nil
		missing.Status = "queued"
		missing.WorkState = "working"
		missing.Delivery = "none"
		missing.WakeMessageID = nil
		missing.PendingTask = nil
		if _, _, err := db.ReserveSubagent(t.Context(), missing); err != nil {
			t.Fatal(err)
		}
		if _, _, err := db.FinalizeSubagent(
			t.Context(),
			store.SubagentFinalize{
				ID:        missing.ID,
				Status:    "completed",
				WorkState: "result_available",
				Result:    new("recovered"),
				SettledAt: time.Now(),
			},
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.OpenOrJoinWake(
			t.Context(),
			parent.ID,
			[]string{missing.ID},
			"wake-recovery-missing",
		); err != nil {
			t.Fatal(err)
		}
		orphan, err := manager.Spawn(
			t.Context(),
			session.SpawnOpts{
				ParentSessionID:  parent.ID,
				ParentTurnID:     caller.TurnID,
				AgentName:        "subagent-test",
				SpawnRole:        "subagent",
				AutoStopOnParent: true,
				NotifyCreatorSet: true,
				Subagent:         &hookspkg.SubagentSpawnPayload{Title: "Orphan", Role: "general", TaskChars: 1},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := service.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		info, err := manager.Status(t.Context(), orphan.ID)
		if err != nil || info.State != session.StateStopped {
			t.Fatal(info, err)
		}
		inputs, err = d.registry.(store.SessionInputQueueStore).ListPendingSessionInputs(t.Context(), parent.ID)
		if err != nil || len(inputs) != 1 || inputs[0].MessageID != "wake-recovery-missing" {
			t.Fatal(inputs, err)
		}
	})
}

// IT-022: an executed pre-create hook denies before a session exists and removes the reservation.
// Owner: daemon integration; verifies the real hook executor, service and database boundary together.
func TestSubagentHookDaemonIntegration(t *testing.T) {
	t.Run("Should remove the reservation on hook denial", func(t *testing.T) {
		capture := filepath.Join(t.TempDir(), "spawn.json")
		script := writeDaemonHookScript(
			t,
			t.TempDir(),
			"deny.sh",
			"#!/bin/sh\ncat > \"$1\"\nprintf '%s\\n' '{\"deny\":true,\"deny_reason\":\"fixture policy\"}'\n",
		)
		d, manager, workspace := newSubagentDaemonConfigured(t, func(cfg *compozyconfig.Config) {
			cfg.Hooks.Declarations = append(
				cfg.Hooks.Declarations,
				hookspkg.HookDecl{
					Name:    "deny-subagent",
					Event:   hookspkg.HookSpawnPreCreate,
					Mode:    hookspkg.HookModeSync,
					Command: script,
					Args:    []string{capture},
				},
			)
		})
		_, caller := startSubagentIntegrationParent(t, manager, workspace)
		_, err := d.SubagentService().
			Delegate(t.Context(), session.SubagentRequest{Caller: caller, Task: "child work", Title: "Denied"})
		if !errors.Is(err, session.ErrSubagentCapabilityDenied) || !strings.Contains(err.Error(), "fixture policy") {
			t.Fatal(err)
		}
		page, err := d.SubagentService().
			List(t.Context(), store.SubagentListQuery{WorkspaceID: workspace, ParentSessionID: caller.SessionID, Limit: 10})
		if err != nil || len(page.Items) != 0 {
			t.Fatal(page, err)
		}
		data, err := os.ReadFile(capture)
		if err != nil {
			t.Fatal(err)
		}
		var payload hookspkg.SpawnPreCreatePayload
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Subagent == nil || payload.Subagent.Title != "Denied" || payload.Subagent.TaskChars != 10 {
			t.Fatal(payload)
		}
	})
}

// IT-015: descendants wake only their immediate parent; an idle child waits for its own live work.
func TestSubagentRecursiveDaemonIntegration(t *testing.T) {
	t.Run("Should wake each immediate parent in order", func(t *testing.T) {
		d, manager, workspace := newSubagentDaemonIntegration(t)
		_, caller := startSubagentIntegrationParent(t, manager, workspace)
		child, err := d.SubagentService().
			Delegate(t.Context(), session.SubagentRequest{Caller: caller, Task: "hold parent", Title: "Child"})
		if err != nil {
			t.Fatal(err)
		}
		childSession, ok := manager.Get(*child.ChildSessionID)
		if !ok {
			t.Fatal("missing child")
		}
		grandchild, err := d.SubagentService().
			Delegate(t.Context(), session.SubagentRequest{Caller: session.SubagentCaller{WorkspaceID: workspace, SessionID: childSession.ID, TurnID: childSession.CurrentTurnID(), ToolCallID: "grandchild"}, Task: "slow child", Title: "Grandchild"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.CancelPromptWithCause(
			t.Context(),
			childSession.ID,
			session.PromptCancelSyntheticAdmission,
		); err != nil {
			t.Fatal(err)
		}
		waitForRuntimeCondition(t, "waiting for grandchild", 5*time.Second, func() bool {
			row, err := d.SubagentService().Get(t.Context(), workspace, child.ID)
			return err == nil && row.WorkState == "waiting_for_children" && row.Status == "running" &&
				row.WakeMessageID == nil
		})
		waitForRuntimeCondition(t, "recursive completion", 10*time.Second, func() bool {
			row, err := d.SubagentService().Get(t.Context(), workspace, child.ID)
			return err == nil && row.Status == "completed" && row.Delivery == "claimed"
		})
		db := d.registry.(store.SubagentStore)
		waitForRuntimeCondition(t, "grandchild wake delivery receipt", 5*time.Second, func() bool {
			row, err := db.GetSubagent(t.Context(), workspace, grandchild.ID)
			return err == nil && row.Delivery == "delivered"
		})
		grand, err := db.GetSubagent(t.Context(), workspace, grandchild.ID)
		if err != nil || grand.WakeMessageID == nil || grand.Delivery != "delivered" {
			t.Fatal(grand, err)
		}
		wake, _, err := db.GetWake(t.Context(), *grand.WakeMessageID)
		if err != nil || wake.ParentSessionID != childSession.ID {
			t.Fatal(wake, err)
		}
	})
}

// IT-017: a launch failure after target resolution persists one failed row and no wake input.
func TestSubagentBootFailureDaemonIntegration(t *testing.T) {
	t.Run("Should persist a launch failure without a wake", func(t *testing.T) {
		d, manager, workspace := newSubagentDaemonConfigured(t, func(cfg *compozyconfig.Config) {
			cfg.Providers["broken"] = compozyconfig.ProviderConfig{
				Command:  "compozy-test-provider-does-not-exist",
				AuthMode: compozyconfig.ProviderAuthModeNone,
			}
		})
		parent, caller := startSubagentIntegrationParent(t, manager, workspace)
		row, err := d.SubagentService().
			Delegate(t.Context(), session.SubagentRequest{Caller: caller, Task: "child work", Title: "Broken", Target: session.SubagentTarget{Provider: "broken"}})
		if err != nil || row.Status != "failed" || row.Error == nil ||
			!strings.Contains(*row.Error, "compozy-test-provider-does-not-exist") {
			t.Fatal(row, err)
		}
		inputs, err := d.registry.(store.SessionInputQueueStore).ListPendingSessionInputs(t.Context(), parent.ID)
		if err != nil || len(inputs) != 0 {
			t.Fatal(inputs, err)
		}
	})
}

// IT-031: a fresh daemon must admit the durable task exactly once after a crash
// between LinkChild and first admission. Owner: daemon restart integration.
func TestSubagentFirstAdmissionRestartDaemonIntegration(t *testing.T) {
	t.Run("Should readmit a linked child exactly once across a full restart", func(t *testing.T) {
		d, manager, workspace := newSubagentDaemonIntegration(t)
		parent, caller := startSubagentIntegrationParent(t, manager, workspace)
		db := d.registry.(store.SubagentStore)
		now := time.Now().UTC()
		row := store.SessionSubagent{
			ID: "sub-first-admission", WorkspaceID: workspace, ParentSessionID: parent.ID, Depth: 1,
			ParentTurnID: caller.TurnID, ParentToolCallID: caller.ToolCallID,
			Origin: store.SubagentOriginDelegated, IdempotencyKey: "first-admission",
			RequestFingerprint: "first-admission", Title: "Recover first admission", Role: "general",
			TaskChars: len("slow child"), PendingTask: new("slow child"),
			RuntimeAgent: "subagent-test", RuntimeProvider: acpmock.ProviderName,
			Status: store.SubagentStatusQueued, WorkState: store.SubagentWorkStateWorking,
			WakePolicy: store.SubagentWakePolicyAlways, Delivery: store.SubagentDeliveryNone,
			CreatedAt: now, UpdatedAt: now,
		}
		if _, _, err := db.ReserveSubagent(t.Context(), row); err != nil {
			t.Fatal(err)
		}
		child, err := manager.Spawn(t.Context(), session.SpawnOpts{
			ParentSessionID: parent.ID, ParentTurnID: caller.TurnID, AgentName: "subagent-test",
			SpawnRole: "subagent", AutoStopOnParent: true, NotifyCreatorSet: true,
			IdempotencyKey: row.ID,
			Subagent:       &hookspkg.SubagentSpawnPayload{Title: row.Title, Role: row.Role, TaskChars: row.TaskChars},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.LinkChild(t.Context(), row.ID, child.ID, now); err != nil {
			t.Fatal(err)
		}
		// Suspend the reactor at the durable commit boundary. Teardown still joins
		// the real processes and closes every database before the new daemon boots.
		manager.SetSubagentService(nil)
		home, cfg := d.homePaths, d.config
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatal(err)
		}
		restarted := reopenSubagentIntegrationDaemon(t, home, &cfg)
		freshManager := restarted.sessions.(*session.Manager)
		service := restarted.SubagentService()
		reaper := &spawnReaper{sessions: freshManager, logger: discardLogger(),
			now: func() time.Time { return time.Now().Add(time.Hour) }}
		report, err := reaper.Sweep(t.Context())
		if err != nil || report.Reaped != 0 {
			t.Fatal(report, err)
		}
		waitForRuntimeCondition(t, "recovered child result", 10*time.Second, func() bool {
			got, err := service.Get(t.Context(), workspace, row.ID)
			return err == nil && store.IsSubagentStatusTerminal(got.Status)
		})
		got, err := restarted.registry.(store.SubagentStore).GetSubagent(t.Context(), workspace, row.ID)
		if got.Error != nil {
			t.Logf("recovery failure: %s", *got.Error)
		}
		if err != nil || got.Status != store.SubagentStatusCompleted || got.PendingTask != nil || got.Result == nil ||
			*got.Result != "slow answer" {
			t.Fatalf("recovered row = %+v, err = %v, failure = %v", got, err, got.Error)
		}
		for range 2 {
			if err := service.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
		page, err := freshManager.TranscriptPage(t.Context(), child.ID, transcript.PageQuery{Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		admissions := 0
		for _, entry := range page.Entries {
			var meta struct {
				MessageID string `json:"message_id"`
			}
			if entry.Message.Role == transcript.UIRoleUser && json.Unmarshal(entry.Message.Metadata, &meta) == nil &&
				meta.MessageID == row.ID {
				admissions++
			}
		}
		if admissions != 1 {
			t.Fatalf("first-prompt admissions = %d, want 1", admissions)
		}
	})
}

// IT-011: replay the durable checkpoint of a settled ACP child whose reactor
// observation was lost, then reopen the entire daemon twice over the same home.
// Owner: daemon restart integration; one retained wake, dispatched only on resume.
func TestSubagentSettledRestartDaemonIntegration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		missed bool
	}{
		{"Should recover an unobserved result and retain its wake until resume", true},
		{"Should retain an already queued wake across shutdown and restart", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, manager, workspace := newSubagentDaemonIntegration(t)
			parent, caller := startSubagentIntegrationParent(t, manager, workspace)
			if tc.missed {
				manager.SetSubagentService(nil)
			}
			row, err := d.SubagentService().Delegate(t.Context(), session.SubagentRequest{
				Caller: caller, Task: "child work", Title: "Unobserved completion",
			})
			if err != nil {
				t.Fatal(err)
			}
			child, ok := manager.Get(*row.ChildSessionID)
			if !ok {
				t.Fatal("missing child")
			}
			waitForRuntimeCondition(t, "persisted child completion", 10*time.Second, func() bool {
				return !child.IsPrompting() && child.CurrentTurnID() == ""
			})
			if err := manager.StopWithCause(
				t.Context(),
				child.ID,
				session.CauseCompleted,
				"fixture task completed",
			); err != nil {
				t.Fatal(err)
			}
			before, err := d.SubagentService().Get(t.Context(), workspace, row.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.missed {
				if before.Status != store.SubagentStatusRunning {
					t.Fatal(before)
				}
			} else {
				waitForRuntimeCondition(t, "wake queued before shutdown", 10*time.Second, func() bool {
					before, err = d.SubagentService().Get(t.Context(), workspace, row.ID)
					return err == nil && before.WakeMessageID != nil && before.Delivery == store.SubagentDeliveryClaimed
				})
			}
			home, cfg := d.homePaths, d.config
			if err := d.Shutdown(testutil.Context(t)); err != nil {
				t.Fatal(err)
			}
			restarted := reopenSubagentIntegrationDaemon(t, home, &cfg)
			settled, wake := requireSubagentRestartWake(t, restarted, workspace, row.ID)
			if before.WakeMessageID != nil && *before.WakeMessageID != *settled.WakeMessageID {
				t.Fatal("queued wake identity changed on shutdown")
			}
			if settled.Status != store.SubagentStatusCompleted || settled.Result == nil ||
				*settled.Result != "child answer" {
				t.Fatalf("settled = %+v", settled)
			}
			if err := restarted.Shutdown(testutil.Context(t)); err != nil {
				t.Fatal(err)
			}
			restarted = reopenSubagentIntegrationDaemon(t, home, &cfg)
			again, secondWake := requireSubagentRestartWake(t, restarted, workspace, row.ID)
			if wake.ID != secondWake.ID || *settled.WakeMessageID != *again.WakeMessageID ||
				!settled.SettledAt.Equal(*again.SettledAt) {
				t.Fatalf("restart changed settlement or wake: %+v / %+v", settled, again)
			}
			requireSubagentRestartDelivery(t, restarted, workspace, row.ID, parent.ID)
		})
	}
}

func reopenSubagentIntegrationDaemon(t *testing.T, home compozyconfig.HomePaths, cfg *compozyconfig.Config) *Daemon {
	t.Helper()
	d := newTestDaemon(t, home, cfg)
	if err := d.boot(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Error(err)
		}
	})
	return d
}

func requireSubagentRestartWake(
	t *testing.T,
	d *Daemon,
	workspace, id string,
) (session.Subagent, store.SessionInputQueueEntry) {
	t.Helper()
	service := d.SubagentService()
	for range 2 {
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	row, err := service.Get(t.Context(), workspace, id)
	if err != nil || !store.IsSubagentStatusTerminal(row.Status) || row.WakeMessageID == nil ||
		row.Delivery != store.SubagentDeliveryClaimed {
		t.Fatalf("recovered row = %+v, err = %v", row, err)
	}
	inputs, err := d.registry.(store.SessionInputQueueStore).ListPendingSessionInputs(t.Context(), row.ParentSessionID)
	if err != nil || len(inputs) != 1 || inputs[0].MessageID != *row.WakeMessageID {
		t.Fatal(inputs, err)
	}
	manager := d.sessions.(*session.Manager)
	if _, active := manager.Get(row.ParentSessionID); active {
		t.Fatal("boot resumed the parent implicitly")
	}
	return row, inputs[0]
}

func requireSubagentRestartDelivery(t *testing.T, d *Daemon, workspace, id, parent string) {
	t.Helper()
	manager := d.sessions.(*session.Manager)
	if _, err := manager.Resume(t.Context(), parent); err != nil {
		t.Fatal(err)
	}
	waitForRuntimeCondition(t, "wake delivery after explicit resume", 10*time.Second, func() bool {
		row, err := d.SubagentService().Get(t.Context(), workspace, id)
		return err == nil && row.Delivery == store.SubagentDeliveryDelivered
	})
	if err := d.SubagentService().Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	events, err := manager.Events(t.Context(), parent, store.EventQuery{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	wakes := 0
	for _, event := range events {
		if event.Type == acp.EventTypeSyntheticReentry {
			wakes++
		}
	}
	if wakes != 1 {
		t.Fatalf("wake responses = %d, want 1", wakes)
	}
}

// IT-012: kill the actual daemon-hosting process mid child turn; ordinary boot
// repair must settle the child and produce one wake retained until parent resume.
// Owner: daemon restart integration, real ACP subprocesses and persisted stores.
func TestSubagentCrashRestartDaemonIntegration(t *testing.T) {
	if handoff := os.Getenv("COMPOZY_SUBAGENT_CRASH_HANDOFF"); handoff != "" {
		runSubagentCrashProcess(t, handoff)
		return
	}
	t.Run("Should recover a child after killing its daemon process", func(t *testing.T) {
		// Own every helper-created temporary directory, including TestMain's seed.
		root, err := os.MkdirTemp("/tmp", "sa-crash-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(root); err != nil {
				t.Error(err)
			}
		})
		handoff := filepath.Join(root, "handoff.json")
		output, err := os.Create(filepath.Join(root, "daemon.log"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := output.Close(); err != nil {
				t.Error(err)
			}
		})
		cmd := exec.CommandContext(
			t.Context(),
			os.Args[0],
			"-test.run=^TestSubagentCrashRestartDaemonIntegration$",
			"-test.timeout=2m",
		)
		cmd.Env = append(os.Environ(), "TMPDIR="+root, "COMPOZY_SUBAGENT_CRASH_HANDOFF="+handoff)
		cmd.Stdout, cmd.Stderr = output, output
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		waited := false
		t.Cleanup(func() {
			if !waited {
				if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
					t.Error(err)
				}
				<-done
			}
		})
		waitForRuntimeCondition(t, "daemon child mid turn", 30*time.Second, func() bool {
			select {
			case err := <-done:
				waited = true
				logs, readErr := os.ReadFile(output.Name())
				t.Fatalf("helper exited before checkpoint: %v (%v)\n%s", err, readErr, logs)
			default:
			}
			_, err := os.Stat(handoff)
			return err == nil
		})
		data, err := os.ReadFile(handoff)
		if err != nil {
			t.Fatal(err)
		}
		var checkpoint subagentCrashCheckpoint
		if err := json.Unmarshal(data, &checkpoint); err != nil {
			t.Fatal(err)
		}
		if err := cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err == nil {
			t.Fatal("daemon helper was not killed")
		}
		waited = true
		cfg, err := compozyconfig.LoadGlobalConfig(checkpoint.Home)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Automation.Enabled = false
		cfg.Roles.AutoTitle.Enabled = false
		cfg.ModelCatalog.Sources.ModelsDev.Enabled = new(false)
		restarted := reopenSubagentIntegrationDaemon(t, checkpoint.Home, &cfg)
		row, _ := requireSubagentRestartWake(t, restarted, checkpoint.Workspace, checkpoint.Row.ID)
		info, err := restarted.sessions.Status(t.Context(), *row.ChildSessionID)
		if err != nil || info.State != session.StateStopped || info.StopReason != store.StopAgentCrashed {
			t.Fatalf("child recovery = %+v, err = %v", info, err)
		}
		if row.Status != store.SubagentStatusFailed || row.Error == nil {
			t.Fatalf("crashed row = %+v", row)
		}
		// The existing dead-runtime contract forbids reviving a process-failed
		// parent. Preserve its mailbox without bypassing that independent gate.
		if _, err := restarted.sessions.Resume(
			t.Context(),
			row.ParentSessionID,
		); !errors.Is(
			err,
			store.ErrSessionNotAttachable,
		) {
			t.Fatalf("Resume(dead parent) = %v, want ErrSessionNotAttachable", err)
		}
		again, _ := requireSubagentRestartWake(t, restarted, checkpoint.Workspace, row.ID)
		if !row.SettledAt.Equal(*again.SettledAt) || *row.WakeMessageID != *again.WakeMessageID {
			t.Fatal("recovery duplicated settlement or wake", again)
		}
	})
}

type subagentCrashCheckpoint struct {
	Home      compozyconfig.HomePaths
	Workspace string
	Row       session.Subagent
}

func runSubagentCrashProcess(t *testing.T, handoff string) {
	t.Helper()
	d, manager, workspace := newSubagentDaemonIntegration(t)
	_, caller := startSubagentIntegrationParent(t, manager, workspace)
	row, err := d.SubagentService().Delegate(t.Context(), session.SubagentRequest{
		Caller: caller, Task: "hold parent", Title: "Crash mid child turn",
	})
	if err != nil {
		t.Fatal(err)
	}
	child, ok := manager.Get(*row.ChildSessionID)
	if !ok {
		t.Fatal("missing child")
	}
	waitForRuntimeCondition(t, "persisted child turn", 10*time.Second, func() bool {
		events, err := manager.Events(t.Context(), child.ID, store.EventQuery{Limit: 100})
		if err != nil || !child.IsPrompting() || child.CurrentTurnID() == "" {
			return false
		}
		for _, event := range events {
			if event.Type == acp.EventTypeThought {
				return true
			}
		}
		return false
	})
	data, err := json.Marshal(subagentCrashCheckpoint{Home: d.homePaths, Workspace: workspace, Row: row})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handoff+".tmp", data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(handoff+".tmp", handoff); err != nil {
		t.Fatal(err)
	}
	<-t.Context().Done()
}
