package vectordb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	appconfig "github.com/compozy/compozy/pkg/config"
)

type qdrantStore struct {
	client     *http.Client
	baseURL    string
	collection string
	dimension  int
	metric     string
	apiKey     string
}

// qdrantSearchResult captures the fields returned by Qdrant search responses.
type qdrantSearchResult struct {
	ID      any            `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

const (
	qdrantDefaultTimeout = 10 * time.Second
	qdrantDefaultTopK    = 5
)

func newQdrantStore(ctx context.Context, cfg *Config) (Store, error) {
	if cfg == nil {
		return nil, errors.New("vector_db config is required")
	}
	base := strings.TrimRight(cfg.DSN, "/")
	if base == "" {
		return nil, fmt.Errorf("vector_db %q: qdrant dsn is required", cfg.ID)
	}
	collection := cfg.Collection
	if collection == "" {
		collection = cfg.Table
	}
	if collection == "" {
		collection = cfg.ID
	}
	timeout := qdrantDefaultTimeout
	if globalCfg := appconfig.FromContext(ctx); globalCfg != nil && globalCfg.Knowledge.VectorHTTPTimeout > 0 {
		timeout = globalCfg.Knowledge.VectorHTTPTimeout
	}
	store := &qdrantStore{
		client:     &http.Client{Timeout: timeout},
		baseURL:    base,
		collection: collection,
		dimension:  cfg.Dimension,
		metric:     chooseMetric(cfg.Metric),
	}
	if key, ok := cfg.Auth["api_key"]; ok {
		store.apiKey = key
	}
	if err := store.ensureCollection(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func chooseMetric(metric string) string {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "euclid", "euclidean", "l2":
		return "Euclid"
	case "dot", "dotproduct":
		return "Dot"
	default:
		return "Cosine"
	}
}

func (q *qdrantStore) ensureCollection(ctx context.Context) error {
	body := map[string]any{
		"vectors": map[string]any{
			"size":     q.dimension,
			"distance": q.metric,
		},
	}
	return q.doRequest(ctx, http.MethodPut, fmt.Sprintf("/collections/%s", q.collection), body, nil)
}

// buildQdrantFilter builds the request filter payload for Qdrant operations.
func buildQdrantFilter(filters map[string]string) map[string]any {
	if len(filters) == 0 {
		return nil
	}
	must := make([]any, 0, len(filters))
	for key, val := range filters {
		must = append(must, map[string]any{
			"key":   key,
			"match": map[string]any{"value": val},
		})
	}
	return map[string]any{"must": must}
}

// mapQdrantResults converts Qdrant search results into the internal Match slice.
func mapQdrantResults(results []qdrantSearchResult, minScore float64) []Match {
	matches := make([]Match, 0, len(results))
	for _, res := range results {
		if res.Score < minScore {
			continue
		}
		id := fmt.Sprint(res.ID)
		payload := core.CloneMap(res.Payload)
		if payload == nil {
			payload = make(map[string]any)
		}
		text := ""
		if raw, ok := payload["text"].(string); ok {
			text = raw
			delete(payload, "text")
		}
		matches = append(matches, Match{
			ID:       id,
			Score:    res.Score,
			Text:     text,
			Metadata: payload,
		})
	}
	return matches
}

func (q *qdrantStore) Upsert(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}
	points := make([]any, 0, len(records))
	for i := range records {
		rec := records[i]
		if len(rec.Embedding) != q.dimension {
			return fmt.Errorf("qdrant: record %q dimension mismatch", rec.ID)
		}
		payload := core.CloneMap(rec.Metadata)
		if payload == nil {
			payload = make(map[string]any)
		}
		payload["text"] = rec.Text
		points = append(points, map[string]any{
			"id":      rec.ID,
			"vector":  rec.Embedding,
			"payload": payload,
		})
	}
	body := map[string]any{
		"points": points,
	}
	return q.doRequest(ctx, http.MethodPut, fmt.Sprintf("/collections/%s/points", q.collection), body, nil)
}

func (q *qdrantStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]Match, error) {
	if len(query) != q.dimension {
		return nil, fmt.Errorf("qdrant: query dimension mismatch")
	}
	limit := opts.TopK
	if limit <= 0 {
		limit = qdrantDefaultTopK
	}
	request := map[string]any{
		"vector":       query,
		"limit":        limit,
		"with_payload": true,
	}
	if filter := buildQdrantFilter(opts.Filters); filter != nil {
		request["filter"] = filter
	}
	var response struct {
		Result []qdrantSearchResult `json:"result"`
	}
	searchPath := fmt.Sprintf("/collections/%s/points/search", q.collection)
	if err := q.doRequest(ctx, http.MethodPost, searchPath, request, &response); err != nil {
		return nil, err
	}
	return mapQdrantResults(response.Result, opts.MinScore), nil
}

func (q *qdrantStore) Delete(ctx context.Context, filter Filter) error {
	request := map[string]any{}
	if len(filter.IDs) > 0 {
		request["points"] = filter.IDs
	}
	if f := buildQdrantFilter(filter.Metadata); f != nil {
		request["filter"] = f
	}
	if len(request) == 0 {
		return nil
	}
	return q.doRequest(ctx, http.MethodPost, fmt.Sprintf("/collections/%s/points/delete", q.collection), request, nil)
}

func (q *qdrantStore) Close(context.Context) error {
	return nil
}

func (q *qdrantStore) doRequest(ctx context.Context, method, path string, body any, out any) error {
	var buf *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("qdrant: marshal request: %w", err)
		}
		buf = bytes.NewReader(payload)
	} else {
		buf = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, buf)
	if err != nil {
		return fmt.Errorf("qdrant: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if q.apiKey != "" {
		req.Header.Set("api-key", q.apiKey)
	}
	resp, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("qdrant: request failed: %w", err)
	}
	defer resp.Body.Close()
	payload, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("qdrant: read response: %w", readErr)
	}
	if resp.StatusCode >= 400 {
		var apiErr struct {
			Result any    `json:"result"`
			Status string `json:"status"`
			Error  string `json:"error"`
		}
		if err := json.Unmarshal(payload, &apiErr); err != nil {
			return fmt.Errorf("qdrant: request failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("qdrant: %s (%d): %s", apiErr.Error, resp.StatusCode, apiErr.Status)
	}
	if out != nil {
		if err := json.Unmarshal(payload, out); err != nil {
			return fmt.Errorf("qdrant: decode response: %w", err)
		}
	}
	return nil
}
