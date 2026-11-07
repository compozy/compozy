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
