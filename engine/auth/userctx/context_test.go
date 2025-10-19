package userctx

import (
	"testing"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithUser(t *testing.T) {
	t.Run("Should add user to context", func(t *testing.T) {
		ctx := t.Context()
		userID, _ := core.NewID()
		user := &model.User{
			ID:    userID,
			Email: "test@example.com",
			Role:  model.RoleUser,
		}

		newCtx := WithUser(ctx, user)

		// Verify context is not the same instance
		assert.NotEqual(t, ctx, newCtx)

		// Verify user can be retrieved
		retrievedUser, ok := UserFromContext(newCtx)
		assert.True(t, ok)
		assert.Equal(t, user, retrievedUser)
	})
}

func TestUserFromContext(t *testing.T) {
	t.Run("Should return user when present", func(t *testing.T) {
		userID, _ := core.NewID()
		user := &model.User{
			ID:    userID,
			Email: "test@example.com",
			Role:  model.RoleAdmin,
		}
		ctx := WithUser(t.Context(), user)

		retrievedUser, ok := UserFromContext(ctx)

		assert.True(t, ok)
		assert.Equal(t, user.ID, retrievedUser.ID)
		assert.Equal(t, user.Email, retrievedUser.Email)
		assert.Equal(t, user.Role, retrievedUser.Role)
	})

	t.Run("Should return false when user not present", func(t *testing.T) {
		ctx := t.Context()

		user, ok := UserFromContext(ctx)

		assert.False(t, ok)
		assert.Nil(t, user)
	})
}

func TestMustUserFromContext(t *testing.T) {
	t.Run("Should return user when present", func(t *testing.T) {
		userID, _ := core.NewID()
		user := &model.User{
			ID:    userID,
			Email: "test@example.com",
			Role:  model.RoleUser,
		}
		ctx := WithUser(t.Context(), user)

		retrievedUser, err := MustUserFromContext(ctx)
		assert.NoError(t, err)

		assert.Equal(t, user, retrievedUser)
	})

	t.Run("Should panic when user not present", func(t *testing.T) {
		ctx := t.Context()

		_, err := MustUserFromContext(ctx)
		assert.Error(t, err)
		assert.Equal(t, "user not found in context", err.Error())
	})

	t.Run("Should panic with correct message", func(t *testing.T) {
		ctx := t.Context()
		require.NotPanics(t, func() {
			_, err := MustUserFromContext(ctx)
			require.Error(t, err)
			assert.Equal(t, "user not found in context", err.Error())
		})
	})
}
