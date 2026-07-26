package config

import (
	"errors"
	"fmt"
	"os"

	burnttoml "github.com/BurntSushi/toml"
)

// ApplyConfigOverlayFile deep-merges an optional TOML config file into dst.
func ApplyConfigOverlayFile(path string, dst *Config) error {
	if dst == nil {
		return errors.New("config: destination config is required")
	}

	overlay, err := loadConfigOverlayFile(path)
	if err != nil {
		return err
	}

	if err := applyConfigOverlay(dst, &overlay, RoleFieldSourceGlobal); err != nil {
		return err
	}
	return nil
}

func applyConfigOverlay(dst *Config, overlay *configOverlay, roleSource string) error {
	if err := overlay.Apply(dst); err != nil {
		return err
	}
	overlay.Roles.recordSources(dst, roleSource)
	return nil
}

func loadConfigOverlayFile(path string) (configOverlay, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return configOverlay{}, nil
		}
		return configOverlay{}, FileError{Op: mergeReadKey, Path: path, Err: err}
	}

	return loadConfigOverlayBytes(contents, path)
}

func loadConfigOverlayBytes(contents []byte, source string) (configOverlay, error) {
	var overlay configOverlay

	meta, err := burnttoml.Decode(string(contents), &overlay)
	if err != nil {
		return overlay, FileError{Op: "decode", Path: source, Err: err}
	}

	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		if err := rejectRemovedProviderModelKeys(source, undecoded); err != nil {
			return overlay, err
		}
		return overlay, fmt.Errorf("unknown config keys in %q: %s", source, joinTOMLKeys(undecoded))
	}

	return overlay, nil
}

func rejectRemovedProviderModelKeys(source string, keys []burnttoml.Key) error {
	for _, key := range sortedTOMLKeys(keys) {
		if len(key) != 3 || key[0] != providersConfigKey {
			continue
		}
		if key[2] == "aliases" {
			return fmt.Errorf(
				"removed config key %q in %q: aliases was removed; reference providers by canonical name",
				key.String(),
				source,
			)
		}
		replacement := ""
		switch key[2] {
		case "default_model":
			replacement = fmt.Sprintf("providers.%s.models.default", key[1])
		case "supported_models":
			replacement = fmt.Sprintf("providers.%s.models.curated", key[1])
		case "supports_reasoning_effort":
			replacement = fmt.Sprintf("providers.%s.models.curated[].reasoning_efforts", key[1])
		}
		if replacement != "" {
			return fmt.Errorf(
				"removed config key %q in %q: use %q",
				key.String(),
				source,
				replacement,
			)
		}
	}
	return nil
}

func (o httpOverlay) Apply(dst *HTTPConfig) {
	if o.Host != nil {
		dst.Host = *o.Host
	}
	if o.Port != nil {
		dst.Port = *o.Port
	}
}

func (o defaultsOverlay) Apply(dst *DefaultsConfig) {
	if o.Agent != nil {
		dst.Agent = *o.Agent
	}
	if o.Provider != nil {
		dst.Provider = *o.Provider
	}
	if o.Sandbox != nil {
		dst.Sandbox = *o.Sandbox
	}
}

func (o agentsOverlay) Apply(dst *AgentsConfig) {
	o.Soul.Apply(&dst.Soul)
	o.Heartbeat.Apply(&dst.Heartbeat)
}

func (o soulOverlay) Apply(dst *SoulConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.MaxBodyBytes != nil {
		dst.MaxBodyBytes = *o.MaxBodyBytes
	}
	if o.ContextProjectionBytes != nil {
		dst.ContextProjectionBytes = *o.ContextProjectionBytes
	}
}

func (o heartbeatOverlay) Apply(dst *HeartbeatConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.MaxBodyBytes != nil {
		dst.MaxBodyBytes = *o.MaxBodyBytes
	}
	if o.ContextProjectionBytes != nil {
		dst.ContextProjectionBytes = *o.ContextProjectionBytes
	}
	if o.MinInterval != nil {
		dst.MinInterval = *o.MinInterval
	}
	if o.DefaultInterval != nil {
		dst.DefaultInterval = *o.DefaultInterval
	}
	if o.WakeCooldown != nil {
		dst.WakeCooldown = *o.WakeCooldown
	}
	if o.MaxWakesPerCycle != nil {
		dst.MaxWakesPerCycle = *o.MaxWakesPerCycle
	}
	if o.ActiveSessionOnly != nil {
		dst.ActiveSessionOnly = *o.ActiveSessionOnly
	}
	if o.AllowActiveHoursPreferences != nil {
		dst.AllowActiveHoursPreferences = *o.AllowActiveHoursPreferences
	}
	if o.WakeEventRetention != nil {
		dst.WakeEventRetention = *o.WakeEventRetention
	}
	if o.SessionHealthStaleAfter != nil {
		dst.SessionHealthStaleAfter = *o.SessionHealthStaleAfter
	}
	if o.SessionHealthHookMinInterval != nil {
		dst.SessionHealthHookMinInterval = *o.SessionHealthHookMinInterval
	}
}

func (o limitsOverlay) Apply(dst *LimitsConfig) {
	if o.MaxConcurrentAgents != nil {
		dst.MaxConcurrentAgents = *o.MaxConcurrentAgents
	}
}

func (o permissionsOverlay) Apply(dst *PermissionsConfig) {
	if o.Mode != nil {
		dst.Mode = *o.Mode
	}
}
