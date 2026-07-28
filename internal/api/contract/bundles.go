package contract

import "time"

type BundleProfileCatalogPayload struct {
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	PrimaryChannel string                 `json:"primary_channel,omitempty"`
	Channels       []BundleChannelPayload `json:"channels,omitempty"`
	LayoutCount    int                    `json:"layout_count,omitempty"`
	AgentCount     int                    `json:"agent_count,omitempty"`
	JobCount       int                    `json:"job_count,omitempty"`
	TriggerCount   int                    `json:"trigger_count,omitempty"`
	BridgeCount    int                    `json:"bridge_count,omitempty"`
}

type BundleCatalogPayload struct {
	ExtensionName string                        `json:"extension_name"`
	BundleName    string                        `json:"bundle_name"`
	Description   string                        `json:"description,omitempty"`
	Profiles      []BundleProfileCatalogPayload `json:"profiles,omitempty"`
}

type BundleChannelPayload struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
}

type BundleAgentPayload struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Provider     string   `json:"provider,omitempty"`
	Model        string   `json:"model,omitempty"`
	CategoryPath []string `json:"category_path,omitempty"`
	HasSoul      bool     `json:"has_soul,omitempty"`
	HasHeartbeat bool     `json:"has_heartbeat,omitempty"`
}

type BundleLayoutPayload struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"display_name"`
	AspectVariant    string   `json:"aspect_variant"`
	ParticipantSlots []string `json:"participant_slots,omitempty"`
	OverflowPolicy   string   `json:"overflow_policy"`
}

type BundleJobPayload struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AgentName string `json:"agent_name"`
	Enabled   bool   `json:"enabled"`
}

type BundleTriggerPayload struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AgentName string `json:"agent_name"`
	Event     string `json:"event"`
	Enabled   bool   `json:"enabled"`
}

type BundleBridgeSecretSlotPayload struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
}

type BundleBridgePayload struct {
	ID            string                          `json:"id"`
	Name          string                          `json:"name"`
	ExtensionName string                          `json:"extension_name"`
	Platform      string                          `json:"platform"`
	DisplayName   string                          `json:"display_name"`
	SecretSlots   []BundleBridgeSecretSlotPayload `json:"secret_slots,omitempty"`
}

type BundleInventoryPayload struct {
	ResourceKind string `json:"resource_kind"`
	ResourceID   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
}

type BundleActivationPayload struct {
	ID                            string                   `json:"id"`
	Version                       int64                    `json:"version"`
	ExtensionName                 string                   `json:"extension_name"`
	BundleName                    string                   `json:"bundle_name"`
	BundleDescription             string                   `json:"bundle_description,omitempty"`
	ProfileName                   string                   `json:"profile_name"`
	ProfileDescription            string                   `json:"profile_description,omitempty"`
	Scope                         string                   `json:"scope"`
	WorkspaceID                   string                   `json:"workspace_id,omitempty"`
	NetworkRequirementDigest      string                   `json:"network_requirement_digest,omitempty"`
	NetworkRequirementConfirmedBy string                   `json:"network_requirement_confirmed_by,omitempty"`
	NetworkRequirementConfirmedAt string                   `json:"network_requirement_confirmed_at,omitempty"`
	Channels                      []BundleChannelPayload   `json:"channels,omitempty"`
	Layouts                       []BundleLayoutPayload    `json:"layouts,omitempty"`
	Agents                        []BundleAgentPayload     `json:"agents,omitempty"`
	Jobs                          []BundleJobPayload       `json:"jobs,omitempty"`
	Triggers                      []BundleTriggerPayload   `json:"triggers,omitempty"`
	Bridges                       []BundleBridgePayload    `json:"bridges,omitempty"`
	Inventory                     []BundleInventoryPayload `json:"inventory,omitempty"`
	SpecDrift                     bool                     `json:"spec_drift"`
	CreatedAt                     time.Time                `json:"created_at"`
	UpdatedAt                     time.Time                `json:"updated_at"`
}

type DeclaredNetworkChannelPayload struct {
	ActivationID  string `json:"activation_id,omitempty"`
	ExtensionName string `json:"extension_name,omitempty"`
	BundleName    string `json:"bundle_name,omitempty"`
	ProfileName   string `json:"profile_name,omitempty"`
	WorkspaceID   string `json:"workspace_id,omitempty"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Primary       bool   `json:"primary,omitempty"`
}

type BundleNetworkSettingsPayload struct {
	DeclaredChannels []DeclaredNetworkChannelPayload `json:"declared_channels,omitempty"`
}

type ActivateBundleRequest struct {
	ExtensionName             string `json:"extension_name"`
	BundleName                string `json:"bundle_name"`
	ProfileName               string `json:"profile_name"`
	Scope                     string `json:"scope,omitempty"`
	Workspace                 string `json:"workspace,omitempty"`
	ConfirmNetworkRequirement bool   `json:"confirm_network_requirement,omitempty"`
}

type UpdateBundleActivationRequest struct {
	ExpectedVersion           int64 `json:"expected_version"`
	ConfirmNetworkRequirement bool  `json:"confirm_network_requirement,omitempty"`
}

type BundlesCatalogResponse struct {
	Bundles []BundleCatalogPayload `json:"bundles"`
}

type BundleActivationResponse struct {
	Activation BundleActivationPayload `json:"activation"`
}

type BundleActivationsResponse struct {
	Activations []BundleActivationPayload `json:"activations"`
}

type BundlePreviewResponse struct {
	Activation BundleActivationPayload `json:"activation"`
}

type BundleNetworkSettingsResponse struct {
	Network BundleNetworkSettingsPayload `json:"network"`
}
