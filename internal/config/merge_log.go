package config

type logOverlay struct {
	Level           *string `toml:"level"`
	MaxSizeMB       *int    `toml:"max_size_mb"`
	MaxBackups      *int    `toml:"max_backups"`
	MaxAgeDays      *int    `toml:"max_age_days"`
	CompressBackups *bool   `toml:"compress_backups"`
}

func (o logOverlay) Apply(dst *LogConfig) {
	if o.Level != nil {
		dst.Level = *o.Level
	}
	if o.MaxSizeMB != nil {
		dst.MaxSizeMB = *o.MaxSizeMB
	}
	if o.MaxBackups != nil {
		dst.MaxBackups = *o.MaxBackups
	}
	if o.MaxAgeDays != nil {
		dst.MaxAgeDays = *o.MaxAgeDays
	}
	if o.CompressBackups != nil {
		dst.CompressBackups = *o.CompressBackups
	}
}
