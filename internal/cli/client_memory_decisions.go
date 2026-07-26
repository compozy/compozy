package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// MemoryDecisionListQuery captures filters for controller decision history.
type MemoryDecisionListQuery struct {
	Scope          memcontract.Scope
	WorkspaceID    string
	AgentName      string
	AgentTier      memcontract.AgentTier
	Operation      string
	TargetFilename string
	Since          time.Time
	Reason         string
	Limit          int
}

// MemoryDecisionListRecord wraps controller decision history.
type MemoryDecisionListRecord = contract.MemoryDecisionListResponse

// MemoryDecisionRecord wraps one controller decision.
type MemoryDecisionRecord = contract.MemoryDecisionResponse

// MemoryDecisionRevertRequest captures a decision revert request.
type MemoryDecisionRevertRequest = contract.MemoryDecisionRevertRequest

// MemoryDecisionRevertRecord captures a decision revert response.
type MemoryDecisionRevertRecord = contract.MemoryDecisionRevertResponse

func (c *unixSocketClient) ListMemoryDecisions(
	ctx context.Context,
	query MemoryDecisionListQuery,
) (MemoryDecisionListRecord, error) {
	var response MemoryDecisionListRecord
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/memory/decisions",
		memoryDecisionValues(query),
		nil,
		&response,
	); err != nil {
		return MemoryDecisionListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetMemoryDecision(ctx context.Context, id string) (MemoryDecisionRecord, error) {
	var response MemoryDecisionRecord
	path := "/api/memory/decisions/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return MemoryDecisionRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) RevertMemoryDecision(
	ctx context.Context,
	id string,
	request MemoryDecisionRevertRequest,
) (MemoryDecisionRevertRecord, error) {
	var response MemoryDecisionRevertRecord
	path := "/api/memory/decisions/" + url.PathEscape(strings.TrimSpace(id)) + "/revert"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return MemoryDecisionRevertRecord{}, err
	}
	return response, nil
}

func memoryDecisionValues(query MemoryDecisionListQuery) url.Values {
	values := memorySelectorValues(MemorySelectorQuery{
		Scope:       query.Scope,
		WorkspaceID: query.WorkspaceID,
		AgentName:   query.AgentName,
		AgentTier:   query.AgentTier,
	})
	if trimmed := strings.TrimSpace(query.Operation); trimmed != "" {
		values.Set("op", trimmed)
	}
	if trimmed := strings.TrimSpace(query.TargetFilename); trimmed != "" {
		values.Set("filename", trimmed)
	}
	if !query.Since.IsZero() {
		values.Set("since", query.Since.UTC().Format(time.RFC3339Nano))
	}
	if trimmed := strings.TrimSpace(query.Reason); trimmed != "" {
		values.Set("reason", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}
