package account

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"maps"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/sse"
)

const (
	checkoutInterest        = "checkout"
	approvalResultRetention = 24 * time.Hour
)

type cloudFeed struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func (s *Service) holdFeed(g *generation, key string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || g != s.generation {
		return
	}
	if _, done := g.results[key]; done {
		return
	}
	if key == checkoutInterest && s.installation.Profile.Plan == "pro" {
		return
	}
	unavailableAfter := s.clock.Now().Add(duration)
	g.pending[key] = unavailableAfter
	s.updateFeedLocked(g)
	s.workers.Go(func() {
		if !s.wait(g.ctx, max(0, s.clock.Until(unavailableAfter))) {
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if g.pending[key].Equal(unavailableAfter) {
			delete(g.pending, key)
			s.updateFeedLocked(g)
		}
	})
}

func (s *Service) updateFeedLocked(g *generation) {
	if s.closed || g != s.generation {
		return
	}
	wanted := len(g.subscribers) > 0 || len(g.pending) > 0
	switch {
	case wanted && g.feed == nil:
		ctx, cancel := context.WithCancel(g.ctx)
		feed := &cloudFeed{ctx: ctx, cancel: cancel}
		g.feed = feed
		s.workers.Go(func() { s.runFeed(g, feed) })
	case !wanted && g.feed != nil:
		g.feed.cancel()
	}
}

func (s *Service) runFeed(g *generation, feed *cloudFeed) {
	defer func() {
		feed.cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		g.feed = nil
		s.updateFeedLocked(g)
	}()
	pause := time.Second
	for feed.ctx.Err() == nil {
		connected, err := s.receiveFeed(feed.ctx, g)
		if connected {
			pause = time.Second
		}
		if !s.wait(feed.ctx, max(pause, cloudclient.RetryDelay(s.clock.Now(), err))) {
			return
		}
		pause = min(30*time.Second, 2*pause)
	}
}

func (s *Service) receiveFeed(ctx context.Context, g *generation) (connected bool, err error) {
	creds, err := s.refresh(g, true)
	if err != nil {
		return false, err
	}
	defer func() {
		if shouldRevalidate(err) {
			s.expireAccess(g, creds.AccessToken)
		}
	}()
	body, err := s.cloud.OpenAccountEvents(ctx, creds)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, body.Close()) }()
	// Subscribe before reading snapshots so an intervening result is observed on at least one path.
	if err := s.syncProfile(ctx, g, creds, true); err != nil {
		return true, err
	}
	approvals, err := s.cloud.Approvals(ctx, creds)
	if err != nil {
		return true, err
	}
	for _, approval := range approvals {
		if approval.AppliedResult != nil {
			s.publishResult(g, approval.ID, *approval.AppliedResult)
		}
	}
	return true, sse.Decode(ctx, body, func(frame sse.Event) error {
		return s.handleCloudEvent(ctx, g, creds, frame)
	})
}

func (s *Service) handleCloudEvent(
	ctx context.Context,
	g *generation,
	creds cloudclient.Credentials,
	frame sse.Event,
) error {
	var payload remoteproto.CloudAccountEvent
	if err := json.Unmarshal(frame.Data, &payload); err != nil {
		return err
	}
	switch frame.Event {
	case "approval.applied":
		s.publishResult(g, payload.ApprovalID, payload.Result)
		return nil
	case "plan.changed", "devices.changed", "computers.changed":
		if err := s.syncProfile(ctx, g, creds, false); err != nil {
			return err
		}
	case "approval.pending", "approval_decided":
	default:
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emitLocked(g, Change{Kind: frame.Event, ApprovalID: payload.ApprovalID, ComputerID: payload.ComputerID,
		DeviceID: payload.DeviceID, SecurityEpoch: payload.SecurityEpoch})
	return nil
}

func (s *Service) syncProfile(ctx context.Context, g *generation, creds cloudclient.Credentials, emit bool) error {
	summary, err := s.cloud.Account(ctx, creds)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.generation != g || ctx.Err() != nil {
		return ErrSignInRequired
	}
	next := s.installation
	if summary.Device.ID != next.Profile.Device.ID {
		return ErrSignInRequired
	}
	if summary.Plan == "pro" {
		delete(g.pending, checkoutInterest)
		s.updateFeedLocked(g)
	}
	if sameProfile(summary, next.Profile) {
		return nil
	}
	next.Profile = summary
	if err := s.store.SaveInstallationAccount(ctx, next.AuthGeneration, next); err != nil {
		return err
	}
	s.installation = next
	if emit {
		s.emitLocked(g, Change{Kind: "profile"})
	}
	return nil
}

func sameProfile(a, b remoteproto.AccountSummary) bool {
	a.Device.LastSeenAt, b.Device.LastSeenAt = time.Time{}, time.Time{}
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	return err == nil && bytes.Equal(left, right)
}

func (s *Service) publishResult(g *generation, id, result string) {
	if id == "" || result == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.generation != g {
		return
	}
	now := s.clock.Now()
	maps.DeleteFunc(
		g.results,
		func(_ string, seen seenResult) bool { return now.Sub(seen.at) >= approvalResultRetention },
	)
	if g.results[id].value != result {
		result := &ApprovalResult{ID: id, Result: result}
		s.emitLocked(g, Change{Kind: "approval.applied", Approval: result})
		g.results[id] = seenResult{value: result.Result, at: now}
	}
	delete(g.pending, id)
	s.updateFeedLocked(g)
}
