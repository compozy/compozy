package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

var _ taskpkg.TerminalRunCommandStore = (*TaskRepo)(nil)

func terminalRunCommandInsertParams(
	command taskpkg.TerminalRunCommand,
) (sqlcgen.InsertTaskRunTerminalCommandParams, error) {
	record, err := command.Record()
	if err != nil {
		return sqlcgen.InsertTaskRunTerminalCommandParams{}, err
	}
	return sqlcgen.InsertTaskRunTerminalCommandParams{
		CommandID:            record.CommandID,
		Kind:                 string(record.Kind),
		Phase:                string(record.Phase),
		SourceStatus:         record.SourceStatus.String(),
		SourceSessionID:      record.SourceSession,
		SourceClaimTokenHash: record.SourceClaimHash,
		SourceLeaseUntil:     nullableTaskTime(record.SourceLeaseUntil),
		IntentJson:           string(record.IntentJSON),
		ActorJson:            string(record.ActorJSON),
		CommandAt:            storepkg.FormatTimestamp(record.CommandAt),
		AdmittedAt:           storepkg.FormatTimestamp(record.AdmittedAt),
		UpdatedAt:            storepkg.FormatTimestamp(record.UpdatedAt),
		RunID:                record.RunID,
		TaskID:               sql.NullString{String: record.TaskID, Valid: true},
		WorkspaceID:          sql.NullString{String: record.WorkspaceID, Valid: true},
	}, nil
}

func taskRunTerminalCommandFromRow(
	row sqlcgen.TaskRunTerminalCommand,
) (taskpkg.TerminalRunCommand, error) {
	commandAt, err := storepkg.ParseTimestamp(row.CommandAt)
	if err != nil {
		return taskpkg.TerminalRunCommand{}, fmt.Errorf("store: parse terminal command timestamp: %w", err)
	}
	admittedAt, err := storepkg.ParseTimestamp(row.AdmittedAt)
	if err != nil {
		return taskpkg.TerminalRunCommand{}, fmt.Errorf("store: parse terminal admission timestamp: %w", err)
	}
	updatedAt, err := storepkg.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return taskpkg.TerminalRunCommand{}, fmt.Errorf("store: parse terminal command update timestamp: %w", err)
	}
	leaseUntil := time.Time{}
	if row.SourceLeaseUntil.Valid {
		leaseUntil, err = storepkg.ParseTimestamp(row.SourceLeaseUntil.String)
		if err != nil {
			return taskpkg.TerminalRunCommand{}, fmt.Errorf("store: parse terminal command lease fence: %w", err)
		}
	}
	command, err := taskpkg.RestoreTerminalRunCommand(taskpkg.TerminalRunCommandRecord{
		CommandID:        row.CommandID,
		RunID:            row.RunID,
		TaskID:           row.TaskID,
		WorkspaceID:      row.WorkspaceID,
		Kind:             taskpkg.TerminalRunCommandKind(row.Kind),
		Phase:            taskpkg.TerminalRunCommandPhase(row.Phase),
		SourceStatus:     taskpkg.ParseRunStatus(row.SourceStatus),
		SourceSession:    row.SourceSessionID,
		SourceClaimHash:  row.SourceClaimTokenHash,
		SourceLeaseUntil: leaseUntil,
		IntentJSON:       json.RawMessage(row.IntentJson),
		ActorJSON:        json.RawMessage(row.ActorJson),
		CommandAt:        commandAt,
		AdmittedAt:       admittedAt,
		UpdatedAt:        updatedAt,
	})
	if err != nil {
		return taskpkg.TerminalRunCommand{}, fmt.Errorf(
			"store: restore terminal command for run %q: %w",
			row.RunID,
			err,
		)
	}
	return command, nil
}

// ReserveTerminalRunCommand admits one exact terminal intent before any
// external session effect. Existing commands are returned only in the same
// workspace/task/run scope.
func (g *TaskRepo) ReserveTerminalRunCommand(
	ctx context.Context,
	command taskpkg.TerminalRunCommand,
) (taskpkg.TerminalRunCommand, bool, error) {
	if err := g.checkReady(ctx, "reserve terminal run command"); err != nil {
		return taskpkg.TerminalRunCommand{}, false, err
	}
	params, err := terminalRunCommandInsertParams(command)
	if err != nil {
		return taskpkg.TerminalRunCommand{}, false, err
	}
	var authoritative taskpkg.TerminalRunCommand
	inserted := false
	err = g.withTaskImmediateTransaction(ctx, "reserve terminal run command", func(exec taskSQLExecutor) error {
		queries := sqlcgen.New(exec)
		affected, insertErr := queries.InsertTaskRunTerminalCommand(ctx, params)
		if insertErr != nil {
			return fmt.Errorf("store: insert terminal command for run %q: %w", command.RunID(), insertErr)
		}
		if affected == 1 {
			authoritative = command
			inserted = true
			return nil
		}
		row, getErr := queries.GetTaskRunTerminalCommandByScope(
			ctx,
			sqlcgen.GetTaskRunTerminalCommandByScopeParams{
				RunID:       command.RunID(),
				TaskID:      command.TaskID(),
				WorkspaceID: command.WorkspaceID(),
			},
		)
		if errors.Is(getErr, sql.ErrNoRows) {
			return fmt.Errorf(
				"%w: task run %q changed before terminal command admission",
				taskpkg.ErrInvalidStatusTransition,
				command.RunID(),
			)
		}
		if getErr != nil {
			return fmt.Errorf("store: read terminal command for run %q: %w", command.RunID(), getErr)
		}
		authoritative, getErr = taskRunTerminalCommandFromRow(row)
		return getErr
	})
	if err != nil {
		return taskpkg.TerminalRunCommand{}, false, err
	}
	return authoritative, inserted, nil
}

// AdvanceTerminalRunCommandPhase durably records the external-effect boundary.
func (g *TaskRepo) AdvanceTerminalRunCommandPhase(
	ctx context.Context,
	command taskpkg.TerminalRunCommand,
	next taskpkg.TerminalRunCommandPhase,
	updatedAt time.Time,
) (taskpkg.TerminalRunCommand, error) {
	if err := g.checkReady(ctx, "advance terminal run command"); err != nil {
		return taskpkg.TerminalRunCommand{}, err
	}
	if _, err := command.Advance(next, updatedAt); err != nil {
		return taskpkg.TerminalRunCommand{}, err
	}
	var advanced taskpkg.TerminalRunCommand
	err := g.withTaskImmediateTransaction(ctx, "advance terminal run command", func(exec taskSQLExecutor) error {
		queries := sqlcgen.New(exec)
		affected, updateErr := queries.AdvanceTaskRunTerminalCommandPhase(
			ctx,
			sqlcgen.AdvanceTaskRunTerminalCommandPhaseParams{
				NextPhase:     string(next),
				UpdatedAt:     storepkg.FormatTimestamp(updatedAt),
				CommandID:     command.CommandID(),
				RunID:         command.RunID(),
				TaskID:        command.TaskID(),
				WorkspaceID:   command.WorkspaceID(),
				ExpectedPhase: string(command.Phase()),
			},
		)
		if updateErr != nil {
			return fmt.Errorf("store: advance terminal command for run %q: %w", command.RunID(), updateErr)
		}
		if affected != 1 {
			return fmt.Errorf("%w: task run %q", taskpkg.ErrTerminalRunCommandInProgress, command.RunID())
		}
		row, getErr := queries.GetTaskRunTerminalCommandByScope(ctx, sqlcgen.GetTaskRunTerminalCommandByScopeParams{
			RunID: command.RunID(), TaskID: command.TaskID(), WorkspaceID: command.WorkspaceID(),
		})
		if getErr != nil {
			return fmt.Errorf("store: read advanced terminal command for run %q: %w", command.RunID(), getErr)
		}
		advanced, getErr = taskRunTerminalCommandFromRow(row)
		return getErr
	})
	if err != nil {
		return taskpkg.TerminalRunCommand{}, err
	}
	return advanced, nil
}

// ReleaseTerminalRunCommand removes only the matching receipt while its full
// source fence is still current.
func (g *TaskRepo) ReleaseTerminalRunCommand(
	ctx context.Context,
	command taskpkg.TerminalRunCommand,
) error {
	if err := g.checkReady(ctx, "release terminal run command"); err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, "release terminal run command", func(exec taskSQLExecutor) error {
		affected, err := sqlcgen.New(exec).ReleaseTaskRunTerminalCommand(
			ctx,
			sqlcgen.ReleaseTaskRunTerminalCommandParams{
				CommandID: command.CommandID(), RunID: command.RunID(),
				TaskID: command.TaskID(), WorkspaceID: command.WorkspaceID(),
				ExpectedPhase: string(command.Phase()),
			},
		)
		if err != nil {
			return fmt.Errorf("store: release terminal command for run %q: %w", command.RunID(), err)
		}
		if affected != 1 {
			return fmt.Errorf("%w: task run %q", taskpkg.ErrTerminalRunCommandInProgress, command.RunID())
		}
		return nil
	})
}

// ListTerminalRunCommands lists durable recovery obligations for daemon boot.
func (g *TaskRepo) ListTerminalRunCommands(
	ctx context.Context,
) ([]taskpkg.TerminalRunCommand, error) {
	if err := g.checkReady(ctx, "list terminal run commands"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListTaskRunTerminalCommands(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: list terminal run commands: %w", err)
	}
	commands := make([]taskpkg.TerminalRunCommand, 0, len(rows))
	for _, row := range rows {
		command, mapErr := taskRunTerminalCommandFromRow(row)
		if mapErr != nil {
			return nil, mapErr
		}
		commands = append(commands, command)
	}
	return commands, nil
}
