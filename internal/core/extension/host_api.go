package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// HostAPIService handles one registered Host API namespace.
type HostAPIService interface {
	Handle(ctx context.Context, extension *RuntimeExtension, verb string, params json.RawMessage) (any, error)
}

// HostAPIServiceFunc adapts a function to HostAPIService.
type HostAPIServiceFunc func(
	ctx context.Context,
	extension *RuntimeExtension,
	verb string,
	params json.RawMessage,
) (any, error)

// Handle invokes the function-backed service.
func (f HostAPIServiceFunc) Handle(
	ctx context.Context,
	extension *RuntimeExtension,
	verb string,
	params json.RawMessage,
) (any, error) {
	return f(ctx, extension, verb, params)
}

// HostAPIRouter routes extension -> host calls to registered namespace handlers.
type HostAPIRouter struct {
	registry *Registry
	audit    AuditHandler

	mu       sync.RWMutex
	services map[string]HostAPIService
}

// NewHostAPIRouter constructs a router for one run-scoped extension registry.
func NewHostAPIRouter(registry *Registry, audit AuditHandler) *HostAPIRouter {
	return &HostAPIRouter{
		registry: registry,
		audit:    audit,
		services: make(map[string]HostAPIService),
	}
}

// RegisterService registers one namespace handler. Callers may pass either
// `tasks` or `host.tasks`; the router normalizes both to the same namespace key.
func (r *HostAPIRouter) RegisterService(namespace string, handler HostAPIService) error {
	if r == nil {
		return fmt.Errorf("register host api service: router is nil")
	}
	if handler == nil {
		return fmt.Errorf("register host api service: handler is nil")
	}

	normalized, err := normalizeHostNamespace(namespace)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.services[normalized]; exists {
		return fmt.Errorf("register host api service: duplicate namespace %q", namespace)
	}
	r.services[normalized] = handler
	return nil
}

// Handle routes one Host API request for the named extension session.
func (r *HostAPIRouter) Handle(
	ctx context.Context,
	extensionName string,
	method string,
	params json.RawMessage,
) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	startedAt := time.Now()
	capability, _ := capabilityForHostMethod(method)

	extension, ok := r.registry.Get(extensionName)
	if !ok {
		err := NewNotInitializedError()
		r.recordAudit(extensionName, method, capability, startedAt, err)
		return nil, err
	}

	state := extension.State()
	switch state {
	case ExtensionStateReady:
	case ExtensionStateDraining, ExtensionStateStopped:
		err := NewShutdownInProgressError(extension.ShutdownDeadline())
		r.recordAudit(extension.normalizedName(), method, capability, startedAt, err)
		return nil, err
	default:
		err := NewNotInitializedError()
		r.recordAudit(extension.normalizedName(), method, capability, startedAt, err)
		return nil, err
	}

	namespace, verb, ok := splitHostMethod(method)
	if !ok {
		err := NewMethodNotFoundError(method)
		r.recordAudit(extension.normalizedName(), method, capability, startedAt, err)
		return nil, err
	}

	if err := extension.Capabilities.CheckHostMethod(method); err != nil {
		converted := toRequestError(err, method)
		r.recordAudit(extension.normalizedName(), method, capability, startedAt, converted)
		return nil, converted
	}

	service, ok := r.service(namespace)
	if !ok {
		err := NewMethodNotFoundError(method)
		r.recordAudit(extension.normalizedName(), method, capability, startedAt, err)
		return nil, err
	}

	result, err := service.Handle(ctx, extension, verb, params)
	if err != nil {
		converted := toRequestError(err, method)
		r.recordAudit(extension.normalizedName(), method, capability, startedAt, converted)
		return nil, converted
	}

	r.recordAudit(extension.normalizedName(), method, capability, startedAt, nil)
	return result, nil
}

func (r *HostAPIRouter) service(namespace string) (HostAPIService, bool) {
	if r == nil {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	service, ok := r.services[namespace]
	return service, ok
}

func (r *HostAPIRouter) recordAudit(
	extensionName string,
	method string,
	capability Capability,
	startedAt time.Time,
	err error,
) {
	if r == nil || r.audit == nil {
		return
	}

	recordErr := r.audit.Record(AuditEntry{
		Timestamp:   time.Now().UTC(),
		Extension:   strings.TrimSpace(extensionName),
		Direction:   AuditDirectionExtToHost,
		Method:      strings.TrimSpace(method),
		Capability:  capability,
		Latency:     time.Since(startedAt),
		Result:      auditResultForError(err),
		ErrorDetail: auditErrorDetail(err),
	})
	if recordErr != nil {
		slog.Warn(
			"record host api audit entry",
			"component", "extension.host_api",
			"extension", strings.TrimSpace(extensionName),
			"method", strings.TrimSpace(method),
			"err", recordErr,
		)
	}
}

func normalizeHostNamespace(namespace string) (string, error) {
	normalized := strings.TrimSpace(namespace)
	normalized = strings.TrimPrefix(normalized, "host.")
	if normalized == "" {
		return "", fmt.Errorf("register host api service: namespace is required")
	}
	if strings.Contains(normalized, ".") {
		return "", fmt.Errorf("register host api service: invalid namespace %q", namespace)
	}
	return normalized, nil
}

func splitHostMethod(method string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(method), ".")
	if len(parts) != 3 {
		return "", "", false
	}
	if parts[0] != "host" || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}
