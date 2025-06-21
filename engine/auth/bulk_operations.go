package auth

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// BulkOperationResult represents the result of a bulk operation
type BulkOperationResult struct {
	TotalUsers      int             `json:"total_users"`
	SuccessfulUsers int             `json:"successful_users"`
	FailedUsers     int             `json:"failed_users"`
	Errors          []BulkUserError `json:"errors,omitempty"`
}

// BulkUserError represents an error for a specific user in bulk operation
type BulkUserError struct {
	UserID core.ID `json:"user_id"`
	Error  string  `json:"error"`
}

// BulkOperationsService handles bulk user operations
type BulkOperationsService struct {
	userService     *user.Service
	activityTracker *ActivityTracker
}

// NewBulkOperationsService creates a new bulk operations service
func NewBulkOperationsService(userService *user.Service, activityTracker *ActivityTracker) *BulkOperationsService {
	return &BulkOperationsService{
		userService:     userService,
		activityTracker: activityTracker,
	}
}

// SuspendUsers suspends multiple users in bulk
func (s *BulkOperationsService) SuspendUsers(
	ctx context.Context,
	orgID, executorID core.ID,
	userIDs []core.ID,
	reason string,
) (*BulkOperationResult, error) {
	log := logger.FromContext(ctx)
	log.With(
		"org_id", orgID,
		"executor_id", executorID,
		"user_count", len(userIDs),
		"operation", "suspend",
	).Info("Executing bulk user suspension")
	result := &BulkOperationResult{
		TotalUsers: len(userIDs),
		Errors:     []BulkUserError{},
	}
	// Get current user states before operation
	userStates := make(map[core.ID]user.Status)
	for _, userID := range userIDs {
		if u, err := s.userService.GetUser(ctx, orgID, userID); err == nil {
			userStates[userID] = u.Status
		}
	}
	// Execute bulk operation using user service
	errs, err := s.userService.ExecuteBulkOperation(ctx, orgID, &user.BulkUserOperation{
		UserIDs:   userIDs,
		Operation: "suspend",
	})
	if err != nil {
		// If the entire operation failed, all users failed
		result.FailedUsers = result.TotalUsers
		return result, fmt.Errorf("bulk suspension failed: %w", err)
	}
	// Process per-user errors
	failedUsers := make(map[core.ID]bool)
	for _, e := range errs {
		result.FailedUsers++
		result.Errors = append(result.Errors, BulkUserError{
			UserID: e.UserID,
			Error:  e.Error.Error(),
		})
		failedUsers[e.UserID] = true
	}
	result.SuccessfulUsers = result.TotalUsers - result.FailedUsers
	// Track activity for successful operations
	for _, userID := range userIDs {
		if !failedUsers[userID] {
			oldStatus := "active" // default
			if status, ok := userStates[userID]; ok {
				oldStatus = string(status)
			}
			if trackErr := s.activityTracker.TrackUserStatusChange(ctx, orgID, executorID, userID, oldStatus, "suspended"); trackErr != nil {
				log.With("user_id", userID, "error", trackErr).Warn("Failed to track user suspension activity")
			}
		}
	}
	return result, nil
}

// ActivateUsers activates multiple suspended users in bulk
func (s *BulkOperationsService) ActivateUsers(
	ctx context.Context,
	orgID, executorID core.ID,
	userIDs []core.ID,
) (*BulkOperationResult, error) {
	log := logger.FromContext(ctx)
	log.With(
		"org_id", orgID,
		"executor_id", executorID,
		"user_count", len(userIDs),
		"operation", "activate",
	).Info("Executing bulk user activation")
	result := &BulkOperationResult{
		TotalUsers: len(userIDs),
		Errors:     []BulkUserError{},
	}
	// Get current user states before operation
	userStates := make(map[core.ID]user.Status)
	for _, userID := range userIDs {
		if u, err := s.userService.GetUser(ctx, orgID, userID); err == nil {
			userStates[userID] = u.Status
		}
	}
	// Execute bulk operation using user service
	errs, err := s.userService.ExecuteBulkOperation(ctx, orgID, &user.BulkUserOperation{
		UserIDs:   userIDs,
		Operation: "activate",
	})
	if err != nil {
		// If the entire operation failed, all users failed
		result.FailedUsers = result.TotalUsers
		return result, fmt.Errorf("bulk activation failed: %w", err)
	}
	// Process per-user errors
	failedUsers := make(map[core.ID]bool)
	for _, e := range errs {
		result.FailedUsers++
		result.Errors = append(result.Errors, BulkUserError{
			UserID: e.UserID,
			Error:  e.Error.Error(),
		})
		failedUsers[e.UserID] = true
	}
	result.SuccessfulUsers = result.TotalUsers - result.FailedUsers
	// Track activity for successful operations
	for _, userID := range userIDs {
		if !failedUsers[userID] {
			oldStatus := "suspended" // default
			if status, ok := userStates[userID]; ok {
				oldStatus = string(status)
			}
			if trackErr := s.activityTracker.TrackUserStatusChange(ctx, orgID, executorID, userID, oldStatus, "active"); trackErr != nil {
				log.With("user_id", userID, "error", trackErr).Warn("Failed to track user activation activity")
			}
		}
	}
	return result, nil
}

// DeleteUsers deletes multiple users in bulk
func (s *BulkOperationsService) DeleteUsers(
	ctx context.Context,
	orgID, executorID core.ID,
	userIDs []core.ID,
) (*BulkOperationResult, error) {
	log := logger.FromContext(ctx)
	log.With(
		"org_id", orgID,
		"executor_id", executorID,
		"user_count", len(userIDs),
		"operation", "delete",
	).Info("Executing bulk user deletion")
	result := &BulkOperationResult{
		TotalUsers: len(userIDs),
		Errors:     []BulkUserError{},
	}
	// Execute bulk operation using user service
	errs, err := s.userService.ExecuteBulkOperation(ctx, orgID, &user.BulkUserOperation{
		UserIDs:   userIDs,
		Operation: "delete",
	})
	if err != nil {
		// If the entire operation failed, all users failed
		result.FailedUsers = result.TotalUsers
		return result, fmt.Errorf("bulk deletion failed: %w", err)
	}
	// Process per-user errors
	failedUsers := make(map[core.ID]bool)
	for _, e := range errs {
		result.FailedUsers++
		result.Errors = append(result.Errors, BulkUserError{
			UserID: e.UserID,
			Error:  e.Error.Error(),
		})
		failedUsers[e.UserID] = true
	}
	result.SuccessfulUsers = result.TotalUsers - result.FailedUsers
	// Track activity for successful deletions
	for _, userID := range userIDs {
		if !failedUsers[userID] {
			if trackErr := s.activityTracker.TrackUserDeleted(ctx, orgID, executorID, userID); trackErr != nil {
				log.With("user_id", userID, "error", trackErr).Warn("Failed to track user deletion activity")
			}
		}
	}
	return result, nil
}

// UpdateUserRoles updates roles for multiple users in bulk
func (s *BulkOperationsService) UpdateUserRoles(
	ctx context.Context,
	orgID, executorID core.ID,
	updates []UserRoleUpdate,
) (*BulkOperationResult, error) {
	log := logger.FromContext(ctx)
	log.With(
		"org_id", orgID,
		"executor_id", executorID,
		"user_count", len(updates),
		"operation", "update_roles",
	).Info("Executing bulk user role update")
	result := &BulkOperationResult{
		TotalUsers: len(updates),
		Errors:     []BulkUserError{},
	}
	// Process each user individually to handle errors gracefully
	for _, update := range updates {
		// Get current user to track old role
		currentUser, err := s.userService.GetUser(ctx, orgID, update.UserID)
		if err != nil {
			result.FailedUsers++
			result.Errors = append(result.Errors, BulkUserError{
				UserID: update.UserID,
				Error:  fmt.Sprintf("failed to get user: %v", err),
			})
			continue
		}
		// Update user role
		err = s.userService.UpdateUserRole(ctx, orgID, update.UserID, update.NewRole)
		if err != nil {
			result.FailedUsers++
			result.Errors = append(result.Errors, BulkUserError{
				UserID: update.UserID,
				Error:  fmt.Sprintf("failed to update role: %v", err),
			})
			continue
		}
		result.SuccessfulUsers++
		// Track activity
		if trackErr := s.activityTracker.TrackUserRoleChange(ctx, orgID, executorID, update.UserID, string(currentUser.Role), string(update.NewRole)); trackErr != nil {
			log.With("user_id", update.UserID, "error", trackErr).Warn("Failed to track user role change activity")
		}
	}
	return result, nil
}

// UserRoleUpdate represents a user role update request
type UserRoleUpdate struct {
	UserID  core.ID   `json:"user_id"  validate:"required"`
	NewRole user.Role `json:"new_role" validate:"required"`
}
