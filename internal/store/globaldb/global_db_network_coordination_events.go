package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type coordinationEventContent struct {
	WorkspaceID     string `json:"workspace_id"`
	Scope           string `json:"scope"`
	TaskID          string `json:"task_id,omitempty"`
	RunID           string `json:"run_id,omitempty"`
	Enabled         bool   `json:"enabled"`
	Revision        int64  `json:"revision"`
	ActorKind       string `json:"actor_kind"`
	ActorID         string `json:"actor_id"`
	InvitationState string `json:"invitation_state,omitempty"`
}

type taskCoordinationProfileEvent struct {
	TaskID               string `json:"task_id"`
	NetworkParticipation string `json:"network_participation"`
	CoordinationRevision int64  `json:"coordination_revision"`
}

func (g *WorkspaceRepo) setTaskCoordinationIntentWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	cmd workspacepkg.SetCoordination,
	setting workspacepkg.CoordinationSetting,
	actor taskpkg.ActorContext,
) error {
	profile, found, err := loadExecutionProfile(ctx, exec, cmd.Ref.TaskID)
	if err != nil {
		return err
	}
	if !found {
		profile = taskpkg.DefaultExecutionProfile(cmd.Ref.TaskID)
	}
	mode := participation.ModeLocal
	request := &participation.Request{Mode: &mode}
	if cmd.Enabled {
		mode = participation.ModeLive
		strategy := participation.StrategyRun
		request.ChannelStrategy = &strategy
	}
	profile.NetworkParticipation = request
	if _, err := g.tasks.upsertExecutionProfileWithExecutor(ctx, exec, &profile); err != nil {
		return err
	}
	return appendTaskEventPayloadWithExecutor(
		ctx,
		exec,
		cmd.Ref.TaskID,
		"",
		eventspkg.TaskExecutionProfileUpdated,
		actor,
		g.now().UTC(),
		taskCoordinationProfileEvent{
			TaskID:               cmd.Ref.TaskID,
			NetworkParticipation: string(mode),
			CoordinationRevision: setting.Revision,
		},
	)
}

func mutateCoordinationInvitation(
	ctx context.Context,
	exec taskSQLExecutor,
	cmd workspacepkg.SetInvitation,
	actor taskpkg.ActorContext,
	now time.Time,
) error {
	scopeID := coordinationScopeID(cmd.Ref)
	queries := sqlcgen.New(exec)
	if !cmd.Dismissed {
		if err := queries.DeleteNetworkCoordinationInvitation(
			ctx,
			sqlcgen.DeleteNetworkCoordinationInvitationParams{
				WorkspaceID: cmd.Ref.WorkspaceID, ScopeKind: cmd.Ref.ScopeKind, ScopeID: scopeID,
			},
		); err != nil {
			return fmt.Errorf("store: reset coordination invitation: %w", err)
		}
		return nil
	}
	_, err := queries.UpsertNetworkCoordinationInvitation(
		ctx,
		sqlcgen.UpsertNetworkCoordinationInvitationParams{
			WorkspaceID: cmd.Ref.WorkspaceID,
			ScopeKind:   cmd.Ref.ScopeKind,
			ScopeID:     scopeID,
			DismissedAt: store.FormatTimestamp(now),
			DismissedBy: actor.Actor.Ref,
		},
	)
	if err != nil {
		return fmt.Errorf("store: dismiss coordination invitation: %w", err)
	}
	return nil
}

func appendCoordinationEventSummary(
	ctx context.Context,
	exec taskSQLExecutor,
	eventType string,
	ref workspacepkg.CoordinationRef,
	setting workspacepkg.CoordinationSetting,
	actor taskpkg.ActorContext,
	invitationState string,
) error {
	content, err := json.Marshal(coordinationEventContent{
		WorkspaceID:     ref.WorkspaceID,
		Scope:           ref.ScopeKind,
		TaskID:          ref.TaskID,
		RunID:           ref.RunID,
		Enabled:         setting.Enabled,
		Revision:        setting.Revision,
		ActorKind:       string(actor.Actor.Kind),
		ActorID:         actor.Actor.Ref,
		InvitationState: invitationState,
	})
	if err != nil {
		return fmt.Errorf("store: marshal coordination event: %w", err)
	}
	_, err = exec.ExecContext(
		ctx,
		`INSERT INTO event_summaries
        (id, workspace_id, type, content_json, task_id, run_id, actor_kind, actor_id, summary, timestamp)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		store.NewID("sum"),
		ref.WorkspaceID,
		eventType,
		string(content),
		ref.TaskID,
		ref.RunID,
		string(actor.Actor.Kind),
		actor.Actor.Ref,
		"Network coordination changed",
		store.FormatTimestamp(setting.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("store: append coordination event %q: %w", eventType, err)
	}
	return nil
}
