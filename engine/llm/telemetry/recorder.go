package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RunRecorder exposes run lifecycle recording methods.
type RunRecorder interface {
	StartRun(ctx context.Context, meta RunMetadata) (context.Context, *Run, error)
	RecordEvent(ctx context.Context, evt *Event)
	RecordTool(ctx context.Context, entry *ToolLogEntry)
	CloseRun(ctx context.Context, run *Run, result RunResult) error
}

// recorder implements RunRecorder writing NDJSON artifacts.
type recorder struct {
	opts     Options
	runDir   string
	toolPath string

	toolMu   sync.Mutex
	toolFile *os.File
}

// NewRecorder builds a new run recorder using options.
func NewRecorder(opts *Options) (RunRecorder, error) {
	cfg := opts.clone()
	runDir := filepath.Join(cfg.ProjectRoot, cfg.RunDirName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, fmt.Errorf("create telemetry run dir: %w", err)
	}
	toolPath := filepath.Join(cfg.ProjectRoot, cfg.ToolLogFile)
	if err := os.MkdirAll(filepath.Dir(toolPath), 0o755); err != nil {
		return nil, fmt.Errorf("create telemetry tool dir: %w", err)
	}
	return &recorder{
		opts:     cfg,
		runDir:   runDir,
		toolPath: toolPath,
	}, nil
}

// StartRun initializes a run scope, returning derived context + handle.
func (r *recorder) StartRun(ctx context.Context, meta RunMetadata) (context.Context, *Run, error) {
	runID := uuid.New().String()
	runPath := filepath.Join(r.runDir, runID+".jsonl")
	file, err := os.OpenFile(runPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return ctx, nil, fmt.Errorf("open run transcript: %w", err)
	}
	run := &Run{
		id:                runID,
		metadata:          meta,
		path:              runPath,
		file:              file,
		captureContent:    r.opts.CaptureContent,
		redactContent:     r.opts.RedactContent,
		contextThresholds: append([]float64{}, r.opts.ContextWarningThresholds...),
		contextWarned:     make(map[float64]bool),
		startedAt:         time.Now(),
	}
	ctx = ContextWithRun(ctx, run)
	ctx = ContextWithRecorder(ctx, r)
	startEvt := Event{
		Stage:     "run_started",
		Severity:  SeverityInfo,
		Timestamp: run.startedAt,
		RunID:     runID,
		AgentID:   meta.AgentID,
		ActionID:  meta.ActionID,
		Workflow:  meta.WorkflowID,
		ExecID:    meta.ExecutionID,
	}
	run.record(&startEvt)
	return ctx, run, nil
}

// RecordEvent appends a run event to the transcript.
func (r *recorder) RecordEvent(ctx context.Context, evt *Event) {
	run, ok := RunFromContext(ctx)
	if !ok {
		return
	}
	run.record(evt)
}

// RecordEvent emits a run event when a recorder is present on the context.
func RecordEvent(ctx context.Context, evt *Event) {
	rec := recorderFromContext(ctx)
	if rec == nil {
		return
	}
	rec.RecordEvent(ctx, evt)
}

// RecordTool appends a tool log event to the shared NDJSON log.
func (r *recorder) RecordTool(ctx context.Context, entry *ToolLogEntry) {
	run, _ := RunFromContext(ctx)
	ts := time.Now().UTC()
	payload := map[string]any{
		"timestamp":    ts.Format(time.RFC3339Nano),
		"status":       entry.Status,
		"tool_call_id": entry.ToolCallID,
		"tool_name":    entry.ToolName,
		"duration_ms":  float64(entry.Duration) / float64(time.Millisecond),
		"metadata":     entry.Metadata,
		"redacted":     entry.Redacted,
	}
	if run != nil {
		payload["run_id"] = run.id
		payload["agent_id"] = run.metadata.AgentID
		payload["action_id"] = run.metadata.ActionID
		payload["workflow_id"] = run.metadata.WorkflowID
		payload["execution_id"] = run.metadata.ExecutionID
	}
	if !entry.Redacted {
		payload["input"] = entry.Input
		payload["output"] = entry.Output
		payload["error"] = entry.Error
	} else if entry.Error != "" {
		payload["error"] = entry.Error
	}
	r.toolMu.Lock()
	defer r.toolMu.Unlock()
	if r.toolFile == nil {
		file, err := os.OpenFile(r.toolPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		r.toolFile = file
	}
	if err := writeJSONLine(r.toolFile, payload); err != nil {
		return
	}
}

// RecordTool stores the provided tool log entry using the recorder from context.
func RecordTool(ctx context.Context, entry *ToolLogEntry) {
	rec := recorderFromContext(ctx)
	if rec == nil {
		return
	}
	rec.RecordTool(ctx, entry)
}

// CloseRun finalizes the run, recording completion state.
func (r *recorder) CloseRun(_ context.Context, run *Run, result RunResult) error {
	if run == nil {
		return nil
	}
	defer run.close()
	event := Event{
		Severity:  SeverityInfo,
		Timestamp: time.Now(),
		RunID:     run.id,
		AgentID:   run.metadata.AgentID,
		ActionID:  run.metadata.ActionID,
		Workflow:  run.metadata.WorkflowID,
		ExecID:    run.metadata.ExecutionID,
	}
	if result.Success {
		event.Stage = "run_completed"
		event.Metadata = result.Summary
	} else {
		event.Stage = "run_failed"
		event.Severity = SeverityError
		if result.Summary == nil {
			result.Summary = map[string]any{}
		}
		if result.Error != nil {
			result.Summary["error"] = result.Error.Error()
		}
		event.Metadata = result.Summary
	}
	run.record(&event)
	return nil
}

// Run represents a single orchestration execution.
type Run struct {
	id             string
	metadata       RunMetadata
	path           string
	file           *os.File
	mu             sync.Mutex
	captureContent bool
	redactContent  bool

	contextThresholds []float64
	contextWarned     map[float64]bool

	startedAt time.Time
}

func (r *Run) record(evt *Event) {
	if evt == nil {
		return
	}
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}
	evt.RunID = r.id
	if evt.AgentID == "" {
		evt.AgentID = r.metadata.AgentID
	}
	if evt.ActionID == "" {
		evt.ActionID = r.metadata.ActionID
	}
	if evt.Workflow == "" {
		evt.Workflow = r.metadata.WorkflowID
	}
	if evt.ExecID == "" {
		evt.ExecID = r.metadata.ExecutionID
	}
	if err := writeJSONLine(r.file, evt); err != nil {
		return
	}
}

func (r *Run) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		_ = r.file.Close()
		r.file = nil
	}
}

func (r *Run) thresholdTriggered(pct float64) (float64, bool) {
	for _, threshold := range r.contextThresholds {
		if pct >= threshold && !r.contextWarned[threshold] {
			r.contextWarned[threshold] = true
			return threshold, true
		}
	}
	return 0, false
}

// CanCapture returns true when raw content capture is enabled.
func (r *Run) CanCapture() bool {
	return r != nil && r.captureContent && !r.redactContent
}

func writeJSONLine(file *os.File, payload any) error {
	if file == nil {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = file.Write(append(data, '\n'))
	return err
}
