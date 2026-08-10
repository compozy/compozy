package config

import "time"

type appOverlay struct {
	UpdateCheck         *bool          `toml:"update_check"`
	UpdateCheckInterval *time.Duration `toml:"update_check_interval"`
}

func (o appOverlay) Apply(dst *AppConfig) {
	if o.UpdateCheck != nil {
		dst.UpdateCheck = *o.UpdateCheck
	}
	if o.UpdateCheckInterval != nil {
		dst.UpdateCheckInterval = *o.UpdateCheckInterval
	}
}
