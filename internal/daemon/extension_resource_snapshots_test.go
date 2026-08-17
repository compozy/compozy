package daemon

import (
	"context"
	"testing"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/soul"
	"github.com/compozy/compozy/internal/windowmanager"
)

const resourceSnapshotGeneration = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type resourceSnapshotRuntime struct {
	*fakeExtensionRuntime
	instances map[extensionpkg.InstanceKey]*extensionpkg.Extension
	fallback  *extensionpkg.Extension
}

func (r *resourceSnapshotRuntime) GetForInstance(key extensionpkg.InstanceKey) (*extensionpkg.Extension, error) {
	if ext := r.instances[key.Normalize()]; ext != nil {
		return ext, nil
	}
	if r.fallback != nil {
		return r.fallback, nil
	}
	return nil, extensionpkg.ErrExtensionNotFound
}

func TestExtensionResourceSnapshotsIncludesWorkspaceDevelopmentLinks(t *testing.T) {
	t.Parallel()
	t.Run("Should include development resources only in their workspace scope", testExtensionResourceSnapshots)
	t.Run("Should normalize workspace IDs before resolving development resources", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		registry := extensionpkg.NewRegistry(db.DB())
		const (
			extensionName = "normalized-resource-kit"
			workspaceID   = "workspace-normalized"
		)
		if _, err := registry.LinkDev(extensionpkg.DevLinkRequest{
			Name: extensionName, WorkspaceID: workspaceID, OriginPath: t.TempDir(),
			GenerationHash: resourceSnapshotGeneration, Format: extensionpkg.FormatCompozy,
		}); err != nil {
			t.Fatalf("registry.LinkDev() error = %v", err)
		}
		if _, err := db.DB().ExecContext(
			t.Context(),
			`UPDATE extension_dev_links SET workspace_id = ? WHERE extension_name = ?`,
			"  "+workspaceID+"  ",
			extensionName,
		); err != nil {
			t.Fatalf("pad persisted workspace ID: %v", err)
		}

		key := extensionpkg.InstanceKey{Name: extensionName, WorkspaceID: workspaceID}
		ext := &extensionpkg.Extension{
			Info: extensionpkg.ExtensionInfo{Name: extensionName, Enabled: true},
			Status: extensionpkg.ExtensionStatus{
				Name: extensionName, Enabled: true, Registered: true, WorkspaceID: workspaceID,
			},
			Manifest: &extensionpkg.Manifest{Name: extensionName, Format: extensionpkg.FormatCompozy},
		}
		runtime := &resourceSnapshotRuntime{
			fakeExtensionRuntime: &fakeExtensionRuntime{},
			instances:            map[extensionpkg.InstanceKey]*extensionpkg.Extension{key: ext},
		}

		snapshots, err := extensionResourceSnapshots(registry, runtime, discardLogger())
		if err != nil {
			t.Fatalf("extensionResourceSnapshots() error = %v", err)
		}
		if len(snapshots) != 1 || snapshots[0].scope.ID != workspaceID {
			t.Fatalf("extensionResourceSnapshots() = %#v, want normalized workspace snapshot", snapshots)
		}
	})
}

func testExtensionResourceSnapshots(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	registry := extensionpkg.NewRegistry(db.DB())
	const (
		extensionName = "resource-only-kit"
		workspaceID   = "workspace-resource-only"
	)
	if _, err := registry.LinkDev(extensionpkg.DevLinkRequest{
		Name:           extensionName,
		WorkspaceID:    workspaceID,
		OriginPath:     t.TempDir(),
		GenerationHash: resourceSnapshotGeneration,
		Format:         extensionpkg.FormatCompozy,
	}); err != nil {
		t.Fatalf("registry.LinkDev() error = %v", err)
	}

	key := extensionpkg.InstanceKey{Name: extensionName, WorkspaceID: workspaceID}
	ext := &extensionpkg.Extension{
		Info: extensionpkg.ExtensionInfo{Name: extensionName, Enabled: true},
		Status: extensionpkg.ExtensionStatus{
			Name: extensionName, Enabled: true, Registered: true, WorkspaceID: workspaceID,
		},
		Manifest: &extensionpkg.Manifest{Name: extensionName, Format: extensionpkg.FormatCompozy},
	}
	runtime := &resourceSnapshotRuntime{
		fakeExtensionRuntime: &fakeExtensionRuntime{},
		instances:            map[extensionpkg.InstanceKey]*extensionpkg.Extension{key: ext},
	}

	snapshots, err := extensionResourceSnapshots(registry, runtime, discardLogger())
	if err != nil {
		t.Fatalf("extensionResourceSnapshots() error = %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("len(extensionResourceSnapshots()) = %d, want 1", len(snapshots))
	}
	if snapshots[0].extension != ext || snapshots[0].scope != (resources.ResourceScope{
		Kind: resources.ResourceScopeKindWorkspace,
		ID:   workspaceID,
	}) {
		t.Fatalf("extensionResourceSnapshots()[0] = %#v, want workspace development snapshot", snapshots[0])
	}
}

func TestExtensionResourceSnapshotsIgnoreInactiveDevelopmentLinks(t *testing.T) {
	t.Parallel()
	t.Run("Should skip staged links that have no matching workspace runtime", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		registry := extensionpkg.NewRegistry(db.DB())
		const (
			extensionName = "staged-kit"
			workspaceID   = "workspace-staged"
		)
		if _, err := registry.LinkDev(extensionpkg.DevLinkRequest{
			Name: extensionName, WorkspaceID: workspaceID, OriginPath: t.TempDir(),
			GenerationHash: resourceSnapshotGeneration, Format: extensionpkg.FormatCompozy,
		}); err != nil {
			t.Fatalf("registry.LinkDev() error = %v", err)
		}
		global := &extensionpkg.Extension{
			Info: extensionpkg.ExtensionInfo{Name: extensionName, Enabled: true},
			Status: extensionpkg.ExtensionStatus{
				Name: extensionName, Enabled: true, Registered: true,
			},
			Manifest: &extensionpkg.Manifest{Name: extensionName, Format: extensionpkg.FormatCompozy},
		}

		testCases := []struct {
			name    string
			runtime extensionRuntime
		}{
			{
				name:    "runtime unavailable",
				runtime: nil,
			},
			{
				name: "missing workspace instance",
				runtime: &resourceSnapshotRuntime{
					fakeExtensionRuntime: &fakeExtensionRuntime{},
					instances:            map[extensionpkg.InstanceKey]*extensionpkg.Extension{},
				},
			},
			{
				name: "global runtime fallback",
				runtime: &resourceSnapshotRuntime{
					fakeExtensionRuntime: &fakeExtensionRuntime{},
					instances:            map[extensionpkg.InstanceKey]*extensionpkg.Extension{},
					fallback:             global,
				},
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				snapshots, err := extensionResourceSnapshots(registry, testCase.runtime, discardLogger())
				if err != nil {
					t.Fatalf("extensionResourceSnapshots() error = %v", err)
				}
				if len(snapshots) != 0 {
					t.Fatalf("extensionResourceSnapshots() = %#v, want no inactive development snapshots", snapshots)
				}
			})
		}
	})
}

func TestExtensionResourceSnapshotsPreserveGlobalResourcesWithoutScopedRuntime(t *testing.T) {
	t.Parallel()
	t.Run("Should keep global snapshots when development lookup is unsupported", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		const (
			extensionName = "global-resource-kit"
			workspaceID   = "workspace-global-fallback"
		)
		installDaemonTestExtension(t, db, extensionName, daemonTestExtensionOptions{}, true)
		registry := extensionpkg.NewRegistry(db.DB())
		if _, err := registry.LinkDev(extensionpkg.DevLinkRequest{
			Name: extensionName, WorkspaceID: workspaceID, OriginPath: t.TempDir(),
			GenerationHash: resourceSnapshotGeneration, Format: extensionpkg.FormatCompozy,
		}); err != nil {
			t.Fatalf("registry.LinkDev() error = %v", err)
		}
		global := &extensionpkg.Extension{
			Info: extensionpkg.ExtensionInfo{Name: extensionName, Enabled: true},
			Status: extensionpkg.ExtensionStatus{
				Name: extensionName, Enabled: true, Registered: true,
			},
			Manifest: &extensionpkg.Manifest{Name: extensionName, Format: extensionpkg.FormatCompozy},
		}

		snapshots, err := extensionResourceSnapshots(
			registry,
			&fakeExtensionRuntime{getExt: global},
			discardLogger(),
		)
		if err != nil {
			t.Fatalf("extensionResourceSnapshots() error = %v", err)
		}
		if len(snapshots) != 1 || snapshots[0].extension != global ||
			snapshots[0].scope.Kind != resources.ResourceScopeKindGlobal {
			t.Fatalf("extensionResourceSnapshots() = %#v, want preserved global snapshot", snapshots)
		}
	})
}

func TestExtensionKitResourcesBindWorkspaceScope(t *testing.T) {
	t.Parallel()
	t.Run("Should bind automation and layout identities to the development workspace", func(t *testing.T) {
		t.Parallel()

		jobCodec, err := automationpkg.NewJobResourceCodec()
		if err != nil {
			t.Fatalf("automation.NewJobResourceCodec() error = %v", err)
		}
		triggerCodec, err := automationpkg.NewTriggerResourceCodec()
		if err != nil {
			t.Fatalf("automation.NewTriggerResourceCodec() error = %v", err)
		}
		layoutCodec, err := windowmanager.NewLayoutResourceCodec()
		if err != nil {
			t.Fatalf("windowmanager.NewLayoutResourceCodec() error = %v", err)
		}
		syncer := &extensionKitSourceSyncer{
			jobCodec: jobCodec, triggerCodec: triggerCodec, layoutCodec: layoutCodec,
		}
		const (
			extensionName = "workspace-kit"
			workspaceID   = "workspace-kit-test"
		)
		ext := &extensionpkg.Extension{
			Info: extensionpkg.ExtensionInfo{Name: extensionName},
			AutomationJobs: []automationpkg.Job{{
				ID: "extension/workspace-kit/automation.job/daily", Scope: automationpkg.AutomationScopeGlobal,
				Name: "workspace-kit/daily", AgentName: "writer", Prompt: "Review repository",
				Schedule: &automationpkg.ScheduleSpec{
					Mode: automationpkg.ScheduleModeAt,
					Time: time.Date(2030, 1, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
				},
				Enabled: true, Retry: automationpkg.DefaultRetryConfig(),
				FireLimit: automationpkg.DefaultFireLimitConfig(), Source: automationpkg.JobSourcePackage,
			}},
			AutomationTriggers: []automationpkg.Trigger{{
				ID:    "extension/workspace-kit/automation.trigger/stopped",
				Scope: automationpkg.AutomationScopeGlobal, Name: "workspace-kit/stopped",
				AgentName: "writer", Prompt: "Review stopped session", Event: "session.stopped", Enabled: true,
				Retry: automationpkg.DefaultRetryConfig(), FireLimit: automationpkg.DefaultFireLimitConfig(),
				Source: automationpkg.JobSourcePackage,
			}},
			Layouts: []windowmanager.LayoutResource{{
				Version: windowmanager.LayoutResourceVersion, ID: "focus", DisplayName: "Focus",
				AspectVariant: windowmanager.LayoutAspectAny, OverflowPolicy: windowmanager.LayoutOverflowStack,
				Document: windowmanager.LayoutDocument{
					Version: windowmanager.SnapshotVersion,
					Desktops: []windowmanager.Desktop{{
						ID: "desktop-main", Name: "Main", Purpose: windowmanager.DesktopPurposeStandard,
						Groups: []windowmanager.LayoutGroup{}, Floating: []windowmanager.WindowID{},
					}},
					Windows: map[windowmanager.WindowID]windowmanager.Window{},
				},
			}},
		}
		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: workspaceID}
		jobs := make(map[string]managedResourceValue[automationpkg.Job])
		triggers := make(map[string]managedResourceValue[automationpkg.Trigger])
		layouts := make(map[string]managedResourceValue[windowmanager.LayoutResource])

		if err := syncer.collectDesiredExtensionKitResources(
			context.Background(), ext, scope, jobs, triggers, layouts,
		); err != nil {
			t.Fatalf("collectDesiredExtensionKitResources() error = %v", err)
		}
		for _, job := range jobs {
			if job.spec.Scope != automationpkg.AutomationScopeWorkspace || job.spec.WorkspaceID != workspaceID {
				t.Fatalf(
					"workspace job scope = %q/%q, want workspace/%q",
					job.spec.Scope,
					job.spec.WorkspaceID,
					workspaceID,
				)
			}
		}
		for _, trigger := range triggers {
			if trigger.spec.Scope != automationpkg.AutomationScopeWorkspace || trigger.spec.WorkspaceID != workspaceID {
				t.Fatalf(
					"workspace trigger scope = %q/%q, want workspace/%q",
					trigger.spec.Scope,
					trigger.spec.WorkspaceID,
					workspaceID,
				)
			}
		}
		for id, layout := range layouts {
			if layout.spec.ID != id {
				t.Fatalf("workspace layout spec ID = %q, want record ID %q", layout.spec.ID, id)
			}
		}
		if len(jobs) != 1 || len(triggers) != 1 || len(layouts) != 1 {
			t.Fatalf("workspace kit counts = %d/%d/%d, want 1/1/1", len(jobs), len(triggers), len(layouts))
		}
	})
}

func TestManagedPublicationIDSeparatesExtensionWorkspaceScopes(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve global IDs and separate workspace publications", testManagedPublicationIDScopes)
}

func testManagedPublicationIDScopes(t *testing.T) {
	t.Parallel()

	owner := extensionOwner("resource-only-kit")
	global := managedPublicationID(
		"daemon.sync.test.",
		resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		"extension/resource-only-kit/agent/writer",
		nil,
		owner,
	)
	workspace := managedPublicationID(
		"daemon.sync.test.",
		resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"},
		"extension/resource-only-kit/agent/writer",
		nil,
		owner,
	)
	if global != "extension/resource-only-kit/agent/writer" {
		t.Fatalf("global publication ID = %q, want stable source key", global)
	}
	if workspace == global {
		t.Fatalf("workspace publication ID = global publication ID %q", global)
	}
}

func TestAgentSidecarsResolveAgentsWithinTheSameResourceScope(t *testing.T) {
	t.Parallel()
	t.Run("Should keep global and workspace sidecars attached to their matching agents", func(t *testing.T) {
		t.Parallel()

		agentCodec, err := compozyconfig.NewAgentResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewAgentResourceCodec() error = %v", err)
		}
		soulCodec, err := soul.NewResourceCodec()
		if err != nil {
			t.Fatalf("soul.NewResourceCodec() error = %v", err)
		}
		globalScope := resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal}
		workspaceScope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"}
		const sourceKey = "extension/resource-only-kit/agent/writer"
		declarations := agentSkillDeclarations{
			agents: []agentPublicationInput{
				{
					sourceKey: sourceKey, scope: globalScope,
					spec: compozyconfig.AgentDef{Name: "writer", Prompt: "global agent"},
				},
				{
					sourceKey: sourceKey, scope: workspaceScope,
					spec: compozyconfig.AgentDef{Name: "writer", Prompt: "workspace agent"},
				},
			},
			souls: []soulPublicationInput{
				{
					sourceKey: sourceKey + "/soul", agentSourceKey: sourceKey,
					scope: globalScope, agentName: "writer", sourcePath: "/global/SOUL.md", body: "global soul",
				},
				{
					sourceKey: sourceKey + "/soul", agentSourceKey: sourceKey,
					scope: workspaceScope, agentName: "writer",
					sourcePath: "/workspace/SOUL.md", body: "workspace soul",
				},
			},
		}
		syncer := &agentSkillSourceSyncer{
			agentCodec: agentCodec,
			soulCodec:  soulCodec,
			providers: []agentSkillDeclarationProvider{func(context.Context) (agentSkillDeclarations, error) {
				return declarations, nil
			}},
		}

		desired, err := syncer.desiredResources(context.Background())
		if err != nil {
			t.Fatalf("desiredResources() error = %v", err)
		}
		if len(desired.agents) != 2 || len(desired.souls) != 2 {
			t.Fatalf("desired resources = agents:%d souls:%d, want 2/2", len(desired.agents), len(desired.souls))
		}
		agentIDs := make(map[resources.ResourceScope]string, len(desired.agents))
		for id, agent := range desired.agents {
			agentIDs[agent.scope] = id
		}
		for _, sidecar := range desired.souls {
			if sidecar.spec.AgentResourceID != agentIDs[sidecar.scope] {
				t.Fatalf(
					"soul in scope %#v references agent %q, want %q",
					sidecar.scope,
					sidecar.spec.AgentResourceID,
					agentIDs[sidecar.scope],
				)
			}
		}
	})
}

var _ extensionRuntime = (*resourceSnapshotRuntime)(nil)
var _ scopedExtensionRuntime = (*resourceSnapshotRuntime)(nil)
