//go:build integration

package extensionpkg

import (
	"slices"
	"testing"
)

func TestRegistryIntegrationLifecycle(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	dir, manifest, checksum := createRegistryTestExtension(t, "lifecycle-ext", registryManifestOptions{
		capabilities: []string{"memory.backend", "prompt.provider"},
		actions:      []string{"observe/health", "sessions/list"},
		extraFiles: map[string]string{
			"hooks/post_prompt.js": "console.log('hook');\n",
		},
	})

	if err := env.registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	listed, err := env.registry.List()
	if err != nil {
		t.Fatalf("List(after install) error = %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("len(List()) = %d, want 1", len(listed))
	}

	if err := env.registry.Enable(manifest.Name); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}

	enabled, err := env.registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("Get(enabled) error = %v", err)
	}
	if !enabled.Enabled {
		t.Fatal("Enabled after Enable() = false, want true")
	}

	if err := env.registry.Disable(manifest.Name); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	disabled, err := env.registry.Get(manifest.Name)
	if err != nil {
		t.Fatalf("Get(disabled) error = %v", err)
	}
	if disabled.Enabled {
		t.Fatal("Enabled after Disable() = true, want false")
	}

	if err := env.registry.Uninstall(manifest.Name); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	remaining, err := env.registry.List()
	if err != nil {
		t.Fatalf("List(after uninstall) error = %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("len(List(after uninstall)) = %d, want 0", len(remaining))
	}
}

func TestRegistryIntegrationMultipleSourcesCoexist(t *testing.T) {
	withDaemonVersion(t, "0.6.0")

	env := newRegistryTestEnv(t)
	userDir, userManifest, userChecksum := createRegistryTestExtension(t, "user-ext", registryManifestOptions{
		capabilities: []string{"memory.backend"},
		actions:      []string{"sessions/list"},
	})
	workspaceDir, workspaceManifest, workspaceChecksum := createRegistryTestExtension(
		t,
		"workspace-ext",
		registryManifestOptions{
			capabilities: []string{"prompt.provider"},
			actions:      []string{"observe/health"},
		},
	)

	if err := env.registry.Install(userManifest, userDir, userChecksum); err != nil {
		t.Fatalf("Install(user) error = %v", err)
	}
	if err := env.registry.Install(
		workspaceManifest,
		workspaceDir,
		workspaceChecksum,
		WithInstallSource(SourceWorkspace),
	); err != nil {
		t.Fatalf("Install(workspace) error = %v", err)
	}

	listed, err := env.registry.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("len(List()) = %d, want 2", len(listed))
	}

	names := []string{listed[0].Name, listed[1].Name}
	if !slices.Equal(names, []string{"user-ext", "workspace-ext"}) {
		t.Fatalf("names = %v, want %v", names, []string{"user-ext", "workspace-ext"})
	}
	if listed[0].Source != SourceUser {
		t.Fatalf("user-ext source = %v, want %v", listed[0].Source, SourceUser)
	}
	if listed[1].Source != SourceWorkspace {
		t.Fatalf("workspace-ext source = %v, want %v", listed[1].Source, SourceWorkspace)
	}
}
