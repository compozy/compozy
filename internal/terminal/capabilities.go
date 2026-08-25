package terminal

const WorkspaceKindLocal = "local"

// ResolveCapabilities reports what one platform/workspace pairing can run.
func ResolveCapabilities(goos, workspaceKind string) Capabilities {
	return Capabilities{Interactive: goos != "windows" && (workspaceKind == "" || workspaceKind == WorkspaceKindLocal)}
}

// RecordingAvailable derives recording support from interactive availability.
func RecordingAvailable(capabilities Capabilities) bool { return capabilities.Interactive }
