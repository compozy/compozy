package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/apptoken"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/remotehost"
	"github.com/compozy/compozy/internal/store"
)

func (d *Daemon) bootAppAccess(ctx context.Context, state *bootState) error {
	if notices, ok := state.registry.(core.MigrationNoticeStore); ok {
		state.deps.MigrationNotices = notices
	}
	if err := d.bootAppTokens(ctx, state); err != nil {
		return err
	}
	ingressStore, ok := state.registry.(ingress.Store)
	if !ok {
		return nil
	}
	if _, err := ingressStore.SweepOrphanedIngressBindings(ctx); err != nil {
		return fmt.Errorf("daemon: sweep orphaned ingress bindings: %w", err)
	}
	audit := ingressAuditSink{now: d.now}
	if writer, ok := state.registry.(store.EventSummaryStore); ok {
		audit.writer = writer
	}
	manager, err := ingress.NewManager(
		ingressStore, state.accessPolicy, audit, d.now,
		func(ctx context.Context) (string, string, bool) {
			principal, ok := remotehost.PrincipalFromContext(ctx)
			return "remote_device", string(principal.DeviceID), ok
		},
	)
	if err != nil {
		return fmt.Errorf("daemon: create ingress service: %w", err)
	}
	state.deps.Ingress = manager
	if host := state.runtimeWorkers.remote; host != nil {
		host.SetWebhookHandler(httpapi.NewRemoteWebhookRoutes(state.deps.Automation, manager.AdmitDelivery, d.now))
	}
	return nil
}

func (d *Daemon) bootAppTokens(ctx context.Context, state *bootState) error {
	tokenStore, ok := state.registry.(interface {
		apptoken.Store
		RevalidateAppTokenEpoch(context.Context, string, uint64) error
	})
	if !ok {
		return nil
	}
	service, err := apptoken.NewService(tokenStore, remotehost.NewMutationGate(tokenStore.RevalidateAppTokenEpoch),
		remotehost.NewConnectionRegistry(), d.now)
	if err != nil {
		return fmt.Errorf("daemon: create app token service: %w", err)
	}
	if err := service.LoadAppTokenOrigins(ctx); err != nil {
		return fmt.Errorf("daemon: load app token origins: %w", err)
	}
	state.deps.AppTokens = service
	return nil
}
