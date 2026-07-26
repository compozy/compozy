package daemon

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
)

const (
	harnessContextFalseKey                   = "false"
	harnessContextHarnessNetworkLivePath     = "harness.network_live"
	harnessContextHarnessDiagnosticLabelPath = "harness.diagnostic_label"
	harnessContextHarnessSessionClassPath    = "harness.session_class"
	harnessContextHarnessSessionTypePath     = "harness.session_type"
	harnessContextHarnessSurfacePath         = "harness.surface"
	harnessContextHarnessTurnOriginPath      = "harness.turn_origin"
	harnessContextTrueKey                    = "true"
)

// TurnOrigin identifies the resolved harness origin for one turn.
type TurnOrigin string

const (
	// TurnOriginUser identifies a standard user-driven turn.
	TurnOriginUser TurnOrigin = "user"
	// TurnOriginNetwork identifies a network-originated turn.
	TurnOriginNetwork TurnOrigin = "network"
	// TurnOriginSynthetic identifies a daemon-owned synthetic turn.
	TurnOriginSynthetic TurnOrigin = "synthetic"
)

// SessionClass is the session-side harness axis derived from durable session
// metadata rather than a top-level profile enum.
type SessionClass string

const (
	// SessionClassInteractive identifies user-owned interactive sessions.
	SessionClassInteractive SessionClass = "interactive"
	// SessionClassDream identifies dream consolidation sessions.
	SessionClassDream SessionClass = "dream"
	// SessionClassSystem identifies daemon-owned system sessions.
	SessionClassSystem SessionClass = "system"
	// SessionClassCoordinator identifies daemon-owned workspace coordinator sessions.
	SessionClassCoordinator SessionClass = "coordinator"
	// SessionClassSpawned identifies bounded child worker sessions.
	SessionClassSpawned SessionClass = "spawned"
)

// HarnessPromptSection identifies a startup prompt section managed by harness policy.
type HarnessPromptSection string

const (
	// HarnessPromptSectionRuntimeIdentity injects the daemon-owned AGH runtime envelope.
	HarnessPromptSectionRuntimeIdentity HarnessPromptSection = "runtime_identity"
	// HarnessPromptSectionSituation injects the bounded AGH situation context.
	HarnessPromptSectionSituation HarnessPromptSection = "situation"
	// HarnessPromptSectionMemory injects durable memory prompt context.
	HarnessPromptSectionMemory HarnessPromptSection = "memory"
	// HarnessPromptSectionSkills injects the active skills catalog prompt context.
	HarnessPromptSectionSkills HarnessPromptSection = "skills"
	// HarnessPromptSectionTools injects AGH-native tool discovery and invocation guidance.
	HarnessPromptSectionTools HarnessPromptSection = "tools"
	// HarnessPromptSectionNetwork injects the bundled AGH network startup section.
	HarnessPromptSectionNetwork HarnessPromptSection = "network"
)

// HarnessAugmenter identifies a prompt input augmenter managed by harness policy.
type HarnessAugmenter string

const (
	// HarnessAugmenterSituation injects fresh bounded AGH situation context.
	HarnessAugmenterSituation HarnessAugmenter = "situation"
	// HarnessAugmenterSkills injects the current effective skills catalog.
	HarnessAugmenterSkills HarnessAugmenter = "skills"
	// HarnessAugmenterDurableMemory enables the durable memory recall augmenter.
	HarnessAugmenterDurableMemory HarnessAugmenter = "durable_memory"
)

// ReentryMode identifies how a resolved policy participates in synthetic reentry.
type ReentryMode string

const (
	// ReentryModeNone means the turn is not synthetic reentry.
	ReentryModeNone ReentryMode = "none"
	// ReentryModeSynthetic means the turn is a validated synthetic reentry path.
	ReentryModeSynthetic ReentryMode = "synthetic"
)

// DetachedRunMode identifies detached task-runtime behavior for the resolved policy.
type DetachedRunMode string

const (
	// DetachedRunModeNone means detached task-runtime behavior is not enabled.
	DetachedRunModeNone DetachedRunMode = "none"
	// DetachedRunModeTaskRuntime means detached work reuses the existing task runtime.
	DetachedRunModeTaskRuntime DetachedRunMode = "task_runtime"
)

// ResolutionSurface identifies which runtime seam is consuming the resolver.
type ResolutionSurface string

const (
	// ResolutionSurfaceStartup resolves startup prompt policy.
	ResolutionSurfaceStartup ResolutionSurface = "startup"
	// ResolutionSurfaceTurn resolves live prompt-turn policy.
	ResolutionSurfaceTurn ResolutionSurface = "turn"
)

// HarnessRuntimeSignals captures daemon-owned capability flags available to the resolver.
type HarnessRuntimeSignals struct {
	RuntimeIdentityPromptSectionEnabled bool
	SituationPromptSectionEnabled       bool
	MemoryPromptSectionEnabled          bool
	SkillsPromptSectionEnabled          bool
	ToolsPromptSectionEnabled           bool
	SkillsAugmenter                     bool
	SituationAugmenter                  bool
	DurableMemoryAugmenter              bool
	SyntheticTurnsEnabled               bool
	DetachedTaskRuntimeEnabled          bool
}

// SyntheticTurnMetadata carries validated daemon-only synthetic reentry metadata.
type SyntheticTurnMetadata struct {
	Reason      string
	Trigger     string
	SourceTask  string
	SourceRunID string
}

// DetachedRunMetadata carries detached task-runtime correlation fields.
type DetachedRunMetadata struct {
	TaskID    string
	TaskRunID string
}

// HarnessTurnRequest carries turn-time metadata into the resolver.
type HarnessTurnRequest struct {
	Source     session.TurnSource
	PromptMeta acp.PromptMeta
	Synthetic  *SyntheticTurnMetadata
	Detached   *DetachedRunMetadata
}

// HarnessResolutionInput captures one resolver request.
type HarnessResolutionInput struct {
	Surface ResolutionSurface
	Session HarnessSessionInput
	Turn    HarnessTurnRequest
}

// HarnessTurnContext is the normalized turn context emitted by the resolver.
type HarnessTurnContext struct {
	Origin     TurnOrigin
	PromptMeta acp.PromptMeta
	Synthetic  *SyntheticTurnMetadata
	Detached   *DetachedRunMetadata
}

// ResolvedHarnessPolicy is the authoritative daemon-owned harness runtime policy.
type ResolvedHarnessPolicy struct {
	SessionClass      SessionClass
	TurnOrigin        TurnOrigin
	IncludeSections   []HarnessPromptSection
	EnableAugmenters  []HarnessAugmenter
	ReentryMode       ReentryMode
	DetachedRunMode   DetachedRunMode
	DiagnosticLabel   string
	ObservabilityTags map[string]string
}

// ResolvedHarnessContext captures the normalized inputs plus the derived policy.
type ResolvedHarnessContext struct {
	Surface ResolutionSurface
	Session HarnessSessionContext
	Turn    HarnessTurnContext
	Policy  ResolvedHarnessPolicy
}

// HarnessContextResolver derives harness policy from durable session state plus
// turn-origin metadata.
type HarnessContextResolver struct {
	runtime HarnessRuntimeSignals
}

// NewHarnessContextResolver constructs a daemon-owned harness context resolver.
func NewHarnessContextResolver(runtime HarnessRuntimeSignals) *HarnessContextResolver {
	return &HarnessContextResolver{runtime: runtime}
}

// ResolveStartup resolves startup policy from durable session context.
func (r *HarnessContextResolver) ResolveStartup(startup session.StartupPromptContext) (ResolvedHarnessContext, error) {
	return r.Resolve(HarnessResolutionInput{
		Surface: ResolutionSurfaceStartup,
		Session: HarnessSessionInput{
			Type:                 startup.SessionType,
			NetworkParticipation: startup.NetworkParticipation,
			WorkspaceID:          startup.WorkspaceID,
			Workspace:            startup.Workspace,
			AgentName:            startup.AgentName,
		},
		Turn: HarnessTurnRequest{
			Source: session.TurnSourceUser,
		},
	})
}

// ResolvePrompt resolves one live prompt-turn policy from session metadata.
func (r *HarnessContextResolver) ResolvePrompt(
	info *session.Info,
	source session.TurnSource,
	meta acp.PromptMeta,
) (ResolvedHarnessContext, error) {
	if info == nil {
		return ResolvedHarnessContext{}, errors.New("daemon: harness resolve prompt requires session info")
	}
	return r.Resolve(HarnessResolutionInput{
		Surface: ResolutionSurfaceTurn,
		Session: HarnessSessionInput{
			Type:                 info.Type,
			NetworkParticipation: info.NetworkParticipation,
			WorkspaceID:          info.WorkspaceID,
			Workspace:            info.Workspace,
			AgentName:            info.AgentName,
		},
		Turn: HarnessTurnRequest{
			Source:     source,
			PromptMeta: meta,
		},
	})
}

// Resolve derives one authoritative harness context from explicit inputs.
func (r *HarnessContextResolver) Resolve(input HarnessResolutionInput) (ResolvedHarnessContext, error) {
	if r == nil {
		return ResolvedHarnessContext{}, errors.New("daemon: harness context resolver is required")
	}

	surface := normalizeResolutionSurface(input.Surface)
	if surface == "" {
		return ResolvedHarnessContext{}, fmt.Errorf("daemon: invalid harness resolution surface %q", input.Surface)
	}

	sessionCtx, err := normalizeHarnessSessionContext(input.Session)
	if err != nil {
		return ResolvedHarnessContext{}, err
	}

	turnCtx, err := r.normalizeHarnessTurnContext(sessionCtx, input.Turn)
	if err != nil {
		return ResolvedHarnessContext{}, err
	}

	policy := ResolvedHarnessPolicy{
		SessionClass:     sessionCtx.SessionClass,
		TurnOrigin:       turnCtx.Origin,
		IncludeSections:  r.resolveSections(sessionCtx),
		EnableAugmenters: r.resolveAugmenters(surface, turnCtx),
		ReentryMode:      r.resolveReentry(turnCtx),
		DetachedRunMode:  r.resolveDetachedRunMode(sessionCtx, turnCtx),
	}
	policy.DiagnosticLabel = buildHarnessDiagnosticLabel(sessionCtx, policy)
	policy.ObservabilityTags = buildHarnessObservabilityTags(surface, sessionCtx, turnCtx, policy)

	return ResolvedHarnessContext{
		Surface: surface,
		Session: sessionCtx,
		Turn:    turnCtx,
		Policy:  policy,
	}, nil
}

func normalizeResolutionSurface(surface ResolutionSurface) ResolutionSurface {
	switch ResolutionSurface(strings.TrimSpace(string(surface))) {
	case ResolutionSurfaceStartup:
		return ResolutionSurfaceStartup
	case "", ResolutionSurfaceTurn:
		return ResolutionSurfaceTurn
	default:
		return ""
	}
}

func normalizeHarnessSessionContext(input HarnessSessionInput) (HarnessSessionContext, error) {
	sessionType := normalizeHarnessSessionType(input.Type)
	if sessionType == "" {
		return HarnessSessionContext{}, fmt.Errorf("daemon: invalid harness session type %q", input.Type)
	}

	sessionClass, err := harnessSessionClassForType(sessionType)
	if err != nil {
		return HarnessSessionContext{}, err
	}

	networkParticipation := input.NetworkParticipation
	if networkParticipation == (participation.Spec{}) {
		networkParticipation = participation.LocalSpec()
	}
	if err := participation.ValidateSpec(networkParticipation); err != nil {
		return HarnessSessionContext{}, fmt.Errorf("daemon: validate harness network participation: %w", err)
	}
	return HarnessSessionContext{
		Type:                 sessionType,
		SessionClass:         sessionClass,
		NetworkParticipation: networkParticipation,
		NetworkLive:          networkParticipation.Mode == participation.ModeLive,
		WorkspaceID:          strings.TrimSpace(input.WorkspaceID),
		Workspace:            strings.TrimSpace(input.Workspace),
		AgentName:            strings.TrimSpace(input.AgentName),
	}, nil
}

func normalizeHarnessSessionType(sessionType session.Type) session.Type {
	switch session.Type(strings.TrimSpace(string(sessionType))) {
	case session.SessionTypeUser:
		return session.SessionTypeUser
	case session.SessionTypeDream:
		return session.SessionTypeDream
	case session.SessionTypeSystem:
		return session.SessionTypeSystem
	case session.SessionTypeCoordinator:
		return session.SessionTypeCoordinator
	case session.SessionTypeSpawned:
		return session.SessionTypeSpawned
	default:
		return ""
	}
}

func harnessSessionClassForType(sessionType session.Type) (SessionClass, error) {
	switch sessionType {
	case session.SessionTypeUser:
		return SessionClassInteractive, nil
	case session.SessionTypeDream:
		return SessionClassDream, nil
	case session.SessionTypeSystem:
		return SessionClassSystem, nil
	case session.SessionTypeCoordinator:
		return SessionClassCoordinator, nil
	case session.SessionTypeSpawned:
		return SessionClassSpawned, nil
	default:
		return "", fmt.Errorf("daemon: unsupported harness session type %q", sessionType)
	}
}
