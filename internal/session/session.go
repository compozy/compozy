package session

import (
	"context"
	"errors"
	"fmt"

	"sync"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
)

var (
	// ErrInvalidStateTransition reports that a session state transition is not allowed.
	ErrInvalidStateTransition = errors.New("session: invalid state transition")
	// ErrPromptInProgress reports that the session already has prompt setup or execution in flight.
	ErrPromptInProgress = errors.New("session: prompt already in progress")
	// ErrPromptNotInProgress reports that an operation requires an active prompt turn.
	ErrPromptNotInProgress = errors.New("session: prompt is not in progress")
)

// State is the lifecycle state of a managed runtime session.
type State string

const (
	StateStarting State = "starting"
	StateActive   State = "active"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
)

// Type identifies why a session was created.
type Type string

const (
	SessionTypeUser        Type = "user"
	SessionTypeDream       Type = "dream"
	SessionTypeSystem      Type = "system"
	SessionTypeCoordinator Type = "coordinator"
	SessionTypeSpawned     Type = "spawned"
)

const (
	// EventTypeSessionStopped is emitted when a session transitions to the stopped state.
	EventTypeSessionStopped = "session_stopped"
)

// Info is the external read model returned by session list/get operations.
type Info struct {
	ID                   string
	Name                 string
	AgentName            string
	Provider             string
	Model                string
	ReasoningEffort      string
	WorkspaceID          string
	Workspace            string
	NetworkParticipation participation.Spec
	NetworkOwnerKey      string
	Type                 Type
	Lineage              *store.SessionLineage
	State                State
	StopReason           store.StopReason
	StopDetail           string
	Failure              *store.SessionFailure
	ACPSessionID         string
	ACPCaps              acp.Caps
	AdvertisedCommands   []store.SessionAdvertisedCommand
	Liveness             *store.SessionLivenessMeta
	Sandbox              *store.SessionSandboxMeta
	SoulSnapshotID       string
	SoulDigest           string
	ParentSoulDigest     string
	AttachedTo           string
	AttachExpiresAt      *time.Time
	TranscriptEpoch      int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Session is the in-memory runtime representation of one active or stopping session.
type Session struct {
	mu sync.RWMutex

	ID                   string
	Name                 string
	AgentName            string
	Provider             string
	Model                string
	ReasoningEffort      string
	EffectivePermissions string
	WorkspaceID          string
	Workspace            string
	CWD                  string
	NetworkParticipation participation.Spec
	NetworkOwnerKey      string
	Type                 Type
	Lineage              *store.SessionLineage
	State                State
	stopCause            StopCause
	stopReason           store.StopReason
	stopDetail           string
	failure              *store.SessionFailure
	ACPSessionID         string
	ACPCaps              acp.Caps
	AdvertisedCommands   []store.SessionAdvertisedCommand
	Liveness             *store.SessionLivenessMeta
	Sandbox              *store.SessionSandboxMeta
	SoulSnapshotID       string
	SoulDigest           string
	ParentSoulDigest     string
	AttachedTo           string
	AttachExpiresAt      *time.Time
	TranscriptEpoch      int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
	creationProfile      *store.SessionCreationProfile
	creationOptions      *store.SessionCreationOptions
	creationIdentity     *store.SessionCreationIdentity
	agentDef             aghconfig.AgentDef

	sessionDir string
	metaPath   string
	dbPath     string
	recorder   EventRecorder
	process    *AgentProcess

	sandboxDestroyOnStop bool
	promptSetupCount     int
	promptSetupDone      chan struct{}
	currentTurnID        string
	currentTurnSource    TurnSource
	currentPromptMessage string
	currentPromptMeta    acp.PromptMeta
	currentPromptCancel  context.CancelFunc
	providerRedactions   []func()
}

func (s *Session) processHandle() *AgentProcess {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.process
}

func (s *Session) addProviderSecretRedactions(cleanups []func()) {
	if s == nil || len(cleanups) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providerRedactions = append(s.providerRedactions, cleanups...)
}

func (s *Session) clearProviderSecretRedactions() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cleanups := append([]func(){}, s.providerRedactions...)
	s.providerRedactions = nil
	s.mu.Unlock()
	runProviderSecretRedactions(cleanups)
}

// ApprovePermission resolves one pending permission request for an active session.
func (s *Session) ApprovePermission(ctx context.Context, req acp.ApproveRequest) error {
	if s == nil {
		return errors.New("session: session is required")
	}
	if ctx == nil {
		return errors.New("session: approval context is required")
	}

	s.mu.RLock()
	state := s.State
	process := s.process
	s.mu.RUnlock()

	if state != StateActive {
		return fmt.Errorf("%w: %s", ErrSessionNotActive, s.ID)
	}
	if process == nil {
		return errors.New("session: agent process is not available")
	}
	return process.ApprovePermission(ctx, req)
}

// RequestPermission asks the active session process for a permission decision.
func (s *Session) RequestPermission(
	ctx context.Context,
	req acp.RequestPermissionRequest,
) (acp.RequestPermissionResponse, error) {
	if s == nil {
		return acp.RequestPermissionResponse{}, errors.New("session: session is required")
	}
	if ctx == nil {
		return acp.RequestPermissionResponse{}, errors.New("session: permission context is required")
	}

	s.mu.RLock()
	state := s.State
	process := s.process
	s.mu.RUnlock()

	if state != StateActive {
		return acp.RequestPermissionResponse{}, fmt.Errorf("%w: %s", ErrSessionNotActive, s.ID)
	}
	if process == nil {
		return acp.RequestPermissionResponse{}, errors.New("session: agent process is not available")
	}
	return process.RequestPermission(ctx, req)
}

func (s *Session) recorderHandle() EventRecorder {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recorder
}

func (s *Session) clearProcess(now time.Time) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.process = nil
	if !now.IsZero() {
		s.UpdatedAt = now
	}
}

func (s *Session) rollbackActivation(now time.Time) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.process = nil
	s.ACPSessionID = ""
	s.ACPCaps = acp.Caps{}
	s.Liveness = nil
	s.State = StateStarting
	if !now.IsZero() {
		s.UpdatedAt = now
	}
}

func (s *Session) observeRuntimeActivity(activity store.SessionActivityMeta, now time.Time) (string, string) {
	return s.observeRuntimeActivityState(activity, now, true)
}
