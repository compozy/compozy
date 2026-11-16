package daemon

import (
	"context"
	"maps"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

const defaultSettingsMCPStatusMaxAge = 30 * time.Second

type settingsMCPStatusKey struct {
	target         mcpauth.Target
	lens           mcpauth.ConsentScope
	definition     string
	authGeneration uint64
	runtime        mcppkg.RuntimeHealthKey
}

type settingsMCPStatusEntry struct {
	status     settingspkg.MCPServerRuntimeStatus
	observedAt time.Time
	sequence   uint64
}

type settingsMCPStatusCache struct {
	mu       sync.Mutex
	entries  map[settingsMCPStatusKey]settingsMCPStatusEntry
	sequence uint64
}

var _ settingspkg.MCPCachedRuntimeProvider = (*settingsRuntimeSurface)(nil)

func (s *settingsRuntimeSurface) MCPServerRuntimeStatus(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (settingspkg.MCPServerRuntimeStatus, error) {
	key, keyErr := s.mcpStatusKey(ctx, target, server)
	s.mcpStatusCache.mu.Lock()
	s.mcpStatusCache.sequence++
	sequence := s.mcpStatusCache.sequence
	s.mcpStatusCache.mu.Unlock()
	status, err := s.probeMCPServerRuntimeStatus(ctx, target, server)
	if err == nil && ctx.Err() == nil && keyErr == nil && status.Probe != settingspkg.MCPServerProbeSkipped {
		if current, currentErr := s.mcpStatusKey(ctx, target, server); currentErr == nil && current == key {
			now := s.mcpStatusNow()
			s.mcpStatusCache.mu.Lock()
			if s.mcpStatusCache.entries == nil {
				s.mcpStatusCache.entries = make(map[settingsMCPStatusKey]settingsMCPStatusEntry)
			}
			maps.DeleteFunc(s.mcpStatusCache.entries, func(_ settingsMCPStatusKey, entry settingsMCPStatusEntry) bool {
				return now.Before(entry.observedAt) || now.Sub(entry.observedAt) >= s.mcpStatusMaxAgeOrDefault()
			})
			if s.mcpStatusCache.entries[key].sequence < sequence {
				s.mcpStatusCache.entries[key] = settingsMCPStatusEntry{
					status:     status,
					observedAt: now,
					sequence:   sequence,
				}
			}
			s.mcpStatusCache.mu.Unlock()
		}
	}
	return status, err
}

func (s *settingsRuntimeSurface) MCPServerCachedRuntimeStatus(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (settingspkg.MCPServerRuntimeStatus, error) {
	unknown := settingspkg.MCPServerRuntimeStatus{
		Configured: true,
		State:      settingspkg.MCPServerRuntimeStateUnknown,
		Probe:      settingspkg.MCPServerProbeSkipped,
		Reason:     "not_observed",
	}
	if err := ctx.Err(); err != nil {
		return unknown, err
	}
	if err := s.requireMCPConsent(ctx, target, server); err != nil {
		unknown.State, unknown.Reason = mcpWaitingApproval, mcpWaitingApproval
		return unknown, nil
	}
	deadKey, tracked := settingsMCPDeadEntityKey(target, server.EffectiveRuntimeName())
	if tracked && s.deadEntities != nil {
		dead, err := s.deadEntities.Status(ctx, deadKey)
		if err != nil {
			return unknown, err
		}
		if dead.Dead {
			return deadMCPRuntimeStatus(dead.Reason), nil
		}
	}
	if err := server.Validate("mcp_server"); err != nil {
		status := runtimeStatusFromMCPProbeError(err)
		status.State = settingspkg.MCPServerRuntimeStateConfigError
		status.Probe, status.Reason = settingspkg.MCPServerProbeSkipped, "config_error"
		return status, nil
	}
	if mcpauth.OAuthCapable(server) {
		auth, err := s.MCPAuthStatus(ctx, target, server)
		if err != nil {
			return unknown, err
		}
		if state, ok := runtimeStateFromMCPAuthStatus(auth); ok {
			unknown.State = state
			unknown.Reason = runtimeReasonFromMCPAuthState(state)
			unknown.Diagnostic = diagnostics.Redact(auth.Diagnostic)
			return unknown, nil
		}
	}
	key, err := s.mcpStatusKey(ctx, target, server)
	if err != nil {
		return unknown, err
	}
	if status, failed := s.mcpRuntimeFailureStatus(key); failed {
		return status, nil
	}

	now := s.mcpStatusNow()
	s.mcpStatusCache.mu.Lock()
	entry, found := s.mcpStatusCache.entries[key]
	s.mcpStatusCache.mu.Unlock()
	if !found {
		return unknown, nil
	}
	if now.Before(entry.observedAt) || now.Sub(entry.observedAt) >= s.mcpStatusMaxAgeOrDefault() {
		unknown.Reason = "stale"
		return unknown, nil
	}
	// Authorization is never cached; a change while reading invalidates the observation.
	current, err := s.mcpStatusKey(ctx, target, server)
	if err != nil {
		return unknown, err
	}
	if current != key {
		return unknown, nil
	}
	if err := s.requireMCPConsent(ctx, target, server); err != nil {
		unknown.State, unknown.Reason = mcpWaitingApproval, mcpWaitingApproval
		return unknown, nil
	}
	return entry.status, nil
}

func (s *settingsRuntimeSurface) mcpStatusNow() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *settingsRuntimeSurface) mcpStatusMaxAgeOrDefault() time.Duration {
	if s.mcpStatusMaxAge > 0 {
		return s.mcpStatusMaxAge
	}
	return defaultSettingsMCPStatusMaxAge
}

func (s *settingsRuntimeSurface) mcpStatusKey(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (settingsMCPStatusKey, error) {
	key := settingsMCPStatusKey{target: target.Normalize(), lens: mcpauth.ConsentScopeFromContext(ctx)}
	var err error
	key.definition, err = compozyconfig.MCPDefinitionDigest(server, s.digestKey)
	if err != nil {
		return key, err
	}
	if s.mcpAuthGeneration != nil {
		key.authGeneration, err = s.mcpAuthGeneration.Snapshot(target)
		if err != nil {
			return key, err
		}
	}
	if s.mcpHealthKey != nil {
		key.runtime, err = s.mcpHealthKey(ctx, target)
	}
	return key, err
}

func (s *settingsRuntimeSurface) mcpRuntimeFailureStatus(
	key settingsMCPStatusKey,
) (settingspkg.MCPServerRuntimeStatus, bool) {
	if s.mcpHealthRegistry == nil {
		return settingspkg.MCPServerRuntimeStatus{}, false
	}
	failures := s.mcpHealthRegistry.Entries(
		key.runtime.InstanceName, key.runtime.WorkspaceID, key.runtime.BundleGeneration,
	)
	for _, failure := range failures {
		if failure.Key.ServerName == key.runtime.ServerName && failure.Key.ResourceID == key.runtime.ResourceID {
			return settingspkg.MCPServerRuntimeStatus{
				Configured: true, State: settingspkg.MCPServerRuntimeStateRuntimeUnavailable,
				Probe: settingspkg.MCPServerProbeSkipped, Reason: "mcp_unreachable",
				Diagnostic: diagnostics.Redact(failure.Message),
			}, true
		}
	}
	return settingspkg.MCPServerRuntimeStatus{}, false
}
