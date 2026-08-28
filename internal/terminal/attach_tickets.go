package terminal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	attachTicketTTL      = 30 * time.Second
	attachTicketCapacity = 256
	attachTicketPrefix   = "tkt-"
	attachTicketBytes    = 24
)

type attachTicketDigest [sha256.Size]byte

type attachTicketRegistry struct {
	mu      sync.Mutex
	tickets map[attachTicketDigest]AttachTicket
	order   []attachTicketDigest
}

func newAttachTicketRegistry() *attachTicketRegistry {
	return &attachTicketRegistry{
		tickets: make(map[attachTicketDigest]AttachTicket),
		order:   make([]attachTicketDigest, 0, attachTicketCapacity),
	}
}

func (m *Service) MintAttachTicket(
	ctx context.Context,
	binding AttachTicketBinding,
	actor Actor,
) (AttachTicket, error) {
	if err := requestContextError(ctx, "mint attach ticket"); err != nil {
		return AttachTicket{}, err
	}
	if err := validateAttachTicketBinding(binding); err != nil {
		return AttachTicket{}, err
	}
	if err := m.admit(ctx, binding.WorkspaceID, actor); err != nil {
		return AttachTicket{}, err
	}
	if strings.TrimSpace(actor.ProfileID) == "" || actor.ProfileID != binding.ProfileID {
		return AttachTicket{}, newTerminalError(
			ErrorCodeTicketInvalid,
			"terminal attach actor does not match ticket scope",
			ErrTicketInvalid,
		)
	}
	handle, err := m.Handle(ctx, binding.WorkspaceID, binding.ProfileID, binding.TerminalID)
	if err != nil {
		return AttachTicket{}, err
	}
	if handle.Info().State != terminalStateRunning {
		return AttachTicket{}, newTerminalError(ErrorCodeExited, errorMessageExited, ErrExited)
	}
	if err := authorizeAttachTicketActor(handle.Info(), actor); err != nil {
		return AttachTicket{}, err
	}
	settings, err := m.settings(ctx, binding.WorkspaceID, binding.ProfileID)
	if err != nil {
		return AttachTicket{}, fmt.Errorf("terminal: load attach settings: %w", err)
	}
	viewers := handle.Info().Viewers
	if settings.MaxSubscribers > 0 && viewers >= settings.MaxSubscribers {
		return AttachTicket{}, &Error{
			Code: ErrorCodeSubscriberLimitReached, Message: "terminal subscriber limit reached",
			Current: viewers, Max: settings.MaxSubscribers, Err: ErrSubscriberLimit,
		}
	}
	token, err := mintAttachTicketToken(m.entropy)
	if err != nil {
		return AttachTicket{}, err
	}
	ticket := AttachTicket{
		Token: token, Binding: binding, Actor: actor, ExpiresAt: m.now().Add(attachTicketTTL),
	}
	m.tickets.mint(ticket, m.now())
	return ticket, nil
}

func authorizeAttachTicketActor(info Info, actor Actor) error {
	if actor.Kind != ActorKindAgent || info.BoundRun == nil {
		return nil
	}
	bound := info.BoundRun
	if actor.SessionID != bound.SessionID || actor.RunID != bound.RunID {
		return &Error{
			Code: ErrorCodeLeaseRevoked, Message: "terminal is bound to a different agent run",
			Controller: info.Controller, Err: ErrLeaseRevoked,
		}
	}
	if actor.Generation != bound.Generation {
		return &Error{
			Code: ErrorCodeGenerationFenced, Message: errorMessageGenerationFenced,
			Controller: info.Controller, Err: ErrGenerationFenced,
		}
	}
	return nil
}

func (m *Service) AttachWithTicket(
	ctx context.Context,
	token string,
	workspaceID string,
	terminalID ID,
	mode string,
	options AttachOptions,
) (Handle, Subscription, AttachTicket, error) {
	if err := requestContextError(ctx, "attach with ticket"); err != nil {
		return nil, nil, AttachTicket{}, err
	}
	// Claim before attachment so a valid token remains single-use even when Handle or Attach fails.
	// Callers must mint a new ticket after any post-claim failure.
	ticket, err := m.tickets.claimStream(token, workspaceID, terminalID, mode, m.now())
	if err != nil {
		return nil, nil, AttachTicket{}, err
	}
	binding := ticket.Binding
	handle, err := m.Handle(ctx, binding.WorkspaceID, binding.ProfileID, binding.TerminalID)
	if err != nil {
		return nil, nil, AttachTicket{}, err
	}
	options.Mode = binding.Mode
	options.Actor = ticket.Actor
	subscription, err := handle.Attach(ctx, options)
	if err != nil {
		return nil, nil, AttachTicket{}, err
	}
	return handle, subscription, ticket, nil
}

func (m *Service) invalidateAttachTicketsOnClose(_ context.Context, event Event) {
	if event.Kind != EventKindClosed || m == nil || m.tickets == nil {
		return
	}
	m.tickets.invalidate(func(binding AttachTicketBinding) bool {
		return binding.WorkspaceID == event.WorkspaceID && binding.ProfileID == event.ProfileID &&
			binding.TerminalID == event.TerminalID
	})
}

func validateAttachTicketBinding(binding AttachTicketBinding) error {
	if strings.TrimSpace(binding.WorkspaceID) == "" || strings.TrimSpace(binding.ProfileID) == "" ||
		strings.TrimSpace(string(binding.TerminalID)) == "" {
		return newTerminalError(ErrorCodeTicketInvalid, "terminal attach ticket scope is incomplete", ErrTicketInvalid)
	}
	if binding.Mode != terminalAccessWrite && binding.Mode != terminalAccessRead {
		return newTerminalError(ErrorCodeTicketInvalid, "terminal attach ticket mode is invalid", ErrTicketInvalid)
	}
	return nil
}

func mintAttachTicketToken(entropy io.Reader) (string, error) {
	bytes := make([]byte, attachTicketBytes)
	if _, err := io.ReadFull(entropy, bytes); err != nil {
		return "", fmt.Errorf("terminal: mint attach ticket: %w", err)
	}
	return attachTicketPrefix + hex.EncodeToString(bytes), nil
}

func (r *attachTicketRegistry) mint(ticket AttachTicket, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepExpiredLocked(now)
	for len(r.tickets) >= attachTicketCapacity && len(r.order) > 0 {
		r.deleteLocked(r.order[0])
	}
	digest := attachTicketTokenDigest(ticket.Token)
	stored := ticket
	stored.Token = ""
	r.tickets[digest] = stored
	r.order = append(r.order, digest)
}

func (r *attachTicketRegistry) claimStream(
	token, workspaceID string,
	terminalID ID,
	mode string,
	now time.Time,
) (AttachTicket, error) {
	digest, ok := parseAttachTicketDigest(token)
	if !ok {
		return AttachTicket{}, newTerminalError(
			ErrorCodeTicketInvalid,
			"terminal attach ticket is invalid",
			ErrTicketInvalid,
		)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ticket, exists := r.tickets[digest]
	if !exists {
		return AttachTicket{}, newTerminalError(
			ErrorCodeTicketInvalid,
			"terminal attach ticket is invalid",
			ErrTicketInvalid,
		)
	}
	if !now.Before(ticket.ExpiresAt) {
		r.deleteLocked(digest)
		return AttachTicket{}, newTerminalError(
			ErrorCodeTicketExpired,
			"terminal attach ticket has expired",
			ErrTicketExpired,
		)
	}
	if ticket.Binding.WorkspaceID != workspaceID || ticket.Binding.TerminalID != terminalID ||
		ticket.Binding.Mode != mode {
		return AttachTicket{}, newTerminalError(
			ErrorCodeTicketInvalid,
			"terminal attach ticket scope does not match",
			ErrTicketInvalid,
		)
	}
	r.deleteLocked(digest)
	ticket.Token = token
	return ticket, nil
}

func (r *attachTicketRegistry) invalidate(matches func(AttachTicketBinding) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for digest, ticket := range r.tickets {
		if matches(ticket.Binding) {
			r.deleteLocked(digest)
		}
	}
}

func (r *attachTicketRegistry) sweepExpiredLocked(now time.Time) {
	for _, digest := range append([]attachTicketDigest(nil), r.order...) {
		if ticket, ok := r.tickets[digest]; ok && !now.Before(ticket.ExpiresAt) {
			r.deleteLocked(digest)
		}
	}
}

func (r *attachTicketRegistry) deleteLocked(digest attachTicketDigest) {
	delete(r.tickets, digest)
	for index, candidate := range r.order {
		if candidate != digest {
			continue
		}
		copy(r.order[index:], r.order[index+1:])
		r.order[len(r.order)-1] = attachTicketDigest{}
		r.order = r.order[:len(r.order)-1]
		return
	}
}

func parseAttachTicketDigest(token string) (attachTicketDigest, bool) {
	const encodedLength = attachTicketBytes * 2
	if len(token) != len(attachTicketPrefix)+encodedLength || !strings.HasPrefix(token, attachTicketPrefix) {
		return attachTicketDigest{}, false
	}
	decoded, err := hex.DecodeString(token[len(attachTicketPrefix):])
	if err != nil || len(decoded) != attachTicketBytes {
		return attachTicketDigest{}, false
	}
	return attachTicketTokenDigest(token), true
}

func attachTicketTokenDigest(token string) attachTicketDigest {
	return attachTicketDigest(sha256.Sum256([]byte(token)))
}
