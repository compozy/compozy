package resource_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/cli/cmd/tools"
	"github.com/compozy/compozy/cli/cmd/workflows"
	"github.com/compozy/compozy/cli/helpers"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

const requestPathTimeout = 2 * time.Second

func newCLIContext(t *testing.T, baseURL string) context.Context {
	t.Helper()
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	manager := config.NewManager(t.Context(), config.NewService())
	_, err := manager.Load(ctx, config.NewDefaultProvider())
	require.NoError(t, err)
	cfg := manager.Get()
	cfg.Server.Auth.Enabled = false
	cfg.CLI.BaseURL = baseURL
	cfg.CLI.Timeout = time.Second
	ctx = config.ContextWithManager(ctx, manager)
	ctx = context.WithValue(ctx, helpers.ConfigKey, cfg)
	return ctx
}

func TestResourceCommands(t *testing.T) {
	t.Parallel()
	t.Run("Should post to workflows export endpoint", func(t *testing.T) {
		t.Parallel()
		pathCh := make(chan string, 1)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			pathCh <- r.URL.String()
			require.Equal(t, http.MethodPost, r.Method)
			_, err := w.Write([]byte(`{"message":"export completed","data":{"written":1}}`))
			require.NoError(t, err)
		}))
		defer server.Close()
		cmdObj := workflows.Cmd()
		cmdObj.SetContext(newCLIContext(t, server.URL))
		cmdObj.SetOut(&bytes.Buffer{})
		cmdObj.SetErr(&bytes.Buffer{})
		cmdObj.SilenceErrors = true
		cmdObj.SilenceUsage = true
		cmdObj.SetArgs([]string{"export"})
		require.NoError(t, cmdObj.Execute())
		select {
		case p := <-pathCh:
			require.Equal(t, "/workflows/export", p)
		case <-time.After(requestPathTimeout):
			t.Fatal("timed out waiting for request path")
		}
	})
	t.Run("Should include strategy query for tool imports", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name         string
			args         []string
			expectedPath string
		}{
			{
				name:         "default strategy",
				args:         []string{"import"},
				expectedPath: "/tools/import?strategy=seed_only",
			},
			{
				name:         "overwrite strategy flag",
				args:         []string{"import", "--strategy", "overwrite_conflicts"},
				expectedPath: "/tools/import?strategy=overwrite_conflicts",
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				pathCh := make(chan string, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					pathCh <- r.URL.String()
					require.Equal(t, http.MethodPost, r.Method)
					_, err := w.Write([]byte(`{"message":"import completed","data":{"imported":1}}`))
					require.NoError(t, err)
				}))
				defer server.Close()
				cmdObj := tools.Cmd()
				cmdObj.SetContext(newCLIContext(t, server.URL))
				cmdObj.SetOut(&bytes.Buffer{})
				cmdObj.SetErr(&bytes.Buffer{})
				cmdObj.SilenceErrors = true
				cmdObj.SilenceUsage = true
				cmdObj.SetArgs(tc.args)
				require.NoError(t, cmdObj.Execute())
				select {
				case p := <-pathCh:
					require.Equal(t, tc.expectedPath, p)
				case <-time.After(requestPathTimeout):
					t.Fatal("timed out waiting for request path")
				}
			})
		}
	})
	t.Run("Should reject invalid strategy flag", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		cmdObj := tools.Cmd()
		cmdObj.SetContext(newCLIContext(t, server.URL))
		cmdObj.SetOut(&bytes.Buffer{})
		cmdObj.SetErr(&bytes.Buffer{})
		cmdObj.SilenceErrors = true
		cmdObj.SilenceUsage = true
		cmdObj.SetArgs([]string{"import", "--strategy", "invalid"})
		require.Error(t, cmdObj.Execute())
	})
}
