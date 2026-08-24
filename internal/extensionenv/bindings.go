// Package extensionenv defines the persistence boundary for extension environment bindings.
package extensionenv

import (
	"context"
	"time"
)

// BindingKind identifies Vault entries owned by extension environment bindings.
const BindingKind = "extension_env"

// Binding is one extension-instance environment binding.
type Binding struct {
	ExtensionName string
	// ProfileID is empty only for the user layer.
	ProfileID   string
	WorkspaceID string
	EnvName     string
	SecretRef   string
	MCPServer   string
	HeaderName  string
	Kind        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Store persists extension-instance environment bindings. An empty profileID
// selects the user layer. List and delete methods address one exact
// (profileID, workspaceID) cell. ResolveEnvBindings merges effective values in
// profile-workspace, profile-user, user-workspace, then user-user precedence.
type Store interface {
	ListEnvBindings(ctx context.Context, extension, profileID, workspaceID string) ([]Binding, error)
	ResolveEnvBindings(ctx context.Context, extension, profileID, workspaceID string) ([]Binding, error)
	PutEnvBinding(ctx context.Context, binding Binding) error
	DeleteEnvBinding(ctx context.Context, extension, profileID, workspaceID, envName string) error
}

// LifecycleStore adds cleanup and owned-reference usage checks.
type LifecycleStore interface {
	Store
	DeleteEnvBindings(ctx context.Context, extension, profileID, workspaceID string) error
	CountEnvBindingsBySecretRef(ctx context.Context, secretRef string) (int64, error)
}
