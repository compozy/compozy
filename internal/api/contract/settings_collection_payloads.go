package contract

import (
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

type SettingsProviderSettingsPayload struct {
	Command         string                                  `json:"command,omitempty"`
	DisplayName     string                                  `json:"display_name,omitempty"`
	Models          *SettingsProviderModelsPayload          `json:"models,omitempty"`
	Harness         string                                  `json:"harness,omitempty"`
	RuntimeProvider string                                  `json:"runtime_provider,omitempty"`
	Transport       string                                  `json:"transport,omitempty"`
	BaseURL         string                                  `json:"base_url,omitempty"`
	AuthMode        string                                  `json:"auth_mode,omitempty"`
	EnvPolicy       string                                  `json:"env_policy,omitempty"`
	HomePolicy      string                                  `json:"home_policy,omitempty"`
	AuthStatusCmd   string                                  `json:"auth_status_command,omitempty"`
	AuthLoginCmd    string                                  `json:"auth_login_command,omitempty"`
	CredentialSlots []SettingsProviderCredentialSlotPayload `json:"credential_slots,omitempty"`
}

type SettingsProviderCredentialSlotPayload struct {
	Name      string `json:"name"`
	TargetEnv string `json:"target_env"`
	SecretRef string `json:"secret_ref"`
	Kind      string `json:"kind,omitempty"`
	Required  bool   `json:"required"`
}

type SettingsProviderCredentialStatusPayload struct {
	Name      string `json:"name"`
	TargetEnv string `json:"target_env"`
	SecretRef string `json:"secret_ref"`
	Kind      string `json:"kind,omitempty"`
	Required  bool   `json:"required"`
	Present   bool   `json:"present"`
	Source    string `json:"source,omitempty"`
}

type SettingsProviderNativeCLIStatusPayload struct {
	Command string `json:"command,omitempty"`
	Present bool   `json:"present"`
	Path    string `json:"path,omitempty"`
	Source  string `json:"source,omitempty"`
	Error   string `json:"error,omitempty"`
}

type SettingsProviderAuthStatusPayload struct {
	Mode       string                                  `json:"mode"`
	EnvPolicy  string                                  `json:"env_policy"`
	HomePolicy string                                  `json:"home_policy"`
	State      string                                  `json:"state"`
	Code       string                                  `json:"code,omitempty"`
	Message    string                                  `json:"message,omitempty"`
	StatusCmd  string                                  `json:"status_command,omitempty"`
	LoginCmd   string                                  `json:"login_command,omitempty"`
	LoginEnv   []string                                `json:"login_env,omitempty"`
	NativeCLI  *SettingsProviderNativeCLIStatusPayload `json:"native_cli,omitempty"`
}

type SettingsProviderSecretWritePayload struct {
	Name      string `json:"name,omitempty"`
	SecretRef string `json:"secret_ref"`
	Kind      string `json:"kind,omitempty"`
	Value     string `json:"value"`
}

type SettingsProviderFallbackPayload struct {
	Source   SettingsSourceRefPayload        `json:"source"`
	Settings SettingsProviderSettingsPayload `json:"settings"`
}

type SettingsProviderItemPayload struct {
	Name             string                                    `json:"name"`
	Settings         SettingsProviderSettingsPayload           `json:"settings"`
	Default          bool                                      `json:"default"`
	CommandAvailable bool                                      `json:"command_available"`
	Credentials      []SettingsProviderCredentialStatusPayload `json:"credentials,omitempty"`
	AuthStatus       *SettingsProviderAuthStatusPayload        `json:"auth_status,omitempty"`
	SourceMetadata   SettingsSourceMetadataPayload             `json:"source_metadata"`
	Fallback         *SettingsProviderFallbackPayload          `json:"fallback,omitempty"`
}

type SettingsMCPServerPayload struct {
	Name      string                        `json:"name"`
	Transport string                        `json:"transport,omitempty"`
	Command   string                        `json:"command,omitempty"`
	Args      []string                      `json:"args,omitempty"`
	Env       map[string]string             `json:"env,omitempty"`
	SecretEnv map[string]string             `json:"secret_env,omitempty"`
	URL       string                        `json:"url,omitempty"`
	Auth      *SettingsMCPAuthConfigPayload `json:"auth,omitempty"`
}

type SettingsMCPAuthConfigPayload struct {
	Type             string   `json:"type,omitempty"`
	IssuerURL        string   `json:"issuer_url,omitempty"`
	MetadataURL      string   `json:"metadata_url,omitempty"`
	AuthorizationURL string   `json:"authorization_url,omitempty"`
	TokenURL         string   `json:"token_url,omitempty"`
	RevocationURL    string   `json:"revocation_url,omitempty"`
	ClientID         string   `json:"client_id,omitempty"`
	ClientSecretRef  string   `json:"client_secret_ref,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
}

type SettingsMCPSecretValuesPayload struct {
	SecretEnv         map[string]string `json:"secret_env,omitempty"`
	OAuthClientSecret *string           `json:"oauth_client_secret,omitempty"`
}

// SettingsMCPSecretPreservationPayload identifies existing bindings to retain without exposing their refs.
type SettingsMCPSecretPreservationPayload struct {
	SecretEnv         []string `json:"secret_env,omitempty"`
	OAuthClientSecret bool     `json:"oauth_client_secret,omitempty"`
}

// SettingsMCPAuthConfigViewPayload is the public, binding-free OAuth configuration projection.
type SettingsMCPAuthConfigViewPayload struct {
	Type                   string   `json:"type,omitempty"`
	IssuerURL              string   `json:"issuer_url,omitempty"`
	MetadataURL            string   `json:"metadata_url,omitempty"`
	AuthorizationURL       string   `json:"authorization_url,omitempty"`
	TokenURL               string   `json:"token_url,omitempty"`
	RevocationURL          string   `json:"revocation_url,omitempty"`
	ClientID               string   `json:"client_id,omitempty"`
	ClientSecretConfigured bool     `json:"client_secret_configured"`
	Scopes                 []string `json:"scopes,omitempty"`
}

type SettingsMCPAuthStatusPayload struct {
	ServerName       string     `json:"server_name"`
	Scope            string     `json:"scope"`
	WorkspaceID      string     `json:"workspace_id,omitempty"`
	Status           string     `json:"status"`
	RemoteURL        string     `json:"remote_url,omitempty"`
	AuthType         string     `json:"auth_type,omitempty"`
	ClientID         string     `json:"client_id,omitempty"`
	Issuer           string     `json:"issuer,omitempty"`
	Scopes           []string   `json:"scopes,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	Refreshable      bool       `json:"refreshable"`
	TokenPresent     bool       `json:"token_present"`
	RevocationURL    string     `json:"revocation_url,omitempty"`
	Diagnostic       string     `json:"diagnostic,omitempty"`
	AuthorizationURL string     `json:"authorization_url,omitempty"`
}

type SettingsMCPServerRuntimeStatusPayload struct {
	Configured  bool   `json:"configured"`
	Initialized bool   `json:"initialized"`
	State       string `json:"state"`
	Probe       string `json:"probe"`
	ToolCount   int    `json:"tool_count"`
	Reason      string `json:"reason,omitempty"`
	Diagnostic  string `json:"diagnostic,omitempty"`
}

type SettingsMCPServerItemPayload struct {
	Name           string                                 `json:"name"`
	Transport      string                                 `json:"transport"`
	Command        string                                 `json:"command,omitempty"`
	Args           []string                               `json:"args,omitempty"`
	EnvKeys        []string                               `json:"env_keys,omitempty"`
	SecretEnvKeys  []string                               `json:"secret_env_keys,omitempty"`
	URL            string                                 `json:"url,omitempty"`
	Auth           *SettingsMCPAuthConfigViewPayload      `json:"auth,omitempty"`
	AuthStatus     *SettingsMCPAuthStatusPayload          `json:"auth_status,omitempty"`
	RuntimeStatus  *SettingsMCPServerRuntimeStatusPayload `json:"runtime_status,omitempty"`
	Scope          SettingsScopeKind                      `json:"scope"`
	WorkspaceID    string                                 `json:"workspace_id,omitempty"`
	CatalogEntry   string                                 `json:"catalog_entry,omitempty"`
	CatalogVersion string                                 `json:"catalog_version,omitempty"`
	SourceMetadata SettingsSourceMetadataPayload          `json:"source_metadata"`
}

type SettingsSandboxProfilePayload struct {
	Backend     string                         `json:"backend"`
	SyncMode    string                         `json:"sync_mode,omitempty"`
	Persistence string                         `json:"persistence,omitempty"`
	RuntimeRoot string                         `json:"runtime_root,omitempty"`
	Env         map[string]string              `json:"env,omitempty"`
	SecretEnv   map[string]string              `json:"secret_env,omitempty"`
	Network     *SettingsSandboxNetworkPayload `json:"network,omitempty"`
	Daytona     *SettingsSandboxDaytonaPayload `json:"daytona,omitempty"`
}

type SettingsSandboxNetworkPayload struct {
	AllowPublicIngress bool     `json:"allow_public_ingress,omitempty"`
	AllowOutbound      bool     `json:"allow_outbound,omitempty"`
	AllowList          []string `json:"allow_list,omitempty"`
	DenyList           []string `json:"deny_list,omitempty"`
	Required           bool     `json:"required,omitempty"`
}

type SettingsSandboxDaytonaPayload struct {
	APIURL      string `json:"api_url,omitempty"`
	Target      string `json:"target,omitempty"`
	Image       string `json:"image,omitempty"`
	Snapshot    string `json:"snapshot,omitempty"`
	Class       string `json:"class,omitempty"`
	AutoStop    string `json:"auto_stop,omitempty"`
	AutoArchive string `json:"auto_archive,omitempty"`
}

type SettingsSandboxItemPayload struct {
	Name                string                        `json:"name"`
	Profile             SettingsSandboxProfilePayload `json:"profile"`
	WorkspaceUsageCount int                           `json:"workspace_usage_count"`
	SourceMetadata      SettingsSourceMetadataPayload `json:"source_metadata"`
}

type SettingsHookDeclarationPayload struct {
	Name         string                    `json:"name"`
	Event        hookspkg.HookEvent        `json:"event"`
	Mode         hookspkg.HookMode         `json:"mode,omitempty"`
	Required     bool                      `json:"required,omitempty"`
	Enabled      *bool                     `json:"enabled,omitempty"`
	Priority     int                       `json:"priority,omitempty"`
	Timeout      string                    `json:"timeout,omitempty"`
	Matcher      hookspkg.HookMatcher      `json:"matcher"`
	ExecutorKind hookspkg.HookExecutorKind `json:"executor_kind,omitempty"`
	Command      string                    `json:"command,omitempty"`
	Args         []string                  `json:"args,omitempty"`
	Env          map[string]string         `json:"env,omitempty"`
	SecretEnv    map[string]string         `json:"secret_env,omitempty"`
	Metadata     map[string]string         `json:"metadata,omitempty"`
}

type SettingsHookItemPayload struct {
	Name           string                         `json:"name"`
	Declaration    SettingsHookDeclarationPayload `json:"declaration"`
	SourceMetadata SettingsSourceMetadataPayload  `json:"source_metadata"`
}
