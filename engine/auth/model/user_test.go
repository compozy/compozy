package model

import (
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
)

func TestUser_Model(t *testing.T) {
	t.Run("Should create user with admin role", func(t *testing.T) {
		userID := core.MustNewID()
		now := time.Now()

		user := &User{
			ID:        userID,
			Email:     "admin@example.com",
			Role:      RoleAdmin,
			CreatedAt: now,
		}

		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "admin@example.com", user.Email)
		assert.Equal(t, RoleAdmin, user.Role)
		assert.Equal(t, now, user.CreatedAt)
	})

	t.Run("Should create user with user role", func(t *testing.T) {
		userID := core.MustNewID()
		now := time.Now()

		user := &User{
			ID:        userID,
			Email:     "user@example.com",
			Role:      RoleUser,
			CreatedAt: now,
		}

		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "user@example.com", user.Email)
		assert.Equal(t, RoleUser, user.Role)
		assert.Equal(t, now, user.CreatedAt)
	})

	t.Run("Should validate role constants", func(t *testing.T) {
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
