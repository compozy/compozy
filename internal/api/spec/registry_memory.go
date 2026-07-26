package spec

import (
	"github.com/compozy/agh/internal/api/contract"

	"github.com/getkin/kin-openapi/openapi3"
)

func registryMemoryOperations() []OperationSpec {
	return []OperationSpec{
		listMemoryOperationSpec(),
		getMemoryHealthOperationSpec(),
		getMemoryConfigMetadataOperationSpec(),
		listMemoryHistoryOperationSpec(),
		showMemoryScopeOperationSpec(),
		readMemoryOperationSpec(),
		writeMemoryOperationSpec(),
		editMemoryOperationSpec(),
		deleteMemoryOperationSpec(),
		searchMemoryOperationSpec(),
		reindexMemoryOperationSpec(),
		promoteMemoryOperationSpec(),
		resetMemoryOperationSpec(),
		reloadMemoryOperationSpec(),
		listMemoryDecisionsOperationSpec(),
		getMemoryDecisionOperationSpec(),
		revertMemoryDecisionOperationSpec(),
		getMemoryRecallTraceOperationSpec(),
	}
}
func listMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory",
		OperationID: "listMemory",
		Summary:     "List Memory v2 curated entries",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  memoryListQueryParams(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryListResponse{}},
			memoryError(400, "Invalid memory filter"),
			memoryError(404, "Workspace or memory not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func getMemoryHealthOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/health",
		OperationID: "getMemoryHealth",
		Summary:     "Get memory health",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  memorySelectorQueryParams(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryHealthPayload{}},
			memoryError(400, "Invalid memory health filter"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func getMemoryConfigMetadataOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/config",
		OperationID: "getMemoryConfigMetadata",
		Summary:     "Get Memory v2 config metadata and provider registry state",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryConfigMetadataResponse{}},
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func listMemoryHistoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/history",
		OperationID: "listMemoryHistory",
		Summary:     "List redacted memory operation history",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: append(memorySelectorQueryParams(),
			queryParam("operation", "Memory operation type", false),
			dateTimeQueryParam("since", "Only operations since this timestamp"),
			intQueryParam("limit", "Maximum number of operations to return"),
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryOperationHistoryResponse{}},
			memoryError(400, "Invalid memory history filter"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func showMemoryScopeOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/scope-show",
		OperationID: "showMemoryScope",
		Summary:     "Resolve the effective Memory v2 scope/tier and precedence chain",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  memorySelectorQueryParams(),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryScopeShowResponse{}},
			memoryError(400, "Invalid memory scope selector"),
			memoryError(404, "Workspace or agent not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func readMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        specAPIMemoryFilenamePath,
		OperationID: "readMemory",
		Summary:     "Read one memory document",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  append([]ParameterSpec{pathParam("filename", "Memory filename")}, memorySelectorQueryParams()...),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryEntryResponse{}},
			memoryError(400, "Invalid memory reference"),
			memoryError(404, "Memory not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func writeMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory",
		OperationID: "writeMemory",
		Summary:     "Create or propose one Memory v2 curated entry",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.MemoryCreateRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryMutationDecisionResponse{}},
			memoryError(400, "Invalid memory write request"),
			memoryError(409, "Memory decision conflict"),
			memoryError(422, "Memory write rejected by policy"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func editMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPatch,
		Path:        specAPIMemoryFilenamePath,
		OperationID: "editMemory",
		Summary:     "Edit one Memory v2 curated entry through the controller",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("filename", "Memory filename"),
		},
		RequestBody: contract.MemoryEditRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryMutationDecisionResponse{}},
			memoryError(400, "Invalid memory edit request"),
			memoryError(404, "Memory not found"),
			memoryError(409, "Memory decision conflict"),
			memoryError(422, "Memory edit rejected by policy"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func deleteMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodDelete,
		Path:        specAPIMemoryFilenamePath,
		OperationID: "deleteMemory",
		Summary:     "Delete one memory document",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  append([]ParameterSpec{pathParam("filename", "Memory filename")}, memorySelectorQueryParams()...),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryDeleteResponse{}},
			memoryError(400, "Invalid memory reference"),
			memoryError(404, "Memory not found"),
			memoryError(409, "Memory decision conflict"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func searchMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/search",
		OperationID: "searchMemory",
		Summary:     "Run deterministic Memory v2 recall/search",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.MemorySearchRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemorySearchResponse{}},
			memoryError(400, "Invalid memory search request"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func reindexMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/reindex",
		OperationID: "reindexMemory",
		Summary:     "Rebuild Memory v2 derived catalog indexes",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.MemoryReindexV2Request{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryReindexResponse{}},
			memoryError(400, "Invalid memory reindex request"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func promoteMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/promote",
		OperationID: "promoteMemory",
		Summary:     "Promote a Memory v2 entry between scopes or agent tiers",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.MemoryPromoteRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryPromoteResponse{}},
			memoryError(400, "Invalid memory promote request"),
			memoryError(404, "Memory not found"),
			memoryError(409, "Memory promotion conflict"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func resetMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/reset",
		OperationID: "resetMemory",
		Summary:     "Reset Memory v2 derived state or curated storage",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.MemoryResetRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryResetResponse{}},
			memoryError(400, "Invalid memory reset request"),
			memoryError(409, "Memory reset confirmation required"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func reloadMemoryOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/reload",
		OperationID: "reloadMemory",
		Summary:     "Invalidate Memory v2 frozen snapshots for the next session boot",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryReloadResponse{}},
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func listMemoryDecisionsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/decisions",
		OperationID: "listMemoryDecisions",
		Summary:     "List Memory v2 controller decisions",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters:  append(memorySelectorQueryParams(), memoryDecisionQueryParams()...),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryDecisionListResponse{}},
			memoryError(400, "Invalid memory decision filter"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func getMemoryDecisionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/decisions/{decision_id}",
		OperationID: "getMemoryDecision",
		Summary:     "Get one Memory v2 controller decision",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("decision_id", "Controller decision id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryDecisionResponse{}},
			memoryError(404, "Memory decision not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func revertMemoryDecisionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/decisions/{decision_id}/revert",
		OperationID: "revertMemoryDecision",
		Summary:     "Revert one applied Memory v2 controller decision",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("decision_id", "Controller decision id"),
		},
		RequestBody: contract.MemoryDecisionRevertRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryDecisionRevertResponse{}},
			memoryError(400, "Invalid memory decision revert request"),
			memoryError(404, "Memory decision not found"),
			memoryError(409, "Memory decision cannot be reverted"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func getMemoryRecallTraceOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/recall-traces/{session_id}/{turn_seq}",
		OperationID: "getMemoryRecallTrace",
		Summary:     "Get one Memory v2 recall trace",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("session_id", "Session id"),
			{
				Name:        "turn_seq",
				In:          openapi3.ParameterInPath,
				Description: "Turn sequence",
				Required:    true,
				Kind:        specIntegerKey,
				Format:      specFormatInt64,
			},
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryRecallTraceResponse{}},
			memoryError(404, "Memory recall trace not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
