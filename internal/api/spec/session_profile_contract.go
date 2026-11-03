package spec

import "strings"

const workspaceSessionPathPrefix = "/api/workspaces/{workspace_id}/sessions/{session_id}"

// applySessionProfileContract documents the profile selector on every route that names one
// session (#1193). The session's profile is authoritative: a selected profile must own it, reads
// may use the all-profiles lens, and writes name one profile.
func applySessionProfileContract(operations []OperationSpec) []OperationSpec {
	for index := range operations {
		path := operations[index].Path
		if path != "/api/sessions/{session_id}/owner" && path != workspaceSessionPathPrefix &&
			!strings.HasPrefix(path, workspaceSessionPathPrefix+"/") {
			continue
		}
		operations[index].Parameters = ensureProfileParameters(
			operations[index].Parameters,
			operations[index].Method == httpMethodGet,
		)
	}
	return operations
}
