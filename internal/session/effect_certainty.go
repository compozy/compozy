package session

// EffectCertainty classifies whether an external effect is proven absent.
type EffectCertainty string

const (
	// EffectKnownFalse proves no target effect crossed its durable linearization.
	EffectKnownFalse EffectCertainty = "known-false"
	// EffectUnknown means the caller must not retry with a different identity.
	EffectUnknown EffectCertainty = "unknown"
)
