package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5"
)

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  Role   `json:"role"  validate:"required"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Email  *string `json:"email,omitempty"  validate:"omitempty,email"`
	Role   *Role   `json:"role,omitempty"`
	Status *Status `json:"status,omitempty"`
}

// BulkUserOperation represents a bulk operation on users
type BulkUserOperation struct {
	UserIDs   []core.ID `json:"user_ids"  validate:"required,min=1"`
	Operation string    `json:"operation" validate:"required,oneof=suspend activate delete"`
}

// BulkOperationError represents an error for a specific user during bulk operation
type BulkOperationError struct {
	UserID core.ID
	Error  error
}

// Service provides user lifecycle management with role-based permissions
type Service struct {
	repo Repository
	db   store.DBInterface
}

// NewService creates a new user service instance
func NewService(repo Repository, db store.DBInterface) *Service {
	return &Service{
		repo: repo,
		db:   db,
	}
}

// CreateUser creates a new user within an organization
func (s *Service) CreateUser(ctx context.Context, orgID core.ID, req *CreateUserRequest) (*User, error) {
	log := logger.FromContext(ctx)
	// Validate request
	if err := s.validateCreateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	// Check if user already exists with same email in organization
	existingUser, err := s.repo.GetByEmail(ctx, orgID, req.Email)
	if err != nil && err != ErrUserNotFound {
		return nil, fmt.Errorf("failed to check user uniqueness: %w", err)
	}
	if existingUser != nil {
		return nil, fmt.Errorf("user with email '%s' already exists in organization", req.Email)
	}
	// Create user entity
	user, err := NewUser(orgID, req.Email, req.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to create user entity: %w", err)
	}
	log.With(
		"user_id", user.ID,
		"org_id", orgID,
		"role", user.Role,
	).Info("Creating user")
	// Create user in database
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	log.With("user_id", user.ID).Info("User created successfully")
	return user, nil
}

// GetUser retrieves a user by ID within an organization
func (s *Service) GetUser(ctx context.Context, orgID, userID core.ID) (*User, error) {
	return s.repo.GetByID(ctx, orgID, userID)
}

// GetUserByEmail retrieves a user by email within an organization
func (s *Service) GetUserByEmail(ctx context.Context, orgID core.ID, email string) (*User, error) {
	return s.repo.GetByEmail(ctx, orgID, email)
}

// UpdateUser updates an existing user
func (s *Service) UpdateUser(ctx context.Context, orgID, userID core.ID, req *UpdateUserRequest) (*User, error) {
	log := logger.FromContext(ctx)
	// Get existing user
	user, err := s.repo.GetByID(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Track if any changes were made
	updated := false
	// Update email if provided
	if req.Email != nil && *req.Email != user.Email {
		// Validate email
		if err := ValidateEmail(*req.Email); err != nil {
			return nil, fmt.Errorf("invalid email: %w", err)
		}
		// Check if email is already taken
		existingUser, err := s.repo.GetByEmail(ctx, orgID, *req.Email)
		if err != nil && err != ErrUserNotFound {
			return nil, fmt.Errorf("failed to check email uniqueness: %w", err)
		}
		if existingUser != nil && existingUser.ID != userID {
			return nil, fmt.Errorf("email '%s' is already taken", *req.Email)
		}
		user.Email = strings.ToLower(strings.TrimSpace(*req.Email))
		updated = true
	}
	// Update role if provided
	if req.Role != nil && *req.Role != user.Role {
		if !req.Role.IsValid() {
			return nil, fmt.Errorf("invalid role: %s", *req.Role)
		}
		user.Role = *req.Role
		updated = true
	}
	// Update status if provided
	if req.Status != nil && *req.Status != user.Status {
		if !req.Status.IsValid() {
			return nil, fmt.Errorf("invalid status: %s", *req.Status)
		}
		user.Status = *req.Status
		updated = true
	}
	// Only update if changes were made
	if !updated {
		return user, nil
	}
	user.UpdatedAt = time.Now().UTC()
	log.With(
		"user_id", userID,
		"org_id", orgID,
		"role", user.Role,
		"status", user.Status,
	).Info("Updating user")
	// Update user in database
	if err := s.repo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}
	log.With("user_id", userID).Info("User updated successfully")
	return user, nil
}

// DeleteUser deletes a user from an organization
func (s *Service) DeleteUser(ctx context.Context, orgID, userID core.ID) error {
	log := logger.FromContext(ctx)
	log.With("user_id", userID, "org_id", orgID).Info("Deleting user")
	// Delete user
	if err := s.repo.Delete(ctx, orgID, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	log.With("user_id", userID).Info("User deleted successfully")
	return nil
}

// ListUsers retrieves users within an organization with pagination
func (s *Service) ListUsers(ctx context.Context, orgID core.ID, limit, offset int) ([]*User, error) {
	return s.repo.List(ctx, orgID, limit, offset)
}

// ListUsersByRole retrieves users by role within an organization
func (s *Service) ListUsersByRole(ctx context.Context, orgID core.ID, role Role, limit, offset int) ([]*User, error) {
	if !role.IsValid() {
		return nil, fmt.Errorf("invalid role: %s", role)
	}
	return s.repo.ListByRole(ctx, orgID, string(role), limit, offset)
}

// UpdateUserRole updates the role of a user
func (s *Service) UpdateUserRole(ctx context.Context, orgID, userID core.ID, role Role) error {
	log := logger.FromContext(ctx)
	if !role.IsValid() {
		return fmt.Errorf("invalid role: %s", role)
	}
	// Check if user exists
	user, err := s.repo.GetByID(ctx, orgID, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	// Don't update if role is the same
	if user.Role == role {
		return nil
	}
	log.With(
		"user_id", userID,
		"org_id", orgID,
		"old_role", user.Role,
		"new_role", role,
	).Info("Updating user role")
	// Update role
	if err := s.repo.UpdateRole(ctx, orgID, userID, string(role)); err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}
	log.With("user_id", userID).Info("User role updated successfully")
	return nil
}

// UpdateUserStatus updates the status of a user
func (s *Service) UpdateUserStatus(ctx context.Context, orgID, userID core.ID, status Status) error {
	log := logger.FromContext(ctx)
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %s", status)
	}
	// Check if user exists
	user, err := s.repo.GetByID(ctx, orgID, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	// Don't update if status is the same
	if user.Status == status {
		return nil
	}
	log.With(
		"user_id", userID,
		"org_id", orgID,
		"old_status", user.Status,
		"new_status", status,
	).Info("Updating user status")
	// Update status
	if err := s.repo.UpdateStatus(ctx, orgID, userID, status); err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}
	log.With("user_id", userID).Info("User status updated successfully")
	return nil
}

// SearchUsersByEmail searches for users by email pattern within an organization
func (s *Service) SearchUsersByEmail(ctx context.Context, orgID core.ID, emailPattern string) ([]*User, error) {
	if emailPattern == "" {
		return nil, fmt.Errorf("email pattern cannot be empty")
	}
	return s.repo.FindByEmail(ctx, orgID, emailPattern)
}

// CountUsers returns the total count of users in an organization
func (s *Service) CountUsers(ctx context.Context, orgID core.ID) (int64, error) {
	return s.repo.CountByOrg(ctx, orgID)
}

// ExecuteBulkOperation performs a bulk operation on multiple users
func (s *Service) ExecuteBulkOperation(
	ctx context.Context,
	orgID core.ID,
	req *BulkUserOperation,
) ([]BulkOperationError, error) {
	log := logger.FromContext(ctx)
	if len(req.UserIDs) == 0 {
		return nil, fmt.Errorf("no user IDs provided")
	}
	// Validate operation before starting transaction
	switch req.Operation {
	case "suspend", "activate", "delete":
		// Valid operations
	default:
		return nil, fmt.Errorf("unsupported operation: %s", req.Operation)
	}
	// De-duplicate user IDs
	uniqueIDs := make(map[core.ID]struct{})
	for _, id := range req.UserIDs {
		uniqueIDs[id] = struct{}{}
	}
	userIDs := make([]core.ID, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		userIDs = append(userIDs, id)
	}
	log.With(
		"org_id", orgID,
		"operation", req.Operation,
		"user_count", len(userIDs),
	).Info("Executing bulk user operation")
	// Execute operation in a transaction for atomicity
	var errors []BulkOperationError
	err := s.withTransaction(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		for _, userID := range userIDs {
			// Verify user exists in organization
			user, err := txRepo.GetByID(txCtx, orgID, userID)
			if err != nil {
				if err == ErrUserNotFound {
					log.With("user_id", userID).Warn("User not found, marking as failed")
					errors = append(errors, BulkOperationError{
						UserID: userID,
						Error:  ErrUserNotFound,
					})
					continue
				}
				return fmt.Errorf("failed to get user %s: %w", userID, err)
			}
			// Execute operation based on type
			switch req.Operation {
			case "suspend":
				if user.Status != StatusSuspended {
					if err := txRepo.UpdateStatus(txCtx, orgID, userID, StatusSuspended); err != nil {
						return fmt.Errorf("failed to suspend user %s: %w", userID, err)
					}
				}
			case "activate":
				if user.Status != StatusActive {
					if err := txRepo.UpdateStatus(txCtx, orgID, userID, StatusActive); err != nil {
						return fmt.Errorf("failed to activate user %s: %w", userID, err)
					}
				}
			case "delete":
				if err := txRepo.Delete(txCtx, orgID, userID); err != nil {
					return fmt.Errorf("failed to delete user %s: %w", userID, err)
				}
			}
		}
		return nil
	})
	return errors, err
}

// CheckPermission checks if a user has a specific permission
func (s *Service) CheckPermission(ctx context.Context, orgID, userID core.ID, permission string) (bool, error) {
	user, err := s.repo.GetByID(ctx, orgID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}
	// Check if user is active
	if user.Status != StatusActive {
		return false, nil
	}
	// Check role permissions
	return user.Role.HasPermission(permission), nil
}

// validateCreateRequest validates the create user request
func (s *Service) validateCreateRequest(req *CreateUserRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if err := ValidateEmail(req.Email); err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}
	if !req.Role.IsValid() {
		return fmt.Errorf("invalid role: %s", req.Role)
	}
	return nil
}

// withTransaction executes a function within a transaction
func (s *Service) withTransaction(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	log := logger.FromContext(ctx)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				log.With("error", rbErr).Error("Failed to rollback transaction after panic")
			}
			panic(p)
		} else if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				err = fmt.Errorf("tx error: %v, rollback error: %w", err, rbErr)
				log.With("error", rbErr).Error("Failed to rollback transaction")
			}
		} else {
			if commitErr := tx.Commit(ctx); commitErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", commitErr)
				log.With("error", commitErr).Error("Failed to commit transaction")
			}
		}
	}()
	err = fn(ctx, tx)
	return err
}
