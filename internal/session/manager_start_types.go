package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/soul"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type sessionStartSpec struct {
	sessionID                string
	profileID                string
	sandboxID                string
	sandbox                  *store.SessionSandboxMeta
	sessionName              string
	agentName                string
	provider                 string
	model                    string
	transportModel           string
	reasoningEffort          string
	speed                    speedpkg.Speed
	acpOptions               []acp.SessionConfigOptionSelection
	selectedRuntime          *RuntimeSelection
	runtimeSelectionRevision int64
	permissions              compozyconfig.PermissionMode
	sandboxDisabled          bool
	workspace                workspacepkg.ResolvedWorkspace
	worktreeID               string
	worktreeRoot             string
	cwd                      string
	networkParticipation     participation.Spec
	networkOwnerKey          string
	participationObservation *participation.ResolvedObservation
	promptOverlay            string
	contractOverlay          string
	runtimeMode              string
	sessionType              Type
	lineage                  *store.SessionLineage
	allowedToolsOverride     []string
	deniedToolsOverride      []string
	creationProfile          *store.SessionCreationProfile
	creationOptions          *store.SessionCreationOptions
	creationIdentity         *store.SessionCreationIdentity
	creationIdentityPinned   bool
	creationIdentityEnabled  bool
	discardStartFailure      bool
	advertisedCommands       []store.SessionAdvertisedCommand
	postEvent                hookspkg.HookEvent
	startAction              string
	cleanupSessionDir        bool
	includePromptUpdatedAt   bool
	preserveStopReason       bool
	resumeReplay             bool
	resumeReplayBlock        string
	resumeReplayMessageCount int
	resumeReplayReason       string
	clearEventStoreOnOpen    bool
	createdAt                time.Time
	acpSessionID             string
	stopReason               store.StopReason
	stopDetail               string
	failure                  *store.SessionFailure
	soulSnapshotID           string
	soulDigest               string
	parentSoulDigest         string
	soulSnapshot             *soul.Snapshot
}

func normalizeRequestedSpeed(requested speedpkg.Speed) (speedpkg.Speed, error) {
	if requested == "" {
		requested = speedpkg.SpeedNormal
	}
	return speedpkg.Parse(string(requested))
}

func normalizeCreateRuntimeOptions(
	opts CreateOpts,
) (speedpkg.Speed, []acp.SessionConfigOptionSelection, error) {
	requestedSpeed := speedpkg.Speed(strings.TrimSpace(string(opts.Speed)))
	if requestedSpeed != "" {
		parsed, err := speedpkg.Parse(string(requestedSpeed))
		if err != nil {
			return "", nil, fmt.Errorf("%w: %w", ErrInvalidRuntimeOverride, err)
		}
		requestedSpeed = parsed
	}
	acpOptions, err := acp.NormalizeSessionConfigOptionSelections(opts.ACPOptions)
	if err != nil {
		return "", nil, fmt.Errorf("%w: %w", ErrInvalidRuntimeOverride, err)
	}
	return requestedSpeed, acpOptions, nil
}
