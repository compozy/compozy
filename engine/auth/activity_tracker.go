package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// ActivityType represents the type of user activity
type ActivityType string

const (
	// User activities
	ActivityUserCreated     ActivityType = "user.created"
	ActivityUserUpdated     ActivityType = "user.updated"
	ActivityUserDeleted     ActivityType = "user.deleted"
	ActivityUserSuspended   ActivityType = "user.suspended"
	ActivityUserActivated   ActivityType = "user.activated"
	ActivityUserRoleChanged ActivityType = "user.role_changed"
	// API key activities
	ActivityAPIKeyCreated ActivityType = "apikey.created"
	ActivityAPIKeyRevoked ActivityType = "apikey.revoked"
	ActivityAPIKeyUsed    ActivityType = "apikey.used"
	// Organization activities
	ActivityOrgCreated   ActivityType = "org.created"
	ActivityOrgUpdated   ActivityType = "org.updated"
	ActivityOrgSuspended ActivityType = "org.suspended"
	// Authentication activities
	ActivityAuthSuccess      ActivityType = "auth.success"
	ActivityAuthFailed       ActivityType = "auth.failed"
	ActivityPermissionDenied ActivityType = "auth.permission_denied"
)

// Activity represents a user activity event
type Activity struct {
	ID         core.ID        `json:"id"          db:"id"`
	OrgID      core.ID        `json:"org_id"      db:"org_id"`
	UserID     core.ID        `json:"user_id"     db:"user_id"`
	Type       ActivityType   `json:"type"        db:"type"`
	ResourceID *core.ID       `json:"resource_id" db:"resource_id"`
	Details    map[string]any `json:"details"     db:"details"`
	IPAddress  string         `json:"ip_address"  db:"ip_address"`
	UserAgent  string         `json:"user_agent"  db:"user_agent"`
	CreatedAt  time.Time      `json:"created_at"  db:"created_at"`
}

// ActivityTracker handles user activity tracking and audit logging
type ActivityTracker struct {
	// In a production system, this would write to a database
	// For now, we'll use structured logging
}

// NewActivityTracker creates a new activity tracker instance
func NewActivityTracker() *ActivityTracker {
	return &ActivityTracker{}
}

// TrackActivity records a user activity event
func (t *ActivityTracker) TrackActivity(ctx context.Context, activity *Activity) error {
	log := logger.FromContext(ctx)
	// Generate ID if not provided
	if activity.ID == "" {
		id, err := core.NewID()
		if err != nil {
			return fmt.Errorf("failed to generate activity ID: %w", err)
		}
		activity.ID = id
	}
	// Set timestamp
	if activity.CreatedAt.IsZero() {
		activity.CreatedAt = time.Now().UTC()
	}
	// Log the activity with structured fields
	log.With(
		"activity_id", activity.ID,
		"org_id", activity.OrgID,
		"user_id", activity.UserID,
		"activity_type", activity.Type,
		"resource_id", activity.ResourceID,
		"ip_address", activity.IPAddress,
		"user_agent", activity.UserAgent,
		"details", activity.Details,
	).Info("User activity tracked")
	// In a production system, this would also:
	// - Write to an audit log database table
	// - Send to a SIEM system
	// - Trigger alerts for suspicious activities
	return nil
}

// TrackUserCreated tracks user creation activity
func (t *ActivityTracker) TrackUserCreated(
	ctx context.Context,
	orgID, creatorID, newUserID core.ID,
	email string,
	role string,
) error {
	return t.TrackActivity(ctx, &Activity{
		OrgID:      orgID,
		UserID:     creatorID,
		Type:       ActivityUserCreated,
		ResourceID: &newUserID,
		Details: map[string]any{
			"email": email,
			"role":  role,
		},
	})
}

// TrackUserUpdated tracks user update activity
func (t *ActivityTracker) TrackUserUpdated(
	ctx context.Context,
	orgID, updaterID, targetUserID core.ID,
	changes map[string]any,
) error {
	return t.TrackActivity(ctx, &Activity{
		OrgID:      orgID,
		UserID:     updaterID,
		Type:       ActivityUserUpdated,
		ResourceID: &targetUserID,
		Details:    changes,
	})
}

// TrackUserDeleted tracks user deletion activity
func (t *ActivityTracker) TrackUserDeleted(ctx context.Context, orgID, deleterID, targetUserID core.ID) error {
	return t.TrackActivity(ctx, &Activity{
		OrgID:      orgID,
		UserID:     deleterID,
		Type:       ActivityUserDeleted,
		ResourceID: &targetUserID,
		Details:    nil,
	})
}

// TrackUserStatusChange tracks user status change activity
func (t *ActivityTracker) TrackUserStatusChange(
	ctx context.Context,
	orgID, changerID, targetUserID core.ID,
	oldStatus, newStatus string,
) error {
	activityType := ActivityUserUpdated
	if newStatus == string(user.StatusSuspended) {
		activityType = ActivityUserSuspended
	} else if newStatus == string(user.StatusActive) && oldStatus == string(user.StatusSuspended) {
		activityType = ActivityUserActivated
	}
	return t.TrackActivity(ctx, &Activity{
		OrgID:      orgID,
		UserID:     changerID,
		Type:       activityType,
		ResourceID: &targetUserID,
		Details: map[string]any{
			"old_status": oldStatus,
			"new_status": newStatus,
		},
	})
}

// TrackUserRoleChange tracks user role change activity
func (t *ActivityTracker) TrackUserRoleChange(
	ctx context.Context,
	orgID, changerID, targetUserID core.ID,
	oldRole, newRole string,
) error {
	return t.TrackActivity(ctx, &Activity{
		OrgID:      orgID,
		UserID:     changerID,
		Type:       ActivityUserRoleChanged,
		ResourceID: &targetUserID,
		Details: map[string]any{
			"old_role": oldRole,
			"new_role": newRole,
		},
	})
}

// TrackAPIKeyCreated tracks API key creation activity
func (t *ActivityTracker) TrackAPIKeyCreated(
	ctx context.Context,
	orgID, creatorID, keyID core.ID,
	keyName string,
) error {
	return t.TrackActivity(ctx, &Activity{
		OrgID:      orgID,
		UserID:     creatorID,
		Type:       ActivityAPIKeyCreated,
		ResourceID: &keyID,
		Details: map[string]any{
			"key_name": keyName,
		},
	})
}

// TrackAPIKeyRevoked tracks API key revocation activity
func (t *ActivityTracker) TrackAPIKeyRevoked(
	ctx context.Context,
	orgID, revokerID, keyID core.ID,
	reason string,
) error {
	return t.TrackActivity(ctx, &Activity{
		OrgID:      orgID,
		UserID:     revokerID,
		Type:       ActivityAPIKeyRevoked,
		ResourceID: &keyID,
		Details: map[string]any{
			"reason": reason,
		},
	})
}

// TrackAuthSuccess tracks successful authentication
func (t *ActivityTracker) TrackAuthSuccess(ctx context.Context, orgID, userID core.ID, method string) error {
	return t.TrackActivity(ctx, &Activity{
		OrgID:  orgID,
		UserID: userID,
		Type:   ActivityAuthSuccess,
		Details: map[string]any{
			"method": method,
		},
	})
}

// TrackAuthFailed tracks failed authentication attempt
func (t *ActivityTracker) TrackAuthFailed(ctx context.Context, orgID core.ID, email, reason string) error {
	// For failed auth, we might not have a user ID
	emptyUserID := core.ID("")
	return t.TrackActivity(ctx, &Activity{
		OrgID:  orgID,
		UserID: emptyUserID,
		Type:   ActivityAuthFailed,
		Details: map[string]any{
			"email":  email,
			"reason": reason,
		},
	})
}

// TrackPermissionDenied tracks permission denied events
func (t *ActivityTracker) TrackPermissionDenied(
	ctx context.Context,
	orgID, userID core.ID,
	permission, resource string,
) error {
	return t.TrackActivity(ctx, &Activity{
		OrgID:  orgID,
		UserID: userID,
		Type:   ActivityPermissionDenied,
		Details: map[string]any{
			"permission": permission,
			"resource":   resource,
		},
	})
}
