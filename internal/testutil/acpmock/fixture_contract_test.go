package acpmock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kballard/go-shellquote"
)

func TestAcpmockBehaviorContracts(t *testing.T) {
	// not parallel: this suite mutates process environment and the package-level driver binary cache.
	t.Run("Should reject fixture bytes containing multiple JSON documents", func(t *testing.T) {
		valid := validContractFixtureJSON("claude")
		cases := []struct {
			name string
			raw  string
		}{
			{
				name: "Should reject a repeated fixture document",
				raw:  valid + valid,
			},
			{
				name: "Should reject trailing scalar JSON document",
				raw:  valid + "\n42",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := ParseFixture([]byte(tc.raw)); err == nil ||
					!strings.Contains(err.Error(), "exactly one document") {
					t.Fatalf("ParseFixture(%s) error = %v, want exact-one-document error", tc.name, err)
				}
			})
		}
	})

	t.Run("Should public driver helpers honor environment override before cache or build", func(t *testing.T) {
		overridePath := createExecutableFile(t, filepath.Join(t.TempDir(), driverBinaryName()))
		cachedPath := createExecutableFile(t, filepath.Join(t.TempDir(), driverBinaryName()))
		restoreDriverBinaryCache(t, cachedPath)
		t.Setenv(driverBinaryEnvVar, overridePath)
		t.Setenv("PATH", t.TempDir())

		defaultPath, err := DefaultDriverPath()
		if err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", err)
		}
		if defaultPath != overridePath {
			t.Fatalf("DefaultDriverPath() = %q, want env override %q", defaultPath, overridePath)
		}
		if got := RequireDriver(t); got != overridePath {
			t.Fatalf("RequireDriver() = %q, want env override %q", got, overridePath)
		}
	})

	t.Run("Should register stores absolute override paths in generated command", func(t *testing.T) {
		homePaths := mockHomePaths(t)
		driverPath := createExecutableFile(t, filepath.Join(t.TempDir(), driverBinaryName()))
		diagnosticsPath := filepath.Join(t.TempDir(), "diag", "mock.jsonl")
		relativeDriver := relativeToWorkingDir(t, driverPath)
		relativeDiagnostics := relativeToWorkingDir(t, diagnosticsPath)

		registration, err := Register(homePaths, RegisterOptions{
			FixturePath:     filepath.Join("testdata", "multi_agent_fixture.json"),
			FixtureAgent:    "alpha",
			AgentName:       "relative-overrides",
			DriverPath:      relativeDriver,
			DiagnosticsPath: relativeDiagnostics,
		})
		if err != nil {
			t.Fatalf("Register(relative overrides) error = %v", err)
		}
		if got, want := registration.DriverPath, driverPath; got != want {
			t.Fatalf("registration.DriverPath = %q, want %q", got, want)
		}
		if got, want := registration.DiagnosticsPath, diagnosticsPath; got != want {
			t.Fatalf("registration.DiagnosticsPath = %q, want %q", got, want)
		}
		argv, err := shellquote.Split(registration.Command)
		if err != nil {
			t.Fatalf("shellquote.Split(%q) error = %v", registration.Command, err)
		}
		if got, want := argv[0], driverPath; got != want {
			t.Fatalf("command driver path = %q, want %q", got, want)
		}
		if got, want := commandFlagValue(t, argv, "--diagnostics"), diagnosticsPath; got != want {
			t.Fatalf("command diagnostics path = %q, want %q", got, want)
		}
	})

	t.Run("Should remove generated agent definition when post-write validation fails", func(t *testing.T) {
		homePaths := mockHomePaths(t)
		driverPath := createExecutableFile(t, filepath.Join(t.TempDir(), driverBinaryName()))
		fixturePath := filepath.Join(t.TempDir(), "fixture.json")
		if err := os.WriteFile(fixturePath, []byte(validContractFixtureJSON("missing-provider")), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", fixturePath, err)
		}

		_, err := Register(homePaths, RegisterOptions{
			FixturePath:  fixturePath,
			FixtureAgent: "alpha",
			AgentName:    "cleanup-after-failure",
			DriverPath:   driverPath,
		})
		if err == nil || !strings.Contains(err.Error(), "resolve written agent definition") {
			t.Fatalf("Register(unknown provider) error = %v, want post-write resolve error", err)
		}
		agentDefPath := filepath.Join(homePaths.AgentsDir, "cleanup-after-failure", "AGENT.md")
		if _, statErr := os.Stat(agentDefPath); statErr == nil || !os.IsNotExist(statErr) {
			t.Fatalf("os.Stat(%q) error = %v, want not exist", agentDefPath, statErr)
		}
	})
}

func validContractFixtureJSON(provider string) string {
	return `{"version":2,"agents":[{"name":"alpha","provider":"` + provider + `","turns":[{"match":{"turn_source":"user","user_text":"hi"},"steps":[{"kind":"assistant","text":"hi"}]}]}]}`
}

func restoreDriverBinaryCache(t testing.TB, path string) {
	t.Helper()

	driverBinaryMu.Lock()
	previous := driverBinaryPath
	driverBinaryPath = path
	driverBinaryMu.Unlock()

	t.Cleanup(func() {
		driverBinaryMu.Lock()
		driverBinaryPath = previous
		driverBinaryMu.Unlock()
	})
}

func createExecutableFile(t testing.TB, path string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	clean, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("filepath.Abs(%q) error = %v", path, err)
	}
	return clean
}

func relativeToWorkingDir(t testing.TB, target string) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	relative, err := filepath.Rel(wd, target)
	if err != nil {
		t.Fatalf("filepath.Rel(%q, %q) error = %v", wd, target, err)
	}
	if filepath.IsAbs(relative) {
		t.Fatalf("filepath.Rel(%q, %q) = %q, want relative path", wd, target, relative)
	}
	return relative
}

func commandFlagValue(t testing.TB, argv []string, flag string) string {
	t.Helper()

	for idx := 0; idx < len(argv)-1; idx++ {
		if argv[idx] == flag {
			return argv[idx+1]
		}
	}
	t.Fatalf("command argv %#v missing flag %q", argv, flag)
	return ""
}
