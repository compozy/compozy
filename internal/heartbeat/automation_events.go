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
	Kind       string `json:"kind"`
	EventID    string `json:"event_id"`
	TriggerID  string `json:"trigger_id"`
	RunID      string `json:"run_id"`
	ReceivedAt string `json:"received_at"`
}

type AutomationEventFact struct {
	ID, InstanceID, Summary, ReceivedAt string
	Instruction                         string
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
		summary := BoundAutomationEventSummary(redact.String(row.Summary))
		quoted := strconv.Quote(summary)
		quoted = strings.NewReplacer("<", `\u003c`, ">", `\u003e`, "&", `\u0026`).Replace(quoted)
		item := DigestItem{
			Source: &AutomationEventSource{
				Kind:       "automation_event",
				EventID:    row.ID,
				TriggerID:  row.TriggerID,
				RunID:      row.RunID,
				ReceivedAt: row.ReceivedAt,
			},
			Key:     "automation_event:" + row.ID,
			Summary: "<untrusted_automation_event>\n" + quoted + "\n</untrusted_automation_event>",
		}
		item.Instruction = BoundAutomationEventSummary(redact.String(row.Instruction))

		encoded, err := json.Marshal(item)
		if err != nil || used+len(encoded)+1 > AutomationEventBatchBytes {
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
