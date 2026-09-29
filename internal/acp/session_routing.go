package acp

import (
	"encoding/json"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
)

// sessionUpdateRoute is the destination of one inbound ACP notification or
// callback, keyed by its ACP session id (ADR-003).
type sessionUpdateRoute uint8

const (
	// sessionUpdateRouteBound is the process's bound session: the existing path.
	sessionUpdateRouteBound sessionUpdateRoute = iota
	// sessionUpdateRouteFork is traffic captured by an open session/fork call.
	sessionUpdateRouteFork
	// sessionUpdateRouteForeign is any other id: dropped, logged once, counted.
	sessionUpdateRouteForeign
)

// maxLoggedForeignSessionIDs bounds the once-per-id log memory so a
// misbehaving agent cannot grow it without limit; the counter keeps counting.
const maxLoggedForeignSessionIDs = 64

// sessionRouter owns the per-process routing state. The bound id mirrors
// AgentProcess.SessionID under a lock because inbound traffic is handled on
// the connection's goroutines while negotiation writes the field.
type sessionRouter struct {
	mu             sync.Mutex
	bound          acpsdk.SessionId
	fork           *forkCapture
	foreignLogged  map[acpsdk.SessionId]struct{}
	foreignDropped uint64
}

// forkCapture collects notifications for non-bound ids while a session/fork
// call is open. The clone id is unknown until the response arrives, so updates
// are grouped by id and attributed retroactively.
type forkCapture struct {
	updates map[acpsdk.SessionId][]acpsdk.SessionNotification
}

func (p *AgentProcess) bindSessionRoute(id acpsdk.SessionId) {
	if p == nil {
		return
	}
	p.sessionRoutes.mu.Lock()
	defer p.sessionRoutes.mu.Unlock()
	p.sessionRoutes.bound = id
}

func (p *AgentProcess) boundSessionRoute() acpsdk.SessionId {
	p.sessionRoutes.mu.Lock()
	defer p.sessionRoutes.mu.Unlock()
	return p.sessionRoutes.bound
}

// routeSessionUpdate classifies inbound traffic by ACP session id. Before the
// process is bound (session/new or session/load still in flight) every id takes
// the existing path, so negotiation-time replay keeps its current behavior.
func (p *AgentProcess) routeSessionUpdate(id acpsdk.SessionId) sessionUpdateRoute {
	p.sessionRoutes.mu.Lock()
	defer p.sessionRoutes.mu.Unlock()
	return p.sessionRoutes.routeLocked(id)
}

func (r *sessionRouter) routeLocked(id acpsdk.SessionId) sessionUpdateRoute {
	switch {
	case r.bound == "" || id == r.bound:
		return sessionUpdateRouteBound
	case r.fork != nil && id != "":
		return sessionUpdateRouteFork
	default:
		return sessionUpdateRouteForeign
	}
}

// captureForkUpdate stores a notification for an open fork capture. It reports
// false when the capture closed between routing and capture.
func (p *AgentProcess) captureForkUpdate(notification acpsdk.SessionNotification) bool {
	p.sessionRoutes.mu.Lock()
	defer p.sessionRoutes.mu.Unlock()
	if p.sessionRoutes.routeLocked(notification.SessionId) != sessionUpdateRouteFork {
		return false
	}
	capture := p.sessionRoutes.fork
	capture.updates[notification.SessionId] = append(capture.updates[notification.SessionId], notification)
	return true
}

func (p *AgentProcess) openForkCapture() *forkCapture {
	capture := &forkCapture{updates: make(map[acpsdk.SessionId][]acpsdk.SessionNotification)}
	p.sessionRoutes.mu.Lock()
	defer p.sessionRoutes.mu.Unlock()
	p.sessionRoutes.fork = capture
	return capture
}

func (p *AgentProcess) closeForkCapture(capture *forkCapture) map[acpsdk.SessionId][]acpsdk.SessionNotification {
	p.sessionRoutes.mu.Lock()
	defer p.sessionRoutes.mu.Unlock()
	if p.sessionRoutes.fork == capture {
		p.sessionRoutes.fork = nil
	}
	return capture.updates
}

// dropForeignSessionTraffic counts one dropped notification or callback for a
// foreign id and logs the first occurrence per id.
func (p *AgentProcess) dropForeignSessionTraffic(id acpsdk.SessionId, kind string) {
	p.sessionRoutes.mu.Lock()
	p.sessionRoutes.foreignDropped++
	dropped := p.sessionRoutes.foreignDropped
	_, logged := p.sessionRoutes.foreignLogged[id]
	shouldLog := !logged && len(p.sessionRoutes.foreignLogged) < maxLoggedForeignSessionIDs
	if shouldLog {
		if p.sessionRoutes.foreignLogged == nil {
			p.sessionRoutes.foreignLogged = make(map[acpsdk.SessionId]struct{})
		}
		p.sessionRoutes.foreignLogged[id] = struct{}{}
	}
	bound := p.sessionRoutes.bound
	p.sessionRoutes.mu.Unlock()
	if !shouldLog {
		return
	}
	p.usageLogger().Warn(
		"acp: dropped traffic for a foreign ACP session id",
		"bound_acp_session_id", string(bound),
		"foreign_acp_session_id", string(id),
		"kind", kind,
		"dropped_total", dropped,
	)
}

// foreignSessionTrafficDropped returns the number of dropped foreign-id items.
func (p *AgentProcess) foreignSessionTrafficDropped() uint64 {
	p.sessionRoutes.mu.Lock()
	defer p.sessionRoutes.mu.Unlock()
	return p.sessionRoutes.foreignDropped
}

// permissionRequestIsBound reports whether a permission callback belongs to the
// bound session. A clone or foreign session cannot ask the bound session's
// operator; foreign requests are also counted and logged.
func (p *AgentProcess) permissionRequestIsBound(id acpsdk.SessionId) bool {
	switch p.routeSessionUpdate(id) {
	case sessionUpdateRouteBound:
		return true
	case sessionUpdateRouteForeign:
		p.dropForeignSessionTraffic(id, acpsdk.ClientMethodSessionRequestPermission)
	case sessionUpdateRouteFork:
	}
	return false
}

// sessionScopedCallbackMethods are the client callbacks that act through the
// bound session's tool host. A clone or foreign session id must never reach it.
var sessionScopedCallbackMethods = map[string]struct{}{
	acpsdk.ClientMethodFsReadTextFile:      {},
	acpsdk.ClientMethodFsWriteTextFile:     {},
	acpsdk.ClientMethodTerminalCreate:      {},
	acpsdk.ClientMethodTerminalKill:        {},
	acpsdk.ClientMethodTerminalOutput:      {},
	acpsdk.ClientMethodTerminalWaitForExit: {},
	acpsdk.ClientMethodTerminalRelease:     {},
}

type wireSessionScopedCallback struct {
	SessionID acpsdk.SessionId `json:"sessionId"`
}

// rejectNonBoundSessionCallback quarantines filesystem and terminal callbacks
// whose sessionId is not the bound session (ADR-003): a fork clone's callbacks
// are refused, foreign ones are refused, counted, and logged once. Undecodable
// params fall through so the method handler reports its own invalid-params error.
func (p *AgentProcess) rejectNonBoundSessionCallback(method string, params json.RawMessage) *acpsdk.RequestError {
	if _, scoped := sessionScopedCallbackMethods[method]; !scoped {
		return nil
	}
	var callback wireSessionScopedCallback
	if err := json.Unmarshal(params, &callback); err != nil {
		return nil
	}
	switch p.routeSessionUpdate(callback.SessionID) {
	case sessionUpdateRouteBound:
		return nil
	case sessionUpdateRouteForeign:
		p.dropForeignSessionTraffic(callback.SessionID, method)
	case sessionUpdateRouteFork:
	}
	return acpsdk.NewInvalidParams(map[string]any{
		EventTypeError: "session " + string(callback.SessionID) + " is not bound to this client",
	})
}
