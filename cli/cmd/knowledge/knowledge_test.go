package knowledge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/cli/cmd/knowledge"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

const requestTimeout = 2 * time.Second

func newCLIContext(t *testing.T, baseURL string) context.Context {
	t.Helper()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	mgr := config.NewManager(t.Context(), config.NewService())
	_, err := mgr.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := mgr.Get()
	cfg.Server.Auth.Enabled = false
	cfg.CLI.BaseURL = baseURL
	cfg.CLI.Timeout = time.Second
	ctx = config.ContextWithManager(ctx, mgr)
	ctx = context.WithValue(ctx, helpers.ConfigKey, cfg)
	return ctx
}

func TestKnowledgeListCommand(t *testing.T) {
	t.Run("ShouldForwardQueryParameters", func(t *testing.T) {
		pathCh := make(chan string, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			pathCh <- r.URL.String()
			_, err := w.Write([]byte(`{"status":200,"data":{"knowledge_bases":[],"page":{"limit":1}}}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		cmdObj := knowledge.Cmd()
		cmdObj.SetContext(newCLIContext(t, server.URL))
		cmdObj.SetOut(&bytes.Buffer{})
		cmdObj.SetErr(&bytes.Buffer{})
		cmdObj.SilenceUsage = true
		cmdObj.SilenceErrors = true
		cmdObj.SetArgs([]string{"list", "--limit", "2", "--cursor", "abc", "--project", "demo"})
		require.NoError(t, cmdObj.Execute())

		select {
		case p := <-pathCh:
			u, err := url.Parse(p)
			require.NoError(t, err)
			require.Equal(t, "/knowledge-bases", u.Path)
			query := u.Query()
			require.Equal(t, "2", query.Get("limit"))
			require.Equal(t, "abc", query.Get("cursor"))
			require.Equal(t, "demo", query.Get("project"))
		case <-time.After(requestTimeout):
			t.Fatal("timed out waiting for request path")
		}
	})

	t.Run("ShouldReturnErrorOnServerFailure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"status":500,"error":"boom"}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		cmdObj := knowledge.Cmd()
		cmdObj.SetContext(newCLIContext(t, server.URL))
		cmdObj.SetOut(&bytes.Buffer{})
		cmdObj.SetErr(&bytes.Buffer{})
		cmdObj.SilenceUsage = true
		cmdObj.SilenceErrors = true
		cmdObj.SetArgs([]string{"list"})
		require.Error(t, cmdObj.Execute())
	})
}

func TestKnowledgeApplyCommand(t *testing.T) {
	t.Run("ShouldSendFilePayload", func(t *testing.T) {
		reqBody := make(chan map[string]any, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			defer r.Body.Close()
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			reqBody <- body
			_, err := w.Write([]byte(`{"status":200,"data":{"knowledge_base":{"id":"docs"}}}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		dir := t.TempDir()
		file := filepath.Join(dir, "kb.json")
		content := `{"id":"docs","embedder":"openai","vector_db":"default","sources":[{"type":"markdown_glob","path":"docs/*.md"}]}`
		require.NoError(t, os.WriteFile(file, []byte(content), 0600))

		cmdObj := knowledge.Cmd()
		cmdObj.SetContext(newCLIContext(t, server.URL))
		cmdObj.SetOut(&bytes.Buffer{})
		cmdObj.SetErr(&bytes.Buffer{})
		cmdObj.SilenceUsage = true
		cmdObj.SilenceErrors = true
		cmdObj.SetArgs([]string{"apply", "--file", file})
		require.NoError(t, cmdObj.Execute())

		select {
		case body := <-reqBody:
			require.Equal(t, "docs", body["id"])
		case <-time.After(requestTimeout):
			t.Fatal("timed out waiting for request body")
		}
	})
}

func TestKnowledgeQueryCommand(t *testing.T) {
	t.Run("ShouldSendQueryOverrides", func(t *testing.T) {
		reqBody := make(chan map[string]any, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			defer r.Body.Close()
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			reqBody <- body
			_, err := w.Write([]byte(`{"status":200,"data":{"matches":[]}}`))
			require.NoError(t, err)
		}))
		defer server.Close()

		cmdObj := knowledge.Cmd()
		cmdObj.SetContext(newCLIContext(t, server.URL))
		cmdObj.SetOut(&bytes.Buffer{})
		cmdObj.SetErr(&bytes.Buffer{})
		cmdObj.SilenceUsage = true
		cmdObj.SilenceErrors = true
		cmdObj.SetArgs(
			[]string{
				"query",
				"docs",
				"--query",
				"hello",
				"--top-k",
				"3",
				"--min-score",
				"0.4",
				"--filter",
				"tag=support",
			},
		)
		require.NoError(t, cmdObj.Execute())

		select {
		case body := <-reqBody:
			require.Equal(t, "hello", body["query"])
			require.Equal(t, float64(3), body["top_k"])
			require.Equal(t, 0.4, body["min_score"])
			filters, ok := body["filters"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "support", filters["tag"])
		case <-time.After(requestTimeout):
			t.Fatal("timed out waiting for request body")
		}
	})
}
