package daemon

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspace"
)

// Suite: daemon session work sources
// Invariant: only operator-visible hierarchy children count as a session's active-child
// work; a session continued or forked from it names it as provenance and never defers its
// stop, and a daemon-owned internal child (auto-title) never holds it waiting.
func TestSessionWorkSourcesChildren(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		agent   string
		typ     session.Type
		lineage store.SessionLineage
	}{
		{
			name:  "Should not count a session continued from the parent as its child work",
			agent: "reviewer", typ: session.SessionTypeUser,
			lineage: store.SessionLineage{
				ParentSessionID: "sess-source", RootSessionID: "sess-source", SpawnDepth: 1,
				Kind: store.LineageKindContinue, OriginAgentName: "coder",
			},
		},
		{
			name:  "Should not count a daemon-owned auto-title child as its child work",
			agent: "titler", typ: session.SessionTypeSpawned,
			lineage: store.SessionLineage{
				ParentSessionID: "sess-source", RootSessionID: "sess-source", SpawnDepth: 1,
				Kind: store.LineageKindSpawn, SpawnRole: session.SpawnRoleAutoTitle,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertNoChildWork(t, tc.agent, tc.typ, tc.lineage)
		})
	}
}

func assertNoChildWork(t *testing.T, agent string, typ session.Type, lineage store.SessionLineage) {
	t.Helper()
	ctx := t.Context()
	registry, err := openDaemonTestGlobalDBAtPath(ctx, filepath.Join(t.TempDir(), "compozy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := registry.Close(context.Background()); err != nil {
			t.Errorf("close work store: %v", err)
		}
	})
	now := time.Now().UTC()
	if err := registry.InsertWorkspace(ctx, workspace.Workspace{
		ID: "ws-work", Name: "ws-work", RootDir: t.TempDir(), CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	homePaths := testHomePaths(t)
	if err := store.WriteSessionMeta(
		store.SessionMetaFile(filepath.Join(homePaths.SessionsDir, "sess-source")),
		&store.SessionMeta{
			ID: "sess-source", ProfileID: store.DefaultProfileID, WorkspaceID: "ws-work",
			AgentName: "coder", Provider: "mock", State: string(session.StateStopped),
			RuntimeStatus: store.SessionRuntimeUnbound, CreatedAt: now, UpdatedAt: now,
		},
	); err != nil {
		t.Fatal(err)
	}
	// The related session has no readable state: counting it would have to read it.
	if err := registry.RegisterSession(ctx, new(store.SessionInfo{
		ProfileID: store.DefaultProfileID, ID: "sess-related", AgentName: agent,
		WorkspaceID: "ws-work", SessionType: string(typ),
		State: string(session.StateActive), RuntimeStatus: store.SessionRuntimeUnbound,
		Lineage: &lineage, CreatedAt: now, UpdatedAt: now,
	})); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
	manager, err := session.NewManager(session.WithHomePaths(homePaths), session.WithLogger(discardLogger()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown work manager: %v", err)
		}
	})
	sources := sessionWorkSources{state: &bootState{registry: registry}, manager: manager, now: time.Now}

	work, err := sources.children(ctx, "sess-source")
	if err != nil {
		t.Fatalf("children() error = %v, want the related session ignored", err)
	}
	if len(work) != 0 {
		t.Fatalf("children() = %#v, want no child work", work)
	}
}

func TestSessionWorkSourcesMonitors(t *testing.T) {
	t.Parallel()
	t.Run("Should project only the owning session live monitors without renewing their deadline", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
		service, err := monitor.NewService(
			t.Context(),
			nil,
			&monitorSessionWaker{sessions: func() monitorSessionManager { return nil }},
			monitor.WithClock(func() time.Time { return now }),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := service.Shutdown(context.Background()); err != nil {
				t.Errorf("shutdown monitors: %v", err)
			}
		})
		sources := sessionWorkSources{state: &bootState{sessionMonitors: service}}
		item, err := service.Create(
			t.Context(),
			monitor.Spec{
				WorkspaceID: "ws",
				SessionID:   "owner",
				Kind:        monitor.KindTimer,
				Description: "wait",
				Delay:       time.Hour,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		signals, err := sources.monitors(t.Context(), "owner")
		if err != nil || len(signals) != 1 {
			t.Fatalf("signals = %#v, %v", signals, err)
		}
		got := signals[0]
		if got.Kind != session.WorkSignalActiveMonitor || got.Ref != item.ID || !got.Since.Equal(item.CreatedAt) ||
			!got.ValidUntil.Equal(item.DeadlineAt) {
			t.Fatalf("signal = %#v", got)
		}
		if signals, err := sources.monitors(t.Context(), "other"); err != nil || len(signals) != 0 {
			t.Fatalf("foreign signals = %#v, %v", signals, err)
		}
		if _, err := service.Cancel("owner", item.ID); err != nil {
			t.Fatal(err)
		}
		if signals, err := sources.monitors(t.Context(), "owner"); err != nil || len(signals) != 0 {
			t.Fatalf("canceled signals = %#v, %v", signals, err)
		}
	})
	t.Run("Should retain waiting while an expired timer retries wake delivery", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			service, err := monitor.NewService(t.Context(), nil, retryMonitorWaker{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := service.Shutdown(context.Background()); err != nil {
					t.Error(err)
				}
			})
			sources := sessionWorkSources{state: &bootState{sessionMonitors: service}, now: time.Now}
			item, err := service.Create(
				t.Context(),
				monitor.Spec{
					WorkspaceID: "ws",
					SessionID:   "owner",
					Kind:        monitor.KindTimer,
					Description: "wait",
					Delay:       time.Minute,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(time.Minute + time.Second)
			synctest.Wait()
			if current := service.List("owner"); len(current) != 1 || current[0].Status != monitor.StatusDelivering {
				t.Fatalf("monitors = %#v", current)
			}
			signals, err := sources.monitors(t.Context(), "owner")
			if err != nil || len(signals) != 1 || !signals[0].ValidUntil.After(time.Now()) ||
				signals[0].Ref != item.ID {
				t.Fatalf("delivering signals = %#v, %v", signals, err)
			}
			if _, err := service.Cancel("owner", item.ID); err != nil {
				t.Fatal(err)
			}
			if signals, err := sources.monitors(t.Context(), "owner"); err != nil || len(signals) != 0 {
				t.Fatalf("canceled signals = %#v, %v", signals, err)
			}
		})
	})
	t.Run("Should fail closed when the monitor service is unavailable", func(t *testing.T) {
		t.Parallel()
		sources := sessionWorkSources{state: &bootState{}}
		if _, err := sources.monitors(t.Context(), "owner"); err == nil {
			t.Fatal("missing monitor service was reported as no work")
		}
	})
}

type retryMonitorWaker struct{}

func (retryMonitorWaker) WakeSession(context.Context, monitor.Notification) error {
	return errors.New("input queue full")
}
