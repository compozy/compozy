package user_test

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDBInterface is a mock implementation of store.DBInterface
type MockDBInterface struct {
	mockPool pgxmock.PgxPoolIface
}

func (m *MockDBInterface) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return m.mockPool.Exec(ctx, sql, arguments...)
}

func (m *MockDBInterface) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.mockPool.Query(ctx, sql, args...)
}

func (m *MockDBInterface) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return m.mockPool.QueryRow(ctx, sql, args...)
}

func (m *MockDBInterface) Begin(ctx context.Context) (pgx.Tx, error) {
	return m.mockPool.Begin(ctx)
}

func TestUserService_CreateUser(t *testing.T) {
	t.Run("Should create user successfully", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		mockDB := &MockDBInterface{mockPool: mockPool}
		// Create repository
		repo := user.NewPostgresRepository(mockDB)
		// Create service
		service := user.NewService(repo, mockDB)
		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		// Test data
		orgID := core.MustNewID()
		request := &user.CreateUserRequest{
			Email: "test@example.com",
			Role:  user.RoleOrgCustomer,
		}
		// Mock repository calls
		// 1. Check for existing user by email (should not exist)
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND email = \\$2").
			WithArgs(orgID, "test@example.com").
			WillReturnError(user.ErrUserNotFound)
		// 2. Create user
		mockPool.ExpectExec("INSERT INTO users").
			WithArgs(
				pgxmock.AnyArg(),     // ID
				orgID,                // OrgID
				"test@example.com",   // Email
				user.RoleOrgCustomer, // Role
				user.StatusActive,    // Status
				pgxmock.AnyArg(),     // CreatedAt
				pgxmock.AnyArg(),     // UpdatedAt
			).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		// Execute
		result, err := service.CreateUser(ctx, orgID, request)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "test@example.com", result.Email)
		assert.Equal(t, user.RoleOrgCustomer, result.Role)
		assert.Equal(t, user.StatusActive, result.Status)
		assert.Equal(t, orgID, result.OrgID)
		assert.NotEmpty(t, result.ID)
		// Verify all expectations were met
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should reject duplicate email in same organization", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		mockDB := &MockDBInterface{mockPool: mockPool}
		// Create repository
		repo := user.NewPostgresRepository(mockDB)
		// Create service
		service := user.NewService(repo, mockDB)
		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		// Test data
		orgID := core.MustNewID()
		request := &user.CreateUserRequest{
			Email: "existing@example.com",
			Role:  user.RoleOrgCustomer,
		}
		// Mock existing user
		existingUser := &user.User{
			ID:    core.MustNewID(),
			OrgID: orgID,
			Email: "existing@example.com",
			Role:  user.RoleOrgAdmin,
		}
		// Mock repository call to find existing user
		rows := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(existingUser.ID, existingUser.OrgID, existingUser.Email, existingUser.Role, user.StatusActive, time.Now(), time.Now())
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND email = \\$2").
			WithArgs(orgID, "existing@example.com").
			WillReturnRows(rows)
		// Execute
		result, err := service.CreateUser(ctx, orgID, request)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "user with email 'existing@example.com' already exists in organization")
		// Verify all expectations were met
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should validate email format", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		mockDB := &MockDBInterface{mockPool: mockPool}
		// Create repository
		repo := user.NewPostgresRepository(mockDB)
		// Create service
		service := user.NewService(repo, mockDB)
		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		// Test data
		orgID := core.MustNewID()
		request := &user.CreateUserRequest{
			Email: "invalid-email",
			Role:  user.RoleOrgCustomer,
		}
		// Execute
		result, err := service.CreateUser(ctx, orgID, request)
		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid email")
		// Verify no DB calls were made
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestUserService_UpdateUser(t *testing.T) {
	t.Run("Should update user email successfully", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		mockDB := &MockDBInterface{mockPool: mockPool}
		// Create repository
		repo := user.NewPostgresRepository(mockDB)
		// Create service
		service := user.NewService(repo, mockDB)
		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		// Test data
		orgID := core.MustNewID()
		userID := core.MustNewID()
		existingUser := &user.User{
			ID:        userID,
			OrgID:     orgID,
			Email:     "old@example.com",
			Role:      user.RoleOrgCustomer,
			Status:    user.StatusActive,
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now().Add(-24 * time.Hour),
		}
		newEmail := "new@example.com"
		request := &user.UpdateUserRequest{
			Email: &newEmail,
		}
		// Mock repository calls
		// 1. Get existing user
		rows := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(existingUser.ID, existingUser.OrgID, existingUser.Email, existingUser.Role, existingUser.Status, existingUser.CreatedAt, existingUser.UpdatedAt)
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID).
			WillReturnRows(rows)
		// 2. Check if new email is already taken (should not exist)
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND email = \\$2").
			WithArgs(orgID, "new@example.com").
			WillReturnError(user.ErrUserNotFound)
		// 3. Update user
		mockPool.ExpectExec("UPDATE users SET").
			WithArgs(
				orgID,
				userID,
				"new@example.com",
				existingUser.Role,
				existingUser.Status,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		// Execute
		result, err := service.UpdateUser(ctx, orgID, userID, request)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "new@example.com", result.Email)
		assert.Equal(t, existingUser.Role, result.Role)
		assert.Equal(t, existingUser.Status, result.Status)
		// Verify all expectations were met
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should update user role and status", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		mockDB := &MockDBInterface{mockPool: mockPool}
		// Create repository
		repo := user.NewPostgresRepository(mockDB)
		// Create service
		service := user.NewService(repo, mockDB)
		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		// Test data
		orgID := core.MustNewID()
		userID := core.MustNewID()
		existingUser := &user.User{
			ID:        userID,
			OrgID:     orgID,
			Email:     "user@example.com",
			Role:      user.RoleOrgCustomer,
			Status:    user.StatusActive,
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now().Add(-24 * time.Hour),
		}
		newRole := user.RoleOrgManager
		newStatus := user.StatusSuspended
		request := &user.UpdateUserRequest{
			Role:   &newRole,
			Status: &newStatus,
		}
		// Mock repository calls
		// 1. Get existing user
		rows := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(existingUser.ID, existingUser.OrgID, existingUser.Email, existingUser.Role, existingUser.Status, existingUser.CreatedAt, existingUser.UpdatedAt)
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID).
			WillReturnRows(rows)
		// 2. Update user
		mockPool.ExpectExec("UPDATE users SET").
			WithArgs(
				orgID,
				userID,
				existingUser.Email,
				newRole,
				newStatus,
			).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		// Execute
		result, err := service.UpdateUser(ctx, orgID, userID, request)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, existingUser.Email, result.Email)
		assert.Equal(t, newRole, result.Role)
		assert.Equal(t, newStatus, result.Status)
		// Verify all expectations were met
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestUserService_BulkOperations(t *testing.T) {
	t.Run("Should execute bulk suspend operation", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		mockDB := &MockDBInterface{mockPool: mockPool}
		// Create repository
		repo := user.NewPostgresRepository(mockDB)
		// Create service
		service := user.NewService(repo, mockDB)
		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		// Test data
		orgID := core.MustNewID()
		userID1 := core.MustNewID()
		userID2 := core.MustNewID()
		request := &user.BulkUserOperation{
			UserIDs:   []core.ID{userID1, userID2},
			Operation: "suspend",
		}
		// Mock transaction
		mockPool.ExpectBegin()
		// Mock repository calls for first user
		rows1 := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(userID1, orgID, "user1@example.com", user.RoleOrgCustomer, user.StatusActive, time.Now(), time.Now())
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID1).
			WillReturnRows(rows1)
		mockPool.ExpectExec("UPDATE users SET status = \\$3, updated_at = CURRENT_TIMESTAMP WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID1, user.StatusSuspended).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		// Mock repository calls for second user
		rows2 := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(userID2, orgID, "user2@example.com", user.RoleOrgCustomer, user.StatusActive, time.Now(), time.Now())
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID2).
			WillReturnRows(rows2)
		mockPool.ExpectExec("UPDATE users SET status = \\$3, updated_at = CURRENT_TIMESTAMP WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID2, user.StatusSuspended).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		// Commit transaction
		mockPool.ExpectCommit()
		// Execute
		errs, err := service.ExecuteBulkOperation(ctx, orgID, request)
		// Assert
		assert.NoError(t, err)
		assert.Empty(t, errs)
		// Verify all expectations were met
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestUserService_Permissions(t *testing.T) {
	t.Run("Should check permissions correctly for active users", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		mockDB := &MockDBInterface{mockPool: mockPool}
		// Create repository
		repo := user.NewPostgresRepository(mockDB)
		// Create service
		service := user.NewService(repo, mockDB)
		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		// Test data
		orgID := core.MustNewID()
		userID := core.MustNewID()
		// Test cases
		testCases := []struct {
			name       string
			role       user.Role
			permission string
			expected   bool
		}{
			{
				name:       "Org admin has workflow write permission",
				role:       user.RoleOrgAdmin,
				permission: user.PermWorkflowWrite,
				expected:   true,
			},
			{
				name:       "Org customer has workflow read permission",
				role:       user.RoleOrgCustomer,
				permission: user.PermWorkflowRead,
				expected:   true,
			},
			{
				name:       "Org customer does not have workflow write permission",
				role:       user.RoleOrgCustomer,
				permission: user.PermWorkflowWrite,
				expected:   false,
			},
			{
				name:       "System admin has all permissions",
				role:       user.RoleSystemAdmin,
				permission: user.PermSystemManage,
				expected:   true,
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Mock repository call
				rows := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
					AddRow(userID, orgID, "user@example.com", tc.role, user.StatusActive, time.Now(), time.Now())
				mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND id = \\$2").
					WithArgs(orgID, userID).
					WillReturnRows(rows)
				// Execute
				hasPermission, err := service.CheckPermission(ctx, orgID, userID, tc.permission)
				// Assert
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, hasPermission)
			})
		}
		// Verify all expectations were met
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
	t.Run("Should deny permissions for suspended users", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()
		mockDB := &MockDBInterface{mockPool: mockPool}
		// Create repository
		repo := user.NewPostgresRepository(mockDB)
		// Create service
		service := user.NewService(repo, mockDB)
		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
		// Test data
		orgID := core.MustNewID()
		userID := core.MustNewID()
		// Mock repository call for suspended user
		rows := mockPool.NewRows([]string{"id", "org_id", "email", "role", "status", "created_at", "updated_at"}).
			AddRow(userID, orgID, "user@example.com", user.RoleOrgAdmin, user.StatusSuspended, time.Now(), time.Now())
		mockPool.ExpectQuery("SELECT (.+) FROM users WHERE org_id = \\$1 AND id = \\$2").
			WithArgs(orgID, userID).
			WillReturnRows(rows)
		// Execute
		hasPermission, err := service.CheckPermission(ctx, orgID, userID, user.PermWorkflowWrite)
		// Assert
		assert.NoError(t, err)
		assert.False(t, hasPermission, "Suspended user should not have permissions")
		// Verify all expectations were met
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
