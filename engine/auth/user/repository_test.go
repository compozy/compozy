package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresRepository_Create(t *testing.T) {
	t.Run("Should create user successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		testUser := &user.User{
			ID:        core.MustNewID(),
			OrgID:     core.MustNewID(),
			Email:     "test@example.com",
			Role:      user.RoleOrgAdmin,
			Status:    user.StatusActive,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		mockPool.ExpectExec("INSERT INTO users").
			WithArgs(
				testUser.ID,
				testUser.OrgID,
				testUser.Email,
				testUser.Role,
				testUser.Status,
				testUser.CreatedAt,
				testUser.UpdatedAt,
			).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		err = repo.Create(ctx, testUser)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_GetByID(t *testing.T) {
	t.Run("Should get user by ID successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		userID := core.MustNewID()
		now := time.Now()
		rows := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(userID, orgID, "test@example.com", user.RoleOrgAdmin, user.StatusActive, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID).
			WillReturnRows(rows)
		result, err := repo.GetByID(ctx, orgID, userID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, userID, result.ID)
		assert.Equal(t, orgID, result.OrgID)
		assert.Equal(t, "test@example.com", result.Email)
		assert.Equal(t, user.RoleOrgAdmin, result.Role)
		assert.Equal(t, user.StatusActive, result.Status)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should return error when user not found", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		userID := core.MustNewID()
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID).
			WillReturnError(pgx.ErrNoRows)
		result, err := repo.GetByID(ctx, orgID, userID)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, user.ErrUserNotFound))
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_GetByEmail(t *testing.T) {
	t.Run("Should get user by email successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		userID := core.MustNewID()
		now := time.Now()
		rows := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(userID, orgID, "test@example.com", user.RoleOrgManager, user.StatusActive, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND email = \\$2").
			WithArgs(orgID, "test@example.com").
			WillReturnRows(rows)
		result, err := repo.GetByEmail(ctx, orgID, "test@example.com")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, userID, result.ID)
		assert.Equal(t, "test@example.com", result.Email)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_Update(t *testing.T) {
	t.Run("Should update user successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		testUser := &user.User{
			ID:        core.MustNewID(),
			OrgID:     core.MustNewID(),
			Email:     "updated@example.com",
			Role:      user.RoleOrgManager,
			Status:    user.StatusActive,
			UpdatedAt: time.Now(),
		}
		mockPool.ExpectExec("UPDATE users").
			WithArgs(
				testUser.OrgID,
				testUser.ID,
				testUser.Email,
				testUser.Role,
				testUser.Status,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		err = repo.Update(ctx, testUser)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should return error when user not found", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		testUser := &user.User{
			ID:        core.MustNewID(),
			OrgID:     core.MustNewID(),
			Email:     "updated@example.com",
			Role:      user.RoleOrgManager,
			Status:    user.StatusActive,
			UpdatedAt: time.Now(),
		}
		mockPool.ExpectExec("UPDATE users").
			WithArgs(
				testUser.OrgID,
				testUser.ID,
				testUser.Email,
				testUser.Role,
				testUser.Status,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 0))
		err = repo.Update(ctx, testUser)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, user.ErrUserNotFound))
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_Delete(t *testing.T) {
	t.Run("Should delete user successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		userID := core.MustNewID()
		mockPool.ExpectExec("DELETE FROM users WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))
		err = repo.Delete(ctx, orgID, userID)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_List(t *testing.T) {
	t.Run("Should list users successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		now := time.Now()
		rows := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(core.MustNewID(), orgID, "user1@example.com", user.RoleOrgAdmin, user.StatusActive, now, now).
			AddRow(core.MustNewID(), orgID, "user2@example.com", user.RoleOrgManager, user.StatusActive, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 ORDER BY created_at DESC LIMIT \\$2 OFFSET \\$3").
			WithArgs(orgID, 10, 0).
			WillReturnRows(rows)
		result, err := repo.List(ctx, orgID, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "user1@example.com", result[0].Email)
		assert.Equal(t, "user2@example.com", result[1].Email)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_ListByRole(t *testing.T) {
	t.Run("Should list users by role successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		now := time.Now()
		rows := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(core.MustNewID(), orgID, "admin1@example.com", user.RoleOrgAdmin, user.StatusActive, now, now).
			AddRow(core.MustNewID(), orgID, "admin2@example.com", user.RoleOrgAdmin, user.StatusActive, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND role = \\$2 ORDER BY created_at DESC LIMIT \\$3 OFFSET \\$4").
			WithArgs(orgID, string(user.RoleOrgAdmin), 10, 0).
			WillReturnRows(rows)
		result, err := repo.ListByRole(ctx, orgID, string(user.RoleOrgAdmin), 10, 0)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, user.RoleOrgAdmin, result[0].Role)
		assert.Equal(t, user.RoleOrgAdmin, result[1].Role)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_UpdateRole(t *testing.T) {
	t.Run("Should update user role successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		userID := core.MustNewID()
		mockPool.ExpectExec("UPDATE users SET role = \\$3, updated_at = CURRENT_TIMESTAMP WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID, string(user.RoleOrgManager)).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		err = repo.UpdateRole(ctx, orgID, userID, string(user.RoleOrgManager))
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_UpdateStatus(t *testing.T) {
	t.Run("Should update user status successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		userID := core.MustNewID()
		mockPool.ExpectExec("UPDATE users SET status = \\$3, updated_at = CURRENT_TIMESTAMP WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID, user.StatusSuspended).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		err = repo.UpdateStatus(ctx, orgID, userID, user.StatusSuspended)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_FindByEmail(t *testing.T) {
	t.Run("Should find users by email pattern successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		now := time.Now()
		rows := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(core.MustNewID(), orgID, "test1@example.com", user.RoleOrgManager, user.StatusActive, now, now).
			AddRow(core.MustNewID(), orgID, "test2@example.com", user.RoleOrgCustomer, user.StatusActive, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND email ILIKE \\$2 ORDER BY email").
			WithArgs(orgID, "%test%").
			WillReturnRows(rows)
		result, err := repo.FindByEmail(ctx, orgID, "test")
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "test1@example.com", result[0].Email)
		assert.Equal(t, "test2@example.com", result[1].Email)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_CountByOrg(t *testing.T) {
	t.Run("Should count users in organization successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		rows := mockPool.NewRows([]string{"count"}).AddRow(int64(42))
		mockPool.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users WHERE org_id = \\$1").
			WithArgs(orgID).
			WillReturnRows(rows)
		count, err := repo.CountByOrg(ctx, orgID)
		assert.NoError(t, err)
		assert.Equal(t, int64(42), count)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_WithTx(t *testing.T) {
	t.Run("Should return repository with transaction", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := user.NewPostgresRepository(mockPool)
		ctx := context.Background()
		mockPool.ExpectBegin()
		mockTx, err := mockPool.Begin(ctx)
		require.NoError(t, err)
		txRepo := repo.WithTx(mockTx)
		assert.NotNil(t, txRepo)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
