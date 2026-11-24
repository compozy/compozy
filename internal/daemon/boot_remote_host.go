package daemon

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/remoteproto"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

const (
	remoteConfigKey  = "remote"
	remoteEnabledKey = "enabled"
)

func (d *Daemon) bootRemoteHost(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	storage, ok := state.registry.(remotehost.Store)
	workers := &state.runtimeWorkers
	if !ok || workers.account == nil {
		return nil
	}
	host, err := remotehost.NewHost(
		ctx,
		storage,
		workers.cloud,
		workers.account,
		state.cfg.Remote,
		remotehost.HostOptions{
			Now: d.now, Logger: state.logger, Operator: remoteOperatorRoutes(&state.deps),
			Persist: d.persistRemoteConfig,
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

func remoteOperatorRoutes(deps *RuntimeDeps) http.Handler {
	acceptance, _ := deps.Sessions.(core.SessionAcceptanceManager)
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		TransportName:      remoteConfigKey,
		MaskInternalErrors: true,
		Sessions:           deps.Sessions,
		SessionAcceptance:  acceptance,
		SessionCatalog:     deps.Registry,
		Worktrees:          deps.Worktrees,
		Workspaces:         deps.WorkspaceService,
		WorkspaceAccess:    deps.WorkspaceAccess,
		Profiles:           deps.Profiles,
		Config:             deps.Config,
	})
	return httpapi.NewRemoteRoutes(func(routes *httpapi.RemoteRoutes) {
		remoteSessions := routes.Group("/api/sessions", remoteproto.ScopeSessions)
		remoteSessions.GET("", handlers.ListSessions)
		remoteSessions.POST("", handlers.CreateSession)
		remoteSessions.GET("/:session_id", handlers.GetSessionByID)
		remoteSessions.GET("/:session_id/stream", handlers.StreamSession)
		remoteSessions.GET("/:session_id/status", handlers.GetSessionStatus)
		remoteSessions.GET("/:session_id/transcript", handlers.SessionTranscript)
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
