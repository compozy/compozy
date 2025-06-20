package audit

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// EventType represents the type of audit event
type EventType string

const (
	// API Key events
	EventAPIKeyCreated        EventType = "api_key.created"
	EventAPIKeyValidated      EventType = "api_key.validated"
	EventAPIKeyValidationFail EventType = "api_key.validation_failed" //nolint:gosec // Not a credential
	EventAPIKeyRevoked        EventType = "api_key.revoked"
	EventAPIKeyExpired        EventType = "api_key.expired"
	EventAPIKeyRateLimited    EventType = "api_key.rate_limited" //nolint:gosec // Not a credential

	// Security events
	EventSecurityThreat     EventType = "security.threat_detected"
	EventSecurityBruteForce EventType = "security.brute_force_attempt"
)

// Event represents an audit event
type Event struct {
	ID        core.ID
	Type      EventType
	OrgID     *core.ID
	UserID    *core.ID
	APIKeyID  *core.ID
	IPAddress string
	UserAgent string
	Details   map[string]any
	Timestamp time.Time
}

// Service provides audit logging functionality
type Service struct {
	// In a production system, this would write to a persistent audit log
	// For now, we'll use structured logging
}

// NewService creates a new audit service
func NewService() *Service {
	return &Service{}
}

// LogEvent logs an audit event
func (s *Service) LogEvent(ctx context.Context, event *Event) {
	log := logger.FromContext(ctx)

	// Build log fields
	fields := []any{
		"audit_event_id", event.ID,
		"event_type", event.Type,
		"timestamp", event.Timestamp,
	}

	if event.OrgID != nil {
		fields = append(fields, "org_id", *event.OrgID)
	}
	if event.UserID != nil {
		fields = append(fields, "user_id", *event.UserID)
	}
	if event.APIKeyID != nil {
		fields = append(fields, "api_key_id", *event.APIKeyID)
	}
	if event.IPAddress != "" {
		fields = append(fields, "ip_address", event.IPAddress)
	}
	if event.UserAgent != "" {
		fields = append(fields, "user_agent", event.UserAgent)
	}

	// Add details
	for k, v := range event.Details {
		fields = append(fields, k, v)
	}

	// Log based on event type severity
	switch event.Type {
	case EventSecurityThreat, EventSecurityBruteForce:
		log.With(fields...).Error("Security audit event")
	case EventAPIKeyValidationFail, EventAPIKeyRateLimited:
		log.With(fields...).Warn("Audit event")
	default:
		log.With(fields...).Info("Audit event")
	}
}

// LogAPIKeyCreated logs an API key creation event
func (s *Service) LogAPIKeyCreated(ctx context.Context, apiKeyID, userID, orgID core.ID, keyName string) {
	id, err := core.NewID()
	if err != nil {
		// In audit logging, we don't want to fail the operation if ID generation fails
		// Use a zero ID as fallback
		id = core.ID("")
	}
	s.LogEvent(ctx, &Event{
		ID:       id,
		Type:     EventAPIKeyCreated,
		OrgID:    &orgID,
		UserID:   &userID,
		APIKeyID: &apiKeyID,
		Details: map[string]any{
			"key_name": keyName,
		},
		Timestamp: time.Now(),
	})
}

// LogAPIKeyValidated logs a successful API key validation
func (s *Service) LogAPIKeyValidated(ctx context.Context, apiKeyID, userID, orgID core.ID) {
	id, err := core.NewID()
	if err != nil {
		// In audit logging, we don't want to fail the operation if ID generation fails
		// Use a zero ID as fallback
		id = core.ID("")
	}
	s.LogEvent(ctx, &Event{
		ID:        id,
		Type:      EventAPIKeyValidated,
		OrgID:     &orgID,
		UserID:    &userID,
		APIKeyID:  &apiKeyID,
		Timestamp: time.Now(),
	})
}

// LogAPIKeyValidationFailed logs a failed API key validation attempt
func (s *Service) LogAPIKeyValidationFailed(
	ctx context.Context,
	reason string,
	ipAddress, userAgent string,
	apiKeyID *core.ID,
) {
	id, err := core.NewID()
	if err != nil {
		// In audit logging, we don't want to fail the operation if ID generation fails
		// Use a zero ID as fallback
		id = core.ID("")
	}
	event := Event{
		ID:        id,
		Type:      EventAPIKeyValidationFail,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]any{
			"reason": reason,
		},
		Timestamp: time.Now(),
	}

	if apiKeyID != nil {
		event.APIKeyID = apiKeyID
	}

	s.LogEvent(ctx, &event)
}

// LogAPIKeyRevoked logs an API key revocation
func (s *Service) LogAPIKeyRevoked(ctx context.Context, apiKeyID, userID, orgID core.ID, revokedBy core.ID) {
	id, err := core.NewID()
	if err != nil {
		// In audit logging, we don't want to fail the operation if ID generation fails
		// Use a zero ID as fallback
		id = core.ID("")
	}
	s.LogEvent(ctx, &Event{
		ID:       id,
		Type:     EventAPIKeyRevoked,
		OrgID:    &orgID,
		UserID:   &userID,
		APIKeyID: &apiKeyID,
		Details: map[string]any{
			"revoked_by": revokedBy,
		},
		Timestamp: time.Now(),
	})
}

// LogAPIKeyExpired logs an expired API key usage attempt
func (s *Service) LogAPIKeyExpired(ctx context.Context, apiKeyID, userID, orgID core.ID, expiredAt time.Time) {
	id, err := core.NewID()
	if err != nil {
		// In audit logging, we don't want to fail the operation if ID generation fails
		// Use a zero ID as fallback
		id = core.ID("")
	}
	s.LogEvent(ctx, &Event{
		ID:       id,
		Type:     EventAPIKeyExpired,
		OrgID:    &orgID,
		UserID:   &userID,
		APIKeyID: &apiKeyID,
		Details: map[string]any{
			"expired_at": expiredAt,
		},
		Timestamp: time.Now(),
	})
}

// LogRateLimitExceeded logs a rate limit exceeded event
func (s *Service) LogRateLimitExceeded(ctx context.Context, apiKeyID, userID, orgID core.ID, limit int) {
	id, err := core.NewID()
	if err != nil {
		// In audit logging, we don't want to fail the operation if ID generation fails
		// Use a zero ID as fallback
		id = core.ID("")
	}
	s.LogEvent(ctx, &Event{
		ID:       id,
		Type:     EventAPIKeyRateLimited,
		OrgID:    &orgID,
		UserID:   &userID,
		APIKeyID: &apiKeyID,
		Details: map[string]any{
			"rate_limit": limit,
		},
		Timestamp: time.Now(),
	})
}

// LogSecurityThreat logs a potential security threat
func (s *Service) LogSecurityThreat(
	ctx context.Context,
	threatType string,
	ipAddress, userAgent string,
	details map[string]any,
) {
	if details == nil {
		details = make(map[string]any)
	}
	details["threat_type"] = threatType

	id, err := core.NewID()
	if err != nil {
		// In audit logging, we don't want to fail the operation if ID generation fails
		// Use a zero ID as fallback
		id = core.ID("")
	}
	s.LogEvent(ctx, &Event{
		ID:        id,
		Type:      EventSecurityThreat,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   details,
		Timestamp: time.Now(),
	})
}

// LogBruteForceAttempt logs a potential brute force attempt
func (s *Service) LogBruteForceAttempt(ctx context.Context, ipAddress string, attempts int) {
	id, err := core.NewID()
	if err != nil {
		// In audit logging, we don't want to fail the operation if ID generation fails
		// Use a zero ID as fallback
		id = core.ID("")
	}
	s.LogEvent(ctx, &Event{
		ID:        id,
		Type:      EventSecurityBruteForce,
		IPAddress: ipAddress,
		Details: map[string]any{
			"attempts": attempts,
		},
		Timestamp: time.Now(),
	})
}
