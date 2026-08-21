package store

import "encoding/json"

// ContentValue returns an isolated copy of the event payload.
func (s EventSummary) ContentValue() json.RawMessage {
	if s.EventSummaryContentState == nil {
		return nil
	}
	return append(json.RawMessage(nil), s.Content...)
}

// SetContent replaces the event payload with an isolated copy.
func (s *EventSummary) SetContent(content json.RawMessage) {
	if len(content) == 0 {
		s.EventSummaryContentState = nil
		return
	}
	s.EventSummaryContentState = &EventSummaryContentState{
		Content: append(json.RawMessage(nil), content...),
	}
}
