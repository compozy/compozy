package extensions

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/compozy/compozy/internal/version"
)

// ManifestFieldError reports a validation error for one manifest field.
type ManifestFieldError struct {
	Field   string
	Value   string
	Message string
}

func (e *ManifestFieldError) Error() string {
	if e == nil {
		return "invalid extension manifest field"
	}

	if strings.TrimSpace(e.Value) == "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("%s=%q: %s", e.Field, e.Value, e.Message)
}

// ValidateManifest validates a parsed manifest against Compozy's manifest rules.
func ValidateManifest(ctx context.Context, manifest *Manifest) error {
	if err := contextError(ctx, "validate extension manifest"); err != nil {
		return err
	}
	if manifest == nil {
		return fmt.Errorf("validate extension manifest: manifest is nil")
	}

	if err := validateExtensionInfo(manifest.Extension); err != nil {
		return err
	}
	if err := validateSubprocess(manifest); err != nil {
		return err
	}
	if err := validateCapabilities(manifest.Security); err != nil {
		return err
	}
	if err := validateHooks(manifest); err != nil {
		return err
	}
	if err := validateResources(manifest); err != nil {
		return err
	}
	if err := validateProviders(manifest); err != nil {
		return err
	}
	if err := validateMinCompozyVersion(manifest.Extension.MinCompozyVersion); err != nil {
		return err
	}

	return nil
}

func newManifestFieldError(field, value, message string) error {
	return &ManifestFieldError{
		Field:   field,
		Value:   strings.TrimSpace(value),
		Message: message,
	}
}

func validateExtensionInfo(info ExtensionInfo) error {
	if strings.TrimSpace(info.Name) == "" {
		return newManifestFieldError("extension.name", "", "value is required")
	}
	if strings.TrimSpace(info.Version) == "" {
		return newManifestFieldError("extension.version", "", "value is required")
	}
	if _, err := parseSemanticVersion(info.Version); err != nil {
		return newManifestFieldError("extension.version", info.Version, "must be a valid semantic version")
	}
	if strings.TrimSpace(info.Description) == "" {
		return newManifestFieldError("extension.description", "", "value is required")
	}
	if strings.TrimSpace(info.MinCompozyVersion) == "" {
		return newManifestFieldError("extension.min_compozy_version", "", "value is required")
	}
	if _, err := parseSemanticVersion(info.MinCompozyVersion); err != nil {
		return newManifestFieldError(
			"extension.min_compozy_version",
			info.MinCompozyVersion,
			"must be a valid semantic version",
		)
	}
	return nil
}

func validateSubprocess(manifest *Manifest) error {
	if manifest.Subprocess == nil {
		if len(manifest.Hooks) > 0 {
			return newManifestFieldError("subprocess", "", "section is required when hooks are declared")
		}
		return nil
	}

	if strings.TrimSpace(manifest.Subprocess.Command) == "" {
		return newManifestFieldError("subprocess.command", "", "value is required")
	}
	return nil
}

func validateCapabilities(security SecurityConfig) error {
	for _, capability := range security.Capabilities {
		value := Capability(strings.TrimSpace(string(capability)))
		if value == "" {
			return newManifestFieldError("security.capabilities", "", "capability name is required")
		}
		if !supportedCapabilities.contains(value) {
			return newManifestFieldError("security.capabilities", string(value), "unknown capability")
		}
	}
	return nil
}

func validateHooks(manifest *Manifest) error {
	for index, hook := range manifest.Hooks {
		field := fmt.Sprintf("hooks[%d].event", index)
		event := HookName(strings.TrimSpace(string(hook.Event)))
		if event == "" {
			return newManifestFieldError(field, "", "value is required")
		}
		if !supportedHookNames.contains(event) {
			return newManifestFieldError(field, string(event), "unknown hook event")
		}
		if hook.Priority < MinHookPriority || hook.Priority > MaxHookPriority {
			return newManifestFieldError(
				fmt.Sprintf("hooks[%d].priority", index),
				fmt.Sprintf("%d", hook.Priority),
				fmt.Sprintf("must be within [%d, %d]", MinHookPriority, MaxHookPriority),
			)
		}

		requiredCapability := capabilityForHook(event)
		if requiredCapability != "" && !hasCapability(manifest.Security, requiredCapability) {
			return newManifestFieldError(
				fmt.Sprintf("hooks[%d].event", index),
				string(event),
				fmt.Sprintf("requires capability %q", requiredCapability),
			)
		}
	}
	return nil
}

func validateResources(manifest *Manifest) error {
	for index, pattern := range manifest.Resources.Skills {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			return newManifestFieldError(fmt.Sprintf("resources.skills[%d]", index), "", "value is required")
		}
		if !hasCapability(manifest.Security, CapabilitySkillsShip) {
			return newManifestFieldError(
				fmt.Sprintf("resources.skills[%d]", index),
				trimmed,
				fmt.Sprintf("requires capability %q", CapabilitySkillsShip),
			)
		}
		if strings.HasPrefix(trimmed, "/") {
			return newManifestFieldError(
				fmt.Sprintf("resources.skills[%d]", index),
				trimmed,
				"must be relative to the extension root",
			)
		}
	}
	return nil
}

func validateProviders(manifest *Manifest) error {
	providerGroups := []struct {
		name    string
		entries []ProviderEntry
	}{
		{name: "providers.ide", entries: manifest.Providers.IDE},
		{name: "providers.review", entries: manifest.Providers.Review},
		{name: "providers.model", entries: manifest.Providers.Model},
	}

	for _, group := range providerGroups {
		for index, entry := range group.entries {
			if !hasCapability(manifest.Security, CapabilityProvidersRegister) {
				return newManifestFieldError(
					fmt.Sprintf("%s[%d]", group.name, index),
					entry.Name,
					fmt.Sprintf("requires capability %q", CapabilityProvidersRegister),
				)
			}
			if strings.TrimSpace(entry.Name) == "" {
				return newManifestFieldError(fmt.Sprintf("%s[%d].name", group.name, index), "", "value is required")
			}
			if strings.TrimSpace(entry.Command) == "" {
				return newManifestFieldError(
					fmt.Sprintf("%s[%d].command", group.name, index),
					"",
					"value is required",
				)
			}
		}
	}

	return nil
}

func validateMinCompozyVersion(raw string) error {
	required, err := parseSemanticVersion(raw)
	if err != nil {
		return newManifestFieldError(
			"extension.min_compozy_version",
			raw,
			"must be a valid semantic version",
		)
	}

	currentRaw := strings.TrimSpace(version.Version)
	if currentRaw == "" || currentRaw == "dev" {
		return nil
	}

	current, err := parseSemanticVersion(currentRaw)
	if err != nil {
		return fmt.Errorf("parse current compozy version %q: %w", version.Version, err)
	}
	if current.LessThan(required) {
		return newManifestFieldError(
			"extension.min_compozy_version",
			raw,
			fmt.Sprintf("requires Compozy %s or newer (current %s)", required, current),
		)
	}

	return nil
}

func parseSemanticVersion(raw string) (*semver.Version, error) {
	return semver.NewVersion(strings.TrimPrefix(strings.TrimSpace(raw), "v"))
}

func hasCapability(security SecurityConfig, target Capability) bool {
	for _, capability := range security.Capabilities {
		if Capability(strings.TrimSpace(string(capability))) == target {
			return true
		}
	}
	return false
}

func capabilityForHook(hook HookName) Capability {
	switch {
	case strings.HasPrefix(string(hook), "plan."):
		return CapabilityPlanMutate
	case strings.HasPrefix(string(hook), "prompt."):
		return CapabilityPromptMutate
	case strings.HasPrefix(string(hook), "agent."):
		return CapabilityAgentMutate
	case strings.HasPrefix(string(hook), "job."):
		return CapabilityJobMutate
	case strings.HasPrefix(string(hook), "run."):
		return CapabilityRunMutate
	case strings.HasPrefix(string(hook), "review."):
		return CapabilityReviewMutate
	case strings.HasPrefix(string(hook), "artifact."):
		return CapabilityArtifactsWrite
	default:
		return ""
	}
}
