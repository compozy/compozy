package session

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

func TestSessionStartDropsUnrestrictedAgentModeUnderRestrictedPermissions(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should start without a globally preferred unrestricted mode when session permissions narrow",
		func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.cfg.Permissions.Mode = compozyconfig.PermissionModeApproveAll
			h.cfg.Permissions.ProviderFullAccess = true
			workspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			workspace.Config = h.cfg
			h.resolver.upsert(&workspace)

			created, err := h.manager.Create(testutil.Context(t), CreateOpts{
				AgentName:   "coder",
				Workspace:   h.workspaceID,
				Permissions: compozyconfig.PermissionModeApproveReads,
			})
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
			t.Cleanup(func() {
				if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
					t.Errorf("Stop() error = %v", err)
				}
			})
			if got := h.driver.startCalls[0].ACPOptions; len(got) != 0 {
				t.Fatalf("driver ACP options = %#v, want no unrestricted mode", got)
			}
		},
	)
	t.Run("Should start with restricted permissions and no inherited unrestricted mode", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		workspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		for index := range workspace.Agents {
			if workspace.Agents[index].Name == "coder" {
				workspace.Agents[index].SetACPOptions([]compozyconfig.ACPOptionSelection{
					{ID: "mode", ValueID: "bypassPermissions"},
				})
			}
		}
		h.resolver.upsert(&workspace)

		created, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:   "coder",
			Workspace:   h.workspaceID,
			Permissions: compozyconfig.PermissionModeApproveReads,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		if got := created.Info().EffectivePermissions; got != string(compozyconfig.PermissionModeApproveReads) {
			t.Fatalf("effective permissions = %q, want approve-reads", got)
		}
		if len(h.driver.startCalls) != 1 {
			t.Fatalf("driver start calls = %d, want 1", len(h.driver.startCalls))
		}
		if got := h.driver.startCalls[0].ACPOptions; len(got) != 0 {
			t.Fatalf("driver ACP options = %#v, want no unrestricted mode", got)
		}
	})
}

// TestCreateAcceptedLogicalRuntimeLifecycle verifies deferred provider binding across creation, prompts, and resume.
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

		// Logical resume has the same admission boundary as initial acceptance.
		h.manager = newManagerWithHarness(t, h, WithModelCatalog(h.manager.modelCatalog))
		if _, err := h.manager.Resume(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		if got := catalogCalls.Load(); got != 0 {
			t.Fatalf("live catalog calls after logical resume = %d, want 0", got)
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

	t.Run("Should bind the selected runtime before dispatching the first prompt", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)

		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{
				AgentName: "coder",
				Workspace: h.workspaceID,
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
			meta.RuntimeFailureValue() == "" {
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

// Invariant: local provider launch filters daemon secrets while preserving session and agent identity.
// Owner: session launch environment; canonical session start environment suite.
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
				ID:        "sess-1",
				AgentName: "coder",
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
		if got := envValue(env, "COMPOZY_AGENT"); got != "coder" {
			t.Fatalf("COMPOZY_AGENT = %q, want coder", got)
		}
		if got := envValue(env, "COMPOZY_AGENT_NAME"); got != "coder" {
			t.Fatalf("COMPOZY_AGENT_NAME = %q, want coder", got)
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
				ID:        "sess-1",
				AgentName: "coder",
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

// Invariant: explicit session IDs preserve valid local filenames and reject path traversal.
func TestCreatePreallocatedSessionIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		id    string
		valid bool
	}{
		{name: "Should preserve a caller supplied identifier", id: "sess~manual", valid: true},
		{name: "Should preserve a long local identifier", id: strings.Repeat("s", 128), valid: true},
		{name: "Should reject traversal", id: "../escape"},
		{name: "Should reject a nested path", id: "parent/child"},
		{name: "Should reject the current directory", id: "."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			created, err := h.manager.Create(
				t.Context(),
				CreateOpts{AgentName: "coder", DesiredSessionID: tc.id, Workspace: h.workspaceID},
			)
			if !tc.valid {
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("Create(%q) error = %v, want validation rejection", tc.id, err)
				}
				if len(h.driver.startCalls) != 0 {
					t.Fatal("invalid identity started the driver")
				}
				return
			}
			if err != nil {
				t.Fatalf("Create(%q) error = %v", tc.id, err)
			}
			reportSessionStop(t, h, created.ID)
			if created.ID != tc.id {
				t.Fatalf("session ID = %q, want %q", created.ID, tc.id)
			}
		})
	}
}

func persistedSessionMetas(t *testing.T, h *harness) []store.SessionMeta {
	t.Helper()
	entries, err := os.ReadDir(h.homePaths.SessionsDir)
	if err != nil {
		t.Fatalf("ReadDir(sessions) error = %v", err)
	}
	metas := make([]store.SessionMeta, 0, len(entries))
	for _, entry := range entries {
		path := store.SessionMetaFile(filepath.Join(h.homePaths.SessionsDir, entry.Name()))
		if _, statErr := os.Stat(path); statErr != nil {
			continue
		}
		metas = append(metas, readMeta(t, path))
	}
	return metas
}

// Invariant: a session-owned create runs the agent chain inside one acceptance
// lifetime — the ledger row exists before each fallback Start, refused attempts leave
// attempt-attributed markers and no failed start, and exhaustion persists exactly one
// failed start (IT-008, UT-030).
func TestCreateFallbackChain(t *testing.T) {
	t.Parallel()

	newLedgerHarness := func(t *testing.T, extraOpts ...Option) (*harness, *globaldb.GlobalDB) {
		t.Helper()
		h := newHarness(t)
		installFallbackAgent(t, h, claudeSeatOneRoute(h), codexRoute(h))
		db := openManagerInputQueueStore(t)
		registerManagerInputQueueWorkspace(t, db, h)
		opts := append([]Option{WithSessionCatalog(db), WithEventLedger(db)}, extraOpts...)
		h.manager = newManagerWithHarness(t, h, opts...)
		cleanupTestManager(t, h.manager)
		return h, db
	}
	ledgerRows := func(t *testing.T, db *globaldb.GlobalDB) int {
		t.Helper()
		rows, err := db.ListEventSummaries(testutil.Context(t), store.EventSummaryQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, Type: eventspkg.SessionFallbackUsed, Limit: 50,
		})
		if err != nil {
			t.Errorf("ListEventSummaries() error = %v", err)
		}
		return len(rows)
	}

	t.Run("Should commit each ledger row before its attempt and bind on the accepting route", func(t *testing.T) {
		t.Parallel()
		h, db := newLedgerHarness(t)
		var observed []int
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			observed = append(observed, ledgerRows(t, db))
			switch opts.Command {
			case fallbackSeatZero:
				return nil, rateLimitRefusal()
			case fallbackSeatOne:
				return nil, rateLimitRefusal()
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-%d", sequence)), nil
		}
		created, err := h.manager.Create(
			testutil.Context(t),
			CreateOpts{AgentName: "reviewer", Workspace: h.workspaceID},
		)
		if err != nil {
			t.Fatalf("Create(reviewer) error = %v", err)
		}
		cleanupSessionStop(t, h, created.ID)
		if !slices.Equal(observed, []int{0, 1, 2}) {
			t.Fatalf("ledger rows observed inside each Start = %v, want [0 1 2]", observed)
		}
		codexCommand := h.cfg.Providers["codex"].Command
		info := created.Info()
		if info.Provider != "codex" || info.ACPSessionID != "acp-3" || info.State != StateActive {
			t.Fatalf(
				"created = %s on %s acp=%q, want active on codex with acp-3",
				info.State,
				info.Provider,
				info.ACPSessionID,
			)
		}
		metas := persistedSessionMetas(t, h)
		if len(metas) != 1 || metas[0].Failure != nil || metas[0].ID != created.ID {
			t.Fatalf("persisted sessions = %#v, want the one accepted session without a failed start", metas)
		}
		markers := transcriptMarkersOfKind(t, h.manager, created.ID, transcript.MarkerProviderFailure)
		if len(markers) != 2 {
			t.Fatalf("provider_failure markers = %d, want 2", len(markers))
		}
		for index, command := range []string{fallbackSeatZero, fallbackSeatOne} {
			if markers[index].Evidence["attempt"] != float64(index) ||
				markers[index].Evidence["provider_command_fingerprint"] != providerCommandFingerprint(command) ||
				markers[index].Evidence["next_action"] != string(acp.ProviderFailureActionUseFallback) {
				t.Fatalf(
					"marker %d evidence = %#v, want attempt %d attributed to %q",
					index,
					markers[index].Evidence,
					index,
					command,
				)
			}
		}
		if got := providerCommandFingerprint(
			created.providerRoutingSnapshot().Command,
		); got != providerCommandFingerprint(
			codexCommand,
		) {
			t.Fatalf("routing snapshot fingerprint = %s, want the accepted codex route", got)
		}
		rows, err := db.ListEventSummaries(testutil.Context(t), store.EventSummaryQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, Type: eventspkg.SessionFallbackUsed,
			SessionID: created.ID, Limit: 10,
		})
		if err != nil || len(rows) != 2 {
			t.Fatalf("session-scoped ledger rows = %d, error = %v, want 2", len(rows), err)
		}
		for _, row := range rows {
			payload := decodeFallbackPayload(t, row)
			if payload.Attempt == 2 && payload.ProviderCommandFingerprint != providerCommandFingerprint(codexCommand) {
				t.Fatalf("attempt 2 payload = %#v, want the codex route fingerprint", payload)
			}
			if payload.Phase != fallbackPhaseCreate {
				t.Fatalf("payload phase = %q, want create", payload.Phase)
			}
		}
	})

	t.Run("Should persist exactly one failed start when every route refuses", func(t *testing.T) {
		t.Parallel()
		h, db := newLedgerHarness(t)
		codexCommand := h.cfg.Providers["codex"].Command
		refuseStartCommands(h, map[string]error{
			fallbackSeatZero: rateLimitRefusal(),
			fallbackSeatOne:  rateLimitRefusal(),
			codexCommand:     rateLimitRefusal(),
		})
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{AgentName: "reviewer", Workspace: h.workspaceID})
		if err == nil || !strings.Contains(err.Error(), "fallback chain exhausted after 3 attempt(s)") {
			t.Fatalf("Create() error = %v, want deterministic exhaustion", err)
		}
		if got := ledgerRows(t, db); got != 2 {
			t.Fatalf("ledger rows = %d, want one per fallback attempt", got)
		}
		metas := persistedSessionMetas(t, h)
		if len(metas) != 1 || metas[0].Failure == nil {
			t.Fatalf("persisted sessions = %#v, want exactly one failed start", metas)
		}
		live, ok := h.manager.Get(metas[0].ID)
		if ok {
			t.Fatalf("failed start stayed registered: %#v", live.Info())
		}
		markers := transcriptMarkersOfKind(t, h.manager, metas[0].ID, transcript.MarkerProviderFailure)
		if len(markers) != 3 {
			t.Fatalf("provider_failure markers = %d, want two advancing refusals plus the failed start", len(markers))
		}
		if got := markers[2].Evidence["provider_command_fingerprint"]; got != providerCommandFingerprint(codexCommand) {
			t.Fatalf("failed-start marker fingerprint = %v, want the last refused route", got)
		}
	})

	// refuseWithProcess makes the fake driver return a live process together with a
	// pre-acceptance refusal for the given commands; stopping such a process fails.
	refuseWithProcess := func(h *harness, commands map[string]bool, stopErr error) {
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			sessionID := fmt.Sprintf("acp-%d", sequence)
			if commands[opts.Command] {
				return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "refused-"+sessionID),
					rateLimitRefusal()
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, sessionID), nil
		}
		h.driver.stopHook = func(proc *fakeProcess) error {
			if strings.HasPrefix(proc.handle.SessionID, "refused-") {
				return stopErr
			}
			proc.exit()
			return nil
		}
	}
	cleanupFailures := func(logs *captureLogHandler) []capturedLogRecord {
		var failures []capturedLogRecord
		for _, record := range logs.Records() {
			if record.Message == "session.fallback.cleanup_failed" {
				failures = append(failures, record)
			}
		}
		return failures
	}

	t.Run("Should log a failed refused-process cleanup and continue to the next route", func(t *testing.T) {
		t.Parallel()
		logs := newCaptureLogHandler()
		h, db := newLedgerHarness(t, WithLogger(slog.New(logs)))
		stopErr := errors.New("refused seat process would not stop")
		refuseWithProcess(h, map[string]bool{fallbackSeatZero: true, fallbackSeatOne: true}, stopErr)
		created, err := h.manager.Create(
			testutil.Context(t),
			CreateOpts{AgentName: "reviewer", Workspace: h.workspaceID},
		)
		if err != nil {
			t.Fatalf("Create(reviewer) error = %v, want the chain to continue past the cleanup failure", err)
		}
		cleanupSessionStop(t, h, created.ID)
		if info := created.Info(); info.Provider != "codex" || info.State != StateActive {
			t.Fatalf("created = %s on %s, want active on the codex route", info.State, info.Provider)
		}
		failures := cleanupFailures(logs)
		if len(failures) != 2 {
			t.Fatalf("cleanup_failed records = %#v, want one per refused attempt", failures)
		}
		for index, record := range failures {
			if record.Level != slog.LevelWarn || record.Attrs["attempt"] != fmt.Sprint(index) ||
				record.Attrs["phase"] != fallbackPhaseCreate ||
				!strings.Contains(record.Attrs["error"], stopErr.Error()) {
				t.Fatalf(
					"cleanup_failed record %d = %#v, want warn attempt %d with the stop error",
					index,
					record,
					index,
				)
			}
		}
		if got := ledgerRows(t, db); got != 2 {
			t.Fatalf("ledger rows = %d, want one per fallback attempt", got)
		}
	})

	t.Run("Should join the refused-process cleanup failure into the exhausted error", func(t *testing.T) {
		t.Parallel()
		logs := newCaptureLogHandler()
		h, _ := newLedgerHarness(t, WithLogger(slog.New(logs)))
		stopErr := errors.New("refused seat process would not stop")
		codexCommand := h.cfg.Providers["codex"].Command
		refuseWithProcess(
			h,
			map[string]bool{fallbackSeatZero: true, fallbackSeatOne: true, codexCommand: true},
			stopErr,
		)
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{AgentName: "reviewer", Workspace: h.workspaceID})
		if err == nil || !strings.Contains(err.Error(), "fallback chain exhausted after 3 attempt(s)") {
			t.Fatalf("Create() error = %v, want deterministic exhaustion", err)
		}
		if !errors.Is(err, stopErr) {
			t.Fatalf("Create() error = %v, want the last attempt's cleanup failure joined", err)
		}
		if got := strings.Count(err.Error(), stopErr.Error()); got != 3 {
			t.Fatalf("Create() error names the cleanup failure %d time(s), want once per attempt: %v", got, err)
		}
		if got := len(cleanupFailures(logs)); got != 3 {
			t.Fatalf("cleanup_failed records = %d, want one per refused attempt", got)
		}
		metas := persistedSessionMetas(t, h)
		if len(metas) != 1 || metas[0].Failure == nil {
			t.Fatalf("persisted sessions = %#v, want exactly one failed start", metas)
		}
	})

	t.Run("Should never run the agent chain for a caller-owned launch", func(t *testing.T) {
		t.Parallel()
		h, db := newLedgerHarness(t)
		refuseStartCommands(h, map[string]error{fallbackSeatZero: rateLimitRefusal()})
		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "reviewer", Workspace: h.workspaceID, ChainOwner: ChainOwnerCaller,
		})
		if err == nil || StartAccepted(err) {
			t.Fatalf("Create(ChainOwnerCaller) error = %v, want the refused primary", err)
		}
		if got := startCommands(h); len(got) != 1 || ledgerRows(t, db) != 0 {
			t.Fatalf("caller-owned starts = %v rows = %d, want exactly the requested route and no event",
				got, ledgerRows(t, db))
		}
	})
}
