package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

// Inbox returns immutable delivered messages for one durable session identity.
func (m *Manager) Inbox(ctx context.Context, sessionID string) ([]Envelope, error) {
	return m.listInbox(ctx, sessionID, "")
}

// WaitInbox waits for the first durable delivery when the current inbox is empty.
func (m *Manager) WaitInbox(
	ctx context.Context,
	sessionID string,
	channel string,
) ([]Envelope, error) {
	if ctx == nil {
		return nil, errors.New("network: wait inbox context is required")
	}
	waiter, err := m.registerInboxWaiter(sessionID)
	if err != nil {
		return nil, err
	}
	defer m.unregisterInboxWaiter(sessionID, waiter)
	if messages, err := m.listInbox(ctx, sessionID, channel); err != nil || len(messages) > 0 {
		return messages, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-m.lifecycleCtx.Done():
		return nil, m.lifecycleCtx.Err()
	case <-waiter:
		if err := m.lifecycleCtx.Err(); err != nil {
			return nil, err
		}
		messages, err := m.listInbox(ctx, sessionID, channel)
		if err != nil {
			if lifecycleErr := m.lifecycleCtx.Err(); lifecycleErr != nil {
				return nil, lifecycleErr
			}
		}
		return messages, err
	}
}

func (m *Manager) listInbox(
	ctx context.Context,
	sessionID string,
	channel string,
) ([]Envelope, error) {
	if ctx == nil {
		return nil, errors.New("network: inbox context is required")
	}
	if m == nil || m.inbox == nil {
		return nil, errors.New("network: durable inbox store is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	runtime, ok := m.sessionSnapshot(sessionID)
	if !ok || strings.TrimSpace(runtime.workspaceID) == "" {
		return nil, fmt.Errorf("network: inbox session %q is not managed", strings.TrimSpace(sessionID))
	}
	entries, err := m.inbox.ListNetworkInbox(
		ctx,
		runtime.workspaceID,
		strings.TrimSpace(sessionID),
		strings.TrimSpace(channel),
		store.NetworkMessageDefaultLimit,
	)
	if err != nil {
		return nil, err
	}
	messages := make([]Envelope, 0, len(entries))
	for _, entry := range entries {
		envelope, convertErr := envelopeFromStoredMessage(entry)
		if convertErr != nil {
			return nil, convertErr
		}
		messages = append(messages, envelope)
	}
	return messages, nil
}

func envelopeFromStoredMessage(entry store.NetworkMessageEntry) (Envelope, error) {
	var ext ExtensionMap
	if len(entry.ExtJSON) > 0 {
		if err := json.Unmarshal(entry.ExtJSON, &ext); err != nil {
			return Envelope{}, fmt.Errorf("network: decode stored message extensions: %w", err)
		}
	}
	envelope := Envelope{
		Protocol:    ProtocolV0,
		ID:          strings.TrimSpace(entry.MessageID),
		WorkspaceID: strings.TrimSpace(entry.WorkspaceID),
		Kind:        Kind(strings.TrimSpace(entry.Kind)),
		Channel:     strings.TrimSpace(entry.Channel),
		Surface:     optionalSurface(entry.Surface),
		ThreadID:    optionalString(entry.ThreadID),
		DirectID:    optionalString(entry.DirectID),
		From:        strings.TrimSpace(entry.PeerFrom),
		To:          optionalString(entry.PeerTo),
		Mentions:    append([]string(nil), entry.Mentions...),
		WorkID:      optionalString(entry.WorkID),
		ReplyTo:     optionalString(entry.ReplyTo),
		TraceID:     optionalString(entry.TraceID),
		CausationID: optionalString(entry.CausationID),
		TS:          entry.Timestamp.UTC().Unix(),
		Body:        append(json.RawMessage(nil), entry.Body...),
		Ext:         ext,
	}
	if entry.Timestamp.IsZero() {
		envelope.TS = time.Now().UTC().Unix()
	}
	return envelope, nil
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalSurface(value string) *Surface {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	surface := Surface(trimmed)
	return &surface
}

func (m *Manager) registerInboxWaiter(sessionID string) (chan struct{}, error) {
	if m == nil {
		return nil, errors.New("network: manager is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return nil, fmt.Errorf("%w: session id is required", ErrMissingField)
	}
	waiter := make(chan struct{})
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, context.Canceled
	}
	if m.waiters[target] == nil {
		m.waiters[target] = make(map[chan struct{}]struct{})
	}
	m.waiters[target][waiter] = struct{}{}
	return waiter, nil
}

func (m *Manager) unregisterInboxWaiter(sessionID string, waiter chan struct{}) {
	if m == nil || waiter == nil {
		return
	}
	target := strings.TrimSpace(sessionID)
	m.mu.Lock()
	defer m.mu.Unlock()
	waiters := m.waiters[target]
	delete(waiters, waiter)
	if len(waiters) == 0 {
		delete(m.waiters, target)
	}
}

func (m *Manager) notifyInboxRecipients(dispositions []store.NetworkMessageDisposition) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, disposition := range dispositions {
		if disposition.Decision != store.NetworkDispositionDeliver {
			continue
		}
		target := strings.TrimSpace(disposition.RecipientSessionID)
		for waiter := range m.waiters[target] {
			close(waiter)
		}
		delete(m.waiters, target)
	}
}
