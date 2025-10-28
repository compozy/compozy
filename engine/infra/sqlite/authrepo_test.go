package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	return logger.ContextWithLogger(t.Context(), logger.NewForTests())
}

func TestAuthRepo(t *testing.T) {
	t.Run("Should_create_user_successfully", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		user := &model.User{ID: core.MustNewID(), Email: "user@example.com", Role: model.RoleAdmin}
		require.NoError(t, repo.CreateUser(ctx, user))

		stored, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.Email, stored.Email)
		assert.Equal(t, user.Role, stored.Role)
		assert.False(t, stored.CreatedAt.IsZero())
	})

	t.Run("Should_get_user_by_id", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		created := createTestUser(t, repo, "lookup@example.com", model.RoleUser)
		found, err := repo.GetUserByID(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.Email, found.Email)
	})

	t.Run("Should_get_user_by_email_case_insensitive", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		createTestUser(t, repo, "Case@Example.com", model.RoleAdmin)
		user, err := repo.GetUserByEmail(ctx, "case@example.com")
		require.NoError(t, err)
		assert.Equal(t, "Case@Example.com", user.Email)
	})

	t.Run("Should_return_error_for_duplicate_email", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		createTestUser(t, repo, "dup@example.com", model.RoleUser)
		err := repo.CreateUser(ctx, &model.User{
			ID:    core.MustNewID(),
			Email: "Dup@example.com",
			Role:  model.RoleAdmin,
		})
		require.ErrorIs(t, err, uc.ErrEmailExists)
	})

	t.Run("Should_list_all_users", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		older := createTestUser(t, repo, "older@example.com", model.RoleUser)
		// Ensure distinct timestamps when the repository sets created_at at insert time.
		time.Sleep(20 * time.Millisecond)
		newer := createTestUser(t, repo, "newer@example.com", model.RoleAdmin)

		users, err := repo.ListUsers(ctx)
		require.NoError(t, err)
		require.Len(t, users, 2)
		assert.Equal(t, newer.Email, users[0].Email)
		assert.Equal(t, older.Email, users[1].Email)
	})

	t.Run("Should_update_user", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		user := createTestUser(t, repo, "update@example.com", model.RoleUser)
		user.Email = "updated@example.com"
		user.Role = model.RoleAdmin
		require.NoError(t, repo.UpdateUser(ctx, user))

		stored, err := repo.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "updated@example.com", stored.Email)
		assert.Equal(t, model.RoleAdmin, stored.Role)
	})

	t.Run("Should_delete_user", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		user := createTestUser(t, repo, "delete@example.com", model.RoleUser)
		require.NoError(t, repo.DeleteUser(ctx, user.ID))

		_, err := repo.GetUserByID(ctx, user.ID)
		assert.ErrorIs(t, err, uc.ErrUserNotFound)
	})

	t.Run("Should_create_api_key", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		user := createTestUser(t, repo, "keys@example.com", model.RoleUser)
		key := createTestAPIKey(t, repo, user.ID, "cpzy_prefix_1")

		stored, err := repo.GetAPIKeyByID(ctx, key.ID)
		require.NoError(t, err)
		assert.Equal(t, key.Prefix, stored.Prefix)
		assert.Equal(t, key.UserID, stored.UserID)
		assert.False(t, stored.CreatedAt.IsZero())
		assert.False(t, stored.LastUsed.Valid)
	})

	t.Run("Should_get_api_key_by_fingerprint", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		user := createTestUser(t, repo, "hash@example.com", model.RoleUser)
		key := createTestAPIKey(t, repo, user.ID, "cpzy_prefix_2")

		stored, err := repo.GetAPIKeyByFingerprint(ctx, key.Fingerprint)
		require.NoError(t, err)
		assert.Equal(t, key.ID, stored.ID)
	})

	t.Run("Should_update_api_key_last_used", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		user := createTestUser(t, repo, "lastused@example.com", model.RoleAdmin)
		key := createTestAPIKey(t, repo, user.ID, "cpzy_prefix_3")

		require.NoError(t, repo.UpdateAPIKeyLastUsed(ctx, key.ID))

		stored, err := repo.GetAPIKeyByID(ctx, key.ID)
		require.NoError(t, err)
		assert.True(t, stored.LastUsed.Valid)
		assert.WithinDuration(t, time.Now(), stored.LastUsed.Time, 2*time.Second)
	})

	t.Run("Should_delete_api_key", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		user := createTestUser(t, repo, "deletekey@example.com", model.RoleUser)
		key := createTestAPIKey(t, repo, user.ID, "cpzy_prefix_4")

		require.NoError(t, repo.DeleteAPIKey(ctx, key.ID))
		_, err := repo.GetAPIKeyByID(ctx, key.ID)
		assert.ErrorIs(t, err, uc.ErrAPIKeyNotFound)
	})

	t.Run("Should_cascade_delete_api_keys_when_user_deleted", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		user := createTestUser(t, repo, "cascade@example.com", model.RoleUser)
		key := createTestAPIKey(t, repo, user.ID, "cpzy_prefix_5")

		require.NoError(t, repo.DeleteUser(ctx, user.ID))
		_, err := repo.GetAPIKeyByID(ctx, key.ID)
		assert.ErrorIs(t, err, uc.ErrAPIKeyNotFound)
	})

	t.Run("Should_enforce_foreign_key_constraint", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		err := repo.CreateAPIKey(ctx, &model.APIKey{
			ID:          core.MustNewID(),
			UserID:      core.MustNewID(),
			Prefix:      "cpzy_prefix_6",
			Hash:        []byte("hash6"),
			Fingerprint: []byte("fp6"),
		})
		require.ErrorContains(t, err, "FOREIGN KEY")
	})

	t.Run("Should_handle_missing_user_gracefully", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		_, err := repo.GetUserByID(ctx, core.MustNewID())
		assert.ErrorIs(t, err, uc.ErrUserNotFound)
	})

	t.Run("Should_handle_missing_api_key_gracefully", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		_, err := repo.GetAPIKeyByID(ctx, core.MustNewID())
		assert.ErrorIs(t, err, uc.ErrAPIKeyNotFound)
	})

	t.Run("Should_reject_invalid_foreign_key", func(t *testing.T) {
		t.Parallel()
		repo := setupAuthRepo(t)
		ctx := testCtx(t)

		user := createTestUser(t, repo, "transient@example.com", model.RoleUser)
		require.NoError(t, repo.DeleteUser(ctx, user.ID))

		err := repo.CreateAPIKey(ctx, &model.APIKey{
			ID:          core.MustNewID(),
			UserID:      user.ID,
			Prefix:      "cpzy_prefix_7",
			Hash:        []byte("hash7"),
			Fingerprint: []byte("fp7"),
		})
		require.ErrorContains(t, err, "FOREIGN KEY")
	})

	t.Run("Should_handle_null_last_used_timestamp", func(t *testing.T) {
		repo := setupAuthRepo(t)
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())

		user := createTestUser(t, repo, "nulllastused@example.com", model.RoleAdmin)
		key := createTestAPIKey(t, repo, user.ID, "cpzy_prefix_8")

		stored, err := repo.GetAPIKeyByID(ctx, key.ID)
		require.NoError(t, err)
		assert.False(t, stored.LastUsed.Valid)
	})
}

func setupAuthRepo(t *testing.T) *AuthRepo {
	t.Helper()
	ctx := testCtx(t)
	dbPath := filepath.Join(t.TempDir(), "authrepo.db")

	require.NoError(t, ApplyMigrations(ctx, dbPath))
	store, err := NewStore(ctx, &Config{Path: dbPath})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close(ctx))
	})
	return NewAuthRepo(store.DB()).(*AuthRepo)
}

func createTestUser(t *testing.T, repo uc.Repository, email string, role model.Role) *model.User {
	t.Helper()
	ctx := testCtx(t)
	user := &model.User{
		ID:    core.MustNewID(),
		Email: email,
		Role:  role,
	}
	require.NoError(t, repo.CreateUser(ctx, user))
	return user
}

func createTestAPIKey(t *testing.T, repo uc.Repository, userID core.ID, prefix string) *model.APIKey {
	t.Helper()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	key := &model.APIKey{
		ID:          core.MustNewID(),
		UserID:      userID,
		Prefix:      prefix,
		Hash:        []byte(prefix + "_hash"),
		Fingerprint: []byte(prefix + "_fp"),
	}
	require.NoError(t, repo.CreateAPIKey(ctx, key))
	return key
}
