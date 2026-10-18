package store

import (
	"encoding/json"
	"fmt"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

// Retained creation witnesses are user data: their versioned field order and
// retired policy fields remain hash inputs. They never restore retired runtime
// behavior. Current creation continues to write version six exclusively.
type legacySessionCreationProfile struct {
	Version         int                         `json:"version"`
	AgentName       string                      `json:"agent_name"`
	Provider        string                      `json:"provider"`
	Model           string                      `json:"model,omitempty"`
	ReasoningEffort string                      `json:"reasoning_effort,omitempty"`
	Speed           speedpkg.Speed              `json:"speed"`
	ACPOptions      []SessionACPOptionSelection `json:"acp_options,omitempty"`
	ProfileID       string                      `json:"profile_id,omitempty"`
	WorkspaceID     string                      `json:"workspace_id"`
	CWD             string                      `json:"cwd"`
	WorktreeRef     string                      `json:"worktree_ref,omitempty"`
	SandboxMode     string                      `json:"sandbox_mode"`
	SandboxRef      string                      `json:"sandbox_ref,omitempty"`
	Permissions     string                      `json:"permissions"`
	AllowedTools    []string                    `json:"allowed_tools,omitempty"`
	AgentTools      []string                    `json:"agent_tools,omitempty"`
	AgentToolsets   []string                    `json:"agent_toolsets,omitempty"`
	DeniedTools     []string                    `json:"denied_tools,omitempty"`
	RuntimeMode     string                      `json:"runtime_mode,omitempty"`
	PromptOverlay   string                      `json:"prompt_overlay,omitempty"`
	ContractOverlay string                      `json:"contract_overlay,omitempty"`
}

type sessionCreationProfileJSON SessionCreationProfile

// UnmarshalJSON retains retired policy fields at the persistence boundary.
func (p *SessionCreationProfile) UnmarshalJSON(data []byte) error {
	var current sessionCreationProfileJSON
	wire := struct {
		*sessionCreationProfileJSON
		SandboxMode string `json:"sandbox_mode"`
		SandboxRef  string `json:"sandbox_ref"`
	}{sessionCreationProfileJSON: &current}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*p = SessionCreationProfile(current)
	if p.Version >= 3 && p.Version < SessionCreationProfileVersion {
		p.legacySandboxMode = wire.SandboxMode
		p.legacySandboxRef = wire.SandboxRef
	}
	return nil
}

// MarshalJSON preserves each retained version's original content address.
func (p SessionCreationProfile) MarshalJSON() ([]byte, error) {
	if p.Version < 3 || p.Version >= SessionCreationProfileVersion {
		return json.Marshal(sessionCreationProfileJSON(p))
	}
	return json.Marshal(legacySessionCreationProfile{
		Version: p.Version, AgentName: p.AgentName, Provider: p.Provider,
		Model: p.Model, ReasoningEffort: p.ReasoningEffort, Speed: p.Speed,
		ACPOptions: p.ACPOptions, ProfileID: p.ProfileID, WorkspaceID: p.WorkspaceID,
		CWD: p.CWD, WorktreeRef: p.WorktreeRef,
		SandboxMode: p.legacySandboxMode, SandboxRef: p.legacySandboxRef,
		Permissions: p.Permissions, AllowedTools: p.AllowedTools, AgentTools: p.AgentTools,
		AgentToolsets: p.AgentToolsets, DeniedTools: p.DeniedTools, RuntimeMode: p.RuntimeMode,
		PromptOverlay: p.PromptOverlay, ContractOverlay: p.ContractOverlay,
	})
}

func (p SessionCreationProfile) validateLegacyWitness() error {
	if p.Version == 3 && p.ProfileID != "" {
		return fmt.Errorf("store: session creation profile version 3 cannot contain profile_id")
	}
	if p.Version < 5 && len(p.ACPOptions) != 0 {
		return fmt.Errorf("store: session creation profile version %d cannot contain ACP options", p.Version)
	}
	switch p.legacySandboxMode {
	case "none":
		if p.legacySandboxRef != "" {
			return fmt.Errorf("store: sandbox_ref requires sandbox_mode=ref")
		}
	case "ref":
		if p.legacySandboxRef == "" {
			return fmt.Errorf("store: sandbox_ref is required for sandbox_mode=ref")
		}
	default:
		return fmt.Errorf("store: invalid retained session creation sandbox mode %q", p.legacySandboxMode)
	}
	return nil
}

type sessionCreationOptionsJSON SessionCreationOptions

// UnmarshalJSON keeps retired Network policy as opaque, hash-bound provenance.
func (o *SessionCreationOptions) UnmarshalJSON(data []byte) error {
	var current sessionCreationOptionsJSON
	wire := struct {
		*sessionCreationOptionsJSON
		NetworkOwnerKey      string          `json:"network_owner_key"`
		NetworkParticipation json.RawMessage `json:"network_participation"`
	}{sessionCreationOptionsJSON: &current}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*o = SessionCreationOptions(current)
	o.legacyNetworkOwnerKey = wire.NetworkOwnerKey
	o.legacyNetworkParticipation = wire.NetworkParticipation
	return nil
}

// MarshalJSON retains the option order used by version three through five.
func (o SessionCreationOptions) MarshalJSON() ([]byte, error) {
	if o.legacyNetworkOwnerKey == "" && len(o.legacyNetworkParticipation) == 0 {
		return json.Marshal(sessionCreationOptionsJSON(o))
	}
	return json.Marshal(struct {
		SessionID            string          `json:"session_id"`
		Name                 string          `json:"name,omitempty"`
		NetworkOwnerKey      string          `json:"network_owner_key"`
		NetworkParticipation json.RawMessage `json:"network_participation"`
		SessionType          string          `json:"session_type"`
	}{o.SessionID, o.Name, o.legacyNetworkOwnerKey, o.legacyNetworkParticipation, o.SessionType})
}
