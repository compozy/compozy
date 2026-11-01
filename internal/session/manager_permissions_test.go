package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestResumeUsesPersistedEffectivePermissions(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve a create-time permission override across resume", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.cfg.Permissions.Mode = compozyconfig.PermissionModeDenyAll
		h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
			Config:  h.cfg,
			Agents: []compozyconfig.AgentDef{
				{
					Name:     compozyconfig.DefaultAgentName,
					Provider: "claude",
					Prompt:   "You are a coding assistant.",
				},
				{
					Name:     "coder",
					Provider: "claude",
					Prompt:   "You are a coding assistant.",
				},
			},
		})
		h.manager = newManagerWithHarness(t, h)

		created, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName:   "coder",
			Workspace:   h.workspaceID,
			Permissions: compozyconfig.PermissionModeApproveAll,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if got := h.driver.startCalls[0].Permissions; got != compozyconfig.PermissionModeApproveAll {
			t.Fatalf("create start permissions = %q, want %q", got, compozyconfig.PermissionModeApproveAll)
		}
		createdPermissions := created.Info().EffectivePermissions
		if got, want := createdPermissions, string(compozyconfig.PermissionModeApproveAll); got != want {
			t.Fatalf("created info effective permissions = %q, want %q", got, want)
		}
		createdMeta := readMeta(t, created.MetaPath())
		got := createdMeta.EffectivePermissionsValue()
		want := string(compozyconfig.PermissionModeApproveAll)
		if got != want {
			t.Fatalf("created metadata effective permissions = %q, want %q", got, want)
		}
		wantAuthMode := string(compozyconfig.ProviderAuthModeNativeCLI)
		if got, want := createdMeta.EffectiveProviderAuthModeValue(), wantAuthMode; got != want {
			t.Fatalf("created metadata provider auth mode = %q, want %q", got, want)
		}

		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stopped, err := h.manager.Status(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Status(stopped) error = %v", err)
		}
		if got, want := stopped.EffectivePermissions, string(compozyconfig.PermissionModeApproveAll); got != want {
			t.Fatalf("stopped info effective permissions = %q, want %q", got, want)
		}
		resumed, err := h.manager.Resume(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("Stop(resumed) error = %v", err)
			}
		})

		if got := h.driver.startCalls[1].Permissions; got != compozyconfig.PermissionModeApproveAll {
			t.Fatalf("resume start permissions = %q, want persisted %q", got, compozyconfig.PermissionModeApproveAll)
		}
		resumedPermissions := resumed.Info().EffectivePermissions
		if got, want := resumedPermissions, string(compozyconfig.PermissionModeApproveAll); got != want {
			t.Fatalf("resumed info effective permissions = %q, want %q", got, want)
		}
		resumedMeta := readMeta(t, resumed.MetaPath())
		got = resumedMeta.EffectivePermissionsValue()
		if got != want {
			t.Fatalf("resumed metadata effective permissions = %q, want %q", got, want)
		}
		if got, want := resumedMeta.EffectiveProviderAuthModeValue(), wantAuthMode; got != want {
			t.Fatalf("resumed metadata provider auth mode = %q, want %q", got, want)
		}
	})
}

func TestInstancePermissionMode(t *testing.T) {
	t.Run("Should apply the parent instance ceiling before a derived session is registered", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		parent := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, parent.ID) })
		h.manager.SetInstanceCeilingReader(
			func(_ context.Context, id, _ string) (*store.SessionPermissionPolicy, string, error) {
				if id == parent.ID {
					return &store.SessionPermissionPolicy{WorkspacePaths: []string{h.workspace}}, "", nil
				}
				return nil, "", nil
			},
		)
		child, err := h.manager.Create(t.Context(), CreateOpts{
			AgentName:   "coder",
			Workspace:   h.workspaceID,
			Permissions: compozyconfig.PermissionModeApproveAll,
			Lineage: &store.SessionLineage{
				ParentSessionID: parent.ID,
				Kind:            store.LineageKindFork,
				OriginAgentName: "coder",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, child.ID) })
		if got := child.Info().EffectivePermissions; got != string(compozyconfig.PermissionModeApproveReads) {
			t.Fatalf("first derived launch permissions = %s", got)
		}
		h.driver.mu.Lock()
		defer h.driver.mu.Unlock()
		if got := h.driver.startCalls[len(h.driver.startCalls)-1].Permissions; got != compozyconfig.PermissionModeApproveReads {
			t.Fatalf("provider received permissions = %s", got)
		}
	})

	for _, tc := range []struct{ mode, want compozyconfig.PermissionMode }{
		{compozyconfig.PermissionModeApproveAll, compozyconfig.PermissionModeApproveReads},
		{compozyconfig.PermissionModeApproveReads, compozyconfig.PermissionModeApproveReads},
		{compozyconfig.PermissionModeDenyAll, compozyconfig.PermissionModeDenyAll},
	} {
		t.Run(
			"Should retain bounded "+string(tc.mode)+" permissions across resume and context replacement",
			func(t *testing.T) {
				t.Parallel()
				h := newHarness(t)
				h.manager = newManagerWithHarness(t, h)
				cleanupTestManager(t, h.manager)
				h.manager.SetInstanceCeilingReader(
					func(context.Context, string, string) (*store.SessionPermissionPolicy, string, error) {
						return &store.SessionPermissionPolicy{WorkspacePaths: []string{h.workspace}}, "", nil
					},
				)
				created, err := h.manager.Create(
					t.Context(),
					CreateOpts{AgentName: "coder", Workspace: h.workspaceID, Permissions: tc.mode},
				)
				if err != nil {
					t.Fatal(err)
				}
				if got := created.Info().EffectivePermissions; got != string(tc.want) {
					t.Fatalf("launch permissions=%s, want %s", got, tc.want)
				}
				if err := h.manager.Stop(t.Context(), created.ID); err != nil {
					t.Fatal(err)
				}
				resumed, err := h.manager.Resume(t.Context(), created.ID)
				if err != nil {
					t.Fatal(err)
				}
				if got := resumed.Info().EffectivePermissions; got != string(tc.want) {
					t.Fatalf("resume permissions=%s, want %s", got, tc.want)
				}
				cleared, err := h.manager.ClearConversation(t.Context(), resumed.ID)
				if err != nil {
					t.Fatal(err)
				}
				if err := waitForSessionStartRun(t.Context(), h.manager.sessionStartRun(cleared.ID)); err != nil {
					t.Fatal(err)
				}
				if got := cleared.Info().EffectivePermissions; got != string(tc.want) {
					t.Fatalf("replacement permissions=%s, want %s", got, tc.want)
				}
				events, err := h.manager.Prompt(t.Context(), cleared.ID, "Continue within the instance permissions")
				if err != nil {
					t.Fatal(err)
				}
				collectEvents(t, events)
				h.driver.mu.Lock()
				defer h.driver.mu.Unlock()
				if len(h.driver.startCalls) != 3 {
					t.Fatalf("provider launches=%d, want 3", len(h.driver.startCalls))
				}
				for _, launch := range h.driver.startCalls {
					if launch.Permissions != tc.want {
						t.Fatalf("provider permissions=%s, want %s", launch.Permissions, tc.want)
					}
				}
			},
		)
	}
}

func TestTriageSessionPermissions(t *testing.T) {
	t.Run("Should preserve triage restrictions across stop and resume", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		created, err := h.manager.Create(t.Context(), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeTriage,
			Permissions: compozyconfig.PermissionModeApproveAll,
		})
		if err != nil {
			t.Fatal(err)
		}
		assertTriage := func() {
			t.Helper()
			opts := h.driver.startCalls[len(h.driver.startCalls)-1]
			if opts.ExecutionPolicy != "triage" || opts.Permissions != compozyconfig.PermissionModeApproveReads ||
				len(opts.MCPServers) != 0 ||
				opts.ActivateMCPServers != nil {
				t.Fatalf(
					"unsafe start opts: policy=%s permissions=%s mcp=%v",
					opts.ExecutionPolicy,
					opts.Permissions,
					opts.MCPServers,
				)
			}
		}
		assertTriage()
		if isPublicSessionCatalogInfo(created.Info()) {
			t.Fatal("triage session leaked into public catalog")
		}
		meta := readMeta(t, created.MetaPath())
		if meta.SessionType != "triage" {
			t.Fatalf("persisted type=%s", meta.SessionType)
		}
		if err := h.manager.Stop(t.Context(), created.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.manager.Resume(t.Context(), created.ID); err != nil {
			t.Fatal(err)
		}
		assertTriage()
	})
}

type triageTestReports struct {
	reports  []agentinstance.Report
	onRecent func(string, int)
}

var _ TriageReportReader = triageTestReports{}

func (r triageTestReports) RecentReports(
	_ context.Context, instanceID string, limit int,
) ([]agentinstance.Report, error) {
	if r.onRecent != nil {
		r.onRecent(instanceID, limit)
	}
	return r.reports, nil
}

func TestTriageDispatcher(t *testing.T) {
	for _, tc := range []struct{ primaryFailure, stopFailure bool }{
		{}, {primaryFailure: true}, {stopFailure: true},
	} {
		name := "Should preserve a proposal and recover failed cleanup after restart"
		if tc.primaryFailure {
			name = "Should preserve the primary failure when cleanup also fails"
		}
		if tc.stopFailure {
			name = "Should preserve a proposal and retry unverified stop cleanup"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cleanupFailure := errors.New("archive unavailable")
			if tc.primaryFailure {
				cleanupFailure = errors.Join(cleanupFailure, context.DeadlineExceeded)
			}
			fault := &triageArchiveFault{failure: cleanupFailure}
			fault.fail.Store(!tc.stopFailure)
			dh := newDeriveHarness(t, func(db *globaldb.GlobalDB) Option {
				fault.GlobalDB = db
				return WithSessionCatalog(fault)
			}, func(*globaldb.GlobalDB) Option {
				return WithSessionStopConfig(compozyconfig.SessionStopConfig{CooperativeGrace: 5 * time.Millisecond})
			})
			h := dh.harness
			if tc.stopFailure {
				h.driver.stopHook = func(*fakeProcess) error { return cleanupFailure }
				h.driver.killHook = func(*fakeProcess) error { return cleanupFailure }
				h.driver.verifyExitHook = func(*AgentProcess) (bool, error) { return false, cleanupFailure }
			}
			h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				events := make(chan acp.AgentEvent, 2)
				events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, SessionID: proc.handle.SessionID, TurnID: req.TurnID, Text: `{"proposals":[{"title":"Review","body":"Inspect work","item_keys":[]}]}`}
				events <- acp.AgentEvent{Type: acp.EventTypeDone, SessionID: proc.handle.SessionID, TurnID: req.TurnID, StopReason: string(acp.PromptStopReasonEndTurn), PromptStopReason: acp.PromptStopReasonEndTurn}
				close(events)
				return events, nil
			}
			dispatcher := &TriageDispatcher{
				Manager:      h.manager,
				Reports:      triageTestReports{},
				MaxProposals: 2,
				StopTimeout:  time.Second,
			}
			var hidden string
			var exited <-chan struct{}
			primary := errors.New("occurrence binding failed")
			outcome, err := dispatcher.Dispatch(t.Context(), heartbeat.CheckTarget{Instance: agentinstance.Instance{
				ID: "cleanup-instance", AgentName: "coder", WorkspaceID: h.workspaceID,
			}}, heartbeat.CheckDigest{}, func(id string) error {
				hidden = id
				child, ok := h.manager.Get(id)
				if !ok {
					return errors.New("triage runtime missing")
				}
				exited = child.processHandle().Done()
				if tc.primaryFailure {
					return primary
				}
				return nil
			})
			if !tc.stopFailure && fault.fail.Load() {
				t.Fatal("archive failure was not exercised")
			}
			if tc.primaryFailure {
				if !errors.Is(err, primary) || errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("error=%v", err)
				}
			} else if err != nil || outcome.Result != "reported" || outcome.Report == nil || !strings.Contains(string(outcome.Report.Payload), "Inspect work") {
				t.Fatalf("outcome=%+v error=%v", outcome, err)
			}
			info, err := h.manager.Status(t.Context(), hidden)
			if err != nil || info.ArchivedAt != nil {
				t.Fatalf("pending cleanup=%+v error=%v", info, err)
			}
			if tc.stopFailure {
				if info.State != StateStopping {
					t.Fatalf("unverified state=%s", info.State)
				}
				h.driver.mu.Lock()
				h.driver.stopHook, h.driver.killHook, h.driver.verifyExitHook = nil, nil, nil
				h.driver.mu.Unlock()
				if err := h.manager.SweepTriageSessions(t.Context(), info.CreatedAt.Add(-time.Hour)); err != nil {
					t.Fatal(err)
				}
				stopped, err := h.manager.AwaitStopped(t.Context(), hidden)
				if err != nil || !stopped.Verified || stopped.FinalState != StateStopped {
					t.Fatalf("recovered stop=%+v error=%v", stopped, err)
				}
			} else if info.State != StateStopped {
				t.Fatalf("state=%s", info.State)
			}
			select {
			case <-exited:
			default:
				t.Fatal("triage provider was not stopped")
			}

			dh.restartManager(t)
			if err := dh.manager.SweepTriageSessions(t.Context(), info.CreatedAt.Add(-time.Hour)); err != nil {
				t.Fatal(err)
			}
			info, err = dh.manager.Status(t.Context(), hidden)
			if err != nil || info.ArchivedAt == nil {
				t.Fatalf("recovered archive=%+v error=%v", info, err)
			}
			if err := dh.manager.SweepTriageSessions(t.Context(), info.ArchivedAt.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if _, err := dh.manager.Status(t.Context(), hidden); !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("expired triage error=%v", err)
			}
		})
	}

	t.Run("Should discard failed triage starts without durable artifacts", func(t *testing.T) {
		t.Parallel()
		dh := newDeriveHarness(t)
		h := dh.harness
		h.driver.startHook = func(acp.StartOpts, int) (*fakeProcess, error) {
			return nil, acp.ErrTriageUnsupported
		}
		dispatcher := &TriageDispatcher{
			Manager: h.manager, Reports: triageTestReports{}, MaxProposals: 2, StopTimeout: time.Second,
		}
		outcome, err := dispatcher.Dispatch(
			t.Context(),
			heartbeat.CheckTarget{Instance: agentinstance.Instance{
				ID: "failed-instance", AgentName: "coder", WorkspaceID: h.workspaceID,
			}},
			heartbeat.CheckDigest{},
			func(string) error { t.Error("failed start bound a session"); return nil },
		)
		if err != nil || outcome.Reason != "triage_unsupported" {
			t.Fatalf("outcome=%+v error=%v", outcome, err)
		}
		infos, err := h.manager.ListAll(t.Context())
		if err != nil || len(infos) != 0 {
			t.Fatalf("sessions=%+v error=%v", infos, err)
		}
		rows, err := dh.db.ListSessions(t.Context(), store.SessionListQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, Archive: store.SessionArchiveInclude,
		})
		if err != nil || len(rows) != 0 {
			t.Fatalf("catalog rows=%v error=%v", rows, err)
		}

		entries, err := os.ReadDir(h.homePaths.SessionsDir)
		if err != nil || len(entries) != 0 {
			t.Fatalf("directories=%v error=%v", entries, err)
		}
	})
	t.Run("Should archive recovered triage and expire only retained triage", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t).harness
		ordinary := createSession(t, h)
		triage, err := h.manager.Create(t.Context(), CreateOpts{
			AgentName: "coder", Workspace: h.workspaceID, Type: SessionTypeTriage,
		})
		if err != nil {
			t.Fatal(err)
		}
		dispatcher := &TriageDispatcher{Manager: h.manager}
		if err := dispatcher.Stop(t.Context(), triage.ID); err != nil {
			t.Fatal(err)
		}
		info, err := h.manager.Status(t.Context(), triage.ID)
		if err != nil || info.ArchivedAt == nil {
			t.Fatalf("info=%+v error=%v", info, err)
		}
		cutoff := *info.ArchivedAt
		if err := dispatcher.Stop(t.Context(), triage.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.manager.SweepTriageSessions(t.Context(), cutoff.Add(-time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := h.manager.Status(t.Context(), triage.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.manager.Stop(t.Context(), ordinary.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.manager.Archive(t.Context(), h.workspaceID, ordinary.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.manager.SweepTriageSessions(t.Context(), cutoff.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if _, err := h.manager.Status(t.Context(), triage.ID); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("expired triage error=%v", err)
		}
		if _, err := h.manager.Status(t.Context(), ordinary.ID); err != nil {
			t.Fatalf("ordinary session lost: %v", err)
		}
		if err := dispatcher.Stop(t.Context(), triage.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Should select the configured triage model from a TOML overlay", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t).harness
		workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(
			path,
			[]byte("[agents.heartbeat.triage_models]\nclaude = \"claude-haiku-4-5\"\n"),
			0600,
		); err != nil {
			t.Fatal(err)
		}
		if err := compozyconfig.ApplyConfigOverlayFile(path, &workspace.Config); err != nil {
			t.Fatal(err)
		}
		h.resolver.upsert(&workspace)
		recentReads := 0
		dispatcher := &TriageDispatcher{
			Manager: h.manager,
			Reports: triageTestReports{onRecent: func(instanceID string, limit int) {
				recentReads++
				if instanceID != "model-instance" || limit != 20 {
					t.Errorf("recent query=%s limit=%d", instanceID, limit)
				}
			}},
			MaxProposals: 2, StopTimeout: time.Second,
		}
		_, err = dispatcher.Dispatch(
			t.Context(),
			heartbeat.CheckTarget{
				Instance: agentinstance.Instance{ID: "model-instance", AgentName: "coder", WorkspaceID: h.workspaceID},
			},
			heartbeat.CheckDigest{},
			func(id string) error {
				info, err := h.manager.Status(t.Context(), id)
				if err != nil {
					return err
				}
				if info.InitiatorKind != "daemon" || info.InitiatorID != "model-instance" {
					t.Errorf("initiator=%s:%s", info.InitiatorKind, info.InitiatorID)
				}
				meta, err := h.manager.readMetaWithContext(t.Context(), id)
				if err != nil {
					return err
				}
				if meta.InitiatorKind != "daemon" || meta.InitiatorID != "model-instance" {
					t.Errorf("persisted initiator=%s:%s", meta.InitiatorKind, meta.InitiatorID)
				}
				if info.Model != "claude-haiku-4-5" {
					t.Errorf("triage model=%s", info.Model)
				}
				return nil
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if recentReads != 1 {
			t.Fatalf("recent report reads=%d", recentReads)
		}
		h.driver.mu.Lock()
		defer h.driver.mu.Unlock()
		if len(h.driver.startCalls) != 1 || h.driver.startCalls[0].PreferredModel != "claude-haiku-4-5" {
			t.Fatalf("launches=%+v", h.driver.startCalls)
		}
	})

	t.Run("Should delete quiet triage without prompting the main session", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t).harness
		main := createSession(t, h)
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			if strings.Contains(req.Message, "```digest") || strings.Contains(req.Message, "```report") ||
				!strings.Contains(req.Message, `\u0060\u0060\u0060digest`) ||
				!strings.Contains(req.Message, `\u0060\u0060\u0060report`) {
				t.Errorf("untrusted triage data escaped its JSON fence")
			}
			events := make(chan acp.AgentEvent, 3)
			now := time.Now().UTC()
			events <- acp.AgentEvent{Type: acp.EventTypePermission, TurnID: req.TurnID, Timestamp: now, Decision: string(acp.PermissionDecisionRejectOnce)}.WithRequestID("triage-denial")
			events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, SessionID: proc.handle.SessionID, TurnID: req.TurnID, Timestamp: now, Text: "NOTHING_TO_REPORT"}
			events <- acp.AgentEvent{Type: acp.EventTypeDone, SessionID: proc.handle.SessionID, TurnID: req.TurnID, Timestamp: now, StopReason: string(acp.PromptStopReasonEndTurn), PromptStopReason: acp.PromptStopReasonEndTurn}
			close(events)
			return events, nil
		}
		dispatcher := &TriageDispatcher{
			Manager: h.manager,
			Reports: triageTestReports{
				reports: []agentinstance.Report{{Payload: []byte("{\"body\":\"```report\"}")}},
			},
			MaxProposals: 2,
			StopTimeout:  time.Second,
		}
		var hidden string
		outcome, err := dispatcher.Dispatch(
			t.Context(),
			heartbeat.CheckTarget{
				Instance: agentinstance.Instance{
					ID:            "instance-test",
					AgentName:     "coder",
					WorkspaceID:   h.workspaceID,
					MainSessionID: main.ID,
				},
			},
			heartbeat.CheckDigest{Items: []heartbeat.DigestItem{{Key: "```digest"}}},
			func(id string) error { hidden = id; return nil },
		)
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Result != "quiet" || outcome.Report != nil || outcome.DeniedCalls != 1 {
			t.Fatalf("outcome=%+v", outcome)
		}
		if _, err := h.manager.Status(t.Context(), hidden); err == nil {
			t.Fatal("quiet session retained")
		}
		if err := dispatcher.Stop(t.Context(), hidden); err != nil {
			t.Fatalf("recover deleted quiet triage: %v", err)
		}
		h.driver.mu.Lock()
		defer h.driver.mu.Unlock()
		if len(h.driver.promptCalls) != 1 {
			t.Fatalf("prompts=%d", len(h.driver.promptCalls))
		}
		if len(h.driver.startCalls) != 2 ||
			h.driver.startCalls[1].PreferredModel != h.driver.startCalls[0].PreferredModel {
			t.Fatalf("triage changed the default model: %+v", h.driver.startCalls)
		}
	})

	t.Run("Should finalize invalid provider output and stop the triage session", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t).harness
		dispatcher := &TriageDispatcher{
			Manager:      h.manager,
			Reports:      triageTestReports{},
			MaxProposals: 2,
			StopTimeout:  time.Second,
		}
		outcome, err := dispatcher.Dispatch(
			t.Context(),
			heartbeat.CheckTarget{
				Instance: agentinstance.Instance{ID: "instance-test", AgentName: "coder", WorkspaceID: h.workspaceID},
			},
			heartbeat.CheckDigest{},
			func(string) error { return nil },
		)
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Result != "failed" || outcome.Reason != "triage_output_invalid" {
			t.Fatalf("outcome=%+v", outcome)
		}
		h.driver.mu.Lock()
		defer h.driver.mu.Unlock()
		if len(h.driver.promptCalls) != 1 || h.driver.stopCalls != 1 {
			t.Fatalf("prompts=%d stops=%d", len(h.driver.promptCalls), h.driver.stopCalls)
		}
	})

	t.Run("Should stop the hidden session without prompting when occurrence binding fails", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t).harness
		dispatcher := &TriageDispatcher{
			Manager:      h.manager,
			Reports:      triageTestReports{},
			MaxProposals: 2,
			StopTimeout:  time.Second,
		}
		denied := errors.New("occurrence cancelled")
		var id string
		_, err := dispatcher.Dispatch(
			t.Context(),
			heartbeat.CheckTarget{
				Instance: agentinstance.Instance{ID: "instance-test", AgentName: "coder", WorkspaceID: h.workspaceID},
			},
			heartbeat.CheckDigest{},
			func(sessionID string) error { id = sessionID; return denied },
		)
		if !errors.Is(err, denied) {
			t.Fatalf("dispatch error=%v", err)
		}
		if id == "" {
			t.Fatal("session was not bound")
		}
		h.driver.mu.Lock()
		defer h.driver.mu.Unlock()
		if len(h.driver.promptCalls) != 0 || h.driver.stopCalls != 1 {
			t.Fatalf("prompts=%d stops=%d", len(h.driver.promptCalls), h.driver.stopCalls)
		}
	})
}

type dueTriageEnvironment struct{ target heartbeat.CheckTarget }

var _ heartbeat.CheckEnvironment = dueTriageEnvironment{}

func (e dueTriageEnvironment) ResolveCheck(context.Context, string) (heartbeat.CheckTarget, error) {
	return e.target, nil
}
func (dueTriageEnvironment) BeforeCheck(context.Context, heartbeat.CheckTarget, string) (string, error) {
	return "", nil
}

func (dueTriageEnvironment) Collect(
	context.Context,
	string,
	heartbeat.CollectorScope,
	[]string,
	string,
	string,
) (heartbeat.CheckDigest, error) {
	return heartbeat.CheckDigest{Fingerprint: "due"}, nil
}

func TestDueInstanceTriagePolicy(t *testing.T) {
	t.Run("Should restrict due triage and persist unsupported provider failure", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t).harness
		h.cfg.Permissions.Mode = compozyconfig.PermissionModeApproveAll
		h.manager = newManagerWithHarness(t, h)
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "triage.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		instance, _, err := db.Instances().
			AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", CreatedByKind: "human", PermissionCeiling: "{}"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().
			ExecContext(t.Context(), "UPDATE agent_instances SET state='active' WHERE id=?", instance.ID); err != nil {
			t.Fatal(err)
		}
		instance.State = "active"
		instance.WorkspaceID = h.workspaceID
		starts := make(chan acp.StartOpts, 1)
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			starts <- opts
			return nil, acp.ErrTriageUnsupported
		}
		dispatcher := &TriageDispatcher{
			Manager:      h.manager,
			Reports:      db.InstanceReports(),
			MaxProposals: 2,
			StopTimeout:  time.Second,
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		instance.CreatedAt = now.Add(-24 * time.Hour)
		service, err := heartbeat.NewCheckService(
			t.Context(),
			db.Instances(),
			dueTriageEnvironment{
				target: heartbeat.CheckTarget{
					Instance: instance,
					Policy:   heartbeat.ResolvedPolicy{Enabled: true, GuidanceMarkdown: "Check work"},
				},
			},
			dispatcher,
			heartbeat.CheckConfig{
				Enabled:       true,
				Interval:      time.Hour,
				MaxInterval:   24 * time.Hour,
				Retention:     24 * time.Hour,
				Timeout:       time.Minute,
				MaxConcurrent: 1,
				BatchSize:     10,
			},
			func() time.Time { return now },
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := service.Tick(t.Context()); err != nil {
			t.Fatal(err)
		}
		select {
		case opts := <-starts:
			if opts.ExecutionPolicy != "triage" || opts.Permissions != compozyconfig.PermissionModeApproveReads ||
				len(opts.MCPServers) != 0 ||
				opts.ActivateMCPServers != nil {
				t.Fatalf("unsafe options: policy=%s permissions=%s", opts.ExecutionPolicy, opts.Permissions)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("due check did not start")
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			reports, err := db.InstanceReports().
				ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 10})
			if err != nil {
				t.Fatal(err)
			}
			if len(reports) == 1 {
				if string(
					reports[0].Payload,
				) != `{"reason":"triage_unsupported","detail":"The provider does not support read-only triage."}` {
					t.Fatalf("report=%s", reports[0].Payload)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("unsupported triage did not settle")
			}
			time.Sleep(time.Millisecond)
		}
		if err := service.Shutdown(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestDraftInstructionsPolicy(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		name := "Should return instructions under read-only policy and delete the temporary session"
		if timeout {
			name = "Should delete the temporary session after drafting times out"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newDeriveHarness(t)
			ctx := t.Context()
			if timeout {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				events := make(chan acp.AgentEvent)
				var once sync.Once
				h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) { return events, nil }
				h.driver.cancelHook = func(_ *fakeProcess) error { once.Do(func() { close(events) }); return nil }
			}
			text, err := h.manager.DraftInstructions(
				ctx,
				DraftInstructionsRequest{Goal: "Review incoming requests", WorkspaceID: h.workspaceID},
			)

			if timeout {
				if !errors.Is(err, ErrAgentDraftFailed) || !errors.Is(err, context.DeadlineExceeded) || text != "" {
					t.Fatalf("timeout draft = %q, %v", text, err)
				}
			} else if err != nil || text != "reply" {
				t.Fatalf("draft = %q, %v", text, err)
			}
			if timeout && len(h.promptMessages()) != 1 {
				t.Fatal("deadline expired before the provider prompt")
			}
			if sessions := h.manager.List(); len(sessions) != 0 {
				t.Fatalf("temporary sessions remain: %v", sessions)
			}
			var count int
			if err := h.db.DB().QueryRowContext(t.Context(), "SELECT COUNT(*) FROM sessions").Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("persisted temporary sessions: %d", count)
			}
			for _, opts := range h.startCallsSnapshot() {
				if opts.ExecutionPolicy != acp.ExecutionPolicyTriage || len(opts.MCPServers) != 0 {
					t.Fatalf("unsafe launch: policy %s, MCP servers %d", opts.ExecutionPolicy, len(opts.MCPServers))
				}
			}
		})
	}

	t.Run("Should reject providers without read-only triage support", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t).harness
		starts := make(chan acp.StartOpts, 1)
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			starts <- opts
			return nil, acp.ErrTriageUnsupported
		}
		text, err := h.manager.DraftInstructions(t.Context(), DraftInstructionsRequest{
			Goal: "Review incoming requests", WorkspaceID: h.workspaceID,
		})
		if !errors.Is(err, acp.ErrTriageUnsupported) || text != "" {
			t.Fatalf("draft = %q, %v; want triage unsupported", text, err)
		}
		select {
		case opts := <-starts:
			if opts.ExecutionPolicy != acp.ExecutionPolicyTriage || len(opts.MCPServers) != 0 {
				t.Fatalf("launch policy = %s, MCP servers = %d", opts.ExecutionPolicy, len(opts.MCPServers))
			}
		default:
			t.Fatal("provider was not started")
		}
	})
}

type triageArchiveFault struct {
	*globaldb.GlobalDB
	failure error
	fail    atomic.Bool
}

var _ store.SessionArchiveStore = (*triageArchiveFault)(nil)

func (f *triageArchiveFault) SetSessionArchived(
	ctx context.Context,
	workspaceID, sessionID string,
	archived bool,
) (store.SessionInfo, error) {
	if archived && f.fail.CompareAndSwap(true, false) {
		return store.SessionInfo{}, f.failure
	}
	return f.GlobalDB.SetSessionArchived(ctx, workspaceID, sessionID, archived)
}
