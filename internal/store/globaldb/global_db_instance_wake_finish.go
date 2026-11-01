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
	if req.InstanceID == "" || req.OccurrenceID == "" || req.Now.IsZero() || !req.ExpiresAt.After(req.Now) || req.DeniedCalls < 0 {
		return false, errors.New("invalid wake finalization")
	}
	if req.Source != "timer" && req.Source != "instance_check" {
		return false, errors.New("invalid proactive wake source")
	}
	changed := false
	err := store.ExecuteWriteOperation(ctx, r.db, "finish instance wake", func(ctx context.Context, tx *store.WriteTx) error {
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
			req.Result, req.Reason, req.Report = "cancelled", "cancelled", nil
			req.NextWakeAt = time.Time{}
		}
		if err := deduplicateInstanceProposals(ctx, q, &req); err != nil {
			return err
		}
		completed := 0
		quiet, failures := row.QuietStreak, row.FailureStreak
		switch req.Result {
		case "quiet":
			completed = 1
			quiet++
			failures = 0
		case "reported":
			completed = 1
			quiet, failures = 0, 0
		case "failed":
			failures++
		case "cancelled":
		default:
			return errors.New("invalid proactive wake result")
		}

		if req.Result == "failed" && req.FatigueLimit > 0 && failures >= req.FatigueLimit {
			if _, err := q.PauseAgentInstance(ctx, sqlcgen.PauseAgentInstanceParams{
				ID: req.InstanceID, PauseReason: "fatigue", UpdatedAt: store.FormatTimestamp(req.Now),
			}); err != nil {
				return err
			}
			req.Report = &agentinstance.ReportInput{Kind: "failure", Payload: []byte(`{"reason":"fatigue","detail":"Proactive checks paused after consecutive failures."}`)}
		}

		if req.Source == "instance_check" || req.Result == "cancelled" {
			req.NextWakeAt = time.Time{}
		} else if req.Interval > 0 && req.MaxInterval > 0 {
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
			req.NextWakeAt = req.Now.Add(instanceCheckBackoff(req.Interval, maximum, streak))
		}
		if req.Report != nil {
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
				ID: reportID, InstanceID: req.InstanceID, SourceKey: report.SourceKey, Kind: report.Kind,
				Payload: string(report.Payload), Fingerprint: report.Fingerprint, CreatedAt: store.FormatTimestamp(req.Now),
			}); err != nil {
				return err
			}
		}
		eventID, err := store.NewID("hwe")
		if err != nil {
			return err
		}
		if err := q.InsertInstanceWakeEvent(ctx, sqlcgen.InsertInstanceWakeEventParams{
			ID: eventID, InstanceID: req.InstanceID, Source: req.Source, Result: req.Result, Reason: req.Reason, Detail: req.Detail,
			DeniedCalls: req.DeniedCalls, OccurrenceID: req.OccurrenceID,
			CreatedAt: store.FormatTimestamp(req.Now), ExpiresAt: store.FormatTimestamp(req.ExpiresAt),
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
	})
	return changed, err
}

func instanceCheckBackoff(interval, maximum time.Duration, streak int64) time.Duration {
	delay := min(interval, maximum)
	for streak > 0 && delay < maximum {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
		streak--
	}
	return delay
}
