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
	eligible bool,
) error {
	if eligible {
		return r.store.ClearRotationWait(ctx, instance.ID, instance.MainSessionID)
	}
	now := r.now()
	since, err := r.store.RotationWaitingSince(ctx, instance.ID, instance.MainSessionID, now)
	if err != nil || now.Sub(since) < time.Hour {
		return err
	}
	payload, err := json.Marshal(
		map[string]string{"reason": "dependencies_pending", "since": since.UTC().Format(time.RFC3339Nano)},
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
