package uc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of uc.Repository
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

func (m *MockRepository) CreateInitialAdminIfNone(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
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

func TestUpdateUser_Execute(t *testing.T) {
	ctx := context.Background()

	t.Run("Should handle email uniqueness check when user not found", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepository)
		userID := core.MustNewID()
		existingUser := &model.User{
			ID:    userID,
			Email: "old@test.com",
			Role:  model.RoleUser,
		}
		newEmail := "new@test.com"
		input := &uc.UpdateUserInput{
			Email: &newEmail,
		}
		// Mock GetUserByID
		mockRepo.On("GetUserByID", ctx, userID).Return(existingUser, nil)
		// Mock GetUserByEmail returns ErrUserNotFound - email is available
		mockRepo.On("GetUserByEmail", ctx, newEmail).Return(nil, uc.ErrUserNotFound)
		// Mock UpdateUser
		mockRepo.On("UpdateUser", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.ID == userID && u.Email == newEmail
		})).Return(nil)
		// Act
		updateUser := uc.NewUpdateUser(mockRepo, userID, input)
		result, err := updateUser.Execute(ctx)
		// Assert
		require.NoError(t, err)
		assert.Equal(t, newEmail, result.Email)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should fail on repository error during email uniqueness check", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepository)
		userID := core.MustNewID()
		existingUser := &model.User{
			ID:    userID,
			Email: "old@test.com",
			Role:  model.RoleUser,
		}
		newEmail := "new@test.com"
		input := &uc.UpdateUserInput{
			Email: &newEmail,
		}
		dbError := errors.New("database connection failed")
		// Mock GetUserByID
		mockRepo.On("GetUserByID", ctx, userID).Return(existingUser, nil)
		// Mock GetUserByEmail returns a database error
		mockRepo.On("GetUserByEmail", ctx, newEmail).Return(nil, dbError)
		// Act
		updateUser := uc.NewUpdateUser(mockRepo, userID, input)
		result, err := updateUser.Execute(ctx)
		// Assert
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "checking email uniqueness")
		assert.ErrorIs(t, err, dbError)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return generic error when email already in use", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepository)
		userID := core.MustNewID()
		otherUserID := core.MustNewID()
		existingUser := &model.User{
			ID:    userID,
			Email: "old@test.com",
			Role:  model.RoleUser,
		}
		otherUser := &model.User{
			ID:    otherUserID,
			Email: "new@test.com",
			Role:  model.RoleUser,
		}
		newEmail := "new@test.com"
		input := &uc.UpdateUserInput{
			Email: &newEmail,
		}
		// Mock GetUserByID
		mockRepo.On("GetUserByID", ctx, userID).Return(existingUser, nil)
		// Mock GetUserByEmail returns another user with the same email
		mockRepo.On("GetUserByEmail", ctx, newEmail).Return(otherUser, nil)
		// Act
		updateUser := uc.NewUpdateUser(mockRepo, userID, input)
		result, err := updateUser.Execute(ctx)
		// Assert
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, "email already in use", err.Error())
		// Verify error does not contain email address (PII)
		assert.NotContains(t, err.Error(), newEmail)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should allow user to keep their current email", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepository)
		userID := core.MustNewID()
		currentEmail := "current@test.com"
		existingUser := &model.User{
			ID:    userID,
			Email: currentEmail,
			Role:  model.RoleUser,
		}
		input := &uc.UpdateUserInput{
			Email: &currentEmail, // Same email
		}
		// Mock GetUserByID
		mockRepo.On("GetUserByID", ctx, userID).Return(existingUser, nil)
		// Mock GetUserByEmail returns the same user
		mockRepo.On("GetUserByEmail", ctx, currentEmail).Return(existingUser, nil)
		// Mock UpdateUser
		mockRepo.On("UpdateUser", ctx, existingUser).Return(nil)
		// Act
		updateUser := uc.NewUpdateUser(mockRepo, userID, input)
		result, err := updateUser.Execute(ctx)
		// Assert
		require.NoError(t, err)
		assert.Equal(t, currentEmail, result.Email)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should update only role when email not provided", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepository)
		userID := core.MustNewID()
		existingUser := &model.User{
			ID:    userID,
			Email: "user@test.com",
			Role:  model.RoleUser,
		}
		newRole := model.RoleAdmin
		input := &uc.UpdateUserInput{
			Role: &newRole,
		}
		// Mock GetUserByID
		mockRepo.On("GetUserByID", ctx, userID).Return(existingUser, nil)
		// Mock UpdateUser
		mockRepo.On("UpdateUser", ctx, mock.MatchedBy(func(u *model.User) bool {
			return u.ID == userID && u.Role == newRole && u.Email == "user@test.com"
		})).Return(nil)
		// Act
		updateUser := uc.NewUpdateUser(mockRepo, userID, input)
		result, err := updateUser.Execute(ctx)
		// Assert
		require.NoError(t, err)
		assert.Equal(t, newRole, result.Role)
		assert.Equal(t, "user@test.com", result.Email)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return error without PII when user not found", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepository)
		userID := core.MustNewID()
		newEmail := "new@test.com"
		input := &uc.UpdateUserInput{
			Email: &newEmail,
		}
		// Mock GetUserByID returns error
		mockRepo.On("GetUserByID", ctx, userID).Return(nil, errors.New("not found"))
		// Act
		updateUser := uc.NewUpdateUser(mockRepo, userID, input)
		result, err := updateUser.Execute(ctx)
		// Assert
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "user not found")
		// Verify error does not contain user ID (potential PII)
		assert.NotContains(t, err.Error(), userID.String())
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return error without PII when update fails", func(t *testing.T) {
		// Arrange
		mockRepo := new(MockRepository)
		userID := core.MustNewID()
		existingUser := &model.User{
			ID:    userID,
			Email: "user@test.com",
			Role:  model.RoleUser,
		}
		newRole := model.RoleAdmin
		input := &uc.UpdateUserInput{
			Role: &newRole,
		}
		// Mock GetUserByID
		mockRepo.On("GetUserByID", ctx, userID).Return(existingUser, nil)
		// Mock UpdateUser returns error
		mockRepo.On("UpdateUser", ctx, mock.Anything).Return(errors.New("database error"))
		// Act
		updateUser := uc.NewUpdateUser(mockRepo, userID, input)
		result, err := updateUser.Execute(ctx)
		// Assert
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to update user")
		// Verify error does not contain user ID (potential PII)
		assert.NotContains(t, err.Error(), userID.String())
		mockRepo.AssertExpectations(t)
	})
}
