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

func (r *InstanceRepo) Finish(ctx context.Context, req heartbeat.FinishRequest) (bool, error) {
	if err := validateInstanceWakeFinish(&req); err != nil {
		return false, err
	}
	changed := false
	err := store.ExecuteWriteOperation(
		ctx,
		r.db,
		"finish instance wake",
		func(ctx context.Context, tx *store.WriteTx) error {
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
			if row.OccurrenceCancelled != 0 || instance.State != "active" || instance.ArchivedAt.Valid {
				req.Result, req.Reason, req.Report = instanceWakeCanceledResult, instanceWakeCanceledResult, nil
				req.NextWakeAt = time.Time{}
			}
			if err := deduplicateInstanceProposals(ctx, q, &req); err != nil {
				return err
			}
			completed, quiet, failures, err := instanceWakeStreaks(req.Result, row.QuietStreak, row.FailureStreak)
			if err != nil {
				return err
			}

			if req.Result == "failed" && req.FatigueLimit > 0 && failures >= req.FatigueLimit {
				if _, err := q.PauseAgentInstance(ctx, sqlcgen.PauseAgentInstanceParams{
					ID: req.InstanceID, PauseReason: "fatigue", UpdatedAt: store.FormatTimestamp(req.Now),
				}); err != nil {
					return err
				}
				req.Report = &agentinstance.ReportInput{
					Kind: "failure",
					Payload: []byte(
						`{"reason":"fatigue","detail":"Proactive checks paused after consecutive failures."}`,
					),
				}
			}

			req.NextWakeAt = nextInstanceWake(&req, quiet, failures)
			if err := insertInstanceWakeReport(ctx, q, &req); err != nil {
				return err
			}
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
			next := ""
			if !req.NextWakeAt.IsZero() {
				next = store.FormatTimestamp(req.NextWakeAt)
			}
			count, err := q.FinishInstanceWake(ctx, sqlcgen.FinishInstanceWakeParams{
				InstanceID: req.InstanceID, OccurrenceID: req.OccurrenceID, Now: store.FormatTimestamp(req.Now),
				Completed: completed, Digest: req.Digest, AutomationCursor: req.AutomationCursor,
				QuietStreak: quiet, FailureStreak: failures, NextWakeAt: next,
			})
			changed = count == 1
			return err
		},
	)
	return changed, err
}

func validateInstanceWakeFinish(req *heartbeat.FinishRequest) error {
	if req.InstanceID == "" || req.OccurrenceID == "" || req.Now.IsZero() || !req.ExpiresAt.After(req.Now) ||
		req.DeniedCalls < 0 {
		return errors.New("invalid wake finalization")
	}
	if req.Source != "timer" && req.Source != "instance_check" {
		return errors.New("invalid proactive wake source")
	}
	return nil
}

func instanceWakeStreaks(result string, quiet, failures int64) (int, int64, int64, error) {
	switch result {
	case "quiet":
		return 1, quiet + 1, 0, nil
	case "reported":
		return 1, 0, 0, nil
	case "failed":
		return 0, quiet, failures + 1, nil
	case instanceWakeCanceledResult:
		return 0, quiet, failures, nil
	default:
		return 0, quiet, failures, errors.New("invalid proactive wake result")
	}
}

func nextInstanceWake(req *heartbeat.FinishRequest, quiet, failures int64) time.Time {
	if req.Source == "instance_check" || req.Result == instanceWakeCanceledResult {
		return time.Time{}
	}
	if req.Interval <= 0 || req.MaxInterval <= 0 {
		return req.NextWakeAt
	}
	streak := int64(0)
	if req.Result == "quiet" {
		streak = quiet / 3
	}
	if req.Result == "failed" {
		streak = failures
	}
	maximum := req.MaxInterval
	if req.Result == "quiet" && req.QuietBackoffCap > 0 {
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
