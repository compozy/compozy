package extensionpkg

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestLoadAutomationResources(t *testing.T) {
	t.Parallel()

	t.Run("Should load jobs and triggers with deterministic defaults", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		writeAutomationResource(t, rootDir, "automation/main.toml", `
[[jobs]]
name = "daily"
agent = "writer"
prompt = "Write a digest."
[jobs.schedule]
mode = "cron"
expr = "0 * * * *"

[[triggers]]
name = "on-task"
agent = "writer"
prompt = "Handle the event."
event = "task.completed"
`)
		jobs, triggers, err := LoadAutomationResources(rootDir, "kit", []string{"automation"})
		if err != nil {
			t.Fatalf("LoadAutomationResources() error = %v", err)
		}
		if len(jobs) != 1 || jobs[0].Name != "kit/daily" || !jobs[0].Enabled {
			t.Fatalf("jobs = %#v, want enabled daily job", jobs)
		}
		if jobs[0].Schedule.Mode != "cron" || jobs[0].Schedule.Expr != "0 * * * *" {
			t.Fatalf("job schedule = %#v, want cron schedule", jobs[0].Schedule)
		}
		if jobs[0].FireLimit.Max == 0 || jobs[0].FireLimit.Window == "" {
			t.Fatalf("job fire limit = %#v, want defaults", jobs[0].FireLimit)
		}
		if !reflect.DeepEqual(jobs[0].Retry, automationpkg.DefaultRetryConfig()) ||
			!reflect.DeepEqual(jobs[0].FireLimit, automationpkg.DefaultFireLimitConfig()) {
			t.Fatalf("job defaults retry=%#v fire_limit=%#v", jobs[0].Retry, jobs[0].FireLimit)
		}
		if len(triggers) != 1 || triggers[0].Name != "kit/on-task" || !triggers[0].Enabled {
			t.Fatalf("triggers = %#v, want enabled on-task trigger", triggers)
		}
		if !reflect.DeepEqual(triggers[0].Retry, automationpkg.DefaultRetryConfig()) ||
			!reflect.DeepEqual(triggers[0].FireLimit, automationpkg.DefaultFireLimitConfig()) {
			t.Fatalf("trigger defaults retry=%#v fire_limit=%#v", triggers[0].Retry, triggers[0].FireLimit)
		}
	})

	t.Run("Should preserve an explicitly disabled job", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		writeAutomationResource(t, rootDir, "automation/jobs.toml", `
[[jobs]]
name = "daily"
agent = "writer"
prompt = "Write a digest."
enabled = false
[jobs.schedule]
mode = "every"
interval = "1h"
`)
		jobs, _, err := LoadAutomationResources(rootDir, "kit", []string{"automation/jobs.toml"})
		if err != nil {
			t.Fatalf("LoadAutomationResources() error = %v", err)
		}
		if len(jobs) != 1 || jobs[0].Enabled {
			t.Fatalf("jobs = %#v, want explicitly disabled job", jobs)
		}
	})
}

func TestLoadAutomationResourcesRejectsInvalidDefinitions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "Should reject webhook triggers",
			body: `
[[triggers]]
name = "incoming"
agent = "writer"
prompt = "Handle it."
event = "webhook"
endpoint_slug = "incoming"
`,
			want: "not supported",
		},
		{
			name: "Should reject an invalid cron expression",
			body: `
[[jobs]]
name = "daily"
agent = "writer"
prompt = "Write a digest."
[jobs.schedule]
mode = "cron"
expr = "not-a-cron"
`,
			want: "expr is invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDir := t.TempDir()
			writeAutomationResource(t, rootDir, "automation/main.toml", tc.body)
			_, _, err := LoadAutomationResources(rootDir, "kit", []string{"automation/main.toml"})
			if !errors.Is(err, ErrManifestInvalid) || !strings.Contains(err.Error(), "main.toml") ||
				!strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadAutomationResources() error = %v, want path and %q", err, tc.want)
			}
		})
	}
}

func TestLoadAutomationResourcesRejectsDuplicateNamesAcrossFiles(t *testing.T) {
	t.Parallel()

	t.Run("Should name both files declaring the duplicate job", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		body := `
[[jobs]]
name = "daily"
agent = "writer"
prompt = "Write a digest."
[jobs.schedule]
mode = "every"
interval = "1h"
`
		writeAutomationResource(t, rootDir, "automation/alpha.toml", body)
		writeAutomationResource(t, rootDir, "automation/beta.toml", body)
		_, _, err := LoadAutomationResources(rootDir, "kit", []string{"automation"})
		if !errors.Is(err, ErrManifestInvalid) || !strings.Contains(err.Error(), "alpha.toml") ||
			!strings.Contains(err.Error(), "beta.toml") || !strings.Contains(err.Error(), "daily") {
			t.Fatalf("LoadAutomationResources() error = %v, want both files and duplicate name", err)
		}
	})
}

func TestValidateAutomationAgentTargets(t *testing.T) {
	t.Parallel()

	t.Run("Should accept only agents shipped by the same extension", func(t *testing.T) {
		t.Parallel()

		agents := []StaticAgent{{Agent: compozyconfig.AgentDef{Name: "writer"}}}
		if err := validateAutomationAgentTargets(
			[]automationpkg.Job{{Name: "kit/daily", AgentName: "writer"}},
			[]automationpkg.Trigger{{Name: "kit/on-task", AgentName: "writer"}},
			agents,
		); err != nil {
			t.Fatalf("validateAutomationAgentTargets() error = %v", err)
		}
	})

	t.Run("Should reject an unshipped target and list the shipped set", func(t *testing.T) {
		t.Parallel()

		err := validateAutomationAgentTargets(
			[]automationpkg.Job{{Name: "kit/daily", AgentName: "authored"}},
			nil,
			[]StaticAgent{{Agent: compozyconfig.AgentDef{Name: "writer"}}},
		)
		if !errors.Is(err, ErrManifestInvalid) || !strings.Contains(err.Error(), "authored") ||
			!strings.Contains(err.Error(), "writer") {
			t.Fatalf("validateAutomationAgentTargets() error = %v, want target and shipped set", err)
		}
	})

	t.Run("Should reject an unshipped trigger target and list the shipped set", func(t *testing.T) {
		t.Parallel()

		err := validateAutomationAgentTargets(
			nil,
			[]automationpkg.Trigger{{Name: "kit/on-task", AgentName: "other-extension-agent"}},
			[]StaticAgent{{Agent: compozyconfig.AgentDef{Name: "writer"}}},
		)
		if !errors.Is(err, ErrManifestInvalid) || !strings.Contains(err.Error(), "other-extension-agent") ||
			!strings.Contains(err.Error(), "writer") {
			t.Fatalf("validateAutomationAgentTargets() error = %v, want trigger target and shipped set", err)
		}
	})
}

func TestManagerMaterializesAutomationResources(t *testing.T) {
	t.Parallel()

	t.Run("Should materialize namespaced package automation resources", func(t *testing.T) {
		t.Parallel()

		rootDir := t.TempDir()
		writeStaticAgentFixture(t, rootDir, "writer", false)
		writeAutomationResource(t, rootDir, "automation/main.toml", `
[[jobs]]
name = "disabled-job"
agent = "writer"
prompt = "Write a digest."
enabled = false
[jobs.schedule]
mode = "every"
interval = "1h"

[[triggers]]
name = "on-task"
agent = "writer"
prompt = "Handle it."
event = "task.completed"
`)
		managed := &managedExtension{
			info:    ExtensionInfo{Name: "kit"},
			rootDir: rootDir,
			manifest: &Manifest{Resources: ResourcesConfig{
				Agents: []string{"agents"}, Automation: []string{"automation"},
			}},
		}
		manager := NewManager(nil)
		agents, err := manager.loadAgentResources(managed)
		if err != nil {
			t.Fatalf("loadAgentResources() error = %v", err)
		}
		jobs, triggers, err := manager.loadAutomationResources(managed, agents)
		if err != nil {
			t.Fatalf("loadAutomationResources() error = %v", err)
		}
		if len(jobs) != 1 || jobs[0].Name != "kit/disabled-job" ||
			jobs[0].ID != "extension/kit/automation.job/disabled-job" || jobs[0].Enabled ||
			jobs[0].Scope != automationpkg.AutomationScopeGlobal ||
			jobs[0].Source != automationpkg.JobSourcePackage {
			t.Fatalf("materialized jobs = %#v, want disabled namespaced package job", jobs)
		}
		if len(triggers) != 1 || triggers[0].Name != "kit/on-task" ||
			triggers[0].ID != "extension/kit/automation.trigger/on-task" || !triggers[0].Enabled ||
			triggers[0].Scope != automationpkg.AutomationScopeGlobal ||
			triggers[0].Source != automationpkg.JobSourcePackage {
			t.Fatalf("materialized triggers = %#v, want enabled namespaced package trigger", triggers)
		}
	})
}

func writeAutomationResource(t *testing.T, rootDir, relativePath, body string) {
	t.Helper()

	path := filepath.Join(rootDir, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	writeFile(t, path, body)
}
