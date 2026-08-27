package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func TestSessionStateTransitions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	session := &Session{
		ID:        "sess-1",
		AgentName: "coder",
		Workspace: t.TempDir(),
		State:     StateStarting,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := session.activate(now.Add(time.Second), false); err != nil {
		t.Fatalf("activate() error = %v", err)
	}
	if got := session.Info().State; got != StateActive {
		t.Fatalf("activate() state = %q, want %q", got, StateActive)
	}

	if err := session.beginStopping(now.Add(2 * time.Second)); err != nil {
		t.Fatalf("beginStopping() error = %v", err)
	}
	if got := session.Info().State; got != StateStopping {
		t.Fatalf("beginStopping() state = %q, want %q", got, StateStopping)
	}

	if err := session.markStopped(now.Add(3 * time.Second)); err != nil {
		t.Fatalf("markStopped() error = %v", err)
	}
	info := session.Info()
	if info.State != StateStopped {
		t.Fatalf("markStopped() state = %q, want %q", info.State, StateStopped)
	}
	if !info.UpdatedAt.Equal(now.Add(3 * time.Second)) {
		t.Fatalf("UpdatedAt = %s, want %s", info.UpdatedAt, now.Add(3*time.Second))
	}
}

func TestSessionInvalidTransitionRejected(t *testing.T) {
	t.Parallel()

	session := &Session{
		ID:        "sess-1",
		AgentName: "coder",
		Workspace: t.TempDir(),
		State:     StateStopped,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := session.activate(time.Now().UTC(), false)
	if err == nil {
		t.Fatal("activate() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("activate() error = %v, want ErrInvalidStateTransition", err)
	}
}

func TestSessionInfoCopiesCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve known empty ACP capabilities in Info", func(t *testing.T) {
		t.Parallel()

		now := time.Now().UTC()
		session := &Session{
			ID:        "sess-1",
			AgentName: "coder",
			Workspace: t.TempDir(),
			State:     StateStarting,
			CreatedAt: now,
			UpdatedAt: now,
		}
		proc := NewAgentProcess(AgentProcessOptions{Caps: acp.Caps{}})
		if err := session.activateWithProcess(proc, now, false, false); err != nil {
			t.Fatalf("activateWithProcess() error = %v", err)
		}

		info := session.Info()
		if !info.ACPCapsKnown {
			t.Fatal("Info().ACPCapsKnown = false, want true for negotiated empty capabilities")
		}
		if info.ACPCaps.PromptImage ||
			info.ACPCaps.PromptAudio ||
			info.ACPCaps.PromptEmbeddedContext {
			t.Fatalf("Info().ACPCaps = %#v, want negotiated all-false capabilities", info.ACPCaps)
		}
	})

	t.Run("Should return deep-copied ACP capabilities from Info", func(t *testing.T) {
		t.Parallel()

		session := &Session{
			ID:           "sess-1",
			AgentName:    "coder",
			Workspace:    t.TempDir(),
			State:        StateActive,
			ACPSessionID: "acp-1",
			ACPCaps: acp.Caps{
				SupportsLoadSession: true,
				SupportedModes:      []string{"chat"},
				ConfigOptions: []acp.SessionConfigOption{{
					ID:             "model",
					Kind:           acp.SessionConfigOptionKindSelect,
					CurrentValueID: "gpt",
					Values: []acp.SessionConfigOptionValue{
						{Value: "gpt", Label: "GPT"},
					},
				}},
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		info := session.Info()
		info.ACPCaps.SupportedModes[0] = "mutated"
		info.ACPCaps.ConfigOptions[0].CurrentValueID = "mutated"
		info.ACPCaps.ConfigOptions[0].Values[0].Value = "mutated"

		latest := session.Info()
		if latest.ACPCaps.SupportedModes[0] != "chat" {
			t.Fatalf("SupportedModes mutated through Info() copy: %#v", latest.ACPCaps.SupportedModes)
		}
		if latest.ACPCaps.ConfigOptions[0].CurrentValueID != "gpt" ||
			latest.ACPCaps.ConfigOptions[0].Values[0].Value != "gpt" {
			t.Fatalf("ConfigOptions mutated through Info() copy: %#v", latest.ACPCaps.ConfigOptions)
		}
	})
}

func TestRuntimeBindingSnapshotRestore(t *testing.T) {
	t.Parallel()

	t.Run("Should restore the agent definition with the runtime binding", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
		session := &Session{
			ID:        "sess-1",
			AgentName: "coder",
			Workspace: t.TempDir(),
			State:     StateActive,
			agentDef: compozyconfig.AgentDef{
				Name:   "coder",
				Prompt: "previous definition",
				Tools:  []string{"read"},
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		snapshot := session.runtimeBindingSnapshot()
		session.setAgentDefinition(compozyconfig.AgentDef{
			Name:   "coder",
			Prompt: "candidate definition",
			Tools:  []string{"write"},
		})

		session.restoreRuntimeBinding(&snapshot, "catalog persistence failed", now.Add(time.Second))
		definition := session.AgentDefinition()
		if definition.Prompt != "previous definition" {
			t.Fatalf("AgentDefinition().Prompt = %q, want prior definition", definition.Prompt)
		}
		if len(definition.Tools) != 1 || definition.Tools[0] != "read" {
			t.Fatalf("AgentDefinition().Tools = %#v, want prior definition tools", definition.Tools)
		}
	})
}

func TestSessionActivateCanPreserveRecoveredStopClassification(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC)
	session := &Session{
		ID:         "sess-recovered",
		AgentName:  "coder",
		Workspace:  t.TempDir(),
		State:      StateStarting,
		stopReason: store.StopAgentCrashed,
		stopDetail: resumeStopDetailAgentCrashed,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := session.activate(now.Add(time.Second), true); err != nil {
		t.Fatalf("activate(preserve) error = %v", err)
	}

	info := session.Info()
	if got := info.State; got != StateActive {
		t.Fatalf("State = %q, want %q", got, StateActive)
	}
	if got := info.StopReason; got != store.StopAgentCrashed {
		t.Fatalf("StopReason = %q, want %q", got, store.StopAgentCrashed)
	}
	if got := info.StopDetail; got != resumeStopDetailAgentCrashed {
		t.Fatalf("StopDetail = %q, want %q", got, resumeStopDetailAgentCrashed)
	}
}

func TestSessionInfoAndMetaIncludeStopFields(t *testing.T) {
	t.Run("Should include stop fields in info and metadata", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
		session := &Session{
			ID:          "sess-stop",
			Name:        "Stopped Session",
			AgentName:   "coder",
			WorkspaceID: "ws-stop",
			Workspace:   t.TempDir(),
			Type:        SessionTypeSystem,
			State:       StateStopped,
			stopCause:   CauseShutdown,
			stopReason:  store.StopShutdown,
			stopDetail:  "daemon shutdown",
			CreatedAt:   now,
			UpdatedAt:   now.Add(time.Minute),
		}

		info := session.Info()
		if info.StopReason != store.StopShutdown {
			t.Fatalf("Info().StopReason = %q, want %q", info.StopReason, store.StopShutdown)
		}
		if info.StopDetail != "daemon shutdown" {
			t.Fatalf("Info().StopDetail = %q, want %q", info.StopDetail, "daemon shutdown")
		}

		meta := session.Meta()
		if meta.StopReason == nil {
			t.Fatal("Meta().StopReason = nil, want non-nil")
		}
		if *meta.StopReason != store.StopShutdown {
			t.Fatalf("Meta().StopReason = %q, want %q", *meta.StopReason, store.StopShutdown)
		}
		if meta.StopDetail != "daemon shutdown" {
			t.Fatalf("Meta().StopDetail = %q, want %q", meta.StopDetail, "daemon shutdown")
		}
	})
}

func TestSessionMetadataRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("Should persist and reload provider and selected runtime in session metadata", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
		session := &Session{
			ID:        "sess-provider",
			Name:      "Provider Session",
			AgentName: "coder",
			Provider:  "codex",
			SelectedRuntime: &RuntimeSelection{
				Provider:        "claude",
				Model:           "claude-fable-5",
				ReasoningEffort: "max",
				Speed:           speedpkg.SpeedFast,
			},
			RuntimeSelectionRevision: 3,
			RuntimeStatus:            RuntimeStatusReady,
			RuntimeTransition:        RuntimeTransitionInitialBind,
			ACPSessionID:             "acp-provider",
			WorkspaceID:              "ws-provider",
			Workspace:                t.TempDir(),
			NetworkParticipation:     testLocalParticipation(),
			State:                    StateActive,
			CreatedAt:                now,
			UpdatedAt:                now,
		}

		meta := session.Meta()
		selectedRuntime, selectionRevision := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelection)
		if got := meta.Provider; got != "codex" {
			t.Fatalf("Meta().Provider = %q, want %q", got, "codex")
		}
		if selectedRuntime == nil || selectedRuntime.Provider != "claude" ||
			selectedRuntime.Model != "claude-fable-5" || selectedRuntime.ReasoningEffort != "max" ||
			selectedRuntime.Speed != speedpkg.SpeedFast || selectionRevision != 3 {
			t.Fatalf(
				"Meta() selected runtime = %#v revision %d, want durable Claude selection at revision 3",
				selectedRuntime,
				selectionRevision,
			)
		}
		if meta.RuntimeStatus != RuntimeStatusReady || meta.RuntimeTransition != RuntimeTransitionInitialBind ||
			derefString(meta.ACPSessionID) != "acp-provider" {
			t.Fatalf("Meta() runtime = %#v, want ready initial bind with ACP session", meta)
		}
		info := session.Info()
		if got := info.Provider; got != "codex" {
			t.Fatalf("Info().Provider = %q, want %q", got, "codex")
		}
		if info.SelectedRuntime == nil || info.SelectedRuntime.Provider != "claude" ||
			info.SelectedRuntime.Model != "claude-fable-5" || info.SelectedRuntime.ReasoningEffort != "max" ||
			info.SelectedRuntime.Speed != speedpkg.SpeedFast || info.RuntimeSelectionRevision != 3 {
			t.Fatalf(
				"Info() selected runtime = %#v revision %d, want durable Claude selection at revision 3",
				info.SelectedRuntime,
				info.RuntimeSelectionRevision,
			)
		}
		if info.RuntimeStatus != RuntimeStatusReady || info.RuntimeTransition != RuntimeTransitionInitialBind ||
			info.ACPSessionID != "acp-provider" {
			t.Fatalf("Info() runtime = %#v, want ready initial bind with ACP session", info)
		}

		metaPath := filepath.Join(t.TempDir(), "meta.json")
		if err := store.WriteSessionMeta(metaPath, meta); err != nil {
			t.Fatalf("WriteSessionMeta() error = %v", err)
		}
		rawMeta, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("os.ReadFile(meta) error = %v", err)
		}
		var persisted struct {
			RuntimeSelection struct {
				Selected map[string]json.RawMessage `json:"selected"`
			} `json:"runtime_selection"`
		}
		if err := json.Unmarshal(rawMeta, &persisted); err != nil {
			t.Fatalf("json.Unmarshal(meta) error = %v", err)
		}
		if _, ok := persisted.RuntimeSelection.Selected["reasoning_effort"]; !ok {
			t.Fatalf(
				"persisted selected runtime keys = %#v, want reasoning_effort",
				persisted.RuntimeSelection.Selected,
			)
		}
		if _, ok := persisted.RuntimeSelection.Selected["ReasoningEffort"]; ok {
			t.Fatalf(
				"persisted selected runtime keys = %#v, want no ReasoningEffort",
				persisted.RuntimeSelection.Selected,
			)
		}

		readBack, err := store.ReadSessionMeta(metaPath)
		if err != nil {
			t.Fatalf("ReadSessionMeta() error = %v", err)
		}
		if got := readBack.Provider; got != "codex" {
			t.Fatalf("ReadSessionMeta().Provider = %q, want %q", got, "codex")
		}
		readSelection, readRevision := store.SessionRuntimeSelectionStateValues(readBack.RuntimeSelection)
		if readSelection == nil || readSelection.Provider != "claude" ||
			readSelection.Model != "claude-fable-5" ||
			readSelection.ReasoningEffort != "max" ||
			readSelection.Speed != speedpkg.SpeedFast || readRevision != 3 {
			t.Fatalf(
				"ReadSessionMeta() selected runtime = %#v revision %d, want durable Claude selection at revision 3",
				readSelection,
				readRevision,
			)
		}
		if readBack.RuntimeStatus != RuntimeStatusReady ||
			readBack.RuntimeTransition != RuntimeTransitionInitialBind ||
			derefString(readBack.ACPSessionID) != "acp-provider" {
			t.Fatalf("ReadSessionMeta() runtime = %#v, want ready initial bind with ACP session", readBack)
		}
	})

	t.Run("Should persist and clone the current advertised command replacement set", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		commands := []store.SessionAdvertisedCommand{{
			Name:        "compact",
			Description: "Compact context",
			Input:       &store.SessionAdvertisedCommandInput{Hint: "optional focus"},
		}}
		session := &Session{
			ID:                   "sess-commands",
			AgentName:            "coder",
			RuntimeStatus:        RuntimeStatusUnbound,
			WorkspaceID:          "ws-commands",
			Workspace:            t.TempDir(),
			NetworkParticipation: testLocalParticipation(),
			State:                StateActive,
			AdvertisedCommands:   commands,
			CreatedAt:            now,
			UpdatedAt:            now,
		}

		meta := session.Meta()
		info := session.Info()
		if meta.RuntimeStatus != RuntimeStatusUnbound || info.RuntimeStatus != RuntimeStatusUnbound {
			t.Fatalf(
				"logical session runtime = meta %q / info %q, want unbound",
				meta.RuntimeStatus,
				info.RuntimeStatus,
			)
		}
		commands[0].Name = "mutated-source"
		if got := meta.AdvertisedCommands[0].Name; got != "compact" {
			t.Fatalf("Meta().AdvertisedCommands[0].Name = %q, want compact", got)
		}
		if got := info.AdvertisedCommands[0].Name; got != "compact" {
			t.Fatalf("Info().AdvertisedCommands[0].Name = %q, want compact", got)
		}

		metaPath := filepath.Join(t.TempDir(), "meta.json")
		if err := store.WriteSessionMeta(metaPath, meta); err != nil {
			t.Fatalf("WriteSessionMeta() error = %v", err)
		}
		readBack, err := store.ReadSessionMeta(metaPath)
		if err != nil {
			t.Fatalf("ReadSessionMeta() error = %v", err)
		}
		if !store.SessionAdvertisedCommandsEqual(readBack.AdvertisedCommands[0], meta.AdvertisedCommands[0]) {
			t.Fatalf(
				"ReadSessionMeta().AdvertisedCommands = %#v, want %#v",
				readBack.AdvertisedCommands,
				meta.AdvertisedCommands,
			)
		}
		if readBack.RuntimeStatus != RuntimeStatusUnbound {
			t.Fatalf("ReadSessionMeta().RuntimeStatus = %q, want %q", readBack.RuntimeStatus, RuntimeStatusUnbound)
		}
	})
}

func TestBeginPromptSetupReturnsErrSessionNotActive(t *testing.T) {
	t.Parallel()

	session := &Session{
		ID:        "sess-1",
		AgentName: "coder",
		Workspace: t.TempDir(),
		State:     StateStopped,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := session.beginPromptSetup()
	if !errors.Is(err, ErrSessionNotActive) {
		t.Fatalf("beginPromptSetup() error = %v, want ErrSessionNotActive", err)
	}
	if !strings.Contains(err.Error(), "sess-1") {
		t.Fatalf("beginPromptSetup() error = %v, want session id context", err)
	}
}

func TestBeginPromptSetupRejectsReservedConversationRewind(t *testing.T) {
	t.Parallel()

	t.Run("Should reject prompt setup during rewind", func(t *testing.T) {
		t.Parallel()

		session := &Session{
			ID:                         "sess-rewind-reserved",
			State:                      StateActive,
			conversationRewindReserved: true,
		}

		err := session.beginPromptSetup()
		if !errors.Is(err, ErrSessionNotActive) {
			t.Fatalf("beginPromptSetup() error = %v, want ErrSessionNotActive", err)
		}
	})
}

func TestNormalizeSessionTypeDefaultsToUser(t *testing.T) {
	t.Parallel()

	if got := normalizeSessionType(""); got != SessionTypeUser {
		t.Fatalf("normalizeSessionType(\"\") = %q, want %q", got, SessionTypeUser)
	}
	if got := normalizeSessionType(" dream "); got != SessionTypeDream {
		t.Fatalf("normalizeSessionType(\" dream \") = %q, want %q", got, SessionTypeDream)
	}
	if got := normalizeSessionType(" coordinator "); got != SessionTypeCoordinator {
		t.Fatalf("normalizeSessionType(\" coordinator \") = %q, want %q", got, SessionTypeCoordinator)
	}
	if got := normalizeSessionType(" spawned "); got != SessionTypeSpawned {
		t.Fatalf("normalizeSessionType(\" spawned \") = %q, want %q", got, SessionTypeSpawned)
	}
	if got := normalizeSessionType("unknown"); got != SessionTypeUser {
		t.Fatalf("normalizeSessionType(\"unknown\") = %q, want %q", got, SessionTypeUser)
	}
}
