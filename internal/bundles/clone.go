package bundles

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/windowmanager"
)

func cloneActivation(value Activation) Activation {
	return value
}

func cloneBundleSpec(value extensionpkg.BundleSpec) extensionpkg.BundleSpec {
	cloned := value
	cloned.Profiles = make([]extensionpkg.BundleProfile, 0, len(value.Profiles))
	for _, profile := range value.Profiles {
		cloned.Profiles = append(cloned.Profiles, cloneBundleProfile(profile))
	}
	return cloned
}

func cloneBundleProfile(value extensionpkg.BundleProfile) extensionpkg.BundleProfile {
	cloned := value
	cloned.Channels = extensionpkg.BundleChannelsConfig{
		Primary: strings.TrimSpace(value.Channels.Primary),
		Items:   append([]extensionpkg.BundleChannel(nil), value.Channels.Items...),
	}
	cloned.Agents = cloneBundleAgents(value.Agents)
	cloned.Layouts = cloneBundleLayouts(value.Layouts)
	cloned.Jobs = cloneBundleJobs(value.Jobs)
	cloned.Triggers = cloneBundleTriggers(value.Triggers)
	cloned.Bridges = cloneBundleBridgePresets(value.Bridges)
	return cloned
}

func cloneBundleLayouts(values []extensionpkg.BundleLayout) []extensionpkg.BundleLayout {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]extensionpkg.BundleLayout, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, extensionpkg.BundleLayout{
			Path:   strings.TrimSpace(value.Path),
			Layout: windowmanager.CloneLayoutResource(value.Layout),
		})
	}
	return cloned
}

func cloneBundleAgents(values []extensionpkg.BundleAgent) []extensionpkg.BundleAgent {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]extensionpkg.BundleAgent, 0, len(values))
	for _, value := range values {
		next := extensionpkg.BundleAgent{
			Path:  strings.TrimSpace(value.Path),
			Agent: aghconfig.CloneAgentDef(value.Agent),
		}
		if value.Soul != nil {
			next.Soul = &extensionpkg.BundleAgentSidecar{
				SourcePath: strings.TrimSpace(value.Soul.SourcePath),
				Body:       value.Soul.Body,
			}
		}
		if value.Heartbeat != nil {
			next.Heartbeat = &extensionpkg.BundleAgentSidecar{
				SourcePath: strings.TrimSpace(value.Heartbeat.SourcePath),
				Body:       value.Heartbeat.Body,
			}
		}
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneInventoryItems(items []InventoryItem) []InventoryItem {
	return append([]InventoryItem(nil), items...)
}

func cloneBundleJobs(values []extensionpkg.BundleJob) []extensionpkg.BundleJob {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]extensionpkg.BundleJob, 0, len(values))
	for _, value := range values {
		next := value
		next.Task = cloneTaskConfig(value.Task)
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneBundleTriggers(values []extensionpkg.BundleTrigger) []extensionpkg.BundleTrigger {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]extensionpkg.BundleTrigger, 0, len(values))
	for _, value := range values {
		next := value
		next.Filter = cloneStringMap(value.Filter)
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneBundleBridgePresets(values []extensionpkg.BundleBridgePreset) []extensionpkg.BundleBridgePreset {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]extensionpkg.BundleBridgePreset, 0, len(values))
	for _, value := range values {
		next := value
		next.DeliveryDefaults = cloneRawMessage(value.DeliveryDefaults)
		next.SecretSlots = append([]extensionpkg.BundleBridgeSecretSlot(nil), value.SecretSlots...)
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneNetworkSettings(value NetworkSettings) NetworkSettings {
	value.DeclaredChannels = append([]DeclaredChannel(nil), value.DeclaredChannels...)
	return value
}

func cloneTaskConfig(value *automationpkg.JobTaskConfig) *automationpkg.JobTaskConfig {
	if value == nil {
		return nil
	}
	cloned := *value
	if value.Owner != nil {
		owner := *value.Owner
		cloned.Owner = &owner
	}
	return &cloned
}

func cloneSchedule(value automationpkg.ScheduleSpec) *automationpkg.ScheduleSpec {
	cloned := value
	return &cloned
}

func cloneRawMessage(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}

func cloneStringMap(value map[string]string) map[string]string {
	if len(value) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(value))
	maps.Copy(cloned, value)
	return cloned
}

func bundleProfileSpecContentHash(bundle extensionpkg.BundleSpec, profile extensionpkg.BundleProfile) (string, error) {
	payload := struct {
		BundleName        string                     `json:"bundle_name"`
		BundleDescription string                     `json:"bundle_description,omitempty"`
		Profile           extensionpkg.BundleProfile `json:"profile"`
	}{
		BundleName:        strings.TrimSpace(bundle.Name),
		BundleDescription: strings.TrimSpace(bundle.Description),
		Profile:           cloneBundleProfile(profile),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf(
			"bundles: compute spec content hash for %s/%s: %w",
			strings.TrimSpace(bundle.Name),
			strings.TrimSpace(profile.Name),
			err,
		)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
