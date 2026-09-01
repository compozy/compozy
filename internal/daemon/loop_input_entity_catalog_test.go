package daemon

import (
	"context"
	"testing"
	"time"

	apitest "github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/loop/dsl"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestDaemonLoopInputEntityCatalogShouldResolveAgentsInTheActingProfile(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	database := openDaemonTestGlobalDB(t)
	homePaths := testHomePaths(t)
	profiles, err := profilepkg.NewManager(
		profilepkg.WithStore(database),
		profilepkg.WithHomePaths(homePaths),
		profilepkg.WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("profile.NewManager() error = %v", err)
	}
	engineering, err := profiles.Create(ctx, profilepkg.CreateInput{Name: "engineering"})
	if err != nil {
		t.Fatalf("profiles.Create(engineering) error = %v", err)
	}

	now := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	workspaceRoot := t.TempDir()
	workspaceID := "ws-profile-loop-input"
	if err := database.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: workspaceID, Name: workspaceID, RootDir: workspaceRoot,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	resolver, err := workspacepkg.NewResolver(
		database,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithProfileAvailabilityChecker(profiles),
		workspacepkg.WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("workspace.NewResolver() error = %v", err)
	}
	agents := newResourceCatalog(cloneAgentDef)
	agents.Replace(1, []resources.Record[compozyconfig.AgentDef]{{
		ID: "extension-profile-engineer",
		Scope: resources.ResourceScope{
			Kind: resources.ResourceScopeKindProfile,
			ID:   engineering.ID,
		},
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("extension"),
			ID:   "qa-lab",
		},
		Spec: compozyconfig.AgentDef{Name: "engineer", Prompt: "Use the Agent-local skill."},
	}})
	catalog := daemonLoopInputEntityCatalog{state: &bootState{
		profiles: profiles, workspaceResolver: resolver, agentCatalog: agents,
	}}

	found, err := catalog.HasInputEntity(
		ctx, "ws-profile-loop-input", engineering.ID, dsl.EntityKindAgent, "engineer",
	)
	if err != nil {
		t.Fatalf("HasInputEntity(engineering engineer) error = %v", err)
	}
	if !found {
		t.Fatal("HasInputEntity(engineering engineer) = false, want true")
	}
	found, err = catalog.HasInputEntity(
		ctx, "ws-profile-loop-input", store.DefaultProfileID, dsl.EntityKindAgent, "engineer",
	)
	if err != nil {
		t.Fatalf("HasInputEntity(default engineer) error = %v", err)
	}
	if found {
		t.Fatal("HasInputEntity(default engineer) = true, want profile isolation")
	}
}

func TestDaemonLoopInputEntityCatalogShouldRejectSessionsOutsideTheActingProfile(t *testing.T) {
	t.Parallel()

	const (
		workspaceID      = "ws-profile-loop-input"
		engineeringID    = "profile-engineering"
		otherProfileID   = "profile-other"
		engineeringSess  = "session-engineering"
		otherProfileSess = "session-other-profile"
	)
	sessions := apitest.StubSessionManager{StatusFn: func(_ context.Context, id string) (*session.Info, error) {
		switch id {
		case engineeringSess:
			return &session.Info{ID: id, ProfileID: engineeringID, WorkspaceID: workspaceID}, nil
		case otherProfileSess:
			return &session.Info{ID: id, ProfileID: otherProfileID, WorkspaceID: workspaceID}, nil
		default:
			return nil, session.ErrSessionNotFound
		}
	}}
	catalog := daemonLoopInputEntityCatalog{state: &bootState{sessions: sessions}}

	found, err := catalog.HasInputEntity(
		t.Context(), workspaceID, engineeringID, dsl.EntityKindSession, engineeringSess,
	)
	if err != nil {
		t.Fatalf("HasInputEntity(engineering session) error = %v", err)
	}
	if !found {
		t.Fatal("HasInputEntity(engineering session) = false, want true")
	}
	found, err = catalog.HasInputEntity(
		t.Context(), workspaceID, engineeringID, dsl.EntityKindSession, otherProfileSess,
	)
	if err != nil {
		t.Fatalf("HasInputEntity(peer Profile session) error = %v", err)
	}
	if found {
		t.Fatal("HasInputEntity(peer Profile session) = true, want Profile isolation")
	}
}
