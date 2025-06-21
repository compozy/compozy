package store

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
)

func TestMustGetOrganizationID(t *testing.T) {
	t.Run("Should panic when organization ID is not in context", func(t *testing.T) {
		ctx := context.Background()

		assert.Panics(t, func() {
			MustGetOrganizationID(ctx)
		}, "Should panic when org ID is missing")
	})

	t.Run("Should panic when organization ID is empty", func(t *testing.T) {
		ctx := WithOrganizationID(context.Background(), core.ID(""))

		assert.Panics(t, func() {
			MustGetOrganizationID(ctx)
		}, "Should panic when org ID is empty")
	})

	t.Run("Should return system ID as valid system organization", func(t *testing.T) {
		systemOrgID := core.ID("system")
		ctx := WithOrganizationID(context.Background(), systemOrgID)

		// Should not panic for system ID (system organization)
		orgID := MustGetOrganizationID(ctx)
		assert.Equal(t, systemOrgID, orgID, "System ID should be returned as valid system organization")
	})

	t.Run("Should panic for zero UUID as it's no longer valid", func(t *testing.T) {
		zeroUUID := core.ID("00000000-0000-0000-0000-000000000000")
		ctx := WithOrganizationID(context.Background(), zeroUUID)

		assert.Panics(t, func() {
			MustGetOrganizationID(ctx)
		}, "Should panic for zero UUID as it's not a valid KSUID or 'system'")
	})

	t.Run("Should return valid organization ID", func(t *testing.T) {
		validOrgID, _ := core.NewID()
		ctx := WithOrganizationID(context.Background(), validOrgID)

		orgID := MustGetOrganizationID(ctx)
		assert.Equal(t, validOrgID, orgID)
	})
}
