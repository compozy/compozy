package extensiontest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/session"
)

// ScriptedPromptEvent is one deterministic agent event emitted by the harness
// driver when the host creates a session prompt.
type ScriptedPromptEvent struct {
	Type       string
	Text       string
	Error      string
	ToolName   string
	ToolCallID string
	ToolInput  json.RawMessage
	ToolError  bool
	Delay      time.Duration
}

// ScriptedPromptDriver is a deterministic in-process session driver used by
// the adapter harness instead of a real ACP subprocess.
type ScriptedPromptDriver struct {
	now       time.Time
	script    []ScriptedPromptEvent
	processes map[*session.AgentProcess]*scriptedPromptProcess
	prompts   []acp.PromptRequest
	mu        sync.Mutex
	startSeq  atomic.Int64
}

type scriptedPromptProcess struct {
	done sync.Once
	ch   chan struct{}
}

// NewScriptedPromptDriver constructs a session driver that replays the provided
// agent events for every prompt.
func NewScriptedPromptDriver(now time.Time, script []ScriptedPromptEvent) *ScriptedPromptDriver {
	return &ScriptedPromptDriver{
		now:       now,
		script:    append([]ScriptedPromptEvent(nil), script...),
		processes: make(map[*session.AgentProcess]*scriptedPromptProcess),
	}
}

// Start implements session.AgentDriver.
func (d *ScriptedPromptDriver) Start(
	_ context.Context,
	opts acp.StartOpts,
) (*session.AgentProcess, error) {
	seq := d.startSeq.Add(1)
	procState := &scriptedPromptProcess{ch: make(chan struct{})}
	proc := session.NewAgentProcess(session.AgentProcessOptions{
		PID:       int(seq),
		AgentName: opts.AgentName,
		Command:   opts.Command,
		Cwd:       opts.Cwd,
		SessionID: fmt.Sprintf("acp-%d", seq),
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

// Prompt implements session.AgentDriver.
func (d *ScriptedPromptDriver) Prompt(
	ctx context.Context,
	_ *session.AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	d.prompts = append(d.prompts, req)
	script := append([]ScriptedPromptEvent(nil), d.script...)
	startedAt := d.now
	d.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}

	events := make(chan acp.AgentEvent, len(script))
	go func() {
		defer close(events)
		for index, item := range script {
			if !waitForScriptedPromptDelay(ctx, item.Delay) {
				return
			}
			event := (acp.AgentEvent{
				Type:       item.Type,
				TurnID:     req.TurnID,
				Timestamp:  startedAt.Add(time.Duration(index+1) * time.Millisecond),
				Text:       item.Text,
				Error:      item.Error,
				ToolCallID: item.ToolCallID,
			}).WithTool(item.ToolName, item.ToolInput, item.ToolError)
			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return events, nil
}

func waitForScriptedPromptDelay(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// Cancel implements session.AgentDriver.
func (d *ScriptedPromptDriver) Cancel(context.Context, *session.AgentProcess) error {
	return nil
}

// Stop implements session.AgentDriver.
func (d *ScriptedPromptDriver) Stop(_ context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	state := d.processes[proc]
	d.mu.Unlock()
	if state == nil {
		return nil
	}
	state.done.Do(func() { close(state.ch) })
	return nil
}
