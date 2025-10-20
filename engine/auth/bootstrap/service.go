package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// Service handles bootstrap operations
type Service struct {
	factory *uc.Factory
}

// NewService creates a new bootstrap service with direct factory access
func NewService(factory *uc.Factory) *Service {
	return &Service{
		factory: factory,
	}
}

// Input contains the input for bootstrap operation
type Input struct {
	Email string
	Force bool
}

// Result contains the result of bootstrap operation
type Result struct {
	UserID string
	Email  string
	APIKey string
}

// Status contains the current bootstrap status
type Status struct {
	IsBootstrapped bool
	AdminCount     int
	UserCount      int
}

// CheckBootstrapStatus checks if the system is already bootstrapped
func (s *Service) CheckBootstrapStatus(ctx context.Context) (*Status, error) {
	if s.factory == nil {
		return nil, fmt.Errorf("factory is required")
	}
	log := logger.FromContext(ctx)
	users, err := s.factory.ListUsers().Execute(ctx)
	if err != nil {
		log.Error("Failed to list users", "error", err)
		return nil, fmt.Errorf("failed to check bootstrap status: %w", err)
	}
	adminCount := 0
	for _, user := range users {
		if user.Role == model.RoleAdmin {
			adminCount++
		}
	}
	return &Status{
		IsBootstrapped: adminCount > 0,
		AdminCount:     adminCount,
		UserCount:      len(users),
	}, nil
}

// BootstrapAdmin creates the initial admin user
func (s *Service) BootstrapAdmin(ctx context.Context, input *Input) (*Result, error) {
	if err := s.validateInput(input); err != nil {
		return nil, err
	}
	status, err := s.CheckBootstrapStatus(ctx)
	if err != nil {
		return nil, core.NewError(
			fmt.Errorf("failed to check bootstrap status: %w", err),
			"BOOTSTRAP_STATUS_CHECK_FAILED",
			map[string]any{"error": err.Error()},
		)
	}
	if status.IsBootstrapped && !input.Force {
		return nil, core.NewError(
			fmt.Errorf("system is already bootstrapped with %d admin user(s)", status.AdminCount),
			"BOOTSTRAP_ALREADY_COMPLETE",
			map[string]any{
				"admin_count": status.AdminCount,
				"user_count":  status.UserCount,
			},
		)
	}
	logger.FromContext(ctx).Info("Creating admin user",
		"email", input.Email,
		"force", input.Force,
		"existing_admins", status.AdminCount)
	if input.Force && status.IsBootstrapped {
		return s.createAdditionalAdmin(ctx, input.Email)
	}
	return s.createInitialAdminUser(ctx, input.Email)
}

// validateInput validates the bootstrap input
func (s *Service) validateInput(input *Input) error {
	if s.factory == nil {
		return core.NewError(
			fmt.Errorf("factory is required"),
			"BOOTSTRAP_INVALID_CONFIG",
			map[string]any{"reason": "factory not initialized"},
		)
	}
	if input.Email == "" {
		return core.NewError(
			fmt.Errorf("email is required"),
			"BOOTSTRAP_INVALID_INPUT",
			map[string]any{"field": "email"},
		)
	}
	return nil
}

// createAdditionalAdmin creates an additional admin user when force flag is set
func (s *Service) createAdditionalAdmin(ctx context.Context, email string) (*Result, error) {
	createInput := &uc.CreateUserInput{
		Email: email,
		Role:  model.RoleAdmin,
	}
	user, err := s.factory.CreateUser(createInput).Execute(ctx)
	if err != nil {
		return nil, core.NewError(
			fmt.Errorf("failed to create additional admin: %w", err),
			"BOOTSTRAP_CREATE_ADMIN_FAILED",
			map[string]any{
				"email": email,
				"error": err.Error(),
			},
		)
	}
	apiKey, err := s.factory.GenerateAPIKey(user.ID).Execute(ctx)
	if err != nil {
		return nil, core.NewError(
			fmt.Errorf("failed to generate API key: %w", err),
			"BOOTSTRAP_APIKEY_GENERATION_FAILED",
			map[string]any{
				"user_id": user.ID.String(),
				"error":   err.Error(),
			},
		)
	}
	return &Result{
		UserID: user.ID.String(),
		Email:  user.Email,
		APIKey: apiKey,
	}, nil
}

// createInitialAdminUser creates the first admin user in the system
func (s *Service) createInitialAdminUser(ctx context.Context, email string) (*Result, error) {
	bootstrapUC := s.factory.BootstrapSystem(email)
	user, apiKey, err := bootstrapUC.Execute(ctx)
	if err != nil {
		// Check if it's a structured error we should preserve
		var coreErr *core.Error
		if errors.As(err, &coreErr) {
			// Preserve structured errors like ALREADY_BOOTSTRAPPED
			return nil, err
		}
		// For non-structured errors, wrap with service-level context
		return nil, core.NewError(
			fmt.Errorf("failed to bootstrap system: %w", err),
			"BOOTSTRAP_SYSTEM_FAILED",
			map[string]any{
				"email": email,
				"error": err.Error(),
			},
		)
	}
	return &Result{
		UserID: user.ID.String(),
		Email:  user.Email,
		APIKey: apiKey,
	}, nil
}

// CreateInitialAdmin is a one-time operation to create the first admin
// This is called from the server-side during initial setup
func (s *Service) CreateInitialAdmin(ctx context.Context, email string) (*model.User, string, error) {
	if s.factory == nil {
		return nil, "", fmt.Errorf("factory is required for server-side bootstrap")
	}
	log := logger.FromContext(ctx)
	log.Info("Creating initial admin user", "email", email)
	// Check if any admin exists
	status, err := s.CheckBootstrapStatus(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to check bootstrap status: %w", err)
	}
	if status.IsBootstrapped {
		return nil, "", fmt.Errorf("system already has %d admin user(s)", status.AdminCount)
	}
	// Use the factory to create user
	createInput := &uc.CreateUserInput{
		Email: email,
		Role:  model.RoleAdmin,
	}
	createdUser, err := s.factory.CreateUser(createInput).Execute(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create admin user: %w", err)
	}
	// Generate API key
	apiKey, err := s.factory.GenerateAPIKey(createdUser.ID).Execute(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate API key: %w", err)
	}
	return createdUser, apiKey, nil
}
