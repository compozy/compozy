package config

// RedactConfig controls additive secret redaction heuristics.
type RedactConfig struct {
	Enabled bool `toml:"enabled"`
}
