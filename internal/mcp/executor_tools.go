package mcp

import (
	"context"
	"errors"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxMCPToolListPages = 1_000

func listMCPTools(ctx context.Context, client *mcpsdk.ClientSession) ([]*mcpsdk.Tool, int, error) {
	tools := make([]*mcpsdk.Tool, 0)
	cursor := ""
	seen := map[string]struct{}{}
	ttlMs := 0
	for range maxMCPToolListPages {
		result, err := client.ListTools(ctx, &mcpsdk.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, 0, err
		}
		if result == nil {
			return nil, 0, errors.New("mcp: tools/list returned an empty result")
		}
		tools = append(tools, result.Tools...)
		if ttl := result.GetTTLMs(); ttl > 0 && (ttlMs == 0 || ttl < ttlMs) {
			ttlMs = ttl
		}
		next := strings.TrimSpace(result.NextCursor)
		if next == "" {
			return tools, ttlMs, nil
		}
		if _, duplicate := seen[next]; duplicate {
			return nil, 0, errors.New("mcp: tools/list returned a repeated cursor")
		}
		seen[next] = struct{}{}
		cursor = next
	}
	return nil, 0, errors.New("mcp: tools/list exceeded maximum page count")
}
