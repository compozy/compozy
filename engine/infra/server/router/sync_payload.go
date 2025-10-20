package router

import (
	"context"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/gin-gonic/gin"
)

// SyncPayloadOptions defines configuration for building synchronous execution payloads.
type SyncPayloadOptions struct {
	Context      *gin.Context
	Repo         task.Repository
	ExecID       core.ID
	Output       *core.Output
	IncludeState bool
	OnState      func(*task.State) (payload any, embedsUsage bool)
	OnStateError func(error)
}

// BuildSyncPayload constructs the response payload returned from synchronous execution endpoints.
func BuildSyncPayload(opts SyncPayloadOptions) gin.H {
	ctx := opts.Context.Request.Context()
	payload := gin.H{"exec_id": opts.ExecID.String()}
	if opts.Output != nil {
		payload["output"] = opts.Output
	}
	snapshotCtx := context.WithoutCancel(ctx)
	embeddedUsage := false
	if opts.IncludeState {
		stateSnapshot, stateErr := opts.Repo.GetState(snapshotCtx, opts.ExecID)
		if stateErr == nil && stateSnapshot != nil {
			if opts.OnState != nil {
				statePayload, embedsUsage := opts.OnState(stateSnapshot)
				if statePayload != nil {
					payload["state"] = statePayload
				}
				if embedsUsage {
					embeddedUsage = true
				}
			}
			if stateSnapshot.Output != nil {
				payload["output"] = stateSnapshot.Output
			}
		} else if stateErr != nil && opts.OnStateError != nil {
			opts.OnStateError(stateErr)
		}
	}
	if !embeddedUsage {
		if summary := ResolveTaskUsageSummary(snapshotCtx, opts.Repo, opts.ExecID); summary != nil {
			payload["usage"] = summary
		}
	}
	return payload
}
