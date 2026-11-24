package remotehost

import (
	"context"
	"crypto/ed25519"
	"slices"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (h *Host) disableListeners(ctx context.Context, cfg config.RemoteConfig) error {
	turnOffHosted := h.config.CloudRoute.Enabled && !cfg.CloudRoute.Enabled && h.identity.ComputerID != ""
	if h.run != nil {
		run := h.run
		if turnOffHosted {
			if err := h.withdrawHostedRoute(ctx, run); err != nil {
				return err
			}
		}
		if err := h.cloud.UpdateRemoteAccess(ctx, h.identity.ComputerID, run.control.key, false, nil); err != nil {
			if turnOffHosted {
				h.restartHostedRoute(run)
			}
			return err
		}
		h.stopLocked()
	} else if turnOffHosted {
		keys, err := loadHostKeys(h.account, h.identity, h.opts.Now())
		if err != nil {
			return err
		}
		if err := h.deleteHostedTunnel(ctx, keys.signing); err != nil {
			return err
		}
	}
	return h.clearIngress(ctx)
}

func (h *Host) reconfigure(ctx context.Context, cfg config.RemoteConfig, enablingHosted bool) error {
	run := h.run
	hostedChanged := h.config.HostedListener != cfg.HostedListener || h.config.CloudRoute != cfg.CloudRoute
	if h.config.Listener != cfg.Listener || hostedChanged || h.config.Direct.Enabled != cfg.Direct.Enabled ||
		!slices.Equal(h.config.Direct.Addresses, cfg.Direct.Addresses) {
		if err := h.reconfigureListeners(ctx, run, cfg, hostedChanged, enablingHosted); err != nil {
			return err
		}
	}
	run.setIngressConfig(cfg)
	run.syncIngress(ctx)
	return run.sessions.Reconfigure(ctx, cfg.RequireDeviceApproval, cfg.MaxScopes)
}

func (h *Host) reconfigureListeners(
	ctx context.Context, run *hostRun, cfg config.RemoteConfig, hostedChanged, enablingHosted bool,
) error {
	off := h.config.CloudRoute.Enabled && !cfg.CloudRoute.Enabled
	if off {
		if err := h.withdrawHostedRoute(ctx, run); err != nil {
			return err
		}
	}
	var hosted string
	var initial remoteproto.HostedTunnel
	publish := func(endpoints []string, bound string) error {
		hosted = bound
		if h.config.Direct.Enabled || cfg.Direct.Enabled {
			if err := h.cloud.UpdateRemoteAccess(
				ctx,
				h.identity.ComputerID,
				run.control.key,
				true,
				endpoints,
			); err != nil {
				return err
			}
		}
		if enablingHosted {
			var err error
			initial, err = h.provisionHosted(ctx, run, hosted)
			return err
		}
		return nil
	}
	if err := run.listeners.apply(run.ctx, cfg, run.handler, publish); err != nil {
		if off {
			h.restartHostedRoute(run)
		}
		return err
	}
	if hostedChanged {
		h.replaceHostedRoute(run, hosted, initial)
	}
	return nil
}

func (h *Host) withdrawHostedRoute(ctx context.Context, run *hostRun) error {
	run.stopHostedRoute()
	if err := h.deleteHostedTunnel(ctx, run.control.key); err != nil {
		h.restartHostedRoute(run)
		return err
	}
	return nil
}

func (h *Host) restartHostedRoute(run *hostRun) {
	if addresses := run.listeners.addresses(ListenerHosted); len(addresses) > 0 {
		h.replaceHostedRoute(run, addresses[0], remoteproto.HostedTunnel{})
	}
}

func (h *Host) deleteHostedTunnel(ctx context.Context, key ed25519.PrivateKey) error {
	if cloud, ok := h.cloud.(hostedCloud); ok {
		return cloud.DeleteHostedTunnel(ctx, h.identity.ComputerID, key)
	}
	return nil
}
