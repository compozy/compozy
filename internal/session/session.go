package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
	compozyconfig "github.com/compozy/compozy/internal/config"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

var (
	// ErrInvalidStateTransition reports that a session state transition is not allowed.
	ErrInvalidStateTransition = errors.New("session: invalid state transition")
	// ErrPromptInProgress reports that the session already has prompt setup or execution in flight.
	ErrPromptInProgress = errors.New("session: prompt already in progress")
	// ErrPromptNotInProgress reports that an operation requires an active prompt turn.
	ErrPromptNotInProgress = errors.New("session: prompt is not in progress")
	// ErrActiveTurnMismatch reports that a busy-input command targeted a different active turn.
	ErrActiveTurnMismatch = errors.New("session: active turn mismatch")
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
	LeaderKernelToken        string `json:"-"`
	BoardColumn              BoardColumn
	ParentTitle              string
	Children                 *store.SessionBoardCounts
	runtimeStrategy          acp.RuntimeApplicationStrategy
	VerdictOnly              bool `json:"-"`
	Supervision              *SupervisionState
	BusyInput                *BusyInputState
	ID                       string
	ProfileID                string
	Name                     string
	AgentName                string
	Provider                 string
	ProviderHomePolicy       compozyconfig.ProviderHomePolicy
	Model                    string
	ReasoningEffort          string
	Speed                    speedpkg.Speed
	ACPOptions               []acp.SessionConfigOptionSelection
	SpeedResolution          *speedpkg.Resolution
	RuntimeStatus            RuntimeStatus
	RuntimeTransition        RuntimeTransitionStrategy
	RuntimeFailure           string
	RuntimeGeneration        int64
	RuntimeRecovery          *store.SessionRuntimeRecovery
	SelectedRuntime          *RuntimeSelection
	RuntimeSelectionRevision int64
	EffectiveWorkspaceAccess string
	EffectivePermissions     string
	WorkspaceID              string
	Workspace                string
	WorktreeID               string
	Type                     Type
	Lineage                  *store.SessionLineage
	// Derivation is present only on continued or forked children.
	Derivation             *store.SessionDerivation
	State                  State
	PendingPermission      bool
	StopReason             store.StopReason
	StopCause              StopCause
	StopEscalated          bool
	StopVerificationFailed bool
	StopDetail             string
	Failure                *store.SessionFailure
	ACPSessionID           string
	ACPCaps                acp.Caps
	ACPCapsKnown           bool
	AdvertisedCommands     []store.SessionAdvertisedCommand
	Liveness               *store.SessionLivenessMeta
	SoulSnapshotID         string
	SoulDigest             string
	ParentSoulDigest       string
	AttachedTo             string
	AttachExpiresAt        *time.Time
	TranscriptEpoch        int64
	PendingPermissionCount int
	PendingClarifyCount    int
	AttentionRevision      int64
	LastSettledRevision    int64
	LastSeenRevision       int64
	LastSeenAt             *time.Time
	AttentionChangedAt     *time.Time
	// EndedEarlyTurnID names the settled turn that ended without delivering work.
	EndedEarlyTurnID    string
	PendingInteractions []store.PendingInteraction
	ArchivedAt          *time.Time
	// Later is the parked state of a stopped session; nil when it is not parked.
	Later *store.SessionLater
	// LaterReturnedAt marks a session brought back from Later until the operator sees or resumes it.
	LaterReturnedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Session is the in-memory runtime representation of one active or stopping session.
type Session struct {
	supervisionInspectMu         sync.Mutex
	supervisionState             *SupervisionState
	supervisionWarningEvent      *acp.AgentEvent
	supervisionStoppedEvent      *acp.AgentEvent
	stopStartedAt                time.Time
	supervisionStopAt            time.Time
	supervisionQuietSince        time.Time
	supervisionProgressAt        time.Time
	supervisionWarningRecordedAt time.Time
	supervisionReportedSources   []SignalSourceState
	// supervisionIdleSince starts when a session with no turn in flight goes quiet.
	supervisionIdleSince time.Time
	// runtimeRelease marks a runtime the supervisor released while the session was idle.
	runtimeRelease *runtimeRelease
	// Claimed finalization owns these fields across persistence retries.
	stopFinalizationErr     error
	stopVerifiedOutcome     StopOutcome // Protected by mu.
	stopStatePending        bool        // Protected by mu.
	stopTerminalEvent       *acp.AgentEvent
	stopTerminalRecorded    bool
	stopTurnID              string // Protected by mu.
	stopProcessExitRecorded bool
	postStopOnce            sync.Once
	followUpMode            func() string
	steerDelivery           store.SteerDeliveryMode
	mu                      sync.RWMutex
	persistMu               sync.Mutex
	// runtimeApplyMu serializes live runtime reconfiguration with prompt runtime admission.
	// Order: runtimeApplyMu before persistMu and mu.
	runtimeApplyMu             sync.Mutex
	goalStopMu                 sync.Mutex // Orders Goal pause persistence before interrupted setup recovery.
	conversationRewindReserved bool

	ID                string
	ProfileID         string
	Name              string
	AgentName         string
	Provider          string
	Model             string
	ReasoningEffort   string
	Speed             speedpkg.Speed
	ACPOptions        []acp.SessionConfigOptionSelection
	SpeedResolution   *speedpkg.Resolution
	RuntimeStatus     RuntimeStatus
	RuntimeTransition RuntimeTransitionStrategy
	RuntimeFailure    string
	// runtimeFailureFromLiveApply marks RuntimeFailure as a refused live selection apply,
	// which a newer saved selection supersedes. Protected by mu.
	runtimeFailureFromLiveApply bool
	// runtimeConfigUnverified marks a failed live rollback: the provider config may differ
	// from the effective selection until a configure or replacement succeeds. Protected by mu.
	runtimeConfigUnverified bool
	// liveTurnRuntime holds the runtime a running turn still uses after a live change its
	// provider honors only from the next turn. Protected by mu.
	liveTurnRuntime           *liveTurnRuntime
	RuntimeGeneration         int64
	RuntimeRecovery           *store.SessionRuntimeRecovery
	SelectedRuntime           *RuntimeSelection
	RuntimeSelectionRevision  int64
	EffectiveWorkspaceAccess  string
	EffectivePermissions      string
	effectiveProviderAuthMode compozyconfig.ProviderAuthMode
	providerHomePolicy        compozyconfig.ProviderHomePolicy
	WorkspaceID               string
	Workspace                 string
	WorktreeID                string
	CWD                       string
	Type                      Type
	Lineage                   *store.SessionLineage
	State                     State
	stopCause                 StopCause
	stopEscalated             bool
	stopVerificationFailed    bool
	stopReason                store.StopReason
	stopDetail                string
	failure                   *store.SessionFailure
	ACPSessionID              string
	ACPCaps                   acp.Caps
	ACPCapsKnown              bool
	AdvertisedCommands        []store.SessionAdvertisedCommand
	Liveness                  *store.SessionLivenessMeta
	SoulSnapshotID            string
	SoulDigest                string
	ParentSoulDigest          string
	AttachedTo                string
	AttachExpiresAt           *time.Time
	TranscriptEpoch           int64
	PendingPermissionCount    int
	PendingClarifyCount       int
	AttentionRevision         int64
	LastSettledRevision       int64
	LastSeenRevision          int64
	LastSeenAt                *time.Time
	AttentionChangedAt        *time.Time
	EndedEarlyTurnID          string
	PendingInteractions       []store.PendingInteraction
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	creationProfile           *store.SessionCreationProfile
	creationOptions           *store.SessionCreationOptions
	creationIdentity          *store.SessionCreationIdentity
	providerRoute             compozyconfig.ResolvedAgent
	acceptedRoute             *store.SessionAcceptedRoute
	restartBoundary           *store.SessionRestartCheckpoint
	// derivation and importedContext exist only on continued or forked children.
	derivation          *store.SessionDerivation
	importedContext     *store.SessionImportedContext
	pendingResumeReplay string
	// pendingDeriveReceipt is taken by the first catalog registration of a derived child.
	pendingDeriveReceipt *store.SessionDerivationReceipt
	// deriveCommitted marks the irreversible commit of a derived child's registration
	// transaction; from then on a failed start retains the child instead of sweeping it.
	deriveCommitted bool
	// acceptedCommand is the explicit route command of the accepted attempt ("" when the
	// route inherited its command); in-memory only, so automatic recovery keeps the seat.
	acceptedCommand        string
	acceptedSessionEnv     compozyconfig.SessionEnv
	agentDef               compozyconfig.AgentDef
	startupManifest        acp.StartupManifest
	nativeToolsUnavailable bool

	sessionDir                 string
	metaPath                   string
	dbPath                     string
	recorder                   EventRecorder
	process                    *AgentProcess
	worktreeForkReserved       bool
	worktreeBindingReserved    bool
	worktreeBindingAdmissionMu sync.RWMutex
	worktreeTransition         *store.SessionWorktreeTransition
	promptSetupCount           int
	promptSetupDone            chan struct{}
	currentTurnID              string
	currentTurnSource          TurnSource
	currentPromptMessage       string
	currentPromptDelivery      PromptDelivery
	currentPromptMeta          acp.PromptMeta
	currentTurnHopFloor        int
	currentSkillInvocations    []commandpkg.Invocation
	currentPromptCancel        context.CancelFunc
	currentPromptCancelTurn    string
	promptCancelRequested      bool
	turnStop                   *turnStopRun
	turnStopPending            bool
	currentPromptDone          chan struct{}
	providerRedactions         []func()
	// inputPromotionActive keeps at most one long-turn queue promotion pass running per session.
	inputPromotionActive atomic.Bool
	// promotedSteers holds long-turn promotions whose injection is still pending, so an asynchronous
	// fallback returns the message to after-turn delivery instead of leaving it as a queued steer.
	promotedSteers sync.Map
	// promotionDeclined maps a queued entry whose promotion input.pre_submit refused to the turn it was
	// refused in, so later passes of that turn leave it for ordinary after-turn dispatch.
	promotionDeclined sync.Map
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

	s.captureProcessCapabilitiesLocked()
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
	s.acceptedRoute = nil
	s.acceptedCommand = ""
	s.acceptedSessionEnv = nil
	s.ACPCaps = acp.Caps{}
	s.ACPCapsKnown = false
	s.Liveness = nil
	s.State = StateStarting
	if !now.IsZero() {
		s.UpdatedAt = now
	}
}

func (s *Session) observeRuntimeActivity(activity store.SessionActivityMeta, now time.Time) (string, string) {
	return s.observeRuntimeActivityState(activity, now, true)
}
