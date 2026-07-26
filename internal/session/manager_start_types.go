package session

import (
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type sessionStartSpec struct {
	sessionID                string
	sandboxID                string
	sandbox                  *store.SessionSandboxMeta
	sessionName              string
	agentName                string
	provider                 string
	model                    string
	reasoningEffort          string
	permissions              aghconfig.PermissionMode
	sandboxDisabled          bool
	workspace                workspacepkg.ResolvedWorkspace
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
	creationProfile          *store.SessionCreationProfile
	creationOptions          *store.SessionCreationOptions
	creationIdentity         *store.SessionCreationIdentity
	creationIdentityPinned   bool
	creationIdentityEnabled  bool
	advertisedCommands       []store.SessionAdvertisedCommand
	postEvent                hookspkg.HookEvent
	startAction              string
	cleanupSessionDir        bool
	includePromptUpdatedAt   bool
	preserveStopReason       bool
	resumeReplay             bool
	resumeReplayBlock        string
	resumeReplayMessageCount int
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
