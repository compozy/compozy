package tools

const (
	// ToolIDExtensionsList lists installed extensions through the extension registry.
	ToolIDExtensionsList ToolID = "compozy__extensions_list"
	// ToolIDExtensionsInfo reads one installed extension status.
	ToolIDExtensionsInfo ToolID = "compozy__extensions_info"
	// ToolIDExtensionsInventory reads shipped and live extension resources.
	ToolIDExtensionsInventory ToolID = "compozy__extensions_inventory"
	// ToolIDExtensionsPreview previews an extension enable without changing state.
	ToolIDExtensionsPreview ToolID = "compozy__extensions_preview"
	// ToolIDExtensionsInstall installs one extension through a managed local or marketplace source.
	ToolIDExtensionsInstall ToolID = "compozy__extensions_install"
	// ToolIDExtensionsUpdate updates one or more marketplace-installed extensions.
	ToolIDExtensionsUpdate ToolID = "compozy__extensions_update"
	// ToolIDExtensionsRemove removes one managed installed extension.
	ToolIDExtensionsRemove ToolID = "compozy__extensions_remove"
	// ToolIDExtensionsEnable enables one installed extension.
	ToolIDExtensionsEnable ToolID = "compozy__extensions_enable"
	// ToolIDExtensionsDisable disables one installed extension.
	ToolIDExtensionsDisable ToolID = "compozy__extensions_disable"
	// ToolIDExtensionsInit scaffolds one extension project from an embedded template.
	ToolIDExtensionsInit ToolID = "compozy__extensions_init"
	// ToolIDExtensionsBuild builds one immutable extension generation.
	ToolIDExtensionsBuild ToolID = "compozy__extensions_build"
	// ToolIDExtensionsValidate validates one extension bundle without executing it.
	ToolIDExtensionsValidate ToolID = "compozy__extensions_validate"
	// ToolIDExtensionsDev links one immutable generation to the trusted workspace.
	ToolIDExtensionsDev ToolID = "compozy__extensions_dev"
	// ToolIDExtensionsReload reloads one trusted workspace extension generation.
	ToolIDExtensionsReload ToolID = "compozy__extensions_reload"
	// ToolIDExtensionsLogs reads redacted logs for the trusted extension instance.
	ToolIDExtensionsLogs ToolID = "compozy__extensions_logs"
	// ToolIDExtensionsSearch discovers extensions across curated and GitHub sources.
	ToolIDExtensionsSearch ToolID = "compozy__extensions_search"
	// ToolIDExtensionsProvenance reads persisted install provenance.
	ToolIDExtensionsProvenance ToolID = "compozy__extensions_provenance"
	// ToolIDExtensionsPublish publishes an immutable generation to GitHub Releases.
	ToolIDExtensionsPublish ToolID = "compozy__extensions_publish"
)
