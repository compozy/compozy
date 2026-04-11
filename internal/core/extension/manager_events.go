package extensions

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func (m *Manager) startEventForwarding() error {
	if m == nil || m.eventBus == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.eventUnsub != nil {
		return nil
	}

	subID, ch, unsubscribe := m.eventBus.Subscribe()
	m.eventSubID = subID
	m.eventUnsub = unsubscribe

	m.backgroundGroup.Add(1)
	go func() {
		defer m.backgroundGroup.Done()
		m.forwardBusEvents(ch)
	}()

	return nil
}

func (m *Manager) stopEventForwarding() {
	if m == nil {
		return
	}

	m.mu.Lock()
	unsubscribe := m.eventUnsub
	m.eventUnsub = nil
	m.eventSubID = 0
	m.mu.Unlock()

	if unsubscribe != nil {
		unsubscribe()
	}
	if m.backgroundStop != nil {
		m.backgroundStop()
	}
}

func (m *Manager) forwardBusEvents(ch <-chan events.Event) {
	for {
		select {
		case <-m.backgroundContext().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			m.forwardEvent(event)
		}
	}
}

func (m *Manager) forwardEvent(event events.Event) {
	if shouldSkipEventForwarding(event) {
		return
	}

	for _, session := range m.sessionSnapshot() {
		if !session.shouldReceiveEvent(event) {
			continue
		}
		if delivered := session.enqueueEvent(event); delivered {
			continue
		}
		dropped := session.recordDroppedEvent()
		slog.Warn(
			"extension event delivery dropped",
			"component", "extension.manager",
			"extension", session.runtime.normalizedName(),
			"event_kind", event.Kind,
			"dropped_total", dropped,
		)
		if err := m.emitLifecycleEvent(
			context.Background(),
			events.EventKindExtensionEvent,
			kinds.ExtensionEventPayload{
				Extension: session.runtime.normalizedName(),
				Kind:      "delivery_dropped",
				Payload:   marshalDropPayload(event.Kind, dropped),
			},
		); err != nil {
			slog.Warn(
				"emit extension delivery drop event",
				"component", "extension.manager",
				"extension", session.runtime.normalizedName(),
				"err", err,
			)
		}
	}
}

func (m *Manager) runEventWorker(baseCtx context.Context, session *extensionSession) {
	defer m.backgroundGroup.Done()

	if baseCtx == nil {
		baseCtx = m.backgroundContext()
	}

	timeoutFailures := 0
	for {
		select {
		case <-m.backgroundContext().Done():
			return
		case <-session.done:
			return
		case event, ok := <-session.onEventQueue:
			if !ok {
				return
			}

			callCtx, cancel := context.WithTimeout(baseCtx, session.defaultHookTimeout)
			err := session.Call(callCtx, "on_event", onEventRequest{Event: event}, &struct{}{})
			cancel()

			if err == nil {
				timeoutFailures = 0
				continue
			}

			if isTimeoutError(err) {
				timeoutFailures++
				if timeoutFailures >= maxEventTimeouts {
					session.degraded.Store(true)
					slog.Warn(
						"extension event delivery degraded after repeated timeouts",
						"component", "extension.manager",
						"extension", session.runtime.normalizedName(),
						"timeouts", timeoutFailures,
					)
					return
				}
			} else {
				timeoutFailures = 0
			}

			slog.Warn(
				"extension on_event delivery failed",
				"component", "extension.manager",
				"extension", session.runtime.normalizedName(),
				"event_kind", event.Kind,
				"err", err,
			)
		}
	}
}

type onEventRequest struct {
	Event events.Event `json:"event"`
}

func (s *extensionSession) shouldReceiveEvent(event events.Event) bool {
	if s == nil || s.runtime == nil {
		return false
	}
	select {
	case <-s.done:
		return false
	default:
	}
	if !s.supports.OnEvent {
		return false
	}
	if !s.runtime.Capabilities.Has(CapabilityEventsRead) {
		return false
	}
	if s.degraded.Load() {
		return false
	}
	if s.runtime.State() != ExtensionStateReady {
		return false
	}
	return s.runtime.WantsEvent(event.Kind)
}

func (s *extensionSession) enqueueEvent(event events.Event) bool {
	if s == nil {
		return false
	}

	select {
	case s.onEventQueue <- event:
		return true
	default:
		return false
	}
}

func (s *extensionSession) recordDroppedEvent() uint64 {
	if s == nil {
		return 0
	}
	return s.dropped.Add(1)
}

func shouldSkipEventForwarding(event events.Event) bool {
	if event.Kind != events.EventKindExtensionEvent {
		return false
	}

	var payload kinds.ExtensionEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return false
	}
	return payload.Kind == "delivery_dropped"
}

func marshalDropPayload(kind events.EventKind, dropped uint64) json.RawMessage {
	payload := map[string]any{
		"event_kind":    kind,
		"dropped_total": dropped,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}
