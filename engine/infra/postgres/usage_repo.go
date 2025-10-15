package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var usageColumns = []string{
	"id",
	"workflow_exec_id",
	"task_exec_id",
	"component",
	"agent_id",
	"provider",
	"model",
	"prompt_tokens",
	"completion_tokens",
	"total_tokens",
	"reasoning_tokens",
	"cached_prompt_tokens",
	"input_audio_tokens",
	"output_audio_tokens",
	"created_at",
	"updated_at",
}

func selectUsageBuilder() squirrel.SelectBuilder {
	return squirrel.
		Select(usageColumns...).
		From("execution_llm_usage").
		PlaceholderFormat(squirrel.Dollar)
}

func buildUsageUpsert(row *usage.Row) (string, []any, error) {
	builder := squirrel.
		Insert("execution_llm_usage").
		Columns(
			"workflow_exec_id",
			"task_exec_id",
			"component",
			"agent_id",
			"provider",
			"model",
			"prompt_tokens",
			"completion_tokens",
			"total_tokens",
			"reasoning_tokens",
			"cached_prompt_tokens",
			"input_audio_tokens",
			"output_audio_tokens",
		).
		Values(
			nullableID(row.WorkflowExecID),
			nullableID(row.TaskExecID),
			string(row.Component),
			nullableID(row.AgentID),
			row.Provider,
			row.Model,
			row.PromptTokens,
			row.CompletionTokens,
			row.TotalTokens,
			nullableInt(row.ReasoningTokens),
			nullableInt(row.CachedPromptTokens),
			nullableInt(row.InputAudioTokens),
			nullableInt(row.OutputAudioTokens),
		).
		PlaceholderFormat(squirrel.Dollar).
		Suffix(usageConflictClause(row))

	return builder.ToSql()
}

func usageConflictClause(row *usage.Row) string {
	target := "(task_exec_id, component) WHERE task_exec_id IS NOT NULL"
	if isZeroID(row.TaskExecID) {
		target = "(workflow_exec_id, component) WHERE task_exec_id IS NULL"
	}
	return fmt.Sprintf(`
ON CONFLICT %s DO UPDATE SET
    workflow_exec_id = EXCLUDED.workflow_exec_id,
    task_exec_id = EXCLUDED.task_exec_id,
    agent_id = EXCLUDED.agent_id,
    provider = EXCLUDED.provider,
    model = EXCLUDED.model,
    prompt_tokens = EXCLUDED.prompt_tokens,
    completion_tokens = EXCLUDED.completion_tokens,
    total_tokens = EXCLUDED.total_tokens,
    reasoning_tokens = EXCLUDED.reasoning_tokens,
    cached_prompt_tokens = EXCLUDED.cached_prompt_tokens,
    input_audio_tokens = EXCLUDED.input_audio_tokens,
    output_audio_tokens = EXCLUDED.output_audio_tokens,
    updated_at = now()`, target)
}

// UsageRepo persists LLM usage rows backed by Postgres.
// It enforces referential integrity with workflow and task state tables.
type UsageRepo struct {
	db DB
}

// NewUsageRepo constructs a UsageRepo using the provided DB interface.
// The DB must satisfy pgx-compatible query semantics.
func NewUsageRepo(db DB) *UsageRepo {
	return &UsageRepo{db: db}
}

// Upsert inserts or updates a usage row, enforcing FK relationships.
// It validates mandatory identifiers and normalizes optional fields.
func (r *UsageRepo) Upsert(ctx context.Context, row *usage.Row) error {
	if row == nil {
		return fmt.Errorf("usage row is required")
	}
	if row.Component == "" {
		return fmt.Errorf("component is required")
	}
	if row.Provider == "" || row.Model == "" {
		return fmt.Errorf("provider and model are required")
	}
	if isZeroID(row.WorkflowExecID) && isZeroID(row.TaskExecID) {
		return fmt.Errorf("either workflow_exec_id or task_exec_id must be provided")
	}
	sql, args, err := buildUsageUpsert(row)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, sql, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
			return fmt.Errorf("upsert usage: foreign key violation: %w", err)
		}
		return fmt.Errorf("upsert usage: %w", err)
	}
	return nil
}

// GetByTaskExecID fetches usage linked to a task execution id.
// It returns usage.ErrNotFound when no matching record exists.
func (r *UsageRepo) GetByTaskExecID(ctx context.Context, id core.ID) (*usage.Row, error) {
	builder := selectUsageBuilder().
		Where(squirrel.Eq{"task_exec_id": id.String()}).
		Limit(1)
	return r.getOne(ctx, builder)
}

// GetByWorkflowExecID fetches workflow-level usage for an execution id.
// It filters on the workflow component to avoid task-level collisions.
func (r *UsageRepo) GetByWorkflowExecID(ctx context.Context, id core.ID) (*usage.Row, error) {
	builder := selectUsageBuilder().
		Where(squirrel.Eq{"workflow_exec_id": id.String()}).
		Where(squirrel.Eq{"component": string(core.ComponentWorkflow)}).
		Limit(1)
	return r.getOne(ctx, builder)
}

// SummarizeByWorkflowExecID aggregates token usage across all components for a workflow execution.
func (r *UsageRepo) SummarizeByWorkflowExecID(ctx context.Context, id core.ID) (*usage.Row, error) {
	builder := selectUsageBuilder().
		Where(squirrel.Eq{"workflow_exec_id": id.String()}).
		Where(squirrel.NotEq{"component": string(core.ComponentWorkflow)}).
		Where("task_exec_id IS NOT NULL")
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build workflow usage summary select: %w", err)
	}
	var rows []usageRowDB
	if err := scanAll(ctx, r.db, &rows, sql, args...); err != nil {
		return nil, fmt.Errorf("query workflow usage summary: %w", err)
	}
	if len(rows) == 0 {
		return nil, usage.ErrNotFound
	}
	summary := &usage.Row{Component: core.ComponentWorkflow}
	workflowID := id
	summary.WorkflowExecID = &workflowID

	var acc usageAccumulator
	for i := range rows {
		acc.add(&rows[i])
	}
	acc.apply(summary)

	return summary, nil
}

type usageAccumulator struct {
	prompt      int
	completion  int
	total       int
	reasoning   optionalAccumulator
	cached      optionalAccumulator
	inputAudio  optionalAccumulator
	outputAudio optionalAccumulator
	providers   map[string]struct{}
	provider    string
	model       string
	createdAt   time.Time
	updatedAt   time.Time
}

func (a *usageAccumulator) add(row *usageRowDB) {
	if row == nil {
		return
	}
	a.prompt += row.PromptTokens
	a.completion += row.CompletionTokens
	a.total += row.TotalTokens
	a.reasoning.add(row.ReasoningTokens)
	a.cached.add(row.CachedPromptTokens)
	a.inputAudio.add(row.InputAudioTokens)
	a.outputAudio.add(row.OutputAudioTokens)
	if a.providers == nil {
		a.providers = make(map[string]struct{})
	}
	comboKey := row.Provider + "|" + row.Model
	if _, ok := a.providers[comboKey]; !ok {
		a.providers[comboKey] = struct{}{}
		if len(a.providers) == 1 {
			a.provider = row.Provider
			a.model = row.Model
		}
	}
	if a.createdAt.IsZero() || row.CreatedAt.Before(a.createdAt) {
		a.createdAt = row.CreatedAt
	}
	if row.UpdatedAt.After(a.updatedAt) {
		a.updatedAt = row.UpdatedAt
	}
}

func (a *usageAccumulator) apply(summary *usage.Row) {
	summary.PromptTokens = a.prompt
	summary.CompletionTokens = a.completion
	summary.TotalTokens = a.total
	if summary.TotalTokens == 0 {
		summary.TotalTokens = summary.PromptTokens + summary.CompletionTokens
	}
	if a.reasoning.set {
		summary.ReasoningTokens = intPtr(a.reasoning.total)
	}
	if a.cached.set {
		summary.CachedPromptTokens = intPtr(a.cached.total)
	}
	if a.inputAudio.set {
		summary.InputAudioTokens = intPtr(a.inputAudio.total)
	}
	if a.outputAudio.set {
		summary.OutputAudioTokens = intPtr(a.outputAudio.total)
	}
	summary.CreatedAt = a.createdAt
	summary.UpdatedAt = a.updatedAt
	if len(a.providers) == 1 {
		summary.Provider = a.provider
		summary.Model = a.model
	} else {
		summary.Provider = "mixed"
		summary.Model = "mixed"
	}
}

type optionalAccumulator struct {
	total int
	set   bool
}

func (o *optionalAccumulator) add(value sql.NullInt32) {
	if !value.Valid {
		return
	}
	o.total += int(value.Int32)
	o.set = true
}

func (r *UsageRepo) getOne(ctx context.Context, builder squirrel.SelectBuilder) (*usage.Row, error) {
	var dbRow usageRowDB
	sql, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build usage select: %w", err)
	}
	if err := scanOne(ctx, r.db, &dbRow, sql, args...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, usage.ErrNotFound
		}
		return nil, fmt.Errorf("query usage: %w", err)
	}
	return dbRow.toDomain(), nil
}

type usageRowDB struct {
	ID                 int64              `db:"id"`
	WorkflowExecID     sql.NullString     `db:"workflow_exec_id"`
	TaskExecID         sql.NullString     `db:"task_exec_id"`
	Component          core.ComponentType `db:"component"`
	AgentID            sql.NullString     `db:"agent_id"`
	Provider           string             `db:"provider"`
	Model              string             `db:"model"`
	PromptTokens       int                `db:"prompt_tokens"`
	CompletionTokens   int                `db:"completion_tokens"`
	TotalTokens        int                `db:"total_tokens"`
	ReasoningTokens    sql.NullInt32      `db:"reasoning_tokens"`
	CachedPromptTokens sql.NullInt32      `db:"cached_prompt_tokens"`
	InputAudioTokens   sql.NullInt32      `db:"input_audio_tokens"`
	OutputAudioTokens  sql.NullInt32      `db:"output_audio_tokens"`
	CreatedAt          time.Time          `db:"created_at"`
	UpdatedAt          time.Time          `db:"updated_at"`
}

func (r *usageRowDB) toDomain() *usage.Row {
	return &usage.Row{
		ID:                 r.ID,
		WorkflowExecID:     toCoreIDPtr(r.WorkflowExecID),
		TaskExecID:         toCoreIDPtr(r.TaskExecID),
		Component:          r.Component,
		AgentID:            toCoreIDPtr(r.AgentID),
		Provider:           r.Provider,
		Model:              r.Model,
		PromptTokens:       r.PromptTokens,
		CompletionTokens:   r.CompletionTokens,
		TotalTokens:        r.TotalTokens,
		ReasoningTokens:    toIntPtr(r.ReasoningTokens),
		CachedPromptTokens: toIntPtr(r.CachedPromptTokens),
		InputAudioTokens:   toIntPtr(r.InputAudioTokens),
		OutputAudioTokens:  toIntPtr(r.OutputAudioTokens),
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

func nullableID(id *core.ID) any {
	if id == nil || id.IsZero() {
		return nil
	}
	return id.String()
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func intPtr(value int) *int {
	v := value
	return &v
}

func toCoreIDPtr(ns sql.NullString) *core.ID {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	id := core.ID(ns.String)
	return &id
}

func toIntPtr(ns sql.NullInt32) *int {
	if !ns.Valid {
		return nil
	}
	value := int(ns.Int32)
	return &value
}

func isZeroID(id *core.ID) bool {
	return id == nil || id.IsZero()
}
