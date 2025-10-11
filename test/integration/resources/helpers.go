package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/resources"
	serverhelpers "github.com/compozy/compozy/test/helpers/server"
	"github.com/stretchr/testify/require"
)

type resourceClient struct {
	t       *testing.T
	harness *serverhelpers.ServerHarness
}

func newResourceClient(t *testing.T) *resourceClient {
	t.Helper()
	return &resourceClient{t: t, harness: serverhelpers.NewServerHarness(t)}
}

func (c *resourceClient) do(
	method string,
	path string,
	payload any,
	headers map[string]string,
) *httptest.ResponseRecorder {
	c.t.Helper()
	var body *bytes.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		require.NoError(c.t, err)
		body = bytes.NewReader(raw)
	} else {
		body = bytes.NewReader(nil)
	}
	url := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		url = "http://localhost" + path
	}
	req := httptest.NewRequest(method, url, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res := httptest.NewRecorder()
	c.harness.Engine.ServeHTTP(res, req)
	return res
}

func (c *resourceClient) store() resources.ResourceStore {
	return c.harness.ResourceStore
}

func decodeData(t *testing.T, res *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope map[string]any
	err := json.Unmarshal(res.Body.Bytes(), &envelope)
	require.NoError(t, err)
	data, ok := envelope["data"].(map[string]any)
	require.True(t, ok)
	return data
}

func agentPayload(id string, instructions string) map[string]any {
	return map[string]any{
		"id":           id,
		"instructions": instructions,
		"model": map[string]any{
			"provider": "openai",
			"model":    "gpt-4o-mini",
		},
	}
}

func taskPayload(id string, instructions string) map[string]any {
	return map[string]any{
		"id":           id,
		"type":         "basic",
		"instructions": instructions,
		"model": map[string]any{
			"provider": "openai",
			"model":    "gpt-4o-mini",
		},
	}
}

func toolPayload(id string, method string, endpoint string) map[string]any {
	return map[string]any{
		"id":   id,
		"type": "http",
		"config": map[string]any{
			"method": method,
			"url":    endpoint,
		},
	}
}

func memoryPayload(id string) map[string]any {
	return map[string]any{
		"resource":    "memory",
		"id":          id,
		"type":        "buffer",
		"persistence": map[string]any{"type": "in_memory"},
	}
}

func mcpPayload(id string) map[string]any {
	return map[string]any{
		"id":        id,
		"transport": "sse",
		"url":       "http://localhost:6001/" + id,
	}
}

func modelPayload(provider string, model string) map[string]any {
	return map[string]any{
		"provider": provider,
		"model":    model,
	}
}

func schemaPayload(body map[string]any) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": body,
	}
}

func workflowPayload(id string, description string) map[string]any {
	return map[string]any{
		"id":          id,
		"description": description,
		"config":      map[string]any{},
		"tasks":       []map[string]any{},
		"agents":      []map[string]any{},
		"tools":       []map[string]any{},
	}
}

func projectPayload(version string, description string) map[string]any {
	return map[string]any{
		"version":     version,
		"description": description,
	}
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	maps.Copy(out, input)
	return out
}

func decodeList(t *testing.T, res *httptest.ResponseRecorder, key string) ([]map[string]any, map[string]any) {
	t.Helper()
	payload := decodeData(t, res)
	rawItems, ok := payload[key].([]any)
	require.True(t, ok)
	items := make([]map[string]any, len(rawItems))
	for i := range rawItems {
		item, ok := rawItems[i].(map[string]any)
		require.True(t, ok)
		items[i] = item
	}
	pageRaw, ok := payload["page"].(map[string]any)
	require.True(t, ok)
	return items, pageRaw
}

func collectIDs(t *testing.T, client *resourceClient, startPath string, collectionKey string, idKey string) []string {
	t.Helper()
	path := startPath
	visited := map[string]bool{}
	ids := make([]string, 0)
	for path != "" {
		if visited[path] {
			break
		}
		visited[path] = true
		res := client.do(http.MethodGet, path, nil, nil)
		if res.Code != http.StatusOK {
			require.Equalf(t, http.StatusOK, res.Code, "failed to fetch page %s: %s", path, res.Body.String())
		}
		items, _ := decodeList(t, res, collectionKey)
		for i := range items {
			val, ok := items[i][idKey].(string)
			require.True(t, ok)
			ids = append(ids, val)
		}
		nextLink := extractLink(res.Header().Get("Link"), "next")
		if nextLink == "" {
			break
		}
		path = normalizeLink(nextLink)
	}
	return ids
}

func extractLink(header string, rel string) string {
	segments := strings.Split(header, ",")
	for i := range segments {
		segment := strings.TrimSpace(segments[i])
		if !strings.Contains(segment, fmt.Sprintf("rel=%q", rel)) {
			continue
		}
		start := strings.Index(segment, "<")
		end := strings.Index(segment, ">")
		if start == -1 || end == -1 || end <= start+1 {
			continue
		}
		return segment[start+1 : end]
	}
	return ""
}

func normalizeLink(link string) string {
	if link == "" {
		return ""
	}
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		u, err := url.Parse(link)
		if err != nil {
			return link
		}
		if u.Path == "" {
			return link
		}
		if u.RawQuery != "" {
			return u.Path + "?" + u.RawQuery
		}
		return u.Path
	}
	return link
}
