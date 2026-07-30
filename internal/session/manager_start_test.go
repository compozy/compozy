package session

import (
	"errors"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

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

	t.Run("Should bind the selected runtime before dispatching the first prompt", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
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
}
