//go:build integration

package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadWorkspaceOverlayMergesAutomationWithoutClobberingGlobal(t *testing.T) {
	workspaceRoot, homePaths := prepareAutomationConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, `
[http]
port = 3030

[automation]
enabled = true
timezone = "UTC"
max_concurrent_jobs = 11
default_fire_limit = { max = 21, window = "2h" }

[[automation.jobs]]
scope = "global"
name = "health-check"
schedule = { mode = "every", interval = "30m" }
agent = "monitor"
prompt = "Check system health"
`)
	writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[session.limits]
timeout = "45m"

[[automation.triggers]]
scope = "workspace"
name = "post-run"
event = "session.stopped"
workspace = "/repo"
agent = "summarizer"
prompt = "Summarize {{ index .Data \"session_id\" }}"
`)

	cfg, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got, want := cfg.HTTP.Port, 3030; got != want {
		t.Fatalf("HTTP.Port = %d, want %d", got, want)
	}
	if got, want := cfg.Session.Limits.Timeout, 45*time.Minute; got != want {
		t.Fatalf("Session.Limits.Timeout = %s, want %s", got, want)
	}
	if got, want := cfg.Automation.Timezone, "UTC"; got != want {
		t.Fatalf("Automation.Timezone = %q, want %q", got, want)
	}
	if got, want := cfg.Automation.MaxConcurrentJobs, 11; got != want {
		t.Fatalf("Automation.MaxConcurrentJobs = %d, want %d", got, want)
	}
	if got, want := cfg.Automation.DefaultFireLimit.Window, "2h"; got != want {
		t.Fatalf("Automation.DefaultFireLimit.Window = %q, want %q", got, want)
	}
	if got, want := len(cfg.Automation.Jobs), 1; got != want {
		t.Fatalf("len(Automation.Jobs) = %d, want %d", got, want)
	}
	if got, want := len(cfg.Automation.Triggers), 1; got != want {
		t.Fatalf("len(Automation.Triggers) = %d, want %d", got, want)
	}
	if got, want := cfg.Automation.Triggers[0].Workspace, "/repo"; got != want {
		t.Fatalf("Automation.Triggers[0].Workspace = %q, want %q", got, want)
	}
}

func TestLoadFailsFastOnInvalidAutomationTriggerTemplate(t *testing.T) {
	workspaceRoot, homePaths := prepareAutomationConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, `
[[automation.triggers]]
scope = "global"
name = "post-run"
event = "session.stopped"
agent = "summarizer"
prompt = "Summarize {{ .EnvelopeID }}"
`)

	_, err := Load(WithWorkspaceRoot(workspaceRoot))
	if err == nil {
		t.Fatal("Load() error = nil, want non-nil")
	}
	if got := err.Error(); !strings.Contains(got, "EnvelopeID") {
		t.Fatalf("Load() error = %q, want EnvelopeID detail", got)
	}
}
