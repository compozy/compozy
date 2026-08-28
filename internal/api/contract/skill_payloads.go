package contract

import "time"

type SkillShadowEntryPayload struct {
	Path             string    `json:"path"`
	Tier             string    `json:"tier"`
	Origin           string    `json:"origin,omitempty"`
	ResolvedToWinner bool      `json:"resolved_to_winner"`
	DetectedAt       time.Time `json:"detected_at"`
}

// SkillPayload is the HTTP response type for a skill.
type SkillPayload struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Version     string                   `json:"version,omitempty"`
	Source      string                   `json:"source"`
	Origin      string                   `json:"origin"`
	OwnerScope  string                   `json:"owner_scope"`
	OwnerID     string                   `json:"owner_id,omitempty"`
	Enabled     bool                     `json:"enabled"`
	Activation  SkillActivationPayload   `json:"activation"`
	Dir         string                   `json:"dir"`
	Metadata    map[string]any           `json:"metadata,omitempty"`
	Provenance  *ProvenancePayload       `json:"provenance,omitempty"`
	Diagnostics []SkillDiagnosticPayload `json:"diagnostics,omitempty"`
	Exposures   *[]SkillExposurePayload  `json:"exposures,omitempty"`
}

// SkillExposurePayload is one provider-root link and its reconciled health.
type SkillExposurePayload struct {
	Target string              `json:"target"`
	Path   string              `json:"path"`
	Status SkillExposureStatus `json:"status"`
}

// SkillExposureStatus is the reconciled provider-link health vocabulary.
type SkillExposureStatus string

const (
	SkillExposureStatusHealthy         SkillExposureStatus = "healthy"
	SkillExposureStatusMissing         SkillExposureStatus = "missing"
	SkillExposureStatusBroken          SkillExposureStatus = "broken"
	SkillExposureStatusForeignConflict SkillExposureStatus = "foreign_conflict"
)

// SkillExposureRequest mutates exposure links for one skill.
type SkillExposureRequest struct {
	Targets     []string `json:"targets"                binding:"required,min=1,dive,required"`
	WorkspaceID string   `json:"workspace_id,omitempty"`
}

// SkillExposureErrorPayload is one deterministic public exposure failure.
type SkillExposureErrorPayload struct {
	Code       string `json:"code"`
	Message    string `json:"message,omitempty"`
	OccupiedBy string `json:"occupied_by,omitempty"`
}

// SkillExposureFailureErrorPayload is the required top-level failure summary.
type SkillExposureFailureErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// SkillExposureTargetResultPayload reports one target without hiding partial outcomes.
type SkillExposureTargetResultPayload struct {
	Target       string                     `json:"target"`
	OK           bool                       `json:"ok"`
	Exposure     *SkillExposurePayload      `json:"exposure,omitempty"`
	Error        *SkillExposureErrorPayload `json:"error,omitempty"`
	CleanupError *SkillExposureErrorPayload `json:"cleanup_error,omitempty"`
}

// SkillExposeResponse is the successful expose result.
type SkillExposeResponse struct {
	Name        string                             `json:"name"`
	WorkspaceID string                             `json:"workspace_id,omitempty"`
	Results     []SkillExposureTargetResultPayload `json:"results"`
	RolledBack  bool                               `json:"rolled_back"`
}

// SkillUnexposeResponse is the successful independent removal result.
type SkillUnexposeResponse struct {
	Name        string                             `json:"name"`
	WorkspaceID string                             `json:"workspace_id,omitempty"`
	Results     []SkillExposureTargetResultPayload `json:"results"`
}

// SkillExposureFailureResponse is the only expose/unexpose failure envelope.
type SkillExposureFailureResponse struct {
	Error       SkillExposureFailureErrorPayload   `json:"error"`
	Name        string                             `json:"name"`
	WorkspaceID string                             `json:"workspace_id,omitempty"`
	Results     []SkillExposureTargetResultPayload `json:"results"`
	RolledBack  *bool                              `json:"rolled_back,omitempty"`
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

// SkillMarketplaceCleanupDiagnosticPayload identifies cleanup degradation after a successful mutation.
type SkillMarketplaceCleanupDiagnosticPayload struct {
	Operation string `json:"operation"`
}

// SkillMarketplaceInstallPayload describes one completed marketplace install.
type SkillMarketplaceInstallPayload struct {
	Name               string                                     `json:"name"`
	Slug               string                                     `json:"slug"`
	Version            string                                     `json:"version,omitempty"`
	Registry           string                                     `json:"registry"`
	Path               string                                     `json:"path"`
	Hash               string                                     `json:"hash"`
	Status             string                                     `json:"status"`
	CleanupDiagnostics []SkillMarketplaceCleanupDiagnosticPayload `json:"cleanup_diagnostics,omitempty"`
}

// SkillMarketplaceUpdatePayload describes one marketplace update outcome.
type SkillMarketplaceUpdatePayload struct {
	Name               string                                     `json:"name"`
	Slug               string                                     `json:"slug"`
	CurrentVersion     string                                     `json:"current_version,omitempty"`
	LatestVersion      string                                     `json:"latest_version,omitempty"`
	Path               string                                     `json:"path"`
	Status             string                                     `json:"status"`
	CleanupDiagnostics []SkillMarketplaceCleanupDiagnosticPayload `json:"cleanup_diagnostics,omitempty"`
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
	Workspace    WorkspacePayload               `json:"workspace"`
	Sessions     []SessionPayload               `json:"sessions,omitempty"`
	Agents       []AgentPayload                 `json:"agents,omitempty"`
	Skills       []WorkspaceSkillPayload        `json:"skills,omitempty"`
	Providers    []SessionProviderOptionPayload `json:"providers,omitempty"`
	ProfileHints []WorkspaceProfileHintPayload  `json:"profile_hints,omitempty"`
}

// WorkspaceProfileHintPayload reports dormant repository content for an absent profile name.
type WorkspaceProfileHintPayload struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Action  string `json:"action"`
}
