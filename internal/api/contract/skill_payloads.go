package contract

import "time"

// SessionProviderOptionPayload is one workspace-visible session provider option.
type SessionProviderOptionPayload struct {
	Name            string `json:"name"`
	DisplayName     string `json:"display_name,omitempty"`
	Harness         string `json:"harness,omitempty"`
	RuntimeProvider string `json:"runtime_provider,omitempty"`
	AuthMode        string `json:"auth_mode,omitempty"`
	EnvPolicy       string `json:"env_policy,omitempty"`
	HomePolicy      string `json:"home_policy,omitempty"`
}

type SkillShadowEntryPayload struct {
	Path             string    `json:"path"`
	Tier             string    `json:"tier"`
	ResolvedToWinner bool      `json:"resolved_to_winner"`
	DetectedAt       time.Time `json:"detected_at"`
}

// SkillPayload is the HTTP response type for a skill.
type SkillPayload struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Version     string                   `json:"version,omitempty"`
	Source      string                   `json:"source"`
	Enabled     bool                     `json:"enabled"`
	Activation  SkillActivationPayload   `json:"activation"`
	Dir         string                   `json:"dir"`
	Metadata    map[string]any           `json:"metadata,omitempty"`
	Provenance  *ProvenancePayload       `json:"provenance,omitempty"`
	Diagnostics []SkillDiagnosticPayload `json:"diagnostics,omitempty"`
}

// SkillMarketplaceInstallRequest installs one remote marketplace skill.
type SkillMarketplaceInstallRequest struct {
	Slug    string `json:"slug"`
	Version string `json:"version,omitempty"`
}

// SkillMarketplaceUpdateRequest checks or applies updates for marketplace skills.
type SkillMarketplaceUpdateRequest struct {
	Name      string `json:"name,omitempty"`
	All       bool   `json:"all,omitempty"`
	CheckOnly bool   `json:"check_only,omitempty"`
}

// SkillMarketplaceInstallPayload describes one completed marketplace install.
type SkillMarketplaceInstallPayload struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Version  string `json:"version,omitempty"`
	Registry string `json:"registry"`
	Path     string `json:"path"`
	Hash     string `json:"hash"`
	Status   string `json:"status"`
}

// SkillMarketplaceUpdatePayload describes one marketplace update outcome.
type SkillMarketplaceUpdatePayload struct {
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	CurrentVersion string `json:"current_version,omitempty"`
	LatestVersion  string `json:"latest_version,omitempty"`
	Path           string `json:"path"`
	Status         string `json:"status"`
}

// SkillMarketplaceRemovePayload describes one removed marketplace skill.
type SkillMarketplaceRemovePayload struct {
	Name   string `json:"name"`
	Slug   string `json:"slug"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

// SkillContentResponse is the explicit response type for one skill body.
type SkillContentResponse struct {
	Content string `json:"content"`
}

// ProvenancePayload is the nested provenance metadata for marketplace skills.
type ProvenancePayload struct {
	Slug                   string                    `json:"slug,omitempty"`
	Registry               string                    `json:"registry,omitempty"`
	Version                string                    `json:"version,omitempty"`
	InstalledAt            *time.Time                `json:"installed_at,omitempty"`
	InstalledFromBundle    string                    `json:"installed_from_bundle,omitempty"`
	InstalledFromExtension string                    `json:"installed_from_extension,omitempty"`
	PrecedenceTier         string                    `json:"precedence_tier"`
	ShadowedBy             []SkillShadowEntryPayload `json:"shadowed_by,omitempty"`
}

// SkillActionResponse is the shared skill enable/disable response payload.
type SkillActionResponse struct {
	OK bool `json:"ok"`
}

// WorkspaceDetailPayload is the shared resolved workspace detail response payload.
type WorkspaceDetailPayload struct {
	Workspace WorkspacePayload               `json:"workspace"`
	Sessions  []SessionPayload               `json:"sessions,omitempty"`
	Agents    []AgentPayload                 `json:"agents,omitempty"`
	Skills    []WorkspaceSkillPayload        `json:"skills,omitempty"`
	Providers []SessionProviderOptionPayload `json:"providers,omitempty"`
}
