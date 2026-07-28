package config

import "fmt"

// AutomationSuggestionsConfig controls consent-first suggestion emission.
type AutomationSuggestionsConfig struct {
	PendingCap int `toml:"pending_cap"`
}

type automationSuggestionsOverlay struct {
	PendingCap *int `toml:"pending_cap"`
}

func (c AutomationSuggestionsConfig) validate(path string) error {
	if c.PendingCap <= 0 {
		return fmt.Errorf("%s.pending_cap must be positive: %d", path, c.PendingCap)
	}
	return nil
}

func (o *automationSuggestionsOverlay) apply(dst *AutomationSuggestionsConfig) {
	if o == nil || o.PendingCap == nil {
		return
	}
	dst.PendingCap = *o.PendingCap
}
