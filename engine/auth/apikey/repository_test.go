package apikey_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/apikey"
	"github.com/compozy/compozy/engine/core"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresRepository_Create(t *testing.T) {
	t.Run("Should create API key successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		expiresAt := time.Now().Add(30 * 24 * time.Hour)
		testKey := &apikey.APIKey{
			ID:        core.MustNewID(),
			OrgID:     core.MustNewID(),
			UserID:    core.MustNewID(),
			KeyPrefix: "cmpz_test",
			KeyHash:   "$2a$10$dummyhash",
			Name:      "Test Key",
			Status:    apikey.StatusActive,
			ExpiresAt: &expiresAt,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		mockPool.ExpectExec("INSERT INTO api_keys").
			WithArgs(
				testKey.ID,
				testKey.OrgID,
				testKey.UserID,
				testKey.KeyPrefix,
				testKey.KeyHash,
				testKey.Name,
				testKey.Status,
				testKey.ExpiresAt,
				testKey.RateLimitPerHour,
				testKey.CreatedAt,
				testKey.UpdatedAt,
			).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		err = repo.Create(ctx, testKey)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_GetByID(t *testing.T) {
	t.Run("Should get API key by ID successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		keyID := core.MustNewID()
		now := time.Now()
		expiresAt := now.Add(30 * 24 * time.Hour)
		var nilTime *time.Time
		rows := mockPool.NewRows([]string{"id", "org_id", "user_id", "key_prefix", "key_hash", "name", "status", "expires_at", "rate_limit_per_hour", "last_used_at", "created_at", "updated_at"}).
			AddRow(keyID, orgID, core.MustNewID(), "cmpz_test", "$2a$10$dummyhash", "Test Key", apikey.StatusActive, &expiresAt, 3600, nilTime, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM api_keys WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, keyID).
			WillReturnRows(rows)
		result, err := repo.GetByID(ctx, orgID, keyID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, keyID, result.ID)
		assert.Equal(t, orgID, result.OrgID)
		assert.Equal(t, "Test Key", result.Name)
		assert.Equal(t, apikey.StatusActive, result.Status)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should return error when API key not found", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		keyID := core.MustNewID()
		mockPool.ExpectQuery("SELECT (.+) FROM api_keys WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, keyID).
			WillReturnError(pgx.ErrNoRows)
		result, err := repo.GetByID(ctx, orgID, keyID)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, apikey.ErrAPIKeyNotFound))
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_GetByPrefix(t *testing.T) {
	t.Run("Should get API key by key_prefix successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		keyID := core.MustNewID()
		now := time.Now()
		var nilTime *time.Time
		rows := mockPool.NewRows([]string{"id", "org_id", "user_id", "key_prefix", "key_hash", "name", "status", "expires_at", "rate_limit_per_hour", "last_used_at", "created_at", "updated_at"}).
			AddRow(keyID, orgID, core.MustNewID(), "cmpz_test", "$2a$10$dummyhash", "Test Key", apikey.StatusActive, nilTime, 3600, nilTime, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM api_keys WHERE org_id = \\$1 AND key_prefix = \\$2").
			WithArgs(orgID, "cmpz_test").
			WillReturnRows(rows)
		result, err := repo.GetByPrefix(ctx, orgID, "cmpz_test")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "cmpz_test", result.KeyPrefix)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_Update(t *testing.T) {
	t.Run("Should update API key successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		lastUsed := time.Now()
		testKey := &apikey.APIKey{
			ID:         core.MustNewID(),
			OrgID:      core.MustNewID(),
			Name:       "Updated Key",
			Status:     apikey.StatusActive,
			ExpiresAt:  nil,
			LastUsedAt: &lastUsed,
			UpdatedAt:  time.Now(),
		}
		mockPool.ExpectExec("UPDATE api_keys").
			WithArgs(
				testKey.OrgID,
				testKey.ID,
				testKey.Name,
				testKey.Status,
				testKey.ExpiresAt,
				testKey.LastUsedAt,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		err = repo.Update(ctx, testKey)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should return error when API key not found", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		testKey := &apikey.APIKey{
			ID:        core.MustNewID(),
			OrgID:     core.MustNewID(),
			Name:      "Updated Key",
			Status:    apikey.StatusActive,
			UpdatedAt: time.Now(),
		}
		mockPool.ExpectExec("UPDATE api_keys").
			WithArgs(
				testKey.OrgID,
				testKey.ID,
				testKey.Name,
				testKey.Status,
				testKey.ExpiresAt,
				testKey.LastUsedAt,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 0))
		err = repo.Update(ctx, testKey)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, apikey.ErrAPIKeyNotFound))
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_Delete(t *testing.T) {
	t.Run("Should delete API key successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		keyID := core.MustNewID()
		mockPool.ExpectExec("DELETE FROM api_keys WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, keyID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))
		err = repo.Delete(ctx, orgID, keyID)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_List(t *testing.T) {
	t.Run("Should list API keys successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		now := time.Now()
		var nilTime *time.Time
		rows := mockPool.NewRows([]string{"id", "org_id", "user_id", "key_prefix", "key_hash", "name", "status", "expires_at", "rate_limit_per_hour", "last_used_at", "created_at", "updated_at"}).
			AddRow(core.MustNewID(), orgID, core.MustNewID(), "cmpz_key1", "$2a$10$hash1", "Key 1", apikey.StatusActive, nilTime, 3600, nilTime, now, now).
			AddRow(core.MustNewID(), orgID, core.MustNewID(), "cmpz_key2", "$2a$10$hash2", "Key 2", apikey.StatusActive, nilTime, 3600, nilTime, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM api_keys WHERE org_id = \\$1 ORDER BY created_at DESC LIMIT \\$2 OFFSET \\$3").
			WithArgs(orgID, 10, 0).
			WillReturnRows(rows)
		result, err := repo.List(ctx, orgID, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "Key 1", result[0].Name)
		assert.Equal(t, "Key 2", result[1].Name)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_ListByUser(t *testing.T) {
	t.Run("Should list API keys by user successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		userID := core.MustNewID()
		now := time.Now()
		var nilTime *time.Time
		rows := mockPool.NewRows([]string{"id", "org_id", "user_id", "key_prefix", "key_hash", "name", "status", "expires_at", "rate_limit_per_hour", "last_used_at", "created_at", "updated_at"}).
			AddRow(core.MustNewID(), orgID, userID, "cmpz_user1", "$2a$10$hash1", "User Key 1", apikey.StatusActive, nilTime, 3600, nilTime, now, now).
			AddRow(core.MustNewID(), orgID, userID, "cmpz_user2", "$2a$10$hash2", "User Key 2", apikey.StatusActive, nilTime, 3600, nilTime, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM api_keys WHERE org_id = \\$1 AND user_id = \\$2 ORDER BY created_at DESC LIMIT \\$3 OFFSET \\$4").
			WithArgs(orgID, userID, 10, 0).
			WillReturnRows(rows)
		result, err := repo.ListByUser(ctx, orgID, userID, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, userID, result[0].UserID)
		assert.Equal(t, userID, result[1].UserID)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_ListActive(t *testing.T) {
	t.Run("Should list active API keys successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		now := time.Now()
		var nilTime *time.Time
		rows := mockPool.NewRows([]string{"id", "org_id", "user_id", "key_prefix", "key_hash", "name", "status", "expires_at", "rate_limit_per_hour", "last_used_at", "created_at", "updated_at"}).
			AddRow(core.MustNewID(), orgID, core.MustNewID(), "cmpz_active1", "$2a$10$hash1", "Active Key 1", apikey.StatusActive, nilTime, 3600, nilTime, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM api_keys WHERE org_id = \\$1 AND status = \\$2 AND \\(expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP\\) ORDER BY created_at DESC LIMIT \\$3 OFFSET \\$4").
			WithArgs(orgID, apikey.StatusActive, 10, 0).
			WillReturnRows(rows)
		result, err := repo.ListActive(ctx, orgID, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, apikey.StatusActive, result[0].Status)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_UpdateStatus(t *testing.T) {
	t.Run("Should update API key status successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		keyID := core.MustNewID()
		mockPool.ExpectExec("UPDATE api_keys SET status = \\$3, updated_at = CURRENT_TIMESTAMP WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, keyID, apikey.StatusRevoked).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		err = repo.UpdateStatus(ctx, orgID, keyID, apikey.StatusRevoked)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_UpdateLastUsed(t *testing.T) {
	t.Run("Should update API key last used timestamp successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		keyID := core.MustNewID()
		lastUsed := time.Now()
		mockPool.ExpectExec("UPDATE api_keys SET last_used_at = \\$3, updated_at = CURRENT_TIMESTAMP WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, keyID, lastUsed).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		err = repo.UpdateLastUsed(ctx, orgID, keyID, lastUsed)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_RevokeExpired(t *testing.T) {
	t.Run("Should revoke expired API keys successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		mockPool.ExpectExec("UPDATE api_keys SET status = \\$2, updated_at = CURRENT_TIMESTAMP WHERE org_id = \\$1 AND status = \\$3 AND expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP").
			WithArgs(orgID, apikey.StatusExpired, apikey.StatusActive).
			WillReturnResult(pgxmock.NewResult("UPDATE", 3))
		count, err := repo.RevokeExpired(ctx, orgID)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), count)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_CountByOrg(t *testing.T) {
	t.Run("Should count API keys in organization successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		rows := mockPool.NewRows([]string{"count"}).AddRow(int64(15))
		mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM api_keys WHERE org_id = \\$1").
			WithArgs(orgID).
			WillReturnRows(rows)
		count, err := repo.CountByOrg(ctx, orgID)
		assert.NoError(t, err)
		assert.Equal(t, int64(15), count)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_CountByUser(t *testing.T) {
	t.Run("Should count API keys by user successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		userID := core.MustNewID()
		rows := mockPool.NewRows([]string{"count"}).AddRow(int64(5))
		mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM api_keys WHERE org_id = \\$1 AND user_id = \\$2").
			WithArgs(orgID, userID).
			WillReturnRows(rows)
		count, err := repo.CountByUser(ctx, orgID, userID)
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_FindByPrefix(t *testing.T) {
	t.Run("Should find API keys by key_prefix pattern successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		now := time.Now()
		var nilTime *time.Time
		rows := mockPool.NewRows([]string{"id", "org_id", "user_id", "key_prefix", "key_hash", "name", "status", "expires_at", "rate_limit_per_hour", "last_used_at", "created_at", "updated_at"}).
			AddRow(core.MustNewID(), orgID, core.MustNewID(), "cmpz_test1", "$2a$10$hash1", "Test Key 1", apikey.StatusActive, nilTime, 3600, nilTime, now, now).
			AddRow(core.MustNewID(), orgID, core.MustNewID(), "cmpz_test2", "$2a$10$hash2", "Test Key 2", apikey.StatusActive, nilTime, 3600, nilTime, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM api_keys WHERE org_id = \\$1 AND key_prefix ILIKE \\$2 ORDER BY key_prefix").
			WithArgs(orgID, "%test%").
			WillReturnRows(rows)
		result, err := repo.FindByPrefix(ctx, orgID, "test")
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "cmpz_test1", result[0].KeyPrefix)
		assert.Equal(t, "cmpz_test2", result[1].KeyPrefix)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_ValidateAndUpdateLastUsed(t *testing.T) {
	t.Run("Should atomically validate and update last used", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		keyID := core.MustNewID()
		mockPool.ExpectExec("UPDATE api_keys").
			WithArgs(orgID, keyID, apikey.StatusActive).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		err = repo.ValidateAndUpdateLastUsed(ctx, orgID, keyID)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should return error when key is inactive or expired", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		keyID := core.MustNewID()
		mockPool.ExpectExec("UPDATE api_keys").
			WithArgs(orgID, keyID, apikey.StatusActive).
			WillReturnResult(pgxmock.NewResult("UPDATE", 0))
		err = repo.ValidateAndUpdateLastUsed(ctx, orgID, keyID)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, apikey.ErrAPIKeyNotFound))
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
func TestPostgresRepository_WithTx(t *testing.T) {
	t.Run("Should return repository with transaction", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := apikey.NewPostgresRepository(mockPool)
		ctx := context.Background()
		mockPool.ExpectBegin()
		mockTx, err := mockPool.Begin(ctx)
		require.NoError(t, err)
		txRepo := repo.WithTx(mockTx)
		assert.NotNil(t, txRepo)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
