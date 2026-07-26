package automation

import (
	"context"

	"errors"
	"fmt"
	"log/slog"

	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/session"
)

const (
	triggerEventWebhook = "webhook"
)

const (
	triggerACPSessionIDKey  = "acp_session_id"
	triggerAgentNameKey     = "agent_name"
	triggerCompletedAtKey   = "completed_at"
	triggerCreatedAtKey     = "created_at"
	triggerDispatchDepthKey = "dispatch_depth"
	triggerErrorKey         = "error"
	triggerHookEventKey     = "hook_event"
	triggerHookModeKey      = "hook_mode"
	triggerHookNameKey      = "hook_name"
	triggerHookOutcomeKey   = "hook_outcome"
	triggerHookSourceKey    = "hook_source"
	triggerRecordedAtKey    = "recorded_at"
	triggerRequiredKey      = "required"
	triggerSessionIDKey     = "session_id"
	triggerSessionNameKey   = "session_name"
	triggerSessionTypeKey   = "session_type"
	triggerStateKey         = "state"
	triggerStopDetailKey    = "stop_detail"
	triggerStopReasonKey    = "stop_reason"
	triggerUpdatedAtKey     = "updated_at"
	triggerWorkspaceKey     = "workspace"
	triggerWorkspaceIDKey   = "workspace_id"
)

var (
	// ErrTriggerAlreadyRegistered reports that the trigger id already exists in the runtime.
	ErrTriggerAlreadyRegistered = errors.New("automation: trigger already registered")
	// ErrTriggerEngineStopped reports that the trigger engine has already been stopped.
	ErrTriggerEngineStopped = errors.New("automation: trigger engine stopped")
	// ErrWebhookEndpointInvalid reports that a webhook endpoint value cannot be normalized.
	ErrWebhookEndpointInvalid = errors.New("automation: invalid webhook endpoint")
	// ErrWebhookTriggerNotRegistered reports that no runtime webhook registration matches the endpoint id.
	ErrWebhookTriggerNotRegistered = errors.New("automation: webhook trigger not registered")
	// ErrWebhookTimestampInvalid reports that a webhook timestamp is outside the accepted freshness window.
	ErrWebhookTimestampInvalid = errors.New("automation: webhook timestamp outside freshness window")
	// ErrWebhookSignatureInvalid reports that a webhook signature does not match the expected HMAC.
	ErrWebhookSignatureInvalid = errors.New("automation: webhook signature invalid")
	// ErrWebhookSecretRequired reports that a webhook registration did not provide auth material.
	ErrWebhookSecretRequired = errors.New("automation: webhook secret is required")
	// ErrWebhookReplayDetected reports that the same authenticated delivery id
	// was already processed within the replay window.
	ErrWebhookReplayDetected = errors.New("automation: webhook delivery already processed")
)

const (
	webhookSignaturePrefix = "sha256="
	sessionEventCreated    = "session.created"
	sessionEventStopped    = "session.stopped"
)

// DefaultWebhookFreshnessWindow is the default accepted clock skew for webhook requests.
const DefaultWebhookFreshnessWindow = 5 * time.Minute

// TriggerDispatcher is the shared execution surface used by matched triggers.
type TriggerDispatcher interface {
	Dispatch(ctx context.Context, req DispatchRequest) (*Run, error)
}

// WebhookDeliveryStore persists authenticated delivery claims across trigger-engine restarts.
type WebhookDeliveryStore interface {
	CreateRun(ctx context.Context, run Run) (Run, error)
	GetRun(ctx context.Context, id string) (Run, error)
	DeleteRun(ctx context.Context, id string) error
}

// HookSessionResolver resolves session metadata for hook-completion ingress.
type HookSessionResolver interface {
	Status(ctx context.Context, id string) (*session.Info, error)
}

// TriggerEngineOption customizes trigger runtime behavior.
type TriggerEngineOption func(*TriggerEngine)

// TriggerResult reports how many triggers matched one activation and which runs were created.
type TriggerResult struct {
	Matched int   `json:"matched"`
	Runs    []Run `json:"runs,omitempty"`
}

// TriggerRegistration stores one runtime trigger definition plus write-only webhook auth material.
type TriggerRegistration struct {
	Trigger        Trigger `json:"trigger"`
	compiledFilter triggerFilter
}

// Validate ensures the runtime registration is internally consistent.
func (r TriggerRegistration) Validate(path string) error {
	if err := r.Trigger.Validate(nestedPath(path, "trigger")); err != nil {
		return err
	}

	if strings.TrimSpace(r.Trigger.Event) != triggerEventWebhook {
		return nil
	}
	if strings.TrimSpace(r.Trigger.WebhookID) == "" {
		return fmt.Errorf(
			"%s is required when trigger.event is %q",
			nestedPath(path, "trigger.webhook_id"),
			triggerEventWebhook,
		)
	}
	return nil
}

// ParsedWebhookEndpoint is the normalized webhook endpoint split into slug and stable webhook id.
type ParsedWebhookEndpoint struct {
	EndpointSlug string `json:"endpoint_slug"`
	WebhookID    string `json:"webhook_id"`
}

// WebhookRequest is the transport-neutral webhook delivery input consumed by the trigger engine.
type WebhookRequest struct {
	Scope       Scope          `json:"scope"`
	WorkspaceID string         `json:"workspace_id,omitempty"`
	Endpoint    string         `json:"endpoint"`
	DeliveryID  string         `json:"delivery_id"`
	Timestamp   time.Time      `json:"timestamp"`
	Signature   string         `json:"signature"`
	Payload     []byte         `json:"payload,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

// Validate ensures the request can be normalized before any dispatch occurs.
func (r WebhookRequest) Validate(path string) error {
	if err := ValidateScopeBinding(r.Scope, r.WorkspaceID, path, triggerWorkspaceIDKey); err != nil {
		return err
	}
	if strings.TrimSpace(r.Endpoint) == "" {
		return errors.New(nestedPath(path, "endpoint") + " is required")
	}
	if strings.TrimSpace(r.DeliveryID) == "" {
		return errors.New(nestedPath(path, "delivery_id") + " is required")
	}
	if r.Timestamp.IsZero() {
		return errors.New(nestedPath(path, "timestamp") + " is required")
	}
	if strings.TrimSpace(r.Signature) == "" {
		return errors.New(nestedPath(path, "signature") + " is required")
	}
	return nil
}

// MemoryConsolidatedEvent is the observer-facing completion payload used for normalized memory ingress.
type MemoryConsolidatedEvent struct {
	WorkspaceID string         `json:"workspace_id,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
	Data        map[string]any `json:"data,omitempty"`
}

// MemoryConsolidationObserver receives dream consolidation completions at the trigger-engine boundary.
type MemoryConsolidationObserver interface {
	OnMemoryConsolidated(ctx context.Context, event MemoryConsolidatedEvent) error
}

// TriggerEngine matches normalized activations against registered triggers and dispatches runs.
type TriggerEngine struct {
	dispatcher TriggerDispatcher
	logger     *slog.Logger
	now        func() time.Time

	webhookFreshnessWindow time.Duration
	hookSessions           HookSessionResolver
	webhookSecrets         WebhookSecretResolver
	webhookDeliveries      WebhookDeliveryStore

	mu            sync.RWMutex
	stopped       bool
	registrations map[string]TriggerRegistration
	webhookIndex  map[string]string
	deliveries    map[string]time.Time
}

// NewTriggerEngine constructs a trigger runtime over the shared dispatcher path.
func NewTriggerEngine(dispatcher TriggerDispatcher, opts ...TriggerEngineOption) (*TriggerEngine, error) {
	if dispatcher == nil {
		return nil, errors.New("automation: trigger dispatcher is required")
	}

	engine := &TriggerEngine{
		dispatcher:             dispatcher,
		logger:                 slog.Default(),
		now:                    func() time.Time { return time.Now().UTC() },
		webhookFreshnessWindow: DefaultWebhookFreshnessWindow,
		registrations:          make(map[string]TriggerRegistration),
		webhookIndex:           make(map[string]string),
		deliveries:             make(map[string]time.Time),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(engine)
		}
	}

	if engine.logger == nil {
		engine.logger = slog.Default()
	}
	if engine.now == nil {
		engine.now = func() time.Time { return time.Now().UTC() }
	}
	if engine.webhookFreshnessWindow <= 0 {
		engine.webhookFreshnessWindow = DefaultWebhookFreshnessWindow
	}
	if engine.registrations == nil {
		engine.registrations = make(map[string]TriggerRegistration)
	}
	if engine.webhookIndex == nil {
		engine.webhookIndex = make(map[string]string)
	}
	if engine.deliveries == nil {
		engine.deliveries = make(map[string]time.Time)
	}

	return engine, nil
}

// WithTriggerEngineLogger overrides the trigger-engine logger.
func WithTriggerEngineLogger(logger *slog.Logger) TriggerEngineOption {
	return func(engine *TriggerEngine) {
		engine.logger = logger
	}
}

// WithTriggerEngineNow overrides the trigger-engine clock.
func WithTriggerEngineNow(now func() time.Time) TriggerEngineOption {
	return func(engine *TriggerEngine) {
		engine.now = now
	}
}

// WithTriggerEngineWebhookFreshnessWindow overrides the accepted webhook clock skew.
func WithTriggerEngineWebhookFreshnessWindow(window time.Duration) TriggerEngineOption {
	return func(engine *TriggerEngine) {
		engine.webhookFreshnessWindow = window
	}
}

// WithTriggerEngineHookSessionResolver injects session lookup support for hook-completion ingress.
func WithTriggerEngineHookSessionResolver(resolver HookSessionResolver) TriggerEngineOption {
	return func(engine *TriggerEngine) {
		engine.hookSessions = resolver
	}
}

// WithTriggerEngineWebhookSecretResolver injects the vault-backed resolver for webhook auth refs.
func WithTriggerEngineWebhookSecretResolver(resolver WebhookSecretResolver) TriggerEngineOption {
	return func(engine *TriggerEngine) {
		engine.webhookSecrets = resolver
	}
}

// WithTriggerEngineWebhookDeliveryStore injects durable replay protection for webhook delivery IDs.
func WithTriggerEngineWebhookDeliveryStore(store WebhookDeliveryStore) TriggerEngineOption {
	return func(engine *TriggerEngine) {
		engine.webhookDeliveries = store
	}
}
