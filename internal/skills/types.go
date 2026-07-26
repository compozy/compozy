// Package skills provides the core types and loading primitives for AgentSkills
// `SKILL.md` files.
package skills

import (
	"io/fs"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

// SkillMeta maps YAML frontmatter fields per the AgentSkills spec.
type SkillMeta struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Version     string         `yaml:"version,omitempty"`
	Metadata    map[string]any `yaml:"metadata,omitempty"`
}

// Skill is the metadata-first in-memory representation of a parsed skill file.
type Skill struct {
	Meta                   SkillMeta
	Source                 SkillSource
	Dir                    string
	FilePath               string
	Enabled                bool
	ActivationGates        ActivationGates
	Activation             SkillActivation
	MCPServers             []MCPServerDecl
	Hooks                  []hookspkg.HookDecl
	Provenance             *Provenance
	InstalledFrom          string
	InstalledFromBundle    string
	InstalledFromExtension string
	Diagnostics            SkillDiagnostics
}

// ActivationGates declares offer-time constraints from metadata.agh.when.
type ActivationGates struct {
	Platforms            []string `json:"platforms,omitempty"             yaml:"platforms,omitempty"`
	Environments         []string `json:"environments,omitempty"          yaml:"environments,omitempty"`
	RequiresTools        []string `json:"requires_tools,omitempty"        yaml:"requires_tools,omitempty"`
	RequiresCapabilities []string `json:"requires_capabilities,omitempty" yaml:"requires_capabilities,omitempty"`
}

// ActivationGate identifies one supported metadata.agh.when family.
type ActivationGate string

const (
	ActivationGatePlatforms            ActivationGate = "platforms"
	ActivationGateEnvironments         ActivationGate = "environments"
	ActivationGateRequiresTools        ActivationGate = "requires_tools"
	ActivationGateRequiresCapabilities ActivationGate = "requires_capabilities"
)

// ActivationReasonCode identifies why an enabled skill is not currently offered.
type ActivationReasonCode string

const (
	ActivationReasonPlatformMismatch              ActivationReasonCode = "platform_mismatch"
	ActivationReasonEnvironmentContextUnavailable ActivationReasonCode = "environment_context_unavailable"
	ActivationReasonEnvironmentMismatch           ActivationReasonCode = "environment_mismatch"
	ActivationReasonToolContextUnavailable        ActivationReasonCode = "tool_context_unavailable"
	ActivationReasonMissingTool                   ActivationReasonCode = "missing_tool"
	ActivationReasonCapabilityContextUnavailable  ActivationReasonCode = "capability_context_unavailable"
	ActivationReasonMissingCapability             ActivationReasonCode = "missing_capability"
)

// ActivationReason is one deterministic unmet activation gate.
type ActivationReason struct {
	Gate    ActivationGate
	Code    ActivationReasonCode
	Missing []string
	Message string
}

// SkillActivation is the latest offer-time evaluation for one resolved skill.
type SkillActivation struct {
	Active    bool
	Evaluated bool
	Reasons   []ActivationReason
}

// SkillSource identifies where a skill was loaded from.
type SkillSource int

const (
	// SourceBundled is the lowest-precedence source backed by go:embed files.
	SourceBundled SkillSource = iota
	// SourceMarketplace identifies skills installed from a marketplace registry.
	SourceMarketplace
	// SourceUser identifies skills loaded from the user-level skill directories.
	SourceUser
	// SourceAdditional identifies skills loaded from additional workspace roots.
	SourceAdditional
	// SourceWorkspace is the highest-precedence source from `<workspace>/.agh/skills/`.
	SourceWorkspace
	// SourceAgentLocal is the final overlay from `<root>/.agh/agents/<name>/skills/`.
	SourceAgentLocal
)

// MCPServerDecl declares an MCP server dependency in skill frontmatter.
type MCPServerDecl struct {
	Name      string            `yaml:"name"`
	Command   string            `yaml:"command"`
	Args      []string          `yaml:"args,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
	SecretEnv map[string]string `yaml:"secret_env,omitempty"`
}

// Provenance stores marketplace install metadata for a skill.
type Provenance struct {
	Hash        string    `json:"hash"`
	Registry    string    `json:"registry"`
	Slug        string    `json:"slug"`
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
}

// WarningSeverity describes the impact of a loader or verifier warning.
type WarningSeverity int

const (
	SeverityInfo WarningSeverity = iota
	SeverityWarning
	SeverityCritical
)

// Warning captures a verification or loading concern associated with a skill.
type Warning struct {
	Severity WarningSeverity
	Message  string
	Pattern  string
}

// SkillDiagnosticState describes how one discovered skill definition resolved.
type SkillDiagnosticState string

const (
	// SkillDiagnosticStateValid reports a loaded definition that participates in the effective skill set.
	SkillDiagnosticStateValid SkillDiagnosticState = "valid"
	// SkillDiagnosticStateShadowed reports a definition superseded by a higher-precedence definition.
	SkillDiagnosticStateShadowed SkillDiagnosticState = "shadowed"
	// SkillDiagnosticStateVerificationFailed reports a definition rejected by provenance or content verification.
	SkillDiagnosticStateVerificationFailed SkillDiagnosticState = "verification_failed"
	// SkillDiagnosticStateInactive reports a valid enabled definition withheld by activation gates.
	SkillDiagnosticStateInactive SkillDiagnosticState = "inactive"
)

// SkillVerificationStatus describes the verifier outcome for one skill definition.
type SkillVerificationStatus string

const (
	// SkillVerificationStatusPassed means no verifier warning or error is attached.
	SkillVerificationStatusPassed SkillVerificationStatus = "passed"
	// SkillVerificationStatusWarning means non-blocking verifier warnings were found.
	SkillVerificationStatusWarning SkillVerificationStatus = "warning"
	// SkillVerificationStatusFailed means the definition was rejected by verification.
	SkillVerificationStatusFailed SkillVerificationStatus = "failed"
)

// SkillDefinitionRef identifies a skill definition involved in resolution diagnostics.
type SkillDefinitionRef struct {
	Source     string
	Path       string
	DetectedAt time.Time
}

// ShadowEntry is the public read model for one declaration participating in
// skill resolution. The winner and every shadowed declaration come from the
// same resolver snapshot.
type ShadowEntry struct {
	Path             string
	Tier             string
	ResolvedToWinner bool
	DetectedAt       time.Time
}

// SkillShadows describes every declaration for one skill name with the
// resolver winner called out explicitly.
type SkillShadows struct {
	Name    string
	Winner  ShadowEntry
	Shadows []ShadowEntry
}

// SkillVerificationFailure captures an actionable verification rejection.
type SkillVerificationFailure struct {
	Code         string
	Message      string
	ExpectedHash string
	ActualHash   string
}

// SkillDiagnostics stores verifier and resolution diagnostics on an effective skill.
type SkillDiagnostics struct {
	VerificationStatus  SkillVerificationStatus
	Warnings            []Warning
	ShadowedDefinitions []SkillDefinitionRef
}

// SkillDiagnostic is the public read model for one effective, shadowed, or rejected definition.
type SkillDiagnostic struct {
	Name               string
	State              SkillDiagnosticState
	Source             string
	Path               string
	WinningSource      string
	WinningPath        string
	VerificationStatus SkillVerificationStatus
	Warnings           []Warning
	Failure            *SkillVerificationFailure
	ActivationReasons  []ActivationReason
}

// RegistryConfig controls how the registry discovers global skills.
type RegistryConfig struct {
	BundledFS      fs.FS
	UserSkillsDir  string
	UserAgentsDir  string
	DisabledSkills []string
}
