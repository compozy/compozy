package daemon

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/contentconv"
	"github.com/compozy/compozy/internal/core/run/transcript"
	"github.com/compozy/compozy/internal/store/rundb"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type runSnapshotBuilder struct {
	jobs     map[int]*runSnapshotJob
	order    []int
	usage    kinds.Usage
	shutdown *apicore.RunShutdownState
}

type runSnapshotJob struct {
	state   apicore.RunJobState
	summary apicore.RunJobSummary
	session *transcript.ViewModel
}

func newRunSnapshotBuilder() *runSnapshotBuilder {
	return &runSnapshotBuilder{
		jobs: make(map[int]*runSnapshotJob),
	}
}

func (b *runSnapshotBuilder) applyEvent(item events.Event) error {
	switch item.Kind {
	case events.EventKindJobQueued:
		return b.applyJobQueued(item)
	case events.EventKindJobStarted:
		return b.applyJobStarted(item)
	case events.EventKindJobRetryScheduled:
		return b.applyJobRetry(item)
	case events.EventKindJobCompleted:
		return b.applyJobCompleted(item)
	case events.EventKindJobFailed:
		return b.applyJobFailed(item)
	case events.EventKindJobCancelled:
		return b.applyJobCancelled(item)
	case events.EventKindSessionUpdate:
		return b.applySessionUpdate(item)
	case events.EventKindShutdownRequested:
		return b.applyShutdownRequested(item)
	case events.EventKindShutdownDraining:
		return b.applyShutdownDraining(item)
	case events.EventKindShutdownTerminated:
		return b.applyShutdownTerminated(item)
	default:
		return nil
	}
}

func (b *runSnapshotBuilder) applyTokenUsageRows(rows []rundb.TokenUsageRow) {
	for _, row := range rows {
		switch {
		case row.TurnID == "run-total":
			b.usage = tokenUsageRowToKinds(row)
		case strings.HasPrefix(row.TurnID, "session-"):
			index, ok := tokenUsageIndex(row.TurnID)
			if !ok {
				continue
			}
			job := b.ensureJob(index)
			job.summary.Usage = tokenUsageRowToKinds(row)
		}
	}
}

func (b *runSnapshotBuilder) jobStates() []apicore.RunJobState {
	if len(b.order) == 0 {
		return nil
	}

	sorted := append([]int(nil), b.order...)
	slices.Sort(sorted)

	result := make([]apicore.RunJobState, 0, len(sorted))
	for _, index := range sorted {
		job := b.jobs[index]
		if job == nil {
			continue
		}
		job.state.Index = index
		job.summary.Index = index
		if snapshot := job.session.Snapshot(); snapshot.Revision != 0 || len(snapshot.Entries) > 0 ||
			len(snapshot.Plan.Entries) > 0 || snapshot.Session.Status != "" ||
			snapshot.Session.CurrentModeID != "" || len(snapshot.Session.AvailableCommands) > 0 {
			job.summary.Session = snapshot
		}
		job.state.Summary = cloneRunJobSummary(job.summary)
		result = append(result, job.state)
	}
	return result
}

func (b *runSnapshotBuilder) ensureJob(index int) *runSnapshotJob {
	if existing := b.jobs[index]; existing != nil {
		return existing
	}

	jobID := fmt.Sprintf("job-%03d", index)
	job := &runSnapshotJob{
		state: apicore.RunJobState{
			Index:     index,
			JobID:     jobID,
			Status:    "queued",
			UpdatedAt: time.Time{},
		},
		summary: apicore.RunJobSummary{
			Index:    index,
			SafeName: jobID,
		},
		session: transcript.NewViewModel(),
	}
	b.jobs[index] = job
	b.order = append(b.order, index)
	return job
}

func (b *runSnapshotBuilder) applyJobQueued(item events.Event) error {
	var payload kinds.JobQueuedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode job queued snapshot payload: %w", err)
	}

	job := b.ensureJob(payload.Index)
	job.state.JobID = firstNonEmpty(strings.TrimSpace(payload.SafeName), job.state.JobID)
	job.state.TaskID = firstNonEmpty(
		strings.TrimSpace(payload.SafeName),
		strings.TrimSpace(payload.TaskTitle),
		strings.TrimSpace(payload.CodeFile),
	)
	job.state.Status = "queued"
	job.state.AgentName = strings.TrimSpace(payload.IDE)
	job.state.UpdatedAt = item.Timestamp.UTC()

	job.summary.CodeFile = strings.TrimSpace(payload.CodeFile)
	job.summary.CodeFiles = append([]string(nil), payload.CodeFiles...)
	job.summary.Issues = payload.Issues
	job.summary.TaskTitle = strings.TrimSpace(payload.TaskTitle)
	job.summary.TaskType = strings.TrimSpace(payload.TaskType)
	job.summary.SafeName = firstNonEmpty(strings.TrimSpace(payload.SafeName), job.summary.SafeName)
	job.summary.IDE = strings.TrimSpace(payload.IDE)
	job.summary.Model = strings.TrimSpace(payload.Model)
	job.summary.ReasoningEffort = strings.TrimSpace(payload.ReasoningEffort)
	job.summary.AccessMode = strings.TrimSpace(payload.AccessMode)
	job.summary.OutLog = strings.TrimSpace(payload.OutLog)
	job.summary.ErrLog = strings.TrimSpace(payload.ErrLog)
	return nil
}

func (b *runSnapshotBuilder) applyJobStarted(item events.Event) error {
	var payload kinds.JobStartedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode job started snapshot payload: %w", err)
	}

	job := b.ensureJob(payload.Index)
	job.state.Status = "running"
	job.state.AgentName = firstNonEmpty(strings.TrimSpace(payload.IDE), job.state.AgentName)
	job.state.UpdatedAt = item.Timestamp.UTC()

	job.summary.Attempt = payload.Attempt
	job.summary.MaxAttempts = payload.MaxAttempts
	job.summary.IDE = firstNonEmpty(strings.TrimSpace(payload.IDE), job.summary.IDE)
	job.summary.Model = firstNonEmpty(strings.TrimSpace(payload.Model), job.summary.Model)
	job.summary.ReasoningEffort = firstNonEmpty(strings.TrimSpace(payload.ReasoningEffort), job.summary.ReasoningEffort)
	job.summary.AccessMode = firstNonEmpty(strings.TrimSpace(payload.AccessMode), job.summary.AccessMode)
	return nil
}

func (b *runSnapshotBuilder) applyJobRetry(item events.Event) error {
	var payload kinds.JobRetryScheduledPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode job retry snapshot payload: %w", err)
	}

	job := b.ensureJob(payload.Index)
	job.state.Status = "retrying"
	job.state.UpdatedAt = item.Timestamp.UTC()

	job.summary.Attempt = payload.Attempt
	job.summary.MaxAttempts = payload.MaxAttempts
	job.summary.RetryReason = strings.TrimSpace(payload.Reason)
	return nil
}

func (b *runSnapshotBuilder) applyJobCompleted(item events.Event) error {
	var payload kinds.JobCompletedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode job completed snapshot payload: %w", err)
	}

	job := b.ensureJob(payload.Index)
	job.state.Status = "completed"
	job.state.UpdatedAt = item.Timestamp.UTC()

	job.summary.Attempt = payload.Attempt
	job.summary.MaxAttempts = payload.MaxAttempts
	job.summary.ExitCode = payload.ExitCode
	return nil
}

func (b *runSnapshotBuilder) applyJobFailed(item events.Event) error {
	var payload kinds.JobFailedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode job failed snapshot payload: %w", err)
	}

	job := b.ensureJob(payload.Index)
	job.state.Status = "failed"
	job.state.UpdatedAt = item.Timestamp.UTC()

	job.summary.CodeFile = firstNonEmpty(strings.TrimSpace(payload.CodeFile), job.summary.CodeFile)
	job.summary.Attempt = payload.Attempt
	job.summary.MaxAttempts = payload.MaxAttempts
	job.summary.ExitCode = payload.ExitCode
	job.summary.OutLog = firstNonEmpty(strings.TrimSpace(payload.OutLog), job.summary.OutLog)
	job.summary.ErrLog = firstNonEmpty(strings.TrimSpace(payload.ErrLog), job.summary.ErrLog)
	job.summary.ErrorText = strings.TrimSpace(payload.Error)
	return nil
}

func (b *runSnapshotBuilder) applyJobCancelled(item events.Event) error {
	var payload kinds.JobCancelledPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode job canceled snapshot payload: %w", err)
	}

	job := b.ensureJob(payload.Index)
	job.state.Status = "canceled"
	job.state.UpdatedAt = item.Timestamp.UTC()

	job.summary.ErrorText = strings.TrimSpace(payload.Reason)
	return nil
}

func (b *runSnapshotBuilder) applySessionUpdate(item events.Event) error {
	var payload kinds.SessionUpdatePayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode session update snapshot payload: %w", err)
	}

	job := b.ensureJob(payload.Index)
	job.state.UpdatedAt = item.Timestamp.UTC()
	update, err := contentconv.InternalSessionUpdate(payload.Update)
	if err != nil {
		return fmt.Errorf("decode internal session update snapshot payload: %w", err)
	}
	if _, changed := job.session.Apply(update); !changed {
		return nil
	}
	if job.state.Status == "queued" {
		job.state.Status = "running"
	}
	return nil
}

func (b *runSnapshotBuilder) applyShutdownRequested(item events.Event) error {
	var payload kinds.ShutdownRequestedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode shutdown requested snapshot payload: %w", err)
	}
	b.shutdown = shutdownStateFromPayload("draining", payload.Source, payload.RequestedAt, payload.DeadlineAt)
	return nil
}

func (b *runSnapshotBuilder) applyShutdownDraining(item events.Event) error {
	var payload kinds.ShutdownDrainingPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode shutdown draining snapshot payload: %w", err)
	}
	b.shutdown = shutdownStateFromPayload("draining", payload.Source, payload.RequestedAt, payload.DeadlineAt)
	return nil
}

func (b *runSnapshotBuilder) applyShutdownTerminated(item events.Event) error {
	var payload kinds.ShutdownTerminatedPayload
	if err := json.Unmarshal(item.Payload, &payload); err != nil {
		return fmt.Errorf("decode shutdown terminated snapshot payload: %w", err)
	}
	phase := "draining"
	if payload.Forced {
		phase = "forcing"
	}
	b.shutdown = shutdownStateFromPayload(phase, payload.Source, payload.RequestedAt, payload.DeadlineAt)
	return nil
}

func cloneRunJobSummary(src apicore.RunJobSummary) *apicore.RunJobSummary {
	dst := src
	if len(src.CodeFiles) > 0 {
		dst.CodeFiles = append([]string(nil), src.CodeFiles...)
	}
	return &dst
}

func tokenUsageRowToKinds(row rundb.TokenUsageRow) kinds.Usage {
	return kinds.Usage{
		InputTokens:  row.InputTokens,
		OutputTokens: row.OutputTokens,
		TotalTokens:  row.TotalTokens,
	}
}

func tokenUsageIndex(turnID string) (int, bool) {
	value := strings.TrimPrefix(strings.TrimSpace(turnID), "session-")
	index, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return index, true
}

func shutdownStateFromPayload(
	phase string,
	source string,
	requestedAt time.Time,
	deadlineAt time.Time,
) *apicore.RunShutdownState {
	return &apicore.RunShutdownState{
		Phase:       strings.TrimSpace(phase),
		Source:      strings.TrimSpace(source),
		RequestedAt: requestedAt,
		DeadlineAt:  deadlineAt,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
