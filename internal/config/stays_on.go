package config

type StaysOnConfig struct {
	MaxActiveInstances int `toml:"max_active_instances"`
}

type staysOnOverlay struct {
	MaxActiveInstances *int `toml:"max_active_instances"`
}

func (o staysOnOverlay) Apply(dst *StaysOnConfig) {
	if o.MaxActiveInstances != nil {
		dst.MaxActiveInstances = *o.MaxActiveInstances
	}
}
