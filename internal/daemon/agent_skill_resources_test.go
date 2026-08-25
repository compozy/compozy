package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestAppendProfiledSkillResources(t *testing.T) {
	t.Parallel()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), ".compozy"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	profileRoot := filepath.Join(homePaths.ProfilesDir, "marketing")
	writeDaemonSkill(t, filepath.Join(profileRoot, ".agents", "skills"), "profile-agent", "Profile agents source")
	writeDaemonSkill(t, filepath.Join(profileRoot, "skills"), "profile-compozy", "Profile compozy source")

	workspaceRoot := t.TempDir()
	writeDaemonSkill(
		t,
		filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ProfilesDirName, "marketing", ".agents", "skills"),
		"workspace-profile-agent",
		"Workspace profile agents source",
	)
	writeDaemonSkill(
		t,
		filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ProfilesDirName, "marketing", "skills"),
		"workspace-profile-compozy",
		"Workspace profile compozy source",
	)

	profile := profilepkg.WithCounts{Profile: profilepkg.Profile{
		ID: "profile-marketing", Name: "marketing", State: profilepkg.StateActive,
	}}
	resolved := workspacepkg.ResolvedWorkspace{
		Workspace:   workspacepkg.Workspace{ID: "workspace-marketing", RootDir: workspaceRoot},
		WorkspaceID: "workspace-marketing", ProfileID: profile.ID, ProfileName: profile.Name,
		ProfileRoot: profileRoot, Config: compozyconfig.DefaultWithHome(homePaths),
	}
	registry := skillspkg.NewRegistry(skillspkg.RegistryConfig{})
	desired := agentSkillDeclarations{}
	if err := appendProfiledSkillResources(
		t.Context(),
		&desired,
		homePaths,
		skillProfileCatalogStub{profiles: []profilepkg.WithCounts{profile}},
		skillProfileWorkspaceResolver{resolved: resolved},
		[]workspacepkg.ResolvedWorkspace{{
			Workspace:   workspacepkg.Workspace{ID: resolved.ID, RootDir: workspaceRoot},
			WorkspaceID: resolved.WorkspaceID,
		}},
		registry,
	); err != nil {
		t.Fatalf("appendProfiledSkillResources() error = %v", err)
	}

	scopesByName := make(map[string]resources.ResourceScope)
	originsByName := make(map[string]string)
	for _, declaration := range desired.skills {
		scopesByName[declaration.spec.Name] = declaration.scope
		originsByName[declaration.spec.Name] = declaration.spec.Origin
	}
	for _, name := range []string{"profile-agent", "profile-compozy"} {
		if scope := scopesByName[name]; scope.Kind != resources.ResourceScopeKindProfile || scope.ID != profile.ID {
			t.Fatalf("%s scope = %#v, want profile owner", name, scope)
		}
	}
	for _, name := range []string{"workspace-profile-agent", "workspace-profile-compozy"} {
		if scope := scopesByName[name]; scope.Kind != resources.ResourceScopeKindWorkspaceProfile ||
			scope.ID != "workspace-marketing@pf:marketing" {
			t.Fatalf("%s scope = %#v, want workspace-profile owner", name, scope)
		}
	}
	if originsByName["profile-agent"] != "agents" || originsByName["workspace-profile-agent"] != "agents" {
		t.Fatalf("agents origins = %#v, want agents", originsByName)
	}
	if originsByName["profile-compozy"] != "" || originsByName["workspace-profile-compozy"] != "" {
		t.Fatalf("compozy origins = %#v, want empty", originsByName)
	}
}

type skillProfileCatalogStub struct {
	profiles []profilepkg.WithCounts
}

func (s skillProfileCatalogStub) List(context.Context) ([]profilepkg.WithCounts, error) {
	return append([]profilepkg.WithCounts(nil), s.profiles...), nil
}

type skillProfileWorkspaceResolver struct {
	resolved workspacepkg.ResolvedWorkspace
}

func (s skillProfileWorkspaceResolver) Resolve(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
	return s.resolved, nil
}

func (s skillProfileWorkspaceResolver) ResolveOrRegister(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
	return s.resolved, nil
}

func (s skillProfileWorkspaceResolver) ResolveForProfile(
	context.Context,
	string,
	string,
) (workspacepkg.ResolvedWorkspace, error) {
	return s.resolved, nil
}

func TestResourceAgentCatalogListsGetsAndResolvesByScope(t *testing.T) {
	t.Parallel()
	t.Run("Should list, get, and resolve agents by scope", func(t *testing.T) {
		t.Parallel()

		catalog := newResourceCatalog(cloneAgentDef)
		catalog.Replace(5, []resources.Record[compozyconfig.AgentDef]{
			{
				ID:    "global:alpha",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				Spec:  compozyconfig.AgentDef{Name: "alpha", Prompt: "global alpha"},
			},
			{
				ID:    "global:onboarding",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				Spec:  compozyconfig.AgentDef{Name: "onboarding", Prompt: "global onboarding"},
			},
			{
				ID:    "global:coder",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				Spec:  compozyconfig.AgentDef{Name: "coder", Prompt: "global coder"},
			},
			{
				ID:    "workspace:coder",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"},
				Spec: compozyconfig.AgentDef{
					Name: "coder", Prompt: "workspace coder", Tools: []string{"compozy__lookup"},
				},
			},
			{
				ID:    "workspace:onboarding",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"},
				Spec:  compozyconfig.AgentDef{Name: "onboarding", Prompt: "workspace onboarding"},
			},
			{
				ID:    "profile:coder",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: store.DefaultProfileID},
				Spec:  compozyconfig.AgentDef{Name: "coder", Prompt: "default-profile coder"},
			},
			{
				ID:    "profile:reviewer",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: store.DefaultProfileID},
				Spec:  compozyconfig.AgentDef{Name: "reviewer", Prompt: "default-profile reviewer"},
			},
			{
				ID: "workspace-profile:coder",
				Scope: resources.ResourceScope{
					Kind: resources.ResourceScopeKindWorkspaceProfile, ID: "ws-1@pf:default",
				},
				Spec: compozyconfig.AgentDef{
					Name: "coder", Prompt: "workspace-profile coder", Tools: []string{"compozy__lookup"},
				},
			},
		})

		dependency := agentCatalogDependency(catalog)
		listed, err := dependency.ListAgents(context.Background())
		if err != nil {
			t.Fatalf("ListAgents() error = %v", err)
		}
		if got, want := len(listed), 4; got != want {
			t.Fatalf("len(ListAgents()) = %d, want %d", got, want)
		}
		if listed[0].Def.Name != "alpha" || listed[1].Def.Name != "coder" ||
			listed[2].Def.Name != "onboarding" || listed[3].Def.Name != "reviewer" {
			t.Fatalf("ListAgents() = %#v, want default-profile agents sorted by name", listed)
		}
		if listed[0].Origin != contract.AgentOriginGlobal || listed[0].WorkspaceID != "" {
			t.Fatalf("ListAgents()[0] origin = %#v", listed[0])
		}

		got, err := dependency.GetAgent(context.Background(), "alpha")
		if err != nil {
			t.Fatalf("GetAgent(alpha) error = %v", err)
		}
		if got.Def.Prompt != "global alpha" {
			t.Fatalf("GetAgent(alpha).Def.Prompt = %q, want global alpha", got.Def.Prompt)
		}
		if _, err := dependency.GetAgent(context.Background(), "missing"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("GetAgent(missing) error = %v, want os.ErrNotExist", err)
		}
		onboardingEntry, err := dependency.GetAgent(context.Background(), "onboarding")
		if err != nil || onboardingEntry.Def.Prompt != "global onboarding" {
			t.Fatalf("GetAgent(onboarding) = %#v, %v", onboardingEntry, err)
		}
		reviewerEntry, err := dependency.GetAgent(context.Background(), "reviewer")
		if err != nil || reviewerEntry.Def.Prompt != "default-profile reviewer" {
			t.Fatalf("GetAgent(reviewer) = %#v, %v", reviewerEntry, err)
		}

		resolved := &workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: "ws-1"},
			ProfileID: store.DefaultProfileID, ProfileName: "default",
		}
		workspaceEntries, err := dependency.ListAgentsForWorkspace(context.Background(), resolved)
		if err != nil {
			t.Fatalf("ListAgentsForWorkspace() error = %v", err)
		}
		if len(workspaceEntries) != 4 || workspaceEntries[1].Origin != contract.AgentOriginWorkspace ||
			workspaceEntries[1].WorkspaceID != "ws-1" {
			t.Fatalf("workspace entries = %#v", workspaceEntries)
		}
		coder, err := dependency.ResolveAgent("coder", resolved)
		if err != nil {
			t.Fatalf("ResolveAgent(coder) error = %v", err)
		}
		if coder.Prompt != "workspace-profile coder" || len(coder.Tools) != 1 || coder.Tools[0] != "compozy__lookup" {
			t.Fatalf("ResolveAgent(coder) = %#v, want workspace-profile override", coder)
		}
		onboarding, err := dependency.ResolveAgent("onboarding", resolved)
		if err != nil {
			t.Fatalf("ResolveAgent(onboarding) error = %v", err)
		}
		if onboarding.Prompt != "workspace onboarding" {
			t.Fatalf("ResolveAgent(onboarding).Prompt = %q, want workspace onboarding", onboarding.Prompt)
		}
	})
}

func TestAgentSkillSourceSyncerSerializesConvergence(t *testing.T) {
	t.Parallel()
	t.Run("Should serialize concurrent Sync calls into ordered convergence passes", func(t *testing.T) {
		t.Parallel()

		entered := make(chan int, 2)
		release := make(chan struct{})
		wantErr := errors.New("stop after provider")
		var calls atomic.Int32
		rawStore, agentStore, agentCodec, soulStore, soulCodec, heartbeatStore, heartbeatCodec,
			skillStore, skillCodec, mcpStore, mcpCodec := agentSkillSyncStores(t)
		syncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
			raw: rawStore, agentStore: agentStore, agentCodec: agentCodec,
			soulStore: soulStore, soulCodec: soulCodec,
			heartbeatStore: heartbeatStore, heartbeatCodec: heartbeatCodec,
			skillStore: skillStore, skillCodec: skillCodec,
			mcpStore: mcpStore, mcpCodec: mcpCodec,
			actor: agentSkillSyncActor(), logger: discardLogger(),
			providers: []agentSkillDeclarationProvider{func(context.Context) (agentSkillDeclarations, error) {
				call := int(calls.Add(1))
				entered <- call
				<-release
				return agentSkillDeclarations{}, wantErr
			}},
		})
		firstDone := make(chan error, 1)
		go func() {
			firstDone <- syncer.Sync(context.Background())
		}()
		if call := <-entered; call != 1 {
			t.Fatalf("first provider call = %d, want 1", call)
		}
		secondDone := make(chan error, 1)
		go func() {
			secondDone <- syncer.Sync(context.Background())
		}()
		select {
		case call := <-entered:
			t.Fatalf("provider call %d entered before the first convergence pass released", call)
		case <-time.After(25 * time.Millisecond):
		}
		close(release)
		if err := <-firstDone; !errors.Is(err, wantErr) {
			t.Fatalf("first Sync() error = %v, want %v", err, wantErr)
		}
		if call := <-entered; call != 2 {
			t.Fatalf("second provider call = %d, want 2", call)
		}
		if err := <-secondDone; !errors.Is(err, wantErr) {
			t.Fatalf("second Sync() error = %v, want %v", err, wantErr)
		}
	})
}

func TestManagedResourceSyncerOrdersMutations(t *testing.T) {
	t.Run("Should put desired and delete stale resources in lexical ID order", func(t *testing.T) {
		t.Parallel()

		codec, err := compozyconfig.NewAgentResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewAgentResourceCodec() error = %v", err)
		}
		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
		actor := agentSkillSyncActor()
		store := &recordingManagedAgentStore{records: []resources.Record[compozyconfig.AgentDef]{
			{ID: "stale-z", Version: 2, Scope: scope, Source: actor.Source},
			{ID: "stale-a", Version: 3, Scope: scope, Source: actor.Source},
		}}
		desired := make(map[string]managedResourceValue[compozyconfig.AgentDef])
		for _, id := range []string{"desired-z", "desired-a"} {
			spec := compozyconfig.AgentDef{Name: id, Provider: "codex", Prompt: "Test deterministic sync."}
			encoded, encodeErr := codec.Encode(spec)
			if encodeErr != nil {
				t.Fatalf("codec.Encode(%q) error = %v", id, encodeErr)
			}
			desired[id] = managedResourceValue[compozyconfig.AgentDef]{
				id: id, scope: scope, spec: spec, encoded: encoded,
			}
		}

		changed, err := syncManagedResources(
			context.Background(),
			actor,
			store,
			codec,
			desired,
			"test agent",
		)
		if err != nil {
			t.Fatalf("syncManagedResources() error = %v", err)
		}
		if !changed {
			t.Fatal("syncManagedResources() changed = false, want true")
		}
		want := []string{"put:desired-a", "put:desired-z", "delete:stale-a", "delete:stale-z"}
		if len(store.operations) != len(want) {
			t.Fatalf("operations = %#v, want %#v", store.operations, want)
		}
		for idx := range want {
			if store.operations[idx] != want[idx] {
				t.Fatalf("operations = %#v, want %#v", store.operations, want)
			}
		}
	})
}

type recordingManagedAgentStore struct {
	records    []resources.Record[compozyconfig.AgentDef]
	operations []string
}

func (s *recordingManagedAgentStore) Put(
	_ context.Context,
	_ resources.MutationActor,
	draft resources.Draft[compozyconfig.AgentDef],
) (resources.Record[compozyconfig.AgentDef], error) {
	s.operations = append(s.operations, "put:"+draft.ID)
	return resources.Record[compozyconfig.AgentDef]{ID: draft.ID, Spec: draft.Spec}, nil
}

func (s *recordingManagedAgentStore) Delete(
	_ context.Context,
	_ resources.MutationActor,
	id string,
	_ int64,
) error {
	s.operations = append(s.operations, "delete:"+id)
	return nil
}

func (s *recordingManagedAgentStore) Get(
	_ context.Context,
	_ resources.MutationActor,
	_ string,
) (resources.Record[compozyconfig.AgentDef], error) {
	return resources.Record[compozyconfig.AgentDef]{}, errors.New("recording store: get is not supported")
}

func (s *recordingManagedAgentStore) List(
	_ context.Context,
	_ resources.MutationActor,
	_ resources.ResourceFilter,
) ([]resources.Record[compozyconfig.AgentDef], error) {
	return s.records, nil
}

func TestResourceAgentCatalogFallsBackToResolvedWorkspaceSnapshot(t *testing.T) {
	t.Parallel()

	resolved := &workspacepkg.ResolvedWorkspace{
		Agents: []compozyconfig.AgentDef{
			{
				Name:   "fallback",
				Prompt: "resolved snapshot",
			},
			{
				Name:   "onboarding",
				Prompt: "workspace onboarding",
			},
		},
	}
	got, err := (&resourceAgentCatalog{}).ResolveAgent("fallback", resolved)
	if err != nil {
		t.Fatalf("ResolveAgent(fallback) error = %v", err)
	}
	if got.Prompt != "resolved snapshot" {
		t.Fatalf("ResolveAgent(fallback).Prompt = %q, want resolved snapshot", got.Prompt)
	}
	if _, err := (&resourceAgentCatalog{}).ResolveAgent(
		"missing",
		resolved,
	); !errors.Is(
		err,
		workspacepkg.ErrAgentNotAvailable,
	) {
		t.Fatalf("ResolveAgent(missing) error = %v, want ErrAgentNotAvailable", err)
	}
	onboarding, err := (&resourceAgentCatalog{}).ResolveAgent("onboarding", resolved)
	if err != nil || onboarding.Prompt != "workspace onboarding" {
		t.Fatalf("ResolveAgent(onboarding) = %#v, %v", onboarding, err)
	}
}

func TestResourceAgentCatalogResolveAgentFallsBackWhenCatalogMissesWorkspaceAgent(t *testing.T) {
	t.Parallel()

	catalog := newResourceCatalog(cloneAgentDef)
	catalog.Replace(1, []resources.Record[compozyconfig.AgentDef]{{
		ID:      "global:other",
		Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		Source:  resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "bench"},
		Version: 1,
		Spec: compozyconfig.AgentDef{
			Name:   "other",
			Prompt: "global other",
		},
	}})

	resolved := &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{ID: "ws-fallback"},
		Agents: []compozyconfig.AgentDef{{
			Name:   "fallback",
			Prompt: "resolved workspace agent",
		}},
	}

	got, err := agentCatalogDependency(catalog).ResolveAgent("fallback", resolved)
	if err != nil {
		t.Fatalf("ResolveAgent(fallback) error = %v", err)
	}
	if got.Prompt != "resolved workspace agent" {
		t.Fatalf("ResolveAgent(fallback).Prompt = %q, want resolved workspace agent", got.Prompt)
	}
}

func TestResourceAgentCatalogResolveAgentUsesCatalogMatches(t *testing.T) {
	t.Parallel()

	catalog := newResourceCatalog(cloneAgentDef)
	catalog.Replace(3, []resources.Record[compozyconfig.AgentDef]{
		{
			ID:      "global:coder:a",
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			Source:  resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "alpha"},
			Version: 1,
			Spec: compozyconfig.AgentDef{
				Name:   "coder",
				Prompt: "older global coder",
			},
		},
		{
			ID:      "global:coder:z",
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			Source:  resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "omega"},
			Version: 1,
			Spec: compozyconfig.AgentDef{
				Name:   "coder",
				Prompt: "latest global coder",
			},
		},
		{
			ID:      "global:other",
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			Source:  resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "bench"},
			Version: 1,
			Spec: compozyconfig.AgentDef{
				Name:   "other",
				Prompt: "other global agent",
			},
		},
	})

	dependency := agentCatalogDependency(catalog)
	t.Run("Should resolve the latest global catalog match without a workspace", func(t *testing.T) {
		t.Parallel()

		got, err := dependency.ResolveAgent("coder", nil)
		if err != nil {
			t.Fatalf("ResolveAgent(coder) error = %v", err)
		}
		if got.Prompt != "latest global coder" {
			t.Fatalf("ResolveAgent(coder).Prompt = %q, want latest global coder", got.Prompt)
		}
	})

	t.Run("Should return unavailable for a missing agent", func(t *testing.T) {
		t.Parallel()

		if _, err := dependency.ResolveAgent("missing", nil); !errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
			t.Fatalf("ResolveAgent(missing) error = %v, want ErrAgentNotAvailable", err)
		}
	})

	t.Run("Should resolve a builtin agent without a workspace", func(t *testing.T) {
		t.Parallel()

		builtin, err := dependency.ResolveAgent(compozyconfig.BuiltinCoordinatorAgentName, nil)
		if err != nil {
			t.Fatalf("ResolveAgent(coordinator builtin) error = %v", err)
		}
		if builtin.Name != compozyconfig.BuiltinCoordinatorAgentName {
			t.Fatalf("ResolveAgent(coordinator builtin).Name = %q, want coordinator", builtin.Name)
		}
	})

	t.Run("Should expose the current catalog revision", func(t *testing.T) {
		t.Parallel()

		if got, want := dependency.AgentCatalogRevision(), int64(3); got != want {
			t.Fatalf("AgentCatalogRevision() = %d, want %d", got, want)
		}
	})
}

func TestResourceAgentCatalogResolveAgentValidation(t *testing.T) {
	t.Parallel()

	if _, err := (&resourceAgentCatalog{}).ResolveAgent(
		"   ",
		&workspacepkg.ResolvedWorkspace{},
	); err == nil || err.Error() != "session: agent name is required" {
		t.Fatalf("ResolveAgent(blank) error = %v, want agent name is required", err)
	}

	if _, err := resolveAgentFromWorkspaceSnapshot(
		"coder",
		nil,
	); err == nil ||
		err.Error() != "session: resolved workspace is required" {
		t.Fatalf("resolveAgentFromWorkspaceSnapshot(nil) error = %v, want resolved workspace is required", err)
	}
}

func TestResourceAgentCatalogResolvesExtensionOwnedArtifactsAndHeartbeatPolicy(t *testing.T) {
	t.Run("Should resolve extension-owned artifacts and heartbeat policy", func(t *testing.T) {
		t.Parallel()

		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"}
		owner := resources.ResourceOwner{
			Kind: extensionResourceOwnerKind,
			ID:   "marketing-kit",
		}
		agentCatalog := newResourceCatalog(cloneAgentDef)
		agentCatalog.Replace(1, []resources.Record[compozyconfig.AgentDef]{
			{
				ID:    "agt-global",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				Spec:  compozyconfig.AgentDef{Name: "marketer", Prompt: "global marketer"},
			},
			{
				ID:    "agt-marketer",
				Scope: scope,
				Owner: owner,
				Spec:  compozyconfig.AgentDef{Name: "marketer", Prompt: "extension marketer"},
			},
		})
		soulCatalog := newResourceCatalog(cloneSoulResourceSpec)
		soulCatalog.Replace(1, []resources.Record[soul.ResourceSpec]{
			{
				ID:    "sol-marketer",
				Scope: scope,
				Owner: owner,
				Spec: soul.ResourceSpec{
					AgentName:       "marketer",
					AgentResourceID: "agt-marketer",
					SourcePath:      ".compozy/extensions/marketing-kit/agents/marketer/SOUL.md",
					Body:            "Lead with campaign context.",
				},
			},
			{
				ID:    "sol-leak",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				Owner: owner,
				Spec: soul.ResourceSpec{
					AgentName:       "marketer",
					AgentResourceID: "agt-global",
					SourcePath:      ".compozy/extensions/marketing-kit/agents/marketer/SOUL.md",
					Body:            "Do not attach this global sidecar.",
				},
			},
		})
		heartbeatCatalog := newResourceCatalog(cloneHeartbeatResourceSpec)
		heartbeatCatalog.Replace(1, []resources.Record[heartbeat.ResourceSpec]{{
			ID:    "hbt-marketer",
			Scope: scope,
			Owner: owner,
			Spec: heartbeat.ResourceSpec{
				AgentName:       "marketer",
				AgentResourceID: "agt-marketer",
				SourcePath:      ".compozy/extensions/marketing-kit/agents/marketer/HEARTBEAT.md",
				Body:            "Inspect campaign status and use Compozy task APIs.",
			},
		}})

		dependency := agentCatalogDependency(agentCatalog, agentSidecarCatalogs{
			soul:      soulCatalog,
			heartbeat: heartbeatCatalog,
		})
		resolved := &workspacepkg.ResolvedWorkspace{Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: t.TempDir()}}
		artifacts, err := dependency.ResolveAgentArtifacts("marketer", resolved)
		if err != nil {
			t.Fatalf("ResolveAgentArtifacts(marketer) error = %v", err)
		}
		if !artifacts.PackageOwned {
			t.Fatal("artifacts.PackageOwned = false, want true")
		}
		if got, want := artifacts.Agent.Prompt, "extension marketer"; got != want {
			t.Fatalf("artifacts.Agent.Prompt = %q, want %q", got, want)
		}
		if got, want := artifacts.SoulBody, "Lead with campaign context."; got != want {
			t.Fatalf("artifacts.SoulBody = %q, want %q", got, want)
		}
		if got, want := artifacts.HeartbeatBody, "Inspect campaign status and use Compozy task APIs."; got != want {
			t.Fatalf("artifacts.HeartbeatBody = %q, want %q", got, want)
		}

		policy, ok, err := dependency.ResolveHeartbeatPolicy(context.Background(), heartbeat.AuthoringTarget{
			AgentName:     "marketer",
			WorkspaceID:   "ws-1",
			WorkspaceRoot: resolved.RootDir,
		})
		if err != nil {
			t.Fatalf("ResolveHeartbeatPolicy(marketer) error = %v", err)
		}
		if !ok {
			t.Fatal("ResolveHeartbeatPolicy(marketer) ok = false, want true")
		}
		if !policy.Present || !policy.Valid || !policy.Active {
			t.Fatalf(
				"heartbeat policy flags = present:%v valid:%v active:%v, want all true",
				policy.Present,
				policy.Valid,
				policy.Active,
			)
		}
	})
}

func TestResourceAgentCatalogMatchesExtensionSidecarsByOwnerScopeAndAgentID(t *testing.T) {
	t.Run("Should match extension sidecars by owner scope and agent ID", func(t *testing.T) {
		t.Parallel()

		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"}
		owner := resources.ResourceOwner{Kind: extensionResourceOwnerKind, ID: "kit"}
		otherOwner := resources.ResourceOwner{Kind: extensionResourceOwnerKind, ID: "other-kit"}
		agentCatalog := newResourceCatalog(cloneAgentDef)
		agentCatalog.Replace(1, []resources.Record[compozyconfig.AgentDef]{
			{
				ID: "shared-agent-id", Scope: scope, Owner: owner,
				Spec: compozyconfig.AgentDef{Name: "coder", Prompt: "extension coder"},
			},
			{
				ID: "plain-agent", Scope: scope,
				Spec: compozyconfig.AgentDef{Name: "plain", Prompt: "operator coder"},
			},
		})
		soulCatalog := newResourceCatalog(cloneSoulResourceSpec)
		soulCatalog.Replace(1, []resources.Record[soul.ResourceSpec]{
			{
				ID: "matching-soul", Scope: scope, Owner: owner,
				Spec: soul.ResourceSpec{
					AgentName: "coder", AgentResourceID: "shared-agent-id", Body: "matching extension soul",
				},
			},
			{
				ID: "wrong-owner-soul", Scope: scope, Owner: otherOwner,
				Spec: soul.ResourceSpec{
					AgentName: "coder", AgentResourceID: "shared-agent-id", Body: "must not attach",
				},
			},
			{
				ID:    "wrong-scope-soul",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, Owner: owner,
				Spec: soul.ResourceSpec{
					AgentName: "coder", AgentResourceID: "shared-agent-id", Body: "must not attach either",
				},
			},
		})
		heartbeatCatalog := newResourceCatalog(cloneHeartbeatResourceSpec)
		heartbeatCatalog.Replace(1, []resources.Record[heartbeat.ResourceSpec]{
			{
				ID: "wrong-id-heartbeat", Scope: scope, Owner: owner,
				Spec: heartbeat.ResourceSpec{
					AgentName: "coder", AgentResourceID: "another-agent-id", Body: "must not attach",
				},
			},
			{
				ID: "matching-heartbeat", Scope: scope, Owner: owner,
				Spec: heartbeat.ResourceSpec{
					AgentName: "coder", AgentResourceID: "shared-agent-id", Body: "matching heartbeat",
				},
			},
		})

		dependency := agentCatalogDependency(agentCatalog, agentSidecarCatalogs{
			soul: soulCatalog, heartbeat: heartbeatCatalog,
		})
		resolved := &workspacepkg.ResolvedWorkspace{Workspace: workspacepkg.Workspace{ID: "ws-1"}}
		artifacts, err := dependency.ResolveAgentArtifacts("coder", resolved)
		if err != nil {
			t.Fatalf("ResolveAgentArtifacts(coder) error = %v", err)
		}
		if !artifacts.PackageOwned || artifacts.OwnerKind != string(extensionResourceOwnerKind) ||
			artifacts.OwnerID != "kit" {
			t.Fatalf("extension artifacts ownership = %#v, want package-owned extension/kit", artifacts)
		}
		if artifacts.SoulBody != "matching extension soul" || artifacts.HeartbeatBody != "matching heartbeat" {
			t.Fatalf("extension artifacts sidecars = %#v, want exact owner/scope/id matches", artifacts)
		}
		plain, err := dependency.ResolveAgentArtifacts("plain", resolved)
		if err != nil {
			t.Fatalf("ResolveAgentArtifacts(plain) error = %v", err)
		}
		if plain.PackageOwned {
			t.Fatalf("plain artifacts = %#v, want PackageOwned=false", plain)
		}
	})
}

func TestAgentSkillSmallHelpers(t *testing.T) {
	t.Parallel()

	var nilPublisher agentSkillPublisherFunc
	if err := nilPublisher.Sync(context.Background()); err != nil {
		t.Fatalf("nil publisher Sync() error = %v", err)
	}
	called := false
	publisher := agentSkillPublisherFunc(func(context.Context) error {
		called = true
		return nil
	})
	if err := publisher.Sync(context.Background()); err != nil {
		t.Fatalf("publisher Sync() error = %v", err)
	}
	if !called {
		t.Fatal("publisher Sync() did not call function")
	}
	if agentCatalogDependency(nil) != nil {
		t.Fatal("agentCatalogDependency(nil) != nil")
	}
	if newAgentProjector(nil) != nil {
		t.Fatal("newAgentProjector(nil) != nil")
	}
	if newSkillProjector(nil) != nil {
		t.Fatal("newSkillProjector(nil) != nil")
	}

	var nilPlan *skillResourceProjectionPlan
	if nilPlan.Kind() != "" || nilPlan.Revision() != 0 || nilPlan.OperationCount() != 0 {
		t.Fatalf(
			"nil skillResourceProjectionPlan methods = (%q,%d,%d), want zero values",
			nilPlan.Kind(),
			nilPlan.Revision(),
			nilPlan.OperationCount(),
		)
	}
	plan := &skillResourceProjectionPlan{
		revision: 4,
		records: []resources.Record[skillspkg.SkillResourceSpec]{{
			ID: "skill",
		}},
	}
	if plan.Kind() != skillspkg.SkillResourceKind || plan.Revision() != 4 || plan.OperationCount() != 1 {
		t.Fatalf(
			"skillResourceProjectionPlan methods = (%q,%d,%d), want skill,4,1",
			plan.Kind(),
			plan.Revision(),
			plan.OperationCount(),
		)
	}
}

func TestAgentSkillSourceSyncerReplacesCanonicalSnapshot(t *testing.T) {
	t.Parallel()

	rawStore, agentStore, agentCodec, soulStore, soulCodec, heartbeatStore, heartbeatCodec,
		skillStore, skillCodec, mcpStore, mcpCodec := agentSkillSyncStores(t)
	agentCatalog := newResourceCatalog(cloneAgentDef)
	desired := agentSkillDeclarations{
		agents: []agentPublicationInput{{
			sourceKey: "test/agent/coder",
			scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			spec: compozyconfig.AgentDef{
				Name:       "coder",
				Prompt:     "Use canonical tools.",
				Tools:      []string{"compozy__lookup"},
				SourcePath: "/extensions/spec-cycle/agents/coder/AGENT.md",
			},
		}},
		skills: []skillPublicationInput{{
			sourceKey: "test/skill/review",
			scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			spec: skillspkg.SkillResourceSpec{
				Name:        "review",
				Description: "Review skill",
				Source:      skillspkg.SkillSourceName(skillspkg.SourceUser),
				Enabled:     true,
			},
		}},
		mcpServers: []mcpServerPublicationInput{{
			sourceKey: "test/mcp/review",
			scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			spec: compozyconfig.MCPServer{
				Name:    "review-mcp",
				Command: "review-command",
			},
		}},
	}
	triggered := make(map[resources.ResourceKind]int)
	syncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
		raw: rawStore, agentStore: agentStore, agentCodec: agentCodec,
		agentProjector: newAgentProjector(agentCatalog),
		soulStore:      soulStore, soulCodec: soulCodec,
		heartbeatStore: heartbeatStore, heartbeatCodec: heartbeatCodec,
		skillStore: skillStore, skillCodec: skillCodec,
		mcpStore: mcpStore, mcpCodec: mcpCodec,
		actor: agentSkillSyncActor(), logger: discardLogger(),
		trigger: func(_ context.Context, kind resources.ResourceKind, _ resources.ReconcileReason) error {
			triggered[kind]++
			return nil
		},
		providers: []agentSkillDeclarationProvider{func(context.Context) (agentSkillDeclarations, error) {
			return desired, nil
		}},
	})

	if err := syncer.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	t.Run("Should retain authored agent provenance in the projected catalog", func(t *testing.T) {
		// not parallel: the parent mutates this catalog after the assertion.
		projected, err := agentCatalogDependency(agentCatalog).GetAgent(context.Background(), "coder")
		if err != nil {
			t.Fatalf("GetAgent(coder) error = %v", err)
		}
		if projected.Def.SourcePath != "/extensions/spec-cycle/agents/coder/AGENT.md" {
			t.Fatalf("GetAgent(coder).Def.SourcePath = %q, want authored extension path", projected.Def.SourcePath)
		}
	})
	assertAgentSkillStoreCounts(t, agentStore, skillStore, mcpStore, 1, 1, 1)
	if triggered[compozyconfig.AgentResourceKind] != 1 ||
		triggered[skillspkg.SkillResourceKind] != 1 ||
		triggered[compozyconfig.MCPServerResourceKind] != 1 {
		t.Fatalf("triggered = %#v, want one trigger per migrated kind", triggered)
	}

	if err := syncer.Sync(context.Background()); err != nil {
		t.Fatalf("second Sync() error = %v", err)
	}
	if triggered[compozyconfig.AgentResourceKind] != 1 ||
		triggered[skillspkg.SkillResourceKind] != 1 ||
		triggered[compozyconfig.MCPServerResourceKind] != 1 {
		t.Fatalf("triggered after no-op = %#v, want no additional triggers", triggered)
	}

	desired.agents = nil
	desired.mcpServers = nil
	desired.skills[0].spec.Description = "Updated review skill"
	if err := syncer.Sync(context.Background()); err != nil {
		t.Fatalf("third Sync() error = %v", err)
	}
	assertAgentSkillStoreCounts(t, agentStore, skillStore, mcpStore, 0, 1, 0)
	if triggered[compozyconfig.AgentResourceKind] != 2 ||
		triggered[skillspkg.SkillResourceKind] != 2 ||
		triggered[compozyconfig.MCPServerResourceKind] != 2 {
		t.Fatalf("triggered after replacement = %#v, want stale-delete/update triggers", triggered)
	}
}

func TestAgentSkillSourceSyncerRetriesAgentProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should retry projection after the durable resource write stops changing", func(t *testing.T) {
		t.Parallel()

		rawStore, agentStore, agentCodec, soulStore, soulCodec, heartbeatStore, heartbeatCodec,
			skillStore, skillCodec, mcpStore, mcpCodec := agentSkillSyncStores(t)
		projectionErr := errors.New("projection unavailable")
		projector := &retryingAgentProjector{buildErrors: []error{projectionErr, nil}}
		syncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
			raw: rawStore, agentStore: agentStore, agentCodec: agentCodec, agentProjector: projector,
			soulStore: soulStore, soulCodec: soulCodec,
			heartbeatStore: heartbeatStore, heartbeatCodec: heartbeatCodec,
			skillStore: skillStore, skillCodec: skillCodec,
			mcpStore: mcpStore, mcpCodec: mcpCodec,
			actor: agentSkillSyncActor(), logger: discardLogger(),
			providers: []agentSkillDeclarationProvider{func(context.Context) (agentSkillDeclarations, error) {
				return agentSkillDeclarations{agents: []agentPublicationInput{{
					sourceKey: "test/agent/coder",
					scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
					spec:      compozyconfig.AgentDef{Name: "coder", Provider: "codex", Prompt: "Code."},
				}}}, nil
			}},
		})

		if err := syncer.Sync(context.Background()); !errors.Is(err, projectionErr) {
			t.Fatalf("first Sync() error = %v, want projection failure", err)
		}
		if err := syncer.Sync(context.Background()); err != nil {
			t.Fatalf("second Sync() error = %v", err)
		}
		if projector.buildCalls != 2 || projector.applyCalls != 1 {
			t.Fatalf(
				"projection calls = build:%d apply:%d, want build:2 apply:1",
				projector.buildCalls,
				projector.applyCalls,
			)
		}
	})
}

type retryingAgentProjector struct {
	buildErrors []error
	buildCalls  int
	applyCalls  int
}

func (p *retryingAgentProjector) Kind() resources.ResourceKind {
	return compozyconfig.AgentResourceKind
}

func (p *retryingAgentProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *retryingAgentProjector) Build(
	context.Context,
	[]resources.Record[compozyconfig.AgentDef],
) (resources.ProjectionPlan, error) {
	p.buildCalls++
	if p.buildCalls <= len(p.buildErrors) && p.buildErrors[p.buildCalls-1] != nil {
		return nil, p.buildErrors[p.buildCalls-1]
	}
	return retryAgentProjectionPlan{}, nil
}

func (p *retryingAgentProjector) Apply(context.Context, resources.ProjectionPlan) error {
	p.applyCalls++
	return nil
}

type retryAgentProjectionPlan struct{}

func (retryAgentProjectionPlan) Kind() resources.ResourceKind { return compozyconfig.AgentResourceKind }
func (retryAgentProjectionPlan) Revision() int64              { return 0 }
func (retryAgentProjectionPlan) OperationCount() int          { return 0 }

func TestAgentSkillSourceSyncerSyncSkillsProjectsRegistrySynchronously(t *testing.T) {
	t.Parallel()

	t.Run("Should project synchronized marketplace skills into the registry immediately", func(t *testing.T) {
		t.Parallel()

		rawStore, agentStore, agentCodec, soulStore, soulCodec, heartbeatStore, heartbeatCodec,
			skillStore, skillCodec, mcpStore, mcpCodec := agentSkillSyncStores(t)
		registry := skillspkg.NewRegistry(skillspkg.RegistryConfig{}, skillspkg.WithLogger(discardLogger()))
		desired := agentSkillDeclarations{
			skills: []skillPublicationInput{{
				sourceKey: "test/skill/review",
				scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				spec: skillspkg.SkillResourceSpec{
					Name:        "review",
					Description: "Review skill",
					Source:      skillspkg.SkillSourceName(skillspkg.SourceMarketplace),
					Enabled:     true,
				},
			}},
		}
		triggered := 0
		syncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
			raw: rawStore, agentStore: agentStore, agentCodec: agentCodec,
			soulStore: soulStore, soulCodec: soulCodec,
			heartbeatStore: heartbeatStore, heartbeatCodec: heartbeatCodec,
			skillStore: skillStore, skillCodec: skillCodec, skillProjector: newSkillProjector(registry),
			mcpStore: mcpStore, mcpCodec: mcpCodec,
			actor: agentSkillSyncActor(), logger: discardLogger(),
			trigger: func(_ context.Context, kind resources.ResourceKind, _ resources.ReconcileReason) error {
				if kind == skillspkg.SkillResourceKind {
					triggered++
				}
				return nil
			},
			providers: []agentSkillDeclarationProvider{func(context.Context) (agentSkillDeclarations, error) {
				return desired, nil
			}},
		})

		if _, ok := registry.Get("review"); ok {
			t.Fatal("registry.Get(review) ok = true before SyncSkills, want false")
		}
		if err := syncer.SyncSkills(context.Background()); err != nil {
			t.Fatalf("SyncSkills() error = %v", err)
		}
		skill, ok := registry.Get("review")
		if !ok {
			t.Fatal("registry.Get(review) ok = false after SyncSkills, want projected skill")
		}
		if skill.Source != skillspkg.SourceMarketplace {
			t.Fatalf("skill.Source = %v, want marketplace", skill.Source)
		}
		if triggered != 1 {
			t.Fatalf("skill trigger count = %d, want 1", triggered)
		}
	})
}

func TestAgentSkillSourceSyncerRepairsLegacyManagedAgentRecordsBeforeDecode(t *testing.T) {
	t.Parallel()

	rawStore, agentStore, agentCodec, soulStore, soulCodec, heartbeatStore, heartbeatCodec,
		skillStore, skillCodec, mcpStore, mcpCodec := agentSkillSyncStores(t)
	legacyScope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-legacy"}
	if _, err := rawStore.PutRaw(
		context.Background(),
		agentSkillSyncActor(),
		resources.RawDraft{
			Kind:  compozyconfig.AgentResourceKind,
			ID:    "daemon.sync.agent.legacy",
			Scope: legacyScope,
			SpecJSON: []byte(`{
				"Name":"general",
				"Tools":["*"],
				"Prompt":"Legacy managed general agent"
			}`),
		},
	); err != nil {
		t.Fatalf("rawStore.PutRaw(legacy agent) error = %v", err)
	}

	syncer := newAgentSkillSourceSyncer(agentSkillSourceSyncerDeps{
		raw: rawStore, agentStore: agentStore, agentCodec: agentCodec,
		soulStore: soulStore, soulCodec: soulCodec,
		heartbeatStore: heartbeatStore, heartbeatCodec: heartbeatCodec,
		skillStore: skillStore, skillCodec: skillCodec,
		mcpStore: mcpStore, mcpCodec: mcpCodec,
		actor: agentSkillSyncActor(), logger: discardLogger(),
		providers: []agentSkillDeclarationProvider{func(context.Context) (agentSkillDeclarations, error) {
			return agentSkillDeclarations{
				agents: []agentPublicationInput{{
					sourceKey: "daemon/general",
					scope:     legacyScope,
					spec: compozyconfig.AgentDef{
						Name:   "general",
						Prompt: "Canonical managed general agent",
					},
				}},
			}, nil
		}},
	})

	if err := syncer.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	assertAgentSkillStoreCounts(t, agentStore, skillStore, mcpStore, 1, 0, 0)

	source := agentSkillSyncActor().Source
	rawAgents, err := rawStore.ListRaw(context.Background(), agentSkillSyncActor(), resources.ResourceFilter{
		Kind:   compozyconfig.AgentResourceKind,
		Source: &source,
	})
	if err != nil {
		t.Fatalf("rawStore.ListRaw() error = %v", err)
	}
	if got, want := len(rawAgents), 1; got != want {
		t.Fatalf("len(rawStore.ListRaw()) = %d, want %d", got, want)
	}
	if got, want := rawAgents[0].ID, "daemon.sync.agent.legacy"; got == want {
		t.Fatalf("rawAgents[0].ID = %q, want legacy record replaced", got)
	}

	agents, err := agentStore.List(
		context.Background(),
		agentSkillSyncActor(),
		resources.ResourceFilter{Source: &source},
	)
	if err != nil {
		t.Fatalf("agentStore.List() error = %v", err)
	}
	if got, want := len(agents), 1; got != want {
		t.Fatalf("len(agentStore.List()) = %d, want %d", got, want)
	}
	if got := agents[0].Spec.Tools; len(got) != 0 {
		t.Fatalf("agents[0].Spec.Tools = %#v, want canonical empty tool set", got)
	}
}

func TestAppendAgentAndSkillResourcesPublishesMCPAttachments(t *testing.T) {
	t.Parallel()

	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-append"}
	decl := hookspkg.HookDecl{
		Name:    "tool-hook",
		Event:   hookspkg.HookToolPreCall,
		Source:  hookspkg.HookSourceAgentDefinition,
		Mode:    hookspkg.HookModeSync,
		Command: "echo",
		Args:    []string{"ok"},
	}
	desired := agentSkillDeclarations{}
	appendAgentResources(&desired, scope, "append", []compozyconfig.AgentDef{{
		Name:   "coder",
		Prompt: "Prompt",
		MCPServers: []compozyconfig.MCPServer{{
			Name:    "agent-mcp",
			Command: "agent-command",
		}},
		Hooks: []hookspkg.HookDecl{decl},
	}})
	appendSkillResources(&desired, scope, skillPublicationSource{prefix: "append"}, []*skillspkg.Skill{{
		Meta: skillspkg.SkillMeta{
			Name:        "review",
			Description: "Review skill",
		},
		Source:  skillspkg.SourceWorkspace,
		Enabled: true,
		MCPServers: []skillspkg.MCPServerDecl{{
			Name:    "skill-mcp",
			Command: "skill-command",
		}},
		Hooks: []hookspkg.HookDecl{{
			Name:    "skill-hook",
			Event:   hookspkg.HookToolPreCall,
			Source:  hookspkg.HookSourceSkill,
			Mode:    hookspkg.HookModeSync,
			Command: "echo",
		}},
	}})

	if got, want := len(desired.agents), 1; got != want {
		t.Fatalf("len(agents) = %d, want %d", got, want)
	}
	if got, want := len(desired.skills), 1; got != want {
		t.Fatalf("len(skills) = %d, want %d", got, want)
	}
	if got, want := len(desired.mcpServers), 2; got != want {
		t.Fatalf("len(mcpServers) = %d, want %d", got, want)
	}
	if desired.mcpServers[0].spec.Name != "agent-mcp" || desired.mcpServers[1].spec.Name != "skill-mcp" {
		t.Fatalf("mcpServers = %#v, want agent and skill MCP attachments", desired.mcpServers)
	}

	t.Run("Should publish only the winning homonym MCP attachment", func(t *testing.T) {
		t.Parallel()

		winnerDesired := agentSkillDeclarations{}
		appendSkillResources(
			&winnerDesired,
			scope,
			skillPublicationSource{prefix: "append"},
			[]*skillspkg.Skill{
				{
					Meta:    skillspkg.SkillMeta{Name: "review", Description: "First root"},
					RootID:  "first",
					Enabled: true,
					MCPServers: []skillspkg.MCPServerDecl{{
						Name: "first-mcp", Command: "first-command",
					}},
				},
				{
					Meta:    skillspkg.SkillMeta{Name: "review", Description: "Winning root"},
					RootID:  "winner",
					Enabled: true,
					MCPServers: []skillspkg.MCPServerDecl{{
						Name: "winner-mcp", Command: "winner-command",
					}},
				},
			},
		)

		if got, want := len(winnerDesired.skills), 2; got != want {
			t.Fatalf("len(skills) = %d, want %d physical candidates", got, want)
		}
		if got, want := len(winnerDesired.mcpServers), 1; got != want {
			t.Fatalf("len(mcpServers) = %d, want %d winner attachment", got, want)
		}
		if got, want := winnerDesired.mcpServers[0].spec.Name, "winner-mcp"; got != want {
			t.Fatalf("mcpServers[0].spec.Name = %q, want %q", got, want)
		}
	})
}

func TestAgentDefinitionSyncRuntimeWiring(t *testing.T) {
	t.Parallel()

	t.Run("Should inject the boot publisher into both server transports", func(t *testing.T) {
		t.Parallel()

		_, httpDeps, udsDeps := bootModelCatalogTestDaemon(t, nil)
		if httpDeps.AgentDefinitionSync == nil {
			t.Fatal("HTTP RuntimeDeps AgentDefinitionSync = nil, want boot publisher")
		}
		if udsDeps.AgentDefinitionSync == nil {
			t.Fatal("UDS RuntimeDeps AgentDefinitionSync = nil, want boot publisher")
		}
		if httpDeps.AgentDefinitionSync != udsDeps.AgentDefinitionSync {
			t.Fatal("HTTP and UDS received different agent definition sync publishers")
		}
	})
}

func agentSkillSyncStores(
	t *testing.T,
) (
	resources.RawStore,
	resources.Store[compozyconfig.AgentDef],
	resources.KindCodec[compozyconfig.AgentDef],
	resources.Store[soul.ResourceSpec],
	resources.KindCodec[soul.ResourceSpec],
	resources.Store[heartbeat.ResourceSpec],
	resources.KindCodec[heartbeat.ResourceSpec],
	resources.Store[skillspkg.SkillResourceSpec],
	resources.KindCodec[skillspkg.SkillResourceSpec],
	resources.Store[compozyconfig.MCPServer],
	resources.KindCodec[compozyconfig.MCPServer],
) {
	t.Helper()

	db := openDaemonTestGlobalDB(t)
	kernel, err := resources.NewKernel(db.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	agentCodec, err := compozyconfig.NewAgentResourceCodec()
	if err != nil {
		t.Fatalf("compozyconfig.NewAgentResourceCodec() error = %v", err)
	}
	agentStore, err := resources.NewStore(kernel, agentCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(agent) error = %v", err)
	}
	soulCodec, err := soul.NewResourceCodec()
	if err != nil {
		t.Fatalf("soul.NewResourceCodec() error = %v", err)
	}
	soulStore, err := resources.NewStore(kernel, soulCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(soul) error = %v", err)
	}
	heartbeatCodec, err := heartbeat.NewResourceCodec()
	if err != nil {
		t.Fatalf("heartbeat.NewResourceCodec() error = %v", err)
	}
	heartbeatStore, err := resources.NewStore(kernel, heartbeatCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(heartbeat) error = %v", err)
	}
	skillCodec, err := skillspkg.NewResourceCodec()
	if err != nil {
		t.Fatalf("skillspkg.NewResourceCodec() error = %v", err)
	}
	skillStore, err := resources.NewStore(kernel, skillCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(skill) error = %v", err)
	}
	mcpCodec, err := compozyconfig.NewMCPServerResourceCodec()
	if err != nil {
		t.Fatalf("compozyconfig.NewMCPServerResourceCodec() error = %v", err)
	}
	mcpStore, err := resources.NewStore(kernel, mcpCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(mcp) error = %v", err)
	}
	return kernel, agentStore, agentCodec, soulStore, soulCodec, heartbeatStore, heartbeatCodec,
		skillStore, skillCodec, mcpStore, mcpCodec
}

func assertAgentSkillStoreCounts(
	t *testing.T,
	agentStore resources.Store[compozyconfig.AgentDef],
	skillStore resources.Store[skillspkg.SkillResourceSpec],
	mcpStore resources.Store[compozyconfig.MCPServer],
	wantAgents int,
	wantSkills int,
	wantMCP int,
) {
	t.Helper()

	source := agentSkillSyncActor().Source
	agents, err := agentStore.List(
		context.Background(),
		agentSkillSyncActor(),
		resources.ResourceFilter{Source: &source},
	)
	if err != nil {
		t.Fatalf("agentStore.List() error = %v", err)
	}
	skills, err := skillStore.List(
		context.Background(),
		agentSkillSyncActor(),
		resources.ResourceFilter{Source: &source},
	)
	if err != nil {
		t.Fatalf("skillStore.List() error = %v", err)
	}
	servers, err := mcpStore.List(
		context.Background(),
		agentSkillSyncActor(),
		resources.ResourceFilter{Source: &source},
	)
	if err != nil {
		t.Fatalf("mcpStore.List() error = %v", err)
	}
	if len(agents) != wantAgents || len(skills) != wantSkills || len(servers) != wantMCP {
		t.Fatalf(
			"store counts = agents:%d skills:%d mcp:%d, want agents:%d skills:%d mcp:%d",
			len(agents),
			len(skills),
			len(servers),
			wantAgents,
			wantSkills,
			wantMCP,
		)
	}
}
