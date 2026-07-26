package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

const SessionCreationProfileVersion = 1

const (
	SessionCreationSandboxNone = "none"
	SessionCreationSandboxRef  = "ref"
)

// SessionCreationProfile is the immutable, secret-free runtime policy used to
// create or reseed a session. New fields require a profile version bump.
type SessionCreationProfile struct {
	Version         int      `json:"version"`
	AgentName       string   `json:"agent_name"`
	Provider        string   `json:"provider"`
	Model           string   `json:"model,omitempty"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
	WorkspaceID     string   `json:"workspace_id"`
	CWD             string   `json:"cwd"`
	SandboxMode     string   `json:"sandbox_mode"`
	SandboxRef      string   `json:"sandbox_ref,omitempty"`
	Permissions     string   `json:"permissions"`
	AllowedTools    []string `json:"allowed_tools,omitempty"`
	AgentTools      []string `json:"agent_tools,omitempty"`
	AgentToolsets   []string `json:"agent_toolsets,omitempty"`
	DeniedTools     []string `json:"denied_tools,omitempty"`
	RuntimeMode     string   `json:"runtime_mode,omitempty"`
	PromptOverlay   string   `json:"prompt_overlay,omitempty"`
	ContractOverlay string   `json:"contract_overlay,omitempty"`
}

// SessionCreationOptions are deterministic per-session options excluded from
// the stable policy digest but included in the creation digest.
type SessionCreationOptions struct {
	SessionID            string             `json:"session_id"`
	Name                 string             `json:"name,omitempty"`
	NetworkOwnerKey      string             `json:"network_owner_key"`
	NetworkParticipation participation.Spec `json:"network_participation"`
	SessionType          string             `json:"session_type"`
}

// NormalizeSessionCreationProfile produces the only canonical digest input.
func NormalizeSessionCreationProfile(profile SessionCreationProfile) SessionCreationProfile {
	profile.AgentName = strings.TrimSpace(profile.AgentName)
	profile.Provider = strings.TrimSpace(profile.Provider)
	profile.Model = strings.TrimSpace(profile.Model)
	profile.ReasoningEffort = strings.TrimSpace(profile.ReasoningEffort)
	profile.WorkspaceID = strings.TrimSpace(profile.WorkspaceID)
	profile.CWD = strings.TrimSpace(profile.CWD)
	profile.SandboxMode = strings.ToLower(strings.TrimSpace(profile.SandboxMode))
	profile.SandboxRef = strings.TrimSpace(profile.SandboxRef)
	profile.Permissions = strings.TrimSpace(profile.Permissions)
	profile.RuntimeMode = strings.ToLower(strings.TrimSpace(profile.RuntimeMode))
	profile.PromptOverlay = strings.TrimSpace(profile.PromptOverlay)
	profile.ContractOverlay = strings.TrimSpace(profile.ContractOverlay)
	profile.AllowedTools = normalizeCreationProfileStrings(profile.AllowedTools)
	profile.AgentTools = normalizeCreationProfileStrings(profile.AgentTools)
	profile.AgentToolsets = normalizeCreationProfileStrings(profile.AgentToolsets)
	profile.DeniedTools = normalizeCreationProfileStrings(profile.DeniedTools)
	return profile
}

// Validate enforces the complete immutable profile contract.
func (p SessionCreationProfile) Validate() error {
	p = NormalizeSessionCreationProfile(p)
	if p.Version != SessionCreationProfileVersion {
		return fmt.Errorf("store: unsupported session creation profile version %d", p.Version)
	}
	required := []struct {
		name  string
		value string
	}{
		{name: "agent_name", value: p.AgentName},
		{name: "provider", value: p.Provider},
		{name: "workspace_id", value: p.WorkspaceID},
		{name: "cwd", value: p.CWD},
	}
	for _, field := range required {
		if field.value == "" {
			return fmt.Errorf("store: session creation profile %s is required", field.name)
		}
	}
	switch p.SandboxMode {
	case SessionCreationSandboxNone:
		if p.SandboxRef != "" {
			return fmt.Errorf("store: sandbox_ref requires sandbox_mode=ref")
		}
	case SessionCreationSandboxRef:
		if p.SandboxRef == "" {
			return fmt.Errorf("store: sandbox_ref is required for sandbox_mode=ref")
		}
	default:
		return fmt.Errorf("store: invalid session creation sandbox mode %q", p.SandboxMode)
	}
	return nil
}

// CanonicalJSON returns the stable serialized profile used by persistence and hashing.
func (p SessionCreationProfile) CanonicalJSON() ([]byte, error) {
	normalized := NormalizeSessionCreationProfile(p)
	if err := normalized.Validate(); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("store: marshal session creation profile: %w", err)
	}
	return payload, nil
}

// Ref returns the content-addressed immutable profile reference.
func (p SessionCreationProfile) Ref() (string, error) {
	payload, err := p.CanonicalJSON()
	if err != nil {
		return "", err
	}
	return "profile:sha256:" + sha256Hex(payload), nil
}

// PolicySpecDigest hashes stable effective policy/profile/contract inputs.
func (p SessionCreationProfile) PolicySpecDigest() (string, error) {
	payload, err := p.CanonicalJSON()
	if err != nil {
		return "", err
	}
	return "policy:sha256:" + sha256Hex(append([]byte("agh-session-policy-v1\x00"), payload...)), nil
}

// CreationDigest binds a stable policy to one preallocated session identity.
func (p SessionCreationProfile) CreationDigest(opts SessionCreationOptions) (string, error) {
	policyDigest, err := p.PolicySpecDigest()
	if err != nil {
		return "", err
	}
	opts.SessionID = strings.TrimSpace(opts.SessionID)
	opts.Name = strings.TrimSpace(opts.Name)
	opts.NetworkOwnerKey = strings.TrimSpace(opts.NetworkOwnerKey)
	opts.SessionType = strings.TrimSpace(opts.SessionType)
	if opts.SessionID == "" || opts.SessionType == "" {
		return "", fmt.Errorf("store: session creation digest requires session_id and session_type")
	}
	if err := participation.ValidateSpec(opts.NetworkParticipation); err != nil {
		return "", fmt.Errorf("store: validate session creation participation: %w", err)
	}
	if err := participation.ValidateOwnerKey(opts.NetworkOwnerKey); err != nil {
		return "", fmt.Errorf("store: validate session creation network owner: %w", err)
	}
	creationJSON, err := json.Marshal(opts)
	if err != nil {
		return "", fmt.Errorf("store: marshal session creation options: %w", err)
	}
	material := append([]byte(policyDigest+"\x00"), creationJSON...)
	return "creation:sha256:" + sha256Hex(material), nil
}

// SessionCreationIdentityStore persists the immutable per-session witness.
type SessionCreationIdentityStore interface {
	RegisterSessionWithCreationIdentity(
		context.Context,
		SessionInfo,
		SessionCreationIdentity,
	) (SessionCreationRegistration, error)
	GetSessionCreationIdentity(context.Context, string) (SessionCreationIdentity, error)
}

// SessionCreationProfileStore persists and loads content-addressed profiles.
type SessionCreationProfileStore interface {
	PutSessionCreationProfile(context.Context, SessionCreationProfile) (string, error)
	GetSessionCreationProfile(context.Context, string) (SessionCreationProfile, error)
}

// SessionCreationStore is the complete durable idempotent-creation catalog.
type SessionCreationStore interface {
	SessionCreationIdentityStore
	SessionCreationProfileStore
}

func normalizeCreationProfileStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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
	slices.Sort(normalized)
	return normalized
}

func sha256Hex(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}
