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

func seedTasks(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	stories []taskStory,
) (int, error) {
	runs := 0
	for _, story := range stories {
		record, err := state.recordFor(story.WorkspaceKey)
		if err != nil {
			return 0, err
		}
		if err := db.CreateTask(ctx, taskRecord(record.ID, story)); err != nil {
			return 0, fmt.Errorf("demo seed: create task %q: %w", story.ID, err)
		}
		imported, err := importTaskHistory(ctx, db, record.ID, story)
		if err != nil {
			return 0, err
		}
		runs += imported
	}
	return runs, seedTaskRelations(ctx, db, state, stories)
}

func importTaskHistory(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspaceID string,
	story taskStory,
) (int, error) {
	actor := operatorActor(workspaceID)
	for _, run := range story.History {
		command, err := task.NewTerminalRunHistoryImport(historicalTaskRun(workspaceID, story, run), actor)
		if err != nil {
			return 0, fmt.Errorf("demo seed: prepare historical run %q: %w", run.ID, err)
		}
		if err := db.ImportTerminalRunHistory(ctx, &command); err != nil {
			return 0, fmt.Errorf("demo seed: import historical run %q: %w", run.ID, err)
		}
	}
	return len(story.History), nil
}

func seedTaskRelations(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	stories []taskStory,
) error {
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
		if err := createTaskBlock(ctx, db, state, story); err != nil {
			return err
		}
	}
	return nil
}

func createTaskBlock(
	ctx context.Context,
	db *globaldb.GlobalDB,
	state *scenario,
	story taskStory,
) error {
	record, err := state.recordFor(story.WorkspaceKey)
	if err != nil {
		return err
	}
	if _, err := db.CreateTaskBlock(ctx, task.CreateTaskBlockMutation{
		Block: task.TaskBlock{
			ID: "block_" + story.ID, WorkspaceID: record.ID, TaskID: story.ID,
			Kind: task.BlockKindNeedsInput, Reason: story.BlockReason,
			Details:   json.RawMessage(story.BlockDetails),
			CreatedBy: task.ActorIdentity{Kind: task.ActorKindHuman, Ref: operatorRef},
			CreatedAt: story.UpdatedAt,
		},
		RecurrenceLimit: 3,
		Actor:           operatorActor(record.ID),
	}); err != nil {
		return fmt.Errorf("demo seed: block task %q: %w", story.ID, err)
	}
	return nil
}

func taskRecord(workspaceID string, story taskStory) task.Task {
	metadata := json.RawMessage(fmt.Sprintf(
		`{"initiative":%q,"company":"Northstar Pay"}`, story.Initiative,
	))
	return task.Task{
		ID: story.ID, Identifier: story.Identifier,
		Scope: task.ScopeWorkspace, WorkspaceID: workspaceID,
		Title: story.Title, Description: story.Description, Priority: task.Priority(story.Priority),
		MaxAttempts: task.DefaultTaskMaxAttempts, Status: task.Status(story.Status),
		ApprovalPolicy: task.ApprovalPolicy(story.ApprovalPolicy),
		ApprovalState:  task.ApprovalState(story.ApprovalState),
		Owner:          &task.Ownership{Kind: task.OwnerKind(story.OwnerKind), Ref: story.OwnerRef},
		CreatedBy:      task.ActorIdentity{Kind: task.ActorKindHuman, Ref: operatorRef},
		Origin:         task.Origin{Kind: task.OriginKindWeb, Ref: originWebRef},
		CreatedAt:      story.CreatedAt, UpdatedAt: story.UpdatedAt, ClosedAt: story.ClosedAt,
		Metadata: metadata,
	}
}

func historicalTaskRun(workspaceID string, story taskStory, run taskRunStory) task.Run {
	record := task.Run{
		ID: run.ID, TaskID: story.ID, WorkspaceID: workspaceID, Attempt: max(1, run.Attempt),
		RunKind: task.RunKindWorker, Status: run.Status,
		Origin:          task.Origin{Kind: task.OriginKindWeb, Ref: originWebRef},
		IdempotencyKey:  "northstar/" + run.ID,
		RunNetworkState: &task.RunNetworkState{NetworkSpec: participation.LocalSpec()},
		QueuedAt:        run.StartedAt.Add(-time.Minute), ClaimedAt: run.StartedAt.Add(-30 * time.Second),
		StartedAt: run.StartedAt, EndedAt: run.EndedAt, TokensUsed: run.TokensUsed,
		Error: run.Error,
	}
	if run.Result != "" {
		record.Result = json.RawMessage(run.Result)
	}
	if run.SessionID != "" {
		record.ClaimedBy = &task.ActorIdentity{Kind: task.ActorKindAgentSession, Ref: run.SessionID}
		record.SessionID = run.SessionID
		record.Origin = task.Origin{Kind: task.OriginKindAgentSession, Ref: run.SessionID}
	}
	return record
}

func operatorActor(workspaceID string) task.ActorContext {
	return task.ActorContext{
		Actor:     task.ActorIdentity{Kind: task.ActorKindHuman, Ref: operatorRef},
		Origin:    task.Origin{Kind: task.OriginKindWeb, Ref: originWebRef},
		Authority: task.Authority{Read: true, Write: true, CreateWorkspace: true},
		Scope:     task.CallerScope{WorkspaceID: workspaceID, Operator: true},
	}
}
