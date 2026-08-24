package extensionpkg

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

var (
	// ErrExtensionNotDevLinked reports that no workspace dev overlay exists.
	ErrExtensionNotDevLinked = errors.New("extension: extension is not dev linked")
	// ErrExtensionGenerationInvalid reports that an immutable generation handle failed validation.
	ErrExtensionGenerationInvalid = errors.New("extension: generation is invalid")
	// ErrExtensionWorkspaceDenied reports an attempted cross-workspace instance access.
	ErrExtensionWorkspaceDenied = errors.New("extension: workspace access denied")
	// ErrExtensionDevOriginMissing reports a persisted dev origin that is no longer present.
	ErrExtensionDevOriginMissing = errors.New("extension: development origin is missing")
)

// InstanceKey is the stable identity for one runtime extension instance.
// An empty WorkspaceID identifies the globally installed instance.
type InstanceKey struct {
	Name        string `json:"name"`
	ProfileID   string `json:"profile_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// Normalize trims one instance key without changing its scope.
func (k InstanceKey) Normalize() InstanceKey {
	return InstanceKey{
		Name:        strings.TrimSpace(k.Name),
		ProfileID:   strings.TrimSpace(k.ProfileID),
		WorkspaceID: strings.TrimSpace(k.WorkspaceID),
	}
}

// Validate reports whether the instance identity is usable.
func (k InstanceKey) Validate() error {
	normalized := k.Normalize()
	if normalized.Name == "" {
		return errors.New("extension: extension name is required")
	}
	return nil
}

// IsGlobal reports whether the key identifies the published global instance.
func (k InstanceKey) IsGlobal() bool {
	return strings.TrimSpace(k.WorkspaceID) == ""
}

// IsProfileScoped reports whether the runtime is isolated to one profile.
func (k InstanceKey) IsProfileScoped() bool {
	return strings.TrimSpace(k.ProfileID) != ""
}

// GlobalInstanceKey returns the published instance identity for name.
func GlobalInstanceKey(name string) InstanceKey {
	return InstanceKey{Name: strings.TrimSpace(name)}
}

// ProfileInstanceKey returns the runtime identity for one acting profile and
// optional workspace. The permanent default profile reuses the base process.
func ProfileInstanceKey(name, profileID, workspaceID string) InstanceKey {
	key := InstanceKey{
		Name:        strings.TrimSpace(name),
		ProfileID:   strings.TrimSpace(profileID),
		WorkspaceID: strings.TrimSpace(workspaceID),
	}
	if key.ProfileID == "" || key.ProfileID == store.DefaultProfileID {
		key.ProfileID = ""
	}
	return key
}

func (k InstanceKey) runtimeID() string {
	normalized := k.Normalize()
	id := normalized.Name
	if normalized.ProfileID != "" {
		id += "@profile:" + normalized.ProfileID
	}
	if normalized.WorkspaceID != "" {
		id += fmt.Sprintf("@workspace:%s", normalized.WorkspaceID)
	}
	return id
}
