package store

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/compozy/compozy/engine/core"
)

// organizationIDKey is the context key for organization ID
type organizationIDKey struct{}

// WithOrganizationID adds organization ID to context
func WithOrganizationID(ctx context.Context, orgID core.ID) context.Context {
	return context.WithValue(ctx, organizationIDKey{}, orgID)
}

// GetOrganizationIDFromContext retrieves organization ID from context
func GetOrganizationIDFromContext(ctx context.Context) (core.ID, bool) {
	orgID, ok := ctx.Value(organizationIDKey{}).(core.ID)
	return orgID, ok
}

// OrganizationContext provides organization-scoped query filtering capabilities
type OrganizationContext struct{}

// NewOrganizationContext creates a new organization context helper
func NewOrganizationContext() *OrganizationContext {
	return &OrganizationContext{}
}

// FilterByOrganization adds org_id filtering to a squirrel SelectBuilder
func (oc *OrganizationContext) FilterByOrganization(
	ctx context.Context,
	sb squirrel.SelectBuilder,
) (squirrel.SelectBuilder, error) {
	orgID, ok := GetOrganizationIDFromContext(ctx)
	if !ok {
		return sb, fmt.Errorf("organization ID not found in context")
	}
	return sb.Where(squirrel.Eq{"org_id": orgID}), nil
}

// BuildOrgAwareInsert creates a new InsertBuilder with org_id column pre-configured
// This ensures all insert operations include the organization ID from the context
func (oc *OrganizationContext) BuildOrgAwareInsert(
	ctx context.Context,
	table string,
) (squirrel.InsertBuilder, core.ID, error) {
	orgID, ok := GetOrganizationIDFromContext(ctx)
	if !ok {
		return squirrel.InsertBuilder{}, "", fmt.Errorf("organization ID not found in context")
	}
	// Create a new InsertBuilder with org_id column already included
	ib := squirrel.Insert(table).Columns("org_id").PlaceholderFormat(squirrel.Dollar)
	return ib, orgID, nil
}

// EnforceOrgIDInValues ensures org_id is included in the values for an insert operation
// This method should be used when building insert queries to guarantee org_id is always present
func (oc *OrganizationContext) EnforceOrgIDInValues(
	ctx context.Context,
	columns []string,
	values []any,
) ([]string, []any, error) {
	orgID, ok := GetOrganizationIDFromContext(ctx)
	if !ok {
		return nil, nil, fmt.Errorf("organization ID not found in context")
	}
	// Check if org_id is already in columns
	hasOrgID := false
	for _, col := range columns {
		if col == "org_id" {
			hasOrgID = true
			break
		}
	}
	if hasOrgID {
		return columns, values, nil
	}
	// Add org_id to columns and values
	newColumns := make([]string, len(columns)+1)
	copy(newColumns, columns)
	newColumns[len(columns)] = "org_id"
	newValues := make([]any, len(values)+1)
	copy(newValues, values)
	newValues[len(values)] = orgID
	return newColumns, newValues, nil
}

// FilterUpdateByOrganization adds org_id filtering to a squirrel UpdateBuilder
func (oc *OrganizationContext) FilterUpdateByOrganization(
	ctx context.Context,
	ub squirrel.UpdateBuilder,
) (squirrel.UpdateBuilder, error) {
	orgID, ok := GetOrganizationIDFromContext(ctx)
	if !ok {
		return ub, fmt.Errorf("organization ID not found in context")
	}
	return ub.Where(squirrel.Eq{"org_id": orgID}), nil
}

// FilterDeleteByOrganization adds org_id filtering to a squirrel DeleteBuilder
func (oc *OrganizationContext) FilterDeleteByOrganization(
	ctx context.Context,
	db squirrel.DeleteBuilder,
) (squirrel.DeleteBuilder, error) {
	orgID, ok := GetOrganizationIDFromContext(ctx)
	if !ok {
		return db, fmt.Errorf("organization ID not found in context")
	}
	return db.Where(squirrel.Eq{"org_id": orgID}), nil
}

// GetOrganizationID retrieves the organization ID from context
func (oc *OrganizationContext) GetOrganizationID(ctx context.Context) (core.ID, error) {
	orgID, ok := GetOrganizationIDFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("organization ID not found in context")
	}
	return orgID, nil
}

// EstablishOrgContextFromWorkflow establishes organization context from a workflow execution ID.
// This is a secure internal helper that should only be used by trusted activity workers.
//
// SECURITY NOTE: This function uses the private getOrganizationID method which bypasses
// tenant isolation. It should ONLY be called from trusted internal services (e.g., activity
// workers) that need to establish context from execution IDs. The execution ID must come
// from a trusted source, never from user input.
func EstablishOrgContextFromWorkflow(
	ctx context.Context,
	repo *WorkflowRepo,
	workflowExecID core.ID,
) (context.Context, error) {
	orgID, err := repo.getOrganizationID(ctx, workflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization ID from workflow: %w", err)
	}
	return WithOrganizationID(ctx, orgID), nil
}

// EstablishOrgContextFromTask establishes organization context from a task execution ID.
// This is a secure internal helper that should only be used by trusted activity workers.
//
// SECURITY NOTE: This function uses the private getOrganizationID method which bypasses
// tenant isolation. It should ONLY be called from trusted internal services (e.g., activity
// workers) that need to establish context from execution IDs. The execution ID must come
// from a trusted source, never from user input.
func EstablishOrgContextFromTask(ctx context.Context, repo *TaskRepo, taskExecID core.ID) (context.Context, error) {
	orgID, err := repo.getOrganizationID(ctx, taskExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization ID from task: %w", err)
	}
	return WithOrganizationID(ctx, orgID), nil
}

// systemOrgID is the system organization ID constant
const systemOrgID = "system"

// isValidOrganizationID validates that an organization ID is either:
// 1. The system organization ID 'system'
// 2. A valid KSUID (27 characters, base62 encoded)
func isValidOrganizationID(id core.ID) bool {
	if id == "" {
		return false
	}
	idStr := string(id)
	// Check if it's the system organization ID
	if idStr == systemOrgID {
		return true
	}
	// Check if it's a KSUID (27 characters, base62 encoded)
	if len(idStr) == 27 {
		// Simple check for KSUID format - all base62 characters
		for _, c := range idStr {
			if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
				return false
			}
		}
		return true
	}
	return false
}

// MustGetOrganizationID retrieves the organization ID from context or panics.
// This should be used ONLY in write paths where a missing organization context
// represents a critical programming error that must be caught during development.
//
// SECURITY: This function enforces fail-safe behavior for multi-tenant data isolation.
// A missing organization context on write paths could lead to cross-tenant data corruption.
func MustGetOrganizationID(ctx context.Context) core.ID {
	orgID, ok := GetOrganizationIDFromContext(ctx)
	if !ok {
		panic("CRITICAL: organization ID not found in context - this is a programming error that must be fixed")
	}
	if !isValidOrganizationID(orgID) {
		panic(
			fmt.Sprintf(
				"CRITICAL: invalid organization ID format in context: %s - must be a valid KSUID or 'system'",
				orgID,
			),
		)
	}
	// Note: The 'system' ID is valid as it's used for the system organization
	return orgID
}
