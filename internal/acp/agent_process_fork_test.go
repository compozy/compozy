package acp

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/testutil"
)

func TestAgentProcessRouteSessionUpdate(t *testing.T) {
	t.Parallel()

	t.Run("Should classify bound, fork-capture, and foreign ids", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{}
		if got := proc.routeSessionUpdate("sess-any"); got != sessionUpdateRouteBound {
			t.Fatalf("routeSessionUpdate(unbound) = %d, want bound", got)
		}
		proc.bindSessionRoute("sess-bound")
		if got := proc.routeSessionUpdate("sess-bound"); got != sessionUpdateRouteBound {
			t.Fatalf("routeSessionUpdate(bound id) = %d, want bound", got)
		}
		if got := proc.routeSessionUpdate("sess-clone"); got != sessionUpdateRouteForeign {
			t.Fatalf("routeSessionUpdate(other id) = %d, want foreign", got)
		}
		capture := proc.openForkCapture()
		if got := proc.routeSessionUpdate("sess-clone"); got != sessionUpdateRouteFork {
			t.Fatalf("routeSessionUpdate(open fork capture) = %d, want fork", got)
		}
		if got := proc.routeSessionUpdate("sess-bound"); got != sessionUpdateRouteBound {
			t.Fatalf("routeSessionUpdate(bound id during fork) = %d, want bound", got)
		}
		proc.closeForkCapture(capture)
		if got := proc.routeSessionUpdate("sess-clone"); got != sessionUpdateRouteForeign {
			t.Fatalf("routeSessionUpdate(after fork) = %d, want foreign", got)
		}
	})

	t.Run("Should drop foreign updates, log once per id, and count every drop", func(t *testing.T) {
		t.Parallel()

		var logs bytes.Buffer
		proc := &AgentProcess{logger: slog.New(slog.NewTextHandler(&logs, nil))}
		proc.bindSessionRoute("sess-bound")
		proc.setCaps(Caps{ConfigOptions: []SessionConfigOption{{ID: "model", CurrentValueID: "bound-model"}}})
		active := beginObservedPrompt(t, proc)

		for _, text := range []string{"one", "two"} {
			handleTestSessionUpdate(t, proc, acpsdk.SessionNotification{
				SessionId: "sess-foreign",
				Update:    acpsdk.UpdateAgentMessageText(text),
			})
		}
		handleTestSessionUpdate(t, proc, acpsdk.SessionNotification{
			SessionId: "sess-foreign",
			Update: acpsdk.SessionUpdate{ConfigOptionUpdate: &acpsdk.SessionConfigOptionUpdate{
				SessionUpdate: sessionUpdateConfigOption,
				ConfigOptions: helperConfigOptions("foreign-model", "high"),
			}},
		})
		handleTestSessionUpdate(t, proc, acpsdk.SessionNotification{
			SessionId: "sess-bound",
			Update:    acpsdk.UpdateAgentMessageText("bound"),
		})

		if got := proc.foreignSessionTrafficDropped(); got != 3 {
			t.Fatalf("foreignSessionTrafficDropped() = %d, want 3", got)
		}
		if got := strings.Count(logs.String(), "foreign_acp_session_id=sess-foreign"); got != 1 {
			t.Fatalf("foreign drop log lines = %d, want 1\n%s", got, logs.String())
		}
		if got := proc.CapsSnapshot().ConfigOptions[0].CurrentValueID; got != "bound-model" {
			t.Fatalf("config option after foreign update = %q, want bound-model", got)
		}
		events := drainObservedPrompt(proc, active)
		if len(events) != 1 || events[0].SessionID != "sess-bound" || events[0].Text != "bound" {
			t.Fatalf("bound prompt stream = %+v, want only the bound-id chunk", events)
		}
	})

	t.Run("Should reject a foreign permission request without touching the active turn", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{}
		proc.bindSessionRoute("sess-bound")
		active := beginObservedPrompt(t, proc)

		response, err := proc.handleRequestPermission(testutil.Context(t), acpsdk.RequestPermissionRequest{
			SessionId: "sess-foreign",
			ToolCall:  acpsdk.ToolCallUpdate{ToolCallId: "tool-1"},
			Options: []acpsdk.PermissionOption{
				{OptionId: "allow", Name: "Allow", Kind: acpsdk.PermissionOptionKindAllowOnce},
			},
		})
		if err != nil {
			t.Fatalf("handleRequestPermission() error = %v", err)
		}
		if response.Outcome.Cancelled == nil { //nolint:misspell // ACP SDK field uses British spelling.
			t.Fatalf("handleRequestPermission() outcome = %+v, want canceled", response.Outcome)
		}
		if got := proc.activeTurnID(); got != "turn-observed" {
			t.Fatalf("activeTurnID() = %q, want turn-observed", got)
		}
		if got := proc.foreignSessionTrafficDropped(); got != 1 {
			t.Fatalf("foreignSessionTrafficDropped() = %d, want 1", got)
		}
		if events := drainObservedPrompt(proc, active); len(events) != 0 {
			t.Fatalf("bound prompt stream = %+v, want no permission events", events)
		}
	})
}

func TestAgentProcessForkSession(t *testing.T) {
	t.Parallel()

	t.Run("Should capture pre-response clone updates without touching the bound session", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "fork_session", "", StartOpts{})
		t.Cleanup(func() {
			stopProcess(t, driver, proc)
		})
		capsBefore := proc.CapsSnapshot()
		active := beginObservedPrompt(t, proc)

		result, err := proc.ForkSession(testutil.Context(t), proc.Cwd, nil)
		if err != nil {
			t.Fatalf("ForkSession() error = %v", err)
		}
		if result.SessionID != "sess-fork" {
			t.Fatalf("ForkSession() SessionID = %q, want sess-fork", result.SessionID)
		}
		if len(result.Updates) != 3 {
			t.Fatalf("ForkSession() Updates = %d, want 3: %+v", len(result.Updates), result.Updates)
		}
		if result.Updates[0].Update.AvailableCommandsUpdate == nil ||
			result.Updates[1].Update.AgentMessageChunk == nil ||
			result.Updates[2].Update.AgentMessageChunk == nil {
			t.Fatalf("ForkSession() Updates = %+v, want commands then two agent chunks", result.Updates)
		}
		for _, update := range result.Updates {
			if update.SessionId != "sess-fork" {
				t.Fatalf("captured update session id = %q, want sess-fork", update.SessionId)
			}
		}
		if proc.SessionID != "sess-new" {
			t.Fatalf("SessionID after fork = %q, want sess-new", proc.SessionID)
		}
		if got := proc.foreignSessionTrafficDropped(); got != 1 {
			t.Fatalf("foreignSessionTrafficDropped() = %d, want the stray id counted once", got)
		}
		if capsAfter := proc.CapsSnapshot(); !jsonEqual(t, capsBefore.ConfigOptions, capsAfter.ConfigOptions) {
			t.Fatalf("config options changed by fork: before %+v after %+v", capsBefore, capsAfter)
		}
		if events := drainObservedPrompt(proc, active); len(events) != 0 {
			t.Fatalf("bound prompt stream = %+v, want nothing from the clone", events)
		}
	})

	t.Run("Should refuse to fork when the agent does not advertise session/fork", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "prompt_capabilities_image", "", StartOpts{})
		t.Cleanup(func() {
			stopProcess(t, driver, proc)
		})

		_, err := proc.ForkSession(testutil.Context(t), proc.Cwd, nil)
		if !errors.Is(err, ErrAgentDoesNotSupportSession) {
			t.Fatalf("ForkSession() error = %v, want ErrAgentDoesNotSupportSession", err)
		}
	})
}

func beginObservedPrompt(t *testing.T, proc *AgentProcess) *activePromptState {
	t.Helper()
	active, err := proc.beginPrompt("turn-observed", 16)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	t.Cleanup(func() { proc.endPrompt(active) })
	return active
}

// drainObservedPrompt closes the observed prompt and returns every event it accepted.
func drainObservedPrompt(proc *AgentProcess, active *activePromptState) []AgentEvent {
	proc.endPrompt(active)
	var events []AgentEvent
	timeout := time.After(time.Second)
	for {
		select {
		case event, ok := <-active.events:
			if !ok {
				return events
			}
			events = append(events, event)
		case <-timeout:
			return append(events, AgentEvent{Type: EventTypeError, Error: "timed out draining prompt events"})
		}
	}
}

func handleTestSessionUpdate(t *testing.T, proc *AgentProcess, notification acpsdk.SessionNotification) {
	t.Helper()
	params, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("json.Marshal(notification) error = %v", err)
	}
	if err := proc.handleSessionUpdate(params); err != nil {
		t.Fatalf("handleSessionUpdate() error = %v", err)
	}
}

func jsonEqual(t *testing.T, left any, right any) bool {
	t.Helper()
	leftJSON, err := json.Marshal(left)
	if err != nil {
		t.Fatalf("json.Marshal(left) error = %v", err)
	}
	rightJSON, err := json.Marshal(right)
	if err != nil {
		t.Fatalf("json.Marshal(right) error = %v", err)
	}
	return bytes.Equal(leftJSON, rightJSON)
}
