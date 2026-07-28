package network

import (
	"encoding/json"
	"strings"
)

func previewForBody(body Body) string {
	preview := ""
	switch value := body.(type) {
	case GreetBody:
		preview = ResolveGreetSummary(value.PeerCard, value.Summary)
	case WhoisBody:
		if value.Type == WhoisTypeRequest {
			preview = strings.TrimSpace(value.Query)
		}
	case SayBody:
		preview = strings.TrimSpace(value.Text)
	case CapabilityBody:
		if summary := strings.TrimSpace(value.Capability.Summary); summary != "" {
			preview = summary
		} else if outcome := strings.TrimSpace(value.Capability.Outcome); outcome != "" {
			preview = outcome
		} else {
			preview = strings.TrimSpace(value.Capability.ID)
		}
	case ReceiptBody:
		if value.Detail != nil {
			preview = strings.TrimSpace(*value.Detail)
		}
	case TraceBody:
		preview = strings.TrimSpace(value.Message)
	}
	return truncateNetworkPreview(preview)
}

// PreviewTextForRawBody derives operator-facing preview text from persisted JSON.
func PreviewTextForRawBody(kind Kind, raw json.RawMessage) string {
	body, err := DecodeBody(kind, raw)
	if err != nil {
		return ""
	}
	return previewForBody(body)
}
