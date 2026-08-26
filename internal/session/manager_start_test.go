package session

import (
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/sandbox"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type globalSessionAgentResolver struct{}

func (globalSessionAgentResolver) ResolveAgent(
	name string,
	_ *workspacepkg.ResolvedWorkspace,
) (compozyconfig.AgentDef, error) {
	return compozyconfig.AgentDef{Name: name, Provider: "claude", Prompt: "You are a coding assistant."}, nil
}

type runtimeFreeSessionAgentResolver struct{}

func (runtimeFreeSessionAgentResolver) ResolveAgent(
	name string,
	_ *workspacepkg.ResolvedWorkspace,
) (compozyconfig.AgentDef, error) {
	return compozyconfig.AgentDef{
		Name:        name,
		Permissions: string(compozyconfig.PermissionModeApproveAll),
		Prompt:      "You are a logical caller identity.",
	}, nil
}

func TestCreateAcceptedLogicalRuntimeLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should create an active unbound session without starting ACP", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
		})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		if created.State != StateActive {
			t.Fatalf("CreateAccepted() State = %q, want %q", created.State, StateActive)
		}
		if created.RuntimeStatus != RuntimeStatusUnbound {
			t.Fatalf("CreateAccepted() RuntimeStatus = %q, want %q", created.RuntimeStatus, RuntimeStatusUnbound)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls = %d, want 0", got)
		}

		live, ok := h.manager.Get(created.ID)
		if !ok {
			t.Fatalf("Get(%q) did not find created session", created.ID)
		}
		meta := readMeta(t, live.MetaPath())
		if meta.RuntimeStatus != RuntimeStatusUnbound {
			t.Fatalf("meta runtime status = %q, want %q", meta.RuntimeStatus, RuntimeStatusUnbound)
		}
		page, err := h.manager.TranscriptPage(testutil.Context(t), created.ID, transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage() error = %v", err)
		}
		if len(page.Entries) != 0 {
			t.Fatalf("TranscriptPage() entries = %#v, want empty", page.Entries)
		}
		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	t.Run("Should defer live model validation until the first runtime bind", func(t *testing.T) {
		t.Parallel()

		const cursorModel = "grok-4.5"
		var catalogCalls atomic.Int64
		h := newHarness(t, WithModelCatalog(modelCatalogStub{
			onList: func(modelcatalog.ListOptions) { catalogCalls.Add(1) },
			models: []modelcatalog.Model{{
				ProviderID:             cursorRuntimeProvider,
				ModelID:                cursorModel,
				AvailabilityState:      modelcatalog.AvailabilityStateAvailableLive,
				DefaultReasoningEffort: new(modelcatalog.ReasoningEffortHigh),
				TransportBindings: []modelcatalog.ModelTransportBinding{{
					TransportModelID: "cursor-grok-4.5-high",
					ReasoningEffort:  new(modelcatalog.ReasoningEffortHigh),
					Fast:             new(false),
				}},
				Sources: []modelcatalog.SourceRef{{
					SourceID:   modelcatalog.SourceKindProviderLiveID(cursorRuntimeProvider),
					SourceKind: modelcatalog.SourceKindProviderLive,
				}},
			}},
		}))
		resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
		}
		for index := range resolved.Agents {
			if resolved.Agents[index].Name == "coder" {
				resolved.Agents[index].Provider = cursorRuntimeProvider
				resolved.Agents[index].Model = cursorModel
				resolved.Agents[index].ReasoningEffort = "high"
			}
		}
		h.resolver.upsert(&resolved)

		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
		})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		if got := catalogCalls.Load(); got != 0 {
			t.Fatalf("live catalog calls after logical acceptance = %d, want 0", got)
		}

		result, err := h.manager.SendPrompt(testutil.Context(t), created.ID, SendPromptOpts{
			Message: "Bind the catalog-validated runtime",
			Runtime: &RuntimeSelection{
				Provider:        created.Provider,
				Model:           created.Model,
				ReasoningEffort: created.ReasoningEffort,
				Speed:           created.Speed,
			},
		})
		if err != nil {
			t.Fatalf("SendPrompt() error = %v", err)
		}
		for range result.Events {
			continue
		}
		if got := catalogCalls.Load(); got == 0 {
			t.Fatal("live catalog calls after first bind = 0, want validation")
		}
		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	t.Run("Should create and resume a profile-owned global session without a workspace", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithAgentResolver(globalSessionAgentResolver{}))
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{
				DesiredSessionID: "sess-global-operator",
				Global:           true,
				AgentName:        "coder",
				DisableSandbox:   true,
			},
		})
		if err != nil {
			t.Fatalf("CreateAccepted(global) error = %v", err)
		}
		if created.Scope != store.SessionScopeGlobal || created.WorkspaceID != "" || created.WorktreeID != "" {
			t.Fatalf("CreateAccepted(global) = %#v, want profile-owned global scope", created)
		}
		live, ok := h.manager.Get(created.ID)
		if !ok {
			t.Fatalf("Get(%q) did not find global session", created.ID)
		}
		meta := readMeta(t, live.MetaPath())
		if meta.Scope != store.SessionScopeGlobal || meta.WorkspaceID != "" {
			t.Fatalf("global session metadata = %#v, want global scope without workspace", meta)
		}

		h.manager = newManagerWithHarness(t, h, WithAgentResolver(globalSessionAgentResolver{}))
		resumed, err := h.manager.Resume(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Resume(global) error = %v", err)
		}
		if info := resumed.Info(); info.Scope != store.SessionScopeGlobal || info.WorkspaceID != "" {
			t.Fatalf("Resume(global) = %#v, want preserved global scope", info)
		}
		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop(global) cleanup error = %v", err)
		}
	})

	t.Run("Should create and resume a runtime-free logical caller without a provider", func(t *testing.T) {
		t.Parallel()

		universe := []toolspkg.ToolID{"compozy__agent_call", "compozy__agent_message"}
		managerOptions := []Option{
			WithAgentResolver(runtimeFreeSessionAgentResolver{}),
			WithToolUniverse(universe),
		}
		h := newHarness(t, managerOptions...)
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			RuntimeFree: true,
			Session: CreateOpts{
				DesiredSessionID: "sess-runtime-free-caller",
				Global:           true,
				AgentName:        "general",
				DisableSandbox:   true,
			},
		})
		if err != nil {
			t.Fatalf("CreateAccepted(runtime-free) error = %v", err)
		}
		if created.Provider != "" || created.Model != "" || created.RuntimeStatus != RuntimeStatusUnbound {
			t.Fatalf("CreateAccepted(runtime-free) = %#v, want providerless unbound session", created)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls = %d, want 0", got)
		}
		wantTools := []string{"compozy__agent_call", "compozy__agent_message"}
		if created.Lineage == nil || !slices.Equal(created.Lineage.PermissionPolicy.Tools, wantTools) {
			t.Fatalf("runtime-free lineage tools = %#v, want %#v", created.Lineage, wantTools)
		}
		live, ok := h.manager.Get(created.ID)
		if !ok {
			t.Fatalf("Get(%q) did not find runtime-free session", created.ID)
		}
		meta := readMeta(t, live.MetaPath())
		if meta.CreationProfile != nil || meta.CreationOptions != nil || meta.Provider != "" {
			t.Fatalf("runtime-free metadata = %#v, want no runtime creation identity", meta)
		}
		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop(runtime-free) error = %v", err)
		}

		h.manager = newManagerWithHarness(t, h, managerOptions...)
		status, err := h.manager.Status(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Status(runtime-free) error = %v", err)
		}
		if status.State != StateStopped || status.Provider != "" {
			t.Fatalf("Status(runtime-free) = %#v, want stopped providerless session", status)
		}
		resumed, err := h.manager.Resume(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Resume(runtime-free) error = %v", err)
		}
		if info := resumed.Info(); info.Provider != "" || info.RuntimeStatus != RuntimeStatusUnbound {
			t.Fatalf("Resume(runtime-free) = %#v, want providerless unbound session", info)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls after Resume(runtime-free) = %d, want 0", got)
		}
		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop(resumed runtime-free) error = %v", err)
		}
	})

	t.Run("Should bind the selected runtime before dispatching the first prompt", func(t *testing.T) {
		t.Parallel()

		provider := &recordingSandboxProvider{}
		h := newHarness(t, WithSandboxRegistry(newRegistryForProvider(t, provider)))
		resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
		}
		resolved.Config.Sandboxes["task-ref"] = compozyconfig.SandboxProfile{
			Backend:     string(sandbox.BackendLocal),
			SyncMode:    string(sandbox.SyncModeNone),
			Persistence: string(sandbox.PersistenceReuse),
		}
		h.resolver.upsert(&resolved)

		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{
				AgentName:  "coder",
				Workspace:  h.workspaceID,
				SandboxRef: "task-ref",
			},
		})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		result, err := h.manager.SendPrompt(testutil.Context(t), created.ID, SendPromptOpts{
			Message: "Inspect the repository",
			Runtime: &RuntimeSelection{
				Provider:        created.Provider,
				Model:           created.Model,
				ReasoningEffort: created.ReasoningEffort,
				Speed:           created.Speed,
			},
		})
		if err != nil {
			t.Fatalf("SendPrompt() error = %v", err)
		}
		for range result.Events {
			continue
		}
		if got := len(h.driver.startCalls); got != 1 {
			t.Fatalf("driver start calls = %d, want 1", got)
		}
		if got := len(provider.prepareRequests); got != 1 {
			t.Fatalf("sandbox prepare calls = %d, want 1", got)
		}
		if got, want := provider.prepareRequests[0].Sandbox.Profile, "task-ref"; got != want {
			t.Fatalf("sandbox profile = %q, want %q", got, want)
		}
		status, err := h.manager.Status(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.RuntimeStatus != RuntimeStatusReady || status.RuntimeTransition != RuntimeTransitionInitialBind {
			t.Fatalf("runtime status = %q/%q, want ready/initial_bind", status.RuntimeStatus, status.RuntimeTransition)
		}
		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	t.Run("Should resume an unbound logical session without starting ACP", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
		})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}

		h.manager = newManagerWithHarness(t, h)
		resumed, err := h.manager.Resume(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		if got := len(h.driver.startCalls); got != 0 {
			t.Fatalf("driver start calls after Resume() = %d, want 0", got)
		}
		if info := resumed.Info(); info.RuntimeStatus != RuntimeStatusUnbound || info.ACPSessionID != "" {
			t.Fatalf(
				"resumed runtime = %q with ACP session %q, want unbound with no ACP session",
				info.RuntimeStatus,
				info.ACPSessionID,
			)
		}
		meta := readMeta(t, resumed.MetaPath())
		if meta.RuntimeStatus != RuntimeStatusUnbound || derefString(meta.ACPSessionID) != "" {
			t.Fatalf(
				"resumed meta runtime = %q with ACP session %q, want unbound with no ACP session",
				meta.RuntimeStatus,
				derefString(meta.ACPSessionID),
			)
		}

		result, err := h.manager.SendPrompt(testutil.Context(t), resumed.ID, SendPromptOpts{
			Message: "Bind after daemon restart",
			Runtime: &RuntimeSelection{
				Provider:        resumed.Provider,
				Model:           resumed.Model,
				ReasoningEffort: resumed.ReasoningEffort,
				Speed:           resumed.Speed,
			},
		})
		if err != nil {
			t.Fatalf("SendPrompt() error = %v", err)
		}
		for range result.Events {
			continue
		}
		if got := len(h.driver.startCalls); got != 1 {
			t.Fatalf("driver start calls after SendPrompt() = %d, want 1", got)
		}
		status, err := h.manager.Status(testutil.Context(t), resumed.ID)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if status.RuntimeStatus != RuntimeStatusReady || status.RuntimeTransition != RuntimeTransitionInitialBind {
			t.Fatalf("runtime status = %q/%q, want ready/initial_bind", status.RuntimeStatus, status.RuntimeTransition)
		}
		if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	t.Run("Should preserve an empty transcript when initial binding fails", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		startErr := errors.New("provider unavailable")
		h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
			return nil, startErr
		}
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
		})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		_, err = h.manager.SendPrompt(testutil.Context(t), created.ID, SendPromptOpts{
			Message: "This must not be recorded",
			Runtime: &RuntimeSelection{Provider: created.Provider, Model: created.Model, Speed: created.Speed},
		})
		if !errors.Is(err, startErr) {
			t.Fatalf("SendPrompt() error = %v, want provider failure", err)
		}
		live, ok := h.manager.Get(created.ID)
		if !ok {
			t.Fatalf("Get(%q) did not find created session", created.ID)
		}
		info := live.Info()
		if info.RuntimeStatus != RuntimeStatusUnbound || strings.TrimSpace(info.RuntimeFailure) == "" {
			t.Fatalf("session runtime after failed initial bind = %#v, want unbound with failure", info)
		}
		meta := readMeta(t, live.MetaPath())
		if meta.RuntimeStatus != RuntimeStatusUnbound ||
			store.SessionRuntimeFailureValue(meta.RuntimeFailure) == "" {
			t.Fatalf("metadata runtime after failed initial bind = %#v, want unbound with failure", meta)
		}
		page, queryErr := h.manager.TranscriptPage(testutil.Context(t), created.ID, transcript.PageQuery{})
		if queryErr != nil {
			t.Fatalf("TranscriptPage() error = %v", queryErr)
		}
		if len(page.Entries) != 0 {
			t.Fatalf("TranscriptPage() entries = %#v, want empty", page.Entries)
		}
		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})
}

func TestSessionStartEnvFiltersDaemonSecrets(t *testing.T) {
	t.Parallel()

	t.Run("Should remove credential shaped daemon variables and keep Compozy session context", func(t *testing.T) {
		t.Parallel()

		env := sessionStartEnv(
			[]string{
				"PATH=/usr/bin",
				"OPENAI_API_KEY=sk-secret",
				"GITHUB_TOKEN=ghp-secret",
				"PROVIDER_HOME=/tmp/provider",
			},
			&Session{
				ID:                   "sess-1",
				AgentName:            "coder",
				NetworkParticipation: testLiveParticipation("ws-test", "ops"),
			},
		)

		if got := envValue(env, "OPENAI_API_KEY"); got != "" {
			t.Fatalf("OPENAI_API_KEY = %q, want filtered", got)
		}
		if got := envValue(env, "GITHUB_TOKEN"); got != "" {
			t.Fatalf("GITHUB_TOKEN = %q, want filtered", got)
		}
		if got := envValue(env, "PROVIDER_HOME"); got != "/tmp/provider" {
			t.Fatalf("PROVIDER_HOME = %q, want %q", got, "/tmp/provider")
		}
		if got := envValue(env, "COMPOZY_SESSION_ID"); got != "sess-1" {
			t.Fatalf("COMPOZY_SESSION_ID = %q, want %q", got, "sess-1")
		}
		if got := envValue(env, "COMPOZY_PEER_ID"); got == "" {
			t.Fatal("COMPOZY_PEER_ID = empty, want network peer id")
		}
	})
}

func TestSessionStartEnvForProviderSupportsIsolatedPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should keep only operational env before adding session context", func(t *testing.T) {
		t.Parallel()

		env := sessionStartEnvForProvider(
			[]string{
				"PATH=/usr/bin",
				"HOME=/Users/operator",
				"OPENAI_API_KEY=sk-secret",
				"FEATURE_FLAG=enabled",
				"PROVIDER_HOME=/tmp/provider",
			},
			&Session{
				ID:                   "sess-1",
				AgentName:            "coder",
				NetworkParticipation: testLiveParticipation("ws-test", "ops"),
			},
			compozyconfig.ProviderEnvPolicyIsolated,
			"",
		)

		if got := envValue(env, "OPENAI_API_KEY"); got != "" {
			t.Fatalf("OPENAI_API_KEY = %q, want isolated env to drop secrets", got)
		}
		if got := envValue(env, "FEATURE_FLAG"); got != "" {
			t.Fatalf("FEATURE_FLAG = %q, want isolated env to drop non-allowlisted variables", got)
		}
		if got := envValue(env, "PATH"); got != "/usr/bin" {
			t.Fatalf("PATH = %q, want %q", got, "/usr/bin")
		}
		if got := envValue(env, "COMPOZY_SESSION_ID"); got != "sess-1" {
			t.Fatalf("COMPOZY_SESSION_ID = %q, want %q", got, "sess-1")
		}
	})

	t.Run("Should use the effective launch effort instead of session state", func(t *testing.T) {
		t.Parallel()

		env := sessionStartEnvForProvider(
			[]string{"PATH=/usr/bin"},
			&Session{
				ID:              "sess-1",
				AgentName:       "coder",
				ReasoningEffort: "low",
			},
			compozyconfig.ProviderEnvPolicyFiltered,
			"high",
		)

		if got := envValue(env, "COMPOZY_REASONING_EFFORT"); got != "high" {
			t.Fatalf("COMPOZY_REASONING_EFFORT = %q, want %q", got, "high")
		}
	})
}
