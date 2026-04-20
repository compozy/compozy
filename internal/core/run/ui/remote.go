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
	Snapshot     apicore.RunSnapshot
	Config       *config
	OwnerSession bool
	LoadSnapshot func(context.Context) (apicore.RunSnapshot, error)
	OpenStream   func(context.Context, apicore.StreamCursor) (apiclient.RunStream, error)
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

	session := setupRemoteUISession(ctx, jobs, &localCfg, nil, true)
	if session == nil {
		return nil, errors.New("remote attach ui session is required")
	}
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
		state, stop = ensureRemoteFollowState(ctx, opts, state)
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
		return handleRemoteRunStreamError(ctx, opts, state, err, ok)
	case item, ok := <-state.itemCh:
		return handleRemoteRunStreamItem(ctx, session, opts, state, item, ok)
	}
}

func handleRemoteRunStreamError(
	ctx context.Context,
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
		return handleRemoteRunEOF(ctx, opts, state)
	}
	if err == nil {
		return state, false
	}
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
		return handleRemoteRunEOF(ctx, opts, state)
	}

	if item.Heartbeat != nil {
		state.lastCursor = remoteMaxCursor(state.lastCursor, item.Heartbeat.Cursor)
		return state, false
	}
	if item.Overflow != nil {
		state.lastCursor = remoteMaxCursor(state.lastCursor, item.Overflow.Cursor)
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
	opts RemoteAttachOptions,
	state remoteFollowState,
) (remoteFollowState, bool) {
	state = resetRemoteFollowState(state)
	if shouldStopAfterRemoteEOF(ctx, opts, state.lastCursor) {
		return state, true
	}
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
	lastCursor apicore.StreamCursor,
) bool {
	if opts.LoadSnapshot == nil {
		return false
	}
	snapshot, err := opts.LoadSnapshot(ctx)
	if err != nil {
		return false
	}
	if !isTerminalRunStatus(snapshot.Run.Status) {
		return false
	}
	if snapshot.NextCursor == nil {
		return true
	}
	return snapshot.NextCursor.Sequence <= lastCursor.Sequence
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
		msgs = append(msgs, jobUpdateMsg{Index: index, Snapshot: snapshot})
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
			Index:    index,
			Success:  true,
			ExitCode: summaryExitCode(summary),
		}}
	case remoteRunStatusFailed:
		return []uiMsg{
			jobFailureMsg{Failure: remoteFailureInfo(summary)},
			jobFinishedMsg{
				Index:    index,
				Success:  false,
				ExitCode: summaryExitCode(summary),
			},
		}
	case remoteRunStatusCanceled:
		return []uiMsg{jobFinishedMsg{
			Index:    index,
			Success:  false,
			ExitCode: exitCodeCanceled,
		}}
	default:
		return nil
	}
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
	switch kind {
	case events.EventKindRunCompleted, events.EventKindRunFailed, events.EventKindRunCancelled:
		return true
	default:
		return false
	}
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
