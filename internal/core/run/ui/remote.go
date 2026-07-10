package ui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const (
	remoteReconnectDelay     = 100 * time.Millisecond
	remoteRunStatusRunning   = "running"
	remoteRunStatusPausing   = "pausing"
	remoteRunStatusPaused    = "paused"
	remoteRunStatusRetrying  = "retrying"
	remoteRunStatusCompleted = "completed"
	remoteRunStatusFailed    = "failed"
	remoteRunStatusCanceled  = "canceled"
	remoteRunStatusCrashed   = "crashed"
)

var setupRemoteUISession = Setup

type remoteFollowState struct {
	currentStream apiclient.RunStream
	itemCh        <-chan apiclient.RunStreamItem
	errCh         <-chan error
	lastCursor    apicore.StreamCursor
}

// RemoteAttachOptions configures a daemon-backed UI attach session.
type RemoteAttachOptions struct {
	Snapshot          apicore.RunSnapshot
	Config            *config
	WorkspaceRoot     string
	OwnerSession      bool
	LoadSnapshot      func(context.Context) (apicore.RunSnapshot, error)
	OpenStream        func(context.Context, apicore.StreamCursor) (apiclient.RunStream, error)
	PauseRunJob       func(context.Context, string, string) (apicore.RunJobControlResponse, error)
	SendRunJobMessage func(
		context.Context,
		string,
		string,
		apicore.RunJobMessageRequest,
	) (apicore.RunJobControlResponse, error)
}

// AttachRemote boots the Bubble Tea cockpit from a daemon snapshot and then follows the daemon stream.
func AttachRemote(ctx context.Context, opts RemoteAttachOptions) (Session, error) {
	jobs, initialMsgs := remoteSnapshotBootstrap(opts.Snapshot)

	cfg := opts.Config
	if cfg == nil {
		cfg = &config{}
	}
	localCfg := *cfg
	localCfg.DetachOnly = !opts.OwnerSession
	localCfg.DaemonOwned = true
	localCfg.RunID = firstNonEmpty(strings.TrimSpace(localCfg.RunID), strings.TrimSpace(opts.Snapshot.Run.RunID))
	if workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot); workspaceRoot != "" {
		localCfg.WorkspaceRoot = workspaceRoot
	}

	session := setupRemoteUISession(ctx, jobs, &localCfg, nil, true)
	if session == nil {
		return nil, errors.New("remote attach ui session is required")
	}
	installRemoteJobControlHandler(session, opts, localCfg.RunID)
	for _, msg := range initialMsgs {
		session.Enqueue(msg)
	}

	if isTerminalRunStatus(opts.Snapshot.Run.Status) || opts.OpenStream == nil {
		return session, nil
	}

	if err := ensureInitialRemoteStream(ctx, opts, session); err != nil {
		session.Shutdown()
		return nil, err
	}
	return session, nil
}

func installRemoteJobControlHandler(session Session, opts RemoteAttachOptions, runID string) {
	if session == nil {
		return
	}
	if handler := newRemoteJobControlHandler(runID, opts.PauseRunJob, opts.SendRunJobMessage); handler != nil {
		session.SetJobControlHandler(handler)
	}
}

func newRemoteJobControlHandler(
	runID string,
	pauseRunJob func(context.Context, string, string) (apicore.RunJobControlResponse, error),
	sendRunJobMessage func(
		context.Context,
		string,
		string,
		apicore.RunJobMessageRequest,
	) (apicore.RunJobControlResponse, error),
) func(context.Context, uiJobControlRequest) (model.JobControlResponse, error) {
	if pauseRunJob == nil || sendRunJobMessage == nil {
		return nil
	}
	return func(ctx context.Context, req uiJobControlRequest) (model.JobControlResponse, error) {
		resolvedRunID := firstNonEmpty(strings.TrimSpace(req.RunID), strings.TrimSpace(runID))
		switch req.Action {
		case uiJobControlPause:
			resp, err := pauseRunJob(ctx, resolvedRunID, req.JobID)
			return modelJobControlResponse(resp), err
		case uiJobControlMessage:
			resp, err := sendRunJobMessage(ctx, resolvedRunID, req.JobID, apicore.RunJobMessageRequest{
				Message: req.Message,
			})
			return modelJobControlResponse(resp), err
		default:
			return model.JobControlResponse{}, model.ErrJobControlConflict
		}
	}
}

func modelJobControlResponse(resp apicore.RunJobControlResponse) model.JobControlResponse {
	return model.JobControlResponse{
		RunID:     resp.RunID,
		JobID:     resp.JobID,
		Index:     resp.Index,
		Status:    model.JobControlStatus(resp.Status),
		SessionID: resp.SessionID,
		MessageID: resp.MessageID,
	}
}

func ensureInitialRemoteStream(ctx context.Context, opts RemoteAttachOptions, session Session) error {
	cursor := streamCursorOrZero(opts.Snapshot.NextCursor)
	stream, err := opts.OpenStream(ctx, cursor)
	if err != nil {
		return fmt.Errorf("open remote run stream: %w", err)
	}
	go followRemoteRun(ctx, session, opts, stream, cursor)
	return nil
}

func followRemoteRun(
	ctx context.Context,
	session Session,
	opts RemoteAttachOptions,
	stream apiclient.RunStream,
	cursor apicore.StreamCursor,
) {
	state := newRemoteFollowState(stream, cursor)
	defer func() {
		closeRemoteRunStream(state.currentStream)
	}()

	for {
		var stop bool
		state, stop = ensureRemoteFollowState(ctx, session, opts, state)
		if stop {
			return
		}
		if state.currentStream == nil {
			continue
		}

		state, stop = waitForRemoteRunUpdate(ctx, session, opts, state)
		if stop {
			return
		}
	}
}

func newRemoteFollowState(stream apiclient.RunStream, cursor apicore.StreamCursor) remoteFollowState {
	itemCh, errCh := remoteStreamChannels(stream)
	return remoteFollowState{
		currentStream: stream,
		itemCh:        itemCh,
		errCh:         errCh,
		lastCursor:    cursor,
	}
}

func ensureRemoteFollowState(
	ctx context.Context,
	session Session,
	opts RemoteAttachOptions,
	state remoteFollowState,
) (remoteFollowState, bool) {
	if state.currentStream != nil {
		return state, false
	}
	reconnected, nextCursor, stop := reopenRemoteRunStream(ctx, opts, state.lastCursor)
	if stop {
		return state, true
	}
	if reconnected == nil {
		return state, false
	}
	state.currentStream = reconnected
	state.lastCursor = nextCursor
	state.itemCh, state.errCh = remoteStreamChannels(reconnected)
	session.Enqueue(remoteConnectionStatusMsg{Reconnecting: false})
	return state, false
}

func waitForRemoteRunUpdate(
	ctx context.Context,
	session Session,
	opts RemoteAttachOptions,
	state remoteFollowState,
) (remoteFollowState, bool) {
	select {
	case <-ctx.Done():
		return state, true
	case err, ok := <-state.errCh:
		return handleRemoteRunStreamError(ctx, session, opts, state, err, ok)
	case item, ok := <-state.itemCh:
		return handleRemoteRunStreamItem(ctx, session, opts, state, item, ok)
	}
}

func handleRemoteRunStreamError(
	ctx context.Context,
	session Session,
	opts RemoteAttachOptions,
	state remoteFollowState,
	err error,
	ok bool,
) (remoteFollowState, bool) {
	if !ok {
		state.errCh = nil
		if state.itemCh != nil {
			return state, false
		}
		return handleRemoteRunEOF(ctx, session, opts, state)
	}
	if err == nil {
		return state, false
	}
	session.Enqueue(remoteConnectionStatusMsg{Reconnecting: true})
	return resetRemoteFollowState(state), false
}

func handleRemoteRunStreamItem(
	ctx context.Context,
	session Session,
	opts RemoteAttachOptions,
	state remoteFollowState,
	item apiclient.RunStreamItem,
	ok bool,
) (remoteFollowState, bool) {
	if !ok {
		state.itemCh = nil
		if state.errCh != nil {
			return state, false
		}
		return handleRemoteRunEOF(ctx, session, opts, state)
	}

	if item.Heartbeat != nil {
		if item.Heartbeat.Cursor.Sequence > state.lastCursor.Sequence {
			if snapshot, stop := terminalRemoteSnapshot(ctx, opts); stop {
				enqueueRemoteTerminalSnapshot(session, snapshot)
				return state, true
			}
			session.Enqueue(remoteConnectionStatusMsg{Reconnecting: true})
			return resetRemoteFollowState(state), false
		}
		state.lastCursor = remoteMaxCursor(state.lastCursor, item.Heartbeat.Cursor)
		return state, false
	}
	if item.Overflow != nil {
		if snapshot, stop := terminalRemoteSnapshot(ctx, opts); stop {
			enqueueRemoteTerminalSnapshot(session, snapshot)
			return state, true
		}
		session.Enqueue(remoteConnectionStatusMsg{Reconnecting: true})
		return resetRemoteFollowState(state), false
	}
	if item.Event == nil {
		return state, false
	}

	state.lastCursor = apicore.CursorFromEvent(*item.Event)
	session.Enqueue(*item.Event)
	if isTerminalRunEvent(item.Event.Kind) {
		return state, true
	}
	return state, false
}

func handleRemoteRunEOF(
	ctx context.Context,
	session Session,
	opts RemoteAttachOptions,
	state remoteFollowState,
) (remoteFollowState, bool) {
	state = resetRemoteFollowState(state)
	if snapshot, stop := terminalRemoteSnapshot(ctx, opts); stop {
		enqueueRemoteTerminalSnapshot(session, snapshot)
		return state, true
	}
	session.Enqueue(remoteConnectionStatusMsg{Reconnecting: true})
	return state, false
}

func resetRemoteFollowState(state remoteFollowState) remoteFollowState {
	closeRemoteRunStream(state.currentStream)
	state.currentStream = nil
	state.itemCh = nil
	state.errCh = nil
	return state
}

func closeRemoteRunStream(stream apiclient.RunStream) {
	if stream == nil {
		return
	}
	_ = stream.Close()
}

func remoteMaxCursor(current apicore.StreamCursor, next apicore.StreamCursor) apicore.StreamCursor {
	if next.Sequence > current.Sequence {
		return next
	}
	return current
}

func reopenRemoteRunStream(
	ctx context.Context,
	opts RemoteAttachOptions,
	lastCursor apicore.StreamCursor,
) (apiclient.RunStream, apicore.StreamCursor, bool) {
	timer := time.NewTimer(remoteReconnectDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, lastCursor, true
	case <-timer.C:
	}

	stream, err := opts.OpenStream(ctx, lastCursor)
	if err == nil {
		return stream, lastCursor, false
	}
	return nil, lastCursor, ctx.Err() != nil
}

func shouldStopAfterRemoteEOF(
	ctx context.Context,
	opts RemoteAttachOptions,
) bool {
	_, stop := terminalRemoteSnapshot(ctx, opts)
	return stop
}

func terminalRemoteSnapshot(
	ctx context.Context,
	opts RemoteAttachOptions,
) (apicore.RunSnapshot, bool) {
	if opts.LoadSnapshot == nil {
		return apicore.RunSnapshot{}, false
	}
	snapshot, err := opts.LoadSnapshot(ctx)
	if err != nil {
		return apicore.RunSnapshot{}, false
	}
	if !isTerminalRunStatus(snapshot.Run.Status) {
		return snapshot, false
	}
	return snapshot, true
}

func enqueueRemoteTerminalSnapshot(session Session, snapshot apicore.RunSnapshot) {
	if session == nil {
		return
	}
	kind, ok := remoteTerminalEventKind(snapshot.Run.Status)
	if !ok {
		return
	}
	session.Enqueue(events.Event{RunID: snapshot.Run.RunID, Kind: kind})
}

func remoteTerminalEventKind(status string) (events.EventKind, bool) {
	switch strings.TrimSpace(status) {
	case remoteRunStatusCompleted:
		return events.EventKindRunCompleted, true
	case remoteRunStatusFailed:
		return events.EventKindRunFailed, true
	case remoteRunStatusCanceled:
		return events.EventKindRunCancelled, true
	case remoteRunStatusCrashed:
		return events.EventKindRunCrashed, true
	default:
		return "", false
	}
}

func remoteSnapshotBootstrap(snapshot apicore.RunSnapshot) ([]job, []uiMsg) {
	states := append([]apicore.RunJobState(nil), snapshot.Jobs...)
	sort.Slice(states, func(i, j int) bool {
		if states[i].Index == states[j].Index {
			return states[i].UpdatedAt.Before(states[j].UpdatedAt)
		}
		return states[i].Index < states[j].Index
	})

	resultJobs := make([]job, 0, len(states))
	resultMsgs := make([]uiMsg, 0, len(states)*4+1)
	if status := strings.TrimSpace(snapshot.Run.Status); status != "" {
		resultMsgs = append(resultMsgs, runStatusMsg{Status: status})
	}
	for _, state := range states {
		jb, msgs := remoteJobBootstrap(state)
		resultJobs = append(resultJobs, jb)
		resultMsgs = append(resultMsgs, msgs...)
	}

	if snapshot.Shutdown != nil {
		resultMsgs = append(resultMsgs, shutdownStatusMsg{
			State: shutdownState{
				Phase:       shutdownPhase(snapshot.Shutdown.Phase),
				Source:      shutdownSource(snapshot.Shutdown.Source),
				RequestedAt: snapshot.Shutdown.RequestedAt,
				DeadlineAt:  snapshot.Shutdown.DeadlineAt,
			},
		})
	}
	return resultJobs, resultMsgs
}

func remoteJobBootstrap(state apicore.RunJobState) (job, []uiMsg) {
	summary := state.Summary
	jb := remoteBootstrapJob(state, summary)
	msgs := make([]uiMsg, 0, 4)
	msgs = append(msgs, remoteBootstrapLifecycleMsgs(state.Index, state.Status, summary)...)
	msgs = append(msgs, remoteBootstrapSummaryMsgs(state.Index, summary)...)
	msgs = append(msgs, remoteBootstrapTerminalMsgs(state.Index, state.Status, summary)...)
	return jb, msgs
}

func remoteBootstrapJob(state apicore.RunJobState, summary *apicore.RunJobSummary) job {
	jb := job{SafeName: state.JobID}
	if summary == nil {
		return jb
	}
	jb.CodeFiles = append([]string(nil), summary.CodeFiles...)
	if len(jb.CodeFiles) == 0 && summary.CodeFile != "" {
		jb.CodeFiles = []string{summary.CodeFile}
	}
	jb.TaskNumber = summary.TaskNumber
	jb.TaskTitle = summary.TaskTitle
	jb.TaskType = summary.TaskType
	jb.SafeName = firstNonEmpty(summary.SafeName, jb.SafeName)
	jb.IDE = summary.IDE
	jb.Model = summary.Model
	jb.ReasoningEffort = summary.ReasoningEffort
	jb.OutLog = summary.OutLog
	jb.ErrLog = summary.ErrLog
	jb.Groups = remoteIssueGroups(summary.Issues)
	return jb
}

func remoteBootstrapLifecycleMsgs(index int, status string, summary *apicore.RunJobSummary) []uiMsg {
	switch status {
	case remoteRunStatusRunning, remoteRunStatusCompleted, remoteRunStatusFailed, remoteRunStatusCanceled:
		return []uiMsg{remoteStartedMsg(index, summary)}
	case remoteRunStatusPausing:
		return []uiMsg{remoteStartedMsg(index, summary), jobPausingMsg{Index: index}}
	case remoteRunStatusPaused:
		return []uiMsg{remoteStartedMsg(index, summary), jobPausedMsg{Index: index}}
	case remoteRunStatusRetrying:
		return []uiMsg{remoteRetryScheduledMsg(index, summary)}
	default:
		return nil
	}
}

func remoteStartedMsg(index int, summary *apicore.RunJobSummary) jobStartedMsg {
	attempt := max(summaryAttempt(summary), 1)
	return jobStartedMsg{
		Index:           index,
		Attempt:         attempt,
		MaxAttempts:     max(summaryMaxAttempts(summary), attempt),
		IDE:             summaryString(summary, func(v *apicore.RunJobSummary) string { return v.IDE }),
		Model:           summaryString(summary, func(v *apicore.RunJobSummary) string { return v.Model }),
		ReasoningEffort: summaryString(summary, func(v *apicore.RunJobSummary) string { return v.ReasoningEffort }),
	}
}

func remoteRetryScheduledMsg(index int, summary *apicore.RunJobSummary) jobRetryMsg {
	attempt := max(summaryAttempt(summary), 1)
	return jobRetryMsg{
		Index:       index,
		Attempt:     attempt,
		MaxAttempts: max(summaryMaxAttempts(summary), attempt),
		Reason:      summaryString(summary, func(v *apicore.RunJobSummary) string { return v.RetryReason }),
	}
}

func remoteBootstrapSummaryMsgs(index int, summary *apicore.RunJobSummary) []uiMsg {
	if summary == nil {
		return nil
	}

	msgs := make([]uiMsg, 0, 2)
	if snapshot := summary.Session; len(snapshot.Entries) > 0 || len(snapshot.Plan.Entries) > 0 ||
		snapshot.Session.Status != "" || snapshot.Session.CurrentModeID != "" ||
		len(snapshot.Session.AvailableCommands) > 0 {
		msgs = append(msgs, jobUpdateMsg{
			Index:             index,
			Snapshot:          uiSessionSnapshot(snapshot),
			HydrateTranslator: true,
		})
	}
	if usage := usageFromSnapshot(summary.Usage); usage.Total() > 0 {
		msgs = append(msgs, usageUpdateMsg{Index: index, Usage: usage})
	}
	return msgs
}

func remoteBootstrapTerminalMsgs(index int, status string, summary *apicore.RunJobSummary) []uiMsg {
	switch status {
	case remoteRunStatusCompleted:
		return []uiMsg{jobFinishedMsg{
			Index:      index,
			Success:    true,
			ExitCode:   summaryExitCode(summary),
			DurationMs: summaryDurationMs(summary),
		}}
	case remoteRunStatusFailed:
		return []uiMsg{
			jobFailureMsg{Failure: remoteFailureInfo(summary)},
			jobFinishedMsg{
				Index:      index,
				Success:    false,
				ExitCode:   summaryExitCode(summary),
				DurationMs: summaryDurationMs(summary),
			},
		}
	case remoteRunStatusCanceled:
		return []uiMsg{jobFinishedMsg{
			Index:      index,
			Success:    false,
			ExitCode:   exitCodeCanceled,
			DurationMs: summaryDurationMs(summary),
		}}
	default:
		return nil
	}
}

func uiSessionSnapshot(snapshot apicore.SessionViewSnapshot) SessionViewSnapshot {
	result := SessionViewSnapshot{
		Revision: snapshot.Revision,
		Entries:  make([]TranscriptEntry, 0, len(snapshot.Entries)),
		Plan: SessionPlanState{
			Entries:      make([]model.SessionPlanEntry, 0, len(snapshot.Plan.Entries)),
			PendingCount: snapshot.Plan.PendingCount,
			RunningCount: snapshot.Plan.RunningCount,
			DoneCount:    snapshot.Plan.DoneCount,
		},
		Session: SessionMetaState{
			CurrentModeID:     snapshot.Session.CurrentModeID,
			AvailableCommands: make([]model.SessionAvailableCommand, 0, len(snapshot.Session.AvailableCommands)),
			Status:            model.SessionStatus(snapshot.Session.Status),
		},
	}
	for _, entry := range snapshot.Entries {
		result.Entries = append(result.Entries, TranscriptEntry{
			ID:            entry.ID,
			Kind:          transcriptEntryKind(entry.Kind),
			Title:         entry.Title,
			Preview:       entry.Preview,
			ToolCallID:    entry.ToolCallID,
			ToolCallState: model.ToolCallState(entry.ToolCallState),
			Blocks:        uiContentBlocks(entry.Blocks),
		})
	}
	for _, entry := range snapshot.Plan.Entries {
		result.Plan.Entries = append(result.Plan.Entries, model.SessionPlanEntry{
			Content:  entry.Content,
			Priority: entry.Priority,
			Status:   entry.Status,
		})
	}
	for _, cmd := range snapshot.Session.AvailableCommands {
		result.Session.AvailableCommands = append(result.Session.AvailableCommands, model.SessionAvailableCommand{
			Name:         cmd.Name,
			Description:  cmd.Description,
			ArgumentHint: cmd.ArgumentHint,
		})
	}
	return result
}

func uiContentBlocks(blocks []apicore.ContentBlock) []model.ContentBlock {
	if len(blocks) == 0 {
		return nil
	}
	result := make([]model.ContentBlock, 0, len(blocks))
	for _, block := range blocks {
		result = append(result, model.ContentBlock{
			Type: model.ContentBlockType(block.Type),
			Data: append([]byte(nil), block.Data...),
		})
	}
	return result
}

func remoteFailureInfo(summary *apicore.RunJobSummary) failInfo {
	return failInfo{
		CodeFile: summaryString(
			summary,
			func(v *apicore.RunJobSummary) string { return firstNonEmpty(v.CodeFile, firstCodeFile(v.CodeFiles)) },
		),
		ExitCode: summaryExitCode(summary),
		OutLog:   summaryString(summary, func(v *apicore.RunJobSummary) string { return v.OutLog }),
		ErrLog:   summaryString(summary, func(v *apicore.RunJobSummary) string { return v.ErrLog }),
		Err: remoteEventError(
			summaryString(summary, func(v *apicore.RunJobSummary) string { return v.ErrorText }),
		),
	}
}

func remoteIssueGroups(count int) map[string][]model.IssueEntry {
	if count <= 0 {
		return nil
	}
	entries := make([]model.IssueEntry, count)
	return map[string][]model.IssueEntry{"snapshot": entries}
}

func summaryAttempt(summary *apicore.RunJobSummary) int {
	if summary == nil {
		return 0
	}
	return summary.Attempt
}

func summaryMaxAttempts(summary *apicore.RunJobSummary) int {
	if summary == nil {
		return 0
	}
	return summary.MaxAttempts
}

func summaryExitCode(summary *apicore.RunJobSummary) int {
	if summary == nil {
		return 0
	}
	return summary.ExitCode
}

func summaryDurationMs(summary *apicore.RunJobSummary) int64 {
	if summary == nil {
		return 0
	}
	return summary.DurationMs
}

func summaryString(summary *apicore.RunJobSummary, read func(*apicore.RunJobSummary) string) string {
	if summary == nil || read == nil {
		return ""
	}
	return read(summary)
}

func streamCursorOrZero(cursor *apicore.StreamCursor) apicore.StreamCursor {
	if cursor == nil {
		return apicore.StreamCursor{}
	}
	return *cursor
}

func usageFromSnapshot(src kinds.Usage) model.Usage {
	return model.Usage{
		InputTokens:  src.InputTokens,
		OutputTokens: src.OutputTokens,
		TotalTokens:  src.TotalTokens,
		CacheReads:   src.CacheReads,
		CacheWrites:  src.CacheWrites,
	}
}

func remoteEventError(message string) error {
	message = firstNonEmpty(message)
	if message == "" {
		return nil
	}
	return errors.New(message)
}

func firstCodeFile(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func isTerminalRunStatus(status string) bool {
	switch status {
	case remoteRunStatusCompleted, remoteRunStatusFailed, remoteRunStatusCanceled, remoteRunStatusCrashed:
		return true
	default:
		return false
	}
}

func isTerminalRunEvent(kind events.EventKind) bool {
	return events.IsRunTerminalKind(kind)
}

func remoteStreamChannels(stream apiclient.RunStream) (<-chan apiclient.RunStreamItem, <-chan error) {
	if stream == nil {
		return nil, nil
	}
	return stream.Items(), stream.Errors()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
