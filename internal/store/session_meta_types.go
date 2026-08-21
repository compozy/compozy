package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

// SessionSandboxMeta is the persisted runtime sandbox state for a session.
type SessionSandboxMeta struct {
	SandboxID             string          `json:"sandbox_id,omitempty"`
	Backend               string          `json:"backend"`
	Profile               string          `json:"profile,omitempty"`
	State                 string          `json:"state,omitempty"`
	InstanceID            string          `json:"instance_id,omitempty"`
	RuntimeRootDir        string          `json:"runtime_root_dir,omitempty"`
	RuntimeAdditionalDirs []string        `json:"runtime_additional_dirs,omitempty"`
	ProviderState         json.RawMessage `json:"provider_state,omitempty"`
	SSHAccessExpiresAt    *time.Time      `json:"ssh_access_expires_at,omitempty"`
	LastSyncAt            *time.Time      `json:"last_sync_at,omitempty"`
	LastSyncError         string          `json:"last_sync_error,omitempty"`
}

// SessionWorktreeState keeps the optional worktree binding compact inside SessionMeta.
// Embedding preserves the flat session metadata JSON contract.
type SessionWorktreeState struct {
	WorktreeID string `json:"worktree_id,omitempty"`
}

// SessionAdvertisedCommandState keeps the optional command catalog compact inside SessionMeta.
// Embedding preserves the flat session metadata JSON contract.
type SessionAdvertisedCommandState struct {
	AdvertisedCommands []SessionAdvertisedCommand `json:"advertised_commands,omitempty"`
}

// SessionProviderAuthState keeps the resolved provider authentication owner compact inside SessionMeta.
// Embedding preserves the flat session metadata JSON contract.
type SessionProviderAuthState struct {
	EffectiveProviderAuthMode string `json:"effective_provider_auth_mode,omitempty"`
}

// SessionMeta is the atomically-written session metadata document.
type SessionMeta struct {
	ID                   string                        `json:"id"`
	Name                 string                        `json:"name,omitempty"`
	AgentName            string                        `json:"agent_name"`
	Provider             string                        `json:"provider,omitempty"`
	Model                string                        `json:"model,omitempty"`
	ReasoningEffort      string                        `json:"reasoning_effort,omitempty"`
	Speed                speedpkg.Speed                `json:"speed,omitempty"`
	SpeedResolution      *speedpkg.Resolution          `json:"speed_resolution,omitempty"`
	RuntimeStatus        SessionRuntimeStatus          `json:"runtime_status"`
	RuntimeTransition    SessionRuntimeTransition      `json:"runtime_transition,omitempty"`
	RuntimeFailure       *string                       `json:"runtime_failure,omitempty"`
	RuntimeSelection     *SessionRuntimeSelectionState `json:"runtime_selection,omitempty"`
	EffectivePermissions string                        `json:"effective_permissions,omitempty"`
	*SessionProviderAuthState
	WorkspaceID string `json:"workspace_id,omitempty"`
	*SessionWorktreeState
	CWD                  string                  `json:"cwd,omitempty"`
	NetworkParticipation *participation.Spec     `json:"network_participation"`
	SessionType          string                  `json:"session_type,omitempty"`
	Lineage              *SessionLineage         `json:"lineage,omitempty"`
	State                string                  `json:"state"`
	StopReason           *StopReason             `json:"stop_reason,omitempty"`
	StopDetail           string                  `json:"stop_detail,omitempty"`
	Failure              *SessionFailure         `json:"failure,omitempty"`
	ACPSessionID         *string                 `json:"acp_session_id,omitempty"`
	Liveness             *SessionLivenessMeta    `json:"liveness,omitempty"`
	Sandbox              *SessionSandboxMeta     `json:"sandbox,omitempty"`
	CreationProfile      *SessionCreationProfile `json:"creation_profile,omitempty"`
	CreationOptions      *SessionCreationOptions `json:"creation_options,omitempty"`
	CreationProfileRef   string                  `json:"creation_profile_ref,omitempty"`
	PolicySpecDigest     string                  `json:"policy_spec_digest,omitempty"`
	CreationDigest       string                  `json:"creation_digest,omitempty"`
	*SessionAdvertisedCommandState
	SoulSnapshotID   string    `json:"soul_snapshot_id,omitempty"`
	SoulDigest       string    `json:"soul_digest,omitempty"`
	ParentSoulDigest string    `json:"parent_soul_digest,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// WorktreeIDValue returns the optional worktree binding without exposing nil embedding details.
func (m SessionMeta) WorktreeIDValue() string {
	if m.SessionWorktreeState == nil {
		return ""
	}
	return m.WorktreeID
}

// SetWorktreeID updates the optional worktree binding with value semantics.
func (m *SessionMeta) SetWorktreeID(worktreeID string) {
	if worktreeID == "" {
		m.SessionWorktreeState = nil
		return
	}
	m.SessionWorktreeState = &SessionWorktreeState{WorktreeID: worktreeID}
}

// AdvertisedCommandsValue returns the optional advertised command catalog.
func (m *SessionMeta) AdvertisedCommandsValue() []SessionAdvertisedCommand {
	if m == nil || m.SessionAdvertisedCommandState == nil {
		return nil
	}
	return m.AdvertisedCommands
}

// SetAdvertisedCommands stores an ownership-safe advertised command catalog.
func (m *SessionMeta) SetAdvertisedCommands(commands []SessionAdvertisedCommand) {
	if len(commands) == 0 {
		m.SessionAdvertisedCommandState = nil
		return
	}
	m.SessionAdvertisedCommandState = &SessionAdvertisedCommandState{
		AdvertisedCommands: CloneSessionAdvertisedCommands(commands),
	}
}

// EffectiveProviderAuthModeValue returns the persisted provider authentication owner.
func (m SessionMeta) EffectiveProviderAuthModeValue() string {
	if m.SessionProviderAuthState == nil {
		return ""
	}
	return m.EffectiveProviderAuthMode
}

// SetEffectiveProviderAuthMode updates the optional provider authentication owner.
func (m *SessionMeta) SetEffectiveProviderAuthMode(mode string) {
	if mode == "" {
		m.SessionProviderAuthState = nil
		return
	}
	m.SessionProviderAuthState = &SessionProviderAuthState{EffectiveProviderAuthMode: mode}
}

// Validate ensures the metadata file remains aligned with the session index schema.
func (m SessionMeta) Validate() error {
	if err := requireField(m.ID, "session id"); err != nil {
		return err
	}
	if err := requireField(m.AgentName, "session agent name"); err != nil {
		return err
	}
	if err := requireField(m.WorkspaceID, "session workspace id"); err != nil {
		return err
	}
	if err := requireField(m.State, "session state"); err != nil {
		return err
	}
	if err := participation.ValidateSpec(m.NetworkSpecSnapshot()); err != nil {
		return fmt.Errorf("store: validate session network participation: %w", err)
	}
	if m.StopReason != nil {
		if err := validateSessionStopReason(*m.StopReason); err != nil {
			return err
		}
	}
	if err := ValidateSessionLineage(m.ID, m.Lineage); err != nil {
		return err
	}
	if m.Failure != nil {
		if err := m.Failure.Validate(); err != nil {
			return err
		}
	}
	if err := m.Liveness.Validate(); err != nil {
		return err
	}
	if err := validateSessionCreationMetadata(m); err != nil {
		return err
	}
	if err := validateSessionSpeedMetadata(m.Speed, m.SpeedResolution); err != nil {
		return err
	}
	if err := validateSessionRuntime(
		m.RuntimeStatus,
		m.RuntimeTransition,
		SessionRuntimeFailureValue(m.RuntimeFailure),
	); err != nil {
		return err
	}
	if err := ValidateSessionRuntimeSelectionState(m.RuntimeSelection); err != nil {
		return err
	}
	advertisedCommands := m.AdvertisedCommandsValue()
	seenCommands := make(map[string]struct{}, len(advertisedCommands))
	for _, command := range advertisedCommands {
		if err := command.Validate(); err != nil {
			return err
		}
		name := strings.TrimSpace(command.Name)
		if _, exists := seenCommands[name]; exists {
			return fmt.Errorf("store: duplicate advertised command %q", name)
		}
		seenCommands[name] = struct{}{}
	}
	if err := validateSessionSoulProvenance(m.SoulSnapshotID, m.SoulDigest); err != nil {
		return err
	}
	return nil
}

func validateSessionSpeedMetadata(speed speedpkg.Speed, resolution *speedpkg.Resolution) error {
	if speed != "" {
		if _, err := speedpkg.Parse(string(speed)); err != nil {
			return fmt.Errorf("store: validate session speed: %w", err)
		}
	}
	if resolution == nil {
		return nil
	}
	if err := speedpkg.ValidateResolution(*resolution); err != nil {
		return fmt.Errorf("store: validate session speed resolution: %w", err)
	}
	if speed == "" {
		return fmt.Errorf("store: session speed resolution requires speed")
	}
	if resolution.Requested != speed {
		return fmt.Errorf(
			"store: session speed resolution requested %q does not match speed %q",
			resolution.Requested,
			speed,
		)
	}
	return nil
}

// NetworkSpecSnapshot returns the immutable participation snapshot stored in metadata.
func (m SessionMeta) NetworkSpecSnapshot() participation.Spec {
	if m.NetworkParticipation == nil {
		return participation.Spec{}
	}
	return *m.NetworkParticipation
}

// NetworkOwnerKeySnapshot returns the immutable budget owner stored in creation metadata.
func (m SessionMeta) NetworkOwnerKeySnapshot() string {
	if m.CreationOptions != nil && strings.TrimSpace(m.CreationOptions.NetworkOwnerKey) != "" {
		return strings.TrimSpace(m.CreationOptions.NetworkOwnerKey)
	}
	return participation.OwnerKey(participation.OwnerRef{Kind: participation.OwnerKindSession, ID: m.ID})
}

func validateSessionCreationMetadata(meta SessionMeta) error {
	identity := SessionCreationIdentity{
		CreationProfileRef: meta.CreationProfileRef,
		PolicySpecDigest:   meta.PolicySpecDigest,
		CreationDigest:     meta.CreationDigest,
	}
	identityPresent := meta.CreationProfileRef != "" || meta.PolicySpecDigest != "" || meta.CreationDigest != ""
	if !identityPresent && meta.CreationProfile == nil && meta.CreationOptions == nil {
		return nil
	}
	if meta.CreationProfile == nil || meta.CreationOptions == nil {
		return fmt.Errorf("store: session creation profile and options are required with identity")
	}
	if meta.NetworkSpecSnapshot() != meta.CreationOptions.NetworkParticipation {
		return fmt.Errorf("store: session network participation does not match creation options")
	}
	if err := identity.Validate(); err != nil {
		return err
	}
	profileRef, err := meta.CreationProfile.Ref()
	if err != nil {
		return err
	}
	if profileRef != identity.CreationProfileRef {
		return fmt.Errorf("store: session creation profile ref does not match profile body")
	}
	policyDigest, err := meta.CreationProfile.PolicySpecDigest()
	if err != nil {
		return err
	}
	if policyDigest != identity.PolicySpecDigest {
		return fmt.Errorf("store: session policy digest does not match profile body")
	}
	creationDigest, err := meta.CreationProfile.CreationDigest(*meta.CreationOptions)
	if err != nil {
		return err
	}
	if creationDigest != identity.CreationDigest {
		return fmt.Errorf("store: session creation digest does not match metadata identity")
	}
	return nil
}
