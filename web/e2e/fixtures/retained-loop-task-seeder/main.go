package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type options struct {
	homeDir     string
	profileID   string
	workspaceID string
	taskID      string
	runID       string
	loopRunID   string
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) (err error) {
	options, err := parseOptions(args)
	if err != nil {
		return err
	}
	homePaths, err := compozyconfig.ResolveHomePathsFrom(options.homeDir)
	if err != nil {
		return fmt.Errorf("resolve isolated home: %w", err)
	}
	store, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
	if err != nil {
		return fmt.Errorf("open isolated global database: %w", err)
	}
	defer func() {
		err = errors.Join(err, store.Close(ctx))
	}()

	now := time.Now().UTC()
	metadata, err := json.Marshal(map[string]string{"loop_run_id": options.loopRunID})
	if err != nil {
		return fmt.Errorf("encode retained task metadata: %w", err)
	}
	record := taskpkg.Task{
		ID:             options.taskID,
		ProfileID:      options.profileID,
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    options.workspaceID,
		Title:          "Retained Loop execution record",
		Priority:       taskpkg.PriorityMedium,
		MaxAttempts:    1,
		Status:         taskpkg.TaskStatusCompleted,
		ApprovalPolicy: taskpkg.ApprovalPolicyNone,
		ApprovalState:  taskpkg.ApprovalStateNotRequired,
		CreatedBy:      taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop-coordinator"},
		Origin: taskpkg.Origin{
			Kind: taskpkg.OriginKindDaemon,
			Ref:  "retention-fixture",
		},
		CreatedAt: now,
		UpdatedAt: now,
		ClosedAt:  now,
		Metadata:  metadata,
	}
	if err := store.CreateTask(ctx, record); err != nil {
		return fmt.Errorf("create retained task: %w", err)
	}
	if _, err := store.DB().ExecContext(
		ctx,
		`INSERT INTO task_runs
			(id, task_id, workspace_id, status, attempt, origin_kind, origin_ref,
			 queued_at, ended_at, metadata_json, run_kind, loop_run_id)
		 VALUES (?, ?, ?, 'completed', 1, 'daemon', 'retention-fixture', ?, ?, '{}', 'worker', ?)`,
		options.runID,
		options.taskID,
		options.workspaceID,
		now.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
		options.loopRunID,
	); err != nil {
		return fmt.Errorf("create retained task run: %w", err)
	}
	return nil
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("retained-loop-task-seeder", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var parsed options
	flags.StringVar(&parsed.homeDir, "home", "", "isolated CompozyOS home")
	flags.StringVar(&parsed.profileID, "profile", "", "owning profile id")
	flags.StringVar(&parsed.workspaceID, "workspace", "", "workspace id")
	flags.StringVar(&parsed.taskID, "task", "", "task id")
	flags.StringVar(&parsed.runID, "run", "", "task run id")
	flags.StringVar(&parsed.loopRunID, "loop-run", "", "retained Loop run id")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if strings.TrimSpace(parsed.homeDir) == "" || strings.TrimSpace(parsed.profileID) == "" ||
		strings.TrimSpace(parsed.workspaceID) == "" ||
		strings.TrimSpace(parsed.taskID) == "" || strings.TrimSpace(parsed.runID) == "" ||
		strings.TrimSpace(parsed.loopRunID) == "" {
		return options{}, errors.New("retained Loop task seeder requires home, profile, workspace, task, run, and loop-run")
	}
	return parsed, nil
}
