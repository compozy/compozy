package bootstrap_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/compozy/compozy/engine/auth/bootstrap"
	"github.com/compozy/compozy/engine/auth/model"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of authuc.Repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateUser(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockRepository) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockRepository) ListUsers(ctx context.Context) ([]*model.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.User), args.Error(1)
}

func (m *MockRepository) UpdateUser(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockRepository) DeleteUser(ctx context.Context, id core.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockRepository) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.APIKey), args.Error(1)
}

func (m *MockRepository) GetAPIKeyByHash(ctx context.Context, hash []byte) (*model.APIKey, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.APIKey), args.Error(1)
}

func (m *MockRepository) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.APIKey), args.Error(1)
}

func (m *MockRepository) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) DeleteAPIKey(ctx context.Context, id core.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) CreateInitialAdminIfNone(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func TestService_CheckBootstrapStatus(t *testing.T) {
	t.Run("Should return bootstrapped status when admin exists", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		adminID, err := core.NewID()
		require.NoError(t, err)
		userID, err := core.NewID()
		require.NoError(t, err)

		users := []*model.User{
			{ID: adminID, Email: "admin@test.com", Role: model.RoleAdmin},
			{ID: userID, Email: "user@test.com", Role: model.RoleUser},
		}
		mockRepo.On("ListUsers", ctx).Return(users, nil)

		// When
		status, err := service.CheckBootstrapStatus(ctx)

		// Then
		require.NoError(t, err)
		assert.True(t, status.IsBootstrapped)
		assert.Equal(t, 1, status.AdminCount)
		assert.Equal(t, 2, status.UserCount)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return not bootstrapped when no admin exists", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		userID, err := core.NewID()
		require.NoError(t, err)

		users := []*model.User{
			{ID: userID, Email: "user@test.com", Role: model.RoleUser},
		}
		mockRepo.On("ListUsers", ctx).Return(users, nil)

		// When
		status, err := service.CheckBootstrapStatus(ctx)

		// Then
		require.NoError(t, err)
		assert.False(t, status.IsBootstrapped)
		assert.Equal(t, 0, status.AdminCount)
		assert.Equal(t, 1, status.UserCount)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return error when repository fails", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		mockRepo.On("ListUsers", ctx).Return(nil, errors.New("database error"))

		// When
		status, err := service.CheckBootstrapStatus(ctx)

		// Then
		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "failed to check bootstrap status")
		mockRepo.AssertExpectations(t)
	})
}

func TestService_BootstrapAdmin(t *testing.T) {
	t.Run("Should create initial admin when system not bootstrapped", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		input := &bootstrap.Input{
			Email: "admin@test.com",
			Force: false,
		}

		// Setup expectations for CheckBootstrapStatus
		mockRepo.On("ListUsers", ctx).Return([]*model.User{}, nil).Once()

		adminID, err := core.NewID()
		require.NoError(t, err)

		// For CreateInitialAdminIfNone called by BootstrapSystem
		mockRepo.On("CreateInitialAdminIfNone", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.Email == "admin@test.com" && u.Role == model.RoleAdmin
		})).Return(nil).Run(func(args mock.Arguments) {
			// Set the ID on the user object passed in
			u := args.Get(1).(*model.User)
			u.ID = adminID
		})

		apiKeyID, err := core.NewID()
		require.NoError(t, err)

		// For CreateAPIKey
		mockRepo.On("CreateAPIKey", ctx, mock.MatchedBy(func(k *model.APIKey) bool {
			return k.UserID == adminID
		})).Return(nil).Run(func(args mock.Arguments) {
			// Set the ID on the API key object passed in
			k := args.Get(1).(*model.APIKey)
			k.ID = apiKeyID
			k.Hash = []byte("test-api-key-hash")
			k.Fingerprint = []byte("test-fingerprint")
			k.Prefix = "cpzy_"
		})

		// When
		result, err := service.BootstrapAdmin(ctx, input)

		// Then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, adminID.String(), result.UserID)
		assert.Equal(t, "admin@test.com", result.Email)
		assert.NotEmpty(t, result.APIKey)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should fail when system already bootstrapped without force", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		input := &bootstrap.Input{
			Email: "admin2@test.com",
			Force: false,
		}

		adminID, err := core.NewID()
		require.NoError(t, err)

		existingAdmin := &model.User{
			ID:    adminID,
			Email: "admin@test.com",
			Role:  model.RoleAdmin,
		}
		mockRepo.On("ListUsers", ctx).Return([]*model.User{existingAdmin}, nil)

		// When
		result, err := service.BootstrapAdmin(ctx, input)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "BOOTSTRAP_ALREADY_COMPLETE", coreErr.Code)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should create additional admin when force is true", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		input := &bootstrap.Input{
			Email: "admin2@test.com",
			Force: true,
		}

		existingAdminID, err := core.NewID()
		require.NoError(t, err)

		existingAdmin := &model.User{
			ID:    existingAdminID,
			Email: "admin@test.com",
			Role:  model.RoleAdmin,
		}
		mockRepo.On("ListUsers", ctx).Return([]*model.User{existingAdmin}, nil)

		newAdminID, err := core.NewID()
		require.NoError(t, err)

		// For GetUserByEmail called by CreateUser use case
		mockRepo.On("GetUserByEmail", ctx, "admin2@test.com").Return(nil, authuc.ErrUserNotFound)

		// For CreateUser
		mockRepo.On("CreateUser", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.Email == "admin2@test.com" && u.Role == model.RoleAdmin
		})).Return(nil).Run(func(args mock.Arguments) {
			// Set the ID on the user object passed in
			u := args.Get(1).(*model.User)
			u.ID = newAdminID
		})

		apiKeyID, err := core.NewID()
		require.NoError(t, err)

		// For CreateAPIKey
		mockRepo.On("CreateAPIKey", ctx, mock.MatchedBy(func(k *model.APIKey) bool {
			return k.UserID == newAdminID
		})).Return(nil).Run(func(args mock.Arguments) {
			// Set the ID on the API key object passed in
			k := args.Get(1).(*model.APIKey)
			k.ID = apiKeyID
			k.Hash = []byte("test-api-key-hash-2")
			k.Fingerprint = []byte("test-fingerprint-2")
			k.Prefix = "cpzy_"
		})

		// When
		result, err := service.BootstrapAdmin(ctx, input)

		// Then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, newAdminID.String(), result.UserID)
		assert.Equal(t, "admin2@test.com", result.Email)
		assert.NotEmpty(t, result.APIKey)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return error when email is empty", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		input := &bootstrap.Input{
			Email: "",
			Force: false,
		}

		// When
		result, err := service.BootstrapAdmin(ctx, input)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		coreErr, ok := err.(*core.Error)
		require.True(t, ok)
		assert.Equal(t, "BOOTSTRAP_INVALID_INPUT", coreErr.Code)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should enforce admin role even if non-admin role is passed", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		input := &bootstrap.Input{
			Email: "user@test.com",
			Force: false,
		}

		// Setup expectations
		mockRepo.On("ListUsers", ctx).Return([]*model.User{}, nil)

		adminID, err := core.NewID()
		require.NoError(t, err)

		// The repository should enforce admin role regardless of what role the user object has
		// This validates that CreateInitialAdminIfNone enforces admin role
		mockRepo.On("CreateInitialAdminIfNone", ctx, mock.MatchedBy(func(u *model.User) bool {
			// Even if a malicious actor somehow passed a user object with non-admin role,
			// the repository should enforce admin role
			// After our fix, u.Role should ALWAYS be model.RoleAdmin here
			return u.Email == "user@test.com" && u.Role == model.RoleAdmin
		})).Return(nil).Run(func(args mock.Arguments) {
			// Verify that the user object was modified to have admin role
			u := args.Get(1).(*model.User)
			assert.Equal(t, model.RoleAdmin, u.Role, "Repository should enforce admin role")
			u.ID = adminID
		})

		apiKeyID, err := core.NewID()
		require.NoError(t, err)

		// For CreateAPIKey
		mockRepo.On("CreateAPIKey", ctx, mock.MatchedBy(func(k *model.APIKey) bool {
			return k.UserID == adminID
		})).Return(nil).Run(func(args mock.Arguments) {
			k := args.Get(1).(*model.APIKey)
			k.ID = apiKeyID
			k.Hash = []byte("test-api-key-hash")
			k.Fingerprint = []byte("test-fingerprint")
			k.Prefix = "cpzy_"
		})

		// When
		result, err := service.BootstrapAdmin(ctx, input)

		// Then
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, adminID.String(), result.UserID)
		assert.Equal(t, "user@test.com", result.Email)
		assert.NotEmpty(t, result.APIKey)
		mockRepo.AssertExpectations(t)
	})
}

func TestService_CreateInitialAdmin(t *testing.T) {
	t.Run("Should create initial admin successfully", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		// Setup expectations for CheckBootstrapStatus
		mockRepo.On("ListUsers", ctx).Return([]*model.User{}, nil).Once()

		adminID, err := core.NewID()
		require.NoError(t, err)

		// For GetUserByEmail called by CreateUser use case
		mockRepo.On("GetUserByEmail", ctx, "admin@test.com").Return(nil, authuc.ErrUserNotFound)

		// For CreateUser
		mockRepo.On("CreateUser", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.Email == "admin@test.com" && u.Role == model.RoleAdmin
		})).Return(nil).Run(func(args mock.Arguments) {
			// Set the ID on the user object passed in
			u := args.Get(1).(*model.User)
			u.ID = adminID
		})

		apiKeyID, err := core.NewID()
		require.NoError(t, err)

		// For CreateAPIKey
		mockRepo.On("CreateAPIKey", ctx, mock.MatchedBy(func(k *model.APIKey) bool {
			return k.UserID == adminID
		})).Return(nil).Run(func(args mock.Arguments) {
			// Set the ID on the API key object passed in
			k := args.Get(1).(*model.APIKey)
			k.ID = apiKeyID
			k.Hash = []byte("test-api-key-hash")
			k.Fingerprint = []byte("test-fingerprint")
			k.Prefix = "cpzy_"
		})

		// When
		user, key, err := service.CreateInitialAdmin(ctx, "admin@test.com")

		// Then
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, "admin@test.com", user.Email)
		assert.Equal(t, model.RoleAdmin, user.Role)
		assert.NotEmpty(t, key)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should fail when admin already exists", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		adminID, err := core.NewID()
		require.NoError(t, err)

		existingAdmin := &model.User{
			ID:    adminID,
			Email: "admin@test.com",
			Role:  model.RoleAdmin,
		}
		mockRepo.On("ListUsers", ctx).Return([]*model.User{existingAdmin}, nil)

		// When
		user, key, err := service.CreateInitialAdmin(ctx, "admin2@test.com")

		// Then
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Empty(t, key)
		assert.Contains(t, err.Error(), "system already has 1 admin user(s)")
		mockRepo.AssertExpectations(t)
	})
}

func TestService_ConcurrentBootstrap(t *testing.T) {
	t.Run("Should handle unique constraint violation gracefully", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		// No admins initially
		mockRepo.On("ListUsers", ctx).Return([]*model.User{}, nil).Once()

		// Simulate unique constraint violation (PostgreSQL error code 23505)
		pgErr := &pgconn.PgError{
			Code:    "23505",
			Message: "duplicate key value violates unique constraint",
		}
		mockRepo.On("CreateInitialAdminIfNone", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.Email == "admin@test.com" && u.Role == model.RoleAdmin
		})).Return(pgErr).Once()

		// When
		result, err := service.BootstrapAdmin(ctx, &bootstrap.Input{
			Email: "admin@test.com",
			Force: false,
		})

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		// The error should be wrapped but indicate bootstrap failure
		assert.Contains(t, err.Error(), "failed to bootstrap system")
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle concurrent bootstrap attempts safely", func(t *testing.T) {
		// Given
		ctx := t.Context()
		mockRepo := new(MockRepository)
		factory := authuc.NewFactory(mockRepo)
		service := bootstrap.NewService(factory)

		// Setup expectations - only one admin should be created successfully
		// First call succeeds
		mockRepo.On("ListUsers", ctx).Return([]*model.User{}, nil).Times(2)

		adminID, err := core.NewID()
		require.NoError(t, err)

		// First CreateInitialAdminIfNone succeeds
		mockRepo.On("CreateInitialAdminIfNone", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.Email == "admin@test.com" && u.Role == model.RoleAdmin
		})).Return(nil).Run(func(args mock.Arguments) {
			u := args.Get(1).(*model.User)
			u.ID = adminID
		}).Once()

		// Second CreateInitialAdminIfNone fails with already bootstrapped
		mockRepo.On("CreateInitialAdminIfNone", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.Email == "admin@test.com" && u.Role == model.RoleAdmin
		})).Return(core.NewError(
			fmt.Errorf("system already bootstrapped"),
			"ALREADY_BOOTSTRAPPED",
			nil,
		)).Once()

		apiKeyID, err := core.NewID()
		require.NoError(t, err)

		// Only one API key should be created (for the successful admin creation)
		mockRepo.On("CreateAPIKey", ctx, mock.MatchedBy(func(k *model.APIKey) bool {
			return k.UserID == adminID
		})).Return(nil).Run(func(args mock.Arguments) {
			k := args.Get(1).(*model.APIKey)
			k.ID = apiKeyID
			k.Hash = []byte("test-api-key-hash")
			k.Fingerprint = []byte("test-fingerprint")
			k.Prefix = "cpzy_"
		}).Once()

		input := &bootstrap.Input{
			Email: "admin@test.com",
			Force: false,
		}

		// When - simulate concurrent bootstrap attempts
		var wg sync.WaitGroup
		results := make([]*bootstrap.Result, 2)
		errs := make([]error, 2)

		for i := range 2 {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				results[index], errs[index] = service.BootstrapAdmin(ctx, input)
			}(i)
		}

		wg.Wait()

		// Then - exactly one should succeed, one should fail
		var successCount, errorCount int
		for i := range 2 {
			if errs[i] == nil {
				successCount++
				assert.NotNil(t, results[i])
				assert.Equal(t, adminID.String(), results[i].UserID)
				assert.Equal(t, "admin@test.com", results[i].Email)
				assert.NotEmpty(t, results[i].APIKey)
			} else {
				errorCount++
				assert.Nil(t, results[i])
				// Should contain the structured error information
				var coreErr *core.Error
				assert.True(t, errors.As(errs[i], &coreErr))
				assert.Equal(t, "ALREADY_BOOTSTRAPPED", coreErr.Code)
			}
		}

		// Verify exactly one success and one failure
		assert.Equal(t, 1, successCount, "Exactly one bootstrap should succeed")
		assert.Equal(t, 1, errorCount, "Exactly one bootstrap should fail")
		mockRepo.AssertExpectations(t)
	})
}
