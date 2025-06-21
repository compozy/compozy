package store

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/compozy/compozy/engine/core"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// QueryFilters provides organization-aware query filtering capabilities
type QueryFilters struct {
	orgContext *OrganizationContext
}

// NewQueryFilters creates a new query filters instance
func NewQueryFilters() *QueryFilters {
	return &QueryFilters{
		orgContext: NewOrganizationContext(),
	}
}

// ApplyOrgFilter applies organization filtering to any squirrel SelectBuilder
func (qf *QueryFilters) ApplyOrgFilter(ctx context.Context, sb squirrel.SelectBuilder) (squirrel.SelectBuilder, error) {
	return qf.orgContext.FilterByOrganization(ctx, sb)
}

// ApplyOrgFilterUpdate applies organization filtering to any squirrel UpdateBuilder
func (qf *QueryFilters) ApplyOrgFilterUpdate(
	ctx context.Context,
	ub squirrel.UpdateBuilder,
) (squirrel.UpdateBuilder, error) {
	return qf.orgContext.FilterUpdateByOrganization(ctx, ub)
}

// ApplyOrgFilterDelete applies organization filtering to any squirrel DeleteBuilder
func (qf *QueryFilters) ApplyOrgFilterDelete(
	ctx context.Context,
	db squirrel.DeleteBuilder,
) (squirrel.DeleteBuilder, error) {
	return qf.orgContext.FilterDeleteByOrganization(ctx, db)
}

// WithOrgFilter is a convenience function that applies org filtering and builds the query
func (qf *QueryFilters) WithOrgFilter(ctx context.Context, sb squirrel.SelectBuilder) (string, []any, error) {
	filteredSB, err := qf.ApplyOrgFilter(ctx, sb)
	if err != nil {
		return "", nil, fmt.Errorf("applying org filter: %w", err)
	}

	sql, args, err := filteredSB.ToSql()
	if err != nil {
		return "", nil, fmt.Errorf("building query: %w", err)
	}

	return sql, args, nil
}

// ExecuteOrgFilteredQuery executes a SELECT query with automatic org filtering
func (qf *QueryFilters) ExecuteOrgFilteredQuery(
	ctx context.Context,
	db DBInterface,
	sb squirrel.SelectBuilder,
	dest any,
) error {
	sql, args, err := qf.WithOrgFilter(ctx, sb)
	if err != nil {
		return err
	}

	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("executing query: %w", err)
	}
	defer rows.Close()

	return ScanRows(ctx, rows, dest)
}

// ExecuteOrgFilteredQueryRow executes a single-row SELECT query with automatic org filtering
func (qf *QueryFilters) ExecuteOrgFilteredQueryRow(
	ctx context.Context,
	db DBInterface,
	sb squirrel.SelectBuilder,
) pgx.Row {
	sql, args, err := qf.WithOrgFilter(ctx, sb)
	if err != nil {
		// Return a row that will error when scanned
		return &errorRow{err: err}
	}

	return db.QueryRow(ctx, sql, args...)
}

// ExecuteOrgFilteredUpdate executes an UPDATE query with automatic org filtering
func (qf *QueryFilters) ExecuteOrgFilteredUpdate(
	ctx context.Context,
	db DBInterface,
	ub squirrel.UpdateBuilder,
) (pgconn.CommandTag, error) {
	filteredUB, err := qf.ApplyOrgFilterUpdate(ctx, ub)
	if err != nil {
		return pgconn.CommandTag{}, fmt.Errorf("applying org filter: %w", err)
	}

	sql, args, err := filteredUB.ToSql()
	if err != nil {
		return pgconn.CommandTag{}, fmt.Errorf("building update query: %w", err)
	}

	cmdTag, err := db.Exec(ctx, sql, args...)
	if err != nil {
		return pgconn.CommandTag{}, fmt.Errorf("executing update: %w", err)
	}

	return cmdTag, nil
}

// ExecuteOrgFilteredDelete executes a DELETE query with automatic org filtering
func (qf *QueryFilters) ExecuteOrgFilteredDelete(
	ctx context.Context,
	db DBInterface,
	delBuilder squirrel.DeleteBuilder,
) (pgconn.CommandTag, error) {
	filteredDB, err := qf.ApplyOrgFilterDelete(ctx, delBuilder)
	if err != nil {
		return pgconn.CommandTag{}, fmt.Errorf("applying org filter: %w", err)
	}

	sql, args, err := filteredDB.ToSql()
	if err != nil {
		return pgconn.CommandTag{}, fmt.Errorf("building delete query: %w", err)
	}

	cmdTag, err := db.Exec(ctx, sql, args...)
	if err != nil {
		return pgconn.CommandTag{}, fmt.Errorf("executing delete: %w", err)
	}

	return cmdTag, nil
}

// BuildOrgAwareInsert creates a new InsertBuilder with org_id column pre-configured
func (qf *QueryFilters) BuildOrgAwareInsert(
	ctx context.Context,
	table string,
) (squirrel.InsertBuilder, core.ID, error) {
	return qf.orgContext.BuildOrgAwareInsert(ctx, table)
}

// EnforceOrgIDInValues ensures org_id is included in the values for an insert operation
func (qf *QueryFilters) EnforceOrgIDInValues(
	ctx context.Context,
	columns []string,
	values []any,
) ([]string, []any, error) {
	return qf.orgContext.EnforceOrgIDInValues(ctx, columns, values)
}

// GetOrgID retrieves the organization ID from context
func (qf *QueryFilters) GetOrgID(ctx context.Context) (core.ID, error) {
	return qf.orgContext.GetOrganizationID(ctx)
}

// PreventCrossOrgAccess validates that no cross-organization access occurs
// Deprecated: For write paths, use MustGetOrganizationID instead to enforce fail-safe behavior.
// This function returns an error that could be ignored, creating a security vulnerability.
// Write methods should enforce organization ID from context, not validate input.
//
// SECURITY: This method now strictly validates empty and zero-value IDs to prevent bypass attempts.
func (qf *QueryFilters) PreventCrossOrgAccess(ctx context.Context, targetOrgID core.ID) error {
	contextOrgID, ok := GetOrganizationIDFromContext(ctx)
	if !ok {
		return core.NewError(nil, "MISSING_ORG_CONTEXT", map[string]any{
			"message": "organization context required but not found",
		})
	}
	// Validate that context organization ID is valid
	if !isValidOrganizationID(contextOrgID) {
		return core.NewError(nil, "INVALID_ORG_CONTEXT", map[string]any{
			"message": "organization ID in context cannot be empty",
			"org_id":  contextOrgID,
		})
	}
	// Validate that target organization ID is valid
	if !isValidOrganizationID(targetOrgID) {
		return core.NewError(nil, "INVALID_TARGET_ORG", map[string]any{
			"message": "target organization ID cannot be empty",
			"org_id":  targetOrgID,
		})
	}
	if contextOrgID != targetOrgID {
		return core.NewError(nil, "CROSS_ORG_ACCESS_DENIED", map[string]any{
			"context_org_id": contextOrgID,
			"target_org_id":  targetOrgID,
			"message":        "access to different organization data is not allowed",
		})
	}
	return nil
}

// errorRow implements pgx.Row to return scanning errors
type errorRow struct {
	err error
}

func (er *errorRow) Scan(_ ...any) error {
	return er.err
}
