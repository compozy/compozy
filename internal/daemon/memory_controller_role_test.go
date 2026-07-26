package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/memory/controller"
	"github.com/compozy/agh/internal/session"
)

func TestMemoryControllerRoleCallOptions(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve every pinned default in the model call options", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		options, err := newRoleResolver(&cfg, nil, nil).resolveMemoryControllerCallOptions(t.Context(), "")
		if err != nil {
			t.Fatalf("resolveMemoryControllerCallOptions() error = %v", err)
		}
		if !options.resolvedRole.Enabled || options.resolvedRole.Provider != "pi" ||
			options.resolvedRole.Model != "anthropic/claude-haiku-4" ||
			options.Timeout != 250*time.Millisecond || options.TopK != 5 ||
			options.PromptVersion != "v1" || options.MaxTokensOut != 256 {
			t.Fatalf("resolveMemoryControllerCallOptions() = %#v, want pinned defaults", options)
		}
	})

	t.Run("Should use the effective workspace role knobs", func(t *testing.T) {
		t.Parallel()

		global := roleResolverConfig()
		workspace := global
		workspace.Providers = map[string]aghconfig.ProviderConfig{
			"gateway": {Command: "gateway acp"},
			"backup":  {Command: "backup acp"},
		}
		workspace.Roles.MemoryController = aghconfig.MemoryControllerRoleConfig{
			Enabled:         true,
			Provider:        "gateway",
			Model:           "workspace-controller",
			ReasoningEffort: "high",
			Timeout:         time.Second,
			TopK:            8,
			PromptVersion:   "v2",
			MaxTokensOut:    512,
			FallbackChain: []aghconfig.RoleFallback{{
				Provider: "backup",
				Model:    "backup-controller",
			}},
		}
		resolver := newRoleResolver(&global, roleWorkspaceResolverStub{configs: map[string]aghconfig.Config{
			"ws-a": workspace,
		}}, nil)
		options, err := resolver.resolveMemoryControllerCallOptions(t.Context(), "ws-a")
		if err != nil {
			t.Fatalf("resolveMemoryControllerCallOptions() error = %v", err)
		}
		if options.resolvedRole.Provider != "gateway" || options.resolvedRole.Model != "workspace-controller" ||
			options.resolvedRole.ReasoningEffort != "high" || options.Timeout != time.Second ||
			options.TopK != 8 || options.PromptVersion != "v2" || options.MaxTokensOut != 512 ||
			len(options.resolvedRole.Fallbacks) != 1 || options.resolvedRole.Fallbacks[0].Provider != "backup" ||
			options.resolvedRole.Fallbacks[0].Model != "backup-controller" {
			t.Fatalf("resolveMemoryControllerCallOptions() = %#v, want workspace options", options)
		}
	})

	t.Run("Should skip runtime resolution when the controller role is disabled", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		cfg.Roles.MemoryController.Enabled = false
		cfg.Roles.MemoryController.Provider = "model-required"
		cfg.Roles.MemoryController.Model = ""
		cfg.Providers["model-required"] = aghconfig.ProviderConfig{
			Command: "pi-acp", Harness: aghconfig.ProviderHarnessPiACP,
		}

		options, err := newRoleResolver(&cfg, nil, nil).resolveMemoryControllerCallOptions(t.Context(), "")
		if err != nil {
			t.Fatalf("resolveMemoryControllerCallOptions() error = %v", err)
		}
		if options.resolvedRole.Enabled || options.resolvedRole.Provider != "model-required" {
			t.Fatalf("resolveMemoryControllerCallOptions() = %#v, want disabled unresolved role", options)
		}
	})
}

func TestMemoryControllerTiebreakerUsesTheLiveRoleCallContract(t *testing.T) {
	t.Parallel()
	t.Run("Should invoke the configured live role with bounded targets", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		cfg.Providers = map[string]aghconfig.ProviderConfig{
			"mock": {Command: "mock acp"},
		}
		cfg.Roles.MemoryController = aghconfig.MemoryControllerRoleConfig{
			Enabled:       true,
			Provider:      "mock",
			Model:         "controller-model",
			Timeout:       time.Second,
			TopK:          2,
			PromptVersion: "v1",
			MaxTokensOut:  32,
		}
		invoker := &memoryControllerInvokerStub{result: session.TransientModelResult{
			Output:   `{"op":"noop","target_id":"","confidence":0.5,"reason":"ambiguous"}`,
			Accepted: true,
		}}
		tiebreaker := &daemonMemoryControllerTiebreaker{
			invoker:   invoker,
			roles:     newRoleResolver(&cfg, nil, nil),
			globalCWD: t.TempDir(),
		}
		result, err := tiebreaker.BreakTie(t.Context(), controller.TiebreakerRequest{
			Candidate: memcontract.Candidate{Scope: memcontract.ScopeGlobal, Content: "candidate"},
			Targets: []controller.Target{
				{ID: "target-a", Content: "a"},
				{ID: "target-b", Content: "b"},
				{ID: "target-c", Content: "c"},
			},
		})
		if err != nil {
			t.Fatalf("BreakTie() error = %v", err)
		}
		if result.Op != memcontract.OpNoop || result.Call == nil || result.Call.Model != "controller-model" {
			t.Fatalf("BreakTie() = %#v, want model-backed noop", result)
		}
		if len(invoker.calls) != 1 {
			t.Fatalf("InvokeTransientModel() calls = %d, want 1", len(invoker.calls))
		}
		call := invoker.calls[0]
		if call.Provider != "mock" || call.Model != "controller-model" || call.MaxOutputBytes != 128 {
			t.Fatalf("TransientModelCall = %#v, want resolved route and 128-byte bound", call)
		}
		if !strings.Contains(call.Prompt, "target-a") || !strings.Contains(call.Prompt, "target-b") ||
			strings.Contains(call.Prompt, "target-c") {
			t.Fatalf("TransientModelCall.Prompt = %q, want top two targets only", call.Prompt)
		}
	})
}

type memoryControllerInvokerStub struct {
	result session.TransientModelResult
	err    error
	calls  []session.TransientModelCall
}

func (s *memoryControllerInvokerStub) InvokeTransientModel(
	_ context.Context,
	call session.TransientModelCall,
) (session.TransientModelResult, error) {
	s.calls = append(s.calls, call)
	return s.result, s.err
}
