package org_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/org"
	"github.com/compozy/compozy/engine/core"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresRepository_Create(t *testing.T) {
	t.Run("Should create organization successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		organization := &org.Organization{
			ID:                core.MustNewID(),
			Name:              "Test Org",
			TemporalNamespace: "compozy-test-org",
			Status:            org.StatusActive,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		mockPool.ExpectExec("INSERT INTO organizations").
			WithArgs(
				organization.ID,
				organization.Name,
				organization.TemporalNamespace,
				organization.Status,
				organization.CreatedAt,
				organization.UpdatedAt,
			).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		err = repo.Create(ctx, organization)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_GetByID(t *testing.T) {
	t.Run("Should get organization by ID successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		now := time.Now()
		rows := mockPool.NewRows([]string{"id", "name", "temporal_namespace", "status", "created_at", "updated_at"}).
			AddRow(orgID, "Test Org", "compozy-test-org", org.StatusActive, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM organizations WHERE id = \\$1").
			WithArgs(orgID).
			WillReturnRows(rows)
		result, err := repo.GetByID(ctx, orgID)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, orgID, result.ID)
		assert.Equal(t, "Test Org", result.Name)
		assert.Equal(t, org.StatusActive, result.Status)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should return error when organization not found", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		mockPool.ExpectQuery("SELECT (.+) FROM organizations WHERE id = \\$1").
			WithArgs(orgID).
			WillReturnError(pgx.ErrNoRows)
		result, err := repo.GetByID(ctx, orgID)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, org.ErrOrganizationNotFound))
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_GetByName(t *testing.T) {
	t.Run("Should get organization by name successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		now := time.Now()
		rows := mockPool.NewRows([]string{"id", "name", "temporal_namespace", "status", "created_at", "updated_at"}).
			AddRow(orgID, "Test Org", "compozy-test-org", org.StatusActive, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM organizations WHERE name = \\$1").
			WithArgs("Test Org").
			WillReturnRows(rows)
		result, err := repo.GetByName(ctx, "Test Org")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, orgID, result.ID)
		assert.Equal(t, "Test Org", result.Name)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_Update(t *testing.T) {
	t.Run("Should update organization successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		organization := &org.Organization{
			ID:                core.MustNewID(),
			Name:              "Updated Org",
			TemporalNamespace: "compozy-updated-org",
			Status:            org.StatusActive,
			UpdatedAt:         time.Now(),
		}
		mockPool.ExpectExec("UPDATE organizations").
			WithArgs(
				organization.ID,
				organization.Name,
				organization.TemporalNamespace,
				organization.Status,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		err = repo.Update(ctx, organization)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should return error when organization not found", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		organization := &org.Organization{
			ID:                core.MustNewID(),
			Name:              "Updated Org",
			TemporalNamespace: "compozy-updated-org",
			Status:            org.StatusActive,
			UpdatedAt:         time.Now(),
		}
		mockPool.ExpectExec("UPDATE organizations").
			WithArgs(
				organization.ID,
				organization.Name,
				organization.TemporalNamespace,
				organization.Status,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 0))
		err = repo.Update(ctx, organization)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, org.ErrOrganizationNotFound))
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_Delete(t *testing.T) {
	t.Run("Should delete organization successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		mockPool.ExpectExec("DELETE FROM organizations WHERE id = \\$1").
			WithArgs(orgID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))
		err = repo.Delete(ctx, orgID)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_List(t *testing.T) {
	t.Run("Should list organizations successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		now := time.Now()
		rows := mockPool.NewRows([]string{"id", "name", "temporal_namespace", "status", "created_at", "updated_at"}).
			AddRow(core.MustNewID(), "Org 1", "compozy-org-1", org.StatusActive, now, now).
			AddRow(core.MustNewID(), "Org 2", "compozy-org-2", org.StatusActive, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM organizations ORDER BY created_at DESC LIMIT \\$1 OFFSET \\$2").
			WithArgs(10, 0).
			WillReturnRows(rows)
		result, err := repo.List(ctx, 10, 0)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "Org 1", result[0].Name)
		assert.Equal(t, "Org 2", result[1].Name)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_UpdateStatus(t *testing.T) {
	t.Run("Should update organization status successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		orgID := core.MustNewID()
		mockPool.ExpectExec("UPDATE organizations SET status = \\$2, updated_at = CURRENT_TIMESTAMP WHERE id = \\$1").
			WithArgs(orgID, org.StatusSuspended).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		err = repo.UpdateStatus(ctx, orgID, org.StatusSuspended)
		assert.NoError(t, err)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_FindByName(t *testing.T) {
	t.Run("Should find organizations by name pattern successfully", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		now := time.Now()
		rows := mockPool.NewRows([]string{"id", "name", "temporal_namespace", "status", "created_at", "updated_at"}).
			AddRow(core.MustNewID(), "Test Org 1", "compozy-test-org-1", org.StatusActive, now, now).
			AddRow(core.MustNewID(), "Test Org 2", "compozy-test-org-2", org.StatusActive, now, now)
		mockPool.ExpectQuery("SELECT (.+) FROM organizations WHERE name ILIKE \\$1 ORDER BY name").
			WithArgs("%Test%").
			WillReturnRows(rows)
		result, err := repo.FindByName(ctx, "Test")
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "Test Org 1", result[0].Name)
		assert.Equal(t, "Test Org 2", result[1].Name)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostgresRepository_WithTx(t *testing.T) {
	t.Run("Should return repository with transaction", func(t *testing.T) {
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		repo := org.NewPostgresRepository(mockPool)
		ctx := context.Background()
		mockPool.ExpectBegin()
		mockTx, err := mockPool.Begin(ctx)
		require.NoError(t, err)
		txRepo := repo.WithTx(mockTx)
		assert.NotNil(t, txRepo)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
