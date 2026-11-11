package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/acp"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session/sessionid"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/version"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"go.opentelemetry.io/otel/attribute"
)

// telemetryShutdownTimeout bounds the final telemetry flush at daemon stop.
const telemetryShutdownTimeout = 5 * time.Second

type telemetryInitializer func(
	context.Context,
	compozyconfig.ObservabilityConfig,
	telemetry.SecretResolver,
	telemetry.Registries,
	version.Info,
	*slog.Logger,
) (telemetry.Runtime, error)

// routeTemplateLister is a server that reports the path templates of its mounted routes.
type routeTemplateLister interface {
	RouteTemplates() []string
}

// telemetrySlot hands the telemetry runtime, started after the servers mount their routes, to the status handlers
// those servers already hold.
type telemetrySlot struct {
	mu      sync.RWMutex
	runtime telemetry.Runtime
}

func (s *telemetrySlot) publish(runtime telemetry.Runtime) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtime = runtime
}

// Status reports OTLP export status; ok is false until telemetry has started.
func (s *telemetrySlot) Status() (telemetry.Status, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.runtime == nil {
		return telemetry.Status{}, false
	}
	return s.runtime.Status(), true
}

// ProviderEnv returns the OTLP variables forwarded to providers; nil until telemetry has started.
func (s *telemetrySlot) ProviderEnv() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.runtime == nil {
		return nil
	}
	return s.runtime.ProviderEnv()
}

// ensureTelemetry returns the boot's telemetry slot, created by its first consumer: the session manager, which reads
// provider forwarding at spawn time, or the status handlers.
func (s *bootState) ensureTelemetry() *telemetrySlot {
	if s.telemetry == nil {
		s.telemetry = &telemetrySlot{}
	}
	return s.telemetry
}

// shutdown flushes pending telemetry for at most telemetryShutdownTimeout. A flush that cannot finish never fails
// daemon shutdown; telemetry logs the timeout itself.
func (s *telemetrySlot) shutdown(ctx context.Context, logger *slog.Logger) {
	if s == nil {
		return
	}
	s.mu.RLock()
	runtime := s.runtime
	s.mu.RUnlock()
	if runtime == nil {
		return
	}
	flushCtx, cancel := context.WithTimeout(ctx, telemetryShutdownTimeout)
	defer cancel()
	if err := runtime.Shutdown(flushCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		logger.WarnContext(ctx, "daemon: shutdown telemetry", "error", err)
	}
}

// bootTelemetry starts OTLP export once the HTTP and UDS servers have mounted their routes, whose templates the
// sanitizer accepts as span names. Export problems surface in status and never fail boot.
func (d *Daemon) bootTelemetry(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state.telemetry == nil || state.httpServer == nil || state.udsServer == nil {
		return errors.New("daemon: telemetry starts after the HTTP and UDS servers mount their routes")
	}
	var refs telemetry.SecretResolver
	if state.providerVault != nil {
		refs = state.providerVault
	}
	runtime, err := d.initTelemetry(
		ctx,
		state.cfg.Observability,
		refs,
		telemetryRegistries(ctx, state),
		version.Current(),
		state.logger,
	)
	if err != nil {
		return fmt.Errorf("daemon: init telemetry: %w", err)
	}
	state.telemetry.publish(runtime)
	// Writes record into the global providers Init just installed; like them, the observer is process-wide.
	slowDBWriteMS := state.cfg.Observability.OTel.SlowDBWriteMS
	store.SetWriteObserver(func(ctx context.Context, write store.WriteObservation) {
		telemetry.RecordDBWrite(ctx, telemetry.DBWrite(write), slowDBWriteMS)
	})
	cleanup.add(func(ctx context.Context) error {
		state.telemetry.shutdown(ctx, state.logger)
		return nil
	})
	return nil
}

// telemetryRegistries holds what the sanitizer checks exported values against: route templates of the mounted
// servers, traced ACP methods, hook events, provider, agent, and native tool membership, and each identity
// attribute's owner validator. Model values have no registry yet and export as "other".
func telemetryRegistries(ctx context.Context, state *bootState) telemetry.Registries {
	var routes []string
	for _, server := range []Server{state.httpServer, state.udsServer} {
		if lister, ok := server.(routeTemplateLister); ok {
			routes = append(routes, lister.RouteTemplates()...)
		}
	}
	events := hookspkg.AllHookEvents()
	hookEvents := make([]string, 0, len(events))
	for _, event := range events {
		hookEvents = append(hookEvents, event.String())
	}
	agents := state.agentCatalog
	return telemetry.Registries{
		RouteTemplates: routes,
		ACPMethods:     acp.TracedMethods(),
		HookEvents:     hookEvents,
		Providers:      providerMembership(state.cfg.Providers),
		Agents: func(name string) bool {
			return agents.Contains(func(agent compozyconfig.AgentDef) bool { return agent.Name == name })
		},
		ToolIDs: nativeToolMembership(ctx, state.nativeToolProvider, state.logger),
		IDs: map[attribute.Key]func(string) bool{
			telemetry.AttrProfileID:    profile.IsProfileID,
			telemetry.AttrWorkspaceID:  workspacepkg.IsWorkspaceIdentifier,
			telemetry.AttrSessionID:    sessionid.IsSessionID,
			telemetry.AttrTurnID:       sessionid.IsTurnID,
			telemetry.AttrRunID:        sessionid.IsRunID,
			telemetry.AttrLoopID:       looppkg.IsLoopID,
			telemetry.AttrAutomationID: automationpkg.IsAutomationID,
		},
	}
}

// providerMembership admits the built-in providers and those config.toml declares at boot; a provider added later
// exports as "other" until the daemon restarts.
func providerMembership(configured map[string]compozyconfig.ProviderConfig) func(string) bool {
	names := make(map[string]struct{}, len(configured))
	for name := range compozyconfig.BuiltinProviders() {
		names[name] = struct{}{}
	}
	for name := range configured {
		names[name] = struct{}{}
	}
	return func(name string) bool {
		_, ok := names[name]
		return ok
	}
}
