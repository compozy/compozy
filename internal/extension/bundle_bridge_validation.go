package extensionpkg

import (
	"encoding/json"

	"fmt"

	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
)

// Validate ensures one bundle trigger is internally consistent.
func (t BundleTrigger) Validate(bundleName string, profileName string) error {
	trigger := automationpkg.Trigger{
		ID:           "bundle-validation",
		Scope:        automationpkg.AutomationScopeGlobal,
		Name:         strings.TrimSpace(t.Name),
		AgentName:    strings.TrimSpace(t.AgentName),
		Prompt:       strings.TrimSpace(t.Prompt),
		Event:        strings.TrimSpace(t.Event),
		Filter:       cloneStringMap(t.Filter),
		Enabled:      t.Enabled,
		Retry:        t.Retry,
		FireLimit:    t.FireLimit,
		Source:       automationpkg.JobSourcePackage,
		EndpointSlug: strings.TrimSpace(t.EndpointSlug),
	}
	if err := trigger.Validate("bundle.triggers"); err != nil {
		return fmt.Errorf(
			"%w: bundle %q profile %q trigger %q: %w",
			ErrBundleInvalid,
			bundleName,
			profileName,
			t.Name,
			err,
		)
	}
	return nil
}

// Validate ensures one bundle bridge preset is internally consistent.
func (b BundleBridgePreset) Validate(bundleName string, profileName string, manifest *Manifest) error {
	displayName := strings.TrimSpace(b.DisplayName)
	if displayName == "" {
		return fmt.Errorf(
			"%w: bundle %q profile %q bridge %q display_name is required",
			ErrBundleInvalid,
			bundleName,
			profileName,
			b.Name,
		)
	}
	if err := b.RoutingPolicy.Validate(); err != nil {
		return fmt.Errorf(
			"%w: bundle %q profile %q bridge %q routing_policy: %w",
			ErrBundleInvalid,
			bundleName,
			profileName,
			b.Name,
			err,
		)
	}
	trimmedDeliveryDefaults := strings.TrimSpace(string(b.DeliveryDefaults))
	if trimmedDeliveryDefaults != "" && !json.Valid([]byte(trimmedDeliveryDefaults)) {
		return fmt.Errorf(
			"%w: bundle %q profile %q bridge %q delivery_defaults: invalid JSON",
			ErrBundleInvalid,
			bundleName,
			profileName,
			b.Name,
		)
	}
	for _, slot := range b.SecretSlots {
		if strings.TrimSpace(slot.Name) == "" {
			return fmt.Errorf(
				"%w: bundle %q profile %q bridge %q secret_slots.name is required",
				ErrBundleInvalid,
				bundleName,
				profileName,
				b.Name,
			)
		}
		if strings.TrimSpace(slot.Kind) == "" {
			return fmt.Errorf(
				"%w: bundle %q profile %q bridge %q secret slot %q kind is required",
				ErrBundleInvalid,
				bundleName,
				profileName,
				b.Name,
				slot.Name,
			)
		}
	}

	if strings.TrimSpace(b.ExtensionName) == "" && manifest != nil && strings.TrimSpace(b.Platform) == "" {
		if !providesCapability(manifest.Capabilities.Provides, "bridge.adapter") {
			return fmt.Errorf(
				"%w: bundle %q profile %q bridge %q must declare extension_name or platform",
				ErrBundleInvalid,
				bundleName,
				profileName,
				b.Name,
			)
		}
	}
	return nil
}
