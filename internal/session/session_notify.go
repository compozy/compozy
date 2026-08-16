package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	compozyconfig "github.com/compozy/compozy/internal/config"
	redactpkg "github.com/compozy/compozy/internal/redact"
)

const (
	NotificationTitleMaxChars = 80
	NotificationBodyMaxChars  = 240
	NotificationRateWindow    = time.Second
)

var ErrInvalidNotification = errors.New("session: invalid operator notification")

type NotifyOutcome string

const (
	NotifyOutcomeDelivered      NotifyOutcome = "delivered"
	NotifyOutcomeNoClient       NotifyOutcome = "no-client"
	NotifyOutcomeMutedWorkspace NotifyOutcome = "muted-workspace"
	NotifyOutcomeRateLimited    NotifyOutcome = "rate-limited"
)

type NotifyRequest struct {
	SessionID   string
	WorkspaceID string
	Title       string
	Body        string
}

type NotifyResult struct {
	Outcome      NotifyOutcome
	RetryAfterMS int64
}

type OperatorNotification struct {
	NotificationID string
	SessionID      string
	WorkspaceID    string
	Title          string
	Body           string
	At             time.Time
}

// SetAttentionConfig applies the live notification policy.
func (m *Manager) SetAttentionConfig(cfg compozyconfig.AttentionConfig) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("session: validate attention config: %w", err)
	}
	cloned := cfg
	cloned.MutedWorkspaces = append([]string(nil), cfg.MutedWorkspaces...)
	m.notifyMu.Lock()
	m.notifyConfig = cloned
	m.notifyMu.Unlock()
	return nil
}

// PublishOperatorNotification validates, rate-limits, and publishes one sanitized event.
func (m *Manager) PublishOperatorNotification(
	ctx context.Context,
	req NotifyRequest,
) (NotifyResult, error) {
	if m == nil {
		return NotifyResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return NotifyResult{}, errors.New("session: notification context is required")
	}
	if err := ctx.Err(); err != nil {
		return NotifyResult{}, fmt.Errorf("session: publish operator notification: %w", err)
	}
	notification, err := m.sanitizedOperatorNotification(req)
	if err != nil {
		return NotifyResult{}, err
	}

	m.notifyMu.Lock()
	if retryAfter := m.notificationRetryAfterLocked(notification.SessionID, notification.At); retryAfter > 0 {
		m.notifyMu.Unlock()
		result := NotifyResult{
			Outcome: NotifyOutcomeRateLimited, RetryAfterMS: retryAfter.Milliseconds(),
		}
		m.logNotificationOutcome(ctx, notification, result, 0)
		return result, nil
	}
	m.notifyLastBySession[notification.SessionID] = notification.At
	muted := notificationWorkspaceMuted(m.notifyConfig.MutedWorkspaces, notification.WorkspaceID)
	m.notifyMu.Unlock()
	if muted {
		result := NotifyResult{Outcome: NotifyOutcomeMutedWorkspace}
		m.logNotificationOutcome(ctx, notification, result, 0)
		return result, nil
	}

	delivered := m.publishSessionCatalogEventCount(CatalogEvent{
		Name:                 CatalogEventNameOperatorNotification,
		WorkspaceID:          notification.WorkspaceID,
		SessionID:            notification.SessionID,
		OperatorNotification: &notification,
	})
	if delivered == 0 {
		result := NotifyResult{Outcome: NotifyOutcomeNoClient}
		m.logNotificationOutcome(ctx, notification, result, 0)
		return result, nil
	}
	result := NotifyResult{Outcome: NotifyOutcomeDelivered}
	m.logNotificationOutcome(ctx, notification, result, delivered)
	return result, nil
}

func (m *Manager) sanitizedOperatorNotification(req NotifyRequest) (OperatorNotification, error) {
	sessionID := strings.TrimSpace(req.SessionID)
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if sessionID == "" {
		return OperatorNotification{}, invalidNotification("session_id", "is required")
	}
	if workspaceID == "" {
		return OperatorNotification{}, invalidNotification("workspace_id", "is required")
	}
	title := sanitizeNotificationText(req.Title)
	body := sanitizeNotificationText(req.Body)
	if title == "" {
		return OperatorNotification{}, invalidNotification("title", "is required")
	}
	if utf8.RuneCountInString(title) > NotificationTitleMaxChars {
		return OperatorNotification{}, invalidNotification(
			"title", fmt.Sprintf("must be at most %d characters", NotificationTitleMaxChars),
		)
	}
	if utf8.RuneCountInString(body) > NotificationBodyMaxChars {
		return OperatorNotification{}, invalidNotification(
			"body", fmt.Sprintf("must be at most %d characters", NotificationBodyMaxChars),
		)
	}
	notificationID, err := m.newNotificationID()
	if err != nil {
		return OperatorNotification{}, fmt.Errorf("session: generate operator notification id: %w", err)
	}
	notificationID = strings.TrimSpace(notificationID)
	if notificationID == "" {
		return OperatorNotification{}, errors.New("session: generated operator notification id is empty")
	}
	return OperatorNotification{
		NotificationID: notificationID,
		SessionID:      sessionID, WorkspaceID: workspaceID,
		Title: title, Body: body, At: m.now().UTC(),
	}, nil
}

func (m *Manager) notificationRetryAfterLocked(sessionID string, now time.Time) time.Duration {
	last, exists := m.notifyLastBySession[sessionID]
	if !exists {
		return 0
	}
	retryAt := last.Add(NotificationRateWindow)
	if !now.Before(retryAt) {
		return 0
	}
	retryAfter := retryAt.Sub(now)
	return ((retryAfter + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
}

func sanitizeNotificationText(value string) string {
	return strings.Join(strings.Fields(redactpkg.String(value)), " ")
}

func (m *Manager) logNotificationOutcome(
	ctx context.Context,
	notification OperatorNotification,
	result NotifyResult,
	deliveredSubscribers int,
) {
	logger := m.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.InfoContext(ctx, "session operator notification",
		"session_id", notification.SessionID,
		"workspace_id", notification.WorkspaceID,
		"outcome", result.Outcome,
		"retry_after_ms", result.RetryAfterMS,
		"delivered_subscribers", deliveredSubscribers,
	)
}

func notificationWorkspaceMuted(workspaceIDs []string, workspaceID string) bool {
	for _, candidate := range workspaceIDs {
		if strings.TrimSpace(candidate) == workspaceID {
			return true
		}
	}
	return false
}

func invalidNotification(field string, detail string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidNotification, field, detail)
}
