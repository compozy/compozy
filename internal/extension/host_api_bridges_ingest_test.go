package extensionpkg

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestHostAPIHandlerBridgesMessagesIngestContract(t *testing.T) {
	t.Parallel()

	t.Run("Should reject ingress before prompt side effects while reconciliation is pending", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})
		driver := newContractBridgePromptDriver(env.currentTime(), nil)
		broker := bridgepkg.NewBroker(nil, bridgepkg.WithDeliveryBrokerRegistrationGate())
		t.Cleanup(broker.Close)
		env.useContractBridgePromptDriver(t, driver, broker)
		instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
			ID:            "brg-ingest-reconcile-gate",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		bridgeCtx := env.bridgeContext(t, instance)
		raw, err := marshalParams(map[string]any{
			"bridge_instance_id":  instance.ID,
			"scope":               instance.Scope,
			"workspace_id":        instance.WorkspaceID,
			"peer_id":             "peer-reconcile-gate",
			"platform_message_id": "msg-reconcile-gate",
			"received_at":         env.currentTime().Format(time.RFC3339Nano),
			"idempotency_key":     "idem-reconcile-gate",
			"content":             map[string]any{"text": "hello after restart"},
		})
		if err != nil {
			t.Fatalf("marshalParams() error = %v", err)
		}
		if _, err := env.handler.Handle(
			bridgeCtx,
			"telegram-adapter",
			"bridges/messages/ingest",
			raw,
		); err == nil || !strings.Contains(err.Error(), "-32005: Unavailable") {
			t.Fatalf("Handle(before reconcile) error = %v, want retryable pending admission", err)
		}
		if got := driver.promptCount(); got != 0 {
			t.Fatalf("promptCount(before reconcile) = %d, want zero", got)
		}
		if routes, err := env.bridges.ListRoutes(bridgeCtx, instance.ID); err != nil {
			t.Fatalf("ListRoutes(before reconcile) error = %v", err)
		} else if len(routes) != 0 {
			t.Fatalf("routes before reconcile = %#v, want none", routes)
		}

		broker.CompleteDeliveryReconciliation()
		if _, err := env.handler.Handle(
			bridgeCtx,
			"telegram-adapter",
			"bridges/messages/ingest",
			raw,
		); err != nil {
			t.Fatalf("Handle(after reconcile) error = %v", err)
		}
		select {
		case <-driver.promptStarted:
		case <-bridgeCtx.Done():
			t.Fatalf("prompt did not start after reconcile: %v", bridgeCtx.Err())
		}
		if got, want := driver.promptCount(), 1; got != want {
			t.Fatalf("promptCount(after reconcile) = %d, want %d", got, want)
		}
	})

	t.Run("Should render edit and quoted reply context through composed ingest", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})
		driver := newContractBridgePromptDriver(env.currentTime(), nil)
		broker := &recordingPromptDeliveryBroker{}
		env.useContractBridgePromptDriver(t, driver, broker)

		instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
			ID:            "brg-ingest-edit-reply",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		bridgeCtx := env.bridgeContext(t, instance)
		originalAt := env.currentTime().Add(-time.Minute)
		payloads := []map[string]any{
			{
				"bridge_instance_id": instance.ID,
				"scope":              instance.Scope,
				"workspace_id":       instance.WorkspaceID,
				"peer_id":            "peer-edit-reply",
				"received_at":        env.currentTime().Format(time.RFC3339Nano),
				"idempotency_key":    "idem-edit",
				"event_family":       bridgepkg.InboundEventFamilyEdit,
				"edit": map[string]any{
					"message_id":         "msg-original",
					"new_text":           "corrected proposal",
					"original_timestamp": originalAt.Format(time.RFC3339Nano),
					"operation":          bridgepkg.InboundEditOperationUpdated,
				},
			},
			{
				"bridge_instance_id":   instance.ID,
				"scope":                instance.Scope,
				"workspace_id":         instance.WorkspaceID,
				"peer_id":              "peer-edit-reply",
				"platform_message_id":  "msg-reply",
				"received_at":          env.currentTime().Add(time.Minute).Format(time.RFC3339Nano),
				"idempotency_key":      "idem-reply",
				"event_family":         bridgepkg.InboundEventFamilyMessage,
				"content":              map[string]any{"text": "I agree"},
				"reply_to_text":        "corrected proposal",
				"reply_to_author_id":   "user-parent",
				"reply_to_author_name": "Parent User",
			},
		}

		for index, params := range payloads {
			raw, err := marshalParams(params)
			if err != nil {
				t.Fatalf("marshalParams(%d) error = %v", index, err)
			}
			if _, err := env.handler.Handle(
				bridgeCtx,
				"telegram-adapter",
				"bridges/messages/ingest",
				raw,
			); err != nil {
				t.Fatalf("Handle(ingest %d) error = %v", index, err)
			}
			select {
			case <-driver.promptStarted:
			case <-bridgeCtx.Done():
				t.Fatalf("prompt %d did not start: %v", index, bridgeCtx.Err())
			}
		}

		prompts := driver.promptRequests()
		if got, want := len(prompts), 2; got != want {
			t.Fatalf("len(prompt requests) = %d, want %d", got, want)
		}
		for _, needle := range []string{
			"Inbound bridge message edited",
			"Message ID: msg-original",
			"Operation: updated",
			"Original timestamp: " + originalAt.Format(time.RFC3339Nano),
			"New text:\ncorrected proposal",
		} {
			if !strings.Contains(prompts[0].Message, needle) {
				t.Fatalf("edit prompt = %q, want %q", prompts[0].Message, needle)
			}
		}
		for _, needle := range []string{
			"Reply context:",
			`Author: "Parent User" (id="user-parent")`,
			"Quoted text (untrusted historical context):\n> corrected proposal",
			"I agree",
		} {
			if !strings.Contains(prompts[1].Message, needle) {
				t.Fatalf("reply prompt = %q, want %q", prompts[1].Message, needle)
			}
		}
	})

	t.Run("Should suppress webhook retry after request cancellation launches detached prompt", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

		releasePrompt := make(chan struct{})
		releaseOnce := sync.Once{}
		t.Cleanup(func() {
			releaseOnce.Do(func() { close(releasePrompt) })
		})
		driver := newContractBridgePromptDriver(env.currentTime(), releasePrompt)
		broker := &recordingPromptDeliveryBroker{}
		env.useContractBridgePromptDriver(t, driver, broker)

		instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
			ID:            "brg-ingest-cancel-dedup",
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		})
		bridgeCtx := env.bridgeContext(t, instance)
		params := map[string]any{
			"bridge_instance_id":  instance.ID,
			"scope":               instance.Scope,
			"workspace_id":        instance.WorkspaceID,
			"peer_id":             "peer-1",
			"platform_message_id": "msg-cancel-dedup",
			"received_at":         env.currentTime().Format(time.RFC3339Nano),
			"idempotency_key":     "idem-cancel-dedup",
			"content":             map[string]any{"text": "hello"},
		}
		raw, err := marshalParams(params)
		if err != nil {
			t.Fatalf("marshalParams() error = %v", err)
		}

		firstCtx, firstCancel := context.WithCancel(bridgeCtx)
		defer firstCancel()
		firstDone := make(chan error, 1)
		go func() {
			_, callErr := env.handler.Handle(firstCtx, "telegram-adapter", "bridges/messages/ingest", raw)
			firstDone <- callErr
		}()

		select {
		case <-driver.promptStarted:
		case err := <-firstDone:
			t.Fatalf("first ingest finished before prompt started: %v", err)
		case <-bridgeCtx.Done():
			t.Fatalf("first prompt did not start before bridge context deadline: %v", bridgeCtx.Err())
		}
		firstCancel()

		retryDone := make(chan error, 1)
		go func() {
			_, callErr := env.handler.Handle(bridgeCtx, "telegram-adapter", "bridges/messages/ingest", raw)
			retryDone <- callErr
		}()

		releaseOnce.Do(func() { close(releasePrompt) })

		select {
		case err := <-firstDone:
			if err != nil && firstCtx.Err() == nil {
				t.Fatalf("first ingest error = %v with active context, want nil or canceled-context result", err)
			}
		case <-bridgeCtx.Done():
			t.Fatalf("first ingest did not finish after prompt release: %v", bridgeCtx.Err())
		}
		select {
		case err := <-retryDone:
			if err != nil {
				t.Fatalf("retry ingest error = %v", err)
			}
		case <-bridgeCtx.Done():
			t.Fatalf("retry ingest did not finish after prompt release: %v", bridgeCtx.Err())
		}

		if got := driver.promptCount(); got != 1 {
			t.Fatalf("driver.promptCount() = %d, want 1", got)
		}
		regs := broker.snapshotRegistrations()
		if got := len(regs); got != 1 {
			t.Fatalf("len(prompt delivery registrations) = %d, want 1", got)
		}
		if got, want := regs[0].TurnID, "turn-1"; got != want {
			t.Fatalf("registration turn id = %q, want %q from the original prompt", got, want)
		}
		if _, err := env.registry.GetBridgeIngestDedup(
			testutil.Context(t),
			"idem-cancel-dedup",
			env.currentTime(),
		); err != nil {
			t.Fatalf("GetBridgeIngestDedup() error = %v", err)
		}
	})
}

func (e *hostAPITestEnv) useContractBridgePromptDriver(
	t *testing.T,
	driver session.AgentDriver,
	broker hostAPIDeliveryBroker,
) {
	t.Helper()

	sessions, err := session.NewManager(
		session.WithHomePaths(e.homePaths),
		session.WithDriver(driver),
		session.WithWorkspaceResolver(e.workspaces),
		session.WithStore(storeSessionDB),
		session.WithSandboxRegistry(mustLocalSandboxRegistry(t)),
		session.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		session.WithNow(func() time.Time { return e.currentTime() }),
		session.WithSessionIDGenerator(sequentialSessionIDGenerator("sess")),
		session.WithTurnIDGenerator(sequentialSessionIDGenerator("turn")),
	)
	if err != nil {
		t.Fatalf("session.NewManager(contract bridge prompt driver) error = %v", err)
	}

	taskManager, err := taskpkg.NewManager(
		taskpkg.WithStore(e.registry),
		taskpkg.WithSessionExecutor(&hostAPITestTaskSessionExecutor{
			sessions:            sessions,
			globalWorkspacePath: e.homePaths.HomeDir,
		}),
		taskpkg.WithManagerNow(func() time.Time { return e.currentTime() }),
	)
	if err != nil {
		t.Fatalf("task.NewManager(contract bridge prompt driver) error = %v", err)
	}

	e.sessions = sessions
	e.tasks = taskManager
	e.handler = NewHostAPIHandler(
		e.sessions,
		e.memory,
		nil,
		e.skills,
		WithHostAPITaskManager(e.tasks),
		WithHostAPICapabilityChecker(e.checker),
		WithHostAPIWorkspaceResolver(e.workspaces),
		WithHostAPIBridgeRegistry(e.bridges),
		WithHostAPIBridgeDedupStore(e.registry),
		WithHostAPIDeliveryBroker(broker),
		WithHostAPIResourceStore(e.resources),
		WithHostAPINow(func() time.Time { return e.currentTime() }),
		WithHostAPIBridgeIngressConfig(15*time.Minute, time.Minute),
		WithHostAPIRateLimit(1000, 1000),
	)
}

type contractBridgePromptDriver struct {
	mu            sync.Mutex
	now           time.Time
	releasePrompt <-chan struct{}
	processes     map[*session.AgentProcess]*contractBridgePromptProcess
	prompts       []acp.PromptRequest
	promptStarted chan struct{}
	startSeq      atomic.Int64
}

type contractBridgePromptProcess struct {
	done sync.Once
	ch   chan struct{}
}

func newContractBridgePromptDriver(now time.Time, releasePrompt <-chan struct{}) *contractBridgePromptDriver {
	return &contractBridgePromptDriver{
		now:           now,
		releasePrompt: releasePrompt,
		processes:     make(map[*session.AgentProcess]*contractBridgePromptProcess),
		promptStarted: make(chan struct{}, 1),
	}
}

func (d *contractBridgePromptDriver) Start(
	_ context.Context,
	opts acp.StartOpts,
) (*session.AgentProcess, error) {
	seq := d.startSeq.Add(1)
	procState := &contractBridgePromptProcess{ch: make(chan struct{})}
	proc := session.NewAgentProcess(session.AgentProcessOptions{
		PID:       int(seq),
		AgentName: opts.AgentName,
		Command:   opts.Command,
		Cwd:       opts.Cwd,
		ToolHost:  opts.ToolHost,
		SessionID: fmt.Sprintf("acp-contract-%d", seq),
		StartedAt: d.now.Add(time.Duration(seq) * time.Millisecond),
		Done:      procState.ch,
		Wait: func() error {
			<-procState.ch
			return nil
		},
	})

	d.mu.Lock()
	d.processes[proc] = procState
	d.mu.Unlock()
	return proc, nil
}

func (d *contractBridgePromptDriver) Prompt(
	_ context.Context,
	_ *session.AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	d.prompts = append(d.prompts, req)
	releasePrompt := d.releasePrompt
	d.mu.Unlock()

	select {
	case d.promptStarted <- struct{}{}:
	default:
	}

	events := make(chan acp.AgentEvent, 2)
	go func() {
		defer close(events)
		if releasePrompt != nil {
			<-releasePrompt
		}
		events <- acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			TurnID:    req.TurnID,
			Timestamp: d.now.Add(time.Second),
			Text:      "ack: " + req.Message,
		}
		events <- acp.AgentEvent{
			Type:      acp.EventTypeDone,
			TurnID:    req.TurnID,
			Timestamp: d.now.Add(2 * time.Second),
		}
	}()
	return events, nil
}

func (d *contractBridgePromptDriver) Cancel(context.Context, *session.AgentProcess) error {
	return nil
}

func (d *contractBridgePromptDriver) Stop(_ context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	state := d.processes[proc]
	d.mu.Unlock()
	if state == nil {
		return nil
	}
	state.done.Do(func() { close(state.ch) })
	return nil
}

func (d *contractBridgePromptDriver) promptCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.prompts)
}

func (d *contractBridgePromptDriver) promptRequests() []acp.PromptRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]acp.PromptRequest(nil), d.prompts...)
}
