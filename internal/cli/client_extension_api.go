package cli

import "context"

type extensionClientAPI interface {
	ListExtensions(context.Context) ([]ExtensionRecord, error)
	ListExtensionsScoped(context.Context, string) ([]ExtensionRecord, error)
	SearchExtensions(context.Context, ExtensionSearchRequest) (ExtensionSearchRecord, error)
	ListExtensionCommands(context.Context, string, string) (ExtensionCommandsRecord, error)
	PreviewExtensionInstall(context.Context, InstallExtensionRequest) (ExtensionInstallPreviewRecord, error)
	InstallExtension(context.Context, InstallExtensionRequest) (ExtensionRecord, error)
	UpdateExtension(context.Context, string, UpdateExtensionRequest) (ExtensionUpdateRecord, error)
	UpdateExtensions(context.Context, UpdateExtensionsRequest) ([]ExtensionUpdateRecord, error)
	RemoveExtension(context.Context, string) (ManagedExtensionRemoveRecord, error)
	EnableExtension(context.Context, string, EnableExtensionRequest) (ExtensionEnableRecord, error)
	DisableExtension(context.Context, string) (ExtensionRecord, error)
	ExtensionStatus(context.Context, string) (ExtensionRecord, error)
	ExtensionStatusScoped(context.Context, string, string) (ExtensionRecord, error)
	ExtensionProvenance(context.Context, string) (ExtensionProvenanceRecord, error)
	ExtensionInventory(context.Context, string) (ExtensionInventoryRecord, error)
	PreviewExtensionEnable(context.Context, string) (ExtensionEnablePreviewRecord, error)
}
