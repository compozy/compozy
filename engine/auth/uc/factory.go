package uc

import (
	"github.com/compozy/compozy/engine/core"
)

// Factory provides methods to create use case instances with proper dependency injection
type Factory struct {
	repo Repository
}

// NewFactory creates a new use case factory
func NewFactory(repo Repository) *Factory {
	return &Factory{
		repo: repo,
	}
}

// CreateUser creates a new create user use case
func (f *Factory) CreateUser(input *CreateUserInput) *CreateUser {
	return NewCreateUser(f.repo, input)
}

// GetUser creates a new get user use case
func (f *Factory) GetUser(userID core.ID) *GetUser {
	return NewGetUser(f.repo, userID)
}

// GetUserByEmail creates a new get user by email use case
func (f *Factory) GetUserByEmail(email string) *GetUserByEmail {
	return NewGetUserByEmail(f.repo, email)
}

// ListUsers creates a new list users use case
func (f *Factory) ListUsers() *ListUsers {
	return NewListUsers(f.repo)
}

// UpdateUser creates a new update user use case
func (f *Factory) UpdateUser(userID core.ID, input *UpdateUserInput) *UpdateUser {
	return NewUpdateUser(f.repo, userID, input)
}

// DeleteUser creates a new delete user use case
func (f *Factory) DeleteUser(userID core.ID) *DeleteUser {
	return NewDeleteUser(f.repo, userID)
}

// ValidateAPIKey creates a new validate API key use case
func (f *Factory) ValidateAPIKey(plaintext string) *ValidateAPIKey {
	return NewValidateAPIKey(f.repo, plaintext)
}

// GenerateAPIKey creates a new generate API key use case
func (f *Factory) GenerateAPIKey(userID core.ID) *GenerateAPIKey {
	return NewGenerateAPIKey(f.repo, userID)
}

// RevokeAPIKey creates a new revoke API key use case
func (f *Factory) RevokeAPIKey(userID, keyID core.ID) *RevokeAPIKey {
	return NewRevokeAPIKey(f.repo, userID, keyID)
}

// GetAPIKey creates a new get API key use case
func (f *Factory) GetAPIKey() *GetAPIKey {
	return NewGetAPIKey(f.repo)
}

// ListAPIKeys creates a new list API keys use case
func (f *Factory) ListAPIKeys(userID core.ID) *ListAPIKeys {
	return NewListAPIKeys(f.repo, userID)
}
