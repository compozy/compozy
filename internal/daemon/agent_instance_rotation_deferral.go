package daemon

import (
	"context"
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
)

func (r *instanceMainRotator) recordRotationDeferral(
	ctx context.Context,
	instance agentinstance.Instance,
	eligible bool, blockers []string,
) error {
	if eligible {
		return r.store.ClearRotationWait(ctx, instance.ID, instance.MainSessionID)
	}
	now := r.now()
	since, err := r.store.RotationWaitingSince(ctx, instance.ID, instance.MainSessionID, now)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(
		map[string]any{
			loopActionRuntimeReasonKey: "dependencies_pending",
			"since":                    since.UTC().Format(time.RFC3339Nano),
			"blockers":                 blockers,
			"detail":                   "Context exhausted. Resolve the listed dependencies to allow rotation.",
		},
	)
	if err != nil {
		return err
	}
	_, err = r.reports.AppendReport(
		ctx,
		instance.ID,
		agentinstance.ReportInput{
			SourceKey: "rotation:deferred:" + instance.ID + ":" + instance.MainSessionID,
			Kind:      "rotation_deferred",
			Payload:   payload,
		},
	)
	return err
}
