package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kballard/go-shellquote"
)

func TestLiveConfig(t *testing.T) {
	t.Parallel()

	overrides := func() []liveOverride {
		return []liveOverride{{table: []string{"providers", "claude"}, key: "command", injected: "mock"}}
	}

	t.Run("Should restore the original bytes when nothing else changed", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "config.toml")
		original := "# keep this comment\n[http]\nport = 47931\n"
		// A config the CLI wrote is not always private, and live mode must still replace it.
		if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		restore, err := applyLiveConfig(path, overrides())
		if err != nil {
			t.Fatalf("applyLiveConfig() error = %v", err)
		}
		if err := restore(); err != nil {
			t.Fatalf("restore() error = %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(got) != original {
			t.Fatalf("config = %q, want the original bytes", got)
		}
	})

	t.Run("Should keep operator edits made while live mode ran", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte("[http]\nport = 47931\n"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		restore, err := applyLiveConfig(path, overrides())
		if err != nil {
			t.Fatalf("applyLiveConfig() error = %v", err)
		}
		during, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		edited := string(during) + "\n[shell.sessions]\ngroup_by = \"project\"\n"
		if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
			t.Fatalf("WriteFile(edit) error = %v", err)
		}
		if err := restore(); err != nil {
			t.Fatalf("restore() error = %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if !strings.Contains(string(got), "group_by") || strings.Contains(string(got), "mock") {
			t.Fatalf("config = %q, want the operator edit kept and the mock command gone", got)
		}
	})
}

func TestMockProviderCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should keep paths with spaces as single arguments", func(t *testing.T) {
		t.Parallel()

		command := mockProviderCommand("/tmp/my tools/driver", "/tmp/my home/fixture.json", "agent", "claude")
		argv, err := shellquote.Split(command)
		if err != nil {
			t.Fatalf("Split() error = %v", err)
		}
		if argv[0] != "/tmp/my tools/driver" || argv[2] != "/tmp/my home/fixture.json" {
			t.Fatalf("argv = %#v, want both paths intact", argv)
		}
	})
}
