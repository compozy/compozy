package packaging

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The tests run with the working directory set to this package, so the extension
// root (holding extension.toml and README.md) is one level up.
const extensionRoot = ".."

var (
	readmePath   = filepath.Join(extensionRoot, "README.md")
	skillDirPath = filepath.Join(extensionRoot, "skills", "cy-capture-decisions")
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// readmeSection returns the body of the first "## " section whose heading contains
// headingSubstr, from that heading up to the next "## " heading (or end of file).
func readmeSection(t *testing.T, headingSubstr string) string {
	t.Helper()
	lines := strings.Split(readFile(t, readmePath), "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") && strings.Contains(line, headingSubstr) {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("README has no '## ' section containing %q", headingSubstr)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// writeUnder writes content to root/rel, creating parent directories as needed.
func writeUnder(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open %s for append: %v", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatalf("close %s: %v", path, closeErr)
		}
	}()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("append %s: %v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func dirNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// gitEnv isolates git from the developer's global/system configuration so
// check-ignore reflects only the repository's own .gitignore.
func gitEnv(home string) []string {
	return append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME="+home,
	)
}

func initRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	cmd := exec.CommandContext(t.Context(), "git", "init", "-q", repo)
	cmd.Env = gitEnv(repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return repo
}
