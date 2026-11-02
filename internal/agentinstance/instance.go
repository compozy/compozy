package agentinstance

import (
	"errors"
	"time"
)

var (
	ErrWorkspaceAccessDenied = errors.New("workspace_access_denied")
	ErrSessionNotRoot        = errors.New("session_not_root")
	ErrPaused                = errors.New("agent_instance_paused")
	ErrLimit                 = errors.New("agent_instance_limit")
	ErrExists                = errors.New("agent_instance_exists")
	ErrStarting              = errors.New("agent_instance_starting")
	ErrForbidden             = errors.New("agent_instance_forbidden")
	ErrDepth                 = errors.New("agent_instance_depth")
	ErrNotStaysOn            = errors.New("agent_not_stays_on")
	ErrDefinitionMissing     = errors.New("agent_definition_missing")
	ErrNotFound              = errors.New("agent_instance_not_found")
	ErrScopeMismatch         = errors.New("session_scope_mismatch")
)

type Instance struct {
	ID                string
	ProfileID         string
	AgentName         string
	WorkspaceID       string
	State             string
	PauseReason       string
	MainSessionID     string
	GrantCeiling      string
	PermissionCeiling string
	ChecksEnabled     bool
	Depth             int
	CreatedByKind     string
	CreatedByID       string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ArchivedAt        *time.Time
}

type Occurrence struct {
	State, Result, Reason, ReportID string
}
