//go:build mage

package main

import (
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/codegen/openapits"
)

func TestValidateWebOpenAPIArtifacts(t *testing.T) {
	t.Parallel()

	t.Run("Should accept complete unique artifact pairs", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(t, root, "openapi/spec.json", "{}\n")
		writeTestFile(t, root, "web/generated/types.d.ts", "export {};\n")
		artifacts := []openapits.Artifact{{
			SpecPath:   filepath.Join(root, "openapi/spec.json"),
			OutputPath: filepath.Join(root, "web/generated/types.d.ts"),
		}}

		got, err := validateWebOpenAPIArtifacts(artifacts, true)
		if err != nil {
			t.Fatalf("validateWebOpenAPIArtifacts() error = %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("validateWebOpenAPIArtifacts() count = %d, want 1", len(got))
		}
	})

	t.Run("Should allow generation to create a missing output", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(t, root, "openapi/spec.json", "{}\n")
		artifacts := []openapits.Artifact{{
			SpecPath:   filepath.Join(root, "openapi/spec.json"),
			OutputPath: filepath.Join(root, "web/generated/types.d.ts"),
		}}

		if _, err := validateWebOpenAPIArtifacts(artifacts, false); err != nil {
			t.Fatalf("validateWebOpenAPIArtifacts() error = %v", err)
		}
	})

	t.Run("Should reject a missing checked output", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeTestFile(t, root, "openapi/spec.json", "{}\n")
		artifacts := []openapits.Artifact{{
			SpecPath:   filepath.Join(root, "openapi/spec.json"),
			OutputPath: filepath.Join(root, "web/generated/types.d.ts"),
		}}

		if _, err := validateWebOpenAPIArtifacts(artifacts, true); err == nil {
			t.Fatal("validateWebOpenAPIArtifacts() error = nil, want missing output failure")
		}
	})

	t.Run("Should reject duplicate registered paths", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		specPath := filepath.Join(root, "openapi/spec.json")
		writeTestFile(t, root, "openapi/spec.json", "{}\n")
		writeTestFile(t, root, "web/generated/one.d.ts", "export {};\n")
		writeTestFile(t, root, "web/generated/two.d.ts", "export {};\n")
		artifacts := []openapits.Artifact{
			{SpecPath: specPath, OutputPath: filepath.Join(root, "web/generated/one.d.ts")},
			{SpecPath: specPath, OutputPath: filepath.Join(root, "web/generated/two.d.ts")},
		}

		if _, err := validateWebOpenAPIArtifacts(artifacts, true); err == nil {
			t.Fatal("validateWebOpenAPIArtifacts() error = nil, want duplicate path failure")
		}
	})

	t.Run("Should reject a path registered across artifact roles", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		firstSpec := filepath.Join(root, "openapi/one.json")
		sharedPath := filepath.Join(root, "openapi/two.json")
		secondOutput := filepath.Join(root, "web/generated/two.d.ts")
		writeTestFile(t, root, "openapi/one.json", "{}\n")
		writeTestFile(t, root, "openapi/two.json", "{}\n")
		writeTestFile(t, root, "web/generated/two.d.ts", "export {};\n")
		artifacts := []openapits.Artifact{
			{SpecPath: firstSpec, OutputPath: sharedPath},
			{SpecPath: sharedPath, OutputPath: secondOutput},
		}

		if _, err := validateWebOpenAPIArtifacts(artifacts, true); err == nil {
			t.Fatal("validateWebOpenAPIArtifacts() error = nil, want cross-role duplicate failure")
		}
	})
}
