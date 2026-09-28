package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	memcontract "github.com/compozy/compozy/internal/memory/contract"
	"github.com/compozy/compozy/internal/memory/controller"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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
		workspace.Providers = map[string]compozyconfig.ProviderConfig{
			"gateway": {Command: "gateway acp"},
			"backup":  {Command: "backup acp"},
		}
		workspace.Roles.MemoryController = compozyconfig.MemoryControllerRoleConfig{
			Enabled:         true,
			Provider:        "gateway",
			Model:           "workspace-controller",
			ReasoningEffort: "high",
			Timeout:         time.Second,
			TopK:            8,
			PromptVersion:   "v2",
			MaxTokensOut:    512,
			FallbackChain: []compozyconfig.RoleFallback{{
				Provider: "backup",
				Model:    "backup-controller",
			}},
		}
		resolver := newRoleResolver(&global, roleWorkspaceResolverStub{configs: map[string]compozyconfig.Config{
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
		cfg.Providers["model-required"] = compozyconfig.ProviderConfig{
			Command: "pi-acp", Harness: compozyconfig.ProviderHarnessPiACP,
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
	t.Run(
		"Should resolve the candidate owner with a live config snapshot and reject unknown owners",
		func(t *testing.T) {
			t.Parallel()
			cfg := roleResolverConfig()
			scoped := loopActionBinderWorkspace(t, nil)
			scoped.ProfileID = "profile-engineering"
			scoped.Config = cfg
			scoped.Config.Roles.MemoryController.Model = "profile-controller"
			workspaces := &loopPolicyProfileWorkspaceResolver{scoped: scoped}
			roles := newRoleResolver(&cfg, workspaces, nil)
			roles.profileNames = loopProfileNameResolverStub{"profile-engineering": "engineering"}
			invoker := &memoryControllerInvokerStub{result: session.TransientModelResult{
				Output: `{"op":"noop","target_id":"","confidence":0.5,"reason":"ambiguous"}`, Accepted: true,
			}}
			tiebreaker := &daemonMemoryControllerTiebreaker{
				invoker:        invoker,
				roles:          roles,
				configSnapshot: func() compozyconfig.Config { return cfg },
				workspaceResolver: loopActionBinderWorkspaceResolver{
					byID: map[string]workspacepkg.ResolvedWorkspace{"ws-loop": scoped},
				},
			}
			request := controller.TiebreakerRequest{Candidate: memcontract.Candidate{
				Scope:       memcontract.ScopeProfile,
				Content:     "candidate",
				WorkspaceID: "ws-loop",
				ProfileID:   "profile-engineering",
			}}
			result, err := tiebreaker.BreakTie(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			if result.Call == nil || result.Call.Model != "profile-controller" || len(invoker.calls) != 1 ||
				invoker.calls[0].Model != "profile-controller" {
				t.Fatalf("wrong controller: result=%#v calls=%#v", result, invoker.calls)
			}
			request.Candidate.ProfileID = "profile-missing"
			if _, err := tiebreaker.BreakTie(
				t.Context(),
				request,
			); err == nil ||
				!strings.Contains(err.Error(), "profile not found") {
				t.Fatalf("error = %v, want profile not found", err)
			}
			if len(invoker.calls) != 1 {
				t.Fatal("unknown Profile invoked a controller")
			}
		},
	)
	t.Run("Should launch the fallback route account and fingerprint it", func(t *testing.T) { // IT-002
		t.Parallel()

		const seatTwo = "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp"
		cfg := memoryControllerFallbackConfig(seatTwo)
		recorder := &roleEventRecorder{}
		invoker := &memoryControllerInvokerStub{responses: []memoryControllerInvokerResponse{
			{err: acp.WrapFailure(store.FailureStartup, "rate limited", errors.New("429"))},
			{result: session.TransientModelResult{
				Output:   `{"op":"noop","target_id":"","confidence":0.5,"reason":"ambiguous"}`,
				Accepted: true,
			}},
		}}
		tiebreaker := &daemonMemoryControllerTiebreaker{
			invoker:   invoker,
			roles:     newRoleResolver(&cfg, nil, nil, recorder),
			globalCWD: t.TempDir(),
		}
		result, err := tiebreaker.BreakTie(t.Context(), controller.TiebreakerRequest{
			Candidate: memcontract.Candidate{Scope: memcontract.ScopeProfile, Content: "candidate"},
		})
		if err != nil {
			t.Fatalf("BreakTie() error = %v", err)
		}
		if result.Op != memcontract.OpNoop || len(invoker.calls) != 2 ||
			invoker.calls[0].Command != "" || invoker.calls[1].Command != seatTwo {
			t.Fatalf("BreakTie() = %#v calls = %#v, want the second call on the seat-two command", result, invoker.calls)
		}
		event := recorder.single(t)
		var payload roleFallbackEventPayload
		if err := json.Unmarshal(event.Content, &payload); err != nil {
			t.Fatalf("json.Unmarshal(event.Content) error = %v", err)
		}
		if payload.Attempt != 1 || payload.ProviderCommandFingerprint != compozyconfig.CommandFingerprint(seatTwo) ||
			strings.Contains(string(event.Content), "claude-work") {
			t.Fatalf("memory controller fallback event = %s, want attempt 1 with fingerprint only", event.Content)
		}
	})

	t.Run("Should stop when the transient start was accepted and then failed", func(t *testing.T) { // IT-006
		t.Parallel()

		cfg := memoryControllerFallbackConfig("CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp")
		recorder := &roleEventRecorder{}
		acceptedErr := acp.WrapAcceptedStart("acp_1", errors.New("configure failed"))
		invoker := &memoryControllerInvokerStub{responses: []memoryControllerInvokerResponse{{err: acceptedErr}}}
		tiebreaker := &daemonMemoryControllerTiebreaker{
			invoker:   invoker,
			roles:     newRoleResolver(&cfg, nil, nil, recorder),
			globalCWD: t.TempDir(),
		}
		result, err := tiebreaker.BreakTie(t.Context(), controller.TiebreakerRequest{
			Candidate: memcontract.Candidate{Scope: memcontract.ScopeProfile, Content: "candidate"},
		})
		if err != nil && !errors.Is(err, acceptedErr) {
			t.Fatalf("BreakTie() error = %v, want the accepted failure or a rules fallback", err)
		}
		if result.Call != nil && result.Call.Model != "controller-model" {
			t.Fatalf("BreakTie() call = %#v, want the accepted primary route", result.Call)
		}
		if len(invoker.calls) != 1 || recorder.count() != 0 {
			t.Fatalf("calls/events = %d/%d, want one accepted attempt and no fallback", len(invoker.calls), recorder.count())
		}
	})

	t.Run("Should invoke the configured live role with bounded targets", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		cfg.Providers = map[string]compozyconfig.ProviderConfig{
			"mock": {Command: "mock acp"},
		}
		cfg.Roles.MemoryController = compozyconfig.MemoryControllerRoleConfig{
			Enabled:  true,
			Provider: "mock",
			Model:    "controller-model",
			Speed:    speedpkg.SpeedFast,
			ACPOptions: []compozyconfig.ACPOptionSelection{{
				ID: "thinking", BoolValue: new(true),
			}},
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
			Candidate: memcontract.Candidate{Scope: memcontract.ScopeProfile, Content: "candidate"},
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
		if call.Provider != "mock" || call.Model != "controller-model" ||
			call.Speed != speedpkg.SpeedFast || call.MaxOutputBytes != 128 ||
			len(call.ACPOptions) != 1 || call.ACPOptions[0].ID != "thinking" ||
			call.ACPOptions[0].BoolValue == nil || !*call.ACPOptions[0].BoolValue {
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
	// responses, when set, script one response per call in order.
	responses []memoryControllerInvokerResponse
}

type memoryControllerInvokerResponse struct {
	result session.TransientModelResult
	err    error
}

func (s *memoryControllerInvokerStub) InvokeTransientModel(
	_ context.Context,
	call session.TransientModelCall,
) (session.TransientModelResult, error) {
	s.calls = append(s.calls, call)
	if index := len(s.calls) - 1; index < len(s.responses) {
		return s.responses[index].result, s.responses[index].err
	}
	return s.result, s.err
}

func memoryControllerFallbackConfig(routeCommand string) compozyconfig.Config {
	cfg := roleResolverConfig()
	cfg.Providers = map[string]compozyconfig.ProviderConfig{"mock": {Command: "mock acp"}}
	cfg.Roles.MemoryController = compozyconfig.MemoryControllerRoleConfig{
		Enabled:       true,
		Provider:      "mock",
		Model:         "controller-model",
		Timeout:       time.Second,
		TopK:          2,
		PromptVersion: "v1",
		MaxTokensOut:  32,
		FallbackChain: []compozyconfig.RoleFallback{{
			Provider: "mock", Model: "backup-controller", Command: routeCommand,
		}},
	}
	return cfg
}
