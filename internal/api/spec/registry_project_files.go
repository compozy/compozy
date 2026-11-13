package spec

import "github.com/compozy/compozy/internal/api/contract"

func registryProjectFileOperations() []OperationSpec {
	operations := []OperationSpec{}
	for _, session := range []bool{false, true} {
		for _, content := range []bool{false, true} {
			route := "/api/workspaces/{workspace_id}"
			id := "listFileEntries"
			summary := "List project file entries"
			params := []ParameterSpec{pathParam("workspace_id", "Workspace id")}
			if session {
				route += "/sessions/{session_id}"
				params = append(params, pathParam("session_id", "Session id"))
				id = "listSessionFileEntries"
			} else {
				params = append(params, queryParam("worktree_id", "Optional worktree id", false))
			}
			params = append(params, queryParam("path", "Root-relative path", content))
			responses := []ResponseSpec{{Status: 200, Description: "OK", Body: contract.FileEntriesResponse{}}}
			if content {
				route += "/files/content"
				summary = "Read project file content"
				id = "readFileContent"
				if session {
					id = "readSessionFileContent"
				}
				params = append(params, boolQueryParam("reveal", "Reveal sensitive content; forbidden to app tokens"))
				responses[0].Body = contract.FileContentResponse{}
			} else {
				route += "/files/entries"
				ignored := boolQueryParam("show_ignored", "Include ignored entries")
				ignored.Default = true
				params = append(params, ignored)
			}
			for _, status := range []int{400, 403, 404, 409, 422, 500} {
				responses = append(
					responses,
					ResponseSpec{Status: status, Description: "File operation failed", Body: contract.ErrorPayload{}},
				)
			}
			operations = append(
				operations,
				OperationSpec{
					Method:      httpMethodGet,
					Path:        route,
					OperationID: id,
					Summary:     summary,
					AppAccess:   AppAccessAllowed,
					Tags:        []string{specWorkspacesKey},
					Transports:  []Transport{TransportHTTP, TransportUDS},
					Parameters:  withProfileScope(params...),
					Responses:   responses,
				},
			)
		}
	}
	return operations
}

// registryProjectFileBytesOperations documents raw file streams. The file path is a trailing
// path suffix, so relative assets of a previewed document resolve inside the same root.
func registryProjectFileBytesOperations() []OperationSpec {
	const workspace = "/api/workspaces/{workspace_id}"
	forms := []struct {
		route, id, summary string
		scope              []ParameterSpec
	}{
		{workspace, "readFileBytes", "Stream project file bytes", nil},
		{
			workspace + "/worktrees/{worktree_id}", "readWorktreeFileBytes", "Stream worktree file bytes",
			[]ParameterSpec{pathParam("worktree_id", "Worktree id")},
		},
		{
			workspace + "/sessions/{session_id}", "readSessionFileBytes", "Stream session file bytes",
			[]ParameterSpec{pathParam("session_id", "Session id")},
		},
	}
	operations := make([]OperationSpec, 0, len(forms))
	for _, form := range forms {
		params := append([]ParameterSpec{pathParam("workspace_id", "Workspace id")}, form.scope...)
		params = append(params, pathParam("path", "Root-relative file path; its slashes are path segments"))
		responses := []ResponseSpec{
			{Status: 200, Description: "File bytes", Body: binaryResponse{}, ContentType: "*/*"},
			{Status: 206, Description: "Requested byte range", Body: binaryResponse{}, ContentType: "*/*"},
			{Status: 304, Description: "File unchanged since the supplied validator"},
		}
		for _, status := range []int{400, 403, 404, 409, 422, 500} {
			responses = append(
				responses,
				ResponseSpec{Status: status, Description: "File operation failed", Body: contract.ErrorPayload{}},
			)
		}
		responses = append(responses, ResponseSpec{Status: 416, Description: "Requested range not satisfiable"})
		operations = append(
			operations,
			OperationSpec{
				Method:      httpMethodGet,
				Path:        form.route + "/files/raw/{path}",
				OperationID: form.id,
				Summary:     form.summary,
				AppAccess:   AppAccessAllowed,
				Tags:        []string{specWorkspacesKey},
				Transports:  []Transport{TransportHTTP, TransportUDS},
				Parameters:  withProfileScope(params...),
				Responses:   responses,
			},
		)
	}
	return operations
}
