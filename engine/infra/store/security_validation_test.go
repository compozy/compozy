package store

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityValidation_EmptyAndZeroValueIDs(t *testing.T) {
	t.Run("Should reject empty organization IDs", func(t *testing.T) {
		// Test PreventCrossOrgAccess with empty IDs
		qf := NewQueryFilters()

		// Empty string org ID in context
		ctx := WithOrganizationID(context.Background(), "")
		err := qf.PreventCrossOrgAccess(ctx, core.MustNewID())
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_ORG_CONTEXT", coreErr.Code)

		// Empty string target org ID
		ctx = WithOrganizationID(context.Background(), core.MustNewID())
		err = qf.PreventCrossOrgAccess(ctx, "")
		require.Error(t, err)
		coreErr, ok = err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_TARGET_ORG", coreErr.Code)
	})

	t.Run("Should accept zero-value UUID as valid system organization", func(t *testing.T) {
		qf := NewQueryFilters()
		systemID := core.ID(systemOrgID)

		// Zero-value UUID in context should be valid
		ctx := WithOrganizationID(context.Background(), systemID)
		err := qf.PreventCrossOrgAccess(ctx, systemID)
		require.NoError(t, err, "System ID should be valid for system organization")

		// Zero-value UUID as target with matching context should succeed
		ctx = WithOrganizationID(context.Background(), systemID)
		err = qf.PreventCrossOrgAccess(ctx, systemID)
		require.NoError(t, err)
	})

	t.Run("Should prevent cross-org access between system ID and other orgs", func(t *testing.T) {
		qf := NewQueryFilters()
		systemID := core.ID(systemOrgID)
		regularOrgID := core.MustNewID()

		// System ID context trying to access regular org
		ctx := WithOrganizationID(context.Background(), systemID)
		err := qf.PreventCrossOrgAccess(ctx, regularOrgID)
		require.Error(t, err)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "CROSS_ORG_ACCESS_DENIED", coreErr.Code)

		// Regular org context trying to access system ID org
		ctx = WithOrganizationID(context.Background(), regularOrgID)
		err = qf.PreventCrossOrgAccess(ctx, systemID)
		require.Error(t, err)
		coreErr, ok = err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "CROSS_ORG_ACCESS_DENIED", coreErr.Code)
	})

	t.Run("Should validate mixed empty and zero-value combinations", func(t *testing.T) {
		qf := NewQueryFilters()
		systemID := core.ID(systemOrgID)

		// Empty context, zero-value target
		ctx := WithOrganizationID(context.Background(), "")
		err := qf.PreventCrossOrgAccess(ctx, systemID)
		require.Error(t, err)
		// Should fail on context validation first
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_ORG_CONTEXT", coreErr.Code)

		// Zero-value context, empty target - should fail on empty target
		ctx = WithOrganizationID(context.Background(), systemID)
		err = qf.PreventCrossOrgAccess(ctx, "")
		require.Error(t, err)
		coreErr, ok = err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "INVALID_TARGET_ORG", coreErr.Code)
	})
}

func TestSecurityValidation_MustGetOrganizationID(t *testing.T) {
	t.Run("Should panic on missing organization context", func(t *testing.T) {
		ctx := context.Background()
		assert.Panics(t, func() {
			MustGetOrganizationID(ctx)
		}, "Should panic when organization ID is not in context")
	})

	t.Run("Should panic on empty organization ID", func(t *testing.T) {
		ctx := WithOrganizationID(context.Background(), "")
		assert.Panics(t, func() {
			MustGetOrganizationID(ctx)
		}, "Should panic when organization ID is empty")
	})

	t.Run("Should return zero-value UUID as valid system organization", func(t *testing.T) {
		systemID := core.ID(systemOrgID)
		ctx := WithOrganizationID(context.Background(), systemID)
		// Should not panic for system ID (system organization)
		orgID := MustGetOrganizationID(ctx)
		assert.Equal(t, systemID, orgID, "System ID should be returned as valid system organization")
	})

	t.Run("Should return valid organization ID", func(t *testing.T) {
		validOrgID := core.MustNewID()
		ctx := WithOrganizationID(context.Background(), validOrgID)

		// Should not panic
		orgID := MustGetOrganizationID(ctx)
		assert.Equal(t, validOrgID, orgID)
	})
}

func TestSecurityValidation_isValidOrganizationID(t *testing.T) {
	t.Run("Should validate organization IDs correctly", func(t *testing.T) {
		testCases := []struct {
			name  string
			id    core.ID
			valid bool
		}{
			{"empty string", "", false},
			{"system organization", systemOrgID, true}, // Valid system organization ID
			{"valid KSUID", core.MustNewID(), true},
			{"non-KSUID string", "not-a-uuid", false},                    // Invalid - not KSUID format or 'system'
			{"zero UUID", "00000000-0000-0000-0000-000000000000", false}, // No longer valid - not KSUID
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.valid, isValidOrganizationID(tc.id), "ID: %s", tc.id)
			})
		}
	})
}
