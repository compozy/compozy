package spec

import "github.com/compozy/agh/internal/api/contract"

func registryMemoryLifecycleOperations() []OperationSpec {
	return []OperationSpec{
		listMemoryDreamsOperationSpec(),
		getMemoryDreamOperationSpec(),
		triggerMemoryDreamOperationSpec(),
		retryMemoryDreamOperationSpec(),
		getMemoryDreamStatusOperationSpec(),
		listMemoryDailyLogsOperationSpec(),
		getMemoryExtractorStatusOperationSpec(),
		listMemoryExtractorFailuresOperationSpec(),
		retryMemoryExtractorOperationSpec(),
		drainMemoryExtractorOperationSpec(),
		listMemoryProvidersOperationSpec(),
		getMemoryProviderOperationSpec(),
		selectMemoryProviderOperationSpec(),
		enableMemoryProviderOperationSpec(),
		disableMemoryProviderOperationSpec(),
		createMemoryAdhocNoteOperationSpec(),
		getMemorySessionLedgerOperationSpec(),
		replayMemorySessionOperationSpec(),
		pruneMemorySessionsOperationSpec(),
		repairMemorySessionsOperationSpec(),
	}
}
func listMemoryDreamsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/dreams",
		OperationID: "listMemoryDreams",
		Summary:     "List Memory v2 dreaming runs",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: append(memorySelectorQueryParams(),
			queryParam("status", "Dream status", false),
			intQueryParam("limit", "Maximum number of dreaming runs to return"),
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryDreamListResponse{}},
			memoryError(400, "Invalid memory dream filter"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func getMemoryDreamOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/dreams/{dream_id}",
		OperationID: "getMemoryDream",
		Summary:     "Get one Memory v2 dreaming run",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("dream_id", "Dreaming run id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryDreamResponse{}},
			memoryError(404, "Memory dream not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func triggerMemoryDreamOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/dreams/trigger",
		OperationID: "triggerMemoryDream",
		Summary:     "Trigger Memory v2 dreaming immediately",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.MemoryDreamTriggerRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryDreamTriggerResponse{}},
			memoryError(400, "Invalid memory dream trigger request"),
			memoryError(409, "Memory dream gate not satisfied"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func retryMemoryDreamOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/dreams/{dream_id}/retry",
		OperationID: "retryMemoryDream",
		Summary:     "Retry a failed Memory v2 dreaming run",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("dream_id", "Dreaming run id"),
		},
		RequestBody: contract.MemoryDreamRetryRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryDreamRetryResponse{}},
			memoryError(400, "Invalid memory dream retry request"),
			memoryError(404, "Memory dream not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func getMemoryDreamStatusOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/dreams/status",
		OperationID: "getMemoryDreamStatus",
		Summary:     "Get Memory v2 dreaming status",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryDreamListResponse{}},
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func listMemoryDailyLogsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/daily",
		OperationID: "listMemoryDailyLogs",
		Summary:     "List Memory v2 daily operation logs",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: append(memorySelectorQueryParams(),
			queryParam("date", "Daily log date in YYYY-MM-DD format", false),
			intQueryParam("limit", "Maximum number of daily logs to return"),
		),
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryDailyLogListResponse{}},
			memoryError(400, "Invalid memory daily log filter"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func getMemoryExtractorStatusOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/extractor/status",
		OperationID: "getMemoryExtractorStatus",
		Summary:     "Get Memory v2 extractor queue status",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryExtractorStatusResponse{}},
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func listMemoryExtractorFailuresOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/extractor/failures",
		OperationID: "listMemoryExtractorFailures",
		Summary:     "List Memory v2 extractor DLQ records",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("session_id", "Filter by session id", false),
			intQueryParam("limit", "Maximum number of failures to return"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryExtractorFailuresResponse{}},
			memoryError(400, "Invalid extractor failure filter"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func retryMemoryExtractorOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/extractor/retry",
		OperationID: "retryMemoryExtractor",
		Summary:     "Retry Memory v2 extractor DLQ records",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.MemoryExtractorRetryRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryExtractorRetryResponse{}},
			memoryError(400, "Invalid extractor retry request"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func drainMemoryExtractorOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/extractor/drain",
		OperationID: "drainMemoryExtractor",
		Summary:     "Drain Memory v2 extractor queue",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryExtractorDrainResponse{}},
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func listMemoryProvidersOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/providers",
		OperationID: "listMemoryProviders",
		Summary:     "List registered Memory v2 providers",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryProviderListResponse{}},
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func getMemoryProviderOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/memory/providers/{provider_name}",
		OperationID: "getMemoryProvider",
		Summary:     "Get one Memory v2 provider",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("provider_name", "Memory provider name"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryProviderResponse{}},
			memoryError(404, "Memory provider not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func selectMemoryProviderOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/providers/select",
		OperationID: "selectMemoryProvider",
		Summary:     "Select the active Memory v2 provider",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.MemoryProviderSelectRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryProviderResponse{}},
			memoryError(400, "Invalid memory provider selection"),
			memoryError(404, "Memory provider not found"),
			memoryError(409, "Memory provider collision"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func enableMemoryProviderOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/providers/{provider_name}/enable",
		OperationID: "enableMemoryProvider",
		Summary:     "Enable a Memory v2 provider",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("provider_name", "Memory provider name"),
		},
		RequestBody: contract.MemoryProviderLifecycleRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryProviderLifecycleResponse{}},
			memoryError(400, "Invalid memory provider enable request"),
			memoryError(404, "Memory provider not found"),
			memoryError(409, "Memory provider collision"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func disableMemoryProviderOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/providers/{provider_name}/disable",
		OperationID: "disableMemoryProvider",
		Summary:     "Disable a Memory v2 provider",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("provider_name", "Memory provider name"),
		},
		RequestBody: contract.MemoryProviderLifecycleRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryProviderLifecycleResponse{}},
			memoryError(400, "Invalid memory provider disable request"),
			memoryError(404, "Memory provider not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func createMemoryAdhocNoteOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/ad-hoc",
		OperationID: "createMemoryAdhocNote",
		Summary:     "Create a Memory v2 ad-hoc note for dreaming reconciliation",
		Tags:        []string{specMemoryKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.MemoryAdhocNoteRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemoryAdhocNoteResponse{}},
			memoryError(400, "Invalid memory ad-hoc note request"),
			memoryError(422, "Memory ad-hoc note rejected by policy"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func getMemorySessionLedgerOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/memory/sessions/{session_id}/ledger",
		OperationID: "getMemorySessionLedger",
		Summary:     "Get one materialized Memory v2 session ledger",
		Tags:        []string{specMemoryKey, specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemorySessionLedgerResponse{}},
			memoryError(404, "Session ledger not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func replayMemorySessionOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/workspaces/{workspace_id}/memory/sessions/{session_id}/replay",
		OperationID: "replayMemorySession",
		Summary:     "Replay one materialized Memory v2 session ledger",
		Tags:        []string{specMemoryKey, specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Workspace id"),
			pathParam("session_id", "Session id"),
		},
		RequestBody: contract.MemorySessionReplayRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemorySessionReplayResponse{}},
			memoryError(400, "Invalid session replay request"),
			memoryError(404, "Session ledger not found"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func pruneMemorySessionsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/sessions/prune",
		OperationID: "pruneMemorySessions",
		Summary:     "Prune materialized Memory v2 session ledger state",
		Tags:        []string{specMemoryKey, specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.MemorySessionsPruneRequest{},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemorySessionsPruneResponse{}},
			memoryError(400, "Invalid session prune request"),
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
func repairMemorySessionsOperationSpec() OperationSpec {
	return OperationSpec{
		Method:      httpMethodPost,
		Path:        "/api/memory/sessions/repair",
		OperationID: "repairMemorySessions",
		Summary:     "Repair materialized Memory v2 session ledgers",
		Tags:        []string{specMemoryKey, specSessionsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.MemorySessionsRepairResponse{}},
			memoryError(500, specInternalServerErrorDescription),
		},
	}
}
