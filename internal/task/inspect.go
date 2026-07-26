package task

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

const (
	inspectRecentRunLimit        = 5
	inspectRecentEventLimit      = 10
	inspectEligibleSessionLimit  = 200
	inspectHashPrefixSHA256      = "sha256:"
	inspectHashTruncatedLength   = 8
	inspectErrorSummaryMaxLength = 160
)

// InspectTask returns a deterministic read-only snapshot for one task id.
func (m *Service) InspectTask(ctx context.Context, taskID string, actor ActorContext) (*InspectView, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}

	trimmedID := strings.TrimSpace(taskID)
	if trimmedID == "" {
		return nil, fmt.Errorf("%w: task id is required", ErrValidation)
	}

	record, err := m.loadAuthorizedTask(ctx, m.store, trimmedID, actor)
	if err != nil {
		return nil, err
	}
	return m.inspectTaskRecord(ctx, record, InspectTargetTask, "")
}

// InspectRun returns a deterministic read-only snapshot rooted at one run id.
func (m *Service) InspectRun(ctx context.Context, runID string, actor ActorContext) (*InspectView, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}

	run, taskRecord, err := m.loadRunWithTask(ctx, runID)
	if err != nil {
		return nil, err
	}
	if err := m.authorizeTaskResource(ctx, actor, taskRecord); err != nil {
		return nil, err
	}
	return m.inspectTaskRecord(ctx, taskRecord, InspectTargetRun, run.ID)
}

func (m *Service) inspectTaskRecord(
	ctx context.Context,
	taskRecord Task,
	target InspectTarget,
	focusedRunID string,
) (*InspectView, error) {
	asOf := m.now().UTC()
	childCount, err := m.store.CountDirectChildren(ctx, taskRecord.ID)
	if err != nil {
		return nil, err
	}
	dependencies, err := m.store.ListDependencies(ctx, taskRecord.ID)
	if err != nil {
		return nil, err
	}
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: taskRecord.ID})
	if err != nil {
		return nil, err
	}
	events, err := m.store.ListTaskEvents(ctx, EventQuery{TaskID: taskRecord.ID, Limit: 1})
	if err != nil {
		return nil, err
	}

	summary, err := m.enrichTaskSummaryFromState(ctx, taskRecord, childCount, dependencies, runs, events)
	if err != nil {
		return nil, err
	}

	currentRun := inspectCurrentRun(runs, taskRecord.CurrentRunID, focusedRunID)
	currentRunSummary := inspectRunSummaryPtr(currentRun, asOf)
	recentRuns := inspectRecentRuns(runs, asOf)
	boundSession, sessionCatalogPresent, eligibleSessionCount, err := m.inspectSessionState(
		ctx,
		taskRecord,
		currentRunSummary,
	)
	if err != nil {
		return nil, err
	}
	scheduler, err := m.inspectSchedulerState(ctx)
	if err != nil {
		return nil, err
	}
	recentEvents, err := m.inspectRecentEvents(ctx, taskRecord.ID, focusedRunID)
	if err != nil {
		return nil, err
	}

	view := &InspectView{
		Target:                target,
		Task:                  summary,
		CurrentRun:            currentRunSummary,
		BoundSession:          boundSession,
		RecentRuns:            recentRuns,
		RecentEvents:          recentEvents,
		Scheduler:             scheduler,
		AsOf:                  asOf,
		EligibleSessionCount:  eligibleSessionCount,
		SessionCatalogPresent: sessionCatalogPresent,
	}
	diagnosticSnapshot := inspectDiagnosticSnapshot{
		Task:                  view.Task,
		CurrentRun:            view.CurrentRun,
		BoundSession:          view.BoundSession,
		RecentRuns:            view.RecentRuns,
		Scheduler:             view.Scheduler,
		AsOf:                  view.AsOf,
		EligibleSessionCount:  view.EligibleSessionCount,
		SessionCatalogPresent: view.SessionCatalogPresent,
	}
	view.Diagnostics = detectInspectDiagnostics(&diagnosticSnapshot)
	view.NextAction = inspectNextAction(view)
	return view, nil
}

func (m *Service) inspectSessionState(
	ctx context.Context,
	taskRecord Task,
	currentRun *InspectRunSummary,
) (*InspectSessionSummary, bool, int, error) {
	if m.inspectReader == nil {
		return nil, false, 0, nil
	}

	var boundSession *InspectSessionSummary
	if currentRun != nil && strings.TrimSpace(currentRun.BoundSessionID) != "" {
		session, err := m.inspectSession(ctx, currentRun.BoundSessionID)
		if err != nil {
			return nil, false, 0, err
		}
		boundSession = session
	}

	query := store.SessionListQuery{Limit: inspectEligibleSessionLimit}
	if taskRecord.Scope.Normalize() == ScopeWorkspace {
		query.WorkspaceID = taskRecord.WorkspaceID
	}
	sessions, err := m.inspectReader.ListSessions(ctx, query)
	if err != nil {
		return nil, false, 0, err
	}
	eligible := 0
	for _, session := range sessions {
		if inspectSessionCanClaim(session) {
			eligible++
		}
	}
	return boundSession, true, eligible, nil
}

func (m *Service) inspectSession(ctx context.Context, sessionID string) (*InspectSessionSummary, error) {
	trimmedID := strings.TrimSpace(sessionID)
	if trimmedID == "" {
		return nil, nil
	}
	sessions, err := m.inspectReader.ListSessions(ctx, store.SessionListQuery{ID: trimmedID, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return &InspectSessionSummary{SessionID: trimmedID, State: inspectSessionMissingState}, nil
	}
	summary := inspectSessionSummaryFromStore(sessions[0])
	return &summary, nil
}

func (m *Service) inspectSchedulerState(ctx context.Context) (InspectSchedulerState, error) {
	if m.inspectReader == nil {
		return InspectSchedulerState{}, nil
	}
	return m.inspectReader.GetSchedulerPauseState(ctx)
}

func (m *Service) inspectRecentEvents(ctx context.Context, taskID string, runID string) ([]InspectEventSummary, error) {
	if m.inspectReader == nil {
		return nil, nil
	}
	query := store.EventSummaryQuery{TaskID: taskID, Limit: inspectRecentEventLimit}
	if strings.TrimSpace(runID) != "" {
		query.RunID = strings.TrimSpace(runID)
	}
	events, err := m.inspectReader.ListEventSummaries(ctx, query)
	if err != nil {
		return nil, err
	}
	return inspectEventSummariesFromStore(events), nil
}

func inspectCurrentRun(runs []Run, currentRunID string, focusedRunID string) *Run {
	if run := inspectRunByID(runs, focusedRunID); run != nil {
		return run
	}
	if run := inspectRunByID(runs, currentRunID); run != nil {
		return run
	}
	var current *Run
	for idx := range runs {
		run := runs[idx]
		if current == nil || inspectRunPreferred(run, *current) {
			current = &runs[idx]
		}
	}
	return current
}

func inspectRunByID(runs []Run, id string) *Run {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return nil
	}
	for idx := range runs {
		if strings.TrimSpace(runs[idx].ID) == trimmedID {
			return &runs[idx]
		}
	}
	return nil
}

func inspectRunPreferred(candidate Run, current Run) bool {
	candidateRank := activeRunRank(candidate.Status)
	currentRank := activeRunRank(current.Status)
	if candidateRank != currentRank {
		return candidateRank > currentRank
	}
	return runComesAfter(candidate, current)
}

func inspectRecentRuns(runs []Run, asOf time.Time) []InspectRunSummary {
	sorted := append([]Run(nil), runs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return runComesAfter(sorted[i], sorted[j])
	})
	if len(sorted) > inspectRecentRunLimit {
		sorted = sorted[:inspectRecentRunLimit]
	}
	summaries := make([]InspectRunSummary, 0, len(sorted))
	for _, run := range sorted {
		summaries = append(summaries, inspectRunSummaryFromRun(run, asOf))
	}
	return summaries
}

func inspectRunSummaryPtr(run *Run, asOf time.Time) *InspectRunSummary {
	if run == nil {
		return nil
	}
	summary := inspectRunSummaryFromRun(*run, asOf)
	return &summary
}

func inspectRunSummaryFromRun(run Run, asOf time.Time) InspectRunSummary {
	heartbeatAge := inspectHeartbeatAgeSeconds(run, asOf)
	return InspectRunSummary{
		RunID:                   run.ID,
		TaskID:                  run.TaskID,
		Status:                  run.Status.Normalize(),
		ClaimTokenHashTruncated: truncateClaimTokenHash(run.ClaimTokenHash),
		LeaseUntil:              run.LeaseUntil,
		HeartbeatAt:             run.HeartbeatAt,
		HeartbeatAgeSeconds:     heartbeatAge,
		Retries:                 inspectRetryCount(run),
		LastErrorSummary:        inspectErrorSummary(run.Error),
		FailureKind:             inspectFailureKind(run),
		BoundSessionID:          strings.TrimSpace(run.SessionID),
		StartedAt:               run.StartedAt,
		EndedAt:                 run.EndedAt,
		PreviousRunID:           inspectPreviousRunID(run),
		QueuedAt:                run.QueuedAt,
		Attempt:                 int(run.Attempt),
		RecoveryCount:           int(run.RecoveryCount),
	}
}

func inspectHeartbeatAgeSeconds(run Run, asOf time.Time) *int64 {
	heartbeatAt := run.HeartbeatAt
	if heartbeatAt.IsZero() && run.Status.Normalize() == TaskRunStatusClaimed {
		heartbeatAt = run.ClaimedAt
	}
	if heartbeatAt.IsZero() || asOf.Before(heartbeatAt) {
		return nil
	}
	age := int64(asOf.Sub(heartbeatAt).Seconds())
	return &age
}

func inspectRetryCount(run Run) int {
	if run.Attempt <= 1 {
		return 0
	}
	return int(run.Attempt - 1)
}

func inspectErrorSummary(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) <= inspectErrorSummaryMaxLength {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:inspectErrorSummaryMaxLength])
}

func inspectFailureKind(run Run) string {
	if strings.TrimSpace(run.FailureKind) != "" {
		return strings.TrimSpace(run.FailureKind)
	}
	if run.Status.Normalize() == TaskRunStatusFailed && strings.TrimSpace(run.Error) != "" {
		return "error"
	}
	return ""
}

func inspectPreviousRunID(run Run) string {
	if strings.TrimSpace(run.PreviousRunID) != "" {
		return strings.TrimSpace(run.PreviousRunID)
	}
	if run.Review == nil {
		return ""
	}
	return strings.TrimSpace(run.Review.ParentRunID)
}

func truncateClaimTokenHash(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, inspectHashPrefixSHA256)
	if len(trimmed) <= inspectHashTruncatedLength {
		return trimmed
	}
	return trimmed[:inspectHashTruncatedLength]
}

func inspectSessionSummaryFromStore(session store.SessionInfo) InspectSessionSummary {
	failureKind := ""
	if session.Failure != nil {
		failureKind = string(session.Failure.Kind)
	}
	return InspectSessionSummary{
		SessionID:      session.ID,
		State:          strings.TrimSpace(session.State),
		AgentName:      strings.TrimSpace(session.AgentName),
		ProviderName:   strings.TrimSpace(session.Provider),
		WorkspaceID:    strings.TrimSpace(session.WorkspaceID),
		StartedAt:      session.CreatedAt,
		LastActivityAt: session.UpdatedAt,
		StopReason:     string(session.StopReason),
		FailureKind:    failureKind,
	}
}

func inspectEventSummariesFromStore(events []store.EventSummary) []InspectEventSummary {
	summaries := make([]InspectEventSummary, 0, len(events))
	for _, event := range events {
		summaries = append(summaries, InspectEventSummary{
			ID:        event.ID,
			Type:      event.Type,
			SessionID: event.SessionID,
			TaskID:    event.TaskID,
			RunID:     event.RunID,
			Outcome:   event.Outcome,
			Summary:   event.Summary,
			Timestamp: event.Timestamp,
		})
	}
	return summaries
}

func inspectSessionCanClaim(session store.SessionInfo) bool {
	state := strings.ToLower(strings.TrimSpace(session.State))
	if state != managerActiveKey {
		return false
	}
	if session.Failure != nil && !session.Failure.IsZero() {
		return false
	}
	return true
}
