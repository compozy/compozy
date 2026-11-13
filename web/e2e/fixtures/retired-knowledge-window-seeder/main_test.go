package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/compozy/compozy/internal/windowmanager/statekeys"
)

func TestRawSnapshotSeeding(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		key   string
		flags []string
	}{
		{name: "default snapshot", key: statekeys.Snapshot("raw-profile")},
		{name: "explicit snapshot", key: statekeys.Snapshot("raw-profile"), flags: []string{"--raw-key", "snapshot"}},
		{name: "legacy", key: statekeys.Legacy("raw-profile"), flags: []string{"--raw-key", "legacy"}},
	} {
		t.Run("Should persist unchanged bytes under the "+test.name+" key", func(t *testing.T) {
			t.Parallel()
			homeDir := t.TempDir()
			rawFile := filepath.Join(t.TempDir(), "broken-v5.json")
			args := []string{
				"--home", homeDir,
				"--workspace", string(windowmanager.GlobalDesktopWorkspaceID),
				"--profile", "raw-profile",
				"--raw-snapshot", rawFile,
			}
			args = append(args, test.flags...)
			for index, raw := range [][]byte{
				[]byte(" \n{\"version\":5,\"windows\":{\"broken\":{}}}\n"),
				[]byte("{ \"windows\": {\"broken\": {}}, \"version\": 5 }\n\n"),
			} {
				if err := os.WriteFile(rawFile, raw, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := run(t.Context(), args); err != nil {
					t.Fatalf("run raw seeder: %v", err)
				}
				entries := readSeededEntries(t, homeDir)
				if len(entries) != 1 {
					t.Fatalf("stored entries = %+v, want only %q", entries, test.key)
				}
				entry := entries[0]
				if entry.Key != test.key || !bytes.Equal(entry.Value, raw) || entry.Rev != uint64(index+1) {
					t.Fatalf("stored key=%q value=%q rev=%d, want key=%q value=%q rev=%d",
						entry.Key, entry.Value, entry.Rev, test.key, raw, index+1)
				}
			}
		})
	}
}

func TestRawSnapshotOptions(t *testing.T) {
	t.Parallel()

	t.Run("Should reject every explicitly supplied layout-building flag", func(t *testing.T) {
		t.Parallel()
		for _, option := range []struct{ name, value string }{
			{"session-window", "session"},
			{"knowledge-window", "knowledge"},
			{"floating", ""},
			{"minimized", "window"},
			{"snapped", "left,right"},
			{"pinned", "window"},
			{"x", "0.55"},
			{"y", "0.1"},
			{"width", "0.4"},
			{"height", "0.7"},
		} {
			t.Run("Should reject --"+option.name, func(t *testing.T) {
				t.Parallel()
				_, err := parseOptions([]string{
					"--home", t.TempDir(),
					"--workspace", string(windowmanager.GlobalDesktopWorkspaceID),
					"--raw-snapshot", "broken-v5.json",
					"--" + option.name, option.value,
				})
				if err == nil || !strings.Contains(err.Error(), "cannot be combined with layout-building flags") ||
					!strings.Contains(err.Error(), "--"+option.name) {
					t.Fatalf("parseOptions error = %v, want a conflict naming --%s", err, option.name)
				}
			})
		}
	})

	t.Run("Should reject invalid raw mode selections", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name  string
			flags []string
			want  string
		}{
			{name: "key without file", flags: []string{"--raw-key", "legacy"}, want: "--raw-key requires --raw-snapshot"},
			{name: "empty file", flags: []string{"--raw-snapshot", ""}, want: "--raw-snapshot requires a file"},
			{name: "unknown key", flags: []string{"--raw-snapshot", "broken-v5.json", "--raw-key", "other"}, want: "--raw-key must be snapshot or legacy"},
		} {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				t.Parallel()
				args := []string{"--home", t.TempDir(), "--workspace", string(windowmanager.GlobalDesktopWorkspaceID)}
				_, err := parseOptions(append(args, test.flags...))
				if err == nil || err.Error() != test.want {
					t.Fatalf("parseOptions error = %v, want %q", err, test.want)
				}
			})
		}
	})
}

func readSeededEntries(t *testing.T, homeDir string) []clientstate.Entry {
	t.Helper()
	engine, err := clientstate.Open(
		t.Context(), clientstate.DatabasePath(homeDir), &workspaceResolver{}, clientstate.DefaultLimits(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	}()
	entries, err := engine.List(
		t.Context(),
		clientstate.WorkspaceID(windowmanager.GlobalDesktopWorkspaceID),
		"window_manager",
	)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
