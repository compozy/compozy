package org_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/org"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockTemporalService is a mock implementation of TemporalService
type MockTemporalService struct {
	mock.Mock
}

func (m *MockTemporalService) ProvisionNamespace(ctx context.Context, namespace string) error {
	args := m.Called(ctx, namespace)
	return args.Error(0)
}

func (m *MockTemporalService) DeleteNamespace(ctx context.Context, namespace string) error {
	args := m.Called(ctx, namespace)
	return args.Error(0)
}

func (m *MockTemporalService) NamespaceExists(ctx context.Context, namespace string) (bool, error) {
	args := m.Called(ctx, namespace)
	return args.Bool(0), args.Error(1)
}

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

func TestOrganizationService_CreateOrganization(t *testing.T) {
	t.Run("Should create organization with Temporal namespace successfully", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()

		mockDB := &MockDBInterface{mockPool: mockPool}
		mockTemporal := &MockTemporalService{}

		// Create repository
		repo := org.NewPostgresRepository(mockDB)

		// Create service
		service := org.NewService(repo, mockTemporal, mockDB, nil)

		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())

		// Test data
		request := &org.CreateOrganizationRequest{
			Name: "Test Organization",
		}

		// Mock repository calls
		// 1. Check for existing organization by name (should not exist)
		mockPool.ExpectQuery("SELECT (.+) FROM organizations WHERE name = \\$1").
			WithArgs("Test Organization").
			WillReturnError(org.ErrOrganizationNotFound)

		// 2. Begin transaction
		mockPool.ExpectBegin()

		// 3. Create organization
		mockPool.ExpectExec("INSERT INTO organizations").
			WithArgs(
				pgxmock.AnyArg(),       // ID
				"Test Organization",    // Name
				pgxmock.AnyArg(),       // TemporalNamespace
				org.StatusProvisioning, // Status
				pgxmock.AnyArg(),       // CreatedAt
				pgxmock.AnyArg(),       // UpdatedAt
			).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		// 4. Commit transaction
		mockPool.ExpectCommit()

		// 5. Mock Temporal namespace existence check (doesn't exist initially)
		mockTemporal.On("NamespaceExists", mock.Anything, mock.MatchedBy(func(ns string) bool {
			return strings.HasPrefix(ns, "org-test-organization-") && len(ns) > 20
		})).Return(false, nil)

		// 6. Mock Temporal namespace provisioning
		mockTemporal.On("ProvisionNamespace", mock.Anything, mock.MatchedBy(func(ns string) bool {
			return strings.HasPrefix(ns, "org-test-organization-") && len(ns) > 20
		})).Return(nil)

		// 6. Begin transaction for status update
		mockPool.ExpectBegin()

		// 7. Update status to active
		mockPool.ExpectExec("UPDATE organizations SET status = \\$2, updated_at = CURRENT_TIMESTAMP WHERE id = \\$1").
			WithArgs(pgxmock.AnyArg(), org.StatusActive).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		// 8. Commit transaction
		mockPool.ExpectCommit()

		// Execute
		result, err := service.CreateOrganization(ctx, request)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Test Organization", result.Name)
		assert.Equal(t, org.StatusActive, result.Status)
		assert.NotEmpty(t, result.ID)
		assert.NotEmpty(t, result.TemporalNamespace)
		assert.Regexp(
			t,
			`^org-test-organization-[a-zA-Z0-9]{8}$`,
			result.TemporalNamespace,
		) // Should have org-{slug}-{uuid} format

		// Verify all expectations were met
		assert.NoError(t, mockPool.ExpectationsWereMet())
		mockTemporal.AssertExpectations(t)
	})

	t.Run("Should handle Temporal namespace provisioning failure", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()

		mockDB := &MockDBInterface{mockPool: mockPool}
		mockTemporal := &MockTemporalService{}

		// Create repository
		repo := org.NewPostgresRepository(mockDB)

		// Create service
		service := org.NewService(repo, mockTemporal, mockDB, nil)

		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())

		// Test data
		request := &org.CreateOrganizationRequest{
			Name: "Test Organization",
		}

		// Mock repository calls
		// 1. Check for existing organization by name (should not exist)
		mockPool.ExpectQuery("SELECT (.+) FROM organizations WHERE name = \\$1").
			WithArgs("Test Organization").
			WillReturnError(org.ErrOrganizationNotFound)

		// 2. Begin transaction
		mockPool.ExpectBegin()

		// 3. Create organization
		mockPool.ExpectExec("INSERT INTO organizations").
			WithArgs(
				pgxmock.AnyArg(),       // ID
				"Test Organization",    // Name
				pgxmock.AnyArg(),       // TemporalNamespace
				org.StatusProvisioning, // Status
				pgxmock.AnyArg(),       // CreatedAt
				pgxmock.AnyArg(),       // UpdatedAt
			).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		// 4. Commit transaction
		mockPool.ExpectCommit()

		// 5. Mock Temporal namespace existence check (doesn't exist initially)
		mockTemporal.On("NamespaceExists", mock.Anything, mock.MatchedBy(func(ns string) bool {
			return strings.HasPrefix(ns, "org-test-organization-") && len(ns) > 20
		})).Return(false, nil)

		// 6. Mock Temporal namespace provisioning failure
		mockTemporal.On("ProvisionNamespace", mock.Anything, mock.MatchedBy(func(ns string) bool {
			return strings.HasPrefix(ns, "org-test-organization-") && len(ns) > 20
		})).Return(assert.AnError)

		// 6. Begin transaction for status update to failed
		mockPool.ExpectBegin()

		// 7. Update status to provisioning_failed
		mockPool.ExpectExec("UPDATE organizations SET status = \\$2, updated_at = CURRENT_TIMESTAMP WHERE id = \\$1").
			WithArgs(pgxmock.AnyArg(), org.StatusProvisioningFailed).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		// 8. Commit transaction
		mockPool.ExpectCommit()

		// Execute
		result, err := service.CreateOrganization(ctx, request)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to provision Temporal namespace")

		// Verify all expectations were met
		assert.NoError(t, mockPool.ExpectationsWereMet())
		mockTemporal.AssertExpectations(t)
	})

	t.Run("Should reject duplicate organization name", func(t *testing.T) {
		// Setup mocks
		mockPool, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mockPool.Close()

		mockDB := &MockDBInterface{mockPool: mockPool}
		mockTemporal := &MockTemporalService{}

		// Create repository
		repo := org.NewPostgresRepository(mockDB)

		// Create service
		service := org.NewService(repo, mockTemporal, mockDB, nil)

		// Setup context with logger
		ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())

		// Test data
		request := &org.CreateOrganizationRequest{
			Name: "Existing Organization",
		}

		// Mock existing organization
		existingOrg := &org.Organization{
			ID:   core.MustNewID(),
			Name: "Existing Organization",
		}

		// Mock repository call to find existing organization
		rows := mockPool.NewRows([]string{"id", "name", "temporal_namespace", "status", "created_at", "updated_at"}).
			AddRow(existingOrg.ID, existingOrg.Name, "existing-namespace", org.StatusActive, time.Now(), time.Now())

		mockPool.ExpectQuery("SELECT (.+) FROM organizations WHERE name = \\$1").
			WithArgs("Existing Organization").
			WillReturnRows(rows)

		// Execute
		result, err := service.CreateOrganization(ctx, request)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "organization with name 'Existing Organization' already exists")

		// Verify all expectations were met
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestOrganizationService_NamespaceGeneration(t *testing.T) {
	t.Run("Should generate proper namespace format through service", func(t *testing.T) {
		// Test cases for namespace generation through the actual service
		testCases := []struct {
			name       string
			orgName    string
			expectedRx string // regex pattern to match
		}{
			{
				name:       "Simple name",
				orgName:    "Acme Corp",
				expectedRx: `^org-acme-corp-[a-zA-Z0-9]{8}$`,
			},
			{
				name:       "Name with underscores",
				orgName:    "Test_Organization",
				expectedRx: `^org-test-organization-[a-zA-Z0-9]{8}$`,
			},
			{
				name:       "Name starting with number",
				orgName:    "123 Company",
				expectedRx: `^org-org-123-company-[a-zA-Z0-9]{8}$`,
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Setup mocks
				mockPool, err := pgxmock.NewPool()
				require.NoError(t, err)
				defer mockPool.Close()
				mockDB := &MockDBInterface{mockPool: mockPool}
				mockTemporal := &MockTemporalService{}
				repo := org.NewPostgresRepository(mockDB)
				service := org.NewService(repo, mockTemporal, mockDB, nil)
				ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
				request := &org.CreateOrganizationRequest{Name: tc.orgName}
				// Mock repository calls
				mockPool.ExpectQuery("SELECT (.+) FROM organizations WHERE name = \\$1").
					WithArgs(tc.orgName).
					WillReturnError(org.ErrOrganizationNotFound)
				mockPool.ExpectBegin()
				mockPool.ExpectExec("INSERT INTO organizations").
					WithArgs(
						pgxmock.AnyArg(), // ID
						tc.orgName,
						pgxmock.AnyArg(), // TemporalNamespace
						org.StatusProvisioning,
						pgxmock.AnyArg(), // CreatedAt
						pgxmock.AnyArg(), // UpdatedAt
					).
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
				mockPool.ExpectCommit()
				mockTemporal.On("NamespaceExists", mock.Anything, mock.AnythingOfType("string")).Return(false, nil)
				mockTemporal.On("ProvisionNamespace", mock.Anything, mock.AnythingOfType("string")).Return(nil)
				mockPool.ExpectBegin()
				mockPool.ExpectExec("UPDATE organizations SET status = \\$2, updated_at = CURRENT_TIMESTAMP WHERE id = \\$1").
					WithArgs(pgxmock.AnyArg(), org.StatusActive).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				mockPool.ExpectCommit()
				// Execute
				result, err := service.CreateOrganization(ctx, request)
				// Assert
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Regexp(t, tc.expectedRx, result.TemporalNamespace)
				// Verify expectations
				assert.NoError(t, mockPool.ExpectationsWereMet())
				mockTemporal.AssertExpectations(t)
			})
		}
	})
}
