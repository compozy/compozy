package terminal

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/redact"
	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
	"github.com/compozy/compozy/internal/toolruntime"
)

const (
	defaultExecYield = 10 * time.Second
	minimumExecYield = 250 * time.Millisecond
	maximumExecYield = 30 * time.Second
	execCaptureLimit = 4 << 20
)

type artifactWriter interface {
	WriteArtifact(context.Context, string, string, string, *ID, []byte, time.Time) (SpillRef, error)
}

type queuedJournal interface {
	RecordQueued(context.Context, Info, CommandRow) error
}

type execRun struct {
	item       *session
	key        terminalKey
	commandID  string
	startedAt  time.Time
	journaled  chan error
	registered atomic.Bool
	decision   chan struct{}
	decideOnce sync.Once
}

func (m *Service) Exec(ctx context.Context, request ExecRequest) (*ExecResult, error) {
	if ctx == nil {
		return nil, errors.New("terminal: exec context is required")
	}
	if err := m.admit(ctx, request.WS, request.Actor); err != nil {
		return nil, err
	}
	argv := append([]string{strings.TrimSpace(request.Command)}, request.Args...)
	if argv[0] == "" {
		return nil, errors.New("terminal: exec command is required")
	}
	request, err := m.authorizeExec(ctx, request, argv)
	if err != nil {
		return nil, err
	}
	yield, err := execYieldDuration(request.YieldMs)
	if err != nil {
		return nil, err
	}
	run, err := m.startExec(ctx, request, argv)
	if err != nil {
		return nil, err
	}
	if request.Visible {
		if err := m.publishExec(ctx, run, request); err != nil {
			return nil, errors.Join(err, cleanupExecRun(ctx, run.item, err))
		}
		run.settlePublication()
	}
	run.item.start()
	go m.recordExec(context.WithoutCancel(ctx), run, request, argv)
	timer := time.NewTimer(yield)
	defer timer.Stop()
	select {
	case <-run.item.done:
		run.settlePublication()
		if journalErr := <-run.journaled; journalErr != nil {
			return nil, journalErr
		}
		result, resultErr := m.execResult(ctx, run, request)
		if !request.Visible {
			if closeErr := run.item.proc.Close(); closeErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("terminal: close completed exec: %w", closeErr))
			}
		}
		return result, resultErr
	case <-timer.C:
		if !request.Visible {
			if err := m.publishExec(ctx, run, request); err != nil {
				run.settlePublication()
				return nil, errors.Join(err, cleanupExecRun(ctx, run.item, err))
			}
		}
		run.settlePublication()
		id := run.item.Info().ID
		return &ExecResult{StillRunning: true, TerminalID: &id, Untrusted: true, CommandID: run.commandID}, nil
	case <-ctx.Done():
		run.settlePublication()
		return nil, errors.Join(ctx.Err(), cleanupExecRun(ctx, run.item, ctx.Err()))
	}
}

func (m *Service) authorizeExec(ctx context.Context, request ExecRequest, argv []string) (ExecRequest, error) {
	if request.Actor.Kind != ActorKindAgent {
		return request, nil
	}
	switch strings.TrimSpace(request.Approval) {
	case "approved_once", "approved_always", "allowlisted":
		return request, nil
	}
	classification := ClassifyArgv(argv, nil)
	if classification.Verdict == CommandVerdictDenied {
		return ExecRequest{}, &Error{
			Code: "approval_rejected", Message: "terminal command is blocked by the irreversible-operation policy",
			Err: ErrApprovalRejected,
		}
	}
	if m.execApprovals == nil {
		return ExecRequest{}, &Error{
			Code: errorCodeApprovalRequired, Message: "agent terminal execution requires approval",
			Err: ErrApprovalRequired,
		}
	}
	approval, err := m.execApprovals.AuthorizeTerminalExec(ctx, request, classification)
	if err != nil {
		return ExecRequest{}, err
	}
	request.Approval = strings.TrimSpace(approval)
	if request.Approval == "" {
		return ExecRequest{}, &Error{
			Code: "approval_required", Message: "agent terminal execution requires approval",
			Err: ErrApprovalRequired,
		}
	}
	return request, nil
}

func execYieldDuration(value int) (time.Duration, error) {
	if value == 0 {
		return defaultExecYield, nil
	}
	duration := time.Duration(value) * time.Millisecond
	if err := ValidateExecYieldDuration(duration); err != nil {
		return 0, err
	}
	return duration, nil
}

// ValidateExecYieldDuration enforces the public terminal exec yield range.
func ValidateExecYieldDuration(duration time.Duration) error {
	if duration < minimumExecYield || duration > maximumExecYield {
		return &Error{
			Code:    "timeout_out_of_range",
			Message: "terminal exec yield_ms must be between 250 and 30000",
			Err:     ErrUnsupported,
		}
	}
	return nil
}

func (m *Service) startExec(ctx context.Context, request ExecRequest, argv []string) (*execRun, error) {
	cwd, workspaceID, err := m.resolveOpenWorkspace(ctx, request.WS, request.Cwd, request.Actor.ProfileID)
	if err != nil {
		return nil, err
	}
	settings, err := m.settings(ctx, workspaceID, request.Actor.ProfileID)
	if err != nil {
		return nil, fmt.Errorf("terminal: resolve exec settings: %w", err)
	}
	if err := validateSettings(settings); err != nil {
		return nil, err
	}
	id, err := newTerminalID(m.entropy)
	if err != nil {
		return nil, err
	}
	commandID, err := newCommandID(m.entropy)
	if err != nil {
		return nil, err
	}
	nonce, err := newMarkerNonce(m.entropy)
	if err != nil {
		return nil, err
	}
	mode := ModePipe
	ptyMode := terminalpty.ModePipe
	if request.Visible {
		if !request.Capabilities.Interactive {
			return nil, &Error{
				Code:    "terminal_interactive_unavailable",
				Message: "Interactive terminals are not available on this platform yet — command execution is.",
				Err:     ErrInteractive,
			}
		}
		mode = ModePTY
		ptyMode = terminalpty.ModePTY
	}
	title := SanitizeTitle(strings.Join(argv, " "))
	spec := ProcSpec{
		Argv:        argv,
		Cwd:         cwd,
		Env:         cloneStringMap(request.Env),
		Cols:        80,
		Rows:        24,
		Mode:        ptyMode,
		MarkerNonce: nonce,
	}
	proc, err := m.pty.Start(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("terminal: start exec %q: %w", argv[0], err)
	}
	info := ownedInfo(Info{
		ID:           id,
		WS:           workspaceID,
		ProfileID:    request.Actor.ProfileID,
		Title:        title,
		Shell:        argv[0],
		Cwd:          cwd,
		Mode:         mode,
		State:        terminalStateRunning,
		Controller:   cloneActor(&request.Actor),
		Capabilities: request.Capabilities,
		CreatedAt:    m.now(),
	}, request.Actor)
	item := newSession(m, proc, info, settings, nonce, m.eventProfileName(ctx, request.Actor.ProfileID), 80, 24, true)
	item.captureOutput = true
	processRecord, err := m.processRegistration(ctx, item, spec)
	if err != nil {
		return nil, errors.Join(err, cleanupUnregisteredProcess(proc))
	}
	item.processRecord = processRecord
	return &execRun{
		item: item, key: terminalKey{workspaceID: workspaceID, profileID: request.Actor.ProfileID, id: id},
		commandID: commandID, startedAt: m.now(), journaled: make(chan error, 1), decision: make(chan struct{}),
	}, nil
}

func (m *Service) publishExec(ctx context.Context, run *execRun, request ExecRequest) error {
	settings := run.item.settings(ctx)
	release, err := m.reserveAdmission(OpenRequest{WS: run.key.workspaceID, Actor: request.Actor}, settings)
	if err != nil {
		return err
	}
	defer release()
	if err := m.insert(run.key, run.item); err != nil {
		return err
	}
	m.registerJournalTerminal(run.item)
	run.registered.Store(true)
	info := run.item.Info()
	m.events.Notify(ctx, Event{
		Kind: EventKindOpened, WorkspaceID: info.WS, ProfileID: info.ProfileID, ProfileName: run.item.profileName,
		TerminalID: info.ID, Actor: request.Actor, Info: &info,
		Detail: &EventDetail{Mode: info.Mode, Cwd: info.Cwd, Title: info.Title}, At: m.now(),
	})
	return nil
}

func (m *Service) recordExec(ctx context.Context, run *execRun, request ExecRequest, argv []string) {
	<-run.item.done
	<-run.decision
	if m.journal == nil {
		run.journaled <- nil
		return
	}
	info := run.item.Info()
	duration := m.now().Sub(run.startedAt).Milliseconds()
	digest := argvDigest(argv)
	approval := terminalApprovalLabel(request.Approval)
	if request.Actor.Kind == ActorKindHuman {
		approval = "human"
	}
	_, captureTruncated, outputBytes := run.item.capturedOutput()
	row := CommandRow{
		ID: run.commandID, ProfileID: info.ProfileID, ProfileName: run.item.profileName, Actor: request.Actor,
		Command: redact.String(strings.Join(argv, " ")), ArgvDigest: &digest, Cwd: info.Cwd, StartedAt: run.startedAt,
		DurationMs: &duration, ExitCause: info.Exit.Cause, ExitCode: info.Exit.Code, ExitSignal: info.Exit.Signal,
		DetectedBy: "exact", Approval: approval, OutputBytes: outputBytes, Truncated: captureTruncated,
	}
	if run.registered.Load() {
		id := info.ID
		row.TerminalID = &id
	}
	if queued, ok := m.journal.(queuedJournal); ok {
		run.journaled <- queued.RecordQueued(ctx, info, row)
		return
	}
	run.journaled <- m.journal.Record(ctx, info.WS, row)
}

func terminalApprovalLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "approved_once", "approved_always", "allowlisted":
		return strings.TrimSpace(value)
	default:
		return "none"
	}
}

func (r *execRun) settlePublication() {
	r.decideOnce.Do(func() { close(r.decision) })
}

func (m *Service) execResult(ctx context.Context, run *execRun, request ExecRequest) (*ExecResult, error) {
	info := run.item.Info()
	content, captureTruncated, _ := run.item.capturedOutput()
	output, truncated, err := shapeOutput(modelFacingOutput(content), request.Output)
	if err != nil {
		return nil, err
	}
	result := &ExecResult{
		ExitCode: info.Exit.Code, Signal: info.Exit.Signal, Output: output,
		Truncated: truncated || captureTruncated, Untrusted: true,
		DurationMs: m.now().Sub(run.startedAt).Milliseconds(), CommandID: run.commandID,
	}
	if run.registered.Load() {
		id := info.ID
		result.TerminalID = &id
	}
	if truncated || captureTruncated {
		writer, ok := m.journal.(artifactWriter)
		if ok {
			spill, err := writer.WriteArtifact(
				ctx,
				info.WS,
				info.ProfileID,
				run.commandID,
				result.TerminalID,
				content,
				m.now().Add(infoSettingsRetention(run.item)),
			)
			if err != nil {
				return nil, fmt.Errorf("terminal: preserve exec spill: %w", err)
			}
			result.Spill = &spill
		}
	}
	return result, nil
}

func infoSettingsRetention(item *session) time.Duration {
	item.mu.RLock()
	defer item.mu.RUnlock()
	return time.Duration(item.policy.RecordingRetentionDays) * 24 * time.Hour
}

func newCommandID(entropy io.Reader) (string, error) {
	raw := make([]byte, 3)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return "", fmt.Errorf("terminal: generate command id: %w", err)
	}
	return "cmd-" + hex.EncodeToString(raw), nil
}

func cleanupExecRun(ctx context.Context, item *session, cause error) error {
	if item == nil {
		return nil
	}
	cleanupErr := cleanupUnregisteredProcess(item.proc)
	var completeErr error
	if item.processRecord != nil {
		completeErr = item.processRecord.Complete(
			context.WithoutCancel(ctx),
			toolruntime.ProcessCompletion{Err: cause, Error: "terminal exec rollback"},
		)
	}
	return errors.Join(cleanupErr, completeErr)
}
