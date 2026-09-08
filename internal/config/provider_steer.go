package config

import "fmt"

type SteerCapability string

const (
	SteerCapabilityExtension        SteerCapability = "steer_ext"
	SteerCapabilityConcurrentPrompt SteerCapability = "concurrent_prompt"
	SteerCapabilityNone             SteerCapability = "none"
)

func (c SteerCapability) Validate(path string) error {
	switch c {
	case "", SteerCapabilityExtension, SteerCapabilityConcurrentPrompt, SteerCapabilityNone:
		return nil
	default:
		return fmt.Errorf("%s must be one of steer_ext, concurrent_prompt, or none", path)
	}
}

func SteerCapabilityValues() []string {
	return []string{
		string(SteerCapabilityExtension),
		string(SteerCapabilityConcurrentPrompt),
		string(SteerCapabilityNone),
	}
}
