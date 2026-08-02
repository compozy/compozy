package demoseed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/task"
)

func seedTasks(ctx context.Context, db *globaldb.GlobalDB, workspaceID string, stories []taskStory) error {
	actor := operatorActor(workspaceID)
	for _, story := range stories {
		record := taskRecord(workspaceID, story)
		if err := db.CreateTask(ctx, record); err != nil {
			return fmt.Errorf("demo seed: create task %q: %w", story.ID, err)
		}
		if story.RunID == "" {
			continue
		}
		command, err := task.NewCompletedRunHistoryImport(
			completedTaskRun(workspaceID, story),
			actor,
		)
		if err != nil {
			return fmt.Errorf("demo seed: prepare completed run %q: %w", story.RunID, err)
		}
		if err := db.ImportCompletedRunHistory(ctx, &command); err != nil {
			return fmt.Errorf("demo seed: create run %q: %w", story.RunID, err)
		}
	}
	for _, story := range stories {
		for _, dependencyID := range story.DependencyTaskIDs {
			if err := db.CreateDependency(ctx, task.Dependency{
				TaskID: story.ID, DependsOnTaskID: dependencyID,
				Kind: task.DependencyKindBlocks, CreatedAt: story.CreatedAt.Add(time.Minute),
			}); err != nil {
				return fmt.Errorf("demo seed: create dependency %q -> %q: %w", story.ID, dependencyID, err)
			}
		}
		if story.BlockReason == "" {
			continue
		}
		if _, err := db.CreateTaskBlock(ctx, task.CreateTaskBlockMutation{
			Block: task.TaskBlock{
				ID: "block_" + story.ID, WorkspaceID: workspaceID, TaskID: story.ID,
				Kind: task.BlockKindNeedsInput, Reason: story.BlockReason,
				Details:   json.RawMessage(`{"market":"MX","missing_window":"14:08-14:30 UTC","owner":"MercadoX"}`),
				CreatedBy: task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: scenarioSessionIDs[0]},
				CreatedAt: story.UpdatedAt,
			},
			RecurrenceLimit: 3,
			Actor:           agentActor(workspaceID, scenarioSessionIDs[0]),
		}); err != nil {
			return fmt.Errorf("demo seed: block task %q: %w", story.ID, err)
		}
	}
	return nil
}

func taskRecord(workspaceID string, story taskStory) task.Task {
	metadata := json.RawMessage(`{"initiative":"checkout-launch","company":"Northstar Pay"}`)
	return task.Task{
		ID: story.ID, Identifier: story.Identifier,
		Scope: task.ScopeWorkspace, WorkspaceID: workspaceID,
		Title: story.Title, Description: story.Description, Priority: task.Priority(story.Priority),
		MaxAttempts: task.DefaultTaskMaxAttempts, Status: task.Status(story.Status),
		ApprovalPolicy: task.ApprovalPolicy(
			story.ApprovalPolicy,
		), ApprovalState: task.ApprovalState(story.ApprovalState),
		Owner:     &task.Ownership{Kind: task.OwnerKind(story.OwnerKind), Ref: story.OwnerRef},
		CreatedBy: task.ActorIdentity{Kind: task.ActorKindHuman, Ref: operatorRef},
		Origin:    task.Origin{Kind: task.OriginKindWeb, Ref: "launch-console"},
		CreatedAt: story.CreatedAt, UpdatedAt: story.UpdatedAt, ClosedAt: story.ClosedAt,
		Metadata: metadata,
	}
}

func completedTaskRun(workspaceID string, story taskStory) task.Run {
	startedAt := story.ClosedAt.Add(-8 * time.Minute)
	return task.Run{
		ID: story.RunID, TaskID: story.ID, WorkspaceID: workspaceID, Attempt: 1,
		RunKind: task.RunKindWorker, Status: task.TaskRunStatusCompleted,
		ClaimedBy:       &task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: story.SessionID},
		SessionID:       story.SessionID,
		Origin:          task.Origin{Kind: task.OriginKindAgentSession, Ref: story.SessionID},
		IdempotencyKey:  "launch/" + story.RunID,
		RunNetworkState: &task.RunNetworkState{NetworkSpec: participation.LocalSpec()},
		QueuedAt:        startedAt.Add(-time.Minute), ClaimedAt: startedAt.Add(-30 * time.Second),
		StartedAt: startedAt, EndedAt: story.ClosedAt, TokensUsed: story.TokensUsed,
		Result: json.RawMessage(story.Result),
	}
}

func operatorActor(workspaceID string) task.ActorContext {
	return task.ActorContext{
		Actor:     task.ActorIdentity{Kind: task.ActorKindHuman, Ref: operatorRef},
		Origin:    task.Origin{Kind: task.OriginKindWeb, Ref: "launch-console"},
		Authority: task.Authority{Read: true, Write: true, CreateWorkspace: true},
		Scope:     task.CallerScope{WorkspaceID: workspaceID, Operator: true},
	}
}

func agentActor(workspaceID string, sessionID string) task.ActorContext {
	return task.ActorContext{
		Actor:     task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: sessionID},
		Origin:    task.Origin{Kind: task.OriginKindAgentSession, Ref: sessionID},
		Authority: task.Authority{Read: true, Write: true},
		Scope:     task.CallerScope{SessionID: sessionID, WorkspaceID: workspaceID},
	}
}
