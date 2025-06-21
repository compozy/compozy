package store_test

import (
	"context"
	"testing"

	"github.com/Masterminds/squirrel"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationContext(t *testing.T) {
	orgID := core.ID("test-org-id")
	ctx := store.WithOrganizationID(context.Background(), orgID)
	oc := store.NewOrganizationContext()

	t.Run("Should add org_id to SelectBuilder", func(t *testing.T) {
		sb := squirrel.Select("*").From("users")

		filteredSB, err := oc.FilterByOrganization(ctx, sb)
		require.NoError(t, err)

		sql, args, err := filteredSB.ToSql()
		require.NoError(t, err)

		assert.Contains(t, sql, "org_id = ?")
		assert.Contains(t, args, orgID)
	})

	t.Run("Should add org_id to UpdateBuilder", func(t *testing.T) {
		ub := squirrel.Update("users").Set("name", "John")

		filteredUB, err := oc.FilterUpdateByOrganization(ctx, ub)
		require.NoError(t, err)

		sql, args, err := filteredUB.ToSql()
		require.NoError(t, err)

		assert.Contains(t, sql, "org_id = ?")
		assert.Contains(t, args, orgID)
	})

	t.Run("Should add org_id to DeleteBuilder", func(t *testing.T) {
		db := squirrel.Delete("users")

		filteredDB, err := oc.FilterDeleteByOrganization(ctx, db)
		require.NoError(t, err)

		sql, args, err := filteredDB.ToSql()
		require.NoError(t, err)

		assert.Contains(t, sql, "org_id = ?")
		assert.Contains(t, args, orgID)
	})

	t.Run("Should create InsertBuilder with org_id", func(t *testing.T) {
		ib, returnedOrgID, err := oc.BuildOrgAwareInsert(ctx, "users")
		require.NoError(t, err)
		assert.Equal(t, orgID, returnedOrgID)

		// Add other columns and values
		ib = ib.Columns("name", "email").Values(orgID, "John", "john@example.com")

		sql, args, err := ib.ToSql()
		require.NoError(t, err)

		// Verify org_id is included in the INSERT statement
		assert.Contains(t, sql, "org_id")
		assert.Contains(t, sql, "name")
		assert.Contains(t, sql, "email")
		assert.Contains(t, args, orgID)
		assert.Contains(t, args, "John")
		assert.Contains(t, args, "john@example.com")
	})

	t.Run("Should enforce org_id in values", func(t *testing.T) {
		columns := []string{"name", "email"}
		values := []any{"John", "john@example.com"}

		newColumns, newValues, err := oc.EnforceOrgIDInValues(ctx, columns, values)
		require.NoError(t, err)

		assert.Contains(t, newColumns, "org_id")
		assert.Contains(t, newValues, orgID)
		assert.Len(t, newColumns, 3)
		assert.Len(t, newValues, 3)
	})

	t.Run("Should error when org_id is not in context", func(t *testing.T) {
		emptyCtx := context.Background()

		// Test Select
		sb := squirrel.Select("*").From("users")
		_, err := oc.FilterByOrganization(emptyCtx, sb)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organization ID not found in context")

		// Test Update
		ub := squirrel.Update("users").Set("name", "John")
		_, err = oc.FilterUpdateByOrganization(emptyCtx, ub)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organization ID not found in context")

		// Test Delete
		db := squirrel.Delete("users")
		_, err = oc.FilterDeleteByOrganization(emptyCtx, db)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organization ID not found in context")

		// Test BuildOrgAwareInsert
		_, _, err = oc.BuildOrgAwareInsert(emptyCtx, "users")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organization ID not found in context")

		// Test EnforceOrgIDInValues
		_, _, err = oc.EnforceOrgIDInValues(emptyCtx, []string{"name"}, []any{"John"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "organization ID not found in context")
	})

	t.Run("Should not duplicate org_id if already present", func(t *testing.T) {
		columns := []string{"name", "email", "org_id"}
		values := []any{"John", "john@example.com", orgID}

		newColumns, newValues, err := oc.EnforceOrgIDInValues(ctx, columns, values)
		require.NoError(t, err)

		// Should return the same columns and values
		assert.Equal(t, columns, newColumns)
		assert.Equal(t, values, newValues)
	})
}

func TestGetOrganizationIDFromContext(t *testing.T) {
	t.Run("Should retrieve org_id from context", func(t *testing.T) {
		orgID := core.ID("test-org-id")
		ctx := store.WithOrganizationID(context.Background(), orgID)

		retrievedOrgID, ok := store.GetOrganizationIDFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, orgID, retrievedOrgID)
	})

	t.Run("Should return false when org_id is not in context", func(t *testing.T) {
		ctx := context.Background()

		_, ok := store.GetOrganizationIDFromContext(ctx)
		assert.False(t, ok)
	})
}
