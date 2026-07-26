package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
)

func TestMCPAuthorizeUsesDaemonOwnedManualExchange(t *testing.T) {
	t.Parallel()

	t.Run("Should exchange a pasted code without exposing it", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		client.listSettingsMCPServersFn = func(
			context.Context,
			contract.SettingsWorkspaceScopeKind,
			string,
		) (contract.SettingsMCPServersResponse, error) {
			return mcpAuthServerResponse(mcpAuthStatus("linear", "global", "", false, nil)), nil
		}
		client.beginSettingsMCPAuthFn = func(
			_ context.Context,
			target SettingsMCPAuthTarget,
			request SettingsMCPAuthBeginRequest,
		) (SettingsMCPAuthBeginRecord, error) {
			if target.Name != "linear" || target.Scope != contract.SettingsWorkspaceScopeGlobal {
				t.Fatalf("begin target = %#v", target)
			}
			if request.Mode != contract.SettingsMCPAuthBeginModeManual {
				t.Fatalf("begin mode = %q, want manual", request.Mode)
			}
			return SettingsMCPAuthBeginRecord{
				AuthorizationURL: "https://auth.example/authorize?state=public-state",
				ExpiresAt:        time.Now().Add(time.Minute),
				ManualSupported:  true,
			}, nil
		}
		client.exchangeSettingsMCPAuthFn = func(
			_ context.Context,
			target SettingsMCPAuthTarget,
			request SettingsMCPAuthExchangeRequest,
		) (SettingsMCPAuthStatusRecord, error) {
			if target.Name != "linear" || request.Code != "one-time-code" || request.RedirectURL != "" {
				t.Fatalf("exchange target/request = %#v / %#v", target, request)
			}
			return mcpAuthStatus("linear", "global", "", true, timePointer(time.Now())), nil
		}

		stdout, stderr, err := executeMCPAuthCommandWithInput(
			t,
			newTestDeps(t, client),
			"one-time-code\n",
			"mcp", "authorize", "linear", "--manual", "-o", "json",
		)
		if err != nil {
			t.Fatalf("execute mcp authorize --manual error = %v", err)
		}
		if !strings.Contains(stderr, "https://auth.example/authorize?state=public-state") {
			t.Fatalf("stderr = %q, want live authorization URL", stderr)
		}
		if strings.Contains(stdout+stderr, "one-time-code") {
			t.Fatalf("command output leaked authorization code: stdout=%q stderr=%q", stdout, stderr)
		}
		var status SettingsMCPAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &status); err != nil {
			t.Fatalf("json.Unmarshal(status) error = %v", err)
		}
		if !confirmedMCPAuthStatus(status) {
			t.Fatalf("status = %#v, want confirmed credential", status)
		}
	})

	t.Run("Should reject a false-success exchange without token presence", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			listSettingsMCPServersFn: func(
				context.Context,
				contract.SettingsWorkspaceScopeKind,
				string,
			) (contract.SettingsMCPServersResponse, error) {
				return mcpAuthServerResponse(mcpAuthStatus("linear", "global", "", false, nil)), nil
			},
			beginSettingsMCPAuthFn: func(
				context.Context,
				SettingsMCPAuthTarget,
				SettingsMCPAuthBeginRequest,
			) (SettingsMCPAuthBeginRecord, error) {
				return SettingsMCPAuthBeginRecord{
					AuthorizationURL: "https://auth.example/authorize",
					ExpiresAt:        time.Now().Add(time.Minute),
				}, nil
			},
			exchangeSettingsMCPAuthFn: func(
				context.Context,
				SettingsMCPAuthTarget,
				SettingsMCPAuthExchangeRequest,
			) (SettingsMCPAuthStatusRecord, error) {
				status := mcpAuthStatus("linear", "global", "", false, nil)
				status.Status = string(mcpauth.StatusAuthenticated)
				return status, nil
			},
		}

		_, _, err := executeMCPAuthCommandWithInput(
			t,
			newTestDeps(t, client),
			"code\n",
			"mcp", "auth", "login", "linear", "--manual",
		)
		if err == nil || !strings.Contains(err.Error(), "confirmed credential") {
			t.Fatalf("mcp auth login error = %v, want confirmed-credential failure", err)
		}
	})

	t.Run("Should distinguish an existing server without OAuth from a missing server", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			listSettingsMCPServersFn: func(
				context.Context,
				contract.SettingsWorkspaceScopeKind,
				string,
			) (contract.SettingsMCPServersResponse, error) {
				return contract.SettingsMCPServersResponse{MCPServers: []contract.SettingsMCPServerItemPayload{{
					Name:  "filesystem",
					Scope: contract.SettingsScopeGlobal,
				}}}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"mcp", "auth", "status", "filesystem", "-o", "json",
		)
		if err != nil {
			t.Fatalf("execute mcp auth status error = %v", err)
		}
		if strings.TrimSpace(stdout) != "[]" {
			t.Fatalf("mcp auth status output = %q, want empty status list", stdout)
		}

		_, _, err = executeRootCommand(
			t,
			newTestDeps(t, client),
			"mcp", "authorize", "filesystem",
		)
		if err == nil || !strings.Contains(err.Error(), "does not configure OAuth") {
			t.Fatalf("mcp authorize error = %v, want OAuth configuration error", err)
		}
	})
}

func TestMCPAuthorizeManualHonorsTimeout(t *testing.T) {
	t.Parallel()

	newClient := func(exchange func(context.Context) error) *stubClient {
		return &stubClient{
			listSettingsMCPServersFn: func(
				context.Context,
				contract.SettingsWorkspaceScopeKind,
				string,
			) (contract.SettingsMCPServersResponse, error) {
				return mcpAuthServerResponse(mcpAuthStatus("linear", "global", "", false, nil)), nil
			},
			beginSettingsMCPAuthFn: func(
				context.Context,
				SettingsMCPAuthTarget,
				SettingsMCPAuthBeginRequest,
			) (SettingsMCPAuthBeginRecord, error) {
				return SettingsMCPAuthBeginRecord{
					AuthorizationURL: "https://auth.example/authorize",
					ExpiresAt:        time.Now().Add(time.Minute),
					ManualSupported:  true,
				}, nil
			},
			exchangeSettingsMCPAuthFn: func(
				ctx context.Context,
				_ SettingsMCPAuthTarget,
				_ SettingsMCPAuthExchangeRequest,
			) (SettingsMCPAuthStatusRecord, error) {
				if exchange == nil {
					t.Fatal("ExchangeSettingsMCPAuth called while input was pending")
				}
				return SettingsMCPAuthStatusRecord{}, exchange(ctx)
			},
		}
	}

	t.Run("Should interrupt pending manual input at the authorization deadline", func(t *testing.T) {
		t.Parallel()

		input := newDelayedMCPAuthReader(200 * time.Millisecond)
		client := newClient(nil)
		cmd := newRootCommand(newTestDeps(t, client))
		cmd.SetIn(input)
		cmd.SetArgs([]string{"mcp", "authorize", "linear", "--manual", "--timeout", "20ms"})
		err := cmd.ExecuteContext(t.Context())
		if err == nil || !strings.Contains(err.Error(), "authorization timed out") {
			t.Fatalf("mcp authorize error = %v, want authorization timeout", err)
		}
	})

	t.Run("Should carry the authorization deadline through manual exchange", func(t *testing.T) {
		t.Parallel()

		client := newClient(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(200 * time.Millisecond):
				return errors.New("exchange context did not expire")
			}
		})
		_, _, err := executeMCPAuthCommandWithInput(
			t,
			newTestDeps(t, client),
			"one-time-code\n",
			"mcp", "authorize", "linear", "--manual", "--timeout", "20ms",
		)
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("mcp authorize error = %v, want context deadline exceeded", err)
		}
	})
}

type delayedMCPAuthReader struct {
	delay  time.Duration
	closed chan struct{}
	once   sync.Once
}

func newDelayedMCPAuthReader(delay time.Duration) *delayedMCPAuthReader {
	return &delayedMCPAuthReader{delay: delay, closed: make(chan struct{})}
}

func (r *delayedMCPAuthReader) Read([]byte) (int, error) {
	timer := time.NewTimer(r.delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return 0, io.EOF
	case <-r.closed:
		return 0, io.ErrClosedPipe
	}
}

func (r *delayedMCPAuthReader) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

func TestReadManualMCPAuthInput(t *testing.T) {
	t.Parallel()

	t.Run("Should hide a full redirect URL read from a terminal", func(t *testing.T) {
		t.Parallel()

		terminalInput, terminalWriter, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		t.Cleanup(func() {
			if err := terminalInput.Close(); err != nil {
				t.Errorf("terminal input close error = %v", err)
			}
		})
		if err := terminalWriter.Close(); err != nil {
			t.Fatalf("terminal writer close error = %v", err)
		}

		const secretRedirect = "http://127.0.0.1:2123/api/mcp/oauth/callback?code=secret-code&state=secret-state"
		var output bytes.Buffer
		input, err := readManualMCPAuthInputWithTerminal(
			terminalInput,
			&output,
			func(inputReader io.Reader) bool {
				return inputReader == terminalInput
			},
			func(fd int) ([]byte, error) {
				if fd != int(terminalInput.Fd()) {
					t.Fatalf("terminal fd = %d, want %d", fd, terminalInput.Fd())
				}
				return []byte(secretRedirect), nil
			},
		)
		if err != nil {
			t.Fatalf("readManualMCPAuthInputWithTerminal() error = %v", err)
		}
		if input != secretRedirect {
			t.Fatalf("input = %q, want secret redirect", input)
		}
		if output.String() != "\n" {
			t.Fatalf("terminal output = %q, want only hidden-input newline", output.String())
		}
		if strings.Contains(output.String(), "secret-code") || strings.Contains(output.String(), "secret-state") {
			t.Fatalf("terminal output leaked OAuth material: %q", output.String())
		}
	})

	t.Run("Should preserve piped input without using terminal reads", func(t *testing.T) {
		t.Parallel()

		const code = "piped-one-time-code"
		var output bytes.Buffer
		input, err := readManualMCPAuthInputWithTerminal(
			strings.NewReader(code+"\n"),
			&output,
			func(io.Reader) bool { return false },
			func(int) ([]byte, error) {
				t.Fatal("terminal password reader called for piped input")
				return nil, nil
			},
		)
		if err != nil {
			t.Fatalf("readManualMCPAuthInputWithTerminal() error = %v", err)
		}
		if input != code+"\n" {
			t.Fatalf("input = %q, want piped code", input)
		}
		if output.Len() != 0 {
			t.Fatalf("piped output = %q, want empty", output.String())
		}
	})
}

func TestMCPAuthorizeWaitsForAChangedConfirmedCredential(t *testing.T) {
	t.Parallel()

	t.Run("Should wait for a changed confirmed credential", func(t *testing.T) {
		t.Parallel()

		oldUpdatedAt := time.Now().Add(-time.Hour).UTC()
		newUpdatedAt := oldUpdatedAt.Add(time.Minute)
		var listCalls atomic.Int32
		client := &stubClient{
			listSettingsMCPServersFn: func(
				context.Context,
				contract.SettingsWorkspaceScopeKind,
				string,
			) (contract.SettingsMCPServersResponse, error) {
				updatedAt := &oldUpdatedAt
				if listCalls.Add(1) >= 3 {
					updatedAt = &newUpdatedAt
				}
				return mcpAuthServerResponse(mcpAuthStatus("linear", "global", "", true, updatedAt)), nil
			},
			beginSettingsMCPAuthFn: func(
				_ context.Context,
				_ SettingsMCPAuthTarget,
				request SettingsMCPAuthBeginRequest,
			) (SettingsMCPAuthBeginRecord, error) {
				if request.Mode != contract.SettingsMCPAuthBeginModeAutomatic {
					t.Fatalf("begin mode = %q, want automatic", request.Mode)
				}
				return SettingsMCPAuthBeginRecord{
					AuthorizationURL: "https://auth.example/authorize",
					ExpiresAt:        time.Now().Add(time.Minute),
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"mcp", "authorize", "linear", "--timeout", "2s", "-o", "json",
		)
		if err != nil {
			t.Fatalf("execute mcp authorize error = %v", err)
		}
		if listCalls.Load() < 3 {
			t.Fatalf("ListSettingsMCPServers calls = %d, want baseline plus changed poll", listCalls.Load())
		}
		var status SettingsMCPAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &status); err != nil {
			t.Fatalf("json.Unmarshal(status) error = %v", err)
		}
		if status.UpdatedAt == nil || !status.UpdatedAt.Equal(newUpdatedAt) {
			t.Fatalf("status.UpdatedAt = %v, want %v", status.UpdatedAt, newUpdatedAt)
		}
	})
}

func TestMCPAuthStatusAndLogoutHonorWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should honor workspace identity for status and logout", func(t *testing.T) {
		t.Parallel()

		workspaceID := "workspace-a"
		wantStatus := mcpAuthStatus("linear", "workspace", workspaceID, true, timePointer(time.Now()))
		client := &stubClient{}
		client.listSettingsMCPServersFn = func(
			_ context.Context,
			scope contract.SettingsWorkspaceScopeKind,
			gotWorkspaceID string,
		) (contract.SettingsMCPServersResponse, error) {
			if scope != contract.SettingsWorkspaceScopeWorkspace || gotWorkspaceID != workspaceID {
				t.Fatalf("status scope/workspace = %q/%q", scope, gotWorkspaceID)
			}
			return mcpAuthServerResponse(wantStatus), nil
		}
		client.logoutSettingsMCPAuthFn = func(
			_ context.Context,
			target SettingsMCPAuthTarget,
		) (SettingsMCPAuthStatusRecord, error) {
			if target.Scope != contract.SettingsWorkspaceScopeWorkspace || target.WorkspaceID != workspaceID {
				t.Fatalf("logout target = %#v", target)
			}
			status := wantStatus
			status.Status = string(mcpauth.StatusNeedsLogin)
			status.TokenPresent = false
			return status, nil
		}
		deps := newTestDeps(t, client)

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"mcp", "auth", "status", "linear",
			"--scope", " workspace ", "--workspace", " "+workspaceID+" ", "-o", "json",
		)
		if err != nil {
			t.Fatalf("execute mcp auth status error = %v", err)
		}
		if strings.Contains(stdout, "access-token") || strings.Contains(stdout, "refresh-token") {
			t.Fatalf("status output leaked token material: %s", stdout)
		}
		var statuses []SettingsMCPAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &statuses); err != nil {
			t.Fatalf("json.Unmarshal(statuses) error = %v", err)
		}
		if len(statuses) != 1 || statuses[0].WorkspaceID != workspaceID {
			t.Fatalf("statuses = %#v", statuses)
		}

		if _, _, err := executeRootCommand(
			t,
			deps,
			"mcp", "auth", "logout", "linear",
			"--scope", "workspace", "--workspace", workspaceID,
		); err != nil {
			t.Fatalf("execute mcp auth logout error = %v", err)
		}
	})
}

func TestManualMCPAuthExchangeRequestClassifiesCodeAndRedirect(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		input        string
		wantCode     bool
		wantRedirect bool
	}{
		{name: "Should classify an opaque code", input: "opaque-code", wantCode: true},
		{
			name:         "Should classify an absolute redirect URL",
			input:        "https://callback.example/oauth?code=opaque&state=public",
			wantRedirect: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request, err := manualMCPAuthExchangeRequest(tc.input)
			if err != nil {
				t.Fatalf("manualMCPAuthExchangeRequest() error = %v", err)
			}
			if (request.Code != "") != tc.wantCode || (request.RedirectURL != "") != tc.wantRedirect {
				t.Fatalf("request = %#v", request)
			}
		})
	}
}

func TestMCPAuthStatusBundlesRenderScopeAndTokenPresence(t *testing.T) {
	t.Parallel()

	t.Run("Should render scope and token presence in status bundles", func(t *testing.T) {
		t.Parallel()

		status := mcpAuthStatus("linear", "workspace", "workspace-a", true, timePointer(fixedTestNow))
		human, err := mcpAuthStatusBundle(status).human()
		if err != nil {
			t.Fatalf("mcpAuthStatusBundle.human() error = %v", err)
		}
		if !strings.Contains(human, "workspace-a") || !strings.Contains(human, "Token Present") {
			t.Fatalf("human status = %q", human)
		}
		toon, err := mcpAuthStatusListBundle([]SettingsMCPAuthStatusRecord{status}).toon()
		if err != nil {
			t.Fatalf("mcpAuthStatusListBundle.toon() error = %v", err)
		}
		if !strings.Contains(toon, "mcp_auth[1]") || !strings.Contains(toon, "workspace") {
			t.Fatalf("toon status = %q", toon)
		}
	})
}

func mcpAuthStatus(
	serverName string,
	scope string,
	workspaceID string,
	tokenPresent bool,
	updatedAt *time.Time,
) SettingsMCPAuthStatusRecord {
	status := string(mcpauth.StatusNeedsLogin)
	if tokenPresent {
		status = string(mcpauth.StatusAuthenticated)
	}
	return SettingsMCPAuthStatusRecord{
		ServerName:   serverName,
		Scope:        scope,
		WorkspaceID:  workspaceID,
		Status:       status,
		ClientID:     "client-id",
		TokenPresent: tokenPresent,
		UpdatedAt:    updatedAt,
	}
}

func mcpAuthServerResponse(status SettingsMCPAuthStatusRecord) contract.SettingsMCPServersResponse {
	return contract.SettingsMCPServersResponse{MCPServers: []contract.SettingsMCPServerItemPayload{{
		Name:        status.ServerName,
		Scope:       contract.SettingsScopeKind(status.Scope),
		WorkspaceID: status.WorkspaceID,
		AuthStatus:  &status,
	}}}
}

func executeMCPAuthCommandWithInput(
	t *testing.T,
	deps commandDeps,
	stdin string,
	args ...string,
) (string, string, error) {
	t.Helper()

	cmd := newRootCommand(deps)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), stderr.String(), err
}
