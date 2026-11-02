package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

func (c *daemonClient) AgentMemory(
	ctx context.Context,
	name string,
	query AgentQuery,
	operation string,
	body any,
) (json.RawMessage, error) {
	method, path, err := agentMemoryRoute(operation)
	if err != nil {
		return nil, err
	}
	var response json.RawMessage
	path = "/api/agents/" + url.PathEscape(strings.TrimSpace(name)) + "/memory" + path
	if err := c.doJSON(ctx, method, path, agentValues(query), body, &response); err != nil {
		return nil, err
	}
	return response, nil
}

func agentMemoryRoute(operation string) (string, string, error) {
	switch operation {
	case loopInspectKey:
		return http.MethodGet, "", nil
	case terminalStreamModeWrite:
		return http.MethodPut, "", nil
	case agentMemoryAddOperation:
		return http.MethodPost, agentMemoryEntriesPath, nil
	case agentMemoryReplaceOperation:
		return http.MethodPatch, agentMemoryEntriesPath, nil
	case string(gatewayProfileTransactionRemove):
		return http.MethodDelete, agentMemoryEntriesPath, nil
	case agentMemorySearchOperation:
		return http.MethodPost, "/search", nil
	case cliValidateVerb:
		return http.MethodPost, "/validate", nil
	case agentMemoryHistoryOperation:
		return http.MethodGet, "/history", nil
	case agentMemoryRollbackOperation:
		return http.MethodPost, "/rollback", nil
	default:
		return "", "", errors.New("unknown agent memory operation")
	}
}
