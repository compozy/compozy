package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPublishExtensionSDKsTargetRequiresVerificationAndPublicAccess(t *testing.T) {
	makefile := readRepoMakefile(t)

	if !strings.Contains(makefile, "publish-extension-sdks: verify build-extension-sdks") {
		t.Fatalf("expected publish target to depend on verify and build-extension-sdks\nMakefile:\n%s", makefile)
	}
	for _, want := range []string{
		"npm publish --workspace @compozy/extension-sdk --access public",
		"npm publish --workspace @compozy/create-extension --access public",
	} {
		if !strings.Contains(makefile, want) {
			t.Fatalf("expected Makefile to contain %q\nMakefile:\n%s", want, makefile)
		}
	}
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
