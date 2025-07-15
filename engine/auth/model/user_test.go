package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_Model(t *testing.T) {
	t.Run("Should validate role constants are properly defined", func(t *testing.T) {
		// Verify that role constants match expected string values
		// This is important for database storage and API contracts
		assert.Equal(t, Role("admin"), RoleAdmin)
		assert.Equal(t, Role("user"), RoleUser)
	})
}

func TestRole_Valid(t *testing.T) {
	t.Run("Should validate admin role", func(t *testing.T) {
		assert.True(t, RoleAdmin.Valid())
	})
	t.Run("Should validate user role", func(t *testing.T) {
		assert.True(t, RoleUser.Valid())
	})
	t.Run("Should reject invalid role", func(t *testing.T) {
		invalidRole := Role("superuser")
		assert.False(t, invalidRole.Valid())
	})
}
