package config

import "time"

const (
	defaultAppUpdateCheckInterval = 6 * time.Hour
	minimumAppUpdateCheckInterval = 15 * time.Minute
	maximumAppUpdateCheckInterval = 168 * time.Hour
	appUpdateCheckIntervalPath    = "app.update_check_interval"
)

// AppConfig controls desktop-app update checks.
type AppConfig struct {
	UpdateCheck         bool          `toml:"update_check"`
	UpdateCheckInterval time.Duration `toml:"update_check_interval"`
}

func defaultAppConfig() AppConfig {
	return AppConfig{
		UpdateCheck:         true,
		UpdateCheckInterval: defaultAppUpdateCheckInterval,
	}
}

// Validate ensures desktop update polling stays within the supported bounds.
func (c AppConfig) Validate() error {
	if c.UpdateCheckInterval < minimumAppUpdateCheckInterval ||
		c.UpdateCheckInterval > maximumAppUpdateCheckInterval {
		return ValidationError{
			Path:    appUpdateCheckIntervalPath,
			Message: "must be between 15m and 168h",
		}
	}
	return nil
}
