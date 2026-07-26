package config

type redactOverlay struct {
	Enabled *bool `toml:"enabled"`
}

func (o redactOverlay) Apply(dst *RedactConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
}
