package providerexec

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func writeOpenCodeFixture(t *testing.T, dir string, name string, executable bool) string {
	t.Helper()
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	contents := "#!/bin/sh\nexit 0\n"
	perm := os.FileMode(0o755)
	if !executable {
		perm = os.FileMode(0o644)
	}
	if err := os.WriteFile(path, []byte(contents), perm); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	if runtime.GOOS != "windows" && executable {
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatalf("Chmod(%q) error = %v", path, err)
		}
	}
	return path
}

func TestResolveOpenCodeCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should prefer installed native for inherited default", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		native := writeOpenCodeFixture(t, dir, "opencode", true)
		writeOpenCodeFixture(t, dir, "npx", true)
		provider := compozyconfig.ProviderConfig{Command: "opencode acp", CommandIsBuiltin: true}
		got, err := ResolveOpenCodeCommand(provider, []string{"PATH=" + dir}, dir)
		if err != nil {
			t.Fatalf("ResolveOpenCodeCommand() error = %v", err)
		}
		if got.Executable != filepath.Clean(native) {
			t.Fatalf("Executable = %q, want %q", got.Executable, native)
		}
		if len(got.Args) != 1 || got.Args[0] != "acp" {
			t.Fatalf("Args = %#v, want [acp]", got.Args)
		}
		if len(got.Environment) != 0 {
			t.Fatalf("Environment = %#v, want empty", got.Environment)
		}
	})

	t.Run("Should use npm only when native is genuinely absent", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		npx := writeOpenCodeFixture(t, dir, "npx", true)
		provider := compozyconfig.ProviderConfig{Command: "opencode acp", CommandIsBuiltin: true}
		got, err := ResolveOpenCodeCommand(provider, []string{"PATH=" + dir}, dir)
		if err != nil {
			t.Fatalf("ResolveOpenCodeCommand() error = %v", err)
		}
		if got.Executable != filepath.Clean(npx) {
			t.Fatalf("Executable = %q, want %q", got.Executable, npx)
		}
		want := []string{"-y", "opencode-ai@latest", "acp"}
		if strings.Join(got.Args, "\x1f") != strings.Join(want, "\x1f") {
			t.Fatalf("Args = %#v, want %#v", got.Args, want)
		}
	})

	t.Run("Should fail without shell execution when neither is present", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		provider := compozyconfig.ProviderConfig{Command: "opencode acp", CommandIsBuiltin: true}
		_, err := ResolveOpenCodeCommand(provider, []string{"PATH=" + dir}, dir)
		if !errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("error = %v, want exec.ErrNotFound", err)
		}
	})

	t.Run("Should never fallback on permission failure", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("permission bits are not enforced on Windows")
		}
		dir := t.TempDir()
		writeOpenCodeFixture(t, dir, "opencode", false)
		writeOpenCodeFixture(t, dir, "npx", true)
		provider := compozyconfig.ProviderConfig{Command: "opencode acp", CommandIsBuiltin: true}
		_, err := ResolveOpenCodeCommand(provider, []string{"PATH=" + dir}, dir)
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("error = %v, want os.ErrPermission", err)
		}
		if errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("error = %v, must not collapse permission into not-found", err)
		}
	})

	t.Run("Should not grant npm fallback to explicit commands", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeOpenCodeFixture(t, dir, "npx", true)
		provider := compozyconfig.ProviderConfig{Command: "opencode acp"}
		_, err := ResolveOpenCodeCommand(provider, []string{"PATH=" + dir}, dir)
		if !errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("error = %v, want exec.ErrNotFound", err)
		}
	})

	t.Run("Should preserve explicit selectors and pinned versions", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		native := writeOpenCodeFixture(t, dir, "opencode", true)
		npx := writeOpenCodeFixture(t, dir, "npx", true)
		absolute, err := ResolveOpenCodeCommand(
			compozyconfig.ProviderConfig{Command: native + " acp"},
			[]string{"PATH=" + dir},
			dir,
		)
		if err != nil {
			t.Fatalf("absolute Resolve error = %v", err)
		}
		if absolute.Executable != filepath.Clean(native) {
			t.Fatalf("absolute Executable = %q, want %q", absolute.Executable, native)
		}
		pinned, err := ResolveOpenCodeCommand(
			compozyconfig.ProviderConfig{Command: "npx -y opencode-ai@1.18.33 acp"},
			[]string{"PATH=" + dir},
			dir,
		)
		if err != nil {
			t.Fatalf("pinned Resolve error = %v", err)
		}
		if pinned.Executable != filepath.Clean(npx) {
			t.Fatalf("pinned Executable = %q, want %q", pinned.Executable, npx)
		}
		want := []string{"-y", "opencode-ai@1.18.33", "acp"}
		if strings.Join(pinned.Args, "\x1f") != strings.Join(want, "\x1f") {
			t.Fatalf("pinned Args = %#v, want %#v", pinned.Args, want)
		}
	})

	t.Run("Should apply private assignments before lookup", func(t *testing.T) {
		t.Parallel()
		binDir := t.TempDir()
		native := writeOpenCodeFixture(t, binDir, "opencode", true)
		cwd := t.TempDir()
		provider := compozyconfig.ProviderConfig{
			Command:          "PATH=" + binDir + " opencode acp",
			CommandIsBuiltin: true,
		}
		got, err := ResolveOpenCodeCommand(provider, []string{"PATH=" + cwd}, cwd)
		if err != nil {
			t.Fatalf("ResolveOpenCodeCommand() error = %v", err)
		}
		if got.Executable != filepath.Clean(native) {
			t.Fatalf("Executable = %q, want %q", got.Executable, native)
		}
		if len(got.Environment) != 1 || got.Environment[0] != "PATH="+binDir {
			t.Fatalf("Environment = %#v, want PATH assignment", got.Environment)
		}
	})
}

func TestDeriveOpenCodeDiscoveryBase(t *testing.T) {
	t.Parallel()

	t.Run("Should derive native base without dropping prefix", func(t *testing.T) {
		t.Parallel()
		base, err := DeriveOpenCodeDiscoveryBase(compozyconfig.LaunchCommand{
			Executable: "/opt/oc-v1/opencode",
			Args:       []string{"acp"},
		})
		if err != nil {
			t.Fatalf("Derive error = %v", err)
		}
		if base.Executable != "/opt/oc-v1/opencode" || len(base.Args) != 0 {
			t.Fatalf("base = %#v, want executable with empty prefix", base)
		}
	})

	t.Run("Should derive npm base preserving package selector", func(t *testing.T) {
		t.Parallel()
		base, err := DeriveOpenCodeDiscoveryBase(compozyconfig.LaunchCommand{
			Executable:  "/usr/bin/npx",
			Args:        []string{"-y", "opencode-ai@1.18.33", "acp"},
			Environment: []string{"PATH=/x"},
		})
		if err != nil {
			t.Fatalf("Derive error = %v", err)
		}
		want := []string{"-y", "opencode-ai@1.18.33"}
		if strings.Join(base.Args, "\x1f") != strings.Join(want, "\x1f") {
			t.Fatalf("base Args = %#v, want %#v", base.Args, want)
		}
		if len(base.Environment) != 1 {
			t.Fatalf("base Environment = %#v, want preserved assignments", base.Environment)
		}
	})

	t.Run("Should derive Windows npm shims like their Unix counterparts", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name       string
			args       []string
			wantPrefix []string
		}{
			{name: "opencode.exe", args: []string{"acp"}},
			{name: "opencode.cmd", args: []string{"acp"}},
			{name: "opencode.bat", args: []string{"acp"}},
			{name: "opencode.com", args: []string{"acp"}},
			{
				name:       "npx.cmd",
				args:       []string{"-y", "opencode-ai@1.18.33", "acp"},
				wantPrefix: []string{"-y", "opencode-ai@1.18.33"},
			},
		} {
			executable := filepath.Join(t.TempDir(), test.name)
			base, err := DeriveOpenCodeDiscoveryBase(compozyconfig.LaunchCommand{
				Executable: executable,
				Args:       test.args,
			})
			if err != nil {
				t.Fatalf("Derive(%q) error = %v, want the shim name accepted", test.name, err)
			}
			if base.Executable != executable {
				t.Fatalf("Derive(%q) executable = %q, want %q", test.name, base.Executable, executable)
			}
			if got := strings.Join(base.Args, "\x1f"); got != strings.Join(test.wantPrefix, "\x1f") {
				t.Fatalf("Derive(%q) args = %#v, want %#v", test.name, base.Args, test.wantPrefix)
			}
		}
	})

	t.Run("Should require explicit override for opaque wrappers", func(t *testing.T) {
		t.Parallel()
		for _, cmd := range []compozyconfig.LaunchCommand{
			{Executable: "/usr/bin/custom-acp-wrapper", Args: nil},
			{Executable: "/usr/bin/opencode", Args: []string{"acp", "--wrapper-only"}},
			{Executable: "/usr/bin/custom-acp-wrapper", Args: []string{"acp"}},
		} {
			_, err := DeriveOpenCodeDiscoveryBase(cmd)
			if err == nil || !strings.Contains(
				err.Error(),
				"requires an explicit models.discovery.command or models.discovery.endpoint",
			) {
				t.Fatalf("Derive(%#v) error = %v, want explicit-override guidance", cmd, err)
			}
		}
	})
}

func TestOpenCodeInvocationFingerprint(t *testing.T) {
	t.Parallel()

	base := compozyconfig.LaunchCommand{Executable: "/usr/bin/opencode", Args: []string{"acp"}}
	builtin := OpenCodeInvocationFingerprint(base, true)
	authored := OpenCodeInvocationFingerprint(base, false)
	if builtin == "" || builtin == authored {
		t.Fatalf("fingerprint builtin=%q authored=%q, want distinct non-empty", builtin, authored)
	}
	withAssign := compozyconfig.LaunchCommand{
		Executable:  "/usr/bin/opencode",
		Args:        []string{"acp"},
		Environment: []string{"PATH=/x"},
	}
	if OpenCodeInvocationFingerprint(withAssign, true) == builtin {
		t.Fatal("assignment change reused invocation fingerprint")
	}
	// shortcut: fingerprint is a hash, not reversible; rotation of the npm tag
	// changes Args and therefore the fingerprint on the next refresh.
}
