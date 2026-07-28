package contract

// SettingsMCPInstallNextStep identifies the deterministic follow-up to catalog installation.
type SettingsMCPInstallNextStep string

const (
	SettingsMCPInstallNextStepNone      SettingsMCPInstallNextStep = "none"
	SettingsMCPInstallNextStepAuthorize SettingsMCPInstallNextStep = "authorize"
)

// SettingsMCPSecretInputPayload contains exactly one write-only value or existing Vault ref.
type SettingsMCPSecretInputPayload struct {
	Value    string `json:"value,omitempty"`
	VaultRef string `json:"vault_ref,omitempty"`
}

// SettingsMCPCatalogInstallValuesPayload contains operator-supplied feed fields.
type SettingsMCPCatalogInstallValuesPayload struct {
	Env               map[string]SettingsMCPSecretInputPayload `json:"env,omitempty"`
	OAuthClientSecret *SettingsMCPSecretInputPayload           `json:"oauth_client_secret,omitempty"`
}

// InstallSettingsMCPServerRequest installs one feed-locked MCP entry.
type InstallSettingsMCPServerRequest struct {
	EntryID     string                                  `json:"entry_id"`
	Name        string                                  `json:"name,omitempty"`
	Scope       SettingsWorkspaceScopeKind              `json:"scope"`
	WorkspaceID string                                  `json:"workspace_id,omitempty"`
	Values      *SettingsMCPCatalogInstallValuesPayload `json:"values"`
}

// InstallSettingsMCPServerResponse returns the persisted item, config-apply outcome, and follow-up.
type InstallSettingsMCPServerResponse struct {
	MCPServer SettingsMCPServerItemPayload `json:"mcp_server"`
	Apply     SettingsApplyResponse        `json:"apply"`
	NextStep  SettingsMCPInstallNextStep   `json:"next_step"`
	Warnings  []DiagnosticItem             `json:"warnings,omitempty"`
}
