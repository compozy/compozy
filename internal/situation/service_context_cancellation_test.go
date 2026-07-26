package situation

import (
	"context"
	"errors"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	skillspkg "github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/soul"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestContextForSessionDependencyContextErrorsContract(t *testing.T) {
	t.Parallel()

	t.Run("Should propagate dependency context errors during session context assembly", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			deps    Deps
			info    *session.Info
			wantErr error
		}{
			{
				name: "skill registry cancellation",
				deps: Deps{
					Now:           fixedNow,
					SkillRegistry: contextErrorSkillRegistry{err: context.Canceled},
				},
				info:    sessionContextCancellationInfo(),
				wantErr: context.Canceled,
			},
			{
				name: "coordinator config deadline",
				deps: Deps{
					Now: fixedNow,
					CoordinatorRole: coordinatorResolverFunc(
						func(context.Context, string) (aghconfig.ResolvedCoordinatorRole, error) {
							return aghconfig.ResolvedCoordinatorRole{}, context.DeadlineExceeded
						},
					),
				},
				info:    sessionContextCancellationInfo(),
				wantErr: context.DeadlineExceeded,
			},
			{
				name: "soul snapshot cancellation",
				deps: Deps{
					Now:           fixedNow,
					SoulSnapshots: contextErrorSoulSnapshotStore{err: context.Canceled},
				},
				info: func() *session.Info {
					info := sessionContextCancellationInfo()
					info.SoulSnapshotID = "soul-snapshot-1"
					info.SoulDigest = "digest-1"
					return info
				}(),
				wantErr: context.Canceled,
			},
		}

		for _, tt := range tests {
			t.Run("Should propagate "+tt.name, func(t *testing.T) {
				t.Parallel()

				service := NewService(tt.deps)
				_, err := service.ContextForSession(context.Background(), tt.info)
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ContextForSession() error = %v, want %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("Should keep ordinary optional dependency failures best effort", func(t *testing.T) {
		t.Parallel()

		service := NewService(Deps{
			Now:           fixedNow,
			SkillRegistry: contextErrorSkillRegistry{err: errors.New("skill registry unavailable")},
			CoordinatorRole: coordinatorResolverFunc(
				func(context.Context, string) (aghconfig.ResolvedCoordinatorRole, error) {
					return aghconfig.ResolvedCoordinatorRole{}, errors.New("coordinator unavailable")
				},
			),
			SoulSnapshots: contextErrorSoulSnapshotStore{err: soul.ErrSnapshotNotFound},
		})

		info := sessionContextCancellationInfo()
		info.SoulSnapshotID = "soul-snapshot-1"
		info.SoulDigest = "digest-1"
		payload, err := service.ContextForSession(context.Background(), info)
		if err != nil {
			t.Fatalf("ContextForSession() error = %v, want nil", err)
		}
		if payload.Soul.Valid {
			t.Fatalf("Soul.Valid = true, want false for ordinary snapshot lookup failure")
		}
	})
}

func TestContextForStartupDependencyContextErrorsContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		deps    Deps
		wantErr error
	}{
		{
			name: "skill registry cancellation",
			deps: Deps{
				Now:           fixedNow,
				SkillRegistry: contextErrorSkillRegistry{err: context.Canceled},
			},
			wantErr: context.Canceled,
		},
		{
			name: "coordinator config deadline",
			deps: Deps{
				Now: fixedNow,
				CoordinatorRole: coordinatorResolverFunc(
					func(context.Context, string) (aghconfig.ResolvedCoordinatorRole, error) {
						return aghconfig.ResolvedCoordinatorRole{}, context.DeadlineExceeded
					},
				),
			},
			wantErr: context.DeadlineExceeded,
		},
	}

	for _, tt := range tests {
		t.Run("Should propagate "+tt.name, func(t *testing.T) {
			t.Parallel()

			service := NewService(tt.deps)
			_, err := service.ContextForStartup(
				context.Background(),
				session.StartupPromptContext{
					SessionID:   "sess-start",
					AgentName:   "coder",
					WorkspaceID: "ws-1",
					Workspace:   "/work/agh",
					SessionType: session.SessionTypeUser,
				},
				aghconfig.AgentDef{Name: "coder", Provider: "codex"},
				nil,
			)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ContextForStartup() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func sessionContextCancellationInfo() *session.Info {
	return &session.Info{
		ID:          "sess-1",
		AgentName:   "coder",
		Provider:    "codex",
		WorkspaceID: "ws-1",
		Workspace:   "/work/agh",
		Type:        session.SessionTypeUser,
		State:       session.StateActive,
		CreatedAt:   fixedTime(),
		UpdatedAt:   fixedTime(),
	}
}

type contextErrorSkillRegistry struct {
	err error
}

func (r contextErrorSkillRegistry) ForWorkspace(
	context.Context,
	*workspacepkg.ResolvedWorkspace,
) ([]*skillspkg.Skill, error) {
	return nil, r.err
}

func (r contextErrorSkillRegistry) ForAgent(
	context.Context,
	*workspacepkg.ResolvedWorkspace,
	string,
) ([]*skillspkg.Skill, error) {
	return nil, r.err
}

type contextErrorSoulSnapshotStore struct {
	err error
}

func (s contextErrorSoulSnapshotStore) GetSoulSnapshot(context.Context, string) (soul.Snapshot, error) {
	return soul.Snapshot{}, s.err
}
