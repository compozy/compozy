package cmdpalette

import "context"

type Provider interface {
	ProvideCommands(context.Context, WorkspaceID) ([]Descriptor, error)
}

// ContributionProvider returns a complete multi-source extension projection.
type ContributionProvider interface {
	Provider
	ProvideContribution(context.Context, WorkspaceID) (Contribution, error)
}

type StaticProvider interface {
	Provider
	StaticCommands() []Descriptor
}

type ProviderRegistration struct {
	Source   Source
	Provider Provider
}

type ActionExecutor interface {
	ExecuteAction(context.Context, ExecutionRequest) (ExecutionResult, error)
}

type ApprovalPreflight interface {
	ApprovalRequired(context.Context, ExecutionRequest) (bool, error)
}

type ClientDirectory interface {
	Clients(context.Context, WorkspaceID) ([]Client, error)
	Context(context.Context, WorkspaceID, ClientID) (ContextSnapshot, error)
	Authorize(context.Context, WorkspaceID, ClientID, string) error
}

type BindingsResolver interface {
	Bindings(context.Context, WorkspaceID) (map[CommandID][]string, map[CommandID]string, error)
}

// SnapshotBindingsResolver resolves bindings from the exact contribution snapshot used by Catalog.
type SnapshotBindingsResolver interface {
	BindingsForCatalogSnapshot(
		context.Context,
		WorkspaceID,
		[]CommandID,
		[]ExtensionDefaultShortcut,
	) (map[CommandID][]string, map[CommandID]string, error)
}

// SnapshotGlobalBindingsResolver resolves intended global bindings from the catalog snapshot.
type SnapshotGlobalBindingsResolver interface {
	GlobalBindingsForCatalogSnapshot(
		context.Context,
		WorkspaceID,
		[]CommandID,
	) (map[CommandID]string, error)
}

// GlobalShortcutStatusDirectory resolves ephemeral registration state for one shell client.
type GlobalShortcutStatusDirectory interface {
	GlobalShortcutStatuses(
		context.Context,
		WorkspaceID,
		ClientID,
	) (map[CommandID]GlobalShortcut, error)
}

type Registry interface {
	Catalog(context.Context, WorkspaceID, ClientID) (Catalog, error)
	Clients(context.Context, WorkspaceID) ([]Client, error)
	Invoke(context.Context, InvokeRequest) (InvokeResult, error)
	RecordUsage(context.Context, Usage) error
	Personalization(context.Context, WorkspaceID) (Snapshot, error)
	PersonalizationSummary(context.Context, WorkspaceID) (PersonalizationSummary, error)
	ResetPersonalization(context.Context, WorkspaceID) error
	Pin(context.Context, WorkspaceID, CommandID) error
	Unpin(context.Context, WorkspaceID, CommandID) error
}

type BindableCatalog interface {
	BindableIDs(context.Context, WorkspaceID) ([]CommandID, error)
}

// ExtensionDefaultCatalog exposes ordered extension shortcut claims without resolving bindings.
type ExtensionDefaultCatalog interface {
	ExtensionDefaults(context.Context, WorkspaceID) ([]ExtensionDefaultShortcut, error)
}
