package store

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryFilters_PreventCrossOrgAccess(t *testing.T) {
	t.Run("Should return error when organization context is missing", func(t *testing.T) {
		qf := NewQueryFilters()
		ctx := context.Background()
		targetOrgID := core.MustNewID()
		err := qf.PreventCrossOrgAccess(ctx, targetOrgID)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "MISSING_ORG_CONTEXT", coreErr.Code)
		assert.Equal(t, "organization context required but not found", coreErr.Details["message"])
	})
	t.Run("Should return error when context organization ID is empty", func(t *testing.T) {
		qf := NewQueryFilters()
		ctx := WithOrganizationID(context.Background(), "")
		targetOrgID := core.MustNewID()
		err := qf.PreventCrossOrgAccess(ctx, targetOrgID)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_ORG_CONTEXT", coreErr.Code)
		assert.Equal(t, "organization ID in context cannot be empty", coreErr.Details["message"])
	})
	t.Run("Should return error when target organization ID is empty", func(t *testing.T) {
		qf := NewQueryFilters()
		contextOrgID := core.MustNewID()
		ctx := WithOrganizationID(context.Background(), contextOrgID)
		err := qf.PreventCrossOrgAccess(ctx, "")
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_TARGET_ORG", coreErr.Code)
		assert.Equal(t, "target organization ID cannot be empty", coreErr.Details["message"])
	})
	t.Run("Should return error when both IDs are empty", func(t *testing.T) {
		qf := NewQueryFilters()
		ctx := WithOrganizationID(context.Background(), "")
		err := qf.PreventCrossOrgAccess(ctx, "")
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		// Should fail on context org ID validation first
		assert.Equal(t, "INVALID_ORG_CONTEXT", coreErr.Code)
	})
	t.Run("Should return error when organization IDs do not match", func(t *testing.T) {
		qf := NewQueryFilters()
		contextOrgID := core.MustNewID()
		targetOrgID := core.MustNewID()
		ctx := WithOrganizationID(context.Background(), contextOrgID)
		err := qf.PreventCrossOrgAccess(ctx, targetOrgID)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "CROSS_ORG_ACCESS_DENIED", coreErr.Code)
		assert.Equal(t, "access to different organization data is not allowed", coreErr.Details["message"])
		assert.Equal(t, contextOrgID, coreErr.Details["context_org_id"])
		assert.Equal(t, targetOrgID, coreErr.Details["target_org_id"])
	})
	t.Run("Should return nil when organization IDs match and are valid", func(t *testing.T) {
		qf := NewQueryFilters()
		orgID := core.MustNewID()
		ctx := WithOrganizationID(context.Background(), orgID)
		err := qf.PreventCrossOrgAccess(ctx, orgID)
		require.NoError(t, err)
	})
	t.Run("Should accept system organization as valid system organization", func(t *testing.T) {
		qf := NewQueryFilters()
		systemOrgID := "system"
		// Test with system ID in context and target - should succeed
		ctx := WithOrganizationID(context.Background(), core.ID(systemOrgID))
		err := qf.PreventCrossOrgAccess(ctx, core.ID(systemOrgID))
		require.NoError(t, err, "System ID should be valid for system organization")
		// Test cross-org access between system and regular org - should fail
		regularOrgID := core.MustNewID()
		ctx = WithOrganizationID(context.Background(), core.ID(systemOrgID))
		err = qf.PreventCrossOrgAccess(ctx, regularOrgID)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "CROSS_ORG_ACCESS_DENIED", coreErr.Code)
	})
}
