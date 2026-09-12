package acp

import "time"

const EventTypePromptDelivery = "prompt_delivery"
const TextEstimateMethod = "bytes_div_4"

type DeliveredSpan struct {
	Key          string `json:"key"`
	Kind         string `json:"kind"`
	Bytes        int64  `json:"bytes"`
	Tokens       *int64 `json:"tokens,omitempty"`
	Unchanged    bool   `json:"unchanged"`
	StartupDedup bool   `json:"startup_dedup,omitzero"`
	Delivery     string `json:"delivery,omitempty"`
	HookModified bool   `json:"hook_modified,omitzero"`
	Name         string `json:"name,omitempty"`
}

type DeliveryManifest struct {
	TurnID   string          `json:"turn_id"`
	SentAt   time.Time       `json:"sent_at"`
	Estimate string          `json:"estimate"`
	Spans    []DeliveredSpan `json:"spans"`
}

// StartupManifest describes disjoint sections of the bound startup prompt.
type StartupManifest struct {
	Spans        []DeliveredSpan
	HookModified bool
}

// EstimateTokens estimates only known text bytes, without consulting a tokenizer.
func EstimateTokens(byteLen int64) int64 {
	if byteLen <= 0 {
		return 0
	}
	return byteLen/4 + min(byteLen%4, 1)
}

// TextSpan measures the exact text selected for delivery.
func TextSpan(key, text string) DeliveredSpan {
	bytes := int64(len(text))
	return DeliveredSpan{Key: key, Kind: "text", Bytes: bytes, Tokens: new(EstimateTokens(bytes))}
}

// OpaqueStartupManifest retains the final prompt when section boundaries are unknown.
func OpaqueStartupManifest(prompt string, hookModified bool) StartupManifest {
	manifest := StartupManifest{HookModified: hookModified}
	if prompt != "" {
		span := TextSpan("system_prompt", prompt)
		span.HookModified = hookModified
		manifest.Spans = []DeliveredSpan{span}
	}
	return manifest
}

// CloneStartupManifest returns an independent startup measurement snapshot.
func CloneStartupManifest(manifest StartupManifest) StartupManifest {
	manifest.Spans = append([]DeliveredSpan(nil), manifest.Spans...)
	for i := range manifest.Spans {
		if tokens := manifest.Spans[i].Tokens; tokens != nil {
			manifest.Spans[i].Tokens = new(*tokens)
		}
	}
	return manifest
}

func cloneDeliveryManifest(manifest *DeliveryManifest) *DeliveryManifest {
	if manifest == nil {
		return nil
	}
	cloned := *manifest
	cloned.Spans = CloneStartupManifest(StartupManifest{Spans: manifest.Spans}).Spans
	return &cloned
}
