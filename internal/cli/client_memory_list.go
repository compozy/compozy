package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// MemoryListQuery captures one stable Memory v2 header page request.
type MemoryListQuery struct {
	MemorySelectorQuery
	Type   memcontract.Type
	Sort   string
	Cursor string
	Limit  int
}

// MemoryListRecord wraps one Memory v2 header page.
type MemoryListRecord = contract.MemoryListResponse

func (c *unixSocketClient) ListMemory(
	ctx context.Context,
	query MemoryListQuery,
) (MemoryListRecord, error) {
	var response MemoryListRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/memory", memoryListValues(query), nil, &response); err != nil {
		return MemoryListRecord{}, err
	}
	return response, nil
}

func memoryListValues(query MemoryListQuery) url.Values {
	values := memorySelectorValues(query.MemorySelectorQuery)
	if trimmed := strings.TrimSpace(string(query.Type)); trimmed != "" {
		values.Set("type", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Sort); trimmed != "" {
		values.Set("sort", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Cursor); trimmed != "" {
		values.Set("cursor", trimmed)
	}
	if query.Limit != 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}
