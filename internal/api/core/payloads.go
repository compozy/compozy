package core

import (
	"encoding/json"
	"io"
	"maps"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	hookspkg "github.com/compozy/agh/internal/hooks"
)

// SSEMessage is the shared SSE envelope.
type SSEMessage struct {
	ID   string
	Name string
	Data any
}

// FlushWriter is an SSE writer that can flush streamed content.
type FlushWriter interface {
	io.Writer
	Flush()
}

// LogsCursor is the shared cursor used for logs streaming.
type LogsCursor struct {
	Timestamp time.Time
	Sequence  int64
	ID        string
}

// HookCatalogPayloadsFromEntries converts resolved hook catalog entries into transport DTOs.
func HookCatalogPayloadsFromEntries(entries []hookspkg.CatalogEntry) []contract.HookCatalogPayload {
	payloads := make([]contract.HookCatalogPayload, 0, len(entries))
	for _, entry := range entries {
		payload := contract.HookCatalogPayload{
			Order:        entry.Order,
			Name:         entry.Name,
			Event:        entry.Event.String(),
			Source:       entry.Source.String(),
			Mode:         string(entry.Mode),
			Required:     entry.Required,
			Priority:     int(entry.Priority),
			ExecutorKind: string(entry.ExecutorKind),
			Matcher:      entry.Matcher,
			Metadata:     cloneCatalogMetadata(entry.Metadata),
		}
		if entry.SkillSource != "" {
			payload.SkillSource = string(entry.SkillSource)
		}
		if entry.Timeout > 0 {
			payload.TimeoutMS = entry.Timeout.Milliseconds()
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

// HookRunPayloadsFromRecords converts persisted hook audit records into transport DTOs.
func HookRunPayloadsFromRecords(records []hookspkg.HookRunRecord) []contract.HookRunPayload {
	payloads := make([]contract.HookRunPayload, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, contract.HookRunPayload{
			HookName:      record.HookName,
			Event:         record.Event.String(),
			Source:        record.Source.String(),
			Mode:          string(record.Mode),
			DurationMS:    record.Duration.Milliseconds(),
			Outcome:       string(record.Outcome),
			DispatchDepth: record.DispatchDepth,
			PatchApplied:  cloneHookRunPatch(record.PatchApplied),
			Error:         record.Error,
			Required:      record.Required,
			RecordedAt:    record.RecordedAt,
		})
	}
	return payloads
}

// HookEventPayloadsFromDescriptors converts hook taxonomy descriptors into transport DTOs.
func HookEventPayloadsFromDescriptors(events []hookspkg.EventDescriptor) []contract.HookEventPayload {
	payloads := make([]contract.HookEventPayload, 0, len(events))
	for _, event := range events {
		payloads = append(payloads, contract.HookEventPayload{
			Event:         event.Event.String(),
			Family:        string(event.Family),
			SyncEligible:  event.SyncEligible,
			PayloadSchema: event.PayloadSchema,
			PatchSchema:   event.PatchSchema,
		})
	}
	return payloads
}

func cloneCatalogMetadata(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	maps.Copy(cloned, src)
	return cloned
}

func cloneHookRunPatch(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), src...)
}
