package runtimeevents

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/contentconv"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type statusCoder interface {
	StatusCode() int
}

func NewRuntimeEvent(runID string, kind events.EventKind, payload any) (events.Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return events.Event{}, fmt.Errorf("marshal %s payload: %w", kind, err)
	}
	return NewRuntimeEventRaw(runID, kind, raw), nil
}

func NewRuntimeEventRaw(runID string, kind events.EventKind, payload json.RawMessage) events.Event {
	return events.Event{
		RunID:   runID,
		Kind:    kind,
		Payload: payload,
	}
}

func UsagePayload(index int, usage model.Usage) kinds.UsageUpdatedPayload {
	return kinds.UsageUpdatedPayload{
		Index: index,
		Usage: PublicUsage(usage),
	}
}

func PublicUsage(usage model.Usage) kinds.Usage {
	return contentconv.PublicUsage(usage)
}

func PublicSessionUpdate(update model.SessionUpdate) (kinds.SessionUpdate, error) {
	return contentconv.PublicSessionUpdate(update)
}

func ProviderStatusCode(err error) int {
	if err == nil {
		return 200
	}
	var coder statusCoder
	if errors.As(err, &coder) {
		return coder.StatusCode()
	}
	return 0
}

func IssueIDFromPath(path string) string {
	base := filepath.Base(strings.TrimSpace(path))
	if base == "." || base == string(filepath.Separator) {
		return strings.TrimSpace(path)
	}
	return base
}
