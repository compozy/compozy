package heartbeat

import (
	"cmp"
	"encoding/json/v2"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/redact"
)

type AutomationEventSource struct {
	SourceID    string `json:"source_id,omitempty"`
	SourceRef   string `json:"source_ref,omitempty"`
	ItemKey     string `json:"item_key,omitempty"`
	URL         string `json:"url,omitempty"`
	TriggerName string `json:"trigger_name"`
	Event       string `json:"event"`
	// Summary is the operator-facing readable line, set only on report sources, never in the check digest.
	Summary    string `json:"summary,omitempty"`
	Kind       string `json:"kind"`
	EventID    string `json:"event_id"`
	TriggerID  string `json:"trigger_id"`
	RunID      string `json:"run_id"`
	ReceivedAt string `json:"received_at"`
}

type AutomationEventFact struct {
	ID, InstanceID, Summary, ReceivedAt string
	Instruction                         string
	TriggerName, Event                  string
	TriggerID, RunID                    string
}

const (
	AutomationEventBatchLimit   = 20
	AutomationEventBatchBytes   = 8 << 10
	AutomationEventSummaryBytes = 2 << 10
)

func collectAutomationEvents(scope CollectorScope, facts CollectorFacts) ([]DigestItem, []string) {
	if scope.InstanceID == "" {
		return nil, nil
	}
	rows := slices.Clone(facts.AutomationEvents)
	slices.SortFunc(rows, func(a, b AutomationEventFact) int {
		return cmp.Or(strings.Compare(a.ReceivedAt, b.ReceivedAt), strings.Compare(a.ID, b.ID))
	})
	var items []DigestItem
	var ids []string
	used := 0
	if facts.EventsOmitted > 0 {
		item := DigestItem{
			Key:     "automation_events:omitted",
			Summary: fmt.Sprintf("%d older events omitted by retention", facts.EventsOmitted),
		}
		encoded, err := json.Marshal(item)
		if err == nil {
			items = append(items, item)
			used = len(encoded) + 1
		}
	}
	seen := map[string]bool{}
	for _, row := range rows {
		if row.InstanceID != scope.InstanceID || row.ID == "" || seen[row.ID] {
			continue
		}
		if len(ids) == AutomationEventBatchLimit {
			break
		}
		// The oldest pending event always fits by shrinking its content, so one
		// escaping-heavy event cannot starve the events queued behind it.
		budget := AutomationEventBatchBytes - used
		item, encoded, err := boundedAutomationEvent(row, budget, len(ids) == 0)
		if err != nil || len(encoded)+1 > budget {
			break
		}
		used += len(encoded) + 1
		seen[row.ID] = true
		items = append(items, item)
		ids = append(ids, row.ID)
	}
	return items, ids
}

func BoundAutomationEventSummary(summary string) string {
	summary = strings.ToValidUTF8(summary, "�")
	if len(summary) <= AutomationEventSummaryBytes {
		return summary
	}
	summary = summary[:AutomationEventSummaryBytes]
	for !utf8.ValidString(summary) {
		summary = summary[:len(summary)-1]
	}
	return summary
}

// boundedAutomationEvent serializes one event. When shrink is set and the item
// exceeds budget, it halves the longest presentation text until the complete
// serialized item fits: content (summary, instruction) shrinks first, then the
// source attribution text (trigger name, event). Durable event, trigger and run
// IDs are never truncated, and stored metadata is untouched.
func boundedAutomationEvent(row AutomationEventFact, budget int, shrink bool) (DigestItem, []byte, error) {
	summary := BoundAutomationEventSummary(redact.String(row.Summary))
	instruction := BoundAutomationEventSummary(redact.String(row.Instruction))
	content := []*string{&summary, &instruction}
	attribution := []*string{&row.TriggerName, &row.Event}
	for {
		item := automationEventItem(row, summary, instruction)
		encoded, err := json.Marshal(item)
		if err != nil || !shrink || len(encoded) < budget {
			return item, encoded, err
		}
		if !halveLongest(content) && !halveLongest(attribution) {
			return item, encoded, nil
		}
	}
}

// halveLongest truncates the longest nonempty text to half its length and
// reports whether any text remained to shrink.
func halveLongest(texts []*string) bool {
	var longest *string
	for _, text := range texts {
		if longest == nil || len(*text) > len(*longest) {
			longest = text
		}
	}
	if longest == nil || *longest == "" {
		return false
	}
	*longest = truncateAutomationText(*longest, len(*longest)/2)
	return true
}

func automationEventItem(row AutomationEventFact, summary, instruction string) DigestItem {
	quoted := strconv.Quote(summary)
	quoted = strings.NewReplacer("<", `\u003c`, ">", `\u003e`, "&", `\u0026`).Replace(quoted)
	return DigestItem{
		Source: &AutomationEventSource{
			Kind:        "event",
			TriggerName: row.TriggerName,
			Event:       row.Event,
			EventID:     row.ID,
			TriggerID:   row.TriggerID,
			RunID:       row.RunID,
			ReceivedAt:  row.ReceivedAt,
		},
		Key:         "automation_event:" + row.ID,
		Summary:     automationEventFenceOpen + quoted + automationEventFenceClose,
		Instruction: instruction,
	}
}

func truncateAutomationText(text string, limit int) string {
	text = text[:limit]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text
}
