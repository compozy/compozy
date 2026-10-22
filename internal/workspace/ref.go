package workspace

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"
)

type RefKind string

const (
	RefWorkspace RefKind = "workspace"
	RefUser      RefKind = "user"
)

type ScopeKind string

const (
	ScopeWorkspace ScopeKind = "workspace"
	ScopeUser      ScopeKind = "user"
)

type Ref struct {
	Kind  RefKind
	Value string
}

func ParseRef(raw string) Ref {
	value := strings.TrimSpace(raw)
	if value == workspaceaccess.UserSelector {
		return Ref{Kind: RefUser}
	}
	return Ref{Kind: RefWorkspace, Value: value}
}

var (
	ErrWorkspaceReserved            = errors.New("workspace_reserved")
	ErrWorkspaceNameReserved        = errors.New("workspace_name_reserved")
	ErrUserScopeUnsupported         = errors.New("user_scope_unsupported")
	ErrUserScopeWorktreeUnsupported = errors.New("user_scope_worktree_unsupported")
	ErrUserScopeWorkDirInvalid      = errors.New("user_scope_work_dir_invalid")
)

func ValidateWorkspaceName(name string) error {
	name = strings.TrimSpace(name)
	if name == "user" || name == "global" || strings.HasPrefix(name, "@") {
		return ErrWorkspaceNameReserved
	}
	return nil
}
