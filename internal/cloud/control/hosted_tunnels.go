// SPDX-License-Identifier: FSL-1.1-MIT

package control

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloud/cloudflare"
	"github.com/compozy/compozy/internal/cloud/plan"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	tunnelReserved     = "reserved"
	tunnelProvisioning = "provisioning"
	tunnelActive       = "active"
	tunnelFailed       = "failed"
	tunnelDeleting     = "deleting"
	tunnelDeleted      = "deleted"
	hostedWorkTimeout  = 22 * time.Second
)

var errHostedUnavailable = &Error{
	http.StatusServiceUnavailable, "cloud_unavailable", "The hosted route is temporarily unavailable.",
}

var errHostedTakenDown = &Error{
	http.StatusForbidden, "hosted_route_taken_down", "This hosted route was removed. Contact support@compozy.com.",
}

func (s *Service) ProvisionHostedTunnel(
	ctx context.Context,
	computer store.ControlComputer,
	proof ComputerRequestAuth,
	request remoteproto.HostedTunnelRequest,
) (remoteproto.HostedTunnel, error) {
	origin, valid := request.Origin()
	if !valid {
		return remoteproto.HostedTunnel{}, ErrInvalidInput
	}
	if err := s.consumeComputerProof(ctx, computer, proof); err != nil {
		return remoteproto.HostedTunnel{}, err
	}

	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), hostedWorkTimeout)
	defer cancel()
	tunnel, err := s.reserveHostedTunnel(work, computer)
	if err != nil {
		return remoteproto.HostedTunnel{}, err
	}
	if tunnel.State == tunnelDeleting || tunnel.State == tunnelFailed {
		if err := s.DeleteHostedGeneration(work, tunnel.TunnelRef); err != nil {
			return remoteproto.HostedTunnel{}, errHostedUnavailable
		}
		tunnel, err = s.reserveHostedTunnel(work, computer)
		if err != nil {
			return remoteproto.HostedTunnel{}, err
		}
	}
	return s.applyHostedTunnel(work, computer, tunnel.TunnelRef, origin)
}

func (s *Service) reserveHostedTunnel(
	ctx context.Context, computer store.ControlComputer,
) (tunnel store.HostedTunnel, err error) {
	err = s.accountTransaction(ctx, computer.AccountID, func(q *store.Queries, account store.ControlAccount) error {
		if err := activeAccount(account); err != nil {
			return err
		}
		current, err := ownedComputer(ctx, q, account.ID, computer.ID)
		if err != nil {
			return err
		}
		if !current.RemoteEnabled {
			return errComputerDisabled
		}
		now := s.config.Now()
		tunnel, err = q.LockHostedTunnel(ctx, current.ID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if tunnel.TakenDown {
			return errHostedTakenDown
		}
		if _, err := s.hostedTunnelMode(ctx, q, account.ID, tunnel, now); err != nil {
			return err
		}
		if s.config.HostedTunnels == nil {
			return errHostedUnavailable
		}
		if tunnel.State != "" && tunnel.State != tunnelDeleted {
			return nil
		}
		if err := admitHostedTunnel(ctx, q, account.ID); err != nil {
			return err
		}
		if tunnel.Generation > 0 {
			tunnel, err = q.ReserveHostedTunnel(ctx, tunnel.TunnelRef, now)
		} else {
			tunnel, err = s.insertHostedTunnel(ctx, q, current, now)
		}
		return err
	})
	return tunnel, err
}

func (s *Service) hostedTunnelMode(
	ctx context.Context, q *store.Queries, accountID string, tunnel store.HostedTunnel, now time.Time,
) (string, error) {
	subscriptions, err := q.Subscriptions(ctx, accountID)
	if err != nil {
		return "", err
	}
	if plan.Derive(subscriptions, s.config.MonthlyPrice, s.config.AnnualPrice, now) == plan.Pro {
		return remoteproto.HostedTunnelActive, nil
	}
	if tunnel.State == tunnelActive && now.Before(tunnel.DeleteAfter) {
		return remoteproto.HostedTunnelEntitlementOnly, nil
	}
	s.config.Metrics.Paywall(errCloudPlanRequired.Code)
	return "", errCloudPlanRequired
}

func admitHostedTunnel(ctx context.Context, q *store.Queries, accountID string) error {
	account, err := q.LockAuthAccount(ctx, accountID)
	if err != nil {
		return err
	}
	used, err := q.AccountHostedRouteUsage(ctx, accountID)
	if err != nil {
		return err
	}
	if used >= int64(account.CloudRoutesMax) {
		return &Error{
			http.StatusPaymentRequired,
			"cloud_routes_limit_reached",
			fmt.Sprintf(
				"Pro includes %d computers on the hosted route. Contact support to raise it.",
				account.CloudRoutesMax,
			),
		}
	}
	return nil
}

func (s *Service) insertHostedTunnel(
	ctx context.Context, q *store.Queries, computer store.ControlComputer, now time.Time,
) (store.HostedTunnel, error) {
	id := strings.TrimPrefix(string(computer.ID), "cmp_")
	for offset := 0; offset+4 <= len(id); offset++ {
		hostname := hostedHostname(computer.Name, id[offset:offset+4], s.config.HostedTunnels.Domain())
		tunnel, err := q.InsertHostedTunnel(ctx, computer.ID, hostname, now)
		if !errors.Is(err, store.ErrNotFound) {
			return tunnel, err
		}
	}
	return store.HostedTunnel{}, errHostedUnavailable
}

func (s *Service) applyHostedTunnel(
	ctx context.Context, computer store.ControlComputer, ref store.TunnelRef, origin string,
) (remoteproto.HostedTunnel, error) {
	if err := s.beginHostedProvision(ctx, ref); err != nil {
		return remoteproto.HostedTunnel{}, err
	}
	var result remoteproto.HostedTunnel
	err := s.accountTransaction(ctx, computer.AccountID, func(q *store.Queries, account store.ControlAccount) error {
		if err := activeAccount(account); err != nil {
			return err
		}
		current, err := ownedComputer(ctx, q, account.ID, computer.ID)
		if err != nil {
			return err
		}
		if !current.RemoteEnabled {
			return errComputerDisabled
		}
		tunnel, err := q.LockHostedTunnel(ctx, ref.ComputerID)
		if err != nil {
			return err
		}
		if tunnel.Generation != ref.Generation || (tunnel.State != tunnelActive && tunnel.State != tunnelProvisioning) {
			return errHostedUnavailable
		}
		mode, err := s.hostedTunnelMode(ctx, q, account.ID, tunnel, s.config.Now())
		if err != nil {
			return err
		}
		result, err = s.provisionGeneration(ctx, q, current, tunnel, origin, mode)
		return err
	})
	if err != nil {
		return remoteproto.HostedTunnel{}, err
	}
	if result.ConnectorToken == "" {
		return remoteproto.HostedTunnel{}, errHostedUnavailable
	}
	return result, nil
}

func (s *Service) beginHostedProvision(ctx context.Context, ref store.TunnelRef) error {
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		tunnel, err := q.LockHostedTunnel(ctx, ref.ComputerID)
		if err != nil || tunnel.Generation != ref.Generation || tunnel.State != tunnelReserved {
			return err
		}
		tunnel.State = tunnelProvisioning
		_, err = q.UpdateHostedTunnel(ctx, tunnel)
		return err
	})
}

func (s *Service) provisionGeneration(
	ctx context.Context,
	q *store.Queries,
	computer store.ControlComputer,
	tunnel store.HostedTunnel,
	origin, mode string,
) (remoteproto.HostedTunnel, error) {
	provider := s.config.HostedTunnels
	result := remoteproto.HostedTunnel{Hostname: tunnel.Hostname, Mode: mode}
	if tunnel.State == tunnelActive {
		var newOrigin string
		if tunnel.Origin != origin {
			newOrigin = origin
		}
		token, err := provider.Credentials(ctx, tunnel.Hostname, tunnel.Generation, tunnel.CloudflareID, newOrigin)
		if err != nil {
			return remoteproto.HostedTunnel{}, errHostedUnavailable
		}
		if newOrigin != "" {
			tunnel.Origin = origin
			if _, err := q.UpdateHostedTunnel(ctx, tunnel); err != nil {
				return remoteproto.HostedTunnel{}, err
			}
		}
		result.ConnectorToken = token
		return result, nil
	}
	call, cancel := context.WithTimeout(ctx, 17*time.Second)
	defer cancel()
	var err error
	for range 3 {
		var id string
		id, result.ConnectorToken, err = provider.Provision(
			call,
			tunnel.Hostname,
			tunnel.Generation,
			tunnel.CloudflareID,
			origin,
		)
		if id != "" {
			tunnel.CloudflareID = id
		}
		if err == nil || call.Err() != nil || errors.Is(err, cloudflare.ErrOutsideDomain) {
			break
		}
	}
	if err != nil {
		tunnel.State = tunnelFailed
		if _, err := q.UpdateHostedTunnel(ctx, tunnel); err != nil {
			return remoteproto.HostedTunnel{}, err
		}
		return remoteproto.HostedTunnel{}, s.hostedAudit(ctx, q, computer, tunnel, "hosted_tunnel_failed")
	}
	tunnel.State, tunnel.Origin = tunnelActive, origin
	if _, err := q.UpdateHostedTunnel(ctx, tunnel); err != nil {
		return remoteproto.HostedTunnel{}, err
	}
	if err := s.hostedAudit(ctx, q, computer, tunnel, "hosted_tunnel_provisioned"); err != nil {
		return remoteproto.HostedTunnel{}, err
	}
	if err := notifyHostedChanged(ctx, q, computer); err != nil {
		return remoteproto.HostedTunnel{}, err
	}
	return result, nil
}

func (s *Service) hostedAudit(
	ctx context.Context, q *store.Queries, computer store.ControlComputer, tunnel store.HostedTunnel, kind string,
) error {
	return s.securityEvent(ctx, q, computer.AccountID, "", kind,
		map[string]any{auditComputerID: computer.ID, "generation": tunnel.Generation, "hostname": tunnel.Hostname})
}

func notifyHostedChanged(ctx context.Context, q *store.Queries, computer store.ControlComputer) error {
	if err := notifyComputerChanged(ctx, q, computer); err != nil {
		return err
	}
	return q.NotifyControlEvent(ctx, store.ControlEvent{Type: remoteproto.ControlPeers, AccountID: computer.AccountID})
}
