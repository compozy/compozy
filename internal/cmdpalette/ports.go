package cmdpalette

import "context"

type Provider interface {
	ProvideCommands(context.Context, CatalogRequest) ([]Descriptor, error)
}

// ContributionProvider returns a complete multi-source extension projection.
type ContributionProvider interface {
	Provider
	ProvideContribution(context.Context, CatalogRequest) (Contribution, error)
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

// ClientDirectory answers the shells attached to one profile lens. Clients are
// registered per profile, so a scoped lens reaches only that profile's clients;
// only the explicit All profiles aggregate widens the read.
type ClientDirectory interface {
	AttachedClients(context.Context, ProfileLens, WorkspaceID) ([]Client, error)
	Clients(context.Context, ProfileLens, WorkspaceID) ([]Client, error)
	Context(context.Context, ProfileLens, WorkspaceID, ClientID) (ContextSnapshot, error)
	Authorize(context.Context, ProfileLens, WorkspaceID, ClientID, string) error
}

type BindingsResolver interface {
	Bindings(context.Context, ProfileLens, WorkspaceID) (map[CommandID][]string, map[CommandID]string, error)
}

// SnapshotBindingsResolver resolves bindings from the exact contribution snapshot used by Catalog.
type SnapshotBindingsResolver interface {
	BindingsForCatalogSnapshot(
		context.Context,
		ProfileLens,
		WorkspaceID,
		[]CommandID,
		[]ExtensionDefaultShortcut,
	) (map[CommandID][]string, map[CommandID]string, error)
}

// SnapshotGlobalBindingsResolver resolves intended global bindings from the catalog snapshot.
type SnapshotGlobalBindingsResolver interface {
	GlobalBindingsForCatalogSnapshot(
		context.Context,
		ProfileLens,
		WorkspaceID,
		[]CommandID,
	) (map[CommandID]string, error)
}

// GlobalShortcutStatusDirectory resolves ephemeral registration state for one shell client.
type GlobalShortcutStatusDirectory interface {
	GlobalShortcutStatuses(
		context.Context,
		ProfileLens,
		WorkspaceID,
		ClientID,
	) (map[CommandID]GlobalShortcut, error)
}

type Registry interface {
	Catalog(context.Context, CatalogRequest) (Catalog, error)
	Clients(context.Context, ProfileLens, WorkspaceID) ([]Client, error)
	Invoke(context.Context, InvokeRequest) (InvokeResult, error)
	RecordUsage(context.Context, Usage) error
	Personalization(context.Context, ProfileLens, WorkspaceID) (Snapshot, error)
	PersonalizationSummary(context.Context, ProfileLens, WorkspaceID) (PersonalizationSummary, error)
	ResetPersonalization(context.Context, ProfileLens, WorkspaceID) error
	Pin(context.Context, ProfileLens, WorkspaceID, CommandID) error
	Unpin(context.Context, ProfileLens, WorkspaceID, CommandID) error
}

type BindableCatalog interface {
	BindableIDs(context.Context, ProfileLens, WorkspaceID) ([]CommandID, error)
}

// ExtensionDefaultCatalog exposes ordered extension shortcut claims without resolving bindings.
type ExtensionDefaultCatalog interface {
	ExtensionDefaults(context.Context, ProfileLens, WorkspaceID) ([]ExtensionDefaultShortcut, error)
}
