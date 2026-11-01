package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

func (c *daemonClient) AgentMemory(ctx context.Context, name string, query AgentQuery, operation string, body any) (json.RawMessage, error) {
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
	case "inspect":
		return "GET", "", nil
	case "write":
		return "PUT", "", nil
	case "add":
		return "POST", "/entries", nil
	case "replace":
		return "PATCH", "/entries", nil
	case "remove":
		return "DELETE", "/entries", nil
	case "search":
		return "POST", "/search", nil
	case "validate":
		return "POST", "/validate", nil
	case "history":
		return "GET", "/history", nil
	case "rollback":
		return "POST", "/rollback", nil
	default:
		return "", "", errors.New("unknown agent memory operation")
	}
}
