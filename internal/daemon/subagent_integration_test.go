//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

// Invariant: real daemon wiring reserves, launches ACP, persists a result and delivers one wake.
// Owner: daemon orchestration; canonical suite for the new subagent journey (IT-001/IT-013 queue and replay halves).
func TestSubagentDaemonIntegration(t *testing.T) {
	d, manager, workspace := newSubagentDaemonIntegration(t)
	ctx := t.Context()
	parent, err := manager.Create(ctx, session.CreateOpts{AgentName: "subagent-test", Workspace: workspace})
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := manager.SendPrompt(ctx, parent.ID, session.SendPromptOpts{Message: "hold parent"})
	if err != nil {
		t.Fatal(err)
	}
	drained := make(chan struct{})
	go func() {
		for range prompt.Events {
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
	req := session.SubagentRequest{Caller: caller, Task: "child work", Title: "Child work"}
	row, err := d.SubagentService().Delegate(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := d.SubagentService().Delegate(ctx, req)
	if err != nil || again.ID != row.ID || again.ChildSessionID == nil || *again.ChildSessionID != *row.ChildSessionID {
		t.Fatal(again, err)
	}
	waitForRuntimeCondition(t, "durable child result", 15*time.Second, func() bool {
		current, err := d.SubagentService().Get(ctx, caller.WorkspaceID, row.ID)
		return err == nil && current.Status == "completed" && current.Result != nil && *current.Result == "child answer"
	})
	db := d.registry.(store.SubagentStore)
	durable, err := db.GetSubagent(ctx, caller.WorkspaceID, row.ID)
	if err != nil || durable.PendingTask != nil || durable.Delivery != "claimed" || durable.WakeMessageID == nil {
		t.Fatal(durable, err)
	}
	child, err := manager.Status(ctx, *row.ChildSessionID)
	if err != nil || child.Lineage == nil || child.Lineage.TTLExpiresAt != nil || child.Lineage.NotifyCreator ||
		child.LastSettledRevision > child.LastSeenRevision {
		t.Fatal(child, err)
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
	if err != nil || len(queue) != 1 || queue[0].Priority != 1 || strings.Contains(queue[0].Text, rows[0].ID) ||
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
	events, err := manager.Events(ctx, parent.ID, store.EventQuery{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	wakeCount := 0
	for _, event := range events {
		if event.Type == acp.EventTypeSyntheticReentry {
			wakeCount++
		}
	}
	if wakeCount != 1 {
		t.Fatalf("synthetic wake count=%d", wakeCount)
	}
	page, err := d.SubagentService().
		List(ctx, store.SubagentListQuery{WorkspaceID: caller.WorkspaceID, ParentSessionID: parent.ID, Limit: 20})
	if err != nil || len(page.Items) != 3 {
		t.Fatal(page, err)
	}
}

func newSubagentDaemonIntegration(t *testing.T, steer ...string) (*Daemon, *session.Manager, string) {
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
	)
	if len(steer) > 0 {
		fixture.Agents[0].SteerOutcome = steer[0]
	}
	nativeData, err := os.ReadFile(filepath.Join("..", "acp", "testdata", "claude_agent_subagent.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	nativeSteps := []acpmock.Step{}
	for _, frame := range strings.Split(strings.TrimSpace(string(nativeData)), "\n") {
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
			},
			PermissionMode: compozyconfig.PermissionModeApproveAll,
			Mutate: func(cfg *compozyconfig.Config) {
				cfg.Automation.Enabled = false
				cfg.Roles.AutoTitle.Enabled = false
				cfg.ModelCatalog.Sources.ModelsDev.Enabled = new(false)
			},
		},
	)
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
