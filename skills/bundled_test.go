package skills

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"

	internal "github.com/compozy/agh/internal/skills"
)

var expectedAghReferences = []string{
	"references/agent-definitions.md",
	"references/capabilities-and-bundles.md",
	"references/contributing-to-agh.md",
	"references/docs-design-and-copy.md",
	"references/memory.md",
	"references/native-tools.md",
	"references/network.md",
	"references/qa-and-verification.md",
	"references/runtime-operations.md",
	"references/tasks-and-orchestration.md",
	"references/tools-and-skills.md",
}

func TestBundledFSContainsOnlyAghSkill(t *testing.T) {
	t.Parallel()

	t.Run("Should embed only the agh directory", func(t *testing.T) {
		t.Parallel()

		entries, err := fs.ReadDir(FS(), ".")
		if err != nil {
			t.Fatalf("ReadDir bundled root error = %v", err)
		}
		var dirs []string
		for _, entry := range entries {
			if entry.IsDir() {
				dirs = append(dirs, entry.Name())
			}
		}
		if !slices.Equal(dirs, []string{"agh"}) {
			t.Fatalf("bundled skill dirs = %#v, want only agh", dirs)
		}
	})

	t.Run("Should load exactly one bundled skill named agh with non-empty description", func(t *testing.T) {
		t.Parallel()

		registry := internal.NewRegistry(internal.RegistryConfig{BundledFS: FS()})
		if err := registry.LoadAll(context.Background()); err != nil {
			t.Fatalf("LoadAll error = %v", err)
		}
		loaded := registry.List()
		if len(loaded) != 1 {
			t.Fatalf("bundled skills count = %d, want 1: %#v", len(loaded), loaded)
		}
		if loaded[0].Meta.Name != "agh" {
			t.Fatalf("bundled skill name = %q, want agh", loaded[0].Meta.Name)
		}
		if strings.TrimSpace(loaded[0].Meta.Description) == "" {
			t.Fatal("bundled skill description is empty")
		}
	})
}

func TestLoadContentReturnsSkillBody(t *testing.T) {
	t.Parallel()

	t.Run("Should strip frontmatter", func(t *testing.T) {
		t.Parallel()

		content, err := LoadContent("agh")
		if err != nil {
			t.Fatalf("LoadContent error = %v", err)
		}
		if strings.Contains(content, "name: agh") {
			t.Fatalf("LoadContent returned frontmatter:\n%s", content)
		}
	})
}

func TestBundledReferencesAreEmbeddedAndReadable(t *testing.T) {
	t.Parallel()

	for _, reference := range expectedAghReferences {
		t.Run("Should read "+reference, func(t *testing.T) {
			t.Parallel()

			content, err := LoadResource("agh", reference)
			if err != nil {
				t.Fatalf("LoadResource(%q) error = %v", reference, err)
			}
			if strings.TrimSpace(content) == "" {
				t.Fatalf("LoadResource(%q) returned empty content", reference)
			}
		})
	}
}

func TestLoadResourceRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		skillName    string
		resourcePath string
		wantErr      error
	}{
		{
			name:         "Should reject empty skill",
			skillName:    "",
			resourcePath: "references/network.md",
			wantErr:      ErrSkillNameRequired,
		},
		{
			name:         "Should reject nested skill",
			skillName:    "agh/network",
			resourcePath: "references/network.md",
			wantErr:      ErrInvalidSkillName,
		},
		{name: "Should reject empty resource", skillName: "agh", resourcePath: "", wantErr: ErrResourcePathRequired},
		{
			name:         "Should reject parent traversal",
			skillName:    "agh",
			resourcePath: "../SKILL.md",
			wantErr:      ErrInvalidResourcePath,
		},
		{
			name:         "Should reject absolute path",
			skillName:    "agh",
			resourcePath: "/references/network.md",
			wantErr:      ErrInvalidResourcePath,
		},
		{
			name:         "Should reject backslash path",
			skillName:    "agh",
			resourcePath: `references\network.md`,
			wantErr:      ErrInvalidResourcePath,
		},
		{
			name:         "Should reject dot-prefixed resource alias",
			skillName:    "agh",
			resourcePath: "./references/network.md",
			wantErr:      ErrInvalidResourcePath,
		},
		{
			name:         "Should reject duplicate separator resource alias",
			skillName:    "agh",
			resourcePath: "references//network.md",
			wantErr:      ErrInvalidResourcePath,
		},
		{
			name:         "Should reject internal parent traversal resource alias",
			skillName:    "agh",
			resourcePath: "references/../SKILL.md",
			wantErr:      ErrInvalidResourcePath,
		},
		{
			name:         "Should reject surrounding whitespace resource alias",
			skillName:    "agh",
			resourcePath: " references/network.md ",
			wantErr:      ErrInvalidResourcePath,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := LoadResource(tc.skillName, tc.resourcePath)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("LoadResource error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
