package cli

import (
	"context"

	"fmt"

	"net/http"
	"net/url"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func (c *unixSocketClient) MemoryHealth(ctx context.Context, workspace string) (MemoryHealthRecord, error) {
	var response MemoryHealthRecord
	values := url.Values{}
	if trimmed := strings.TrimSpace(workspace); trimmed != "" {
		values.Set("workspace", trimmed)
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/memory/health", values, nil, &response); err != nil {
		return MemoryHealthRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) MemoryHistory(
	ctx context.Context,
	query MemoryHistoryQuery,
) ([]MemoryHistoryRecord, error) {
	var response contract.MemoryOperationHistoryResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/memory/history",
		memoryHistoryValues(query),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Operations, nil
}

func (c *unixSocketClient) ShowMemory(
	ctx context.Context,
	filename string,
	query MemorySelectorQuery,
) (MemoryEntryRecord, error) {
	var response MemoryEntryRecord
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/memory/"+url.PathEscape(strings.TrimSpace(filename)),
		memorySelectorValues(query),
		nil,
		&response,
	); err != nil {
		return MemoryEntryRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) CreateMemory(
	ctx context.Context,
	request MemoryCreateRequest,
) (MemoryMutationRecord, error) {
	var response MemoryMutationRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/memory", nil, request, &response); err != nil {
		return MemoryMutationRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) EditMemory(
	ctx context.Context,
	filename string,
	request MemoryEditRequest,
) (MemoryMutationRecord, error) {
	var response MemoryMutationRecord
	if err := c.doJSON(
		ctx,
		http.MethodPatch,
		"/api/memory/"+url.PathEscape(strings.TrimSpace(filename)),
		nil,
		request,
		&response,
	); err != nil {
		return MemoryMutationRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) DeleteMemory(
	ctx context.Context,
	filename string,
	query MemorySelectorQuery,
) (MemoryDeleteRecord, error) {
	var response MemoryDeleteRecord
	if err := c.doJSON(
		ctx,
		http.MethodDelete,
		"/api/memory/"+url.PathEscape(strings.TrimSpace(filename)),
		memorySelectorValues(query),
		nil,
		&response,
	); err != nil {
		return MemoryDeleteRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) SearchMemory(
	ctx context.Context,
	request MemorySearchRequest,
) (MemorySearchRecord, error) {
	var response MemorySearchRecord
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/memory/search",
		nil,
		request,
		&response,
	); err != nil {
		return MemorySearchRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ReindexMemory(
	ctx context.Context,
	request MemoryReindexRequest,
) (MemoryReindexRecord, error) {
	var response MemoryReindexRecord
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/memory/reindex",
		nil,
		request,
		&response,
	); err != nil {
		return MemoryReindexRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) PromoteMemory(
	ctx context.Context,
	request MemoryPromoteRequest,
) (MemoryPromoteRecord, error) {
	var response MemoryPromoteRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/memory/promote", nil, request, &response); err != nil {
		return MemoryPromoteRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ResetMemory(ctx context.Context, request MemoryResetRequest) (MemoryResetRecord, error) {
	var response MemoryResetRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/memory/reset", nil, request, &response); err != nil {
		return MemoryResetRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ReloadMemory(ctx context.Context, request MemorySelectorQuery) (MemoryReloadRecord, error) {
	var response MemoryReloadRecord
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/memory/reload",
		memorySelectorValues(request),
		nil,
		&response,
	); err != nil {
		return MemoryReloadRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) MemoryScopeShow(
	ctx context.Context,
	query MemorySelectorQuery,
) (MemoryScopeShowRecord, error) {
	var response MemoryScopeShowRecord
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/memory/scope-show",
		memorySelectorValues(query),
		nil,
		&response,
	); err != nil {
		return MemoryScopeShowRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetMemoryRecallTrace(
	ctx context.Context,
	sessionID string,
	turnSeq int64,
) (MemoryRecallTraceRecord, error) {
	var response MemoryRecallTraceRecord
	path := fmt.Sprintf(
		"/api/memory/recall-traces/%s/%d",
		url.PathEscape(strings.TrimSpace(sessionID)),
		turnSeq,
	)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return MemoryRecallTraceRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListMemoryDreams(ctx context.Context) (MemoryDreamListRecord, error) {
	var response MemoryDreamListRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/memory/dreams", nil, nil, &response); err != nil {
		return MemoryDreamListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetMemoryDream(ctx context.Context, id string) (MemoryDreamRecord, error) {
	var response MemoryDreamRecord
	path := "/api/memory/dreams/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return MemoryDreamRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) TriggerMemoryDream(
	ctx context.Context,
	request MemoryDreamTriggerRequest,
) (MemoryDreamTriggerRecord, error) {
	var response MemoryDreamTriggerRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/memory/dreams/trigger", nil, request, &response); err != nil {
		return MemoryDreamTriggerRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) RetryMemoryDream(
	ctx context.Context,
	id string,
	request MemoryDreamRetryRequest,
) (MemoryDreamRetryRecord, error) {
	var response MemoryDreamRetryRecord
	path := "/api/memory/dreams/" + url.PathEscape(strings.TrimSpace(id)) + "/retry"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return MemoryDreamRetryRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetMemoryDreamStatus(ctx context.Context) (MemoryDreamListRecord, error) {
	var response MemoryDreamListRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/memory/dreams/status", nil, nil, &response); err != nil {
		return MemoryDreamListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListMemoryDailyLogs(
	ctx context.Context,
	query MemorySelectorQuery,
) (MemoryDailyLogListRecord, error) {
	var response MemoryDailyLogListRecord
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/memory/daily",
		memorySelectorValues(query),
		nil,
		&response,
	); err != nil {
		return MemoryDailyLogListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetMemoryExtractorStatus(
	ctx context.Context,
	sessionID string,
) (MemoryExtractorStatusRecord, error) {
	var response MemoryExtractorStatusRecord
	values := url.Values{}
	if trimmed := strings.TrimSpace(sessionID); trimmed != "" {
		values.Set("session_id", trimmed)
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/memory/extractor/status", values, nil, &response); err != nil {
		return MemoryExtractorStatusRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListMemoryExtractorFailures(ctx context.Context) (MemoryExtractorFailuresRecord, error) {
	var response MemoryExtractorFailuresRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/memory/extractor/failures", nil, nil, &response); err != nil {
		return MemoryExtractorFailuresRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) RetryMemoryExtractor(
	ctx context.Context,
	request MemoryExtractorRetryRequest,
) (MemoryExtractorRetryRecord, error) {
	var response MemoryExtractorRetryRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/memory/extractor/retry", nil, request, &response); err != nil {
		return MemoryExtractorRetryRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) DrainMemoryExtractor(ctx context.Context) (MemoryExtractorDrainRecord, error) {
	var response MemoryExtractorDrainRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/memory/extractor/drain", nil, nil, &response); err != nil {
		return MemoryExtractorDrainRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListMemoryProviders(ctx context.Context) (MemoryProviderListRecord, error) {
	var response MemoryProviderListRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/memory/providers", nil, nil, &response); err != nil {
		return MemoryProviderListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetMemoryProvider(ctx context.Context, name string) (MemoryProviderRecord, error) {
	var response MemoryProviderRecord
	path := "/api/memory/providers/" + url.PathEscape(strings.TrimSpace(name))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return MemoryProviderRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) SelectMemoryProvider(
	ctx context.Context,
	request MemoryProviderSelectRequest,
) (MemoryProviderLifecycleRecord, error) {
	var response MemoryProviderLifecycleRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/memory/providers/select", nil, request, &response); err != nil {
		return MemoryProviderLifecycleRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) EnableMemoryProvider(
	ctx context.Context,
	name string,
	request MemoryProviderLifecycleRequest,
) (MemoryProviderLifecycleRecord, error) {
	var response MemoryProviderLifecycleRecord
	path := "/api/memory/providers/" + url.PathEscape(strings.TrimSpace(name)) + "/enable"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return MemoryProviderLifecycleRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) DisableMemoryProvider(
	ctx context.Context,
	name string,
	request MemoryProviderLifecycleRequest,
) (MemoryProviderLifecycleRecord, error) {
	var response MemoryProviderLifecycleRecord
	path := "/api/memory/providers/" + url.PathEscape(strings.TrimSpace(name)) + "/disable"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return MemoryProviderLifecycleRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) CreateMemoryAdhocNote(
	ctx context.Context,
	request MemoryAdhocNoteRequest,
) (MemoryAdhocNoteRecord, error) {
	var response MemoryAdhocNoteRecord
	if err := c.doJSON(ctx, http.MethodPost, "/api/memory/ad-hoc", nil, request, &response); err != nil {
		return MemoryAdhocNoteRecord{}, err
	}
	return response, nil
}
