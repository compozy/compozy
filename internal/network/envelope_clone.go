package network

import (
	"encoding/json"
	"strings"

	"github.com/compozy/agh/internal/store"
)

func normalizeEnvelopeCopy(env Envelope) Envelope {
	return Envelope{
		Protocol:    strings.TrimSpace(env.Protocol),
		ID:          strings.TrimSpace(env.ID),
		WorkspaceID: strings.TrimSpace(env.WorkspaceID),
		Kind:        Kind(strings.TrimSpace(string(env.Kind))),
		Channel:     strings.TrimSpace(env.Channel),
		Surface:     normalizeOptionalSurface(env.Surface),
		ThreadID:    normalizeOptionalIdentifier(env.ThreadID),
		DirectID:    normalizeOptionalIdentifier(env.DirectID),
		From:        strings.TrimSpace(env.From),
		Mentions:    normalizeEnvelopeMentions(env.Mentions),
		TS:          env.TS,
		Body:        cloneRawMessage(env.Body),
		Proof:       cloneProof(env.Proof),
		Ext:         cloneExtensionMap(env.Ext),
		WorkID:      normalizeOptionalIdentifier(env.WorkID),
		ReplyTo:     normalizeOptionalIdentifier(env.ReplyTo),
		TraceID:     normalizeOptionalIdentifier(env.TraceID),
		CausationID: normalizeOptionalIdentifier(env.CausationID),
		To:          normalizeOptionalIdentifier(env.To),
		ExpiresAt:   cloneInt64Ptr(env.ExpiresAt),
	}
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func cloneExtensionMap(ext ExtensionMap) ExtensionMap {
	if ext == nil {
		return nil
	}
	cloned := make(ExtensionMap, len(ext))
	for key, value := range ext {
		cloned[key] = cloneRawMessage(value)
	}
	return cloned
}

func cloneProof(proof *Proof) *Proof {
	if proof == nil {
		return nil
	}
	cloned := make(Proof, len(*proof))
	for key, value := range *proof {
		cloned[key] = cloneRawMessage(value)
	}
	return &cloned
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func normalizeOptionalSurface(value *Surface) *Surface {
	if value == nil {
		return nil
	}
	normalized := Surface(strings.TrimSpace(string(*value)))
	if normalized == "" {
		return nil
	}
	return &normalized
}

func normalizeOptionalIdentifier(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeEnvelopeMentions(values []string) []string {
	normalized, err := store.NormalizeNetworkPeerIDs(values, "mentions")
	if err != nil {
		return append([]string(nil), values...)
	}
	return normalized
}

func normalizeOptionalText(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	text := *value
	return &text
}

func normalizeStringList(values []string) []string {
	if values == nil {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}
