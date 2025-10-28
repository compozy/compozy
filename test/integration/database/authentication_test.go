package database_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/repo"
	"github.com/compozy/compozy/test/helpers"
)

func TestMultiDriver_Authentication(t *testing.T) {
	forEachDriver(t, "Authentication", func(t *testing.T, _ string, provider *repo.Provider) {
		authRepo := provider.NewAuthRepo()

		t.Run("Should create and retrieve users", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			user := &model.User{
				ID:    core.MustNewID(),
				Email: fmt.Sprintf("user-%s@example.com", core.MustNewID()),
				Role:  model.RoleAdmin,
			}
			require.NoError(t, authRepo.CreateUser(ctx, user))

			byID, err := authRepo.GetUserByID(ctx, user.ID)
			require.NoError(t, err)
			assert.Equal(t, user.Email, byID.Email)

			byEmail, err := authRepo.GetUserByEmail(ctx, user.Email)
			require.NoError(t, err)
			assert.Equal(t, user.ID, byEmail.ID)

			users, err := authRepo.ListUsers(ctx)
			require.NoError(t, err)
			assert.NotEmpty(t, users)
		})

		t.Run("Should authenticate with api key", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			user := createTestUser(ctx, t, authRepo)
			key := makeAPIKey(user.ID)
			require.NoError(t, authRepo.CreateAPIKey(ctx, key))

			retrieved, err := authRepo.GetAPIKeyByID(ctx, key.ID)
			require.NoError(t, err)
			assert.Equal(t, key.UserID, retrieved.UserID)

			fingerprint := sha256.Sum256(key.Hash)
			byFingerprint, err := authRepo.GetAPIKeyByFingerprint(ctx, fingerprint[:])
			require.NoError(t, err)
			assert.Equal(t, key.ID, byFingerprint.ID)

			require.NoError(t, authRepo.UpdateAPIKeyLastUsed(ctx, key.ID))
			updated, err := authRepo.GetAPIKeyByID(ctx, key.ID)
			require.NoError(t, err)
			assert.True(t, updated.LastUsed.Valid)
		})

		t.Run("Should cascade delete api keys", func(t *testing.T) {
			ctx := helpers.NewTestContext(t)
			user := createTestUser(ctx, t, authRepo)
			key := makeAPIKey(user.ID)
			require.NoError(t, authRepo.CreateAPIKey(ctx, key))

			require.NoError(t, authRepo.DeleteUser(ctx, user.ID))

			_, err := authRepo.GetUserByID(ctx, user.ID)
			require.ErrorIs(t, err, uc.ErrUserNotFound)

			_, err = authRepo.GetAPIKeyByID(ctx, key.ID)
			require.ErrorIs(t, err, uc.ErrAPIKeyNotFound)
		})
	})
}

func createTestUser(ctx context.Context, t *testing.T, repo uc.Repository) *model.User {
	t.Helper()
	user := &model.User{
		ID:    core.MustNewID(),
		Email: fmt.Sprintf("user-%s@example.com", core.MustNewID()),
		Role:  model.RoleUser,
	}
	require.NoError(t, repo.CreateUser(ctx, user))
	return user
}

func makeAPIKey(userID core.ID) *model.APIKey {
	randomID := core.MustNewID().String()
	hash := []byte("hash-" + randomID)
	fp := sha256.Sum256(hash)
	return &model.APIKey{
		ID:          core.MustNewID(),
		UserID:      userID,
		Hash:        hash,
		Fingerprint: fp[:],
		Prefix:      "cpzy_" + core.MustNewID().String()[:8],
	}
}
