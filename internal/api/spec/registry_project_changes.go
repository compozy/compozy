package spec

import "github.com/compozy/compozy/internal/api/contract"

func registryProjectChangeOperations() []OperationSpec {
	operations := []OperationSpec{}
	for _, item := range []struct {
		id, suffix string
		body       any
	}{
		{"getSessionChanges", "", contract.SessionChangesResponse{}},
		{"getSessionChangePatches", "/patches", contract.SessionChangePatchesResponse{}},
		{"getSessionChangeContents", "/contents", contract.SessionChangeContentsResponse{}},
		{"listSessionChangeBases", "/bases", contract.SessionChangeBasesResponse{}},
	} {
		params := []ParameterSpec{pathParam("workspace_id", "Workspace id"), pathParam("session_id", "Session id")}
		if item.suffix == "/bases" {
			params = append(
				params,
				queryParam("q", "Substring ref filter", false),
				intQueryParam("limit", "Default 200, maximum 500"),
			)
		} else {
			params = append(
				params,
				enumQueryParam("scope", "Changes scope", []string{"branch", "uncommitted"}),
				queryParam("base", "Optional base ref", false),
			)
			if item.suffix != "" {
				path := queryParam("path", "Root-relative path", true)
				old := queryParam("old_path", "Aligned old path for renames", false)
				if item.suffix == "/patches" {
					path.Kind = specArrayKind
					old.Kind = specArrayKind
				}
				params = append(
					params,
					queryParam("revision", "Summary revision fence", true),
					path,
					old,
					boolQueryParam("reveal", "Reveal sensitive contents; forbidden to app tokens"),
				)
			}
		}
		responses := []ResponseSpec{{Status: 200, Description: "OK", Body: item.body}}
		for _, status := range []int{400, 403, 404, 409, 413, 422, 500} {
			responses = append(
				responses,
				ResponseSpec{Status: status, Description: "Changes operation failed", Body: contract.ErrorPayload{}},
			)
		}
		operations = append(
			operations,
			OperationSpec{
				Method:      httpMethodGet,
				Path:        "/api/workspaces/{workspace_id}/sessions/{session_id}/changes" + item.suffix,
				OperationID: item.id,
				Summary:     "Read session changes",
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
