package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadParsesAutomationConfigAndAppliesConfigSourceDefaults(t *testing.T) {
	workspaceRoot, homePaths := prepareAutomationConfigTestEnv(t)
	writeFile(t, homePaths.ConfigFile, `
[automation]
timezone = "UTC"
max_concurrent_jobs = 7
default_fire_limit = { max = 9, window = "30m" }

[automation.suggestions]
pending_cap = 4

[[automation.jobs]]
scope = "workspace"
name = "health-check"
workspace = "/repo"
schedule = { mode = "every", interval = "30m" }
agent = "monitor"
prompt = "Check system health"
task = { title = "Check health", network_participation = { mode = "live", channel_strategy = "named", channel_id = "ops", bounds = { max_wakes = 2, max_input_tokens = 4096, max_output_tokens = 2048 } } }

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

	if got, want := cfg.Automation.Timezone, "UTC"; got != want {
		t.Fatalf("Automation.Timezone = %q, want %q", got, want)
	}
	if got, want := cfg.Automation.MaxConcurrentJobs, 7; got != want {
		t.Fatalf("Automation.MaxConcurrentJobs = %d, want %d", got, want)
	}
	if got, want := cfg.Automation.DefaultFireLimit.Max, 9; got != want {
		t.Fatalf("Automation.DefaultFireLimit.Max = %d, want %d", got, want)
	}
	if got, want := cfg.Automation.Suggestions.PendingCap, 4; got != want {
		t.Fatalf("Automation.Suggestions.PendingCap = %d, want %d", got, want)
	}
	if got, want := len(cfg.Automation.Jobs), 1; got != want {
		t.Fatalf("len(Automation.Jobs) = %d, want %d", got, want)
	}
	if got, want := len(cfg.Automation.Triggers), 1; got != want {
		t.Fatalf("len(Automation.Triggers) = %d, want %d", got, want)
	}

	job := cfg.Automation.Jobs[0]
	if got, want := job.Scope, "workspace"; string(got) != want {
		t.Fatalf("job.Scope = %q, want %q", got, want)
	}
	if got, want := job.Workspace, "/repo"; got != want {
		t.Fatalf("job.Workspace = %q, want %q", got, want)
	}
	if got, want := string(job.Source), "config"; got != want {
		t.Fatalf("job.Source = %q, want %q", got, want)
	}
	if got, want := string(job.Retry.Strategy), "none"; got != want {
		t.Fatalf("job.Retry.Strategy = %q, want %q", got, want)
	}
	if got, want := job.FireLimit.Window, "30m"; got != want {
		t.Fatalf("job.FireLimit.Window = %q, want %q", got, want)
	}
	if job.Task == nil || job.Task.NetworkParticipation == nil {
		t.Fatalf("job.Task.NetworkParticipation = %#v, want TOML-authored participation", job.Task)
	}
	request := job.Task.NetworkParticipation
	if request.Mode == nil || *request.Mode != "live" ||
		request.ChannelStrategy == nil || *request.ChannelStrategy != "named" ||
		request.ChannelID == nil || *request.ChannelID != "ops" ||
		request.Bounds == nil || request.Bounds.MaxWakes == nil || *request.Bounds.MaxWakes != 2 ||
		request.Bounds.MaxInputTokens == nil || *request.Bounds.MaxInputTokens != 4096 ||
		request.Bounds.MaxOutputTokens == nil || *request.Bounds.MaxOutputTokens != 2048 {
		t.Fatalf("job participation = %#v, want complete TOML field mapping", request)
	}

	trigger := cfg.Automation.Triggers[0]
	if got, want := trigger.Scope, "workspace"; string(got) != want {
		t.Fatalf("trigger.Scope = %q, want %q", got, want)
	}
	if got, want := trigger.Workspace, "/repo"; got != want {
		t.Fatalf("trigger.Workspace = %q, want %q", got, want)
	}
	if got, want := string(trigger.Source), "config"; got != want {
		t.Fatalf("trigger.Source = %q, want %q", got, want)
	}
	if !trigger.Enabled {
		t.Fatal("trigger.Enabled = false, want true")
	}
	if got, want := trigger.FireLimit.Max, 9; got != want {
		t.Fatalf("trigger.FireLimit.Max = %d, want %d", got, want)
	}
}

func TestLoadRejectsAutomationScopeWorkspaceInvariants(t *testing.T) {
	testCases := []struct {
		name     string
		contents string
		wantErr  string
	}{
		{
			name: "global with workspace binding",
			contents: `
[[automation.jobs]]
scope = "global"
name = "health-check"
schedule = { mode = "every", interval = "30m" }
agent = "monitor"
workspace = "/repo"
prompt = "Check system health"
`,
			wantErr: "automation.jobs[0].workspace",
		},
		{
			name: "workspace without binding",
			contents: `
[[automation.triggers]]
scope = "workspace"
name = "post-run"
event = "session.stopped"
agent = "summarizer"
prompt = "Summarize {{ .Kind }}"
`,
			wantErr: "automation.triggers[0].workspace",
		},
		{
			name: "global direct task with live participation",
			contents: `
[[automation.jobs]]
scope = "global"
name = "network-health-check"
schedule = { mode = "every", interval = "30m" }
agent = "monitor"
prompt = "Check network health"
task = { title = "Check network health", network_participation = { mode = "live", channel_strategy = "named", channel_id = "ops" } }
`,
			wantErr: "automation.jobs[0].task.network_participation cannot be live",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workspaceRoot, homePaths := prepareAutomationConfigTestEnv(t)
			writeFile(t, homePaths.ConfigFile, tc.contents)

			_, err := Load(WithWorkspaceRoot(workspaceRoot))
			if err == nil {
				t.Fatal("Load() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Load() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestLoadRejectsAutomationWebhookFieldMismatches(t *testing.T) {
	t.Setenv("AGH_AUTOMATION_WEBHOOK_SECRET", "super-secret")

	testCases := []struct {
		name     string
		contents string
		wantErr  string
	}{
		{
			name: "non webhook trigger with endpoint slug",
			contents: `
[[automation.triggers]]
scope = "global"
name = "post-run"
event = "session.stopped"
agent = "summarizer"
prompt = "Summarize {{ .Kind }}"
endpoint_slug = "deploy-review"
`,
			wantErr: "endpoint_slug",
		},
		{
			name: "webhook trigger without endpoint slug",
			contents: `
[[automation.triggers]]
scope = "global"
name = "deploy"
	event = "webhook"
	agent = "summarizer"
	prompt = "Review {{ index .Data \"payload\" }}"
	webhook_secret_ref = "env:AGH_AUTOMATION_WEBHOOK_SECRET"
	`,
			wantErr: "endpoint_slug",
		},
		{
			name: "webhook trigger without secret ref",
			contents: `
[[automation.triggers]]
scope = "global"
name = "deploy"
event = "webhook"
endpoint_slug = "deploy-review"
agent = "summarizer"
prompt = "Review {{ index .Data \"payload\" }}"
`,
			wantErr: "webhook_secret_ref",
		},
		{
			name: "non webhook trigger with secret ref",
			contents: `
[[automation.triggers]]
scope = "global"
name = "post-run"
	event = "session.stopped"
	agent = "summarizer"
	prompt = "Summarize {{ .Kind }}"
	webhook_secret_ref = "env:AGH_AUTOMATION_WEBHOOK_SECRET"
	`,
			wantErr: "webhook_secret_ref",
		},
		{
			name: "webhook trigger with invalid secret ref",
			contents: `
[[automation.triggers]]
scope = "global"
name = "deploy"
event = "webhook"
	endpoint_slug = "deploy-review"
	agent = "summarizer"
	prompt = "Review {{ index .Data \"payload\" }}"
	webhook_secret_ref = "vault:mcp/wrong/webhook-secret"
	`,
			wantErr: "webhook_secret_ref",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workspaceRoot, homePaths := prepareAutomationConfigTestEnv(t)
			writeFile(t, homePaths.ConfigFile, tc.contents)

			_, err := Load(WithWorkspaceRoot(workspaceRoot))
			if err == nil {
				t.Fatal("Load() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Load() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func TestLoadRejectsInvalidAutomationPolicies(t *testing.T) {
	testCases := []struct {
		name     string
		contents string
		wantErr  string
	}{
		{
			name: "non-positive suggestion pending cap",
			contents: `
[automation.suggestions]
pending_cap = 0
`,
			wantErr: "automation.suggestions.pending_cap must be positive",
		},
		{
			name: "unsupported schedule mode",
			contents: `
[[automation.jobs]]
scope = "global"
name = "health-check"
schedule = { mode = "later", interval = "30m" }
agent = "monitor"
prompt = "Check system health"
`,
			wantErr: "schedule.mode",
		},
		{
			name: "malformed retry settings",
			contents: `
[[automation.jobs]]
scope = "global"
name = "health-check"
schedule = { mode = "every", interval = "30m" }
agent = "monitor"
prompt = "Check system health"
retry = { strategy = "backoff", max_retries = 0, base_delay = "2s" }
`,
			wantErr: "retry.max_retries",
		},
		{
			name: "malformed fire limit window",
			contents: `
[[automation.triggers]]
scope = "global"
name = "post-run"
event = "session.stopped"
agent = "summarizer"
prompt = "Summarize {{ .Kind }}"
fire_limit = { max = 2, window = "later" }
`,
			wantErr: "fire_limit.window",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workspaceRoot, homePaths := prepareAutomationConfigTestEnv(t)
			writeFile(t, homePaths.ConfigFile, tc.contents)

			_, err := Load(WithWorkspaceRoot(workspaceRoot))
			if err == nil {
				t.Fatal("Load() error = nil, want non-nil")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("Load() error = %q, want substring %q", got, tc.wantErr)
			}
		})
	}
}

func prepareAutomationConfigTestEnv(t *testing.T) (string, HomePaths) {
	t.Helper()

	workspaceRoot := t.TempDir()
	homeRoot := filepath.Join(t.TempDir(), "home")
	t.Setenv("AGH_HOME", homeRoot)

	homePaths, err := ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths() error = %v", err)
	}
	if err := EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	return workspaceRoot, homePaths
}
