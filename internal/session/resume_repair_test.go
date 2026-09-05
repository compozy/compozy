package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestClassifyPreviousStop(t *testing.T) {
	t.Parallel()

	existingReason := store.StopUserCanceled

	testCases := []struct {
		name        string
		meta        store.SessionMeta
		wantChanged bool
		wantState   string
		wantReason  *store.StopReason
		wantDetail  string
		wantACP     *string
	}{
		{
			name:        "active session classified as crashed",
			meta:        store.SessionMeta{State: string(StateActive), RuntimeStatus: store.SessionRuntimeReady},
			wantChanged: true,
			wantState:   string(StateStopped),
			wantReason:  stopReasonPointer(store.StopAgentCrashed),
			wantDetail:  resumeStopDetailAgentCrashed,
		},
		{
			name:        "stopping session classified as crashed",
			meta:        store.SessionMeta{State: string(StateStopping), RuntimeStatus: store.SessionRuntimeReady},
			wantChanged: true,
			wantState:   string(StateStopped),
			wantReason:  stopReasonPointer(store.StopAgentCrashed),
			wantDetail:  "stop did not complete",
		},
		{
			name: "starting session classified as error",
			meta: store.SessionMeta{
				State:         string(StateStarting),
				RuntimeStatus: store.SessionRuntimeBinding,
				ACPSessionID:  stringPointer("acp-stale"),
			},
			wantChanged: true,
			wantState:   string(StateStopped),
			wantReason:  stopReasonPointer(store.StopError),
			wantDetail:  resumeStopDetailStartIncomplete,
			wantACP:     nil,
		},
		{
			name: "stopped session preserves existing reason",
			meta: store.SessionMeta{
				State:         string(StateStopped),
				RuntimeStatus: store.SessionRuntimeReady,
				StopReason:    &existingReason,
				StopDetail:    "requested by user",
			},
			wantChanged: false,
			wantState:   string(StateStopped),
			wantReason:  &existingReason,
			wantDetail:  "requested by user",
		},
		{
			name:        "stopped session with no reason remains unchanged",
			meta:        store.SessionMeta{State: string(StateStopped), RuntimeStatus: store.SessionRuntimeReady},
			wantChanged: false,
			wantState:   string(StateStopped),
			wantReason:  nil,
			wantDetail:  "",
		},
		{
			name: "stopped incomplete start clears stale acp session id",
			meta: store.SessionMeta{
				State:         string(StateStopped),
				RuntimeStatus: store.SessionRuntimeReady,
				StopReason:    stopReasonPointer(store.StopError),
				StopDetail:    resumeStopDetailStartIncomplete,
				ACPSessionID:  stringPointer("acp-stale"),
			},
			wantChanged: true,
			wantState:   string(StateStopped),
			wantReason:  stopReasonPointer(store.StopError),
			wantDetail:  resumeStopDetailStartIncomplete,
			wantACP:     nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.meta.State != string(StateStopped) {
				started, err := procutil.StartedAt(os.Getpid())
				if err != nil {
					t.Fatal(err)
				}
				previousIdentity := started.Add(-time.Hour)
				tc.meta.Liveness = &store.SessionLivenessMeta{
					SubprocessPID: os.Getpid(), SubprocessStartedAt: &previousIdentity,
				}
			}
			gotMeta, gotChanged := classifyPreviousStop(tc.meta)
			if gotChanged != tc.wantChanged {
				t.Fatalf("classifyPreviousStop() changed = %v, want %v", gotChanged, tc.wantChanged)
			}
			if gotMeta.State != tc.wantState {
				t.Fatalf("classifyPreviousStop() state = %q, want %q", gotMeta.State, tc.wantState)
			}
			assertOptionalStopReasonEqual(t, gotMeta.StopReason, tc.wantReason)
			if gotMeta.StopDetail != tc.wantDetail {
				t.Fatalf("classifyPreviousStop() detail = %q, want %q", gotMeta.StopDetail, tc.wantDetail)
			}
			assertOptionalStringEqual(t, gotMeta.ACPSessionID, tc.wantACP)
		})
	}
}

func TestClassifyInactiveMetaForRecoveryPreservesFailureDetails(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should retain interrupted startup attribution and provider failure through repeated inventory",
		func(t *testing.T) {
			t.Parallel()
			meta := store.SessionMeta{
				State:   string(StateStarting),
				Failure: &store.SessionFailure{Kind: store.FailureProviderAuth, Summary: "provider login required"},
			}
			now := time.Now()
			first, changed := ClassifyInactiveMetaForRecovery(now, meta)
			if !changed || first.State != string(StateStopping) || !interruptedStartupMeta(&first) ||
				first.Failure.Kind != store.FailureProviderAuth || first.Failure.Summary != meta.Failure.Summary {
				t.Fatalf("startup inventory lost provider failure or startup attribution: %#v", first)
			}
			second, changed := ClassifyInactiveMetaForRecovery(now, first)
			if changed || !interruptedStartupMeta(&second) || !second.StopVerificationFailed ||
				second.Failure.Kind != store.FailureProviderAuth || second.Failure.Summary != meta.Failure.Summary {
				t.Fatalf("repeated inventory changed startup evidence: %#v", second)
			}
		},
	)
	for _, state := range []State{StateActive, StateStopping, StateStarting} {
		for _, liveness := range []*store.SessionLivenessMeta{nil, {}, {SubprocessPID: os.Getpid()}} {
			t.Run(
				fmt.Sprintf("Should retain %s without complete process identity %v", state, liveness),
				func(t *testing.T) {
					t.Parallel()
					meta := store.SessionMeta{State: string(state), Liveness: liveness}
					recovered, changed := ClassifyInactiveMetaForRecovery(time.Now(), meta)
					if !changed || recovered.State != string(StateStopping) || !recovered.StopVerificationFailed {
						t.Fatalf("missing identity fabricated terminal state: %#v", recovered)
					}
				},
			)
		}
	}

	t.Run("Should retain unverified stop attention when process identity is unavailable", func(t *testing.T) {
		t.Parallel()
		meta := store.SessionMeta{State: string(StateStopping), StopVerificationFailed: true}
		recovered, _ := ClassifyInactiveMetaForRecovery(time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC), meta)
		if recovered.State != string(StateStopping) || !recovered.StopVerificationFailed {
			t.Fatalf("unverified stop lost its exit-proof requirement: %#v", recovered)
		}
	})

	t.Run("Should preserve stopping until the recorded process identity is gone", func(t *testing.T) {
		t.Parallel()
		started, err := procutil.StartedAt(os.Getpid())
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name    string
			started time.Time
			want    State
			remote  bool
		}{
			{"Should retain a matching live process", started, StateStopping, false},
			{"Should retain an unverifiable identity", time.Time{}, StateStopping, false},
			{"Should recognize an exited identity when the PID is reused", started.Add(-time.Hour), StateStopped, false},
			{"Should not treat a reused local PID as remote exit proof", started.Add(-time.Hour), StateStopping, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				meta := store.SessionMeta{State: string(StateActive), Liveness: &store.SessionLivenessMeta{
					SubprocessPID: os.Getpid(), SubprocessStartedAt: &tc.started,
				}}
				if tc.remote {
					meta.Sandbox = &store.SessionSandboxMeta{Backend: "daytona"}
				}
				recovered, changed := ClassifyInactiveMetaForRecovery(started.Add(time.Hour), meta)
				if !changed || recovered.State != string(tc.want) ||
					recovered.StopVerificationFailed != (tc.want == StateStopping) {
					t.Fatalf("recovery state = %q, changed=%t", recovered.State, changed)
				}
				if !procutil.Alive(os.Getpid()) {
					t.Fatal("classification signaled a live process")
				}
			})
		}
	})

	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	testCases := []struct {
		name         string
		state        State
		fallbackKind store.FailureKind
	}{
		{name: "ShouldPreserveActiveFailureDetails", state: StateActive, fallbackKind: store.FailureProcess},
		{name: "ShouldPreserveStoppingFailureDetails", state: StateStopping, fallbackKind: store.FailureProcess},
		{name: "ShouldPreserveStartingFailureDetails", state: StateStarting, fallbackKind: store.FailureStartup},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			meta := store.SessionMeta{
				State:         string(tc.state),
				RuntimeStatus: store.SessionRuntimeReady,
				Failure: &store.SessionFailure{
					CrashBundlePath: "/tmp/compozy/crash-bundles/existing.json",
				},
			}

			repaired, changed := ClassifyInactiveMetaForRecovery(now, meta)
			if !changed {
				t.Fatal("ClassifyInactiveMetaForRecovery() changed = false, want true")
			}
			if repaired.Failure == nil {
				t.Fatal("ClassifyInactiveMetaForRecovery().Failure = nil, want repaired failure")
			}
			if got, want := repaired.Failure.Kind, tc.fallbackKind; got != want {
				t.Fatalf("Failure.Kind = %q, want %q", got, want)
			}
			if repaired.Failure.Summary == "" {
				t.Fatal("Failure.Summary = empty, want fallback summary filled")
			}
			if got, want := repaired.Failure.CrashBundlePath, "/tmp/compozy/crash-bundles/existing.json"; got != want {
				t.Fatalf("Failure.CrashBundlePath = %q, want %q", got, want)
			}
		})
	}
}

func TestValidateInfrastructure(t *testing.T) {
	t.Parallel()

	t.Run("Should valid infrastructure", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := validResumeMeta(h, "sess-valid")
		writeResumeEventStore(t, h.homePaths, meta.ID, []byte("not-empty"))

		errs := h.manager.validateInfrastructure(testutil.Context(t), meta)
		if len(errs) != 0 {
			t.Fatalf("validateInfrastructure() errors = %#v, want none", errs)
		}
	})

	t.Run("Should missing workspace directory", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		missingWorkspace := filepath.Join(t.TempDir(), "missing-workspace")
		h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      h.workspaceID,
				RootDir: missingWorkspace,
				Name:    h.workspaceName,
			},
			Config: h.cfg,
			Agents: []compozyconfig.AgentDef{{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			}},
		})
		meta := validResumeMeta(h, "sess-missing-workspace")
		writeResumeEventStore(t, h.homePaths, meta.ID, []byte("not-empty"))

		errs := h.manager.validateInfrastructure(testutil.Context(t), meta)
		assertErrorContains(t, errs, missingWorkspace)
	})

	t.Run("Should unresolvable agent", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := validResumeMeta(h, "sess-missing-agent")
		meta.AgentName = "missing-agent"
		writeResumeEventStore(t, h.homePaths, meta.ID, []byte("not-empty"))

		errs := h.manager.validateInfrastructure(testutil.Context(t), meta)
		assertErrorContains(t, errs, "missing-agent")
	})

	t.Run("Should missing event store", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := validResumeMeta(h, "sess-missing-store")

		errs := h.manager.validateInfrastructure(testutil.Context(t), meta)
		assertErrorContains(t, errs, store.SessionDBFile(filepath.Join(h.homePaths.SessionsDir, meta.ID)))
	})

	t.Run("Should empty event store", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := validResumeMeta(h, "sess-empty-store")
		writeResumeEventStore(t, h.homePaths, meta.ID, nil)

		errs := h.manager.validateInfrastructure(testutil.Context(t), meta)
		assertErrorContains(t, errs, "file is empty")
	})

	t.Run("Should invalid meta fields", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		meta := validResumeMeta(h, "")
		writeResumeEventStore(t, h.homePaths, "ignored", []byte("not-empty"))

		errs := h.manager.validateInfrastructure(testutil.Context(t), meta)
		assertErrorContains(t, errs, "session id")
	})

	t.Run("Should collects multiple failures", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		missingWorkspace := filepath.Join(t.TempDir(), "missing-workspace")
		h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      h.workspaceID,
				RootDir: missingWorkspace,
				Name:    h.workspaceName,
			},
			Config: h.cfg,
			Agents: []compozyconfig.AgentDef{{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			}},
		})
		meta := validResumeMeta(h, "sess-multi")
		meta.AgentName = "missing-agent"
		writeResumeEventStore(t, h.homePaths, meta.ID, nil)

		errs := h.manager.validateInfrastructure(testutil.Context(t), meta)
		if got, want := len(errs), 3; got != want {
			t.Fatalf("len(validateInfrastructure() errors) = %d, want %d (%#v)", got, want, errs)
		}
		assertErrorContains(t, errs, missingWorkspace)
		assertErrorContains(t, errs, "missing-agent")
		assertErrorContains(t, errs, "file is empty")
	})
}

func validResumeMeta(h *harness, sessionID string) store.SessionMeta {
	return store.SessionMeta{
		ID:                   sessionID,
		Name:                 "resume-session",
		AgentName:            "coder",
		WorkspaceID:          h.workspaceID,
		NetworkParticipation: testLocalParticipationPtr(),
		SessionType:          string(SessionTypeUser),
		State:                string(StateStopped),
		RuntimeStatus:        store.SessionRuntimeReady,
	}
}

func TestRepairRejectsNilContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(*testing.T) error
		want error
	}{
		{
			name: "Should reject nil context in repairInactiveMeta",
			call: func(t *testing.T) error {
				t.Helper()

				h := newHarness(t)
				meta := validResumeMeta(h, "sess-nil-repair-context")
				var nilCtx context.Context
				_, err := h.manager.repairInactiveMeta(nilCtx, filepath.Join(t.TempDir(), "meta.json"), meta)
				return err
			},
			want: errResumeRepairContextRequired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.call(t)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func writeResumeEventStore(t *testing.T, homePaths compozyconfig.HomePaths, sessionID string, contents []byte) string {
	t.Helper()

	sessionDir := filepath.Join(homePaths.SessionsDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", sessionDir, err)
	}

	dbPath := store.SessionDBFile(sessionDir)
	if err := os.WriteFile(dbPath, contents, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", dbPath, err)
	}
	return dbPath
}

func assertOptionalStopReasonEqual(t *testing.T, got *store.StopReason, want *store.StopReason) {
	t.Helper()

	switch {
	case got == nil && want == nil:
		return
	case got == nil || want == nil:
		t.Fatalf("stop reason = %v, want %v", got, want)
	case *got != *want:
		t.Fatalf("stop reason = %q, want %q", *got, *want)
	}
}

func assertOptionalStringEqual(t *testing.T, got *string, want *string) {
	t.Helper()

	switch {
	case got == nil && want == nil:
		return
	case got == nil || want == nil:
		t.Fatalf("string = %v, want %v", got, want)
	case *got != *want:
		t.Fatalf("string = %q, want %q", *got, *want)
	}
}

func assertErrorContains(t *testing.T, errs []error, fragment string) {
	t.Helper()

	for _, err := range errs {
		if err != nil && strings.Contains(err.Error(), fragment) {
			return
		}
	}
	t.Fatalf("errors %#v do not contain %q", errs, fragment)
}
