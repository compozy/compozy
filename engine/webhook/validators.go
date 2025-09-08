package webhook

import "fmt"

// ValidateTrigger validates a webhook trigger configuration.
func ValidateTrigger(cfg *Config) error {
	if err := validateBasics(cfg); err != nil {
		return err
	}
	if err := validateEvents(cfg); err != nil {
		return err
	}
	if err := validateVerify(cfg); err != nil {
		return err
	}
	return nil
}

func validateBasics(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("webhook config is required")
	}
	if cfg.Slug == "" {
		return fmt.Errorf("webhook slug is required")
	}
	if cfg.Method != "" {
		m := cfg.Method
		if m != "POST" && m != "PUT" && m != "PATCH" {
			return fmt.Errorf("invalid webhook method: %s", m)
		}
	}
	if len(cfg.Events) == 0 {
		return fmt.Errorf("webhook events are required and cannot be empty")
	}
	return nil
}

func validateEvents(cfg *Config) error {
	evtNames := map[string]struct{}{}
	for idx := range cfg.Events {
		e := &cfg.Events[idx]
		if e.Name == "" {
			return fmt.Errorf("event[%d] name is required", idx)
		}
		if _, dup := evtNames[e.Name]; dup {
			return fmt.Errorf("duplicate event name '%s' in webhook trigger", e.Name)
		}
		evtNames[e.Name] = struct{}{}
		if e.Filter == "" {
			return fmt.Errorf("event[%s] filter is required", e.Name)
		}
		if len(e.Input) == 0 {
			return fmt.Errorf("event[%s] input is required and cannot be empty", e.Name)
		}
		if e.Schema != nil {
			if _, err := e.Schema.Compile(); err != nil {
				return fmt.Errorf("invalid event schema for %s: %w", e.Name, err)
			}
		}
	}
	return nil
}

func validateVerify(cfg *Config) error {
	if cfg.Verify == nil {
		return nil
	}
	s := cfg.Verify.Strategy
	if s != StrategyNone && s != StrategyHMAC && s != StrategyStripe && s != StrategyGitHub {
		return fmt.Errorf("invalid verify.strategy: %s", s)
	}
	if s == StrategyHMAC {
		if cfg.Verify.Secret == "" || cfg.Verify.Header == "" {
			return fmt.Errorf("hmac verification requires secret and header")
		}
	}
	return nil
}
