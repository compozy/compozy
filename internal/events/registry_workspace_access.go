package events

const (
	WorkspaceAccessDenied  = "workspace.access_denied"
	WorkspaceAccessGranted = "workspace.access_granted"
)

var workspaceAccessRegistryEntries = []Metadata{
	global(notify(warning(WorkspaceAccessDenied, "workspace.access", ComponentWorkspace))),
	global(success(WorkspaceAccessGranted, "workspace.access", ComponentWorkspace)),
}
