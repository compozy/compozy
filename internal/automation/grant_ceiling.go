package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

func updatedJobGrant(ctx context.Context, current, next Job) (string, error) {
	ceiling := current.GrantCeiling
	current.GrantCeiling, next.GrantCeiling = "", ""
	current.Name, next.Name = "", ""
	current.Enabled, next.Enabled = false, false
	current.CreatedAt, next.CreatedAt = time.Time{}, time.Time{}
	current.UpdatedAt, next.UpdatedAt = time.Time{}, time.Time{}
	before, err := json.Marshal(current)
	if err != nil {
		return "", err
	}
	after, err := json.Marshal(next)
	if err != nil {
		return "", err
	}
	return updatedGrant(ctx, ceiling, !bytes.Equal(before, after)), nil
}

func updatedTriggerGrant(ctx context.Context, current, next Trigger) (string, error) {
	ceiling := current.GrantCeiling
	current.GrantCeiling, next.GrantCeiling = "", ""
	current.Name, next.Name = "", ""
	current.Enabled, next.Enabled = false, false
	current.CreatedAt, next.CreatedAt = time.Time{}, time.Time{}
	current.UpdatedAt, next.UpdatedAt = time.Time{}, time.Time{}
	before, err := json.Marshal(current)
	if err != nil {
		return "", err
	}
	after, err := json.Marshal(next)
	if err != nil {
		return "", err
	}
	return updatedGrant(ctx, ceiling, !bytes.Equal(before, after)), nil
}

func updatedGrant(ctx context.Context, ceiling string, executableChanged bool) string {
	origin := workspaceaccess.OriginFromContext(ctx)
	if origin.Kind == workspaceaccess.ActorHuman {
		return workspaceaccess.Unrestricted
	}
	if !executableChanged {
		return ceiling
	}
	return workspaceaccess.NarrowGrant(ceiling, origin.Effective(workspaceaccess.Unrestricted))
}
