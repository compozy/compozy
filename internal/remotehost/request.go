package remotehost

import (
	"context"
	"errors"
	"sync"

	"github.com/compozy/compozy/internal/store"
)

type requestKey struct{}

type liveRequest struct {
	principal Principal
	cancel    context.CancelCauseFunc
	mu        sync.Mutex
	onClose   func(int, string)
	reason    error
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	live, ok := ctx.Value(requestKey{}).(*liveRequest)
	if !ok {
		return Principal{}, false
	}
	return live.principal, true
}

// OnStreamClose sends a policy close on an upgraded stream before returning its acknowledgement.
func OnStreamClose(ctx context.Context, closeStream func(int, string)) {
	live, ok := ctx.Value(requestKey{}).(*liveRequest)
	if !ok {
		return
	}
	live.mu.Lock()
	reason := live.reason
	if reason == nil {
		live.onClose = closeStream
	}
	live.mu.Unlock()
	if reason != nil {
		closeStream(streamClose(reason))
	}
}

func (s *Sessions) BeginRequest(
	ctx context.Context,
	principal Principal,
	mutation bool,
) (context.Context, func(), error) {
	s.serial.Lock()
	defer s.serial.Unlock()
	session, err := s.store.RemoteSession(ctx, principal.session.CredentialHash)
	if err != nil {
		return nil, nil, err
	}
	if err := s.checkDevice(ctx, session); err != nil {
		return nil, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.admission.check(session, s.now()); err != nil {
		return nil, nil, err
	}
	principal.epoch = s.admission.snapshot.SecurityEpoch
	// #nosec G118 -- the registered request owns cancel until release or revocation.
	request, cancel := context.WithCancelCause(ctx)
	live := &liveRequest{principal: principal, cancel: cancel}
	s.live[live] = struct{}{}
	request = context.WithValue(request, requestKey{}, live)
	if mutation {
		request = store.ContextWithMutationCommitGuard(request, s.acquireCommit)
	}
	release := func() {
		s.mu.Lock()
		delete(s.live, live)
		s.mu.Unlock()
		cancel(nil)
	}
	return request, release, nil
}

func (s *Sessions) acquireCommit(ctx context.Context) (func(), error) {
	s.mu.RLock()
	p, ok := PrincipalFromContext(ctx)
	err := s.commitError(ctx, p, ok)
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	return s.mu.RUnlock, nil
}

func (s *Sessions) commitError(ctx context.Context, p Principal, ok bool) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if !ok || p.epoch != s.admission.snapshot.SecurityEpoch || s.admission.blocked {
		return ErrDeviceRevoked
	}
	return s.admission.check(p.session, s.now())
}

func (s *Sessions) endRequests(match func(Session) error) int {
	type closedRequest struct {
		live   *liveRequest
		reason error
	}
	s.mu.Lock()
	var closed []closedRequest
	for live := range s.live {
		if reason := match(live.principal.session); reason != nil {
			closed = append(closed, closedRequest{live, reason})
			delete(s.live, live)
		}
	}
	s.mu.Unlock()
	for _, entry := range closed {
		entry.live.end(entry.reason)
	}
	return len(closed)
}

func (r *liveRequest) end(reason error) {
	r.mu.Lock()
	r.reason = reason
	closeStream := r.onClose
	r.onClose = nil
	r.mu.Unlock()
	if closeStream != nil {
		closeStream(streamClose(reason))
	}
	r.cancel(reason)
}

func streamClose(err error) (int, string) {
	if errors.Is(err, ErrEntitlement) || errors.Is(err, ErrPlanRequired) {
		return 4402, DescribeError(err).Code
	}
	if errors.Is(err, ErrSessionExpired) {
		return 4401, ErrSessionExpired.Code
	}
	return 4503, "device_revoked"
}
