package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type InstanceEventRepo struct {
	*repoBase
	minInterval time.Duration
}

var _ automation.InstanceEventSink = (*InstanceEventRepo)(nil)

func (g *GlobalDB) InstanceEvents(minInterval time.Duration) *InstanceEventRepo {
	return &InstanceEventRepo{repoBase: newRepoBase(g.db, g.readDB, g.now, &g.closed), minInterval: minInterval}
}

func (r *InstanceEventRepo) Deliver(
	ctx context.Context,
	target automation.InstanceTarget,
	event automation.InstanceEvent,
) (automation.InstanceEventDelivery, error) {
	if err := r.checkReady(ctx, "deliver instance event"); err != nil {
		return automation.InstanceEventDelivery{}, err
	}
	if target.ProfileID == "" || target.AgentName == "" || event.RunID == "" || event.TriggerID == "" ||
		r.minInterval <= 0 {
		return automation.InstanceEventDelivery{}, errors.New(
			"instance event requires target, run, trigger and positive minimum interval",
		)
	}
	created := false
	var result automation.InstanceEventDelivery
	err := store.ExecuteWriteOperation(
		ctx,
		r.db,
		"deliver instance event",
		func(ctx context.Context, tx *store.WriteTx) error {
			q := sqlcgen.New(tx)
			instance, err := q.FindLiveAgentInstance(
				ctx,
				sqlcgen.FindLiveAgentInstanceParams{
					ProfileID:   target.ProfileID,
					AgentName:   target.AgentName,
					WorkspaceID: target.WorkspaceID,
				},
			)
			if errors.Is(err, sql.ErrNoRows) {
				return automation.ErrTriggerTargetInstanceNotFound
			}
			if err != nil {
				return err
			}
			supported, err := instanceEventCheckSupported(ctx, q, instance.ID, target.ChecksSupported)
			if err != nil {
				return err
			}
			result = instanceEventDelivery(instance, supported)
			receipt, err := q.GetInstanceEventReceipt(ctx, sqlcgen.GetInstanceEventReceiptParams{
				ID:        event.RunID,
				ProfileID: sql.NullString{String: target.ProfileID, Valid: true},
				TriggerID: sql.NullString{String: event.TriggerID, Valid: true},
			})
			if err != nil {
				return err
			}
			recordedEventID, validEvent := receipt.EventID.(string)
			recordedInstanceID, validInstance := receipt.InstanceID.(string)
			if !validEvent || !validInstance {
				return errors.New("invalid instance event receipt")
			}
			if recordedEventID != "" {
				if recordedInstanceID != instance.ID {
					return errors.New("automation run already delivered to another instance")
				}
				result.EventID = recordedEventID
				return nil
			}
			result.EventID, err = store.NewID("aie")
			if err != nil {
				return err
			}
			created = true
			now := r.now()
			if err := appendInstanceEvent(ctx, q, instance.ID, event, result.EventID, now); err != nil {
				return err
			}
			if !supported || instance.ChecksEnabled == 0 {
				return nil
			}
			return r.scheduleInstanceEvent(ctx, q, instance.ID, now)
		},
	)
	observeInstanceEvent(ctx, result, event.RunID, created, err)
	return result, err
}

func (r *InstanceEventRepo) scheduleInstanceEvent(
	ctx context.Context,
	q *sqlcgen.Queries,
	instanceID string,
	now time.Time,
) error {
	wake, err := q.GetInstanceWakeState(ctx, instanceID)
	if err != nil {
		return err
	}
	next := now
	if wake.LastWakeAt != "" {
		last, err := store.ParseTimestamp(wake.LastWakeAt)
		if err != nil {
			return err
		}
		if floor := last.Add(r.minInterval); floor.After(next) {
			next = floor
		}
	}
	return q.PullForwardInstanceEventWake(
		ctx,
		sqlcgen.PullForwardInstanceEventWakeParams{InstanceID: instanceID, NextWakeAt: store.FormatTimestamp(next)},
	)
}

func trimInstanceEvents(ctx context.Context, q *sqlcgen.Queries, instanceID string) error {
	omitted, err := q.CountOmittedInstanceEvents(ctx, instanceID)
	if err != nil {
		return err
	}
	if err := q.AddOmittedInstanceEvents(
		ctx,
		sqlcgen.AddOmittedInstanceEventsParams{InstanceID: instanceID, Omitted: omitted},
	); err != nil {
		return err
	}
	return q.TrimInstanceEvents(ctx, instanceID)
}

func instanceEventDelivery(instance sqlcgen.AgentInstance, supported bool) automation.InstanceEventDelivery {
	result := automation.InstanceEventDelivery{InstanceID: instance.ID, CheckScheduled: true, Reason: inputStatusQueued}
	switch {
	case !supported:
		result.CheckScheduled, result.Reason = false, "triage_unsupported"
	case instance.ChecksEnabled == 0:
		result.CheckScheduled, result.Reason = false, "checks_disabled"
	case instance.State != globalDBSessionStateActive:
		result.CheckScheduled, result.Reason = false, "instance_"+instance.State
	}
	return result
}

func instanceEventCheckSupported(
	ctx context.Context,
	q *sqlcgen.Queries,
	instanceID string,
	supported bool,
) (bool, error) {
	if !supported {
		return false, nil
	}
	reason, err := q.LatestInstanceEventCheckReason(ctx, instanceID)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return reason != "triage_unsupported", nil
}

func appendInstanceEvent(
	ctx context.Context,
	q *sqlcgen.Queries,
	instanceID string,
	event automation.InstanceEvent,
	eventID string,
	now time.Time,
) error {
	if err := q.InsertAgentInstanceEvent(ctx, sqlcgen.InsertAgentInstanceEventParams{
		ID:         eventID,
		InstanceID: instanceID,
		TriggerID:  event.TriggerID,
		RunID:      event.RunID,
		Summary: heartbeat.BoundAutomationEventSummary(
			heartbeat.CompactAutomationEventSummary(redact.String(event.Summary)),
		),
		ReceivedAt: store.FormatTimestamp(now),
	}); err != nil {
		return err
	}
	if err := q.InsertInstanceWakeState(ctx, instanceID); err != nil {
		return err
	}
	if err := trimInstanceEvents(ctx, q, instanceID); err != nil {
		return err
	}
	return q.RecordInstanceEventReceipt(
		ctx,
		sqlcgen.RecordInstanceEventReceiptParams{RunID: event.RunID, EventID: eventID, InstanceID: instanceID,
			TriggerName: event.TriggerName, Event: event.Event, Instruction: event.Instruction},
	)
}

func observeInstanceEvent(
	ctx context.Context,
	result automation.InstanceEventDelivery,
	runID string,
	created bool,
	err error,
) {
	if err == nil && created {
		slog.InfoContext(
			ctx,
			"instance.event.received",
			"instance_id",
			result.InstanceID,
			"event_id",
			result.EventID,
			"run_id",
			runID,
		)
	}
}
