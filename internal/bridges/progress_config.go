package bridges

import (
	"encoding/json"
	"time"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

// ProgressMode controls which tool lifecycle events become bridge presentation events.
type ProgressMode string

const (
	ProgressModeOff     ProgressMode = ProgressMode(bridgecontract.ProgressModeOff)
	ProgressModeNew     ProgressMode = ProgressMode(bridgecontract.ProgressModeNew)
	ProgressModeAll     ProgressMode = ProgressMode(bridgecontract.ProgressModeAll)
	ProgressModeVerbose ProgressMode = ProgressMode(bridgecontract.ProgressModeVerbose)
)

// Normalize returns the canonical progress-mode representation.
func (m ProgressMode) Normalize() ProgressMode {
	return ProgressMode(bridgecontract.ProgressMode(m).Normalize())
}

// Validate reports whether the mode belongs to the supported progress taxonomy.
func (m ProgressMode) Validate() error {
	return bridgecontract.ProgressMode(m).Validate()
}

// ProgressGrouping controls whether progress lines accumulate or post separately.
type ProgressGrouping string

const (
	ProgressGroupingAccumulate ProgressGrouping = ProgressGrouping(bridgecontract.ProgressGroupingAccumulate)
	ProgressGroupingSeparate   ProgressGrouping = ProgressGrouping(bridgecontract.ProgressGroupingSeparate)
)

// Normalize returns the canonical grouping representation.
func (g ProgressGrouping) Normalize() ProgressGrouping {
	return ProgressGrouping(bridgecontract.ProgressGrouping(g).Normalize())
}

// Validate reports whether the grouping belongs to the supported set.
func (g ProgressGrouping) Validate() error {
	return bridgecontract.ProgressGrouping(g).Validate()
}

// ProgressConfig is the typed effective bridge progress configuration.
type ProgressConfig struct {
	ToolProgress ProgressMode     `json:"tool_progress"`
	Grouping     ProgressGrouping `json:"grouping"`
	Typing       bool             `json:"typing"`
	Reactions    bool             `json:"reactions"`
	PreviewLimit int              `json:"-"`
	EditInterval time.Duration    `json:"-"`
}

// Validate reports whether all public progress settings are supported.
func (c ProgressConfig) Validate() error {
	return progressConfigToContract(c).Validate()
}

func (c ProgressConfig) effective() ProgressConfig {
	return progressConfigFromContract(bridgecontract.NormalizeProgressConfig(progressConfigToContract(c)))
}

// ResolveProgressConfig applies instance override, then platform, then global defaults.
func ResolveProgressConfig(instance *BridgeInstance, platform string) ProgressConfig {
	var contractInstance *bridgecontract.BridgeInstance
	if instance != nil {
		converted := BridgeInstanceToContract(*instance)
		contractInstance = &converted
	}
	return progressConfigFromContract(bridgecontract.ResolveProgressConfig(contractInstance, platform))
}

// NormalizeDeliveryDefaultsJSON validates and canonicalizes bridge delivery default JSON.
func NormalizeDeliveryDefaultsJSON(raw json.RawMessage) (json.RawMessage, error) {
	return bridgecontract.NormalizeDeliveryDefaultsJSON(raw)
}
