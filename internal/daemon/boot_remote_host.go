package daemon

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

const (
	remoteConfigKey  = "remote"
	remoteEnabledKey = "enabled"
)

func WithCloudflaredInstaller(install func(context.Context, string) (string, error)) Option {
	return func(d *Daemon) { d.installCloudflared = install }
}

type remoteOperatorHandler struct {
	current atomic.Pointer[http.Handler]
}

func newRemoteOperatorHandler(initial http.Handler) *remoteOperatorHandler {
	slot := &remoteOperatorHandler{}
	slot.swap(initial)
	return slot
}

func (h *remoteOperatorHandler) swap(next http.Handler) {
	h.current.Store(&next)
}

func (h *remoteOperatorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if current := h.current.Load(); current != nil && *current != nil {
		(*current).ServeHTTP(w, r)
		return
	}
	http.NotFound(w, r)
}

func (d *Daemon) bootRemoteHost(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	storage, ok := state.registry.(remotehost.Store)
	workers := &state.runtimeWorkers
	if !ok || workers.account == nil {
		return nil
	}
	state.remoteOperator = newRemoteOperatorHandler(remoteOperatorRoutes(&state.deps))
	host, err := remotehost.NewHost(
		ctx,
		storage,
		workers.cloud,
		workers.account,
		state.cfg.Remote,
		remotehost.HostOptions{
			Now:                d.now,
			Logger:             state.logger,
			Home:               d.homePaths.HomeDir,
			Operator:           state.remoteOperator,
			Persist:            d.persistRemoteConfig,
			InstallCloudflared: d.installCloudflared,
			SyncIngress:        remoteIngressSync(state, d.now),
		},
	)
	if err != nil {
		return fmt.Errorf("daemon: start remote host: %w", err)
	}
	workers.remote = host
	state.deps.Remote = host
	cleanup.add(func(context.Context) error { host.Close(); return nil })
	return nil
}

func remoteIngressSync(state *bootState, now func() time.Time) func(context.Context, ingress.RemoteEndpoint) error {
	registry, ok := state.registry.(interface {
		SyncRemoteIngressEndpoint(context.Context, ingress.RemoteEndpoint, time.Time) error
	})
	if !ok {
		return nil
	}
	return func(ctx context.Context, endpoint ingress.RemoteEndpoint) error {
		return registry.SyncRemoteIngressEndpoint(ctx, endpoint, now())
	}
}

func remoteOperatorRoutes(deps *RuntimeDeps) http.Handler {
	config := &core.BaseHandlerConfig{
		TransportName:                remoteConfigKey,
		MaskInternalErrors:           true,
		IncludeSessionWorkspaceInSSE: true,
		Automation:                   deps.Automation,
		Sessions:                     deps.Sessions,
		SessionCatalog:               deps.Registry,
		SessionHealth:                deps.SessionHealth,
		Worktrees:                    deps.Worktrees,
		Workspaces:                   deps.WorkspaceService,
		WorkspaceAccess:              deps.WorkspaceAccess,
		Profiles:                     deps.Profiles,
		Config:                       deps.Config,
	}
	if acceptance, ok := deps.Sessions.(core.SessionAcceptanceManager); ok {
		config.SessionAcceptance = acceptance
	}
	handlers := core.NewBaseHandlers(config)
	return httpapi.NewRemoteRoutes(func(routes *httpapi.RemoteRoutes) {
		remoteSessions := routes.Group("/api/sessions", remoteproto.ScopeSessions)
		remoteSessions.GET("", handlers.ListSessions)
		remoteSessions.GET("/facets", handlers.ListSessionFacets)
		remoteSessions.POST("", handlers.CreateSession)
		remoteSessions.GET("/:session_id", handlers.GetSessionByID)
		remoteSessions.GET("/:session_id/stream", handlers.StreamSession)
		remoteSessions.GET("/:session_id/status", handlers.GetSessionStatus)
		remoteSessions.GET("/:session_id/transcript", handlers.SessionTranscript)
		remoteWorkspaces := routes.Group("/api/workspaces", remoteproto.ScopeSessions)
		remoteWorkspaces.GET("", handlers.ListWorkspaces)
		remoteJobs := routes.Group("/api/automation/jobs", remoteproto.ScopeSessions)
		remoteJobs.GET("", handlers.ListAutomationJobs)
		remoteJobs.POST("", handlers.CreateAutomationJob)
		remoteJobs.GET("/:id", handlers.GetAutomationJob)
		remoteJobs.PATCH("/:id", handlers.UpdateAutomationJob)
		remoteJobs.DELETE("/:id", handlers.DeleteAutomationJob)
		remoteJobs.POST("/:id/trigger", handlers.TriggerAutomationJob)
		remoteJobs.GET("/:id/runs", handlers.AutomationJobRuns)
		sessions := routes.Group(
			"/api/workspaces/:workspace_id/sessions",
			remoteproto.ScopeSessions,
			handlers.WorkspaceReferenceGuard,
		)
		sessions.PATCH("/:session_id", handlers.RenameSession)
		sessions.POST("/:session_id/prompt", func(c *gin.Context) {
			if dispatch, ok := handlers.DispatchSessionPrompt(c); ok {
				handlers.RespondPromptV1(c, dispatch)
			}
		})
	})
}

func (d *Daemon) persistRemoteConfig(
	ctx context.Context, patch remotehost.Patch, apply func(config.RemoteConfig) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target, err := config.ResolveConfigWriteTarget(d.homePaths, "", config.WriteScopeUser, "")
	if err != nil {
		return err
	}
	_, err = config.EditConfigOverlayAndApply(d.homePaths, "", target, func(editor *config.OverlayEditor) error {
		changes := []struct {
			path  []string
			value *bool
		}{
			{[]string{remoteConfigKey, remoteEnabledKey}, patch.Enabled},
			{[]string{remoteConfigKey, "direct", remoteEnabledKey}, patch.Direct},
			{[]string{remoteConfigKey, "cloud_route", remoteEnabledKey}, patch.CloudRoute},
			{[]string{remoteConfigKey, "require_device_approval"}, patch.RequireDeviceApproval},
		}
		for _, change := range changes {
			if change.value != nil {
				if err := editor.SetValue(change.path, *change.value); err != nil {
					return err
				}
			}
		}
		return nil
	}, func(next config.Config) error { return apply(next.Remote) })
	return err
}

func (a daemonSettingsRuntimeApplier) applyRemoteConfig(
	ctx context.Context,
	previous, next *config.Config,
) *settingspkg.ApplyFailure {
	host := a.state.runtimeWorkers.remote
	if host == nil || reflect.DeepEqual(previous.Remote, next.Remote) {
		return nil
	}
	if err := host.ApplyConfig(ctx, next.Remote); err != nil {
		return new(configApplyFailure(remoteConfigKey, "Remote access settings could not be applied", err))
	}
	return nil
}
