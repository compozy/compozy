// Package workspace defines workspace domain models, sentinel errors, and
// resolver contracts used across AGH runtime packages.
package workspace

import (
	"context"
	"errors"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/sandbox"
)

var (
	// ErrWorkspaceNotFound reports that no registered workspace matched the lookup.
	ErrWorkspaceNotFound = errors.New("workspace not found")
	// ErrWorkspaceRootMissing reports that the registered workspace root no longer exists on disk.
	ErrWorkspaceRootMissing = errors.New("workspace root directory no longer exists")
	// ErrAgentNotAvailable reports that the requested agent cannot be resolved in the workspace.
	ErrAgentNotAvailable = errors.New("agent not available in workspace")
	// ErrWorkspaceResolverUnavailable reports that workspace resolution cannot run because its dependency is absent.
	ErrWorkspaceResolverUnavailable = errors.New("workspace resolver unavailable")
	// ErrWorkspaceNameTaken reports that a workspace name is already registered.
	ErrWorkspaceNameTaken = errors.New("workspace name already in use")
	// ErrWorkspacePathTaken reports that a workspace root path is already registered.
	ErrWorkspacePathTaken = errors.New("workspace path already registered")
	// ErrWorkspaceHasSessions reports that a workspace cannot be deleted because sessions still reference it.
	ErrWorkspaceHasSessions = errors.New("workspace has sessions")
	// ErrWorkspaceHasActiveSessions reports that a workspace cannot be deleted because active sessions are running.
	ErrWorkspaceHasActiveSessions = errors.New("workspace has active sessions")
	// ErrWorkspaceIdentityInvalid reports a malformed .agh/workspace.toml identity file.
	ErrWorkspaceIdentityInvalid = errors.New("workspace identity invalid")
	// ErrWorkspaceIdentityPermissionDenied reports a fail-closed identity file permission failure.
	ErrWorkspaceIdentityPermissionDenied = errors.New("workspace identity permission denied")
)

// Workspace is the persisted workspace registration stored in the global database.
type Workspace struct {
	ID             string
	RootDir        string
	AdditionalDirs []string
	Name           string
	DefaultAgent   string
	SandboxRef     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ResolvedWorkspace is the computed workspace snapshot returned by a resolver.
type ResolvedWorkspace struct {
	Workspace
	WorkspaceID      string
	Config           aghconfig.Config
	Agents           []aghconfig.AgentDef
	AgentDiagnostics []AgentDiagnostic
	Skills           []SkillPath
	Sandbox          sandbox.Resolved
	ResolvedAt       time.Time
}

// AgentDiagnostic reports one workspace-visible AGENT.md file that could not be loaded.
type AgentDiagnostic struct {
	Name      string
	Path      string
	ErrorKind string
	Message   string
}

// SkillPath identifies a discovered skill directory and its origin.
type SkillPath struct {
	Dir    string
	Source string
}

// RuntimeResolver resolves persisted workspaces into computed runtime snapshots.
type RuntimeResolver interface {
	Resolve(ctx context.Context, idOrPath string) (ResolvedWorkspace, error)
	ResolveOrRegister(ctx context.Context, path string) (ResolvedWorkspace, error)
}
