package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	coreworkspace "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store"
)

// WorkspaceRegistry exposes durable workspace registration operations.
type WorkspaceRegistry interface {
	Resolve(context.Context, string) (Workspace, error)
	ResolveOrRegister(context.Context, string) (Workspace, error)
	Register(context.Context, string, string) (Workspace, error)
	Get(context.Context, string) (Workspace, error)
	List(context.Context) ([]Workspace, error)
	Unregister(context.Context, string) error
}

var _ WorkspaceRegistry = (*GlobalDB)(nil)

var (
	// ErrWorkspaceNotFound reports missing workspace registrations.
	ErrWorkspaceNotFound = errors.New("globaldb: workspace not found")
	// ErrWorkflowNotFound reports missing workflow rows.
	ErrWorkflowNotFound = errors.New("globaldb: workflow not found")
	// ErrRunNotFound reports missing run rows.
	ErrRunNotFound = errors.New("globaldb: run not found")
	// ErrWorkspaceHasActiveRuns reports unregister conflicts.
	ErrWorkspaceHasActiveRuns = errors.New("globaldb: workspace has active runs")
	// ErrWorkflowSlugConflict reports an active workflow slug collision.
	ErrWorkflowSlugConflict = errors.New("globaldb: workflow slug conflict")
	// ErrRunAlreadyExists reports a duplicate run identifier.
	ErrRunAlreadyExists = errors.New("globaldb: run already exists")
)

// Workspace captures one durable workspace registration.
type Workspace struct {
	ID        string
	RootDir   string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Workflow captures one durable workflow identity row.
type Workflow struct {
	ID           string
	WorkspaceID  string
	Slug         string
	ArchivedAt   *time.Time
	LastSyncedAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ListWorkflowsOptions controls workflow listing behavior.
type ListWorkflowsOptions struct {
	WorkspaceID     string
	IncludeArchived bool
}

// ListRunsOptions controls durable run listing behavior.
type ListRunsOptions struct {
	WorkspaceID string
	Status      string
	Mode        string
	Limit       int
}

// Run captures one durable global run index row.
type Run struct {
	RunID            string
	WorkspaceID      string
	WorkflowID       *string
	Mode             string
	Status           string
	PresentationMode string
	StartedAt        time.Time
	EndedAt          *time.Time
	ErrorText        string
	RequestID        string
}

// ActiveRunsError reports how many active runs blocked a workspace unregister.
type ActiveRunsError struct {
	WorkspaceID string
	ActiveRuns  int
}

func (e ActiveRunsError) Error() string {
	return fmt.Sprintf(
		"globaldb: workspace %q has %d active run(s)",
		e.WorkspaceID,
		e.ActiveRuns,
	)
}

func (e ActiveRunsError) Is(target error) bool {
	return target == ErrWorkspaceHasActiveRuns
}

// Resolve resolves a filesystem path and lazily registers the owning workspace.
func (g *GlobalDB) Resolve(ctx context.Context, path string) (Workspace, error) {
	return g.ResolveOrRegister(ctx, path)
}

// ResolveOrRegister resolves a filesystem path and returns one stable workspace row.
func (g *GlobalDB) ResolveOrRegister(ctx context.Context, path string) (Workspace, error) {
	if err := g.requireContext(ctx, "resolve workspace"); err != nil {
		return Workspace{}, err
	}

	rootDir, err := discoverWorkspaceRoot(ctx, path)
	if err != nil {
		return Workspace{}, err
	}

	existing, err := g.getWorkspaceByRootDir(ctx, rootDir)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrWorkspaceNotFound) {
		return Workspace{}, err
	}

	return g.registerResolvedWorkspace(ctx, rootDir, "")
}

// Register explicitly registers a workspace path and is idempotent on normalized roots.
func (g *GlobalDB) Register(ctx context.Context, path string, name string) (Workspace, error) {
	if err := g.requireContext(ctx, "register workspace"); err != nil {
		return Workspace{}, err
	}

	rootDir, err := discoverWorkspaceRoot(ctx, path)
	if err != nil {
		return Workspace{}, err
	}

	return g.registerResolvedWorkspace(ctx, rootDir, name)
}

// Get loads a workspace by id or normalized root path.
func (g *GlobalDB) Get(ctx context.Context, idOrPath string) (Workspace, error) {
	if err := g.requireContext(ctx, "get workspace"); err != nil {
		return Workspace{}, err
	}

	trimmed := strings.TrimSpace(idOrPath)
	if trimmed == "" {
		return Workspace{}, errors.New("globaldb: workspace id or path is required")
	}

	workspace, err := g.getWorkspaceByID(ctx, trimmed)
	if err == nil {
		return workspace, nil
	}
	if !errors.Is(err, ErrWorkspaceNotFound) {
		return Workspace{}, err
	}

	rootDir, resolveErr := discoverWorkspaceRoot(ctx, trimmed)
	if resolveErr == nil {
		workspace, err = g.getWorkspaceByRootDir(ctx, rootDir)
		if err == nil {
			return workspace, nil
		}
		if !errors.Is(err, ErrWorkspaceNotFound) {
			return Workspace{}, err
		}
	}

	workspace, err = g.getWorkspaceByRootDir(ctx, filepath.Clean(trimmed))
	if err == nil {
		return workspace, nil
	}
	if !errors.Is(err, ErrWorkspaceNotFound) {
		return Workspace{}, err
	}

	if resolveErr != nil {
		return Workspace{}, ErrWorkspaceNotFound
	}
	return Workspace{}, ErrWorkspaceNotFound
}

// List returns all registered workspaces in stable root order.
func (g *GlobalDB) List(ctx context.Context) ([]Workspace, error) {
	if err := g.requireContext(ctx, "list workspaces"); err != nil {
		return nil, err
	}

	rows, err := g.db.QueryContext(
		ctx,
		`SELECT id, root_dir, name, created_at, updated_at
		 FROM workspaces
		 ORDER BY root_dir ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query workspaces: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	workspaces := make([]Workspace, 0)
	for rows.Next() {
		workspace, scanErr := scanWorkspace(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		workspaces = append(workspaces, workspace)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate workspaces: %w", err)
	}

	return workspaces, nil
}

// Unregister removes a workspace only when no active runs are present.
func (g *GlobalDB) Unregister(ctx context.Context, idOrPath string) error {
	if err := g.requireContext(ctx, "unregister workspace"); err != nil {
		return err
	}

	workspace, err := g.Get(ctx, idOrPath)
	if err != nil {
		return err
	}

	activeRuns, err := g.countActiveRunsForWorkspace(ctx, workspace.ID)
	if err != nil {
		return err
	}
	if activeRuns > 0 {
		return ActiveRunsError{WorkspaceID: workspace.ID, ActiveRuns: activeRuns}
	}

	result, err := g.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, workspace.ID)
	if err != nil {
		return fmt.Errorf("globaldb: delete workspace %q: %w", workspace.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("globaldb: rows affected for workspace %q: %w", workspace.ID, err)
	}
	if affected == 0 {
		return ErrWorkspaceNotFound
	}

	return nil
}

// PutWorkflow inserts or updates one workflow identity row.
func (g *GlobalDB) PutWorkflow(ctx context.Context, workflow Workflow) (Workflow, error) {
	if err := g.requireContext(ctx, "put workflow"); err != nil {
		return Workflow{}, err
	}

	if strings.TrimSpace(workflow.ID) == "" {
		return g.insertWorkflow(ctx, workflow)
	}
	return g.updateWorkflow(ctx, workflow)
}

// GetWorkflow loads one workflow by primary key.
func (g *GlobalDB) GetWorkflow(ctx context.Context, id string) (Workflow, error) {
	if err := g.requireContext(ctx, "get workflow"); err != nil {
		return Workflow{}, err
	}

	row := g.db.QueryRowContext(
		ctx,
		`SELECT id, workspace_id, slug, archived_at, last_synced_at, created_at, updated_at
		 FROM workflows
		 WHERE id = ?`,
		strings.TrimSpace(id),
	)
	workflow, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workflow{}, ErrWorkflowNotFound
		}
		return Workflow{}, err
	}
	return workflow, nil
}

// GetActiveWorkflowBySlug loads the active workflow row for one workspace and slug.
func (g *GlobalDB) GetActiveWorkflowBySlug(ctx context.Context, workspaceID string, slug string) (Workflow, error) {
	if err := g.requireContext(ctx, "get active workflow"); err != nil {
		return Workflow{}, err
	}

	row := g.db.QueryRowContext(
		ctx,
		`SELECT id, workspace_id, slug, archived_at, last_synced_at, created_at, updated_at
		 FROM workflows
		 WHERE workspace_id = ? AND slug = ? AND archived_at IS NULL`,
		strings.TrimSpace(workspaceID),
		strings.TrimSpace(slug),
	)
	workflow, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workflow{}, ErrWorkflowNotFound
		}
		return Workflow{}, err
	}
	return workflow, nil
}

// ListWorkflows returns workflow rows for one workspace.
func (g *GlobalDB) ListWorkflows(ctx context.Context, opts ListWorkflowsOptions) ([]Workflow, error) {
	if err := g.requireContext(ctx, "list workflows"); err != nil {
		return nil, err
	}

	workspaceID := strings.TrimSpace(opts.WorkspaceID)
	if workspaceID == "" {
		return nil, errors.New("globaldb: workflow workspace id is required")
	}

	query := `
		SELECT id, workspace_id, slug, archived_at, last_synced_at, created_at, updated_at
		FROM workflows
		WHERE workspace_id = ?`
	args := []any{workspaceID}
	if !opts.IncludeArchived {
		query += ` AND archived_at IS NULL`
	}
	query += ` ORDER BY slug ASC, created_at ASC, id ASC`

	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query workflows: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	workflows := make([]Workflow, 0)
	for rows.Next() {
		workflow, scanErr := scanWorkflow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		workflows = append(workflows, workflow)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate workflows: %w", err)
	}

	return workflows, nil
}

// PutRun inserts one durable run index row.
func (g *GlobalDB) PutRun(ctx context.Context, run Run) (Run, error) {
	if err := g.requireContext(ctx, "put run"); err != nil {
		return Run{}, err
	}

	run.RunID = strings.TrimSpace(run.RunID)
	run.WorkspaceID = strings.TrimSpace(run.WorkspaceID)
	run.Mode = strings.TrimSpace(run.Mode)
	run.Status = normalizeRunStatus(run.Status)
	run.PresentationMode = strings.TrimSpace(run.PresentationMode)
	if run.RunID == "" {
		return Run{}, errors.New("globaldb: run id is required")
	}
	if run.WorkspaceID == "" {
		return Run{}, errors.New("globaldb: run workspace id is required")
	}
	if run.Mode == "" {
		return Run{}, errors.New("globaldb: run mode is required")
	}
	if run.Status == "" {
		return Run{}, errors.New("globaldb: run status is required")
	}
	if run.PresentationMode == "" {
		return Run{}, errors.New("globaldb: run presentation mode is required")
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = g.now()
	}

	_, err := g.db.ExecContext(
		ctx,
		`INSERT INTO runs (
			run_id, workspace_id, workflow_id, mode, status, presentation_mode,
			started_at, ended_at, error_text, request_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.RunID,
		run.WorkspaceID,
		store.NullableString(stringValue(run.WorkflowID)),
		run.Mode,
		run.Status,
		run.PresentationMode,
		store.FormatTimestamp(run.StartedAt),
		nullableTimestamp(run.EndedAt),
		strings.TrimSpace(run.ErrorText),
		strings.TrimSpace(run.RequestID),
	)
	if err != nil {
		if isDuplicateRunError(err) {
			return Run{}, ErrRunAlreadyExists
		}
		return Run{}, fmt.Errorf("globaldb: insert run %q: %w", run.RunID, err)
	}

	return g.GetRun(ctx, run.RunID)
}

// UpdateRun updates one durable run index row in place.
func (g *GlobalDB) UpdateRun(ctx context.Context, run Run) (Run, error) {
	if err := g.requireContext(ctx, "update run"); err != nil {
		return Run{}, err
	}

	run.RunID = strings.TrimSpace(run.RunID)
	run.WorkspaceID = strings.TrimSpace(run.WorkspaceID)
	run.Mode = strings.TrimSpace(run.Mode)
	run.Status = normalizeRunStatus(run.Status)
	run.PresentationMode = strings.TrimSpace(run.PresentationMode)
	if run.RunID == "" {
		return Run{}, errors.New("globaldb: run id is required")
	}
	if run.WorkspaceID == "" {
		return Run{}, errors.New("globaldb: run workspace id is required")
	}
	if run.Mode == "" {
		return Run{}, errors.New("globaldb: run mode is required")
	}
	if run.Status == "" {
		return Run{}, errors.New("globaldb: run status is required")
	}
	if run.PresentationMode == "" {
		return Run{}, errors.New("globaldb: run presentation mode is required")
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = g.now()
	}

	result, err := g.db.ExecContext(
		ctx,
		`UPDATE runs
		 SET workspace_id = ?,
		     workflow_id = ?,
		     mode = ?,
		     status = ?,
		     presentation_mode = ?,
		     started_at = ?,
		     ended_at = ?,
		     error_text = ?,
		     request_id = ?
		 WHERE run_id = ?`,
		run.WorkspaceID,
		store.NullableString(stringValue(run.WorkflowID)),
		run.Mode,
		run.Status,
		run.PresentationMode,
		store.FormatTimestamp(run.StartedAt),
		nullableTimestamp(run.EndedAt),
		strings.TrimSpace(run.ErrorText),
		strings.TrimSpace(run.RequestID),
		run.RunID,
	)
	if err != nil {
		return Run{}, fmt.Errorf("globaldb: update run %q: %w", run.RunID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return Run{}, fmt.Errorf("globaldb: rows affected for run %q: %w", run.RunID, err)
	}
	if affected == 0 {
		return Run{}, ErrRunNotFound
	}

	return g.GetRun(ctx, run.RunID)
}

// GetRun loads one run row by run identifier.
func (g *GlobalDB) GetRun(ctx context.Context, runID string) (Run, error) {
	if err := g.requireContext(ctx, "get run"); err != nil {
		return Run{}, err
	}

	row := g.db.QueryRowContext(
		ctx,
		`SELECT run_id, workspace_id, workflow_id, mode, status, presentation_mode,
		        started_at, ended_at, error_text, request_id
		 FROM runs
		 WHERE run_id = ?`,
		strings.TrimSpace(runID),
	)
	run, err := scanRun(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Run{}, ErrRunNotFound
		}
		return Run{}, err
	}
	return run, nil
}

// ListRuns returns run rows filtered by workspace, mode, or status.
func (g *GlobalDB) ListRuns(ctx context.Context, opts ListRunsOptions) ([]Run, error) {
	if err := g.requireContext(ctx, "list runs"); err != nil {
		return nil, err
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT run_id, workspace_id, workflow_id, mode, status, presentation_mode,
		       started_at, ended_at, error_text, request_id
		FROM runs
		WHERE 1 = 1`
	args := make([]any, 0, 4)

	if workspaceID := strings.TrimSpace(opts.WorkspaceID); workspaceID != "" {
		query += ` AND workspace_id = ?`
		args = append(args, workspaceID)
	}
	if status := strings.TrimSpace(opts.Status); status != "" {
		status = normalizeRunStatus(status)
		query += ` AND status = ?`
		args = append(args, status)
	}
	if mode := strings.TrimSpace(opts.Mode); mode != "" {
		query += ` AND mode = ?`
		args = append(args, mode)
	}
	query += ` ORDER BY started_at DESC, run_id ASC LIMIT ?`
	args = append(args, limit)

	rows, err := g.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query runs: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	runs := make([]Run, 0)
	for rows.Next() {
		run, scanErr := scanRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate runs: %w", err)
	}

	return runs, nil
}

func (g *GlobalDB) registerResolvedWorkspace(
	ctx context.Context,
	rootDir string,
	name string,
) (Workspace, error) {
	now := g.now()
	inserted := Workspace{
		ID:        g.newID("ws"),
		RootDir:   rootDir,
		Name:      normalizeWorkspaceName(name, rootDir),
		CreatedAt: now,
		UpdatedAt: now,
	}

	result, err := g.db.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO workspaces (id, root_dir, name, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		inserted.ID,
		inserted.RootDir,
		inserted.Name,
		store.FormatTimestamp(inserted.CreatedAt),
		store.FormatTimestamp(inserted.UpdatedAt),
	)
	if err != nil {
		return Workspace{}, fmt.Errorf("globaldb: insert workspace %q: %w", inserted.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return Workspace{}, fmt.Errorf("globaldb: rows affected for workspace %q: %w", inserted.ID, err)
	}
	if affected == 0 {
		existing, err := g.getWorkspaceByRootDir(ctx, rootDir)
		if err != nil {
			return Workspace{}, err
		}
		return existing, nil
	}

	return inserted, nil
}

func (g *GlobalDB) getWorkspaceByID(ctx context.Context, id string) (Workspace, error) {
	return getWorkspaceByID(ctx, g.db, id)
}

func (g *GlobalDB) getWorkspaceByRootDir(ctx context.Context, rootDir string) (Workspace, error) {
	return getWorkspaceByRootDir(ctx, g.db, rootDir)
}

func getWorkspaceByID(ctx context.Context, querier singleWorkspaceQuerier, id string) (Workspace, error) {
	row := querier.QueryRowContext(
		ctx,
		`SELECT id, root_dir, name, created_at, updated_at
		 FROM workspaces
		 WHERE id = ?`,
		strings.TrimSpace(id),
	)
	workspace, err := scanWorkspace(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workspace{}, ErrWorkspaceNotFound
		}
		return Workspace{}, err
	}
	return workspace, nil
}

func getWorkspaceByRootDir(ctx context.Context, querier singleWorkspaceQuerier, rootDir string) (Workspace, error) {
	row := querier.QueryRowContext(
		ctx,
		`SELECT id, root_dir, name, created_at, updated_at
		 FROM workspaces
		 WHERE root_dir = ?`,
		strings.TrimSpace(rootDir),
	)
	workspace, err := scanWorkspace(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Workspace{}, ErrWorkspaceNotFound
		}
		return Workspace{}, err
	}
	return workspace, nil
}

type singleWorkspaceQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type rowScanner interface {
	Scan(...any) error
}

func scanWorkspace(scanner rowScanner) (Workspace, error) {
	var (
		workspace    Workspace
		createdAtRaw string
		updatedAtRaw string
	)
	if err := scanner.Scan(
		&workspace.ID,
		&workspace.RootDir,
		&workspace.Name,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		return Workspace{}, err
	}

	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return Workspace{}, err
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return Workspace{}, err
	}

	workspace.CreatedAt = createdAt
	workspace.UpdatedAt = updatedAt
	return workspace, nil
}

func (g *GlobalDB) insertWorkflow(ctx context.Context, workflow Workflow) (Workflow, error) {
	workflow.WorkspaceID = strings.TrimSpace(workflow.WorkspaceID)
	workflow.Slug = strings.TrimSpace(workflow.Slug)
	if workflow.WorkspaceID == "" {
		return Workflow{}, errors.New("globaldb: workflow workspace id is required")
	}
	if workflow.Slug == "" {
		return Workflow{}, errors.New("globaldb: workflow slug is required")
	}
	if workflow.ID == "" {
		workflow.ID = g.newID("wf")
	}
	if workflow.CreatedAt.IsZero() {
		workflow.CreatedAt = g.now()
	}
	if workflow.UpdatedAt.IsZero() {
		workflow.UpdatedAt = workflow.CreatedAt
	}

	_, err := g.db.ExecContext(
		ctx,
		`INSERT INTO workflows (
			id, workspace_id, slug, archived_at, last_synced_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		workflow.ID,
		workflow.WorkspaceID,
		workflow.Slug,
		nullableTimestamp(workflow.ArchivedAt),
		nullableTimestamp(workflow.LastSyncedAt),
		store.FormatTimestamp(workflow.CreatedAt),
		store.FormatTimestamp(workflow.UpdatedAt),
	)
	if err != nil {
		if isWorkflowSlugConflict(err) {
			return Workflow{}, ErrWorkflowSlugConflict
		}
		return Workflow{}, fmt.Errorf("globaldb: insert workflow %q: %w", workflow.ID, err)
	}

	return g.GetWorkflow(ctx, workflow.ID)
}

func (g *GlobalDB) updateWorkflow(ctx context.Context, workflow Workflow) (Workflow, error) {
	workflow.ID = strings.TrimSpace(workflow.ID)
	workflow.WorkspaceID = strings.TrimSpace(workflow.WorkspaceID)
	workflow.Slug = strings.TrimSpace(workflow.Slug)
	if workflow.ID == "" {
		return Workflow{}, errors.New("globaldb: workflow id is required")
	}
	if workflow.WorkspaceID == "" {
		return Workflow{}, errors.New("globaldb: workflow workspace id is required")
	}
	if workflow.Slug == "" {
		return Workflow{}, errors.New("globaldb: workflow slug is required")
	}
	if workflow.UpdatedAt.IsZero() {
		workflow.UpdatedAt = g.now()
	}

	result, err := g.db.ExecContext(
		ctx,
		`UPDATE workflows
		 SET workspace_id = ?, slug = ?, archived_at = ?, last_synced_at = ?, updated_at = ?
		 WHERE id = ?`,
		workflow.WorkspaceID,
		workflow.Slug,
		nullableTimestamp(workflow.ArchivedAt),
		nullableTimestamp(workflow.LastSyncedAt),
		store.FormatTimestamp(workflow.UpdatedAt),
		workflow.ID,
	)
	if err != nil {
		if isWorkflowSlugConflict(err) {
			return Workflow{}, ErrWorkflowSlugConflict
		}
		return Workflow{}, fmt.Errorf("globaldb: update workflow %q: %w", workflow.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return Workflow{}, fmt.Errorf("globaldb: rows affected for workflow %q: %w", workflow.ID, err)
	}
	if affected == 0 {
		return Workflow{}, ErrWorkflowNotFound
	}

	return g.GetWorkflow(ctx, workflow.ID)
}

func scanWorkflow(scanner rowScanner) (Workflow, error) {
	var (
		workflow        Workflow
		archivedAtRaw   sql.NullString
		lastSyncedAtRaw sql.NullString
		createdAtRaw    string
		updatedAtRaw    string
	)
	if err := scanner.Scan(
		&workflow.ID,
		&workflow.WorkspaceID,
		&workflow.Slug,
		&archivedAtRaw,
		&lastSyncedAtRaw,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		return Workflow{}, err
	}

	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return Workflow{}, err
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return Workflow{}, err
	}

	workflow.CreatedAt = createdAt
	workflow.UpdatedAt = updatedAt
	if archivedAtRaw.Valid {
		archivedAt, err := store.ParseTimestamp(archivedAtRaw.String)
		if err != nil {
			return Workflow{}, err
		}
		workflow.ArchivedAt = &archivedAt
	}
	if lastSyncedAtRaw.Valid {
		lastSyncedAt, err := store.ParseTimestamp(lastSyncedAtRaw.String)
		if err != nil {
			return Workflow{}, err
		}
		workflow.LastSyncedAt = &lastSyncedAt
	}

	return workflow, nil
}

func scanRun(scanner rowScanner) (Run, error) {
	var (
		run           Run
		workflowIDRaw sql.NullString
		endedAtRaw    sql.NullString
		startedAtRaw  string
	)
	if err := scanner.Scan(
		&run.RunID,
		&run.WorkspaceID,
		&workflowIDRaw,
		&run.Mode,
		&run.Status,
		&run.PresentationMode,
		&startedAtRaw,
		&endedAtRaw,
		&run.ErrorText,
		&run.RequestID,
	); err != nil {
		return Run{}, err
	}

	startedAt, err := store.ParseTimestamp(startedAtRaw)
	if err != nil {
		return Run{}, err
	}
	run.StartedAt = startedAt
	run.WorkflowID = store.NullString(workflowIDRaw)
	if endedAtRaw.Valid {
		endedAt, err := store.ParseTimestamp(endedAtRaw.String)
		if err != nil {
			return Run{}, err
		}
		run.EndedAt = &endedAt
	}

	return run, nil
}

func discoverWorkspaceRoot(ctx context.Context, path string) (string, error) {
	rootDir, err := coreworkspace.Discover(ctx, path)
	if err != nil {
		return "", fmt.Errorf("globaldb: discover workspace root: %w", err)
	}
	return normalizeWorkspaceRoot(rootDir)
}

func normalizeWorkspaceRoot(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("globaldb: workspace path is required")
	}

	absolutePath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("globaldb: resolve workspace path %q: %w", path, err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		return "", fmt.Errorf("globaldb: resolve workspace path symlinks %q: %w", path, err)
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("globaldb: stat workspace path %q: %w", resolvedPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("globaldb: workspace path %q is not a directory", resolvedPath)
	}

	canonicalPath, err := canonicalizeExistingPathCase(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("globaldb: canonicalize workspace path %q: %w", resolvedPath, err)
	}
	return filepath.Clean(canonicalPath), nil
}

func canonicalizeExistingPathCase(path string) (string, error) {
	return canonicalizeExistingPathCaseWith(path, os.ReadDir)
}

func canonicalizeExistingPathCaseWith(
	path string,
	readDir func(string) ([]os.DirEntry, error),
) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("globaldb: workspace path is required")
	}
	cleanPath := filepath.Clean(trimmed)
	if !filepath.IsAbs(cleanPath) {
		return cleanPath, nil
	}

	volume := filepath.VolumeName(cleanPath)
	current := string(filepath.Separator)
	remainder := strings.TrimPrefix(cleanPath, current)
	if volume != "" {
		current = volume + string(filepath.Separator)
		remainder = strings.TrimPrefix(cleanPath, current)
	}
	if remainder == "" {
		return filepath.Clean(current), nil
	}

	for _, component := range strings.Split(remainder, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}

		entries, err := readDir(current)
		if err != nil {
			// Case normalization is best-effort once callers have a real path.
			return cleanPath, nil
		}

		matchedName, ok := matchPathComponentCase(component, entries)
		if !ok {
			return cleanPath, nil
		}
		current = filepath.Join(current, matchedName)
	}

	return filepath.Clean(current), nil
}

func matchPathComponentCase(component string, entries []os.DirEntry) (string, bool) {
	for _, entry := range entries {
		if entry.Name() == component {
			return entry.Name(), true
		}
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), component) {
			return entry.Name(), true
		}
	}
	return "", false
}

func normalizeWorkspaceName(name string, rootDir string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed != "" {
		return trimmed
	}

	base := strings.TrimSpace(filepath.Base(rootDir))
	switch base {
	case "", ".", string(filepath.Separator):
		return "workspace"
	default:
		return base
	}
}

func nullableTimestamp(value *time.Time) any {
	if value == nil {
		return nil
	}
	return store.FormatTimestamp(*value)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (g *GlobalDB) countActiveRunsForWorkspace(ctx context.Context, workspaceID string) (int, error) {
	var count int
	if err := g.db.QueryRowContext(
		ctx,
		`SELECT COUNT(1)
		 FROM runs
		 WHERE workspace_id = ?
		   AND status NOT IN ('completed', 'failed', 'canceled', 'crashed')`,
		strings.TrimSpace(workspaceID),
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("globaldb: count active runs for workspace %q: %w", workspaceID, err)
	}
	return count, nil
}

func isWorkflowSlugConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "workflows.workspace_id, workflows.slug") ||
		strings.Contains(message, "uq_workflows_active_slug")
}

func isDuplicateRunError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "runs.run_id")
}
