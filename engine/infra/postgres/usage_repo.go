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
	"e.id AS id",
	"e.workflow_exec_id AS workflow_exec_id",
	"e.task_exec_id AS task_exec_id",
	"e.component AS component",
	"e.agent_id AS agent_id",
	"e.provider AS provider",
	"e.model AS model",
	"e.prompt_tokens AS prompt_tokens",
	"e.completion_tokens AS completion_tokens",
	"e.total_tokens AS total_tokens",
	"e.reasoning_tokens AS reasoning_tokens",
	"e.cached_prompt_tokens AS cached_prompt_tokens",
	"e.input_audio_tokens AS input_audio_tokens",
	"e.output_audio_tokens AS output_audio_tokens",
	"e.created_at AS created_at",
	"e.updated_at AS updated_at",
}

func selectUsageBuilder() squirrel.SelectBuilder {
	return squirrel.
		Select(usageColumns...).
		From("execution_llm_usage e").
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
		target = "(workflow_exec_id, component) WHERE task_exec_id IS NULL AND workflow_exec_id IS NOT NULL"
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
		Where(squirrel.Eq{"e.task_exec_id": id.String()}).
		OrderBy("e.updated_at DESC").
		Limit(1)
	return r.getOne(ctx, builder)
}

// GetByWorkflowExecID fetches workflow-level usage for an execution id.
// It filters on the workflow component to avoid task-level collisions.
func (r *UsageRepo) GetByWorkflowExecID(ctx context.Context, id core.ID) (*usage.Row, error) {
	builder := selectUsageBuilder().
		Where(squirrel.Eq{"e.workflow_exec_id": id.String()}).
		Where(squirrel.Eq{"e.component": string(core.ComponentWorkflow)}).
		OrderBy("e.updated_at DESC").
		Limit(1)
	return r.getOne(ctx, builder)
}

// SummarizeByWorkflowExecID aggregates token usage across all components for a workflow execution.
func (r *UsageRepo) SummarizeByWorkflowExecID(ctx context.Context, id core.ID) (*usage.Row, error) {
	builder := selectUsageBuilder().
		LeftJoin("task_states ts ON e.task_exec_id = ts.task_exec_id").
		Where(squirrel.NotEq{"e.component": string(core.ComponentWorkflow)}).
		Where(squirrel.Or{
			squirrel.Eq{"e.workflow_exec_id": id.String()},
			squirrel.Eq{"ts.workflow_exec_id": id.String()},
		})
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

// SummariesByWorkflowExecIDs aggregates usage rows for multiple workflow executions in a single query.
// Missing executions are omitted from the result map so callers can fall back to secondary strategies.
func (r *UsageRepo) SummariesByWorkflowExecIDs(
	ctx context.Context,
	ids []core.ID,
) (map[core.ID]*usage.Row, error) {
	requested := dedupeWorkflowExecIDs(ids)
	if len(requested) == 0 {
		return map[core.ID]*usage.Row{}, nil
	}
	rows, err := r.loadWorkflowUsageRows(ctx, requested)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return map[core.ID]*usage.Row{}, nil
	}
	accumulators := reduceWorkflowUsage(rows, requested)
	if len(accumulators) == 0 {
		return map[core.ID]*usage.Row{}, nil
	}
	return buildWorkflowSummaries(accumulators, requested), nil
}

func dedupeWorkflowExecIDs(ids []core.ID) map[string]core.ID {
	unique := make(map[string]core.ID, len(ids))
	for _, id := range ids {
		if id.IsZero() {
			continue
		}
		unique[id.String()] = id
	}
	return unique
}

func (r *UsageRepo) loadWorkflowUsageRows(
	ctx context.Context,
	requested map[string]core.ID,
) ([]workflowUsageRow, error) {
	idValues := make([]string, 0, len(requested))
	for key := range requested {
		idValues = append(idValues, key)
	}
	builder := selectUsageBuilder().
		LeftJoin("task_states ts ON e.task_exec_id = ts.task_exec_id").
		Column("ts.workflow_exec_id AS task_workflow_exec_id").
		Where(squirrel.NotEq{"e.component": string(core.ComponentWorkflow)}).
		Where(squirrel.Or{
			squirrel.Eq{"e.workflow_exec_id": idValues},
			squirrel.Eq{"ts.workflow_exec_id": idValues},
		})
	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build workflow usage batch select: %w", err)
	}
	var rows []workflowUsageRow
	if err := scanAll(ctx, r.db, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("query workflow usage batch: %w", err)
	}
	return rows, nil
}

func reduceWorkflowUsage(
	rows []workflowUsageRow,
	requested map[string]core.ID,
) map[string]*usageAccumulator {
	acc := make(map[string]*usageAccumulator, len(requested))
	for i := range rows {
		key := resolvedWorkflowExecKey(&rows[i])
		if key == "" {
			continue
		}
		if _, ok := requested[key]; !ok {
			continue
		}
		collector := acc[key]
		if collector == nil {
			collector = &usageAccumulator{}
			acc[key] = collector
		}
		rowCopy := rows[i].usageRowDB
		collector.add(&rowCopy)
	}
	return acc
}

func resolvedWorkflowExecKey(row *workflowUsageRow) string {
	if row == nil {
		return ""
	}
	if row.WorkflowExecID.Valid && row.WorkflowExecID.String != "" {
		return row.WorkflowExecID.String
	}
	if row.TaskWorkflowExecID.Valid && row.TaskWorkflowExecID.String != "" {
		return row.TaskWorkflowExecID.String
	}
	return ""
}

func buildWorkflowSummaries(
	accumulators map[string]*usageAccumulator,
	requested map[string]core.ID,
) map[core.ID]*usage.Row {
	summaries := make(map[core.ID]*usage.Row, len(accumulators))
	for key, collector := range accumulators {
		idValue, ok := requested[key]
		if !ok {
			continue
		}
		idCopy := idValue
		summary := &usage.Row{
			Component:      core.ComponentWorkflow,
			WorkflowExecID: &idCopy,
		}
		collector.apply(summary)
		summaries[idValue] = summary
	}
	return summaries
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

func (o *optionalAccumulator) add(value sql.NullInt64) {
	if !value.Valid {
		return
	}
	o.total += int(value.Int64)
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
	ReasoningTokens    sql.NullInt64      `db:"reasoning_tokens"`
	CachedPromptTokens sql.NullInt64      `db:"cached_prompt_tokens"`
	InputAudioTokens   sql.NullInt64      `db:"input_audio_tokens"`
	OutputAudioTokens  sql.NullInt64      `db:"output_audio_tokens"`
	CreatedAt          time.Time          `db:"created_at"`
	UpdatedAt          time.Time          `db:"updated_at"`
}

type workflowUsageRow struct {
	usageRowDB
	TaskWorkflowExecID sql.NullString `db:"task_workflow_exec_id"`
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

func toIntPtr(ns sql.NullInt64) *int {
	if !ns.Valid {
		return nil
	}
	value := int(ns.Int64)
	return &value
}

func isZeroID(id *core.ID) bool {
	return id == nil || id.IsZero()
}
