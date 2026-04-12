package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/subprocess"
	"github.com/compozy/compozy/internal/version"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const (
	defaultExtensionHookTimeout   = 5 * time.Second
	defaultExtensionHealthTimeout = 5 * time.Second
	defaultExtensionShutdown      = 10 * time.Second
	defaultEventQueueDepth        = 256
	maxEventTimeouts              = 6
	executionModeLabelExec        = "exec"
)

var (
	extensionHealthTimeout = defaultExtensionHealthTimeout
	extensionEventQueueCap = defaultEventQueueDepth
)

const (
	extensionFailurePhaseSpawn      = "spawn"
	extensionFailurePhaseInitialize = "initialize"
	extensionFailurePhaseTransport  = "transport"
	extensionFailurePhaseHealth     = "health"
	extensionFailurePhaseShutdown   = "shutdown"
)

const (
	shutdownReasonRunCompleted = "run_completed"
	shutdownReasonRunCancelled = "run_canceled"
	shutdownReasonRunFailed    = "run_failed"
	shutdownReasonManagerError = "manager_error"
	shutdownReasonHealthFailed = "health_failed"
)

// Start launches every enabled executable extension and transitions successful
// sessions to Ready.
func (m *Manager) Start(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	if m.started {
		err := m.startErr
		m.mu.Unlock()
		return err
	}
	m.started = true
	if m.backgroundCtx == nil {
		m.backgroundCtx, m.backgroundStop = context.WithCancel(context.Background())
	}
	m.mu.Unlock()

	var startErr error
	for _, extension := range m.executableExtensions() {
		if err := m.emitLifecycleEvent(ctx, events.EventKindExtensionLoaded, kinds.ExtensionLoadedPayload{
			Extension:    extension.normalizedName(),
			Source:       string(extension.Ref.Source),
			Version:      extension.manifestVersion(),
			ManifestPath: extension.ManifestPath,
		}); err != nil {
			startErr = errors.Join(startErr, err)
			continue
		}

		extension.SetState(ExtensionStateInitializing)
		if err := m.startExtension(ctx, extension); err != nil {
			startErr = errors.Join(startErr, err)
		}
	}

	if startErr == nil {
		if err := m.startEventForwarding(); err != nil {
			startErr = errors.Join(startErr, err)
		}
	}

	if startErr != nil {
		startErr = errors.Join(startErr, m.shutdownWithReason(context.Background(), shutdownReasonManagerError))
	} else {
		registerActiveManager(m)
	}

	m.mu.Lock()
	m.startErr = startErr
	m.mu.Unlock()
	return startErr
}

// DispatchMutable routes one mutable hook through the priority-ordered chain.
func (m *Manager) DispatchMutable(ctx context.Context, hook HookName, input any) (any, error) {
	if m == nil || m.dispatcher == nil {
		return input, nil
	}
	return m.dispatcher.DispatchMutable(ctx, hook, input)
}

// DispatchMutableHook adapts the generic runtime-manager hook interface onto
// the extension hook dispatcher.
func (m *Manager) DispatchMutableHook(ctx context.Context, hook string, input any) (any, error) {
	return m.DispatchMutable(ctx, HookName(strings.TrimSpace(hook)), input)
}

// DispatchObserver fans out one observe-only hook using the dispatcher’s
// existing best-effort semantics.
func (m *Manager) DispatchObserver(ctx context.Context, hook HookName, payload any) {
	if m == nil || m.dispatcher == nil {
		return
	}
	m.dispatcher.DispatchObserver(ctx, hook, payload)
}

// DispatchObserverHook adapts the generic runtime-manager hook interface onto
// the extension hook dispatcher.
func (m *Manager) DispatchObserverHook(ctx context.Context, hook string, payload any) {
	m.DispatchObserver(ctx, HookName(strings.TrimSpace(hook)), payload)
}

func (m *Manager) executableExtensions() []*RuntimeExtension {
	if m == nil || m.registry == nil {
		return nil
	}

	extensions := make([]*RuntimeExtension, 0)
	for _, extension := range m.registry.Extensions() {
		if extension == nil || extension.Manifest == nil || extension.Manifest.Subprocess == nil {
			continue
		}
		extensions = append(extensions, extension)
	}
	return extensions
}

func (m *Manager) registerSession(session *extensionSession) {
	if m == nil || session == nil || session.runtime == nil {
		return
	}

	name := sessionKeyForRuntime(session.runtime)
	if name == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sessions == nil {
		m.sessions = make(map[string]*extensionSession)
	}
	if m.subprocs == nil {
		m.subprocs = make(map[string]*subprocess.Process)
	}
	m.sessions[name] = session
	m.subprocs[name] = session.process
}

func (m *Manager) sessionSnapshot() []*extensionSession {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*extensionSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

func (m *Manager) sessionForExtension(name string) (*extensionSession, bool) {
	if m == nil {
		return nil, false
	}

	normalized := normalizeSessionKey(name)
	if normalized == "" {
		return nil, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[normalized]
	return session, ok
}

func sessionKeyForRuntime(extension *RuntimeExtension) string {
	if extension == nil {
		return ""
	}
	return normalizeSessionKey(extension.normalizedName())
}

func normalizeSessionKey(name string) string {
	return strings.TrimSpace(name)
}

func (m *Manager) emitLifecycleEvent(ctx context.Context, kind events.EventKind, payload any) error {
	if m == nil {
		return nil
	}

	event, err := newHostRuntimeEvent(m.runID, kind, payload)
	if err != nil {
		return err
	}

	switch {
	case m.journal != nil:
		_, err = m.journal.SubmitWithSeq(ctx, event)
		return err
	case m.eventBus != nil:
		m.eventBus.Publish(ctx, event)
		return nil
	default:
		return nil
	}
}

func (m *Manager) emitFailureEvent(
	ctx context.Context,
	extension *RuntimeExtension,
	phase string,
	err error,
) error {
	if extension == nil || err == nil {
		return nil
	}

	slog.ErrorContext(
		ctx,
		"extension lifecycle failure",
		"component", "extension.manager",
		"extension", extension.normalizedName(),
		"phase", strings.TrimSpace(phase),
		"err", err,
	)

	return m.emitLifecycleEvent(ctx, events.EventKindExtensionFailed, kinds.ExtensionFailedPayload{
		Extension: extension.normalizedName(),
		Source:    string(extension.Ref.Source),
		Version:   extension.manifestVersion(),
		Phase:     strings.TrimSpace(phase),
		Error:     err.Error(),
	})
}

func (m *Manager) recordLifecycleAudit(
	extension *RuntimeExtension,
	method string,
	latency time.Duration,
	callErr error,
) {
	if m == nil || m.audit == nil || extension == nil {
		return
	}

	entry := AuditEntry{
		Timestamp: time.Now().UTC(),
		Extension: extension.normalizedName(),
		Direction: AuditDirectionHostToExt,
		Method:    strings.TrimSpace(method),
		Latency:   latency,
		Result:    AuditResultOK,
	}
	if callErr != nil {
		entry.Result = AuditResultError
		entry.ErrorDetail = callErr.Error()
	}
	if err := m.audit.Record(entry); err != nil {
		slog.Warn(
			"record extension lifecycle audit",
			"component", "extension.manager",
			"extension", extension.normalizedName(),
			"method", method,
			"error", err,
		)
	}
}

func (m *Manager) backgroundContext() context.Context {
	if m == nil {
		return context.Background()
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.backgroundCtx != nil {
		return m.backgroundCtx
	}
	return context.Background()
}

func invokingCommandForMode(mode model.ExecutionMode) string {
	switch mode {
	case model.ExecutionModeExec:
		return executionModeLabelExec
	case model.ExecutionModePRReview:
		return invokingCommandFixReviews
	default:
		return invokingCommandStart
	}
}

func defaultShutdownTimeout(extension *RuntimeExtension) time.Duration {
	if extension == nil {
		return defaultExtensionShutdown
	}
	if deadline := extension.ShutdownDeadline(); deadline > 0 {
		return deadline
	}
	return defaultExtensionShutdown
}

func defaultHealthInterval(extension *RuntimeExtension) time.Duration {
	if extension == nil || extension.Manifest == nil || extension.Manifest.Subprocess == nil {
		return 0
	}
	return extension.Manifest.Subprocess.HealthCheckPeriod
}

func marshalResult(result any) (json.RawMessage, error) {
	if result == nil {
		return json.RawMessage(`{}`), nil
	}

	raw, ok := result.(json.RawMessage)
	if ok {
		if len(raw) == 0 {
			return json.RawMessage(`{}`), nil
		}
		return append(json.RawMessage(nil), raw...), nil
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if len(encoded) == 0 || string(encoded) == "null" {
		return json.RawMessage(`{}`), nil
	}
	return encoded, nil
}

func requestErrorFromError(err error, method string) *subprocess.RequestError {
	if err == nil {
		return nil
	}

	var requestErr *subprocess.RequestError
	if errors.As(err, &requestErr) {
		return requestErr
	}

	return subprocess.NewInternalError(map[string]any{
		"method": strings.TrimSpace(method),
		"error":  err.Error(),
	})
}

func (e *RuntimeExtension) manifestVersion() string {
	if e == nil || e.Manifest == nil {
		return ""
	}
	return strings.TrimSpace(e.Manifest.Extension.Version)
}

func grantedCapabilities(extension *RuntimeExtension) []Capability {
	if extension == nil || extension.Manifest == nil {
		return nil
	}
	return cloneCapabilities(extension.Manifest.Security.Capabilities)
}

func compozyProtocolVersion() string {
	return strings.TrimSpace(version.ExtensionProtocolVersion)
}

func isTimeoutError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}
