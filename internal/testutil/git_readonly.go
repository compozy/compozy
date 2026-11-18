package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ConfigureGitReadMarkers installs repository-configured programs that leave a marker if executed.
func ConfigureGitReadMarkers(t *testing.T, root string) string {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "marker")
	script := filepath.Join(t.TempDir(), "evil.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf ran > '"+marker+"'\n"), 0600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// #nosec G302 -- the owner-only test fixture must be executable by Git.
	if err := os.Chmod(script, 0700); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"core.fsmonitor", "filter.evil.clean", "filter.evil.process", "filter.evil.smudge",
		"diff.evil.textconv", "diff.external", "core.pager",
	} {
		cmd := exec.CommandContext(t.Context(), "git", "config", key, script)
		cmd.Dir = root
		data, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("configure git: %v: %s", err, data)
		}
	}
	if err := os.WriteFile(
		filepath.Join(root, ".gitattributes"),
		[]byte("* filter=evil diff=evil\n"),
		0600,
	); err != nil {
		t.Fatal(err)
	}
	return marker
}
