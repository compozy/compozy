package task

import (
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

const (
	// CoordinatorModeInherit uses daemon/workspace coordinator defaults.
	CoordinatorModeInherit CoordinatorMode = "inherit"
	// CoordinatorModeGuided injects task-specific guidance into the existing coordinator.
	CoordinatorModeGuided CoordinatorMode = "guided"

	// WorkerModeInherit uses normal task/run and workspace worker defaults.
	WorkerModeInherit WorkerMode = "inherit"
	// WorkerModeSelect narrows worker selection using the task profile.
	WorkerModeSelect WorkerMode = "select"

	// SandboxModeInherit uses workspace/global sandbox defaults.
	SandboxModeInherit SandboxMode = "inherit"
	// SandboxModeNone disables task-level sandbox selection when config permits it.
	SandboxModeNone SandboxMode = "none"
	// SandboxModeRef selects one named sandbox reference at session start.
	SandboxModeRef SandboxMode = "ref"

	// RuntimeModeDefault uses the normal task-session runtime contract.
	RuntimeModeDefault RuntimeMode = "default"
	// RuntimeModeEvidence explicitly allows runtime/browser/mobile evidence work.
	RuntimeModeEvidence RuntimeMode = "evidence"
)

const (
	defaultCoordinatorGuidanceMaxBytes = 8192
	profileSelectorMaxBytes            = 128
)

// CoordinatorMode identifies task-specific coordinator bootstrap behavior.
type CoordinatorMode string

// WorkerMode identifies how a task narrows worker selection.
type WorkerMode string

// SandboxMode identifies task-level sandbox selection behavior.
type SandboxMode string

// RuntimeMode identifies task-level runtime evidence behavior.
type RuntimeMode string

// ExecutionProfile is the typed task-owned orchestration selection state.
type ExecutionProfile struct {
	TaskID               string                 `json:"task_id"`
	Coordinator          CoordinatorProfile     `json:"coordinator"`
	Worker               WorkerProfile          `json:"worker"`
	Review               ReviewProfile          `json:"review"`
	Participants         ParticipantPolicy      `json:"participants"`
	Sandbox              SandboxPolicy          `json:"sandbox"`
	Runtime              RuntimePolicy          `json:"runtime"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

// CoordinatorProfile supplies optional guidance to the existing coordinator runtime.
type CoordinatorProfile struct {
	Mode      CoordinatorMode `json:"mode"`
	AgentName string          `json:"agent_name,omitempty"`
	Provider  string          `json:"provider,omitempty"`
	Model     string          `json:"model,omitempty"`
	Guidance  string          `json:"guidance,omitempty"`
}

// WorkerProfile narrows eligible task workers without granting runtime authority.
type WorkerProfile struct {
	Mode                  WorkerMode `json:"mode"`
	AgentName             string     `json:"agent_name,omitempty"`
	Provider              string     `json:"provider,omitempty"`
	Model                 string     `json:"model,omitempty"`
	AllowedAgentNames     []string   `json:"allowed_agent_names,omitempty"`
	PreferredAgentNames   []string   `json:"preferred_agent_names,omitempty"`
	RequiredCapabilities  []string   `json:"required_capabilities,omitempty"`
	PreferredCapabilities []string   `json:"preferred_capabilities,omitempty"`
}

// ReviewProfile narrows reviewer execution shape; verdict authority stays in task review APIs.
type ReviewProfile struct {
	AgentName             string   `json:"agent_name,omitempty"`
	Provider              string   `json:"provider,omitempty"`
	Model                 string   `json:"model,omitempty"`
	AllowedAgentNames     []string `json:"allowed_agent_names,omitempty"`
	PreferredAgentNames   []string `json:"preferred_agent_names,omitempty"`
	AllowedChannelIDs     []string `json:"allowed_channel_ids,omitempty"`
	PreferredChannelIDs   []string `json:"preferred_channel_ids,omitempty"`
	AllowedPeerIDs        []string `json:"allowed_peer_ids,omitempty"`
	PreferredPeerIDs      []string `json:"preferred_peer_ids,omitempty"`
	RequiredCapabilities  []string `json:"required_capabilities,omitempty"`
	PreferredCapabilities []string `json:"preferred_capabilities,omitempty"`
}

// ParticipantPolicy is an upper-bound routing policy, not a permission grant.
type ParticipantPolicy struct {
	AllowedChannelIDs     []string `json:"allowed_channel_ids,omitempty"`
	PreferredChannelIDs   []string `json:"preferred_channel_ids,omitempty"`
	AllowedPeerIDs        []string `json:"allowed_peer_ids,omitempty"`
	PreferredPeerIDs      []string `json:"preferred_peer_ids,omitempty"`
	AllowedAgentNames     []string `json:"allowed_agent_names,omitempty"`
	PreferredAgentNames   []string `json:"preferred_agent_names,omitempty"`
	RequiredCapabilities  []string `json:"required_capabilities,omitempty"`
	PreferredCapabilities []string `json:"preferred_capabilities,omitempty"`
}

// SandboxPolicy selects task-level sandbox behavior at session start.
type SandboxPolicy struct {
	Mode       SandboxMode `json:"mode"`
	SandboxRef string      `json:"sandbox_ref,omitempty"`
}

// RuntimePolicy opts a task into broader runtime-evidence operations.
type RuntimePolicy struct {
	Mode RuntimeMode `json:"mode"`
}

// ExecutionProfileValidationOptions carries config-backed gates without coupling task to config.
type ExecutionProfileValidationOptions struct {
	AllowProviderOverride       bool
	AllowSandboxNone            bool
	AllowSandboxRef             bool
	MaxCoordinatorGuidanceBytes int
}

// DefaultExecutionProfileValidationOptions returns the permissive built-in gates.
func DefaultExecutionProfileValidationOptions() ExecutionProfileValidationOptions {
	return ExecutionProfileValidationOptions{
		AllowProviderOverride:       true,
		AllowSandboxNone:            true,
		AllowSandboxRef:             true,
		MaxCoordinatorGuidanceBytes: defaultCoordinatorGuidanceMaxBytes,
	}
}

// Normalize returns the normalized coordinator mode.
func (m CoordinatorMode) Normalize() CoordinatorMode {
	return CoordinatorMode(strings.ToLower(strings.TrimSpace(string(m))))
}

// Normalize returns the normalized worker mode.
func (m WorkerMode) Normalize() WorkerMode {
	return WorkerMode(strings.ToLower(strings.TrimSpace(string(m))))
}

// Normalize returns the normalized sandbox mode.
func (m SandboxMode) Normalize() SandboxMode {
	return SandboxMode(strings.ToLower(strings.TrimSpace(string(m))))
}

// Normalize returns the normalized runtime mode.
func (m RuntimeMode) Normalize() RuntimeMode {
	return RuntimeMode(strings.ToLower(strings.TrimSpace(string(m))))
}

func normalizeCoordinatorProfile(profile CoordinatorProfile) CoordinatorProfile {
	profile.Mode = profile.Mode.Normalize()
	if profile.Mode == "" {
		profile.Mode = CoordinatorModeInherit
	}
	profile.AgentName = strings.TrimSpace(profile.AgentName)
	profile.Provider = strings.TrimSpace(profile.Provider)
	profile.Model = strings.TrimSpace(profile.Model)
	profile.Guidance = strings.TrimSpace(profile.Guidance)
	return profile
}

func normalizeWorkerProfile(profile WorkerProfile) WorkerProfile {
	profile.Mode = profile.Mode.Normalize()
	if profile.Mode == "" {
		profile.Mode = WorkerModeInherit
	}
	profile.AgentName = strings.TrimSpace(profile.AgentName)
	profile.Provider = strings.TrimSpace(profile.Provider)
	profile.Model = strings.TrimSpace(profile.Model)
	profile.AllowedAgentNames = normalizeProfileSelectorList(profile.AllowedAgentNames)
	profile.PreferredAgentNames = normalizeProfileSelectorList(profile.PreferredAgentNames)
	profile.RequiredCapabilities = normalizeProfileSelectorList(profile.RequiredCapabilities)
	profile.PreferredCapabilities = normalizeProfileSelectorList(profile.PreferredCapabilities)
	return profile
}

func normalizeReviewProfile(profile ReviewProfile) ReviewProfile {
	profile.AgentName = strings.TrimSpace(profile.AgentName)
	profile.Provider = strings.TrimSpace(profile.Provider)
	profile.Model = strings.TrimSpace(profile.Model)
	profile.AllowedAgentNames = normalizeProfileSelectorList(profile.AllowedAgentNames)
	profile.PreferredAgentNames = normalizeProfileSelectorList(profile.PreferredAgentNames)
	profile.AllowedChannelIDs = normalizeProfileSelectorList(profile.AllowedChannelIDs)
	profile.PreferredChannelIDs = normalizeProfileSelectorList(profile.PreferredChannelIDs)
	profile.AllowedPeerIDs = normalizeProfileSelectorList(profile.AllowedPeerIDs)
	profile.PreferredPeerIDs = normalizeProfileSelectorList(profile.PreferredPeerIDs)
	profile.RequiredCapabilities = normalizeProfileSelectorList(profile.RequiredCapabilities)
	profile.PreferredCapabilities = normalizeProfileSelectorList(profile.PreferredCapabilities)
	return profile
}

func normalizeParticipantPolicy(policy ParticipantPolicy) ParticipantPolicy {
	policy.AllowedChannelIDs = normalizeProfileSelectorList(policy.AllowedChannelIDs)
	policy.PreferredChannelIDs = normalizeProfileSelectorList(policy.PreferredChannelIDs)
	policy.AllowedPeerIDs = normalizeProfileSelectorList(policy.AllowedPeerIDs)
	policy.PreferredPeerIDs = normalizeProfileSelectorList(policy.PreferredPeerIDs)
	policy.AllowedAgentNames = normalizeProfileSelectorList(policy.AllowedAgentNames)
	policy.PreferredAgentNames = normalizeProfileSelectorList(policy.PreferredAgentNames)
	policy.RequiredCapabilities = normalizeProfileSelectorList(policy.RequiredCapabilities)
	policy.PreferredCapabilities = normalizeProfileSelectorList(policy.PreferredCapabilities)
	return policy
}

func normalizeSandboxPolicy(policy SandboxPolicy) SandboxPolicy {
	policy.Mode = policy.Mode.Normalize()
	if policy.Mode == "" {
		policy.Mode = SandboxModeInherit
	}
	policy.SandboxRef = strings.TrimSpace(policy.SandboxRef)
	return policy
}

func normalizeRuntimePolicy(policy RuntimePolicy) RuntimePolicy {
	policy.Mode = policy.Mode.Normalize()
	if policy.Mode == "" {
		policy.Mode = RuntimeModeDefault
	}
	return policy
}

func normalizeProfileSelectorList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}
