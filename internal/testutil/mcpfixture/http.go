package mcpfixture

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// StreamableHTTPHandler returns a stateless, JSON-response Streamable HTTP handler.
func (f *Fixture) StreamableHTTPHandler() http.Handler {
	server := f.Server()
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		Stateless:                  true,
		JSONResponse:               true,
		DisableLocalhostProtection: true,
	})
	if f.profile == ProfileLegacy2025 {
		return legacyDiscoverFallbackHandler(handler)
	}
	return handler
}

// StartHTTP starts the fixture on a loopback server and closes it with t.
func (f *Fixture) StartHTTP(t testing.TB) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(f.StreamableHTTPHandler())
	t.Cleanup(server.Close)
	return server
}

func legacyDiscoverFallbackHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			next.ServeHTTP(writer, request)
			return
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "read request body", http.StatusBadRequest)
			return
		}
		if err := request.Body.Close(); err != nil {
			http.Error(writer, "close request body", http.StatusBadRequest)
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))

		var envelope struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil || envelope.Method != "server/discover" {
			next.ServeHTTP(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(writer).Encode(struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   *jsonrpc.Error  `json:"error"`
		}{
			JSONRPC: "2.0",
			ID:      envelope.ID,
			Error: &jsonrpc.Error{
				Code:    jsonrpc.CodeMethodNotFound,
				Message: "server/discover is unavailable in the legacy fixture",
			},
		}); err != nil {
			return
		}
	})
}
