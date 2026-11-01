package cli

import (
	"context"
	"encoding/json"
	"errors"
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
		return "GET", "", nil
	case terminalStreamModeWrite:
		return "PUT", "", nil
	case agentMemoryAddOperation:
		return "POST", agentMemoryEntriesPath, nil
	case agentMemoryReplaceOperation:
		return "PATCH", agentMemoryEntriesPath, nil
	case string(gatewayProfileTransactionRemove):
		return "DELETE", agentMemoryEntriesPath, nil
	case agentMemorySearchOperation:
		return "POST", "/search", nil
	case cliValidateVerb:
		return "POST", "/validate", nil
	case agentMemoryHistoryOperation:
		return "GET", "/history", nil
	case agentMemoryRollbackOperation:
		return "POST", "/rollback", nil
	default:
		return "", "", errors.New("unknown agent memory operation")
	}
}
