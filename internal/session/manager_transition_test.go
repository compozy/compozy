package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var errRecordingCatalogMissingSession = errors.New("recording catalog missing session")

func TestPromptRuntimeReappliesProviderDrift(t *testing.T) {
	t.Parallel()
	for _, stopFails := range []bool{false, true} {
		t.Run(
			fmt.Sprintf(
				"Should unbind failed rollback and recover the requested model with stop failure %t",
				stopFails,
			),
			func(t *testing.T) {
				t.Parallel()
				h := newHarness(t)
				driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
				h.manager.driver = driver
				active := createSession(t, h)
				t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
				initial := active.runtimeBindingSnapshot()
				selection := initial.selection
				selection.Model, selection.ReasoningEffort = "old-model", ""
				active.completeRuntimeTransition(initial.process, selection, RuntimeTransitionInitialBind, time.Now())
				initial.process.setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{
					{ID: "model", Kind: acp.SessionConfigOptionKindSelect, CurrentValueID: "old-model"},
				}})
				refusal := &acp.NegotiationError{
					Code:      acp.NegotiationCodeReasoningEffortUnsupported,
					Requested: "ultra",
				}
				rollbackErr := &acp.NegotiationError{Code: acp.NegotiationCodeModelUnavailable, Requested: "old-model"}
				driver.configHook = func(process *AgentProcess, config acp.RuntimeConfig) error {
					if config.Model == "new-model" {
						caps := process.CapsSnapshot()
						caps.ConfigOptions[0].CurrentValueID = "new-model"
						process.setCaps(caps)
						return refusal
					}
					return rollbackErr
				}
				stopErr := errors.New("provider process has not exited")
				if stopFails {
					h.driver.stopHook = func(*fakeProcess) error { return stopErr }
					t.Cleanup(func() {
						h.driver.mu.Lock()
						defer h.driver.mu.Unlock()
						h.driver.stopHook = nil
					})
				}
				changed := selection
				changed.Model, changed.ReasoningEffort = "new-model", "ultra"
				plan, err := h.manager.preparePromptRuntimePlan(t.Context(), active, changed)
				if err != nil {
					t.Fatal(err)
				}
				snapshot := active.runtimeBindingSnapshot()
				_, err = h.manager.configureOrReplacePromptRuntime(t.Context(), active, driver, &snapshot, plan)
				if !errors.Is(err, refusal) || !errors.Is(err, rollbackErr) {
					t.Fatalf("failed rollback = %v, want both provider errors", err)
				}
				if active.processHandle() != nil || active.Info().RuntimeStatus != RuntimeStatusUnbound ||
					active.Info().Model != "old-model" || active.Info().RuntimeFailure == "" {
					t.Fatal("failed rollback retained an unsafe binding or lost requested intent/diagnostic")
				}
				if stopFails {
					if !errors.Is(err, stopErr) || isProcessDone(initial.process) {
						t.Fatal("failed process stop was not retained")
					}
					if _, err := h.manager.Prompt(
						t.Context(),
						active.ID,
						"must stay blocked",
					); !errors.Is(
						err,
						stopErr,
					) {
						t.Fatalf("prompt before verified exit = %v, want stop failure", err)
					}
					h.driver.mu.Lock()
					h.driver.stopHook = nil
					h.driver.mu.Unlock()
				}
				events, err := h.manager.Prompt(t.Context(), active.ID, "use the original requested model")
				if err != nil {
					t.Fatal(err)
				}
				collectEvents(t, events)
				if err := h.manager.WaitForPromptDrains(t.Context()); err != nil {
					t.Fatal(err)
				}
				h.driver.mu.Lock()
				starts := slices.Clone(h.driver.startCalls)
				h.driver.mu.Unlock()
				if !isProcessDone(initial.process) || active.processHandle() == initial.process ||
					len(starts) != 2 || starts[1].PreferredModel != "old-model" || active.Info().Model != "old-model" {
					t.Fatalf("recovery did not replace the unsafe process with the requested model: starts=%#v", starts)
				}
			},
		)
	}
	t.Run(
		"Should preserve Fast intent and accept the next prompt after a failed change rolls back to normal",
		func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
			h.manager.driver = driver
			active := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
			snapshot := active.runtimeBindingSnapshot()
			selection := snapshot.selection
			selection.ReasoningEffort = "max"
			selection.Speed = speedpkg.SpeedFast
			active.completeRuntimeTransition(snapshot.process, selection, RuntimeTransitionInitialBind, time.Now())
			caps := acp.Caps{
				ConfigOptions: []acp.SessionConfigOption{
					{ID: "effort", Kind: acp.SessionConfigOptionKindSelect, CurrentValueID: "max"},
					{
						ID:          "fast",
						Category:    "model_config",
						Kind:        acp.SessionConfigOptionKindBoolean,
						CurrentBool: new(false),
					},
				},
				SpeedResolution: &speedpkg.Resolution{
					Requested: speedpkg.SpeedFast,
					Status:    speedpkg.ResolutionRejected,
					Reason:    speedpkg.ReasonProviderRejected,
				},
			}
			snapshot.process.setCaps(caps)
			refusal := &acp.NegotiationError{Code: acp.NegotiationCodeReasoningEffortUnsupported, Requested: "ultra"}
			driver.configHook = func(process *AgentProcess, config acp.RuntimeConfig) error {
				if config.ReasoningEffort == "ultra" {
					return refusal
				}
				observed := acp.CloneCaps(caps)
				observed.ConfigOptions[1].CurrentBool = new(config.Speed == speedpkg.SpeedFast)
				observed.SpeedResolution = &speedpkg.Resolution{
					Requested: config.Speed,
					Status:    speedpkg.ResolutionApplied,
				}
				process.setCaps(observed)
				return nil
			}
			changed := selection
			changed.ReasoningEffort = "ultra"
			_, err := h.manager.ensurePromptRuntime(t.Context(), active, &changed, snapshot.process)
			if !errors.Is(err, refusal) {
				t.Fatalf("failed change = %v, want provider refusal", err)
			}
			configs := driver.configSnapshots()
			if len(configs) != 2 || configs[1].Speed != speedpkg.SpeedNormal {
				t.Fatalf("rollback = %#v, want observed normal", configs)
			}
			info, meta := active.Info(), active.Meta()
			if info.Speed != speedpkg.SpeedFast || info.SpeedResolution.Requested != speedpkg.SpeedFast ||
				info.SpeedResolution.Status != speedpkg.ResolutionRejected || info.EffectiveRuntime().Speed != speedpkg.SpeedNormal ||
				meta.SpeedResolution.Requested != meta.Speed {
				t.Fatalf("rollback intent mismatch: info=%#v meta=%#v", info.SpeedResolution, meta.SpeedResolution)
			}
			events, err := h.manager.Prompt(t.Context(), active.ID, "retry the preserved request")
			if err != nil {
				t.Fatalf("next prompt after rollback = %v", err)
			}
			collectEvents(t, events)
			if err := h.manager.WaitForPromptDrains(t.Context()); err != nil {
				t.Fatal(err)
			}
			if active.runtimeBindingSnapshot().process != snapshot.process || len(driver.configSnapshots()) != 3 {
				t.Fatal("retry did not reconfigure the preserved process exactly once")
			}
		},
	)
	t.Run("Should replace a process after a transport failure wrapped by negotiation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
		h.manager.driver = driver
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		snapshot := active.runtimeBindingSnapshot()
		selection := snapshot.selection
		selection.Speed = speedpkg.SpeedFast
		driver.configErr = &acp.NegotiationError{
			Code: acp.NegotiationCodeSpeedRejected, Stage: "speed", Requested: "fast", Cause: io.EOF,
		}
		h.driver.mu.Lock()
		startsBefore := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		process, err := h.manager.ensurePromptRuntime(t.Context(), active, &selection, snapshot.process)
		h.driver.mu.Lock()
		startsAfter := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		if err != nil || process == snapshot.process || startsAfter != startsBefore+1 {
			t.Fatalf("transport recovery = %p, %v, starts %d -> %d", process, err, startsBefore, startsAfter)
		}
	})
	t.Run("Should reapply advertised speed with no confirmed current value", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
		h.manager.driver = driver
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		snapshot := active.runtimeBindingSnapshot()
		selection := snapshot.selection
		selection.ReasoningEffort = "high"
		selection.Speed = speedpkg.SpeedFast
		active.completeRuntimeTransition(snapshot.process, selection, RuntimeTransitionInitialBind, time.Now())
		snapshot.process.setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{
			{ID: "effort", Kind: acp.SessionConfigOptionKindSelect, CurrentValueID: "high"},
			{
				ID: "fast", Category: "model_config", Kind: acp.SessionConfigOptionKindSelect,
				Values: []acp.SessionConfigOptionValue{{Value: "on"}, {Value: "off"}},
			},
		}})
		process, err := h.manager.ensurePromptRuntime(t.Context(), active, &selection, snapshot.process)
		if err != nil || process != snapshot.process || len(driver.configSnapshots()) != 1 {
			t.Fatalf("unconfirmed speed = %p, %v, configs = %#v", process, err, driver.configSnapshots())
		}
		snapshot.process.setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{
			{ID: "effort", Kind: acp.SessionConfigOptionKindSelect, CurrentValueID: "high"},
		}})
		process, err = h.manager.ensurePromptRuntime(t.Context(), active, &selection, snapshot.process)
		if err != nil || process != snapshot.process || len(driver.configSnapshots()) != 1 {
			t.Fatalf("unsupported speed retried: %p, %v, configs = %#v", process, err, driver.configSnapshots())
		}
	})
	t.Run("Should reapply provider drift while preserving launch-bound controls", func(t *testing.T) {
		h := newHarness(t)
		driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
		h.manager.driver = driver
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		snapshot := active.runtimeBindingSnapshot()
		selection := snapshot.selection
		selection.Speed = speedpkg.SpeedFast
		selection.ReasoningEffort = "max"
		active.completeRuntimeTransition(snapshot.process, selection, RuntimeTransitionInitialBind, time.Now())
		snapshot.process.setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{
			{ID: "effort", Kind: acp.SessionConfigOptionKindSelect, CurrentValueID: "high"},
			{
				ID:             "fast",
				Category:       "model_config",
				Kind:           acp.SessionConfigOptionKindSelect,
				CurrentValueID: "off",
				Values:         []acp.SessionConfigOptionValue{{Value: "on"}, {Value: "off"}},
			},
		}})
		process, err := h.manager.ensurePromptRuntime(testutil.Context(t), active, &selection, snapshot.process)
		if err != nil {
			t.Fatal(err)
		}
		configs := driver.configSnapshots()
		if process != snapshot.process || len(configs) != 1 || configs[0].Speed != speedpkg.SpeedFast ||
			configs[0].ReasoningEffort != "max" {
			t.Fatalf("runtime configuration = %#v, want same-process reapplication of max and Fast", configs)
		}
		snapshot.process.setCaps(acp.Caps{})
		process, err = h.manager.ensurePromptRuntime(testutil.Context(t), active, &selection, snapshot.process)
		if err != nil || process != snapshot.process || len(driver.configSnapshots()) != 2 {
			t.Fatalf("missing selector runtime = %p, %v, want renegotiation of requested effort", process, err)
		}
		// Invariant: a permanent refusal gets one attempt per prompt, with no process replacement.
		snapshot.process.setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{
			{ID: "effort", Kind: acp.SessionConfigOptionKindSelect, CurrentValueID: "max"},
			{ID: "fast", Category: "model_config", Kind: acp.SessionConfigOptionKindBoolean, CurrentBool: new(false)},
		}})
		refusal := &acp.NegotiationError{Code: acp.NegotiationCodeSpeedRejected, Stage: "speed", Requested: "fast"}
		driver.configErr = refusal
		h.driver.mu.Lock()
		startsBefore := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		for attempt := range 2 {
			_, err = h.manager.ensurePromptRuntime(testutil.Context(t), active, &selection, snapshot.process)
			if !errors.Is(err, refusal) || len(driver.configSnapshots()) != 3+attempt {
				t.Fatalf("refused attempt %d = %v, configs = %#v", attempt, err, driver.configSnapshots())
			}
		}
		h.driver.mu.Lock()
		startsAfter := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		if startsAfter != startsBefore || active.runtimeBindingSnapshot().process != snapshot.process ||
			active.Info().Speed != speedpkg.SpeedFast || active.Info().RuntimeFailure == "" {
			t.Fatalf(
				"refusal replaced process, lost intent, or lost diagnostic: starts %d -> %d",
				startsBefore,
				startsAfter,
			)
		}
		// Cursor aliases apply these controls in the launch model, not ACP selectors.
		active.setProviderRouting(
			compozyconfig.ResolvedAgent{Provider: selection.Provider, RuntimeProvider: cursorRuntimeProvider},
		)
		process, err = h.manager.ensurePromptRuntime(testutil.Context(t), active, &selection, snapshot.process)
		if err != nil || process != snapshot.process || len(driver.configSnapshots()) != 4 {
			t.Fatalf("launch-bound runtime = %p, %v, want unchanged process without ACP reconfiguration", process, err)
		}
		got := active.Info().EffectiveRuntime()
		if got.Speed != selection.Speed || got.ReasoningEffort != selection.ReasoningEffort {
			t.Fatalf("launch-bound effective runtime = %#v, want selected launch configuration", got)
		}
	})
}

func TestPromptRuntimeKeepsRestrictedSessionAfterRuntimeChange(t *testing.T) {
	t.Parallel()
	t.Run("Should keep the launch-time full-access preference filtered during a runtime change", func(t *testing.T) {
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
		active, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID,
			Permissions: compozyconfig.PermissionModeApproveReads,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })

		driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
		h.manager.driver = driver
		snapshot := active.runtimeBindingSnapshot()
		selection := snapshot.selection
		selection.ACPOptions = []acp.SessionConfigOptionSelection{{ID: "thinking", BoolValue: new(true)}}
		if _, err := h.manager.ensurePromptRuntime(
			testutil.Context(t), active, &selection, snapshot.process,
		); err != nil {
			t.Fatalf("ensurePromptRuntime() error = %v", err)
		}
		configs := driver.configSnapshots()
		if len(configs) != 1 || len(configs[0].ACPOptions) != 1 ||
			configs[0].ACPOptions[0].ID != "thinking" || configs[0].ACPOptions[0].BoolValue == nil ||
			!*configs[0].ACPOptions[0].BoolValue {
			t.Fatalf("ConfigureRuntime() configs = %#v, want thinking=true without unrestricted mode", configs)
		}
	})
}

func TestPromptRuntimeRejectsUnrestrictedModeForPersistedRestrictedSession(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.cfg.Permissions.Mode = compozyconfig.PermissionModeApproveAll
	workspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	workspace.Config = h.cfg
	h.resolver.upsert(&workspace)
	active, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder", Workspace: h.workspaceID,
		Permissions: compozyconfig.PermissionModeApproveReads,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
	if got := active.Info().EffectivePermissions; got != string(compozyconfig.PermissionModeApproveReads) {
		t.Fatalf("effective permissions = %q, want approve-reads", got)
	}

	h.cfg.Permissions.ProviderFullAccess = true
	workspace.Config = h.cfg
	h.resolver.upsert(&workspace)
	driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
	h.manager.driver = driver
	snapshot := active.runtimeBindingSnapshot()
	selection := snapshot.selection
	selection.Model = "claude-fable-5"
	if _, err := h.manager.ensurePromptRuntime(
		testutil.Context(t), active, &selection, snapshot.process,
	); err == nil || !strings.Contains(err.Error(), "requires approve-all") {
		t.Fatalf("ensurePromptRuntime() error = %v, want restricted-session refusal", err)
	}
	if got := len(driver.configSnapshots()); got != 0 {
		t.Fatalf("ConfigureRuntime() calls = %d, want none", got)
	}
	if got := active.runtimeBindingSnapshot().process; got != snapshot.process {
		t.Fatal("restricted-session refusal changed the bound process")
	}
}

func TestPermissionInteractionPayloadShouldPreserveCanonicalToolID(t *testing.T) {
	t.Parallel()

	payload, title := permissionInteractionPayload(acp.AgentEvent{
		Title: "Terminal Exec",
		Raw: []byte(`{
			"tool_id":"compozy__terminal_exec",
			"options":[{"decision":"allow-once"}],
			"tool_call":{"title":"Terminal Exec"}
		}`),
	})
	if got, want := title, "Terminal Exec"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
	if got, want := payload.ToolID, "compozy__terminal_exec"; got != want {
		t.Fatalf("tool id = %q, want %q", got, want)
	}
	if got, want := payload.Decisions, []string{"allow-once"}; !slices.Equal(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

// TestManagerLifecycleCatalogTransitions keeps memory, persisted metadata, and catalog transitions consistent.
func TestManagerLifecycleCatalogTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should retain the creation witness across initial binding and provider replacement", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		ctx := t.Context()
		workspace, err := h.resolver.Resolve(ctx, h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		for index := range workspace.Agents {
			workspace.Agents[index].Provider = "codex"
			workspace.Agents[index].Model = "gpt-6.1-sol"
		}
		h.resolver.upsert(&workspace)
		created, err := h.manager.CreateAccepted(ctx, CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, created.ID) })
		active, ok := h.manager.Get(created.ID)
		if !ok {
			t.Fatal("logical session is unavailable")
		}
		meta := active.Meta()
		before := creationIdentityFromMeta(&meta)
		if before == nil || meta.CreationProfile == nil || meta.CreationProfile.Provider != "codex" ||
			active.Info().RuntimeStatus != RuntimeStatusUnbound {
			t.Fatalf("logical creation = %#v, want unbound Codex creation witness", meta)
		}
		for _, test := range []struct {
			runtime  RuntimeSelection
			strategy RuntimeTransitionStrategy
		}{
			{RuntimeSelection{Provider: "claude", Model: "claude-opus-5-5", ReasoningEffort: "xhigh"}, RuntimeTransitionInitialBind},
			{RuntimeSelection{Provider: "codex", Model: "gpt-6.1-sol", ReasoningEffort: "medium"}, RuntimeTransitionProcessReplacement},
		} {
			previous := active.runtimeBindingSnapshot()
			process, err := h.manager.ensurePromptRuntime(ctx, active, &test.runtime, previous.process)
			if err != nil {
				t.Fatal(err)
			}
			current := active.runtimeBindingSnapshot()
			persisted := readMeta(t, active.MetaPath())
			after := creationIdentityFromMeta(&persisted)
			if current.transition != test.strategy || process == previous.process ||
				persisted.Provider != test.runtime.Provider || after == nil || *after != *before {
				t.Fatalf("runtime transition = %#v, creation identity = %#v; want %s and %#v",
					current, after, test.strategy, before)
			}
		}
		previous := active.runtimeBindingSnapshot()
		invalid := RuntimeSelection{Provider: "missing-provider", Model: "missing-model"}
		_, err = h.manager.ensurePromptRuntime(ctx, active, &invalid, previous.process)
		if !errors.Is(err, compozyconfig.ErrProviderUnavailable) {
			t.Fatalf("invalid provider error = %v, want ErrProviderUnavailable", err)
		}
		if current := active.runtimeBindingSnapshot(); current.process != previous.process ||
			current.selection.Provider != previous.selection.Provider {
			t.Fatal("rejected provider replaced the accepted runtime")
		}
	})

	t.Run("Should pass prompt ACP options through live runtime configuration", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
		h.manager.driver = driver
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })

		snapshot := active.runtimeBindingSnapshot()
		selection := snapshot.selection
		selection.ACPOptions = []acp.SessionConfigOptionSelection{{ID: "thinking", BoolValue: new(true)}}
		process, err := h.manager.ensurePromptRuntime(
			testutil.Context(t),
			active,
			&selection,
			snapshot.process,
		)
		if err != nil {
			t.Fatalf("ensurePromptRuntime() error = %v", err)
		}
		if process != snapshot.process {
			t.Fatal("ensurePromptRuntime() replaced a session-config process, want live reconfiguration")
		}
		configs := driver.configSnapshots()
		if len(configs) != 1 || len(configs[0].ACPOptions) != 1 ||
			configs[0].ACPOptions[0].ID != "thinking" || configs[0].ACPOptions[0].BoolValue == nil ||
			!*configs[0].ACPOptions[0].BoolValue {
			t.Fatalf("ConfigureRuntime() configs = %#v, want thinking=true", configs)
		}
		if got := active.Info().ACPOptions; len(got) != 1 || got[0].ID != "thinking" ||
			got[0].BoolValue == nil || !*got[0].BoolValue {
			t.Fatalf("active ACP options = %#v, want thinking=true", got)
		}
	})

	t.Run("Should replace Cursor process when a launch-bound ACP option changes", func(t *testing.T) {
		t.Parallel()

		const logicalModel = "opus-5"
		const regularTransport = "claude-opus-5-low"
		const thinkingTransport = "claude-opus-5-thinking-high"
		thinkingOff := false
		thinkingOn := true
		h := newHarness(t, WithModelCatalog(modelCatalogStub{models: []modelcatalog.Model{{
			ProviderID:        cursorRuntimeProvider,
			ModelID:           logicalModel,
			AvailabilityState: modelcatalog.AvailabilityStateAvailableLive,
			ConfigOptions: []modelcatalog.ModelOptionDescriptor{{
				ID: "thinking", Kind: modelcatalog.ModelOptionKindBoolean, CurrentBool: &thinkingOff,
			}},
			TransportBindings: []modelcatalog.ModelTransportBinding{
				{
					TransportModelID: regularTransport,
					Thinking:         &thinkingOff,
					OptionSelections: []modelcatalog.ModelOptionSelection{{ID: "thinking", BoolValue: &thinkingOff}},
				},
				{
					TransportModelID: thinkingTransport,
					Thinking:         &thinkingOn,
					OptionSelections: []modelcatalog.ModelOptionSelection{{ID: "thinking", BoolValue: &thinkingOn}},
				},
			},
			Sources: []modelcatalog.SourceRef{
				{
					SourceID: modelcatalog.SourceKindProviderLiveID(
						cursorRuntimeProvider,
					),
					SourceKind: modelcatalog.SourceKindProviderLive,
				},
			},
		}}}))
		resolvedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		for index := range resolvedWorkspace.Agents {
			if resolvedWorkspace.Agents[index].Name == "coder" {
				resolvedWorkspace.Agents[index].Provider = cursorRuntimeProvider
				resolvedWorkspace.Agents[index].Model = logicalModel
			}
		}
		h.resolver.upsert(&resolvedWorkspace)

		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		snapshot := active.runtimeBindingSnapshot()
		selection := snapshot.selection
		selection.ACPOptions = []acp.SessionConfigOptionSelection{{ID: "thinking", BoolValue: &thinkingOn}}
		replacement, err := h.manager.ensurePromptRuntime(
			testutil.Context(t),
			active,
			&selection,
			snapshot.process,
		)
		if err != nil {
			t.Fatalf("ensurePromptRuntime() error = %v", err)
		}
		if replacement == snapshot.process {
			t.Fatal("ensurePromptRuntime() reused process, want launch-bound replacement")
		}

		h.driver.mu.Lock()
		startCalls := append([]acp.StartOpts(nil), h.driver.startCalls...)
		stopCalls := h.driver.stopCalls
		h.driver.mu.Unlock()
		if len(startCalls) != 2 || startCalls[0].LaunchModelID != regularTransport ||
			startCalls[1].LaunchModelID != thinkingTransport {
			t.Fatalf("Cursor starts = %#v, want regular then thinking transport", startCalls)
		}
		if stopCalls != 1 {
			t.Fatalf("Cursor replacement stop calls = %d, want 1", stopCalls)
		}
		info := active.Info()
		if info.Model != logicalModel || len(info.ACPOptions) != 1 ||
			info.ACPOptions[0].BoolValue == nil || !*info.ACPOptions[0].BoolValue {
			t.Fatalf(
				"active runtime = model %q options %#v, want logical model with thinking=true",
				info.Model,
				info.ACPOptions,
			)
		}
	})

	t.Run("Should expire both decision kinds after restart without allowing a late answer", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []string{store.PendingInteractionKindPermission, store.PendingInteractionKindClarify} {
			t.Run("Should expire "+kind, func(t *testing.T) {
				t.Parallel()
				_, restarted, catalog, sessionID := newOrphanResolutionHarness(
					t,
					kind,
					store.PendingInteractionPayload{},
				)
				if _, err := catalog.CreatePendingInteraction(t.Context(), store.PendingInteractionCreate{
					InteractionID:     "interaction-pending",
					SessionID:         sessionID,
					Kind:              kind,
					ProviderRequestID: "request-pending",
					TurnID:            "turn-lost",
					Title:             "Pending decision",
					CreatedAt:         restarted.now(),
				}); err != nil {
					t.Fatal(err)
				}
				count, err := restarted.ExpireRestartInteractions(t.Context(), sessionID)
				if err != nil || count != 2 {
					t.Fatalf("expire = %d, %v", count, err)
				}
				rows, err := restarted.PendingInteractions(
					t.Context(),
					sessionID,
					[]string{store.PendingInteractionStatusCanceled},
				)
				if err != nil || len(rows) != 2 {
					t.Fatalf("expired decisions: %#v, %v", rows, err)
				}
				if rows[0].Resolution != "failed-by-restart" || rows[0].ResolvedBy != "system" ||
					rows[0].ResolvedAt == nil {
					t.Fatalf("restart attribution: %#v", rows[0])
				}
				late, err := restarted.transitionPendingInteraction(t.Context(), store.PendingInteractionTransition{
					InteractionID: rows[0].InteractionID, Status: store.PendingInteractionStatusResolved,
					Resolution: "allow", ResolvedBy: "operator", At: restarted.now(),
				})
				if err != nil || late.Outcome != store.PendingInteractionOutcomeAlreadyResolved ||
					late.Interaction.Status != store.PendingInteractionStatusCanceled {
					t.Fatalf("late answer changed restart expiry: %#v, %v", late, err)
				}
				attention, err := catalog.GetSessionAttention(t.Context(), sessionID)
				if err != nil || attention.PendingPermissionCount != 0 || attention.PendingClarifyCount != 0 {
					t.Fatalf("expired attention: %#v, %v", attention, err)
				}
				if count, err := restarted.ExpireRestartInteractions(t.Context(), sessionID); err != nil || count != 0 {
					t.Fatalf("repeated expiry: %d, %v", count, err)
				}
			})
		}
	})
	t.Run("Should persist canceled permissions and clear their attention without restart", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		h := newHarness(t)
		if err := h.manager.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
		catalog := openManagerInputQueueStore(t)
		registerManagerInputQueueWorkspace(t, catalog, h)
		h.manager = newManagerWithHarness(t, h, WithSessionCatalog(catalog))
		cleanupTestManager(t, h.manager)
		target := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, target.ID) })
		at := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
		event := acp.AgentEvent{
			Type: acp.EventTypePermission, TurnID: "turn-canceled", Title: "Stop the review queue", Timestamp: at,
		}.WithRequestID("request-canceled")
		if err := h.manager.recordEvent(ctx, target, event); err != nil {
			t.Fatal(err)
		}
		if got := BadgeForInfo(target.Info()); got != BadgeWaitingForAuth {
			t.Fatalf("pending badge = %q, want waiting-for-auth", got)
		}
		event.Decision = "canceled"
		event.Timestamp = at.Add(time.Second)
		event = event.WithResolvedBy("system")
		if err := h.manager.recordEvent(ctx, target, event); err != nil {
			t.Fatal(err)
		}
		rows, err := catalog.ListPendingInteractions(ctx, target.ID, []string{store.PendingInteractionStatusCanceled})
		if err != nil || len(rows) != 1 {
			t.Fatalf("canceled permissions = %#v, %v, want one durable record", rows, err)
		}
		if rows[0].ProviderRequestID != "request-canceled" || rows[0].Resolution != "canceled" ||
			rows[0].ResolvedBy != "system" || rows[0].ResolvedAt == nil {
			t.Fatalf("cancellation attribution = %#v", rows[0])
		}
		attention, err := catalog.GetSessionAttention(ctx, target.ID)
		if err != nil || attention.PendingPermissionCount != 0 {
			t.Fatalf("durable attention = %#v, %v, want no pending permission", attention, err)
		}
		if got := BadgeForInfo(target.Info()); got == BadgeWaitingForAuth {
			t.Fatal("canceled permission retained the live approval badge")
		}
	})

	t.Run("Should atomically resolve an orphaned permission and preserve the first winner", func(t *testing.T) {
		t.Parallel()

		_, restarted, catalog, sessionID := newOrphanResolutionHarness(
			t,
			store.PendingInteractionKindPermission,
			store.PendingInteractionPayload{Decisions: []string{"allow-once", "reject-once"}},
		)
		ctx := WithActingSession(testutil.Context(t), "sess-creator")
		result, err := restarted.ApprovePermission(ctx, sessionID, acp.ApproveRequest{
			RequestID: "request-orphan",
			Decision:  "allow-once",
		})
		if err != nil {
			t.Fatalf("ApprovePermission(orphan) error = %v", err)
		}
		if result.Outcome != store.PendingInteractionOutcomeResolvedAfterRestart ||
			result.ResolvedDecision != "allow-once" {
			t.Fatalf("ApprovePermission(orphan) = %#v", result)
		}
		assertOrphanResolutionQueue(t, catalog, sessionID, "Response: allow-once")
		resolved, err := catalog.ListPendingInteractions(
			ctx,
			sessionID,
			[]string{store.PendingInteractionStatusResolved},
		)
		if err != nil {
			t.Fatalf("ListPendingInteractions() error = %v", err)
		}
		if len(resolved) != 1 || resolved[0].ResolvedBy != "agent_session:sess-creator" {
			t.Fatalf("resolved interaction = %#v, want acting session", resolved)
		}

		duplicate, err := restarted.ApprovePermission(ctx, sessionID, acp.ApproveRequest{
			RequestID: "request-orphan",
			Decision:  "reject-once",
		})
		if err != nil {
			t.Fatalf("ApprovePermission(duplicate) error = %v", err)
		}
		if duplicate.Outcome != store.PendingInteractionOutcomeAlreadyResolved ||
			duplicate.ResolvedDecision != "allow-once" {
			t.Fatalf("ApprovePermission(duplicate) = %#v", duplicate)
		}
		assertOrphanResolutionQueue(t, catalog, sessionID, "Response: allow-once")
	})

	t.Run("Should report the durable winner when live permission resolution races", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		h := newHarness(t)
		if err := h.manager.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown(initial manager) error = %v", err)
		}
		catalog := openManagerInputQueueStore(t)
		registerManagerInputQueueWorkspace(t, catalog, h)
		h.manager = newManagerWithHarness(t, h, WithSessionCatalog(catalog))
		cleanupTestManager(t, h.manager)
		target := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, target.ID) })

		createdAt := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
		if _, err := catalog.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
			InteractionID:     "interaction-race",
			SessionID:         target.ID,
			Kind:              store.PendingInteractionKindPermission,
			ProviderRequestID: "request-race",
			TurnID:            "turn-race",
			Title:             "Allow the command?",
			Payload:           store.PendingInteractionPayload{Decisions: []string{"allow-once", "reject-once"}},
			CreatedAt:         createdAt,
		}); err != nil {
			t.Fatalf("CreatePendingInteraction() error = %v", err)
		}
		h.driver.approveHook = func(*fakeProcess, acp.ApproveRequest) error {
			if _, err := catalog.TransitionPendingInteraction(ctx, store.PendingInteractionTransition{
				InteractionID: "interaction-race",
				Status:        store.PendingInteractionStatusResolved,
				Resolution:    "reject-once",
				ResolvedBy:    "operator:competing-client",
				At:            createdAt.Add(time.Second),
			}); err != nil {
				return err
			}
			return acp.ErrPendingPermissionConflict
		}

		result, err := h.manager.ApprovePermission(ctx, target.ID, acp.ApproveRequest{
			RequestID: "request-race",
			Decision:  "allow-once",
		})
		if err != nil {
			t.Fatalf("ApprovePermission(race) error = %v", err)
		}
		if result.Outcome != store.PendingInteractionOutcomeAlreadyResolved ||
			result.ResolvedDecision != "reject-once" {
			t.Fatalf("ApprovePermission(race) = %#v, want durable winner", result)
		}
	})

	t.Run("Should atomically resolve an orphaned clarification with its validated choice", func(t *testing.T) {
		t.Parallel()

		h, restarted, catalog, sessionID := newOrphanResolutionHarness(
			t,
			store.PendingInteractionKindClarify,
			store.PendingInteractionPayload{Choices: []string{"Fast", "Safe"}},
		)
		ctx := WithActingSession(testutil.Context(t), "sess-creator")
		choice := 1
		result, err := restarted.ResolveOrphanedClarification(
			ctx,
			toolspkg.Scope{
				WorkspaceID: h.workspaceID,
				SessionID:   sessionID,
				AgentName:   "coder",
				Operator:    true,
			},
			"request-orphan",
			toolspkg.ClarifyAnswerRequest{ChoiceIndex: &choice},
		)
		if err != nil {
			t.Fatalf("ResolveOrphanedClarification() error = %v", err)
		}
		if result.Outcome != store.PendingInteractionOutcomeResolvedAfterRestart ||
			result.Choice == nil || *result.Choice != choice || result.ResolvedAnswer != "Safe" {
			t.Fatalf("ResolveOrphanedClarification() = %#v", result)
		}
		assertOrphanResolutionQueue(t, catalog, sessionID, "Response: Safe")
		resolved, err := catalog.ListPendingInteractions(
			ctx,
			sessionID,
			[]string{store.PendingInteractionStatusResolved},
		)
		if err != nil {
			t.Fatalf("ListPendingInteractions() error = %v", err)
		}
		if len(resolved) != 1 || resolved[0].ResolvedBy != "agent_session:sess-creator" {
			t.Fatalf("resolved interaction = %#v, want acting session", resolved)
		}

		other := 0
		duplicate, err := restarted.ResolveOrphanedClarification(
			ctx,
			toolspkg.Scope{
				WorkspaceID: h.workspaceID,
				SessionID:   sessionID,
				AgentName:   "coder",
				Operator:    true,
			},
			"request-orphan",
			toolspkg.ClarifyAnswerRequest{ChoiceIndex: &other},
		)
		if err != nil {
			t.Fatalf("ResolveOrphanedClarification(duplicate) error = %v", err)
		}
		if duplicate.Outcome != store.PendingInteractionOutcomeAlreadyResolved ||
			duplicate.Choice == nil || *duplicate.Choice != choice || duplicate.ResolvedAnswer != "Safe" {
			t.Fatalf("ResolveOrphanedClarification(duplicate) = %#v", duplicate)
		}
	})

	t.Run("Should hydrate durable attention badges in a fresh manager", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		h := newHarness(t)
		cleanupTestManager(t, h.manager)
		catalog := openManagerInputQueueStore(t)
		registerManagerInputQueueWorkspace(t, catalog, h)
		baseAt := time.Date(2026, 8, 15, 19, 0, 0, 0, time.UTC)
		register := func(id string) {
			t.Helper()
			if err := catalog.RegisterSession(ctx, store.SessionInfo{
				ID: id, Name: id, AgentName: "coder", ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID,
				SessionType: string(SessionTypeUser), State: string(StateActive),
				RuntimeStatus: store.SessionRuntimeReady, CreatedAt: baseAt, UpdatedAt: baseAt,
			}); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", id, err)
			}
		}
		for index, testCase := range []struct {
			id    string
			kind  string
			badge Badge
		}{
			{id: "sess-restart-permission", kind: store.PendingInteractionKindPermission, badge: BadgeWaitingForAuth},
			{id: "sess-restart-clarify", kind: store.PendingInteractionKindClarify, badge: BadgeWaitingForInput},
		} {
			register(testCase.id)
			if _, err := catalog.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
				InteractionID: "interaction-" + testCase.id, SessionID: testCase.id,
				Kind: testCase.kind, ProviderRequestID: "request-" + testCase.id,
				Title: "Restart interaction", CreatedAt: baseAt.Add(time.Duration(index+1) * time.Second),
			}); err != nil {
				t.Fatalf("CreatePendingInteraction(%q) error = %v", testCase.id, err)
			}
		}
		register("sess-restart-done")
		if _, err := catalog.MarkSessionSettled(
			ctx,
			"sess-restart-done",
			false,
			baseAt.Add(3*time.Second),
		); err != nil {
			t.Fatalf("MarkSessionSettled(done) error = %v", err)
		}

		restarted := newManagerWithHarness(t, h, WithSessionCatalog(catalog))
		cleanupTestManager(t, restarted)
		for _, testCase := range []struct {
			id    string
			badge Badge
		}{
			{id: "sess-restart-permission", badge: BadgeWaitingForAuth},
			{id: "sess-restart-clarify", badge: BadgeWaitingForInput},
			{id: "sess-restart-done", badge: BadgeDone},
		} {
			target := &Session{ID: testCase.id, State: StateActive}
			if err := restarted.hydrateSessionAttention(ctx, target); err != nil {
				t.Fatalf("hydrateSessionAttention(%q) error = %v", testCase.id, err)
			}
			if got := BadgeForInfo(target.Info()); got != testCase.badge {
				t.Fatalf("BadgeForInfo(%q) = %q, want %q", testCase.id, got, testCase.badge)
			}
		}
		for _, transition := range []store.PendingInteractionTransition{
			{
				InteractionID: "interaction-sess-restart-permission",
				Status:        store.PendingInteractionStatusResolved,
				Resolution:    "allow-once",
				ResolvedBy:    "operator:test",
				At:            baseAt.Add(4 * time.Second),
			},
			{
				InteractionID: "interaction-sess-restart-clarify",
				Status:        store.PendingInteractionStatusTimedOut,
				At:            baseAt.Add(4 * time.Second),
			},
		} {
			if _, err := catalog.TransitionPendingInteraction(ctx, transition); err != nil {
				t.Fatalf("TransitionPendingInteraction(%q) error = %v", transition.InteractionID, err)
			}
			targetID := strings.TrimPrefix(transition.InteractionID, "interaction-")
			target := &Session{ID: targetID, State: StateActive}
			if err := restarted.hydrateSessionAttention(ctx, target); err != nil {
				t.Fatalf("hydrateSessionAttention(%q after clear) error = %v", targetID, err)
			}
			if got := BadgeForInfo(target.Info()); got != BadgeIdle {
				t.Fatalf("BadgeForInfo(%q after clear) = %q, want %q", targetID, got, BadgeIdle)
			}
		}

		if _, err := catalog.MarkSessionSeen(ctx, "sess-restart-done", baseAt.Add(5*time.Second)); err != nil {
			t.Fatalf("MarkSessionSeen() error = %v", err)
		}
		seen := &Session{ID: "sess-restart-done", State: StateActive}
		if err := restarted.hydrateSessionAttention(ctx, seen); err != nil {
			t.Fatalf("hydrateSessionAttention(seen) error = %v", err)
		}
		if got := BadgeForInfo(seen.Info()); got != BadgeIdle {
			t.Fatalf("BadgeForInfo(seen restart) = %q, want %q", got, BadgeIdle)
		}
	})

	t.Run("Should keep memory meta and catalog consistent on stopping transition", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)
		// Observe the stopping snapshot before asynchronous cancellation advances runtime metadata.
		release := make(chan struct{})
		cancelStarted := make(chan struct{})
		signalStarted := sync.OnceFunc(func() { close(cancelStarted) })
		unblock := sync.OnceFunc(func() { close(release) })
		h.driver.cancelHook = func(*fakeProcess) error { signalStarted(); <-release; return nil }
		t.Cleanup(func() {
			unblock()
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})
		ctx := testutil.Context(t)
		if err := h.manager.RequestStopWithCause(ctx, session.ID, CauseUserRequested, ""); err != nil {
			t.Fatalf("RequestStopWithCause() error = %v", err)
		}
		select {
		case <-cancelStarted:
		case <-ctx.Done():
			t.Fatal("cancellation hook did not start:", ctx.Err())
		}

		if got, want := session.Info().State, StateStopping; got != want {
			t.Fatalf("memory state after request stop = %q, want %q", got, want)
		}
		stoppingMeta := readMeta(t, session.MetaPath())
		if stoppingMeta.State != string(StateStopping) {
			t.Fatalf("meta state after request stop = %q, want %q", stoppingMeta.State, StateStopping)
		}
		stoppingCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after request stop", session.ID)
		}
		if stoppingCatalog.State != string(StateStopping) {
			t.Fatalf("catalog state after request stop = %q, want %q", stoppingCatalog.State, StateStopping)
		}
		stoppingAttention := stoppingCatalog.AttentionSnapshot()
		if stoppingAttention.AttentionRevision != 1 || stoppingAttention.AttentionChangedAt == nil ||
			!stoppingAttention.AttentionChangedAt.Equal(stoppingCatalog.UpdatedAt) {
			t.Fatalf("catalog attention after request stop = %#v, want revision 1 at updated_at", stoppingCatalog)
		}
		if stoppingCatalog.Provider != stoppingMeta.Provider || stoppingCatalog.Model != stoppingMeta.Model ||
			stoppingCatalog.ReasoningEffort != stoppingMeta.ReasoningEffort || stoppingCatalog.Speed != stoppingMeta.Speed ||
			stoppingCatalog.RuntimeStatus != stoppingMeta.RuntimeStatus ||
			stoppingCatalog.RuntimeTransition != stoppingMeta.RuntimeTransition ||
			stoppingCatalog.RuntimeFailure != stoppingMeta.RuntimeFailureValue() {
			t.Fatalf(
				"catalog runtime after request stop = %#v, want metadata runtime %#v",
				stoppingCatalog,
				stoppingMeta,
			)
		}
	})

	t.Run("Should leave metadata as recovery source when stopping catalog update fails", func(t *testing.T) {
		t.Parallel()

		updateErr := errors.New("catalog update failed")
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)
		catalog.setUpdateErr(updateErr)

		err := h.manager.RequestStopWithCause(testutil.Context(t), session.ID, CauseUserRequested, "")
		if !errors.Is(err, updateErr) {
			t.Fatalf("RequestStopWithCause() error = %v, want wrapped catalog error", err)
		}
		t.Cleanup(func() {
			catalog.setUpdateErr(nil)
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})

		stoppingMeta := readMeta(t, session.MetaPath())
		if stoppingMeta.State != string(StateStopping) {
			t.Fatalf("meta state after catalog failure = %q, want %q", stoppingMeta.State, StateStopping)
		}
		activeCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after failed update", session.ID)
		}
		if activeCatalog.State != string(StateActive) {
			t.Fatalf("catalog state after failed update = %q, want %q", activeCatalog.State, StateActive)
		}

		catalog.setUpdateErr(nil)
		_, reconcileErr := catalog.ReconcileSessions(testutil.Context(t), []store.SessionInfo{{
			ID:          session.ID,
			WorkspaceID: stoppingMeta.WorkspaceID,
			State:       stoppingMeta.State,
			UpdatedAt:   stoppingMeta.UpdatedAt,
		}})
		if reconcileErr != nil {
			t.Fatalf("ReconcileSessions() error = %v", reconcileErr)
		}
		reconciledCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after reconcile", session.ID)
		}
		if reconciledCatalog.State != string(StateStopping) {
			t.Fatalf("catalog state after reconcile = %q, want %q", reconciledCatalog.State, StateStopping)
		}
	})

	t.Run("Should keep memory meta and catalog consistent on active and stopped transitions", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)

		activeInfo := session.Info()
		if activeInfo.State != StateActive {
			t.Fatalf("memory state after create = %q, want %q", activeInfo.State, StateActive)
		}
		activeMeta := readMeta(t, session.MetaPath())
		if activeMeta.State != string(StateActive) {
			t.Fatalf("meta state after create = %q, want %q", activeMeta.State, StateActive)
		}
		activeCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after create", session.ID)
		}
		if activeCatalog.State != string(StateActive) {
			t.Fatalf("catalog state after create = %q, want %q", activeCatalog.State, StateActive)
		}

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stoppedMeta := readMeta(t, session.MetaPath())
		if stoppedMeta.State != string(StateStopped) {
			t.Fatalf("meta state after stop = %q, want %q", stoppedMeta.State, StateStopped)
		}
		stoppedCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after stop", session.ID)
		}
		if stoppedCatalog.State != string(StateStopped) {
			t.Fatalf("catalog state after stop = %q, want %q", stoppedCatalog.State, StateStopped)
		}
		stoppedAttention := stoppedCatalog.AttentionSnapshot()
		if stoppedAttention.AttentionRevision == 0 || stoppedAttention.AttentionChangedAt == nil {
			t.Fatalf("catalog attention after stop = %#v, want durable lifecycle edge", stoppedCatalog)
		}
	})

	t.Run("Should register stopped startup failure when create fails before active registration", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		catalog.requireExistingUpdates()
		h := newHarness(t, WithSessionCatalog(catalog))
		startErr := errors.New("driver start failed")
		h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
			return nil, startErr
		}

		_, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "failed",
			Workspace: h.workspaceID,
		})
		if err == nil {
			t.Fatal("Create() error = nil, want startup failure")
		}
		if errors.Is(err, errRecordingCatalogMissingSession) {
			t.Fatalf("Create() error = %v, want failed start to register catalog row", err)
		}

		failedMeta := readMeta(t, store.SessionMetaFile(filepath.Join(h.homePaths.SessionsDir, "sess-1")))
		assertStartupFailureMeta(t, &failedMeta)
		failedCatalog, ok := catalog.get("sess-1")
		if !ok {
			t.Fatal("catalog missing failed create session sess-1")
		}
		assertStartupFailureCatalog(t, failedCatalog)
	})

	t.Run("Should restore catalog projection from metadata after resume start failure", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		beforeMeta := readMeta(t, session.MetaPath())
		beforeCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing stopped session %q before resume", session.ID)
		}
		resumeErr := errors.New("resume failed")
		h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
			return nil, resumeErr
		}

		_, err := h.manager.Resume(testutil.Context(t), session.ID)
		if !errors.Is(err, resumeErr) {
			t.Fatalf("Resume() error = %v, want wrapped resume failure", err)
		}
		afterMeta := readMeta(t, session.MetaPath())
		afterCatalog, ok := catalog.get(session.ID)
		if !ok {
			t.Fatalf("catalog missing session %q after failed resume", session.ID)
		}
		assertRestoredResumeMeta(t, &afterMeta, &beforeMeta)
		assertRestoredResumeCatalog(t, afterCatalog, beforeCatalog)
	})

	t.Run("Should reject archive after a resume transition begins", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		stopped := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), stopped.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		archiveReadStarted := make(chan struct{})
		releaseArchiveRead := make(chan struct{})
		var archiveReadOnce sync.Once
		catalog.archiveReadHook = func() {
			archiveReadOnce.Do(func() { close(archiveReadStarted) })
			<-releaseArchiveRead
		}
		resumeResult := make(chan error, 1)
		go func() {
			_, resumeErr := h.manager.Resume(testutil.Context(t), stopped.ID)
			resumeResult <- resumeErr
		}()
		<-archiveReadStarted

		_, archiveErr := h.manager.Archive(testutil.Context(t), h.workspaceID, stopped.ID)
		if !errors.Is(archiveErr, ErrSessionArchiveRequiresStopped) {
			t.Fatalf("Archive() error = %v, want ErrSessionArchiveRequiresStopped", archiveErr)
		}
		close(releaseArchiveRead)
		if resumeErr := <-resumeResult; resumeErr != nil {
			t.Fatalf("Resume() error = %v", resumeErr)
		}
		if err := h.manager.Stop(testutil.Context(t), stopped.ID); err != nil {
			t.Fatalf("Stop(resumed) error = %v", err)
		}
	})

	t.Run("Should synchronize a successful attach CAS into the active session", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})
		attachedAt := active.Info().UpdatedAt.Add(time.Minute)
		attach, err := h.manager.AttachSession(testutil.Context(t), store.SessionAttachRequest{
			SessionID:  active.ID,
			AttachedTo: "uds:operator",
			Now:        attachedAt,
			TTL:        time.Minute,
		})
		if err != nil {
			t.Fatalf("AttachSession() error = %v", err)
		}
		info := active.Info()
		if info.AttachedTo != attach.AttachedTo || info.AttachExpiresAt == nil ||
			!info.AttachExpiresAt.Equal(attach.AttachExpiresAt) || !info.UpdatedAt.Equal(attachedAt) {
			t.Fatalf("active info after attach = %#v, want synchronized lock %#v", info, attach)
		}
		if AttachableForInfo(info, attachedAt) {
			t.Fatal("AttachableForInfo(attached) = true, want locked")
		}
		healthByID, err := h.manager.SessionHealthForPage(testutil.Context(t), []*Info{info})
		if err != nil {
			t.Fatalf("SessionHealthForPage() error = %v", err)
		}
		if healthByID[active.ID].Attachable {
			t.Fatalf("session health after attach = %#v, want not attachable", healthByID[active.ID])
		}
	})

	t.Run("Should leave active memory unchanged when attach CAS fails", func(t *testing.T) {
		t.Parallel()

		attachErr := errors.New("attach CAS failed")
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})
		before := active.Info()
		catalog.setAttachErr(attachErr)
		_, err := h.manager.AttachSession(testutil.Context(t), store.SessionAttachRequest{
			SessionID:  active.ID,
			AttachedTo: "uds:operator",
			Now:        before.UpdatedAt.Add(time.Minute),
			TTL:        time.Minute,
		})
		if !errors.Is(err, attachErr) {
			t.Fatalf("AttachSession() error = %v, want wrapped CAS failure", err)
		}
		after := active.Info()
		if after.AttachedTo != before.AttachedTo || after.AttachExpiresAt != nil ||
			!after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Fatalf("active info after failed CAS = %#v, want unchanged %#v", after, before)
		}
	})

	t.Run("Should persist selected runtime without changing the active effective runtime", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop() cleanup error = %v", err)
			}
		})
		before := active.Info()
		updated, err := h.manager.SetRuntimeSelection(
			testutil.Context(t),
			active.ID,
			RuntimeSelection{Provider: "claude", Model: "claude-fable-5", ReasoningEffort: "max"},
			0,
		)
		if err != nil {
			t.Fatalf("SetRuntimeSelection() error = %v", err)
		}
		if updated.SelectedRuntime == nil || updated.SelectedRuntime.Provider != "claude" ||
			updated.SelectedRuntime.Model != "claude-fable-5" ||
			updated.SelectedRuntime.ReasoningEffort != "max" || updated.RuntimeSelectionRevision != 1 {
			t.Fatalf("SetRuntimeSelection() info = %#v, want Claude selection at revision 1", updated)
		}
		if updated.Provider != before.Provider || updated.Model != before.Model ||
			updated.ReasoningEffort != before.ReasoningEffort || updated.RuntimeStatus != before.RuntimeStatus {
			t.Fatalf("effective runtime after selection = %#v, want unchanged %#v", updated, before)
		}
		meta := readMeta(t, active.MetaPath())
		selectedRuntime, selectionRevision := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelectionValue())
		catalogInfo, ok := catalog.get(active.ID)
		if !ok || selectedRuntime == nil || catalogInfo.SelectedRuntime == nil ||
			selectionRevision != 1 || catalogInfo.RuntimeSelectionRevision != 1 {
			t.Fatalf(
				"durable selection meta = %#v catalog = %#v, want revision 1",
				selectedRuntime,
				catalogInfo.SelectedRuntime,
			)
		}
		_, err = h.manager.ClearRuntimeSelection(testutil.Context(t), active.ID, 0)
		if !errors.Is(err, ErrRuntimeSelectionConflict) {
			t.Fatalf("ClearRuntimeSelection(stale revision) error = %v, want revision conflict", err)
		}
		cleared, err := h.manager.ClearRuntimeSelection(testutil.Context(t), active.ID, 1)
		if err != nil {
			t.Fatalf("ClearRuntimeSelection(active) error = %v", err)
		}
		if cleared.State != StateActive || cleared.SelectedRuntime != nil ||
			cleared.RuntimeSelectionRevision != 2 {
			t.Fatalf("ClearRuntimeSelection(active) info = %#v, want active clear at revision 2", cleared)
		}
		clearedMeta := readMeta(t, active.MetaPath())
		clearedSelection, clearedRevision := store.SessionRuntimeSelectionStateValues(
			clearedMeta.RuntimeSelectionValue(),
		)
		clearedCatalog, ok := catalog.get(active.ID)
		if !ok || clearedSelection != nil || clearedCatalog.SelectedRuntime != nil ||
			clearedRevision != 2 || clearedCatalog.RuntimeSelectionRevision != 2 {
			t.Fatalf(
				"cleared active selection meta = %#v catalog = %#v, want revision 2",
				clearedSelection,
				clearedCatalog.SelectedRuntime,
			)
		}
	})

	for _, providerID := range []string{"cursor", "cursor-secondary"} {
		t.Run("Should persist only an advertised Cursor runtime selection for "+providerID, func(t *testing.T) {
			t.Parallel()

			const advertisedModel = "grok-4.5"
			const transportModel = "cursor-grok-4.5-high"
			h := newHarness(t, WithModelCatalog(modelCatalogStub{models: []modelcatalog.Model{{
				ProviderID:             providerID,
				ModelID:                advertisedModel,
				AvailabilityState:      modelcatalog.AvailabilityStateAvailableLive,
				DefaultReasoningEffort: new(modelcatalog.ReasoningEffortHigh),
				TransportBindings: []modelcatalog.ModelTransportBinding{{
					TransportModelID: transportModel,
					ReasoningEffort:  new(modelcatalog.ReasoningEffortHigh),
					Fast:             new(false),
				}},
				Sources: []modelcatalog.SourceRef{{
					SourceID:   modelcatalog.SourceKindProviderLiveID(providerID),
					SourceKind: modelcatalog.SourceKindProviderLive,
				}},
			}}}))
			resolvedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			provider := compozyconfig.BuiltinProviders()["cursor"]
			provider.RuntimeProvider = "cursor"
			resolvedWorkspace.Config.Providers[providerID] = provider
			for index := range resolvedWorkspace.Agents {
				if resolvedWorkspace.Agents[index].Name == "coder" {
					resolvedWorkspace.Agents[index].Provider = providerID
					resolvedWorkspace.Agents[index].Model = ""
				}
			}
			h.resolver.upsert(&resolvedWorkspace)
			h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
				proc := newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-cursor-runtime-selection")
				proc.handle.setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{{
					ID:             "model",
					Category:       "model",
					Kind:           acp.SessionConfigOptionKindSelect,
					CurrentValueID: transportModel,
					Values:         []acp.SessionConfigOptionValue{{Value: transportModel}},
				}}})
				return proc, nil
			}
			active := createSession(t, h)
			t.Cleanup(func() {
				if err := h.manager.Stop(
					testutil.Context(t),
					active.ID,
				); err != nil &&
					!errors.Is(err, ErrSessionNotFound) {
					t.Errorf("Stop() cleanup error = %v", err)
				}
			})

			_, err = h.manager.SetRuntimeSelection(
				testutil.Context(t),
				active.ID,
				RuntimeSelection{Provider: providerID, Model: "cursor-grok-4.5-high"},
				0,
			)
			negotiationErr, ok := errors.AsType[*acp.NegotiationError](err)
			if !ok || negotiationErr.Code != acp.NegotiationCodeModelUnavailable {
				t.Fatalf("SetRuntimeSelection(alias) error = %v, want model_unavailable NegotiationError", err)
			}
			rejectedMeta := readMeta(t, active.MetaPath())
			rejectedSelection, rejectedRevision := store.SessionRuntimeSelectionStateValues(
				rejectedMeta.RuntimeSelectionValue(),
			)
			if rejectedSelection != nil || rejectedRevision != 0 {
				t.Fatalf(
					"rejected Cursor selection = %#v at revision %d, want no persisted selection",
					rejectedSelection,
					rejectedRevision,
				)
			}

			updated, err := h.manager.SetRuntimeSelection(
				testutil.Context(t),
				active.ID,
				RuntimeSelection{Provider: providerID, Model: advertisedModel},
				0,
			)
			if err != nil {
				t.Fatalf("SetRuntimeSelection(advertised model) error = %v", err)
			}
			if updated.SelectedRuntime == nil || updated.SelectedRuntime.Model != advertisedModel ||
				updated.RuntimeSelectionRevision != 1 {
				t.Fatalf(
					"SetRuntimeSelection(advertised model) info = %#v, want exact persisted model at revision 1",
					updated,
				)
			}
			if err := h.manager.Stop(t.Context(), active.ID); err != nil {
				t.Fatal(err)
			}
			_, err = h.manager.SetRuntimeSelection(
				t.Context(),
				active.ID,
				RuntimeSelection{Provider: providerID, Model: transportModel},
				1,
			)
			negotiationErr, ok = errors.AsType[*acp.NegotiationError](err)
			if !ok || negotiationErr.Code != acp.NegotiationCodeModelUnavailable {
				t.Fatalf("stopped selection error = %v", err)
			}
		})
	}

	t.Run("Should update selected runtime while stopped without starting ACP", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		h.driver.mu.Lock()
		startsBefore := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		updated, err := h.manager.SetRuntimeSelection(
			testutil.Context(t),
			active.ID,
			RuntimeSelection{Provider: "claude", Model: "claude-fable-5", ReasoningEffort: "max"},
			0,
		)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(stopped) error = %v", err)
		}
		if updated.State != StateStopped || updated.SelectedRuntime == nil ||
			updated.SelectedRuntime.Model != "claude-fable-5" || updated.RuntimeSelectionRevision != 1 {
			t.Fatalf("SetRuntimeSelection(stopped) info = %#v, want stopped Claude selection at revision 1", updated)
		}
		cleared, err := h.manager.ClearRuntimeSelection(testutil.Context(t), active.ID, 1)
		if err != nil {
			t.Fatalf("ClearRuntimeSelection(stopped) error = %v", err)
		}
		if cleared.State != StateStopped || cleared.SelectedRuntime != nil ||
			cleared.RuntimeSelectionRevision != 2 {
			t.Fatalf("ClearRuntimeSelection(stopped) info = %#v, want stopped clear at revision 2", cleared)
		}
		clearedMeta := readMeta(t, active.MetaPath())
		clearedSelection, clearedRevision := store.SessionRuntimeSelectionStateValues(
			clearedMeta.RuntimeSelectionValue(),
		)
		clearedCatalog, ok := catalog.get(active.ID)
		if !ok || clearedSelection != nil || clearedCatalog.SelectedRuntime != nil ||
			clearedRevision != 2 || clearedCatalog.RuntimeSelectionRevision != 2 {
			t.Fatalf(
				"cleared stopped selection meta = %#v catalog = %#v, want revision 2",
				clearedSelection,
				clearedCatalog.SelectedRuntime,
			)
		}
		h.driver.mu.Lock()
		startsAfter := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		if got := startsAfter; got != startsBefore {
			t.Fatalf("driver starts after stopped selection = %d, want %d", got, startsBefore)
		}
	})

	t.Run("Should rename an active user session across memory metadata catalog and events", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		events, cancel, err := h.manager.SubscribeSessionCatalogEvents(
			testutil.Context(t),
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()

		updated, err := h.manager.Rename(
			testutil.Context(t),
			h.workspaceID,
			active.ID,
			"  Checkout\n  webhook   retries  ",
		)
		if err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
		const want = "Checkout webhook retries"
		catalogInfo, ok := catalog.get(active.ID)
		if updated.Name != want || active.Info().Name != want ||
			readMeta(t, active.MetaPath()).Name != want || !ok || catalogInfo.Name != want {
			t.Fatalf("renamed identity = info %q memory %q catalog %#v", updated.Name, active.Info().Name, catalogInfo)
		}
		select {
		case event := <-events:
			if event.Kind != CatalogEventUpserted ||
				event.WorkspaceID != h.workspaceID ||
				event.SessionID != active.ID {
				t.Fatalf("rename catalog event = %#v, want upserted %s/%s", event, h.workspaceID, active.ID)
			}
		case <-time.After(time.Second):
			t.Fatal("rename catalog event was not published")
		}
	})

	t.Run("Should rename a stopped archived session without starting ACP or losing archive state", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if _, err := h.manager.Archive(testutil.Context(t), h.workspaceID, active.ID); err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		h.driver.mu.Lock()
		startsBefore := len(h.driver.startCalls)
		h.driver.mu.Unlock()

		updated, err := h.manager.Rename(testutil.Context(t), h.workspaceID, active.ID, "Archived checkout")
		if err != nil {
			t.Fatalf("Rename(stopped) error = %v", err)
		}
		if updated.Name != "Archived checkout" || updated.ArchivedAt == nil {
			t.Fatalf("Rename(stopped) = %#v, want renamed archived session", updated)
		}
		catalogInfo, ok := catalog.get(active.ID)
		if !ok || catalogInfo.Name != updated.Name || catalogInfo.ArchivedAt == nil {
			t.Fatalf("archived catalog after rename = %#v", catalogInfo)
		}
		h.driver.mu.Lock()
		startsAfter := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		if startsAfter != startsBefore {
			t.Fatalf("driver starts after stopped rename = %d, want %d", startsAfter, startsBefore)
		}
	})

	t.Run("Should reject invalid manual names managed sessions and foreign workspaces", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		for _, test := range []struct {
			name        string
			workspaceID string
			value       string
			want        error
		}{
			{name: "empty", workspaceID: h.workspaceID, value: " \n\t ", want: ErrValidation},
			{name: "too long", workspaceID: h.workspaceID, value: strings.Repeat("界", SessionNameMaxRunes+1), want: ErrValidation},
			{name: "foreign workspace", workspaceID: "ws-foreign", value: "Private", want: ErrSessionNotFound},
		} {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				t.Parallel()
				_, err := h.manager.Rename(testutil.Context(t), test.workspaceID, active.ID, test.value)
				if !errors.Is(err, test.want) {
					t.Fatalf("Rename() error = %v, want %v", err, test.want)
				}
			})
		}

		managed, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeSystem,
		})
		if err != nil {
			t.Fatalf("Create(system) error = %v", err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, managed.ID) })
		_, err = h.manager.Rename(
			testutil.Context(t),
			h.workspaceID,
			managed.ID,
			"Operator name",
		)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("Rename(system) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should roll back an active rename when catalog persistence fails", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		before := active.Info()
		persistErr := errors.New("catalog rename unavailable")
		catalog.registerHook = func(_ context.Context, info store.SessionInfo) error {
			if info.Name == "New name" {
				return persistErr
			}
			return nil
		}

		_, err := h.manager.Rename(testutil.Context(t), h.workspaceID, active.ID, "New name")
		if !errors.Is(err, persistErr) {
			t.Fatalf("Rename() error = %v, want catalog failure", err)
		}
		memoryName := active.Info().Name
		metaName := readMeta(t, active.MetaPath()).Name
		if memoryName != before.Name || metaName != before.Name {
			t.Fatalf("failed rename changed identity: memory=%q meta=%q", memoryName, metaName)
		}
	})
}

type runtimeConfigRecordingDriver struct {
	*fakeDriver
	configMu   sync.Mutex
	configs    []acp.RuntimeConfig
	configErr  error
	configHook func(*AgentProcess, acp.RuntimeConfig) error
}

func (d *runtimeConfigRecordingDriver) ConfigureRuntime(
	_ context.Context,
	process *AgentProcess,
	config acp.RuntimeConfig,
) error {
	d.configMu.Lock()
	defer d.configMu.Unlock()
	config.ACPOptions = acp.CloneSessionConfigOptionSelections(config.ACPOptions)
	d.configs = append(d.configs, config)
	if d.configHook != nil {
		return d.configHook(process, config)
	}
	return d.configErr
}

func (d *runtimeConfigRecordingDriver) configSnapshots() []acp.RuntimeConfig {
	d.configMu.Lock()
	defer d.configMu.Unlock()
	result := append([]acp.RuntimeConfig(nil), d.configs...)
	for index := range result {
		result[index].ACPOptions = acp.CloneSessionConfigOptionSelections(result[index].ACPOptions)
	}
	return result
}

func newOrphanResolutionHarness(
	t *testing.T,
	kind string,
	payload store.PendingInteractionPayload,
) (*harness, *Manager, *globaldb.GlobalDB, string) {
	t.Helper()

	ctx := testutil.Context(t)
	h := newHarness(t)
	cleanupTestManager(t, h.manager)
	target := createSession(t, h)
	if err := h.manager.Stop(ctx, target.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	catalog := openManagerInputQueueStore(t)
	registerManagerInputQueueWorkspace(t, catalog, h)
	registerManagerInputQueueSession(t, catalog, h, target)
	if _, err := catalog.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
		InteractionID:     "interaction-orphan",
		SessionID:         target.ID,
		Kind:              kind,
		ProviderRequestID: "request-orphan",
		TurnID:            "turn-orphan",
		Title:             "Which path?",
		Payload:           payload,
		CreatedAt:         time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreatePendingInteraction() error = %v", err)
	}
	restarted := newManagerWithHarness(
		t,
		h,
		WithSessionCatalog(catalog),
		WithSessionInputQueueStore(catalog),
	)
	cleanupTestManager(t, restarted)
	orphaned, err := restarted.RecoverPendingInteractions(ctx, target.ID)
	if err != nil {
		t.Fatalf("RecoverPendingInteractions() error = %v", err)
	}
	if orphaned != 1 {
		t.Fatalf("RecoverPendingInteractions() = %d, want 1", orphaned)
	}
	return h, restarted, catalog, target.ID
}

func assertOrphanResolutionQueue(
	t *testing.T,
	queueStore store.SessionInputQueueStore,
	sessionID string,
	wantText string,
) {
	t.Helper()

	entries, err := queueStore.ListPendingSessionInputs(testutil.Context(t), sessionID)
	if err != nil {
		t.Fatalf("ListPendingSessionInputs() error = %v", err)
	}
	if len(entries) != 1 || !strings.Contains(entries[0].Text, wantText) {
		t.Fatalf("queued orphan resolution = %#v, want one entry containing %q", entries, wantText)
	}
}

type recordingSessionCatalog struct {
	mu              sync.Mutex
	sessions        map[string]store.SessionInfo
	registerHook    func(context.Context, store.SessionInfo) error
	updateErr       error
	updateHook      func(store.SessionStateUpdate) error
	deleteErr       error
	deleteHook      func(context.Context, string) error
	attachErr       error
	archiveReadHook func()
	strictUpdates   bool
}

func newRecordingSessionCatalog() *recordingSessionCatalog {
	return &recordingSessionCatalog{sessions: make(map[string]store.SessionInfo)}
}

func (c *recordingSessionCatalog) RegisterSession(ctx context.Context, session store.SessionInfo) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.registerHook != nil {
		if err := c.registerHook(ctx, session); err != nil {
			return err
		}
	}
	c.sessions[session.ID] = session
	return nil
}

func (c *recordingSessionCatalog) UpdateSessionState(_ context.Context, update store.SessionStateUpdate) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.updateHook != nil {
		if err := c.updateHook(update); err != nil {
			return err
		}
	}
	if c.updateErr != nil {
		return c.updateErr
	}
	current, ok := c.sessions[update.ID]
	if c.strictUpdates && !ok {
		return errRecordingCatalogMissingSession
	}
	current.State = update.State
	current.ACPSessionID = update.ACPSessionID
	if update.RuntimeSet {
		current.Provider = update.Provider
		current.Model = update.Model
		current.ReasoningEffort = update.ReasoningEffort
		current.Speed = update.Speed
		current.SpeedResolution = speedpkg.CloneResolution(update.SpeedResolution)
		current.RuntimeStatus = update.RuntimeStatus
		current.RuntimeTransition = update.RuntimeTransition
		current.RuntimeFailure = update.RuntimeFailure
	}
	if update.SelectedRuntimeSet {
		current.SelectedRuntime = store.CloneSessionRuntimeSelection(update.SelectedRuntime)
		current.RuntimeSelectionRevision = update.RuntimeSelectionRevision
	}
	if update.StopReasonSet {
		current.StopReason = store.StopReason("")
		if update.StopReason != nil {
			current.StopReason = store.StopReason(*update.StopReason)
		}
		current.StopDetail = update.StopDetail
	}
	if update.FailureSet {
		current.Failure = store.CloneSessionFailure(update.Failure)
	}
	if update.AttentionTransition {
		attention := current.AttentionSnapshot()
		attention.AttentionRevision++
		changedAt := update.UpdatedAt.UTC()
		attention.AttentionChangedAt = &changedAt
		current.Attention = &attention
	}
	current.Liveness = store.CloneSessionLivenessMeta(update.Liveness)
	current.UpdatedAt = update.UpdatedAt
	c.sessions[update.ID] = current
	return nil
}

func (c *recordingSessionCatalog) DeleteSession(ctx context.Context, id string) error {
	if c.deleteHook != nil {
		if err := c.deleteHook(ctx, id); err != nil {
			return err
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deleteErr != nil {
		return c.deleteErr
	}
	if _, ok := c.sessions[id]; !ok {
		return store.ErrSessionNotFound
	}
	delete(c.sessions, id)
	return nil
}

func (c *recordingSessionCatalog) SessionArchivedAt(
	_ context.Context,
	workspaceID string,
	sessionID string,
) (*time.Time, error) {
	if c.archiveReadHook != nil {
		c.archiveReadHook()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.sessions[sessionID]
	if !ok || current.WorkspaceID != workspaceID {
		return nil, fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	return cloneTimePointer(current.ArchivedAt), nil
}

func (c *recordingSessionCatalog) SetSessionArchived(
	_ context.Context,
	workspaceID string,
	sessionID string,
	archived bool,
) (store.SessionInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.sessions[sessionID]
	if !ok || current.WorkspaceID != workspaceID {
		return store.SessionInfo{}, fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	if archived && current.State != string(StateStopped) {
		return store.SessionInfo{}, fmt.Errorf("%w: %s", store.ErrSessionArchiveRequiresStopped, sessionID)
	}
	if archived == (current.ArchivedAt != nil) {
		return current, nil
	}
	if archived {
		now := time.Now().UTC()
		current.ArchivedAt = &now
	} else {
		current.ArchivedAt = nil
	}
	c.sessions[sessionID] = current
	return current, nil
}
func (c *recordingSessionCatalog) ListSessions(
	_ context.Context,
	query store.SessionListQuery,
) ([]store.SessionInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]store.SessionInfo, 0, len(c.sessions))
	for _, session := range c.sessions {
		if query.WorkspaceID != "" && session.WorkspaceID != query.WorkspaceID {
			continue
		}
		out = append(out, session)
	}
	return out, nil
}

func (c *recordingSessionCatalog) AttachSession(
	_ context.Context,
	req store.SessionAttachRequest,
) (store.SessionAttach, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attachErr != nil {
		return store.SessionAttach{}, c.attachErr
	}
	normalized := req.Normalize()
	current, ok := c.sessions[normalized.SessionID]
	if !ok {
		return store.SessionAttach{}, store.ErrSessionNotFound
	}
	if current.State != string(StateActive) {
		return store.SessionAttach{}, store.ErrSessionNotAttachable
	}
	attachExpiresAt := current.AttachExpiresAtValue()
	if current.AttachedToValue() != "" && attachExpiresAt != nil && attachExpiresAt.After(normalized.Now) {
		return store.SessionAttach{}, store.ErrSessionAttachLocked
	}
	expiresAt := normalized.Now.Add(normalized.TTL).UTC()
	current.SetAttach(normalized.AttachedTo, &expiresAt)
	current.UpdatedAt = normalized.Now
	c.sessions[normalized.SessionID] = current
	return store.SessionAttach{
		SessionID:       normalized.SessionID,
		AttachedTo:      normalized.AttachedTo,
		AttachedAt:      normalized.Now,
		AttachExpiresAt: expiresAt,
	}, nil
}

func (c *recordingSessionCatalog) ReconcileSessions(
	_ context.Context,
	sessions []store.SessionInfo,
) (store.ReconcileResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, session := range sessions {
		c.sessions[session.ID] = session
	}
	indexed := make([]string, 0, len(sessions))
	for _, session := range sessions {
		indexed = append(indexed, session.ID)
	}
	return store.ReconcileResult{Indexed: indexed}, nil
}

func (c *recordingSessionCatalog) get(id string) (store.SessionInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	session, ok := c.sessions[id]
	return session, ok
}

func (c *recordingSessionCatalog) setUpdateErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateErr = err
}

func (c *recordingSessionCatalog) setAttachErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.attachErr = err
}

func (c *recordingSessionCatalog) requireExistingUpdates() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.strictUpdates = true
}

func assertStartupFailureMeta(t *testing.T, meta *store.SessionMeta) {
	t.Helper()

	if meta.State != string(StateStopped) {
		t.Fatalf("failed meta state = %q, want %q", meta.State, StateStopped)
	}
	if meta.Failure == nil {
		t.Fatal("failed meta failure = nil, want startup failure")
	}
	if got, want := meta.Failure.Kind, store.FailureStartup; got != want {
		t.Fatalf("failed meta failure kind = %q, want %q", got, want)
	}
}

func assertStartupFailureCatalog(t *testing.T, info store.SessionInfo) {
	t.Helper()

	if info.State != string(StateStopped) {
		t.Fatalf("failed catalog state = %q, want %q", info.State, StateStopped)
	}
	if info.Failure == nil {
		t.Fatal("failed catalog failure = nil, want startup failure")
	}
	if got, want := info.Failure.Kind, store.FailureStartup; got != want {
		t.Fatalf("failed catalog failure kind = %q, want %q", got, want)
	}
}

func assertRestoredResumeMeta(t *testing.T, after, before *store.SessionMeta) {
	t.Helper()

	if after.State != before.State {
		t.Fatalf("restored meta state = %q, want %q", after.State, before.State)
	}
	if derefString(after.ACPSessionID) != derefString(before.ACPSessionID) {
		t.Fatalf(
			"restored meta acp session id = %q, want %q",
			derefString(after.ACPSessionID),
			derefString(before.ACPSessionID),
		)
	}
	if sessionMetaStopReason(after) != sessionMetaStopReason(before) {
		t.Fatalf(
			"restored meta stop reason = %q, want %q",
			sessionMetaStopReason(after),
			sessionMetaStopReason(before),
		)
	}
	assertSameSessionFailure(t, after.Failure, before.Failure, "restored meta")
}

func assertRestoredResumeCatalog(t *testing.T, after store.SessionInfo, before store.SessionInfo) {
	t.Helper()

	if after.State != before.State {
		t.Fatalf("restored catalog state = %q, want %q", after.State, before.State)
	}
	if derefString(after.ACPSessionID) != derefString(before.ACPSessionID) {
		t.Fatalf(
			"restored catalog acp session id = %q, want %q",
			derefString(after.ACPSessionID),
			derefString(before.ACPSessionID),
		)
	}
	if after.StopReason != before.StopReason {
		t.Fatalf("restored catalog stop reason = %q, want %q", after.StopReason, before.StopReason)
	}
	if after.StopDetail != before.StopDetail {
		t.Fatalf("restored catalog stop detail = %q, want %q", after.StopDetail, before.StopDetail)
	}
	assertSameSessionFailure(t, after.Failure, before.Failure, "restored catalog")
}

func assertSameSessionFailure(
	t *testing.T,
	after *store.SessionFailure,
	before *store.SessionFailure,
	label string,
) {
	t.Helper()

	if before == nil || after == nil {
		if before != after {
			t.Fatalf("%s failure = %#v, want %#v", label, after, before)
		}
		return
	}
	if after.Kind != before.Kind {
		t.Fatalf("%s failure kind = %q, want %q", label, after.Kind, before.Kind)
	}
	if after.Summary != before.Summary {
		t.Fatalf("%s failure summary = %q, want %q", label, after.Summary, before.Summary)
	}
}

func TestPromptRuntimePiStartupRecovery(t *testing.T) {
	t.Parallel()
	t.Run("Should clear unavailable tools when a non-Pi replacement commits", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		active.mu.Lock()
		active.nativeToolsUnavailable = true
		active.mu.Unlock()
		plan, err := h.manager.preparePromptRuntimePlan(t.Context(), active,
			RuntimeSelection{Provider: "codex", Model: "gpt-6.1-sol"})
		if err != nil {
			t.Fatal(err)
		}
		snapshot := active.runtimeBindingSnapshot()
		if _, err := h.manager.replacePromptRuntime(t.Context(), active, &snapshot, plan); err != nil {
			t.Fatal(err)
		}
		if active.NativeToolsUnavailable() {
			t.Fatal("non-Pi runtime retained failed Pi availability")
		}
	})

	t.Run("Should rebuild from authored instructions on every Pi recovery", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		const overlay = "Focus only on tests."
		active, err := h.manager.Create(t.Context(), CreateOpts{
			AgentName: "coder", Name: "session", Workspace: h.workspaceID, PromptOverlay: overlay,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		original := active.AgentDefinition()
		for range 2 {
			prior := active.AgentDefinition()
			prior.Prompt = original.Prompt + "\nPreviously assembled native tool instructions."
			active.setAgentDefinition(prior, acp.OpaqueStartupManifest(prior.Prompt, false))
			plan, err := h.manager.preparePromptRuntimePlan(
				t.Context(),
				active,
				RuntimeSelection{Provider: "pi", Model: "test-model"},
			)
			if err != nil {
				t.Fatal(err)
			}
			opts := h.manager.preparePiBridgeStartup(&plan.spec, active, &plan.runtime, acp.StartOpts{})
			prompt, _, err := opts.FinalizeStartup(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(prompt, original.Prompt) || strings.Count(prompt, overlay) != 1 ||
				strings.Contains(prompt, "Previously assembled") || !active.NativeToolsUnavailable() {
				t.Fatalf("recovery retained stale guidance: %q", prompt)
			}
		}
	})
}
