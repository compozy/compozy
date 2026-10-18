package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	authproviders "github.com/compozy/compozy/internal/providers"
)

func agyTestResolver(t *testing.T) func(context.Context, string, []string, string) (string, error) {
	t.Helper()

	return func(_ context.Context, command string, _ []string, _ string) (string, error) {
		if command != "agy_acp_server.par" {
			t.Fatalf("resolve command = %q, want agy_acp_server.par", command)
		}
		return "/test/bin/agy_acp_server.par", nil
	}
}

// Invariant: cheap agy reads report presence without spawning the server.
// Owner: CLI provider surfaces (IT-004).
func TestProviderAGYCheapReads(t *testing.T) {
	t.Parallel()

	t.Run("Should inspect builtin agy without launching a probe", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &profileAwareStubClient{
			stubClient: withWorkspaceResolution(&stubClient{}),
			profileClientStub: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
			}},
		})
		deps.resolveProviderAuthCommand = agyTestResolver(t)
		deps.runACPProbe = func(context.Context, acp.SessionInspectionRequest) error {
			t.Fatal("runACPProbe called during cheap inspect")
			return nil
		}
		deps.runProviderAuthCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthCommand(%q) called during cheap inspect", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "inspect", "agy", "-o", "json")
		if err != nil {
			t.Fatalf("provider inspect agy error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider inspect) error = %v", err)
		}
		if got, want := record.Provider, "agy"; got != want {
			t.Fatalf("Provider = %q, want %q", got, want)
		}
		if got, want := record.State, "unknown"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if !record.Login.Configured || record.Login.Source != "auth_acp_method" ||
			record.Login.Executable != "agy_acp_server.par" ||
			record.Login.Presence != authproviders.ProviderLoginPresencePresent {
			t.Fatalf("Login = %#v, want present auth_acp_method descriptor", record.Login)
		}
		if record.Probe != nil {
			t.Fatalf("Probe = %#v, want no probe on inspect", record.Probe)
		}
	})

	t.Run("Should keep no-probe status free of ACP traffic", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.resolveProviderAuthCommand = agyTestResolver(t)
		deps.runACPProbe = func(context.Context, acp.SessionInspectionRequest) error {
			t.Fatal("runACPProbe called with --no-probe")
			return nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "agy", "--no-probe", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status --no-probe error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "unknown"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if record.Probe != nil {
			t.Fatalf("Probe = %#v, want no probe with --no-probe", record.Probe)
		}
	})
}

// Invariant: local agy login and status run the shared ACP operations with
// JSON stdout and stderr-only progress semantics. Owner: CLI (IT-005).
func TestProviderAGYAuthOperations(t *testing.T) {
	t.Parallel()

	t.Run("Should probe agy readiness without a probe object", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.resolveProviderAuthCommand = agyTestResolver(t)
		deps.runACPProbe = func(_ context.Context, req acp.SessionInspectionRequest) error {
			if !strings.HasPrefix(strings.TrimSpace(req.Command), "agy_acp_server.par") {
				t.Fatalf("Command = %q, want builtin server command", req.Command)
			}
			return nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "agy", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status agy error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "authenticated"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if record.Probe != nil {
			t.Fatalf("Probe = %#v, want omitted probe for ACP operations", record.Probe)
		}
		if !record.Login.Configured || record.Login.Source != "auth_acp_method" {
			t.Fatalf("Login = %#v, want auth_acp_method source", record.Login)
		}
		if strings.Contains(stdout, "/test/bin/agy_acp_server.par") {
			t.Fatalf("stdout leaked resolved server path: %s", stdout)
		}
	})

	t.Run("Should classify an ACP auth refusal as needs login", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.resolveProviderAuthCommand = agyTestResolver(t)
		deps.runACPProbe = func(context.Context, acp.SessionInspectionRequest) error {
			return errors.New("ACP session/new failed: not authenticated")
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "agy", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status agy error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "needs_login"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if got, want := record.Code, contract.CodeProviderNotAuthenticated; got != want {
			t.Fatalf("Code = %q, want %q", got, want)
		}
		if record.Probe != nil {
			t.Fatalf("Probe = %#v, want omitted probe for ACP operations", record.Probe)
		}
	})

	t.Run("Should log in through the configured method with stderr progress", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.resolveProviderAuthCommand = agyTestResolver(t)
		var stderr bytes.Buffer
		deps.acpLoginStderr = &stderr
		deps.runACPLogin = func(_ context.Context, req acp.AuthenticationRequest) error {
			if got, want := req.MethodID, "oauth-personal"; got != want {
				t.Fatalf("MethodID = %q, want %q", got, want)
			}
			if !strings.HasPrefix(strings.TrimSpace(req.Command), "agy_acp_server.par") {
				t.Fatalf("Command = %q, want builtin server command", req.Command)
			}
			if req.InteractiveOutput == nil {
				t.Fatal("InteractiveOutput = nil, want operator stderr sink")
			}
			if _, err := req.InteractiveOutput.Write([]byte("sign in at example.test\n")); err != nil {
				t.Fatalf("InteractiveOutput.Write() error = %v", err)
			}
			return nil
		}
		deps.runACPProbe = func(context.Context, acp.SessionInspectionRequest) error { return nil }
		deps.runProviderAuthLoginCommand = func(
			_ context.Context,
			spec providerAuthCommandSpec,
		) (providerAuthCommandResult, error) {
			t.Fatalf("runProviderAuthLoginCommand(%q) called, want ACP login", spec.Command)
			return providerAuthCommandResult{}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "agy", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth login agy error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth login) error = %v", err)
		}
		if got, want := record.State, "authenticated"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if strings.TrimSpace(stderr.String()) == "" {
			t.Fatal("stderr empty, want live operator progress")
		}
		if strings.Contains(stdout, "example.test") {
			t.Fatalf("stdout carries operator progress: %s", stdout)
		}
	})

	t.Run("Should refuse login when the server executable is missing", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.resolveProviderAuthCommand = func(context.Context, string, []string, string) (string, error) {
			return "", exec.ErrNotFound
		}
		deps.runACPLogin = func(context.Context, acp.AuthenticationRequest) error {
			t.Fatal("runACPLogin called with missing executable")
			return nil
		}

		_, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "agy")
		if err == nil {
			t.Fatal("provider auth login agy error = nil, want missing CLI error")
		}
		if !strings.Contains(err.Error(), `native CLI "agy_acp_server.par" was not found on PATH`) {
			t.Fatalf("error = %v, want missing server guidance", err)
		}
	})

	t.Run("Should name the method when it is not advertised", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.resolveProviderAuthCommand = agyTestResolver(t)
		deps.runACPLogin = func(context.Context, acp.AuthenticationRequest) error {
			return &acp.AuthMethodUnavailableError{MethodID: "oauth-personal", Reason: acp.AuthMethodUnavailableAbsent}
		}

		_, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "agy", "-o", "json")
		if err == nil {
			t.Fatal("provider auth login agy error = nil, want method error")
		}
		if !strings.Contains(
			err.Error(),
			`provider auth: ACP method "oauth-personal" is not advertised as an agent method`,
		) {
			t.Fatalf("error = %v, want stable method message", err)
		}
	})

	t.Run("Should report a supplied login timeout", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, nil)
		deps.resolveProviderAuthCommand = agyTestResolver(t)
		deps.runACPLogin = func(context.Context, acp.AuthenticationRequest) error {
			return context.DeadlineExceeded
		}

		_, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "agy", "--timeout", "1s", "-o", "json")
		if err == nil {
			t.Fatal("provider auth login agy error = nil, want timeout error")
		}
		if !strings.Contains(err.Error(), "provider auth: ACP login timed out") {
			t.Fatalf("error = %v, want stable timeout message", err)
		}
	})
}

// Invariant: production ACP probe/login requests carry the resolved working
// directory, so ACP base validation reaches the server instead of rejecting
// with cwd-required before launch. Owner: CLI provider surfaces (IT-005, D-001).
func TestProviderAGYProbeCarriesWorkingDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should populate the probe env working directory", func(t *testing.T) {
		t.Parallel()

		workdir := t.TempDir()
		deps := newTestDeps(t, nil)
		deps.getwd = func() (string, error) { return workdir, nil }
		deps.resolveProviderAuthCommand = agyTestResolver(t)
		runtime, err := providerAuthRuntime(deps)
		if err != nil {
			t.Fatalf("providerAuthRuntime() error = %v", err)
		}
		providerName, provider, err := resolveProviderAuthTarget(&runtime.Config, "agy")
		if err != nil {
			t.Fatalf("resolveProviderAuthTarget(agy) error = %v", err)
		}
		probeEnv, err := providerAuthProbeEnv(runtime.HomePaths, providerName, provider, deps)
		if err != nil {
			t.Fatalf("providerAuthProbeEnv() error = %v", err)
		}
		if got := probeEnv.CommandDir; got != workdir {
			t.Fatalf("CommandDir = %q, want %q", got, workdir)
		}
	})

	t.Run("Should send a launchable probe request to the ACP probe", func(t *testing.T) {
		t.Parallel()

		workdir := t.TempDir()
		deps := newTestDeps(t, nil)
		deps.getwd = func() (string, error) { return workdir, nil }
		deps.resolveProviderAuthCommand = agyTestResolver(t)
		var seen acp.SessionInspectionRequest
		deps.runACPProbe = func(_ context.Context, req acp.SessionInspectionRequest) error {
			seen = req
			return nil
		}

		stdout, _, err := executeRootCommand(t, deps, "provider", "auth", "status", "agy", "-o", "json")
		if err != nil {
			t.Fatalf("provider auth status agy error = %v", err)
		}

		var record providerAuthStatusRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("json.Unmarshal(provider auth status) error = %v", err)
		}
		if got, want := record.State, "authenticated"; got != want {
			t.Fatalf("State = %q, want %q", got, want)
		}
		if got := strings.TrimSpace(seen.Cwd); got != workdir {
			t.Fatalf("probe Cwd = %q, want %q", got, workdir)
		}
		// The production-built request must pass the real ACP base
		// validation that rejected D-001 before launch.
		if err := (acp.StartOpts{
			AgentName: seen.AgentName,
			Command:   seen.Command,
			Cwd:       seen.Cwd,
			Env:       seen.Env,
		}).Validate(); err != nil {
			t.Fatalf("production probe request Validate() error = %v", err)
		}
		// Past validation the real probe entry reaches launch: with an
		// unresolvable command it must fail at startup, never with
		// cwd-required.
		probeErr := acp.ProbeAuthentication(context.Background(), acp.SessionInspectionRequest{
			AgentName: seen.AgentName,
			Command:   "compozy-test-missing-probe-binary",
			Cwd:       seen.Cwd,
			Env:       seen.Env,
		})
		if probeErr == nil {
			t.Fatal("ProbeAuthentication() error = nil, want startup failure")
		}
		if strings.Contains(probeErr.Error(), "cwd is required") {
			t.Fatalf("ProbeAuthentication() error = %v, want launch failure past cwd validation", probeErr)
		}
	})

	t.Run("Should send a launchable login request to the ACP login", func(t *testing.T) {
		t.Parallel()

		workdir := t.TempDir()
		deps := newTestDeps(t, nil)
		deps.getwd = func() (string, error) { return workdir, nil }
		deps.resolveProviderAuthCommand = agyTestResolver(t)
		var seenLogin acp.AuthenticationRequest
		deps.runACPLogin = func(_ context.Context, req acp.AuthenticationRequest) error {
			seenLogin = req
			return nil
		}
		deps.runACPProbe = func(context.Context, acp.SessionInspectionRequest) error { return nil }

		if _, _, err := executeRootCommand(t, deps, "provider", "auth", "login", "agy", "-o", "json"); err != nil {
			t.Fatalf("provider auth login agy error = %v", err)
		}
		if got := strings.TrimSpace(seenLogin.Cwd); got != workdir {
			t.Fatalf("login Cwd = %q, want %q", got, workdir)
		}
		if info, err := os.Stat(seenLogin.Cwd); err != nil || !info.IsDir() {
			t.Fatalf("login Cwd = %q, want an existing directory (%v)", seenLogin.Cwd, err)
		}
		if err := (acp.StartOpts{
			AgentName: seenLogin.AgentName,
			Command:   seenLogin.Command,
			Cwd:       seenLogin.Cwd,
			Env:       seenLogin.Env,
		}).Validate(); err != nil {
			t.Fatalf("production login request Validate() error = %v", err)
		}
	})
}
