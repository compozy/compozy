package extensionpkg

import (
	"context"
	"testing"
	"time"

	automationpkg "github.com/compozy/agh/internal/automation"
	hookspkg "github.com/compozy/agh/internal/hooks"
)

func TestHostAPIAutomationJobsTriggerContract(t *testing.T) {
	t.Parallel()

	t.Run("Should propagate manual trigger payload to job pre fire hooks", func(t *testing.T) {
		t.Parallel()

		payloads := make(chan map[string]any, 1)
		hooks := hookspkg.NewHooks(
			hookspkg.WithNativeDeclarations([]hookspkg.HookDecl{{
				Name:         "capture-manual-job-payload",
				Event:        hookspkg.HookAutomationJobPreFire,
				Mode:         hookspkg.HookModeSync,
				ExecutorKind: hookspkg.HookExecutorNative,
			}}),
			hookspkg.WithExecutorResolver(func(hookspkg.HookDecl) (hookspkg.Executor, error) {
				return hookspkg.NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ hookspkg.RegisteredHook,
						payload hookspkg.AutomationJobPreFirePayload,
					) (hookspkg.AutomationFirePatch, error) {
						payloads <- payload.Payload
						repo, _ := payload.Payload["repo"].(string)
						prompt := "Manual trigger for " + repo
						return hookspkg.AutomationFirePatch{Prompt: &prompt}, nil
					},
				), nil
			}),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("hooks.Rebuild() error = %v", err)
		}
		t.Cleanup(hooks.Close)

		env := newHostAPITestEnv(t, withHostAPIContractHooks(hooks))
		env.grant(
			"ext-automation",
			[]string{"automation/jobs/create", "automation/jobs/trigger"},
			[]string{"automation.write"},
		)

		createResult, err := env.call(t, "ext-automation", "automation/jobs/create", map[string]any{
			"name":         "manual-payload-job",
			"scope":        "workspace",
			"workspace_id": env.workspaceID,
			"agent_name":   "coder",
			"prompt":       "Original prompt",
			"schedule": map[string]any{
				"mode":     "every",
				"interval": "5m",
			},
		})
		if err != nil {
			t.Fatalf("Handle(automation/jobs/create) error = %v", err)
		}

		var created automationpkg.Job
		decodeResult(t, createResult, &created)
		if created.ID == "" {
			t.Fatal("automation/jobs/create id = empty, want non-empty")
		}

		_, err = env.call(t, "ext-automation", "automation/jobs/trigger", map[string]any{
			"id": created.ID,
			"payload": map[string]any{
				"repo": "acme/api",
				"metadata": map[string]any{
					"branch": "main",
				},
			},
		})
		if err != nil {
			t.Fatalf("Handle(automation/jobs/trigger) error = %v", err)
		}

		var observed map[string]any
		select {
		case observed = <-payloads:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for job pre-fire payload")
		}
		if got, want := observed["repo"], "acme/api"; got != want {
			t.Fatalf("pre-fire payload repo = %#v, want %q", got, want)
		}
		metadata, ok := observed["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("pre-fire payload metadata = %#v, want map", observed["metadata"])
		}
		if got, want := metadata["branch"], "main"; got != want {
			t.Fatalf("pre-fire payload branch = %#v, want %q", got, want)
		}

		prompts := hostAPIFakePromptMessages(env.driver)
		if len(prompts) == 0 {
			t.Fatal("driver prompt calls = 0, want job dispatch prompt")
		}
		if got, want := prompts[len(prompts)-1], "Manual trigger for acme/api"; got != want {
			t.Fatalf("last prompt message = %q, want %q", got, want)
		}
	})
}

func withHostAPIContractHooks(hooks *hookspkg.Hooks) hostAPITestEnvOption {
	return func(cfg *hostAPITestEnvConfig) {
		cfg.hooks = hooks
	}
}

func hostAPIFakePromptMessages(driver *hostAPIFakeDriver) []string {
	if driver == nil {
		return nil
	}
	driver.mu.Lock()
	defer driver.mu.Unlock()

	messages := make([]string, 0, len(driver.prompts))
	for _, prompt := range driver.prompts {
		messages = append(messages, prompt.Message)
	}
	return messages
}
