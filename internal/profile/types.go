// Package profile owns durable profile identity, selection, resolution, and
// crash-convergent lifecycle operations.
package profile

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type State string

const (
	StateActive   State = "active"
	StateArchived State = "archived"
)

type Profile struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Color      string     `json:"color"`
	Icon       string     `json:"icon,omitempty"`
	Emoji      string     `json:"emoji,omitempty"`
	State      State      `json:"state"`
	CreatedAt  time.Time  `json:"created_at"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

type CreateInput struct {
	Name, Color, Icon, Emoji string
	Activate                 *Lens
}

type DeclaredInput struct {
	Extension string
	Name      string
	Seed      DeclaredSeed
}

type IdentityPatch struct{ Color, Icon, Emoji *string }

type RenameOptions struct {
	NewName      string
	Repos        RepoChoice
	PlanRevision string
}

type RenamePlan struct {
	Revision          string
	MachineFolders    []string
	RepoCandidates    []RepoFolderRef
	DormantPlacements []PlacementRef
	VaultRefRewrites  int
}

type ArchivePlan struct {
	Revision           string
	RunningSessions    []string
	ApprovalBlockers   []string
	LeasedRuns         int
	QueuedRunsToFreeze int
	AutomationsToPause []string
}

type DeletePlan struct {
	Revision          string
	Removed           RemovalSummary
	SelectionsToSweep int
	ApprovalBlockers  []string
}

type RemovalSummary struct {
	Agents, Skills, Loops, MCPServers, ConfigKeys, CredentialOverrides int
	MemoryEntries, DesktopPartitions                                   int
	PaletteUsage, PaletteQueryHits, PalettePins, TerminalApprovals     int
}

type RenameResult struct {
	RepoResults       []RepoRenameOutcome
	DormantPlacements []PlacementRef
}

type ArchiveResult struct {
	PausedAutomations []string
	FrozenQueuedRuns  int
}

type UnarchiveResult struct {
	Profile           Profile
	PausedAutomations []string
}

type DeleteResult struct {
	Removed         RemovalSummary
	SweptSelections int
}

type DeclaredSeed struct {
	Color, Icon, Emoji string
	Defaults           PersonaDefaults
	CredentialAsks     []CredentialAsk
}

type PersonaDefaults struct{ Agent, Provider, Sandbox string }
type CredentialAsk struct{ Provider, Slot string }
type RepoChoice struct {
	All, None    bool
	WorkspaceIDs []string
}
type ProfileWithCounts struct {
	Profile
	WorkItems              int
	NeedsSetup             bool
	CredentialRequirements []CredentialRequirement
}
type CredentialRequirement struct {
	Provider, Slot, SourceExtension string
	Missing                         bool
}
type RepoFolderRef struct{ WorkspaceID, WorkspaceName, Path string }
type PlacementRef struct{ Extension, Resource, ProfileName string }
type RepoRenameOutcome struct {
	WorkspaceID string
	Renamed     bool
	Reason      string
}

type ResolveInput struct {
	Flag, Env        string
	SessionProfileID string
	Lens             Lens
}

type LifecycleOp struct{ ID, Kind, Profile, Status, Step, Error string }

type SelectionLens string

const (
	SelectionLensWorkspace SelectionLens = "workspace"
	SelectionLensGlobal    SelectionLens = "global"
)

type Lens struct {
	Kind        SelectionLens
	WorkspaceID string
}

type Selection struct {
	Lens        SelectionLens
	WorkspaceID string
	ProfileID   string
}

type SelectionStore interface {
	Get(context.Context, SelectionLens, string) (Selection, bool, error)
	List(context.Context) ([]Selection, error)
	Put(context.Context, Selection) error
	SweepProfile(context.Context, string) error
}

type ResolutionSource string

const (
	ResolutionSourceFlag       ResolutionSource = "flag"
	ResolutionSourceEnv        ResolutionSource = "env"
	ResolutionSourceRemembered ResolutionSource = "remembered"
	ResolutionSourceSession    ResolutionSource = "session"
	ResolutionSourceDefault    ResolutionSource = "default"
)

type ResolutionNote string

const (
	ResolutionNoteNone                       ResolutionNote = ""
	ResolutionNoteArchivedRememberedFallback ResolutionNote = "archived_remembered_fallback"
	ResolutionNoteNoRememberedChoice         ResolutionNote = "no_remembered_choice"
)

type Resolution struct {
	Profile Profile
	Source  ResolutionSource
	Note    ResolutionNote
}

type ReadScope = store.ReadScope
