package globaldb

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (r *InstanceRepo) Finish(ctx context.Context, req heartbeat.FinishRequest) (heartbeat.FinishResult, error) {
	if err := validateInstanceWakeFinish(&req); err != nil {
		return heartbeat.FinishResult{}, err
	}
	original := req
	result := heartbeat.FinishResult{}
	err := store.ExecuteWriteOperation(
		ctx,
		r.db,
		"finish instance wake",
		func(ctx context.Context, tx *store.WriteTx) error {
			req = original
			result = heartbeat.FinishResult{}
			q := sqlcgen.New(tx)
			row, err := q.GetInstanceWakeState(ctx, req.InstanceID)
			if err != nil {
				return err
			}
			if row.OccurrenceID != req.OccurrenceID {
				return nil
			}
			instance, err := q.GetAgentInstance(ctx, req.InstanceID)
			if err != nil {
				return err
			}
			if row.OccurrenceCancelled != 0 || instance.State != globalDBSessionStateActive ||
				instance.ArchivedAt.Valid {
				req.Result, req.Reason, req.Report = instanceWakeCanceledResult, instanceWakeCanceledResult, nil
				req.NextWakeAt = time.Time{}
			}
			if err := deduplicateInstanceProposals(ctx, q, &req); err != nil {
				return err
			}
			completed, quiet, failures, err := instanceWakeStreaks(&req, row.QuietStreak, row.FailureStreak)
			if err != nil {
				return err
			}

			if err := countFinishedCheck(ctx, q, &req); err != nil {
				return err
			}

			if err := pauseFatiguedInstance(ctx, q, &req, failures); err != nil {
				return err
			}

			req.NextWakeAt = nextInstanceWake(&req, quiet, failures)
			if err := insertInstanceWakeReport(ctx, q, &req); err != nil {
				return err
			}
			if err := consumeFinishedInstanceEvents(ctx, q, req, completed); err != nil {
				return err
			}
			if err := finishHeartbeatSources(ctx, q, req, completed); err != nil {
				return err
			}
			if err := recordFinishedWakeEvent(ctx, q, &req); err != nil {
				return err
			}

			next := ""
			if !req.NextWakeAt.IsZero() {
				next = store.FormatTimestamp(req.NextWakeAt)
			}
			count, err := q.FinishInstanceWake(ctx, sqlcgen.FinishInstanceWakeParams{
				InstanceID: req.InstanceID, OccurrenceID: req.OccurrenceID, Now: store.FormatTimestamp(req.Now),
				Completed: completed, Digest: req.Digest, AutomationCursor: req.AutomationCursor,
				QuietStreak: quiet, FailureStreak: failures, NextWakeAt: next,
			})
			result.Changed = count == 1
			if err != nil {
				return err
			}
			result.Paused, err = finishedInstancePause(ctx, q, instance)
			return err
		},
	)
	if err != nil {
		return heartbeat.FinishResult{}, err
	}
	return result, nil
}

func validateInstanceWakeFinish(req *heartbeat.FinishRequest) error {
	if req.InstanceID == "" || req.OccurrenceID == "" || req.Now.IsZero() || !req.ExpiresAt.After(req.Now) ||
		req.DeniedCalls < 0 || req.UnattendedTurnLimit < 0 {
		return errors.New("invalid wake finalization")
	}
	if req.Source != "timer" && req.Source != instanceManualCheckSource {
		return errors.New("invalid proactive wake source")
	}
	return nil
}

func instanceWakeStreaks(req *heartbeat.FinishRequest, quiet, failures int64) (int, int64, int64, error) {
	if req.Result == loopNodeOutputFailed && !countsAsCheckFailure(req) {
		return 0, quiet, failures, nil
	}
	switch req.Result {
	case instanceWakeQuietResult:
		return 1, quiet + 1, 0, nil
	case instanceWakeReportedResult:
		return 1, 0, 0, nil
	case loopNodeOutputFailed:
		return 0, quiet, failures + 1, nil
	case instanceWakeCanceledResult:
		return 0, quiet, failures, nil
	default:
		return 0, quiet, failures, errors.New("invalid proactive wake result")
	}
}

func nextInstanceWake(req *heartbeat.FinishRequest, quiet, failures int64) time.Time {
	if req.Source == instanceManualCheckSource || req.Result == instanceWakeCanceledResult {
		return time.Time{}
	}
	if req.Interval <= 0 || req.MaxInterval <= 0 {
		return req.NextWakeAt
	}
	// A rejected configuration fails the same way until the operator fixes it.
	if req.Result == loopNodeOutputFailed && !countsAsCheckFailure(req) {
		return heartbeat.NextCheckTime(req.Now, req.MaxInterval, req.InstanceID)
	}
	if req.SourceMore && (req.Result == instanceWakeQuietResult || req.Result == instanceWakeReportedResult) {
		return heartbeat.NextCheckTime(req.Now, req.MinInterval, req.InstanceID)
	}
	streak := int64(0)
	if req.Result == instanceWakeQuietResult && !req.HasSources {
		streak = quiet / 3
	}
	if req.Result == loopNodeOutputFailed {
		streak = failures
	}
	maximum := req.MaxInterval
	if req.Result == instanceWakeQuietResult && req.QuietBackoffCap > 0 {
		maximum = req.QuietBackoffCap
	}
	return heartbeat.NextCheckTime(req.Now, heartbeat.CheckBackoff(req.Interval, maximum, streak), req.InstanceID)
}

func insertInstanceWakeReport(ctx context.Context, q *sqlcgen.Queries, req *heartbeat.FinishRequest) error {
	if req.Report == nil {
		return nil
	}
	report := *req.Report
	report.SourceKey = "occ:" + req.OccurrenceID
	if err := report.Validate(); err != nil {
		return err
	}
	reportID, err := store.NewID("arpt")
	if err != nil {
		return err
	}
	if err := q.InsertAgentInstanceReport(ctx, sqlcgen.InsertAgentInstanceReportParams{
		ID:         reportID,
		InstanceID: req.InstanceID,
		SourceKey:  report.SourceKey,
		Kind:       report.Kind,
		Payload: string(
			report.Payload,
		),
		Fingerprint: report.Fingerprint,
		CreatedAt:   store.FormatTimestamp(req.Now),
	}); err != nil {
		return err
	}
	return nil
}

const instanceWakeCanceledResult = "cancelled" //nolint:misspell // Matches the persisted wake result.

const instanceWakeQuietResult = "quiet"

func pauseFatiguedInstance(
	ctx context.Context,
	q *sqlcgen.Queries,
	req *heartbeat.FinishRequest,
	failures int64,
) error {
	if countsAsCheckFailure(req) && req.FatigueLimit > 0 && failures >= req.FatigueLimit {
		if changed, err := q.PauseAgentInstance(ctx, sqlcgen.PauseAgentInstanceParams{
			ID: req.InstanceID, PauseReason: "fatigue", UpdatedAt: store.FormatTimestamp(req.Now),
		}); err != nil {
			return err
		} else if changed == 0 {
			return nil
		}
		req.Report = &agentinstance.ReportInput{
			Kind: "failure",
			Payload: []byte(
				`{"reason":"fatigue","detail":"Proactive checks paused after consecutive failures."}`,
			),
		}
	}
	return nil
}

func finishedInstancePause(
	ctx context.Context,
	q *sqlcgen.Queries,
	before sqlcgen.AgentInstance,
) (*agentinstance.Instance, error) {
	if before.State != "active" {
		return nil, nil
	}
	after, err := q.GetAgentInstance(ctx, before.ID)
	if err != nil || after.State != goalStatusPaused {
		return nil, err
	}
	instance, err := instanceFromRow(after)
	if err != nil {
		return nil, err
	}
	return &instance, nil
}

func recordFinishedWakeEvent(ctx context.Context, q *sqlcgen.Queries, req *heartbeat.FinishRequest) error {
	eventID, err := store.NewID("hwe")
	if err != nil {
		return err
	}
	if err := q.InsertInstanceWakeEvent(ctx, sqlcgen.InsertInstanceWakeEventParams{
		ID:           eventID,
		InstanceID:   req.InstanceID,
		Source:       req.Source,
		Result:       req.Result,
		Reason:       req.Reason,
		Detail:       req.Detail,
		DeniedCalls:  req.DeniedCalls,
		OccurrenceID: req.OccurrenceID,
		CreatedAt:    store.FormatTimestamp(req.Now),
		ExpiresAt:    store.FormatTimestamp(req.ExpiresAt),
	}); err != nil {
		return err
	}
	return nil
}

const instanceWakeReportedResult = "reported"

func countFinishedCheck(ctx context.Context, q *sqlcgen.Queries, req *heartbeat.FinishRequest) error {
	if req.Result != instanceWakeReportedResult && !countsAsCheckFailure(req) {
		return nil
	}
	_, err := advanceInstanceUnattended(ctx, q, req.InstanceID, req.OccurrenceID, req.UnattendedTurnLimit, req.Now)
	return err
}

// countsAsCheckFailure excludes provider configuration rejections: the triage turn never ran,
// so the rejection must not advance the failure streak, fatigue pause, or unattended budget.
func countsAsCheckFailure(req *heartbeat.FinishRequest) bool {
	return req.Result == loopNodeOutputFailed && req.Reason != heartbeat.ReasonTriageConfigUnavailable
}
