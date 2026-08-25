package core

import (
	"container/list"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

const (
	terminalTicketTTL      = 30 * time.Second
	terminalTicketCapacity = 256
)

var (
	errTerminalTicketInvalid = errors.New("terminal attach ticket is invalid")
	errTerminalTicketExpired = errors.New("terminal attach ticket is expired")
)

type terminalTicketBinding struct {
	WorkspaceID string
	ProfileID   string
	TerminalID  terminalpkg.ID
	Mode        string
}

type terminalTicket struct {
	Token     string
	Binding   terminalTicketBinding
	Actor     terminalpkg.Actor
	CreatedAt time.Time
	ExpiresAt time.Time
	group     *terminalTicketGroup
}

type terminalTicketGroup struct {
	invalidUntil atomic.Int64
	count        atomic.Int64
}

type terminalTicketDigest [sha256.Size]byte

type terminalTicketStore struct {
	now func() time.Time

	mu        sync.Mutex
	tickets   map[terminalTicketDigest]terminalTicket
	byTarget  sync.Map
	order     *list.List
	positions map[terminalTicketDigest]*list.Element
}

func newTerminalTicketStore(provider TerminalProvider, now func() time.Time) *terminalTicketStore {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	store := &terminalTicketStore{
		now: now, tickets: make(map[terminalTicketDigest]terminalTicket),
		order: list.New(), positions: make(map[terminalTicketDigest]*list.Element),
	}
	provider.Observe(store.observe)
	return store
}

func (s *terminalTicketStore) Mint(binding terminalTicketBinding, actor terminalpkg.Actor) (terminalTicket, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return terminalTicket{}, fmt.Errorf("terminal ticket: generate token: %w", err)
	}
	now := s.now()
	ticket := terminalTicket{
		Token: "tkt-" + hex.EncodeToString(tokenBytes), Binding: binding,
		Actor:     actor,
		CreatedAt: now, ExpiresAt: now.Add(terminalTicketTTL),
	}
	s.mu.Lock()
	s.sweepExpiredLocked(now)
	s.sweepInvalidTargetsLocked(now)
	for len(s.tickets) >= terminalTicketCapacity && s.order.Len() > 0 {
		s.deleteLocked(s.order.Front().Value.(terminalTicketDigest))
	}
	group := s.targetGroupLocked(binding)
	if group.invalidUntil.Load() != 0 {
		s.mu.Unlock()
		return terminalTicket{}, errTerminalTicketInvalid
	}
	group.count.Add(1)
	ticket.group = group
	digest := terminalTicketDigest(sha256.Sum256([]byte(ticket.Token)))
	stored := ticket
	stored.Token = ""
	s.tickets[digest] = stored
	s.positions[digest] = s.order.PushBack(digest)
	s.mu.Unlock()
	return ticket, nil
}

func (s *terminalTicketStore) Consume(token string, expected terminalTicketBinding) (terminalTicket, error) {
	ticket, err := s.consume(token)
	if err != nil {
		return terminalTicket{}, err
	}
	if ticket.Binding != expected {
		return terminalTicket{}, errTerminalTicketInvalid
	}
	return ticket, nil
}

func (s *terminalTicketStore) ConsumeStream(
	token, workspaceID string,
	terminalID terminalpkg.ID,
	mode string,
) (terminalTicket, error) {
	ticket, err := s.consume(token)
	if err != nil {
		return terminalTicket{}, err
	}
	if ticket.Binding.WorkspaceID != workspaceID || ticket.Binding.TerminalID != terminalID || ticket.Binding.Mode != mode {
		return terminalTicket{}, errTerminalTicketInvalid
	}
	return ticket, nil
}

func (s *terminalTicketStore) consume(token string) (terminalTicket, error) {
	digest, validFormat := terminalTicketDigestFromToken(token)
	if !validFormat {
		return terminalTicket{}, errTerminalTicketInvalid
	}
	now := s.now()
	s.mu.Lock()
	ticket, ok := s.tickets[digest]
	valid := ok && ticket.group != nil && ticket.group.invalidUntil.Load() == 0
	if ok {
		s.deleteLocked(digest)
	}
	s.mu.Unlock()
	if !ok {
		return terminalTicket{}, errTerminalTicketInvalid
	}
	if !now.Before(ticket.ExpiresAt) {
		return terminalTicket{}, errTerminalTicketExpired
	}
	if !valid {
		return terminalTicket{}, errTerminalTicketInvalid
	}
	return ticket, nil
}

func terminalTicketDigestFromToken(token string) (terminalTicketDigest, bool) {
	const encodedBytes = 24 * 2
	if len(token) != len("tkt-")+encodedBytes || !strings.HasPrefix(token, "tkt-") {
		return terminalTicketDigest{}, false
	}
	decoded, err := hex.DecodeString(token[len("tkt-"):])
	if err != nil || len(decoded) != 24 {
		return terminalTicketDigest{}, false
	}
	return terminalTicketDigest(sha256.Sum256([]byte(token))), true
}

func (s *terminalTicketStore) observe(_ context.Context, event terminalpkg.TerminalEvent) {
	if event.Kind != terminalpkg.EventKindClosed {
		return
	}
	invalidUntil := s.now().Add(terminalTicketTTL).UnixNano()
	for _, mode := range []string{"read", "write"} {
		binding := terminalTicketBinding{
			WorkspaceID: event.WorkspaceID, ProfileID: event.ProfileID,
			TerminalID: event.TerminalID, Mode: mode,
		}
		group := &terminalTicketGroup{}
		actual, _ := s.byTarget.LoadOrStore(binding, group)
		group = actual.(*terminalTicketGroup)
		group.invalidUntil.Store(invalidUntil)
		time.AfterFunc(terminalTicketTTL, func() { s.expireTargetTombstone(binding, group, invalidUntil) })
	}
}

func (s *terminalTicketStore) expireTargetTombstone(
	binding terminalTicketBinding,
	group *terminalTicketGroup,
	invalidUntil int64,
) {
	if group.count.Load() == 0 && group.invalidUntil.Load() == invalidUntil {
		s.byTarget.CompareAndDelete(binding, group)
	}
}

func (s *terminalTicketStore) targetGroupLocked(binding terminalTicketBinding) *terminalTicketGroup {
	if value, ok := s.byTarget.Load(binding); ok {
		return value.(*terminalTicketGroup)
	}
	group := &terminalTicketGroup{}
	actual, _ := s.byTarget.LoadOrStore(binding, group)
	return actual.(*terminalTicketGroup)
}

func (s *terminalTicketStore) sweepInvalidTargetsLocked(now time.Time) {
	nowUnix := now.UnixNano()
	s.byTarget.Range(func(key, value any) bool {
		group := value.(*terminalTicketGroup)
		invalidUntil := group.invalidUntil.Load()
		if group.count.Load() == 0 && invalidUntil > 0 && invalidUntil <= nowUnix {
			s.byTarget.CompareAndDelete(key, group)
		}
		return true
	})
}

func (s *terminalTicketStore) sweepExpiredLocked(now time.Time) {
	for element := s.order.Front(); element != nil; {
		next := element.Next()
		digest := element.Value.(terminalTicketDigest)
		ticket, ok := s.tickets[digest]
		if ok && !now.Before(ticket.ExpiresAt) {
			s.deleteLocked(digest)
		}
		element = next
	}
}

func (s *terminalTicketStore) deleteLocked(digest terminalTicketDigest) {
	ticket, ok := s.tickets[digest]
	if !ok {
		return
	}
	delete(s.tickets, digest)
	if ticket.group != nil {
		remaining := ticket.group.count.Add(-1)
		if remaining == 0 && ticket.group.invalidUntil.Load() == 0 {
			s.byTarget.CompareAndDelete(ticket.Binding, ticket.group)
		}
	}
	if position := s.positions[digest]; position != nil {
		s.order.Remove(position)
		delete(s.positions, digest)
	}
}
