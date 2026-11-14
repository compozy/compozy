package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/runtimemcp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/tools"
)

// EventTypeMCPReloaded identifies the durable outcome of one prompt-boundary reload.
const EventTypeMCPReloaded = "session.mcp_reloaded"

// MCPReloadFailedCode is the authoritative pre-dispatch refusal shared by all prompt transports.
const MCPReloadFailedCode = "session_mcp_reload_failed"

type reloadRecord struct {
	At      time.Time `json:"at"`
	Outcome string    `json:"outcome"`
	Reason  string    `json:"reason"`
}

type activationState struct {
	boundDigest    string
	boundServers   map[string]int
	servedDigest   string
	runtimeMCP     string
	runtimeDigest  string
	runtimeRequest *runtimemcp.Request
	lastReload     *reloadRecord
}

func (a activationState) clone() activationState {
	a.boundServers = maps.Clone(a.boundServers)
	if a.runtimeRequest != nil {
		a.runtimeRequest = new(*a.runtimeRequest)
		a.runtimeRequest.Context.Env = maps.Clone(a.runtimeRequest.Context.Env)
	}
	if a.lastReload != nil {
		a.lastReload = new(*a.lastReload)
	}
	return a
}

// MCPReloadError refuses admission when the requested MCP runtime could not be prepared.
type MCPReloadError struct {
	Provider        string
	ClientMessageID string
	Cause           error
}

var _ error = (*MCPReloadError)(nil)

func (e *MCPReloadError) Error() string {
	return fmt.Sprintf(
		"Couldn't start %s with the new MCP servers: %s. Your message wasn't sent. (%s)",
		e.Provider, diagnostics.RedactAndBound(e.Cause.Error(), 2048), MCPReloadFailedCode,
	)
}
func (e *MCPReloadError) Unwrap() error { return e.Cause }

type activationAdmissionKey struct{}
type activationAdmission struct{ checked bool }

func withActivationAdmission(ctx context.Context) context.Context {
	return context.WithValue(ctx, activationAdmissionKey{}, &activationAdmission{})
}

func activationCheckDue(ctx context.Context) bool {
	admission, ok := ctx.Value(activationAdmissionKey{}).(*activationAdmission)
	if !ok {
		return true
	}
	if admission.checked {
		return false
	}
	admission.checked = true
	return true
}

type hostedActivationCatalog interface {
	ActivationCatalog(context.Context, string) (tools.AdvertisedCatalog, string, error)
	BoundActivationCatalog(string) (tools.AdvertisedCatalog, bool)
}

func activationFingerprint(catalog, mode, runtimeDigest string) string {
	sum := sha256.Sum256([]byte(catalog + "\x00" + mode + "\x00" + runtimeDigest))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) needsActivationReload(ctx context.Context, s *Session) (bool, string) {
	s.mu.RLock()
	state, process := s.activation.clone(), s.process
	s.mu.RUnlock()
	if state.lastReload != nil && state.lastReload.Outcome == "failed" {
		return true, state.lastReload.Reason
	}
	if process == nil {
		return false, ""
	}
	mode := state.runtimeMCP
	if plan, err := m.currentMCPRuntimePlan(ctx, s); err == nil {
		mode = plan.spec.workspace.Config.Providers[plan.selection.Provider].EffectiveRuntimeMCP()
	}
	if mode != state.runtimeMCP {
		return true, "runtime_mcp"
	}
	runtimeDigest := m.currentRuntimeMCPDigest(ctx, state, mode)
	if runtimeDigest != state.runtimeDigest {
		return true, "runtime_inventory"
	}
	source, ok := m.hostedMCP.(hostedActivationCatalog)
	if !ok {
		return m.hostedMCPToolsChanged(ctx, s), "fingerprint"
	}
	if state.boundDigest == "" {
		return false, ""
	}
	catalog, served, err := source.ActivationCatalog(ctx, s.ID)
	if err != nil {
		return errors.Is(err, ErrHostedMCPBindMissing), "served_stale"
	}
	s.mu.Lock()
	s.activation.servedDigest = served
	s.mu.Unlock()
	if !s.hostedReloadRequired() {
		return served == "" || served != catalog.Digest, "served_stale"
	}
	return state.boundDigest != activationFingerprint(catalog.Digest, mode, runtimeDigest), "fingerprint"
}

func (s *Session) hostedReloadRequired() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hostedMCPRequiresReload
}

func (m *Manager) prepareActivationCatalog(
	ctx context.Context, s *Session, runtime *sessionStartRuntime,
) error {
	if m.hostedMCP != nil && len(runtime.mcpServers) == 0 {
		return nil
	}
	if source, ok := m.hostedMCP.(hostedActivationCatalog); ok {
		catalog, _, err := source.ActivationCatalog(ctx, s.ID)
		if err != nil {
			return err
		}
		runtime.activationCatalog = &catalog
	}
	return nil
}

func (m *Manager) commitActivation(s *Session, runtime *sessionStartRuntime) {
	// Prefer the exact initial revision accepted by the proxy over the prelaunch snapshot.
	if source, ok := m.hostedMCP.(hostedActivationCatalog); ok && runtime.activationCatalog != nil {
		if catalog, bound := source.BoundActivationCatalog(s.ID); bound {
			runtime.activationCatalog = &catalog
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activation.runtimeMCP = runtime.activationMode
	s.activation.runtimeDigest = runtime.activationDigest
	s.activation.runtimeRequest = runtime.activationRequest
	s.activation.boundDigest = ""
	s.activation.boundServers = nil
	if runtime.activationCatalog != nil {
		s.activation.boundDigest = activationFingerprint(
			runtime.activationCatalog.Digest, runtime.activationMode, runtime.activationDigest,
		)
		s.activation.boundServers = maps.Clone(runtime.activationCatalog.Servers)
	}
}

func (m *Manager) reloadActivationRuntime(
	ctx context.Context, s *Session, snapshot *runtimeBindingSnapshot, selection RuntimeSelection, reason string,
) (proc *AgentProcess, err error) {
	before := snapshot.activation
	started := m.now()
	attempted := before.clone()
	var plan *promptRuntimePlan
	if source, ok := m.hostedMCP.(hostedActivationCatalog); ok {
		if catalog, _, catalogErr := source.ActivationCatalog(ctx, s.ID); catalogErr == nil {
			attempted.boundServers = catalog.Servers
		}
	}
	defer func() {
		outcome := "applied"
		if err != nil {
			outcome = "failed"
		} else if plan != nil && plan.replayed {
			outcome = "replayed"
		}
		if err == nil {
			s.mu.RLock()
			attempted = s.activation.clone()
			s.mu.RUnlock()
		}
		cleanupCtx, cancel := m.lifecycleCleanupContext()
		defer cancel()
		eventErr := m.recordMCPReload(cleanupCtx, s, before, attempted, outcome, reason)
		m.sessionLogger(s).Info("session.mcp_reload", "reason", reason, "outcome", outcome,
			"duration_ms", m.now().Sub(started).Milliseconds())
		if err != nil || eventErr != nil {
			s.mu.Lock()
			last := s.activation.lastReload
			s.activation = before.clone()
			s.activation.lastReload = last
			s.mu.Unlock()
			err = &MCPReloadError{Provider: selection.Provider, Cause: errors.Join(err, eventErr)}
		}
	}()
	command, env := "", snapshot.acceptedSessionEnv
	if runtimeSelectionsEqual(snapshot.selection, selection) {
		command = snapshot.acceptedCommand
	}
	plan, err = m.preparePromptRuntimePlanForRoute(ctx, s, selection, command, env)
	if err != nil {
		return nil, err
	}
	if err = m.validateExplicitStartModel(ctx, &plan.runtime, &plan.spec); err != nil {
		return nil, err
	}
	if err = m.releaseBoundRuntime(ctx, s); err != nil {
		return nil, err
	}
	released := s.runtimeBindingSnapshot()
	return m.rebindReleasedRuntime(ctx, s, &released, plan)
}

func (m *Manager) recordMCPReload(
	ctx context.Context, s *Session, before, after activationState, outcome, reason string,
) error {
	added, removed := []string{}, []string{}
	for name := range after.boundServers {
		if _, exists := before.boundServers[name]; !exists {
			added = append(added, name)
		}
	}
	for name := range before.boundServers {
		if _, exists := after.boundServers[name]; !exists {
			removed = append(removed, name)
		}
	}
	slices.Sort(added)
	slices.Sort(removed)
	content, err := json.Marshal(struct {
		Outcome string   `json:"outcome"`
		Added   []string `json:"added"`
		Removed []string `json:"removed"`
		Reason  string   `json:"reason"`
	}{outcome, added, removed, reason})
	if err != nil {
		return err
	}
	eventID, err := store.NewID("ev")
	if err != nil {
		return err
	}
	persisted, _, err := m.appendDurableSessionEvent(ctx, s.ID, store.SessionEvent{
		ID: eventID, SessionID: s.ID, TurnID: s.CurrentTurnID(), AgentName: s.AgentName,
		Type: EventTypeMCPReloaded, Content: string(content), Timestamp: m.now(),
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.activation.lastReload = &reloadRecord{At: persisted.Timestamp, Outcome: outcome, Reason: reason}
	s.mu.Unlock()
	m.publishSessionEventByID(ctx, s.ID, persisted)
	return nil
}
