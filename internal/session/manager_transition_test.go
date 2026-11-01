package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/modelcatalog"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/transcript"
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

func TestRuntimeSelectionAppliesLive(t *testing.T) {
	t.Parallel()

	t.Run("Should apply a same-provider effort change to the idle process immediately", func(t *testing.T) {
		t.Parallel()
		h, driver, active := newLiveRuntimeHarness(t, runtimeProviderClaude, nil)
		updated, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "medium"), 0)
		if err != nil {
			t.Fatalf("SetRuntimeSelection() error = %v", err)
		}
		configs := driver.configSnapshots()
		if len(configs) != 1 || configs[0].ReasoningEffort != "medium" {
			t.Fatalf("ConfigureRuntime() configs = %#v, want one live medium apply", configs)
		}
		if updated.ReasoningEffort != "medium" || updated.RuntimeStatus != RuntimeStatusReady ||
			updated.RuntimeTransition != RuntimeTransitionLiveConfiguration || updated.RuntimeFailure != "" {
			t.Fatalf("SetRuntimeSelection() info = %#v, want effective medium via live configuration", updated)
		}
		if meta := readMeta(t, active.MetaPath()); meta.ReasoningEffort != "medium" {
			t.Fatalf("durable effective effort = %q, want medium", meta.ReasoningEffort)
		}
		markers := transcriptMarkersOfKind(t, h.manager, active.ID, transcript.MarkerRuntimeChanged)
		if len(markers) != 1 || markers[0].Evidence["reasoning_effort"] != "medium" ||
			markers[0].Evidence["previous_reasoning_effort"] != "xhigh" ||
			markers[0].Evidence["during_turn"] != false ||
			markers[0].Evidence["live_apply"] != string(acp.RuntimeLiveApplyNextStep) {
			t.Fatalf("runtime changed markers = %#v, want one idle xhigh->medium marker", markers)
		}

		events, err := h.manager.Prompt(t.Context(), active.ID, "next prompt keeps the live runtime")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, events)
		h.driver.mu.Lock()
		starts := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		if got := len(driver.configSnapshots()); got != 1 || starts != 1 {
			t.Fatalf("prompt after live apply configs=%d starts=%d, want no reapply or replacement", got, starts)
		}
	})

	t.Run("Should apply mid-turn without a status flip, replacement, or turn cancellation", func(t *testing.T) {
		t.Parallel()
		h, driver, active := newLiveRuntimeHarness(t, runtimeProviderClaude, nil)
		process := active.processHandle()
		events, turnID, release := startBlockedLivePrompt(t, h, active)

		updated, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "medium"), 0)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(mid-turn) error = %v", err)
		}
		if got := driver.configSnapshots(); len(got) != 1 || got[0].ReasoningEffort != "medium" {
			t.Fatalf("mid-turn configs = %#v, want immediate medium apply", got)
		}
		if updated.ReasoningEffort != "medium" || updated.RuntimeStatus != RuntimeStatusReady ||
			active.processHandle() != process || !active.IsPrompting() {
			t.Fatalf("mid-turn info = %#v, want ready effective medium on the same running process", updated)
		}
		markers := transcriptMarkersOfKind(t, h.manager, active.ID, transcript.MarkerRuntimeChanged)
		if len(markers) != 1 || markers[0].Evidence["during_turn"] != true {
			t.Fatalf("mid-turn markers = %#v, want one during-turn marker", markers)
		}
		if marker := liveRuntimeMarkerEvent(t, h, active); marker.TurnID != turnID {
			t.Fatalf("mid-turn marker turn = %q, want running turn %q", marker.TurnID, turnID)
		}

		release()
		if last := collectEvents(t, events); len(last) == 0 || last[len(last)-1].Type != acp.EventTypeDone {
			t.Fatalf("turn events = %#v, want the running turn to finish", last)
		}
		if effort := liveAgentMessageEffort(t, h, active, turnID); effort != "medium" {
			t.Fatalf("next-step provider message effort = %q, want the applied medium", effort)
		}
	})

	t.Run("Should keep the running turn's runtime for a provider that honors changes next turn", func(t *testing.T) {
		t.Parallel()
		h, driver, active := newLiveRuntimeHarness(t, liveRuntimeNextTurnProvider, nil)
		events, turnID, release := startBlockedLivePrompt(t, h, active)
		updated, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "medium"), 0)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(mid-turn codex) error = %v", err)
		}
		if got := driver.configSnapshots(); len(got) != 1 || updated.ReasoningEffort != "medium" {
			t.Fatalf("codex configs = %#v info = %#v, want the change sent and configured", got, updated)
		}
		markers := transcriptMarkersOfKind(t, h.manager, active.ID, transcript.MarkerRuntimeChanged)
		if len(markers) != 1 || markers[0].Evidence["live_apply"] != string(acp.RuntimeLiveApplyNextTurn) {
			t.Fatalf("codex markers = %#v, want next_turn semantics", markers)
		}
		release()
		collectEvents(t, events)
		if effort := liveAgentMessageEffort(t, h, active, turnID); effort != "xhigh" {
			t.Fatalf("next-turn provider message effort = %q, want the running turn's xhigh", effort)
		}
	})

	t.Run("Should attribute a prompt waiting on a live apply to the applied runtime", func(t *testing.T) {
		t.Parallel()
		entered, unblock := make(chan struct{}), make(chan struct{})
		var once sync.Once
		blockMedium := func(config acp.RuntimeConfig) error {
			if config.ReasoningEffort == "medium" {
				once.Do(func() { close(entered) })
				<-unblock
			}
			return nil
		}
		h, _, active := newLiveRuntimeHarness(t, liveRuntimeNextTurnProvider, blockMedium)
		applied := make(chan error, 1)
		go func() {
			_, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "medium"), 0)
			applied <- err
		}()
		waitLiveApplyEntered(t, entered)
		h.driver.mu.Lock()
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 2)
			sessionID := active.Info().ACPSessionID
			events <- acp.AgentEvent{
				Type: acp.EventTypeAgentMessage, SessionID: sessionID, TurnID: req.TurnID, Text: "after the change",
			}
			events <- acp.AgentEvent{Type: acp.EventTypeDone, SessionID: sessionID, TurnID: req.TurnID}
			close(events)
			return events, nil
		}
		h.driver.mu.Unlock()
		prompted := make(chan string, 1)
		go func() {
			events, err := h.manager.Prompt(t.Context(), active.ID, "admitted behind the live apply")
			if err != nil {
				t.Errorf("Prompt() error = %v", err)
				prompted <- ""
				return
			}
			collectEvents(t, events)
			prompted <- "done"
		}()
		var turnID string
		deadline := time.Now().Add(5 * time.Second)
		for turnID == "" && time.Now().Before(deadline) {
			turnID = active.CurrentTurnID()
			time.Sleep(time.Millisecond)
		}
		close(unblock)
		if err := <-applied; err != nil {
			t.Fatalf("SetRuntimeSelection() error = %v", err)
		}
		if outcome := <-prompted; outcome != "done" || turnID == "" {
			t.Fatalf("prompt outcome = %q turn = %q, want a dispatched turn", outcome, turnID)
		}
		if effort := liveAgentMessageEffort(t, h, active, turnID); effort != "medium" {
			t.Fatalf("waiting prompt message effort = %q, want the applied medium", effort)
		}
		markers := transcriptMarkersOfKind(t, h.manager, active.ID, transcript.MarkerRuntimeChanged)
		if len(markers) != 1 || markers[0].Evidence["during_turn"] != false {
			t.Fatalf("markers = %#v, want no during-turn claim for an undispatched prompt", markers)
		}
	})

	t.Run("Should leave a canceled prompt waiting on a live apply", func(t *testing.T) {
		t.Parallel()
		entered, unblock := make(chan struct{}), make(chan struct{})
		var once sync.Once
		blockMedium := func(config acp.RuntimeConfig) error {
			if config.ReasoningEffort == "medium" {
				once.Do(func() { close(entered) })
				<-unblock
			}
			return nil
		}
		h, _, active := newLiveRuntimeHarness(t, runtimeProviderClaude, blockMedium)
		applied := make(chan error, 1)
		go func() {
			_, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "medium"), 0)
			applied <- err
		}()
		waitLiveApplyEntered(t, entered)
		process := active.processHandle()
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := h.manager.ensurePromptRuntime(canceled, active, nil, process)
		close(unblock)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ensurePromptRuntime(canceled) error = %v, want context.Canceled", err)
		}
		if err := <-applied; err != nil {
			t.Fatalf("SetRuntimeSelection() error = %v", err)
		}
		for range 32 { // The slot is free now; a canceled caller must still never take it.
			_, err := h.manager.ensurePromptRuntime(canceled, active, nil, process)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("ensurePromptRuntime(canceled, free slot) error = %v, want context.Canceled", err)
			}
		}
	})

	t.Run("Should defer removing an ACP option the provider cannot unset", func(t *testing.T) {
		t.Parallel()
		h, driver, active := newLiveRuntimeHarness(t, runtimeProviderClaude, nil)
		snapshot := active.runtimeBindingSnapshot()
		option := acp.SessionConfigOptionSelection{ID: "thinking", ValueID: "on"}
		selection := snapshot.selection
		selection.ACPOptions = []acp.SessionConfigOptionSelection{option}
		active.completeRuntimeTransition(snapshot.process, selection, RuntimeTransitionInitialBind, time.Now())

		updated, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "medium"), 0)
		if err != nil {
			t.Fatalf("SetRuntimeSelection() error = %v", err)
		}
		removed := liveEffortSelection(active, "medium")
		removed.ACPOptions = nil
		updated, err = h.manager.SetRuntimeSelection(t.Context(), active.ID, removed, updated.RuntimeSelectionRevision)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(removal) error = %v", err)
		}
		if got := driver.configSnapshots(); len(got) != 1 || len(updated.ACPOptions) != 1 {
			t.Fatalf("removal configs = %#v options = %#v, want it deferred with the option still effective",
				got, updated.ACPOptions)
		}
	})

	t.Run("Should reconfigure a reattached process after a release rejected the live commit", func(t *testing.T) {
		t.Parallel()
		var active *Session
		var detached *AgentProcess
		detachOnMedium := func(config acp.RuntimeConfig) error {
			if config.ReasoningEffort == "medium" {
				active.mu.Lock()
				detached, active.process = active.process, nil
				active.mu.Unlock()
			}
			return nil
		}
		h, driver, session := newLiveRuntimeHarness(t, runtimeProviderClaude, detachOnMedium)
		active = session
		updated, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "medium"), 0)
		if err != nil {
			t.Fatalf("SetRuntimeSelection() error = %v", err)
		}
		active.mu.Lock()
		active.process = detached
		active.mu.Unlock()
		snapshot := active.runtimeBindingSnapshot()
		if updated.ReasoningEffort != "xhigh" || !snapshot.configUnverified || !runtimeControlsDrifted(&snapshot) {
			t.Fatalf("rejected commit info = %#v, want effective xhigh with a required reconfigure", updated)
		}
		// With no stored selection the effective runtime already matches, so only the
		// unverified config can make admission send this reconfigure.
		if _, err := h.manager.ClearRuntimeSelection(t.Context(), active.ID, 1); err != nil {
			t.Fatalf("ClearRuntimeSelection() error = %v", err)
		}
		events, err := h.manager.Prompt(t.Context(), active.ID, "dispatch on the reattached process")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)
		configs := driver.configSnapshots()
		if len(configs) != 2 || configs[1].ReasoningEffort != "xhigh" ||
			active.runtimeBindingSnapshot().configUnverified {
			t.Fatalf("configs = %#v, want admission to reconfigure the recorded runtime before dispatch", configs)
		}
	})

	t.Run("Should apply a selection saved under a provider alias", func(t *testing.T) {
		t.Parallel()
		h, driver, active := newLiveRuntimeHarness(t, runtimeProviderClaude, nil)
		selection := liveEffortSelection(active, "medium")
		selection.Provider = "claude-code"
		updated, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, selection, 0)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(alias) error = %v", err)
		}
		if got := driver.configSnapshots(); len(got) != 1 || updated.ReasoningEffort != "medium" ||
			updated.SelectedRuntime == nil || updated.SelectedRuntime.Provider != runtimeProviderClaude {
			t.Fatalf("alias selection configs = %#v info = %#v, want a canonical live apply", got, updated)
		}
	})

	t.Run("Should keep a failed rollback unverified when a release detached the process", func(t *testing.T) {
		t.Parallel()
		var active *Session
		var detached *AgentProcess
		detachAndRefuse := func(acp.RuntimeConfig) error {
			active.mu.Lock()
			defer active.mu.Unlock()
			if active.process != nil {
				detached, active.process = active.process, nil
			}
			return errors.New("provider transport closed")
		}
		h, _, session := newLiveRuntimeHarness(t, runtimeProviderClaude, detachAndRefuse)
		active = session
		if _, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "max"), 0); err != nil {
			t.Fatalf("SetRuntimeSelection() error = %v", err)
		}
		active.mu.Lock()
		active.process = detached
		active.mu.Unlock()
		if !active.runtimeBindingSnapshot().configUnverified {
			t.Fatal("a failed rollback on a detached process lost its unverified mark")
		}
	})

	t.Run("Should reconfigure before dispatch after a failed rollback with a matching selection", func(t *testing.T) {
		t.Parallel()
		var rollbackFails atomic.Bool
		rollbackFails.Store(true)
		hook := func(config acp.RuntimeConfig) error {
			switch {
			case config.ReasoningEffort == "max":
				return &acp.NegotiationError{Code: acp.NegotiationCodeReasoningEffortUnsupported, Requested: "max"}
			case config.ReasoningEffort == "xhigh" && rollbackFails.CompareAndSwap(true, false):
				return errors.New("provider transport closed during rollback")
			}
			return nil
		}
		h, driver, active := newLiveRuntimeHarness(t, runtimeProviderClaude, hook)
		maxSelection := liveEffortSelection(active, "max")
		if _, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, maxSelection, 0); err != nil {
			t.Fatalf("SetRuntimeSelection(max) error = %v", err)
		}
		if !active.runtimeBindingSnapshot().configUnverified {
			t.Fatal("failed rollback did not mark the provider config unverified")
		}
		reselected, err := h.manager.SetRuntimeSelection(
			t.Context(), active.ID, liveEffortSelection(active, "xhigh"), 1,
		)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(original) error = %v", err)
		}
		configs := driver.configSnapshots()
		if len(configs) != 3 || configs[2].ReasoningEffort != "xhigh" || reselected.RuntimeFailure != "" ||
			active.runtimeBindingSnapshot().configUnverified {
			t.Fatalf("configs = %#v info = %#v, want a verified reapply of the matching selection", configs, reselected)
		}
	})

	t.Run("Should not attach a superseded apply's failure to a newer selection", func(t *testing.T) {
		t.Parallel()
		entered, unblock := make(chan struct{}), make(chan struct{})
		var once sync.Once
		blockAndRefuseMax := func(config acp.RuntimeConfig) error {
			if config.ReasoningEffort == "max" {
				once.Do(func() { close(entered) })
				<-unblock
				return &acp.NegotiationError{Code: acp.NegotiationCodeReasoningEffortUnsupported, Requested: "max"}
			}
			return nil
		}
		h, _, active := newLiveRuntimeHarness(t, runtimeProviderClaude, blockAndRefuseMax)
		first := make(chan error, 1)
		go func() {
			_, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "max"), 0)
			first <- err
		}()
		waitLiveApplyEntered(t, entered)
		second := make(chan *Info, 1)
		go func() {
			info, err := h.manager.SetRuntimeSelection(
				t.Context(), active.ID, RuntimeSelection{Provider: "codex", Model: "gpt-5"}, 1,
			)
			if err != nil {
				t.Errorf("SetRuntimeSelection(newer) error = %v", err)
			}
			second <- info
		}()
		deadline := time.Now().Add(time.Second)
		for active.Info().RuntimeSelectionRevision != 2 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		close(unblock)
		if err := <-first; err != nil {
			t.Fatalf("SetRuntimeSelection(superseded) error = %v", err)
		}
		if info := <-second; info == nil || info.RuntimeFailure != "" || active.Info().RuntimeFailure != "" {
			t.Fatalf("newer selection info = %#v, want no failure from the superseded apply", info)
		}
	})

	t.Run("Should keep the turn and the stored selection when the provider refuses mid-turn", func(t *testing.T) {
		t.Parallel()
		refusal := &acp.NegotiationError{Code: acp.NegotiationCodeReasoningEffortUnsupported, Requested: "max"}
		refuseMax := func(config acp.RuntimeConfig) error {
			if config.ReasoningEffort == "max" {
				return refusal
			}
			return nil
		}
		h, driver, active := newLiveRuntimeHarness(t, runtimeProviderClaude, refuseMax)
		process := active.processHandle()
		events, _, release := startBlockedLivePrompt(t, h, active)

		updated, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "max"), 0)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(refused) error = %v, want persisted selection", err)
		}
		if updated.SelectedRuntime == nil || updated.SelectedRuntime.ReasoningEffort != "max" ||
			updated.ReasoningEffort != "xhigh" || !strings.Contains(updated.RuntimeFailure, `"max"`) ||
			updated.RuntimeStatus != RuntimeStatusReady || active.processHandle() != process || !active.IsPrompting() {
			t.Fatalf("refused info = %#v, want stored max, effective xhigh, failure, same running process", updated)
		}
		if got := driver.configSnapshots(); len(got) != 2 || got[1].ReasoningEffort != "xhigh" {
			t.Fatalf("refused configs = %#v, want apply then rollback", got)
		}
		release()
		if last := collectEvents(t, events); len(last) == 0 || last[len(last)-1].Type != acp.EventTypeDone {
			t.Fatalf("refused turn events = %#v, want completed turn", last)
		}

		if _, err := h.manager.Prompt(t.Context(), active.ID, "retry the stored selection"); !errors.Is(err, refusal) {
			t.Fatalf("Prompt(retry) error = %v, want the provider refusal", err)
		}
		if got := driver.configSnapshots(); len(got) < 3 || got[2].ReasoningEffort != "max" {
			t.Fatalf("next prompt configs = %#v, want the stored selection retried", got)
		}
	})

	t.Run("Should clear a refused live apply failure when a newer selection supersedes it", func(t *testing.T) {
		t.Parallel()
		refuseMax := func(config acp.RuntimeConfig) error {
			if config.ReasoningEffort == "max" {
				return &acp.NegotiationError{Code: acp.NegotiationCodeReasoningEffortUnsupported, Requested: "max"}
			}
			return nil
		}
		h, _, active := newLiveRuntimeHarness(t, runtimeProviderClaude, refuseMax)
		refused, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "max"), 0)
		if err != nil || refused.RuntimeFailure == "" {
			t.Fatalf("SetRuntimeSelection(max) = %#v, %v; want recorded refusal", refused, err)
		}
		deferred, err := h.manager.SetRuntimeSelection(
			t.Context(), active.ID, RuntimeSelection{Provider: "codex", Model: "gpt-5"}, 1,
		)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(provider change) error = %v", err)
		}
		if deferred.RuntimeFailure != "" || deferred.ReasoningEffort != "xhigh" {
			t.Fatalf("superseding selection info = %#v, want stale refusal cleared and effective unchanged", deferred)
		}
	})

	t.Run("Should defer a provider change to the next prompt", func(t *testing.T) {
		t.Parallel()
		h, driver, active := newLiveRuntimeHarness(t, runtimeProviderClaude, nil)
		before := active.Info()
		updated, err := h.manager.SetRuntimeSelection(
			t.Context(), active.ID, RuntimeSelection{Provider: "codex", Model: "gpt-5"}, 0,
		)
		if err != nil {
			t.Fatalf("SetRuntimeSelection(provider change) error = %v", err)
		}
		h.driver.mu.Lock()
		starts := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		if len(driver.configSnapshots()) != 0 || starts != 1 || updated.Provider != before.Provider ||
			updated.ReasoningEffort != before.ReasoningEffort {
			t.Fatalf("provider change info = %#v starts=%d, want no live apply", updated, starts)
		}
	})

	t.Run("Should serialize a live apply ahead of prompt runtime admission", func(t *testing.T) {
		t.Parallel()
		entered, unblock := make(chan struct{}), make(chan struct{})
		var once sync.Once
		blockMedium := func(config acp.RuntimeConfig) error {
			if config.ReasoningEffort == "medium" {
				once.Do(func() { close(entered) })
				<-unblock
			}
			return nil
		}
		h, driver, active := newLiveRuntimeHarness(t, runtimeProviderClaude, blockMedium)
		applied := make(chan error, 1)
		go func() {
			_, err := h.manager.SetRuntimeSelection(t.Context(), active.ID, liveEffortSelection(active, "medium"), 0)
			applied <- err
		}()
		waitLiveApplyEntered(t, entered)
		prompted := make(chan error, 1)
		go func() {
			events, err := h.manager.Prompt(t.Context(), active.ID, "admitted after the live apply")
			if err == nil {
				collectEvents(t, events)
			}
			prompted <- err
		}()
		select {
		case err := <-prompted:
			t.Fatalf("Prompt() finished during the live apply: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
		close(unblock)
		if err := <-applied; err != nil {
			t.Fatalf("SetRuntimeSelection() error = %v", err)
		}
		if err := <-prompted; err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		if got := driver.configSnapshots(); len(got) != 1 || active.Info().ReasoningEffort != "medium" {
			t.Fatalf("configs = %#v effort = %q, want one apply and no admission reapply", got,
				active.Info().ReasoningEffort)
		}
	})
}

// liveRuntimeNextTurnProvider is a custom provider: it takes the default next_turn semantics
// without depending on a native CLI being installed on the host.
const liveRuntimeNextTurnProvider = "acme-next-turn"

// newLiveRuntimeHarness binds a Claude session at xhigh whose provider reports applied efforts.
func newLiveRuntimeHarness(
	t *testing.T,
	providerID string,
	hook func(acp.RuntimeConfig) error,
) (*harness, *runtimeConfigRecordingDriver, *Session) {
	t.Helper()
	h := newHarness(t)
	driver := &runtimeConfigRecordingDriver{fakeDriver: h.driver}
	h.manager.driver = driver
	if providerID != runtimeProviderClaude {
		resolved, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		resolved.Config.Providers[providerID] = compozyconfig.ProviderConfig{Command: providerID + " --acp"}
		for index := range resolved.Agents {
			if resolved.Agents[index].Name == "coder" {
				resolved.Agents[index].Provider = providerID
				resolved.Agents[index].Model = ""
			}
		}
		h.resolver.upsert(&resolved)
	}
	active := createSession(t, h)
	t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
	snapshot := active.runtimeBindingSnapshot()
	selection := snapshot.selection
	selection.ReasoningEffort = "xhigh"
	efforts := []acp.SessionConfigOptionValue{{Value: "medium"}, {Value: "xhigh"}, {Value: "max"}}
	snapshot.process.setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{{
		ID: "effort", Category: "thought_level", Kind: acp.SessionConfigOptionKindSelect,
		CurrentValueID: "xhigh", Values: efforts,
	}}})
	active.completeRuntimeTransition(snapshot.process, selection, RuntimeTransitionInitialBind, time.Now())
	driver.configHook = func(process *AgentProcess, config acp.RuntimeConfig) error {
		if hook != nil {
			if err := hook(config); err != nil {
				return err
			}
		}
		caps := process.CapsSnapshot()
		caps.ConfigOptions[0].CurrentValueID = config.ReasoningEffort
		process.setCaps(caps)
		return nil
	}
	return h, driver, active
}

func liveEffortSelection(active *Session, effort string) RuntimeSelection {
	selection := active.runtimeBindingSnapshot().selection
	selection.ReasoningEffort = effort
	return selection
}

func startBlockedLivePrompt(
	t *testing.T,
	h *harness,
	active *Session,
) (<-chan acp.AgentEvent, string, func()) {
	t.Helper()
	started, release := make(chan string, 1), make(chan struct{})
	h.driver.mu.Lock()
	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		started <- req.TurnID
		events := make(chan acp.AgentEvent)
		go func() {
			defer close(events)
			<-release
			sessionID := active.Info().ACPSessionID
			events <- acp.AgentEvent{
				Type: acp.EventTypeAgentMessage, SessionID: sessionID, TurnID: req.TurnID, Text: "after the change",
			}
			events <- acp.AgentEvent{Type: acp.EventTypeDone, SessionID: sessionID, TurnID: req.TurnID}
		}()
		return events, nil
	}
	h.driver.mu.Unlock()
	events, err := h.manager.Prompt(t.Context(), active.ID, "long turn")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	var turnID string
	select {
	case turnID = <-started:
	case <-time.After(time.Second):
		t.Fatal("driver prompt did not start")
	}
	var once sync.Once
	releaseOnce := func() { once.Do(func() { close(release) }) }
	t.Cleanup(releaseOnce)
	return events, turnID, releaseOnce
}

func waitLiveApplyEntered(t *testing.T, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("live apply did not reach ConfigureRuntime")
	}
}

func liveAgentMessageEffort(t *testing.T, h *harness, active *Session, turnID string) string {
	t.Helper()
	if err := h.manager.WaitForPromptDrains(t.Context()); err != nil {
		t.Fatal(err)
	}
	events, err := h.manager.Events(t.Context(), active.ID, store.EventQuery{Type: acp.EventTypeAgentMessage})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.TurnID != turnID || !strings.Contains(event.Content, "after the change") {
			continue
		}
		var payload struct {
			PromptRuntime struct {
				ReasoningEffort string `json:"reasoning_effort"`
			} `json:"prompt_runtime"`
		}
		if err := json.Unmarshal([]byte(event.Content), &payload); err != nil {
			t.Fatal(err)
		}
		return payload.PromptRuntime.ReasoningEffort
	}
	t.Fatalf("agent message after the change not recorded for turn %q", turnID)
	return ""
}

func liveRuntimeMarkerEvent(t *testing.T, h *harness, active *Session) store.SessionEvent {
	t.Helper()
	events, err := h.manager.Events(t.Context(), active.ID, store.EventQuery{Type: eventspkg.TranscriptMarkerCreated})
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if strings.Contains(event.Content, transcript.MarkerRuntimeChanged) {
			return event
		}
	}
	t.Fatal("runtime changed marker event not recorded")
	return store.SessionEvent{}
}

type runtimeConfigRecordingDriver struct {
	*fakeDriver
	configMu       sync.Mutex
	configs        []acp.RuntimeConfig
	configErr      error
	configHook     func(*AgentProcess, acp.RuntimeConfig) error
	startupPrompts []string
}

func (d *runtimeConfigRecordingDriver) ReplacePendingStartupPrompt(
	_ *AgentProcess,
	prompt string,
	_ acp.StartupManifest,
) bool {
	d.configMu.Lock()
	defer d.configMu.Unlock()
	d.startupPrompts = append(d.startupPrompts, prompt)
	return true
}

func (d *runtimeConfigRecordingDriver) replacedStartupPrompts() []string {
	d.configMu.Lock()
	defer d.configMu.Unlock()
	return append([]string(nil), d.startupPrompts...)
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

	t.Run("Should preserve Pi guidance after bridge materialization fails", func(t *testing.T) {
		t.Parallel()
		h := newHostedMCPHarness(t)
		active := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, active.ID) })
		plan, err := h.manager.preparePromptRuntimePlan(t.Context(), active,
			RuntimeSelection{Provider: "pi", Model: "test-model"})
		if err != nil {
			t.Fatal(err)
		}
		// A custom command can resemble Claude while the provider still owns a Pi harness.
		plan.runtime.agent.Command = "claude-agent-acp"
		plan.runtime.mcpServers = []compozyconfig.MCPServer{{Command: "unused-test-command"}}
		// Block materialization at its filesystem boundary.
		if err := os.WriteFile(
			filepath.Join(active.sessionDir, "provider-runtime"),
			[]byte("blocked"),
			0o600,
		); err != nil {
			t.Fatal(err)
		}
		opts, err := h.manager.prepareSessionLaunch(t.Context(), &plan.spec, active, &plan.runtime, nil)
		if err != nil {
			t.Fatal(err)
		}
		prompt, _, err := opts.FinalizeStartup(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if !active.NativeToolsUnavailable() || active.AgentDefinition().Prompt != prompt {
			t.Fatal("Pi finalizer did not commit unavailable-tool guidance")
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
		for attempt := range 3 {
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
			startOpts := acp.StartOpts{Command: "pi-acp", Cwd: t.TempDir()}
			if attempt > 0 {
				startOpts.MCPServers = []compozyconfig.MCPServer{{Command: "unused-test-command"}}
			}
			opts := h.manager.preparePiBridgeStartup(&plan.spec, active, &plan.runtime, startOpts)
			if attempt > 0 {
				config := readProviderJSON[piBridgeConfig](t, envValue(opts.Env, piBridgeConfigEnv))
				status := piBridgeStatus{Nonce: config.Nonce, Code: "pi_native_tools_connection_failed"}
				if attempt == 2 {
					status.Code, status.Count = "ready", 4
				}
				if err := writeProviderJSON(config.Status, status); err != nil {
					t.Fatal(err)
				}
			}
			prompt, _, err := opts.FinalizeStartup(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if active.AgentDefinition().Prompt != prompt {
				t.Fatal("stored session definition differs from finalized guidance")
			}
			if !strings.HasPrefix(prompt, original.Prompt) || strings.Count(prompt, overlay) != 1 ||
				strings.Contains(prompt, "Previously assembled") || active.NativeToolsUnavailable() != (attempt != 2) {
				t.Fatalf("recovery retained stale guidance: %q", prompt)
			}
		}
	})
}
