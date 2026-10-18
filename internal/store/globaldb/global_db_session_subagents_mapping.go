package globaldb

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func subagentFromSQL(row *sqlcgen.SessionSubagent) (store.SessionSubagent, error) {
	out := store.SessionSubagent{
		ID:        row.ID,
		Isolation: row.Isolation,
		Worktree: &store.SubagentWorktreeState{
			ID:      row.WorktreeID.String,
			Name:    row.WorktreeName.String,
			Branch:  row.WorktreeBranch.String,
			BaseRef: row.WorktreeBaseRef.String,
			BaseSHA: row.WorktreeBaseSha.String,
			Path:    row.WorktreePath.String,
			Cleanup: row.WorktreeCleanup.String,
			Facts: store.SubagentWorktreeFacts{
				HeadSHA:      row.GitHeadSha.String,
				CommitsAhead: subagentIntPointer(row.GitCommitsAhead),
				DirtyFiles:   subagentIntPointer(row.GitDirtyFiles),
				PRStatus:     row.PrStatus.String,
				PRURL:        row.PrUrl.String,
				PRNumber:     subagentIntPointer(row.PrNumber),
			},
		},
		WorkspaceID:        row.WorkspaceID,
		ParentSessionID:    row.ParentSessionID,
		ParentTurnID:       row.ParentTurnID,
		ParentToolCallID:   row.ParentToolCallID,
		ChildSessionID:     subagentStringPointer(row.ChildSessionID),
		Origin:             row.Origin,
		ProviderToolCallID: row.ProviderToolCallID,
		IdempotencyKey:     row.IdempotencyKey,
		RequestFingerprint: row.RequestFingerprint,
		Title:              row.Title,
		Role:               row.Role,
		TaskChars:          int(row.TaskChars),
		PendingTask:        subagentStringPointer(row.PendingTask),
		RuntimeAgent:       row.RuntimeAgent,
		RuntimeProvider:    row.RuntimeProvider,
		RuntimeModel:       row.RuntimeModel,
		RuntimeEffort:      row.RuntimeReasoningEffort,
		RuntimeSpeed:       row.RuntimeSpeed,
		Depth:              int(row.Depth),
		Status:             row.Status,
		WorkState:          row.WorkState,
		Progress:           row.Progress,
		Result:             subagentStringPointer(row.Result),
		ResultTruncated:    row.ResultTruncated != 0,
		Error:              subagentStringPointer(row.Error),
		WakePolicy:         row.WakePolicy,
		Delivery:           row.Delivery,
		WakeMessageID:      subagentStringPointer(row.WakeMessageID),
		AcknowledgedTurnID: row.AcknowledgedTurnID,
	}
	var err error
	if row.GitObservedAt.Valid {
		out.Worktree.Facts.ObservedAt, err = store.ParseTimestamp(row.GitObservedAt.String)
		if err != nil {
			return out, err
		}
	}
	out.StartedAt, err = parseOptionalSessionInputTimestamp(row.StartedAt)
	if err != nil {
		return out, fmt.Errorf("store: subagent StartedAt: %w", err)
	}
	out.SettledAt, err = parseOptionalSessionInputTimestamp(row.SettledAt)
	if err != nil {
		return out, fmt.Errorf("store: subagent SettledAt: %w", err)
	}
	out.CreatedAt, err = store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return out, fmt.Errorf("store: subagent CreatedAt: %w", err)
	}
	out.UpdatedAt, err = store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return out, fmt.Errorf("store: subagent UpdatedAt: %w", err)
	}
	if out.Worktree.ID == "" && out.Worktree.Cleanup == "" {
		out.Worktree = nil
	}
	return out, nil
}

func subagentsFromSQL(rows []sqlcgen.SessionSubagent) ([]store.SessionSubagent, error) {
	out := make([]store.SessionSubagent, 0, len(rows))
	for i := range rows {
		v, err := subagentFromSQL(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func subagentStringPointer(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return new(v.String)
}
func subagentNullString(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}
func subagentNullTime(v *time.Time) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: store.FormatTimestamp(*v), Valid: true}
}
func subagentBool(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
func subagentWakeFromSQL(row sqlcgen.SessionSubagentWake) (store.SessionSubagentWake, error) {
	out := store.SessionSubagentWake{
		WakeMessageID:   row.WakeMessageID,
		WorkspaceID:     row.WorkspaceID,
		ParentSessionID: row.ParentSessionID,
		State:           row.State,
		Route:           row.Route,
		InputEntryID:    row.InputEntryID,
		SteerRequeued:   row.SteerRequeued != 0,
		Attempts:        int(row.Attempts),
	}
	var err error
	out.CreatedAt, err = store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return out, err
	}
	out.UpdatedAt, err = store.ParseTimestamp(row.UpdatedAt)
	return out, err
}
func subagentInsertParams(row store.SessionSubagent) sqlcgen.ReserveSubagentParams {
	return sqlcgen.ReserveSubagentParams{
		ID:                     row.ID,
		Isolation:              row.Isolation,
		WorkspaceID:            row.WorkspaceID,
		ParentSessionID:        row.ParentSessionID,
		ParentTurnID:           row.ParentTurnID,
		ParentToolCallID:       row.ParentToolCallID,
		ChildSessionID:         subagentNullString(row.ChildSessionID),
		Origin:                 row.Origin,
		ProviderToolCallID:     row.ProviderToolCallID,
		IdempotencyKey:         row.IdempotencyKey,
		RequestFingerprint:     row.RequestFingerprint,
		Title:                  row.Title,
		Role:                   row.Role,
		TaskChars:              int64(row.TaskChars),
		PendingTask:            subagentNullString(row.PendingTask),
		RuntimeAgent:           row.RuntimeAgent,
		RuntimeProvider:        row.RuntimeProvider,
		RuntimeModel:           row.RuntimeModel,
		RuntimeReasoningEffort: row.RuntimeEffort,
		RuntimeSpeed:           row.RuntimeSpeed,
		Depth:                  int64(row.Depth),
		Status:                 row.Status,
		WorkState:              row.WorkState,
		Progress:               row.Progress,
		Result:                 subagentNullString(row.Result),
		ResultTruncated:        subagentBool(row.ResultTruncated),
		Error:                  subagentNullString(row.Error),
		WakePolicy:             row.WakePolicy,
		Delivery:               row.Delivery,
		WakeMessageID:          subagentNullString(row.WakeMessageID),
		AcknowledgedTurnID:     row.AcknowledgedTurnID,
		StartedAt:              subagentNullTime(row.StartedAt),
		SettledAt:              subagentNullTime(row.SettledAt),
		CreatedAt:              store.FormatTimestamp(row.CreatedAt),
		UpdatedAt:              store.FormatTimestamp(row.UpdatedAt),
	}
}
