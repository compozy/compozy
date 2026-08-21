package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

func TestMCPAuthCommandTreeHardCutsAuthorizeAlias(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve canonical login and reject the removed authorize alias", func(t *testing.T) {
		t.Parallel()

		root := newRootCommand(commandDeps{})
		login, remaining, err := root.Find([]string{"mcp", "auth", "login"})
		if err != nil {
			t.Fatalf("Find(mcp auth login) error = %v", err)
		}
		if len(remaining) != 0 {
			t.Fatalf("Find(mcp auth login) remaining = %v, want none", remaining)
		}
		if got := strings.TrimSpace(login.CommandPath()); got != "compozy mcp auth login" {
			t.Fatalf("mcp auth login path = %q, want canonical login path", got)
		}

		legacy, legacyRemaining, legacyErr := root.Find([]string{"mcp", "authorize"})
		if legacyErr == nil && len(legacyRemaining) == 0 &&
			strings.TrimSpace(legacy.CommandPath()) == "compozy mcp authorize" {
			t.Fatal("mcp authorize should not resolve after the hard cut")
		}
	})
}

func TestMCPAuthLoginUsesDaemonOwnedManualExchange(t *testing.T) {
	t.Parallel()

	t.Run("Should exchange a pasted code without exposing it", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		client.getSettingsMCPAuthStatusFn = func(
			context.Context,
			SettingsMCPAuthTarget,
		) (SettingsMCPAuthStatusRecord, error) {
			return mcpAuthStatus("linear", "user", "", false, nil), nil
		}
		client.beginSettingsMCPAuthFn = func(
			_ context.Context,
			target SettingsMCPAuthTarget,
			request SettingsMCPAuthBeginRequest,
		) (SettingsMCPAuthBeginRecord, error) {
			if target.Name != "linear" || target.Scope != contract.SettingsLayeredScopeUser {
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
			wantRedirectURL := "http://127.0.0.1:2123/api/mcp/oauth/callback" +
				"?code=one-time-code&state=public-state"
			if target.Name != "linear" || request.RedirectURL != wantRedirectURL {
				t.Fatalf("exchange target/request = %#v / %#v", target, request)
			}
			return mcpAuthStatus("linear", "user", "", true, timePointer(time.Now())), nil
		}

		stdout, stderr, err := executeMCPAuthCommandWithInput(
			t,
			newWorkspaceTestDeps(t, client),
			"http://127.0.0.1:2123/api/mcp/oauth/callback?code=one-time-code&state=public-state\n",
			"mcp", "auth", "login", "linear", "--manual", "-o", "json",
		)
		if err != nil {
			t.Fatalf("execute mcp auth login --manual error = %v", err)
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
			getSettingsMCPAuthStatusFn: func(
				context.Context,
				SettingsMCPAuthTarget,
			) (SettingsMCPAuthStatusRecord, error) {
				return mcpAuthStatus("linear", "user", "", false, nil), nil
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
				status := mcpAuthStatus("linear", "user", "", false, nil)
				status.Status = string(mcpauth.StatusAuthenticated)
				return status, nil
			},
		}

		_, _, err := executeMCPAuthCommandWithInput(
			t,
			newWorkspaceTestDeps(t, client),
			"http://127.0.0.1:2123/api/mcp/oauth/callback?code=one-time-code&state=public-state\n",
			"mcp", "auth", "login", "linear", "--manual",
		)
		if err == nil || !strings.Contains(err.Error(), "confirmed credential") {
			t.Fatalf("mcp auth login error = %v, want confirmed-credential failure", err)
		}
	})

	t.Run("Should distinguish an existing server without OAuth from a missing server", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			getSettingsMCPAuthStatusFn: func(
				context.Context,
				SettingsMCPAuthTarget,
			) (SettingsMCPAuthStatusRecord, error) {
				return SettingsMCPAuthStatusRecord{
					ServerName: "filesystem", Scope: "user", Status: string(mcpauth.StatusUnconfigured),
				}, nil
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newWorkspaceTestDeps(t, client),
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
			newWorkspaceTestDeps(t, client),
			"mcp", "auth", "login", "filesystem",
		)
		if err == nil || !strings.Contains(err.Error(), "does not configure OAuth") {
			t.Fatalf("mcp auth login error = %v, want OAuth configuration error", err)
		}
	})
}

func TestMCPAuthLoginManualHonorsTimeout(t *testing.T) {
	t.Parallel()

	newClient := func(exchange func(context.Context) error) *stubClient {
		return &stubClient{
			getSettingsMCPAuthStatusFn: func(
				context.Context,
				SettingsMCPAuthTarget,
			) (SettingsMCPAuthStatusRecord, error) {
				return mcpAuthStatus("linear", "user", "", false, nil), nil
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

		input, writer := newInheritedMCPAuthInput(t)
		assertMCPAuthInputBlocking(t, input, writer)
		client := newClient(nil)
		cmd := newRootCommand(newWorkspaceTestDeps(t, client))
		cmd.SetIn(input)
		cmd.SetArgs([]string{"mcp", "auth", "login", "linear", "--manual", "--timeout", "20ms"})
		err := cmd.ExecuteContext(t.Context())
		const expectedError = "cli: MCP authorization timed out: context deadline exceeded"
		if err == nil || err.Error() != expectedError {
			t.Fatalf("mcp auth login error = %v, want %q", err, expectedError)
		}
		if _, err := input.Stat(); err != nil {
			t.Fatalf("borrowed manual input was closed: %v", err)
		}
		assertMCPAuthInputBlocking(t, input, writer)
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
			newWorkspaceTestDeps(t, client),
			"http://127.0.0.1:2123/api/mcp/oauth/callback?code=one-time-code&state=public-state\n",
			"mcp", "auth", "login", "linear", "--manual", "--timeout", "20ms",
		)
		if err == nil || !errors.Is(err, context.DeadlineExceeded) ||
			!strings.Contains(err.Error(), "authorization timed out") {
			t.Fatalf("mcp auth login error = %v, want stable authorization timeout", err)
		}
	})
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

func TestReadManualMCPAuthInputContext(t *testing.T) {
	t.Parallel()

	t.Run("Should keep terminal input open until the hidden read returns", func(t *testing.T) {
		t.Parallel()

		terminalInput, terminalWriter, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		t.Cleanup(func() {
			if err := terminalInput.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Errorf("terminal input close error = %v", err)
			}
		})
		t.Cleanup(func() {
			if err := terminalWriter.Close(); err != nil {
				t.Errorf("terminal writer close error = %v", err)
			}
		})

		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		t.Cleanup(cancel)
		passwordReadStarted := make(chan struct{})
		allowPasswordReadReturn := make(chan struct{})
		var releasePasswordRead sync.Once
		t.Cleanup(func() { releasePasswordRead.Do(func() { close(allowPasswordReadReturn) }) })
		results := make(chan error, 1)
		go func() {
			_, readErr := readManualMCPAuthInputContextWithTerminal(
				ctx,
				terminalInput,
				io.Discard,
				func(io.Reader) bool { return true },
				func(fd int) ([]byte, error) {
					if fd != int(terminalInput.Fd()) {
						return nil, fmt.Errorf("terminal fd = %d, want %d", fd, terminalInput.Fd())
					}
					close(passwordReadStarted)
					<-allowPasswordReadReturn
					return []byte("http://127.0.0.1:2123/api/mcp/oauth/callback?code=code&state=state"), nil
				},
			)
			results <- readErr
		}()

		select {
		case <-passwordReadStarted:
		case <-t.Context().Done():
			t.Fatalf("hidden password read did not start: %v", t.Context().Err())
		}
		<-ctx.Done()
		returnTimer := time.NewTimer(100 * time.Millisecond)
		defer returnTimer.Stop()
		select {
		case readErr := <-results:
			t.Fatalf("readManualMCPAuthInputContextWithTerminal() returned before password read finished: %v", readErr)
		case <-returnTimer.C:
		}
		if _, err := terminalInput.Stat(); err != nil {
			t.Fatalf("terminal input was closed before password read returned: %v", err)
		}

		releasePasswordRead.Do(func() { close(allowPasswordReadReturn) })
		if readErr := <-results; !errors.Is(readErr, context.DeadlineExceeded) ||
			!strings.Contains(readErr.Error(), "authorization timed out") {
			t.Fatalf("readManualMCPAuthInputContextWithTerminal() error = %v, want authorization timeout", readErr)
		}
	})
}

func TestReadManualMCPAuthFileContext(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a non-terminal file open after cancellation", func(t *testing.T) {
		t.Parallel()

		input, writer, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := input.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
				t.Errorf("manual input close error = %v", closeErr)
			}
		})
		t.Cleanup(func() {
			if closeErr := writer.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
				t.Errorf("manual input writer close error = %v", closeErr)
			}
		})

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		readStarted := make(chan struct{})
		results := make(chan error, 1)
		go func() {
			_, readErr := readManualMCPAuthFileContext(ctx, input, func(reader io.Reader) (string, error) {
				close(readStarted)
				buffer := make([]byte, 1)
				_, inputErr := reader.Read(buffer)
				return string(buffer), inputErr
			})
			results <- readErr
		}()

		select {
		case <-readStarted:
		case <-t.Context().Done():
			t.Fatalf("manual file input did not start: %v", t.Context().Err())
		}
		cancel()

		readErr := <-results
		if !errors.Is(readErr, context.Canceled) {
			t.Fatalf("readManualMCPAuthFileContext() error = %v, want stable cancellation timeout", readErr)
		}
		if _, err := input.Stat(); err != nil {
			t.Fatalf("manual input was closed by cancellation: %v", err)
		}

		if _, err := writer.WriteString("r"); err != nil {
			t.Fatalf("write to input after cancellation error = %v", err)
		}
		buffer := make([]byte, 1)
		if _, err := io.ReadFull(input, buffer); err != nil {
			t.Fatalf("read from input after cancellation error = %v", err)
		}
		if got := string(buffer); got != "r" {
			t.Fatalf("input after cancellation = %q, want %q", got, "r")
		}
	})

	t.Run("Should translate a file deadline and clear it before returning", func(t *testing.T) {
		t.Parallel()

		input, writer, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := input.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
				t.Errorf("manual input close error = %v", closeErr)
			}
		})
		t.Cleanup(func() {
			if closeErr := writer.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
				t.Errorf("manual input writer close error = %v", closeErr)
			}
		})

		_, err = readManualMCPAuthFileContext(t.Context(), input, func(reader io.Reader) (string, error) {
			if deadlineErr := input.SetReadDeadline(time.Now().Add(-time.Second)); deadlineErr != nil {
				return "", fmt.Errorf("set file deadline: %w", deadlineErr)
			}
			buffer := make([]byte, 1)
			_, inputErr := reader.Read(buffer)
			return string(buffer), inputErr
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("readManualMCPAuthFileContext() error = %v, want stable deadline timeout", err)
		}

		if _, err := writer.WriteString("r"); err != nil {
			t.Fatalf("write to input after deadline error = %v", err)
		}
		buffer := make([]byte, 1)
		if _, err := io.ReadFull(input, buffer); err != nil {
			t.Fatalf("read from input after deadline error = %v", err)
		}
		if got := string(buffer); got != "r" {
			t.Fatalf("input after deadline = %q, want %q", got, "r")
		}
	})
}

func TestMCPAuthLoginWaitsForAChangedConfirmedCredential(t *testing.T) {
	t.Parallel()

	t.Run("Should wait for a changed confirmed credential", func(t *testing.T) {
		t.Parallel()

		oldUpdatedAt := time.Now().Add(-time.Hour).UTC()
		newUpdatedAt := oldUpdatedAt.Add(time.Minute)
		var statusCalls atomic.Int32
		client := &stubClient{
			getSettingsMCPAuthStatusFn: func(
				context.Context,
				SettingsMCPAuthTarget,
			) (SettingsMCPAuthStatusRecord, error) {
				updatedAt := &oldUpdatedAt
				if statusCalls.Add(1) >= 3 {
					updatedAt = &newUpdatedAt
				}
				return mcpAuthStatus("linear", "user", "", true, updatedAt), nil
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
			newWorkspaceTestDeps(t, client),
			"mcp", "auth", "login", "linear", "--timeout", "2s", "-o", "json",
		)
		if err != nil {
			t.Fatalf("execute mcp auth login error = %v", err)
		}
		if statusCalls.Load() < 3 {
			t.Fatalf("GetSettingsMCPAuthStatus calls = %d, want baseline plus changed poll", statusCalls.Load())
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
		client.getSettingsMCPAuthStatusFn = func(
			_ context.Context,
			target SettingsMCPAuthTarget,
		) (SettingsMCPAuthStatusRecord, error) {
			if target.Scope != contract.SettingsLayeredScopeWorkspace || target.WorkspaceID != workspaceID {
				t.Fatalf("status target = %#v", target)
			}
			return wantStatus, nil
		}
		client.logoutSettingsMCPAuthFn = func(
			_ context.Context,
			target SettingsMCPAuthTarget,
		) (SettingsMCPAuthStatusRecord, error) {
			if target.Scope != contract.SettingsLayeredScopeWorkspace || target.WorkspaceID != workspaceID {
				t.Fatalf("logout target = %#v", target)
			}
			status := wantStatus
			status.Status = string(mcpauth.StatusNeedsLogin)
			status.TokenPresent = false
			return status, nil
		}
		deps := newDefaultProfileWorkspaceTestDeps(t, client)

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
		var fields []map[string]json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &fields); err != nil {
			t.Fatalf("json.Unmarshal(status fields) error = %v", err)
		}
		if len(fields) != 1 {
			t.Fatalf("status field records = %d, want 1", len(fields))
		}
		if _, found := fields[0]["resolution_source"]; found {
			t.Fatalf("mcp auth status JSON contains resolution_source, want raw daemon payload: %s", stdout)
		}
		var statuses []SettingsMCPAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &statuses); err != nil {
			t.Fatalf("json.Unmarshal(statuses) error = %v", err)
		}
		if len(statuses) != 1 || statuses[0].WorkspaceID != workspaceID {
			t.Fatalf("statuses = %#v", statuses)
		}

		logout, _, err := executeRootCommand(
			t,
			deps,
			"mcp", "auth", "logout", "linear",
			"--scope", "workspace", "--workspace", workspaceID, "-o", "json",
		)
		if err != nil {
			t.Fatalf("execute mcp auth logout error = %v", err)
		}
		var logoutFields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(logout), &logoutFields); err != nil {
			t.Fatalf("json.Unmarshal(logout fields) error = %v", err)
		}
		if _, found := logoutFields["resolution_source"]; found {
			t.Fatalf("mcp auth logout JSON contains resolution_source, want raw daemon payload: %s", logout)
		}
	})

	t.Run("Should honor the active profile for profile auth", func(t *testing.T) {
		t.Parallel()

		client := &profileAwareStubClient{
			stubClient: withWorkspaceResolution(&stubClient{getSettingsMCPAuthStatusFn: func(
				_ context.Context,
				target SettingsMCPAuthTarget,
			) (SettingsMCPAuthStatusRecord, error) {
				if target.Scope != contract.SettingsLayeredScopeProfile || target.Profile != "marketing" ||
					target.WorkspaceID != "" {
					t.Fatalf("profile auth target = %#v", target)
				}
				return mcpAuthStatus("linear", "profile", "", true, timePointer(time.Now())), nil
			}}),
			profileClientStub: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
				{Name: "marketing", State: "active"},
			}},
		}
		_, _, err := executeRootCommand(
			t, newTestDeps(t, client),
			"--profile", "marketing", "mcp", "auth", "status", "linear", "--scope", "profile", "-o", "json",
		)
		if err != nil {
			t.Fatalf("profile MCP auth status error = %v", err)
		}
	})

	t.Run("Should combine workspace and active profile for workspace-profile auth", func(t *testing.T) {
		t.Parallel()

		client := &profileAwareStubClient{
			stubClient: withWorkspaceResolution(&stubClient{getSettingsMCPAuthStatusFn: func(
				_ context.Context,
				target SettingsMCPAuthTarget,
			) (SettingsMCPAuthStatusRecord, error) {
				if target.Scope != contract.SettingsLayeredScopeProfile || target.Profile != "marketing" ||
					target.WorkspaceID != "workspace-a" {
					t.Fatalf("workspace-profile auth target = %#v", target)
				}
				return mcpAuthStatus(
					"linear", string(mcpauth.ScopeWorkspaceProfile), "workspace-a@pf:marketing", true,
					timePointer(time.Now()),
				), nil
			}}),
			profileClientStub: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
				{Name: "marketing", State: "active"},
			}},
		}
		_, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"--profile", "marketing",
			"mcp", "auth", "status", "linear",
			"--scope", "profile",
			"--workspace", "workspace-a",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("workspace-profile MCP auth status error = %v", err)
		}
	})

	t.Run("Should keep an unqualified status list inside one profile", func(t *testing.T) {
		t.Parallel()

		marketingStatus := mcpAuthStatus("linear", "profile", "", true, timePointer(time.Now()))
		marketingStatus.Profile = "marketing"
		salesStatus := mcpAuthStatus("github", "profile", "", true, timePointer(time.Now()))
		salesStatus.Profile = "sales"
		servers := []contract.SettingsMCPServerItemPayload{
			{
				Name: "linear", Scope: contract.SettingsScopeProfile, Profile: "marketing",
				AuthStatus: &marketingStatus,
			},
			{
				Name: "github", Scope: contract.SettingsScopeProfile, Profile: "sales",
				AuthStatus: &salesStatus,
			},
		}
		statuses := mcpAuthStatuses(servers, mcpAuthCommandOptions{
			scope: "profile", profile: "marketing",
		})
		if len(statuses) != 1 || statuses[0].Profile != "marketing" {
			t.Fatalf("profile MCP auth statuses = %#v", statuses)
		}
	})
}

func TestManualMCPAuthExchangeRequestRequiresFullRedirectURL(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		input        string
		wantRedirect string
		wantErr      bool
	}{
		{
			name:         "Should classify an absolute redirect URL",
			input:        "https://callback.example/oauth?code=opaque&state=public",
			wantRedirect: "https://callback.example/oauth?code=opaque&state=public",
		},
		{
			name:    "Should reject an opaque authorization code",
			input:   "opaque-authorization-code",
			wantErr: true,
		},
		{
			name:    "Should reject a non-absolute callback path",
			input:   "/oauth/callback?code=opaque&state=public",
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request, err := manualMCPAuthExchangeRequest(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("manualMCPAuthExchangeRequest() error = nil, want rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("manualMCPAuthExchangeRequest() error = %v", err)
			}
			if request.RedirectURL != tc.wantRedirect {
				t.Fatalf("request.RedirectURL = %q, want %q", request.RedirectURL, tc.wantRedirect)
			}
		})
	}
}

func TestMCPAuthLoginRequiresExplicitScopeEscalationApproval(t *testing.T) {
	t.Parallel()

	t.Run("Should require explicit scope escalation approval", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(
			t,
			newWorkspaceTestDeps(t, &stubClient{}),
			"mcp",
			"auth",
			"login",
			"linear",
			"--approved-scope",
			"tools.write",
		)
		if err == nil || !strings.Contains(err.Error(), "--approve-scope-escalation") {
			t.Fatalf("mcp auth login error = %v, want explicit scope escalation approval", err)
		}
	})

	t.Run("Should forward explicitly approved scopes", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			getSettingsMCPAuthStatusFn: func(
				context.Context,
				SettingsMCPAuthTarget,
			) (SettingsMCPAuthStatusRecord, error) {
				return mcpAuthStatus("linear", "user", "", false, nil), nil
			},
			beginSettingsMCPAuthFn: func(
				_ context.Context,
				_ SettingsMCPAuthTarget,
				request SettingsMCPAuthBeginRequest,
			) (SettingsMCPAuthBeginRecord, error) {
				if strings.Join(request.ApprovedScopes, ",") != "tools.write,resources.read" {
					t.Fatalf("begin approved scopes = %v", request.ApprovedScopes)
				}
				if !request.ApproveScopeEscalation {
					t.Fatal("begin scope escalation approval = false, want true")
				}
				return SettingsMCPAuthBeginRecord{
					AuthorizationURL: "https://auth.example/authorize",
					ExpiresAt:        time.Now().Add(time.Minute),
					ManualSupported:  true,
				}, nil
			},
			exchangeSettingsMCPAuthFn: func(
				context.Context,
				SettingsMCPAuthTarget,
				SettingsMCPAuthExchangeRequest,
			) (SettingsMCPAuthStatusRecord, error) {
				return mcpAuthStatus("linear", "user", "", true, timePointer(time.Now())), nil
			},
		}
		_, _, err := executeMCPAuthCommandWithInput(
			t,
			newWorkspaceTestDeps(t, client),
			"http://127.0.0.1:2123/api/mcp/oauth/callback?code=one-time-code&state=public-state\n",
			"mcp", "auth", "login", "linear", "--manual",
			"--approved-scope", "tools.write",
			"--approved-scope", "resources.read",
			"--approve-scope-escalation",
		)
		if err != nil {
			t.Fatalf("execute mcp auth login with approved scopes error = %v", err)
		}
	})
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
