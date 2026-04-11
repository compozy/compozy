package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPublishExtensionSDKsTargetRequiresVerificationAndPublicAccess(t *testing.T) {
	t.Parallel()

	makefile := readRepoMakefile(t)
	prereqs := mustMakeTargetPrereqs(t, makefile, "publish-extension-sdks")

	prerequisiteTests := []struct {
		name string
		want string
	}{
		{name: "Should depend on verify", want: "verify"},
		{name: "Should depend on build-extension-sdks", want: "build-extension-sdks"},
	}
	for _, tt := range prerequisiteTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, ok := prereqs[tt.want]; !ok {
				t.Fatalf(
					"expected target %q prerequisites to include %q\nPrerequisites: %#v\nMakefile:\n%s",
					"publish-extension-sdks",
					tt.want,
					prereqs,
					makefile,
				)
			}
		})
	}

	publishTests := []struct {
		name string
		want string
	}{
		{
			name: "Should publish @compozy/extension-sdk publicly",
			want: "npm publish --workspace @compozy/extension-sdk --access public",
		},
		{
			name: "Should publish @compozy/create-extension publicly",
			want: "npm publish --workspace @compozy/create-extension --access public",
		},
	}
	for _, tt := range publishTests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !strings.Contains(makefile, tt.want) {
				t.Fatalf("expected Makefile to contain %q\nMakefile:\n%s", tt.want, makefile)
			}
		})
	}
}

func mustMakeTargetPrereqs(t *testing.T, makefile string, target string) map[string]struct{} {
	t.Helper()

	for _, line := range strings.Split(makefile, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "\t") {
			continue
		}

		name, deps, ok := strings.Cut(trimmed, ":")
		if !ok || strings.TrimSpace(name) != target {
			continue
		}

		prereqs := make(map[string]struct{})
		for _, dep := range strings.Fields(deps) {
			prereqs[dep] = struct{}{}
		}
		return prereqs
	}

	t.Fatalf("expected Makefile target %q\nMakefile:\n%s", target, makefile)
	return nil
}

func readRepoMakefile(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}

	makefilePath := filepath.Join(filepath.Dir(currentFile), "..", "..", "Makefile")
	content, err := os.ReadFile(filepath.Clean(makefilePath))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	return string(content)
}
