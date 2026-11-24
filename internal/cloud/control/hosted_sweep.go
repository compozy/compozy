package control

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
)

const (
	hostedIdleLimit          = 14 * 24 * time.Hour
	hostedProvisionTimeLimit = 15 * time.Minute
)

func (s *Service) DeleteHostedTunnel(
	ctx context.Context, computer store.ControlComputer, proof ComputerRequestAuth,
) error {
	if err := s.consumeComputerProof(ctx, computer, proof); err != nil {
		return err
	}
	if s.config.HostedTunnels == nil {
		return nil
	}
	var ref store.TunnelRef
	err := s.accountTransaction(ctx, computer.AccountID, func(q *store.Queries, account store.ControlAccount) error {
		current, err := ownedComputer(ctx, q, account.ID, computer.ID)
		if err != nil {
			return err
		}
		ref, err = s.markHostedDeleting(ctx, q, current)
		return err
	})
	if err == nil {
		s.finishHostedDelete(ctx, ref)
	}
	return err
}

func (s *Service) markHostedDeleting(
	ctx context.Context, q *store.Queries, computer store.ControlComputer,
) (store.TunnelRef, error) {
	tunnel, err := q.LockHostedTunnel(ctx, computer.ID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && tunnel.State == tunnelDeleted) {
		return store.TunnelRef{}, nil
	}
	if err != nil || tunnel.State == tunnelDeleting {
		return tunnel.TunnelRef, err
	}
	return tunnel.TunnelRef, markHostedDeleting(ctx, q, computer, tunnel)
}

func markHostedDeleting(
	ctx context.Context, q *store.Queries, computer store.ControlComputer, tunnel store.HostedTunnel,
) error {
	tunnel.State = tunnelDeleting
	if _, err := q.UpdateHostedTunnel(ctx, tunnel); err != nil {
		return err
	}
	return notifyHostedChanged(ctx, q, computer)
}

func (s *Service) finishHostedDelete(ctx context.Context, ref store.TunnelRef) {
	if ref.ComputerID == "" || s.config.HostedTunnels == nil {
		return
	}
	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), hostedWorkTimeout)
	defer cancel()
	if err := s.DeleteHostedGeneration(work, ref); err != nil {
		s.config.Logger.WarnContext(ctx, "Hosted tunnel deletion will be retried", "computer_id", ref.ComputerID)
	}
}

// DeleteHostedGeneration completes a previously marked deletion for exactly that generation.
func (s *Service) DeleteHostedGeneration(ctx context.Context, ref store.TunnelRef) error {
	computer, err := s.db.ControlComputer(ctx, ref.ComputerID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.db.Transaction(ctx, func(q *store.Queries) error {
		tunnel, err := q.LockHostedTunnel(ctx, ref.ComputerID)
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if tunnel.Generation != ref.Generation || (tunnel.State != tunnelDeleting && tunnel.State != tunnelFailed) {
			return nil
		}
		if err := s.config.HostedTunnels.Delete(
			ctx,
			tunnel.Hostname,
			tunnel.Generation,
			tunnel.CloudflareID,
		); err != nil {
			return err
		}
		tunnel.State, tunnel.CloudflareID, tunnel.Origin = tunnelDeleted, "", ""
		if _, err := q.UpdateHostedTunnel(ctx, tunnel); err != nil {
			return err
		}
		return s.hostedAudit(ctx, q, computer, tunnel, "hosted_tunnel_deleted")
	})
}

func (s *Service) markSweepCandidate(ctx context.Context, computer store.ControlComputer, ref store.TunnelRef) error {
	return s.accountTransaction(ctx, computer.AccountID, func(q *store.Queries, _ store.ControlAccount) error {
		tunnel, err := q.LockHostedTunnel(ctx, ref.ComputerID)
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		if err != nil || tunnel.Generation != ref.Generation || tunnel.State == tunnelDeleting {
			return err
		}
		now := s.config.Now()
		due, err := q.HostedTunnelCanSweep(ctx, ref.ComputerID, now,
			now.Add(-hostedIdleLimit), now.Add(-hostedProvisionTimeLimit))
		if err != nil || !due {
			return err
		}
		return markHostedDeleting(ctx, q, computer, tunnel)
	})
}

func (s *Service) SweepHostedTunnels(ctx context.Context) error {
	if s.config.HostedTunnels == nil {
		return nil
	}
	now := s.config.Now()
	candidates, err := s.db.SweepableHostedTunnels(
		ctx,
		now,
		now.Add(-hostedIdleLimit),
		now.Add(-hostedProvisionTimeLimit),
	)
	if err != nil {
		return err
	}
	var failures []error
	for _, ref := range candidates {
		if ctx.Err() != nil {
			return errors.Join(append(failures, ctx.Err())...)
		}
		if err := s.sweepHostedGeneration(ctx, ref); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *Service) sweepHostedGeneration(ctx context.Context, ref store.TunnelRef) error {
	computer, err := s.db.ControlComputer(ctx, ref.ComputerID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.markSweepCandidate(ctx, computer, ref); err != nil {
		return err
	}
	return s.DeleteHostedGeneration(ctx, ref)
}

func (s *Service) LogHostedTunnelCapacity(ctx context.Context) error {
	count, err := s.db.CountAllocatedHostedTunnels(ctx)
	if err != nil {
		return err
	}
	level := slog.LevelInfo
	switch {
	case count >= 900:
		level = slog.LevelError
	case count >= 700:
		level = slog.LevelWarn
	}
	if level != slog.LevelInfo {
		s.config.Logger.Log(
			ctx,
			level,
			"Cloudflare tunnel capacity threshold reached",
			"cloud_tunnels_allocated",
			count,
		)
	}
	return nil
}

func (s *Service) attachHostedRoutes(
	ctx context.Context, q *store.Queries, accountID string, now time.Time, computers []Computer,
) error {
	hosts, err := q.ActiveHostedTunnels(ctx, accountID, now)
	if err != nil {
		return err
	}
	for i := range computers {
		computers[i].HostedHostname = hosts[computers[i].ID]
	}
	return nil
}
