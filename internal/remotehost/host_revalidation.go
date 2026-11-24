package remotehost

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
)

const dailyRevalidation = 24 * time.Hour

func (h *Host) wakeRevalidation() {
	select {
	case h.revalidateWake <- struct{}{}:
	default:
	}
}

func (h *Host) revalidateLoop() {
	var current remoteproto.ComputerID
	var next time.Time
	for h.ctx.Err() == nil {
		h.mu.Lock()
		identity := h.identity
		if h.run != nil && h.run.unlinked.Load() {
			identity = Identity{}
		}
		h.mu.Unlock()
		if identity.ComputerID == "" {
			current = ""
			select {
			case <-h.ctx.Done():
				return
			case <-h.revalidateWake:
				continue
			}
		}
		if current != identity.ComputerID {
			current = identity.ComputerID
			next = identity.LinkedAt.Add(dailyRevalidation)
			status := h.account.Status()
			if status.Source == "link" && status.LastValidatedAt.After(identity.LinkedAt) {
				next = status.LastValidatedAt.Add(dailyRevalidation)
			}
		}
		timer := h.opts.Clock.NewTimer(max(0, h.opts.Clock.Until(next)))
		select {
		case <-h.ctx.Done():
			timer.Stop()
			return
		case <-h.revalidateWake:
			timer.Stop()
		case <-timer.Chan():
			delay := dailyRevalidation
			if err := h.revalidateLink(h.ctx, identity); err != nil {
				delay = max(5*time.Minute, cloudclient.RetryDelay(h.opts.Now(), err))
				h.opts.Logger.DebugContext(h.ctx, "Computer revalidation deferred", "code", hostFailureCode(err))
			}
			next = h.opts.Clock.Now().Add(delay)
		}
	}
}

func (h *Host) revalidateLink(ctx context.Context, identity Identity) error {
	h.linkMu.Lock()
	defer h.linkMu.Unlock()
	h.mu.Lock()
	if h.closed || h.identity.ComputerID != identity.ComputerID || (h.run != nil && h.run.unlinked.Load()) {
		h.mu.Unlock()
		return nil
	}
	keys, err := loadHostKeys(h.account, identity, h.opts.Now())
	h.mu.Unlock()
	if err != nil {
		return err
	}
	installation := h.account.Status()
	err = h.cloud.RevalidateComputer(ctx, identity.ComputerID, keys.signing, installation.InstallationID)
	if err == nil {
		return h.account.ObserveHostLink(ctx, identity.AccountID, "")
	}
	failure, ok := errors.AsType[*cloudclient.Failure](err)
	if !ok {
		return err
	}
	reason := failure.Code
	if reason == "reauth_required" {
		reason = failure.Reason
	}
	switch reason {
	case "account_suspended":
		return h.account.ObserveHostLink(ctx, identity.AccountID, reason)
	case reasonComputerUnlinked, "account_deleted":
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.clearLinkedComputer(ctx, identity, reason)
	default:
		return err
	}
}
