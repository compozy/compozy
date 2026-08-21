package config

// AttentionConfig controls operator-facing attention delivery.
type AttentionConfig struct {
	Toasts bool `toml:"toasts"`
	Sound  bool `toml:"sound"`
	System bool `toml:"system"`
}

// DefaultAttentionConfig returns the built-in operator attention policy.
func DefaultAttentionConfig() AttentionConfig {
	return AttentionConfig{Toasts: true, Sound: true}
}
