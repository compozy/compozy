package spec

import "github.com/compozy/compozy/internal/api/contract"

func registryProjectSearchOperations() []OperationSpec {
	operations := []OperationSpec{}
	for _, session := range []bool{false, true} {
		for _, search := range []bool{false, true} {
			route := "/api/workspaces/{workspace_id}"
			params := []ParameterSpec{pathParam("workspace_id", "Workspace id")}
			id := "findFiles"
			if session {
				route += "/sessions/{session_id}"
				params = append(params, pathParam("session_id", "Session id"))
				id = "findSessionFiles"
			} else {
				params = append(params, queryParam("worktree_id", "Optional worktree id", false))
			}
			params = append(params, queryParam("q", "Search query", true))
			route += "/files/"
			responses := []ResponseSpec{{Status: 200, Description: "OK", Body: contract.FileFindResponse{}}}
			summary := "Find project files"
			if search {
				route += "search"
				id = "searchFileContents"
				if session {
					id = "searchSessionFileContents"
				}
				summary = "Search project file contents"
				responses[0].Body = contract.FileSearchResponse{}
				params = append(
					params,
					boolQueryParam("regex", "Use POSIX ERE"),
					boolQueryParam("case_sensitive", "Match case"),
					boolQueryParam("whole_word", "Match whole words"),
				)
				for _, name := range []string{specIncludeKey, specExcludeKey} {
					param := queryParam(name, "Root-relative globs; repeat at most ten times", false)
					param.Kind = specArrayKind
					params = append(params, param)
				}
			} else {
				route += "find"
				params = append(params, intQueryParam("limit", "Result limit; defaults and clamps to 50"))
			}
			for _, status := range []int{400, 403, 404, 409, 500} {
				responses = append(
					responses,
					ResponseSpec{Status: status, Description: "Search failed", Body: contract.ErrorPayload{}},
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
