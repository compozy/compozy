package config

// CloneMemoryConfig returns an ownership-safe copy of the memory configuration.
func CloneMemoryConfig(source *MemoryConfig) MemoryConfig {
	cloned := *source
	cloned.Controller.Policy.AllowOrigins = append([]string(nil), source.Controller.Policy.AllowOrigins...)
	return cloned
}
