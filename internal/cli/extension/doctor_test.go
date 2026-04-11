package extension

import (
	"strings"
	"testing"

	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/version"
)

func TestDoctorWarnsOnPriorityTie(t *testing.T) {
	deps := newTestDeps(t)

	writeManifestJSON(t, userExtensionDir(deps.homeDir, "alpha"), manifestWithPromptHook("alpha", "1.0.0"))
	writeManifestJSON(t, userExtensionDir(deps.homeDir, "beta"), manifestWithPromptHook("beta", "1.0.0"))
	enableUserExtension(t, deps.homeDir, "alpha")
	enableUserExtension(t, deps.homeDir, "beta")

	output, err := executeExtCommand(t, deps, "doctor")
	if err != nil {
		t.Fatalf("execute ext doctor: %v\noutput:\n%s", err, output)
	}
	if !strings.Contains(output, "priority tie on prompt.post_build at 500 across alpha, beta") {
		t.Fatalf("expected priority tie warning\noutput:\n%s", output)
	}
}

func TestDoctorWarnsOnUnusedTasksCreateCapability(t *testing.T) {
	deps := newTestDeps(t)

	manifest := manifestFixture("unused-tasks-create")
	manifest.Security.Capabilities = []extensions.Capability{extensions.CapabilityTasksCreate}
	writeManifestJSON(t, userExtensionDir(deps.homeDir, "unused-tasks-create"), manifest)

	output, err := executeExtCommand(t, deps, "doctor")
	if err != nil {
		t.Fatalf("execute ext doctor: %v\noutput:\n%s", err, output)
	}
	if !strings.Contains(output, `extension "unused-tasks-create" declares capability "tasks.create"`) {
		t.Fatalf("expected unused capability warning\noutput:\n%s", output)
	}
}

func TestDoctorReturnsErrorForUnsupportedMinCompozyVersion(t *testing.T) {
	deps := newTestDeps(t)
	withCompozyVersion(t, "1.0.0")

	manifest := manifestFixture("future-ext")
	manifest.Extension.MinCompozyVersion = "9.0.0"
	writeManifestJSON(t, userExtensionDir(deps.homeDir, "future-ext"), manifest)

	output, err := executeExtCommand(t, deps, "doctor")
	if err == nil {
		t.Fatalf("expected ext doctor to fail on unsupported min version\noutput:\n%s", output)
	}
	if !strings.Contains(output, "requires Compozy 9.0.0 or newer") {
		t.Fatalf("expected min version error in output\noutput:\n%s", output)
	}
}

func TestCapabilityHasManifestEvidenceMapping(t *testing.T) {
	promptManifest := manifestWithPromptHook("prompt-ext", "1.0.0")
	providerManifest := manifestFixture("provider-ext")
	providerManifest.Security.Capabilities = []extensions.Capability{extensions.CapabilityProvidersRegister}
	providerManifest.Providers.IDE = []extensions.ProviderEntry{{Name: "demo", Command: "bin/demo"}}

	skillsManifest := manifestFixture("skills-ext")
	skillsManifest.Security.Capabilities = []extensions.Capability{extensions.CapabilitySkillsShip}
	skillsManifest.Resources.Skills = []string{"skills/*"}

	subprocessManifest := manifestFixture("subprocess-ext")
	subprocessManifest.Subprocess = &extensions.SubprocessConfig{Command: "bin/subprocess-ext"}

	artifactManifest := manifestFixture("artifact-ext")
	artifactManifest.Subprocess = &extensions.SubprocessConfig{Command: "bin/artifact-ext"}
	artifactManifest.Hooks = []extensions.HookDeclaration{{Event: extensions.HookArtifactPreWrite}}

	testCases := []struct {
		name       string
		manifest   *extensions.Manifest
		capability extensions.Capability
		want       bool
	}{
		{
			name:       "prompt mutate uses hook evidence",
			manifest:   promptManifest,
			capability: extensions.CapabilityPromptMutate,
			want:       true,
		},
		{
			name:       "providers register uses provider evidence",
			manifest:   providerManifest,
			capability: extensions.CapabilityProvidersRegister,
			want:       true,
		},
		{
			name:       "skills ship uses resource evidence",
			manifest:   skillsManifest,
			capability: extensions.CapabilitySkillsShip,
			want:       true,
		},
		{
			name:       "tasks create without subprocess warns",
			manifest:   manifestFixture("tasks-ext"),
			capability: extensions.CapabilityTasksCreate,
			want:       false,
		},
		{
			name:       "tasks create with subprocess is considered possible",
			manifest:   subprocessManifest,
			capability: extensions.CapabilityTasksCreate,
			want:       true,
		},
		{
			name:       "artifacts write uses artifact hook evidence",
			manifest:   artifactManifest,
			capability: extensions.CapabilityArtifactsWrite,
			want:       true,
		},
	}

	for _, tc := range testCases {
		if got := capabilityHasManifestEvidence(tc.manifest, tc.capability); got != tc.want {
			t.Fatalf("%s: capabilityHasManifestEvidence() = %t, want %t", tc.name, got, tc.want)
		}
	}
}

func TestCapabilityHasManifestEvidenceNilAndEventsReadCases(t *testing.T) {
	if capabilityHasManifestEvidence(nil, extensions.CapabilityEventsRead) {
		t.Fatal("expected nil manifest to have no evidence")
	}

	manifest := manifestFixture("events-ext")
	if capabilityHasManifestEvidence(manifest, extensions.CapabilityEventsRead) {
		t.Fatal("expected events.read without subprocess to warn")
	}

	manifest.Subprocess = &extensions.SubprocessConfig{Command: "bin/events-ext"}
	if !capabilityHasManifestEvidence(manifest, extensions.CapabilityEventsRead) {
		t.Fatal("expected events.read with subprocess to be considered possible")
	}
}

func withCompozyVersion(t *testing.T, value string) {
	t.Helper()

	previous := version.Version
	version.Version = value
	t.Cleanup(func() {
		version.Version = previous
	})
}
